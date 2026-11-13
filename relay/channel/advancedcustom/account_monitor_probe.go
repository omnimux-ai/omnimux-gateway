package advancedcustom

import (
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

const accountMonitorProbeContextKey = "advancedcustom.account-monitor-probe"

type accountMonitorProbeContext struct {
	model  string
	policy string
}

// EnableAccountMonitorProbe bounds a validated management probe's transport and
// body read. It does not enable shape collection or inspect client input.
func EnableAccountMonitorProbe(c *gin.Context, model, policy string) bool {
	if c == nil || !accountMonitorProbePolicyMatches(model, policy) {
		return false
	}
	c.Set(accountMonitorProbeContextKey, &accountMonitorProbeContext{model: model, policy: policy})
	return true
}

func accountMonitorProbePolicyMatches(model, policy string) bool {
	switch model {
	case "x-user", "tiktok-user":
		return policy == dto.AdvancedCustomResponseValidationTikHubUser
	case "x-posts", "tiktok-posts":
		return policy == dto.AdvancedCustomResponseValidationTikHubPosts
	}
	return false
}

func accountMonitorProbeEnabled(c *gin.Context, model, policy string) bool {
	if c == nil || !accountMonitorProbePolicyMatches(model, policy) {
		return false
	}
	value, exists := c.Get(accountMonitorProbeContextKey)
	if !exists {
		return false
	}
	probe, ok := value.(*accountMonitorProbeContext)
	return ok && probe != nil && probe.model == model && probe.policy == policy
}
