package apimart

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// Video ModelPrice is USD per second for the model base tier (APIMart after_discount / 0.7).
// EstimateBilling multiplies by seconds and optional tier ratio (resolution / mode).
//
// Rate tables: APIMart pricing page after_discount USD/s (2026-08-09).

const defaultVideoSeconds = 5

// seedance-2-0 base = 480P cost 0.066 → sell ModelPrice = 0.066/0.7
// seedance-2-0-fast base = 480P cost 0.05312
// seedance-2-5 base = default 0.216
// kling-v2-6 base = default 0.0368
// kling-v3 base = default 0.0672

var seedance20Rates = map[string]float64{
	"480p":        0.066,
	"480p-input":  0.04,
	"720p":        0.142,
	"720p-input":  0.08584,
	"1080p":       0.3544,
	"1080p-input": 0.21568,
	"4k":          0.722,
	"4k-input":    0.44432,
}

var seedance20FastRates = map[string]float64{
	"480p":       0.05312,
	"480p-input": 0.0316,
	"720p":       0.11416,
	"720p-input": 0.0684,
}

var seedance20MiniRates = map[string]float64{
	"480p":       0.01056,
	"480p-input": 0.0064,
	"720p":       0.02288,
	"720p-input": 0.01384,
}

var seedance25Rates = map[string]float64{
	"default":    0.216,
	"480p":       0.09608,
	"480p-input": 0.0576,
	"720p":       0.216,
	"720p-input": 0.1296,
}

var klingV26Rates = map[string]float64{
	"default":         0.0368,
	"pro":             0.0625,
	"pro-sound":       0.125,
	"pro-sound-voice": 0.15,
}

var klingV3Rates = map[string]float64{
	"default":   0.0672,
	"pro":       0.0896,
	"sound":     0.1008,
	"pro-sound": 0.1344,
	"4k":        0.42856,
	"4k-sound":  0.42856,
}

// EstimateVideoBilling exposes the adapter's own duration and resolution tier
// factors so a product surface can show the same amount the pre-charge will use
// instead of duplicating the rate tables. It does not mutate billing state.
func EstimateVideoBilling(modelName string, raw map[string]any) (map[string]float64, error) {
	return estimateVideoBilling(modelName, raw)
}

func estimateVideoBilling(modelName string, raw map[string]any) (map[string]float64, error) {
	sec, err := parseDurationSeconds(raw)
	if err != nil {
		return nil, err
	}
	if sec == -1 {
		canonical, ok := canonicalPhaseOneVideoModel(modelName)
		if !ok || (canonical != "seedance-2-5" && canonical != "wan-3.0") {
			return nil, fmt.Errorf("duration -1 is not supported by %s", modelName)
		}
		// Both official APIs pre-authorize their documented 30-second cap and
		// settle to the actual output duration when the task completes.
		sec = 30
	} else if sec <= 0 {
		sec = defaultVideoSeconds
	}
	if sec > relaycommon.MaxTaskDurationSeconds {
		sec = relaycommon.MaxTaskDurationSeconds
	}

	tier, err := resolveTierRatio(modelName, raw)
	if err != nil {
		return nil, err
	}

	out := map[string]float64{"seconds": float64(sec)}
	if tier != 1 {
		out["tier"] = tier
	}
	return out, nil
}

func parseDurationSeconds(raw map[string]any) (int, error) {
	if raw == nil {
		return 0, nil
	}
	for _, key := range []string{"duration", "seconds"} {
		if v, ok := raw[key]; ok && v != nil {
			n, err := anyToInt(v)
			if err != nil {
				return 0, fmt.Errorf("%s must be a positive integer: %w", key, err)
			}
			if n < -1 {
				return 0, fmt.Errorf("%s must be -1 or between 1 and %d", key, relaycommon.MaxTaskDurationSeconds)
			}
			return n, nil
		}
	}
	return 0, nil
}

func anyToInt(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		if t != math.Trunc(t) {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(t), nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, nil
		}
		return strconv.Atoi(s)
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

func resolveTierRatio(modelName string, raw map[string]any) (float64, error) {
	m := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case strings.Contains(m, "seedance-2-5") || strings.Contains(m, "seedance-2.5"):
		return seedanceTierRatio(seedance25Rates, "default", raw)
	case strings.Contains(m, "seedance-2-0-fast") || strings.Contains(m, "seedance-2.0-fast"):
		return seedanceTierRatio(seedance20FastRates, "480p", raw)
	case strings.Contains(m, "seedance-2-0-mini") || strings.Contains(m, "seedance-2.0-mini"):
		return seedanceTierRatio(seedance20MiniRates, "480p", raw)
	case strings.Contains(m, "seedance-2-0") || strings.Contains(m, "seedance-2.0") || strings.Contains(m, "doubao-seedance-2.0"):
		return seedanceTierRatio(seedance20Rates, "480p", raw)
	case strings.Contains(m, "kling-v2-6") || strings.Contains(m, "kling-v2.6"):
		return klingTierRatio(klingV26Rates, "default", raw)
	case strings.Contains(m, "kling-v3"):
		return klingTierRatio(klingV3Rates, "default", raw)
	default:
		// Unknown video model: charge duration only (tier=1). Safer than inventing rates.
		return 1, nil
	}
}

func seedanceTierRatio(rates map[string]float64, baseKey string, raw map[string]any) (float64, error) {
	base, ok := rates[baseKey]
	if !ok || base <= 0 {
		return 1, nil
	}
	res, explicit := parseResolution(raw)
	hasInput := hasMediaInput(raw)
	key := res
	if key == "" {
		key = baseKey
	}
	if hasInput && !strings.HasSuffix(key, "-input") {
		// only apply -input when that tier exists
		if _, ok := rates[key+"-input"]; ok {
			key = key + "-input"
		}
	}
	rate, ok := rates[key]
	if !ok {
		if explicit {
			return 0, fmt.Errorf("unsupported resolution %q for this model", res)
		}
		rate = base
	}
	return rate / base, nil
}

func klingTierRatio(rates map[string]float64, baseKey string, raw map[string]any) (float64, error) {
	base, ok := rates[baseKey]
	if !ok || base <= 0 {
		return 1, nil
	}
	key := parseKlingMode(raw)
	rate, ok := rates[key]
	if !ok {
		// v2-6 has pro-sound but not plain "sound"
		if key == "sound" {
			if r, ok2 := rates["pro-sound"]; ok2 {
				rate, ok = r, true
			}
		}
	}
	if !ok {
		return 0, fmt.Errorf("unsupported kling mode/quality %q", key)
	}
	return rate / base, nil
}

func parseResolution(raw map[string]any) (res string, explicit bool) {
	if raw == nil {
		return "", false
	}
	for _, key := range []string{"resolution", "quality"} {
		if v, ok := raw[key]; ok && v != nil {
			s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
			if s == "" {
				continue
			}
			explicit = true
			s = strings.ReplaceAll(s, " ", "")
			switch {
			case strings.Contains(s, "4k") || s == "2160p" || s == "2160":
				return "4k", true
			case strings.Contains(s, "1080"):
				return "1080p", true
			case strings.Contains(s, "720"):
				return "720p", true
			case strings.Contains(s, "480"):
				return "480p", true
			case s == "default":
				return "default", true
			default:
				// unknown non-empty value
				return s, true
			}
		}
	}
	return "", false
}

func hasMediaInput(raw map[string]any) bool {
	if raw == nil {
		return false
	}
	for _, key := range []string{
		"image_urls", "image_with_roles", "video_urls", "audio_urls",
		"file_url", "link_url", "images", "image", "input_reference",
	} {
		if v, ok := raw[key]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return true
				}
			case []any:
				if len(t) > 0 {
					return true
				}
			case []string:
				if len(t) > 0 {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func parseKlingMode(raw map[string]any) string {
	if raw == nil {
		return "default"
	}
	// explicit mode string
	for _, key := range []string{"mode", "quality", "model_mode"} {
		if v, ok := raw[key]; ok && v != nil {
			s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
			s = strings.ReplaceAll(s, "_", "-")
			if s == "" || s == "std" || s == "standard" || s == "default" {
				return "default"
			}
			if s == "pro" || s == "professional" {
				// may combine with sound
				if truthy(raw["sound"]) || truthy(raw["with_audio"]) || truthy(raw["audio"]) {
					if truthy(raw["voice"]) {
						return "pro-sound-voice"
					}
					return "pro-sound"
				}
				return "pro"
			}
			if s == "pro-sound" || s == "pro-sound-voice" || s == "sound" || s == "4k" || s == "4k-sound" {
				return s
			}
		}
	}
	// flags only
	if truthy(raw["4k"]) || strings.Contains(strings.ToLower(fmt.Sprint(raw["resolution"])), "4k") {
		if truthy(raw["sound"]) || truthy(raw["with_audio"]) {
			return "4k-sound"
		}
		return "4k"
	}
	if truthy(raw["sound"]) || truthy(raw["with_audio"]) || truthy(raw["audio"]) {
		return "sound"
	}
	return "default"
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes" || s == "on"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}
