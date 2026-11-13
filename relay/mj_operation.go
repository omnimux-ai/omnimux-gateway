package relay

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// Midjourney request-side routing (fork: mj-operation-routing).
//
// The routing tables and price multipliers live in relaykit/types
// (midjourney.go) because the pre-consume price path needs them too and
// relay/helper cannot import relay.

// ApplyMidjourneyOperation rewrites a Midjourney image request so the requested
// operation reaches the upstream model field, passes the Evolink-only fields
// through, and returns the billing multiplier to apply. Requests outside the
// Midjourney scheme are left untouched with handled=false.
//
// An unsupported operation or an unpriced parameter dimension is rejected
// (handled=false with a caller-facing reason) rather than silently downgraded,
// because variation / upscale / enhance share one request shape and the quality /
// speed dimensions can only be priced once their rate is known: guessing would hit
// the wrong upstream model or charge the wrong price.
func ApplyMidjourneyOperation(info *relaycommon.RelayInfo, request *dto.ImageRequest) (multiplier float64, reason string, handled bool) {
	if info == nil || request == nil {
		return 0, "", false
	}
	modelName := info.OriginModelName
	if !types.IsMidjourneyImageModel(modelName) {
		return 0, "", false
	}
	operation, _ := midjourneyRequestedOperation(request)
	quality, speed, operationType := midjourneyRequestedDimensions(request)
	if rejection := types.MidjourneyParameterError(modelName, operation, quality, speed, operationType); rejection != "" {
		return 0, rejection, false
	}

	priceMultiplier, ok := types.MidjourneyMultiplier(modelName, operation, quality, speed, operationType)
	if !ok {
		return 0, operation, false
	}
	// Resolve the upstream id for every request, not only for actions: the channel
	// maps the public version id to itself, while upstream spells its generation
	// model with a dot (mj-v8.1). Skipping this for plain generation would send
	// "mj-v8-1" upstream, which Evolink does not serve.
	upstreamModel, _, known := types.MidjourneyUpstreamModel(modelName, operation)
	if !known {
		return 0, operation, false
	}
	info.UpstreamModelName = upstreamModel
	// The image DTO always serializes "model", so the request itself has to carry
	// the rewritten id: OptionalParams only fills keys the struct does not declare.
	request.Model = upstreamModel
	mergeMidjourneyExtraFields(request)
	return priceMultiplier, operation, true
}

// midjourneyRequestedDimensions reads the priced non-operation dimensions:
// quality is the top-level OpenAI image field that Evolink also serves, while
// speed and the operation-specific type travel inside model_params.
func midjourneyRequestedDimensions(request *dto.ImageRequest) (quality string, speed string, operationType string) {
	if request == nil {
		return "", "", ""
	}
	quality = request.Quality
	if request.Extra == nil {
		return quality, "", ""
	}
	raw, ok := request.Extra["model_params"]
	if !ok || len(raw) == 0 {
		return quality, "", ""
	}
	var params struct {
		Speed string `json:"speed"`
		Type  string `json:"type"`
	}
	if err := common.Unmarshal(raw, &params); err != nil {
		return quality, "", ""
	}
	return quality, params.Speed, params.Type
}

// midjourneyRequestedOperation reads model_params.operation from a parsed image
// request.
func midjourneyRequestedOperation(request *dto.ImageRequest) (string, bool) {
	if request == nil || request.Extra == nil {
		return "", false
	}
	raw, ok := request.Extra["model_params"]
	if !ok || len(raw) == 0 {
		return "", false
	}
	return types.MidjourneyOperationFromRawParams(raw)
}

// mergeMidjourneyExtraFields registers the declared Evolink-only fields as
// optional params, so the image path carries them into the outgoing body even
// though the OpenAI image DTO does not declare them.
func mergeMidjourneyExtraFields(request *dto.ImageRequest) {
	if request == nil {
		return
	}
	if request.OptionalParams == nil {
		request.OptionalParams = make(map[string]json.RawMessage, len(types.MidjourneyPassthroughExtraFields))
	}
	for _, key := range types.MidjourneyPassthroughExtraFields {
		if _, ok := request.OptionalParams[key]; !ok {
			request.OptionalParams[key] = nil
		}
	}
}

// marshalImageRequestBody renders the exact upstream JSON body for a Midjourney
// request. Used by tests to prove the operation and reference images survive the
// image path's re-serialization.
func marshalImageRequestBody(request *dto.ImageRequest) ([]byte, error) {
	if request == nil {
		return nil, nil
	}
	mergeMidjourneyExtraFields(request)
	data, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	return data, nil
}
