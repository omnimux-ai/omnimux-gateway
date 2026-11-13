package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// RequireInfluencerCreation runs after UserAuth. The new creation surface
// requires an explicit scoped grant even during the legacy-token grace period.
func RequireInfluencerCreation(scopeAction string) gin.HandlerFunc {
	requiredScope := "influencer_creation:" + scopeAction
	validAction := scopeAction == "read" || scopeAction == "write"
	return func(c *gin.Context) {
		if validAction && !c.GetBool("use_access_token") {
			if identity, ok := GetSessionAuthIdentity(c); ok && identity.UserID == c.GetInt("id") {
				c.Next()
				return
			}
		}
		scopes, _ := c.Get("access_token_scopes")
		grantedScopes, _ := scopes.([]string)
		if validAction && c.GetBool("use_access_token") && !c.GetBool("access_token_legacy") &&
			c.GetInt("id") > 0 && c.GetInt("access_token_id") > 0 && service.AccessTokenScopeGranted(grantedScopes, requiredScope) {
			c.Next()
			return
		}
		setAccessTokenAuditParam(c, "required_scope", requiredScope)
		setAccessTokenAuditParam(c, "failure_reason", "scope_denied")
		abortWithWebMessage(c, http.StatusForbidden, "ACCESS_TOKEN_SCOPE_DENIED", common.NewMessage("The access token lacks the required permission: {{scope}}", map[string]any{"scope": requiredScope}))
	}
}
