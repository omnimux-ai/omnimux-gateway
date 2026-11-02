package service

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateInfluencerVideoMetadata(t *testing.T) {
	valid := InfluencerVideoMetadata{
		Duration: 8.055873, Width: 720, Height: 1280,
		FrameRate: 24, VideoCodec: "h264", AudioCodec: "aac", Container: "mp4",
	}
	for _, tc := range []struct {
		name      string
		change    func(*InfluencerVideoMetadata)
		wantError bool
	}{
		{name: "original user clip"},
		{name: "thirty seconds boundary", change: func(m *InfluencerVideoMetadata) { m.Duration = 30 }},
		{name: "fraction over boundary", change: func(m *InfluencerVideoMetadata) { m.Duration = 30.001 }, wantError: true},
		{name: "rounded long clip", change: func(m *InfluencerVideoMetadata) { m.Duration = 30.4 }, wantError: true},
		{name: "ordinary reference below four seconds", change: func(m *InfluencerVideoMetadata) { m.Duration = 3.999 }},
		{name: "ordinary reference lower boundary", change: func(m *InfluencerVideoMetadata) { m.Duration = 2 }},
		{name: "below ordinary reference boundary", change: func(m *InfluencerVideoMetadata) { m.Duration = 1.999 }, wantError: true},
		{name: "nonfinite duration", change: func(m *InfluencerVideoMetadata) { m.Duration = math.Inf(1) }, wantError: true},
		{name: "missing duration", change: func(m *InfluencerVideoMetadata) { m.Duration = 0 }, wantError: true},
		{name: "too few pixels", change: func(m *InfluencerVideoMetadata) { m.Width, m.Height = 360, 640 }, wantError: true},
		{name: "too many pixels", change: func(m *InfluencerVideoMetadata) { m.Width, m.Height = 6000, 6000 }, wantError: true},
		{name: "fps below contract", change: func(m *InfluencerVideoMetadata) { m.FrameRate = 23 }, wantError: true},
		{name: "fps above contract", change: func(m *InfluencerVideoMetadata) { m.FrameRate = 61 }, wantError: true},
		{name: "webm is not declared input", change: func(m *InfluencerVideoMetadata) { m.Container = "webm" }, wantError: true},
		{name: "hevc supported", change: func(m *InfluencerVideoMetadata) { m.VideoCodec = "hevc" }},
		{name: "unsupported video codec", change: func(m *InfluencerVideoMetadata) { m.VideoCodec = "av1" }, wantError: true},
		{name: "no audio is valid", change: func(m *InfluencerVideoMetadata) { m.AudioCodec = "" }},
		{name: "mp4 pcm unsupported", change: func(m *InfluencerVideoMetadata) { m.AudioCodec = "pcm_s16le" }, wantError: true},
		{name: "mov pcm supported", change: func(m *InfluencerVideoMetadata) { m.Container, m.AudioCodec = "mov", "pcm_s16le" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata := valid
			if tc.change != nil {
				tc.change(&metadata)
			}
			err := ValidateInfluencerVideoMetadata(metadata)
			if tc.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInfluencerVideoReferenceRulesSeparateInputAndEditing(t *testing.T) {
	valid := InfluencerVideoMetadata{Duration: 2, Width: 1000, Height: 800, FrameRate: 24, VideoCodec: "h264", Container: "mp4", SizeBytes: InfluencerVideoMaxBytes}
	for _, tc := range []struct {
		name, mode string
		duration   float64
		wantError  string
	}{
		{"two second ordinary input", "motion", 2, ""},
		{"fractional ordinary input", "motion", 3.999, ""},
		{"editing below boundary", "replace", 3.999, "4至30秒"},
		{"editing at boundary", "replace", 4, ""},
		{"retain fractional original", "replace", 9.708, ""},
		{"unknown mode", "other", 4, "参考方式无效"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			media := valid
			media.Duration = tc.duration
			err := ValidateInfluencerVideoReferenceMetadata(media, tc.mode)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
	for _, dimensions := range [][2]int{{1000, 800}, {800, 800}, {960, 720}, {1680, 720}, {600, 1500}, {1500, 600}} {
		media := valid
		media.Width, media.Height = dimensions[0], dimensions[1]
		require.NoError(t, ValidateInfluencerVideoReferenceMetadata(media, "motion"), "input must accept continuous aspect ratios independently from output choices")
	}
	valid.SizeBytes++
	require.ErrorContains(t, ValidateInfluencerVideoReferenceMetadata(valid, "motion"), "200MiB")
	valid.SizeBytes = 0
	valid.Duration, valid.VideoDuration = 4, 2
	require.NoError(t, ValidateInfluencerVideoReferenceMetadata(valid, "motion"))
	require.ErrorContains(t, ValidateInfluencerVideoReferenceMetadata(valid, "replace"), "4至30秒", "a longer audio tail cannot make a short visual input valid for editing")
}

func TestInfluencerVideoInputRulesAndMetadataJSON(t *testing.T) {
	for _, mode := range []string{"motion", "replace"} {
		rules, err := GetInfluencerVideoInputRules(mode)
		require.NoError(t, err)
		encoded, err := common.Marshal(rules)
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		assert.Equal(t, float64(209715200), decoded["max_bytes"])
		assert.Equal(t, 0.4, decoded["min_aspect_ratio"])
		assert.Equal(t, 2.5, decoded["max_aspect_ratio"])
		minDuration := 2.0
		if mode == "replace" {
			minDuration = 4
		}
		assert.Equal(t, minDuration, decoded["min_duration_seconds"])
		assert.Equal(t, 30.0, decoded["max_duration_seconds"])
	}
	_, err := GetInfluencerVideoInputRules("other")
	require.Error(t, err)
	encoded, err := common.Marshal(InfluencerVideoMetadata{Duration: 9.708, Width: 1000, Height: 800, FrameRate: 24, VideoCodec: "h264", Container: "mp4", SizeBytes: 1234})
	require.NoError(t, err)
	var metadata map[string]any
	require.NoError(t, common.Unmarshal(encoded, &metadata))
	assert.Equal(t, 9.708, metadata["duration"])
	assert.Equal(t, float64(1234), metadata["size_bytes"])
	assert.Equal(t, 24.0, metadata["frame_rate"])
	assert.Equal(t, "h264", metadata["video_codec"])
}
