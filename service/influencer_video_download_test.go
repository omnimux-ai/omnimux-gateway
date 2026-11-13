package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type referenceDownloadTransport func(*http.Request) (*http.Response, error)

func (f referenceDownloadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestReferenceVideoDownloadNeverReflectsSignedURL(t *testing.T) {
	previous := httpClient
	setting := system_setting.GetFetchSetting()
	old := *setting
	setting.EnableSSRFProtection = false
	t.Cleanup(func() { httpClient = previous; *setting = old })
	httpClient = &http.Client{Transport: referenceDownloadTransport(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("signed-url-secret-marker: %s", req.URL.String())
	})}
	err := downloadInfluencerVideo(context.Background(), "https://files.example/reference.mp4?signature=signed-url-secret-marker", filepath.Join(t.TempDir(), "reference"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "signed-url-secret-marker")
	assert.NotContains(t, err.Error(), "files.example")
	assert.Equal(t, http.StatusServiceUnavailable, InfluencerVideoInspectionStatus(err))
}

func TestReferenceVideoProbeMissingCopyIsSafeUnavailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-copy-marker", "missing-reference")
	_, err := ProbeInfluencerVideoFile(context.Background(), path)
	require.Error(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, InfluencerVideoInspectionStatus(err))
	assert.NotContains(t, err.Error(), path)
}

func TestReferenceVideoDownloadUnknownLengthEnforcesActualCeiling(t *testing.T) {
	previous := httpClient
	setting := system_setting.GetFetchSetting()
	old := *setting
	setting.EnableSSRFProtection = false
	t.Cleanup(func() { httpClient = previous; *setting = old })
	for _, tc := range []struct {
		name string
		size int64
		want int
	}{
		{"above-old-ceiling", 104857601, 0},
		{"exact-ceiling", 209715200, 0},
		{"one-byte-over-ceiling", 209715201, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &referenceCapacityBody{remaining: tc.size}
			httpClient = &http.Client{Transport: referenceDownloadTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, ContentLength: -1, Body: body}, nil
			})}
			dest := filepath.Join(t.TempDir(), "reference")
			err := downloadInfluencerVideo(context.Background(), "https://files.example/reference.mp4", dest)
			if tc.want == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.want, InfluencerVideoInspectionStatus(err))
			}
			info, statErr := os.Stat(dest)
			require.NoError(t, statErr)
			assert.Equal(t, tc.size, info.Size(), "the actual streamed bytes decide capacity, not ContentLength")
			assert.True(t, body.closed, "download must release the response body even when invalid")
		})
	}
}

type referenceCapacityBody struct {
	remaining int64
	closed    bool
}

func (b *referenceCapacityBody) Read(data []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(data))
	if b.remaining < n {
		n = b.remaining
	}
	clear(data[:int(n)])
	b.remaining -= n
	return int(n), nil
}
func (b *referenceCapacityBody) Close() error { b.closed = true; return nil }

type failingReferenceBody struct{}

func (failingReferenceBody) Read([]byte) (int, error) {
	return 0, errors.New("signed-body-secret-marker")
}
func (failingReferenceBody) Close() error { return nil }

func TestReferenceVideoDownloadUnavailableAndCancellationStaySafe(t *testing.T) {
	previous := httpClient
	setting := system_setting.GetFetchSetting()
	old := *setting
	setting.EnableSSRFProtection = false
	t.Cleanup(func() { httpClient = previous; *setting = old })
	for _, tc := range []struct {
		name   string
		status int
		body   io.ReadCloser
	}{
		{"source-refused", http.StatusForbidden, io.NopCloser(strings.NewReader("signed-body-secret-marker"))},
		{"response-read-failed", http.StatusOK, failingReferenceBody{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpClient = &http.Client{Transport: referenceDownloadTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, ContentLength: -1, Body: tc.body}, nil
			})}
			err := downloadInfluencerVideo(context.Background(), "https://files.example/reference.mp4?signature=signed-url-secret-marker", filepath.Join(t.TempDir(), "reference"))
			require.Error(t, err)
			assert.Equal(t, http.StatusServiceUnavailable, InfluencerVideoInspectionStatus(err))
			assert.NotContains(t, err.Error(), "secret-marker")
		})
	}
	t.Run("cancel-in-transport", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		httpClient = &http.Client{Transport: referenceDownloadTransport(func(*http.Request) (*http.Response, error) { cancel(); return nil, ctx.Err() })}
		err := downloadInfluencerVideo(ctx, "https://files.example/reference.mp4", filepath.Join(t.TempDir(), "reference"))
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, http.StatusRequestTimeout, InfluencerVideoInspectionStatus(err))
	})
}

func TestReferenceVideoDownloadCreateFailureIsSafeUnavailable(t *testing.T) {
	previous := httpClient
	setting := system_setting.GetFetchSetting()
	old := *setting
	setting.EnableSSRFProtection = false
	t.Cleanup(func() { httpClient = previous; *setting = old })
	httpClient = &http.Client{Transport: referenceDownloadTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("bytes"))}, nil
	})}
	path := filepath.Join(t.TempDir(), "missing-private-dir", "reference")
	err := downloadInfluencerVideo(context.Background(), "https://files.example/reference.mp4", path)
	require.Error(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, InfluencerVideoInspectionStatus(err))
	assert.NotContains(t, err.Error(), path)
}

func TestReferenceVideoDownloadSeparatesInvalidCapacityFromInfrastructure(t *testing.T) {
	previous := httpClient
	setting := system_setting.GetFetchSetting()
	old := *setting
	setting.EnableSSRFProtection = false
	t.Cleanup(func() { httpClient = previous; *setting = old })
	for _, tc := range []struct {
		name     string
		response *http.Response
	}{
		{"declared-oversize", &http.Response{StatusCode: 200, ContentLength: InfluencerVideoMaxBytes + 1, Body: io.NopCloser(strings.NewReader("ignored"))}},
		{"empty", &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpClient = &http.Client{Transport: referenceDownloadTransport(func(*http.Request) (*http.Response, error) { return tc.response, nil })}
			err := downloadInfluencerVideo(context.Background(), "https://files.example/reference.mp4", filepath.Join(t.TempDir(), "reference"))
			require.Error(t, err)
			assert.Equal(t, http.StatusBadRequest, InfluencerVideoInspectionStatus(err))
		})
	}
}
