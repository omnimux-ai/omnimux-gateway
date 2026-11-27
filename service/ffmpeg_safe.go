package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type safeVideoContainerKind int

const (
	safeVideoContainerUnknown safeVideoContainerKind = iota
	safeVideoContainerISOBMFF                        // MP4 / MOV / M4A / 3GP
	safeVideoContainerEBML                           // Matroska / WebM
)

var (
	errUnsupportedVideoContainer = errors.New("unsupported or invalid video container")
	errFFmpegOutputLimitExceeded = errors.New("media processing output limit exceeded")
)

// detectSafeVideoContainer inspects the container magic bytes so playlists
// (HLS/M3U8, concat, SDP) and arbitrary non-media inputs are rejected before
// ffmpeg is ever spawned.
func detectSafeVideoContainer(header []byte) safeVideoContainerKind {
	if len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp")) {
		return safeVideoContainerISOBMFF
	}
	if len(header) >= 4 && header[0] == 0x1a && header[1] == 0x45 && header[2] == 0xdf && header[3] == 0xa3 {
		return safeVideoContainerEBML
	}
	return safeVideoContainerUnknown
}

func detectSafeVideoContainerFile(path string) (safeVideoContainerKind, error) {
	f, err := os.Open(path)
	if err != nil {
		return safeVideoContainerUnknown, err
	}
	defer f.Close()
	var buf [16]byte
	n, err := io.ReadFull(f, buf[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return safeVideoContainerUnknown, err
	}
	kind := detectSafeVideoContainer(buf[:n])
	if kind == safeVideoContainerUnknown {
		return safeVideoContainerUnknown, errUnsupportedVideoContainer
	}
	return kind, nil
}

func safeVideoFFmpegInputArgs(kind safeVideoContainerKind, inputPath string) []string {
	args := []string{
		"-nostdin", "-hide_banner", "-v", "error",
		"-threads", "1",
		"-max_alloc", "67108864",
		"-probesize", "10M",
		"-analyzeduration", "5M",
		"-protocol_whitelist", "file,pipe",
	}
	switch kind {
	case safeVideoContainerEBML:
		args = append(args, "-format_whitelist", "matroska,webm")
	default:
		args = append(args,
			"-format_whitelist", "mov,mp4,m4a,3gp,3g2,mj2",
			"-enable_drefs", "0",
			"-use_absolute_path", "0",
		)
	}
	return append(args, "-i", inputPath)
}

func safePipeVideoFFmpegInputArgs(kind safeVideoContainerKind, readAheadBytes int64) []string {
	args := []string{
		"-nostdin", "-hide_banner", "-v", "error",
		"-threads", "1",
		"-max_alloc", "67108864",
		"-probesize", "10M",
		"-analyzeduration", "5M",
		"-protocol_whitelist", "cache,pipe",
		"-read_ahead_limit", strconv.FormatInt(readAheadBytes, 10),
	}
	switch kind {
	case safeVideoContainerEBML:
		args = append(args, "-format_whitelist", "matroska,webm")
	default:
		args = append(args,
			"-format_whitelist", "mov,mp4,m4a,3gp,3g2,mj2",
			"-enable_drefs", "0",
			"-use_absolute_path", "0",
		)
	}
	return append(args, "-i", "cache:pipe:0")
}

// runBoundedFFmpegOutput executes tool with strict stdout/stderr byte caps so
// malicious media cannot exhaust process memory.
func runBoundedFFmpegOutput(ctx context.Context, tool string, args []string, stdin io.Reader, maxStdoutBytes, maxStderrBytes int) ([]byte, string, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(runCtx, tool, args...)
	cmd.Stdin = stdin
	stdout := &influencerVideoProbeBuffer{limit: maxStdoutBytes, cancel: cancel}
	stderr := &influencerVideoProbeBuffer{limit: maxStderrBytes, cancel: cancel}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 2 * time.Second

	err := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return nil, stderr.String(), errFFmpegOutputLimitExceeded
	}
	if err != nil {
		return nil, stderr.String(), err
	}
	return stdout.Bytes(), stderr.String(), nil
}
