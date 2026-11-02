package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type nativeFailoverFixture struct {
	db      *gorm.DB
	c       *gin.Context
	info    *relaycommon.RelayInfo
	task    *model.Task
	user    model.User
	primary model.Channel
	backup  model.Channel
}

// The primary GoEasy request is genuinely prepared by the production adaptor.
// An invalid URL fails inside DoTaskApiRequest before transport, then the
// native selector picks the alternate group and rebuilds the Sora protocol.
func newNativeFailoverFixture(t *testing.T, primaryURL, backupURL string, sameGroup bool) *nativeFailoverFixture {
	t.Helper()
	db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.Log{}, &model.SystemTaskLock{}, &model.UserSubscription{}, &model.SubscriptionPlan{})
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldMemory, oldBatch, oldConsume, oldRetries := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes = false, false, false, false, 1
	t.Setenv("LOG_SQL_DSN", "")
	oldMaster := common.IsMasterNode
	common.IsMasterNode = false
	require.NoError(t, model.InitLogDB())
	common.IsMasterNode = oldMaster
	oldGroups, oldUsable := ratio_setting.GroupRatio2JSONString(), setting.UserUsableGroups2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"backup":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","backup":"Backup"}`))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled, common.RetryTimes = oldRedis, oldMemory, oldBatch, oldConsume, oldRetries
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
	})
	withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr", "failover-video": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("request", fixed(0.01))`, "failover-video": `tier("request", fixed(0.01))`})
	name := "gpt-image-2.5"
	primaryType, backupType := constant.ChannelTypeGoEasyAPI, constant.ChannelTypeOpenAI
	group, mode, groups := "backup", model.GenerationRoutingAuto, []string{"default", "backup"}
	if sameGroup {
		name, primaryType, backupType = "failover-video", constant.ChannelTypeTaskPlugin, constant.ChannelTypeTaskPlugin
		group, mode, groups = "default", model.GenerationRoutingGroup, []string{"default"}
		source := `export const meta={apiVersion:1,key:"native-failover-video",name:"Failover video",version:"1.0.0",author:{name:"Test"},models:["failover-video"],fetchMode:"per_task"};export function buildSubmitRequest(ctx){return {url:ctx.baseUrl+"/submit",body:ctx.baseUrl.includes("not-sent.invalid")?{file:{__fileRef:"missing",encoding:"base64"}}:ctx.requestBody};}export function parseSubmitResponse(ctx,resp){return {taskId:resp.body.id,taskData:resp.body,immediate:{status:"SUCCESS",url:resp.body.url}};}export function buildQueryRequest(){return {}}export function parseTaskResult(){return {status:"SUCCESS"}};`
		_, err := pluginruntime.DefaultRegistry.Register(source, pluginruntime.Options{})
		require.NoError(t, err)
		t.Cleanup(func() { pluginruntime.DefaultRegistry.Unregister("native-failover-video") })
	}
	user := model.User{Username: "native_failover", AffCode: "native_failover", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	primary := model.Channel{Name: "primary", Type: primaryType, Status: common.ChannelStatusEnabled, Key: "primary-test-only", Group: "default", Models: name, BaseURL: &primaryURL, Priority: common.GetPointer(int64(20))}
	backup := model.Channel{Name: "backup", Type: backupType, Status: common.ChannelStatusEnabled, Key: "backup-test-only", Group: group, Models: name, BaseURL: &backupURL, Priority: common.GetPointer(int64(10))}
	if sameGroup {
		primary.Setting, backup.Setting = common.GetPointer(`{"task_plugin_key":"native-failover-video"}`), common.GetPointer(`{"task_plugin_key":"native-failover-video"}`)
	}
	require.NoError(t, db.Create(&primary).Error)
	require.NoError(t, db.Create(&backup).Error)
	for _, ch := range []model.Channel{primary, backup} {
		require.NoError(t, db.Create(&model.Ability{Group: ch.Group, Model: name, ChannelId: ch.Id, Enabled: true, Priority: ch.Priority}).Error)
	}
	body := fmt.Sprintf(`{"model":%q,"prompt":"person","size":"1024x1024","images":["https://example.test/ref.png"],"duration":5,"resolution":"720p","audio":true}`, name)
	task := &model.Task{TaskID: "task_native_failover", UserId: user.Id, Group: "default", Action: ActionInfluencerSheet, Status: model.TaskStatusDispatching, Properties: model.Properties{OriginModelName: name}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: []byte(body), ClaimOwner: "native-scan", Routing: &model.GenerationRoutingPolicy{Mode: mode, Groups: groups, MaxAttempts: 2}}}}
	require.NoError(t, task.Insert())
	scanner := &model.SystemTask{TaskID: "native-scan"}
	require.NoError(t, db.Create(&model.SystemTaskLock{Type: model.SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body)).WithContext(context.Background())
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", user.Id)
	user.ToBaseUser().WriteContext(c)
	c.Set("generation_queue_reserved_task", task)
	c.Set("generation_queue_scanner", scanner)
	c.Set("generation_queue_runner", "runner")
	common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, groups)
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "default")
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, &primary, name))
	tokenGroup := "auto"
	if sameGroup {
		tokenGroup = "default"
	}
	info := &relaycommon.RelayInfo{UserId: user.Id, UserGroup: "default", UsingGroup: "default", TokenGroup: tokenGroup, OriginModelName: name, TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: task.TaskID}}
	return &nativeFailoverFixture{db: db, c: c, info: info, task: task, user: user, primary: primary, backup: backup}
}

func TestNativeGenerationFailoverUsesActualPreSendEvidenceAcrossProtocols(t *testing.T) {
	var calls atomic.Int32
	var server *httptest.Server
	var pixels bytes.Buffer
	require.NoError(t, png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/result.png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pixels.Bytes())
			return
		}
		calls.Add(1)
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		assert.Equal(t, "person", body["prompt"])
		assert.Equal(t, "Bearer backup-test-only", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"url":%q}]}`, server.URL+"/result.png")
	}))
	defer server.Close()
	f := newNativeFailoverFixture(t, ":bad-url", server.URL, false)
	outcome, err := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.Nil(t, err)
	require.NotNil(t, outcome)
	assert.Equal(t, int32(1), calls.Load())
	assert.Equal(t, f.backup.Id, outcome.Task.ChannelId)
	assert.Equal(t, 5000, outcome.Task.Quota)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), outcome.Task.Status)
	response, getErr := http.Get(outcome.Task.PrivateData.ResultURL)
	require.NoError(t, getErr)
	defer response.Body.Close()
	_, getErr = png.Decode(response.Body)
	require.NoError(t, getErr)
	var user model.User
	require.NoError(t, f.db.First(&user, f.user.Id).Error)
	assert.Equal(t, 95000, user.Quota)
}

func TestNativeGenerationFailoverSameGroupVideoRetainsAllInputs(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		assert.Equal(t, float64(5), body["duration"])
		assert.Equal(t, "720p", body["resolution"])
		assert.Equal(t, true, body["audio"])
		assert.Equal(t, []any{"https://example.test/ref.png"}, body["images"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"video-b","url":"https://example.test/result.mp4"}`))
	}))
	defer server.Close()
	f := newNativeFailoverFixture(t, "https://not-sent.invalid", server.URL, true)
	outcome, err := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.Nil(t, err)
	require.NotNil(t, outcome)
	assert.Equal(t, int32(1), calls.Load())
	assert.Equal(t, f.backup.Id, outcome.Task.ChannelId)
	assert.Equal(t, "video-b", outcome.Task.PrivateData.UpstreamTaskID)
}

func TestNativeGenerationPreparedMotionSurvivesFirstSendAndFailover(t *testing.T) {
	for _, failFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail_first_%t", failFirst), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Equal(t, "/v1/videos", r.URL.Path)
				var body map[string]any
				require.NoError(t, common.DecodeJson(r.Body, &body))
				assert.Equal(t, "planned performance prompt", body["prompt"])
				_, containsGroup := body["group"]
				assert.False(t, containsGroup, "internal route must not reach the supplier")
				assert.Equal(t, []any{"https://example.test/face.png", "https://example.test/body.png"}, body["images"])
				assert.Equal(t, []any{"https://example.test/motion.mp4"}, body["videos"])
				assert.Equal(t, []any{"https://example.test/reference.mp3"}, body["audios"])
				assert.Equal(t, "9:16", body["aspect_ratio"])
				assert.Equal(t, "720p", body["resolution"])
				assert.Equal(t, "5", body["seconds"])
				assert.Equal(t, true, body["generate_audio"])
				assert.Equal(t, map[string]any{"camera": map[string]any{"movement": "tracking"}}, body["metadata"])
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"motion-accepted"}`)
			}))
			defer server.Close()
			f := newNativeFailoverFixture(t, server.URL, server.URL, false)
			const motionModel = "wan-2.6-r2v"
			withTieredBillingConfig(t, map[string]string{motionModel: "tiered_expr"}, map[string]string{motionModel: `tier("request", fixed(0.01))`})
			source, err := os.ReadFile("../plugins/tasks/sora/plugin.js")
			require.NoError(t, err)
			plugin, err := pluginruntime.CompilePlugin(string(source), pluginruntime.Options{})
			require.NoError(t, err)
			f.c.Set(pluginruntime.ContextKeyPinnedPlugin, pluginruntime.PinnedPlugin{Generation: pluginruntime.DefaultRegistry.Generation(), Plugin: plugin})
			for _, channel := range []*model.Channel{&f.primary, &f.backup} {
				channel.Type, channel.Group, channel.Models = constant.ChannelTypeOpenAI, "default", motionModel
				require.NoError(t, f.db.Save(channel).Error)
				require.NoError(t, f.db.Model(&model.Ability{}).Where("channel_id = ?", channel.Id).Updates(map[string]any{"group": "default", "model": motionModel}).Error)
			}
			original := map[string]any{
				"model": motionModel, "group": "default", "prompt": "obsolete frozen prompt", "seconds": "5", "aspect_ratio": "9:16", "resolution": "720p",
				"images": []string{"https://example.test/face.png", "https://example.test/body.png"}, "videos": []string{"https://example.test/motion.mp4"}, "audios": []string{"https://example.test/reference.mp3"}, "generate_audio": true,
				"metadata": map[string]any{"camera": map[string]any{"movement": "tracking"}},
			}
			frozen, err := common.Marshal(original)
			require.NoError(t, err)
			f.task.Action, f.task.Properties.OriginModelName = ActionInfluencerMotion, motionModel
			f.task.PrivateData.GenerationQueue.Body = frozen
			f.task.PrivateData.GenerationQueue.Routing = &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{"default"}, MaxAttempts: 2}
			require.NoError(t, f.db.Save(f.task).Error)
			f.info.OriginModelName, f.info.TokenGroup = motionModel, "default"
			f.c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(frozen))
			f.c.Request.Header.Set("Content-Type", "application/json")
			require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, motionModel))
			original["prompt"] = "planned performance prompt"
			setInfluencerMotionTaskRequest(f.c, original)
			attempt := 0
			outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
				attempt++
				if failFirst && attempt == 1 {
					gate := info.BeforeTaskSend
					info.BeforeTaskSend = func(platform constant.TaskPlatform) error {
						if err := gate(platform); err != nil {
							return err
						}
						// The production plugin already prepared its complete body.
						// Force the shared helper's URL recheck to reject before transport.
						info.ChannelBaseUrl = "https://blocked-before-send.invalid"
						return nil
					}
					defer func() { info.BeforeTaskSend = gate }()
				}
				result, taskErr := relay.RelayTaskSubmit(c, info)
				if failFirst && attempt == 1 {
					current, _ := c.Get("task_request")
					providerCopy := current.(map[string]any)
					providerCopy["prompt"] = "provider-local mutation"
					providerCopy["metadata"].(map[string]any)["camera"].(map[string]any)["movement"] = "provider-local mutation"
				}
				return result, taskErr
			})
			require.Nil(t, taskErr)
			require.NotNil(t, outcome)
			assert.Equal(t, int32(1), calls.Load())
			if failFirst {
				assert.Equal(t, 2, attempt)
				assert.Equal(t, f.backup.Id, outcome.Task.ChannelId)
			} else {
				assert.Equal(t, 1, attempt)
				assert.Equal(t, f.primary.Id, outcome.Task.ChannelId)
			}
		})
	}
}

func TestNativeGenerationFailoverNeverRepeatsUncertainSubmission(t *testing.T) {
	for _, body := range []string{`{"error":{"code":"model_not_found","type":"new_api_error","message":"no available channel"}}`, `not-json`, `{"id":"accepted"}`} {
		t.Run(body, func(t *testing.T) {
			var first, second atomic.Int32
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first.Add(1)
				w.WriteHeader(503)
				_, _ = w.Write([]byte(body))
			}))
			defer primary.Close()
			backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
			defer backup.Close()
			f := newNativeFailoverFixture(t, primary.URL, backup.URL, false)
			_, err := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
			require.NotNil(t, err)
			assert.False(t, err.NoTaskCreated)
			assert.Equal(t, int32(1), first.Load())
			assert.Equal(t, int32(0), second.Load())
			var stored model.Task
			require.NoError(t, f.db.First(&stored, f.task.ID).Error)
			assert.Equal(t, model.TaskStatusDispatching, stored.Status)
			assert.Equal(t, model.GenerationAttemptUnknown, stored.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
		})
	}
}

func TestNativeGenerationFailoverAllPreSendRejectionsReleaseOnce(t *testing.T) {
	f := newNativeFailoverFixture(t, ":bad-url", ":bad-backup", false)
	_, err := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, err)
	assert.True(t, err.NoTaskCreated)
	var stored model.Task
	require.NoError(t, f.db.First(&stored, f.task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Zero(t, stored.Quota)
	var user model.User
	require.NoError(t, f.db.First(&user, f.user.Id).Error)
	assert.Equal(t, 100000, user.Quota)
	assert.True(t, f.c.GetBool("generation_queue_safe_failure"))
}

func TestGenerationQuoteChecksComputedChargeAndNativeBasis(t *testing.T) {
	base := &model.TaskBillingContext{ModelPrice: 0.01, GroupRatio: 1, PerCallBilling: true, OtherRatios: map[string]float64{"n": 1}}
	quote := &generationSubmissionQuote{Quota: 5000, Billing: base}
	assert.True(t, quote.Matches(5000, base))
	assert.False(t, quote.Matches(6000, base), "same reservation cannot conceal a new charge")
	copy := *base
	copy.OtherRatios = map[string]float64{"n": 2}
	assert.False(t, quote.Matches(5000, &copy), "equal rounded charge cannot conceal quantity drift")
}

func TestNativeGenerationBackupPriceChangeStopsBeforeSending(t *testing.T) {
	var calls atomic.Int32
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer backup.Close()
	f := newNativeFailoverFixture(t, ":not-sent", backup.URL, false)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"backup":2}`))
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, taskErr)
	assert.Equal(t, "generation_quote_changed", taskErr.Code)
	assert.Zero(t, calls.Load())
	var user model.User
	require.NoError(t, f.db.First(&user, f.user.Id).Error)
	assert.Equal(t, 100000, user.Quota)
}

func TestNativeGenerationFixedAdjustmentAfterAcceptanceDoesNotChargeExtra(t *testing.T) {
	var calls atomic.Int32
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"url":"https://example.test/accepted.png"}]}`)
	}))
	defer backup.Close()
	f := newNativeFailoverFixture(t, ":not-sent", backup.URL, false)
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
		result, err := relay.RelayTaskSubmit(c, info)
		if result != nil {
			result.Quota += 5000
		}
		return result, err
	})
	require.NotNil(t, taskErr)
	assert.Equal(t, "generation_quote_changed", taskErr.Code)
	assert.Equal(t, int32(1), calls.Load())
	var user model.User
	require.NoError(t, f.db.First(&user, f.user.Id).Error)
	assert.Equal(t, 95000, user.Quota, "original reservation is retained, with no extra debit")
	var task model.Task
	require.NoError(t, f.db.First(&task, f.task.ID).Error)
	assert.Equal(t, model.TaskStatusDispatching, task.Status)
	assert.Equal(t, model.GenerationAttemptAccepted, task.PrivateData.GenerationQueue.Routing.Attempts[1].Outcome)
	assert.NotEmpty(t, task.PrivateData.UpstreamTaskID, "accepted receipt remains available for reconciliation")
}

func TestNativeGenerationLateAcceptedReceiptIsSavedWithoutRetry(t *testing.T) {
	var first, second atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"late-accepted","status":"queued"}`)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
	defer backup.Close()
	f := newNativeFailoverFixture(t, primary.URL, backup.URL, false)
	ctx, cancel := context.WithCancel(f.c.Request.Context())
	defer cancel()
	f.c.Request = f.c.Request.WithContext(ctx)
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
		result, err := relay.RelayTaskSubmit(c, info)
		cancel()
		return result, err
	})
	require.NotNil(t, taskErr)
	assert.Equal(t, "request_cancelled", taskErr.Code)
	assert.Equal(t, int32(1), first.Load())
	assert.Zero(t, second.Load())
	var task model.Task
	require.NoError(t, f.db.First(&task, f.task.ID).Error)
	assert.Equal(t, "late-accepted", task.PrivateData.UpstreamTaskID)
	assert.Equal(t, model.TaskStatusDispatching, task.Status)
	assert.Equal(t, model.GenerationAttemptAccepted, task.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
}

func TestNativeGenerationFinalPersistenceFailureRetainsReceiptAcrossRestart(t *testing.T) {
	var first, second atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"accepted-before-database-failure","status":"queued"}`)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
	defer backup.Close()
	f := newNativeFailoverFixture(t, primary.URL, backup.URL, false)
	require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register("test:fail-final-submission", func(tx *gorm.DB) {
		if task, ok := tx.Statement.Dest.(*model.Task); ok && task.TaskID == f.task.TaskID && task.PrivateData.GenerationQueue == nil {
			tx.AddError(errors.New("injected final persistence failure"))
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.db.Callback().Update().Remove("test:fail-final-submission")) })
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, taskErr)
	assert.Equal(t, "task_insert_failed", taskErr.Code)
	var task model.Task
	require.NoError(t, f.db.First(&task, f.task.ID).Error)
	assert.Equal(t, "accepted-before-database-failure", task.PrivateData.UpstreamTaskID)
	assert.Equal(t, model.TaskStatusDispatching, task.Status)
	assert.Equal(t, 5000, task.Quota)
	assert.Equal(t, model.GenerationAttemptAccepted, task.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
	claimed, err := model.ClaimGenerationTasks(context.Background(), f.user.Id, "after-restart")
	require.NoError(t, err)
	assert.Empty(t, claimed)
	scanner, _ := f.c.Get("generation_queue_scanner")
	dispatchInfluencerGeneration(context.Background(), &task, scanner.(*model.SystemTask), "runner")
	assert.Equal(t, int32(1), first.Load())
	assert.Zero(t, second.Load())
	var user model.User
	require.NoError(t, f.db.First(&user, f.user.Id).Error)
	assert.Equal(t, 95000, user.Quota)
}

func TestNativeGenerationAcceptedOrMissingReceiptAndDisconnectNeverUseBackup(t *testing.T) {
	for _, scenario := range []string{"accepted", "missing_receipt", "disconnected"} {
		t.Run(scenario, func(t *testing.T) {
			var first, second atomic.Int32
			primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				first.Add(1)
				if scenario == "disconnected" {
					connection, _, err := w.(http.Hijacker).Hijack()
					require.NoError(t, err)
					_ = connection.Close()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if scenario == "accepted" {
					_, _ = io.WriteString(w, `{"id":"first-accepted","status":"queued"}`)
				} else {
					_, _ = io.WriteString(w, `{"status":"queued"}`)
				}
			}))
			defer primary.Close()
			backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
			defer backup.Close()
			f := newNativeFailoverFixture(t, primary.URL, backup.URL, false)
			outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
			if scenario == "accepted" {
				require.Nil(t, taskErr)
				require.NotNil(t, outcome)
				assert.Equal(t, "first-accepted", outcome.Task.PrivateData.UpstreamTaskID)
			} else {
				require.NotNil(t, taskErr)
			}
			assert.Equal(t, int32(1), first.Load())
			assert.Zero(t, second.Load())
		})
	}
}
