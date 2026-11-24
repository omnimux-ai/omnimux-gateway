package apimart

import (
	"math"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
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

func TestEstimateVideoBilling_Wan30EvoLinkResolutionTiers(t *testing.T) {
	for _, tc := range []struct {
		resolution string
		wantTier   float64
	}{
		{"480P", 0.25},
		{"720p", 0.5},
		{"1080P", 1},
		{"", 0.5},
	} {
		body := map[string]any{"duration": 5}
		if tc.resolution != "" {
			body["resolution"] = tc.resolution
		}
		r, err := estimateVideoBillingWithRateModel("wan-3.0", "wan3.0-text-to-video", body)
		require.NoError(t, err, tc.resolution)
		assert.Equal(t, 5.0, r["seconds"], tc.resolution)
		tier := 1.0
		if value, ok := r["tier"]; ok {
			tier = value
		}
		assert.InDelta(t, tc.wantTier, tier, 1e-9, tc.resolution)
	}
}

func TestEstimateVideoBilling_Wan30PrimeUpstreamUsesStandardTable(t *testing.T) {
	r, err := estimateVideoBillingWithRateModel("wan-3.0", "wan3.0-prime-text-to-video", map[string]any{
		"duration":   5,
		"resolution": "720P",
	})
	require.NoError(t, err)
	// The Prime group applies its 1.5 group ratio; the resolution tier itself
	// is the same official Standard table.
	assert.InDelta(t, 0.5, r["tier"], 1e-9)
}

func TestEstimateVideoBilling_PidoiWanSkuKeepsNoTier(t *testing.T) {
	r, err := estimateVideoBillingWithRateModel("wan-3.0", "wan30-720p", map[string]any{
		"duration":   5,
		"resolution": "720P",
	})
	require.NoError(t, err)
	_, hasTier := r["tier"]
	assert.False(t, hasTier)
}

func TestEstimateVideoBilling_Wan30UnknownResolutionRejected(t *testing.T) {
	_, err := estimateVideoBillingWithRateModel("wan-3.0", "wan3.0-text-to-video", map[string]any{
		"duration":   5,
		"resolution": "2K",
	})
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

func TestEstimateVideoBillingWithPolicy_NoneDisablesTierAndKeepsBounds(t *testing.T) {
	cases := []struct {
		name        string
		originModel string
		rateModel   string
		body        map[string]any
		wantSeconds float64
		wantAuto    bool
	}{
		{
			name:        "seedance-2-5 480p with media input has no tier discount",
			originModel: "seedance-2-5",
			rateModel:   "seedance-2.5",
			body:        map[string]any{"duration": 5, "resolution": "480p", "image_urls": []any{"https://cdn.example/a.png"}},
			wantSeconds: 5,
		},
		{
			name:        "seedance-2-5 1080p allowed under group pricing",
			originModel: "seedance-2-5",
			rateModel:   "seedance-2.5",
			body:        map[string]any{"duration": 6, "resolution": "1080p"},
			wantSeconds: 6,
		},
		{
			name:        "seedance-2-5 auto duration marks autoDuration and holds 30s",
			originModel: "seedance-2-5",
			rateModel:   "seedance-2.5",
			body:        map[string]any{"duration": -1, "resolution": "1080p"},
			wantSeconds: 30,
			wantAuto:    true,
		},
		{
			name:        "kling-v2-6 pro-sound skips preset tier",
			originModel: "kling-v2-6",
			rateModel:   "kling-v2-6",
			body:        map[string]any{"duration": 8, "mode": "pro", "sound": true},
			wantSeconds: 8,
		},
		{
			name:        "wan-3.0 skips preset tier",
			originModel: "wan-3.0",
			rateModel:   "wan-3.0",
			body:        map[string]any{"duration": 5, "resolution": "480P"},
			wantSeconds: 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ratios, auto, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyNone, tc.originModel, tc.rateModel, tc.body)
			require.NoError(t, err)
			assert.Equal(t, map[string]float64{"seconds": tc.wantSeconds}, ratios)
			assert.Equal(t, tc.wantAuto, auto)
		})
	}

	// Duration bounds and model auto-duration eligibility remain enforced under "none".
	for _, invalid := range []struct {
		name  string
		model string
		body  map[string]any
	}{
		{"above max task duration", "kling-v3", map[string]any{"duration": relaycommon.MaxTaskDurationSeconds + 1}},
		{"negative below -1", "seedance-2-5", map[string]any{"duration": -2}},
		{"fractional duration", "seedance-2-5", map[string]any{"duration": 4.5}},
		{"NaN duration", "seedance-2-5", map[string]any{"duration": math.NaN()}},
		{"Inf duration", "seedance-2-5", map[string]any{"duration": math.Inf(1)}},
		{"unsupported auto duration model", "seedance-2-0", map[string]any{"duration": -1}},
	} {
		t.Run("rejects "+invalid.name, func(t *testing.T) {
			_, _, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyNone, invalid.model, invalid.model, invalid.body)
			require.Error(t, err)
		})
	}
}

func TestEstimateVideoBillingWithPolicy_EmptyDefaultsToPresetAndInvalidFailsClosed(t *testing.T) {
	body := map[string]any{"duration": 5, "resolution": "480p", "image_urls": []any{"https://cdn.example/a.png"}}

	emptyRatios, emptyAuto, err := estimateVideoBillingWithPolicy("", "seedance-2-5", "seedance-2.5", body)
	require.NoError(t, err)
	presetRatios, presetAuto, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyPreset, "seedance-2-5", "seedance-2.5", body)
	require.NoError(t, err)
	assert.Equal(t, presetRatios, emptyRatios)
	assert.Equal(t, presetAuto, emptyAuto)
	assert.InDelta(t, 0.0576/0.216, presetRatios["tier"], 1e-9)

	// seedance-2-5 1080p remains fail-closed in preset / empty mode with guidance.
	for _, policy := range []string{"", kitdto.VideoTierBillingPolicyPreset} {
		_, _, err := estimateVideoBillingWithPolicy(policy, "seedance-2-5", "seedance-2.5", map[string]any{
			"duration":   5,
			"resolution": "1080p",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `video_tier_billing_policy="none"`)
	}

	// Unknown policy values fail closed.
	for _, invalid := range []string{"custom", "flat", "NONE", " "} {
		_, _, err := estimateVideoBillingWithPolicy(invalid, "seedance-2-5", "seedance-2.5", map[string]any{"duration": 5})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "video_tier_billing_policy")
	}
}

func TestEstimateVideoBilling_Wan30HyphenAndDotParityAndFixedSKU(t *testing.T) {
	for _, tc := range []struct {
		resolution string
		wantTier   float64
	}{
		{"480P", 0.25},
		{"720p", 0.5},
		{"1080P", 1},
		{"", 0.5},
	} {
		body := map[string]any{"duration": 5}
		if tc.resolution != "" {
			body["resolution"] = tc.resolution
		}
		hyphenRatios, _, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyPreset, "wan-3.0", "wan-3.0", body)
		require.NoError(t, err, tc.resolution)
		dotRatios, _, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyPreset, "wan-3.0", "wan3.0-text-to-video", body)
		require.NoError(t, err, tc.resolution)
		assert.Equal(t, dotRatios, hyphenRatios, tc.resolution)

		tier := 1.0
		if v, ok := hyphenRatios["tier"]; ok {
			tier = v
		}
		assert.InDelta(t, tc.wantTier, tier, 1e-9, tc.resolution)
	}

	// Unknown resolution rejected for public wan-3.0 just like wan3.0-*.
	_, _, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyPreset, "wan-3.0", "wan-3.0", map[string]any{
		"duration":   5,
		"resolution": "2K",
	})
	require.Error(t, err)

	// Fixed-resolution SKUs without dot/hyphen keep no tier.
	for _, fixedSKU := range []string{"wan30-720p", "wan30-1080p-fast"} {
		ratios, _, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyPreset, "wan-3.0", fixedSKU, map[string]any{
			"duration":   5,
			"resolution": "720P",
		})
		require.NoError(t, err, fixedSKU)
		assert.Equal(t, map[string]float64{"seconds": 5}, ratios, fixedSKU)
	}
}

func TestParseBoundedPositiveSeconds(t *testing.T) {
	for _, valid := range []struct {
		in   any
		want float64
	}{
		{8, 8},
		{int64(12), 12},
		{float64(8.5), 8.5},
		{"15", 15},
		{" 7.25 ", 7.25},
		{float64(relaycommon.MaxTaskDurationSeconds), float64(relaycommon.MaxTaskDurationSeconds)},
	} {
		got, ok := parseBoundedPositiveSeconds(valid.in)
		require.True(t, ok, "%v", valid.in)
		assert.Equal(t, valid.want, got)
	}

	for _, invalid := range []any{
		nil, "", "   ", "abc", 0, -1, -5.5, "0", "-3",
		math.NaN(), math.Inf(1), math.Inf(-1), "NaN", "Infinity",
		relaycommon.MaxTaskDurationSeconds + 1,
		float64(relaycommon.MaxTaskDurationSeconds) + 0.1,
		uint64(18446744073686646784),
	} {
		_, ok := parseBoundedPositiveSeconds(invalid)
		assert.False(t, ok, "%v", invalid)
	}
}
