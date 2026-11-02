package social

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{},
		&model.Log{},
		&model.SocialTenant{},
		&model.SocialAccount{},
		&model.SocialPost{},
		&model.SocialSeat{},
		// fork: tiktok-direct
		&model.SocialOAuthState{},
	); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func clearSocialTables(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Exec("DELETE FROM social_posts").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM social_seats").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM social_accounts").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM social_tenants").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM users").Error)
}

func TestAccountGuard_RejectsForeignAccount(t *testing.T) {
	clearSocialTables(t)
	a := &model.SocialAccount{
		UserId:            1,
		Provider:          ProviderZernio,
		ProviderAccountID: "acc_a",
		Platform:          "twitter",
		Status:            model.SocialAccountStatusActive,
	}
	b := &model.SocialAccount{
		UserId:            2,
		Provider:          ProviderZernio,
		ProviderAccountID: "acc_b",
		Platform:          "twitter",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, model.DB.Create(a).Error)
	require.NoError(t, model.DB.Create(b).Error)

	_, err := AccountGuard(1, []int64{b.ID})
	assert.ErrorIs(t, err, ErrAccountDenied)

	owned, err := AccountGuard(1, []int64{a.ID})
	require.NoError(t, err)
	require.Len(t, owned, 1)
	assert.Equal(t, a.ID, owned[0].ID)
}

func TestAccountGuard_RejectsDisconnected(t *testing.T) {
	clearSocialTables(t)
	a := &model.SocialAccount{
		UserId:            1,
		Provider:          ProviderZernio,
		ProviderAccountID: "acc_x",
		Platform:          "linkedin",
		Status:            model.SocialAccountStatusDisconnected,
	}
	require.NoError(t, model.DB.Create(a).Error)
	_, err := AccountGuard(1, []int64{a.ID})
	assert.ErrorIs(t, err, ErrAccountDenied)
}

func TestAccountGuard_Empty(t *testing.T) {
	_, err := AccountGuard(1, nil)
	assert.ErrorIs(t, err, ErrAccountDenied)
}

func TestNormalizePlatform(t *testing.T) {
	assert.Equal(t, "twitter", NormalizePlatform("X"))
	assert.Equal(t, "linkedin", NormalizePlatform("LinkedIn"))
}

func TestConfigDefaultOff(t *testing.T) {
	t.Setenv("SOCIAL_OPS_ENABLED", "")
	t.Setenv("SOCIAL_ZERNIO_API_KEY", "")
	cfg := LoadConfig()
	assert.False(t, cfg.Enabled)
	assert.False(t, cfg.Ready())
}

func TestValidateRedirectURL(t *testing.T) {
	assert.NoError(t, validateRedirectURL("https://omnimux.ai/cb", "https://omnimux.ai/social/oauth/callback"))
	assert.ErrorIs(t, validateRedirectURL("https://evil.example/cb", "https://omnimux.ai/cb"), ErrRedirectDenied)
	assert.NoError(t, validateRedirectURL("http://localhost:3000/cb", ""))
	assert.ErrorIs(t, validateRedirectURL("http://evil.example/cb", ""), ErrRedirectDenied)
}

func TestServiceRequireReady(t *testing.T) {
	s := NewService(Config{Enabled: false})
	assert.ErrorIs(t, s.requireReady(), ErrDisabled)
	s = NewService(Config{Enabled: true, APIKey: "", BaseURL: defaultBaseURL})
	assert.ErrorIs(t, s.requireReady(), ErrNotConfigured)
}
