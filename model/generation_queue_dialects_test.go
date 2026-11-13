package model

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// These tests replace the single-connection TestMain fixture only for their
// lifetime. Server databases must be explicitly selected and task-owned;
// ordinary model tests still use their existing in-memory SQLite fixture.
func TestGenerationQueueDatabaseMatrix(t *testing.T) {
	selected := os.Getenv("__TEST_GENERATION_QUEUE_DIALECT")
	if selected == "" {
		selected = "sqlite"
	}
	dialects := []struct {
		name string
		kind common.DatabaseType
	}{
		{"sqlite", common.DatabaseTypeSQLite},
		{"mysql", common.DatabaseTypeMySQL},
		{"postgres", common.DatabaseTypePostgreSQL},
	}
	found := false
	for _, dialect := range dialects {
		if dialect.name != selected {
			continue
		}
		found = true
		t.Run(dialect.name, func(t *testing.T) {
			dsn := os.Getenv("__TEST_GENERATION_QUEUE_DSN")
			var driver gorm.Dialector
			switch dialect.kind {
			case common.DatabaseTypeSQLite:
				if dsn == "" {
					dsn = filepath.Join(t.TempDir(), "__test_generation_queue.db")
				}
				require.NotContains(t, dsn, ":memory:", "multi-connection proof requires a shared file")
				require.Contains(t, filepath.Base(strings.Split(dsn, "?")[0]), "__test_generation_queue")
				driver = sqlite.Open(dsn + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
			case common.DatabaseTypeMySQL:
				require.NotEmpty(t, dsn, "explicit task-owned test DSN is required; no skipped matrix")
				cfg, err := mysqldriver.ParseDSN(dsn)
				require.NoError(t, err)
				require.Equal(t, "tcp", cfg.Net)
				require.True(t, strings.HasPrefix(cfg.DBName, "__test_"))
				host, port, err := net.SplitHostPort(cfg.Addr)
				require.NoError(t, err)
				require.Equal(t, "127.0.0.1", host)
				number, err := strconv.Atoi(port)
				require.NoError(t, err)
				require.GreaterOrEqual(t, number, 49152)
				driver = mysql.Open(dsn)
			case common.DatabaseTypePostgreSQL:
				require.NotEmpty(t, dsn, "explicit task-owned test DSN is required; no skipped matrix")
				cfg, err := url.Parse(dsn)
				require.NoError(t, err)
				require.Equal(t, "postgres", cfg.Scheme)
				require.Equal(t, "127.0.0.1", cfg.Hostname())
				require.True(t, strings.HasPrefix(cfg.Path, "/__test_"))
				number, err := strconv.Atoi(cfg.Port())
				require.NoError(t, err)
				require.GreaterOrEqual(t, number, 49152)
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(8)
			pool.SetMaxIdleConns(8)
			oldDB, oldLog := DB, LOG_DB
			oldMain, oldLogKind := common.MainDatabaseType(), common.LogDatabaseType()
			DB, LOG_DB = db, db
			common.SetDatabaseTypes(dialect.kind, dialect.kind)
			initCol()
			t.Cleanup(func() {
				DB, LOG_DB = oldDB, oldLog
				common.SetDatabaseTypes(oldMain, oldLogKind)
				initCol()
				require.NoError(t, pool.Close())
			})
			// This fixture creates missing native tables; it is not a restart
			// migration test for the unrelated User schema.
			for _, table := range []any{&User{}, &Task{}, &SystemTaskLock{}} {
				if !db.Migrator().HasTable(table) {
					require.NoError(t, db.AutoMigrate(table))
				}
			}
			var version string
			versionQuery := "SELECT version()"
			if dialect.kind == common.DatabaseTypeSQLite {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("DATABASE_PROOF dialect=%s version=%s max_open=%d max_idle=8", dialect.name, version, pool.Stats().MaxOpenConnections)
			t.Run("ConcurrentMixedFIFOAccountIsolation", func(t *testing.T) {
				testGenerationQueueDialectConcurrency(t, db, pool, dialect.kind)
			})
			t.Run("ScannerOwnerSnapshotAndTerminalFences", testGenerationQueueDialectOwnership)
			t.Run("ExpiryReleasesStaleDispatchSlots", testGenerationQueueDialectExpiry)
		})
	}
	require.True(t, found, "unsupported test database selection %q", selected)
}

func testGenerationQueueDialectConcurrency(t *testing.T, db *gorm.DB, pool *sql.DB, dialect common.DatabaseType) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	user := User{Username: "matrix_queue_one", AffCode: "matrix_queue_one", Status: common.UserStatusEnabled}
	other := User{Username: "matrix_queue_two", AffCode: "matrix_queue_two", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&other).Error)
	t.Cleanup(func() {
		require.NoError(t, db.Where("user_id IN ?", []int{user.Id, other.Id}).Delete(&Task{}).Error)
		require.NoError(t, db.Unscoped().Where("id IN ?", []int{user.Id, other.Id}).Delete(&User{}).Error)
	})
	var ids []int64
	for _, account := range []User{user, other} {
		for i := 0; i < 6; i++ {
			action := "influencer_sheet"
			if i%2 == 1 {
				action = "influencer_motion"
			}
			task := &Task{TaskID: GenerateTaskID(), UserId: account.Id, Action: action, Status: TaskStatusWaiting, Progress: "0%", Properties: Properties{Input: `{"history":"preserve"}`}, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{Path: "/pg/video/generations", Body: []byte(`{"prompt":"数据库测试"}`)}}}
			require.NoError(t, task.Insert())
			if account.Id == user.Id {
				ids = append(ids, task.ID)
			}
		}
	}

	// The barrier runs inside each real ClaimGenerationTasks transaction before
	// its first lock/write, proving overlap rather than pool serialization.
	type contenderKey struct{}
	type arrival struct {
		transaction bool
		connection  int64
		err         error
	}
	entered := make(chan arrival, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var first [2]sync.Once
	callback := func(tx *gorm.DB) {
		index, ok := tx.Statement.Context.Value(contenderKey{}).(int)
		if !ok {
			return
		}
		first[index].Do(func() {
			_, realTransaction := tx.Statement.ConnPool.(*sql.Tx)
			proof := arrival{transaction: realTransaction}
			if dialect != common.DatabaseTypeSQLite {
				query := "SELECT CONNECTION_ID()"
				if dialect == common.DatabaseTypePostgreSQL {
					query = "SELECT pg_backend_pid()"
				}
				proof.err = tx.Statement.ConnPool.QueryRowContext(tx.Statement.Context, query).Scan(&proof.connection)
			}
			entered <- proof
			select {
			case <-release:
			case <-ctx.Done():
				tx.AddError(ctx.Err())
			}
		})
	}
	callbackName := "test:queue_matrix_transaction_overlap"
	if dialect == common.DatabaseTypeSQLite {
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, callback))
		defer db.Callback().Update().Remove(callbackName)
	} else {
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, callback))
		defer db.Callback().Query().Remove(callbackName)
	}
	type result struct {
		tasks []*Task
		err   error
	}
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			tasks, err := ClaimGenerationTasks(context.WithValue(ctx, contenderKey{}, index), user.Id, "matrix-owner")
			results <- result{tasks, err}
		}(index)
	}
	var proofs []arrival
	for i := 0; i < 2; i++ {
		select {
		case proof := <-entered:
			proofs = append(proofs, proof)
		case <-ctx.Done():
			releaseOnce.Do(func() { close(release) })
			workers.Wait()
			t.Fatal("two independent claim transactions did not reach the bounded barrier")
		}
	}
	stats := pool.Stats()
	releaseOnce.Do(func() { close(release) })
	workers.Wait()
	close(results)
	for _, proof := range proofs {
		require.NoError(t, proof.err)
		assert.True(t, proof.transaction)
	}
	assert.GreaterOrEqual(t, stats.InUse, 2)
	if dialect != common.DatabaseTypeSQLite {
		assert.NotEqual(t, proofs[0].connection, proofs[1].connection)
	}
	t.Logf("TRANSACTION_PROOF overlap=2 pool_in_use=%d connection_ids=[%d,%d]", stats.InUse, proofs[0].connection, proofs[1].connection)
	var claimedIDs []int64
	var claimed []*Task
	for result := range results {
		require.NoError(t, result.err)
		claimed = append(claimed, result.tasks...)
		for _, task := range result.tasks {
			claimedIDs = append(claimedIDs, task.ID)
			assert.Zero(t, task.Quota)
		}
	}
	sort.Slice(claimedIDs, func(i, j int) bool { return claimedIDs[i] < claimedIDs[j] })
	require.Len(t, claimedIDs, 5)
	assert.Equal(t, ids[:5], claimedIDs)
	var sixth Task
	require.NoError(t, db.First(&sixth, ids[5]).Error)
	assert.Equal(t, TaskStatusWaiting, sixth.Status)
	isolated, err := ClaimGenerationTasks(ctx, other.Id, "other-owner")
	require.NoError(t, err)
	require.Len(t, isolated, 5)
	for _, task := range isolated {
		assert.Equal(t, other.Id, task.UserId)
	}
	require.NoError(t, RejectGenerationDispatch(ctx, claimed[0], "test failure"))
	next, err := ClaimGenerationTasks(ctx, user.Id, "matrix-owner")
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.Equal(t, ids[5], next[0].ID)
	var rows, publicRows int64
	require.NoError(t, db.Model(&Task{}).Where("user_id = ?", user.Id).Count(&rows).Error)
	require.NoError(t, db.Model(&Task{}).Where("user_id = ?", user.Id).Distinct("task_id").Count(&publicRows).Error)
	assert.EqualValues(t, 6, rows)
	assert.EqualValues(t, 6, publicRows)
	t.Logf("QUEUE_PROOF initial_claims=5 sixth=WAITING refill_id=%d account_two_claims=5 account_one_rows=%d public_ids=%d", next[0].ID, rows, publicRows)
}

func testGenerationQueueDialectOwnership(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	user := User{Username: "matrix_owner", AffCode: "matrix_owner", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	reserved := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusWaiting, Properties: Properties{Input: `{"name":"历史","keep":true}`}, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{Body: []byte(`{"prompt":"测试"}`)}}}
	require.NoError(t, reserved.Insert())
	old, err := ClaimGenerationTasks(ctx, user.Id, "old-scanner")
	require.NoError(t, err)
	require.Len(t, old, 1)
	scanner := &SystemTask{TaskID: "matrix-new-scanner"}
	lease := &SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "matrix-runner", LockedUntil: common.GetTimestamp() + 300}
	require.NoError(t, DB.Create(lease).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&Task{}).Error)
		require.NoError(t, DB.Unscoped().Where("id = ?", user.Id).Delete(&User{}).Error)
		require.NoError(t, DB.Where("type = ? AND task_id = ?", lease.Type, lease.TaskID).Delete(&SystemTaskLock{}).Error)
	})
	assert.Error(t, RecoverGenerationClaims(ctx, scanner, "stale-runner"))
	require.NoError(t, RecoverGenerationClaims(ctx, scanner, "matrix-runner"))
	current, err := ClaimGenerationTasks(ctx, user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	begun, err := BeginGenerationDispatch(ctx, old[0], &SystemTask{TaskID: "old-scanner"}, "old-runner")
	assert.Error(t, err)
	assert.False(t, begun)
	begun, err = BeginGenerationDispatch(ctx, current[0], scanner, "stale-runner")
	assert.Error(t, err)
	assert.False(t, begun)
	begun, err = BeginGenerationDispatch(ctx, current[0], scanner, "matrix-runner")
	require.NoError(t, err)
	require.True(t, begun)

	snapshot := &Task{UserId: user.Id, TaskID: reserved.TaskID, Quota: 123, ChannelId: 61, Platform: "matrix-test", Group: "test-group", Properties: Properties{OriginModelName: "public-model", UpstreamModelName: "supplier-model"}, PrivateData: TaskPrivateData{BillingSource: "subscription", SubscriptionId: 77, SubscriptionWalletConsumed: 23, SubscriptionAllowanceEpoch: 9, SubscriptionAllowanceAllocated: 100, Execution: &TaskExecutionSnapshot{RequestID: "matrix-native-request", RequestPath: "/pg/video/generations"}, BillingContext: &TaskBillingContext{ModelPrice: 0.25, GroupRatio: 0.8, PerCallBilling: true, OtherRatios: map[string]float64{"seconds": 5}}}}
	assert.Error(t, PersistGenerationSendSnapshot(ctx, old[0], snapshot, scanner, "matrix-runner"))
	assert.Error(t, PersistGenerationSendSnapshot(ctx, current[0], snapshot, scanner, "stale-runner"))
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()-1).Error)
	assert.Error(t, PersistGenerationSendSnapshot(ctx, current[0], snapshot, scanner, "matrix-runner"))
	require.NoError(t, DB.Model(&SystemTaskLock{}).Where("task_id = ?", scanner.TaskID).Update("locked_until", common.GetTimestamp()+300).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
	assert.Error(t, PersistGenerationSendSnapshot(ctx, current[0], snapshot, scanner, "matrix-runner"))
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("status", common.UserStatusEnabled).Error)
	require.NoError(t, PersistGenerationSendSnapshot(ctx, current[0], snapshot, scanner, "matrix-runner"))
	require.NoError(t, PersistGenerationSendSnapshot(ctx, current[0], snapshot, scanner, "matrix-runner"), "a matching funding snapshot must remain valid without a data change")
	var saved Task
	require.NoError(t, DB.First(&saved, reserved.ID).Error)
	assert.Equal(t, reserved.TaskID, saved.TaskID)
	assert.Equal(t, reserved.Properties.Input, saved.Properties.Input)
	assert.Equal(t, snapshot.Quota, saved.Quota)
	assert.Equal(t, snapshot.ChannelId, saved.ChannelId)
	assert.Equal(t, snapshot.Group, saved.Group)
	assert.Equal(t, snapshot.Properties.OriginModelName, saved.Properties.OriginModelName)
	assert.Equal(t, snapshot.Properties.UpstreamModelName, saved.Properties.UpstreamModelName)
	assert.Equal(t, snapshot.PrivateData.BillingSource, saved.PrivateData.BillingSource)
	assert.Equal(t, snapshot.PrivateData.SubscriptionId, saved.PrivateData.SubscriptionId)
	assert.Equal(t, snapshot.PrivateData.SubscriptionAllowanceEpoch, saved.PrivateData.SubscriptionAllowanceEpoch)
	assert.Equal(t, snapshot.PrivateData.SubscriptionAllowanceAllocated, saved.PrivateData.SubscriptionAllowanceAllocated)
	assert.Equal(t, snapshot.PrivateData.SubscriptionWalletConsumed, saved.PrivateData.SubscriptionWalletConsumed)
	assert.Equal(t, snapshot.PrivateData.Execution, saved.PrivateData.Execution)
	assert.Equal(t, snapshot.PrivateData.BillingContext, saved.PrivateData.BillingContext)
	require.NotNil(t, saved.PrivateData.GenerationQueue)
	assert.Equal(t, scanner.TaskID, saved.PrivateData.GenerationQueue.ClaimOwner)
	assert.Positive(t, saved.PrivateData.GenerationQueue.SendStartedAt)
	assert.JSONEq(t, `{"prompt":"测试"}`, string(saved.PrivateData.GenerationQueue.Body))
	var waitingID int64
	for i := 0; i < 5; i++ {
		status := TaskStatusDispatching
		if i == 4 {
			status = TaskStatusWaiting
		}
		task := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: status, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{ClaimOwner: scanner.TaskID}}}
		require.NoError(t, task.Insert())
		waitingID = task.ID
	}
	require.NoError(t, RecoverGenerationClaims(ctx, scanner, "matrix-runner"))
	blocked, err := ClaimGenerationTasks(ctx, user.Id, scanner.TaskID)
	require.NoError(t, err)
	assert.Empty(t, blocked, "persisted unknown dispatch must keep its fifth slot")
	var recovered Task
	require.NoError(t, DB.First(&recovered, saved.ID).Error)
	assert.Equal(t, saved, recovered, "recovery must not reset or erase a dispatch snapshot")

	// Stale writes must leave the entire current snapshot untouched, including
	// terminal states. Comparison uses reloaded values, not dialect JSON SQL.
	for _, status := range []TaskStatus{TaskStatusDispatching, TaskStatusSuccess, TaskStatusFailure} {
		t.Run(string(status), func(t *testing.T) {
			require.NoError(t, DB.Model(&Task{}).Where("id = ?", saved.ID).Updates(map[string]any{"status": status, "fail_reason": "current-owner", "progress": "100%"}).Error)
			var before Task
			require.NoError(t, DB.First(&before, saved.ID).Error)
			assert.Error(t, PersistGenerationSendSnapshot(ctx, old[0], &Task{UserId: user.Id, TaskID: reserved.TaskID, Quota: 999}, scanner, "matrix-runner"))
			require.NoError(t, RejectGenerationDispatch(ctx, old[0], "stale failure"))
			refundErr := RejectGenerationDispatch(ctx, old[0], "stale refunded failure", &Task{Quota: 0})
			if status == TaskStatusDispatching {
				assert.Error(t, refundErr)
			} else {
				assert.NoError(t, refundErr)
			}
			assert.Error(t, PersistGenerationSubmission(ctx, &Task{UserId: user.Id, Status: TaskStatusSuccess, Quota: 999}, old[0]))
			var after Task
			require.NoError(t, DB.First(&after, saved.ID).Error)
			assert.Equal(t, before, after)
		})
	}
	// Matching owners still cannot mutate an already terminal row.
	assert.Error(t, PersistGenerationSendSnapshot(ctx, current[0], snapshot, scanner, "matrix-runner"))
	require.NoError(t, RejectGenerationDispatch(ctx, current[0], "late failure"))
	assert.Error(t, PersistGenerationSubmission(ctx, &Task{UserId: user.Id, Status: TaskStatusSuccess}, current[0]))
	var terminal Task
	require.NoError(t, DB.First(&terminal, saved.ID).Error)
	assert.EqualValues(t, TaskStatusFailure, terminal.Status)
	assert.Equal(t, "current-owner", terminal.FailReason)
	var rows int64
	require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", reserved.TaskID).Count(&rows).Error)
	assert.EqualValues(t, 1, rows)
	// A valid owner publishes success into the reserved row, not a new row.
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", saved.ID).Update("status", TaskStatusDispatching).Error)
	published := &Task{UserId: user.Id, Status: TaskStatusSuccess, Progress: "100%", Quota: 123, PrivateData: saved.PrivateData, Properties: Properties{OriginModelName: "public-model"}}
	require.NoError(t, PersistGenerationSubmission(ctx, published, current[0]))
	var success Task
	require.NoError(t, DB.First(&success, saved.ID).Error)
	assert.EqualValues(t, TaskStatusSuccess, success.Status)
	assert.Equal(t, reserved.TaskID, success.TaskID)
	assert.Equal(t, reserved.Properties.Input, success.Properties.Input)
	require.NoError(t, DB.Model(&Task{}).Where("task_id = ?", reserved.TaskID).Count(&rows).Error)
	assert.EqualValues(t, 1, rows)
	refilled, err := ClaimGenerationTasks(ctx, user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, refilled, 1)
	assert.Equal(t, waitingID, refilled[0].ID)
	t.Logf("OWNER_PROOF stale_snapshot_success_failure_blocked=true public_id=%s rows=%d snapshot_roundtrip=true", reserved.TaskID, rows)
	// No settle/refund/preconsume is invoked: the numeric snapshot is fixture
	// data, not evidence of funds being charged or recovered.
}

func testGenerationQueueDialectExpiry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	user := User{Username: "matrix_expiry", AffCode: "matrix_expiry", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	scanner := &SystemTask{TaskID: "matrix-expiry-scanner"}
	lease := &SystemTaskLock{Type: SystemTaskTypeGenerationQueue, TaskID: scanner.TaskID, LockedBy: "matrix-expiry-runner", LockedUntil: common.GetTimestamp() + 300}
	require.NoError(t, DB.Create(lease).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&Task{}).Error)
		require.NoError(t, DB.Unscoped().Where("id = ?", user.Id).Delete(&User{}).Error)
		require.NoError(t, DB.Where("type = ? AND task_id = ?", lease.Type, lease.TaskID).Delete(&SystemTaskLock{}).Error)
	})
	const cutoff int64 = 10_000
	// Slot rows: 0 funded, 1 release already begun by its worker, 2 still
	// running in this scanner, 3–4 never sent.
	rows := make([]*Task, GenerationConcurrency)
	for i := range rows {
		queue := &GenerationQueueRequest{ClaimOwner: "old-scanner"}
		switch i {
		case 0:
			queue.SendStartedAt = cutoff - 2
		case 1:
			queue.SendStartedAt = cutoff - 2
		case 2:
			queue.ClaimOwner = scanner.TaskID
			queue.SendStartedAt = cutoff - 2
		}
		rows[i] = &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_motion", Status: TaskStatusDispatching, StartTime: cutoff - 1, Quota: 777,
			PrivateData: TaskPrivateData{BillingSource: "wallet", GenerationQueue: queue}}
		require.NoError(t, rows[i].Insert())
	}
	require.NoError(t, MarkGenerationReleaseStarted(ctx, rows[1]))
	waiting := &Task{TaskID: GenerateTaskID(), UserId: user.Id, Action: "influencer_sheet", Status: TaskStatusWaiting, PrivateData: TaskPrivateData{GenerationQueue: &GenerationQueueRequest{}}}
	require.NoError(t, waiting.Insert())
	blocked, err := ClaimGenerationTasks(ctx, user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Empty(t, blocked)
	inFlight := []int64{rows[2].ID}

	_, err = ExpireGenerationDispatches(ctx, scanner, "stale-runner", cutoff, 60, inFlight)
	assert.Error(t, err)
	refundable, err := ExpireGenerationDispatches(ctx, scanner, "matrix-expiry-runner", cutoff, 60, inFlight)
	require.NoError(t, err)
	require.Len(t, refundable, 1)
	assert.Equal(t, rows[0].ID, refundable[0].ID)
	again, err := ExpireGenerationDispatches(ctx, scanner, "matrix-expiry-runner", cutoff, 60, inFlight)
	require.NoError(t, err)
	assert.Empty(t, again)
	assert.Error(t, MarkGenerationReleaseStarted(ctx, rows[0]), "a late worker cannot release an expired row")
	var failed int64
	require.NoError(t, DB.Model(&Task{}).Where("user_id = ? AND status = ?", user.Id, TaskStatusFailure).Count(&failed).Error)
	assert.EqualValues(t, GenerationConcurrency-1, failed)
	var running Task
	require.NoError(t, DB.First(&running, rows[2].ID).Error)
	assert.Equal(t, TaskStatusDispatching, running.Status, "a row whose worker still runs is never expired")
	var kept Task
	require.NoError(t, DB.First(&kept, rows[0].ID).Error)
	assert.Equal(t, 777, kept.Quota, "expiry leaves funding to the native release")
	refilled, err := ClaimGenerationTasks(ctx, user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, refilled, 1)
	assert.Equal(t, waiting.ID, refilled[0].ID)
}
