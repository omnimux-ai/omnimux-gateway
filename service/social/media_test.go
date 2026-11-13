package social

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePublicMediaSourceURLRejectsLoopbackAliases(t *testing.T) {
	for _, rawURL := range []string{
		"https://2130706433:8443/internal",
		"https://2852039166/internal",
		"https://127.1/internal",
		"https://localhost./internal",
		"https://127.0.0.1/internal",
	} {
		_, err := validatePublicMediaSourceURL(rawURL, false)
		assert.Error(t, err, rawURL)
	}
}

func TestNormalizePresignInput(t *testing.T) {
	in, err := NormalizePresignInput(map[string]any{
		"filename":     "clip.mp4",
		"content_type": "video/mp4",
		"size_bytes":   float64(1024),
	})
	require.NoError(t, err)
	assert.Equal(t, "clip.mp4", in.Filename)
	assert.Equal(t, "video/mp4", in.ContentType)
	assert.Equal(t, int64(1024), in.SizeBytes)

	body := ZernioPresignBody(in)
	assert.Equal(t, "clip.mp4", body["filename"])
	assert.Equal(t, "video/mp4", body["contentType"])

	_, err = NormalizePresignInput(map[string]any{"filename": "x.png"})
	assert.Error(t, err)

	_, err = NormalizePresignInput(map[string]any{
		"filename":     "x.bin",
		"content_type": "application/octet-stream",
	})
	assert.Error(t, err)
}

func TestNormalizeMediaItems(t *testing.T) {
	items, err := NormalizeMediaItems([]map[string]any{
		{"url": "https://media.zernio.com/a.png"},
		{"public_url": "https://media.zernio.com/b.mp4", "type": "video"},
	}, 10)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "https://media.zernio.com/a.png", items[0]["url"])
	assert.Equal(t, "image", items[0]["type"])
	assert.Equal(t, "video", items[1]["type"])

	_, err = NormalizeMediaItems([]map[string]any{{"url": "not-a-url"}}, 10)
	assert.Error(t, err)

	_, err = NormalizeMediaItems([]map[string]any{
		{"url": "https://x/a"}, {"url": "https://x/b"}, {"url": "https://x/c"},
	}, 2)
	assert.Error(t, err)
}

// fork: social-media-own-storage — routing and ticket shape for the own bucket.
type fakeMediaStorage struct {
	publicBase  string
	readTicket  string
	putErr      error
	putCalls    int
	readCalls   int
	lastKey     string
	lastContent string
}

func (f *fakeMediaStorage) Upload(context.Context, string, []byte, string) (string, error) {
	return "", nil
}

func (f *fakeMediaStorage) Get(context.Context, string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, storage.ErrObjectNotFound
}

func (f *fakeMediaStorage) Delete(context.Context, string) error { return nil }

func (f *fakeMediaStorage) GetPublicURL(key string) string {
	if f.publicBase == "" {
		return ""
	}
	return f.publicBase + "/" + key
}

func (f *fakeMediaStorage) IsR2() bool { return true }

func (f *fakeMediaStorage) PresignPut(_ context.Context, key, contentType string, ttl time.Duration) (*storage.PresignResult, error) {
	f.putCalls++
	f.lastKey = key
	f.lastContent = contentType
	if f.putErr != nil {
		return nil, f.putErr
	}
	return &storage.PresignResult{URL: "https://r2.example/put/" + key, Method: "PUT", Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (f *fakeMediaStorage) PresignGet(context.Context, string, string, string, time.Duration) (*storage.PresignResult, error) {
	f.readCalls++
	return &storage.PresignResult{URL: f.readTicket, Method: "GET"}, nil
}

func installFakeMediaStorage(t *testing.T, driver storage.StorageDriver) {
	t.Helper()
	storage.SetGlobalDriver(driver)
	t.Cleanup(storage.ResetGlobalDriver)
}

func TestPresignUsesOwnStorageRouting(t *testing.T) {
	direct := &Service{cfg: Config{Enabled: true, TikTokDirectEnabled: true}}
	rolledBack := &Service{cfg: Config{Enabled: true, TikTokDirectEnabled: true, RollbackTikTokToZernio: true}}
	disabled := &Service{cfg: Config{Enabled: true}}

	cases := []struct {
		name string
		svc  *Service
		body map[string]any
		want bool
	}{
		{"explicit direct", disabled, map[string]any{"provider": "tiktok_direct"}, true},
		{"explicit zernio wins over active direct", direct, map[string]any{"provider": "zernio"}, false},
		{"no provider follows active direct", direct, map[string]any{}, true},
		{"no provider follows rollback", rolledBack, map[string]any{}, false},
		{"no provider with direct disabled", disabled, map[string]any{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.svc.presignUsesOwnStorage(tc.body))
		})
	}
}

func TestSocialMediaObjectKey(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	key := socialMediaObjectKey(42, "Clip.MP4", now)
	require.True(t, strings.HasPrefix(key, "social/42/20261103/"), key)
	require.True(t, strings.HasSuffix(key, ".mp4"), key)

	traversal := socialMediaObjectKey(42, "../../etc/passwd", now)
	require.True(t, strings.HasPrefix(traversal, "social/42/20261103/"), traversal)
	require.NotContains(t, traversal, "..")

	odd := socialMediaObjectKey(42, "file.we!rd", now)
	require.True(t, strings.HasPrefix(odd, "social/42/20261103/"), odd)
	require.NotContains(t, odd, "!")
}

func TestPresignOwnMediaStorageTicket(t *testing.T) {
	driver := &fakeMediaStorage{publicBase: "https://cdn.example"}
	installFakeMediaStorage(t, driver)

	svc := &Service{cfg: Config{Enabled: true, TikTokDirectEnabled: true, MediaStorageMaxBytes: 1024, MediaStoragePresignTTL: time.Minute}}
	out, err := svc.PresignMedia(context.Background(), 7, map[string]any{
		"provider":     "tiktok_direct",
		"filename":     "a.png",
		"content_type": "image/png",
		"size_bytes":   float64(512),
	})
	require.NoError(t, err)
	assert.Equal(t, "PUT", out["upload_method"])
	assert.Equal(t, "image/png", out["content_type"])
	key, ok := out["key"].(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(key, "social/7/"), key)
	assert.Equal(t, "https://r2.example/put/"+key, out["upload_url"])
	assert.Equal(t, "https://cdn.example/"+key, out["public_url"])
	assert.Equal(t, 1, driver.putCalls)
	assert.Equal(t, "image/png", driver.lastContent)
}

func TestPresignOwnMediaStoragePublicURLFallsBackToReadTicket(t *testing.T) {
	driver := &fakeMediaStorage{readTicket: "https://r2.example/get/object"}
	installFakeMediaStorage(t, driver)

	svc := &Service{cfg: Config{Enabled: true, TikTokDirectEnabled: true, MediaStorageMaxBytes: 1024, MediaStoragePresignTTL: time.Minute}}
	out, err := svc.PresignMedia(context.Background(), 7, map[string]any{
		"provider":     "tiktok_direct",
		"filename":     "a.png",
		"content_type": "image/png",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://r2.example/get/object", out["public_url"])
	assert.Equal(t, 1, driver.readCalls)
}

func TestPresignOwnMediaStorageFailsClosed(t *testing.T) {
	driver := &fakeMediaStorage{putErr: storage.ErrPresignUnsupported}
	installFakeMediaStorage(t, driver)

	svc := &Service{cfg: Config{Enabled: true, TikTokDirectEnabled: true, MediaStorageMaxBytes: 1024, MediaStoragePresignTTL: time.Minute}}
	_, err := svc.PresignMedia(context.Background(), 7, map[string]any{
		"provider":     "tiktok_direct",
		"filename":     "a.png",
		"content_type": "image/png",
	})
	require.ErrorIs(t, err, ErrMediaStorageUnavailable)
}

func TestPresignOwnMediaStorageRejectsOversize(t *testing.T) {
	driver := &fakeMediaStorage{publicBase: "https://cdn.example"}
	installFakeMediaStorage(t, driver)

	svc := &Service{cfg: Config{Enabled: true, TikTokDirectEnabled: true, MediaStorageMaxBytes: 100, MediaStoragePresignTTL: time.Minute}}
	_, err := svc.PresignMedia(context.Background(), 7, map[string]any{
		"provider":     "tiktok_direct",
		"filename":     "a.png",
		"content_type": "image/png",
		"size_bytes":   float64(101),
	})
	require.ErrorIs(t, err, ErrMediaInvalid)
	assert.Equal(t, 0, driver.putCalls)
}

func TestPresignDisabledServiceFailsBeforeStorage(t *testing.T) {
	driver := &fakeMediaStorage{publicBase: "https://cdn.example"}
	installFakeMediaStorage(t, driver)

	svc := &Service{cfg: Config{Enabled: false, TikTokDirectEnabled: true}}
	_, err := svc.PresignMedia(context.Background(), 7, map[string]any{
		"provider":     "tiktok_direct",
		"filename":     "a.png",
		"content_type": "image/png",
	})
	require.ErrorIs(t, err, ErrDisabled)
	assert.Equal(t, 0, driver.putCalls)
}
