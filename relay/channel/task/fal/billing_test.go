package fal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestParseDurationInt(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"nil input", nil, 5},
		{"int 5", 5, 5},
		{"int 10", 10, 10},
		{"int64 10", int64(10), 10},
		{"float64 10.0", float64(10.0), 10},
		{"string 5", "5", 5},
		{"string 10", "10", 10},
		{"string 5s", "5s", 5},
		{"string 10s", "10s", 10},
		{"string 10S uppercase", "10S", 10},
		{"invalid string", "invalid", 0}, // strconv.Atoi fails -> returns 0, < 10 -> 5s fallback
		{"boolean", true, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDurationInt(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestEstimateBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &TaskAdaptor{}

	t.Run("default duration (5s) gives 1.0x", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
		c.Set("task_request_raw", map[string]any{
			"prompt": "a cinematic video",
		})

		ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{})
		assert.NotNil(t, ratios)
		assert.Equal(t, 1.0, ratios["duration"])
	})

	t.Run("explicit 5s duration gives 1.0x", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
		c.Set("task_request_raw", map[string]any{
			"prompt":   "a cinematic video",
			"duration": 5,
		})

		ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{})
		assert.NotNil(t, ratios)
		assert.Equal(t, 1.0, ratios["duration"])
	})

	t.Run("explicit 10s duration gives 2.0x", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
		c.Set("task_request_raw", map[string]any{
			"prompt":   "a cinematic video",
			"duration": 10,
		})

		ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{})
		assert.NotNil(t, ratios)
		assert.Equal(t, 2.0, ratios["duration"])
	})

	t.Run("string 10s duration gives 2.0x", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
		c.Set("task_request_raw", map[string]any{
			"prompt":   "a cinematic video",
			"duration": "10s",
		})

		ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{})
		assert.NotNil(t, ratios)
		assert.Equal(t, 2.0, ratios["duration"])
	})
}
