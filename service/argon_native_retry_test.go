package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const argonAuthDenial = `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`

func argonRetryContext(t *testing.T) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-4-argon:generateContent", nil)
	require.True(t, ConfigureArgonNativeRetry(c, &relaycommon.RelayInfo{RelayFormat: types.RelayFormatGemini, OriginModelName: "gemini-4-argon", UsingGroup: "gemini-4-argon-multimodal"}))
	return c
}

func observeArgonAuth(c *gin.Context) {
	ObserveArgonNativeRejection(c, &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(argonAuthDenial))})
}

func TestConfigureArgonNativeRetryIsExact(t *testing.T) {
	for _, test := range []struct {
		name, model, group, method, path string
		format                           types.RelayFormat
		want                             bool
	}{
		{"native", "gemini-4-argon", "gemini-4-argon-multimodal", "POST", "/v1beta/models/gemini-4-argon:generateContent", types.RelayFormatGemini, true},
		{"native-stream", "gemini-4-argon", "gemini-4-argon-multimodal", "POST", "/v1/models/gemini-4-argon:streamGenerateContent", types.RelayFormatGemini, true},
		{"old-model", "gemini-3.8-flash", "gemini-4-argon-multimodal", "POST", "/v1beta/models/gemini-3.8-flash:generateContent", types.RelayFormatGemini, false},
		{"other-group", "gemini-4-argon", "default", "POST", "/v1beta/models/gemini-4-argon:generateContent", types.RelayFormatGemini, false},
		{"other-protocol", "gemini-4-argon", "gemini-4-argon-multimodal", "POST", "/v1beta/models/gemini-4-argon:generateContent", types.RelayFormatOpenAI, false},
		{"count", "gemini-4-argon", "gemini-4-argon-multimodal", "POST", "/v1beta/models/gemini-4-argon:countTokens", types.RelayFormatGemini, false},
		{"get", "gemini-4-argon", "gemini-4-argon-multimodal", "GET", "/v1beta/models/gemini-4-argon:generateContent", types.RelayFormatGemini, false},
		{"prefix", "gemini-4-argon-preview", "gemini-4-argon-multimodal", "POST", "/v1beta/models/gemini-4-argon-preview:generateContent", types.RelayFormatGemini, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(test.method, test.path, nil)
			assert.Equal(t, test.want, ConfigureArgonNativeRetry(c, &relaycommon.RelayInfo{RelayFormat: test.format, OriginModelName: test.model, UsingGroup: test.group}))
		})
	}
}

func TestArgonNativeRejectionRequiresCompleteUnmixedSupplierDenial(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		want       bool
	}{
		{"apikeyfun-observed", argonAuthDenial, 401, true},
		{"evolink-observed", `{"error":{"code":"unauthorized","message":"Invalid API key (request id: 20261017114438443672418VlN01QDy)","param":null,"type":"authentication_error"}}`, 401, true},
		{"evolink-documented", `{"error":{"code":401,"message":"Invalid or expired token","type":"authentication_error"}}`, 401, true},
		{"raw-server", `{"error":{"code":500,"message":"unknown","status":"INTERNAL"}}`, 500, false},
		{"rate-without-proof", `{"error":{"code":429,"message":"quota","status":"RESOURCE_EXHAUSTED"}}`, 429, false},
		{"mapped-server", argonAuthDenial, 500, false},
		{"different-message", `{"error":{"code":401,"message":"Generation accepted but delivery failed","status":"UNAUTHENTICATED"}}`, 401, false},
		{"candidates", `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"},"candidates":[{}]}`, 401, false},
		{"usage", `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"},"usageMetadata":{"totalTokenCount":1}}`, 401, false},
		{"zero-usage", `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"},"usageMetadata":{"totalTokenCount":0}}`, 401, false},
		{"accepted-id", `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED","id":"accepted"}}`, 401, false},
		{"duplicate-envelope", `{"error":{"responseId":"accepted"},"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`, 401, false},
		{"duplicate-code", `{"error":{"code":200,"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`, 401, false},
		{"truncated", strings.TrimSuffix(argonAuthDenial, "}"), 401, false},
		{"trailing-data", argonAuthDenial + `{"candidates":[]}`, 401, false},
		{"proxy-html", `<html>401 Unauthorized</html>`, 401, false},
	} {
		t.Run(test.name, func(t *testing.T) { assert.Equal(t, test.want, isArgonNativeRejection(test.status, []byte(test.body))) })
	}
}

type argonInterruptedReader struct{ body string }

func (r *argonInterruptedReader) Read(p []byte) (int, error) {
	n := copy(p, r.body)
	r.body = r.body[n:]
	if r.body == "" {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

func TestObserveArgonNativeRejectionIsBoundedAndReadFailureStops(t *testing.T) {
	for _, test := range []struct {
		name, contentType string
		reader            io.Reader
	}{
		{"read-error", "application/json", &argonInterruptedReader{body: argonAuthDenial}},
		{"oversized", "application/json", strings.NewReader(argonAuthDenial + strings.Repeat(" ", 64<<10))},
		{"wrong-content-type", "text/html", strings.NewReader(argonAuthDenial)},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := argonRetryContext(t)
			resp := &http.Response{StatusCode: 401, Header: http.Header{"Content-Type": []string{test.contentType}}, Body: io.NopCloser(test.reader)}
			ObserveArgonNativeRejection(c, resp)
			assert.False(t, argonNativeRetry(c).rejectionConfirmed)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.LessOrEqual(t, len(body), (64<<10)+1)
		})
	}
}

func TestArgonNativeRetryResetsEvidenceAndHonorsExistingConstraints(t *testing.T) {
	err := types.NewOpenAIError(errors.New("authentication rejected"), types.ErrorCodeBadResponseStatusCode, 401)
	c := argonRetryContext(t)
	assert.Equal(t, "acceptance_unknown", DecideArgonNativeRetry(c, err, 2).Reason)
	observeArgonAuth(c)
	assert.Equal(t, "retry", DecideArgonNativeRetry(c, err, 2).Action)
	BeginArgonNativeAttempt(c)
	assert.Equal(t, "acceptance_unknown", DecideArgonNativeRetry(c, err, 1).Reason)
	for _, test := range []struct {
		name    string
		setup   func(*gin.Context)
		retries int
		reason  string
	}{
		{"budget", func(*gin.Context) {}, 0, "attempt_budget_exhausted"},
		{"single-pin", func(c *gin.Context) {
			GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, RetryMode: dto.PinRetrySingleAttempt})
		}, 2, "pinned_channel"},
		{"same-channel-pin", func(c *gin.Context) {
			GetChannelConstraints(c).AddPin(dto.ChannelPin{ChannelId: 1, RetryMode: dto.PinRetrySameChannel})
		}, 2, "pinned_channel"},
		{"strict", func(c *gin.Context) { c.Set(ginKeyChannelAffinitySkipRetry, true) }, 2, "strict_session"},
		{"written", func(c *gin.Context) { c.Writer.WriteHeaderNow() }, 2, "request_already_started_or_cancelled"},
		{"cancelled", func(c *gin.Context) {
			ctx, cancel := context.WithCancel(c.Request.Context())
			cancel()
			c.Request = c.Request.WithContext(ctx)
		}, 2, "request_already_started_or_cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := argonRetryContext(t)
			observeArgonAuth(c)
			test.setup(c)
			decision := DecideArgonNativeRetry(c, err, test.retries)
			assert.Equal(t, "stop", decision.Action)
			assert.Equal(t, test.reason, decision.Reason)
		})
	}
	keyErr := types.NewError(errors.New("no key"), types.ErrorCodeChannelNoAvailableKey)
	local := argonRetryContext(t)
	assert.Equal(t, "acceptance_unknown", DecideArgonNativeRetry(local, keyErr, 1).Reason)
	MarkArgonNativeNotSent(local)
	assert.Equal(t, "retry", DecideArgonNativeRetry(local, keyErr, 1).Action)
	BeginArgonNativeAttempt(local)
	assert.Equal(t, "acceptance_unknown", DecideArgonNativeRetry(local, keyErr, 1).Reason)
	assert.Equal(t, "stop", DecideArgonNativeRetry(argonRetryContext(t), keyErr, 0).Action)
	spoof := types.WithOpenAIError(types.OpenAIError{Code: string(types.ErrorCodeChannelNoAvailableKey), Message: "not a local error"}, 500)
	MarkArgonNativeNotSent(local)
	assert.Equal(t, "acceptance_unknown", DecideArgonNativeRetry(local, spoof, 1).Reason)
	plain, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, DecideRelayRetry(plain, err, 1), DecideArgonNativeRetry(plain, err, 1))
}
