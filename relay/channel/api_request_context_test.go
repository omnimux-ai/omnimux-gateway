package channel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hostcommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contextAPIAdaptor struct {
	Adaptor
	url      string
	urlCalls int
}

func (a *contextAPIAdaptor) GetRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	a.urlCalls++
	return a.url, nil
}

func (a *contextAPIAdaptor) SetupRequestHeader(_ *gin.Context, headers *http.Header, _ *relaycommon.RelayInfo) error {
	headers.Set("X-Request-Contract", "original-header")
	return nil
}

func TestDoApiRequestWithContextRejectsMissingContextBeforeRouting(t *testing.T) {
	adaptor := &contextAPIAdaptor{}
	for _, c := range []*gin.Context{nil, {}} {
		response, err := DoApiRequestWithContext(adaptor, c, nil, nil)
		assert.Nil(t, response)
		require.EqualError(t, err, "client request context is missing")
	}
	assert.Zero(t, adaptor.urlCalls)
}

func TestDoApiRequestContextVariantsShareRequestContract(t *testing.T) {
	service.InitHttpClient()
	for _, explicitContext := range []bool{false, true} {
		name := "existing-background"
		if explicitContext {
			name = "explicit-context"
		}
		t.Run(name, func(t *testing.T) {
			type observed struct {
				method, path, query, header, contentType string
				body                                     []byte
				err                                      error
			}
			observations := make(chan observed, 1)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				body, err := io.ReadAll(r.Body)
				observations <- observed{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("X-Request-Contract"), r.Header.Get("Content-Type"), body, err}
				_, _ = io.WriteString(w, "shared-response")
			}))
			defer server.Close()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/relay", nil)
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			adaptor := &contextAPIAdaptor{url: server.URL + "/fixed-route?fixed=query"}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{HeadersOverride: map[string]any{"X-Request-Contract": "override-header", "Content-Type": "application/json"}}}
			request := DoApiRequest
			if explicitContext {
				request = DoApiRequestWithContext
			}
			response, err := request(adaptor, c, info, strings.NewReader(`{"fixed":"body"}`))
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, "shared-response", string(body))
			got := <-observations
			require.NoError(t, got.err)
			assert.Equal(t, http.MethodPost, got.method)
			assert.Equal(t, "/fixed-route", got.path)
			assert.Equal(t, "fixed=query", got.query)
			assert.Equal(t, "override-header", got.header)
			assert.Equal(t, "application/json", got.contentType)
			assert.Equal(t, `{"fixed":"body"}`, string(got.body))
			assert.EqualValues(t, 1, requests.Load())
			assert.Equal(t, 1, adaptor.urlCalls)
			if explicitContext {
				assert.Same(t, ctx, response.Request.Context())
			} else {
				assert.Equal(t, context.Background(), response.Request.Context())
			}
		})
	}
}

type contextPrivacyLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *contextPrivacyLog) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(data)
}

func (l *contextPrivacyLog) contains(value string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Contains(l.b.Bytes(), []byte(value))
}

func TestDoApiRequestWithContextCancellationDoesNotDiscloseQueryAuth(t *testing.T) {
	service.InitHttpClient()
	const sentinel = "fake-query-auth-privacy-597-unique"
	for _, debug := range []bool{false, true} {
		name := "normal-logging"
		if debug {
			name = "debug-logging"
		}
		t.Run(name, func(t *testing.T) {
			capture := &contextPrivacyLog{}
			hostcommon.LogWriterMu.Lock()
			previousWriter, previousErrorWriter := gin.DefaultWriter, gin.DefaultErrorWriter
			gin.DefaultWriter, gin.DefaultErrorWriter = capture, capture
			hostcommon.LogWriterMu.Unlock()
			previousDebug := hostcommon.DebugEnabled
			hostcommon.DebugEnabled = debug
			defer func() {
				hostcommon.DebugEnabled = previousDebug
				hostcommon.LogWriterMu.Lock()
				gin.DefaultWriter, gin.DefaultErrorWriter = previousWriter, previousErrorWriter
				hostcommon.LogWriterMu.Unlock()
			}()
			entered := make(chan struct{})
			canceled := make(chan struct{})
			release := make(chan struct{})
			var requests atomic.Int32
			var authMatched atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				authMatched.Store(r.URL.Query().Get("private_query_auth") == sentinel)
				close(entered)
				select {
				case <-r.Context().Done():
					close(canceled)
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/fixed-diagnostic", nil)
			ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Millisecond)
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			adaptor := &contextAPIAdaptor{url: server.URL + "/fixed-upstream?private_query_auth=" + sentinel}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
			done := make(chan error, 1)
			go func() {
				response, err := DoApiRequestWithContext(adaptor, c, info, http.NoBody)
				if response != nil {
					_ = response.Body.Close()
				}
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("local privacy fixture was not reached")
			}
			var resultErr error
			select {
			case resultErr = <-done:
			case <-time.After(time.Second):
				t.Fatal("private diagnostic cancellation did not finish")
			}
			if resultErr == nil {
				t.Fatal("private diagnostic returned no cancellation failure")
			}
			// Never print the captured log or error, even on the red run. Only
			// fixed boolean failure facts may reach the retained test receipt.
			for current := resultErr; current != nil; current = errors.Unwrap(current) {
				if strings.Contains(current.Error(), sentinel) || strings.Contains(current.Error(), server.URL) {
					t.Error("returned_error_contains_private_request_data=true")
					break
				}
			}
			if capture.contains(sentinel) || capture.contains(server.URL) {
				t.Error("captured_log_contains_private_request_data=true")
			}
			select {
			case <-canceled:
			case <-time.After(200 * time.Millisecond):
				t.Error("upstream_observed_context_cancellation=false")
			}
			assert.True(t, authMatched.Load())
			assert.EqualValues(t, 1, requests.Load())
		})
	}
}
