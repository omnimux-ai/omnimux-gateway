package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Keep all router package tests isolated from live network calls.
	inspirationCatalogAllowDefaultSiteFetch = false
}

const inspirationTestManifestJSON = `{
  "version": 1,
  "generatedAt": "2026-10-01T15:07:07.254Z",
  "pageSize": 48,
  "total": 31440,
  "zones": [
    {
      "id": "seedance-2.5",
      "label": "Seedance 2.5",
      "mediaType": "video",
      "count": 1933,
      "quarantined": 0,
      "categories": ["UGC Ads", "Product Demo", "Brand Story"]
    },
    {
      "id": "gpt-image-2",
      "label": "GPT Image 2",
      "mediaType": "image",
      "count": 29507,
      "quarantined": 0,
      "categories": ["Ecommerce Product", "Poster"]
    },
    {
      "id": "../bad-slug",
      "label": "Bad Slug",
      "mediaType": "video",
      "count": 10,
      "quarantined": 0
    },
    {
      "id": "empty-zone",
      "label": "Empty Zone",
      "mediaType": "video",
      "count": 0,
      "quarantined": 2
    }
  ]
}`

func withStubbedInspirationManifest(t *testing.T, fn func(siteURL string) ([]byte, error)) {
	t.Helper()
	prev := inspirationCatalogFetch
	t.Cleanup(func() { inspirationCatalogFetch = prev })
	inspirationCatalogFetch = fn
}

func TestInspirationModelPagesRoutingMetadata404AndQuarantine(t *testing.T) {
	withStubbedInspirationManifest(t, func(string) ([]byte, error) {
		return []byte(inspirationTestManifestJSON), nil
	})
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(routes.serve(pages))

	// Valid model slug renders indexable dynamic head + noscript summary.
	resp := performHostRequest(outer, http.MethodGet, "/inspiration/models/seedance-2.5", "omnimux.ai")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Empty(t, resp.Header().Get("X-Robots-Tag"))
	assert.Equal(t, middleware.PublicHTMLCacheControl, resp.Header().Get("Cache-Control"))
	body := resp.Body.String()
	titleMatch := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(body)
	require.Len(t, titleMatch, 2)
	assert.LessOrEqual(t, len(titleMatch[1]), 60)
	assert.Contains(t, titleMatch[1], "Seedance 2.5 Prompts (1,933+)")
	descMatch := regexp.MustCompile(`<meta name="description" content="([^"]+)"`).FindStringSubmatch(body)
	require.Len(t, descMatch, 2)
	assert.LessOrEqual(t, len(descMatch[1]), 160)
	assert.Contains(t, body, `<meta name="omnimux-server-head-path" content="/inspiration/models/seedance-2.5" />`)
	assert.Contains(t, body, `<link rel="canonical" href="https://omnimux.ai/inspiration/models/seedance-2.5" />`)
	assert.Contains(t, body, `<meta name="robots" content="index,follow" />`)
	assert.Contains(t, body, `"@type":"CollectionPage"`)
	assert.Contains(t, body, `"@type":"BreadcrumbList"`)
	assert.Contains(t, body, `<noscript><div id="geo-ssr">`)
	assert.Contains(t, body, `UGC Ads`)
	assert.NotContains(t, body, " · ")

	// Trailing slash and mixed-case slugs redirect 301 preserving query.
	trailing := performHostRequest(outer, http.MethodGet, "/inspiration/models/seedance-2.5/?utm=1", "omnimux.ai")
	assert.Equal(t, http.StatusMovedPermanently, trailing.Code)
	assert.Equal(t, "/inspiration/models/seedance-2.5?utm=1", trailing.Header().Get("Location"))

	mixedCase := performHostRequest(outer, http.MethodGet, "/inspiration/models/Seedance-2.5?utm=1", "omnimux.ai")
	assert.Equal(t, http.StatusMovedPermanently, mixedCase.Code)
	assert.Equal(t, "/inspiration/models/seedance-2.5?utm=1", mixedCase.Header().Get("Location"))

	// Unknown or empty slugs return real 404 (including GET, HEAD, and ?quarantine=1).
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/inspiration/models/does-not-exist"},
		{http.MethodGet, "/inspiration/models/does-not-exist?quarantine=1"},
		{http.MethodGet, "/inspiration/models/empty-zone"},
		{http.MethodGet, "/inspiration/models/"},
		{http.MethodHead, "/inspiration/models/does-not-exist"},
	} {
		miss := performHostRequest(outer, tc.method, tc.path, "omnimux.ai")
		assert.Equal(t, http.StatusNotFound, miss.Code, tc.method+" "+tc.path)
		assert.Equal(t, "noindex, follow", miss.Header().Get("X-Robots-Tag"), tc.method+" "+tc.path)
		assert.NotContains(t, miss.Body.String(), "canonical", tc.method+" "+tc.path)
		if tc.method == http.MethodHead {
			assert.Empty(t, miss.Body.String())
		}
	}

	// Quarantine view stays 200 OK with noindex and no dynamic canonical/server-head marker.
	for _, query := range []string{"?quarantine=1", "?quarantine=true"} {
		q := performHostRequest(outer, http.MethodGet, "/inspiration/models/seedance-2.5"+query, "omnimux.ai")
		assert.Equal(t, http.StatusOK, q.Code, query)
		assert.Equal(t, "noindex, follow", q.Header().Get("X-Robots-Tag"), query)
		assert.Equal(t, middleware.PrivateHTMLCacheControl, q.Header().Get("Cache-Control"), query)
		assert.NotContains(t, q.Body.String(), "omnimux-server-head-path", query)
		assert.NotContains(t, q.Body.String(), "canonical", query)
	}

	// api. subdomain never indexes even valid model slugs.
	apiResp := performHostRequest(outer, http.MethodGet, "/inspiration/models/gpt-image-2", "api.omnimux.ai")
	assert.Equal(t, http.StatusOK, apiResp.Code)
	assert.Equal(t, "noindex, follow", apiResp.Header().Get("X-Robots-Tag"))
	assert.Equal(t, middleware.PrivateHTMLCacheControl, apiResp.Header().Get("Cache-Control"))
}

func TestInspirationCatalogFailOpenStaleCacheAndSources(t *testing.T) {
	// Unavailable manifest fails open (200 OK noindex shell) so outages do not emit 404s.
	withStubbedInspirationManifest(t, func(string) ([]byte, error) {
		return nil, errors.New("upstream unavailable")
	})
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	outage := performHostRequest(outer, http.MethodGet, "/inspiration/models/does-not-exist", "omnimux.ai")
	assert.Equal(t, http.StatusOK, outage.Code)
	assert.Equal(t, "noindex, follow", outage.Header().Get("X-Robots-Tag"))
	assert.Contains(t, outage.Body.String(), "<title>Workspace | OmniMux</title>")

	// Refresh failure keeps last-known-good snapshot after TTL expiry.
	now := time.Unix(1_700_000_000, 0)
	fetches := 0
	store := &inspirationCatalogStore{
		now: func() time.Time { return now },
		fetch: func(string) ([]byte, error) {
			fetches++
			if fetches == 1 {
				return []byte(inspirationTestManifestJSON), nil
			}
			return nil, errors.New("refresh failed")
		},
	}
	first := store.get("https://omnimux.ai")
	require.NotNil(t, first)
	require.Len(t, first.zones, 2)
	_, rejectedInvalid := first.byID["../bad-slug"]
	assert.False(t, rejectedInvalid)
	_, rejectedEmpty := first.byID["empty-zone"]
	assert.False(t, rejectedEmpty)

	now = now.Add(inspirationCatalogTTL + time.Second)
	stale := store.get("https://omnimux.ai")
	require.NotNil(t, stale)
	assert.Len(t, stale.zones, 2)
	assert.Equal(t, 2, fetches)

	// File and HTTP manifest sources load cleanly without relying on package hook.
	withStubbedInspirationManifest(t, nil)
	filePath := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(filePath, []byte(inspirationTestManifestJSON), 0o600))
	t.Setenv("INSPIRATION_CATALOG_MANIFEST_PATH", filePath)
	fileStore := newInspirationCatalogStore()
	fromFile := fileStore.get("https://omnimux.ai")
	require.NotNil(t, fromFile)
	assert.Len(t, fromFile.zones, 2)

	t.Setenv("INSPIRATION_CATALOG_MANIFEST_PATH", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(inspirationTestManifestJSON))
	}))
	defer server.Close()
	t.Setenv("INSPIRATION_CATALOG_MANIFEST_URL", server.URL)
	urlStore := newInspirationCatalogStore()
	fromURL := urlStore.get("https://omnimux.ai")
	require.NotNil(t, fromURL)
	assert.Equal(t, "2026-10-01", fromURL.lastmod)
}

func TestInspirationDynamicSitemapMergeAndMiddleware(t *testing.T) {
	snapshot, err := parseInspirationCatalogManifest([]byte(inspirationTestManifestJSON))
	require.NoError(t, err)

	staticXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://omnimux.ai/</loc><lastmod>2026-11-28</lastmod></url>
  <url><loc>https://omnimux.ai/inspiration/models/seedance-2.5</loc><lastmod>2026-11-28</lastmod></url>
</urlset>
`)
	// Nil snapshot returns static sitemap unchanged.
	assert.Equal(t, string(staticXML), string(renderDynamicSitemap(staticXML, "https://omnimux.ai", nil)))

	merged := string(renderDynamicSitemap(staticXML, "https://omnimux.ai", snapshot))
	assert.Equal(t, 1, strings.Count(merged, "https://omnimux.ai/inspiration/models/seedance-2.5"))
	assert.Contains(t, merged, "<url><loc>https://omnimux.ai/inspiration/models/gpt-image-2</loc><lastmod>2026-10-01</lastmod></url>")
	assert.NotContains(t, merged, "empty-zone")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(merged), "</urlset>"))

	// Exercise serveDynamicSitemap over GET and HEAD.
	engine := gin.New()
	engine.GET("/sitemap.xml", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		serveDynamicSitemap(c, staticXML, "https://omnimux.ai", snapshot)
	})
	engine.HEAD("/sitemap.xml", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		serveDynamicSitemap(c, staticXML, "https://omnimux.ai", snapshot)
	})

	getResp := performPluginRequest(engine, http.MethodGet, "/sitemap.xml")
	require.Equal(t, http.StatusOK, getResp.Code)
	assert.Equal(t, "application/xml; charset=utf-8", getResp.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", getResp.Header().Get("Cache-Control"))
	assert.Contains(t, getResp.Body.String(), "https://omnimux.ai/inspiration/models/gpt-image-2")

	headResp := performPluginRequest(engine, http.MethodHead, "/sitemap.xml")
	require.Equal(t, http.StatusOK, headResp.Code)
	assert.Empty(t, headResp.Body.String())
}
