package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/modelcontracts"
)

// GetModelContract publishes one public model id's declared capability contract.
//
// fork: OmniMux — the catalog surfaces describe price and routing, so a caller
// has no way to learn which parameter values a model accepts before submitting
// and only finds out from a rejected request. This returns the same contract the
// model page renders from, addressed by public model id, so a client can read it
// instead of maintaining its own table.
//
// The contract is the single authority: whatever a model declares here is what
// the page offers and what a request must supply. Read-only and descriptive — it
// never participates in billing, quota, precharge or settlement.
// See FORK_CUSTOMIZATIONS.md.
func GetModelContract(c *gin.Context) {
	modelID := c.Param("model")
	if modelID == "" {
		common.ApiErrorMsg(c, "a model id is required")
		return
	}

	contract, ok := modelcontracts.For(modelID)
	if !ok {
		// Absence is a real answer, not an error: the model exists upstream but
		// no governance spec describes it yet. A caller must treat it as
		// "not established" rather than "no such capability".
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "no capability contract is published for model " + modelID,
		})
		return
	}

	document := modelcontracts.All()
	common.ApiSuccess(c, gin.H{
		"contract": contract,
		"version":  modelcontracts.Version(&document),
	})
}
