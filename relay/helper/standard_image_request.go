package helper

import (
	"fmt"
	"math"
	"mime"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
)

var standardImageReferenceHTTPClient = service.NewInfluencerReferenceHTTPClient()

// IsGPTImageStandardRoute scopes the reviewed quote contract to the original
// public model and the concrete native group selected for this attempt. The
// caller additionally checks the final supplier model before enforcing it.
func IsGPTImageStandardRoute(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	origin := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if origin == "" {
		origin = info.OriginModelName
	}
	if origin != "gpt-image-2.5" {
		return false
	}
	group := info.UsingGroup
	if selected := common.GetContextKeyString(c, constant.ContextKeyAutoGroup); selected != "" {
		group = selected
	}
	return group == "gpt-image-2.5-economy"
}

// GPTImageStandardPixels preserves the seven reviewed output specifications.
// The 3:4 pair uses supplier-valid multiples of sixteen rather than the older
// GoEasy 1365-pixel edge; explicit old pixels are not silently changed.
func GPTImageStandardPixels(size string) (string, bool) {
	switch strings.TrimSpace(size) {
	case "", "1:1", "1024x1024":
		return "1024x1024", true
	case "2:3", "1024x1536":
		return "1024x1536", true
	case "3:2", "1536x1024":
		return "1536x1024", true
	case "3:4", "1024x1360":
		return "1024x1360", true
	case "4:3", "1360x1024":
		return "1360x1024", true
	case "9:16", "1088x1920":
		return "1088x1920", true
	case "16:9", "1920x1088":
		return "1920x1088", true
	default:
		return "", false
	}
}

// ValidateGPTImageStandardRequest runs before provider/plugin preparation and
// native reservation. It rejects unreviewed options instead of overwriting them.
func ValidateGPTImageStandardRequest(c *gin.Context, info *relaycommon.RelayInfo) error {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("standard image group requires application/json; multipart uploads are not supported")
	}
	if len(info.ParamOverride) > 0 || info.ChannelOtherSettings.AdvancedCustom != nil && len(info.ChannelOtherSettings.AdvancedCustom.ModelByParam) > 0 {
		return fmt.Errorf("standard image group does not support channel parameter overrides")
	}
	var body map[string]any
	if err := common.UnmarshalBodyReusable(c, &body); err != nil || body == nil {
		return fmt.Errorf("standard image group requires a JSON object")
	}
	for key := range body {
		switch key {
		case "model", "group", "prompt", "size", "aspect_ratio", "n", "quality", "image", "images", "image_urls", "input_reference", "metadata", "seed", "output_format":
		default:
			return fmt.Errorf("standard image group does not support parameter %s", key)
		}
	}
	if group, exists := body["group"]; exists {
		if _, ok := group.(string); !ok {
			return fmt.Errorf("group must contain a native routing group name")
		}
	}
	prompt, ok := body["prompt"].(string)
	if !ok {
		return fmt.Errorf("prompt must contain at most 8192 Unicode characters and 16384 UTF-8 bytes")
	}
	if err := ValidateGPTImagePrompt("gpt-image-2.5", prompt); err != nil {
		return err
	}
	if value, exists := body["n"]; exists && value != float64(1) {
		return fmt.Errorf("standard image group supports exactly one output image (n=1)")
	}
	if value, exists := body["quality"]; exists && value != "medium" {
		return fmt.Errorf("standard image group supports omitted quality or quality=medium")
	}
	var pixels string
	for _, key := range []string{"size", "aspect_ratio"} {
		if value, exists := body[key]; exists {
			size, ok := value.(string)
			converted, supported := GPTImageStandardPixels(size)
			if !ok || !supported || pixels != "" && pixels != converted {
				return fmt.Errorf("standard image group supports the seven reviewed image sizes and matching aspect ratios")
			}
			pixels = converted
		}
	}
	if value, exists := body["output_format"]; exists && value != "png" {
		return fmt.Errorf("standard image group supports omitted output_format or output_format=png")
	}
	if value, exists := body["seed"]; exists && !standardImageSeed(value) {
		return fmt.Errorf("seed must be a nonnegative integer")
	}
	if value, exists := body["metadata"]; exists {
		metadata, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("standard image metadata supports only a seed object")
		}
		for key, value := range metadata {
			if key != "seed" || !standardImageSeed(value) {
				return fmt.Errorf("standard image metadata supports only a nonnegative integer seed")
			}
		}
	}
	// Count every raw alias, including repeated references in separate fields.
	// This is cardinality validation; supplier preparation owns normalization.
	references := make([]string, 0, 1)
	for _, key := range []string{"image", "images", "image_urls", "input_reference"} {
		value, exists := body[key]
		if !exists {
			continue
		}
		values, array := value.([]any)
		if !array {
			values = []any{value}
		}
		for _, value := range values {
			switch reference := value.(type) {
			case string:
				if strings.TrimSpace(reference) == "" {
					return fmt.Errorf("image reference must contain a public HTTPS URL")
				}
				references = append(references, strings.TrimSpace(reference))
			case map[string]any:
				if len(reference) != 1 {
					return fmt.Errorf("image reference must contain one URL")
				}
				for key, value := range reference {
					raw, ok := value.(string)
					if (key != "url" && key != "image_url") || !ok || strings.TrimSpace(raw) == "" {
						return fmt.Errorf("image reference must contain one URL")
					}
					references = append(references, strings.TrimSpace(raw))
				}
			default:
				return fmt.Errorf("image reference must contain a public HTTPS URL")
			}
		}
	}
	if len(references) > 1 {
		return fmt.Errorf("standard image group supports at most one reference across all image fields")
	}
	if len(references) == 1 {
		raw := references[0]
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return fmt.Errorf("image reference must contain a public HTTPS URL")
		}
		// Workbench dispatch already ran the same protected validator freshly.
		// Its durable URL marker alone is never used before that dispatch check.
		if c.GetString("influencer_reference_preflight") != raw && c.GetString("standard_image_reference_preflight") != raw {
			if err := service.ValidateInfluencerReference(c.Request.Context(), raw, standardImageReferenceHTTPClient); err != nil {
				return err
			}
			c.Set("standard_image_reference_preflight", raw)
		}
	}
	return nil
}

// ValidateGPTImagePrompt is the existing service envelope shared by image
// contracts. It does not truncate text or guarantee supplier token costs.
func ValidateGPTImagePrompt(modelName, prompt string) error {
	if strings.TrimSpace(prompt) == "" || !utf8.ValidString(prompt) || utf8.RuneCountInString(prompt) > 8192 || len(prompt) > 16384 {
		return fmt.Errorf("prompt must contain at most 8192 Unicode characters and 16384 UTF-8 bytes")
	}
	if tokenkit.Count(modelName, prompt) > 5000 {
		return fmt.Errorf("standard image group supports at most 5000 prompt tokens counted by the service")
	}
	return nil
}

func standardImageSeed(value any) bool {
	seed, ok := value.(float64)
	return ok && seed >= 0 && seed < float64(math.MaxInt64) && seed == math.Trunc(seed)
}
