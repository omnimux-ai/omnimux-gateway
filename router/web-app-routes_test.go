package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const appRouteTestManifest = `{"version":1,"paths":["/","/pricing","/influencer","/dashboard","/sign-in","/oauth","/oauth/$provider","/pricing/$modelId","/s/$shareId","/cli/login","/inspiration/models/$modelSlug"],"legacy":{"exactPaths":["/login","/forbidden","/console"],"prefixPaths":["/console/"]}}`

func TestWebAppRoutesPreserveEffectiveAndLegacyPaths(t *testing.T) {
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	for _, path := range []string{
		"/", "/influencer", "/INFLUENCER/", "/dashboard///", "/sign-in", "/oauth", "/oauth/google", "/oauth/custom-provider/",
		"/pricing/seedance-2.5", "/pricing/vendor%2Fmodel", "/s/share-123", "/cli/login", "/inspiration/models/image-model",
		"/login", "/login/", "/forbidden", "/console", "/console/chat/old/thread", "/console/unknown-historical-entry",
	} {
		t.Run(path, func(t *testing.T) { assert.True(t, routes.matches(path)) })
	}
	for _, path := range []string{
		"/not-a-page", "/LOGIN", "/console2/old", "/CONSOLE/old", "/s", "/s//", "/s/id/extra", "/oauth/google/extra",
		"/inspiration/models/id/extra", "/pricing//model", "/static/js/missing.js", "/privacy-policy-missing",
		"/%69nfluencer", "/%70ricing",
	} {
		t.Run(path, func(t *testing.T) { assert.False(t, routes.matches(path)) })
	}
}

func TestWebAppRoutesUnknownSafeRequestsAre404(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(routes.serve(pages))
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{"/does-not-exist?source=search", "/unknown.js", "/s/id/extra", "/%70ricing"} {
			t.Run(method+path, func(t *testing.T) {
				response := performPluginRequest(outer, method, path)
				assert.Equal(t, http.StatusNotFound, response.Code)
				assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
				assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
				if method == http.MethodHead {
					assert.Empty(t, response.Body.String())
				} else {
					assert.Contains(t, response.Body.String(), "Workspace | OmniMux")
					assert.NotContains(t, response.Body.String(), "canonical")
				}
			})
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodOptions} {
		response := performPluginRequest(outer, method, "/does-not-exist")
		assert.Equal(t, http.StatusOK, response.Code, "preserve existing behavior for %s", method)
	}
	for _, path := range []string{"/influencer", "/oauth/github?code=callback&state=state", "/console/legacy", "/pricing/vendor%2Fmodel"} {
		response := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, http.StatusOK, response.Code, path)
		assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"), path)
	}
	response := performPluginRequest(outer, http.MethodGet, "/privacy")
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("X-Robots-Tag"))
	response = performPluginRequest(outer, http.MethodGet, "/pricing/?source=search")
	assert.Equal(t, http.StatusMovedPermanently, response.Code)
	assert.Equal(t, "/pricing?source=search", response.Header().Get("Location"))
}

func TestWebAppRoutesInvalidManifestRetainsNoindexFallback(t *testing.T) {
	for name, data := range map[string]string{
		"missing": "", "invalid JSON": "{", "empty": "{}", "wrong version": `{"version":2,"paths":["/"]}`,
		"unsupported shape": `{"version":1,"paths":["/","/s/$"]}`,
		"missing root":      `{"version":1,"paths":["/influencer"]}`,
		"missing legacy":    `{"version":1,"paths":["/"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			routes, err := buildWebAppRoutes([]byte(data))
			require.Error(t, err)
			pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
			require.NoError(t, err)
			outer := gin.New()
			outer.NoRoute(routes.serve(pages))
			response := performPluginRequest(outer, http.MethodGet, "/unknown")
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
			assert.Contains(t, response.Body.String(), "Workspace | OmniMux")
		})
	}
}

func TestWebAppRoutesDoNotOverridePluginOrRegisteredResponses(t *testing.T) {
	plugin := compileRouterPlugin(t, "unknown-page-owner", "1.0.0", `[{method:"GET",path:"/vendor/custom-page",type:"dynamic"}]`)
	outer, registry := newPluginRouterTest(t, []*jsplugin.LoadedPlugin{plugin}, testPluginRouteHandlers(
		func(c *gin.Context, _ *jsplugin.RoutingGeneration, _ jsplugin.RouteBinding) {
			c.String(http.StatusAccepted, "plugin result")
		},
	))
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	outer.GET("/api/status", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	outer.GET("/assets/existing.js", func(c *gin.Context) { c.String(http.StatusOK, "script") })
	outer.NoRoute((&pluginRouteDispatcher{registry: registry}).dispatch, routes.serve(pages))
	for _, test := range []struct {
		path string
		code int
		body string
	}{
		{"/vendor/custom-page", http.StatusAccepted, "plugin result"},
		{"/api/status", http.StatusNoContent, ""},
		{"/assets/existing.js", http.StatusOK, "script"},
	} {
		response := performPluginRequest(outer, http.MethodGet, test.path)
		assert.Equal(t, test.code, response.Code)
		assert.Equal(t, test.body, response.Body.String())
		assert.Empty(t, response.Header().Get("X-Robots-Tag"))
	}
}

func TestWebAppRouteManifestRejectsDamagedLegacyPolicy(t *testing.T) {
	var manifest webAppRouteManifest
	require.NoError(t, common.Unmarshal([]byte(appRouteTestManifest), &manifest))
	for _, prefix := range []string{"/", "/console", "/console/$rest/"} {
		manifest.Legacy.PrefixPaths = []string{prefix}
		data, err := common.Marshal(manifest)
		require.NoError(t, err)
		_, err = buildWebAppRoutes(data)
		assert.Error(t, err, prefix)
	}
}

// Exercise the real request URL parser: an escaped slash belongs to one path
// parameter even though Request.URL.Path contains its decoded slash.
func TestWebAppRoutesEncodedDynamicParameter(t *testing.T) {
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "/pricing/vendor%2Fmodel", nil)
	assert.True(t, routes.matches(request.URL.EscapedPath()))
	assert.False(t, routes.matches(request.URL.Path))
}
