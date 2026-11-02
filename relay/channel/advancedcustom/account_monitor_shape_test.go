package advancedcustom

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func shapeContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, recorder
}

func TestAccountMonitorShapeEnableIsPrivateAndModelBound(t *testing.T) {
	assert.False(t, EnableAccountMonitorShapeDiagnostic(nil, "tiktok-user"))
	assert.Nil(t, GetAccountMonitorShapeEvidence(nil))
	for _, model := range []string{"", "x-user", "tiktok-posts", "other", " tiktok-user", "x-posts "} {
		c, _ := shapeContext()
		assert.False(t, EnableAccountMonitorShapeDiagnostic(c, model))
		assert.Nil(t, accountMonitorShapeCollectorFromContext(c))
	}
	c, _ := shapeContext()
	c.Request.Header.Set("shape_diagnostic", "true")
	c.Request.Header.Set(accountMonitorShapeContextKey, "true")
	assert.Nil(t, accountMonitorShapeCollectorFor(c, "tiktok-user", "tikhub-user"))
	c.Set(accountMonitorShapeContextKey, map[string]any{"model": "tiktok-user"})
	assert.Nil(t, accountMonitorShapeCollectorFromContext(c))
	assert.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
	assert.NotNil(t, accountMonitorShapeCollectorFor(c, "tiktok-user", "tikhub-user"))
	assert.Nil(t, accountMonitorShapeCollectorFor(c, "x-user", "tikhub-user"))
	assert.Nil(t, accountMonitorShapeCollectorFor(c, "tiktok-user", "tikhub-posts"))
	assert.Nil(t, accountMonitorShapeCollectorFor(c, "tiktok-user", ""))
	assert.Nil(t, GetAccountMonitorShapeEvidence(c))
}

func TestAccountMonitorShapeClosedTypesAndContainerEmptiness(t *testing.T) {
	for _, tt := range []struct {
		name, raw, kind string
		present         bool
		nonempty        *bool
	}{
		{"missing", "", "missing", false, nil},
		{"null", "null", "null", true, nil},
		{"false", "false", "boolean", true, nil},
		{"true", "true", "boolean", true, nil},
		{"zero", "0", "number", true, nil},
		{"string", `"sentinel-secret"`, "string", true, nil},
		{"empty object", `{  }`, "object", true, shapeBool(false)},
		{"object", `{"sentinel-key":"sentinel-secret"}`, "object", true, shapeBool(true)},
		{"empty array", `[  ]`, "array", true, shapeBool(false)},
		{"null item", `[null]`, "array", true, shapeBool(true)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			slot := accountMonitorShapeSlot(gjson.Parse(tt.raw))
			assert.Equal(t, AccountMonitorShapeSlot{Present: tt.present, Type: tt.kind, Nonempty: tt.nonempty}, slot)
			encoded, err := common.Marshal(slot)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), "sentinel")
		})
	}
}

func shapeBool(value bool) *bool { return &value }

func TestAccountMonitorTikTokShapeKeepsSchemaOneForUnrecognizedValues(t *testing.T) {
	const sentinel = "private-shape-sentinel"
	body := `{"code":200,"data":{"userInfo":{"user":{"nickname":" ","signature":"\t","avatarLarger":"private-shape-sentinel"},"stats":{"followerCount":-12345,"videoCount":-1}},"user_info":{"nickname":"private-shape-sentinel","follower_count":456},"unknown-private-shape-sentinel":{"deep":{"nickname":"private-shape-sentinel"}}}}`
	c, recorder := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
	adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
	usage, apiErr := adaptor.DoResponse(c, shapeResponse(body), info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.Equal(t, "account monitoring upstream profile is unrecognized", apiErr.Error())
	assert.Empty(t, recorder.Body.String())
	evidence := GetAccountMonitorShapeEvidence(c)
	require.NotNil(t, evidence)
	assert.Equal(t, accountMonitorShapeObserved, evidence.Status)
	assert.Len(t, evidence.Slots, 22)
	assert.Equal(t, "object", evidence.Slots[3].Type)
	assert.Equal(t, "string", evidence.Slots[6].Type)
	assert.Equal(t, "number", evidence.Slots[9].Type)
	assert.Equal(t, "number", evidence.Slots[10].Type)
	assert.Equal(t, "missing", evidence.Slots[11].Type)
	assert.Equal(t, "object", evidence.Slots[16].Type)
	assert.Equal(t, "string", evidence.Slots[17].Type)
	encoded, err := common.Marshal(evidence)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(encoded), accountMonitorShapeMaxBytes)
	assert.NotContains(t, string(encoded), sentinel)
	assert.NotContains(t, apiErr.Error(), sentinel)
	assert.NotContains(t, string(encoded), "nickname")
	assert.NotContains(t, string(encoded), "12345")
	collector := accountMonitorShapeCollectorFromContext(c)
	require.NotNil(t, collector)
	// The internal collector has only a fixed model and typed final evidence.
	assert.Equal(t, "tiktok-user", collector.model)
	assert.Same(t, evidence, collector.evidence)
	_, validationErr := ValidateAccountMonitorResponse("tikhub-user", []byte(body))
	require.Error(t, validationErr)
	assert.Equal(t, validationErr.Error(), apiErr.Error())
}

func TestAccountMonitorShapeDoesNotUnwrapOrDiscoverUnknownPaths(t *testing.T) {
	for _, body := range []string{
		`{"data":{"data":{"userInfo":{"user":{"nickname":"sentinel"}}}}}`,
		`{"data":{"unknown":{"userInfo":{"user":{"nickname":"sentinel"}}}}}`,
		`{"data":{"unknown":[{"nickname":"sentinel"}],"items":[null,{"userInfo":{"user":{"nickname":"sentinel"}}}]}}`,
	} {
		c, _ := shapeContext()
		require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
		adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
		usage, apiErr := adaptor.DoResponse(c, shapeResponse(body), info)
		require.NotNil(t, apiErr)
		assert.Nil(t, usage)
		evidence := GetAccountMonitorShapeEvidence(c)
		require.NotNil(t, evidence)
		assert.Equal(t, "missing", evidence.Slots[3].Type)
		assert.Equal(t, "missing", evidence.Slots[6].Type)
	}
}

func TestAccountMonitorXInstructionHypothesesRemainUnrecognized(t *testing.T) {
	body := `{"data":{"timeline":{"instructions":[{"entries":[{"content":{"itemContent":{"tweet_results":{"result":{"rest_id":"private-shape-sentinel","legacy":{"full_text":"private-shape-sentinel"}}}}}}]}]}}}`
	c, recorder := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "x-posts"))
	adaptor, info := monitorResponseAdaptor(t, "x-posts", "tikhub-posts")
	usage, apiErr := adaptor.DoResponse(c, shapeResponse(body), info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.Empty(t, recorder.Body.String())
	assert.Equal(t, "account monitoring upstream posts are unrecognized", apiErr.Error())
	evidence := GetAccountMonitorShapeEvidence(c)
	require.NotNil(t, evidence)
	assert.Equal(t, 3, evidence.SchemaVersion)
	require.Len(t, evidence.Slots, 32)
	assert.Equal(t, "object", evidence.Slots[15].Type)
	assert.Equal(t, "missing", evidence.Slots[23].Type)
	encoded, err := common.Marshal(evidence)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "private-shape-sentinel")
	assert.LessOrEqual(t, len(encoded), accountMonitorShapeMaxBytes)
}

func TestAccountMonitorShapeOnlyMatchingUnrecognizedFailures(t *testing.T) {
	for _, tt := range []struct {
		name, model, policy, body string
		status                    int
		success                   bool
	}{
		{"root business", "tiktok-user", "tikhub-user", `{"success":false,"data":{"userInfo":{"user":{"nickname":"sentinel"}}}}`, 200, false},
		{"profile business", "tiktok-user", "tikhub-user", `{"data":{"user":{"success":false,"nickname":"sentinel"}}}`, 200, false},
		{"profile nested business", "tiktok-user", "tikhub-user", `{"data":{"result":{"user":{"error":{"message":"sentinel"},"nickname":"sentinel"}}}}`, 200, false},
		{"array selected business", "tiktok-user", "tikhub-user", `{"data":{"items":[null,{"ok":false,"nickname":"sentinel"}]}}`, 200, false},
		{"post business", "x-posts", "tikhub-posts", `{"data":{"result":{"status_code":500,"timeline":{"instructions":[]}}}}`, 200, false},
		{"invalid JSON", "tiktok-user", "tikhub-user", `{"data":{"userInfo":"sentinel"}`, 200, false},
		{"scalar JSON", "tiktok-user", "tikhub-user", `"sentinel"`, 200, false},
		{"HTTP failure", "tiktok-user", "tikhub-user", `{"data":{"nickname":"sentinel"}}`, 502, false},
		{"profile success", "tiktok-user", "tikhub-user", `{"data":{"user":{"nickname":"sentinel"}}}`, 200, true},
		{"zero followers", "tiktok-user", "tikhub-user", `{"data":{"follower_count":0}}`, 200, true},
		{"empty posts success", "x-posts", "tikhub-posts", `{"data":{"items":[]}}`, 200, true},
		{"parent identity success", "x-posts", "tikhub-posts", `{"data":{"items":[{"id":"sentinel","node":{}}]}}`, 200, true},
		{"node identity success", "x-posts", "tikhub-posts", `{"data":{"edges":[{"node":{"id":"sentinel"}}]}}`, 200, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := shapeContext()
			require.True(t, EnableAccountMonitorShapeDiagnostic(c, tt.model))
			adaptor, info := monitorResponseAdaptor(t, tt.model, tt.policy)
			response := shapeResponse(tt.body)
			response.StatusCode = tt.status
			usage, apiErr := adaptor.DoResponse(c, response, info)
			assert.Nil(t, GetAccountMonitorShapeEvidence(c))
			if tt.success {
				require.Nil(t, apiErr)
				assert.NotNil(t, usage)
			} else {
				require.NotNil(t, apiErr)
				assert.Nil(t, usage)
				assert.NotContains(t, apiErr.Error(), "sentinel")
			}
		})
	}
}

func TestAccountMonitorShapeDoesNotChangeValidationResults(t *testing.T) {
	for _, tt := range []struct{ model, policy, body string }{
		{"tiktok-user", "tikhub-user", `{"data":{"userInfo":{"user":{"nickname":"sentinel"}}}}`},
		{"tiktok-user", "tikhub-user", `{"data":{"items":[null,{"nickname":"sentinel"}]}}`},
		{"tiktok-user", "tikhub-user", `{"data":{"user":{"ok":false,"nickname":"sentinel"}}}`},
		{"x-posts", "tikhub-posts", `{"data":{"timeline":{"instructions":[]}}}`},
		{"x-posts", "tikhub-posts", `{"data":{"items":[null,{"id":"sentinel"}]}}`},
		{"x-posts", "tikhub-posts", `{"data":{"items":[{"id":"sentinel","node":{}}]}}`},
		{"x-posts", "tikhub-posts", `{"data":{"items":[{"node":{"node":{"id":"sentinel"}}}]}}`},
		{"x-posts", "tikhub-posts", `{"data":{"items":[]}}`},
		{"x-posts", "tikhub-posts", `[ {"id":"sentinel"} ]`},
	} {
		var previousError string
		var previousBody string
		var previousUsage bool
		for _, enabled := range []bool{false, true} {
			c, recorder := shapeContext()
			if enabled {
				require.True(t, EnableAccountMonitorShapeDiagnostic(c, tt.model))
			}
			adaptor, info := monitorResponseAdaptor(t, tt.model, tt.policy)
			usage, apiErr := adaptor.DoResponse(c, shapeResponse(tt.body), info)
			errText := ""
			if apiErr != nil {
				errText = apiErr.Error()
			}
			if !enabled {
				assert.Nil(t, GetAccountMonitorShapeEvidence(c))
				previousError, previousBody, previousUsage = errText, recorder.Body.String(), usage != nil
			} else {
				assert.Equal(t, previousError, errText)
				assert.Equal(t, previousBody, recorder.Body.String())
				assert.Equal(t, previousUsage, usage != nil)
			}
		}
	}
}

type shapeReadCloser struct {
	reader   io.Reader
	read     int
	closes   int
	readErr  error
	closeErr error
	onRead   func()
}

func (r *shapeReadCloser) Read(p []byte) (int, error) {
	if r.onRead != nil {
		r.onRead()
	}
	if r.readErr != nil {
		return 0, r.readErr
	}
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}

func (r *shapeReadCloser) Close() error {
	r.closes++
	return r.closeErr
}

func shapeResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestAccountMonitorShapeReadsOnceAndStopsAtBodyBoundary(t *testing.T) {
	base := `{"data":{"userInfo":{"user":{"nickname":"sentinel"}}}}`
	for _, extra := range []int{-1, 0, 1, 1000} {
		length := accountMonitorShapeMaxBody + extra
		body := base + strings.Repeat(" ", length-len(base))
		c, recorder := shapeContext()
		require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
		adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
		reader := &shapeReadCloser{reader: strings.NewReader(body)}
		response := shapeResponse("")
		response.Body = reader
		usage, apiErr := adaptor.DoResponse(c, response, info)
		require.NotNil(t, apiErr)
		assert.Nil(t, usage)
		assert.Empty(t, recorder.Body.String())
		assert.Equal(t, 1, reader.closes)
		assert.LessOrEqual(t, reader.read, accountMonitorShapeMaxBody+1)
		evidence := GetAccountMonitorShapeEvidence(c)
		require.NotNil(t, evidence)
		if extra > 0 {
			assert.Equal(t, accountMonitorShapeTooLarge, evidence.Status)
			assert.Empty(t, evidence.Slots)
			assert.Equal(t, "account monitoring diagnostic response exceeded limit", apiErr.Error())
		} else {
			assert.Equal(t, accountMonitorShapeObserved, evidence.Status)
			assert.Equal(t, length, reader.read)
		}
		assert.True(t, types.IsSkipRetryError(apiErr))
	}
	// A disabled probe, mismatched model and unconfigured route retain the
	// normal read behavior rather than silently receiving the diagnostic cap.
	for _, tt := range []struct{ model, policy, enable string }{
		{"tiktok-user", "tikhub-user", ""},
		{"x-user", "tikhub-user", "tiktok-user"},
		{"x-posts", "", "x-posts"},
	} {
		c, _ := shapeContext()
		if tt.enable != "" {
			require.True(t, EnableAccountMonitorShapeDiagnostic(c, tt.enable))
		}
		body := base + strings.Repeat(" ", accountMonitorShapeMaxBody+1)
		reader := &shapeReadCloser{reader: strings.NewReader(body)}
		response := shapeResponse("")
		response.Body = reader
		adaptor, info := monitorResponseAdaptor(t, tt.model, tt.policy)
		_, _ = adaptor.DoResponse(c, response, info)
		assert.Equal(t, len(body), reader.read)
		assert.Nil(t, GetAccountMonitorShapeEvidence(c))
	}
}

func TestAccountMonitorShapeReadFailureAndCancellationKeepNoEvidence(t *testing.T) {
	for _, tt := range []struct {
		name        string
		readErr     error
		cancelEarly bool
		cancelRead  bool
	}{
		{"reader error", errors.New("private-reader-sentinel"), false, false},
		{"cancel before read", nil, true, false},
		{"cancel during read", nil, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := shapeContext()
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
			reader := &shapeReadCloser{reader: strings.NewReader(`{"data":{"userInfo":{"nickname":"private-reader-sentinel"}}}`), readErr: tt.readErr}
			if tt.cancelEarly {
				cancel()
			}
			if tt.cancelRead {
				reader.onRead = cancel
			}
			response := shapeResponse("")
			response.Body = reader
			adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
			usage, apiErr := adaptor.DoResponse(c, response, info)
			require.NotNil(t, apiErr)
			assert.Nil(t, usage)
			assert.Nil(t, GetAccountMonitorShapeEvidence(c))
			assert.Empty(t, recorder.Body.String())
			assert.Equal(t, "account monitoring diagnostic response read failed", apiErr.Error())
			assert.NotContains(t, apiErr.Error(), "private-reader-sentinel")
			assert.Equal(t, 1, reader.closes)
		})
	}
}

func TestAccountMonitorShapeSummaryBoundsFailClosed(t *testing.T) {
	maximum := &AccountMonitorShapeEvidence{SchemaVersion: 1, Model: "tiktok-user", Status: accountMonitorShapeObserved}
	for i := 0; i < accountMonitorShapeMaxSlots; i++ {
		maximum.Slots = append(maximum.Slots, AccountMonitorShapeSlot{Present: true, Type: "object", Nonempty: shapeBool(true)})
	}
	require.Same(t, maximum, boundAccountMonitorShapeEvidence(maximum))
	encoded, err := common.Marshal(maximum)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(encoded), accountMonitorShapeMaxBytes)
	for _, tt := range []struct {
		name   string
		mutate func(*AccountMonitorShapeEvidence)
	}{
		{"slot limit", func(e *AccountMonitorShapeEvidence) { e.Slots = append(e.Slots, e.Slots[0]) }},
		{"invalid schema", func(e *AccountMonitorShapeEvidence) { e.SchemaVersion = 2 }},
		{"invalid type", func(e *AccountMonitorShapeEvidence) {
			e.Slots[0].Type = strings.Repeat("private-sentinel", accountMonitorShapeMaxBytes)
		}},
		{"unknown status", func(e *AccountMonitorShapeEvidence) { e.Status = "private-sentinel" }},
		{"missing with present", func(e *AccountMonitorShapeEvidence) {
			e.Slots[0] = AccountMonitorShapeSlot{Present: true, Type: "missing"}
		}},
		{"string with container flag", func(e *AccountMonitorShapeEvidence) { e.Slots[0].Type = "string" }},
		{"container without flag", func(e *AccountMonitorShapeEvidence) { e.Slots[0].Nonempty = nil }},
		{"too large with slots", func(e *AccountMonitorShapeEvidence) { e.Status = accountMonitorShapeTooLarge }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *maximum
			candidate.Slots = append([]AccountMonitorShapeSlot(nil), maximum.Slots...)
			tt.mutate(&candidate)
			bounded := boundAccountMonitorShapeEvidence(&candidate)
			require.NotNil(t, bounded)
			assert.Equal(t, accountMonitorShapeLimit, bounded.Status)
			assert.Empty(t, bounded.Slots)
			data, err := common.Marshal(bounded)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "private-sentinel")
			assert.LessOrEqual(t, len(data), accountMonitorShapeMaxBytes)
		})
	}
	assert.Nil(t, boundAccountMonitorShapeEvidence(nil))
	assert.Nil(t, boundAccountMonitorShapeEvidence(&AccountMonitorShapeEvidence{Model: "private-sentinel"}))
}

func TestAccountMonitorShapeCloseErrorLogsOnlyLocalText(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "diagnostic-close.log")
	logFile, err := os.Create(logPath)
	require.NoError(t, err)
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = logFile
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = originalWriter
		common.LogWriterMu.Unlock()
		assert.NoError(t, logFile.Close())
	})
	c, _ := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
	response := shapeResponse("")
	reader := &shapeReadCloser{
		reader:   strings.NewReader(`{"data":{"unknown":"private-close-sentinel"}}`),
		closeErr: errors.New("private-close-sentinel"),
	}
	response.Body = reader
	adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
	usage, apiErr := adaptor.DoResponse(c, response, info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.Equal(t, 1, reader.closes)
	logs, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(logs), "account monitoring diagnostic response close failed")
	assert.NotContains(t, string(logs), "private-close-sentinel")
}

func TestAccountMonitorShapeEvidenceDoesNotSurviveNextResponse(t *testing.T) {
	c, _ := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
	adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
	usage, apiErr := adaptor.DoResponse(c, shapeResponse(`{"data":{"userInfo":{}}}`), info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	require.NotNil(t, GetAccountMonitorShapeEvidence(c))
	usage, apiErr = adaptor.DoResponse(c, shapeResponse(`{"data":{"nickname":"recognized"}}`), info)
	require.Nil(t, apiErr)
	assert.NotNil(t, usage)
	assert.Nil(t, GetAccountMonitorShapeEvidence(c))
}

func TestAccountMonitorShapeNoRawBodyInContextOrFailureRecorder(t *testing.T) {
	c, recorder := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "x-posts"))
	body := `{"data":{"timeline":{"instructions":[{"entries":[{"content":{"itemContent":{"tweet_results":{"result":{"legacy":{"full_text":"private-secret-sentinel","entities":{"media":[{"url":"https://private-secret-sentinel"}]}},"rest_id":"private-secret-sentinel"}}}}}]}]},"private-secret-sentinel":{"private-secret-sentinel":"private-secret-sentinel"}}}`
	adaptor, info := monitorResponseAdaptor(t, "x-posts", "tikhub-posts")
	usage, apiErr := adaptor.DoResponse(c, shapeResponse(body), info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.Empty(t, recorder.Body.String())
	assert.NotContains(t, apiErr.Error(), "private-secret-sentinel")
	require.Len(t, c.Keys, 2)
	for key, value := range c.Keys {
		switch key {
		case accountMonitorShapeContextKey:
			collector, ok := value.(*accountMonitorShapeCollector)
			require.True(t, ok)
			data, err := common.Marshal(collector.evidence)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "private-secret-sentinel")
		case accountMonitorProbeContextKey:
			probe, ok := value.(*accountMonitorProbeContext)
			require.True(t, ok)
			assert.Equal(t, "x-posts", probe.model)
			assert.Equal(t, "tikhub-posts", probe.policy)
		default:
			t.Errorf("unexpected diagnostic context entry: %s", key)
		}
	}
	assert.NotContains(t, recorder.Header().Get("Content-Type"), "private-secret-sentinel")
	assert.False(t, bytes.Contains(recorder.Body.Bytes(), []byte("private-secret-sentinel")))
}

func TestAccountMonitorShapeNetworkCancellation(t *testing.T) {
	service.InitHttpClient()
	for _, flushBody := range []bool{false, true} {
		for _, deadline := range []bool{false, true} {
			name := "before-headers/cancel"
			if flushBody {
				name = "during-body/cancel"
			}
			if deadline {
				name += "-deadline"
			}
			t.Run(name, func(t *testing.T) {
				entered := make(chan struct{})
				canceled := make(chan struct{})
				release := make(chan struct{})
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if flushBody {
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, `{"data":{"userInfo":{}}}`)
						w.(http.Flusher).Flush()
					}
					close(entered)
					select {
					case <-r.Context().Done():
						close(canceled)
					case <-release:
					}
				}))
				defer server.Close()
				defer close(release)
				c, recorder := shapeContext()
				var requestContext context.Context
				var cancel context.CancelFunc
				if deadline {
					requestContext, cancel = context.WithTimeout(c.Request.Context(), 100*time.Millisecond)
				} else {
					requestContext, cancel = context.WithCancel(c.Request.Context())
					stop := time.AfterFunc(100*time.Millisecond, cancel)
					defer stop.Stop()
				}
				defer cancel()
				c.Request = c.Request.WithContext(requestContext)
				require.True(t, EnableAccountMonitorShapeDiagnostic(c, "tiktok-user"))
				adaptor, info := monitorResponseAdaptor(t, "tiktok-user", "tikhub-user")
				info.ChannelBaseUrl = server.URL
				type result struct {
					usage any
					err   error
				}
				done := make(chan result, 1)
				go func() {
					response, err := adaptor.DoRequest(c, info, http.NoBody)
					if err != nil {
						done <- result{err: err}
						return
					}
					usage, apiErr := adaptor.DoResponse(c, response.(*http.Response), info)
					var responseErr error
					if apiErr != nil {
						responseErr = apiErr
					}
					done <- result{usage: usage, err: responseErr}
				}()
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("local upstream was not reached")
				}
				select {
				case got := <-done:
					assert.Error(t, got.err)
					assert.Nil(t, got.usage)
					assert.Nil(t, GetAccountMonitorShapeEvidence(c))
					assert.Empty(t, recorder.Body.String())
				case <-time.After(time.Second):
					t.Error("diagnostic transport did not stop after request context cancellation")
				}
				select {
				case <-canceled:
				case <-time.After(200 * time.Millisecond):
					t.Error("local upstream did not observe transport context cancellation")
				}
				assert.EqualValues(t, 1, requests.Load())
			})
		}
	}
}

func TestAccountMonitorShapeNetworkCancellationIsOptIn(t *testing.T) {
	service.InitHttpClient()
	for _, tt := range []struct{ name, model, policy, enabledModel string }{
		{"disabled", "tiktok-user", "tikhub-user", ""},
		{"different actual model", "x-user", "tikhub-user", "tiktok-user"},
		{"unconfigured policy", "x-posts", "", "x-posts"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entered := make(chan struct{})
			canceled := make(chan struct{})
			release := make(chan struct{})
			var released bool
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				close(entered)
				select {
				case <-r.Context().Done():
					close(canceled)
				case <-release:
					_, _ = io.WriteString(w, `{"data":{"nickname":"normal-response"}}`)
				}
			}))
			defer server.Close()
			defer func() {
				if !released {
					close(release)
				}
			}()
			c, recorder := shapeContext()
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			if tt.enabledModel != "" {
				require.True(t, EnableAccountMonitorShapeDiagnostic(c, tt.enabledModel))
			}
			adaptor, info := monitorResponseAdaptor(t, tt.model, tt.policy)
			info.ChannelBaseUrl = server.URL
			type result struct {
				body []byte
				err  error
			}
			done := make(chan result, 1)
			go func() {
				response, err := adaptor.DoRequest(c, info, http.NoBody)
				if err != nil {
					done <- result{err: err}
					return
				}
				resp := response.(*http.Response)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				done <- result{body: body, err: err}
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("local upstream was not reached")
			}
			cancel()
			select {
			case <-done:
				t.Fatal("ordinary relay unexpectedly inherited diagnostic cancellation")
			case <-canceled:
				t.Fatal("ordinary upstream unexpectedly received diagnostic cancellation")
			case <-time.After(100 * time.Millisecond):
			}
			close(release)
			released = true
			select {
			case got := <-done:
				require.NoError(t, got.err)
				assert.Equal(t, `{"data":{"nickname":"normal-response"}}`, string(got.body))
			case <-time.After(time.Second):
				t.Fatal("ordinary relay did not finish after local upstream release")
			}
			assert.Nil(t, GetAccountMonitorShapeEvidence(c))
			assert.Empty(t, recorder.Body.String())
			assert.EqualValues(t, 1, requests.Load())
		})
	}
}
