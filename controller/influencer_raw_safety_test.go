package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/sora"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This tests the internal submission primitive, not permission to open the
// raw endpoint. The public entry is separately required to reject before funds.
func newSunburstRawPrimitiveFixture(t *testing.T, primaryURL, backupURL string) *nativeFailoverFixture {
	t.Helper()
	f := newNativeFailoverFixture(t, primaryURL, backupURL, false)
	withTieredBillingConfig(t, map[string]string{influencerImageModel: "tiered_expr"}, map[string]string{influencerImageModel: `tier("request", fixed(0.01))`})
	contract := `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"input_formats":["png"],"sizes":["1024x576"]}}`
	for _, ch := range []*model.Channel{&f.primary, &f.backup} {
		ch.Type, ch.Models, ch.OtherSettings = constant.ChannelTypeOpenAI, influencerImageModel, contract
		require.NoError(t, f.db.Model(&model.Channel{}).Where("id = ?", ch.Id).Updates(map[string]any{"type": ch.Type, "models": ch.Models, "settings": contract}).Error)
		require.NoError(t, f.db.Model(&model.Ability{}).Where("channel_id = ?", ch.Id).Update("model", influencerImageModel).Error)
	}
	for _, key := range []string{"generation_queue_reserved_task", "generation_queue_scanner", "generation_queue_runner"} {
		delete(f.c.Keys, key)
	}
	require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, influencerImageModel))
	f.info.OriginModelName = influencerImageModel
	f.info.PublicTaskID = model.GenerateTaskID()
	f.info.IsPlayground = true // isolated wallet fixture; never an API token acceptance claim
	require.NoError(t, influencerRewriteBody(f.c, map[string]any{"model": influencerImageModel, "prompt": "a blue kettle", "size": influencerImagePixels, "quality": "high", "n": 1, "output_format": "png", "response_format": "url", "image_mode": "text"}))
	common.RetryTimes = 8
	return f
}

func TestSunburstRawPrimitiveNeverRepeatsUnknown503AndExplainsClosedEntry(t *testing.T) {
	var first, second atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first.Add(1)
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"error":{"code":"model_not_found","message":"no available channel"}}`)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
	defer backup.Close()
	f := newSunburstRawPrimitiveFixture(t, primary.URL, backup.URL)
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, taskErr)
	assert.False(t, taskErr.NoTaskCreated)
	assert.Equal(t, int32(1), first.Load())
	assert.Zero(t, second.Load())
	// This is the inherited unqueued funding defect: the generic defer returns
	// funds even though no rejection proof exists. The raw entry stays closed.
	require.Eventually(t, func() bool {
		var user model.User
		return f.db.First(&user, f.user.Id).Error == nil && user.Quota == 100000
	}, time.Second, 10*time.Millisecond)
	var rows int64
	require.NoError(t, f.db.Model(&model.Task{}).Where("task_id = ?", f.info.PublicTaskID).Count(&rows).Error)
	assert.Zero(t, rows, "unknown raw submissions have no durable reconciliation row, so this path must not be publicly enabled")
}

func TestSunburstRawPrimitiveAcceptedResultHasNoBackup(t *testing.T) {
	var first, second atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"url":"https://8.8.4.4/accepted.png"}]}`)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { second.Add(1) }))
	defer backup.Close()
	f := newSunburstRawPrimitiveFixture(t, primary.URL, backup.URL)
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	assert.Equal(t, int32(1), first.Load())
	assert.Zero(t, second.Load())
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), outcome.Task.Status)
}

func TestSunburstRawPrimitiveKnownUnsentSkipsChannelWithTwoAttemptBudget(t *testing.T) {
	var backupCalls atomic.Int32
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"url":"https://8.8.4.4/accepted.png"}]}`)
	}))
	defer backup.Close()
	f := newSunburstRawPrimitiveFixture(t, ":invalid-not-sent", backup.URL)
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	assert.Equal(t, int32(1), backupCalls.Load())
	assert.Equal(t, f.backup.Id, outcome.Task.ChannelId)
	assert.Len(t, f.c.GetStringSlice("use_channel"), 2)
}

func TestSunburstRawPrimitiveGroupChangeCannotExpandTwoAttemptBudget(t *testing.T) {
	var thirdCalls atomic.Int32
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { thirdCalls.Add(1) }))
	defer third.Close()
	f := newSunburstRawPrimitiveFixture(t, ":first-invalid-not-sent", ":second-invalid-not-sent")
	thirdChannel := model.Channel{Type: constant.ChannelTypeOpenAI, Name: "third-not-permitted-by-budget", Status: common.ChannelStatusEnabled, Group: "backup", Models: influencerImageModel, BaseURL: &third.URL, Priority: common.GetPointer(int64(0)), OtherSettings: f.backup.OtherSettings}
	require.NoError(t, f.db.Create(&thirdChannel).Error)
	require.NoError(t, f.db.Create(&model.Ability{Group: "backup", Model: influencerImageModel, ChannelId: thirdChannel.Id, Enabled: true, Priority: thirdChannel.Priority}).Error)
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, taskErr)
	assert.Nil(t, outcome)
	assert.True(t, taskErr.NoTaskCreated)
	assert.Zero(t, thirdCalls.Load())
	assert.Len(t, f.c.GetStringSlice("use_channel"), 2, "native group retry cursor must not enlarge the total attempt budget")
}

func TestSunburstAcceptedUsesNativeImagePollerAndRejectsFalseSuccess(t *testing.T) {
	var posts, polls, backupCalls, phase atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations" {
			posts.Add(1)
			_, _ = io.WriteString(w, `{"id":"original-image-job","status":"pending"}`)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/v1/tasks/original-image-job" {
			polls.Add(1)
			switch phase.Load() {
			case 0:
				_, _ = io.WriteString(w, `{"code":"success","data":{"status":"SUCCESS"}}`)
			case 1:
				_, _ = io.WriteString(w, `{"id":"original-image-job","status":"completed","results":[{"b64_json":"AAAA"}]}`)
			default:
				_, _ = fmt.Fprint(w, `{"id":"original-image-job","status":"completed","results":["https://images.example/final.png"]}`)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { backupCalls.Add(1) }))
	defer backup.Close()
	f := newSunburstRawPrimitiveFixture(t, upstream.URL, backup.URL)
	f.c.Set(ctxInfluencerImageInputs, &model.GenerationImageInputs{Version: 1, Mode: "text"})
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	task := outcome.Task
	require.Equal(t, sora.ImageTaskPlatformV1, task.Platform, "the persisted platform must resolve to the image parser after process restart")
	require.NotNil(t, task.PrivateData.ImageInputs)
	task.Action = ActionInfluencerSheet
	task.SubmitTime = time.Now().Add(-2 * time.Hour).Unix()
	require.NoError(t, f.db.Save(task).Error)
	var funded model.User
	require.NoError(t, f.db.First(&funded, f.user.Id).Error)
	require.Equal(t, 95000, funded.Quota)
	oldFactory, oldMax, oldTimeout, oldLimit := service.GetTaskAdaptorFunc, constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes, constant.TaskQueryLimit
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
	constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes, constant.TaskQueryLimit = 1, 1, 100
	t.Cleanup(func() {
		service.GetTaskAdaptorFunc, constant.TaskPollMaxFailures, constant.TaskTimeoutMinutes, constant.TaskQueryLimit = oldFactory, oldMax, oldTimeout, oldLimit
	})
	for stage := int32(0); stage < 2; stage++ {
		phase.Store(stage)
		summary := service.RunTaskPollingOnce(context.Background(), nil)
		require.Equal(t, 1, summary.UnfinishedTasks)
		var original model.Task
		require.NoError(t, f.db.First(&original, task.ID).Error)
		assert.NotEqualValues(t, model.TaskStatusSuccess, original.Status)
		assert.NotEqualValues(t, model.TaskStatusFailure, original.Status)
		assert.True(t, original.PrivateData.ImageResultUnverified)
		assert.Empty(t, original.PrivateData.ResultURL)
		assert.Equal(t, 5000, original.Quota)
		require.NoError(t, f.db.First(&funded, f.user.Id).Error)
		assert.Equal(t, 95000, funded.Quota)
	}
	phase.Store(2)
	service.RunTaskPollingOnce(context.Background(), nil)
	var original model.Task
	require.NoError(t, f.db.First(&original, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, original.Status)
	assert.False(t, original.PrivateData.ImageResultUnverified)
	assert.Equal(t, "https://images.example/final.png", original.PrivateData.ResultURL)
	assert.Equal(t, "original-image-job", original.PrivateData.UpstreamTaskID)
	assert.Equal(t, int32(1), posts.Load())
	assert.Equal(t, int32(3), polls.Load())
	assert.Zero(t, backupCalls.Load())
	require.NoError(t, f.db.First(&funded, f.user.Id).Error)
	assert.Equal(t, 95000, funded.Quota)
}
