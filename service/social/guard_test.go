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
	assert.False(t, cfg.TikTokSandboxEnabled)
	assert.False(t, cfg.Ready())
}

func TestConfigTikTokSandboxCredentialsPrecedence(t *testing.T) {
	cases := []struct {
		name           string
		cfg            Config
		wantKey        string
		wantSecret     string
		wantConfigured bool
	}{
		{
			name: "sandbox disabled uses prod and ignores leftover sandbox vars",
			cfg: Config{
				TikTokClientKey:           "prod-key",
				TikTokClientSecret:        "prod-secret",
				TikTokSandboxEnabled:      false,
				TikTokSandboxClientKey:    "sbx-key",
				TikTokSandboxClientSecret: "sbx-secret",
				TikTokRedirectURI:         defaultTikTokRedirectURI,
			},
			wantKey:        "prod-key",
			wantSecret:     "prod-secret",
			wantConfigured: true,
		},
		{
			name: "sandbox disabled with only sandbox vars stays unconfigured",
			cfg: Config{
				TikTokSandboxEnabled:      false,
				TikTokSandboxClientKey:    "sbx-key",
				TikTokSandboxClientSecret: "sbx-secret",
				TikTokRedirectURI:         defaultTikTokRedirectURI,
			},
			wantKey:        "",
			wantSecret:     "",
			wantConfigured: false,
		},
		{
			name: "sandbox enabled selects sandbox pair over prod",
			cfg: Config{
				TikTokClientKey:           "prod-key",
				TikTokClientSecret:        "prod-secret",
				TikTokSandboxEnabled:      true,
				TikTokSandboxClientKey:    "sbx-key",
				TikTokSandboxClientSecret: "sbx-secret",
				TikTokRedirectURI:         defaultTikTokRedirectURI,
			},
			wantKey:        "sbx-key",
			wantSecret:     "sbx-secret",
			wantConfigured: true,
		},
		{
			name: "sandbox enabled fails closed when sandbox key missing",
			cfg: Config{
				TikTokClientKey:           "prod-key",
				TikTokClientSecret:        "prod-secret",
				TikTokSandboxEnabled:      true,
				TikTokSandboxClientKey:    "",
				TikTokSandboxClientSecret: "sbx-secret",
				TikTokRedirectURI:         defaultTikTokRedirectURI,
			},
			wantKey:        "",
			wantSecret:     "",
			wantConfigured: false,
		},
		{
			name: "sandbox enabled fails closed when sandbox secret missing",
			cfg: Config{
				TikTokClientKey:           "prod-key",
				TikTokClientSecret:        "prod-secret",
				TikTokSandboxEnabled:      true,
				TikTokSandboxClientKey:    "sbx-key",
				TikTokSandboxClientSecret: "",
				TikTokRedirectURI:         defaultTikTokRedirectURI,
			},
			wantKey:        "",
			wantSecret:     "",
			wantConfigured: false,
		},
		{
			name: "sandbox enabled fails closed when both sandbox vars missing",
			cfg: Config{
				TikTokClientKey:      "prod-key",
				TikTokClientSecret:   "prod-secret",
				TikTokSandboxEnabled: true,
				TikTokRedirectURI:    defaultTikTokRedirectURI,
			},
			wantKey:        "",
			wantSecret:     "",
			wantConfigured: false,
		},
		{
			name: "sandbox enabled fails closed on whitespace-only sandbox secret",
			cfg: Config{
				TikTokClientKey:           "prod-key",
				TikTokClientSecret:        "prod-secret",
				TikTokSandboxEnabled:      true,
				TikTokSandboxClientKey:    "sbx-key",
				TikTokSandboxClientSecret: "   ",
				TikTokRedirectURI:         defaultTikTokRedirectURI,
			},
			wantKey:        "",
			wantSecret:     "",
			wantConfigured: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotSecret := tc.cfg.TikTokCredentials()
			assert.Equal(t, tc.wantKey, gotKey)
			assert.Equal(t, tc.wantSecret, gotSecret)
			assert.Equal(t, tc.wantConfigured, tc.cfg.TikTokDirectConfigured())
		})
	}
}

func TestLoadConfigTikTokSandboxEnabledEnv(t *testing.T) {
	t.Run("unset defaults to false and prefers prod over leftover sandbox", func(t *testing.T) {
		t.Setenv("SOCIAL_OPS_ENABLED", "true")
		t.Setenv("TIKTOK_DIRECT_ENABLED", "true")
		t.Setenv("TIKTOK_SANDBOX_ENABLED", "")
		t.Setenv("TIKTOK_CLIENT_KEY", "prod-k")
		t.Setenv("TIKTOK_CLIENT_SECRET", "prod-s")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_KEY", "sbx-k")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_SECRET", "sbx-s")

		cfg := LoadConfig()
		assert.False(t, cfg.TikTokSandboxEnabled)
		key, secret := cfg.TikTokCredentials()
		assert.Equal(t, "prod-k", key)
		assert.Equal(t, "prod-s", secret)
		assert.True(t, cfg.TikTokDirectConfigured())
		assert.True(t, cfg.Ready())
	})

	t.Run("explicit true selects sandbox credentials", func(t *testing.T) {
		t.Setenv("SOCIAL_OPS_ENABLED", "true")
		t.Setenv("TIKTOK_DIRECT_ENABLED", "true")
		t.Setenv("TIKTOK_SANDBOX_ENABLED", "true")
		t.Setenv("TIKTOK_CLIENT_KEY", "prod-k")
		t.Setenv("TIKTOK_CLIENT_SECRET", "prod-s")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_KEY", "sbx-k")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_SECRET", "sbx-s")

		cfg := LoadConfig()
		assert.True(t, cfg.TikTokSandboxEnabled)
		key, secret := cfg.TikTokCredentials()
		assert.Equal(t, "sbx-k", key)
		assert.Equal(t, "sbx-s", secret)
		assert.True(t, cfg.TikTokDirectConfigured())
		assert.True(t, cfg.Ready())
	})

	t.Run("explicit true with missing sandbox secret fails closed without prod fallback", func(t *testing.T) {
		t.Setenv("SOCIAL_OPS_ENABLED", "true")
		t.Setenv("TIKTOK_DIRECT_ENABLED", "true")
		t.Setenv("TIKTOK_SANDBOX_ENABLED", "true")
		t.Setenv("TIKTOK_CLIENT_KEY", "prod-k")
		t.Setenv("TIKTOK_CLIENT_SECRET", "prod-s")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_KEY", "sbx-k")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_SECRET", "")

		cfg := LoadConfig()
		assert.True(t, cfg.TikTokSandboxEnabled)
		key, secret := cfg.TikTokCredentials()
		assert.Empty(t, key)
		assert.Empty(t, secret)
		assert.False(t, cfg.TikTokDirectConfigured())
		assert.False(t, cfg.Ready())
		assert.ErrorIs(t, NewService(cfg).requireReady(), ErrNotConfigured)
	})

	t.Run("invalid bool value defaults safely to false", func(t *testing.T) {
		t.Setenv("TIKTOK_SANDBOX_ENABLED", "not-a-bool")
		t.Setenv("TIKTOK_CLIENT_KEY", "prod-k")
		t.Setenv("TIKTOK_CLIENT_SECRET", "prod-s")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_KEY", "sbx-k")
		t.Setenv("TIKTOK_SANDBOX_CLIENT_SECRET", "sbx-s")

		cfg := LoadConfig()
		assert.False(t, cfg.TikTokSandboxEnabled)
		key, secret := cfg.TikTokCredentials()
		assert.Equal(t, "prod-k", key)
		assert.Equal(t, "prod-s", secret)
	})
}

func TestValidateRedirectURL(t *testing.T) {
	assert.NoError(t, validateRedirectURL("https://omnimux.ai/cb", "https://omnimux.ai/social/oauth/callback"))
	assert.ErrorIs(t, validateRedirectURL("https://evil.example/cb", "https://omnimux.ai/cb"), ErrRedirectDenied)
	assert.ErrorIs(t, validateRedirectURL("javascript://omnimux.ai/cb", "https://omnimux.ai/cb"), ErrRedirectDenied)
	assert.ErrorIs(t, validateRedirectURL("http://omnimux.ai/cb", "https://omnimux.ai/cb"), ErrRedirectDenied)
	assert.ErrorIs(t, validateRedirectURL("https://user@omnimux.ai/cb", "https://omnimux.ai/cb"), ErrRedirectDenied)
	assert.NoError(t, validateRedirectURL("http://localhost:3000/cb", ""))
	assert.ErrorIs(t, validateRedirectURL("http://evil.example/cb", ""), ErrRedirectDenied)
}

func TestServiceRequireReady(t *testing.T) {
	s := NewService(Config{Enabled: false})
	assert.ErrorIs(t, s.requireReady(), ErrDisabled)
	s = NewService(Config{Enabled: true, APIKey: "", BaseURL: defaultBaseURL, ZernioEnabled: true})
	assert.ErrorIs(t, s.requireReady(), ErrNotConfigured)
	// fork: zernio-retire — a configured but retired proxy is not an upstream.
	s = NewService(Config{Enabled: true, APIKey: "k", BaseURL: defaultBaseURL, ZernioEnabled: false})
	assert.ErrorIs(t, s.requireReady(), ErrNotConfigured)
}

func TestValidateProviderRetirement(t *testing.T) {
	cases := []struct {
		name         string
		provider     string
		zernioActive bool
		want         error
	}{
		{"official direct", ProviderTikTokDirect, false, nil},
		{"retired source", ProviderZernio, false, ErrProviderRetired},
		{"rollback source", ProviderZernio, true, nil},
		{"unknown source", "unknown", false, ErrProviderInvalid},
		{"missing source", "", false, ErrProviderInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, validateProvider(tc.provider, tc.zernioActive), tc.want)
		})
	}
}

func TestStatusNamesOfficialSourceOnly(t *testing.T) {
	retired := NewService(Config{
		Enabled: true, ZernioEnabled: false, APIKey: "k", BaseURL: defaultBaseURL,
		Platforms: []string{"tiktok", "instagram"}, TikTokDirectEnabled: true,
		TikTokClientKey: "key", TikTokClientSecret: "secret",
		TikTokRedirectURI: "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
	})
	got := retired.Status()
	assert.Equal(t, "official", got["provider"])
	assert.Equal(t, []string{"tiktok"}, got["platforms"])

	rollback := NewService(Config{
		Enabled: true, ZernioEnabled: true, APIKey: "k", BaseURL: defaultBaseURL,
		Platforms: []string{"tiktok", "instagram"},
	})
	got = rollback.Status()
	assert.Equal(t, ProviderZernio, got["provider"])
	assert.Equal(t, []string{"tiktok", "instagram"}, got["platforms"])
}

func TestConfigTikTokWebhookDefaultsClampingAndStatus(t *testing.T) {
	t.Run("default off and default tolerance 300s", func(t *testing.T) {
		t.Setenv("SOCIAL_OPS_ENABLED", "")
		t.Setenv("TIKTOK_DIRECT_ENABLED", "")
		t.Setenv("TIKTOK_WEBHOOK_ENABLED", "")
		t.Setenv("TIKTOK_WEBHOOK_TOLERANCE_SECONDS", "")
		t.Setenv("TIKTOK_CLIENT_KEY", "")
		t.Setenv("TIKTOK_CLIENT_SECRET", "")
		t.Setenv("TIKTOK_SANDBOX_ENABLED", "")

		cfg := LoadConfig()
		assert.False(t, cfg.TikTokWebhookEnabled)
		assert.Equal(t, 300, cfg.TikTokWebhookToleranceSeconds)
		assert.Equal(t, 300, int(cfg.TikTokWebhookTolerance().Seconds()))
		assert.False(t, cfg.TikTokWebhookConfigured())
		assert.False(t, cfg.TikTokWebhookActive())
	})

	t.Run("tolerance clamps to [30, 3600] and falls back on invalid", func(t *testing.T) {
		cases := []struct {
			raw  string
			want int
		}{
			{"", 300},
			{"not-an-int", 300},
			{"-10", 30},
			{"0", 30},
			{"29", 30},
			{"30", 30},
			{"600", 600},
			{"3600", 3600},
			{"7200", 3600},
		}
		for _, tc := range cases {
			t.Setenv("TIKTOK_WEBHOOK_TOLERANCE_SECONDS", tc.raw)
			cfg := LoadConfig()
			assert.Equal(t, tc.want, cfg.TikTokWebhookToleranceSeconds, "raw=%q", tc.raw)
			assert.Equal(t, tc.want, int(cfg.TikTokWebhookTolerance().Seconds()), "raw=%q", tc.raw)
		}
	})

	t.Run("status preserves top-level webhook_configured and exposes tiktok_direct booleans", func(t *testing.T) {
		svc := NewService(Config{
			Enabled:              true,
			WebhookSecret:        "legacy-zernio-secret",
			TikTokDirectEnabled:  true,
			TikTokWebhookEnabled: true,
			TikTokClientKey:      "prod-key",
			TikTokClientSecret:   "prod-secret",
			TikTokRedirectURI:    defaultTikTokRedirectURI,
		})
		status := svc.Status()
		assert.Equal(t, true, status["webhook_configured"])
		td, ok := status["tiktok_direct"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, td["webhook_enabled"])
		assert.Equal(t, true, td["webhook_configured"])

		// Sandbox enabled without sandbox secret fails closed for webhook_configured too.
		sbxSvc := NewService(Config{
			Enabled:              true,
			TikTokDirectEnabled:  true,
			TikTokWebhookEnabled: true,
			TikTokClientKey:      "prod-key",
			TikTokClientSecret:   "prod-secret",
			TikTokSandboxEnabled: true,
			TikTokSandboxClientKey: "sbx-key",
			TikTokRedirectURI:    defaultTikTokRedirectURI,
		})
		sbxStatus := sbxSvc.Status()
		assert.Equal(t, false, sbxStatus["webhook_configured"])
		sbxTD, ok := sbxStatus["tiktok_direct"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, sbxTD["webhook_enabled"])
		assert.Equal(t, false, sbxTD["webhook_configured"])
		assert.False(t, sbxSvc.TikTokWebhookReady())
	})
}
