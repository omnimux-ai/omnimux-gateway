package router

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	chatwootPlaceholder = "<!--chatwoot-->"
	chatwootConfigTagID = "omnimux-chatwoot-config"
)

type webChatwootConfig struct {
	BaseURL      string `json:"baseUrl"`
	WebsiteToken string `json:"websiteToken"`
	DarkMode     string `json:"darkMode"`
}

var scriptJSONEscaper = strings.NewReplacer(
	"<", `\u003c`,
	">", `\u003e`,
	"&", `\u0026`,
	"\u2028", `\u2028`,
	"\u2029", `\u2029`,
)

func normalizeChatwootBaseURL(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "https" && scheme != "http") || parsed.Host == "" {
		return "", false
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	parsed.Scheme = scheme
	return strings.TrimRight(parsed.String(), "/"), true
}

func normalizeChatwootToken(raw string) (string, bool) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", false
	}
	for i := range len(token) {
		ch := token[i]
		if ch <= 0x20 || ch == 0x7f {
			return "", false
		}
	}
	return token, true
}

func normalizeChatwootDarkMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "light":
		return "light"
	case "auto":
		return "auto"
	default:
		return "dark"
	}
}

func marshalSafeScriptJSON(v any) (string, bool) {
	encoded, err := common.Marshal(v)
	if err != nil {
		return "", false
	}
	return scriptJSONEscaper.Replace(string(encoded)), true
}

// fork: inject serialized public Chatwoot widget settings after SPA fragments
// and standalone guide documents are assembled; secrets remain server-only.
func injectWebChatwoot(page []byte, baseURL, websiteToken, darkMode string) []byte {
	document := string(page)
	marker := `id="` + chatwootConfigTagID + `"`
	if strings.Contains(document, marker) {
		return []byte(strings.ReplaceAll(document, chatwootPlaceholder, ""))
	}
	normalizedURL, okURL := normalizeChatwootBaseURL(baseURL)
	normalizedToken, okToken := normalizeChatwootToken(websiteToken)
	if !okURL || !okToken {
		return []byte(strings.ReplaceAll(document, chatwootPlaceholder, ""))
	}
	safeJSON, okJSON := marshalSafeScriptJSON(webChatwootConfig{
		BaseURL:      normalizedURL,
		WebsiteToken: normalizedToken,
		DarkMode:     normalizeChatwootDarkMode(darkMode),
	})
	if !okJSON {
		return []byte(strings.ReplaceAll(document, chatwootPlaceholder, ""))
	}
	configTag := `<script id="` + chatwootConfigTagID + `" type="application/json">` + safeJSON + `</script>` + "\n"
	if strings.Contains(document, chatwootPlaceholder) {
		document = strings.Replace(document, chatwootPlaceholder, configTag, 1)
		return []byte(strings.ReplaceAll(document, chatwootPlaceholder, ""))
	}
	if before, after, ok := strings.Cut(document, "</head>"); ok {
		return []byte(before + configTag + "</head>" + after)
	}
	return []byte(document + configTag)
}

func (pages *webSearchPages) injectChatwoot(baseURL, websiteToken, darkMode string) {
	for path, page := range pages.public {
		pages.public[path] = injectWebChatwoot(page, baseURL, websiteToken, darkMode)
	}
	pages.private = injectWebChatwoot(pages.private, baseURL, websiteToken, darkMode)
	pages.unavailable = injectWebChatwoot(pages.unavailable, "", "", "")
}
