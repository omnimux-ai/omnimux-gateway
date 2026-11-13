package model

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"os"
	"path/filepath"
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

// Explicitly selected server databases must be disposable task-owned local
// databases. No application DSN, standard server port, or remote host is used.
func TestInfluencerRequestDatabaseMatrix(t *testing.T) {
	selected := os.Getenv("__TEST_INFLUENCER_REQUEST_DIALECT")
	if selected == "" {
		selected = "sqlite"
	}
	dsn := os.Getenv("__TEST_INFLUENCER_REQUEST_DSN")
	var driver gorm.Dialector
	var kind common.DatabaseType
	switch selected {
	case "sqlite":
		kind = common.DatabaseTypeSQLite
		if dsn == "" {
			dsn = filepath.Join(t.TempDir(), "__test_influencer_request.db")
		}
		require.NotContains(t, dsn, ":memory:")
		require.Contains(t, filepath.Base(strings.Split(dsn, "?")[0]), "__test_influencer_request")
		driver = sqlite.Open(dsn + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
	case "mysql":
		kind = common.DatabaseTypeMySQL
		require.NotEmpty(t, dsn, "matrix requires an explicit task-owned DSN")
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
	case "postgres":
		kind = common.DatabaseTypePostgreSQL
		require.NotEmpty(t, dsn, "matrix requires an explicit task-owned DSN")
		cfg, err := url.Parse(dsn)
		require.NoError(t, err)
		require.Equal(t, "postgres", cfg.Scheme)
		require.Equal(t, "127.0.0.1", cfg.Hostname())
		require.True(t, strings.HasPrefix(cfg.Path, "/__test_"))
		number, err := strconv.Atoi(cfg.Port())
		require.NoError(t, err)
		require.GreaterOrEqual(t, number, 49152)
		driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	default:
		t.Fatalf("unsupported test database %q", selected)
	}
	db, err := gorm.Open(driver, &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	pool.SetMaxIdleConns(8)
	oldDB, oldLogDB := DB, LOG_DB
	oldMainKind, oldLogKind := common.MainDatabaseType(), common.LogDatabaseType()
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(kind, kind)
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMainKind, oldLogKind)
		initCol()
		require.NoError(t, pool.Close())
	})
	for _, table := range []any{&User{}, &UserAccessToken{}, &Task{}} {
		if !db.Migrator().HasTable(table) {
			require.NoError(t, db.AutoMigrate(table))
		}
	}
	// First installation and restart migration of the new table must preserve
	// accepted rows and its composite primary key on all three databases.
	require.NoError(t, db.AutoMigrate(&InfluencerGenerationRequest{}))
	var version string
	versionQuery := "SELECT version()"
	if kind == common.DatabaseTypeSQLite {
		versionQuery = "SELECT sqlite_version()"
	}
	require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
	t.Logf("DATABASE_PROOF dialect=%s version=%s max_open=%d", selected, version, pool.Stats().MaxOpenConnections)
	user := createAccessTokenTestUser(t, "influencer-matrix-owner")
	t.Cleanup(func() {
		require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&InfluencerGenerationRequest{}).Error)
		require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&Task{}).Error)
		require.NoError(t, db.Where("user_id = ?", user.Id).Delete(&UserAccessToken{}).Error)
		require.NoError(t, db.Unscoped().Where("id = ?", user.Id).Delete(&User{}).Error)
	})
	t.Run("IndependentTransactionsAcceptExactlyOnce", func(t *testing.T) {
		testInfluencerRequestDialectConcurrency(t, db, pool, kind, user.Id)
	})
	t.Run("CaseSensitiveIDsAndRestartMigration", func(t *testing.T) {
		ctx := context.Background()
		first, _, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "CaseRequest0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
		require.NoError(t, err)
		second, _, err := CommitInfluencerGenerationRequest(ctx, influencerRequestTestTask(user.Id), "caserequest0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
		require.NoError(t, err)
		assert.NotEqual(t, first.TaskID, second.TaskID)
		require.NoError(t, db.AutoMigrate(&InfluencerGenerationRequest{}))
		for _, requestID := range []string{"CaseRequest0001", "caserequest0001"} {
			receipt, err := GetInfluencerGenerationRequest(ctx, user.Id, requestID)
			require.NoError(t, err)
			assert.Equal(t, requestID, receipt.RequestID)
		}
		original, err := GetInfluencerGenerationRequest(ctx, user.Id, "CaseRequest0001")
		require.NoError(t, err)
		duplicate := *original
		duplicate.RequestID = "different-raw-id"
		assert.Error(t, db.Create(&duplicate).Error, "database key must independently reject duplicate account/request identity")
	})
}

func testInfluencerRequestDialectConcurrency(t *testing.T, db *gorm.DB, pool *sql.DB, dialect common.DatabaseType, userID int) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	type contenderKey struct{}
	type proof struct {
		transaction bool
		connection  int64
		err         error
	}
	entered := make(chan proof, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var once [2]sync.Once
	callback := func(tx *gorm.DB) {
		index, ok := tx.Statement.Context.Value(contenderKey{}).(int)
		if !ok {
			return
		}
		once[index].Do(func() {
			_, isTransaction := tx.Statement.ConnPool.(*sql.Tx)
			arrival := proof{transaction: isTransaction}
			if dialect != common.DatabaseTypeSQLite {
				query := "SELECT CONNECTION_ID()"
				if dialect == common.DatabaseTypePostgreSQL {
					query = "SELECT pg_backend_pid()"
				}
				arrival.err = tx.Statement.ConnPool.QueryRowContext(tx.Statement.Context, query).Scan(&arrival.connection)
			}
			entered <- arrival
			select {
			case <-release:
			case <-ctx.Done():
				tx.AddError(ctx.Err())
			}
		})
	}
	callbackName := "test:influencer_receipt_transaction_overlap"
	if dialect == common.DatabaseTypeSQLite {
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, callback))
		defer db.Callback().Update().Remove(callbackName)
	} else {
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, callback))
		defer db.Callback().Query().Remove(callbackName)
	}
	type result struct {
		task   *Task
		replay bool
		err    error
	}
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			task, replay, err := CommitInfluencerGenerationRequest(context.WithValue(ctx, contenderKey{}, i), influencerRequestTestTask(userID), "matrix-concurrent-0001", strings.Repeat("a", 64), 0, influencerRequestTestScope)
			results <- result{task, replay, err}
		}(i)
	}
	proofs := make([]proof, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case arrival := <-entered:
			proofs = append(proofs, arrival)
		case <-ctx.Done():
			releaseOnce.Do(func() { close(release) })
			workers.Wait()
			t.Fatal("two acceptance transactions did not reach the bounded overlap barrier")
		}
	}
	stats := pool.Stats()
	releaseOnce.Do(func() { close(release) })
	workers.Wait()
	close(results)
	for _, arrival := range proofs {
		require.NoError(t, arrival.err)
		assert.True(t, arrival.transaction)
	}
	assert.GreaterOrEqual(t, stats.InUse, 2)
	if dialect != common.DatabaseTypeSQLite {
		assert.NotEqual(t, proofs[0].connection, proofs[1].connection)
	}
	t.Logf("TRANSACTION_PROOF overlap=2 pool_in_use=%d connection_ids=[%d,%d]", stats.InUse, proofs[0].connection, proofs[1].connection)
	var taskID string
	newAcceptances, replays := 0, 0
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.task)
		if taskID == "" {
			taskID = result.task.TaskID
		}
		assert.Equal(t, taskID, result.task.TaskID)
		if result.replay {
			replays++
		} else {
			newAcceptances++
		}
	}
	assert.Equal(t, 1, newAcceptances)
	assert.Equal(t, 1, replays)
	var taskCount, receiptCount int64
	require.NoError(t, db.Model(&Task{}).Where("user_id = ?", userID).Count(&taskCount).Error)
	require.NoError(t, db.Model(&InfluencerGenerationRequest{}).Where("user_id = ?", userID).Count(&receiptCount).Error)
	assert.EqualValues(t, 1, taskCount)
	assert.EqualValues(t, 1, receiptCount)
}
