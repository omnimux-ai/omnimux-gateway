package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSunburstAcceptedReceiptRecoversOriginalAfterInterruptedHandoff(t *testing.T) {
	for _, synchronous := range []bool{true, false} {
		for _, cancelled := range []bool{true, false} {
			name := "asynchronous"
			if synchronous {
				name = "synchronous"
			}
			if cancelled {
				name += "/cancelled"
			} else {
				name += "/handoff_database_failed"
			}
			t.Run(name, func(t *testing.T) {
				const original = "https://images.example/true-original-asset.png"
				var posts, gets atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodPost {
						posts.Add(1)
						if synchronous {
							_, _ = io.WriteString(w, `{"data":[{"url":"https://images.example/true-original-asset.png"}]}`)
						} else {
							_, _ = io.WriteString(w, `{"id":"real-upstream-before-handoff","status":"pending"}`)
						}
						return
					}
					gets.Add(1)
					assert.False(t, synchronous, "a synchronous original URL must not trigger supplier task queries")
					assert.Equal(t, "/v1/tasks/real-upstream-before-handoff", r.URL.Path)
					_, _ = io.WriteString(w, `{"id":"real-upstream-before-handoff","status":"completed","results":["https://images.example/true-original-asset.png"]}`)
				}))
				defer upstream.Close()
				f := newSunburstRawPrimitiveFixture(t, upstream.URL, ":no-backup")
				scanner := &model.SystemTask{TaskID: "native-scan"}
				f.task.Properties.OriginModelName = influencerImageModel
				f.task.PrivateData.GenerationQueue.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text"}
				require.NoError(t, f.db.Model(&model.Task{}).Where("id = ?", f.task.ID).Updates(map[string]any{"properties": f.task.Properties, "private_data": f.task.PrivateData}).Error)
				f.c.Set("generation_queue_reserved_task", f.task)
				f.c.Set("generation_queue_scanner", scanner)
				f.c.Set("generation_queue_runner", "runner")
				f.c.Set(ctxInfluencerImageInputs, f.task.PrivateData.GenerationQueue.ImageInputs)
				ctx, cancel := context.WithCancel(f.c.Request.Context())
				defer cancel()
				f.c.Request = f.c.Request.WithContext(ctx)
				if !cancelled {
					require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register("test_sunburst_handoff_failure", func(tx *gorm.DB) {
						if row, ok := tx.Statement.Dest.(*model.Task); ok && row.Status != model.TaskStatusDispatching {
							tx.AddError(errors.New("injected final handoff write failure"))
						}
					}))
				}
				outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError) {
					result, failure := relay.RelayTaskSubmit(c, info)
					if cancelled && failure == nil && result != nil {
						cancel()
					}
					return result, failure
				})
				if !cancelled {
					require.NoError(t, f.db.Callback().Update().Remove("test_sunburst_handoff_failure"))
				}
				require.Nil(t, outcome)
				require.NotNil(t, taskErr)
				var saved model.Task
				require.NoError(t, f.db.First(&saved, f.task.ID).Error)
				assert.EqualValues(t, model.TaskStatusSubmitted, saved.Status, "a completed worker immediately hands its durable original receipt to polling")
				require.NotNil(t, saved.PrivateData.GenerationAcceptedResult)
				require.NotNil(t, saved.PrivateData.GenerationRouting)
				assert.Equal(t, model.GenerationAttemptAccepted, saved.PrivateData.GenerationRouting.Attempts[0].Outcome)
				if synchronous {
					assert.Equal(t, original, saved.PrivateData.ResultURL, "original output must cross the durable receipt barrier")
				}
				require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "runner"))
				require.NoError(t, f.db.First(&saved, f.task.ID).Error)
				assert.EqualValues(t, model.TaskStatusSubmitted, saved.Status)
				assert.Nil(t, saved.PrivateData.GenerationQueue)
				assert.True(t, saved.PrivateData.GenerationReservationUnlogged)
				require.NotNil(t, saved.PrivateData.ImageInputs)
				oldFactory, oldLimit := service.GetTaskAdaptorFunc, constant.TaskQueryLimit
				service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
				constant.TaskQueryLimit = 100
				t.Cleanup(func() { service.GetTaskAdaptorFunc, constant.TaskQueryLimit = oldFactory, oldLimit })
				service.RunTaskPollingOnce(context.Background(), nil)
				require.NoError(t, f.db.First(&saved, f.task.ID).Error)
				assert.EqualValues(t, model.TaskStatusSuccess, saved.Status)
				assert.Equal(t, original, saved.PrivateData.ResultURL)
				assert.False(t, saved.PrivateData.ImageResultUnverified)
				assert.Equal(t, int32(1), posts.Load())
				if synchronous {
					assert.Zero(t, gets.Load())
				} else {
					assert.Equal(t, int32(1), gets.Load())
				}
				// A repeated restart/scanner pass cannot submit or consume again.
				require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "runner"))
				service.RunTaskPollingOnce(context.Background(), nil)
				assert.Equal(t, int32(1), posts.Load())
				var user model.User
				require.NoError(t, f.db.First(&user, f.user.Id).Error)
				assert.Equal(t, 95000, user.Quota)
			})
		}
	}
}
