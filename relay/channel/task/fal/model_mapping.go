package fal

import (
	"fmt"
	"strings"
)

// isExplicitEndpoint checks if the model name explicitly points to a specific sub-endpoint.
func isExplicitEndpoint(m string) (string, bool) {
	switch m {
	case "minimax/h3-max/text-to-video", "minimax/h3-max-t2v", "h3-max-t2v", "h3-max-text-to-video", "h3-max/text-to-video", "minimax/h3-t2v", "h3-t2v", "minimax/h3/text-to-video", "h3/text-to-video":
		return ModelMinimaxH3MaxTextToVideo, true
	case "minimax/h3-max/image-to-video", "minimax/h3-max-i2v", "h3-max-i2v", "h3-max-image-to-video", "h3-max/image-to-video", "minimax/h3-i2v", "h3-i2v", "minimax/h3/image-to-video", "h3/image-to-video":
		return ModelMinimaxH3MaxImageToVideo, true
	case "minimax/h3-max/reference-to-video", "minimax/h3-max-r2v", "h3-max-r2v", "h3-max-reference-to-video", "h3-max/reference-to-video", "minimax/h3-r2v", "h3-r2v", "minimax/h3/reference-to-video", "h3/reference-to-video":
		return ModelMinimaxH3MaxReferenceToVideo, true
	case "minimax/h3-max-turbo/text-to-video", "minimax/h3-max-turbo-t2v", "h3-max-turbo-t2v", "h3-max-turbo-text-to-video", "h3-max-turbo/text-to-video", "minimax/h3-turbo-t2v", "h3-turbo-t2v", "minimax/h3-turbo/text-to-video", "h3-turbo/text-to-video":
		return ModelMinimaxH3MaxTurboTextToVideo, true
	case "minimax/h3-max-turbo/image-to-video", "minimax/h3-max-turbo-i2v", "h3-max-turbo-i2v", "h3-max-turbo-image-to-video", "h3-max-turbo/image-to-video", "minimax/h3-turbo-i2v", "h3-turbo-i2v", "minimax/h3-turbo/image-to-video", "h3-turbo/image-to-video":
		return ModelMinimaxH3MaxTurboImageToVideo, true
	case "minimax/h3-max/lip-sync/image-to-video", "minimax/h3-max/lip-sync", "minimax-h3-lip-sync", "h3-max-lip-sync", "h3-lip-sync", "minimax/h3-lip-sync":
		return ModelMinimaxH3MaxLipSyncImageToVideo, true
	default:
		return "", false
	}
}

// isH3MaxBaseModel checks if the model name refers to the base H3 Max model.
func isH3MaxBaseModel(m string) bool {
	switch m {
	case "minimax/h3-max", "h3-max", "minimax/h3", "h3", "minimax-h3", "minimax-h3-task", "minimax-h3-max":
		return true
	default:
		return false
	}
}

// isH3MaxTurboBaseModel checks if the model name refers to the base H3 Max Turbo model.
func isH3MaxTurboBaseModel(m string) bool {
	switch m {
	case "minimax/h3-max-turbo", "h3-max-turbo", "minimax/h3-turbo", "h3-turbo", "minimax-h3-turbo", "minimax-h3-max-turbo":
		return true
	default:
		return false
	}
}

// GetFalEndpoint resolves a model name or alias to the canonical fal.ai Minimax H3 endpoint.
func GetFalEndpoint(modelName string) string {
	m := strings.ToLower(strings.TrimSpace(modelName))
	m = strings.TrimPrefix(m, "fal-ai/")
	m = strings.TrimPrefix(m, "fal/")

	if ep, ok := isExplicitEndpoint(m); ok {
		return ep
	}
	if isH3MaxBaseModel(m) {
		return ModelMinimaxH3MaxTextToVideo
	}
	if isH3MaxTurboBaseModel(m) {
		return ModelMinimaxH3MaxTurboTextToVideo
	}
	return m
}

// ResolveFalEndpoint dynamically routes a model name to the appropriate fal.ai Minimax H3 endpoint
// based on request body parameters (multimodal dispatch), while preserving explicitly specified endpoints.
func ResolveFalEndpoint(modelName string, raw map[string]any) string {
	m := strings.ToLower(strings.TrimSpace(modelName))
	m = strings.TrimPrefix(m, "fal-ai/")
	m = strings.TrimPrefix(m, "fal/")

	// 1. Explicit specific endpoint takes precedence (backward compatibility)
	if ep, ok := isExplicitEndpoint(m); ok {
		return ep
	}

	// fork: Canonical operation takes precedence over media inference.
	op, _ := raw["operation"].(string)
	media, _ := parseMediaInputs(raw) // Payload validation reports malformed inputs.
	if isH3MaxBaseModel(m) {
		switch op {
		case "text_to_video":
			return ModelMinimaxH3MaxTextToVideo
		case "first_frame":
			return ModelMinimaxH3MaxImageToVideo
		case "video_multi_ref":
			return ModelMinimaxH3MaxReferenceToVideo
		}
		if len(media.videos)+len(media.audios) > 0 || len(media.images) > 1 {
			return ModelMinimaxH3MaxReferenceToVideo
		}
		if len(media.images) == 1 {
			return ModelMinimaxH3MaxImageToVideo
		}
	}
	if isH3MaxTurboBaseModel(m) {
		if op == "first_frame" || (op == "" && len(media.images) > 0) {
			return ModelMinimaxH3MaxTurboImageToVideo
		}
	}

	return GetFalEndpoint(modelName)
}

// IsSupportedModel checks if the model is one of the supported Minimax H3 models.
func IsSupportedModel(modelName string) bool {
	ep := GetFalEndpoint(modelName)
	for _, m := range ModelList {
		if ep == m {
			return true
		}
	}
	return false
}

// FormatUpstreamTaskID encodes endpoint and requestID into a composite ID: "{endpoint}:{requestID}".
func FormatUpstreamTaskID(endpoint, requestID string) string {
	endpoint = strings.TrimSpace(endpoint)
	requestID = strings.TrimSpace(requestID)
	if endpoint == "" {
		return requestID
	}
	return fmt.Sprintf("%s:%s", endpoint, requestID)
}

// ParseUpstreamTaskID decodes a composite ID into endpoint and requestID.
// If the taskID does not contain ':', endpoint is returned as empty string and the caller may fallback.
func ParseUpstreamTaskID(taskID string) (string, string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", "", fmt.Errorf("task_id is empty")
	}
	idx := strings.LastIndex(taskID, ":")
	if idx > 0 && idx < len(taskID)-1 {
		endpoint := taskID[:idx]
		reqID := taskID[idx+1:]
		return endpoint, reqID, nil
	}
	return "", taskID, nil
}
