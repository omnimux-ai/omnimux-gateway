package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
