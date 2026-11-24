package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocialTikTokOAuthCallbackAliasFullRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NotPanics(t, func() {
		SetRouter(engine, WebAssets{IndexPage: []byte(searchPageTestShell)})
	})

	registered := map[string]string{}
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}
	for _, key := range []string{
		"GET /oauth/tiktok/callback",
		"POST /oauth/tiktok/callback",
		"GET /api/social/v1/oauth/tiktok/callback",
		"POST /api/social/v1/oauth/tiktok/callback",
	} {
		handler, ok := registered[key]
		require.True(t, ok, "route %s must be registered", key)
		assert.Contains(t, handler, "SocialTikTokCallback", key)
	}

	// Both GET and POST on the root callback alias hit SocialTikTokCallback rather than the Web SPA.
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/oauth/tiktok/callback"},
		{method: http.MethodPost, path: "/oauth/tiktok/callback", body: ""},
		{method: http.MethodGet, path: "/api/social/v1/oauth/tiktok/callback"},
		{method: http.MethodPost, path: "/api/social/v1/oauth/tiktok/callback", body: ""},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), "missing code or state")
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
			assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		})
	}

	// Frontend OAuth SPA paths remain routed to the Web SPA fallback and are not hijacked.
	for _, spaPath := range []string{"/oauth", "/oauth/github?code=abc&state=xyz"} {
		t.Run("web spa "+spaPath, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, spaPath, nil))
			assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), `<div id="root"></div>`)
			assert.Contains(t, rec.Body.String(), `/assets/app.js`)
			assert.NotContains(t, rec.Body.String(), "TikTok 授权失败")
		})
	}
}

func TestSocialTikTokOAuthCallbackAliasAccessLogRedaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldWriter := gin.DefaultWriter
	var logBuf bytes.Buffer
	gin.DefaultWriter = &logBuf
	t.Cleanup(func() { gin.DefaultWriter = oldWriter })

	engine := gin.New()
	middleware.SetUpLogger(engine)
	SetRouter(engine, WebAssets{IndexPage: []byte(searchPageTestShell)})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/tiktok/callback?code=secret-code-123&state=secret-state-456", nil))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	logLine := logBuf.String()
	assert.Contains(t, logLine, "| api |")
	assert.Contains(t, logLine, "GET /oauth/tiktok/callback")
	assert.NotContains(t, logLine, "secret-code-123")
	assert.NotContains(t, logLine, "secret-state-456")
}

func TestSocialTikTokWebhookFullRouterWiringAndFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NotPanics(t, func() {
		SetRouter(engine, WebAssets{IndexPage: []byte(searchPageTestShell)})
	})

	registered := map[string]string{}
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}
	handler, ok := registered["POST /api/social/v1/webhooks/tiktok"]
	require.True(t, ok, "POST /api/social/v1/webhooks/tiktok must be registered")
	assert.Contains(t, handler, "SocialTikTokWebhook")
	_, getRegistered := registered["GET /api/social/v1/webhooks/tiktok"]
	assert.False(t, getRegistered, "GET /api/social/v1/webhooks/tiktok must not be registered")

	// Default-off full-router POST returns 403 Forbidden (not UserAuth 401) with security headers.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))

	// Request exceeding AnonymousRequestBodyLimit (512 KiB runtime default) returns 413 with security headers.
	prevLimitKB := constant.AnonymousRequestBodyLimitKB
	constant.AnonymousRequestBodyLimitKB = 512
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = prevLimitKB })
	bigBody := bytes.Repeat([]byte("a"), 512<<10+1)
	recBig := httptest.NewRecorder()
	reqBig := httptest.NewRequest(http.MethodPost, "/api/social/v1/webhooks/tiktok", bytes.NewReader(bigBody))
	engine.ServeHTTP(recBig, reqBig)
	assert.Equal(t, http.StatusRequestEntityTooLarge, recBig.Code)
	assert.Equal(t, "no-store", recBig.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recBig.Header().Get("X-Content-Type-Options"))
}
