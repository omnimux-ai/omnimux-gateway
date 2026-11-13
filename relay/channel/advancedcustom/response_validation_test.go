package advancedcustom

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorResponseRejectsFailuresBeforeUsageOrBody(t *testing.T) {
	tests := []struct {
		name   string
		policy string
		model  string
		body   string
	}{
		{"business code", "tikhub-user", "x-user", `{"code":400,"message":"private-upstream-detail","data":{"name":"A"}}`},
		{"business flag", "tikhub-user", "x-user", `{"success":false,"data":{"name":"A"}}`},
		{"business error", "tikhub-user", "x-user", `{"error":{"message":"private-upstream-detail"},"data":{"name":"A"}}`},
		{"nested business failure", "tikhub-user", "x-user", `{"code":200,"data":{"status_code":10204,"user":{"name":"A"}}}`},
		{"profile business failure", "tikhub-user", "x-user", `{"code":200,"data":{"user":{"status_code":10204,"name":"A"}}}`},
		{"deep envelope business failure", "tikhub-posts", "x-posts", `{"code":200,"data":{"result":{"data":{"success":false,"items":[{"id":"12345"}]}}}}`},
		{"invalid JSON", "tikhub-user", "x-user", `<html>private-upstream-detail</html>`},
		{"unrecognized user", "tikhub-user", "x-user", `{"code":200,"data":{"unexpected":"A"}}`},
		{"unrecognized posts", "tikhub-posts", "x-posts", `{"code":200,"data":{"unexpected":[]}}`},
		{"invalid nonempty posts", "tikhub-posts", "x-posts", `{"code":200,"data":{"items":[{},null,"invalid"]}}`},
		{"bare empty posts array", "tikhub-posts", "x-posts", `[]`},
		{"bare posts array", "tikhub-posts", "x-posts", `[{"id":"12345"}]`},
		{"enveloped bare posts array", "tikhub-posts", "x-posts", `{"code":200,"data":[{"id":"12345"}]}`},
		{"unrecognized nested node", "tikhub-posts", "x-posts", `{"code":200,"data":{"items":[{"node":{"node":{"id":"12345"}}}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor, info := monitorResponseAdaptor(t, tt.model, tt.policy)
			response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(tt.body))}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			usage, apiErr := adaptor.DoResponse(c, response, info)
			require.NotNil(t, apiErr)
			assert.Nil(t, usage)
			assert.False(t, c.Writer.Written())
			assert.Empty(t, recorder.Body.String())
			assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			assert.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
			assert.True(t, types.IsSkipRetryError(apiErr))
			assert.NotContains(t, apiErr.Error(), "private-upstream-detail")
		})
	}
}

func TestAccountMonitorResponsePreservesRecognizedSuccess(t *testing.T) {
	tests := []struct{ name, policy, model, body string }{
		{"zero followers", "tikhub-user", "x-user", `{"code":200,"data":{"follower_count":0}}`},
		{"wrapped user", "tikhub-user", "tiktok-user", `{"code":200,"data":{"user":{"nickname":"A"}}}`},
		{"known empty posts", "tikhub-posts", "tiktok-posts", `{"code":200,"data":{"aweme_list":[]}}`},
		{"recognized post identity", "tikhub-posts", "x-posts", `{"code":200,"data":{"items":[{"tweet_id":"12345"},{}]}}`},
		{"parent identity with empty node", "tikhub-posts", "x-posts", `{"code":200,"data":{"items":[{"id":"12345","node":{}}]}}`},
		{"parent and child identities", "tikhub-posts", "x-posts", `{"code":200,"data":{"items":[{"id":"12345","node":{"id":"67890"}}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor, info := monitorResponseAdaptor(t, tt.model, tt.policy)
			response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(tt.body))}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			usageAny, apiErr := adaptor.DoResponse(c, response, info)
			require.Nil(t, apiErr)
			usage, ok := usageAny.(*dto.Usage)
			require.True(t, ok)
			assert.Equal(t, 1, usage.TotalTokens)
			assert.Equal(t, tt.body, recorder.Body.String())
		})
	}
}

func monitorResponseAdaptor(t *testing.T, model, policy string) (*Adaptor, *relaycommon.RelayInfo) {
	t.Helper()
	paths := map[string]string{
		"x-user":       "/api/v1/twitter/web/fetch_user_profile",
		"x-posts":      "/api/v1/twitter/web/fetch_user_post_tweet",
		"tiktok-user":  "/api/v1/tiktok/web/fetch_user_profile",
		"tiktok-posts": "/api/v1/tiktok/app/v3/fetch_user_post_videos",
	}
	raw, err := common.Marshal(map[string]any{"advanced_routes": []any{map[string]any{
		"incoming_path": "/v1/chat/completions", "upstream_path": paths[model], "upstream_method": "GET", "models": []string{model}, "response_validation": policy,
	}}})
	require.NoError(t, err)
	var config dto.AdvancedCustomConfig
	require.NoError(t, common.Unmarshal(raw, &config))
	info := advancedCustomRelayInfo(&config)
	info.OriginModelName = model
	info.UpstreamModelName = model
	return &Adaptor{}, info
}

func TestAccountMonitorSummaryAndUnconfiguredCompatibility(t *testing.T) {
	tests := []struct {
		name, policy, body string
		want               AccountMonitorResponseSummary
	}{
		{"profile", "tikhub-user", `{"data":{"user":{"nickname":"A"}}}`, AccountMonitorResponseSummary{ProfileRecognized: true}},
		{"nested empty posts", "tikhub-posts", `{"data":{"result":{"items":[]}}}`, AccountMonitorResponseSummary{}},
		{"edges", "tikhub-posts", `{"data":{"edges":[{"node":{"id":"12345"}},{"node":{"id":"67890"}}]}}`, AccountMonitorResponseSummary{PostCount: 2}},
		{"partial rows", "tikhub-posts", `{"data":{"items":[{"tweet_id":"12345"},{}]}}`, AccountMonitorResponseSummary{PostCount: 1}},
		{"parent identity with empty node", "tikhub-posts", `{"data":{"items":[{"id":"12345","node":{}}]}}`, AccountMonitorResponseSummary{PostCount: 1}},
		{"empty edges", "tikhub-posts", `{"data":{"edges":[]}}`, AccountMonitorResponseSummary{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := ValidateAccountMonitorResponse(tt.policy, []byte(tt.body))
			require.NoError(t, err)
			assert.Equal(t, tt.want, summary)
		})
	}
	_, err := ValidateAccountMonitorResponse("unknown", []byte(`{"items":[]}`))
	require.Error(t, err)

	adaptor, info := monitorResponseAdaptor(t, "x-user", "")
	body := `{"code":500,"message":"legacy payload"}`
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(body))}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	usage, apiErr := adaptor.DoResponse(c, response, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, body, recorder.Body.String())
}
