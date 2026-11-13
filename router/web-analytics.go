package router

import (
	"html"
	"strings"
)

// fork: inject Umami only after SPA fragments and complete documents are assembled.
// Keep the existing environment contract and upstream ownership marker.
func injectWebUmami(page []byte, siteID, scriptURL string) []byte {
	const placeholder = "<!--umami-->"
	const attribution = "<!--Umami QuantumNous-->"
	document := string(page)
	if strings.Contains(document, attribution) {
		return []byte(strings.ReplaceAll(document, placeholder, ""))
	}
	analytics := attribution + "\n"
	if siteID != "" {
		if scriptURL == "" {
			scriptURL = "https://analytics.umami.is/script.js"
		}
		analytics = `<script defer src="` + html.EscapeString(scriptURL) + `" data-website-id="` + html.EscapeString(siteID) + `"></script>` + analytics
	}
	if strings.Contains(document, placeholder) {
		document = strings.Replace(document, placeholder, analytics, 1)
		return []byte(strings.ReplaceAll(document, placeholder, ""))
	}
	if before, after, ok := strings.Cut(document, "</head>"); ok {
		return []byte(before + analytics + "</head>" + after)
	}
	// An older fallback shell can lack head markers and must remain usable.
	return []byte(document + analytics)
}

func (pages *webSearchPages) injectUmami(siteID, scriptURL string) {
	for path, page := range pages.public {
		pages.public[path] = injectWebUmami(page, siteID, scriptURL)
	}
	pages.private = injectWebUmami(pages.private, siteID, scriptURL)
	pages.unavailable = injectWebUmami(pages.unavailable, siteID, scriptURL)
}
