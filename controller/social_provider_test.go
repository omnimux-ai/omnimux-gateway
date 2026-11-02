package controller

// fork: social-provider-isolation — HTTP boundaries must reject cross-provider actions before upstream I/O.

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSocialProviderHTTPIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldService := model.DB, socialSvc
	model.DB = db
	t.Cleanup(func() { model.DB, socialSvc = oldDB, oldService; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.SocialAccount{}, &model.SocialPost{}, &model.SocialTenant{}, &model.SocialOAuthState{}))
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"message":"Account paused: payment required"}`))
	}))
	t.Cleanup(upstream.Close)
	setSocialService(social.NewService(social.Config{
		Enabled: true, APIKey: "test-zernio", BaseURL: upstream.URL,
		Platforms: []string{"tiktok"}, TikTokDirectEnabled: true,
		TikTokClientKey: "test-key", TikTokClientSecret: "test-secret",
		TikTokRedirectURI: "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		SocialTokenEncKey: base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}))
	accounts := []model.SocialAccount{
		{ID: 1, UserId: 7, Provider: "zernio", ProviderAccountID: "z1", Platform: "tiktok", DisplayName: "same", Status: "active"},
		{ID: 2, UserId: 7, Provider: "tiktok_direct", ProviderAccountID: "d2", Platform: "tiktok", DisplayName: "same", Status: "active"},
		{ID: 3, UserId: 8, Provider: "tiktok_direct", ProviderAccountID: "d3", Platform: "tiktok", Status: "active"},
		{ID: 4, UserId: 7, Provider: "zernio", ProviderAccountID: "z4", Platform: "tiktok", Status: "active"},
	}
	require.NoError(t, db.Create(&accounts).Error)
	require.NoError(t, db.Create(&model.SocialTenant{UserId: 7, Provider: "zernio", ProviderProfileID: "p7"}).Error)
	require.NoError(t, db.Create(&model.SocialPost{ID: 1, UserId: 7, Provider: "zernio", ProviderPostID: "post-z1", Status: "submitted", IdempotencyKey: "z-key", RequestSnapshot: `{"account_ids":[1]}`}).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7); c.Next() })
	router.GET("/accounts", SocialListAccounts)
	router.GET("/accounts/:id/creator-info", SocialAccountCreatorInfo)
	router.POST("/connect", SocialConnect)
	router.DELETE("/accounts/:id", SocialDisconnectAccount)
	router.POST("/posts", SocialCreatePost)
	router.GET("/posts/:id", SocialGetPost)
	router.GET("/public/media/:token", SocialPublicMediaProxy)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "z-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	t.Run("official list does not touch Zernio or include its cached rows", func(t *testing.T) {
		before := calls.Load()
		w := request("GET", "/accounts?provider=tiktok_direct", "")
		require.Equal(t, 200, w.Code, w.Body.String())
		var result struct {
			Data struct {
				Accounts []model.SocialAccount `json:"accounts"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		require.Len(t, result.Data.Accounts, 1)
		assert.Equal(t, int64(2), result.Data.Accounts[0].ID)
		assert.Equal(t, before, calls.Load())
	})
	t.Run("Zernio list excludes direct accounts even when using cached data", func(t *testing.T) {
		w := request("GET", "/accounts?provider=zernio", "")
		require.Equal(t, 200, w.Code, w.Body.String())
		var result struct {
			Data struct {
				Accounts []model.SocialAccount `json:"accounts"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		require.Len(t, result.Data.Accounts, 2)
		for _, account := range result.Data.Accounts {
			assert.Equal(t, "zernio", account.Provider)
			assert.Contains(t, []int64{1, 4}, account.ID)
		}
	})
	for _, tc := range []struct {
		method, path, body, code string
		status                   int
	}{
		{"GET", "/accounts", "", "invalid-provider", 400},
		{"GET", "/accounts?provider=unknown", "", "invalid-provider", 400},
		{"POST", "/connect", `{"platform":"tiktok"}`, "invalid-provider", 400},
		{"POST", "/connect", `{"provider":"unknown","platform":"tiktok"}`, "invalid-provider", 400},
		{"POST", "/connect", `{"provider":"tiktok_direct","platform":"instagram"}`, "SOCIAL_PLATFORM_DENIED", 400},
		{"DELETE", "/accounts/1", "", "invalid-provider", 400},
		{"POST", "/posts", `{"content":"test","account_ids":[1]}`, "invalid-provider", 400},
		{"GET", "/posts/1", "", "invalid-provider", 400},
		{"DELETE", "/accounts/1?provider=tiktok_direct", "", "account-provider-mismatch", 409},
		{"DELETE", "/accounts/2?provider=zernio", "", "account-provider-mismatch", 409},
		{"GET", "/accounts/1/creator-info?provider=tiktok_direct", "", "account-provider-mismatch", 409},
		{"GET", "/accounts/2/creator-info?provider=zernio", "", "account-provider-mismatch", 409},
		{"GET", "/accounts/3/creator-info?provider=tiktok_direct", "", "SOCIAL_ACCOUNT_NOT_FOUND", 404},
		{"GET", "/public/media/invalid-token", "", "SOCIAL_MEDIA_INVALID", 400},
		{"GET", "/posts/1?provider=tiktok_direct", "", "post-provider-mismatch", 409},
		{"POST", "/posts", `{"provider":"tiktok_direct","content":"test","account_ids":[1]}`, "account-provider-mismatch", 409},
		{"POST", "/posts", `{"provider":"tiktok_direct","content":"test","account_ids":[2]}`, "post-provider-mismatch", 409},
		{"POST", "/posts", `{"provider":"zernio","content":"test","account_ids":[4]}`, "post-provider-mismatch", 409},
		{"POST", "/posts", `{"provider":"zernio","content":"test","account_ids":[1,4]}`, "post-provider-mismatch", 409},
		{"POST", "/posts", `{"provider":"zernio","content":"test","account_ids":[1,2]}`, "account-provider-mismatch", 409},
		{"DELETE", "/accounts/3?provider=tiktok_direct", "", "SOCIAL_ACCOUNT_NOT_FOUND", 404},
	} {
		t.Run(fmt.Sprintf("%s %s %s", tc.method, tc.path, tc.body), func(t *testing.T) {
			before := calls.Load()
			w := request(tc.method, tc.path, tc.body)
			assert.Equal(t, tc.status, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), `"code":"`+tc.code+`"`)
			assert.Equal(t, before, calls.Load(), "rejected request must not reach upstream")
		})
	}
	t.Run("same-source same-account idempotency replay is local", func(t *testing.T) {
		before := calls.Load()
		w := request("POST", "/posts", `{"provider":"zernio","content":"test","account_ids":[1]}`)
		require.Equal(t, 200, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), `"provider":"zernio"`)
		assert.Equal(t, before, calls.Load())
	})
	t.Run("idempotency cannot trust a missing or malformed saved account set", func(t *testing.T) {
		for _, snapshot := range []string{`{}`, `{"account_ids":[]}`, `not-json`} {
			require.NoError(t, db.Model(&model.SocialPost{}).Where("id = ?", 1).Update("request_snapshot", snapshot).Error)
			before := calls.Load()
			w := request("POST", "/posts", `{"provider":"zernio","content":"test","account_ids":[1]}`)
			assert.Equal(t, 409, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), `"code":"post-provider-mismatch"`)
			assert.Equal(t, before, calls.Load())
		}
	})
	var unchanged model.SocialAccount
	require.NoError(t, db.First(&unchanged, 1).Error)
	assert.Equal(t, "active", unchanged.Status)
	assert.Equal(t, "zernio", unchanged.Provider)
}
