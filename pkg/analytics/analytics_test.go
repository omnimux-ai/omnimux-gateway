package analytics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSiteID        = "00000000-0000-4000-8000-000000000001"
	tenantReferrer    = "https://12025550142.workspace.example.test/pricing"
	tenantPhoneDigits = "12025550142"
)

func testConfig(endpoint string) config {
	return config{
		endpoint:      endpoint,
		siteID:        testSiteID,
		siteURL:       "https://gateway.example.net/unused?private=value#private",
		userKeySecret: "dedicated-test-key",
	}
}

type analyticsLogRecords chan string

func (records analyticsLogRecords) Write(line []byte) (int, error) {
	records <- string(line)
	return len(line), nil
}

func captureAnalyticsLogs(t *testing.T) (audit, warnings analyticsLogRecords) {
	t.Helper()
	audit, warnings = make(analyticsLogRecords, 32), make(analyticsLogRecords, 32)
	common.LogWriterMu.Lock()
	previousWriter, previousErrorWriter := gin.DefaultWriter, gin.DefaultErrorWriter
	gin.DefaultWriter, gin.DefaultErrorWriter = audit, warnings
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter, gin.DefaultErrorWriter = previousWriter, previousErrorWriter
		common.LogWriterMu.Unlock()
	})
	return audit, warnings
}

func TestCollectorSendsServerEventsWithWhitelistedData(t *testing.T) {
	audit, _ := captureAnalyticsLogs(t)
	type request struct {
		body      []byte
		method    string
		path      string
		userAgent string
	}
	received := make(chan request, 8)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- request{body, req.Method, req.URL.Path, req.UserAgent()}
		_, _ = io.WriteString(writer, `{"cache":"acknowledgment","sessionId":"sample"}`)
	}))
	t.Cleanup(server.Close)

	client := newClient(testConfig(server.URL + "/api/send"))
	require.NotNil(t, client.queue)
	t.Cleanup(func() {
		close(client.queue)
		client.workers.Wait()
	})

	key := common.HmacSha256("user:7", "dedicated-test-key")
	tests := []struct {
		event Event
		want  map[string]any
	}{
		{
			event: Event{Name: EventSignUpSuccess, UserID: 7, Data: map[string]any{
				"method": "oauth", "user_key": "7", "device_id": "A0000000-0000-4000-8000-000000000001", "app_version": "v1.2.3-beta.1+build.7",
				"referrer": "https://gateway.example.net/pricing?email=private#hidden",
			}},
			want: map[string]any{
				"method": "oauth", "referrer": "gateway.example.net/pricing",
				"device_id": "A0000000-0000-4000-8000-000000000001", "app_version": "v1.2.3-beta.1+build.7",
			},
		},
		{
			event: Event{Name: EventSignUpSuccess, UserID: 7, Data: map[string]any{
				"method": "oauth", "referrer": tenantReferrer,
			}},
			want: map[string]any{"method": "oauth", "device_id": "", "app_version": "", "referrer": ""},
		},
		{
			event: Event{Name: EventFirstVideoStart, UserID: 7, Data: map[string]any{
				"task_id": "task-start", "model_name": "example/video-model", "entry": "api",
				"template_id": "template-1", "email": "ignored",
			}},
			want: map[string]any{
				"task_id": "task-start", "model_name": "example/video-model",
				"entry": "api", "template_id": "template-1",
			},
		},
		{
			event: Event{Name: EventFirstVideoSuccess, UserID: 7, Data: map[string]any{
				"task_id": "task-success", "model_name": "video-model", "duration_ms": int64(4200),
				"file_size": int64(0), "output_type": "image", "result_url": "https://private.example.net",
			}},
			want: map[string]any{
				"user_key": key, "task_id": "task-success", "model_name": "video-model",
				"duration_ms": float64(4200), "output_type": "video", "is_first_success": true,
			},
		},
		{
			event: Event{Name: EventTopUpPaid, UserID: 7, OrderRef: "private-order-ref-7", Data: map[string]any{
				"amount": 4.75, "currency": "USD", "payment_method": "provider",
				"paid_at": int64(1783000000), "order_id": "unhashed-order-7", "pack_id": "ignored",
			}},
			want: map[string]any{
				"order_id": common.HmacSha256("order:private-order-ref-7", "dedicated-test-key"),
				"amount":   4.75, "currency": "USD", "payment_method": "provider",
				"paid_at": float64(1783000000), "pack_id": "",
			},
		},
	}

	wanted := make(map[int64]int, len(tests))
	for i, tc := range tests {
		tc.event.OccurredAt = 1_783_000_000 + int64(i)
		wanted[tc.event.OccurredAt] = i
		client.enqueue(func(ctx context.Context) (Event, bool) {
			_, hasDeadline := ctx.Deadline()
			return tc.event, hasDeadline
		})
	}
	posted := make([]string, 0, len(tests))
	for range tests {
		select {
		case req := <-received:
			posted = append(posted, string(req.body)+"\n")
			var envelope umamiEnvelope
			require.NoError(t, common.Unmarshal(req.body, &envelope))
			index, expected := wanted[envelope.Payload.Timestamp]
			require.True(t, expected, "unexpected or duplicate event timestamp")
			delete(wanted, envelope.Payload.Timestamp)
			assert.Equal(t, tests[index].event.Name, envelope.Payload.Name)
			assert.NotContains(t, string(req.body), tenantPhoneDigits, "collector POST contains tenant referrer digits")
			assert.Equal(t, http.MethodPost, req.method)
			assert.Equal(t, "/api/send", req.path)
			assert.Contains(t, req.userAgent, "OmniMux-Gateway/")
			assert.Equal(t, "event", envelope.Type)
			assert.Equal(t, testSiteID, envelope.Payload.Website)
			assert.Equal(t, "gateway.example.net", envelope.Payload.Hostname)
			assert.Equal(t, "/server/"+envelope.Payload.Name, envelope.Payload.URL)
			assert.Equal(t, key, envelope.Payload.ID)
			assert.Equal(t, tests[index].want, envelope.Payload.Data)
		case <-time.After(5 * time.Second):
			t.Fatal("collector did not receive all expected events")
		}
	}
	require.Len(t, audit, len(tests), "audit must precede HTTP")
	logged := make([]string, 0, len(tests))
	for range tests {
		line := <-audit
		assert.NotContains(t, line, tenantPhoneDigits, "online audit contains tenant referrer digits")
		logged = append(logged, line)
	}
	assert.ElementsMatch(t, posted, logged)
}

func TestCollectorAuditsWhenUnreachable(t *testing.T) {
	audit, warnings := captureAnalyticsLogs(t)
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	cfg := testConfig(server.URL + "/api/send")
	client := newClient(cfg)
	require.NotNil(t, client.queue)
	event := Event{Name: EventSignUpSuccess, UserID: 123456789, Data: map[string]any{
		"method": "oauth", "user_id": 123456789, "email": "visitor@example.net", "ip": "192.0.2.24", "prompt": "hidden prompt",
		"device_id": "visitor@example.net", "app_version": "415.555.0101", "token": cfg.userKeySecret,
		"referrer": "https://gateway.example.net/register?code=private-parameter#private-fragment",
	}}
	tenantEvent := event
	tenantEvent.Data = map[string]any{"method": "oauth", "referrer": tenantReferrer}
	for _, signup := range []Event{event, tenantEvent} {
		client.enqueue(func(context.Context) (Event, bool) { return signup, true })
	}
	close(client.queue)
	client.workers.Wait()

	require.Len(t, audit, 2)
	var referrers []string
	for range 2 {
		line := <-audit
		for _, private := range []string{"123456789", "visitor@example.net", "192.0.2.24", "415.555.0101", "hidden prompt", "private-parameter", "private-fragment", tenantPhoneDigits, cfg.userKeySecret} {
			assert.NotContains(t, line, private)
		}
		var envelope umamiEnvelope
		require.NoError(t, common.Unmarshal([]byte(line), &envelope))
		assert.Regexp(t, `^[0-9a-f]{64}$`, envelope.Payload.ID)
		assert.Equal(t, EventSignUpSuccess, envelope.Payload.Name)
		referrer, ok := envelope.Payload.Data["referrer"].(string)
		require.True(t, ok)
		referrers = append(referrers, referrer)
		assert.Equal(t, map[string]any{
			"method": "oauth", "device_id": "", "app_version": "", "referrer": referrer,
		}, envelope.Payload.Data)
	}
	assert.ElementsMatch(t, []string{"gateway.example.net/register", ""}, referrers)
	select {
	case warning := <-warnings:
		assert.Contains(t, warning, "analytics: collector unavailable")
		assert.NotContains(t, warning, server.URL)
	case <-time.After(3 * time.Second):
		t.Fatal("collector warning did not reach the log")
	}
	assert.Empty(t, warnings)
	t.Log("two sanitized audit records and one unavailable warning without a listening collector")
}

func TestCollectorRejectsUnsafeSignupHeaders(t *testing.T) {
	cases := []struct{ deviceID, version string }{
		{"visitor@example.net", "browser/1.2.3"},
		{"192.0.2.24", "192.0.2.24"},
		{"sk-example-access-token", "1.2.3?session=value"},
		{"device-15550123456", "1.2.3+accountname"},
		{"device_123456789", "1.2.3-private-identifier"},
		{"device-" + strings.Repeat("a", 58), "1.2.3+" + strings.Repeat("b", 59)},
		{"device-sample\nsecond-line", " 1.2.3"},
	}
	for _, tc := range cases {
		data := SignupData("oauth", tc.deviceID, tc.version, "")
		assert.Equal(t, "", data["device_id"])
		assert.Equal(t, "", data["app_version"])
	}
}

func TestCollectorRemovesSensitiveReferrerComponents(t *testing.T) {
	c := &client{hostname: "gateway.example.net"}
	tests := []struct{ input, want string }{
		{"https://GATEWAY.example.net./sign-up?code=private#private", "gateway.example.net/sign-up"},
		{"https://gateway.example.net/workspaces/private-identifier", "gateway.example.net"},
		{"gateway.example.net/register", "gateway.example.net/register"},
		{"https://source.example.net/pricing", ""},
		{"https://tenant.gateway.example.net/register", ""},
		{"https://gateway.example.net.example.test/register", ""},
		{"https://user:secret@source.example.net/register", ""},
		{"https://192.0.2.1/register", ""},
		{"https://127.1/register", ""},
		{"https://[2001:db8::1]/register", ""},
		{"javascript:alert(1)", ""},
	}
	for _, tc := range tests {
		result, ok := c.payload(Event{Name: EventSignUpSuccess, UserID: 7, Data: map[string]any{
			"method": "password", "referrer": tc.input,
		}})
		require.True(t, ok)
		assert.Equal(t, tc.want, result.Data["referrer"], tc.input)
	}
	c.hostname = ""
	result, ok := c.payload(Event{Name: EventSignUpSuccess, UserID: 7, Data: map[string]any{
		"method": "oauth", "referrer": "https://gateway.example.net/register",
	}})
	require.True(t, ok)
	assert.Equal(t, "", result.Data["referrer"])
}

func TestCollectorAcknowledgmentAndRedirect(t *testing.T) {
	event := Event{Name: EventSignUpSuccess, UserID: 7, Data: map[string]any{"method": "password"}}

	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"recorded event", http.StatusOK, `{"cache":"cache-token","sessionId":"session"}`, ""},
		{"filtered event despite 200", http.StatusOK, `{"beep":"boop"}`, "collector rejected event"},
		{"disabled site despite 200", http.StatusOK, `{"cache":"not-a-recorded-event","disabled":true}`, "collector rejected event"},
		{"ambiguous failure", http.StatusServiceUnavailable, `{}`, "collector rejected event"},
		{"redirect", http.StatusTemporaryRedirect, `{}`, "collector rejected event"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
				attempts.Add(1)
				if req.URL.Path != "/api/send" {
					writer.WriteHeader(http.StatusInternalServerError)
					return
				}
				writer.Header().Set("Location", "/other")
				writer.WriteHeader(tc.status)
				_, _ = io.WriteString(writer, tc.body)
			}))
			t.Cleanup(server.Close)
			client := newClient(testConfig(server.URL + "/api/send"))
			require.NotNil(t, client.queue)
			t.Cleanup(func() {
				close(client.queue)
				client.workers.Wait()
			})
			assert.Equal(t, tc.want, client.send(event))
			assert.EqualValues(t, 1, attempts.Load())
		})
	}
}

func TestCollectorRequiresDedicatedSecretAndSafeConfiguration(t *testing.T) {
	configs := []config{
		{},
		{endpoint: "https://collector.example.net/api/send", siteID: testSiteID},
		testConfig("http://collector.example.net/api/send"),
		testConfig("https://credentials@collector.example.net/api/send"),
		testConfig("https://collector.example.net/api/send?key=private"),
	}
	for _, cfg := range configs {
		c := newClient(cfg)
		if c.queue != nil {
			close(c.queue)
			c.workers.Wait()
			t.Fatal("invalid collector configuration created a queue")
		}
		c.enqueue(func(context.Context) (Event, bool) {
			t.Error("disabled collector executed a job")
			return Event{}, false
		})
	}
}

func TestCollectorDropsNewWorkWhenQueueIsFull(t *testing.T) {
	c := &client{queue: make(chan func(context.Context) (Event, bool), 1)}
	c.enqueue(func(context.Context) (Event, bool) { return Event{UserID: 7}, true })

	done := make(chan struct{})
	go func() {
		c.enqueue(func(context.Context) (Event, bool) { return Event{UserID: 8}, true })
		close(done)
	}()
	select {
	case <-done:
		event, _ := (<-c.queue)(context.Background())
		assert.Equal(t, 7, event.UserID)
		assert.Empty(t, c.queue)
	case <-time.After(2 * time.Second):
		close(c.queue)
		t.Fatal("enqueue blocked behind a full queue")
	}
}
