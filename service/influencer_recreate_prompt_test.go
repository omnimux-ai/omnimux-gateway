package service

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// influencerRecreateTestClip renders a small H.264 clip so the clip preparation
// reads real bytes instead of a stubbed probe.
func influencerRecreateTestClip(t *testing.T, seconds int) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not available")
	}
	path := filepath.Join(t.TempDir(), "clip.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=480x854:r=24",
		"-t", itoa(seconds), "-c:v", "libx264", "-pix_fmt", "yuv420p", "-threads", "1", path,
	).CombinedOutput()
	require.NoError(t, err, string(output))
	bytes, err := os.ReadFile(path)
	require.NoError(t, err)
	return bytes
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// The generated length follows the clip and the vision turn receives timestamped
// JPEG frames, not the clip itself. A caller can neither choose the length nor
// make the planner send a video to the analysis model.
func TestInfluencerRecreatePrepareClipFollowsClip(t *testing.T) {
	for _, seconds := range []int{4, 6, 15, 30} {
		seconds := seconds
		t.Run(itoa(seconds)+"s", func(t *testing.T) {
			clip, err := influencerRecreatePrepareClip(context.Background(), influencerRecreateTestClip(t, seconds))
			require.NoError(t, err)
			assert.Equal(t, seconds, clip.DurationSec)
			assert.InDelta(t, float64(seconds), clip.ReferenceSeconds, 0.2)
			expected := int(float64(seconds)*influencerRecreateFrameSecondsPer + 0.5)
			if expected < influencerRecreateFrameMinCount {
				expected = influencerRecreateFrameMinCount
			}
			if expected > influencerRecreateFrameMaxCount {
				expected = influencerRecreateFrameMaxCount
			}
			assert.Len(t, clip.Frames, expected)
			require.Len(t, clip.FrameTimes, expected)
			for index, frame := range clip.Frames {
				require.Greater(t, len(frame), 1024)
				assert.True(t, bytes.HasPrefix(frame, []byte{0xFF, 0xD8}), "frame %d is not a JPEG", index)
				if index > 0 {
					assert.Greater(t, clip.FrameTimes[index], clip.FrameTimes[index-1])
				}
				assert.GreaterOrEqual(t, clip.FrameTimes[index], 0.0)
				assert.Less(t, clip.FrameTimes[index], float64(seconds))
			}
		})
	}
}

func TestInfluencerRecreatePrepareClipBounds(t *testing.T) {
	_, err := influencerRecreatePrepareClip(context.Background(), influencerRecreateTestClip(t, 2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "至少")

	// The shared probe already refuses media longer than the 30-second input
	// ceiling, so the planner never sees an over-limit clip.
	_, err = influencerRecreatePrepareClip(context.Background(), influencerRecreateTestClip(t, 31))
	require.Error(t, err)
}

func TestInfluencerRecreatePrepareClipRejectsBrokenBytes(t *testing.T) {
	_, err := influencerRecreatePrepareClip(context.Background(), []byte("not a video"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无法解析")
}

// The vision turn must carry the frames plus their timestamps and no video part,
// because the clip is decomposition material only.
func TestInfluencerRecreateAnalyseVideoSendsFrames(t *testing.T) {
	original := InfluencerMotionChat
	t.Cleanup(func() { InfluencerMotionChat = original })
	var captured []map[string]any
	InfluencerMotionChat = func(ctx context.Context, system string, content []map[string]any, check func(string) error, session *influencerMotionPlanSession) (string, error) {
		captured = content
		reply := `{"shots":[{"shot_id":"S01","start_s":0,"end_s":4.0,"narrative_job":"hook"}]}`
		if check != nil {
			require.NoError(t, check(reply))
		}
		return reply, nil
	}

	clip := influencerRecreateClip{
		DurationSec:      8,
		ReferenceSeconds: 7.6,
		Frames:           [][]byte{{0xFF, 0xD8, 0xFF, 0xD9}, {0xFF, 0xD8, 0xFF, 0xD9}},
		FrameTimes:       []float64{1.9, 5.7},
	}
	_, err := influencerRecreateAnalyseVideo(context.Background(), clip, InfluencerRecreatePlanInput{UserPrompt: "换我的产品"}, newInfluencerMotionPlanSession())
	require.NoError(t, err)

	images := 0
	for _, block := range captured {
		assert.NotEqual(t, "file", block["type"], "the plan must not send a video part")
		if block["type"] == "image_url" {
			images++
		}
	}
	assert.Equal(t, 2, images)
	joined := ""
	for _, block := range captured {
		if text, ok := block["text"].(string); ok {
			joined += text
		}
	}
	assert.Contains(t, joined, "1.9s")
	assert.Contains(t, joined, "5.7s")
	assert.Contains(t, joined, "关键帧共 2 张")
}

// A plan that runs past the request budget maps to the retryable timeout error.
func TestInfluencerRecreatePlanFailureMapsDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := influencerRecreatePlanFailure(ctx, context.DeadlineExceeded)
	assert.ErrorIs(t, err, ErrInfluencerRecreateTimeout)
	assert.ErrorIs(t, influencerRecreatePlanFailure(context.Background(), context.DeadlineExceeded), ErrInfluencerRecreateTimeout)
	assert.NotErrorIs(t, influencerRecreatePlanFailure(context.Background(), assert.AnError), ErrInfluencerRecreateTimeout)
}
