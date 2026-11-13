package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// upstreamFixture is a page shaped exactly like the sidecar's /v1/inspirations
// response, including the fields the public projection must not publish.
const upstreamFixture = `{"success":true,"data":{"total":42,"page":1,"size":30,"items":[
 {"id":2789,"type":"video","title":"sleep aid","content":"Give it a try",
  "source_url":"https://www.tiktok.com/@creator/video/gxgen-aa5d20e5-729e","cover_key":"/media/inspiration-covers/2789",
  "media_keys":["/media/r2/publications/x/clip.mp4","https://cdn.example.com/direct.mp4"],
  "hot_score":95600000,"country_code":"de","category":"Health \u0026 Wellness","duration":24,"views":95600000,
  "traffic_type":"organic","posted_at":"2025-08-08T16:00:00Z","is_favorite":true,
  "analysis":{"creator":{"name":"charlottegreat2000","handle":"charlottegreat2000"},"target_goal":"internal narrative"}},
 {"id":2790,"type":"image","title":"unclassified","content":"",
  "source_url":"https://www.tiktok.com/@other/video/7590000000000000000","cover_key":"","media_keys":[],
  "country_code":"","category":"","duration":0,"views":120,"traffic_type":"ad","posted_at":null,"analysis":null}
]}}`

type publicEnvelope struct {
	Success bool                  `json:"success"`
	Data    inspirationPublicPage `json:"data"`
}

func publicUpstream(t *testing.T, handler http.HandlerFunc) *inspirationClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := newInspirationClient(inspirationConfig{Enabled: true, BaseURL: srv.URL, Token: "super-secret-token"})
	withInspirationClient(t, client)
	return client
}

func callPublicList(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	return inspirationRecorder(t, http.MethodGet, "/api/inspiration/v1/public/inspirations"+query, "", InspirationPublicList)
}

func TestInspirationPublicListProjectsAllowlistedFields(t *testing.T) {
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamFixture))
	})

	rec := callPublicList(t, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=120", rec.Header().Get("Cache-Control"))

	body := rec.Body.String()
	assert.NotContains(t, body, "super-secret-token", "the sidecar token must never reach an anonymous caller")
	assert.NotContains(t, body, "target_goal", "internal analysis must not be projected")
	assert.NotContains(t, body, "hot_score")
	assert.NotContains(t, body, "is_favorite")
	assert.NotContains(t, body, "cdn.example.com", "absolute media URLs are withheld")

	var payload publicEnvelope
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, int64(42), payload.Data.Total)
	assert.Equal(t, 2, payload.Data.PageCount, "42 rows over 30 per page")
	require.Len(t, payload.Data.Items, 2)

	video := payload.Data.Items[0]
	assert.Equal(t, "video", video.Type)
	assert.Equal(t, "DE", video.Country, "region codes are normalised to upper case")
	assert.Equal(t, 24, video.Duration)
	assert.Equal(t, int64(95600000), video.Views)
	assert.Equal(t, "organic", video.TrafficType)
	require.NotNil(t, video.PostedAt)
	assert.Equal(t, "2025-08-08T16:00:00Z", *video.PostedAt)
	require.NotNil(t, video.CoverURL)
	assert.Equal(t, "/api/inspiration/v1/public/media/inspiration-covers/2789", *video.CoverURL)
	require.Len(t, video.MediaURLs, 1, "the absolute media URL is dropped, the object key becomes a public path")
	assert.Equal(t, "/api/inspiration/v1/public/media/r2/publications/x/clip.mp4", video.MediaURLs[0])
	assert.Nil(t, video.SourceURL, "internal asset ids make the imported source URL unusable")
	require.NotNil(t, video.Creator)
	assert.Equal(t, "charlottegreat2000", video.Creator.Handle)

	image := payload.Data.Items[1]
	assert.Equal(t, "", image.Country)
	assert.Equal(t, 0, image.Duration)
	assert.Nil(t, image.PostedAt)
	assert.Nil(t, image.CoverURL)
	assert.Empty(t, image.MediaURLs)
	assert.Nil(t, image.Creator)
	require.NotNil(t, image.SourceURL, "a real post URL is published")
	assert.Equal(t, "https://www.tiktok.com/@other/video/7590000000000000000", *image.SourceURL)
}

func TestInspirationPublicListForwardsOnlyAllowlistedParams(t *testing.T) {
	var gotQuery atomic.Value
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery.Store(r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"total":0,"page":2,"size":10,"items":[]}}`))
	})

	rec := callPublicList(t, "?page=2&page_size=10&country=us&type=video&traffic_type=ad&sort=views"+
		"&duration_min=15&duration_max=30&views_min=1000&posted_after=2026-01-01&posted_before=2026-06-30"+
		"&is_favorite=true&q=secret&tags=internal&api_key=leak")
	require.Equal(t, http.StatusOK, rec.Code)

	forwarded := gotQuery.Load().(string)
	for _, want := range []string{"page=2", "page_size=10", "country=US", "type=video", "traffic_type=ad",
		"sort=views", "duration_min=15", "duration_max=30", "views_min=1000", "posted_after=2026-01-01", "posted_before=2026-06-30"} {
		assert.Contains(t, forwarded, want)
	}
	for _, leak := range []string{"is_favorite", "q=secret", "tags=internal", "api_key"} {
		assert.NotContains(t, forwarded, leak, "the public endpoint never forwards unknown parameters")
	}
}

func TestInspirationPublicListRejectsInvalidParams(t *testing.T) {
	var calls int64
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		_, _ = w.Write([]byte(`{"success":true,"data":{"total":0,"page":1,"size":30,"items":[]}}`))
	})

	cases := []string{
		"?page=0",
		"?page=abc",
		"?page_size=101",
		"?country=USA",
		"?country=1A",
		"?type=audio",
		"?traffic_type=paid",
		"?sort=revenue",
		"?duration_min=-1",
		"?views_max=1e9",
		"?posted_after=2026/01/01",
		"?posted_before=2026-13-01",
	}
	for _, query := range cases {
		rec := callPublicList(t, query)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "query %s must be rejected", query)
		assert.Contains(t, rec.Body.String(), "INVALID_PARAM", "query %s", query)
	}
	assert.Zero(t, atomic.LoadInt64(&calls), "invalid parameters never reach the sidecar")
}

func TestInspirationPublicListFailsClosedWhenDisabled(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
	}))
	t.Cleanup(srv.Close)
	withInspirationClient(t, newInspirationClient(inspirationConfig{
		Enabled: false,
		BaseURL: srv.URL,
		Token:   "super-secret-token",
	}))

	rec := callPublicList(t, "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "INSPIRATION_DISABLED")
	assert.Zero(t, atomic.LoadInt64(&calls))
}

func TestInspirationPublicListReportsUpstreamFailure(t *testing.T) {
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	rec := callPublicList(t, "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "INSPIRATION_UPSTREAM")
}

func publicMediaRecorder(t *testing.T, key string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/inspiration/v1/public/media"+key, nil)
	ctx.Params = gin.Params{{Key: "key", Value: key}}
	InspirationPublicMedia(ctx)
	return rec
}

func TestInspirationPublicMediaRejectsTraversalKeys(t *testing.T) {
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a traversal key must not reach the sidecar: %s", r.URL.Path)
	})

	for _, key := range []string{"/../secrets", "//etc/passwd", "/a/../b"} {
		rec := publicMediaRecorder(t, key)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "key %s", key)
	}
}

func TestInspirationPublicMediaStreamsUpstreamBody(t *testing.T) {
	publicUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/media/inspiration-covers/2789", r.URL.Path)
		assert.Equal(t, "Bearer super-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("jpeg-bytes"))
	})

	rec := publicMediaRecorder(t, "/inspiration-covers/2789")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.Equal(t, "jpeg-bytes", rec.Body.String())
	assert.False(t, strings.Contains(rec.Body.String(), "super-secret-token"))
}
