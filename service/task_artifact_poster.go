package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/QuantumNous/new-api/model"
)

// TaskArtifactPosterKey names the stored first-frame cover of a video artifact.
// It lives beside the video object under the same task prefix, so a cover can
// never outlive or drift from the video it belongs to.
const TaskArtifactPosterKey = "poster"

// maxTaskArtifactPosterBytes bounds one extracted cover. A 9:16 still at the
// sizes the workbench renders stays far below this; the cap only exists so a
// misbehaving decoder cannot stream an unbounded image into memory.
const maxTaskArtifactPosterBytes int64 = 2 << 20

// errTaskArtifactPosterEmpty reports a decode that produced no frame at all.
var errTaskArtifactPosterEmpty = errors.New("poster frame is empty")

// VideoPosterExtractor turns a video payload into a JPEG cover. It is a
// package variable so tests can substitute a deterministic implementation
// instead of depending on the host's ffmpeg build.
var VideoPosterExtractor = extractVideoPosterFrame

// extractVideoPosterFrame decodes the first frame of an in-memory video. The
// bounded cache pipe permits seeking back when an MP4 index follows its media.
// Only the local pipe/cache protocols are available to the decoder.
func extractVideoPosterFrame(ctx context.Context, video []byte) ([]byte, error) {
	if len(video) == 0 {
		return nil, errTaskArtifactPosterEmpty
	}
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", "cache,pipe",
		"-read_ahead_limit", "33554432",
		"-i", "cache:pipe:0",
		"-frames:v", "1",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(video)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("poster decode: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("poster decode: %w", err)
	}
	frame, readErr := io.ReadAll(io.LimitReader(stdout, maxTaskArtifactPosterBytes+1))
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, fmt.Errorf("poster decode: %w", readErr)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("poster decode: %w: %s", waitErr, trimPosterStderr(stderr.String()))
	}
	if int64(len(frame)) > maxTaskArtifactPosterBytes {
		return nil, fmt.Errorf("poster decode: frame exceeds %d bytes", maxTaskArtifactPosterBytes)
	}
	if len(frame) == 0 {
		return nil, errTaskArtifactPosterEmpty
	}
	return frame, nil
}

func trimPosterStderr(value string) string {
	const limit = 240
	trimmed := bytes.TrimSpace([]byte(value))
	if len(trimmed) > limit {
		trimmed = trimmed[:limit]
	}
	return string(trimmed)
}

// PersistVideoArtifactPoster stores the first frame of a video payload as the
// task's cover. Callers treat a failure as non-fatal: the video itself is
// already served, and the cover is regenerated the next time the video is
// persisted or preheated.
func PersistVideoArtifactPoster(ctx context.Context, task *model.Task, video []byte) error {
	frame, err := VideoPosterExtractor(ctx, video)
	if err != nil {
		return err
	}
	_, err = PersistTaskArtifact(ctx, task, TaskArtifactPosterKey, "image/jpeg", frame)
	return err
}

// IsTaskPosterStored reports whether the task already has a stored cover.
func IsTaskPosterStored(task *model.Task) bool {
	return IsTaskArtifactStored(task, TaskArtifactPosterKey)
}
