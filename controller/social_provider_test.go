package controller

// fork: social-provider-isolation — HTTP boundaries must reject cross-provider actions before upstream I/O.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
		// fork: zernio-retire — keep both sources reachable for provider isolation.
		ZernioEnabled: true,
		Platforms:     []string{"tiktok"}, TikTokDirectEnabled: true,
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

// fork: zernio-retire — the retired source is gone at the HTTP boundary; local
// capabilities (accounts list of the official source, seats) keep working.
func TestSocialProviderRetirement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldService := model.DB, socialSvc
	model.DB = db
	t.Cleanup(func() { model.DB, socialSvc = oldDB, oldService; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.SocialAccount{}, &model.SocialPost{}, &model.SocialSeat{}))
	require.NoError(t, db.Create(&model.SocialAccount{
		ID: 2, UserId: 7, Provider: "tiktok_direct", ProviderAccountID: "d2", Platform: "tiktok", Status: "active",
		ScopesGranted: "user.info.basic,video.upload",
	}).Error)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(upstream.Close)
	setSocialService(social.NewService(social.Config{
		Enabled: true, APIKey: "test-zernio", BaseURL: upstream.URL,
		ZernioEnabled: false,
		Platforms:     []string{"tiktok"}, TikTokDirectEnabled: true,
		TikTokScopes:    "user.info.basic,video.upload",
		TikTokClientKey: "test-key", TikTokClientSecret: "test-secret",
		TikTokRedirectURI: "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		SocialTokenEncKey: base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}))
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7); c.Next() })
	router.GET("/accounts", SocialListAccounts)
	router.GET("/seats", SocialSeats)
	router.POST("/connect", SocialConnect)
	router.DELETE("/accounts/:id", SocialDisconnectAccount)
	router.POST("/posts", SocialCreatePost)
	router.GET("/posts/:id", SocialGetPost)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/accounts?provider=zernio", ""},
		{"POST", "/connect", `{"provider":"zernio","platform":"tiktok"}`},
		{"DELETE", "/accounts/2?provider=zernio", ""},
		{"POST", "/posts", `{"provider":"zernio","content":"test","account_ids":[2]}`},
		{"GET", "/posts/1?provider=zernio", ""},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := request(tc.method, tc.path, tc.body)
			assert.Equal(t, http.StatusGone, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), `"code":"provider-retired"`)
		})
	}
	assert.Zero(t, calls.Load(), "retired source must not reach upstream")

	w := request("GET", "/accounts?provider=tiktok_direct", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"tiktok_direct"`)
	assert.Contains(t, w.Body.String(), `"upload_draft":true`)
	assert.Contains(t, w.Body.String(), `"publish_direct":false`)

	w = request("GET", "/seats", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"billing_mode"`)
}

func TestRenderTikTokCallbackPageEscapesDynamicValues(t *testing.T) {
	page := renderTikTokCallbackPage("<script>alert(1)</script>", `<img src=x onerror=alert(1)>`, "")
	assert.NotContains(t, page, "<script>alert(1)</script>")
	assert.Contains(t, page, "&lt;script&gt;alert(1)&lt;/script&gt;")
}

func TestSocialOAuthBrowserCookieBinding(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "social-oauth-cookie-test"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	state := "state-123"
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	setSocialOAuthBrowserCookie(c, state)
	cookies := recorder.Result().Cookies()
	require.Len(t, cookies, 1)

	callbackRecorder := httptest.NewRecorder()
	callbackContext, _ := gin.CreateTestContext(callbackRecorder)
	callbackContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	callbackContext.Request.AddCookie(cookies[0])
	assert.True(t, socialOAuthBrowserCookieValid(callbackContext, state))

	otherRecorder := httptest.NewRecorder()
	otherContext, _ := gin.CreateTestContext(otherRecorder)
	otherContext.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	otherContext.Request.AddCookie(cookies[0])
	assert.False(t, socialOAuthBrowserCookieValid(otherContext, "state-other"))
}

func TestSocialTikTokOAuthStartAndCallbackOWASP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldService := model.DB, socialSvc
	oldSecret, oldSecure := common.SessionSecret, common.SessionCookieSecure
	model.DB = db
	common.SessionSecret = "test-tiktok-oauth-session-secret"
	common.SessionCookieSecure = true
	t.Cleanup(func() {
		model.DB, socialSvc = oldDB, oldService
		common.SessionSecret, common.SessionCookieSecure = oldSecret, oldSecure
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SocialAccount{}, &model.SocialPost{}, &model.SocialSeat{}, &model.SocialOAuthState{}))
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "u7", Quota: 1000000}).Error)

	var grantedScope atomic.Value
	grantedScope.Store("user.info.basic,video.upload")
	var openIDCounter atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/oauth/token/":
			oid := fmt.Sprintf("oid-cb-%d", openIDCounter.Add(1))
			scope, _ := grantedScope.Load().(string)
			_, _ = fmt.Fprintf(w, `{"access_token":"at-%s","refresh_token":"rt-%s","expires_in":7200,"refresh_expires_in":15552000,"open_id":"%s","scope":"%s","token_type":"Bearer"}`, oid, oid, oid, scope)
		case "/v2/user/info/":
			oid := fmt.Sprintf("oid-cb-%d", openIDCounter.Load())
			_, _ = fmt.Fprintf(w, `{"data":{"user":{"open_id":"%s","display_name":"Creator %s"}},"error":{"code":"ok","message":"","log_id":"log-ui"}}`, oid, oid)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(upstream.Close)

	encKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	baseCfg := social.Config{
		Enabled:             true,
		Platforms:           []string{"tiktok"},
		TikTokDirectEnabled: true,
		TikTokClientKey:     "tt-ck",
		TikTokClientSecret:  "tt-cs",
		TikTokRedirectURI:   "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		TikTokAuthorizeURL:  "https://www.tiktok.com/v2/auth/authorize/",
		TikTokBaseURL:       upstream.URL,
		TikTokScopes:        "user.info.basic,video.upload",
		SocialTokenEncKey:   encKey,
	}
	setSocialService(social.NewService(baseCfg))

	router := gin.New()
	router.GET("/api/social/v1/oauth/tiktok/start", SocialTikTokOAuthStart)
	router.GET("/api/social/v1/oauth/tiktok/callback", SocialTikTokCallback)
	authed := router.Group("/api/social/v1")
	authed.Use(func(c *gin.Context) { c.Set("id", 7); c.Next() })
	authed.POST("/connect", SocialConnect)
	authed.GET("/accounts", SocialListAccounts)
	authed.POST("/posts", SocialCreatePost)

	assertStartSecurityHeaders := func(t *testing.T, h http.Header) {
		t.Helper()
		assert.Equal(t, "no-store", h.Get("Cache-Control"))
		assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
		assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
		assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
		assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	}

	connect := func(t *testing.T, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/social/v1/connect", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	extractAuthURL := func(t *testing.T, w *httptest.ResponseRecorder) *url.URL {
		t.Helper()
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		assert.Empty(t, w.Result().Cookies(), "POST /connect must not rely on setting the OAuth browser cookie")
		var resp struct {
			Data struct {
				AuthURL string `json:"auth_url"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
		parsed, parseErr := url.Parse(resp.Data.AuthURL)
		require.NoError(t, parseErr)
		return parsed
	}

	t.Run("cross-browser launch ticket start sets __Host cookie and completes callback", func(t *testing.T) {
		startURL := extractAuthURL(t, connect(t, `{"provider":"tiktok_direct","platform":"tiktok"}`))
		assert.Equal(t, "https", startURL.Scheme)
		assert.Equal(t, "omnimux.ai", startURL.Host)
		assert.Equal(t, "/api/social/v1/oauth/tiktok/start", startURL.Path)
		ticket := startURL.Query().Get("ticket")
		require.NotEmpty(t, ticket)

		// Extra untrusted query parameters must be ignored (no open redirect).
		startReq := httptest.NewRequest(http.MethodGet, startURL.RequestURI()+"&redirect_url=https://evil.example/steal&next=https://evil.example/steal", nil)
		startRec := httptest.NewRecorder()
		router.ServeHTTP(startRec, startReq)
		require.Equal(t, http.StatusFound, startRec.Code)
		assertStartSecurityHeaders(t, startRec.Header())

		loc, parseErr := url.Parse(startRec.Header().Get("Location"))
		require.NoError(t, parseErr)
		assert.Equal(t, "https", loc.Scheme)
		assert.Equal(t, "www.tiktok.com", loc.Host)
		assert.Equal(t, "/v2/auth/authorize/", loc.Path)
		state := loc.Query().Get("state")
		require.NotEmpty(t, state)
		assert.NotEqual(t, ticket, state)

		cookies := startRec.Result().Cookies()
		require.Len(t, cookies, 1)
		sum := sha256.Sum256([]byte(state))
		wantName := "__Host-social_tiktok_oauth_" + hex.EncodeToString(sum[:])
		assert.Equal(t, wantName, cookies[0].Name)
		assert.True(t, cookies[0].HttpOnly)
		assert.True(t, cookies[0].Secure)
		assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
		assert.Equal(t, "/", cookies[0].Path)
		assert.Empty(t, cookies[0].Domain)

		// Ticket replay is rejected without leaking ticket value.
		replayRec := httptest.NewRecorder()
		router.ServeHTTP(replayRec, httptest.NewRequest(http.MethodGet, startURL.RequestURI(), nil))
		assert.Equal(t, http.StatusBadRequest, replayRec.Code)
		assertStartSecurityHeaders(t, replayRec.Header())
		assert.NotContains(t, replayRec.Body.String(), ticket)

		// Callback without browser cookie is rejected and does NOT consume state.
		noCookieRec := httptest.NewRecorder()
		router.ServeHTTP(noCookieRec, httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=c1&state="+url.QueryEscape(state), nil))
		assert.Equal(t, http.StatusBadRequest, noCookieRec.Code)
		assert.Contains(t, noCookieRec.Body.String(), "authorization flow does not belong to this browser")

		// Callback with forged/other-state cookie is rejected.
		wrongCookieReq := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=c1&state="+url.QueryEscape(state), nil)
		wrongCookieReq.AddCookie(&http.Cookie{Name: wantName, Value: "forged-hmac"})
		wrongCookieRec := httptest.NewRecorder()
		router.ServeHTTP(wrongCookieRec, wrongCookieReq)
		assert.Equal(t, http.StatusBadRequest, wrongCookieRec.Code)

		// Callback with the browser cookie succeeds and clears the cookie in Set-Cookie.
		cbReq := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=c1&state="+url.QueryEscape(state), nil)
		cbReq.AddCookie(cookies[0])
		cbRec := httptest.NewRecorder()
		router.ServeHTTP(cbRec, cbReq)
		require.Equal(t, http.StatusOK, cbRec.Code, cbRec.Body.String())
		assert.Contains(t, cbRec.Body.String(), "TikTok 已连接")
		cleared := cbRec.Result().Cookies()
		require.Len(t, cleared, 1)
		assert.Equal(t, wantName, cleared[0].Name)
		assert.Less(t, cleared[0].MaxAge, 0, "callback must emit Set-Cookie clearing header before response write")

		// State replay even with the old browser cookie fails.
		cbReplayReq := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=c1&state="+url.QueryEscape(state), nil)
		cbReplayReq.AddCookie(cookies[0])
		cbReplayRec := httptest.NewRecorder()
		router.ServeHTTP(cbReplayRec, cbReplayReq)
		assert.Equal(t, http.StatusBadRequest, cbReplayRec.Code)
	})

	t.Run("missing, invalid, expired ticket and expired state return safe HTML", func(t *testing.T) {
		for _, path := range []string{
			"/api/social/v1/oauth/tiktok/start",
			"/api/social/v1/oauth/tiktok/start?ticket=nonexistent-secret-ticket-999",
		} {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assertStartSecurityHeaders(t, rec.Header())
			assert.NotContains(t, rec.Body.String(), "nonexistent-secret-ticket-999")
		}

		// Expired ticket
		require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
			State:     "expired-ticket-secret-1",
			UserId:    7,
			Provider:  social.ProviderTikTokDirectStart,
			ExpiresAt: time.Now().UTC().Add(-time.Minute),
		}))
		expTicketRec := httptest.NewRecorder()
		router.ServeHTTP(expTicketRec, httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/start?ticket=expired-ticket-secret-1", nil))
		assert.Equal(t, http.StatusBadRequest, expTicketRec.Code)
		assertStartSecurityHeaders(t, expTicketRec.Header())
		assert.NotContains(t, expTicketRec.Body.String(), "expired-ticket-secret-1")

		// Expired state with valid browser cookie
		const expiredState = "expired-state-1"
		require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
			State:     expiredState,
			UserId:    7,
			Provider:  social.ProviderTikTokDirect,
			ExpiresAt: time.Now().UTC().Add(-time.Minute),
		}))
		cookieRec := httptest.NewRecorder()
		cookieCtx, _ := gin.CreateTestContext(cookieRec)
		cookieCtx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		setSocialOAuthBrowserCookie(cookieCtx, expiredState)
		expCbReq := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=c2&state="+expiredState, nil)
		expCbReq.AddCookie(cookieRec.Result().Cookies()[0])
		expCbRec := httptest.NewRecorder()
		router.ServeHTTP(expCbRec, expCbReq)
		assert.Equal(t, http.StatusBadRequest, expCbRec.Code)
		require.Len(t, expCbRec.Result().Cookies(), 1)
		assert.Less(t, expCbRec.Result().Cookies()[0].MaxAge, 0)
	})

	t.Run("foreign redirect_url and unapproved authorize host are blocked", func(t *testing.T) {
		w := connect(t, `{"provider":"tiktok_direct","platform":"tiktok","redirect_url":"https://evil.example/cb"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), `"code":"SOCIAL_REDIRECT_DENIED"`)

		badCfg := baseCfg
		badCfg.TikTokAuthorizeURL = "https://evil.example/v2/auth/authorize/"
		setSocialService(social.NewService(badCfg))
		t.Cleanup(func() { setSocialService(social.NewService(baseCfg)) })

		require.NoError(t, model.CreateSocialOAuthState(&model.SocialOAuthState{
			State:     "ticket-bad-auth-host",
			UserId:    7,
			Provider:  social.ProviderTikTokDirectStart,
			ExpiresAt: time.Now().UTC().Add(time.Minute),
		}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/start?ticket=ticket-bad-auth-host", nil))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Empty(t, rec.Header().Get("Location"))
		assertStartSecurityHeaders(t, rec.Header())
	})

	t.Run("user denying video.upload exposes false capabilities and blocks post with 409 SOCIAL_TIKTOK_SCOPE_MISSING", func(t *testing.T) {
		setSocialService(social.NewService(baseCfg))
		grantedScope.Store("user.info.basic")
		t.Cleanup(func() { grantedScope.Store("user.info.basic,video.upload") })

		startURL := extractAuthURL(t, connect(t, `{"provider":"tiktok_direct","platform":"tiktok"}`))
		startRec := httptest.NewRecorder()
		router.ServeHTTP(startRec, httptest.NewRequest(http.MethodGet, startURL.RequestURI(), nil))
		require.Equal(t, http.StatusFound, startRec.Code)
		loc, parseErr := url.Parse(startRec.Header().Get("Location"))
		require.NoError(t, parseErr)
		state := loc.Query().Get("state")

		cbReq := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=c-basic&state="+url.QueryEscape(state), nil)
		cbReq.AddCookie(startRec.Result().Cookies()[0])
		cbRec := httptest.NewRecorder()
		router.ServeHTTP(cbRec, cbReq)
		require.Equal(t, http.StatusOK, cbRec.Code, cbRec.Body.String())

		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/social/v1/accounts?provider=tiktok_direct", nil))
		require.Equal(t, http.StatusOK, listRec.Code, listRec.Body.String())
		var listResp struct {
			Data struct {
				Accounts []struct {
					ID           int64 `json:"id"`
					Capabilities struct {
						UploadDraft   bool `json:"upload_draft"`
						PublishDirect bool `json:"publish_direct"`
					} `json:"capabilities"`
				} `json:"accounts"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(listRec.Body.Bytes(), &listResp))
		require.NotEmpty(t, listResp.Data.Accounts)
		// Latest created account is ordered first (ORDER BY id DESC).
		basicAcc := listResp.Data.Accounts[0]
		assert.False(t, basicAcc.Capabilities.UploadDraft)
		assert.False(t, basicAcc.Capabilities.PublishDirect)

		postBody := fmt.Sprintf(`{"provider":"tiktok_direct","content":"hello","account_ids":[%d],"publish_now":true,"media_items":[{"url":"https://cdn.example.com/v.mp4"}]}`, basicAcc.ID)
		postReq := httptest.NewRequest(http.MethodPost, "/api/social/v1/posts", strings.NewReader(postBody))
		postReq.Header.Set("Content-Type", "application/json")
		postRec := httptest.NewRecorder()
		router.ServeHTTP(postRec, postReq)
		assert.Equal(t, http.StatusConflict, postRec.Code, postRec.Body.String())
		assert.Contains(t, postRec.Body.String(), `"code":"SOCIAL_TIKTOK_SCOPE_MISSING"`)
	})
}

func TestSocialTikTokPhotoDraftAndInboxStatusHTTP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldService := model.DB, socialSvc
	model.DB = db
	t.Cleanup(func() { model.DB, socialSvc = oldDB, oldService; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.SocialAccount{}, &model.SocialPost{}, &model.SocialSeat{}))

	var gotInitBody string
	var statusPolls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/post/publish/content/init/", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 16384)
		n, _ := r.Body.Read(buf)
		gotInitBody = string(buf[:n])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"publish_id":"pub-http-photo-1"},"error":{"code":"ok","message":""}}`))
	})
	mux.HandleFunc("/v2/post/publish/status/fetch/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if statusPolls.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"data":{"status":"SEND_TO_USER_INBOX","publish_id":"pub-http-photo-1"},"error":{"code":"ok","message":""}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"status":"PUBLISH_COMPLETE","publish_id":"pub-http-photo-1"},"error":{"code":"ok","message":""}}`))
	})
	upstream := httptest.NewServer(mux)
	t.Cleanup(upstream.Close)

	encKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	setSocialService(social.NewService(social.Config{
		Enabled:             true,
		Platforms:           []string{"tiktok"},
		TikTokDirectEnabled: true,
		TikTokClientKey:     "test-key",
		TikTokClientSecret:  "test-secret",
		TikTokRedirectURI:   "https://omnimux.ai/api/social/v1/oauth/tiktok/callback",
		TikTokBaseURL:       upstream.URL,
		TikTokScopes:        "user.info.basic,video.upload",
		SocialTokenEncKey:   encKey,
		MaxMediaItems:       35,
	}))

	encTok, err := social.SealToken("at-http-photo", encKey)
	require.NoError(t, err)
	acc := &model.SocialAccount{
		UserId:               7,
		Provider:             social.ProviderTikTokDirect,
		ProviderAccountID:    "oid-http-photo",
		Platform:             "tiktok",
		Status:               model.SocialAccountStatusActive,
		AccessTokenEncrypted: encTok,
		ScopesGranted:        "user.info.basic,video.upload",
	}
	require.NoError(t, db.Create(acc).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7); c.Next() })
	router.POST("/api/social/v1/posts", SocialCreatePost)
	router.GET("/api/social/v1/posts/:id", SocialGetPost)

	postReqBody := fmt.Sprintf(`{
		"provider":"tiktok_direct",
		"account_ids":[%d],
		"media_items":[
			{"url":"https://cdn.example.com/slide1.jpg"},
			{"url":"https://cdn.example.com/slide2.webp"}
		],
		"tiktok_options":{
			"title":"Carousel Draft",
			"description":"Two photos",
			"photo_cover_index":0,
			"is_aigc":false,
			"privacy_level":"PUBLIC_TO_EVERYONE"
		}
	}`, acc.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/social/v1/posts", strings.NewReader(postReqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var created struct {
		Data struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "submitted", created.Data.Status)
	assert.Contains(t, gotInitBody, `"post_mode":"MEDIA_UPLOAD"`)
	assert.Contains(t, gotInitBody, `"media_type":"PHOTO"`)
	assert.Contains(t, gotInitBody, `"photo_cover_index":0`)
	assert.Contains(t, gotInitBody, `"is_aigc":false`)
	assert.NotContains(t, gotInitBody, "privacy_level")

	// First refresh returns sent_to_inbox with original upstream result.status preserved.
	get1 := httptest.NewRecorder()
	router.ServeHTTP(get1, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/social/v1/posts/%d?provider=tiktok_direct&refresh=1", created.Data.ID), nil))
	require.Equal(t, http.StatusOK, get1.Code, get1.Body.String())
	var statusResp1 struct {
		Data struct {
			Status string         `json:"status"`
			Result map[string]any `json:"result"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(get1.Body.Bytes(), &statusResp1))
	assert.Equal(t, "sent_to_inbox", statusResp1.Data.Status)
	assert.Equal(t, "SEND_TO_USER_INBOX", statusResp1.Data.Result["status"])

	// Second refresh continues polling from sent_to_inbox and transitions to published.
	get2 := httptest.NewRecorder()
	router.ServeHTTP(get2, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/social/v1/posts/%d?provider=tiktok_direct&refresh=1", created.Data.ID), nil))
	require.Equal(t, http.StatusOK, get2.Code, get2.Body.String())
	var statusResp2 struct {
		Data struct {
			Status string         `json:"status"`
			Result map[string]any `json:"result"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(get2.Body.Bytes(), &statusResp2))
	assert.Equal(t, "published", statusResp2.Data.Status)
	assert.Equal(t, "PUBLISH_COMPLETE", statusResp2.Data.Result["status"])

	// Invalid photo format and mixed media return 400 SOCIAL_MEDIA_INVALID.
	for _, badBody := range []string{
		fmt.Sprintf(`{"provider":"tiktok_direct","account_ids":[%d],"media_items":[{"url":"https://cdn.example.com/a.png"}]}`, acc.ID),
		fmt.Sprintf(`{"provider":"tiktok_direct","account_ids":[%d],"media_items":[{"url":"https://cdn.example.com/a.jpg"},{"url":"https://cdn.example.com/v.mp4"}]}`, acc.ID),
	} {
		badReq := httptest.NewRequest(http.MethodPost, "/api/social/v1/posts", strings.NewReader(badBody))
		badReq.Header.Set("Content-Type", "application/json")
		badRec := httptest.NewRecorder()
		router.ServeHTTP(badRec, badReq)
		assert.Equal(t, http.StatusBadRequest, badRec.Code, badRec.Body.String())
		assert.Contains(t, badRec.Body.String(), `"code":"SOCIAL_MEDIA_INVALID"`)
	}
}
