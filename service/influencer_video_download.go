package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// influencerVideoPreparationTimeout bounds one reference-video download and the
// optional local probe that reads it. It is shared by the server inspector and
// the browser-declared preparation path.
const influencerVideoPreparationTimeout = 90 * time.Second

func downloadInfluencerVideo(ctx context.Context, videoURL, dest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateSSRFProtectedFetchURL(videoURL); err != nil {
		return errors.New("参考视频地址不符合安全要求")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return err
	}
	resp, err := GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: 下载参考视频失败", ErrInfluencerVideoUnavailable)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: 参考视频暂时无法下载", ErrInfluencerVideoUnavailable)
	}
	if resp.ContentLength > InfluencerVideoMaxBytes {
		return errors.New("参考视频超过200MiB接收上限")
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("%w: 暂时无法保存参考视频", ErrInfluencerVideoUnavailable)
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, InfluencerVideoMaxBytes+1))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("%w: 下载参考视频失败", ErrInfluencerVideoUnavailable)
	}
	if n <= 0 || n > InfluencerVideoMaxBytes {
		return errors.New("参考视频为空或超过200MiB接收上限")
	}
	return nil
}
