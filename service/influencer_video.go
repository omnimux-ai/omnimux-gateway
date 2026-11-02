package service

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// InfluencerVideoMaxBytes is the existing public 200 MiB reference-video contract.
const InfluencerVideoMaxBytes = 200 * 1024 * 1024

// InfluencerVideoMetadata describes the actual reference media, not output
// settings. The shape lives in dto so upload responses can echo a persisted
// declaration without the dto package depending on service.
type InfluencerVideoMetadata = dto.InfluencerVideoMetadata

// InfluencerVideoInputConstraints describes input acceptance independently from output settings.
type InfluencerVideoInputConstraints struct {
	MaxBytes           int64               `json:"max_bytes"`
	MinDurationSeconds float64             `json:"min_duration_seconds"`
	MaxDurationSeconds float64             `json:"max_duration_seconds"`
	MinWidth           int                 `json:"min_width"`
	MaxWidth           int                 `json:"max_width"`
	MinHeight          int                 `json:"min_height"`
	MaxHeight          int                 `json:"max_height"`
	MinPixels          int64               `json:"min_pixels"`
	MaxPixels          int64               `json:"max_pixels"`
	MinAspectRatio     float64             `json:"min_aspect_ratio"`
	MaxAspectRatio     float64             `json:"max_aspect_ratio"`
	MinFrameRate       float64             `json:"min_frame_rate"`
	MaxFrameRate       float64             `json:"max_frame_rate"`
	Containers         []string            `json:"containers"`
	VideoCodecs        []string            `json:"video_codecs"`
	AudioCodecs        map[string][]string `json:"audio_codecs"`
}

func GetInfluencerVideoInputRules(mode string) (InfluencerVideoInputConstraints, error) {
	minimum := 2.0
	switch mode {
	case "motion":
	case "replace":
		minimum = 4
	default:
		return InfluencerVideoInputConstraints{}, errors.New("参考方式无效")
	}
	return InfluencerVideoInputConstraints{
		MaxBytes: InfluencerVideoMaxBytes, MinDurationSeconds: minimum, MaxDurationSeconds: 30,
		MinWidth: 300, MaxWidth: 6000, MinHeight: 300, MaxHeight: 6000,
		MinPixels: 407696, MaxPixels: 8295044, MinAspectRatio: 0.4, MaxAspectRatio: 2.5,
		MinFrameRate: 24, MaxFrameRate: 60, Containers: []string{"mp4", "mov"}, VideoCodecs: []string{"h264", "hevc"},
		AudioCodecs: map[string][]string{"mp4": {"aac", "mp3"}, "mov": {"aac", "mp3", "pcm"}},
	}, nil
}

// ValidateInfluencerVideoMetadata validates ordinary reference inputs. Editing has
// a separate duration minimum; neither validator promises supplier availability.
func ValidateInfluencerVideoMetadata(media InfluencerVideoMetadata) error {
	return ValidateInfluencerVideoReferenceMetadata(media, "motion")
}

func ValidateInfluencerVideoReferenceMetadata(media InfluencerVideoMetadata, mode string) error {
	rules, err := GetInfluencerVideoInputRules(mode)
	if err != nil {
		return err
	}
	if media.SizeBytes < 0 || media.SizeBytes > rules.MaxBytes {
		return errors.New("参考视频超过200MiB接收上限")
	}
	if math.IsNaN(media.Duration) || math.IsInf(media.Duration, 0) || media.Duration <= 0 {
		return errors.New("无法读取视频真实时长，请检查文件")
	}
	if media.Duration < rules.MinDurationSeconds || media.Duration > rules.MaxDurationSeconds {
		return fmt.Errorf("视频实际时长为%.3f秒，必须在%.0f至%.0f秒之间，不会自动剪短", media.Duration, rules.MinDurationSeconds, rules.MaxDurationSeconds)
	}
	if media.VideoDuration != 0 && (math.IsNaN(media.VideoDuration) || math.IsInf(media.VideoDuration, 0) || media.VideoDuration < rules.MinDurationSeconds || media.VideoDuration > rules.MaxDurationSeconds) {
		return fmt.Errorf("视频实际画面时长为%.3f秒，必须在%.0f至%.0f秒之间", media.VideoDuration, rules.MinDurationSeconds, rules.MaxDurationSeconds)
	}
	if media.Container != "mp4" && media.Container != "mov" {
		return errors.New("参考视频必须使用真实MP4或MOV格式")
	}
	if media.VideoCodec != "h264" && media.VideoCodec != "hevc" {
		return errors.New("参考视频编码必须为H.264或H.265")
	}
	if media.Width < rules.MinWidth || media.Width > rules.MaxWidth || media.Height < rules.MinHeight || media.Height > rules.MaxHeight {
		return fmt.Errorf("参考视频尺寸为%d×%d，宽高必须各在300至6000像素之间", media.Width, media.Height)
	}
	pixels := int64(media.Width) * int64(media.Height)
	if pixels < rules.MinPixels || pixels > rules.MaxPixels {
		return fmt.Errorf("参考视频总像素为%d，必须在407696至8295044之间", pixels)
	}
	ratio := float64(media.Width) / float64(media.Height)
	if ratio < rules.MinAspectRatio || ratio > rules.MaxAspectRatio {
		return fmt.Errorf("参考视频宽高比为%.3f，必须在0.4至2.5之间", ratio)
	}
	if math.IsNaN(media.FrameRate) || math.IsInf(media.FrameRate, 0) || media.FrameRate < rules.MinFrameRate || media.FrameRate > rules.MaxFrameRate {
		return errors.New("参考视频帧率必须在每秒24至60帧之间")
	}
	if media.AudioCodec == "" || media.AudioCodec == "aac" || media.AudioCodec == "mp3" {
		return nil
	}
	if media.Container == "mov" && (media.AudioCodec == "pcm" || strings.HasPrefix(media.AudioCodec, "pcm_")) {
		return nil
	}
	return errors.New("参考视频音轨编码不符合要求")
}
