package router

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// fork: this server-only manifest is emitted from TanStack's effective route tree.
type webAppRouteManifest struct {
	Version int      `json:"version"`
	Paths   []string `json:"paths"`
	Legacy  struct {
		ExactPaths  []string `json:"exactPaths"`
		PrefixPaths []string `json:"prefixPaths"`
	} `json:"legacy"`
}

type webAppRoutes struct {
	available      bool
	paths          [][]string
	legacyExact    map[string]bool
	legacyPrefixes []string
}

// webPricingModelExists reports whether a pricing detail page has a model
// behind it; injectable for tests. An empty catalogue (startup or cache
// failure) fails open so a transient outage never turns real pages into 404s.
var webPricingModelExists = func(name string) bool {
	catalogue := model.GetPricing()
	if len(catalogue) == 0 {
		return true
	}
	return slices.ContainsFunc(catalogue, func(pricing model.Pricing) bool { return pricing.ModelName == name })
}

var webAppRouteSegment = regexp.MustCompile(`^(?:[A-Za-z0-9._~-]+|\$[A-Za-z_][A-Za-z0-9_]*)$`)

func buildWebAppRoutes(data []byte) (webAppRoutes, error) {
	var manifest webAppRouteManifest
	var routes webAppRoutes
	if err := common.Unmarshal(data, &manifest); err != nil {
		return routes, err
	}
	if manifest.Version != 1 || len(manifest.Paths) == 0 {
		return routes, fmt.Errorf("missing or unsupported application route manifest")
	}
	hasRoot := false
	for _, path := range manifest.Paths {
		segments, err := webAppRouteSegments(path)
		if err != nil {
			return webAppRoutes{}, err
		}
		hasRoot = hasRoot || path == "/"
		routes.paths = append(routes.paths, segments)
	}
	if !hasRoot {
		return webAppRoutes{}, fmt.Errorf("application route manifest has no homepage")
	}
	routes.legacyExact = make(map[string]bool)
	for _, path := range manifest.Legacy.ExactPaths {
		if _, err := webAppRouteSegments(path); err != nil || strings.Contains(path, "$") {
			return webAppRoutes{}, fmt.Errorf("invalid legacy application route: %s", path)
		}
		routes.legacyExact[path] = true
	}
	for _, prefix := range manifest.Legacy.PrefixPaths {
		if _, err := webAppRouteSegments(strings.TrimSuffix(prefix, "/")); err != nil || !strings.HasSuffix(prefix, "/") || prefix == "/" || strings.Contains(prefix, "$") {
			return webAppRoutes{}, fmt.Errorf("invalid legacy application route prefix: %s", prefix)
		}
		routes.legacyPrefixes = append(routes.legacyPrefixes, prefix)
	}
	// A truncated manifest must never quietly turn existing legacy links into 404s.
	if len(routes.legacyExact) == 0 || len(routes.legacyPrefixes) == 0 {
		return webAppRoutes{}, fmt.Errorf("application route manifest has no legacy entry policy")
	}
	routes.available = true
	return routes, nil
}

func webAppRouteSegments(path string) ([]string, error) {
	if path == "/" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("unsupported application route shape: %s", path)
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, segment := range segments {
		if !webAppRouteSegment.MatchString(segment) {
			return nil, fmt.Errorf("unsupported application route shape: %s", path)
		}
	}
	return segments, nil
}

func (routes webAppRoutes) matches(escapedPath string) bool {
	_, matched := routes.canonicalPath(escapedPath)
	return matched
}

// canonicalPath resolves a request path to the route's own spelling: literal
// segments take the route's case, dynamic segments keep the request bytes.
func (routes webAppRoutes) canonicalPath(escapedPath string) (string, bool) {
	// TanStack ignores terminal slashes and matches literal paths without case.
	path := strings.TrimRight(escapedPath, "/")
	if path == "" {
		path = "/"
	}
	// Legacy redirects deliberately retain their current case-sensitive policy.
	if routes.legacyExact[path] {
		return path, true
	}
	for _, prefix := range routes.legacyPrefixes {
		if strings.HasPrefix(path, prefix) {
			return path, true
		}
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if path == "/" {
		segments = nil
	}
	for _, route := range routes.paths {
		if len(route) != len(segments) {
			continue
		}
		canonical := make([]string, len(route))
		matched := true
		for index, expected := range route {
			actual := segments[index]
			if actual == "" || (!strings.HasPrefix(expected, "$") && !strings.EqualFold(expected, actual)) {
				matched = false
				break
			}
			canonical[index] = actual
			if !strings.HasPrefix(expected, "$") {
				canonical[index] = expected
			}
		}
		if matched {
			return "/" + strings.Join(canonical, "/"), true
		}
	}
	return "", false
}

// webDynamicPageExists rejects dynamic application pages whose record is
// known to be missing, so they answer a real 404 instead of a soft 404 shell.
func webDynamicPageExists(canonical string) bool {
	modelId, isPricingModel := strings.CutPrefix(canonical, "/pricing/")
	if !isPricingModel || strings.Contains(modelId, "/") {
		return true
	}
	name, err := url.PathUnescape(modelId)
	return err != nil || webPricingModelExists(name)
}

func (routes webAppRoutes) serve(pages webSearchPages) gin.HandlerFunc {
	return func(c *gin.Context) {
		// A runtime artifact error retains the old noindex SPA fallback. The web
		// build independently fails if the generator never produces this manifest.
		if !routes.available || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			pages.serve(c)
			return
		}
		escapedPath := c.Request.URL.EscapedPath()
		for _, publicPath := range publicSearchPagePaths {
			if escapedPath == publicPath || (publicPath != "/" && escapedPath == publicPath+"/") {
				pages.serve(c)
				return
			}
		}
		// App routes keep their canonical trailing-slash redirect even when the
		// path is no longer a public search page (e.g. /pricing).
		if escapedPath != "/" && strings.HasSuffix(escapedPath, "/") {
			if canonical, matched := routes.canonicalPath(strings.TrimSuffix(escapedPath, "/")); matched {
				location := canonical
				if c.Request.URL.RawQuery != "" {
					location += "?" + c.Request.URL.RawQuery
				}
				c.Header("Cache-Control", "no-cache")
				c.Redirect(http.StatusMovedPermanently, location)
				return
			}
		}
		canonical, matched := routes.canonicalPath(escapedPath)
		// fork: seo-audit-batch2 — /Pricing or /About is one permanent hop to the
		// route's own spelling instead of a second, noindex copy of the page.
		if matched && canonical != escapedPath {
			location := canonical
			if c.Request.URL.RawQuery != "" {
				location += "?" + c.Request.URL.RawQuery
			}
			c.Header("Cache-Control", "no-cache")
			c.Redirect(http.StatusMovedPermanently, location)
			return
		}
		if matched && webDynamicPageExists(canonical) {
			pages.serve(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Robots-Tag", "noindex, follow")
		if c.Request.Method == http.MethodHead {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Status(http.StatusNotFound)
			c.Writer.WriteHeaderNow()
			return
		}
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", pages.private)
	}
}
