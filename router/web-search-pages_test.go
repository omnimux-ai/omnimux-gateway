package router

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const searchPageTestShell = `<!doctype html><html lang="en"><head><!--omnimux-page-head--><title>Old homepage</title><link rel="canonical" href="https://omnimux.ai/" /><!--/omnimux-page-head--><script data-analytics="injected">window.analytics=true</script></head><body><!--omnimux-page-summary--><main>Old homepage summary</main><!--/omnimux-page-summary--><div id="root"></div><script src="/assets/app.js"></script></body></html>`

func searchPageTestManifest(t *testing.T) []byte {
	t.Helper()
	pages := map[string]webSearchPage{}
	for path, name := range map[string]string{"/": "Home", "/ai-influencer-studio": "AI Influencer Studio", "/pricing": "Pricing", "/about": "About", "/privacy": "Privacy", "/terms": "Terms", "/refunds": "Refunds", "/acceptable-use": "Acceptable use", "/inspiration": "Inspiration", "/tools/tiktok-video-downloader": "TikTok Video Downloader", "/tools/tiktok-photo-downloader": "TikTok Photo Downloader", "/zh/tools/tiktok-video-downloader": "TikTok 视频下载", "/zh/tools/tiktok-photo-downloader": "TikTok 图文下载"} {
		pages[path] = webSearchPage{
			Head:    fmt.Sprintf(`<title>%s | OmniMux</title><link rel="canonical" href="https://omnimux.ai%s" />`, name, path),
			Summary: fmt.Sprintf(`<main><h1>%s page</h1><p>%s content</p></main>`, name, name),
		}
	}
	for _, path := range []string{"/guides", "/guides/create-ai-character", "/guides/recreate-video-with-your-character"} {
		head := fmt.Sprintf(`<title>%s | OmniMux</title><link rel="canonical" href="https://omnimux.ai%s" />`, path, path)
		pages[path] = webSearchPage{
			Head: head,
			DocumentHTML: `<!doctype html><html lang="en"><head>` + head + `<!--umami-->
</head><body><main><h1>English guide</h1><p>Complete tutorial content for ` + path + `.</p></main></body></html>`,
		}
	}
	// A future manifest entry must not expand the router's public-page policy.
	pages["/console"] = webSearchPage{Head: "console metadata", Summary: "console summary"}
	manifest, err := common.Marshal(webSearchManifest{
		Pages: pages,
		Private: webSearchPage{
			Head:    `<title>Workspace | OmniMux</title><meta name="robots" content="noindex,follow" />`,
			Summary: "private summary must not be rendered",
		},
	})
	require.NoError(t, err)
	return manifest
}

func TestWebSearchPagesServeCompleteStaticGuides(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	for _, path := range []string{"/guides", "/guides/create-ai-character", "/guides/recreate-video-with-your-character"} {
		t.Run(path, func(t *testing.T) {
			response := performPluginRequest(outer, http.MethodGet, path)
			require.Equal(t, http.StatusOK, response.Code)
			body := response.Body.String()
			assert.Contains(t, body, `<main><h1>English guide</h1><p>Complete tutorial content for `+path+`.</p></main>`)
			assert.Contains(t, body, `rel="canonical" href="https://omnimux.ai`+path+`"`)
			assert.NotContains(t, body, "<noscript>")
			assert.NotContains(t, body, `id="root"`)
			assert.NotContains(t, body, "/assets/app.js")
			assert.Empty(t, response.Header().Get("X-Robots-Tag"))
			redirect := performPluginRequest(outer, http.MethodGet, path+"/?from=home&lang=en")
			assert.Equal(t, http.StatusMovedPermanently, redirect.Code)
			assert.Equal(t, path+"?from=home&lang=en", redirect.Header().Get("Location"))
		})
	}
}

func TestWebSearchPagesUnavailableGuidesReturn503(t *testing.T) {
	for name, document := range map[string]string{
		"missing":       "",
		"fragment":      `<main><h1>Incomplete guide</h1></main>`,
		"empty shell":   `<!doctype html><html><head></head><body><div id="root"></div></body></html>`,
		"noscript only": `<!doctype html><html><head></head><body><noscript><main>Invisible guide</main></noscript></body></html>`,
	} {
		t.Run(name, func(t *testing.T) {
			var manifest webSearchManifest
			require.NoError(t, common.Unmarshal(searchPageTestManifest(t), &manifest))
			guide := manifest.Pages["/guides/create-ai-character"]
			guide.DocumentHTML = document
			manifest.Pages["/guides/create-ai-character"] = guide
			data, err := common.Marshal(manifest)
			require.NoError(t, err)
			pages, err := buildWebSearchPages([]byte(searchPageTestShell), data)
			require.ErrorContains(t, err, "/guides/create-ai-character")
			outer := gin.New()
			outer.NoRoute(pages.serve)
			response := performPluginRequest(outer, http.MethodGet, "/guides/create-ai-character")
			assert.Equal(t, http.StatusServiceUnavailable, response.Code)
			assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
			assert.Contains(t, response.Body.String(), "Guide temporarily unavailable")
			assert.NotContains(t, response.Body.String(), `id="root"`)
			assert.NotContains(t, response.Body.String(), "canonical")
			for _, path := range []string{"/", "/guides", "/guides/recreate-video-with-your-character"} {
				valid := performPluginRequest(outer, http.MethodGet, path)
				assert.Equal(t, http.StatusOK, valid.Code)
				assert.Empty(t, valid.Header().Get("X-Robots-Tag"))
			}
		})
	}
}

func TestWebSearchPagesStaticGuidesSurviveLegacyShell(t *testing.T) {
	pages, err := buildWebSearchPages([]byte("legacy dashboard"), searchPageTestManifest(t))
	require.Error(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	response := performPluginRequest(outer, http.MethodGet, "/guides")
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "English guide")
	assert.Empty(t, response.Header().Get("X-Robots-Tag"))
}

func TestWebSearchPagesServeDistinctPublicContent(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	for path, name := range map[string]string{"/": "Home", "/about": "About", "/privacy": "Privacy", "/terms": "Terms", "/refunds": "Refunds", "/acceptable-use": "Acceptable use", "/pricing": "Pricing", "/inspiration": "Inspiration"} {
		t.Run(name, func(t *testing.T) {
			response := performPluginRequest(outer, http.MethodGet, path+"?source=search")
			require.Equal(t, http.StatusOK, response.Code)
			body := response.Body.String()
			assert.Contains(t, body, "<title>"+name+" | OmniMux</title>")
			assert.Contains(t, body, `rel="canonical" href="https://omnimux.ai`+path+`"`)
			assert.Contains(t, body, "<h1>"+name+" page</h1>")
			assert.Contains(t, body, "<p>"+name+" content</p>")
			assert.NotContains(t, body, "Old homepage")
			assert.Contains(t, body, `<script data-analytics="injected">window.analytics=true</script>`)
			assert.Contains(t, body, `<script src="/assets/app.js"></script>`)
			assert.Empty(t, response.Header().Get("X-Robots-Tag"))
			assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
		})
	}
}

func TestWebSearchPagesExcludePrivateAndUnknownPaths(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	for _, path := range []string{"/console", "/console/characters?tab=photos", "/login", "/security", "/not-a-page", "/privacy-policy", "/user-agreement", "/PRICING"} {
		t.Run(path, func(t *testing.T) {
			response := performPluginRequest(outer, http.MethodGet, path)
			require.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
			assert.Contains(t, response.Body.String(), "<title>Workspace | OmniMux</title>")
			assert.NotContains(t, response.Body.String(), "canonical")
			assert.NotContains(t, response.Body.String(), "Old homepage")
			assert.NotContains(t, response.Body.String(), "console summary")
			assert.NotContains(t, response.Body.String(), "private summary must not be rendered")
			assert.Contains(t, response.Body.String(), `data-analytics="injected"`)
		})
	}
}

func TestWebSearchPagesRedirectPublicTrailingSlash(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	response := performPluginRequest(outer, http.MethodGet, "/about/?source=search&lang=zh")
	assert.Equal(t, http.StatusMovedPermanently, response.Code)
	assert.Equal(t, "/about?source=search&lang=zh", response.Header().Get("Location"))
	assert.Empty(t, response.Header().Get("X-Robots-Tag"))
}

func TestWebSearchPagesFailClosedWithoutManifest(t *testing.T) {
	for name, data := range map[string][]byte{"missing": nil, "invalid JSON": []byte("{"), "empty": []byte("{}")} {
		t.Run(name, func(t *testing.T) {
			pages, err := buildWebSearchPages([]byte(searchPageTestShell), data)
			require.Error(t, err)
			outer := gin.New()
			outer.NoRoute(pages.serve)
			for _, path := range []string{"/", "/pricing", "/console"} {
				response := performPluginRequest(outer, http.MethodGet, path)
				assert.Equal(t, http.StatusOK, response.Code)
				assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
				assert.NotContains(t, response.Body.String(), "canonical")
				assert.NotContains(t, response.Body.String(), "Old homepage")
				assert.Contains(t, response.Body.String(), `<div id="root"></div>`)
				assert.Contains(t, response.Body.String(), `data-analytics="injected"`)
			}
			for _, path := range []string{"/guides", "/guides/create-ai-character", "/guides/recreate-video-with-your-character"} {
				response := performPluginRequest(outer, http.MethodGet, path)
				assert.Equal(t, http.StatusServiceUnavailable, response.Code)
				assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
				assert.Contains(t, response.Body.String(), "Guide temporarily unavailable")
			}
		})
	}
}

func TestWebSearchPagesUmamiIsInjectedAfterEveryPageAssembly(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(fmt.Sprint(configured), func(t *testing.T) {
			pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
			require.NoError(t, err)
			siteID := ""
			if configured {
				siteID = "test-website-id"
			}
			pages.injectUmami(siteID, "https://analytics.example/script.js")
			outer := gin.New()
			outer.NoRoute(pages.serve)
			for _, path := range []string{"/", "/pricing", "/guides", "/guides/create-ai-character", "/influencer", "/unknown"} {
				body := performPluginRequest(outer, http.MethodGet, path).Body.String()
				assert.Equal(t, 1, strings.Count(body, "<!--Umami QuantumNous-->"), path)
				wantScripts := 0
				if configured {
					wantScripts = 1
				}
				assert.Equal(t, wantScripts, strings.Count(body, `data-website-id="test-website-id"`), path)
				assert.NotContains(t, body, "<!--umami-->")
			}
		})
	}
}

func TestWebSearchPagesMissingPageDoesNotReuseHome(t *testing.T) {
	var manifest webSearchManifest
	require.NoError(t, common.Unmarshal(searchPageTestManifest(t), &manifest))
	delete(manifest.Pages, "/about")
	data, err := common.Marshal(manifest)
	require.NoError(t, err)
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), data)
	require.ErrorContains(t, err, "/about")
	outer := gin.New()
	outer.NoRoute(pages.serve)
	response := performPluginRequest(outer, http.MethodGet, "/about")
	assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
	assert.NotContains(t, response.Body.String(), "Home page")
	assert.NotContains(t, response.Body.String(), "canonical")
	home := performPluginRequest(outer, http.MethodGet, "/")
	assert.Empty(t, home.Header().Get("X-Robots-Tag"))
	assert.Contains(t, home.Body.String(), "Home page")
}

func TestWebSearchPagesLegacyShellRemainsUsableButNoindex(t *testing.T) {
	pages, err := buildWebSearchPages([]byte("dashboard"), searchPageTestManifest(t))
	require.Error(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	response := performPluginRequest(outer, http.MethodGet, "/security")
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "dashboard", response.Body.String())
	assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"))
}

func TestWebSearchPagesShareIndexableOnlyForPermanentCompleteShares(t *testing.T) {
	previous := inspirationShareLookup
	t.Cleanup(func() { inspirationShareLookup = previous })
	permanent := &model.InspirationShare{
		ShareId:     "insp_permanent01",
		Title:       "Vintage poster character",
		Description: "A vintage poster look recreated with an OmniMux character",
		CoverUrl:    "https://files.omnimux.ai/cover.jpg",
	}
	expiresAt := time.Now().Add(time.Hour)
	ephemeral := &model.InspirationShare{
		ShareId:     "insp_ephemeral01",
		Title:       "Temporary look",
		Description: "A temporary share",
		CoverUrl:    "https://files.omnimux.ai/cover.jpg",
		ExpiresAt:   &expiresAt,
	}
	inspirationShareLookup = func(shareId string) (*model.InspirationShare, error) {
		switch shareId {
		case permanent.ShareId:
			return permanent, nil
		case ephemeral.ShareId:
			return ephemeral, nil
		}
		return nil, gorm.ErrRecordNotFound
	}

	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)

	// Eligible permanent share gets its own indexable head.
	response := performPluginRequest(outer, http.MethodGet, "/s/insp_permanent01")
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "<title>Vintage poster character | OmniMux Inspiration</title>")
	assert.Contains(t, body, `rel="canonical" href="https://omnimux.ai/s/insp_permanent01"`)
	assert.Contains(t, body, `name="robots" content="index,follow"`)
	assert.Contains(t, body, `og:image" content="https://files.omnimux.ai/cover.jpg"`)
	assert.Empty(t, response.Header().Get("X-Robots-Tag"))

	// Ephemeral share stays noindex.
	ephemeralResponse := performPluginRequest(outer, http.MethodGet, "/s/insp_ephemeral01")
	assert.Equal(t, http.StatusOK, ephemeralResponse.Code)
	assert.Equal(t, "noindex, follow", ephemeralResponse.Header().Get("X-Robots-Tag"))
	assert.Contains(t, ephemeralResponse.Body.String(), "<title>Workspace | OmniMux</title>")
	assert.NotContains(t, ephemeralResponse.Body.String(), "Temporary look | OmniMux Inspiration")

	// Missing share falls back to the noindex application shell.
	missing := performPluginRequest(outer, http.MethodGet, "/s/insp_missing99")
	assert.Equal(t, http.StatusOK, missing.Code)
	assert.Equal(t, "noindex, follow", missing.Header().Get("X-Robots-Tag"))
	assert.Contains(t, missing.Body.String(), "<title>Workspace | OmniMux</title>")
}

func TestWebSearchArtifactsHaveOnePublicEntry(t *testing.T) {
	outer := gin.New()
	SetWebRouter(outer, WebAssets{IndexPage: []byte(searchPageTestShell)}, func(c *gin.Context) { c.Next() })
	index := performPluginRequest(outer, http.MethodGet, "/index.html?source=search")
	assert.Equal(t, http.StatusMovedPermanently, index.Code)
	assert.Equal(t, "/?source=search", index.Header().Get("Location"))
	manifest := performPluginRequest(outer, http.MethodGet, "/search-pages.json")
	assert.Equal(t, http.StatusNotFound, manifest.Code)
	assert.Equal(t, "noindex, follow", manifest.Header().Get("X-Robots-Tag"))
	assert.NotContains(t, manifest.Body.String(), "Old homepage")
	assert.Equal(t, "no-cache", manifest.Header().Get("Cache-Control"))
	for _, path := range []string{"/robots.txt", "/sitemap.xml"} {
		response := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
	}
	for _, path := range []string{"/api/missing", "/v1/missing", "/assets/missing.js"} {
		response := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Contains(t, response.Body.String(), "Invalid URL")
		assert.Contains(t, response.Header().Get("Cache-Control"), "no-store")
	}
}

func TestWebSearchPagesSetChineseRootLanguageForZhPaths(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	zh := performPluginRequest(outer, http.MethodGet, "/zh/tools/tiktok-video-downloader")
	require.Equal(t, http.StatusOK, zh.Code)
	assert.Contains(t, zh.Body.String(), `<html lang="zh-CN"`)
	assert.Contains(t, zh.Body.String(), "TikTok 视频下载 page")
	en := performPluginRequest(outer, http.MethodGet, "/tools/tiktok-video-downloader")
	assert.Contains(t, en.Body.String(), `<html lang="en"`)
}
