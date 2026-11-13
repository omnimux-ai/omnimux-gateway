package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Reuses the native queue, account, funding and route fixture. Only the model's
// explicit input/delivery contract is replaced; no retry outcome is fabricated.
func nativeInlineFailoverFixture(t *testing.T, primaryURL, backupURL, userGroup, funding string) *nativeFailoverFixture {
	t.Helper()
	f := newNativeFailoverFixture(t, primaryURL, backupURL, false)
	common.LogConsumeEnabled = true
	// Native cache jobs must finish before the base fixture restores globals.
	// Keep unrelated low-balance notifications out of this sufficient-balance test.
	previousThreshold := common.QuotaRemindThreshold
	previousQueryLimit := constant.TaskQueryLimit
	constant.TaskQueryLimit = 100
	common.QuotaRemindThreshold = 0
	t.Cleanup(func() {
		require.Eventually(t, func() bool { return gopool.WorkerCount() == 0 }, 5*time.Second, time.Millisecond, "native cache/notification jobs did not drain")
		common.QuotaRemindThreshold = previousThreshold
		constant.TaskQueryLimit = previousQueryLimit
	})

	configureNativeFailoverBilling(t, f, userGroup, funding)
	const group = "gpt-image-2.5-sunburst-image"
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image-2.5-sunburst-image":1}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"gpt-image-2.5-sunburst-image":0.6}}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-image-2.5-sunburst-image":"Image"}`))
	withTieredBillingConfig(t, map[string]string{influencerImageModel: "tiered_expr"}, map[string]string{influencerImageModel: `tier("request", fixed(0.01))`})
	for _, ch := range []*model.Channel{&f.primary, &f.backup} {
		ch.Type = constant.ChannelTypeOpenAI
		ch.Group = group
		ch.Models = influencerImageModel
		ch.ModelMapping = common.GetPointer(`{"gpt-image-2.5-sunburst":"gpt-image-2.5-sunburst-1k"}`)
		ch.OtherSettings = `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"],"delivery":"b64_json"}}`
		require.NoError(t, f.db.Save(ch).Error)
		require.NoError(t, f.db.Model(&model.Ability{}).Where("channel_id = ?", ch.Id).Updates(map[string]any{"group": group, "model": influencerImageModel}).Error)
	}
	const body = `{"model":"gpt-image-2.5-sunburst","group":"gpt-image-2.5-sunburst-image","image_mode":"text","prompt":"A single blue kettle on white.","size":"1024x576","quality":"high","n":1,"output_format":"png","response_format":"url"}`
	inputs := &model.GenerationImageInputs{Version: 1, Mode: "text"}
	f.task.Group = group
	f.task.Properties.OriginModelName = influencerImageModel
	f.task.PrivateData.GenerationQueue.Body = []byte(body)
	f.task.PrivateData.GenerationQueue.ImageInputs = inputs
	f.task.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{group}, MaxAttempts: 2}
	require.NoError(t, f.db.Save(f.task).Error)
	common.CleanupBodyStorage(f.c)
	f.c.Set(common.KeyRequestBody, []byte(body))
	f.c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	f.c.Request.Header.Set("Content-Type", "application/json")
	f.c.Set("influencer_image_inputs", inputs)
	f.c.Set("influencer_image_mode", "text")
	f.c.Set(helper.ImageTaskRequestSizeContextKey, "1024x576")
	common.SetContextKey(f.c, constant.ContextKeyTokenAutoGroups, []string{group})
	common.SetContextKey(f.c, constant.ContextKeyAutoGroup, group)
	require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, influencerImageModel))
	f.info.UserQuota = f.user.Quota
	f.info.OriginModelName = influencerImageModel
	f.info.TokenGroup = group
	f.info.UsingGroup = group
	oldSecret, oldAddress, oldPublic := common.CryptoSecret, system_setting.ServerAddress, system_setting.TaskPublicAddress
	previousDriver := storage.GetStorageDriver()
	common.CryptoSecret = "native-inline-test-secret"
	system_setting.ServerAddress = "https://native-images.example"
	system_setting.TaskPublicAddress = ""
	storage.SetGlobalDriver(newInfluencerPresetTestDriver())
	t.Cleanup(func() {
		common.CryptoSecret = oldSecret
		system_setting.ServerAddress = oldAddress
		system_setting.TaskPublicAddress = oldPublic
		storage.SetGlobalDriver(previousDriver)
		common.CleanupBodyStorage(f.c)
	})
	return f
}

func nativeInlineResponse(t *testing.T) ([]byte, []byte) {
	t.Helper()
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 7, 4))))
	response, err := common.Marshal(map[string]any{"id": "supplier-id-is-not-the-task-key", "data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(imageBytes.Bytes())}}})
	require.NoError(t, err)
	return response, imageBytes.Bytes()
}

func TestNativeInlineFailoverKeepsOriginalFundingAndSingleArtifact(t *testing.T) {
	response, original := nativeInlineResponse(t)
	for _, userGroup := range []string{"default", "member"} {
		for _, funding := range []string{"wallet", "subscription"} {
			for _, fallback := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/fallback=%t", userGroup, funding, fallback), func(t *testing.T) {
					var posts, otherRequests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
							otherRequests.Add(1)
							w.WriteHeader(http.StatusNotFound)
							return
						}
						posts.Add(1)
						var sent map[string]any
						require.NoError(t, common.DecodeJson(r.Body, &sent))
						assert.Equal(t, "gpt-image-2.5-sunburst-1k", sent["model"])
						assert.Equal(t, "b64_json", sent["response_format"])
						assert.Equal(t, "1024x576", sent["size"])
						assert.Equal(t, "high", sent["quality"])
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write(response)
					}))
					defer server.Close()
					primaryURL := server.URL
					if fallback {
						primaryURL = ":invalid-native-inline-primary"
					}
					f := nativeInlineFailoverFixture(t, primaryURL, server.URL, userGroup, funding)
					if fallback {
						// The first candidate uses URL delivery; only the new candidate's
						// independent wire body requests inline output.
						f.primary.OtherSettings = strings.Replace(f.primary.OtherSettings, `"delivery":"b64_json"`, `"delivery":"url"`, 1)
						require.NoError(t, f.db.Save(&f.primary).Error)
						require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, influencerImageModel))
					}
					writes := observeNativeFailoverFunding(t, f)
					var quotes []*generationSubmissionQuote
					var sessions []relaycommon.BillingSettler
					firstNotSent := false
					outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
						result, err := relay.RelayTaskSubmit(c, info)
						snapshot, snapshotErr := taskFundingSnapshot(c, info, constant.TaskPlatform("image_task_v1"))
						require.NoError(t, snapshotErr)
						quotes = append(quotes, &generationSubmissionQuote{Quota: snapshot.Quota, Billing: snapshot.PrivateData.BillingContext})
						sessions = append(sessions, info.Billing)
						if info.ChannelId == f.primary.Id {
							firstNotSent = err != nil && err.NoTaskCreated
						}
						return result, err
					})
					require.Nil(t, taskErr)
					require.NotNil(t, outcome)
					assert.Equal(t, f.task.TaskID, outcome.Task.TaskID)
					assert.Equal(t, f.task.ID, outcome.Task.ID)
					assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), outcome.Task.Status)
					wantQuota, wantAttempts := 5000, 1
					if userGroup == "member" {
						wantQuota = 3000
					}
					if fallback {
						wantAttempts = 2
						assert.Equal(t, f.backup.Id, outcome.Task.ChannelId)
					} else {
						assert.Equal(t, f.primary.Id, outcome.Task.ChannelId)
					}
					assert.Equal(t, wantQuota, outcome.Task.Quota)
					assert.Equal(t, fallback, firstNotSent)
					require.Len(t, quotes, wantAttempts)
					if fallback {
						assert.True(t, quotes[0].Matches(quotes[1].Quota, quotes[1].Billing))
						assert.Same(t, sessions[0], sessions[1], "fallback must reuse the same real native reservation")
					}
					assert.Equal(t, int32(1), posts.Load())
					assert.Zero(t, otherRequests.Load(), "inline completion must never fetch the supplier-generated public id")
					stored, _, err := service.ReadTaskArtifact(t.Context(), outcome.Task, "image", service.MaxInlineImageTaskBytes)
					require.NoError(t, err)
					assert.Equal(t, original, stored)
					assert.Equal(t, sha256.Sum256(original), sha256.Sum256(stored))
					assert.NotContains(t, string(outcome.Task.Data), "b64_json")
					assert.NotContains(t, string(outcome.Task.Data), "supplier-id-is-not-the-task-key")
					var account model.User
					require.NoError(t, f.db.First(&account, f.user.Id).Error)
					assert.Equal(t, wantQuota, account.UsedQuota)
					assert.Equal(t, 1, account.RequestCount)
					if funding == "wallet" {
						assert.Equal(t, 100000-wantQuota, account.Quota)
						assert.Equal(t, int32(1), writes.walletDebits.Load())
						assert.Zero(t, writes.walletCredits.Load())
					} else {
						var sub model.UserSubscription
						require.NoError(t, f.db.First(&sub, outcome.Task.PrivateData.SubscriptionId).Error)
						assert.EqualValues(t, wantQuota, sub.AmountUsed)
						assert.Equal(t, int32(1), writes.subscriptionReservations.Load())
						assert.Zero(t, writes.subscriptionReleases.Load())
						assert.Equal(t, 100000, account.Quota)
					}
					var taskCount int64
					require.NoError(t, f.db.Model(&model.Task{}).Count(&taskCount).Error)
					assert.EqualValues(t, 1, taskCount)
					var logs []model.Log
					require.NoError(t, f.db.Where("user_id = ? AND type = ?", f.user.Id, model.LogTypeConsume).Find(&logs).Error)
					require.Len(t, logs, 1)
					assert.Equal(t, wantQuota, logs[0].Quota)
					require.NoError(t, service.SettleBilling(f.c, f.info, wantQuota))
					require.NoError(t, service.SettleBilling(f.c, f.info, wantQuota))
					assert.Equal(t, int32(1), posts.Load())
				})
			}
		}
	}
}

func TestNativeInlineUncertainPrimaryNeverUsesBackupAcrossRestart(t *testing.T) {
	for _, scenario := range []string{"timeout", "disconnect", "empty_2xx"} {
		t.Run(scenario, func(t *testing.T) {
			var first, second atomic.Int32
			firstStarted := make(chan struct{})
			serverRelease := make(chan struct{})
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first.Add(1)
				close(firstStarted)
				switch scenario {
				case "timeout":
					select {
					case <-r.Context().Done():
					case <-serverRelease:
					}
					return
				case "disconnect":
					conn, _, err := w.(http.Hijacker).Hijack()
					require.NoError(t, err)
					_ = conn.Close()
					return
				default:
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":[{}]}`)
				}
			}))
			defer primary.Close()
			defer close(serverRelease)
			backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
			defer backup.Close()
			f := nativeInlineFailoverFixture(t, primary.URL, backup.URL, "default", "wallet")
			writes := observeNativeFailoverFunding(t, f)
			if scenario == "timeout" {
				ctx, cancel := context.WithTimeout(f.c.Request.Context(), time.Second)
				defer cancel()
				f.c.Request = f.c.Request.WithContext(ctx)
			}
			_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
			require.NotNil(t, taskErr)
			assert.False(t, taskErr.NoTaskCreated)
			select {
			case <-firstStarted:
			default:
				t.Fatal("primary request never reached the local supplier")
			}
			assert.Equal(t, int32(1), first.Load())
			assert.Zero(t, second.Load())
			var stored model.Task
			require.NoError(t, f.db.First(&stored, f.task.ID).Error)
			assert.Equal(t, model.TaskStatusDispatching, stored.Status)
			assert.Greater(t, stored.PrivateData.GenerationQueue.SendStartedAt, int64(0))
			assert.Nil(t, stored.PrivateData.GenerationAcceptedResult)
			claimed, err := model.ClaimGenerationTasks(t.Context(), f.user.Id, "restarted-inline-scanner")
			require.NoError(t, err)
			assert.Empty(t, claimed)
			scanner, _ := f.c.Get("generation_queue_scanner")
			dispatchInfluencerGeneration(t.Context(), &stored, scanner.(*model.SystemTask), "runner")
			assert.Equal(t, int32(1), first.Load())
			assert.Zero(t, second.Load())
			assert.Equal(t, int32(1), writes.walletDebits.Load())
			assert.Zero(t, writes.walletCredits.Load())
			var user model.User
			require.NoError(t, f.db.First(&user, f.user.Id).Error)
			assert.Equal(t, 95000, user.Quota)
			assert.Zero(t, user.UsedQuota)
		})
	}
}

// This wrapper injects only the existing settler boundary; all reservations,
// successful settlement and refund behavior still delegate to the real native
// BillingSession. No substitute wallet or quota value is introduced.
type inlineSettlementFault struct {
	relaycommon.BillingSettler
	fail  bool
	calls int
}

func (s *inlineSettlementFault) Settle(quota int) error {
	s.calls++
	if s.fail {
		return errors.New("injected native settlement boundary failure")
	}
	return s.BillingSettler.Settle(quota)
}

func TestNativeInlineSavedImageCannotAuthorizeResendAfterReceiptOrFundingFailure(t *testing.T) {
	response, original := nativeInlineResponse(t)
	for _, scenario := range []string{"accepted_receipt_write", "final_handoff", "canceled_after_saved_response", "settlement"} {
		t.Run(scenario, func(t *testing.T) {
			var first, second atomic.Int32
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(response)
			}))
			defer primary.Close()
			backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
			defer backup.Close()
			f := nativeInlineFailoverFixture(t, primary.URL, backup.URL, "default", "wallet")
			writes := observeNativeFailoverFunding(t, f)
			callbackName := "inline_fault_" + scenario
			callbackActive := false
			if scenario == "accepted_receipt_write" || scenario == "final_handoff" {
				callbackActive = true
				require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
					if scenario == "accepted_receipt_write" {
						if fields, ok := tx.Statement.Dest.(map[string]any); ok {
							if private, ok := fields["private_data"].(model.TaskPrivateData); ok && private.GenerationAcceptedResult != nil {
								tx.AddError(errors.New("injected accepted receipt write failure"))
							}
						}
					} else if task, ok := tx.Statement.Dest.(*model.Task); ok && task.TaskID == f.task.TaskID && task.PrivateData.GenerationQueue == nil {
						tx.AddError(errors.New("injected final handoff write failure"))
					}
				}))
				t.Cleanup(func() {
					if callbackActive {
						require.NoError(t, f.db.Callback().Update().Remove(callbackName))
					}
				})
			}
			requestCtx, cancel := context.WithCancel(f.c.Request.Context())
			defer cancel()
			f.c.Request = f.c.Request.WithContext(requestCtx)
			var settlement *inlineSettlementFault
			outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
				result, err := relay.RelayTaskSubmit(c, info)
				if err == nil && result != nil {
					if scenario == "canceled_after_saved_response" {
						cancel()
					}
					if scenario == "settlement" {
						settlement = &inlineSettlementFault{BillingSettler: info.Billing, fail: true}
						info.Billing = settlement
					}
				}
				return result, err
			})
			if callbackActive {
				require.NoError(t, f.db.Callback().Update().Remove(callbackName))
				callbackActive = false
			}
			require.NotNil(t, taskErr)
			assert.Nil(t, outcome)
			assert.Equal(t, int32(1), first.Load())
			assert.Zero(t, second.Load())
			stored, exists, err := model.GetByTaskId(f.user.Id, f.task.TaskID)
			require.NoError(t, err)
			require.True(t, exists)
			saved, _, err := service.ReadTaskArtifact(t.Context(), stored, "image", service.MaxInlineImageTaskBytes)
			require.NoError(t, err)
			assert.Equal(t, original, saved)
			wantStatus := model.TaskStatus(model.TaskStatusSubmitted)
			if scenario == "accepted_receipt_write" {
				wantStatus = model.TaskStatusDispatching
			}
			if scenario == "settlement" {
				wantStatus = model.TaskStatusSuccess
				require.NotNil(t, settlement)
				assert.Equal(t, "task_billing_settlement_failed", taskErr.Code)
			}
			assert.Equal(t, wantStatus, stored.Status)
			assert.Equal(t, 5000, stored.Quota)
			if scenario == "accepted_receipt_write" {
				assert.Nil(t, stored.PrivateData.GenerationAcceptedResult)
				assert.Empty(t, stored.PrivateData.UpstreamTaskID)
			} else {
				require.NotNil(t, stored.PrivateData.GenerationAcceptedResult)
				assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.PrivateData.GenerationAcceptedResult.Status)
				assert.Equal(t, f.task.TaskID, stored.PrivateData.UpstreamTaskID)
				assert.Contains(t, stored.PrivateData.GenerationAcceptedResult.URL, "/artifacts/image/content")
			}
			// A restart cannot reclaim a sent/unknown/accepted task as a fresh request.
			claimed, err := model.ClaimGenerationTasks(t.Context(), f.user.Id, "new-inline-executor")
			require.NoError(t, err)
			assert.Empty(t, claimed)
			scanner, _ := f.c.Get("generation_queue_scanner")
			dispatchInfluencerGeneration(t.Context(), stored, scanner.(*model.SystemTask), "runner")
			assert.Equal(t, int32(1), first.Load())
			assert.Zero(t, second.Load())
			assert.Equal(t, int32(1), writes.walletDebits.Load())
			assert.Zero(t, writes.walletCredits.Load())
			var account model.User
			require.NoError(t, f.db.First(&account, f.user.Id).Error)
			assert.Equal(t, 95000, account.Quota)
			if scenario == "final_handoff" || scenario == "canceled_after_saved_response" {
				// The native original-result observer may finish a genuine saved terminal
				// receipt locally. An object without that receipt must remain unknown.
				oldFactory := service.GetTaskAdaptorFunc
				service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
				t.Cleanup(func() { service.GetTaskAdaptorFunc = oldFactory })
				service.RunTaskPollingOnce(t.Context(), nil)
				require.NoError(t, f.db.First(&stored, stored.ID).Error)
				assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
				assert.False(t, stored.PrivateData.ImageResultUnverified)
				service.RunTaskPollingOnce(t.Context(), nil)
				var observed model.User
				require.NoError(t, f.db.First(&observed, f.user.Id).Error)
				assert.Equal(t, 95000, observed.Quota)
				// Original-result recovery does not manufacture a missing accounting
				// stage. Funding is held once; the known-unlogged stage stays explicit.
				assert.Zero(t, observed.UsedQuota)
				assert.Zero(t, observed.RequestCount)
				require.NoError(t, f.db.First(&stored, stored.ID).Error)
				assert.True(t, stored.PrivateData.GenerationReservationUnlogged)
				assert.Equal(t, model.TaskConsumptionLogPending, stored.PrivateData.GenerationConsumptionLogState)
				var logs []model.Log
				require.NoError(t, f.db.Where("user_id = ? AND type = ?", f.user.Id, model.LogTypeConsume).Find(&logs).Error)
				assert.Empty(t, logs, "original image observation alone cannot invent an unknown consumption log")
				assert.Equal(t, int32(1), first.Load())
				assert.Zero(t, second.Load())
				assert.Equal(t, int32(1), writes.walletDebits.Load())
				assert.Zero(t, writes.walletCredits.Load())
			}
			if scenario == "settlement" {
				settlement.fail = false
				recovery, _ := gin.CreateTestContext(httptest.NewRecorder())
				recovery.Request = httptest.NewRequest(http.MethodGet, "/", nil)
				require.NoError(t, service.SettleBilling(recovery, f.info, 5000))
				require.NoError(t, service.SettleBilling(recovery, f.info, 5000))
				assert.Equal(t, 3, settlement.calls)
				assert.Equal(t, int32(1), writes.walletDebits.Load())
				assert.Zero(t, writes.walletCredits.Load())
			}
		})
	}
}

func TestNativeInlineCrossGroupDuplicateAndRetryBudgetStayGlobal(t *testing.T) {
	var thirdCalls atomic.Int32
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { thirdCalls.Add(1) }))
	defer third.Close()
	f := nativeInlineFailoverFixture(t, ":invalid-first", ":invalid-second", "default", "wallet")
	writes := observeNativeFailoverFunding(t, f)
	const group = "gpt-image-2.5-sunburst-image"
	const alternate = "sunburst-backup-test"
	oldRetries := common.RetryTimes
	common.RetryTimes = 5
	t.Cleanup(func() { common.RetryTimes = oldRetries })
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"gpt-image-2.5-sunburst-image":1,"sunburst-backup-test":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"gpt-image-2.5-sunburst-image":"Image","sunburst-backup-test":"Backup"}`))
	f.backup.Group = alternate
	require.NoError(t, f.db.Save(&f.backup).Error)
	require.NoError(t, f.db.Model(&model.Ability{}).Where("channel_id = ?", f.backup.Id).Update("group", alternate).Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: alternate, Model: influencerImageModel, ChannelId: f.primary.Id, Enabled: true, Priority: f.primary.Priority}).Error)
	extra := f.backup
	extra.Id = 0
	extra.Name = "third channel must remain unused"
	extra.BaseURL = &third.URL
	extra.Priority = common.GetPointer(int64(1))
	require.NoError(t, f.db.Create(&extra).Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: alternate, Model: influencerImageModel, ChannelId: extra.Id, Enabled: true, Priority: extra.Priority}).Error)
	f.task.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingAuto, Groups: []string{group, alternate}, MaxAttempts: 2}
	require.NoError(t, f.db.Save(f.task).Error)
	f.info.TokenGroup = "auto"
	common.SetContextKey(f.c, constant.ContextKeyTokenAutoGroups, []string{group, alternate})
	common.SetContextKey(f.c, constant.ContextKeyAutoGroup, group)
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, taskErr)
	assert.True(t, taskErr.NoTaskCreated)
	assert.Zero(t, thirdCalls.Load())
	stored, exists, err := model.GetByTaskId(f.user.Id, f.task.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	require.NotNil(t, stored.PrivateData.GenerationQueue)
	attempts := stored.PrivateData.GenerationQueue.Routing.Attempts
	require.Len(t, attempts, 2)
	assert.Equal(t, f.primary.Id, attempts[0].ChannelID)
	assert.Equal(t, f.backup.Id, attempts[1].ChannelID)
	assert.NotEqual(t, attempts[0].ChannelID, attempts[1].ChannelID)
	assert.Equal(t, int32(1), writes.walletDebits.Load())
	assert.Equal(t, int32(1), writes.walletCredits.Load())
	require.NoError(t, service.SettleBilling(f.c, f.info, 0))
	require.True(t, service.ReleaseUnloggedTaskReservation(t.Context(), stored, "reloaded terminal inline rejection"))
	assert.Equal(t, int32(1), writes.walletCredits.Load())
}

func TestNativeInlineRequiresPreparedFrozenIdentityBeforeAnySend(t *testing.T) {
	for _, scenario := range []string{"missing prepared input", "different frozen source"} {
		t.Run(scenario, func(t *testing.T) {
			var posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { posts.Add(1) }))
			defer server.Close()
			f := nativeInlineFailoverFixture(t, server.URL, server.URL, "default", "wallet")
			writes := observeNativeFailoverFunding(t, f)
			if scenario == "missing prepared input" {
				f.c.Set("influencer_image_inputs", (*model.GenerationImageInputs)(nil))
			} else {
				// An SQL rehydrated value may be a distinct allocation, but its full frozen
				// source identity cannot silently change while the prepared request stays.
				f.task.PrivateData.GenerationQueue.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: "text", Source: &model.GenerationImageReference{FileID: "foreign-source", SHA256: "not-the-prepared-source"}}
				require.NoError(t, f.db.Save(f.task).Error)
			}
			_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
			require.NotNil(t, taskErr)
			assert.Zero(t, posts.Load())
			assert.Zero(t, writes.walletDebits.Load())
			assert.Zero(t, writes.walletCredits.Load())
			var user model.User
			require.NoError(t, f.db.First(&user, f.user.Id).Error)
			assert.Equal(t, 100000, user.Quota)
			if scenario == "missing prepared input" {
				value, _ := f.c.Get("influencer_image_inputs")
				assert.Nil(t, value.(*model.GenerationImageInputs), "validation must not fabricate a new trusted context")
			}
		})
	}
}
