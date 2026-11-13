package service

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const argonNativeRetryContextKey = "argon_native_retry"

type argonNativeRetryState struct {
	rejectionConfirmed bool
	localNotSent       bool
}

// ConfigureArgonNativeRetry confines the fork's four-route policy to the
// audited model, selected supplier group and native generation endpoints.
func ConfigureArgonNativeRetry(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil || info == nil ||
		info.RelayFormat != types.RelayFormatGemini || info.OriginModelName != "gemini-4-argon" ||
		info.UsingGroup != "gemini-4-argon-multimodal" || c.Request.Method != http.MethodPost {
		return false
	}
	path := c.Request.URL.Path
	for _, version := range []string{"v1", "v1beta"} {
		prefix := "/" + version + "/models/gemini-4-argon:"
		if path == prefix+"generateContent" || path == prefix+"streamGenerateContent" {
			c.Set(argonNativeRetryContextKey, &argonNativeRetryState{})
			return true
		}
	}
	return false
}

func argonNativeRetry(c *gin.Context) *argonNativeRetryState {
	if c != nil {
		if value, found := c.Get(argonNativeRetryContextKey); found {
			state, _ := value.(*argonNativeRetryState)
			return state
		}
	}
	return nil
}

// ArgonNativeRetryConfigured reports the server-created exact-scope state.
func ArgonNativeRetryConfigured(c *gin.Context) bool {
	return argonNativeRetry(c) != nil
}

func BeginArgonNativeAttempt(c *gin.Context) {
	if state := argonNativeRetry(c); state != nil {
		state.rejectionConfirmed = false
		state.localNotSent = false
	}
}

// MarkArgonNativeNotSent is called only for a failed local key selection,
// before the native handler or transport has run for the selected candidate.
func MarkArgonNativeNotSent(c *gin.Context) {
	if state := argonNativeRetry(c); state != nil {
		state.localNotSent = true
	}
}

// ObserveArgonNativeRejection preserves only bounded, completely read native
// rejection evidence before the shared error decoder and status-code mapping.
// Unknown formats and all transport failures remain unsafe to replay.
func ObserveArgonNativeRejection(c *gin.Context, resp *http.Response) {
	state := argonNativeRetry(c)
	if state == nil || resp == nil || resp.Body == nil {
		return
	}
	const maxRejectionBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRejectionBytes+1))
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil || len(body) > maxRejectionBytes ||
		!strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		return
	}
	state.rejectionConfirmed = isArgonNativeRejection(resp.StatusCode, body)
}

func isArgonNativeRejection(status int, body []byte) bool {
	// These 401 shapes match the suppliers' documented/observed authentication
	// rejection. A bare status, generic 429/5xx or mixed error/result is unknown.
	if status != http.StatusUnauthorized {
		return false
	}
	var envelope map[string]common.RawMessage
	if common.Unmarshal(body, &envelope) != nil || len(envelope) != 1 || len(envelope["error"]) == 0 {
		return false
	}
	fieldCount := 0
	gjson.ParseBytes(body).ForEach(func(_, _ gjson.Result) bool { fieldCount++; return true })
	if fieldCount != len(envelope) {
		return false
	}
	var fields map[string]common.RawMessage
	if common.Unmarshal(envelope["error"], &fields) != nil {
		return false
	}
	fieldCount = 0
	gjson.ParseBytes(envelope["error"]).ForEach(func(_, _ gjson.Result) bool { fieldCount++; return true })
	if fieldCount != len(fields) {
		return false
	}
	var code int
	var nativeStatus, errorType, codeName, message string
	if common.Unmarshal(fields["message"], &message) != nil {
		return false
	}
	if len(fields) == 3 && common.Unmarshal(fields["code"], &code) == nil && code == status {
		if common.Unmarshal(fields["status"], &nativeStatus) == nil {
			return nativeStatus == "UNAUTHENTICATED" && message == "Invalid API key"
		}
		return common.Unmarshal(fields["type"], &errorType) == nil && errorType == "authentication_error" && message == "Invalid or expired token"
	}
	if len(fields) != 4 || common.Unmarshal(fields["code"], &codeName) != nil || codeName != "unauthorized" ||
		common.Unmarshal(fields["type"], &errorType) != nil || errorType != "authentication_error" ||
		!bytes.Equal(bytes.TrimSpace(fields["param"]), []byte("null")) {
		return false
	}
	if message == "Invalid API key" {
		return true
	}
	requestID, found := strings.CutPrefix(message, "Invalid API key (request id: ")
	if !found || !strings.HasSuffix(requestID, ")") {
		return false
	}
	requestID = strings.TrimSuffix(requestID, ")")
	if len(requestID) == 0 || len(requestID) > 128 {
		return false
	}
	for _, char := range requestID {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
			return false
		}
	}
	return true
}

// DecideArgonNativeRetry only narrows the existing decision. Every attempt
// starts unknown; a prior rejection cannot authorize replay of a later request.
func DecideArgonNativeRetry(c *gin.Context, err *types.NewAPIError, retries int) PolicyDecision {
	decision := DecideRelayRetry(c, err, retries)
	state := argonNativeRetry(c)
	if state == nil || decision.Action != "retry" {
		return decision
	}
	stop := func(reason string) PolicyDecision {
		return PolicyDecision{Action: "stop", Reason: reason, Source: "argon_native"}
	}
	if retries <= 0 {
		return stop("attempt_budget_exhausted")
	}
	if _, pinned, _ := GetChannelConstraints(c).ResolvedPin(); pinned {
		return stop("pinned_channel")
	}
	if c.Request.Context().Err() != nil || c.Writer.Written() {
		return stop("request_already_started_or_cancelled")
	}
	if state.localNotSent && err.GetErrorType() == types.ErrorTypeNewAPIError && err.GetErrorCode() == types.ErrorCodeChannelNoAvailableKey {
		return decision
	}
	if !state.rejectionConfirmed {
		return stop("acceptance_unknown")
	}
	return decision
}
