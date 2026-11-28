package router

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withStubbedPricingPagesCatalog(t *testing.T, rows []model.Pricing) {
	t.Helper()
	prevCatalog := publicPricingCatalog
	prevExists := webPricingModelExists
	t.Cleanup(func() {
		publicPricingCatalog = prevCatalog
		webPricingModelExists = prevExists
	})
	publicPricingCatalog = func() []model.Pricing {
		return filterPublicPricingRows(rows, map[string]string{"default": "Default group"})
	}
	webPricingModelExists = func(name string) bool {
		for _, row := range rows {
			if row.ModelName == name {
				return true
			}
		}
		return false
	}
}

func pricingTestContract(modelID, brand string, ops ...string) *modelcontracts.Contract {
	if c, ok := modelcontracts.For(modelID); ok {
		return &c
	}
	opList := make([]modelcontracts.Operation, 0, len(ops))
	for _, op := range ops {
		opList = append(opList, modelcontracts.Operation{ID: op, Executable: true})
	}
	return &modelcontracts.Contract{
		ModelID:         modelID,
		SchemaVersion:   modelcontracts.SchemaVersion,
		ExecutionStatus: modelcontracts.ExecutionStatusLive,
		BrandName:       brand,
		OperationIDs:    ops,
		Operations:      opList,
	}
}

func TestPricingModelPagesRoutingMetadataAndSitemap(t *testing.T) {
	seedanceContract := pricingTestContract("seedance-2-5", "ByteDance", "text_to_video")
	gptImageContract := pricingTestContract("gpt-image-2.5", "OpenAI", "text_to_image")
	jevContract := pricingTestContract("jev", "TypeSafe AI", "chat")

	rows := []model.Pricing{
		{
			ModelName:   "seedance-2-5",
			BrandName:   "ByteDance",
			Description: "字节跳动豆包 Seedance 2.5 视频模型，中文描述不能泄漏到英文页面",
			EnableGroup: []string{"default"},
			UpdatedTime: 1795824000, // 2026-11-28 UTC
			Contract:    seedanceContract,
		},
		{
			ModelName:   "gpt-image-2.5",
			BrandName:   "OpenAI",
			Description: "GPT Image 2.5 image service billed per request.",
			EnableGroup: []string{"all"},
			Contract:    gptImageContract,
		},
		{
			ModelName:   "jev-1.13.0",
			BrandName:   "TypeSafe AI",
			EnableGroup: []string{"default"},
			Contract:    jevContract,
		},
		{
			ModelName:   "jev-latest",
			BrandName:   "TypeSafe AI",
			EnableGroup: []string{"default"},
			Contract:    jevContract,
		},
		{
			ModelName:   "jev-preview",
			BrandName:   "TypeSafe AI",
			EnableGroup: []string{"default"},
			Contract:    jevContract,
		},
		{
			ModelName:   "vendor/model",
			BrandName:   "OmniMux",
			EnableGroup: []string{"default"},
			Contract:    pricingTestContract("vendor/model", "OmniMux", "chat"),
		},
		{
			ModelName:   "tiktok-video",
			BrandName:   "OmniMux",
			EnableGroup: []string{"default"},
			Contract: &modelcontracts.Contract{
				ModelID:         "tiktok-video",
				SchemaVersion:   modelcontracts.SchemaVersion,
				ExecutionStatus: modelcontracts.ExecutionStatusLive,
				Family:          "data",
				BrandName:       "OmniMux",
				Operations:      []modelcontracts.Operation{{ID: "query", Executable: true}},
				Billing:         &modelcontracts.Billing{Unit: "call"},
			},
		},
		{
			ModelName:   "internal-vip-only",
			BrandName:   "OmniMux",
			EnableGroup: []string{"vip-internal"},
			Contract:    pricingTestContract("internal-vip-only", "OmniMux", "chat"),
		},
		{
			ModelName:   "uncontracted-legacy",
			BrandName:   "OmniMux",
			EnableGroup: []string{"default"},
			Contract:    nil,
		},
	}

	withStubbedPricingPagesCatalog(t, rows)
	pages, err := buildWebSearchPages([]byte(searchPageTestShell), searchPageTestManifest(t))
	require.NoError(t, err)
	routes, err := buildWebAppRoutes([]byte(appRouteTestManifest))
	require.NoError(t, err)
	outer := gin.New()
	outer.NoRoute(routes.serve(pages))

	// Canonical public live model renders indexable head + specs + cross-links without CJK leakage.
	resp := performHostRequest(outer, http.MethodGet, "/pricing/seedance-2-5", "omnimux.ai")
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Empty(t, resp.Header().Get("X-Robots-Tag"))
	assert.Equal(t, middleware.PublicHTMLCacheControl, resp.Header().Get("Cache-Control"))
	body := resp.Body.String()
	assert.False(t, containsCJK(body), "English pricing model page must not leak Chinese DB description")
	assert.NotContains(t, body, "\u00b7")

	titleMatch := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(body)
	require.Len(t, titleMatch, 2)
	assert.LessOrEqual(t, len(titleMatch[1]), 60)
	assert.Contains(t, titleMatch[1], "Seedance 2.5")

	descMatch := regexp.MustCompile(`<meta name="description" content="([^"]+)"`).FindStringSubmatch(body)
	require.Len(t, descMatch, 2)
	assert.LessOrEqual(t, len([]rune(descMatch[1])), 160)
	assert.Contains(t, descMatch[1], "no subscription")

	assert.Contains(t, body, `<meta name="omnimux-server-head-path" content="/pricing/seedance-2-5" />`)
	assert.Contains(t, body, `<link rel="canonical" href="https://omnimux.ai/pricing/seedance-2-5" />`)
	assert.Contains(t, body, `<meta name="robots" content="index,follow" />`)
	assert.Contains(t, body, `"@type":"SoftwareApplication"`)
	assert.Contains(t, body, `"@type":"BreadcrumbList"`)
	assert.Contains(t, body, `<noscript><div id="geo-ssr">`)
	assert.Contains(t, body, `https://omnimux.ai/inspiration/models/seedance-2.5`)
	assert.Contains(t, body, `https://omnimux.ai/blog/seedance-2-5-prompting-timestamps-and-camera-guide`)
	assert.Contains(t, body, `Text-to-Video`)

	// Trailing slash on model URL redirects 301 cleanly.
	trailing := performHostRequest(outer, http.MethodGet, "/pricing/seedance-2-5/?utm=1", "omnimux.ai")
	assert.Equal(t, http.StatusMovedPermanently, trailing.Code)
	assert.Equal(t, "/pricing/seedance-2-5?utm=1", trailing.Header().Get("Location"))

	// Version snapshot, non-public group, encoded slash, and uncontracted models stay 200 noindex.
	for _, path := range []string{"/pricing/jev-1.13.0", "/pricing/jev-latest", "/pricing/jev-preview", "/pricing/vendor%2Fmodel", "/pricing/internal-vip-only", "/pricing/uncontracted-legacy"} {
		res := performHostRequest(outer, http.MethodGet, path, "omnimux.ai")
		assert.Equal(t, http.StatusOK, res.Code, path)
		assert.Equal(t, "noindex, follow", res.Header().Get("X-Robots-Tag"), path)
		assert.Contains(t, res.Body.String(), "<title>Workspace | OmniMux</title>", path)
	}

	// Data-family model with billing.unit="call" renders "Data API" and "Billed per call".
	dataResp := performHostRequest(outer, http.MethodGet, "/pricing/tiktok-video", "omnimux.ai")
	require.Equal(t, http.StatusOK, dataResp.Code)
	assert.Contains(t, dataResp.Body.String(), "Data API")
	assert.Contains(t, dataResp.Body.String(), "Billed per call using pay-per-generation credits")

	// Non-existent model returns real 404.
	missing := performHostRequest(outer, http.MethodGet, "/pricing/no-such-model", "omnimux.ai")
	assert.Equal(t, http.StatusNotFound, missing.Code)
	assert.Equal(t, "noindex, follow", missing.Header().Get("X-Robots-Tag"))

	// api. subdomain never indexes public model pages.
	apiResp := performHostRequest(outer, http.MethodGet, "/pricing/gpt-image-2.5", "api.omnimux.ai")
	assert.Equal(t, http.StatusOK, apiResp.Code)
	assert.Equal(t, "noindex, follow", apiResp.Header().Get("X-Robots-Tag"))
	assert.Equal(t, middleware.PrivateHTMLCacheControl, apiResp.Header().Get("Cache-Control"))

	// Decoupled dynamic sitemap appends indexable pricing models even when inspiration snapshot is nil,
	// preserving xmlns:xhtml and excluding aliases, private models, and uncontracted models.
	staticXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url><loc>https://omnimux.ai/</loc><lastmod>2026-11-28</lastmod></url>
</urlset>
`)
	merged := string(renderDynamicSitemapWithModels(staticXML, "https://omnimux.ai", nil, indexablePricingModels()))
	assert.Contains(t, merged, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`)
	assert.Contains(t, merged, `<url><loc>https://omnimux.ai/pricing/seedance-2-5</loc><lastmod>2026-11-28</lastmod></url>`)
	assert.Contains(t, merged, `<url><loc>https://omnimux.ai/pricing/gpt-image-2.5</loc><lastmod>2026-11-28</lastmod></url>`)
	assert.NotContains(t, merged, "jev-1.13.0")
	assert.NotContains(t, merged, "jev-latest")
	assert.NotContains(t, merged, "jev-preview")
	assert.NotContains(t, merged, "vendor%2Fmodel")
	assert.NotContains(t, merged, "internal-vip-only")
	assert.NotContains(t, merged, "uncontracted-legacy")
}
