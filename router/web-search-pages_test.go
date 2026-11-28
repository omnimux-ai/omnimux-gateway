package router

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
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
	for path, name := range map[string]string{
		"/": "Home", "/ai-influencer-studio": "AI Influencer Studio", "/pricing": "Pricing",
		"/about": "About", "/privacy": "Privacy", "/terms": "Terms", "/refunds": "Refunds",
		"/acceptable-use": "Acceptable use", "/inspiration": "Inspiration", "/inspiration/tiktok": "TikTok Ad Library",
		"/tools/tiktok-video-downloader": "TikTok Video Downloader", "/tools/tiktok-photo-downloader": "TikTok Photo Downloader", "/tools/tiktok-transcript-generator": "TikTok Transcript Generator",
		"/zh/tools/tiktok-video-downloader": "TikTok 视频下载", "/zh/tools/tiktok-photo-downloader": "TikTok 图文下载", "/zh/tools/tiktok-transcript-generator": "TikTok 字幕提取",
		"/zh-tw/tools/tiktok-video-downloader": "TikTok 影片下載", "/zh-tw/tools/tiktok-photo-downloader": "TikTok 圖文下載", "/zh-tw/tools/tiktok-transcript-generator": "TikTok 逐字稿工具",
		"/ja/tools/tiktok-video-downloader": "TikTok 動画保存", "/ja/tools/tiktok-photo-downloader": "TikTok 画像保存", "/ja/tools/tiktok-transcript-generator": "TikTok 文字起こし",
		"/es/tools/tiktok-video-downloader": "Descargar Video TikTok", "/es/tools/tiktok-photo-downloader": "Descargar Fotos TikTok", "/es/tools/tiktok-transcript-generator": "Transcribir TikTok",
		"/id/tools/tiktok-video-downloader": "Download Video TikTok", "/id/tools/tiktok-photo-downloader": "Download Foto TikTok", "/id/tools/tiktok-transcript-generator": "Transkrip TikTok",
		"/cli": "CLI", "/skills": "Skills",
		"/zh/cli": "CLI 简体", "/zh/skills": "Skills 简体",
		"/zh-tw/cli": "CLI 繁體", "/zh-tw/skills": "Skills 繁體",
		"/ja/cli": "CLI 日本語", "/ja/skills": "Skills 日本語",
		"/es/cli": "CLI ES", "/es/skills": "Skills ES",
		"/id/cli": "CLI ID", "/id/skills": "Skills ID",
	} {
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
	for path, name := range map[string]string{"/": "Home", "/about": "About", "/privacy": "Privacy", "/terms": "Terms", "/refunds": "Refunds", "/acceptable-use": "Acceptable use", "/pricing": "Pricing", "/inspiration": "Inspiration", "/inspiration/tiktok": "TikTok Ad Library"} {
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
			assert.Equal(t, middleware.PublicHTMLCacheControl, response.Header().Get("Cache-Control"))
			assert.Equal(t, "Accept-Encoding", response.Header().Get("Vary"))
		})
	}
}

func TestWebSearchPagesExcludePrivateAndUnknownPaths(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	for _, path := range []string{"/console", "/console/characters?tab=photos", "/login", "/cli/login", "/security", "/not-a-page", "/privacy-policy", "/user-agreement", "/PRICING"} {
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
	agentRedirect := performPluginRequest(outer, http.MethodGet, "/zh/skills/?from=nav")
	assert.Equal(t, http.StatusMovedPermanently, agentRedirect.Code)
	assert.Equal(t, "/zh/skills?from=nav", agentRedirect.Header().Get("Location"))
	assert.Empty(t, agentRedirect.Header().Get("X-Robots-Tag"))
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

	// Missing or expired share is a real 404 with the noindex application shell.
	missing := performPluginRequest(outer, http.MethodGet, "/s/insp_missing99")
	assert.Equal(t, http.StatusNotFound, missing.Code)
	assert.Equal(t, "noindex, follow", missing.Header().Get("X-Robots-Tag"))
	assert.Contains(t, missing.Body.String(), "<title>Workspace | OmniMux</title>")
}

func TestWebSearchArtifactsHaveOnePublicEntry(t *testing.T) {
	outer := gin.New()
	SetWebRouter(outer, WebAssets{IndexPage: []byte(searchPageTestShell)}, func(c *gin.Context) { c.Next() })
	index := performPluginRequest(outer, http.MethodGet, "/index.html?source=search")
	assert.Equal(t, http.StatusMovedPermanently, index.Code)
	assert.Equal(t, "/?source=search", index.Header().Get("Location"))
	for path, location := range map[string]string{"/zh": "/", "/zh/": "/", "/zh?lang=zh": "/?lang=zh", "/zh/?lang=zh": "/?lang=zh"} {
		zhRoot := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, http.StatusMovedPermanently, zhRoot.Code, path)
		assert.Equal(t, location, zhRoot.Header().Get("Location"), path)
	}
	manifest := performPluginRequest(outer, http.MethodGet, "/search-pages.json")
	assert.Equal(t, http.StatusNotFound, manifest.Code)
	assert.Equal(t, "noindex, follow", manifest.Header().Get("X-Robots-Tag"))
	assert.NotContains(t, manifest.Body.String(), "Old homepage")
	assert.Equal(t, "no-cache", manifest.Header().Get("Cache-Control"))
	for _, path := range []string{"/robots.txt", "/sitemap.xml"} {
		response := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
		assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	}
	home := performPluginRequest(outer, http.MethodGet, "/")
	assert.Equal(t, "nosniff", home.Header().Get("X-Content-Type-Options"))
	for _, path := range []string{"/api/missing", "/v1/missing", "/assets/missing.js"} {
		response := performPluginRequest(outer, http.MethodGet, path)
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Contains(t, response.Body.String(), "Invalid URL")
		assert.Contains(t, response.Header().Get("Cache-Control"), "no-store")
	}
}

// The full router carries the relay's /:mode/mj group, which turns an
// unregistered /zh/ into gin's trailing-slash redirect; assert on it.
func TestWebZhRootRedirectsOnceOnFullRouter(t *testing.T) {
	engine := gin.New()
	SetRouter(engine, WebAssets{IndexPage: []byte(searchPageTestShell)})
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for path, location := range map[string]string{"/zh": "/", "/zh/": "/", "/zh?lang=zh": "/?lang=zh", "/zh/?lang=zh": "/?lang=zh"} {
			response := performPluginRequest(engine, method, path)
			assert.Equal(t, http.StatusMovedPermanently, response.Code, method+" "+path)
			assert.Equal(t, location, response.Header().Get("Location"), method+" "+path)
		}
	}
	// A zh mode prefix still reaches the relay's Midjourney group.
	mj := performPluginRequest(engine, http.MethodPost, "/zh/mj/submit/imagine")
	assert.Equal(t, http.StatusUnauthorized, mj.Code)
	assert.Empty(t, mj.Header().Get("Location"))
}

func TestWebSearchPagesSetChineseRootLanguageForZhPaths(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(pages.serve)
	for path, lang := range map[string]string{
		"/tools/tiktok-video-downloader":           "en",
		"/tools/tiktok-transcript-generator":       "en",
		"/zh/tools/tiktok-video-downloader":        "zh-Hans",
		"/zh/tools/tiktok-transcript-generator":    "zh-Hans",
		"/zh-tw/tools/tiktok-video-downloader":     "zh-Hant",
		"/zh-tw/tools/tiktok-photo-downloader":     "zh-Hant",
		"/zh-tw/tools/tiktok-transcript-generator": "zh-Hant",
		"/ja/tools/tiktok-video-downloader":        "ja",
		"/ja/tools/tiktok-photo-downloader":        "ja",
		"/ja/tools/tiktok-transcript-generator":    "ja",
		"/es/tools/tiktok-video-downloader":        "es",
		"/es/tools/tiktok-photo-downloader":        "es",
		"/es/tools/tiktok-transcript-generator":    "es",
		"/id/tools/tiktok-video-downloader":        "id",
		"/id/tools/tiktok-photo-downloader":        "id",
		"/id/tools/tiktok-transcript-generator":    "id",
		"/cli":                                     "en",
		"/zh/cli":                                  "zh-Hans",
		"/zh/skills":                               "zh-Hans",
		"/zh-tw/cli":                               "zh-Hant",
		"/ja/cli":                                  "ja",
		"/es/cli":                                  "es",
		"/id/cli":                                  "id",
	} {
		t.Run(path, func(t *testing.T) {
			resp := performPluginRequest(outer, http.MethodGet, path)
			require.Equal(t, http.StatusOK, resp.Code)
			assert.Contains(t, resp.Body.String(), `<html lang="`+lang+`"`)
			if path == "/zh/tools/tiktok-video-downloader" {
				assert.Contains(t, resp.Body.String(), "TikTok 视频下载 page")
			}
		})
	}
	t.Run("trailing slash redirect on localized tool route", func(t *testing.T) {
		resp := performPluginRequest(outer, http.MethodGet, "/ja/tools/tiktok-transcript-generator/")
		require.Equal(t, http.StatusMovedPermanently, resp.Code)
		assert.Equal(t, "/ja/tools/tiktok-transcript-generator", resp.Header().Get("Location"))
	})
}

// Public SEO HTML is edge-cacheable while private application shells and
// unknown paths stay out of shared caches.
func TestWebSearchPagesCachePolicySplitsPublicPrivateAnd404(t *testing.T) {
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(middleware.Cache())
	engine.NoRoute(routes.serve(pages))

	for _, path := range []string{"/", "/about", "/pricing"} {
		response := performPluginRequest(engine, http.MethodGet, path)
		require.Equal(t, http.StatusOK, response.Code, path)
		assert.Equal(t, middleware.PublicHTMLCacheControl, response.Header().Get("Cache-Control"), path)
		assert.Equal(t, "Accept-Encoding", response.Header().Get("Vary"), path)
		assert.NotEmpty(t, response.Header().Get("Cache-Version"), path)
	}
	for _, path := range []string{"/console", "/dashboard", "/does-not-exist"} {
		response := performPluginRequest(engine, http.MethodGet, path)
		assert.Equal(t, middleware.PrivateHTMLCacheControl, response.Header().Get("Cache-Control"), path)
		assert.Equal(t, "Accept-Encoding", response.Header().Get("Vary"), path)
		assert.Equal(t, "noindex, follow", response.Header().Get("X-Robots-Tag"), path)
	}
}
