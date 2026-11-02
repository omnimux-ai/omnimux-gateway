package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Console validation errors must keep their API fallback and expose the same source key.
func TestConsoleValidationMessageKeys(t *testing.T) {
	for _, tc := range []struct {
		name, path, message string
		handler             gin.HandlerFunc
	}{
		{"cleanup mode", "/?mode=bad", "invalid mode, must be by_count or by_days", CleanupLogFiles},
		{"cleanup value", "/?mode=by_days&value=0", "invalid value, must be a positive integer", CleanupLogFiles},
		{"audit pagination", "/?p=100000001", "Invalid audit pagination", GetAuditLogs},
		{"audit filters", "/?category=invalid", "Invalid audit filters", GetAuditLogs},
		{"audit timestamp", "/?start_timestamp=invalid", "Invalid audit time range", GetAuditLogs},
		{"audit range", "/?start_timestamp=10&end_timestamp=1", "Invalid audit time range", GetAuditLogs},
		{"audit result", "/?success=invalid", "Invalid audit result", GetAuditLogs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/", tc.handler)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, http.StatusOK, response.Code)
			var body struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Key     string `json:"message_key"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
			require.False(t, body.Success)
			require.Equal(t, tc.message, body.Message)
			require.Equal(t, tc.message, body.Key)
		})
	}
}
