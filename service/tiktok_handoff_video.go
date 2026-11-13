package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// fork: tool-to-workbench — the TikTok tool page hands one parsed video to the
// workbench's viral-recreate slot. The server copies the post's video into the
// redeeming user's own upload library; this file holds the transfer half:
// one SSRF-protected download, the existing 200 MiB reference cap, a real
// probe, and the recreate mode's 4–30 s window. The temporary copy is always
// removed; the stored upload expires through the native file cleanup.

const (
	TikTokHandoffMinSeconds = 4
	TikTokHandoffMaxSeconds = 30
)

// ErrTikTokHandoffDuration marks a video outside the recreate window.
var ErrTikTokHandoffDuration = errors.New("参考复刻视频时长必须在4至30秒之间")

// ErrTikTokHandoffTooLarge marks a video above the reference-video cap.
var ErrTikTokHandoffTooLarge = errors.New("参考视频超过200MiB接收上限")

// tiktokHandoffValidateURL and tiktokHandoffClient are the same SSRF guard and
// client the reference-video downloader uses; tests swap them to reach a
// loopback fixture.
var (
	tiktokHandoffValidateURL = ValidateSSRFProtectedFetchURL
	tiktokHandoffClient      = GetSSRFProtectedHTTPClient
	tiktokHandoffProbe       = ProbeInfluencerVideoFile
)

// TikTokHandoffDurationAllowed reports whether a duration in seconds fits the
// viral-recreate reference window.
func TikTokHandoffDurationAllowed(seconds float64) bool {
	return seconds >= TikTokHandoffMinSeconds && seconds <= TikTokHandoffMaxSeconds
}

// TransferTikTokHandoffVideo downloads one TikTok CDN video, probes it and
// returns its bytes with the probed metadata. The caller has already checked
// that the host is a TikTok media host.
func TransferTikTokHandoffVideo(parent context.Context, videoURL string) ([]byte, InfluencerVideoMetadata, error) {
	var empty InfluencerVideoMetadata
	ctx, cancel := context.WithTimeout(parent, influencerVideoPreparationTimeout)
	defer cancel()
	if err := tiktokHandoffValidateURL(videoURL); err != nil {
		return nil, empty, errors.New("参考视频地址不符合安全要求")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, empty, err
	}
	// The CDN serves the same headers the tool page's media proxy sends.
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://www.tiktok.com/")
	resp, err := tiktokHandoffClient().Do(req)
	if err != nil {
		return nil, empty, fmt.Errorf("%w: 下载参考视频失败", ErrInfluencerVideoUnavailable)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, empty, fmt.Errorf("%w: 参考视频暂时无法下载", ErrInfluencerVideoUnavailable)
	}
	if resp.ContentLength > InfluencerVideoMaxBytes {
		return nil, empty, ErrTikTokHandoffTooLarge
	}
	file, err := os.CreateTemp("", "tiktok-handoff-*.mp4")
	if err != nil {
		return nil, empty, fmt.Errorf("%w: 暂时无法保存参考视频", ErrInfluencerVideoUnavailable)
	}
	path := file.Name()
	defer os.Remove(path)
	n, copyErr := io.Copy(file, io.LimitReader(resp.Body, InfluencerVideoMaxBytes+1))
	closeErr := file.Close()
	if ctx.Err() != nil {
		return nil, empty, ctx.Err()
	}
	if copyErr != nil || closeErr != nil {
		return nil, empty, fmt.Errorf("%w: 下载参考视频失败", ErrInfluencerVideoUnavailable)
	}
	if n > InfluencerVideoMaxBytes {
		return nil, empty, ErrTikTokHandoffTooLarge
	}
	if n <= 0 {
		return nil, empty, fmt.Errorf("%w: 参考视频为空", ErrInfluencerVideoUnavailable)
	}
	media, err := tiktokHandoffProbe(ctx, path)
	if err != nil {
		return nil, empty, err
	}
	if !TikTokHandoffDurationAllowed(media.Duration) {
		return nil, empty, ErrTikTokHandoffDuration
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, empty, fmt.Errorf("%w: 暂时无法读取参考视频", ErrInfluencerVideoUnavailable)
	}
	return data, media, nil
}
