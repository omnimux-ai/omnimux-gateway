package controller

import (
	"errors"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	ctxInfluencerRouting    = "influencer_generation_routing"
	ctxInfluencerQuoteGroup = "influencer_quote_group"
)

func influencerGenerationAttemptLimit() int {
	if common.RetryTimes <= 0 {
		return 1
	}
	return 2
}

// influencerImageAutoGroups narrows the maintained native order to model-owned
// routes with the original effective sale price. Shared and arbitrary groups
// cannot become a workbench fallback merely because they serve the same model.
func influencerImageAutoGroups(c *gin.Context, modelName, anchor string) ([]string, error) {
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if !service.IsUserSelectableGroup(userGroup, anchor) {
		return nil, errors.New("image quote group is not available to this account")
	}
	anchorRatio := service.GetUserGroupRatio(userGroup, anchor)
	if anchorRatio < 0 || math.IsNaN(anchorRatio) || math.IsInf(anchorRatio, 0) || model.DB == nil {
		return nil, errors.New("image quote group is unavailable")
	}
	abilities, err := model.GetAllEnableAbilityWithChannels()
	if err != nil {
		return nil, err
	}
	channelIDs := make([]int, 0)
	for _, ability := range abilities {
		if ability.Model == modelName && !slices.Contains(channelIDs, ability.ChannelId) {
			channelIDs = append(channelIDs, ability.ChannelId)
		}
	}
	var enabledIDs []int
	if len(channelIDs) > 0 {
		if err := model.DB.Model(&model.Channel{}).Where("id IN ? AND status = ?", channelIDs, common.ChannelStatusEnabled).Pluck("id", &enabledIDs).Error; err != nil {
			return nil, err
		}
	}
	available := make(map[string]bool)
	for _, ability := range abilities {
		if ability.Model == modelName && slices.Contains(enabledIDs, ability.ChannelId) {
			available[ability.Group] = true
		}
	}
	groups := make([]string, 0)
	for _, group := range service.GetUserAutoGroup(userGroup) {
		if strings.HasPrefix(group, modelName+"-") && available[group] && service.GetUserGroupRatio(userGroup, group) == anchorRatio {
			groups = append(groups, group)
		}
	}
	if len(groups) == 0 {
		return nil, errors.New("no approved image generation route is available")
	}
	return groups, nil
}

func prepareInfluencerSheetRouting(c *gin.Context, modelName, group string, hasImage bool) bool {
	routing := &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{group}, MaxAttempts: influencerGenerationAttemptLimit()}
	if hasImage {
		routing.MaxAttempts = 1
	}
	if (modelName == "gpt-image-2.5" && group == influencerReferenceGroup) || (modelName == influencerImageModel && group == influencerImageGroup) {
		groups, err := influencerImageAutoGroups(c, modelName, group)
		if (hasImage || modelName == influencerImageModel) && err == nil {
			groups, err = influencerReferenceAutoGroups(c, modelName, groups)
		}
		if err != nil {
			if hasImage {
				influencerError(c, http.StatusServiceUnavailable, "reference_route_unavailable", "reference image route is unavailable")
				return false
			}
			influencerError(c, http.StatusServiceUnavailable, "generation_route_unavailable", "image generation route is unavailable")
			return false
		}
		routing.Mode, routing.Groups, routing.MaxAttempts = model.GenerationRoutingAuto, groups, influencerGenerationAttemptLimit()
		c.Set(ctxInfluencerQuoteGroup, group)
		applyInfluencerRoutingContext(c, routing)
	}
	c.Set(ctxInfluencerRouting, routing)
	return true
}

// The private policy is the only source of the native Auto list. Restoring a
// queued request never inherits the account's broader global routing list.
func applyInfluencerRoutingContext(c *gin.Context, routing *model.GenerationRoutingPolicy) {
	for _, key := range []constant.ContextKey{constant.ContextKeyAutoGroup, constant.ContextKeyAutoGroupIndex, constant.ContextKeyAutoGroupRetryIndex} {
		delete(c.Keys, string(key))
	}
	if routing.Mode == model.GenerationRoutingAuto {
		c.Request.Header.Set("X-Omnimux-Group", "auto")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "auto")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "auto")
		common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, append([]string(nil), routing.Groups...))
		common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, true)
		return
	}
	c.Request.Header.Set("X-Omnimux-Group", routing.Groups[0])
	common.SetContextKey(c, constant.ContextKeyUsingGroup, routing.Groups[0])
	common.SetContextKey(c, constant.ContextKeyTokenGroup, routing.Groups[0])
}

func generationRequestRouting(c *gin.Context, group, path string) *model.GenerationRoutingPolicy {
	if value, ok := c.Get(ctxInfluencerRouting); ok {
		if routing, ok := value.(*model.GenerationRoutingPolicy); ok && routing != nil {
			return &model.GenerationRoutingPolicy{Mode: routing.Mode, Groups: append([]string(nil), routing.Groups...), MaxAttempts: routing.MaxAttempts}
		}
	}
	maxAttempts := influencerGenerationAttemptLimit()
	if path == "/pg/images/edits" {
		maxAttempts = 1
	}
	return &model.GenerationRoutingPolicy{Mode: model.GenerationRoutingGroup, Groups: []string{group}, MaxAttempts: maxAttempts}
}
