package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Allowance-generation isolation for late subscription refunds (finding 08).
//
// A subscription reservation belongs to the allowance window that was open when
// it was charged. Once a reset zeroes amount_used, a late refund of that older
// reservation may no longer give allowance back to the window that is open now,
// or it would restore usage the user already burned after the reset.
// ---------------------------------------------------------------------------

// seedRefundPeriodSubscription seeds an active subscription whose allowance window
// can actually roll over, unlike the never-reset fixture used by the debt tests.
func seedRefundPeriodSubscription(t *testing.T, wallet int, total, used int64) (User, UserSubscription) {
	t.Helper()
	resetSubscriptionPeriodTestTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, wallet)
	plan := SubscriptionPlan{
		Title:                   t.Name(),
		QuotaResetPeriod:        SubscriptionResetCustom,
		QuotaResetCustomSeconds: 3600,
		DurationUnit:            SubscriptionDurationMonth,
		DurationValue:           1,
		TotalAmount:             total,
	}
	require.NoError(t, DB.Create(&plan).Error)
	// Recreating the table restarts plan ids, so a cached plan for the same id would
	// otherwise be reused and the window would never roll over.
	InvalidateSubscriptionPlanCache(plan.Id)
	now := GetDBTimestamp()
	sub := UserSubscription{
		UserId: user.Id, PlanId: plan.Id, AmountTotal: total, AmountUsed: used,
		Status: "active", StartTime: now - 3*3600, EndTime: now + 30*24*3600,
		LastResetTime: now - 3*3600, NextResetTime: now + 3600, AllowanceEpoch: 1,
	}
	require.NoError(t, DB.Create(&sub).Error)
	return user, sub
}

func resetSubscriptionPeriodTestTables(t *testing.T) {
	t.Helper()
	// Recreate rather than truncate: schema introspection of an existing table is
	// unavailable on some external engines on this host, and a fresh table needs none.
	require.NoError(t, DB.Migrator().DropTable(&SubscriptionPreConsumeRecord{}, &Task{}, &UserSubscription{}, &SubscriptionPlan{}, &User{}))
	require.NoError(t, DB.AutoMigrate(&User{}, &SubscriptionPlan{}, &UserSubscription{}, &Task{}, &SubscriptionPreConsumeRecord{}))
}

// closeAllowanceWindow makes the subscription's window due and lets the native
// reset path roll the allowance generation over.
func closeAllowanceWindow(t *testing.T, subID int) int64 {
	t.Helper()
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subID).
		Update("next_reset_time", GetDBTimestamp()-1).Error)
	count, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count, "the seeded subscription must be the only one rolled over")
	after := getSubscriptionResetSub(t, subID)
	require.Zero(t, after.AmountUsed)
	return after.AllowanceEpoch
}

func preConsumeRecordStatus(t *testing.T, requestId string) string {
	t.Helper()
	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&record).Error)
	return record.Status
}

// useExternalTestDatabase points the global test database at a real MySQL or
// PostgreSQL instance through the production dialector and returns the restore
// function. The scenarios always recreate their own schema first: schema
// introspection of an already existing table fails on this host's pgx
// simple-protocol path (the same pre-existing failure reproduces in the untouched
// TestMigrationSchemaStability), and a fresh table needs no introspection.
func useExternalTestDatabase(t *testing.T, envName string) func() {
	t.Helper()
	previousDB, previousLogDB := DB, LOG_DB
	var (
		db     *gorm.DB
		dbType common.DatabaseType
		err    error
	)
	db, dbType, err = chooseDB(envName, false)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(dbType, dbType)
	return func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
		_ = sqlDB.Close()
	}
}

// TestSubscriptionRefundPeriodInvariantAcrossEngines runs every acceptance scenario
// on the in-memory SQLite harness and, when the DSNs are configured, on MySQL and
// PostgreSQL as well. Each engine reports its own subtest so the evidence states
// exactly which database an assertion ran on.
func TestSubscriptionRefundPeriodInvariantAcrossEngines(t *testing.T) {
	scenarios := []struct {
		name string
		run  func(*testing.T)
	}{
		{"request_refund", oldPeriodRequestRefundScenario},
		{"task_settlement", oldPeriodTaskSettlementScenario},
		{"crossing_overrun", crossingOverrunScenario},
		{"supplemental_rollback", supplementalRollbackScenario},
		{"wallet_and_debt", walletAndDebtScenario},
	}
	engines := []struct {
		name    string
		envName string
	}{
		{"sqlite", ""},
		{"mysql", "TEST_MYSQL_DSN"},
		{"postgres", "TEST_POSTGRES_DSN"},
	}
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			if engine.envName != "" {
				if os.Getenv(engine.envName) == "" {
					t.Skipf("%s is not configured", engine.envName)
				}
				restore := useExternalTestDatabase(t, engine.envName)
				defer restore()
				// The engine must be able to create the schema the scenarios seed. A
				// driver that cannot is a harness limitation of this host, not a result
				// of the change under test.
				const probe = "allowance_engine_probe"
				require.NoError(t, DB.Exec("DROP TABLE IF EXISTS "+probe).Error)
				t.Cleanup(func() { _ = DB.Exec("DROP TABLE IF EXISTS " + probe).Error })
				if err := DB.Table(probe).AutoMigrate(&migrationAllowanceProbe{}); err != nil {
					t.Skipf("%s harness cannot create a schema on this host: %v", engine.name, err)
				}
			}
			for _, scenario := range scenarios {
				t.Run(scenario.name, scenario.run)
			}
		})
	}
}

// oldPeriodRequestRefundScenario is the reported finding: reserve in one window,
// let the window roll over, then refund the old reservation.
func oldPeriodRequestRefundScenario(t *testing.T) {
	user, sub := seedRefundPeriodSubscription(t, 0, 1000, 0)

	old, err := PreConsumeUserSubscription("old-period-request", user.Id, "", 0, 100)
	require.NoError(t, err)
	oldEpoch := getSubscriptionResetSub(t, sub.Id).AllowanceEpoch
	require.EqualValues(t, oldEpoch, old.AllowanceEpoch)
	require.EqualValues(t, 100, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	newEpoch := closeAllowanceWindow(t, sub.Id)
	require.Greater(t, newEpoch, oldEpoch)
	current, err := PreConsumeUserSubscription("current-period-request", user.Id, "", 0, 80)
	require.NoError(t, err)
	require.EqualValues(t, newEpoch, current.AllowanceEpoch)
	require.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// Retrying the old request is idempotent and keeps its original generation.
	retried, err := PreConsumeUserSubscription("old-period-request", user.Id, "", 0, 100)
	require.NoError(t, err)
	assert.EqualValues(t, oldEpoch, retried.AllowanceEpoch, "a retried reservation keeps the window it reserved in")
	assert.EqualValues(t, 100, retried.PreConsumed)
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// The late failure refund of the old request must not restore the new window.
	require.NoError(t, RefundSubscriptionPreConsume("old-period-request"))
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed,
		"an old-window refund must not give back allowance burned in the new window")
	assert.Equal(t, "refunded", preConsumeRecordStatus(t, "old-period-request"), "the reservation is retired, not left replayable")
	require.NoError(t, RefundSubscriptionPreConsume("old-period-request"))
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// A late negative settlement of the same old request is retired as well.
	late, err := SettleUserSubscriptionDelta(user.Id, sub.Id, -100, 0,
		SubscriptionAllowanceRef{Epoch: old.AllowanceEpoch, Allocated: old.PreConsumed})
	require.NoError(t, err)
	assert.Zero(t, late.SubscriptionDelta)
	assert.EqualValues(t, 80, late.AmountUsed)
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// Same-window refunds still restore that window's allowance.
	require.NoError(t, RefundSubscriptionPreConsume("current-period-request"))
	assert.Zero(t, getSubscriptionResetSub(t, sub.Id).AmountUsed)
}

// oldPeriodTaskSettlementScenario covers asynchronous work whose binding is
// persisted in task private data and settles after the window rolled over.
func oldPeriodTaskSettlementScenario(t *testing.T) {
	user, sub := seedRefundPeriodSubscription(t, 0, 1000, 0)

	reservation, err := PreConsumeUserSubscription("old-period-task-request", user.Id, "", 0, 100)
	require.NoError(t, err)
	task := Task{TaskID: "old-period-task", UserId: user.Id, Quota: 100, PrivateData: TaskPrivateData{
		BillingSource: "subscription", SubscriptionId: sub.Id,
		SubscriptionAllowanceEpoch:     reservation.AllowanceEpoch,
		SubscriptionAllowanceAllocated: reservation.PreConsumed,
	}}
	require.NoError(t, task.Insert())

	newEpoch := closeAllowanceWindow(t, sub.Id)
	current, err := PreConsumeUserSubscription("current-period-request", user.Id, "", 0, 80)
	require.NoError(t, err)
	require.EqualValues(t, newEpoch, current.AllowanceEpoch)
	require.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	refunded, err := SettleTaskSubscriptionQuota(&task, 0)
	require.NoError(t, err)
	assert.Equal(t, -100, refunded.QuotaDelta, "request-level accounting still reconciles the whole task")
	assert.Zero(t, refunded.SubscriptionDelta, "the retired subscription share never reaches the open window")
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	var persisted Task
	require.NoError(t, DB.First(&persisted, task.ID).Error)
	assert.Zero(t, persisted.Quota)
	assert.Zero(t, persisted.PrivateData.SubscriptionWalletConsumed)

	replayed, err := SettleTaskSubscriptionQuota(&persisted, 0)
	require.NoError(t, err)
	assert.Zero(t, replayed.QuotaDelta)
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// The same-window task still refunds its own window normally.
	require.NoError(t, RefundSubscriptionPreConsume("current-period-request"))
	assert.Zero(t, getSubscriptionResetSub(t, sub.Id).AmountUsed)
}

// crossingOverrunScenario pins the unchanged positive late-settlement policy and
// the claim re-tagging that keeps a later refund bounded by what the task really
// charged into the window that is open now.
func crossingOverrunScenario(t *testing.T) {
	user, sub := seedRefundPeriodSubscription(t, 0, 1000, 0)

	reservation, err := PreConsumeUserSubscription("crossing-request", user.Id, "", 0, 100)
	require.NoError(t, err)
	task := Task{TaskID: "crossing-task", UserId: user.Id, Quota: 100, PrivateData: TaskPrivateData{
		BillingSource: "subscription", SubscriptionId: sub.Id,
		SubscriptionAllowanceEpoch:     reservation.AllowanceEpoch,
		SubscriptionAllowanceAllocated: reservation.PreConsumed,
	}}
	require.NoError(t, task.Insert())

	newEpoch := closeAllowanceWindow(t, sub.Id)
	settled, err := SettleTaskSubscriptionQuota(&task, 150)
	require.NoError(t, err)
	assert.EqualValues(t, 50, settled.SubscriptionDelta, "the positive policy still funds the overrun from the open window")
	assert.Zero(t, settled.WalletDelta)
	assert.EqualValues(t, 50, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	var persisted Task
	require.NoError(t, DB.First(&persisted, task.ID).Error)
	assert.EqualValues(t, newEpoch, persisted.PrivateData.SubscriptionAllowanceEpoch)
	assert.EqualValues(t, 50, persisted.PrivateData.SubscriptionAllowanceAllocated)

	_, err = PreConsumeUserSubscription("unrelated-request", user.Id, "", 0, 70)
	require.NoError(t, err)
	require.EqualValues(t, 120, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	refunded, err := SettleTaskSubscriptionQuota(&persisted, 0)
	require.NoError(t, err)
	assert.Equal(t, -150, refunded.QuotaDelta)
	assert.EqualValues(t, -50, refunded.SubscriptionDelta, "only what this task charged into the open window comes back")
	assert.EqualValues(t, 70, getSubscriptionResetSub(t, sub.Id).AmountUsed, "unrelated new-window usage stays burned")
}

// supplementalRollbackScenario covers the extra pre-consume reservation and its
// rollback, both inside one window and interrupted by a rollover.
func supplementalRollbackScenario(t *testing.T) {
	user, sub := seedRefundPeriodSubscription(t, 0, 1000, 0)

	initial, err := PreConsumeUserSubscription("supplemental-request", user.Id, "", 0, 100)
	require.NoError(t, err)
	claim := SubscriptionAllowanceRef{Epoch: initial.AllowanceEpoch, Allocated: initial.PreConsumed}

	claim, err = PostConsumeUserSubscriptionDelta(sub.Id, 50, claim)
	require.NoError(t, err)
	require.EqualValues(t, 150, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	claim, err = PostConsumeUserSubscriptionDelta(sub.Id, -50, claim)
	require.NoError(t, err)
	assert.EqualValues(t, 100, getSubscriptionResetSub(t, sub.Id).AmountUsed, "a same-window rollback restores the reservation")
	assert.EqualValues(t, 100, claim.Allocated)

	claim, err = PostConsumeUserSubscriptionDelta(sub.Id, 50, claim)
	require.NoError(t, err)
	require.EqualValues(t, 150, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	closeAllowanceWindow(t, sub.Id)
	_, err = PreConsumeUserSubscription("new-window-request", user.Id, "", 0, 80)
	require.NoError(t, err)
	require.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	claim, err = PostConsumeUserSubscriptionDelta(sub.Id, -50, claim)
	require.NoError(t, err)
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed,
		"a rollback from the previous window must not touch the open window")
	assert.EqualValues(t, initial.AllowanceEpoch, claim.Epoch,
		"the claim stays bound to its closed window and can never be applied again")
	assert.Less(t, claim.Epoch, getSubscriptionResetSub(t, sub.Id).AllowanceEpoch)
}

// walletAndDebtScenario proves the wallet side is untouched: this request's real
// wallet charge still comes back, and a stale-generation refund never restores the
// open window even while the wallet is in debt.
func walletAndDebtScenario(t *testing.T) {
	user, sub := seedRefundPeriodSubscription(t, 0, 1000, 0)

	old, err := PreConsumeUserSubscription("wallet-request", user.Id, "", 0, 100)
	require.NoError(t, err)

	closeAllowanceWindow(t, sub.Id)
	current, err := PreConsumeUserSubscription("current-period-request", user.Id, "", 0, 80)
	require.NoError(t, err)
	require.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	late, err := SettleUserSubscriptionDelta(user.Id, sub.Id, -60, 60,
		SubscriptionAllowanceRef{Epoch: old.AllowanceEpoch, Allocated: old.PreConsumed})
	require.NoError(t, err)
	assert.EqualValues(t, -60, late.WalletDelta, "this request's own wallet charge still comes back")
	assert.Zero(t, late.SubscriptionDelta)
	assert.Equal(t, 60, getUserQuotaFromDB(t, user.Id))
	assert.EqualValues(t, 80, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// Same window: usage beyond the remaining allowance becomes native wallet debt.
	overrun, err := SettleUserSubscriptionDelta(user.Id, sub.Id, 1000, 0,
		SubscriptionAllowanceRef{Epoch: current.AllowanceEpoch, Allocated: current.PreConsumed})
	require.NoError(t, err)
	assert.EqualValues(t, 920, overrun.SubscriptionDelta)
	assert.EqualValues(t, 80, overrun.WalletDelta)
	assert.Equal(t, -20, getUserQuotaFromDB(t, user.Id))
	require.ErrorIs(t, CheckUserWalletDebt(user.Id), ErrUserWalletDebt)
	// Debt still fails the strict supplemental reservation closed.
	require.ErrorIs(t, postDelta(sub.Id, 1, SubscriptionAllowanceRef{}), ErrUserWalletDebt)

	// Debt does not change the wallet rule, and still cannot credit the open window.
	stale, err := SettleUserSubscriptionDelta(user.Id, sub.Id, -60, 60,
		SubscriptionAllowanceRef{Epoch: old.AllowanceEpoch, Allocated: old.PreConsumed})
	require.NoError(t, err)
	assert.EqualValues(t, -60, stale.WalletDelta)
	assert.Zero(t, stale.SubscriptionDelta)
	assert.Equal(t, 40, getUserQuotaFromDB(t, user.Id))
	assert.EqualValues(t, 1000, getSubscriptionResetSub(t, sub.Id).AmountUsed)
	require.NoError(t, CheckUserWalletDebt(user.Id), "the wallet refund settles the debt it was charged for")
}

// TestSubscriptionRefundPeriodLegacyClaimsStayClosed pins the declared legacy
// behaviour: a reservation or task written before generation tracking carries
// generation 0, which can never be proven current, while a legacy subscription
// itself is backfilled to generation 1 and keeps refunding normally.
func TestSubscriptionRefundPeriodLegacyClaimsStayClosed(t *testing.T) {
	user, sub := seedRefundPeriodSubscription(t, 0, 1000, 0)
	now := GetDBTimestamp()

	// A reservation record written before the column existed.
	legacy := SubscriptionPreConsumeRecord{RequestId: "legacy-request", UserId: user.Id,
		UserSubscriptionId: sub.Id, PreConsumed: 100, Status: "consumed", AllowanceEpoch: 0}
	require.NoError(t, DB.Create(&legacy).Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).
		Updates(map[string]any{"amount_used": 300}).Error)

	require.NoError(t, RefundSubscriptionPreConsume("legacy-request"))
	assert.EqualValues(t, 300, getSubscriptionResetSub(t, sub.Id).AmountUsed,
		"an unproven generation never credits the open window")
	assert.Equal(t, "refunded", preConsumeRecordStatus(t, "legacy-request"))

	// A task written before the column existed.
	legacyTask := Task{TaskID: "legacy-task", UserId: user.Id, Quota: 100, PrivateData: TaskPrivateData{
		BillingSource: "subscription", SubscriptionId: sub.Id,
	}}
	require.NoError(t, legacyTask.Insert())
	refunded, err := SettleTaskSubscriptionQuota(&legacyTask, 0)
	require.NoError(t, err)
	assert.Equal(t, -100, refunded.QuotaDelta)
	assert.Zero(t, refunded.SubscriptionDelta)
	assert.EqualValues(t, 300, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// A subscription that never reset since the deploy still refunds its own window.
	fresh, err := PreConsumeUserSubscription("fresh-request", user.Id, "", 0, 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, fresh.AllowanceEpoch, "the column default backfills legacy subscriptions to generation 1")
	require.NoError(t, RefundSubscriptionPreConsume("fresh-request"))
	assert.EqualValues(t, 300, getSubscriptionResetSub(t, sub.Id).AmountUsed)

	// Manual admin reset advances the generation as well.
	_, err = AdminResetUserSubscriptionsByPlan(user.Id, sub.PlanId, false)
	require.NoError(t, err)
	assert.EqualValues(t, 2, getSubscriptionResetSub(t, sub.Id).AllowanceEpoch)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).
		Updates(map[string]any{"amount_used": 500, "updated_at": now}).Error)
	stale, err := SettleUserSubscriptionDelta(user.Id, sub.Id, -100, 0,
		SubscriptionAllowanceRef{Epoch: fresh.AllowanceEpoch, Allocated: fresh.PreConsumed})
	require.NoError(t, err)
	assert.Zero(t, stale.SubscriptionDelta)
	assert.EqualValues(t, 500, getSubscriptionResetSub(t, sub.Id).AmountUsed)
}

// migrationAllowanceProbe is a throwaway table used only to prove the engine can
// introspect a schema before the allowance scenarios seed their own.
type migrationAllowanceProbe struct {
	ID int `gorm:"primaryKey"`
}

// TestSubscriptionAllowanceEpochColumnBackfillAcrossEngines executes the additive
// migration the fix relies on as raw DDL on every engine and asserts the existing
// rows are backfilled to generation 1, which is what keeps a legacy subscription
// refundable. Raw DDL is used because the GORM schema introspection of this harness
// is unavailable on some external engines on this host.
func TestSubscriptionAllowanceEpochColumnBackfillAcrossEngines(t *testing.T) {
	engines := []struct {
		name    string
		envName string
	}{
		{"sqlite", ""},
		{"mysql", "TEST_MYSQL_DSN"},
		{"postgres", "TEST_POSTGRES_DSN"},
	}
	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			if engine.envName != "" {
				if os.Getenv(engine.envName) == "" {
					t.Skipf("%s is not configured", engine.envName)
				}
				restore := useExternalTestDatabase(t, engine.envName)
				defer restore()
			}
			const table = "allowance_epoch_backfill_probe"
			require.NoError(t, DB.Exec("DROP TABLE IF EXISTS "+table).Error)
			t.Cleanup(func() { _ = DB.Exec("DROP TABLE IF EXISTS " + table).Error })
			require.NoError(t, DB.Exec("CREATE TABLE "+table+" (id bigint PRIMARY KEY, amount_used bigint NOT NULL DEFAULT 0)").Error)
			require.NoError(t, DB.Exec("INSERT INTO "+table+" (id, amount_used) VALUES (1, 500)").Error)
			require.NoError(t, DB.Exec("ALTER TABLE "+table+" ADD COLUMN allowance_epoch bigint NOT NULL DEFAULT 1").Error)

			var epoch int64
			require.NoError(t, DB.Raw("SELECT allowance_epoch FROM "+table+" WHERE id = 1").Scan(&epoch).Error)
			assert.EqualValues(t, 1, epoch, "an existing subscription is backfilled to generation 1")

			require.NoError(t, DB.Exec("INSERT INTO "+table+" (id, amount_used) VALUES (2, 0)").Error)
			require.NoError(t, DB.Raw("SELECT allowance_epoch FROM "+table+" WHERE id = 2").Scan(&epoch).Error)
			assert.EqualValues(t, 1, epoch, "a row inserted without the column takes the generation-1 default")
		})
	}
}
