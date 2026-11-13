package oauth

import (
	"net"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// OAuthRedirectURI is the redirect URI shared by the authorize request and the
// token exchange. The frontend starts a flow on the host the user is on, so the
// callback host has to be used for the exchange too; otherwise providers reject
// the code with redirect_uri_mismatch when the flow started on a sibling host
// such as d.omnimux.ai. Only the configured ServerAddress host and its
// subdomains are trusted; anything else falls back to ServerAddress.
func OAuthRedirectURI(c *gin.Context, slug string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	origin := requestOrigin(c, base)
	if origin == "" {
		origin = base
	}
	return origin + "/oauth/" + slug
}

func requestOrigin(c *gin.Context, serverAddress string) string {
	if c == nil || c.Request == nil {
		return ""
	}
	base, err := url.Parse(serverAddress)
	if err != nil || base.Host == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(c.Request.Host, "."))
	if host == "" {
		return ""
	}
	hostname := host
	if value, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		hostname = value
	}
	baseHost := strings.ToLower(base.Hostname())
	if hostname == "" || baseHost == "" {
		return ""
	}
	if hostname != baseHost && !strings.HasSuffix(hostname, "."+baseHost) {
		return ""
	}
	scheme := base.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + host
}
