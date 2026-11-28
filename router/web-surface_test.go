package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func performHostRequest(handler http.Handler, method, path, host string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(""))
	request.Host = host
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestDesktopSurfaceShellMarksSurfaceAndKeepsRobots(t *testing.T) {
	page := []byte(`<!doctype html><html lang="en" class="dark"><head><meta name="robots" content="index,follow" /></head><body></body></html>`)
	shell := string(desktopSurfaceShell(page))
	assert.Contains(t, shell, `data-surface="desktop"`)
	assert.Contains(t, shell, `data-pricing-group="desktop-720p"`)
	// The homepage stays indexable: only the surface marker is injected.
	assert.Contains(t, shell, `<meta name="robots" content="index,follow" />`)
}

func TestWebSearchPagesHomeIsTheDesktopSurfaceOnEveryHost(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)

	outer := gin.New()
	outer.NoRoute(pages.serve)

	for _, host := range []string{"omnimux.ai", "www.omnimux.ai", "d.omnimux.ai"} {
		t.Run(host, func(t *testing.T) {
			response := performHostRequest(outer, http.MethodGet, "/", host)
			require.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Body.String(), `data-surface="desktop"`)
			assert.Contains(t, response.Body.String(), `data-pricing-group="desktop-720p"`)
			assert.Contains(t, response.Body.String(), `rel="canonical" href="https://omnimux.ai/"`)
			assert.Empty(t, response.Header().Get("X-Robots-Tag"))
		})
	}
}

func TestWebSearchPagesPublicPricingStaysIndexable(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)

	outer := gin.New()
	outer.NoRoute(pages.serve)

	response := performHostRequest(outer, http.MethodGet, "/pricing", "omnimux.ai")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `data-surface="desktop"`)
	assert.Empty(t, response.Header().Get("X-Robots-Tag"))
	assert.Contains(t, response.Body.String(), `rel="canonical" href="https://omnimux.ai/pricing"`)
}

func TestWebSearchPagesPrivateShellIsNoindex(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)

	outer := gin.New()
	outer.NoRoute(pages.serve)

	response := performHostRequest(outer, http.MethodGet, "/dashboard", "omnimux.ai")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `data-surface="desktop"`)
	assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
}

func TestInstallScriptsRedirectToGatewayRawWhenEmbeddedMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	outer := gin.New()
	SetWebRouter(outer, WebAssets{IndexPage: []byte(searchPageTestShell)}, func(c *gin.Context) { c.Next() })

	cases := map[string]string{
		"/install.sh":  "https://raw.githubusercontent.com/omnimux-ai/omnimux-gateway/main/web/public/install.sh",
		"/install.ps1": "https://raw.githubusercontent.com/omnimux-ai/omnimux-gateway/main/web/public/install.ps1",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			response := performHostRequest(outer, http.MethodGet, path, "omnimux.ai")
			require.Equal(t, http.StatusFound, response.Code)
			assert.Equal(t, want, response.Header().Get("Location"))
		})
	}
}

func TestWebSearchPagesAPISubdomainIsNoindex(t *testing.T) {
	previous := inspirationShareLookup
	t.Cleanup(func() { inspirationShareLookup = previous })
	inspirationShareLookup = func(shareId string) (*model.InspirationShare, error) {
		if shareId == "insp_permanent01" {
			return &model.InspirationShare{
				ShareId:     "insp_permanent01",
				Title:       "Vintage poster character",
				Description: "A vintage poster look recreated with an OmniMux character",
				CoverUrl:    "https://files.omnimux.ai/cover.jpg",
			}, nil
		}
		return nil, nil
	}

	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)

	outer := gin.New()
	outer.NoRoute(pages.serve)

	for _, host := range []string{"api.omnimux.ai", "api.omnimux.ai:443"} {
		for _, path := range []string{"/", "/pricing", "/privacy", "/ai-influencer-studio", "/guides", "/s/insp_permanent01"} {
			t.Run(host+path, func(t *testing.T) {
				response := performHostRequest(outer, http.MethodGet, path, host)
				require.Equal(t, http.StatusOK, response.Code)
				assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
				assert.Equal(t, middleware.PrivateHTMLCacheControl, response.Header().Get("Cache-Control"))
				assert.Contains(t, response.Body.String(), `<link rel="canonical" href="https://omnimux.ai`)
			})
		}
	}

	primary := performHostRequest(outer, http.MethodGet, "/", "omnimux.ai")
	require.Equal(t, http.StatusOK, primary.Code)
	assert.Empty(t, primary.Header().Get("X-Robots-Tag"))
}

func TestWebRouterHidesSitemapOnAPISubdomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	outer := gin.New()
	SetWebRouter(outer, WebAssets{IndexPage: []byte(searchPageTestShell)}, func(c *gin.Context) { c.Next() })

	apiSitemap := performHostRequest(outer, http.MethodGet, "/sitemap.xml", "api.omnimux.ai")
	assert.Equal(t, http.StatusNotFound, apiSitemap.Code)
	assert.Equal(t, "noindex, follow", apiSitemap.Header().Get("X-Robots-Tag"))
	assert.Equal(t, "no-cache", apiSitemap.Header().Get("Cache-Control"))

	// robots.txt on api.omnimux.ai is not aborted by the sitemap guard (with empty BuildFS here it falls through to servePage).
	apiRobots := performHostRequest(outer, http.MethodGet, "/robots.txt", "api.omnimux.ai")
	assert.Equal(t, http.StatusOK, apiRobots.Code)

	// Primary site /sitemap.xml is not aborted by the API host guard.
	webSitemap := performHostRequest(outer, http.MethodGet, "/sitemap.xml", "omnimux.ai")
	assert.Equal(t, http.StatusOK, webSitemap.Code)
}
