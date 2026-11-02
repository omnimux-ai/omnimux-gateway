package fal

import (
	"fmt"
	"strconv"
	"strings"
)

func buildSubmitPayload(endpoint string, raw map[string]any) (*FalVideoSubmitPayload, error) {
	endpoint = GetFalEndpoint(endpoint)
	prompt := extractPrompt(raw)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	payload := &FalVideoSubmitPayload{
		Prompt:          prompt,
		Duration:        extractDuration(raw),
		AspectRatio:     extractAspectRatio(raw),
		Resolution:      extractResolution(raw),
		PromptOptimizer: true,
	}

	if po, ok := raw["prompt_optimizer"].(bool); ok {
		payload.PromptOptimizer = po
	}

	media, err := parseMediaInputs(raw)
	if err != nil {
		return nil, err
	}
	if err := validateMediaOperation(endpoint, raw, media); err != nil {
		return nil, err
	}
	if len([]rune(prompt)) > 50000 {
		return nil, fmt.Errorf("prompt exceeds 50000 characters")
	}
	if resolution := normalizedResolution(raw); resolution == "720P" || resolution == "720" {
		return nil, fmt.Errorf("720p is retired for H3; explicitly select 768p")
	}
	switch endpoint {
	case ModelMinimaxH3MaxImageToVideo, ModelMinimaxH3MaxTurboImageToVideo:
		payload.ImageURL = media.images[0]
	case ModelMinimaxH3MaxReferenceToVideo:
		payload.ReferenceImageURLs = media.images
		payload.ReferenceVideoURLs = media.videos
		payload.ReferenceAudioURLs = media.audios
	}

	return payload, nil
}

func extractPrompt(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	if p, ok := raw["prompt"].(string); ok && strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	if p, ok := raw["text"].(string); ok && strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	if p, ok := raw["description"].(string); ok && strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	return ""
}

func extractImageURL(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	if u, ok := raw["image_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if u, ok := raw["image"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if u, ok := raw["input_reference"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if u, ok := raw["first_frame_image"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if imgs, ok := raw["images"].([]any); ok && len(imgs) > 0 {
		switch first := imgs[0].(type) {
		case string:
			if strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		case map[string]any:
			if u, ok := first["url"].(string); ok && strings.TrimSpace(u) != "" {
				return strings.TrimSpace(u)
			}
		}
	}
	if imgs, ok := raw["images"].([]string); ok && len(imgs) > 0 {
		if strings.TrimSpace(imgs[0]) != "" {
			return strings.TrimSpace(imgs[0])
		}
	}
	return ""
}

func extractVideoURL(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	if u, ok := raw["video_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if u, ok := raw["reference_video_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if u, ok := raw["video"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if vids, ok := raw["videos"].([]any); ok && len(vids) > 0 {
		switch first := vids[0].(type) {
		case string:
			if strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		case map[string]any:
			if u, ok := first["url"].(string); ok && strings.TrimSpace(u) != "" {
				return strings.TrimSpace(u)
			}
		}
	}
	if vids, ok := raw["videos"].([]string); ok && len(vids) > 0 {
		if strings.TrimSpace(vids[0]) != "" {
			return strings.TrimSpace(vids[0])
		}
	}
	return ""
}

func extractDuration(raw map[string]any) string {
	if raw == nil {
		return Duration5
	}
	dVal, ok := raw["duration"]
	if !ok {
		return Duration5
	}
	d := parseDurationInt(dVal)
	if d >= 10 {
		return Duration10
	}
	return Duration5
}

func parseDurationInt(val any) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		v = strings.TrimSuffix(v, "s")
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 5
	}
}

func extractAspectRatio(raw map[string]any) string {
	if raw == nil {
		return AspectRatio16x9
	}
	if ar, ok := raw["aspect_ratio"].(string); ok && strings.TrimSpace(ar) != "" {
		ar = strings.TrimSpace(ar)
		switch ar {
		case AspectRatio16x9, AspectRatio9x16, AspectRatio1x1, AspectRatio4x3, AspectRatio21x9:
			return ar
		}
	}
	return AspectRatio16x9
}

func normalizedResolution(raw map[string]any) string {
	resolution, _ := raw["resolution"].(string)
	return strings.ToUpper(strings.TrimSpace(resolution))
}

func extractResolution(raw map[string]any) string {
	if raw == nil {
		return Resolution768P
	}
	if res := normalizedResolution(raw); res != "" {
		switch res {
		case Resolution1080P, "1080":
			return Resolution1080P
		case Resolution768P, "768", "720", "720P":
			return Resolution768P
		case Resolution480P, "480":
			return Resolution480P
		}
	}
	return Resolution768P
}

func extractAudioURL(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	for _, key := range []string{"audio_url", "audio", "audio_path", "sound_url", "sound"} {
		if u, ok := raw[key].(string); ok && strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u)
		}
	}
	if auds, ok := raw["audios"].([]any); ok && len(auds) > 0 {
		switch first := auds[0].(type) {
		case string:
			if strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		case map[string]any:
			if u, ok := first["url"].(string); ok && strings.TrimSpace(u) != "" {
				return strings.TrimSpace(u)
			}
		}
	}
	return ""
}

func extractLipSyncResolution(raw map[string]any) string {
	res := normalizedResolution(raw)
	switch res {
	case "480P", "480":
		return "480P"
	case "768P", "768", "720", "720P":
		return "768P"
	case "1080P", "1080":
		return "1080P"
	case "2K", "1440P", "1440":
		return "2K"
	default:
		return "768P"
	}
}

func buildLipSyncPayload(raw map[string]any) (*FalLipSyncSubmitPayload, error) {
	imageURL := extractImageURL(raw)
	if imageURL == "" {
		return nil, fmt.Errorf("image_url is required for lip-sync")
	}
	audioURL := extractAudioURL(raw)
	if audioURL == "" {
		return nil, fmt.Errorf("audio_url is required for lip-sync")
	}

	payload := &FalLipSyncSubmitPayload{
		ImageURL:            imageURL,
		AudioURL:            audioURL,
		Resolution:          extractLipSyncResolution(raw),
		EnableTranscription: false,
	}

	if et, ok := raw["enable_transcription"].(bool); ok {
		payload.EnableTranscription = et
	}
	if seedVal, ok := raw["seed"]; ok {
		switch s := seedVal.(type) {
		case int:
			val := int64(s)
			payload.Seed = &val
		case int64:
			payload.Seed = &s
		case float64:
			val := int64(s)
			payload.Seed = &val
		}
	}
	if sc, ok := raw["enable_safety_checker"].(bool); ok {
		payload.EnableSafetyChecker = &sc
	}

	return payload, nil
}
