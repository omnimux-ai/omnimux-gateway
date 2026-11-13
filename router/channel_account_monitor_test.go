package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAccountMonitorChannelTestUsesOperatePermissionAndAdminAuth(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodPost, "/test/:id", authz.ChannelOperate, controller.TestAccountMonitorChannel)
	assertChannelRoutePermission(t, http.MethodGet, "/test/:id", authz.ChannelOperate, controller.TestChannel)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/channel/test/29", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
