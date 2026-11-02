package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func withInspirationClient(t *testing.T, c *inspirationClient) {
	t.Helper()
	prev := setInspirationClientForTest(c)
	t.Cleanup(func() { setInspirationClientForTest(prev) })
}

func inspirationRecorder(t *testing.T, method, path, body string, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	ctx.Request = httptest.NewRequest(method, path, rdr)
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set(common.RequestIdKey, "req-test")
	h(ctx)
	return rec
}

func TestLoadInspirationConfigDefaultsOff(t *testing.T) {
	t.Setenv("INSPIRATION_ENABLED", "")
	t.Setenv("INSPIRATION_URL", "")
	t.Setenv("INSPIRATION_TOKEN", "")
	t.Setenv("INSPIRATION_TIMEOUT", "")
	t.Setenv("INSPIRATION_MEDIA_TIMEOUT", "")
	cfg := loadInspirationConfig()
	assert.False(t, cfg.Enabled)
	assert.False(t, cfg.configured())
	assert.Equal(t, 5*time.Second, cfg.Timeout)
	assert.Equal(t, 30*time.Second, cfg.MediaTimeout)
}

func TestInspirationStatusDoesNotLeakToken(t *testing.T) {
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: false,
		BaseURL: "http://inspiration:8210",
		Token:   "super-secret-token",
	}))
	rec := inspirationRecorder(t, http.MethodGet, "/api/inspiration/v1/status", "", InspirationStatus)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "super-secret-token")
	assert.NotContains(t, body, "inspiration:8210")
	var payload struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.Equal(t, false, payload.Data["enabled"])
	assert.Equal(t, true, payload.Data["configured"])
	assert.Equal(t, false, payload.Data["gateway_ready"])
}

func TestInspirationDisabledDoesNotHitNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: false,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodGet, "/api/inspiration/v1/inspirations", "", InspirationList)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "INSPIRATION_DISABLED")
	assert.Equal(t, int32(0), hits.Load())
}

func TestInspirationNotConfiguredDoesNotHitNetwork(t *testing.T) {
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodGet, "/api/inspiration/v1/inspirations", "", InspirationList)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "INSPIRATION_NOT_CONFIGURED")
}

func TestInspirationListInjectsBearerAndRewritesMedia(t *testing.T) {
	var gotAuth, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"cover_key":"/media/covers/a.jpg","media_keys":["/media/v/a.mp4"]}]}}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "sidecar-token",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodGet, "/api/inspiration/v1/inspirations?q=%E9%92%A9%E5%AD%90&sort=hot", "", InspirationList)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Bearer sidecar-token", gotAuth)
	assert.Equal(t, "/v1/inspirations", gotPath)
	values := mustParseQuery(t, gotQuery)
	assert.Equal(t, "钩子", values.Get("q"))
	assert.Equal(t, "hot", values.Get("sort"))
	assert.Contains(t, rec.Body.String(), `/api/inspiration/v1/media/covers/a.jpg`)
	assert.Contains(t, rec.Body.String(), `/api/inspiration/v1/media/v/a.mp4`)
	assert.NotContains(t, rec.Body.String(), `"/media/`)
}

func TestInspirationPassthroughConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"success":false,"code":"DUPLICATE_URL","message":"url exists"}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodPost, "/api/inspiration/v1/inspirations", `{"source_url":"https://x"}`, InspirationCreate)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "DUPLICATE_URL")
}

func TestInspirationUpstream5xxBecomes503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"success":false,"code":"INTERNAL"}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodGet, "/api/inspiration/v1/tags", "", InspirationListTags)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "INSPIRATION_UPSTREAM")
}

func TestInspirationGetRetriesOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":1}}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/inspiration/v1/inspirations/1", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	InspirationGet(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(2), hits.Load())
}

func TestInspirationPostDoesNotRetry(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodPost, "/api/inspiration/v1/inspirations", `{"source_url":"https://x"}`, InspirationCreate)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, int32(1), hits.Load())
}

func TestInspirationQueryMapsBodyToUpstreamQuery(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"total":0,"items":[]}}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodPost, "/api/inspiration/v1/query", `{"type":"video","q":"钩子","page":1,"is_favorite":true}`, InspirationQuery)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/v1/inspirations", gotPath)
	values := mustParseQuery(t, gotQuery)
	assert.Equal(t, "video", values.Get("type"))
	assert.Equal(t, "钩子", values.Get("q"))
	assert.Equal(t, "1", values.Get("page"))
	assert.Equal(t, "true", values.Get("is_favorite"))
}

func TestInspirationBulkMapsColonPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"created":1}}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: true,
		BaseURL: srv.URL,
		Token:   "tok",
		Timeout: 2 * time.Second,
	}))
	rec := inspirationRecorder(t, http.MethodPost, "/api/inspiration/v1/inspirations/bulk", `{"items":[]}`, InspirationBulk)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/v1/inspirations:bulk", gotPath)
}

func TestInspirationMediaForwardsRangeAndRejectsTraversal(t *testing.T) {
	var gotRange, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("abcd"))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled:      true,
		BaseURL:      srv.URL,
		Token:        "tok",
		Timeout:      2 * time.Second,
		MediaTimeout: 5 * time.Second,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/inspiration/v1/media/v/a.mp4", nil)
	ctx.Request.Header.Set("Range", "bytes=0-3")
	ctx.Params = gin.Params{{Key: "key", Value: "/v/a.mp4"}}
	InspirationMedia(ctx)
	assert.Equal(t, http.StatusPartialContent, rec.Code)
	assert.Equal(t, "bytes=0-3", gotRange)
	assert.Equal(t, "/media/v/a.mp4", gotPath)
	assert.Equal(t, "abcd", rec.Body.String())
	assert.Equal(t, "video/mp4", rec.Header().Get("Content-Type"))

	bad := httptest.NewRecorder()
	badCtx, _ := gin.CreateTestContext(bad)
	badCtx.Request = httptest.NewRequest(http.MethodGet, "/api/inspiration/v1/media/../etc/passwd", nil)
	badCtx.Params = gin.Params{{Key: "key", Value: "/../etc/passwd"}}
	InspirationMedia(badCtx)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Contains(t, bad.Body.String(), "INVALID_PARAM")
}

func TestInspirationUploadMedia(t *testing.T) {
	var gotAuth, gotPath, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"data":{"key":"covers/2026/08/x.jpg","url":"/media/covers/2026/08/x.jpg"}}`))
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled:      true,
		BaseURL:      srv.URL,
		Token:        "sidecar-tok",
		Timeout:      2 * time.Second,
		MediaTimeout: 5 * time.Second,
	}))

	rec := inspirationRecorder(t, http.MethodPost, "/api/inspiration/v1/media", `{"url":"https://example.com/cover.jpg"}`, InspirationUploadMedia)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "Bearer sidecar-tok", gotAuth)
	assert.Equal(t, "/v1/media", gotPath)
	assert.Equal(t, "application/json", gotType)
	assert.Contains(t, rec.Body.String(), `/api/inspiration/v1/media/covers/2026/08/x.jpg`)
}

func TestValidInspirationMediaKey(t *testing.T) {
	assert.True(t, validInspirationMediaKey("covers/a.jpg"))
	assert.False(t, validInspirationMediaKey(""))
	assert.False(t, validInspirationMediaKey("../x"))
	assert.False(t, validInspirationMediaKey("/etc/passwd"))
	assert.False(t, validInspirationMediaKey(`a\b`))
}

func mustParseQuery(t *testing.T, raw string) mapGetter {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://x?"+raw, nil)
	require.NoError(t, err)
	return req.URL.Query()
}

type mapGetter interface {
	Get(string) string
}
