package hostedaccount

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedgateway"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.SubscriptionOrder{}, &model.Log{}); err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(model.HostedModels()...); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func clearHosted(t *testing.T) {
	t.Helper()
	for _, table := range []string{
		"hosted_tasks", "hosted_warmup_plans", "hosted_allocations",
		"hosted_orders", "hosted_entitlements", "hosted_accounts", "hosted_skus",
		"subscription_orders", "users",
	} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
}

func seedUserSKU(t *testing.T, quota int) (*model.User, *model.HostedSKU) {
	t.Helper()
	u := &model.User{Username: "u1", Password: "password1", Quota: quota, Status: 1, AffCode: common.GetRandomString(8)}
	require.NoError(t, model.DB.Create(u).Error)
	sku := &model.HostedSKU{Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS, PriceUSD: 99, Enabled: true}
	require.NoError(t, model.DB.Create(sku).Error)
	return u, sku
}

func TestFSMTransitions(t *testing.T) {
	assert.True(t, CanTransitionAccount(model.HostedAccountAssigned, model.HostedAccountWarmingUp))
	assert.True(t, CanTransitionAccount(model.HostedAccountReady, model.HostedAccountActive))
	assert.False(t, CanTransitionAccount(model.HostedAccountReady, model.HostedAccountAvailable))
	assert.False(t, CanTransitionAccount(model.HostedAccountActive, model.HostedAccountAssigned))
	assert.ErrorIs(t, ChannelUserFacingBlocked(model.HostedAccountWarmingUp), ErrChannelNotReady)
	assert.ErrorIs(t, ChannelUserFacingBlocked(model.HostedAccountSuspended), ErrChannelSuspended)
	assert.NoError(t, ChannelUserFacingBlocked(model.HostedAccountReady))
}

func TestCreateOrderDeliversAvailablePhone(t *testing.T) {
	clearHosted(t)
	u, sku := seedUserSKU(t, 100000000)
	phone := &model.HostedAccount{
		PhoneID:     "phone-1",
		Platform:    model.HostedPlatformTikTok,
		Region:      model.HostedRegionUS,
		Status:      model.HostedAccountAvailable,
		Allocatable: true,
	}
	require.NoError(t, model.DB.Create(phone).Error)

	svc := NewService(nil)
	res, err := svc.CreateOrder(context.Background(), u.Id, CreateOrderInput{SkuID: sku.ID, Quantity: 1, IdempotencyKey: "ord-1"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Entitlement.Quantity)
	assert.Greater(t, res.Entitlement.ExpireAt, int64(0))

	var acc model.HostedAccount
	require.NoError(t, model.DB.First(&acc, phone.ID).Error)
	assert.Equal(t, u.Id, acc.AssignedUserID)
	assert.Equal(t, model.HostedAccountWarmingUp, acc.Status)
	assert.Empty(t, acc.CredentialsEnc)

	view, err := svc.GetChannel(context.Background(), u.Id, acc.ID)
	require.NoError(t, err)
	assert.NotContains(t, view, "credentials")
	assert.NotContains(t, view, "credentials_enc")
}

func TestDistributeGateAndProfileEdit(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	warming := &model.HostedAccount{
		PhoneID: "p-warm", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountWarmingUp, AssignedUserID: u.Id,
	}
	ready := &model.HostedAccount{
		PhoneID: "p-ready", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(warming).Error)
	require.NoError(t, model.DB.Create(ready).Error)

	svc := NewService(nil)
	_, err := svc.CreateTask(context.Background(), u.Id, warming.ID, CreateTaskInput{Type: model.HostedTaskDistribute})
	assert.ErrorIs(t, err, ErrChannelNotReady)

	edit, err := svc.CreateTask(context.Background(), u.Id, warming.ID, CreateTaskInput{Type: model.HostedTaskProfileEdit, IdempotencyKey: "edit-1"})
	require.NoError(t, err)
	assert.Equal(t, model.HostedTaskPending, edit.Status)

	again, err := svc.CreateTask(context.Background(), u.Id, warming.ID, CreateTaskInput{Type: model.HostedTaskProfileEdit, IdempotencyKey: "edit-1"})
	require.NoError(t, err)
	assert.Equal(t, edit.ID, again.ID)

	dist, err := svc.CreateTask(context.Background(), u.Id, ready.ID, CreateTaskInput{Type: model.HostedTaskDistribute, IdempotencyKey: "d1"})
	require.NoError(t, err)
	assert.Equal(t, model.HostedTaskPending, dist.Status)
}

func TestMapRegion(t *testing.T) {
	p, r, ok := MapRegion("美区", "USA")
	assert.True(t, ok)
	assert.Equal(t, model.HostedPlatformTikTok, p)
	assert.Equal(t, model.HostedRegionUS, r)
	_, _, ok = MapRegion("东南亚", "TH")
	assert.False(t, ok)
}

func TestPresignWithoutGateway(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.PresignMedia(context.Background(), "a.mp4", "video/mp4")
	assert.ErrorIs(t, err, ErrGatewayUnavailable)
}

func TestCreateTaskStaysPendingWithoutDispatch(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	ready := &model.HostedAccount{
		PhoneID: "p-pending", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(ready).Error)
	svc := NewService(nil)
	dist, err := svc.CreateTask(context.Background(), u.Id, ready.ID, CreateTaskInput{
		Type: model.HostedTaskDistribute, IdempotencyKey: "stay-pending", MediaURL: "https://cdn.example/a.mp4",
	})
	require.NoError(t, err)
	assert.Equal(t, model.HostedTaskPending, dist.Status)
	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, dist.ID).Error)
	assert.Equal(t, model.HostedTaskPending, stored.Status)
	view, err := svc.GetTask(context.Background(), u.Id, dist.ID)
	require.NoError(t, err)
	assert.NotContains(t, view, "credentials")
	assert.NotContains(t, view, "password")
}

func TestPutCredentialsOmitsFromDTO(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_CRED_KEY", testCredKey())
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "p-cred", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountWarmingUp, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	svc := NewService(nil)
	row, err := svc.PutCredentials(context.Background(), acc.ID, PutCredentialsInput{
		Username: "user", Password: "pw", TOTPSecret: "totp", RecoveryEmail: "a@b.c",
	})
	require.NoError(t, err)
	assert.Equal(t, true, row["ok"])
	assert.Equal(t, acc.ID, row["account_id"])
	assert.NotContains(t, row, "password")
	assert.NotContains(t, row, "username")
	assert.NotContains(t, row, "totp_secret")
	assert.NotContains(t, row, "credentials")
	assert.NotContains(t, row, "credentials_enc")

	view, err := svc.GetChannel(context.Background(), u.Id, acc.ID)
	require.NoError(t, err)
	assert.NotContains(t, view, "credentials")
	assert.NotContains(t, view, "password")
	assert.NotContains(t, view, "totp_secret")

	admin, err := svc.AdminGetAccount(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.NotContains(t, admin, "credentials")
	assert.NotContains(t, admin, "password")

	var stored model.HostedAccount
	require.NoError(t, model.DB.First(&stored, acc.ID).Error)
	assert.NotEmpty(t, stored.CredentialsEnc)
	assert.NotContains(t, stored.CredentialsEnc, "pw")
}

func TestPublicDTOJSONOmitsCredentialSubstring(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_CRED_KEY", testCredKey())
	u, _ := seedUserSKU(t, 1000)
	enc, err := Seal(CredentialPlain{Username: "u", Password: "secret-pw", TOTPSecret: "totp"})
	require.NoError(t, err)
	acc := &model.HostedAccount{
		PhoneID: "p-json", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountWarmingUp, AssignedUserID: u.Id, CredentialsEnc: enc,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	svc := NewService(nil)
	channels, err := svc.ListChannels(context.Background(), u.Id, "")
	require.NoError(t, err)
	view, err := svc.GetChannel(context.Background(), u.Id, acc.ID)
	require.NoError(t, err)
	admin, err := svc.AdminGetAccount(context.Background(), acc.ID)
	require.NoError(t, err)
	put, err := svc.PutCredentials(context.Background(), acc.ID, PutCredentialsInput{Username: "u2", Password: "pw2"})
	require.NoError(t, err)
	blob, err := json.Marshal([]any{channels, view, admin, put})
	require.NoError(t, err)
	s := strings.ToLower(string(blob))
	for _, needle := range []string{"credentials_enc", `"password"`, "totp_secret", "secret-pw", "recovery_email"} {
		assert.NotContains(t, s, needle)
	}
}

func TestPutCredentialsReservedPhoneAllowed(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_CRED_KEY", testCredKey())
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: ReservedMonthlyPhoneID, Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReserved, Allocatable: false, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	svc := NewService(nil)
	row, err := svc.PutCredentials(context.Background(), acc.ID, PutCredentialsInput{Username: "ops", Password: "x"})
	require.NoError(t, err)
	assert.Equal(t, true, row["ok"])
	var stored model.HostedAccount
	require.NoError(t, model.DB.First(&stored, acc.ID).Error)
	assert.NotEmpty(t, stored.CredentialsEnc)
}

func TestPutCredentialsMissingKey(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_CRED_KEY", "")
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "p-nokey", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountAvailable, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	svc := NewService(nil)
	_, err := svc.PutCredentials(context.Background(), acc.ID, PutCredentialsInput{Username: "u", Password: "p"})
	assert.ErrorIs(t, err, ErrCredKeyMissing)
	var stored model.HostedAccount
	require.NoError(t, model.DB.First(&stored, acc.ID).Error)
	assert.Empty(t, stored.CredentialsEnc)
}

func TestAdminPoolOmitsCredentialsAndRefusesReservedPhone(t *testing.T) {
	clearHosted(t)
	u, sku := seedUserSKU(t, 1000)
	reserved := &model.HostedAccount{
		PhoneID: ReservedMonthlyPhoneID, Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReserved, Allocatable: false, DisplayName: "10 美区",
		Tags: "group=美区,country=USA", CredentialsEnc: "secret-must-not-leak",
	}
	stock := &model.HostedAccount{
		PhoneID: "phone-ok", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountAvailable, Allocatable: true, DisplayName: "21 美区",
	}
	require.NoError(t, model.DB.Create(reserved).Error)
	require.NoError(t, model.DB.Create(stock).Error)
	now := common.GetTimestamp()
	order := &model.HostedOrder{
		UserID: u.Id, SkuID: sku.ID, Kind: model.HostedOrderKindInitial,
		Quantity: 1, DeliveredCount: 0, Status: model.HostedOrderPaid,
		DeadlineAt: now + 3600, IdempotencyKey: "adm-ord-1", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(order).Error)

	svc := NewService(nil)
	pool, err := svc.AdminListPool(context.Background())
	require.NoError(t, err)
	require.Len(t, pool, 2)
	for _, row := range pool {
		assert.NotContains(t, row, "credentials")
		assert.NotContains(t, row, "credentials_enc")
		assert.NotContains(t, row, "password")
	}

	_, err = svc.SetAllocatable(context.Background(), reserved.ID, true)
	assert.ErrorIs(t, err, ErrReservedPhone)
	var still model.HostedAccount
	require.NoError(t, model.DB.First(&still, reserved.ID).Error)
	assert.False(t, still.Allocatable)

	updated, err := svc.SetAllocatable(context.Background(), stock.ID, false)
	require.NoError(t, err)
	assert.Equal(t, false, updated["allocatable"])

	orders, err := svc.AdminListOrders(context.Background())
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, u.Username, orders[0]["user"])
	assert.Equal(t, 0, orders[0]["delivered"])
	assert.NotContains(t, orders[0], "credentials")

	skus, err := svc.AdminListSKUs(context.Background())
	require.NoError(t, err)
	require.Len(t, skus, 1)
	assert.Equal(t, int64(1), skus[0]["open_orders"])
	assert.Equal(t, int64(0), skus[0]["stock"])
}

func TestAdminSKUToggleAndAccountDetailOmitsCredentials(t *testing.T) {
	clearHosted(t)
	u, sku := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "phone-detail", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountAssigned, Allocatable: false, DisplayName: "22 美区",
		AssignedUserID: u.Id, CredentialsEnc: "secret-must-not-leak",
		BrandConfig: `{"nick_name":"north"}`,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskSucceeded, IdempotencyKey: "adm-task-1",
		Metrics: `{"views":4,"likes":1,"comments":0}`, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)

	svc := NewService(nil)
	disabled, err := svc.SetSKUEnabled(context.Background(), sku.ID, false)
	require.NoError(t, err)
	assert.Equal(t, false, disabled["enabled"])
	var stored model.HostedSKU
	require.NoError(t, model.DB.First(&stored, sku.ID).Error)
	assert.False(t, stored.Enabled)

	enabled, err := svc.SetSKUEnabled(context.Background(), sku.ID, true)
	require.NoError(t, err)
	assert.Equal(t, true, enabled["enabled"])

	detail, err := svc.AdminGetAccount(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Username, detail["buyer"])
	assert.Equal(t, "north", detail["brand"])
	assert.NotContains(t, detail, "credentials")
	assert.NotContains(t, detail, "credentials_enc")
	assert.NotContains(t, detail, "password")
	tasks, ok := detail["tasks"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, tasks, 1)
	assert.NotContains(t, tasks[0], "credentials")

	_, err = svc.AdminSync(context.Background())
	assert.ErrorIs(t, err, ErrGatewayUnavailable)
}

func TestAdminCancelAndRetryTask(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "phone-task", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	pending := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskPending, IdempotencyKey: "adm-cancel-1",
		CreatedAt: now, UpdatedAt: now,
	}
	failed := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskFailed, IdempotencyKey: "adm-retry-1",
		Payload: `{"caption":"retry-me"}`, ErrorCode: "gateway", ErrorMsg: "boom",
		CreatedAt: now, UpdatedAt: now,
	}
	succeeded := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskSucceeded, IdempotencyKey: "adm-ok-1",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(pending).Error)
	require.NoError(t, model.DB.Create(failed).Error)
	require.NoError(t, model.DB.Create(succeeded).Error)

	svc := NewService(nil)
	cancelled, err := svc.AdminCancelTask(context.Background(), pending.ID)
	require.NoError(t, err)
	assert.Equal(t, model.HostedTaskCancelled, cancelled["status"])
	assert.NotContains(t, cancelled, "credentials")
	assert.NotContains(t, cancelled, "payload")

	_, err = svc.AdminCancelTask(context.Background(), succeeded.ID)
	assert.ErrorIs(t, err, ErrInvalidRequest)
	_, err = svc.AdminCancelTask(context.Background(), failed.ID)
	assert.ErrorIs(t, err, ErrInvalidRequest)

	_, err = svc.AdminRetryTask(context.Background(), failed.ID)
	assert.ErrorIs(t, err, ErrGatewayUnavailable)

	ready := NewService(&hostedgateway.Client{BaseURL: "http://127.0.0.1:9", APIKey: "k"})
	_, err = ready.AdminRetryTask(context.Background(), pending.ID)
	assert.ErrorIs(t, err, ErrInvalidRequest)
	_, err = ready.AdminRetryTask(context.Background(), succeeded.ID)
	assert.ErrorIs(t, err, ErrInvalidRequest)

	retry, err := ready.AdminRetryTask(context.Background(), failed.ID)
	require.NoError(t, err)
	assert.Equal(t, model.HostedTaskPending, retry["status"])
	assert.Equal(t, acc.ID, retry["channel_id"])
	assert.NotContains(t, retry, "credentials")
	assert.NotContains(t, retry, "credentials_enc")
	assert.NotContains(t, retry, "password")
	assert.NotContains(t, retry, "payload")
	assert.NotEqual(t, failed.ID, retry["id"])

	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, failed.ID).Error)
	assert.Equal(t, model.HostedTaskFailed, stored.Status)
}
