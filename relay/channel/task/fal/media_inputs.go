package fal

import (
	"fmt"
	"net/url"
	"strings"
)

// falMediaInputs preserves the order of each canonical reference collection.
type falMediaInputs struct {
	images []string
	videos []string
	audios []string
	framed bool
}

func parseMediaInputs(raw map[string]any) (falMediaInputs, error) {
	media := falMediaInputs{}
	groups := []struct {
		keys []string
		out  *[]string
	}{
		{[]string{"image_with_roles", "image_urls", "reference_image_urls", "images", "image_url", "image", "input_reference", "first_frame_image"}, &media.images},
		{[]string{"video_urls", "reference_video_urls", "videos", "video_url", "reference_video_url", "video"}, &media.videos},
		{[]string{"audio_urls", "reference_audio_urls"}, &media.audios},
	}
	for _, group := range groups {
		selected := ""
		for _, key := range group.keys {
			value, present := raw[key]
			if !present {
				continue
			}
			if selected != "" {
				return media, fmt.Errorf("ambiguous media fields: %s and %s", selected, key)
			}
			selected = key
			var rows []any
			switch v := value.(type) {
			case string:
				rows = []any{v}
			case []any:
				rows = v
			case []string:
				for _, item := range v {
					rows = append(rows, item)
				}
			default:
				return media, fmt.Errorf("%s must contain URL strings", key)
			}
			for _, row := range rows {
				var text string
				if object, ok := row.(map[string]any); ok {
					text, _ = object["url"].(string)
					if key == "image_with_roles" {
						if object["role"] != "first_frame" {
							return media, fmt.Errorf("H3 only supports the first_frame image role")
						}
						media.framed = true
					}
				} else {
					text, _ = row.(string)
				}
				text = strings.TrimSpace(text)
				if !validHTTPMediaURL(text) {
					return media, fmt.Errorf("%s contains an invalid HTTP(S) media URL", key)
				}
				*group.out = append(*group.out, text)
			}
		}
	}
	return media, nil
}

// validHTTPMediaURL validates media syntax without probing the remote resource.
func validHTTPMediaURL(text string) bool {
	parsed, err := url.Parse(text)
	return err == nil && parsed.Hostname() != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

func validateMediaOperation(endpoint string, raw map[string]any, media falMediaInputs) error {
	op, _ := raw["operation"].(string)
	if value, exists := raw["operation"]; exists && (op == "" || value == nil) {
		return fmt.Errorf("operation must be a non-empty string")
	}
	want := "text_to_video"
	switch endpoint {
	case ModelMinimaxH3MaxImageToVideo, ModelMinimaxH3MaxTurboImageToVideo:
		want = "first_frame"
		if len(media.images) != 1 || len(media.videos)+len(media.audios) != 0 {
			return fmt.Errorf("first_frame requires exactly one image and no video or audio")
		}
	case ModelMinimaxH3MaxReferenceToVideo:
		want = "video_multi_ref"
		if media.framed {
			return fmt.Errorf("reference inputs cannot contain frame roles")
		}
		total := len(media.images) + len(media.videos) + len(media.audios)
		if len(media.images)+len(media.videos) == 0 || total > 12 || len(media.images) > 9 || len(media.videos) > 3 || len(media.audios) > 3 {
			return fmt.Errorf("reference-to-video requires 1–12 references, at most 9 images, 3 videos and 3 audio files")
		}
	case ModelMinimaxH3MaxTextToVideo, ModelMinimaxH3MaxTurboTextToVideo:
		if len(media.images)+len(media.videos)+len(media.audios) != 0 {
			return fmt.Errorf("text_to_video does not consume media; select a supported media operation")
		}
	default:
		return fmt.Errorf("unsupported fal endpoint: %s", endpoint)
	}
	if op != "" && op != want {
		return fmt.Errorf("operation %s is incompatible with %s", op, endpoint)
	}
	return nil
}
