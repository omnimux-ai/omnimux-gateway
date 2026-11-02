package fal

import (
	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// EstimateBilling computes duration-based billing multiplier:
// - 5s duration: 1.0x multiplier
// - 10s duration: 2.0x multiplier
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	var body map[string]any
	if c != nil {
		if raw, ok := c.Get("task_request_raw"); ok {
			body, _ = raw.(map[string]any)
		}
		if body == nil {
			body = map[string]any{}
			_ = common.UnmarshalBodyReusable(c, &body)
		}
	}

	duration := parseDurationInt(body["duration"])
	if duration >= 10 {
		return map[string]float64{"duration": 2.0}
	}
	return map[string]float64{"duration": 1.0}
}
