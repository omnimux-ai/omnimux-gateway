package apimart

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/modelcontracts"
)

const (
	opTextToVideo     = "text_to_video"
	opFirstFrame      = "first_frame"
	opEndFrame        = "end_frame"
	opFirstLastFrame  = "first_last_frame"
	opVideoMultiRef   = "video_multi_ref"
	opVideoEdit       = "video_edit"
	opVideoExtend     = "video_extend"
	opDocumentToVideo = "document_to_video"
	opWebpageToVideo  = "webpage_to_video"
)

var phaseOneWireModels = map[string]string{
	"seedance-2-0":           "seedance-2.0",
	"seedance-2-0-fast":      "seedance-2.0-fast",
	"seedance-2-0-mini":      "seedance-2.0-mini",
	"seedance-2-5":           "seedance-2.5",
	"seedance-2-5-flash":     "seedance-2-5-flash",
	"magi-2":                 "magi-2-text-to-video",
	"wan-3.0":                "wan3.0-video",
	"minimax-h3":             "MiniMax-H3",
	"grok-imagine-video-1-5": "grok-imagine-video-1.5",
}

func canonicalPhaseOneVideoModel(name string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "seedance-2-0", "seedance-2.0":
		return "seedance-2-0", true
	case "seedance-2-0-fast", "seedance-2.0-fast":
		return "seedance-2-0-fast", true
	case "seedance-2-0-mini", "seedance-2.0-mini":
		return "seedance-2-0-mini", true
	case "seedance-2-5", "seedance-2.5":
		return "seedance-2-5", true
	case "seedance-2-5-flash", "seedance-2.5-flash", "doubao-seedance-2-5-flash", "doubao-seedance-2-5-flash-261115":
		return "seedance-2-5-flash", true
	case "magi-2":
		return "magi-2", true
	case "wan-3.0", "wan3.0-video":
		return "wan-3.0", true
	case "minimax-h3":
		return "minimax-h3", true
	case "grok-imagine-video-1-5", "grok-imagine-video-1.5":
		return "grok-imagine-video-1-5", true
	default:
		return "", false
	}
}

func isPhaseOneVideoModel(name string) bool {
	_, ok := canonicalPhaseOneVideoModel(name)
	return ok
}

type roleCounts struct {
	first     int
	last      int
	reference int
}

type phaseOneMedia struct {
	images  []string
	videos  []string
	audios  []string
	roles   roleCounts
	hasFile bool
	hasLink bool
}

func validatePhaseOneVideoRequest(modelName string, raw map[string]any) (map[string]any, error) {
	modelID, ok := canonicalPhaseOneVideoModel(modelName)
	if !ok {
		return raw, nil
	}
	normalized, _, err := normalizePhaseOneVideoRequest(modelID, raw)
	return normalized, err
}

func normalizePhaseOneVideoRequest(modelID string, raw map[string]any) (map[string]any, phaseOneMedia, error) {
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	// The gateway's generic video body carries reference images as `images`,
	// while APIMart names the same field `image_urls`. Convert the alias before
	// validation so a native task adaptor never rejects its own gateway body.
	if value, exists := out["images"]; exists && value != nil {
		if _, hasCanonical := out["image_urls"]; hasCanonical {
			return nil, phaseOneMedia{}, fmt.Errorf("images and image_urls cannot both be set for %s", modelID)
		}
		out["image_urls"] = value
		delete(out, "images")
	}
	if err := normalizePhaseOneGatewayFields(modelID, out); err != nil {
		return nil, phaseOneMedia{}, err
	}
	for _, key := range []string{"image", "image_tail", "reference_images", "input_reference"} {
		if value, exists := out[key]; exists && value != nil {
			return nil, phaseOneMedia{}, fmt.Errorf("%s is not an APIMart field for %s", key, modelID)
		}
	}

	media, err := normalizePhaseOneMedia(modelID, out)
	if err != nil {
		return nil, phaseOneMedia{}, err
	}
	operation := strings.TrimSpace(asString(out["operation"]))
	if operation == "" {
		operation = inferPhaseOneOperation(modelID, out, media)
	}
	if !operationAllowed(modelID, operation) {
		return nil, phaseOneMedia{}, fmt.Errorf("operation %q is not supported by %s", operation, modelID)
	}
	out["operation"] = operation

	if err := validatePhaseOnePrompt(modelID, operation, out); err != nil {
		return nil, phaseOneMedia{}, err
	}
	if err := validatePhaseOneParameters(modelID, operation, out); err != nil {
		return nil, phaseOneMedia{}, err
	}
	if err := validatePhaseOneOperationMedia(modelID, operation, media, out); err != nil {
		return nil, phaseOneMedia{}, err
	}
	return out, media, nil
}

// normalizeManagedVideoRequest keeps the operation used for routing and the
// vendor task type used at submission in agreement.
func normalizeManagedVideoRequest(requestModel string, raw map[string]any) (map[string]any, phaseOneMedia, error) {
	modelID, _ := canonicalPhaseOneVideoModel(requestModel)
	if modelID != "seedance-2-5" {
		return nil, phaseOneMedia{}, fmt.Errorf("no verified request decoder for this video profile")
	}
	draft := raw["draft"]
	draftTaskID := raw["draft_task_id"]
	if (draft != nil && draft != false) || (draftTaskID != nil && draftTaskID != "") {
		return nil, phaseOneMedia{}, fmt.Errorf("draft and draft_task_id are unavailable for compatible routing")
	}
	normalized, media, err := normalizePhaseOneVideoRequest(modelID, raw)
	if err != nil {
		return nil, phaseOneMedia{}, err
	}

	operation := asString(normalized["operation"])
	taskType := strings.TrimSpace(asString(normalized["omni_reference_task_type"]))
	requiredTaskType := seedanceTaskType(operation)
	if requiredTaskType == "" && taskType != "" && taskType != "auto" {
		return nil, phaseOneMedia{}, fmt.Errorf("omni_reference_task_type %q conflicts with %s", taskType, operation)
	}
	if requiredTaskType != "" && taskType == "" && (operation != opVideoMultiRef || len(media.videos) > 0) {
		// BuildRequestBody strips operation. Only an explicit operation can
		// resolve ambiguous provider auto detection of video references.
		if strings.TrimSpace(asString(raw["operation"])) == "" {
			return nil, phaseOneMedia{}, fmt.Errorf("set operation to video_multi_ref, video_edit, or video_extend when supplying a reference video")
		}
		normalized["omni_reference_task_type"] = requiredTaskType
	}

	audio, hasAudio := normalized["audio"].(bool)
	generateAudio, hasGenerateAudio := normalized["generate_audio"].(bool)
	if hasGenerateAudio && hasAudio && generateAudio != audio {
		return nil, phaseOneMedia{}, fmt.Errorf("generate_audio and audio must agree for compatible routing")
	}
	return normalized, media, nil
}

// VideoRoutingRequest extracts intent using the registered native decoder
// shared by channel selection and submission. Omitted parameters use each
// channel's verified defaults.
func VideoRoutingRequest(requestModel string, raw map[string]any) (modelcontracts.VideoRequest, error) {
	normalized, media, err := normalizeManagedVideoRequest(requestModel, raw)
	if err != nil {
		return modelcontracts.VideoRequest{}, err
	}
	operation := asString(normalized["operation"])
	videoSlot := "reference_videos"
	if operation == opVideoEdit || operation == opVideoExtend {
		videoSlot = "source_videos"
	}
	request := modelcontracts.VideoRequest{
		Operation: operation,
		Slots: map[string]int{
			"prompt":           1,
			"first_frame":      media.roles.first,
			"last_frame":       media.roles.last,
			"reference_images": len(media.images) + media.roles.reference,
			videoSlot:          len(media.videos),
			"reference_audios": len(media.audios),
		},
		Parameters: map[string]any{},
	}
	for slot, count := range request.Slots {
		if count == 0 {
			delete(request.Slots, slot)
		}
	}
	for _, name := range []string{"duration", "resolution", "output_format"} {
		if value, exists := normalized[name]; exists {
			request.Parameters[name] = value
		}
	}
	if size, exists := normalized["size"]; exists {
		request.Parameters["aspect_ratio"] = size
	}
	if value, exists := normalized["generate_audio"]; exists {
		request.Parameters["generate_audio"] = value
	} else if value, exists := normalized["audio"]; exists {
		request.Parameters["generate_audio"] = value
	}
	return request, nil
}

// normalizePhaseOneGatewayFields translates the shared gateway task request
// names into the APIMart phase-one contract. `seconds`/`aspect_ratio` are the
// generic video submit fields used across the gateway; APIMart expects
// `duration`/`size` for the seedance and wan families.
func normalizePhaseOneGatewayFields(modelID string, out map[string]any) error {
	if _, exists := out["duration"]; !exists {
		if value, exists := out["seconds"]; exists && value != nil {
			if seconds, err := anyToInt(value); err == nil {
				out["duration"] = seconds
			} else {
				out["duration"] = value
			}
		}
	}
	delete(out, "seconds")
	switch modelID {
	case "seedance-2-5-flash", "magi-2", "wan-3.0":
		resVal, hasRes, err := optionalString(out, "resolution")
		if err != nil {
			return err
		}
		qualVal, hasQual, err := optionalString(out, "quality")
		if err != nil {
			return err
		}
		if hasRes && hasQual && !strings.EqualFold(resVal, qualVal) {
			return fmt.Errorf("resolution and quality must match when both are provided")
		}
		if !hasRes && hasQual {
			out["resolution"] = qualVal
		}
		delete(out, "quality")
	}
	switch modelID {
	case "seedance-2-0", "seedance-2-0-fast", "seedance-2-0-mini", "seedance-2-5", "seedance-2-5-flash", "wan-3.0":
		if modelID == "seedance-2-5-flash" {
			if err := normalizeSeedanceFlashPixelSize(out); err != nil {
				return err
			}
		}
		if _, exists := out["size"]; !exists {
			if value, exists := out["aspect_ratio"]; exists && value != nil {
				out["size"] = value
			}
		}
		delete(out, "aspect_ratio")
	case "magi-2":
		if _, exists := out["aspect_ratio"]; !exists {
			if value, exists := out["size"]; exists && value != nil {
				out["aspect_ratio"] = value
			}
		}
		delete(out, "size")
	}
	return nil
}

// normalizeSeedanceFlashPixelSize translates OpenAI-style WxH pixel sizes
// (e.g. "854x480", "1280x720") into resolution + aspect ratio for
// seedance-2-5-flash so tier-group example sizes work end-to-end.
func normalizeSeedanceFlashPixelSize(out map[string]any) error {
	rawSize, hasSize, err := optionalString(out, "size")
	if err != nil || !hasSize {
		return err
	}
	normalized := strings.NewReplacer("×", "x", "*", "x", "X", "x").Replace(strings.ToLower(rawSize))
	normalized = strings.ReplaceAll(normalized, " ", "")
	wStr, hStr, found := strings.Cut(normalized, "x")
	if !found {
		return nil
	}
	w, wErr := anyToInt(wStr)
	h, hErr := anyToInt(hStr)
	if wErr != nil || hErr != nil || w <= 0 || h <= 0 {
		return nil
	}
	var tierRes string
	switch min(w, h) {
	case 480:
		tierRes = "480p"
	case 720:
		tierRes = "720p"
	default:
		return nil
	}
	if existingRes, hasRes, err := optionalString(out, "resolution"); err != nil {
		return err
	} else if hasRes && !strings.EqualFold(existingRes, tierRes) {
		return fmt.Errorf("resolution %q conflicts with size %q", existingRes, rawSize)
	} else if !hasRes {
		out["resolution"] = tierRes
	}
	if _, hasAspect := out["aspect_ratio"]; hasAspect && out["aspect_ratio"] != nil {
		delete(out, "size")
		return nil
	}
	switch {
	case w > h:
		out["size"] = "16:9"
	case h > w:
		out["size"] = "9:16"
	default:
		out["size"] = "1:1"
	}
	return nil
}

func operationAllowed(modelID, operation string) bool {
	allowed := map[string]map[string]bool{
		"seedance-2-0": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
		},
		"seedance-2-0-fast": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
		},
		"seedance-2-0-mini": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
		},
		"seedance-2-5": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
			opVideoEdit: true, opVideoExtend: true,
		},
		"seedance-2-5-flash": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
			opVideoEdit: true, opVideoExtend: true,
		},
		"magi-2": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
		},
		"wan-3.0": {
			opTextToVideo: true, opFirstFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
			opDocumentToVideo: true, opWebpageToVideo: true,
		},
		"minimax-h3": {
			opTextToVideo: true, opFirstFrame: true, opEndFrame: true, opFirstLastFrame: true, opVideoMultiRef: true,
		},
		"grok-imagine-video-1-5": {
			opTextToVideo: true, opVideoMultiRef: true,
		},
	}
	return allowed[modelID][operation]
}

func inferPhaseOneOperation(modelID string, out map[string]any, media phaseOneMedia) string {
	if media.hasFile {
		return opDocumentToVideo
	}
	if media.hasLink {
		return opWebpageToVideo
	}
	taskType := strings.ToLower(strings.TrimSpace(asString(out["omni_reference_task_type"])))
	if taskType == "edit" {
		return opVideoEdit
	}
	if taskType == "extend" {
		return opVideoExtend
	}
	if media.roles.first > 0 && media.roles.last > 0 {
		return opFirstLastFrame
	}
	if media.roles.first > 0 {
		return opFirstFrame
	}
	if media.roles.last > 0 && modelID == "minimax-h3" {
		return opEndFrame
	}
	if len(media.images)+len(media.videos)+len(media.audios)+media.roles.reference > 0 {
		return opVideoMultiRef
	}
	return opTextToVideo
}

func normalizePhaseOneMedia(modelID string, out map[string]any) (phaseOneMedia, error) {
	var media phaseOneMedia
	var err error
	if media.images, err = normalizeStringArray(out, "image_urls"); err != nil {
		return media, err
	}
	if media.videos, err = normalizeStringArray(out, "video_urls"); err != nil {
		return media, err
	}
	if media.audios, err = normalizeStringArray(out, "audio_urls"); err != nil {
		return media, err
	}
	media.roles, err = normalizeRoleImages(out)
	if err != nil {
		return media, err
	}
	fileURL, fileExists, err := optionalString(out, "file_url")
	if err != nil {
		return media, err
	}
	linkURL, linkExists, err := optionalString(out, "link_url")
	if err != nil {
		return media, err
	}
	media.hasFile = fileExists && fileURL != ""
	media.hasLink = linkExists && linkURL != ""
	if media.hasFile && media.hasLink {
		return media, fmt.Errorf("file_url and link_url are mutually exclusive")
	}
	if media.hasFile && !isHTTPURL(fileURL, false) {
		return media, fmt.Errorf("file_url must be a public HTTP(S) URL")
	}
	if media.hasFile && !isAPIMartDocumentURL(fileURL) {
		return media, fmt.Errorf("file_url extension is not a supported APIMart document format")
	}
	if media.hasLink && !isHTTPURL(linkURL, false) {
		return media, fmt.Errorf("link_url must be a public HTTP(S) URL")
	}
	if modelID == "grok-imagine-video-1-5" {
		for _, item := range media.images {
			if !isHTTPURL(item, true) {
				return media, fmt.Errorf("grok image_urls entries must be public HTTPS URLs")
			}
		}
	}
	if modelID == "magi-2" {
		if media.hasFile || media.hasLink {
			return media, fmt.Errorf("file_url and link_url are not supported by magi-2")
		}
		for _, list := range []struct {
			name  string
			items []string
		}{
			{"image_urls", media.images},
			{"video_urls", media.videos},
			{"audio_urls", media.audios},
		} {
			for _, item := range list.items {
				if !isHTTPURL(item, true) || len(item) > 8192 {
					return media, fmt.Errorf("magi-2 %s entries must be public HTTPS URLs of at most 8192 characters", list.name)
				}
			}
		}
		if rows, ok := out["image_with_roles"].([]any); ok {
			for _, row := range rows {
				if entry, ok := row.(map[string]any); ok {
					item := asString(entry["url"])
					if !isHTTPURL(item, true) || len(item) > 8192 {
						return media, fmt.Errorf("magi-2 image_with_roles URLs must be public HTTPS URLs of at most 8192 characters")
					}
				}
			}
		}
	}
	return media, nil
}

func isAPIMartDocumentURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	allowed := map[string]bool{
		".docx": true, ".doc": true, ".xlsx": true, ".xls": true,
		".pptx": true, ".ppt": true, ".pdf": true, ".txt": true,
		".key": true, ".pages": true, ".numbers": true, ".md": true,
	}
	return allowed[strings.ToLower(path.Ext(parsed.Path))]
}

func normalizeStringArray(out map[string]any, key string) ([]string, error) {
	value, exists := out[key]
	if !exists || value == nil {
		return nil, nil
	}
	var raw []any
	switch typed := value.(type) {
	case []any:
		raw = typed
	case []string:
		raw = make([]any, len(typed))
		for index, item := range typed {
			raw[index] = item
		}
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	if len(raw) == 0 {
		delete(out, key)
		return nil, nil
	}
	items := make([]string, 0, len(raw))
	for index, item := range raw {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string", key, index)
		}
		items = append(items, strings.TrimSpace(text))
	}
	out[key] = items
	return items, nil
}

func normalizeRoleImages(out map[string]any) (roleCounts, error) {
	var counts roleCounts
	value, exists := out["image_with_roles"]
	if !exists || value == nil {
		return counts, nil
	}
	items, ok := value.([]any)
	if !ok {
		return counts, fmt.Errorf("image_with_roles must be an array")
	}
	if len(items) == 0 {
		delete(out, "image_with_roles")
		return counts, nil
	}
	normalized := make([]any, 0, len(items))
	for index, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return counts, fmt.Errorf("image_with_roles[%d] must be an object", index)
		}
		imageURL := strings.TrimSpace(asString(row["url"]))
		if imageURL == "" {
			return counts, fmt.Errorf("image_with_roles[%d].url is required", index)
		}
		role := strings.ToLower(strings.TrimSpace(asString(row["role"])))
		switch role {
		case "first", "first_frame":
			role = "first_frame"
			counts.first++
		case "last", "last_frame":
			role = "last_frame"
			counts.last++
		case "reference", "reference_image":
			role = "reference_image"
			counts.reference++
		default:
			return counts, fmt.Errorf("image_with_roles[%d].role is unsupported", index)
		}
		normalized = append(normalized, map[string]any{"url": imageURL, "role": role})
	}
	out["image_with_roles"] = normalized
	return counts, nil
}

func validatePhaseOnePrompt(modelID, operation string, out map[string]any) error {
	prompt, exists, err := optionalString(out, "prompt")
	if err != nil {
		return err
	}
	required := modelID == "minimax-h3" ||
		modelID == "grok-imagine-video-1-5" ||
		modelID == "seedance-2-5" ||
		modelID == "seedance-2-5-flash" ||
		(modelID == "magi-2" && operation == opTextToVideo) ||
		(strings.HasPrefix(modelID, "seedance-2-0") && operation == opTextToVideo) ||
		(modelID == "wan-3.0" && operation == opTextToVideo)
	if required && (!exists || strings.TrimSpace(prompt) == "") {
		return fmt.Errorf("prompt is required for %s %s", modelID, operation)
	}
	length := utf8.RuneCountInString(prompt)
	switch modelID {
	case "seedance-2-0", "seedance-2-0-fast":
		if length > 4000 {
			return fmt.Errorf("prompt exceeds 4000 Unicode characters")
		}
	case "seedance-2-5", "seedance-2-5-flash":
		// Vendor does not publish a hard prompt character ceiling for Seedance 2.5.
	case "magi-2":
		if length > 15000 {
			return fmt.Errorf("prompt exceeds 15000 Unicode characters")
		}
	case "wan-3.0":
		if length > 20000 {
			return fmt.Errorf("prompt exceeds APIMart's 20000-character boundary")
		}
	case "minimax-h3":
		if length > 7000 {
			return fmt.Errorf("prompt exceeds 7000 Unicode characters")
		}
	case "grok-imagine-video-1-5":
		if length > 8000 {
			return fmt.Errorf("prompt exceeds 8000 Unicode characters")
		}
	}
	return nil
}

func validatePhaseOneParameters(modelID, operation string, out map[string]any) error {
	switch modelID {
	case "seedance-2-0", "seedance-2-0-fast", "seedance-2-0-mini":
		if err := validateDuration(out, 4, 15, false); err != nil {
			return err
		}
		resolutions := []string{"480p", "720p"}
		if modelID == "seedance-2-0" {
			resolutions = append(resolutions, "1080p", "4k")
		}
		if err := validateEnum(out, "resolution", resolutions, false); err != nil {
			return err
		}
		if err := validateEnum(out, "size", seedanceRatios(), false); err != nil {
			return err
		}
	case "seedance-2-5":
		if err := validateDuration(out, 4, 30, true); err != nil {
			return err
		}
		if err := validateEnum(out, "resolution", []string{"480p", "720p", "1080p"}, false); err != nil {
			return err
		}
		if err := validateEnum(out, "size", seedanceRatios(), false); err != nil {
			return err
		}
		if err := validateEnum(out, "output_format", []string{"mp4", "mov"}, false); err != nil {
			return err
		}
		if err := validateEnum(out, "omni_reference_task_type", []string{"auto", "reference", "edit", "extend"}, false); err != nil {
			return err
		}
	case "seedance-2-5-flash":
		if draft, exists := out["draft"]; exists && draft != nil && draft != false {
			return fmt.Errorf("draft is not supported by %s", modelID)
		}
		if draftID, exists := out["draft_task_id"]; exists && draftID != nil && strings.TrimSpace(asString(draftID)) != "" {
			return fmt.Errorf("draft_task_id is not supported by %s", modelID)
		}
		if err := validateDuration(out, 4, 30, true); err != nil {
			return err
		}
		if err := validateEnum(out, "resolution", []string{"480p", "720p"}, true); err != nil {
			return err
		}
		if err := validateEnum(out, "size", seedanceRatios(), false); err != nil {
			return err
		}
		if (operation == opFirstFrame || operation == opFirstLastFrame) && asString(out["size"]) != "" && asString(out["size"]) != "adaptive" {
			return fmt.Errorf("Seedance 2.5 Flash %s requires size adaptive", operation)
		}
		if err := validateEnum(out, "output_format", []string{"mp4", "mov"}, false); err != nil {
			return err
		}
		if err := validateEnum(out, "omni_reference_task_type", []string{"auto", "reference", "edit", "extend"}, false); err != nil {
			return err
		}
	case "magi-2":
		for _, unsupported := range []string{
			"output_format", "watermark", "return_last_frame",
			"omni_reference_task_type", "generation_type", "tools",
			"draft", "draft_task_id", "file_url", "link_url",
		} {
			if value, exists := out[unsupported]; exists && value != nil {
				return fmt.Errorf("%s is not supported by magi-2", unsupported)
			}
		}
		if err := validateDuration(out, 4, 30, false); err != nil {
			return err
		}
		if err := validateEnum(out, "resolution", []string{"1080p"}, true); err != nil {
			return err
		}
		if err := validateEnum(out, "aspect_ratio", []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, false); err != nil {
			return err
		}
		callbackURL, hasCallback, err := optionalString(out, "callback_url")
		if err != nil {
			return err
		}
		if hasCallback && (!isHTTPURL(callbackURL, true) || len(callbackURL) > 2048) {
			return fmt.Errorf("callback_url must be a public HTTPS URL of at most 2048 characters")
		}
	case "wan-3.0":
		if err := validateDuration(out, 2, 30, true); err != nil {
			return err
		}
		if err := validateEnum(out, "resolution", []string{"480P", "720P", "1080P"}, true); err != nil {
			return err
		}
		if err := validateEnum(out, "size", []string{"adaptive", "16:9", "4:3", "1:1", "3:4", "9:16"}, false); err != nil {
			return err
		}
		if err := validateEnum(out, "generation_type", []string{"frame", "reference"}, false); err != nil {
			return err
		}
	case "minimax-h3":
		if err := validateDuration(out, 4, 15, false); err != nil {
			return err
		}
		if err := validateEnum(out, "resolution", []string{"2K", "768P"}, false); err != nil {
			return err
		}
		ratios := []string{"adaptive", "21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}
		if err := validateEnum(out, "aspect_ratio", ratios, false); err != nil {
			return err
		}
	case "grok-imagine-video-1-5":
		if err := validateDuration(out, 1, 15, false); err != nil {
			return err
		}
		if err := validateEnum(out, "resolution", []string{"480p", "720p", "1080p"}, false); err != nil {
			return err
		}
		if err := validateEnum(out, "aspect_ratio", []string{"auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}, false); err != nil {
			return err
		}
	}
	for _, field := range []string{"generate_audio", "audio", "watermark", "return_last_frame", "nsfw_check"} {
		if err := validateOptionalBool(out, field); err != nil {
			return err
		}
	}
	if modelID == "magi-2" {
		audio, hasAudio := out["audio"].(bool)
		generateAudio, hasGenerateAudio := out["generate_audio"].(bool)
		if hasAudio && hasGenerateAudio && audio != generateAudio {
			return fmt.Errorf("generate_audio and audio must agree for magi-2")
		}
	}
	if value, exists := out["seed"]; exists && value != nil {
		seed, err := strictInt64(value)
		if err != nil {
			return fmt.Errorf("seed must be an integer")
		}
		if modelID == "wan-3.0" && (seed < 0 || seed > 2147483647) {
			return fmt.Errorf("seed must be between 0 and 2147483647")
		}
		if modelID == "magi-2" && (seed < 0 || seed > 4294967295) {
			return fmt.Errorf("seed must be between 0 and 4294967295")
		}
		out["seed"] = seed
	}
	if tools, exists := out["tools"]; exists && tools != nil {
		rows, ok := tools.([]any)
		if !ok {
			return fmt.Errorf("tools must be an array")
		}
		for _, item := range rows {
			row, ok := item.(map[string]any)
			if !ok || asString(row["type"]) != "web_search" {
				return fmt.Errorf("tools only supports type web_search")
			}
		}
	}
	if operation == opVideoEdit {
		if asString(out["size"]) != "" && asString(out["size"]) != "adaptive" {
			return fmt.Errorf("Seedance 2.5 video_edit requires size adaptive")
		}
		if value, exists := out["duration"]; exists && value != nil {
			duration, _ := strictInteger(value)
			if duration != -1 {
				return fmt.Errorf("Seedance 2.5 video_edit requires duration -1")
			}
		}
	}
	if operation == opVideoExtend && asString(out["size"]) != "" && asString(out["size"]) != "adaptive" {
		return fmt.Errorf("Seedance 2.5 video_extend requires size adaptive")
	}
	return nil
}

func validatePhaseOneOperationMedia(modelID, operation string, media phaseOneMedia, out map[string]any) error {
	frameCount := media.roles.first + media.roles.last
	referenceImageCount := len(media.images) + media.roles.reference
	referenceCount := referenceImageCount + len(media.videos) + len(media.audios)
	if frameCount > 0 && referenceCount > 0 {
		return fmt.Errorf("frame and reference media families are mutually exclusive")
	}
	if media.hasFile || media.hasLink {
		if frameCount+referenceCount > 0 {
			return fmt.Errorf("file/page references cannot be combined with media arrays in this contract")
		}
	}
	switch operation {
	case opTextToVideo:
		if frameCount+referenceCount > 0 || media.hasFile || media.hasLink {
			return fmt.Errorf("text_to_video does not accept reference media")
		}
	case opFirstFrame:
		if media.roles.first != 1 || media.roles.last != 0 || media.roles.reference != 0 || referenceCount != 0 {
			return fmt.Errorf("first_frame requires exactly one first_frame image")
		}
	case opEndFrame:
		if media.roles.last != 1 || media.roles.first != 0 || media.roles.reference != 0 || referenceCount != 0 {
			return fmt.Errorf("end_frame requires exactly one last_frame image")
		}
	case opFirstLastFrame:
		if media.roles.first != 1 || media.roles.last != 1 || media.roles.reference != 0 || referenceCount != 0 {
			return fmt.Errorf("first_last_frame requires exactly one first_frame and one last_frame image")
		}
	case opVideoMultiRef:
		if frameCount > 0 || referenceCount == 0 {
			return fmt.Errorf("video_multi_ref requires at least one reference asset and no frame roles")
		}
	case opVideoEdit, opVideoExtend:
		if frameCount > 0 || len(media.videos) == 0 {
			return fmt.Errorf("%s requires at least one source video and no frame roles", operation)
		}
	case opDocumentToVideo:
		if !media.hasFile || media.hasLink {
			return fmt.Errorf("document_to_video requires file_url only")
		}
	case opWebpageToVideo:
		if !media.hasLink || media.hasFile {
			return fmt.Errorf("webpage_to_video requires link_url only")
		}
	}

	switch modelID {
	case "seedance-2-0", "seedance-2-0-fast", "seedance-2-0-mini", "minimax-h3":
		if referenceImageCount > 9 || len(media.videos) > 3 || len(media.audios) > 3 {
			return fmt.Errorf("reference media count exceeds the documented 9 image / 3 video / 3 audio limits")
		}
		if operation == opVideoMultiRef && len(media.audios) > 0 && referenceImageCount+len(media.videos) == 0 {
			return fmt.Errorf("reference audio cannot be used alone for %s", modelID)
		}
	case "seedance-2-5":
		if referenceImageCount > 30 || len(media.videos) > 10 || len(media.audios) > 10 {
			return fmt.Errorf("reference media count exceeds the documented 30 image / 10 video / 10 audio limits")
		}
	case "seedance-2-5-flash":
		if referenceImageCount > 10 || len(media.videos) > 5 || len(media.audios) > 5 {
			return fmt.Errorf("reference media count exceeds the documented 10 image / 5 video / 5 audio limits")
		}
	case "magi-2":
		if referenceImageCount > 10 || len(media.videos) > 5 || len(media.audios) > 5 {
			return fmt.Errorf("MAGI-2 accepts at most 10 reference images, 5 reference videos, and 5 reference audios")
		}
	case "wan-3.0":
		if referenceImageCount > 10 || len(media.videos) > 5 || len(media.audios) > 5 {
			return fmt.Errorf("Wan 3.0 accepts at most 10 reference images, 5 reference videos, and 5 reference audios")
		}
	case "grok-imagine-video-1-5":
		// APIMart publishes no image_urls count ceiling for Grok 1.5.
		if len(media.videos) > 0 || len(media.audios) > 0 || frameCount > 0 {
			return fmt.Errorf("Grok 1.5 generation only accepts image_urls references")
		}
	}
	if modelID == "seedance-2-5" || modelID == "seedance-2-5-flash" {
		taskType := strings.TrimSpace(asString(out["omni_reference_task_type"]))
		expected := seedanceTaskType(operation)
		if expected != "" && taskType != "" && taskType != expected {
			return fmt.Errorf("%s requires omni_reference_task_type %s", operation, expected)
		}
	}
	if modelID == "wan-3.0" {
		generationType := strings.TrimSpace(asString(out["generation_type"]))
		expected := ""
		if operation == opFirstFrame || operation == opFirstLastFrame {
			expected = "frame"
		} else if operation == opVideoMultiRef {
			expected = "reference"
		}
		if expected != "" && generationType != "" && generationType != expected {
			return fmt.Errorf("%s requires generation_type %s", operation, expected)
		}
	}
	return nil
}

func seedanceTaskType(operation string) string {
	switch operation {
	case opVideoMultiRef:
		return "reference"
	case opVideoEdit:
		return "edit"
	case opVideoExtend:
		return "extend"
	default:
		return ""
	}
}

func seedanceRatios() []string {
	return []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}
}

// omniFlashExtModels 是 APIMart 的 Omni-Flash-Ext —— 公开 ID gemini-omni-1.1-flash-task 的上游。
// fork: APIMart 按「分辨率 × 时长」分档收费，下游按次价只覆盖它的 ≤1080P 档
// （360P/720P/1080P × 4/6/8/10 秒，最贵 $0.40/次）；4K 档上游 $0.75–0.90/次高于售价，超档即亏。
// 见 docs/ops/model-tier-pricing-decisions-zh.md §十三。
var omniFlashExtModels = map[string]struct{}{
	"omni-flash-ext":             {},
	"gemini-omni-1.1-flash-ext":  {},
	"gemini-omni-1.1-flash-task": {},
}

var omniFlashExtResolutions = map[string]struct{}{
	"360p":  {},
	"720p":  {},
	"1080p": {},
}

const (
	omniFlashExtMinSeconds = 4
	omniFlashExtMaxSeconds = 10
)

func isOmniFlashExtModel(name string) bool {
	_, ok := omniFlashExtModels[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// validateOmniFlashExtCaps 把请求参数限制在已定价的档位内，超档直接拒绝（上游因此不产生费用）。
// 只做范围校验，不改写调用方传来的值 —— 上游对 resolution 的大小写口径未经证实。
func validateOmniFlashExtCaps(out map[string]any) error {
	for _, key := range []string{"resolution", "quality"} {
		value, exists, err := optionalString(out, key)
		if err != nil {
			return err
		}
		if !exists || value == "" {
			continue
		}
		if _, ok := omniFlashExtResolutions[strings.ToLower(strings.TrimSpace(value))]; !ok {
			return fmt.Errorf("%s %q is outside this model's priced tiers (360p, 720p, 1080p)", key, value)
		}
	}
	if value, exists := out["4k"]; exists && truthy(value) {
		return fmt.Errorf("4k is outside this model's priced tiers (360p, 720p, 1080p)")
	}
	// 未传时长时交给上游默认档（默认档 $0.35/次，在覆盖区间内）；-1（自动时长）无法预先判断档位，拒绝。
	seconds, err := parseDurationSeconds(out)
	if err != nil {
		return err
	}
	if seconds == -1 || seconds > omniFlashExtMaxSeconds || seconds != 0 && seconds < omniFlashExtMinSeconds {
		return fmt.Errorf("duration must be an integer from %d to %d seconds for this model", omniFlashExtMinSeconds, omniFlashExtMaxSeconds)
	}
	return nil
}

func validateDuration(out map[string]any, min, max int, allowAuto bool) error {
	value, exists := out["duration"]
	if !exists || value == nil {
		return nil
	}
	duration, err := strictInteger(value)
	if err != nil || (!allowAuto || duration != -1) && (duration < min || duration > max) {
		if allowAuto {
			return fmt.Errorf("duration must be -1 or an integer from %d to %d", min, max)
		}
		return fmt.Errorf("duration must be an integer from %d to %d", min, max)
	}
	out["duration"] = duration
	return nil
}

func validateEnum(out map[string]any, key string, allowed []string, caseInsensitive bool) error {
	value, exists, err := optionalString(out, key)
	if err != nil || !exists || value == "" {
		return err
	}
	for _, candidate := range allowed {
		if value == candidate || caseInsensitive && strings.EqualFold(value, candidate) {
			out[key] = candidate
			return nil
		}
	}
	return fmt.Errorf("%s does not accept %q", key, value)
}

func validateOptionalBool(out map[string]any, key string) error {
	value, exists := out[key]
	if !exists || value == nil {
		return nil
	}
	if _, ok := value.(bool); !ok {
		return fmt.Errorf("%s must be boolean", key)
	}
	return nil
}

func strictInteger(value any) (int, error) {
	n, err := strictInt64(value)
	if err != nil {
		return 0, err
	}
	if n < math.MinInt || n > math.MaxInt {
		return 0, fmt.Errorf("integer out of range")
	}
	return int(n), nil
}

func strictInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed {
			return 0, fmt.Errorf("not an integer")
		}
		if typed < -9007199254740991 || typed > 9007199254740991 {
			return 0, fmt.Errorf("integer out of range")
		}
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	default:
		return 0, fmt.Errorf("unsupported integer type %T", value)
	}
}

func optionalString(out map[string]any, key string) (string, bool, error) {
	value, exists := out[key]
	if !exists || value == nil {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", true, fmt.Errorf("%s must be a string", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		delete(out, key)
		return "", false, nil
	}
	out[key] = text
	return text, true, nil
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

func isHTTPURL(value string, httpsOnly bool) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	if httpsOnly {
		return strings.EqualFold(parsed.Scheme, "https")
	}
	return strings.EqualFold(parsed.Scheme, "https") || strings.EqualFold(parsed.Scheme, "http")
}
