package service

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfluencerVideoAudioContainerRounding(t *testing.T) {
	for _, test := range []struct {
		name    string
		length  int64
		codec   string
		allowed bool
	}{
		{"AAC one sample longer", 2049, "aac", true},
		{"AAC two samples longer", 2050, "aac", false},
		{"PCM one sample longer", 2049, "pcm_s16le", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var result influencerVideoProbeResult
			length := strconv.FormatInt(test.length, 10)
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"`+test.codec+`","sample_rate":"44100","time_base":"1/44100","nb_read_frames":"2"}],"packets":[{"stream_index":0,"pts":0,"duration":120},{"stream_index":1,"pts":0,"duration":`+length+`},{"stream_index":1,"pts":`+length+`,"duration":2048}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120},{"media_type":"audio","stream_index":1,"pts":0,"best_effort_timestamp":0,"duration":`+length+`,"nb_samples":2048},{"media_type":"audio","stream_index":1,"pts":`+length+`,"best_effort_timestamp":`+length+`,"duration":2048,"nb_samples":2048}],"format":{"format_name":"mov,mp4","duration":"4"}}`), &result))
			_, err := influencerVideoMetadataFromProbe(result, true)
			if test.allowed {
				require.NoError(t, err, "container rounding must not manufacture an audio gap")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInfluencerVideoAudioTailTrimIsNotAppliedTwice(t *testing.T) {
	var result influencerVideoProbeResult
	require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"44100","time_base":"1/44100","nb_read_frames":"1"}],"packets":[{"stream_index":0,"pts":0,"duration":120},{"stream_index":1,"pts":0,"duration":962,"side_data_list":[{"side_data_type":"Skip Samples","discard_padding":1107}]}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120},{"media_type":"audio","stream_index":1,"pts":0,"best_effort_timestamp":0,"duration":962,"nb_samples":2048}],"format":{"format_name":"mov,mp4","duration":"4"}}`), &result))
	metadata, err := influencerVideoMetadataFromProbe(result, true)
	require.NoError(t, err, "tail discard describes decoded samples, not already shortened packet duration")
	assert.Equal(t, 4.0, metadata.Duration)
}

func TestInfluencerVideoAudioTailPresentation(t *testing.T) {
	for _, test := range []struct {
		name     string
		duration string
		allowed  bool
	}{
		{"single movie edit explains two tail bounds", "4.021337868", true},
		{"decoded tail with missing declared duration", "", true},
		{"decoded tail overrides rounded declared duration", "4.03", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"44100","time_base":"1/44100","duration":"`+test.duration+`","nb_read_frames":"1"}],"packets":[{"stream_index":0,"pts":0,"duration":120},{"stream_index":1,"pts":176400,"duration":962,"side_data_list":[{"side_data_type":"Skip Samples","discard_padding":1107}]}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120},{"media_type":"audio","stream_index":1,"pts":176400,"best_effort_timestamp":176400,"duration":962,"nb_samples":2048}],"format":{"format_name":"mov,mp4","duration":"4.03"}}`), &result))
			metadata, err := influencerVideoMetadataFromProbe(result, true)
			if test.allowed {
				require.NoError(t, err)
				assert.InDelta(t, 4+941.0/44100, metadata.Duration, 1e-12, "tail must retain 941 samples, not 962 or a second subtraction")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInfluencerVideoAudioCompatibilityBoundaries(t *testing.T) {
	for _, test := range []struct {
		name                                string
		length, next, tail, discard, offset int64
		flags                               string
		allowed                             bool
	}{
		{"one sample shorter with exact container continuity", 2047, 2047, 962, 1107, 0, "", true},
		{"unexplained sample gap", 2048, 2049, 962, 1107, 0, "", false},
		{"rounding plus extra gap", 2049, 2050, 962, 1107, 0, "", false},
		{"two internal short samples", 2046, 2046, 962, 1107, 0, "", false},
		{"corrupt packet", 2049, 2049, 962, 1107, 0, "C", false},
		{"discarded packet with presented sound", 2049, 2049, 962, 1107, 0, "D", false},
		{"discard exceeds raw samples", 2048, 2048, 962, 2049, 0, "", false},
		{"negative discard", 2048, 2048, 962, -1, 0, "", false},
		{"full tail with padding only", 2048, 2048, 2048, 1107, 0, "", true},
		{"short tail without padding", 2048, 2048, 962, 0, 0, "", true},
		{"exact effective thirty seconds", 2049, 2049, 962, 1107, 1323000 - 2049 - 941, "", true},
		{"one effective sample beyond thirty", 2049, 2049, 962, 1107, 1323001 - 2049 - 941, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"44100","time_base":"1/44100","nb_read_frames":"2"}],"packets":[{"stream_index":0,"pts":0,"duration":900},{"stream_index":1,"pts":0,"duration":2048},{"stream_index":1,"pts":2048,"duration":962,"side_data_list":[{"side_data_type":"Skip Samples","discard_padding":1107}]}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":900},{"media_type":"audio","stream_index":1,"pts":0,"best_effort_timestamp":0,"duration":2048,"nb_samples":2048},{"media_type":"audio","stream_index":1,"pts":2048,"best_effort_timestamp":2048,"duration":962,"nb_samples":2048}],"format":{"format_name":"mov,mp4","duration":"30"}}`), &result))
			*result.Packets[1].PTS, *result.Frames[1].PTS, *result.Frames[1].BestPTS = test.offset, test.offset, test.offset
			*result.Packets[1].Duration, *result.Frames[1].Duration = test.length, test.length
			*result.Packets[2].PTS, *result.Frames[2].PTS, *result.Frames[2].BestPTS = test.offset+test.next, test.offset+test.next, test.offset+test.next
			*result.Packets[2].Duration, *result.Frames[2].Duration = test.tail, test.tail
			result.Packets[2].SideData[0].Discard = test.discard
			result.Packets[1].Flags = test.flags
			metadata, err := influencerVideoMetadataFromProbe(result, true)
			if test.allowed {
				require.NoError(t, err)
				assert.Equal(t, 30.0, metadata.Duration)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInfluencerVideoAudioSameBlockSkipAndDiscard(t *testing.T) {
	for _, skip := range []int64{100, 942} {
		t.Run(strconv.FormatInt(skip, 10), func(t *testing.T) {
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"44100","time_base":"1/44100","nb_read_frames":"1"}],"packets":[{"stream_index":0,"pts":0,"duration":120},{"stream_index":1,"pts":-100,"duration":962,"side_data_list":[{"side_data_type":"Skip Samples","skip_samples":100,"discard_padding":1107}]}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120},{"media_type":"audio","stream_index":1,"pts":-100,"best_effort_timestamp":-100,"duration":962,"nb_samples":2048}],"format":{"format_name":"mov,mp4","duration":"4"}}`), &result))
			*result.Packets[1].PTS, *result.Frames[1].PTS, *result.Frames[1].BestPTS = -skip, -skip, -skip
			result.Packets[1].SideData[0].Skip = skip
			_, err := influencerVideoMetadataFromProbe(result, true)
			if skip == 100 {
				require.NoError(t, err)
			} else {
				require.Error(t, err, "head and tail cuts cannot consume more than the effective block")
			}
		})
	}
}

func TestInfluencerVideoAACRoundingPrefixes(t *testing.T) {
	for _, test := range []struct {
		name    string
		deltas  []int64
		allowed bool
	}{
		{"positive cumulative drift", []int64{1, 1}, false},
		{"negative cumulative drift", []int64{-1, -1}, false},
		{"intermediate drift cannot be hidden by final zero", []int64{1, 1, -1, -1}, false},
		{"alternating quantization remains bounded", []int64{1, -1, 1, -1}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"44100","time_base":"1/44100"}],"packets":[{"stream_index":0,"pts":0,"duration":120}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120}],"format":{"format_name":"mov,mp4","duration":"4"}}`), &result))
			result.Streams[1].DecodedCount = strconv.Itoa(len(test.deltas) + 1)
			var pts int64
			for _, delta := range append(test.deltas, 0) {
				position, duration, samples := pts, int64(2048)+delta, int64(2048)
				result.Packets = append(result.Packets, influencerVideoProbeEvent{Index: 1, PTS: &position, Duration: &duration})
				result.Frames = append(result.Frames, influencerVideoProbeEvent{Index: 1, Kind: "audio", PTS: &position, BestPTS: &position, Duration: &duration, Samples: &samples})
				pts += duration
			}
			_, err := influencerVideoMetadataFromProbe(result, true)
			if test.allowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err, "every cumulative prefix must remain bounded")
			}
		})
	}
}

func TestInfluencerVideoRecordedCompatibleVersions(t *testing.T) {
	root := os.Getenv("OMNIMUX_COMPAT_PROBE_RESULTS")
	if root == "" {
		t.Skip("set OMNIMUX_COMPAT_PROBE_RESULTS for recorded media engine compatibility")
	}
	for _, name := range []string{"probe.json", "image-probe.json"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(root + string(os.PathSeparator) + name)
			require.NoError(t, err)
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal(data, &result))
			metadata, err := influencerVideoMetadataFromProbe(result, true)
			require.NoError(t, err)
			require.NoError(t, ValidateInfluencerVideoMetadata(metadata))
			assert.Equal(t, 10.1, metadata.Duration)
		})
	}
}

func TestProbeInfluencerVideoCompatibleOriginal(t *testing.T) {
	path := os.Getenv("OMNIMUX_COMPAT_REFERENCE_VIDEO")
	if path == "" {
		t.Skip("set OMNIMUX_COMPAT_REFERENCE_VIDEO for the user's original media acceptance")
	}
	metadata, err := ProbeInfluencerVideoFile(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, 576, metadata.Width)
	assert.Equal(t, 1024, metadata.Height)
	assert.InDelta(t, 10.1, metadata.Duration, 0.000001)
}
