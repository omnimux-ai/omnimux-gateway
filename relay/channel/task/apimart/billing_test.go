package apimart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateVideoBilling_DefaultSeconds(t *testing.T) {
	r, err := estimateVideoBilling("kling-v2-6", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, float64(defaultVideoSeconds), r["seconds"])
	_, hasTier := r["tier"]
	assert.False(t, hasTier)
}

func TestEstimateVideoBilling_DurationField(t *testing.T) {
	r, err := estimateVideoBilling("kling-v3", map[string]any{"duration": 8})
	require.NoError(t, err)
	assert.Equal(t, 8.0, r["seconds"])
}

func TestEstimateVideoBilling_SecondsString(t *testing.T) {
	r, err := estimateVideoBilling("kling-v3", map[string]any{"seconds": "6"})
	require.NoError(t, err)
	assert.Equal(t, 6.0, r["seconds"])
}

func TestEstimateVideoBilling_Seedance720pTier(t *testing.T) {
	r, err := estimateVideoBilling("seedance-2-0", map[string]any{
		"duration":   5,
		"resolution": "720p",
	})
	require.NoError(t, err)
	assert.Equal(t, 5.0, r["seconds"])
	// 0.142 / 0.066
	assert.InDelta(t, 0.142/0.066, r["tier"], 1e-6)
}

func TestEstimateVideoBilling_SeedanceFast480pNoTier(t *testing.T) {
	r, err := estimateVideoBilling("seedance-2-0-fast", map[string]any{"duration": 4})
	require.NoError(t, err)
	assert.Equal(t, 4.0, r["seconds"])
	_, hasTier := r["tier"]
	assert.False(t, hasTier)
}

func TestEstimateVideoBilling_SeedanceMiniUsesLedgerTier(t *testing.T) {
	r, err := estimateVideoBilling("seedance-2-0-mini", map[string]any{
		"duration":   5,
		"resolution": "720P",
	})
	require.NoError(t, err)
	assert.InDelta(t, 0.02288/0.01056, r["tier"], 1e-6)
}

func TestEstimateVideoBilling_UnknownResolution(t *testing.T) {
	_, err := estimateVideoBilling("seedance-2-0", map[string]any{
		"duration":   5,
		"resolution": "999p",
	})
	require.Error(t, err)
}

func TestEstimateVideoBilling_KlingPro(t *testing.T) {
	r, err := estimateVideoBilling("kling-v2-6", map[string]any{
		"duration": 5,
		"mode":     "pro",
	})
	require.NoError(t, err)
	assert.InDelta(t, 0.0625/0.0368, r["tier"], 1e-6)
}

func TestEstimateVideoBilling_PerCallModelsNotUsed(t *testing.T) {
	// music/mj go through units path in EstimateBilling; estimateVideoBilling still works if called
	r, err := estimateVideoBilling("some-unknown-video", map[string]any{"duration": 3})
	require.NoError(t, err)
	assert.Equal(t, 3.0, r["seconds"])
}

func TestEstimateVideoBilling_AutoDurationUsesOfficialThirtySecondHold(t *testing.T) {
	for _, model := range []string{"seedance-2-5", "wan-3.0"} {
		r, err := estimateVideoBilling(model, map[string]any{"duration": -1})
		require.NoError(t, err)
		assert.Equal(t, 30.0, r["seconds"])
	}
	_, err := estimateVideoBilling("seedance-2-0", map[string]any{"duration": -1})
	require.Error(t, err)
}

func TestEstimateVideoBilling_AspectSizeIsNotMisreadAsResolution(t *testing.T) {
	r, err := estimateVideoBilling("seedance-2-0", map[string]any{
		"duration": 5,
		"size":     "16:9",
	})
	require.NoError(t, err)
	assert.Equal(t, 5.0, r["seconds"])
}

func TestEstimateVideoBilling_FractionalDurationRejected(t *testing.T) {
	_, err := estimateVideoBilling("wan-3.0", map[string]any{"duration": 4.5})
	require.Error(t, err)
}

func TestHasMediaInput_RecognizesRoleAudioAndDocumentFields(t *testing.T) {
	for _, body := range []map[string]any{
		{"image_with_roles": []any{map[string]any{"url": "asset://a", "role": "first_frame"}}},
		{"audio_urls": []any{"asset://a"}},
		{"file_url": "https://example.com/a.pdf"},
		{"link_url": "https://example.com/page"},
	} {
		assert.True(t, hasMediaInput(body))
	}
}
