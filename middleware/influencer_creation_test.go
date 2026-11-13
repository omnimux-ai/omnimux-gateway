package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfluencerCreationRequiresNewExplicitGrantOrBrowserSession(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	gin.SetMode(gin.TestMode)
	legacy := "creation-auth-legacy"
	user := createMiddlewarePATUser(t, "creation-auth-owner", legacy)
	old, _ := createMiddlewareScopedToken(t, user.Id, 0, "profile:read", "influencer:read", "usage:read")
	read, _ := createMiddlewareScopedToken(t, user.Id, 0, "influencer_creation:read")
	write, writeEntry := createMiddlewareScopedToken(t, user.Id, 0, "influencer_creation:write")
	admin, _ := createMiddlewareScopedToken(t, user.Id, 0, "influencer_admin:write")

	now := time.Now().Unix()
	session := &model.UserSession{SID: "creation-auth-browser", UserID: user.Id, Version: 1, UserAuthVersion: user.AuthVersion,
		Status: model.UserSessionStatusActive, RefreshHash: "creation-auth-refresh", LoginMethod: "password", LastActiveAt: now, ExpiresAt: now + 3600}
	require.NoError(t, model.CreateUserSession(session))
	browser, _, err := service.IssueAccessToken(service.AuthIdentity{UserID: user.Id, SessionID: session.SID, UserAuthVersion: user.AuthVersion, SessionVersion: session.Version})
	require.NoError(t, err)

	router := gin.New()
	for _, action := range []string{"read", "write"} {
		path := "/creation-auth-test/" + action
		DeclareAccessTokenPermissionRoute(http.MethodGet, path, authz.Permission{Resource: "influencer_creation", Action: action})
		t.Cleanup(func() {
			permissionRouteRulesMu.Lock()
			defer permissionRouteRulesMu.Unlock()
			delete(permissionRouteRules, http.MethodGet+" "+path)
		})
		router.GET(path, UserAuth(), RequireInfluencerCreation(action), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"user_id": c.GetInt("id"), "token_id": c.GetInt("access_token_id")})
		})
	}
	for _, test := range []struct {
		name, action, credential string
		status                   int
	}{
		{"browser read", "read", browser, http.StatusOK},
		{"browser write", "write", browser, http.StatusOK},
		{"read grant", "read", read, http.StatusOK},
		{"write grant", "write", write, http.StatusOK},
		{"read cannot generate", "write", read, http.StatusForbidden},
		{"write cannot list history", "read", write, http.StatusForbidden},
		{"old grant cannot read", "read", old, http.StatusForbidden},
		{"old grant cannot generate", "write", old, http.StatusForbidden},
		{"admin grant is unrelated", "write", admin, http.StatusForbidden},
		{"legacy cannot read", "read", legacy, http.StatusForbidden},
		{"legacy cannot generate", "write", legacy, http.StatusForbidden},
		{"missing identity", "read", "", http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := middlewareBearerRequest(router, "/creation-auth-test/"+test.action, test.credential)
			assert.Equal(t, test.status, response.Code, response.Body.String())
			if test.status == http.StatusForbidden {
				assert.Contains(t, response.Body.String(), `"code":"ACCESS_TOKEN_SCOPE_DENIED"`)
			}
		})
	}
	// The guard cannot turn a revoked token into a browser identity or retain access.
	require.NoError(t, model.DB.Delete(writeEntry).Error)
	assert.Equal(t, http.StatusUnauthorized, middlewareBearerRequest(router, "/creation-auth-test/write", write).Code)
	// Disabled accounts remain blocked even when their explicit creation grant exists.
	require.NoError(t, model.DB.Model(user).Update("status", common.UserStatusDisabled).Error)
	assert.Equal(t, http.StatusUnauthorized, middlewareBearerRequest(router, "/creation-auth-test/read", read).Code)
}

func TestInfluencerCreationGuardFailsClosedWithoutUserAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, action := range []string{"read", "write", "admin", ""} {
		router := gin.New()
		router.GET("/creation", RequireInfluencerCreation(action), func(c *gin.Context) { c.Status(http.StatusOK) })
		request := httptest.NewRequest(http.MethodGet, "/creation", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		assert.Equal(t, http.StatusForbidden, response.Code)
	}
}
