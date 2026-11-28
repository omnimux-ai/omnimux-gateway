package router

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

const (
	inspirationCatalogTTL          = 15 * time.Minute
	inspirationCatalogErrorBackoff = time.Minute
	inspirationCatalogMaxBytes     = 2 << 20
	inspirationCatalogHTTPTimeout  = 5 * time.Second
	inspirationModelsPrefix        = "/inspiration/models/"
)

var (
	errInspirationCatalogUnavailable = errors.New("inspiration catalog manifest unavailable")
	inspirationZoneSlugPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	inspirationDatePrefixPattern     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	inspirationSitemapLocPattern     = regexp.MustCompile(`<loc>([^<]+)</loc>`)
	inspirationSitemapLastmodPattern = regexp.MustCompile(`<lastmod>([^<]+)</lastmod>`)
	// Set to false by router test init() so tests never issue live network calls unless injected.
	inspirationCatalogAllowDefaultSiteFetch = true
	// inspirationCatalogFetch overrides manifest loading in tests.
	inspirationCatalogFetch func(siteURL string) ([]byte, error)
	inspirationJSONReplacer = strings.NewReplacer(
		"<", `\u003c`,
		">", `\u003e`,
		"&", `\u0026`,
		"\u2028", `\u2028`,
		"\u2029", `\u2029`,
	)
)

type inspirationCatalogZone struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	MediaType   string   `json:"mediaType"`
	Count       int      `json:"count"`
	Quarantined int      `json:"quarantined"`
	Categories  []string `json:"categories"`
}

type inspirationCatalogManifest struct {
	Version     int                      `json:"version"`
	GeneratedAt string                   `json:"generatedAt"`
	Total       int                      `json:"total"`
	Zones       []inspirationCatalogZone `json:"zones"`
}

type inspirationCatalogSnapshot struct {
	zones   []inspirationCatalogZone
	byID    map[string]inspirationCatalogZone
	lastmod string
}

type inspirationCatalogStore struct {
	mu        sync.Mutex
	snapshot  *inspirationCatalogSnapshot
	expiresAt time.Time
	now       func() time.Time
	fetch     func(siteURL string) ([]byte, error)
}

func newInspirationCatalogStore() *inspirationCatalogStore {
	return &inspirationCatalogStore{
		now: time.Now,
	}
}

func (s *inspirationCatalogStore) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *inspirationCatalogStore) fetchManifest(siteURL string) ([]byte, error) {
	if s.fetch != nil {
		return s.fetch(siteURL)
	}
	return fetchInspirationCatalogManifest(siteURL)
}

func (s *inspirationCatalogStore) get(siteURL string) *inspirationCatalogSnapshot {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.currentTime()
	if !s.expiresAt.IsZero() && now.Before(s.expiresAt) {
		return s.snapshot
	}

	raw, err := s.fetchManifest(siteURL)
	if err == nil {
		if parsed, parseErr := parseInspirationCatalogManifest(raw); parseErr == nil {
			s.snapshot = parsed
			s.expiresAt = now.Add(inspirationCatalogTTL)
			return s.snapshot
		}
	}
	s.expiresAt = now.Add(inspirationCatalogErrorBackoff)
	return s.snapshot
}

func fetchInspirationCatalogManifest(siteURL string) ([]byte, error) {
	if inspirationCatalogFetch != nil {
		return inspirationCatalogFetch(siteURL)
	}
	if filePath := strings.TrimSpace(os.Getenv("INSPIRATION_CATALOG_MANIFEST_PATH")); filePath != "" {
		return readBoundedManifestFile(filePath)
	}
	rawURL := strings.TrimSpace(os.Getenv("INSPIRATION_CATALOG_MANIFEST_URL"))
	if rawURL == "" && inspirationCatalogAllowDefaultSiteFetch && strings.TrimSpace(siteURL) != "" {
		rawURL = strings.TrimRight(strings.TrimSpace(siteURL), "/") + "/inspiration-catalog/manifest.json"
	}
	if rawURL == "" {
		return nil, errInspirationCatalogUnavailable
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid inspiration catalog manifest URL")
	}
	client := &http.Client{Timeout: inspirationCatalogHTTPTimeout}
	resp, err := client.Get(parsed.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected catalog manifest status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, inspirationCatalogMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > inspirationCatalogMaxBytes {
		return nil, fmt.Errorf("inspiration catalog manifest exceeds size limit")
	}
	return data, nil
}

func readBoundedManifestFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, inspirationCatalogMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > inspirationCatalogMaxBytes {
		return nil, fmt.Errorf("inspiration catalog manifest exceeds size limit")
	}
	return data, nil
}

func parseInspirationCatalogManifest(raw []byte) (*inspirationCatalogSnapshot, error) {
	var manifest inspirationCatalogManifest
	if err := common.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	snapshot := &inspirationCatalogSnapshot{
		byID: make(map[string]inspirationCatalogZone, len(manifest.Zones)),
	}
	if len(manifest.GeneratedAt) >= 10 {
		prefix := manifest.GeneratedAt[:10]
		if inspirationDatePrefixPattern.MatchString(prefix) {
			snapshot.lastmod = prefix
		}
	}
	for _, zone := range manifest.Zones {
		id := strings.TrimSpace(zone.ID)
		if !inspirationZoneSlugPattern.MatchString(id) || zone.Count <= 0 {
			continue
		}
		if _, exists := snapshot.byID[id]; exists {
			continue
		}
		zone.ID = id
		zone.Label = strings.TrimSpace(zone.Label)
		if zone.Label == "" {
			zone.Label = id
		}
		snapshot.zones = append(snapshot.zones, zone)
		snapshot.byID[id] = zone
	}
	if len(snapshot.zones) == 0 {
		return nil, fmt.Errorf("inspiration catalog manifest has no indexable zones")
	}
	return snapshot, nil
}

func formatCommaInt(n int) string {
	if n < 0 {
		return "-" + formatCommaInt(-n)
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := 0
	for i := range s {
		if i > maxBytes {
			break
		}
		cut = i
	}
	return strings.TrimSpace(s[:cut])
}

func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	count := 0
	for i := range s {
		if count == maxRunes {
			return strings.TrimSpace(s[:i])
		}
		count++
	}
	return s
}

func inspirationModelMetaTitle(zone inspirationCatalogZone) string {
	withCount := fmt.Sprintf("%s Prompts (%s+) | OmniMux", zone.Label, formatCommaInt(zone.Count))
	if len(withCount) <= 60 {
		return withCount
	}
	withoutCount := fmt.Sprintf("%s Prompts | OmniMux", zone.Label)
	if len(withoutCount) <= 60 {
		return withoutCount
	}
	return truncateUTF8Bytes(withoutCount, 60)
}

func inspirationModelMetaDescription(zone inspirationCatalogZone) string {
	mediaNoun := "AI"
	switch strings.ToLower(strings.TrimSpace(zone.MediaType)) {
	case "video":
		mediaNoun = "video"
	case "image":
		mediaNoun = "image"
	}
	desc := fmt.Sprintf(
		"Browse %s+ %s %s prompts with examples on OmniMux. Copy any prompt or recreate it with your own AI character in the studio.",
		formatCommaInt(zone.Count),
		zone.Label,
		mediaNoun,
	)
	return truncateRunes(desc, 160)
}

func marshalStructuredDataJSON(v any) string {
	raw, err := common.Marshal(v)
	if err != nil {
		return "{}"
	}
	return inspirationJSONReplacer.Replace(string(raw))
}

func inspirationModelHead(siteURL string, zone inspirationCatalogZone) string {
	esc := htmlEscaper.Replace
	rawTitle := inspirationModelMetaTitle(zone)
	rawDesc := inspirationModelMetaDescription(zone)
	title := esc(rawTitle)
	desc := esc(rawDesc)
	routePath := inspirationModelsPrefix + zone.ID
	canonical := siteURL + routePath
	ogImage := siteURL + "/og.png"

	graph := map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type":       "CollectionPage",
				"@id":         canonical + "#webpage",
				"url":         canonical,
				"name":        rawTitle,
				"description": rawDesc,
				"isPartOf": map[string]any{
					"@type": "WebSite",
					"name":  "OmniMux",
					"url":   siteURL + "/",
				},
				"breadcrumb": map[string]any{"@id": canonical + "#breadcrumb"},
			},
			map[string]any{
				"@type": "BreadcrumbList",
				"@id":   canonical + "#breadcrumb",
				"itemListElement": []any{
					map[string]any{"@type": "ListItem", "position": 1, "name": "OmniMux", "item": siteURL + "/"},
					map[string]any{"@type": "ListItem", "position": 2, "name": "Prompt inspiration", "item": siteURL + "/inspiration"},
					map[string]any{"@type": "ListItem", "position": 3, "name": zone.Label, "item": canonical},
				},
			},
		},
	}

	return strings.Join([]string{
		`<title>` + title + `</title>`,
		`<meta name="title" content="` + title + `" />`,
		`<meta name="omnimux-seo-title" content="` + title + `" />`,
		`<meta name="omnimux-server-head-path" content="` + esc(routePath) + `" />`,
		`<meta name="description" content="` + desc + `" />`,
		`<meta name="robots" content="index,follow" />`,
		`<link rel="canonical" href="` + esc(canonical) + `" />`,
		`<meta property="og:type" content="website" />`,
		`<meta property="og:url" content="` + esc(canonical) + `" />`,
		`<meta property="og:title" content="` + title + `" />`,
		`<meta property="og:description" content="` + desc + `" />`,
		`<meta property="og:site_name" content="OmniMux" />`,
		`<meta property="og:image" content="` + esc(ogImage) + `" />`,
		`<meta property="og:locale" content="en_US" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
		`<meta name="twitter:title" content="` + title + `" />`,
		`<meta name="twitter:description" content="` + desc + `" />`,
		`<meta name="twitter:image" content="` + esc(ogImage) + `" />`,
		`<script type="application/ld+json">` + marshalStructuredDataJSON(graph) + `</script>`,
	}, "\n")
}

func inspirationModelSummary(siteURL string, zone inspirationCatalogZone) string {
	esc := htmlEscaper.Replace
	var cats strings.Builder
	limit := min(12, len(zone.Categories))
	for _, cat := range zone.Categories[:limit] {
		trimmed := strings.TrimSpace(cat)
		if trimmed == "" {
			continue
		}
		cats.WriteString("<li>")
		cats.WriteString(esc(trimmed))
		cats.WriteString("</li>")
	}
	categoryBlock := ""
	if cats.Len() > 0 {
		categoryBlock = "<h2>Categories</h2><ul>" + cats.String() + "</ul>"
	}
	heading := fmt.Sprintf("%s Prompts (%s+ Examples)", zone.Label, formatCommaInt(zone.Count))
	desc := inspirationModelMetaDescription(zone)
	nav := strings.Join([]string{
		fmt.Sprintf(`<a href="%s/">Home</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration">Prompt inspiration</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration/tiktok">TikTok Ad Library</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/guides">Guides</a>`, esc(siteURL)),
	}, " | ")
	related := strings.Join([]string{
		fmt.Sprintf(`<a href="%s/inspiration">All prompt libraries</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/inspiration/tiktok">TikTok Ad Library</a>`, esc(siteURL)),
		fmt.Sprintf(`<a href="%s/ai-influencer-studio">AI Influencer Studio</a>`, esc(siteURL)),
	}, " | ")
	return fmt.Sprintf(
		`<noscript><div id="geo-ssr"><header><p>OmniMux</p><nav>%s</nav></header><main><h1>%s</h1><p>%s</p><p>%s prompts indexed in the public library.</p>%s<h2>Explore more</h2><p>%s</p></main></div></noscript>`,
		nav,
		esc(heading),
		esc(desc),
		esc(formatCommaInt(zone.Count)),
		categoryBlock,
		related,
	)
}

func isInspirationQuarantineQuery(values url.Values) bool {
	v := strings.TrimSpace(strings.ToLower(values.Get("quarantine")))
	return v == "1" || v == "true"
}

func (pages webSearchPages) serveInspirationModelNotFound(c *gin.Context) {
	middleware.SetHTMLCacheHeaders(c, false)
	c.Header("X-Robots-Tag", "noindex, follow")
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusNotFound)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", pages.private)
}

func (pages webSearchPages) serveInspirationModelPrivateOK(c *gin.Context) {
	middleware.SetHTMLCacheHeaders(c, false)
	c.Header("X-Robots-Tag", "noindex, follow")
	c.Data(http.StatusOK, "text/html; charset=utf-8", pages.private)
}

func (pages webSearchPages) serveInspirationModelPage(c *gin.Context, urlPath string) {
	if strings.HasSuffix(urlPath, "/") && urlPath != inspirationModelsPrefix {
		location := strings.TrimRight(urlPath, "/")
		if location != "/inspiration/models" {
			if c.Request.URL.RawQuery != "" {
				location += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-cache")
			c.Redirect(http.StatusMovedPermanently, location)
			return
		}
	}
	slug := strings.TrimPrefix(urlPath, inspirationModelsPrefix)
	if slug == "" || strings.Contains(slug, "/") {
		pages.serveInspirationModelNotFound(c)
		return
	}

	siteURL := searchPageCanonicalHost(pages.public["/"])
	snapshot := pages.catalog.get(siteURL)
	if snapshot == nil {
		// Fail open when the runtime catalog cannot be reached so outages do not emit 404s.
		pages.serveInspirationModelPrivateOK(c)
		return
	}

	if lower := strings.ToLower(slug); lower != slug {
		if _, ok := snapshot.byID[lower]; ok {
			location := inspirationModelsPrefix + lower
			if c.Request.URL.RawQuery != "" {
				location += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-cache")
			c.Redirect(http.StatusMovedPermanently, location)
			return
		}
	}

	zone, found := snapshot.byID[slug]
	if !found {
		pages.serveInspirationModelNotFound(c)
		return
	}
	if isInspirationQuarantineQuery(c.Request.URL.Query()) {
		pages.serveInspirationModelPrivateOK(c)
		return
	}

	page, err := renderWebSearchPage(string(pages.private), webSearchPage{
		Head:    inspirationModelHead(siteURL, zone),
		Summary: inspirationModelSummary(siteURL, zone),
	})
	if err != nil {
		pages.serveInspirationModelPrivateOK(c)
		return
	}
	if isAPISubdomainHost(c.Request.Host, siteURL) {
		middleware.SetHTMLCacheHeaders(c, false)
		c.Header("X-Robots-Tag", "noindex, follow")
	} else {
		middleware.SetHTMLCacheHeaders(c, true)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", page)
}

func renderDynamicSitemap(staticSitemap []byte, siteURL string, snapshot *inspirationCatalogSnapshot) []byte {
	site := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	if len(staticSitemap) == 0 || snapshot == nil || len(snapshot.zones) == 0 || site == "" {
		return staticSitemap
	}
	raw := string(staticSitemap)
	closeTag := "</urlset>"
	closeIndex := strings.Index(raw, closeTag)
	if closeIndex < 0 {
		return staticSitemap
	}

	existing := make(map[string]bool)
	for _, match := range inspirationSitemapLocPattern.FindAllStringSubmatch(raw, -1) {
		existing[match[1]] = true
	}
	lastmod := snapshot.lastmod
	if lastmod == "" {
		if match := inspirationSitemapLastmodPattern.FindStringSubmatch(raw); len(match) > 1 {
			lastmod = match[1]
		}
	}

	esc := htmlEscaper.Replace
	var extra strings.Builder
	for _, zone := range snapshot.zones {
		loc := site + inspirationModelsPrefix + zone.ID
		if existing[loc] {
			continue
		}
		existing[loc] = true
		extra.WriteString("  <url><loc>")
		extra.WriteString(esc(loc))
		extra.WriteString("</loc>")
		if lastmod != "" {
			extra.WriteString("<lastmod>")
			extra.WriteString(esc(lastmod))
			extra.WriteString("</lastmod>")
		}
		extra.WriteString("</url>\n")
	}
	if extra.Len() == 0 {
		return staticSitemap
	}
	before := raw[:closeIndex]
	if before != "" && !strings.HasSuffix(before, "\n") {
		before += "\n"
	}
	return []byte(before + extra.String() + raw[closeIndex:])
}

func serveDynamicSitemap(c *gin.Context, staticSitemap []byte, siteURL string, snapshot *inspirationCatalogSnapshot) {
	body := renderDynamicSitemap(staticSitemap, siteURL, snapshot)
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "application/xml; charset=utf-8")
		c.Status(http.StatusOK)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusOK, "application/xml; charset=utf-8", body)
}
