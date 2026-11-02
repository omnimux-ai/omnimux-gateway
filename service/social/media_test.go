package social

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
