package router

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWebUmamiInjectionPreservesSingleScriptAndAttribution(t *testing.T) {
	for name, document := range map[string]string{
		"placeholder":           `<html><head><!--umami--></head><body></body></html>`,
		"duplicate placeholder": `<html><head><!--umami--><!--umami--></head><body></body></html>`,
		"no placeholder":        `<html><head></head><body>Older document</body></html>`,
		"legacy fallback":       `legacy dashboard`,
	} {
		t.Run(name, func(t *testing.T) {
			body := injectWebUmami([]byte(document), "website-id", "")
			body = injectWebUmami(body, "website-id", "")
			assert.Equal(t, 1, strings.Count(string(body), `src="https://analytics.umami.is/script.js"`))
			assert.Equal(t, 1, strings.Count(string(body), `data-website-id="website-id"`))
			assert.Equal(t, 1, strings.Count(string(body), "<!--Umami QuantumNous-->"))
			assert.NotContains(t, string(body), "<!--umami-->")
		})
	}
	body := string(injectWebUmami([]byte(`<head><!--umami--></head>`), "", "https://ignored.example/script.js"))
	assert.NotContains(t, body, "<script")
	assert.Contains(t, body, "<!--Umami QuantumNous-->")
}

func TestWebRouterUmamiIncludesManifestFailureAndGuide503(t *testing.T) {
	t.Setenv("UMAMI_WEBSITE_ID", "fallback-website-id")
	t.Setenv("UMAMI_SCRIPT_URL", "https://analytics.example/script.js")
	outer := gin.New()
	SetWebRouter(outer, WebAssets{IndexPage: []byte(searchPageTestShell)}, func(c *gin.Context) { c.Next() })
	for path, status := range map[string]int{"/influencer": http.StatusOK, "/guides": http.StatusServiceUnavailable} {
		response := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, status, response.Code)
		assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
		assert.Equal(t, 1, strings.Count(response.Body.String(), `data-website-id="fallback-website-id"`))
		assert.Equal(t, 1, strings.Count(response.Body.String(), "<!--Umami QuantumNous-->"))
	}
}
