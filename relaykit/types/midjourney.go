package types

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

// Midjourney operation routing tables.
//
// Evolink exposes every Midjourney action as its own upstream model id
// (mj-v7, mj-v7-upscale, mj-v8.1-edit, ...) on the single endpoint
// POST /v1/images/generations. OmniMux used to mirror that 1:1 as public model
// ids, which turned one capability into 19 catalog entries. The public surface is
// now the version id alone (mj-v7 / mj-v8-1) and the action travels inside the
// request as model_params.operation; a request without that key is plain
// generation.
//
// Multipliers are relative to each version's own ModelPrice. Both versions anchor
// on Evolink's measured 0.54 CNY generation tier (2026-09-12 paid measurements),
// and every operation sits on one of two measured tiers:
//
//	CNY 0.54 -> 1.0x    generation, enhance, pan, outpaint, remove-bg
//	CNY 0.81 -> 1.5x    upscale, variation, edit, inpaint, remix, retexture, upload-paint
//
// Measured tiers that multiply on top of the operation tier:
//
//	quality hd (v8.1 only)   1.5x   Evolink documents the multiplier
//	speed draft / fast       1.0x   Evolink documents both as 1x
//	speed turbo              2.0x   measured 2026-09-12: pan turbo = 10.8 credits = CNY 1.08
//	upscale type creative    4/3x   measured 2026-09-12: creative = 10.8 credits = CNY 1.08
//
// The measurements exist because the 2026-08-31 VIP snapshot (CNY 0.27 for v7
// generation) stopped matching what upstream charges: v7 generation now bills the
// CNY 0.54 tier, so the old anchor under-charged every generation call.
var MidjourneyOperationMultipliers = map[string]map[string]float64{
	"mj-v7": {
		"generate":     1,
		"enhance":      1,
		"pan":          1,
		"outpaint":     1,
		"remove-bg":    1,
		"upscale":      1.5,
		"variation":    1.5,
		"edit":         1.5,
		"inpaint":      1.5,
		"remix":        1.5,
		"retexture":    1.5,
		"upload-paint": 1.5,
	},
	"mj-v8-1": {
		"generate":     1,
		"enhance":      1,
		"pan":          1,
		"outpaint":     1,
		"remove-bg":    1,
		"upscale":      1.5,
		"variation":    1.5,
		"edit":         1.5,
		"inpaint":      1.5,
		"remix":        1.5,
		"retexture":    1.5,
		"upload-paint": 1.5,
	},
}

// MidjourneyOperationSuffix maps a public operation name to the upstream model id
// suffix. Plain generation carries no suffix.
var MidjourneyOperationSuffix = map[string]string{
	"generate":     "",
	"enhance":      "-enhance",
	"pan":          "-pan",
	"outpaint":     "-outpaint",
	"remove-bg":    "-remove-bg",
	"upscale":      "-upscale",
	"variation":    "-variation",
	"edit":         "-edit",
	"inpaint":      "-inpaint",
	"remix":        "-remix",
	"retexture":    "-retexture",
	"upload-paint": "-upload-paint",
}

// MidjourneyUpstreamPrefix maps a public version id to the upstream model id
// prefix. Evolink spells the v8.1 generation id with a dot (mj-v8.1); public ids
// never carry dots.
var MidjourneyUpstreamPrefix = map[string]string{
	"mj-v7":   "mj-v7",
	"mj-v8-1": "mj-v8.1",
}

// MidjourneyPassthroughExtraFields are the Evolink-side fields that the generic
// OpenAI image DTO does not declare. They arrive in the request's unknown-field
// map, which the image path drops when it re-serializes the request, so
// Midjourney requests copy them back explicitly.
var MidjourneyPassthroughExtraFields = []string{"model_params", "image_urls"}

// IsMidjourneyImageModel reports whether modelName belongs to the Midjourney
// scheme that routes by operation. Only the two version ids qualify; an
// operation-suffixed id such as mj-v7-upscale is the legacy spelling and keeps
// its existing behaviour (no rewrite, no multiplier), so existing integrations
// stay unaffected until they migrate.
func IsMidjourneyImageModel(modelName string) bool {
	_, ok := MidjourneyUpstreamPrefix[strings.TrimSpace(strings.ToLower(modelName))]
	return ok
}

// MidjourneyOperationFromBody reads model_params.operation from a raw JSON body.
// It is the zero-allocation probe used on the pre-consume path, before the
// request has been parsed into a typed DTO.
func MidjourneyOperationFromBody(body []byte) (string, bool) {
	if len(body) == 0 {
		return "", false
	}
	result := gjson.GetBytes(body, "model_params.operation")
	if result.Type != gjson.String {
		return "", false
	}
	operation := strings.ToLower(strings.TrimSpace(result.String()))
	if operation == "" {
		return "", false
	}
	return operation, true
}

// MidjourneyUpstreamModel resolves the upstream model id and billing multiplier
// for a public Midjourney model plus operation. ok=false means the pair is not in
// the registry and the caller must keep the original model id and price.
func MidjourneyUpstreamModel(modelName, operation string) (string, float64, bool) {
	model := strings.TrimSpace(strings.ToLower(modelName))
	prefix, known := MidjourneyUpstreamPrefix[model]
	if !known {
		return "", 0, false
	}
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation == "" || operation == "generate" {
		return prefix, 1, true
	}
	multiplier, known := MidjourneyOperationMultipliers[model][operation]
	if !known {
		return "", 0, false
	}
	suffix, known := MidjourneyOperationSuffix[operation]
	if !known {
		return "", 0, false
	}
	return prefix + suffix, multiplier, true
}

// MidjourneyPriceMultiplier returns the billing multiplier for a public
// Midjourney model plus operation. ok=false means "do not adjust the price".
func MidjourneyPriceMultiplier(modelName, operation string) (float64, bool) {
	_, multiplier, ok := MidjourneyUpstreamModel(modelName, operation)
	return multiplier, ok
}

// MidjourneyOperationRatioFromBody returns the billing multiplier for a raw
// image request body plus public model name. It returns 1 whenever the request
// carries no operation or the pair is unknown, so billing falls back to the plain
// per-call price instead of guessing.
//
// It also covers the parameter dimensions (quality / speed) so pre-consume and
// settlement agree; an unsupported value returns 1, and relay rejects the request
// before it reaches upstream (see MidjourneyParameterError).
func MidjourneyOperationRatioFromBody(modelName string, body []byte) float64 {
	if !IsMidjourneyImageModel(modelName) {
		return 1
	}
	var probe struct {
		Quality     string `json:"quality"`
		ModelParams struct {
			Operation string `json:"operation"`
			Speed     string `json:"speed"`
			Type      string `json:"type"`
		} `json:"model_params"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return 1
	}
	multiplier, ok := MidjourneyMultiplier(modelName, probe.ModelParams.Operation, probe.Quality, probe.ModelParams.Speed, probe.ModelParams.Type)
	if !ok {
		return 1
	}
	return multiplier
}

// MidjourneyQualityMultipliers prices the output quality dimension. Evolink
// documents `hd` as a 1.5x multiplier on the official price and `standard` as 1x;
// `hd` is only served by mj-v8-1.
var MidjourneyQualityMultipliers = map[string]float64{
	"":         1,
	"standard": 1,
	"hd":       1.5,
}

// MidjourneySpeedMultipliers prices the speed dimension. Evolink documents
// `draft` and `fast` as sharing the 1x multiplier (draft only changes the number
// of returned images).
var MidjourneySpeedMultipliers = map[string]float64{
	"":      1,
	"draft": 1,
	"fast":  1,
	"turbo": 2,
}

// MidjourneyTypeMultipliers prices the operation-specific `type` parameter.
// `upgrade` is creative upscale: measured at 10.8 credits against the CNY 0.81
// upscale tier (CNY 1.08), i.e. 4/3x. Values measured at the operation's own tier
// (subtle/strong variation, strong/subtle remix, standard upscale) stay at 1x.
var MidjourneyTypeMultipliers = map[string]map[string]float64{
	"upscale": {
		"standard": 1,
		"creative": 4.0 / 3.0,
	},
	"variation": {
		"subtle": 1,
		"strong": 1,
	},
	"remix": {
		"subtle": 1,
		"strong": 1,
	},
}

// MidjourneyTypeOperations lists the operations whose `type` parameter is priced
// above. For every other operation a `type` value is not a priced dimension and is
// ignored (upstream validates it).
var MidjourneyTypeOperations = map[string]bool{
	"upscale":   true,
	"variation": true,
	"remix":     true,
}

// MidjourneyParameterError validates the priced parameter space for a public
// Midjourney model. It returns a caller-facing reason when the request uses a
// combination we cannot bill correctly; an empty string means the request is
// fully priced.
func MidjourneyParameterError(modelName, operation, quality, speed, typeValue string) string {
	model := strings.TrimSpace(strings.ToLower(modelName))
	if _, known := MidjourneyUpstreamPrefix[model]; !known {
		return ""
	}
	quality = strings.ToLower(strings.TrimSpace(quality))
	speed = strings.ToLower(strings.TrimSpace(speed))
	typeValue = strings.ToLower(strings.TrimSpace(typeValue))

	switch quality {
	case "", "standard":
	case "hd":
		// Evolink serves native HD through the V8.1 route only.
		if model != "mj-v8-1" {
			return "quality=hd is only supported by mj-v8-1"
		}
	default:
		return "unsupported quality " + quality
	}

	if _, known := MidjourneySpeedMultipliers[speed]; !known {
		return "unsupported model_params.speed " + speed
	}

	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "" && operation != "generate" {
		if _, known := MidjourneyOperationMultipliers[model][operation]; !known {
			return "unsupported operation " + operation
		}
	}

	// `type` is only a priced dimension for the operations that declare it (upscale
	// standard/creative, variation and remix strength). An unrecognised value there
	// would change the upstream price without changing ours, so it is refused.
	if MidjourneyTypeOperations[operation] && typeValue != "" {
		if _, known := MidjourneyTypeMultipliers[operation][typeValue]; !known {
			return "unsupported model_params.type " + typeValue + " for " + operation
		}
	}
	return ""
}

// MidjourneyMultiplier multiplies the priced dimensions of one request:
// operation x quality x speed. ok=false means at least one dimension is not in
// the registry, so the caller must not bill a guessed price.
func MidjourneyMultiplier(modelName, operation, quality, speed, typeValue string) (float64, bool) {
	model := strings.TrimSpace(strings.ToLower(modelName))
	if _, known := MidjourneyUpstreamPrefix[model]; !known {
		return 0, false
	}
	operationMultiplier, ok := MidjourneyPriceMultiplier(model, operation)
	if !ok {
		return 0, false
	}
	qualityMultiplier, ok := MidjourneyQualityMultipliers[strings.ToLower(strings.TrimSpace(quality))]
	if !ok {
		return 0, false
	}
	speedMultiplier, ok := MidjourneySpeedMultipliers[strings.ToLower(strings.TrimSpace(speed))]
	if !ok {
		return 0, false
	}
	typeMultiplier := 1.0
	if op := strings.ToLower(strings.TrimSpace(operation)); MidjourneyTypeOperations[op] {
		if value := strings.ToLower(strings.TrimSpace(typeValue)); value != "" {
			measured, known := MidjourneyTypeMultipliers[op][value]
			if !known {
				return 0, false
			}
			typeMultiplier = measured
		}
	}
	return operationMultiplier * qualityMultiplier * speedMultiplier * typeMultiplier, true
}

// MidjourneyOperationFromRawParams reads the operation out of an already-decoded
// model_params object.
func MidjourneyOperationFromRawParams(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var params struct {
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", false
	}
	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation == "" {
		return "", false
	}
	return operation, true
}
