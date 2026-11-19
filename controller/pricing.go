package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

// filterPricingDefaults keeps only recommendation entries whose model is visible
// in this response and still carries at least one enabling group. A recommendation
// must never leak a model id the caller cannot actually select.
func filterPricingDefaults(pricing []model.Pricing, defaults map[string]string) map[string]string {
	if len(defaults) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(pricing))
	for _, item := range pricing {
		if len(item.EnableGroup) > 0 {
			present[item.ModelName] = struct{}{}
		}
	}
	filtered := make(map[string]string)
	for operation, modelID := range defaults {
		if _, ok := present[modelID]; ok {
			filtered[operation] = modelID
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func pricingGroupView(c *gin.Context) (string, map[string]string, map[string]float64) {
	groupRatio := ratio_setting.GetGroupRatioCopy()
	var group string
	if userID := c.GetInt("id"); userID > 0 {
		user, err := model.GetUserCache(userID)
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup := service.GetUserUsableGroups(group)
	for group := range groupRatio {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}
	return group, usableGroup, groupRatio
}

func GetPricing(c *gin.Context) {
	pricing := model.GetCatalogPricing()
	group, usableGroup, groupRatio := pricingGroupView(c)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	if modelcontracts.ActiveVideoCapabilities() != nil {
		c.Header("Cache-Control", "private, no-store")
		for i := range pricing {
			pricing[i].EffectiveVideo = pricing[i].VideoCapabilitiesForGroups(groupRatio)
		}
	}

	response := gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	}
	if defaults := filterPricingDefaults(pricing, modelcontracts.All().Defaults); len(defaults) > 0 {
		response["defaults"] = defaults
	}
	c.JSON(200, response)
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccessT(c, "Model ratios reset", nil)
}
