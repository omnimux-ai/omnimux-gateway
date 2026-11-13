package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomResponseValidationRejectsUnsupportedRoute(t *testing.T) {
	tests := []struct {
		name   string
		change map[string]any
	}{
		{"unknown policy", map[string]any{"response_validation": "anything"}},
		{"wrong capability", map[string]any{"models": []string{"tiktok-video"}}},
		{"wrong policy for capability", map[string]any{"response_validation": "tikhub-posts"}},
		{"catch-all capability", map[string]any{"models": []string{}}},
		{"pattern capability", map[string]any{"models": []string{"x-*"}}},
		{"multiple capabilities", map[string]any{"models": []string{"x-user", "tiktok-user"}}},
		{"wrong upstream path", map[string]any{"upstream_path": "/arbitrary"}},
		{"full URL", map[string]any{"upstream_path": "https://example.test/api/v1/twitter/web/fetch_user_profile"}},
		{"query appended", map[string]any{"upstream_path": "/api/v1/twitter/web/fetch_user_profile?query=1"}},
		{"wrong incoming path", map[string]any{"incoming_path": "/v1/responses"}},
		{"wrong method", map[string]any{"upstream_method": "POST", "passthrough_response": true}},
		{"no method", map[string]any{"upstream_method": "", "passthrough_response": true}},
		{"disabled passthrough", map[string]any{"passthrough_response": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := map[string]any{"incoming_path": "/v1/chat/completions", "upstream_path": "/api/v1/twitter/web/fetch_user_profile", "upstream_method": "GET", "models": []string{"x-user"}, "response_validation": "tikhub-user"}
			for key, value := range tt.change {
				route[key] = value
			}
			raw, err := json.Marshal(map[string]any{"advanced_routes": []any{route}})
			require.NoError(t, err)
			var config AdvancedCustomConfig
			require.NoError(t, json.Unmarshal(raw, &config))
			assert.Error(t, config.Validate())
		})
	}
}

func TestAdvancedCustomResponseValidationAcceptsFourContractsAndRoundTrips(t *testing.T) {
	tests := []struct{ model, policy, path string }{
		{"x-user", AdvancedCustomResponseValidationTikHubUser, "/api/v1/twitter/web/fetch_user_profile"},
		{"x-posts", AdvancedCustomResponseValidationTikHubPosts, "/api/v1/twitter/web/fetch_user_post_tweet"},
		{"tiktok-user", AdvancedCustomResponseValidationTikHubUser, "/api/v1/tiktok/web/fetch_user_profile"},
		{"tiktok-posts", AdvancedCustomResponseValidationTikHubPosts, "/api/v1/tiktok/app/v3/fetch_user_post_videos"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			config := AdvancedCustomConfig{Routes: []AdvancedCustomRoute{{
				IncomingPath: "/v1/chat/completions", UpstreamPath: tt.path, Models: []string{tt.model}, UpstreamMethod: "GET", ResponseValidation: tt.policy,
			}}}
			require.NoError(t, config.Validate())
			raw, err := json.Marshal(config)
			require.NoError(t, err)
			var restored AdvancedCustomConfig
			require.NoError(t, json.Unmarshal(raw, &restored))
			require.Len(t, restored.Routes, 1)
			assert.Equal(t, tt.policy, restored.Routes[0].ResponseValidation)
			require.NoError(t, restored.Validate())
		})
	}
	existing := AdvancedCustomConfig{Routes: []AdvancedCustomRoute{{IncomingPath: "/v1/chat/completions", UpstreamPath: "/arbitrary", UpstreamMethod: "GET"}}}
	require.NoError(t, existing.Validate())
}
