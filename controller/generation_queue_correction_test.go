package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"path/filepath"
)

// Only the provider is simulated: routing, scheduler, task persistence and native wallet run unchanged.
func generationCorrectionFixture(t *testing.T, provider http.Handler, customize ...func(string) string) (*gorm.DB, model.User, *model.SystemTask) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "queue.sqlite")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Native SystemTask SQL names its real tables; isolate with a fresh database,
	// not a table prefix, and allow actual independent connections.
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Task{}, &model.Log{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.UserSubscription{}, &model.SubscriptionPlan{}, &model.SubscriptionPreConsumeRecord{}))
	dialect := common.DatabaseTypeSQLite
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
	redisServer := miniredis.RunT(t)
	oldRDB := common.RDB
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	common.RDB = redisClient
	t.Cleanup(func() { common.RDB = oldRDB; require.NoError(t, redisClient.Close()) })
	common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, false, true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLog
		common.SetDatabaseTypes(oldMain, oldLogType)
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldRedis, oldMemory, oldBatch, oldConsume
	})
	oldGroups := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups)) })
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))
	withTieredBillingConfig(t, map[string]string{"queue-correction": "tiered_expr"}, map[string]string{"queue-correction": `u("units") * 0.01`})
	upstream := httptest.NewServer(provider)
	t.Cleanup(upstream.Close)
	source := `export const meta={apiVersion:1,key:"901",channelTypes:[901],name:"Queue",version:"1.0.0",author:{name:"Test"},models:["queue-correction"],protocols:["openai_image","openai_video"],fetchMode:"per_task",usageSchema:{units:{type:"number",unit:"count"}}};export function buildSubmitRequest(ctx){return {url:ctx.baseUrl+"/submit",body:ctx.requestBody};}export function parseSubmitResponse(ctx,resp){return {taskId:resp.body.id,taskData:resp.body,immediate:resp.body.status?{status:resp.body.status,progress:"100%"}:undefined};}export function extractUsage(){return {units:1};}export function listArtifacts(){return [];}export function buildContentRequest(){return {};}export function buildQueryRequest(){return {}}export function parseTaskResult(){return {status:"SUCCESS"}};const decodeRequest=function(ctx){return {kind:"submit",model:ctx.model,requestBody:ctx.body.value};};export const protocols={openai_image:{decodeRequest,render:function(){return {data:[]};}},openai_video:{decodeRequest,render:function(){return {};}}};`
	for _, change := range customize {
		source = change(source)
	}
	_, err = pluginruntime.DefaultRegistry.Register(source, pluginruntime.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { pluginruntime.DefaultRegistry.Unregister("901") })
	user := model.User{Username: "queue_correction", AffCode: "queue_correction", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&user).Error)
	ch := model.Channel{Name: "local only", Type: 901, Status: common.ChannelStatusEnabled, Key: "test-only", Group: "default", Models: "queue-correction", BaseURL: &upstream.URL, Setting: common.GetPointer(`{"task_plugin_key":"901"}`)}
	require.NoError(t, db.Create(&ch).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "queue-correction", ChannelId: ch.Id, Enabled: true}).Error)
	scanner, err := model.CreateSystemTask(model.SystemTaskTypeGenerationQueue, nil, nil)
	require.NoError(t, err)
	scanner, claimed, err := model.ClaimSystemTask(scanner.ID, scanner.Type, "correction", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimed)
	// This cleanup is registered last: native quota/cache jobs must finish before
	// shared DB/Redis configuration or the registry is restored.
	t.Cleanup(func() {
		require.Eventually(t, func() bool { return gopool.WorkerCount() == 0 }, 5*time.Second, time.Millisecond, "native background jobs did not finish")
	})
	return db, user, scanner
}

func insertCorrectionTask(t *testing.T, user model.User, number int, video bool) *model.Task {
	t.Helper()
	action, path := ActionInfluencerSheet, "/pg/images/generations"
	if video {
		action, path = ActionInfluencerMotion, "/pg/video/generations"
	}
	task := &model.Task{TaskID: fmt.Sprintf("correction-%d", number), UserId: user.Id, Group: "default", Action: action, Status: model.TaskStatusWaiting, Progress: "0%", Properties: model.Properties{Input: `{"brief":"person"}`, OriginModelName: "queue-correction"}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: path, Body: []byte(fmt.Sprintf(`{"model":"queue-correction","prompt":"%d"}`, number))}}}
	require.NoError(t, task.Insert())
	return task
}

func TestGenerationQueueIndependentConnectionsCannotClaimSixthSlot(t *testing.T) {
	_, user, _ := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("claiming must not submit upstream") }))
	for i := 1; i <= 6; i++ {
		insertCorrectionTask(t, user, i, i%2 == 0)
	}
	start := make(chan struct{})
	results := make(chan []*model.Task, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tasks, err := model.ClaimGenerationTasks(context.Background(), user.Id, "claimant")
			results <- tasks
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var claimed []*model.Task
	for batch := range results {
		claimed = append(claimed, batch...)
	}
	require.Len(t, claimed, 5)
	for _, task := range claimed {
		assert.NotEqual(t, "correction-6", task.TaskID)
	}
	stored, found, err := model.GetByTaskId(user.Id, "correction-6")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusWaiting, stored.Status)
}

func TestGenerationQueueDefiniteHTTP400FailsRefundsAndReleasesSlot(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"rejected"}`))
	}))
	task := insertCorrectionTask(t, user, 1, false)
	for i := 2; i <= 5; i++ {
		active := insertCorrectionTask(t, user, i, false)
		require.NoError(t, db.Model(active).Update("status", model.TaskStatusDispatching).Error)
	}
	sixth := insertCorrectionTask(t, user, 6, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Zero(t, stored.Quota)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Zero(t, account.UsedQuota)
	var channel model.Channel
	require.NoError(t, db.First(&channel).Error)
	assert.Zero(t, channel.UsedQuota)
	require.Eventually(t, func() bool {
		updated, err := model.GetUserById(user.Id, false)
		return err == nil && updated.Quota == 100000
	}, time.Second, 10*time.Millisecond, "native wallet precharge must be refunded")
	next, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.Equal(t, sixth.ID, next[0].ID)
	assert.Equal(t, int32(1), calls.Load())
}

func TestGenerationQueueUnknownResponsesKeepSlotAndNeverResend(t *testing.T) {
	for _, mode := range []string{"timeout", "disconnect", "missing_ack", "payload400", "http408", "http500"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			providerStarted := make(chan struct{}, 1)
			unblock := make(chan struct{})
			db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				providerStarted <- struct{}{}
				switch mode {
				case "timeout":
					<-unblock
				case "disconnect":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
				case "missing_ack":
					_, _ = w.Write([]byte(`{"unexpected":"accepted but no id"}`))
				case "payload400":
					_, _ = w.Write([]byte(`{"error":{"code":400,"message":"no receipt"}}`))
				case "http408":
					w.WriteHeader(http.StatusRequestTimeout)
				case "http500":
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			task := insertCorrectionTask(t, user, 1, false)
			for i := 2; i <= 5; i++ {
				active := insertCorrectionTask(t, user, i, false)
				require.NoError(t, db.Model(active).Update("status", model.TaskStatusDispatching).Error)
			}
			insertCorrectionTask(t, user, 6, false)
			claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			ctx := context.Background()
			var cancel context.CancelFunc
			if mode == "timeout" {
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
			}
			done := make(chan struct{})
			go func() { dispatchInfluencerGeneration(ctx, claimed[0], scanner, "correction"); close(done) }()
			select {
			case <-providerStarted:
			case <-time.After(time.Second):
				require.FailNow(t, "local upstream request did not begin")
			}
			if mode == "timeout" {
				cancel() // provider receipt, not elapsed time, orders cancellation
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				close(unblock)
				require.FailNow(t, "local submission did not return")
			}
			close(unblock)
			stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, model.TaskStatusDispatching, stored.Status)
			require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "correction"))
			again, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
			require.NoError(t, err)
			assert.Empty(t, again)
			assert.Equal(t, int32(1), calls.Load())
			// An accepted-but-unknown POST retains actual native funding for reconciliation.
			updated, err := model.GetUserById(user.Id, false)
			require.NoError(t, err)
			assert.Equal(t, 95000, updated.Quota)
			assert.Equal(t, 5000, stored.Quota)
			assert.Equal(t, "wallet", stored.PrivateData.BillingSource)
			require.NotNil(t, stored.PrivateData.BillingContext)
			require.NotNil(t, stored.PrivateData.Execution)
			assert.Contains(t, stored.FailReason, "待核对")
			assert.Contains(t, stored.FailReason, "阶段：submit")
			assert.Contains(t, stored.FailReason, "代码：")
			assert.NotContains(t, stored.FailReason, "no receipt")
			if mode == "missing_ack" {
				dispatchInfluencerGeneration(context.Background(), stored, scanner, "correction")
				assert.Equal(t, int32(1), calls.Load(), "a reloaded unknown dispatch must never enter submission again")
			}
		})
	}
}

func TestGenerationQueueLateOwnerCannotPersistOverNewDispatch(t *testing.T) {
	db, user, _ := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("no provider request expected") }))
	old := insertCorrectionTask(t, user, 1, false)
	old.Status = model.TaskStatusDispatching
	old.PrivateData.GenerationQueue.ClaimOwner = "old"
	require.NoError(t, old.Update())
	newer := *old
	newer.PrivateData.GenerationQueue = &model.GenerationQueueRequest{ClaimOwner: "new"}
	require.NoError(t, newer.Update())
	result := &model.Task{UserId: user.Id, Status: model.TaskStatusSuccess, Progress: "100%"}
	assert.Error(t, model.PersistGenerationSubmission(context.Background(), result, old))
	stored, found, err := model.GetByTaskId(user.Id, old.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	require.NotNil(t, stored.PrivateData.GenerationQueue)
	assert.Equal(t, "new", stored.PrivateData.GenerationQueue.ClaimOwner)
	require.NoError(t, db.Model(&model.Task{}).Where("id = ?", old.ID).Update("status", model.TaskStatusFailure).Error)
}

func TestGenerationQueueFiveRequestsStartAndRefillBeforeLongHTTPFinishes(t *testing.T) {
	started := make(chan string, 10)
	release := make([]chan struct{}, 8)
	for i := 1; i <= 7; i++ {
		release[i] = make(chan struct{})
	}
	var finished atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := common.DecodeJson(r.Body, &body); err != nil {
			http.Error(w, "invalid fixture", 500)
			return
		}
		var number int
		_, _ = fmt.Sscanf(body.Prompt, "%d", &number)
		started <- body.Prompt
		<-release[number]
		finished.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":"vendor-%d","status":"SUCCESS"}`, number)
	}))
	for i := 1; i <= 6; i++ {
		insertCorrectionTask(t, user, i, i == 4 || i == 5)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { generationQueueHandler{}.Run(ctx, scanner, "correction"); close(done) }()
	var once sync.Once
	seenFirstReleased := false
	cleanup := func() {
		once.Do(func() {
			for i := 1; i <= 7; i++ {
				if i != 1 || !seenFirstReleased {
					close(release[i])
				}
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				cancel()
				<-done
				t.Error("scanner did not stop")
			}
			cancel()
		})
	}
	defer cleanup()
	seen := map[string]bool{}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for len(seen) < 5 {
		select {
		case id := <-started:
			seen[id] = true
		case <-deadline.C:
			require.FailNow(t, "five real upstream requests must start while all responses are blocked", fmt.Sprintf("started=%v", seen))
		}
	}
	assert.NotContains(t, seen, "6")
	var sixth model.Task
	require.NoError(t, db.Where("task_id = ?", "correction-6").First(&sixth).Error)
	assert.Equal(t, model.TaskStatusWaiting, sixth.Status)
	assert.Zero(t, finished.Load())
	other := model.User{Username: "queue_other", AffCode: "queue_other", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000, Setting: `{"billing_preference":"wallet_only"}`}
	require.NoError(t, db.Create(&other).Error)
	insertCorrectionTask(t, other, 7, false)
	select {
	case id := <-started:
		require.Equal(t, "7", id)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "periodic scan must start newly waiting account while all five first-account HTTP calls block")
	}
	assert.Zero(t, finished.Load())
	// Completion of one synchronous image must refill while four HTTP calls stay blocked.
	close(release[1])
	seenFirstReleased = true
	select {
	case id := <-started:
		require.Equal(t, "6", id)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "sixth request did not refill the free slot")
	}
	assert.Equal(t, int32(1), finished.Load())
	t.Log("five upstream requests started; sixth remained WAITING; one completion refilled sixth with four HTTP responses still blocked")
}
