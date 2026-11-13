package hostedgateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}

func TestLoadConfigReady(t *testing.T) {
	t.Setenv("HOSTED_GATEWAY_URL", "")
	t.Setenv("HOSTED_GATEWAY_API_KEY", "")
	cfg := LoadConfig()
	assert.False(t, cfg.Ready())
	assert.Equal(t, defaultTimeoutMS, cfg.TimeoutMS)

	t.Setenv("HOSTED_GATEWAY_URL", "http://127.0.0.1:8888/")
	t.Setenv("HOSTED_GATEWAY_API_KEY", "test-key")
	t.Setenv("HOSTED_GATEWAY_TIMEOUT_MS", "12000")
	cfg = LoadConfig()
	assert.True(t, cfg.Ready())
	assert.Equal(t, "http://127.0.0.1:8888", cfg.BaseURL)
	assert.Equal(t, 12000, cfg.TimeoutMS)
}

func TestNotReadyDoesNotHitNetwork(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{})
	_, err := c.QueueStatus(context.Background())
	require.ErrorIs(t, err, ErrGatewayUnavailable)
	assert.False(t, hit)

	c = NewClient(Config{BaseURL: srv.URL})
	_, err = c.TaskAdd(context.Background(), map[string]any{"taskType": 2})
	require.ErrorIs(t, err, ErrGatewayUnavailable)
	assert.False(t, hit)
}

func TestUnauthorizedMapsAndSendsBearer(t *testing.T) {
	var gotAuth string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(fixture(t, "unauthorized.json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "secret-key", TimeoutMS: 5000})
	_, err := c.QueueStatus(context.Background())
	require.ErrorIs(t, err, ErrUnauthorized)
	var api *APIError
	require.ErrorAs(t, err, &api)
	assert.Equal(t, 401, api.Status)
	assert.Equal(t, "Bearer secret-key", gotAuth)
	assert.NotContains(t, gotBody, "secret-key")
	assert.NotContains(t, err.Error(), "secret-key")
}

func TestQueueStatusFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/automation/queue/status", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "queue_status.json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	q, err := c.QueueStatus(context.Background())
	require.NoError(t, err)
	assert.True(t, q.JobsEnabled)
	assert.Equal(t, 2, q.EffectiveCap)
	assert.Equal(t, 3, q.PlanCap)
}

func TestTaskAddPlaintext400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("list must be non-empty\n"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	_, err := c.TaskAdd(context.Background(), map[string]any{"taskType": 2, "list": []any{}})
	require.ErrorIs(t, err, ErrInvalidRequest)
	assert.Contains(t, err.Error(), "list must be non-empty")
}

func TestDeviceNotMonthly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write(fixture(t, "device_not_monthly.json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	_, err := c.TaskAdd(context.Background(), map[string]any{
		"taskType": 2,
		"list":     []any{map[string]any{"envId": "PHONE_ID"}},
	})
	require.ErrorIs(t, err, ErrDeviceNotMonthly)
	var api *APIError
	require.ErrorAs(t, err, &api)
	assert.Equal(t, "PHONE_ID", api.PhoneID)
	assert.Equal(t, "DEVICE_NOT_MONTHLY", api.Code)
}

func TestAnalyticsPlayCountStale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture(t, "analytics_data.json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	page, err := c.AnalyticsData(context.Background(), map[string]any{"page": 1, "pageSize": 10})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.False(t, page.Items[0].PlayCountStale)
	assert.Equal(t, int64(0), page.Items[0].PlayCount)
	assert.True(t, page.Items[1].PlayCountStale)
	assert.Equal(t, int64(-1), page.Items[1].PlayCount)
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k", TimeoutMS: 15})
	_, err := c.QueueStatus(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTimeout), "got %v", err)
}

func TestUpstreamCodeOnHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "upstream_40005.json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	_, err := c.TaskAdd(context.Background(), map[string]any{"taskType": 2})
	require.ErrorIs(t, err, ErrUpstream)
	var api *APIError
	require.ErrorAs(t, err, &api)
	assert.Equal(t, "40005", api.Code)
	assert.True(t, strings.Contains(api.Message, "resource not found"))
}

func TestHealthAnonymous(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write(fixture(t, "health.json"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	h, err := c.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", h.Status)
	assert.Equal(t, "geelark-gateway", h.Service)
	assert.Empty(t, gotAuth)
}

func TestJobsRequireQueue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"JOBS_REQUIRE_QUEUE","msg":"use jobs"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(Config{BaseURL: srv.URL, APIKey: "k"})
	_, err := c.TiktokEdit(context.Background(), map[string]any{"id": "p", "scheduleAt": 1, "bio": "x"})
	require.ErrorIs(t, err, ErrJobsRequired)
}
