package common

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeURLForLogMasksSensitiveQueryValues(t *testing.T) {
	rawURL := "https://example.test/v1beta/models/gemini:streamGenerateContent?alt=sse&key=sk-secret&access_token=ya29-secret&api-version=2024-02-01"

	got := SanitizeURLForLog(rawURL)

	assert.NotContains(t, got, "sk-secret")
	assert.NotContains(t, got, "ya29-secret")
	parsedURL, err := url.Parse(got)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "***masked***", query.Get("key"))
	assert.Equal(t, "***masked***", query.Get("access_token"))
	assert.Equal(t, "sse", query.Get("alt"))
	assert.Equal(t, "2024-02-01", query.Get("api-version"))
}

func TestSanitizeURLForLogMasksAWSAndSecretLikeQueryKeys(t *testing.T) {
	rawURL := "https://example.test/path?X-Amz-Credential=credential&X-Amz-Signature=signature&session_token=session&client_secret=secret&model=gpt-test"

	got := SanitizeURLForLog(rawURL)

	assert.NotContains(t, got, "X-Amz-Credential=credential")
	assert.NotContains(t, got, "X-Amz-Signature=signature")
	assert.NotContains(t, got, "session_token=session")
	assert.NotContains(t, got, "client_secret=secret")
	parsedURL, err := url.Parse(got)
	require.NoError(t, err)
	query := parsedURL.Query()
	assert.Equal(t, "***masked***", query.Get("X-Amz-Credential"))
	assert.Equal(t, "***masked***", query.Get("X-Amz-Signature"))
	assert.Equal(t, "***masked***", query.Get("session_token"))
	assert.Equal(t, "***masked***", query.Get("client_secret"))
	assert.Equal(t, "gpt-test", query.Get("model"))
}

func TestSanitizeURLForLogKeepsURLWithoutSensitiveQuery(t *testing.T) {
	rawURL := "https://example.test/v1/chat/completions?api-version=2024-02-01&alt=sse"

	got := SanitizeURLForLog(rawURL)

	assert.Equal(t, rawURL, got)
}

func TestValidateMultipartDirectNormalizesImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           string
		expectedImages []string
	}{
		{
			name:           "string image url with whitespace",
			body:           `{"model":"wan2.7-i2v","prompt":"animate","image":" https://example.com/first.png "}`,
			expectedImages: []string{"https://example.com/first.png"},
		},
		{
			name:           "object image url (xAI / grok format)",
			body:           `{"model":"grok-imagine-video-1.5","prompt":"animate","image":{"url":"https://example.com/grok.png"}}`,
			expectedImages: []string{"https://example.com/grok.png"},
		},
		{
			name:           "array of object images",
			body:           `{"model":"kling-v3","prompt":"animate","images":[{"url":"https://example.com/1.png"},{"image_url":"https://example.com/2.png"}]}`,
			expectedImages: []string{"https://example.com/1.png", "https://example.com/2.png"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = request
			info := &RelayInfo{
				TaskRelayInfo: &TaskRelayInfo{},
			}

			taskErr := ValidateMultipartDirect(context, info)
			require.Nil(t, taskErr)
			storedReq, err := GetTaskRequest(context)
			require.NoError(t, err)
			require.Equal(t, tc.expectedImages, storedReq.Images)
			require.Equal(t, constant.TaskActionImageToVideo, info.Action)
		})
	}
}

// TestTaskDurationBounds guards the billing invariant that user-supplied
// video duration (a quota multiplier via OtherRatio "seconds") is bounded, so
// it can never overflow quota calculation into a negative charge.
func TestTaskDurationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(t *testing.T, body string) (*gin.Context, *RelayInfo) {
		request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = request
		return context, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}
	}

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "huge duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":9999999999}`,
			wantErr: true,
		},
		{
			name:    "huge seconds string is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","seconds":"9999999999"}`,
			wantErr: true,
		},
		{
			name:    "negative duration is rejected",
			body:    `{"model":"sora-2","prompt":"a cat","duration":-8}`,
			wantErr: true,
		},
		{
			name: "normal duration is accepted",
			body: `{"model":"sora-2","prompt":"a cat","seconds":"8"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (multipart direct)", func(t *testing.T) {
			context, info := newContext(t, tt.body)
			taskErr := ValidateMultipartDirect(context, info)
			if tt.wantErr {
				require.NotNil(t, taskErr)
				require.Equal(t, "invalid_seconds", taskErr.Code)
			} else {
				require.Nil(t, taskErr)
			}
		})
		t.Run(tt.name+" (basic task request)", func(t *testing.T) {
			context, info := newContext(t, tt.body)
			taskErr := ValidateBasicTaskRequest(context, info, constant.TaskActionImageToVideo)
			if tt.wantErr {
				require.NotNil(t, taskErr)
				require.Equal(t, "invalid_seconds", taskErr.Code)
			} else {
				require.Nil(t, taskErr)
			}
		})
	}
}

func TestValidateGeminiModelPathAndAPIVersion(t *testing.T) {
	m, act, ok := ValidateGeminiModelPath("/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse")
	require.True(t, ok)
	assert.Equal(t, "gemini-2.5-flash", m)
	assert.Equal(t, "streamGenerateContent", act)

	for _, bad := range []string{
		"/v1beta/models/../../admin/keys:generateContent",
		"/v1beta/models/gemini-2.5-flash/extra:generateContent",
		"/v1beta/models/gemini-2.5-flash:evilAction",
		"/v1beta/models/gemini-2.5-flash%2f..:generateContent",
		"/v1/models/gemini-2.5-flash:generateContent#frag",
		"/v1beta/models/:generateContent",
	} {
		_, _, ok := ValidateGeminiModelPath(bad)
		assert.False(t, ok, "path %q must be rejected", bad)
	}

	assert.True(t, ValidateRelayURLPath("/v1/chat/completions"))
	assert.False(t, ValidateRelayURLPath("/v1/../admin"))
	assert.False(t, ValidateRelayURLPath("//evil.example/v1"))
	assert.False(t, ValidateRelayURLPath("/v1/chat\\completions"))

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("api_version", "2024-02-01")
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?api-version=2024-02-01%26injected=1", nil)
	assert.Equal(t, "2024-02-01", GetAPIVersion(ctx), "injected query api-version must fall back to channel default")

	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?api-version=2025-01-01-preview", nil)
	assert.Equal(t, "2025-01-01-preview", GetAPIVersion(ctx))
}
