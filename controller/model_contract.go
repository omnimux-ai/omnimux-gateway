package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
)

// GetModelContract publishes one model's declared capability contract and, for
// enrolled video models, the verified variants served to the caller's groups.
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
	response := gin.H{
		"contract": contract,
		"version":  modelcontracts.Version(&document),
	}
	if contract.GatewayProtocol == modelcontracts.VideoProtocolGenerations ||
		modelcontracts.ActiveVideoCapabilities().Managed(modelID) {
		c.Header("Cache-Control", "private, no-store")
		_, _, groupRatios := pricingGroupView(c)
		pricing := model.Pricing{ModelName: modelID, Contract: &contract}
		for _, row := range model.GetCatalogPricing() {
			if row.ModelName == modelID {
				pricing = row
				break
			}
			if row.ModelName == contract.ModelID {
				pricing = row
			}
		}
		if effective := pricing.VideoCapabilitiesForGroups(groupRatios); effective != nil {
			response["effective_video"] = effective
		}
	}
	common.ApiSuccess(c, response)
}
