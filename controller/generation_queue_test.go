package controller

import (
	"context"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGenerationQueueWorkerRebuildsIdentityAndChargesNativeWallet(t *testing.T) {
	db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.Log{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.UserSubscription{}, &model.SubscriptionPlan{})
	oldDB, oldLog := model.DB, model.LOG_DB
	oldMain, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldMemory, oldBatch, oldConsume := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	t.Setenv("LOG_SQL_DSN", "")
	oldMaster := common.IsMasterNode
	common.IsMasterNode = false
	require.NoError(t, model.InitLogDB())
	common.IsMasterNode = oldMaster
	require.NoError(t, i18n.Init())
	common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, false, true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLog
		common.SetDatabaseTypes(oldMain, oldLogType)
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldRedis, oldMemory, oldBatch, oldConsume
	})
	oldGroups := ratio_setting.GroupRatio2JSONString()
	defer ratio_setting.UpdateGroupRatioByJSONString(oldGroups)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	withTieredBillingConfig(t, map[string]string{"queue-image": "tiered_expr"}, map[string]string{"queue-image": `u("units") * 0.01`})
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Empty(t, r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"vendor-accepted"}`))
	}))
	defer upstream.Close()
	source := `export const meta={apiVersion:1,key:"queue-provider",name:"Queue",version:"1.0.0",author:{name:"Test"},models:["queue-image"],protocols:["openai_image"],fetchMode:"per_task",usageSchema:{units:{type:"number",unit:"count"}}};export function buildSubmitRequest(ctx){return {url:ctx.baseUrl+"/submit",body:ctx.requestBody};}export function parseSubmitResponse(ctx,resp){return {taskId:resp.body.id,taskData:resp.body};}export function extractUsage(){return {units:1};}export function buildQueryRequest(){return {}}export function parseTaskResult(){return {status:"SUCCESS"}};export const protocols={openai_image:{decodeRequest:function(ctx){return {kind:"submit",model:ctx.model,requestBody:ctx.body.value};},render:function(){return {data:[]};}}};`
	_, err := pluginruntime.DefaultRegistry.Register(source, pluginruntime.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { pluginruntime.DefaultRegistry.Unregister("queue-provider") })
	user := model.User{Username: "worker_queue", AffCode: "worker_queue", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	ch := model.Channel{Name: "local provider", Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Key: "test-only", Group: "default", Models: "queue-image", BaseURL: &upstream.URL, Setting: common.GetPointer(`{"task_plugin_key":"queue-provider"}`)}
	require.NoError(t, db.Create(&ch).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "queue-image", ChannelId: ch.Id, Enabled: true}).Error)
	task := &model.Task{TaskID: "task_queue_worker", UserId: user.Id, Group: "default", Action: ActionInfluencerSheet, Status: model.TaskStatusWaiting, Progress: "0%", Properties: model.Properties{Input: `{"brief":"person"}`, OriginModelName: "queue-image"}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: "/pg/images/generations", Body: []byte(`{"model":"queue-image","prompt":"person"}`)}}}
	require.NoError(t, task.Insert())
	scanner := &model.SystemTask{TaskID: "scan_test"}
	require.NoError(t, db.Create(&model.SystemTaskLock{Type: model.SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "runner", LockedUntil: common.GetTimestamp() + 60}).Error)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "runner")
	require.Equal(t, int32(1), calls.Load())
	var stored model.Task
	require.NoError(t, db.First(&stored, task.ID).Error)
	assert.Equal(t, "vendor-accepted", stored.PrivateData.UpstreamTaskID)
	assert.Equal(t, "task_queue_worker", stored.TaskID)
	assert.Nil(t, stored.PrivateData.GenerationQueue)
	assert.Equal(t, 5000, stored.Quota)
	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 95000, updated.Quota)
	var logs []model.Log
	require.NoError(t, db.Where("user_id = ?", user.Id).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, 5000, logs[0].Quota)
	var count int64
	require.NoError(t, db.Model(&model.Task{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	t.Log(fmt.Sprintf("native submit count=%d, charge=%d", calls.Load(), stored.Quota))
}

func TestGenerationQueuedSubmissionKeepsPublicRecordAndNativeBilling(t *testing.T) {
	// Native memory notification limits spawn a perpetual cleanup worker. Use
	// local Redis so all jobs owned by this fixture can genuinely drain.
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	oldRedis, oldConsume, oldRDB := common.RedisEnabled, common.LogConsumeEnabled, common.RDB
	common.RedisEnabled, common.LogConsumeEnabled, common.RDB = true, false, client
	t.Cleanup(func() {
		common.RedisEnabled, common.LogConsumeEnabled, common.RDB = oldRedis, oldConsume, oldRDB
		require.NoError(t, client.Close())
	})
	events := []string{}
	db := setupTaskSubmissionDatabase(t, true, &events)
	// Registered after fixture setup: native NotifyUser/cache jobs must finish
	// before cleanup restores any shared database or global configuration.
	t.Cleanup(func() {
		require.Eventually(t, func() bool { return gopool.WorkerCount() == 0 }, 5*time.Second, time.Millisecond, "native background jobs did not finish")
	})
	reserved := &model.Task{TaskID: "task_waiting", UserId: 1, Action: ActionInfluencerMotion, Status: model.TaskStatusDispatching, Progress: "0%", Properties: model.Properties{Input: `{"kind":"motion"}`}}
	require.NoError(t, reserved.Insert())
	c := taskSubmissionTestContext()
	c.Set("generation_queue_reserved_task", reserved)
	info := taskSubmissionRelayInfo(&taskSubmissionTestBilling{events: &events})
	outcome, taskErr := executeTaskSubmissionWith(c, info, func(_ *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
		assert.Equal(t, "task_waiting", info.PublicTaskID)
		return &relay.TaskSubmitResult{UpstreamTaskID: "vendor_job", Platform: constant.TaskPlatform("video"), Quota: 3}, nil
	})
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	var rows []model.Task
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, reserved.ID, rows[0].ID)
	assert.Equal(t, "task_waiting", rows[0].TaskID)
	assert.Equal(t, ActionInfluencerMotion, rows[0].Action)
	assert.Equal(t, `{"kind":"motion"}`, rows[0].Properties.Input)
	assert.Equal(t, "vendor_job", rows[0].PrivateData.UpstreamTaskID)
	assert.Contains(t, events, "settle")
	assert.NotContains(t, events, "refund")
}

func TestGenerationQueueOldWorkerCannotRejectNewClaim(t *testing.T) {
	events := []string{}
	db := setupTaskSubmissionDatabase(t, true, &events)
	old := &model.Task{TaskID: "old_worker", UserId: 1, Status: model.TaskStatusClaimed, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{ClaimOwner: "old"}}}
	require.NoError(t, old.Insert())
	newer := *old
	newer.PrivateData.GenerationQueue = &model.GenerationQueueRequest{ClaimOwner: "new"}
	require.NoError(t, newer.Update())
	failGenerationBeforeDispatch(context.Background(), old, "old rejection")
	var stored model.Task
	require.NoError(t, db.First(&stored, old.ID).Error)
	assert.Equal(t, model.TaskStatusClaimed, stored.Status)
	assert.Empty(t, stored.FailReason)
}

func TestGenerationQueueReceiptPersistsWithoutBillingOrSecrets(t *testing.T) {
	events := []string{}
	db := setupTaskSubmissionDatabase(t, true, &events)
	c := taskSubmissionTestContext()
	c.Set("id", 17)
	c.Request.Header.Set("Authorization", "secret")
	c.Request.Header.Set("Cookie", "private")
	c.Request.Header.Set("X-Omnimux-Group", "image-group")
	influencerRelaySubmit(c, "/pg/images/generations", ActionInfluencerSheet, map[string]any{"model": "gpt-image-2.5", "prompt": "person"}, map[string]any{"brief": "person"})
	assert.Equal(t, 200, c.Writer.Status())
	var tasks []model.Task
	require.NoError(t, db.Find(&tasks).Error)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskStatusWaiting, tasks[0].Status)
	assert.Zero(t, tasks[0].Quota)
	assert.NotNil(t, tasks[0].PrivateData.GenerationQueue)
	assert.JSONEq(t, `{"brief":"person"}`, tasks[0].Properties.Input)
	encoded, err := common.Marshal(tasks[0].PrivateData)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "secret")
	assert.NotContains(t, string(encoded), "private")
	assert.Equal(t, []string{"insert"}, events)
}
