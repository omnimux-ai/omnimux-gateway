package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A local account/subscription quota rejection must reach the client as a 403
// with a clear message, must not be retried against another channel, and must
// not be recorded as an upstream channel failure.
func TestTaskQuotaErrorStopsRetryAndResponds403(t *testing.T) {
	require.NoError(t, i18n.Init())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))

	apiErr := types.NewErrorWithStatusCode(
		errors.New("用户剩余额度: 0.000000, 需要预扣费额度: 500.000000"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)
	taskErr := service.TaskErrorFromAPIError(c, apiErr)
	require.NotNil(t, taskErr)
	assert.Equal(t, string(types.ErrorCodeInsufficientUserQuota), taskErr.Code)
	assert.Equal(t, http.StatusForbidden, taskErr.StatusCode)
	assert.True(t, taskErr.LocalError, "a local quota rejection is not a channel failure")
	assert.NotContains(t, taskErr.Message, "0.000000")
	assert.NotContains(t, taskErr.Message, "500.000000")
	assert.Contains(t, taskErr.Message, i18n.T(c, i18n.MsgQuotaInsufficientForTask))

	assert.Equal(t, service.PolicyDecision{Action: "stop", Reason: "non_retryable_error", Source: "system"},
		decideTaskRetry(c, taskErr, 3), "insufficient quota must not retry another channel")

	adapted := taskSubmissionAPIError(taskErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, adapted.GetErrorCode())
	assert.True(t, types.IsSkipRetryError(adapted))
	assert.Equal(t, http.StatusForbidden, adapted.StatusCode)

	respondTaskSubmissionError(c, taskErr)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `"code":"insufficient_user_quota"`)
	assert.Contains(t, body, i18n.T(c, i18n.MsgQuotaInsufficientForTask))
}

// When the retry group runs out of channels, the client must see the last real
// attempt error, not a generic "no available channel" that hides the cause.
func TestTaskChannelSelectionErrorKeepsLastAttempt(t *testing.T) {
	last := &taskdto.TaskError{
		Code:       string(types.ErrorCodeBadResponseStatusCode),
		Message:    "upstream returned 502",
		StatusCode: http.StatusBadGateway,
	}
	channelErr := types.NewErrorWithStatusCode(
		errors.New("no available channel"),
		types.ErrorCodeGetChannelFailed,
		http.StatusServiceUnavailable,
		types.ErrOptionWithSkipRetry(),
	)

	assert.Same(t, last, taskChannelSelectionError(last, channelErr, 1),
		"retry exhaustion must keep the last real attempt error")

	first := taskChannelSelectionError(nil, channelErr, 0)
	require.NotNil(t, first)
	assert.Equal(t, "get_channel_failed", first.Code)
	assert.Equal(t, http.StatusServiceUnavailable, first.StatusCode)
	assert.True(t, first.LocalError)
}
