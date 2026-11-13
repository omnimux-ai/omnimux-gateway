package service

import (
	"context"
	"math/big"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These fixtures exercise the installed media engine and the workbench's local
// input envelope. They do not establish acceptance by any supplier SKU.
func TestInfluencerVideoRealSupportedFormats(t *testing.T) {
	for _, test := range []struct {
		name, extension, size, rate, video, audio string
		width, height                             int
		fps                                       float64
		rotate                                    bool
	}{
		{"mov pcm square 25fps", "mov", "720x720", "25", "h264", "pcm_s16le", 720, 720, 25, false},
		{"hevc portrait 24fps", "mp4", "640x960", "24", "hevc", "", 640, 960, 24, false},
		{"mp3 rotated 90 degrees", "mp4", "1280x720", "30", "h264", "mp3", 720, 1280, 30, true},
		{"landscape 60fps", "mp4", "1280x720", "60", "h264", "", 1280, 720, 60, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "reference."+test.extension)
			args := []string{"-f", "lavfi", "-i", "color=c=black:s=" + test.size + ":r=" + test.rate}
			if test.audio != "" {
				args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100")
			}
			encoder := "libx264"
			if test.video == "hevc" {
				encoder = "libx265"
			}
			args = append(args, "-t", "4", "-c:v", encoder, "-threads", "1")
			if test.video == "hevc" {
				args = append(args, "-x265-params", "pools=none:frame-threads=1:log-level=error", "-tag:v", "hvc1")
			}
			if test.audio == "" {
				args = append(args, "-an")
			} else if test.audio == "mp3" {
				args = append(args, "-c:a", "libmp3lame")
			} else {
				args = append(args, "-c:a", test.audio)
			}
			runInfluencerFormatFFmpeg(t, append(args, file)...)
			if test.rotate {
				rotated := filepath.Join(t.TempDir(), "rotated.mp4")
				help, err := exec.Command("ffmpeg", "-hide_banner", "-h", "full").CombinedOutput()
				require.NoError(t, err)
				if strings.Contains(string(help), "-display_rotation") {
					runInfluencerFormatFFmpeg(t, "-display_rotation", "90", "-i", file, "-map", "0", "-c", "copy", rotated)
				} else {
					runInfluencerFormatFFmpeg(t, "-i", file, "-map", "0", "-c", "copy", "-metadata:s:v:0", "rotate=90", rotated)
				}
				file = rotated
				output, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream_side_data=rotation", "-of", "json", file).CombinedOutput()
				require.NoError(t, err, string(output))
				var result struct {
					Streams []struct {
						SideData []struct {
							Rotation int `json:"rotation"`
						} `json:"side_data_list"`
					} `json:"streams"`
				}
				require.NoError(t, common.Unmarshal(output, &result))
				require.NotEmpty(t, result.Streams)
				require.Len(t, result.Streams[0].SideData, 1, "fixture must contain a real display matrix")
				require.Equal(t, 90, result.Streams[0].SideData[0].Rotation)
			}
			metadata, err := ProbeInfluencerVideoFile(context.Background(), file)
			require.NoError(t, err, "declared local supported format must survive real full decoding")
			require.NoError(t, ValidateInfluencerVideoMetadata(metadata))
			assert.Equal(t, test.extension, metadata.Container)
			assert.Equal(t, test.video, metadata.VideoCodec)
			assert.Equal(t, test.audio, metadata.AudioCodec)
			assert.Equal(t, test.width, metadata.Width)
			assert.Equal(t, test.height, metadata.Height)
			assert.Equal(t, test.fps, metadata.FrameRate)
			assert.InDelta(t, 4, metadata.Duration, 0.05, "MP3 tail padding may extend the four-second generated input")
			t.Logf("real probe+validate accepted: %+v", metadata)
		})
	}
}

func TestInfluencerVideoRealInvalidStreams(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "audio only is not reference video",
			args: []string{"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "4", "-c:a", "aac"},
			want: "一条有效视频画面",
		},
		{
			name: "two video streams are ambiguous",
			args: []string{"-f", "lavfi", "-i", "color=c=black:s=720x720:r=30", "-t", "4", "-map", "0:v", "-map", "0:v", "-an", "-c:v", "libx264", "-threads", "1"},
			want: "一条有效视频画面",
		},
		{
			name: "mpeg4 codec is outside the local envelope",
			args: []string{"-f", "lavfi", "-i", "color=c=black:s=720x720:r=30", "-t", "4", "-an", "-c:v", "mpeg4", "-threads", "1"},
			want: "编码必须为",
		},
		{
			name: "vfr with actual average below 24fps",
			// Keep every frame in the first two seconds and every third frame
			// thereafter: a real varying cadence, not a forged FPS header.
			args: []string{"-f", "lavfi", "-i", "color=c=black:s=720x720:r=30:d=5", "-vf", "select='lt(t,2)+gte(t,2)*not(mod(n,3))'", "-fps_mode", "vfr", "-an", "-c:v", "libx264", "-threads", "1"},
			want: "帧率必须",
		},
		{
			name: "small dimensions do not pass by mp4 extension",
			args: []string{"-f", "lavfi", "-i", "color=c=black:s=360x640:r=30", "-t", "4", "-an", "-c:v", "libx264", "-threads", "1"},
			want: "总像素",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "reference.mp4")
			runInfluencerFormatFFmpeg(t, append(test.args, file)...)
			if test.name == "vfr with actual average below 24fps" {
				output, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_packets", "-show_entries", "packet=pts:stream=avg_frame_rate", "-of", "json", file).CombinedOutput()
				require.NoError(t, err, string(output))
				var result struct {
					Packets []struct {
						PTS *int64 `json:"pts"`
					} `json:"packets"`
					Streams []struct {
						Rate string `json:"avg_frame_rate"`
					} `json:"streams"`
				}
				require.NoError(t, common.Unmarshal(output, &result))
				// Packet duration is not the presentation interval on older engines.
				// Sort presentation timestamps because B-frames arrive in decode order.
				positions := make([]int64, 0, len(result.Packets))
				for _, packet := range result.Packets {
					require.NotNil(t, packet.PTS, "fixture requires actual presentation timestamps")
					positions = append(positions, *packet.PTS)
				}
				require.Len(t, positions, 90, "selection must preserve the intended frames")
				sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
				intervals := make(map[int64]bool)
				for i := 1; i < len(positions); i++ {
					interval := positions[i] - positions[i-1]
					require.Positive(t, interval, "presentation timestamps must be distinct")
					intervals[interval] = true
				}
				require.Greater(t, len(intervals), 1, "fixture must have genuinely variable cadence")
				require.Len(t, result.Streams, 1)
				rate, ok := new(big.Rat).SetString(result.Streams[0].Rate)
				require.True(t, ok, "fixture average frame rate must be a rational number")
				require.Positive(t, rate.Sign())
				require.Negative(t, rate.Cmp(big.NewRat(24, 1)), "fixture average must remain below the real input gate")
				t.Logf("real VFR presentation_intervals=%v average_rate=%+v", intervals, result.Streams)
			}
			_, err := ProbeInfluencerVideoFile(context.Background(), file)
			require.ErrorContains(t, err, test.want)
			t.Logf("real probe rejected: %v", err)
		})
	}
}

func runInfluencerFormatFFmpeg(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args = append([]string{"-nostdin", "-v", "error"}, args...)
	command := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := command.CombinedOutput()
	exit := -1
	if command.ProcessState != nil {
		exit = command.ProcessState.ExitCode()
	}
	t.Logf("fixture ffmpeg argv=%q exit=%d", args, exit)
	require.NoError(t, err, string(output))
}
