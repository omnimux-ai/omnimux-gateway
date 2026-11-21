package controller

import (
	"slices"
	"sync/atomic"

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

type pricingResponseRow struct {
	model.Pricing
	DefaultGroup string   `json:"default_group"`
	GroupOrder   []string `json:"group_order"`
}

var (
	pricingGroupFallbackRows atomic.Uint64
	pricingGroupReadFailures atomic.Uint64
)

func pricingDefaultGroup(configured, userGroup string, enabled, order []string, usable map[string]string) string {
	if configured != "" {
		if _, ok := usable[configured]; ok && slices.Contains(enabled, configured) {
			return configured
		}
		return ""
	}
	for _, group := range []string{userGroup, "default"} {
		if _, ok := usable[group]; ok && group != "" && slices.Contains(enabled, group) {
			return group
		}
	}
	// The order has already been checked against enable_groups.
	for _, group := range order {
		if _, ok := usable[group]; ok && group != "" {
			return group
		}
	}
	return ""
}

func GetPricing(c *gin.Context) {
	pricing := model.GetCatalogPricing()
	group, usableGroup, groupRatio := pricingGroupView(c)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	c.Header("Cache-Control", "private, no-store")
	config, err := model.LoadPricingGroupConfig(c.Request.Context())
	if err != nil {
		pricingGroupReadFailures.Add(1)
	}

	var rows []pricingResponseRow
	if pricing != nil {
		rows = make([]pricingResponseRow, 0, len(pricing))
	}
	var fallbackRows uint64
	videoManaged := modelcontracts.ActiveVideoCapabilities() != nil
	for _, item := range pricing {
		entry := config.Models[item.ModelName]
		hasOrder := entry.HasValidOrder(item.EnableGroup)
		order := entry.GroupOrder
		if !hasOrder {
			order = item.EnableGroup
		}
		if entry.DefaultGroup == "" || !hasOrder {
			fallbackRows++
		}
		if videoManaged {
			item.EffectiveVideo = item.VideoCapabilitiesForGroups(groupRatio)
		}
		rows = append(rows, pricingResponseRow{
			Pricing:      item,
			DefaultGroup: pricingDefaultGroup(entry.DefaultGroup, group, item.EnableGroup, order, usableGroup),
			GroupOrder:   order,
		})
	}
	if fallbackRows > 0 {
		pricingGroupFallbackRows.Add(fallbackRows)
	}

	response := gin.H{
		"success":            true,
		"data":               rows,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
		"pricing_version":    "model-group-display-v1",
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
