package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetUpLoggerRedactsSocialOAuthAndPublicMediaWithoutMutatingHandlerInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var output bytes.Buffer
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	SetUpLogger(router)

	var gotStartTicket string
	router.GET("/api/social/v1/oauth/tiktok/start", func(c *gin.Context) {
		gotStartTicket = c.Query("ticket")
		c.Status(http.StatusFound)
	})

	var gotCallbackCode, gotCallbackState, gotCallbackScopes string
	callbackHandler := func(c *gin.Context) {
		gotCallbackCode = c.Query("code")
		gotCallbackState = c.Query("state")
		gotCallbackScopes = c.Query("scopes")
		c.Status(http.StatusOK)
	}
	router.GET("/api/social/v1/oauth/tiktok/callback", callbackHandler)
	router.POST("/api/social/v1/oauth/tiktok/callback", callbackHandler)

	var gotMediaToken, gotMediaQuery string
	router.GET("/api/social/v1/public/media/:token", func(c *gin.Context) {
		gotMediaToken = c.Param("token")
		gotMediaQuery = c.Request.URL.RawQuery
		c.Status(http.StatusOK)
	})

	var gotBenignProvider, gotBenignRefresh, gotBenignPage string
	router.GET("/api/social/v1/posts/:id", func(c *gin.Context) {
		gotBenignProvider = c.Query("provider")
		gotBenignRefresh = c.Query("refresh")
		gotBenignPage = c.Query("page")
		c.Status(http.StatusOK)
	})

	var gotSensitive map[string]string
	router.GET("/api/social/v1/accounts", func(c *gin.Context) {
		gotSensitive = map[string]string{
			"provider":      c.Query("provider"),
			"ticket":        c.Query("ticket"),
			"CODE":          c.Query("CODE"),
			"State":         c.Query("State"),
			"access_token":  c.Query("access_token"),
			"Refresh_Token": c.Query("Refresh_Token"),
			"id_token":      c.Query("id_token"),
			"refresh":       c.Query("refresh"),
		}
		c.Status(http.StatusOK)
	})

	t.Run("oauth start strips ticket query in log only", func(t *testing.T) {
		output.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/start?ticket=launch-secret-123", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusFound, rec.Code)
		assert.Equal(t, "launch-secret-123", gotStartTicket)
		logLine := output.String()
		assert.Contains(t, logLine, "GET /api/social/v1/oauth/tiktok/start\n")
		assert.NotContains(t, logLine, "launch-secret-123")
		assert.NotContains(t, logLine, "ticket=")
	})

	t.Run("oauth GET callback strips code state scopes in log only", func(t *testing.T) {
		output.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/social/v1/oauth/tiktok/callback?code=auth-code-456&state=csrf-state-789&scopes=user.info.basic,video.upload", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "auth-code-456", gotCallbackCode)
		assert.Equal(t, "csrf-state-789", gotCallbackState)
		assert.Equal(t, "user.info.basic,video.upload", gotCallbackScopes)
		logLine := output.String()
		assert.Contains(t, logLine, "GET /api/social/v1/oauth/tiktok/callback\n")
		assert.NotContains(t, logLine, "auth-code-456")
		assert.NotContains(t, logLine, "csrf-state-789")
		assert.NotContains(t, logLine, "user.info.basic")
		assert.NotContains(t, logLine, "video.upload")
	})

	t.Run("oauth POST callback strips query in log only", func(t *testing.T) {
		output.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/social/v1/oauth/tiktok/callback?code=post-code-1&state=post-state-2", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "post-code-1", gotCallbackCode)
		assert.Equal(t, "post-state-2", gotCallbackState)
		logLine := output.String()
		assert.Contains(t, logLine, "POST /api/social/v1/oauth/tiktok/callback\n")
		assert.NotContains(t, logLine, "post-code-1")
		assert.NotContains(t, logLine, "post-state-2")
	})

	t.Run("public media redacts token segment and strips query in log only", func(t *testing.T) {
		output.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/social/v1/public/media/signed-media-token-abc.xyz?dl=1&sig=extra-secret", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "signed-media-token-abc.xyz", gotMediaToken)
		assert.Equal(t, "dl=1&sig=extra-secret", gotMediaQuery)
		logLine := output.String()
		assert.Contains(t, logLine, "GET /api/social/v1/public/media/[REDACTED]\n")
		assert.NotContains(t, logLine, "signed-media-token-abc.xyz")
		assert.NotContains(t, logLine, "extra-secret")
		assert.NotContains(t, logLine, "dl=1")
	})

	t.Run("benign query params stay intact in log and handler", func(t *testing.T) {
		output.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/social/v1/posts/99?provider=tiktok_direct&refresh=1&page=2", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "tiktok_direct", gotBenignProvider)
		assert.Equal(t, "1", gotBenignRefresh)
		assert.Equal(t, "2", gotBenignPage)
		logLine := output.String()
		assert.Contains(t, logLine, "GET /api/social/v1/posts/99?provider=tiktok_direct&refresh=1&page=2\n")
	})

	t.Run("other routes redact sensitive query keys case-insensitively while keeping benign params and handler inputs", func(t *testing.T) {
		output.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/social/v1/accounts?provider=tiktok_direct&ticket=t-secret&CODE=c-secret&State=s-secret&access_token=at-secret&Refresh_Token=rt-secret&id_token=id-secret&refresh=1",
			nil,
		)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]string{
			"provider":      "tiktok_direct",
			"ticket":        "t-secret",
			"CODE":          "c-secret",
			"State":         "s-secret",
			"access_token":  "at-secret",
			"Refresh_Token": "rt-secret",
			"id_token":      "id-secret",
			"refresh":       "1",
		}, gotSensitive)

		logLine := output.String()
		assert.Contains(
			t,
			logLine,
			"GET /api/social/v1/accounts?provider=tiktok_direct&ticket=[REDACTED]&CODE=[REDACTED]&State=[REDACTED]&access_token=[REDACTED]&Refresh_Token=[REDACTED]&id_token=[REDACTED]&refresh=1\n",
		)
		for _, leaked := range []string{"t-secret", "c-secret", "s-secret", "at-secret", "rt-secret", "id-secret"} {
			assert.NotContains(t, logLine, leaked)
		}
	})

	t.Run("encoded question mark in path does not bypass sensitive query redaction", func(t *testing.T) {
		output.Reset()
		var gotTicket string
		router.GET("/api/other/:item", func(c *gin.Context) {
			gotTicket = c.Query("ticket")
			c.Status(http.StatusOK)
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/other/foo%3Fbar?ticket=encoded-path-secret&provider=tiktok_direct", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "encoded-path-secret", gotTicket)
		logLine := output.String()
		assert.Contains(t, logLine, "ticket=[REDACTED]&provider=tiktok_direct\n")
		assert.NotContains(t, logLine, "encoded-path-secret")
	})
}

func TestRedactAccessLogPathEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "existing /api/oauth/ strips query",
			in:   "/api/oauth/github?code=abc&state=xyz",
			want: "/api/oauth/github",
		},
		{
			name: "existing /oauth/ strips query",
			in:   "/oauth/oidc?code=abc&state=xyz",
			want: "/oauth/oidc",
		},
		{
			name: "social oauth start strips query",
			in:   "/api/social/v1/oauth/tiktok/start?ticket=t1",
			want: "/api/social/v1/oauth/tiktok/start",
		},
		{
			name: "social oauth with duplicate slashes strips query",
			in:   "//api/social/v1/oauth/tiktok/callback?code=c1&state=s1",
			want: "//api/social/v1/oauth/tiktok/callback",
		},
		{
			name: "public media token single segment",
			in:   "/api/social/v1/public/media/tok-abc.123",
			want: "/api/social/v1/public/media/[REDACTED]",
		},
		{
			name: "public media token with trailing slash",
			in:   "/api/social/v1/public/media/tok-abc.123/",
			want: "/api/social/v1/public/media/[REDACTED]",
		},
		{
			name: "public media token with multi-segment and query",
			in:   "/api/social/v1/public/media/tok-abc/extra/seg?foo=bar",
			want: "/api/social/v1/public/media/[REDACTED]",
		},
		{
			name: "public media duplicate slashes",
			in:   "//api/social/v1/public/media//tok-abc/",
			want: "/api/social/v1/public/media/[REDACTED]",
		},
		{
			name: "public media dot segment traversal containing token",
			in:   "/api/social/v1/public/media/tok-abc/..",
			want: "/api/social/v1/public/media/[REDACTED]",
		},
		{
			name: "public media dot segment resolving into prefix",
			in:   "/api/social/v1/other/../public/media/tok-abc?x=1",
			want: "/api/social/v1/public/media/[REDACTED]",
		},
		{
			name: "public media empty token segment strips query",
			in:   "/api/social/v1/public/media/?foo=bar",
			want: "/api/social/v1/public/media/",
		},
		{
			name: "unrelated inspiration public media untouched",
			in:   "/api/inspiration/v1/public/media/r2/cover.jpg?w=320",
			want: "/api/inspiration/v1/public/media/r2/cover.jpg?w=320",
		},
		{
			name: "percent-encoded sensitive query key redacted",
			in:   "/api/other?%74icket=sec1&access%5Ftoken=sec2&provider=tiktok_direct",
			want: "/api/other?%74icket=[REDACTED]&access%5Ftoken=[REDACTED]&provider=tiktok_direct",
		},
		{
			name: "base64 padding in sensitive query value fully redacted",
			in:   "/api/other?access_token=abc==&refresh=1",
			want: "/api/other?access_token=[REDACTED]&refresh=1",
		},
		{
			name: "repeated mixed-case sensitive query keys redacted",
			in:   "/api/other?code=first&CODE=second&page=3",
			want: "/api/other?code=[REDACTED]&CODE=[REDACTED]&page=3",
		},
		{
			name: "semicolon delimited sensitive param redacted",
			in:   "/api/other?provider=tiktok_direct;access_token=secret-val",
			want: "/api/other?provider=tiktok_direct;access_token=[REDACTED]",
		},
		{
			name: "valueless sensitive key kept without adding value",
			in:   "/api/other?ticket&provider=tiktok_direct",
			want: "/api/other?ticket&provider=tiktok_direct",
		},
		{
			name: "benign query unchanged",
			in:   "/api/social/v1/posts/42?provider=tiktok_direct&refresh=1&page=1&page_size=20",
			want: "/api/social/v1/posts/42?provider=tiktok_direct&refresh=1&page=1&page_size=20",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redactAccessLogPath(tc.in))
		})
	}
}
