package helper

import (
	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// fork: mj-operation-routing
//
// Midjourney actions are routed by the request (model_params.operation) and each
// action carries its own official price tier. Pre-consume runs before the typed
// image request is available, so the operation is probed straight from the raw
// body; the settlement path re-derives the same value from the parsed request and
// both use the shared tables in relaykit/types.
const midjourneyOperationRatioKey = "mj_operation"

// midjourneyOperationRatio returns the billing multiplier for a Midjourney image
// request. It returns 1 for every request that is not a Midjourney action, so the
// caller can apply the ratio unconditionally.
func midjourneyOperationRatio(c *gin.Context, info *relaycommon.RelayInfo) float64 {
	if c == nil || info == nil {
		return 1
	}
	modelName := info.GetOriginModelName()
	if !types.IsMidjourneyImageModel(modelName) {
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
	if err := common.UnmarshalBodyReusable(c, &probe); err != nil {
		return 1
	}
	multiplier, ok := types.MidjourneyMultiplier(
		modelName,
		probe.ModelParams.Operation,
		probe.Quality,
		probe.ModelParams.Speed,
		probe.ModelParams.Type,
	)
	if !ok {
		return 1
	}
	return multiplier
}
