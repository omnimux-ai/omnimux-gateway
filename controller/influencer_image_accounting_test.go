package controller

import (
	"context"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Stop exactly after the same native durable handoff as executeTaskSubmission,
// before its separate settle/log tail, representing loss of that process.
func TestSunburstHandoffCrashBeforeConsumptionKeepsRefundAndUsageCorrect(t *testing.T) {
	var posts, gets atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			posts.Add(1)
			_, _ = io.WriteString(w, `{"id":"real-handoff-crash-job","status":"pending"}`)
			return
		}
		gets.Add(1)
		assert.Equal(t, "/v1/tasks/real-handoff-crash-job", r.URL.Path)
		_, _ = io.WriteString(w, `{"id":"real-handoff-crash-job","status":"failed","error":{"message":"confirmed original generation failure"}}`)
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
	f.info.PublicTaskID = f.task.TaskID
	f.info.BeforeTaskSend = func(platform constant.TaskPlatform) error {
		snap, err := taskFundingSnapshot(f.c, f.info, platform)
		if err != nil {
			return err
		}
		return model.PersistGenerationSendSnapshot(f.c.Request.Context(), f.task, snap, scanner, "runner", true)
	}
	result, failure := relay.RelayTaskSubmit(f.c, f.info)
	require.Nil(t, failure)
	require.NotNil(t, result)
	require.NoError(t, persistGenerationAcceptedReceipt(f.c, f.task, f.info, result, nil))
	task, err := taskFundingSnapshot(f.c, f.info, result.Platform)
	require.NoError(t, err)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.Quota = result.Quota
	task.Data = result.TaskData
	attachInfluencerImageInputs(f.c, task)
	require.NoError(t, model.PersistGenerationSubmission(context.Background(), task, f.task))
	// No consumption log or usage accounting has run, exactly as at the crash window.
	var before model.User
	require.NoError(t, f.db.First(&before, f.user.Id).Error)
	var saved model.Task
	require.NoError(t, f.db.First(&saved, f.task.ID).Error)
	var consumeBefore int64
	require.NoError(t, f.db.Model(&model.Log{}).Where("user_id = ? AND type = ?", f.user.Id, model.LogTypeConsume).Count(&consumeBefore).Error)
	require.Equal(t, 95000, before.Quota)
	require.Zero(t, before.UsedQuota)
	require.Zero(t, consumeBefore)
	t.Logf("before poll status=%s unlogged=%v quota=%d userquota=%d used=%d consumeLogs=%d", saved.Status, saved.PrivateData.GenerationReservationUnlogged, saved.Quota, before.Quota, before.UsedQuota, consumeBefore)
	oldFactory, oldLimit := service.GetTaskAdaptorFunc, constant.TaskQueryLimit
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return relay.GetTaskAdaptor(platform) }
	constant.TaskQueryLimit = 100
	t.Cleanup(func() { service.GetTaskAdaptorFunc, constant.TaskQueryLimit = oldFactory, oldLimit })
	summary := service.RunTaskPollingOnce(context.Background(), nil)
	require.Equal(t, 1, summary.UnfinishedTasks)
	var after model.User
	require.NoError(t, f.db.First(&after, f.user.Id).Error)
	require.NoError(t, f.db.First(&saved, f.task.ID).Error)
	var channel model.Channel
	require.NoError(t, f.db.First(&channel, f.primary.Id).Error)
	t.Logf("after poll status=%s posts=%d gets=%d unlogged=%v taskquota=%d userquota=%d userused=%d channelused=%d", saved.Status, posts.Load(), gets.Load(), saved.PrivateData.GenerationReservationUnlogged, saved.Quota, after.Quota, after.UsedQuota, channel.UsedQuota)
	assert.EqualValues(t, model.TaskStatusFailure, saved.Status)
	assert.Equal(t, 100000, after.Quota)
	assert.Zero(t, saved.Quota)
	assert.Zero(t, after.UsedQuota, "unwritten consumption must not be rolled back below zero")
	assert.Zero(t, channel.UsedQuota, "unwritten channel consumption must not be rolled back below zero")
	assert.Equal(t, int32(1), posts.Load())
	assert.Equal(t, int32(1), gets.Load())
}
