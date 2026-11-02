package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

// fork: only pages with public, route-specific content may be indexed.
var publicSearchPagePaths = [...]string{"/", "/ai-influencer-studio", "/about", "/privacy", "/terms", "/refunds", "/acceptable-use", "/pricing", "/inspiration", "/guides", "/guides/create-ai-character", "/guides/recreate-video-with-your-character", "/tools/tiktok-video-downloader", "/tools/tiktok-photo-downloader", "/zh/tools/tiktok-video-downloader", "/zh/tools/tiktok-photo-downloader"}

// fork: these editorial routes must never degrade to a successful application shell.
var staticGuidePagePaths = map[string]bool{
	"/guides": true, "/guides/create-ai-character": true, "/guides/recreate-video-with-your-character": true,
}

type webSearchPage struct {
	Head         string `json:"head"`
	Summary      string `json:"summary"`
	DocumentHTML string `json:"documentHtml,omitempty"`
}

type webSearchManifest struct {
	Pages   map[string]webSearchPage `json:"pages"`
	Private webSearchPage            `json:"private"`
}

// inspirationShareLookup resolves an active share by id; injectable for tests.
var inspirationShareLookup = model.GetInspirationShareByShareId

type webSearchPages struct {
	public      map[string][]byte
	private     []byte
	unavailable []byte
}

func buildWebSearchPages(indexPage, manifestData []byte) (webSearchPages, error) {
	pages := webSearchPages{
		public: make(map[string][]byte), private: indexPage,
		unavailable: []byte(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Guide temporarily unavailable | OmniMux</title><meta name="robots" content="noindex,follow"><!--umami-->
</head><body><main><h1>Guide temporarily unavailable</h1><p>Please try again later.</p><a href="/">Back to OmniMux</a></main></body></html>`),
	}
	index := string(indexPage)
	var problems []string
	privatePage, err := renderWebSearchPage(index, webSearchPage{
		Head: `<meta name="robots" content="noindex,follow" />`,
	})
	if err != nil {
		// Older shells remain usable, but the response header still excludes them.
		problems = append(problems, err.Error())
	} else {
		pages.private = privatePage
	}
	var manifest webSearchManifest
	if err := common.Unmarshal(manifestData, &manifest); err != nil {
		return pages, err
	}
	if manifest.Private.Head != "" && err == nil {
		privatePage, err = renderWebSearchPage(index, webSearchPage{Head: manifest.Private.Head})
		if err != nil {
			return pages, err
		}
		pages.private = privatePage
	}
	for _, path := range publicSearchPagePaths {
		page, ok := manifest.Pages[path]
		if staticGuidePagePaths[path] {
			if !ok || strings.TrimSpace(page.Head) == "" {
				problems = append(problems, "missing static document metadata: "+path)
				continue
			}
			body, docErr := renderWebGuideDocument(page.DocumentHTML)
			if docErr != nil {
				problems = append(problems, path+": "+docErr.Error())
				continue
			}
			pages.public[path] = body
			continue
		}
		if !ok || strings.TrimSpace(page.Head) == "" || strings.TrimSpace(page.Summary) == "" {
			problems = append(problems, "missing public page metadata: "+path)
			continue
		}
		body, err := renderWebSearchPage(index, page)
		if err != nil {
			problems = append(problems, path+": "+err.Error())
			continue
		}
		// fork: the desktop landing page is the omnimux.ai homepage, so every
		// public page carries the desktop surface marker and pricing group.
		body = desktopSurfaceShell(body)
		// fork: /zh/ pages are Simplified Chinese documents; the shared shell
		// is English, so the root language follows the path.
		if strings.HasPrefix(path, "/zh/") {
			body = []byte(strings.Replace(string(body), `<html lang="en"`, `<html lang="zh-CN"`, 1))
		}
		pages.public[path] = body
	}
	if len(problems) > 0 {
		return pages, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	// fork: `d.omnimux.ai` is retired and redirects here, so the request host no
	// longer selects a surface; the private shell gets the same marker.
	pages.private = desktopSurfaceShell(pages.private)
	return pages, nil
}

func renderWebGuideDocument(document string) ([]byte, error) {
	lower := strings.ToLower(document)
	for _, marker := range []string{"<!doctype html>", "<html", "</html>", "<head", "</head>", "<body", "</body>"} {
		if !strings.Contains(lower, marker) {
			return nil, fmt.Errorf("incomplete static document")
		}
	}
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return nil, fmt.Errorf("invalid static document: %w", err)
	}
	// A complete shell or noscript summary is not an editorial document.
	var hasMainContent func(*html.Node, bool) bool
	hasMainContent = func(node *html.Node, inMain bool) bool {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "noscript", "script", "style", "template":
				return false
			case "main":
				inMain = true
			}
		}
		if inMain && node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
			return true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if hasMainContent(child, inMain) {
				return true
			}
		}
		return false
	}
	if !hasMainContent(doc, false) {
		return nil, fmt.Errorf("static document has no visible main content")
	}
	return []byte(document), nil
}

func renderWebSearchPage(index string, page webSearchPage) ([]byte, error) {
	for _, block := range []struct {
		name    string
		content string
	}{{"head", page.Head}, {"summary", page.Summary}} {
		start := "<!--omnimux-page-" + block.name + "-->"
		end := "<!--/omnimux-page-" + block.name + "-->"
		if strings.Count(index, start) != 1 || strings.Count(index, end) != 1 {
			return nil, fmt.Errorf("missing or duplicate search-page %s markers", block.name)
		}
		before, after, _ := strings.Cut(index, start)
		_, remainder, found := strings.Cut(after, end)
		if !found {
			return nil, fmt.Errorf("invalid search-page %s markers", block.name)
		}
		index = before + start + block.content + end + remainder
	}
	return []byte(index), nil
}

// searchPageCanonicalHost returns the public site URL recorded in the landing
// head (the build-time VITE_SITE_URL), so share-page canonical links stay on
// omnimux.ai regardless of the request host.
func searchPageCanonicalHost(home []byte) string {
	marker := `rel="canonical" href="`
	index := strings.Index(string(home), marker)
	if index < 0 {
		return ""
	}
	url := string(home)[index+len(marker):]
	end := strings.Index(url, `"`)
	if end < 0 {
		return ""
	}
	return strings.TrimSuffix(url[:end], "/")
}

// fork: inspiration share pages get their own indexable head only when the
// share is a durable, complete record. Ephemeral (72h) and incomplete shares
// stay noindex so temporary links never enter the index.
func inspirationShareIndexable(share *model.InspirationShare) bool {
	return share != nil &&
		share.ExpiresAt == nil &&
		strings.TrimSpace(share.Title) != "" &&
		strings.TrimSpace(share.Description) != "" &&
		strings.TrimSpace(share.CoverUrl) != ""
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
)

// fork: render the share-specific head inside the private application shell so
// the SPA still hydrates PromptGallery while crawlers read real metadata.
func inspirationShareHead(siteUrl, shareId string, share *model.InspirationShare) string {
	esc := htmlEscaper.Replace
	title := fmt.Sprintf("%s | OmniMux Inspiration", esc(share.Title))
	description := strings.TrimSpace(share.Description)
	if len(description) > 160 {
		description = strings.TrimSpace(description[:160])
	}
	description = esc(description)
	canonical := fmt.Sprintf("%s/s/%s", siteUrl, esc(shareId))
	cover := esc(share.CoverUrl)
	return strings.Join([]string{
		`<title>` + title + `</title>`,
		`<meta name="title" content="` + title + `" />`,
		`<meta name="omnimux-seo-title" content="` + title + `" />`,
		`<meta name="description" content="` + description + `" />`,
		`<meta name="robots" content="index,follow" />`,
		`<link rel="canonical" href="` + canonical + `" />`,
		`<meta property="og:type" content="article" />`,
		`<meta property="og:url" content="` + canonical + `" />`,
		`<meta property="og:title" content="` + title + `" />`,
		`<meta property="og:description" content="` + description + `" />`,
		`<meta property="og:site_name" content="OmniMux" />`,
		`<meta property="og:image" content="` + cover + `" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
		`<meta name="twitter:title" content="` + title + `" />`,
		`<meta name="twitter:description" content="` + description + `" />`,
		`<meta name="twitter:image" content="` + cover + `" />`,
	}, "\n")
}

// serveSharePage handles /s/<share_id>. Eligible permanent shares get an
// indexable head; everything else falls back to the noindex application shell.
func (pages webSearchPages) serveSharePage(c *gin.Context, index string) {
	servePrivate := func() {
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Robots-Tag", "noindex, follow")
		c.Data(http.StatusOK, "text/html; charset=utf-8", pages.private)
	}
	shareId := strings.TrimPrefix(index, "/s/")
	if shareId == "" || strings.Contains(shareId, "/") {
		servePrivate()
		return
	}
	share, err := inspirationShareLookup(shareId)
	if err != nil || !inspirationShareIndexable(share) {
		servePrivate()
		return
	}
	siteUrl := searchPageCanonicalHost(pages.public["/"])
	head := inspirationShareHead(siteUrl, shareId, share)
	page, renderErr := renderWebSearchPage(string(pages.private), webSearchPage{Head: head, Summary: ""})
	if renderErr != nil {
		servePrivate()
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", page)
}

func (pages webSearchPages) serve(c *gin.Context) {
	urlPath := c.Request.URL.Path
	if strings.HasPrefix(urlPath, "/s/") {
		pages.serveSharePage(c, urlPath)
		return
	}
	if urlPath != "/" && strings.HasSuffix(urlPath, "/") {
		canonicalPath := strings.TrimSuffix(urlPath, "/")
		_, public := pages.public[canonicalPath]
		if public || staticGuidePagePaths[canonicalPath] {
			location := strings.TrimSuffix(urlPath, "/")
			if c.Request.URL.RawQuery != "" {
				location += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-cache")
			c.Redirect(http.StatusMovedPermanently, location)
			return
		}
	}
	page, public := pages.public[urlPath]
	status := http.StatusOK
	if !public {
		page = pages.private
		if staticGuidePagePaths[urlPath] {
			page = pages.unavailable
			status = http.StatusServiceUnavailable
		}
		c.Header("X-Robots-Tag", "noindex, follow")
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(status, "text/html; charset=utf-8", page)
}
