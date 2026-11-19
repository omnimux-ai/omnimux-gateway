package analytics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSiteID = "00000000-0000-4000-8000-000000000001"

func testConfig(endpoint string) config {
	return config{
		endpoint:      endpoint,
		siteID:        testSiteID,
		siteURL:       "https://gateway.example.net/unused?private=value#private",
		userKeySecret: "dedicated-test-key",
	}
}

func TestCollectorSendsServerEventsWithWhitelistedData(t *testing.T) {
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
				"method": "oauth", "user_key": "7", "device_id": "device-7", "app_version": "example",
				"referrer": "https://source.example.net/pricing?email=private#hidden",
			}},
			want: map[string]any{
				"user_key": key, "method": "oauth", "referrer": "source.example.net/pricing",
				"device_id": "", "app_version": "",
			},
		},
		{
			event: Event{Name: EventFirstVideoStart, UserID: 7, Data: map[string]any{
				"task_id": "task-start", "model_name": "example/video-model", "entry": "api",
				"template_id": "template-1", "email": "ignored",
			}},
			want: map[string]any{
				"user_key": key, "task_id": "task-start", "model_name": "example/video-model",
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
				"user_key": key, "order_id": common.HmacSha256("order:private-order-ref-7", "dedicated-test-key"),
				"amount": 4.75, "currency": "USD", "payment_method": "provider",
				"paid_at": float64(1783000000), "pack_id": "",
			},
		},
	}

	wanted := make(map[string]map[string]any, len(tests))
	timestamps := make(map[string]int64, len(tests))
	for i, tc := range tests {
		tc.event.OccurredAt = 1_783_000_000 + int64(i)
		wanted[tc.event.Name] = tc.want
		timestamps[tc.event.Name] = tc.event.OccurredAt
		client.enqueue(func(ctx context.Context) (Event, bool) {
			_, hasDeadline := ctx.Deadline()
			return tc.event, hasDeadline
		})
	}
	for range tests {
		select {
		case req := <-received:
			var envelope umamiEnvelope
			require.NoError(t, common.Unmarshal(req.body, &envelope))
			want, expected := wanted[envelope.Payload.Name]
			require.True(t, expected, "unexpected or duplicate event type")
			delete(wanted, envelope.Payload.Name)
			assert.Equal(t, http.MethodPost, req.method)
			assert.Equal(t, "/api/send", req.path)
			assert.Contains(t, req.userAgent, "OmniMux-Gateway/")
			assert.Equal(t, "event", envelope.Type)
			assert.Equal(t, testSiteID, envelope.Payload.Website)
			assert.Equal(t, "gateway.example.net", envelope.Payload.Hostname)
			assert.Equal(t, "/server/"+envelope.Payload.Name, envelope.Payload.URL)
			assert.Equal(t, key, envelope.Payload.ID)
			assert.Equal(t, timestamps[envelope.Payload.Name], envelope.Payload.Timestamp)
			assert.Equal(t, want, envelope.Payload.Data)
		case <-time.After(5 * time.Second):
			t.Fatalf("collector did not receive events: %v", wanted)
		}
	}
}

func TestCollectorRemovesSensitiveReferrerComponents(t *testing.T) {
	c := &client{config: testConfig("")}
	tests := []struct{ input, want string }{
		{"https://source.example.net/sign-up?code=private#private", "source.example.net/sign-up"},
		{"https://source.example.net/workspaces/private-identifier", "source.example.net"},
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
		assert.Equal(t, tc.want, result.Data["referrer"])
	}
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
