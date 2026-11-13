package sora

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchTaskVideoPrefersVideosEndpoint ensures video tasks poll /v1/videos/{id}
// and never probe /v1/tasks/{id} (which could break Sora/grok video polling).
func TestFetchTaskVideoPrefersVideosEndpoint(t *testing.T) {
	var taskHits, videoHits, imageHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tasks/task_v":
			atomic.AddInt32(&taskHits, 1)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case "/v1/videos/task_v":
			atomic.AddInt32(&videoHits, 1)
			_, _ = w.Write([]byte(`{"model":"grok-imagine-video-1.5","status":"pending","progress":10}`))
		case "/v1/images/tasks/task_v":
			atomic.AddInt32(&imageHits, 1)
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTaskMap(srv.URL, "key", map[string]any{
		"task_id":      "task_v",
		"action":       "generate",
		"origin_model": "grok-imagine-video-1-5",
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Video task must ONLY hit /v1/videos — no /v1/tasks probe.
	assert.EqualValues(t, 0, taskHits, "video task must not probe /v1/tasks")
	assert.EqualValues(t, 1, videoHits)
}

// TestFetchTaskImageFallsBackToImageTasks ensures image tasks try /v1/tasks/{id}
// first and fall back to /v1/images/tasks/{id} when the first returns 404/405.
func TestFetchTaskImageFallsBackToImageTasks(t *testing.T) {
	var taskHits, imageHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tasks/task_i":
			atomic.AddInt32(&taskHits, 1)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case "/v1/images/tasks/task_i":
			atomic.AddInt32(&imageHits, 1)
			_, _ = w.Write([]byte(`{"status":"completed","data":[{"url":"http://127.0.0.1:8000/v1/media/images/img_x"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTaskMap(srv.URL, "key", map[string]any{
		"task_id":      "task_i",
		"action":       "generate",
		"origin_model": "grok-imagine-image-2-0",
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 1, taskHits)
	assert.EqualValues(t, 1, imageHits)
}

// TestFetchTaskImagePrefersUnifiedTaskEndpoint ensures an implemented /v1/tasks
// endpoint is used directly (Evolink/Apimart standard) without extra probes.
func TestFetchTaskImagePrefersUnifiedTaskEndpoint(t *testing.T) {
	var taskHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tasks/task_i2" {
			atomic.AddInt32(&taskHits, 1)
			_, _ = w.Write([]byte(`{"status":"queued"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTaskMap(srv.URL, "key", map[string]any{
		"task_id":      "task_i2",
		"action":       "generate",
		"origin_model": "grok-imagine-image",
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 1, taskHits)
}

// TestFetchTaskUnknownKindFallsBack verifies unknown kinds degrade gracefully
// and return the first real (non-404/405) response.
func TestFetchTaskUnknownKindFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/videos/task_u":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"pending"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTaskMap(srv.URL, "key", map[string]any{
		"task_id":      "task_u",
		"action":       "generate",
		"origin_model": "",
	}, "")
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestFetchTaskImageAllNotFound verifies an explicit error is returned when
// every candidate endpoint 404s, instead of a nil response.
func TestFetchTaskImageAllNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTaskMap(srv.URL, "key", map[string]any{
		"task_id":      "task_none",
		"action":       "generate",
		"origin_model": "grok-imagine-image",
	}, "")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, fmt.Sprintf("%v", err), "not found")
}
