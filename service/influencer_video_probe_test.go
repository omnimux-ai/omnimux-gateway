package service

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeInfluencerVideoReadsCompleteMedia(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("ffmpeg required for real media gate acceptance")
	}
	for _, seconds := range []string{"4", "30", "30.1"} {
		t.Run(seconds, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "reference.mp4")
			command := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=30", "-t", seconds, "-an", "-c:v", "libx264", "-threads", "1", file)
			result, err := command.CombinedOutput()
			require.NoError(t, err, string(result))
			metadata, err := ProbeInfluencerVideoFile(context.Background(), file)
			if seconds == "30.1" {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 720, metadata.Width)
			assert.Equal(t, 1280, metadata.Height)
			assert.Equal(t, 30.0, metadata.FrameRate)
			assert.NoError(t, ValidateInfluencerVideoMetadata(metadata))
		})
	}
}

func TestInfluencerVideoActualTimelineRejectsShortClipWithFalseHeader(t *testing.T) {
	var result influencerVideoProbeResult
	require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","duration":"8","nb_read_frames":"30","nb_frames":"30","sample_aspect_ratio":"1:1"}],"packets":[{"stream_index":0,"pts":0,"duration":1},{"stream_index":0,"pts":29,"duration":1}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"8","tags":{"major_brand":"isom"}}}`), &result))
	for i := int64(0); i < 30; i++ {
		pts, duration := i, int64(1)
		result.Frames = append(result.Frames, influencerVideoProbeEvent{Index: 0, Kind: "video", PTS: &pts, BestPTS: &pts, Duration: &duration})
	}
	result.Packets = append([]influencerVideoProbeEvent(nil), result.Frames...)
	metadata, err := influencerVideoMetadataFromProbe(result, true)
	if err == nil {
		err = ValidateInfluencerVideoMetadata(metadata)
	}
	require.Error(t, err, "actual one-second video must not pass with forged eight-second metadata")
}

func TestProbeInfluencerVideoExact30WithAudio(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exact30-audio.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "30", "-c:v", "libx264", "-threads", "1", "-c:a", "aac", file).CombinedOutput()
	require.NoError(t, err, string(output))
	metadata, err := ProbeInfluencerVideoFile(context.Background(), file)
	require.NoError(t, err, "legal AAC preroll and final partial sample block must not reject exact30")
	assert.Equal(t, 30.0, metadata.Duration)
}

func TestProbeInfluencerVideoRejectsComplexEditList(t *testing.T) {
	file := filepath.Join(t.TempDir(), "edited.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=30", "-t", "4", "-an", "-c:v", "libx264", "-threads", "1", file).CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	at := bytes.Index(data, []byte("elst"))
	require.Greater(t, at, 0)
	// A dwell edit (rate zero) is not ordinary single-segment encoder preroll.
	data[at+20], data[at+21] = 0, 0
	require.NoError(t, os.WriteFile(file, data, 0600))
	_, err = ProbeInfluencerVideoFile(context.Background(), file)
	require.Error(t, err)
}

func TestProbeInfluencerVideoOriginalReadOnly(t *testing.T) {
	path := os.Getenv("OMNIMUX_REFERENCE_VIDEO_ORIGINAL")
	if path == "" {
		t.Skip("set OMNIMUX_REFERENCE_VIDEO_ORIGINAL to the original read-only fixture")
	}
	metadata, err := ProbeInfluencerVideoFile(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, 720, metadata.Width)
	assert.Equal(t, 1280, metadata.Height)
	assert.InDelta(t, 8.055873, metadata.Duration, 0.000001)
}

func TestInfluencerVideoDecodedTimelineRejectsUncertainPresentation(t *testing.T) {
	for _, frame := range []string{
		`{"stream_index":0,"media_type":"video","pts":900,"best_effort_timestamp":900,"duration":1}`,
		`{"stream_index":0,"media_type":"video","pts":0,"best_effort_timestamp":0}`,
		`{"stream_index":0,"media_type":"video","pts":-1,"best_effort_timestamp":-1,"duration":120}`,
		`{"stream_index":0,"media_type":"video","pts":0,"best_effort_timestamp":1,"duration":120}`,
	} {
		t.Run(frame, func(t *testing.T) {
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","duration":"4","nb_read_frames":"1","nb_frames":"1","sample_aspect_ratio":"1:1"}],"packets":[{"stream_index":0,"pts":0,"duration":120}],"frames":[`+frame+`],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"4"}}`), &result))
			_, err := influencerVideoMetadataFromProbe(result, true)
			require.Error(t, err)
		})
	}
}

func TestInfluencerVideoDecodedTimelineRejectsMissingFrames(t *testing.T) {
	var result influencerVideoProbeResult
	require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","duration":"4","nb_read_frames":"120","nb_frames":"120","sample_aspect_ratio":"1:1"}],"packets":[{"stream_index":0,"pts":0,"duration":120}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"4"}}`), &result))
	_, err := influencerVideoMetadataFromProbe(result, true)
	require.Error(t, err, "packets and a declared decoded count cannot replace actual presented frames")
}

func TestProbeInfluencerVideoWarningClassification(t *testing.T) {
	for _, test := range []struct {
		name, warning string
		allowed       bool
	}{
		{"ordinary AAC edit preroll", "[mov @ 123] st: 1 edit list: 1 Missing key frame while searching for timestamp: 0\n[mov @ 123] st: 1 edit list 1 Cannot find an index entry before timestamp: 0", true},
		{"external reference", "[mov @ 123] Skipped opening external track: stream 1", false},
		{"unknown warning", "[mov @ 123] unknown damaged timeline", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			tool := filepath.Join(dir, "ffprobe")
			json := `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","duration":"4","time_base":"1/30","nb_read_frames":"1","nb_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"8000","time_base":"1/8000","nb_read_frames":"2"}],"packets":[{"stream_index":0,"pts":0,"duration":120},{"stream_index":1,"pts":-1,"duration":1,"flags":"KD","side_data_list":[{"side_data_type":"Skip Samples","skip_samples":1}]},{"stream_index":1,"pts":0,"duration":4}],"frames":[{"media_type":"audio","stream_index":1,"pts":-1,"best_effort_timestamp":-1,"duration":1,"nb_samples":1},{"media_type":"audio","stream_index":1,"pts":0,"best_effort_timestamp":0,"duration":4,"nb_samples":4},{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120}],"format":{"format_name":"mov,mp4","duration":"4"}}`
			script := "#!/bin/sh\nprintf '%s\\n' '" + test.warning + "' >&2\nprintf '%s\\n' '" + json + "'\n"
			require.NoError(t, os.WriteFile(tool, []byte(script), 0700))
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			file := filepath.Join(dir, "input.mp4")
			require.NoError(t, os.WriteFile(file, []byte{0, 0, 0, 8, 'f', 'r', 'e', 'e'}, 0600))
			_, err := ProbeInfluencerVideoFile(context.Background(), file)
			if test.allowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInfluencerVideoProbeOutputLimitCancelsProcess(t *testing.T) {
	cancelled := false
	buffer := &influencerVideoProbeBuffer{limit: 4, cancel: func() { cancelled = true }}
	_, err := buffer.Write([]byte("12345"))
	require.Error(t, err)
	assert.True(t, cancelled)
	assert.Zero(t, buffer.Len())
}

func TestProbeInfluencerVideoRejectsNonMediaAndCancellation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "fake.mp4")
	require.NoError(t, os.WriteFile(file, []byte("#EXTM3U\nfile:///etc/passwd\n"), 0600))
	_, err := ProbeInfluencerVideoFile(context.Background(), file)
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ProbeInfluencerVideoFile(ctx, file)
	require.Error(t, err)
}

func TestProbeInfluencerVideoReapsCancelledProcess(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "ready")
	require.NoError(t, syscall.Mkfifo(fifo, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ffprobe"), []byte("#!/bin/sh\nprintf '%s\\n' $$ > '"+fifo+"'\nexec sleep 60\n"), 0700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	file := filepath.Join(dir, "input.mp4")
	require.NoError(t, os.WriteFile(file, []byte{0, 0, 0, 8, 'f', 'r', 'e', 'e'}, 0600))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := ProbeInfluencerVideoFile(ctx, file); done <- err }()
	ready, err := os.ReadFile(fifo)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(ready)))
	require.NoError(t, err)
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(4 * time.Second):
		t.Fatal("cancelled media process did not return")
	}
	assert.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH, "direct media process must have exited before slot release")
}

func TestProbeInfluencerVideoReapsOutputOverflow(t *testing.T) {
	python, err := exec.LookPath("python3")
	require.NoError(t, err)
	dir := t.TempDir()
	script := "#!" + python + "\n" + `import os,time
data=b'x'*(2*1024*1024+1)
while data:
 n=os.write(1,data)
 data=data[n:]
time.sleep(60)
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(script), 0700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	file := filepath.Join(dir, "input.mp4")
	require.NoError(t, os.WriteFile(file, []byte{0, 0, 0, 8, 'f', 'r', 'e', 'e'}, 0600))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err = ProbeInfluencerVideoFile(ctx, file)
	require.ErrorIs(t, err, ErrInfluencerVideoOutputLimit)
	require.NoError(t, ctx.Err(), "overflow must cancel the process rather than waiting for the request deadline")
}

func TestProbeInfluencerVideoExistingLyingAndDamagedFixtures(t *testing.T) {
	for _, name := range []string{"lying30-valid.mp4", "damaged-original.mp4", "original-tail.mp4"} {
		t.Run(name, func(t *testing.T) {
			root := os.Getenv("OMNIMUX_REFERENCE_VIDEO_FIXTURES")
			if root == "" {
				t.Skip("set OMNIMUX_REFERENCE_VIDEO_FIXTURES for report fixture acceptance")
			}
			metadata, err := ProbeInfluencerVideoFile(context.Background(), filepath.Join(root, name))
			if name == "original-tail.mp4" {
				require.NoError(t, err)
				// Both complete decoded receipts differ by six AAC samples between
				// engine versions. Match the observed results, not a production tolerance.
				assert.Contains(t, []float64{8.055873015873017, 8.056009070294785}, metadata.Duration)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInfluencerVideoRecordedMediaVersions(t *testing.T) {
	root := os.Getenv("OMNIMUX_REFERENCE_VIDEO_FIXTURES")
	if root == "" {
		t.Skip("set OMNIMUX_REFERENCE_VIDEO_FIXTURES for two-version decoded output acceptance")
	}
	for _, version := range []string{"local", "image"} {
		for _, name := range []string{"original", "exact30-audio", "lying30", "tail"} {
			t.Run(version+"/"+name, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(root, version+"-"+name+"-full.json"))
				require.NoError(t, err)
				var result influencerVideoProbeResult
				require.NoError(t, common.Unmarshal(data, &result))
				metadata, err := influencerVideoMetadataFromProbe(result, true)
				if name == "lying30" {
					require.Error(t, err)
					require.Contains(t, err.Error(), "超过30秒")
					return
				}
				require.NoError(t, err)
				require.NoError(t, ValidateInfluencerVideoMetadata(metadata))
				if name == "exact30-audio" {
					assert.Equal(t, 30.0, metadata.Duration)
				}
			})
		}
	}
}

func TestInfluencerVideoNumericBounds(t *testing.T) {
	for _, base := range []string{"1e100000000/1", "1/1000000000000000000000000000", "1000000001/1", "0/30", "-1/30"} {
		var result influencerVideoProbeResult
		require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"`+base+`"}],"format":{"format_name":"mov,mp4","duration":"4"}}`), &result))
		_, err := influencerVideoMetadataFromProbe(result, false)
		require.Error(t, err)
	}
}

func TestInfluencerVideoAudioEffectiveSamples(t *testing.T) {
	for _, test := range []struct {
		name                        string
		pts, samples, skip, discard int64
		allowed                     bool
	}{
		{"exact boundary", 239998, 4, 0, 2, true},
		{"one sample over", 239998, 4, 0, 1, false},
		{"negative without skip", -2, 4, 0, 0, false},
		{"explained preroll", -2, 4, 2, 0, true},
		{"oversized decoded sample block", 0, 65537, 0, 0, false},
		{"discard beyond decoded samples", 0, 4, 0, 5, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			side := `{"side_data_type":"Skip Samples","skip_samples":` + strconv.FormatInt(test.skip, 10) + `,"discard_padding":` + strconv.FormatInt(test.discard, 10) + `}`
			audioPacket := `{"stream_index":1,"pts":` + strconv.FormatInt(test.pts, 10) + `,"duration":` + strconv.FormatInt(test.samples, 10) + `,"side_data_list":[` + side + `]}`
			audioFrame := `{"media_type":"audio","stream_index":1,"pts":` + strconv.FormatInt(test.pts, 10) + `,"best_effort_timestamp":` + strconv.FormatInt(test.pts, 10) + `,"duration":` + strconv.FormatInt(test.samples, 10) + `,"nb_samples":` + strconv.FormatInt(test.samples, 10) + `}`
			var result influencerVideoProbeResult
			require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","duration":"30","nb_read_frames":"1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"8000","time_base":"1/8000","nb_read_frames":"1"}],"packets":[{"stream_index":0,"pts":0,"duration":900},`+audioPacket+`],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":900},`+audioFrame+`],"format":{"format_name":"mov,mp4","duration":"30"}}`), &result))
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

func TestInfluencerVideoRejectsNonzeroPresentationOriginWithoutFloatRounding(t *testing.T) {
	var result influencerVideoProbeResult
	require.NoError(t, common.Unmarshal([]byte(`{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":720,"height":1280,"avg_frame_rate":"30/1","time_base":"1/30","nb_read_frames":"1"}],"packets":[{"stream_index":0,"pts":0,"duration":120}],"frames":[{"media_type":"video","stream_index":0,"pts":0,"best_effort_timestamp":0,"duration":120}],"format":{"format_name":"mov,mp4","duration":"4","start_time":"1e-1000"}}`), &result))
	_, err := influencerVideoMetadataFromProbe(result, true)
	require.Error(t, err)
}

func TestProbeInfluencerVideoRejectsExternalDataReference(t *testing.T) {
	file := filepath.Join(t.TempDir(), "external-dref.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=30", "-t", "4", "-an", "-c:v", "libx264", "-threads", "1", file).CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	at := bytes.Index(data, []byte("url "))
	require.Greater(t, at, 0)
	// The URL entry is no longer self-contained; no path is supplied or opened.
	data[at+7] = 0
	require.NoError(t, os.WriteFile(file, data, 0600))
	_, err = ProbeInfluencerVideoFile(context.Background(), file)
	require.Error(t, err)
}

func outputProbeTestVideo(t *testing.T) []byte {
	t.Helper()
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg required for real media fixture")
	}
	file := filepath.Join(t.TempDir(), "generated-output.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "4", "-c:v", "libx264", "-threads", "1", "-c:a", "aac", file).CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	return data
}

func TestProbeInfluencerVideoPayloadReadsBufferedOutputHeaderOnly(t *testing.T) {
	data := outputProbeTestVideo(t)
	metadata, err := ProbeInfluencerVideoPayload(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, 720, metadata.Width)
	assert.Equal(t, 1280, metadata.Height)
	assert.Equal(t, 24.0, metadata.FrameRate)
	assert.Equal(t, "h264", metadata.VideoCodec)
	assert.Equal(t, "aac", metadata.AudioCodec)
	assert.Equal(t, "mp4", metadata.Container)
	assert.Equal(t, int64(len(data)), metadata.SizeBytes)
	assert.Equal(t, 4.0, metadata.Duration)
	// Header-only passes have no decoded timeline, so the picture span mirrors
	// the container duration instead of being decoded.
	assert.Equal(t, metadata.Duration, metadata.VideoDuration)
}

func TestProbeInfluencerVideoPayloadRejectsInvalidAndOversizeBytes(t *testing.T) {
	_, err := ProbeInfluencerVideoPayload(context.Background(), []byte("not-a-video"))
	require.Error(t, err)
	_, err = ProbeInfluencerVideoPayload(context.Background(), nil)
	require.Error(t, err)
	// The public and internal paths are both bounded by the buffered byte cap.
	data := bytes.Repeat([]byte("x"), 64)
	_, err = probeInfluencerVideoPayload(context.Background(), data, int64(len(data)-1))
	require.Error(t, err)
}

func TestProbeInfluencerVideoPayloadSkipsWhenProbeSlotsBusy(t *testing.T) {
	influencerVideoProbeSlots <- struct{}{}
	influencerVideoProbeSlots <- struct{}{}
	t.Cleanup(func() {
		<-influencerVideoProbeSlots
		<-influencerVideoProbeSlots
	})
	_, err := ProbeInfluencerVideoPayload(context.Background(), []byte("busy"))
	require.ErrorIs(t, err, ErrInfluencerVideoBusy)
}
