package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSunburstMultikeyIdentitySurvivesHandoffRecoveryAndChannelChanges(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		for _, sameGroupFallback := range []bool{false, true} {
			t.Run(fmt.Sprintf("interrupted=%t/same_group_fallback=%t", interrupted, sameGroupFallback), func(t *testing.T) {
				const selected = "fixture-original-account"
				var posts, gets atomic.Int32
				var f *nativeFailoverFixture
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "Bearer "+selected, r.Header.Get("Authorization"))
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodPost {
						posts.Add(1)
						var beforeSend model.Task
						if assert.NoError(t, f.db.First(&beforeSend, f.task.ID).Error) {
							assert.Equal(t, selected, beforeSend.PrivateData.Key, "the selected identity is durable before the first supplier byte")
						}
						_, _ = io.WriteString(w, `{"id":"identity-bound-original","status":"pending"}`)
						return
					}
					gets.Add(1)
					assert.Equal(t, "/v1/tasks/identity-bound-original", r.URL.Path)
					_, _ = io.WriteString(w, `{"id":"identity-bound-original","status":"completed","results":["https://images.example/identity-bound.png"]}`)
				}))
				defer upstream.Close()
				primaryURL, backupURL := upstream.URL, ":not-used"
				if sameGroupFallback {
					primaryURL, backupURL = ":known-unsent-invalid-url", upstream.URL
				}
				f = newSunburstRawPrimitiveFixture(t, primaryURL, backupURL)
				channel := &f.primary
				if sameGroupFallback {
					channel = &f.backup
					channel.Group = "default"
					require.NoError(t, f.db.Model(channel).Update("group", channel.Group).Error)
					require.NoError(t, f.db.Model(&model.Ability{}).Where("channel_id = ?", channel.Id).Update("group", channel.Group).Error)
					f.task.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{"default"}, MaxAttempts: 2}
					f.info.TokenGroup = "default"
				}
				channel.Key = selected + "\nfixture-other-account"
				channel.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModeRandom, MultiKeyStatusList: map[int]int{0: common.ChannelStatusEnabled, 1: common.ChannelStatusManuallyDisabled}}
				require.NoError(t, f.db.Model(channel).Updates(map[string]any{"key": channel.Key, "channel_info": channel.ChannelInfo}).Error)
				require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, influencerImageModel))
				f.task.Properties.OriginModelName = influencerImageModel
				f.task.PrivateData.GenerationQueue.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text"}
				require.NoError(t, f.db.Model(f.task).Updates(map[string]any{"properties": f.task.Properties, "private_data": f.task.PrivateData}).Error)
				scanner := &model.SystemTask{TaskID: "native-scan"}
				f.c.Set("generation_queue_reserved_task", f.task)
				f.c.Set("generation_queue_scanner", scanner)
				f.c.Set("generation_queue_runner", "runner")
				f.c.Set(ctxInfluencerImageInputs, f.task.PrivateData.GenerationQueue.ImageInputs)
				ctx, cancel := context.WithCancel(f.c.Request.Context())
				defer cancel()
				f.c.Request = f.c.Request.WithContext(ctx)
				outcome, failure := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError) {
					result, taskErr := relay.RelayTaskSubmit(c, info)
					if interrupted && taskErr == nil && result != nil {
						cancel()
					}
					return result, taskErr
				})
				if interrupted {
					require.Nil(t, outcome)
					require.NotNil(t, failure)
				} else {
					require.Nil(t, failure)
					require.NotNil(t, outcome)
				}
				var saved model.Task
				require.NoError(t, f.db.First(&saved, f.task.ID).Error)
				assert.Equal(t, selected, saved.PrivateData.Key)
				require.NotNil(t, saved.PrivateData.GenerationRouting)
				wantAttempts := 1
				if sameGroupFallback {
					wantAttempts = 2
					assert.Equal(t, model.GenerationAttemptRejected, saved.PrivateData.GenerationRouting.Attempts[0].Outcome)
					assert.NotEqual(t, saved.PrivateData.GenerationRouting.Attempts[0].ChannelID, saved.PrivateData.GenerationRouting.Attempts[1].ChannelID)
				}
				assert.Len(t, saved.PrivateData.GenerationRouting.Attempts, wantAttempts)
				// Fresh polling after a channel/key rotation must use the stored account,
				// even if that channel is now disabled for new generation selection.
				require.NoError(t, f.db.Model(channel).Updates(map[string]any{"key": "fixture-rotated-account", "status": common.ChannelStatusManuallyDisabled}).Error)
				oldFactory, oldLimit := service.GetTaskAdaptorFunc, constant.TaskQueryLimit
				service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
				constant.TaskQueryLimit = 100
				t.Cleanup(func() { service.GetTaskAdaptorFunc, constant.TaskQueryLimit = oldFactory, oldLimit })
				require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "runner"))
				service.RunTaskPollingOnce(context.Background(), nil)
				require.NoError(t, f.db.First(&saved, f.task.ID).Error)
				assert.EqualValues(t, model.TaskStatusSuccess, saved.Status)
				assert.Equal(t, selected, saved.PrivateData.Key)
				assert.Equal(t, "identity-bound-original", saved.PrivateData.UpstreamTaskID)
				assert.Equal(t, int32(1), posts.Load())
				assert.Equal(t, int32(1), gets.Load())
				service.RunTaskPollingOnce(context.Background(), nil)
				assert.Equal(t, int32(1), posts.Load())
				assert.Equal(t, int32(1), gets.Load())
				public, err := common.Marshal(tasksToDto([]*model.Task{&saved}, false, common.RoleCommonUser))
				require.NoError(t, err)
				assert.NotContains(t, string(public), selected)
				assert.NotContains(t, string(public), "fixture-other-account")
				assert.NotContains(t, string(public), "fixture-rotated-account")
				var user model.User
				require.NoError(t, f.db.First(&user, f.user.Id).Error)
				assert.Equal(t, 95000, user.Quota)
			})
		}
	}
}
