package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetInspirationRouterRegistersWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NotPanics(t, func() { SetApiRouter(engine) })

	routes := engine.Routes()
	actual := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		actual[route.Method+" "+route.Path] = struct{}{}
	}

	// Read routes under /api/inspiration/v1 (UserAuth or Public)
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/status")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/categories")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/public/categories")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/public/inspirations")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/public/media/*key")
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/query")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/inspirations")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/inspirations/:id")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/tags")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/media/*key")
	assert.Contains(t, actual, http.MethodGet+" /api/inspiration/v1/share/:id")
	// Unified share entry, with the two older paths still registered as aliases.
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/share")
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/publish")
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/inspirations/:id/share")

	// Admin write routes under /api/inspiration/v1 (AdminAuth)
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/inspirations")
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/inspirations/bulk")
	assert.Contains(t, actual, http.MethodPatch+" /api/inspiration/v1/inspirations/:id")
	assert.Contains(t, actual, http.MethodDelete+" /api/inspiration/v1/inspirations/:id")
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/tags")
	assert.Contains(t, actual, http.MethodDelete+" /api/inspiration/v1/tags/:id")
	assert.Contains(t, actual, http.MethodPost+" /api/inspiration/v1/media")
}
