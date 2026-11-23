package apimart

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
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

// wan-3.0 public ModelPrice anchors the official wan3.0-video standard 1080P
// list price (¥1.20/s). EvoLink exposes the same capability across 480P/720P/
// 1080P and bills each resolution at the official Alibaba ratio; the ratios
// below apply to EvoLink's upstream SKUs only. Pidoi's fixed-resolution SKUs
// (wan30-720p / wan30-1080p-fast) deliberately do not match these names and
// keep their existing group-ratio pricing.
// The Prime group reuses this table with a 1.5 group ratio: official Prime is
// exactly 1.5x the Standard list price at every resolution.
var wan30Rates = map[string]float64{
	"480p":  0.25,
	"720p":  0.5,
	"1080p": 1.0,
}

// EstimateVideoBilling exposes the adapter's own duration and resolution tier
// factors so a product surface can show the same amount the pre-charge will use
// instead of duplicating the rate tables. It does not mutate billing state.
func EstimateVideoBilling(modelName string, raw map[string]any) (map[string]float64, error) {
	return estimateVideoBilling(modelName, raw)
}

func estimateVideoBilling(modelName string, raw map[string]any) (map[string]float64, error) {
	return estimateVideoBillingWithRateModel(modelName, modelName, raw)
}

func estimateVideoBillingWithRateModel(modelName, rateModel string, raw map[string]any) (map[string]float64, error) {
	ratios, _, err := estimateVideoBillingWithPolicy(kitdto.VideoTierBillingPolicyPreset, modelName, rateModel, raw)
	return ratios, err
}

// estimateVideoBillingWithPolicy validates the channel tier billing policy and
// duration bounds, then optionally applies the built-in tier table for preset
// channels. Channels configured with video_tier_billing_policy="none" bill by
// duration only (tier=1) so group pricing never double-discounts.
func estimateVideoBillingWithPolicy(policy, modelName, rateModel string, raw map[string]any) (map[string]float64, bool, error) {
	effectivePolicy, err := (&kitdto.ChannelOtherSettings{VideoTierBillingPolicy: policy}).EffectiveVideoTierBillingPolicy()
	if err != nil {
		return nil, false, err
	}

	sec, err := parseDurationSeconds(raw)
	if err != nil {
		return nil, false, err
	}
	autoDuration := false
	if sec == -1 {
		canonical, ok := canonicalPhaseOneVideoModel(modelName)
		if !ok || (canonical != "seedance-2-5" && canonical != "wan-3.0") {
			return nil, false, fmt.Errorf("duration -1 is not supported by %s", modelName)
		}
		// Both official APIs pre-authorize their documented 30-second cap and
		// settle to the actual output duration when the task completes.
		sec = 30
		autoDuration = true
	} else if sec <= 0 {
		sec = defaultVideoSeconds
	}
	if sec > relaycommon.MaxTaskDurationSeconds {
		return nil, false, fmt.Errorf("duration must be -1 or between 1 and %d", relaycommon.MaxTaskDurationSeconds)
	}

	out := map[string]float64{"seconds": float64(sec)}
	if effectivePolicy == kitdto.VideoTierBillingPolicyNone {
		return out, autoDuration, nil
	}

	tier, err := resolveTierRatio(rateModel, raw)
	if err != nil {
		return nil, false, err
	}
	if tier != 1 {
		out["tier"] = tier
	}
	return out, autoDuration, nil
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
			if n < -1 || n > relaycommon.MaxTaskDurationSeconds {
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
		if t < math.MinInt || t > math.MaxInt {
			return 0, fmt.Errorf("integer out of range")
		}
		return int(t), nil
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) || t != math.Trunc(t) {
			return 0, fmt.Errorf("must be an integer")
		}
		if t < -1e15 || t > 1e15 {
			return 0, fmt.Errorf("integer out of range")
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

// parseBoundedPositiveSeconds parses a completion-reported duration in seconds
// and accepts only finite values in (0, relaycommon.MaxTaskDurationSeconds].
func parseBoundedPositiveSeconds(v any) (float64, bool) {
	var sec float64
	switch t := v.(type) {
	case float64:
		sec = t
	case float32:
		sec = float64(t)
	case int:
		sec = float64(t)
	case int64:
		sec = float64(t)
	case uint:
		sec = float64(t)
	case uint64:
		sec = float64(t)
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		sec = parsed
	default:
		return 0, false
	}
	if math.IsNaN(sec) || math.IsInf(sec, 0) || sec <= 0 || sec > float64(relaycommon.MaxTaskDurationSeconds) {
		return 0, false
	}
	return sec, true
}

func resolveTierRatio(modelName string, raw map[string]any) (float64, error) {
	m := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case strings.Contains(m, "seedance-2-5") || strings.Contains(m, "seedance-2.5"):
		return seedanceTierRatio(seedance25Rates, "default", raw, true)
	case strings.Contains(m, "seedance-2-0-fast") || strings.Contains(m, "seedance-2.0-fast"):
		return seedanceTierRatio(seedance20FastRates, "480p", raw, false)
	case strings.Contains(m, "seedance-2-0-mini") || strings.Contains(m, "seedance-2.0-mini"):
		return seedanceTierRatio(seedance20MiniRates, "480p", raw, false)
	case strings.Contains(m, "seedance-2-0") || strings.Contains(m, "seedance-2.0") || strings.Contains(m, "doubao-seedance-2.0"):
		return seedanceTierRatio(seedance20Rates, "480p", raw, false)
	case strings.Contains(m, "kling-v2-6") || strings.Contains(m, "kling-v2.6"):
		return klingTierRatio(klingV26Rates, "default", raw)
	case strings.Contains(m, "kling-v3"):
		return klingTierRatio(klingV3Rates, "default", raw)
	case strings.Contains(m, "wan3.0") || strings.Contains(m, "wan-3.0"):
		return wan30TierRatio(raw)
	default:
		// Unknown video model: charge duration only (tier=1). Safer than inventing rates.
		return 1, nil
	}
}

func wan30TierRatio(raw map[string]any) (float64, error) {
	resolution, explicit := parseResolution(raw)
	key := resolution
	if key == "" {
		// EvoLink defaults an omitted quality to 720p. Charging the 1080P base
		// here would overcharge every default request.
		key = "720p"
	}
	rate, ok := wan30Rates[key]
	if !ok {
		if explicit {
			return 0, fmt.Errorf("unsupported resolution %q for this model", resolution)
		}
		rate = wan30Rates["720p"]
	}
	return rate, nil
}

func seedanceTierRatio(rates map[string]float64, baseKey string, raw map[string]any, suggestGroupPricingOn1080p bool) (float64, error) {
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
			if suggestGroupPricingOn1080p && res == "1080p" {
				return 0, fmt.Errorf("unsupported resolution %q for preset tier billing on this model; configure group pricing with video_tier_billing_policy=%q", res, kitdto.VideoTierBillingPolicyNone)
			}
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
