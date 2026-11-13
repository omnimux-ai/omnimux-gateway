package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tiktokHandoffFixture serves body from a loopback server and lets the
// transfer reach it; the probe is replaced so no ffprobe is needed.
func tiktokHandoffFixture(t *testing.T, body []byte, declared int64, duration float64) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "https://www.tiktok.com/", r.Header.Get("Referer"))
		if declared > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(declared, 10))
		}
		_, _ = w.Write(body)
	}))
	validate, client, probe := tiktokHandoffValidateURL, tiktokHandoffClient, tiktokHandoffProbe
	tiktokHandoffValidateURL = func(string) error { return nil }
	tiktokHandoffClient = server.Client
	tiktokHandoffProbe = func(_ context.Context, path string) (InfluencerVideoMetadata, error) {
		return InfluencerVideoMetadata{Duration: duration, Container: "mp4"}, nil
	}
	t.Cleanup(func() {
		server.Close()
		tiktokHandoffValidateURL, tiktokHandoffClient, tiktokHandoffProbe = validate, client, probe
	})
	return server.URL + "/v.mp4"
}

func TestTransferTikTokHandoffVideoRefusesPrivateAddressesThroughTheSSRFGuard(t *testing.T) {
	InitHttpClient()
	_, _, err := TransferTikTokHandoffVideo(context.Background(), "http://127.0.0.1:1/v.mp4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "安全要求")
}

func TestTransferTikTokHandoffVideoReturnsBytesInsideTheRecreateWindow(t *testing.T) {
	target := tiktokHandoffFixture(t, []byte("mp4-bytes"), 0, 15)
	data, media, err := TransferTikTokHandoffVideo(context.Background(), target)
	require.NoError(t, err)
	assert.Equal(t, []byte("mp4-bytes"), data)
	assert.InDelta(t, 15, media.Duration, 0.001)
}

func TestTransferTikTokHandoffVideoRejectsDurationsOutsideFourToThirtySeconds(t *testing.T) {
	for _, seconds := range []float64{3.9, 30.1} {
		target := tiktokHandoffFixture(t, []byte("mp4-bytes"), 0, seconds)
		_, _, err := TransferTikTokHandoffVideo(context.Background(), target)
		assert.ErrorIs(t, err, ErrTikTokHandoffDuration, "%.1fs", seconds)
	}
}

func TestTransferTikTokHandoffVideoRejectsADeclaredSizeAboveTheCap(t *testing.T) {
	target := tiktokHandoffFixture(t, nil, InfluencerVideoMaxBytes+1, 15)
	_, _, err := TransferTikTokHandoffVideo(context.Background(), target)
	assert.ErrorIs(t, err, ErrTikTokHandoffTooLarge)
}

func TestTikTokHandoffDurationAllowedIncludesBothBounds(t *testing.T) {
	assert.True(t, TikTokHandoffDurationAllowed(4))
	assert.True(t, TikTokHandoffDurationAllowed(30))
	assert.False(t, TikTokHandoffDurationAllowed(0))
}
