package router

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWebChatwootInjectionAndSafety(t *testing.T) {
	for name, document := range map[string]string{
		"placeholder":           `<html><head><!--chatwoot--></head><body></body></html>`,
		"duplicate placeholder": `<html><head><!--chatwoot--><!--chatwoot--></head><body></body></html>`,
		"static guide no tag":   `<!doctype html><html lang="en"><head><title>Guide</title></head><body><main>hi</main></body></html>`,
		"legacy fallback":       `legacy dashboard`,
	} {
		t.Run(name, func(t *testing.T) {
			body := injectWebChatwoot([]byte(document), "https://support.example.com/", "cw_token_123", "")
			body = injectWebChatwoot(body, "https://support.example.com/", "cw_token_123", "")
			text := string(body)
			assert.Equal(t, 1, strings.Count(text, `id="omnimux-chatwoot-config"`))
			assert.NotContains(t, text, "<!--chatwoot-->")
			assert.Contains(t, text, `"baseUrl":"https://support.example.com"`)
			assert.Contains(t, text, `"websiteToken":"cw_token_123"`)
			assert.Contains(t, text, `"darkMode":"dark"`)
		})
	}

	disabledCases := []struct {
		name  string
		url   string
		token string
	}{
		{"missing url", "", "cw_token"},
		{"missing token", "https://support.example.com", "  "},
		{"javascript scheme", "javascript:alert(1)", "cw_token"},
		{"ftp scheme", "ftp://support.example.com", "cw_token"},
		{"url with query", "https://support.example.com?foo=1", "cw_token"},
		{"url with userinfo", "https://user@support.example.com", "cw_token"},
		{"token with whitespace", "https://support.example.com", "bad token"},
	}
	for _, tc := range disabledCases {
		t.Run(tc.name, func(t *testing.T) {
			body := string(injectWebChatwoot([]byte(`<html><head><!--chatwoot--></head><body></body></html>`), tc.url, tc.token, "dark"))
			assert.NotContains(t, body, `id="omnimux-chatwoot-config"`)
			assert.NotContains(t, body, "<!--chatwoot-->")
			assert.NotContains(t, body, "<script")
		})
	}

	// Verify darkMode normalization and script breakout escaping.
	escapedBody := string(injectWebChatwoot([]byte(`<head><!--chatwoot--></head>`), "https://support.example.com", `tok</script><script>alert(1)`, "auto"))
	assert.Contains(t, escapedBody, `\u003c/script\u003e`)
	assert.Contains(t, escapedBody, `"darkMode":"auto"`)
	assert.Equal(t, 1, strings.Count(escapedBody, `</script>`))

	defaultModeBody := string(injectWebChatwoot([]byte(`<head><!--chatwoot--></head>`), "https://support.example.com", "tok", "invalid"))
	assert.Contains(t, defaultModeBody, `"darkMode":"dark"`)
}

func TestWebRouterChatwootEnvContract(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "https://chat.example.com/")
	t.Setenv("CHATWOOT_WEBSITE_TOKEN", "site-token-abc")
	t.Setenv("CHATWOOT_DARK_MODE", "light")
	outer := gin.New()
	SetWebRouter(outer, WebAssets{IndexPage: []byte(searchPageTestShell)}, func(c *gin.Context) { c.Next() })

	okResp := performPluginRequest(outer, http.MethodGet, "/influencer")
	require.Equal(t, http.StatusOK, okResp.Code)
	assert.Equal(t, 1, strings.Count(okResp.Body.String(), `id="omnimux-chatwoot-config"`))
	assert.Contains(t, okResp.Body.String(), `"baseUrl":"https://chat.example.com"`)
	assert.Contains(t, okResp.Body.String(), `"websiteToken":"site-token-abc"`)
	assert.Contains(t, okResp.Body.String(), `"darkMode":"light"`)

	var parsed struct {
		BaseURL      string `json:"baseUrl"`
		WebsiteToken string `json:"websiteToken"`
		DarkMode     string `json:"darkMode"`
	}
	_, after, found := strings.Cut(okResp.Body.String(), `<script id="omnimux-chatwoot-config" type="application/json">`)
	require.True(t, found)
	jsonBody, _, foundEnd := strings.Cut(after, `</script>`)
	require.True(t, foundEnd)
	require.NoError(t, common.Unmarshal([]byte(jsonBody), &parsed))
	assert.Equal(t, "https://chat.example.com", parsed.BaseURL)
	assert.Equal(t, "site-token-abc", parsed.WebsiteToken)
	assert.Equal(t, "light", parsed.DarkMode)

	guideUnavailableResp := performPluginRequest(outer, http.MethodGet, "/guides")
	require.Equal(t, http.StatusServiceUnavailable, guideUnavailableResp.Code)
	assert.NotContains(t, guideUnavailableResp.Body.String(), `id="omnimux-chatwoot-config"`)
}

func TestWebSearchPagesChatwootIsInjectedAfterEveryPageAssembly(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(fmt.Sprint(configured), func(t *testing.T) {
			pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
			require.NoError(t, err)
			baseURL, token := "", ""
			if configured {
				baseURL = "https://chat.example.com/"
				token = "public-token-xyz"
			}
			pages.injectChatwoot(baseURL, token, "dark")
			outer := gin.New()
			outer.NoRoute(pages.serve)
			for _, path := range []string{"/", "/pricing", "/guides", "/guides/create-ai-character", "/influencer", "/unknown"} {
				resp := performPluginRequest(outer, http.MethodGet, path)
				require.Equal(t, http.StatusOK, resp.Code, path)
				body := resp.Body.String()
				wantTags := 0
				if configured {
					wantTags = 1
				}
				assert.Equal(t, wantTags, strings.Count(body, `id="omnimux-chatwoot-config"`), path)
				assert.NotContains(t, body, "<!--chatwoot-->", path)
			}
		})
	}
}
