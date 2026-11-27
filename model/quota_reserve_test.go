package model

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func createReserveTestUser(t *testing.T, quota int) User {
	t.Helper()
	user := User{
		Username:    "reserve-user-" + common.GetRandomString(6),
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		Quota:       quota,
		AffCode:     "reserve-aff-" + common.GetRandomString(8),
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func createReserveTestToken(t *testing.T, remainQuota int) Token {
	t.Helper()
	token := Token{
		UserId:      1,
		Key:         "reserve-token-" + common.GetRandomString(8),
		Name:        "reserve-test",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: remainQuota,
	}
	require.NoError(t, token.Insert())
	return token
}

func getUserQuotaFromDB(t *testing.T, id int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").First(&user, id).Error)
	return user.Quota
}

func getTokenFromDB(t *testing.T, id int) Token {
	t.Helper()
	var token Token
	require.NoError(t, DB.First(&token, id).Error)
	return token
}

func resetBatchUpdateTestState(t *testing.T) {
	t.Helper()
	oldBatchEnabled := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	for i := range BatchUpdateTypeCount {
		batchUpdateLocks[i].Lock()
		batchUpdateStores[i] = make(map[int]int)
		batchUpdateLocks[i].Unlock()
	}
	t.Cleanup(func() {
		common.BatchUpdateEnabled = oldBatchEnabled
		for i := range BatchUpdateTypeCount {
			batchUpdateLocks[i].Lock()
			batchUpdateStores[i] = make(map[int]int)
			batchUpdateLocks[i].Unlock()
		}
	})
}

func TestTryReserveQuotaWithoutRedis(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)

	user := createReserveTestUser(t, 100)
	reserved, err := TryReserveUserQuota(user.Id, 60)
	require.NoError(t, err)
	assert.True(t, reserved)
	assert.Equal(t, 40, getUserQuotaFromDB(t, user.Id))

	reserved, err = TryReserveUserQuota(user.Id, 41)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 40, getUserQuotaFromDB(t, user.Id))

	token := createReserveTestToken(t, 80)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 25, false)
	require.NoError(t, err)
	assert.True(t, reserved)
	reloaded := getTokenFromDB(t, token.Id)
	assert.Equal(t, 55, reloaded.RemainQuota)
	assert.Equal(t, 25, reloaded.UsedQuota)

	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 56, false)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 55, getTokenFromDB(t, token.Id).RemainQuota)
}

func TestRedisBatchReserveNeverFallsBackToStaleDatabaseBalance(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true

	user := createReserveTestUser(t, 10)
	reserved, err := TryReserveUserQuota(user.Id, 8)
	require.NoError(t, err)
	assert.True(t, reserved)
	assert.Equal(t, 10, getUserQuotaFromDB(t, user.Id), "batch delta is not flushed yet")

	reserved, err = TryReserveUserQuota(user.Id, 3)
	require.NoError(t, err)
	assert.False(t, reserved, "stale DB balance must not authorize a second spend")
	cachedUser, err := GetUserCache(user.Id)
	require.NoError(t, err)
	assert.Equal(t, 2, cachedUser.Quota)

	token := createReserveTestToken(t, 9)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 7, false)
	require.NoError(t, err)
	assert.True(t, reserved)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 3, false)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 9, getTokenFromDB(t, token.Id).RemainQuota)

	batchUpdate()
	assert.Equal(t, 2, getUserQuotaFromDB(t, user.Id))
	reloadedToken := getTokenFromDB(t, token.Id)
	assert.Equal(t, 2, reloadedToken.RemainQuota)
	assert.Equal(t, 7, reloadedToken.UsedQuota)
}

func TestBatchUpdateAccumulatesTwoMaximumRequestCharges(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	common.BatchUpdateEnabled = true

	user := createReserveTestUser(t, common.MaxQuota*2+100)
	require.NoError(t, DecreaseUserQuota(user.Id, common.MaxQuota, false))
	require.NoError(t, DecreaseUserQuota(user.Id, common.MaxQuota, false))

	batchUpdate()
	assert.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
}

func TestBatchUpdateAccumulatorSaturatesOverflow(t *testing.T) {
	resetBatchUpdateTestState(t)

	addNewRecord(BatchUpdateTypeUserQuota, 1, math.MaxInt)
	addNewRecord(BatchUpdateTypeUserQuota, 1, 1)
	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	assert.Equal(t, math.MaxInt, batchUpdateStores[BatchUpdateTypeUserQuota][1])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()

	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	batchUpdateStores[BatchUpdateTypeUserQuota] = make(map[int]int)
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
	addNewRecord(BatchUpdateTypeUserQuota, 1, math.MinInt)
	addNewRecord(BatchUpdateTypeUserQuota, 1, -1)
	batchUpdateLocks[BatchUpdateTypeUserQuota].Lock()
	assert.Equal(t, math.MinInt, batchUpdateStores[BatchUpdateTypeUserQuota][1])
	batchUpdateLocks[BatchUpdateTypeUserQuota].Unlock()
}

func TestReserveFallsBackToDatabaseWhenRedisIsUnavailable(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	server := useUserCacheMiniRedis(t)

	user := createReserveTestUser(t, 20)
	require.NoError(t, populateUserCache(user))
	server.Close()

	// Redis 故障时降级为数据库条件更新：服务保持可用且不会超扣。
	reserved, err := TryReserveUserQuota(user.Id, 5)
	require.NoError(t, err)
	assert.True(t, reserved)
	assert.Equal(t, 15, getUserQuotaFromDB(t, user.Id))

	reserved, err = TryReserveUserQuota(user.Id, 16)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Equal(t, 15, getUserQuotaFromDB(t, user.Id))
}

func TestSynchronousReserveCompensatesCacheWhenPersistenceFails(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)

	user := createReserveTestUser(t, 10)
	require.NoError(t, populateUserCache(user))
	require.NoError(t, DB.Delete(&user).Error)

	reserved, err := TryReserveUserQuota(user.Id, 6)
	assert.False(t, reserved)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	cached, cacheErr := cacheGetUserBase(user.Id)
	require.NoError(t, cacheErr)
	assert.Equal(t, 10, cached.Quota)

	token := createReserveTestToken(t, 12)
	_, err = GetTokenByKey(token.Key, true)
	require.NoError(t, err)
	require.NoError(t, DB.Delete(&token).Error)
	reserved, err = TryReserveTokenQuota(token.Id, token.Key, 7, false)
	assert.False(t, reserved)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	cachedToken, cacheErr := cacheGetTokenByKey(token.Key)
	require.NoError(t, cacheErr)
	assert.Equal(t, 12, cachedToken.RemainQuota)
	assert.Zero(t, cachedToken.UsedQuota)
}

func TestTokenCacheInitPreservesLiveQuotaAndFenceBlocksStaleSnapshot(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	server := useUserCacheMiniRedis(t)

	token := createReserveTestToken(t, 100)
	loaded, err := GetTokenByKey(token.Key, true)
	require.NoError(t, err)
	stale := *loaded

	result, err := cacheApplyTokenQuotaDelta(token.Id, token.Key, -70)
	require.NoError(t, err)
	require.Equal(t, cacheQuotaOK, result)

	// 已存在的哈希只刷新 TTL：数据库快照不得覆盖已被原子预扣的余额。
	code, err := cacheInitToken(stale)
	require.NoError(t, err)
	assert.Equal(t, 2, code)
	cached, err := cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, 30, cached.RemainQuota)

	// 变更期间：fence 删除缓存并拦截并发读者手中的过期快照。
	require.NoError(t, invalidateTokenCacheForMutation(token.Key))
	code, err = cacheInitToken(stale)
	require.NoError(t, err)
	assert.Zero(t, code, "the pre-mutation snapshot must not be published while fenced")
	_, err = cacheGetTokenByKey(token.Key)
	assert.Error(t, err)

	// fence 过期后可重新从数据库水合。
	server.FastForward(time.Duration(tokenCacheFenceSeconds+1) * time.Second)
	fresh, err := GetTokenByKey(token.Key, false)
	require.NoError(t, err)
	assert.Equal(t, 100, fresh.RemainQuota)
	cached, err = cacheGetTokenByKey(token.Key)
	require.NoError(t, err)
	assert.Equal(t, 100, cached.RemainQuota)
}

func TestDirectPurchaseRespectsUnflushedRedisQuotaAndRenewSocialSeatIsAtomic(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true

	// H-8: User has 100 quota, spends 80 through relay (only in Redis while batch
	// update is unflushed). Direct purchase needing 50 must fail against Redis.
	user := createReserveTestUser(t, 100)
	reserved, err := TryReserveUserQuota(user.Id, 80)
	require.NoError(t, err)
	require.True(t, reserved)
	assert.Equal(t, 100, getUserQuotaFromDB(t, user.Id), "DB still has stale balance")

	plan := &SubscriptionPlan{
		Title:         "Direct Purchase Guard",
		PriceAmount:   1,
		Enabled:       true,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
	}
	require.NoError(t, DB.Create(plan).Error)
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 50
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	err = PurchaseSubscriptionWithBalance(user.Id, plan.Id)
	require.Error(t, err, "direct balance purchase must not spend quota already reserved in Redis")
	assert.Equal(t, 100, getUserQuotaFromDB(t, user.Id))

	// H-8: Atomic social seat renewal charges once and skips concurrent/second pass.
	seatUser := createReserveTestUser(t, 100)
	now := time.Now().UTC()
	expiredAt := now.Add(-time.Hour)
	accID := int64(999)
	seat := &SocialSeat{
		UserId:            seatUser.Id,
		Status:            SocialSeatStatusOccupied,
		ExpiresAt:         expiredAt,
		OccupiedAccountID: &accID,
		OpenedQuota:       40,
		CreatedAt:         expiredAt.Add(-SocialSeatWindow()),
		UpdatedAt:         expiredAt,
	}
	require.NoError(t, DB.Create(seat).Error)

	var firstRenewed, secondRenewed *SocialSeat
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var rErr error
		firstRenewed, _, _, rErr = RenewSocialSeatTx(tx, seat.ID, seatUser.Id, 40, now)
		return rErr
	}))
	require.NotNil(t, firstRenewed)
	assert.Equal(t, 60, getUserQuotaFromDB(t, seatUser.Id))

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var rErr error
		secondRenewed, _, _, rErr = RenewSocialSeatTx(tx, seat.ID, seatUser.Id, 40, now)
		return rErr
	}))
	assert.Nil(t, secondRenewed, "second renewal pass must be a no-op")
	assert.Equal(t, 60, getUserQuotaFromDB(t, seatUser.Id))
}

type legacyTopUpRow struct {
	Id            int `gorm:"primaryKey"`
	UserId        int `gorm:"index"`
	Amount        int64
	Money         float64
	TradeNo       string `gorm:"uniqueIndex;type:varchar(255)"`
	PaymentMethod string `gorm:"type:varchar(50)"`
	CreateTime    int64
	CompleteTime  int64
	Status        string `gorm:"type:varchar(50)"`
}

func (legacyTopUpRow) TableName() string { return "top_ups" }

type legacySubscriptionOrderRow struct {
	Id            int64 `gorm:"primaryKey"`
	UserId        int   `gorm:"index"`
	PlanId        int64 `gorm:"index"`
	Money         float64
	TradeNo       string `gorm:"uniqueIndex;type:varchar(128)"`
	PaymentMethod string `gorm:"type:varchar(32)"`
	CreateTime    int64
	CompleteTime  int64
	Status        string `gorm:"type:varchar(32)"`
}

func (legacySubscriptionOrderRow) TableName() string { return "subscription_orders" }

func verifySecurityMigrationOnDB(t *testing.T, openDB func() (*gorm.DB, func()), dbType common.DatabaseType) {
	t.Helper()
	models := []any{&User{}, &TopUp{}, &SubscriptionOrder{}, &Task{}, &VoiceClone{}}

	t.Run("fresh_and_idempotent", func(t *testing.T) {
		db, cleanup := openDB()
		defer cleanup()
		oldDB := DB
		oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
		DB = db
		common.SetDatabaseTypes(dbType, oldLog)
		defer func() {
			DB = oldDB
			common.SetDatabaseTypes(oldMain, oldLog)
		}()

		require.NoError(t, db.AutoMigrate(models...))
		require.NoError(t, db.AutoMigrate(models...))
		require.NoError(t, BackfillVoiceClones())
		require.NoError(t, BackfillVoiceClones())

		created, err := ClaimOrVerifyVoiceCloneOwner(11, "omnimux_mig_spk_1", "task_1")
		require.NoError(t, err)
		assert.True(t, created)
		// Same-owner retry is idempotent and does not mark as newly claimed.
		createdAgain, err := ClaimOrVerifyVoiceCloneOwner(11, "omnimux_mig_spk_1", "task_1")
		require.NoError(t, err)
		assert.False(t, createdAgain)
		// Unique index rejects duplicate speaker_id row.
		require.Error(t, db.Create(&VoiceClone{SpeakerID: "omnimux_mig_spk_1", UserId: 22}).Error)
		// Foreign owner claim is rejected.
		_, err = ClaimOrVerifyVoiceCloneOwner(22, "omnimux_mig_spk_1", "task_2")
		require.ErrorIs(t, err, ErrVoiceCloneOwnedByOther)
		// lockForUpdate works inside a transaction on this engine.
		require.NoError(t, db.Create(&User{Id: 11, Username: "mig-lock-user", Password: "x", AffCode: "miglock1", Quota: 50, Status: 1}).Error)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			var u User
			return lockForUpdate(tx).Where("id = ?", 11).First(&u).Error
		}))
	})

	t.Run("upgrade_from_legacy_schema", func(t *testing.T) {
		db, cleanup := openDB()
		defer cleanup()
		oldDB := DB
		oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
		DB = db
		common.SetDatabaseTypes(dbType, oldLog)
		defer func() {
			DB = oldDB
			common.SetDatabaseTypes(oldMain, oldLog)
		}()

		// Seed pre-change schema and representative rows.
		require.NoError(t, db.AutoMigrate(&User{}, &legacyTopUpRow{}, &legacySubscriptionOrderRow{}, &Task{}))
		require.NoError(t, db.Create(&legacyTopUpRow{
			UserId: 7, Amount: 10, Money: 80, TradeNo: "legacy-topup-1", PaymentMethod: "stripe", Status: common.TopUpStatusPending,
		}).Error)
		require.NoError(t, db.Create(&legacySubscriptionOrderRow{
			UserId: 7, PlanId: 2, Money: 29, TradeNo: "legacy-sub-1", PaymentMethod: "stripe", Status: common.TopUpStatusPending,
		}).Error)
		// Seed an earlier failed attempt by another user before the succeeded clone
		// to ensure BackfillVoiceClones prioritizes SUCCESS and skips FAILURE rows.
		require.NoError(t, db.Create(&Task{
			TaskID: "task_vc_failed_earlier", UserId: 99, Action: constant.TaskActionVoiceClone, Status: TaskStatusFailure,
			PrivateData: TaskPrivateData{UpstreamTaskID: "omnimux_legacy_speaker"},
		}).Error)
		require.NoError(t, db.Create(&Task{
			TaskID: "task_vc_legacy", UserId: 7, Action: constant.TaskActionVoiceClone, Status: TaskStatusSuccess,
			PrivateData: TaskPrivateData{UpstreamTaskID: "omnimux_legacy_speaker"},
		}).Error)

		// Run new AutoMigrate + Backfill twice to verify upgrade + idempotency.
		require.NoError(t, db.AutoMigrate(models...))
		require.NoError(t, BackfillVoiceClones())
		require.NoError(t, db.AutoMigrate(models...))
		require.NoError(t, BackfillVoiceClones())

		// Verify legacy rows survived and new columns are writable.
		var tu TopUp
		require.NoError(t, db.Where("trade_no = ?", "legacy-topup-1").First(&tu).Error)
		assert.Equal(t, int64(10), tu.Amount)
		require.NoError(t, db.Model(&tu).Updates(map[string]any{
			"credited_quota": 5000000, "expected_amount_minor": 8000, "expected_currency": "usd",
		}).Error)

		var sub SubscriptionOrder
		require.NoError(t, db.Where("trade_no = ?", "legacy-sub-1").First(&sub).Error)
		assert.Equal(t, 2, sub.PlanId)
		require.NoError(t, db.Model(&sub).Updates(map[string]any{
			"expected_amount_minor": 2900, "expected_currency": "usd", "provider_session_id": "cs_legacy",
		}).Error)

		// Verify legacy VOICE_CLONE task was backfilled into voice_clones.
		ownerID, found, err := GetVoiceCloneOwner("omnimux_legacy_speaker")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, 7, ownerID)
	})
}

func TestSecuritySchemaMigrationFreshUpgradeAndIdempotency(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		verifySecurityMigrationOnDB(t, func() (*gorm.DB, func()) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			return db, func() { _ = sqlDB.Close() }
		}, common.DatabaseTypeSQLite)
	})

	if pgDSN := os.Getenv("SECURITY_VERIFY_PG_DSN"); pgDSN != "" {
		t.Run("postgres", func(t *testing.T) {
			verifySecurityMigrationOnDB(t, func() (*gorm.DB, func()) {
				adminDB, err := gorm.Open(postgres.Open(pgDSN), &gorm.Config{})
				require.NoError(t, err)
				schema := "sec_mig_" + common.GetRandomString(8)
				require.NoError(t, adminDB.Exec("CREATE SCHEMA "+schema).Error)
				dsnWithSchema := pgDSN + " search_path=" + schema
				db, err := gorm.Open(postgres.Open(dsnWithSchema), &gorm.Config{})
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				adminSQL, err := adminDB.DB()
				require.NoError(t, err)
				return db, func() {
					_ = sqlDB.Close()
					_ = adminDB.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
					_ = adminSQL.Close()
				}
			}, common.DatabaseTypePostgreSQL)
		})
	}

	if mysqlDSN := os.Getenv("SECURITY_VERIFY_MYSQL_DSN"); mysqlDSN != "" {
		t.Run("mysql", func(t *testing.T) {
			verifySecurityMigrationOnDB(t, func() (*gorm.DB, func()) {
				cfg, err := mysqlDriver.ParseDSN(mysqlDSN)
				require.NoError(t, err)
				adminCfg := *cfg
				adminCfg.DBName = ""
				adminDB, err := gorm.Open(gormmysql.Open(adminCfg.FormatDSN()), &gorm.Config{})
				require.NoError(t, err)
				dbName := "sec_mig_" + common.GetRandomString(8)
				require.NoError(t, adminDB.Exec("CREATE DATABASE `"+dbName+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error)
				testCfg := *cfg
				testCfg.DBName = dbName
				db, err := gorm.Open(gormmysql.Open(testCfg.FormatDSN()), &gorm.Config{})
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				adminSQL, err := adminDB.DB()
				require.NoError(t, err)
				return db, func() {
					_ = sqlDB.Close()
					_ = adminDB.Exec("DROP DATABASE IF EXISTS `" + dbName + "`").Error
					_ = adminSQL.Close()
				}
			}, common.DatabaseTypeMySQL)
		})
	}
}
