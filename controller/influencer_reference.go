package controller

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/sora"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const influencerReferenceGroup = "gpt-image-2.5-economy"

var influencerReferenceHTTPClient = service.NewInfluencerReferenceHTTPClient()

// PrepareInfluencerSheet validates routing and media before channel distribution.
func PrepareInfluencerSheet(c *gin.Context) {
	var req influencerSheetRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		c.Abort()
		return
	}
	modelName, suffixGroup := influencerModelGroup(req.Model)
	group, ok := influencerSheetGroup(c, req.Group, suffixGroup)
	if !ok {
		c.Abort()
		return
	}
	var normalized map[string]any
	if err := common.UnmarshalBodyReusable(c, &normalized); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		c.Abort()
		return
	}
	normalized["model"] = modelName
	normalized["group"] = group
	if err := influencerRewriteBody(c, normalized); err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to normalize request")
		c.Abort()
		return
	}
	if modelName == influencerImageModel {
		if _, present := normalized["seed"]; present {
			influencerError(c, 400, "invalid_request", "seed is not supported by the reviewed image contract")
			c.Abort()
			return
		}
		for field := range normalized {
			switch field {
			case "model", "group", "tier", "selection", "brief", "size", "image_url", "image_mode", "mask_url", "edit_instruction", "quality", "n", "output_format", "response_format":
			default:
				influencerError(c, 400, "invalid_request", "the image request contains an unsupported parameter: "+field)
				c.Abort()
				return
			}
		}
		if group != influencerImageGroup {
			influencerError(c, http.StatusBadRequest, "reference_group_unsupported", "the reviewed Sunburst image group is required")
			c.Abort()
			return
		}
		req.Model = modelName
		out, _, code, err := buildInfluencerSheet(req)
		if err != nil {
			influencerError(c, 400, code, err.Error())
			c.Abort()
			return
		}
		c.Set(ctxInfluencerImageMode, out["image_mode"])
		defer discardUnacceptedInfluencerImages(c)
		if !prepareInfluencerImageInputs(c, req, out) || !prepareInfluencerSheetRouting(c, modelName, group, req.ImageURL != "") {
			c.Abort()
			return
		}
		c.Next()
		return
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		if !prepareInfluencerSheetRouting(c, modelName, group, false) {
			c.Abort()
		}
		return
	}
	if (modelName != "gpt-image-2.5" || group != influencerReferenceGroup) && (modelName != influencerImageModel || group != influencerImageGroup) {
		influencerError(c, http.StatusBadRequest, "reference_group_unsupported", "reference image requires the reviewed economy image route")
		c.Abort()
		return
	}
	if !prepareInfluencerSheetRouting(c, modelName, group, true) {
		c.Abort()
		return
	}
	if err := service.ValidateInfluencerReference(c.Request.Context(), req.ImageURL, influencerReferenceHTTPClient); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_reference_image", err.Error())
		c.Abort()
		return
	}
	c.Set("influencer_reference_preflight", strings.TrimSpace(req.ImageURL))
}

// PrepareInfluencerMultiView only prepares native routing. Parent ownership,
// signed artifact access and readable media are still checked by the handler
// before durable acceptance, funding or any supplier submission.
func PrepareInfluencerMultiView(c *gin.Context) {
	var req influencerMultiViewRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		c.Abort()
		return
	}
	modelName, suffixGroup := influencerModelGroup(req.Model)
	if modelName == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "model is required")
		c.Abort()
		return
	}
	if strings.TrimSpace(req.ParentTaskID) == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "parent_task_id is required")
		c.Abort()
		return
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "image_url is required")
		c.Abort()
		return
	}
	if size := strings.TrimSpace(req.Size); size != "" && size != influencerMultiViewSize {
		influencerError(c, http.StatusBadRequest, "invalid_request", "size must be 16:9")
		c.Abort()
		return
	}
	group, ok := influencerSheetGroup(c, req.Group, suffixGroup)
	if !ok {
		c.Abort()
		return
	}
	if (modelName != "gpt-image-2.5" || group != influencerReferenceGroup) && (modelName != influencerImageModel || group != influencerImageGroup) {
		influencerError(c, http.StatusBadRequest, "reference_group_unsupported", "reference image requires the reviewed economy image route")
		c.Abort()
		return
	}
	var normalized map[string]any
	if common.UnmarshalBodyReusable(c, &normalized) != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		c.Abort()
		return
	}
	normalized["model"], normalized["group"] = modelName, group
	if err := influencerRewriteBody(c, normalized); err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to normalize request")
		c.Abort()
		return
	}
	if modelName == influencerImageModel {
		for field := range normalized {
			switch field {
			case "model", "group", "parent_task_id", "image_url", "size":
			default:
				influencerError(c, 400, "invalid_request", "the multi-view request contains an unsupported parameter: "+field)
				c.Abort()
				return
			}
		}
		c.Set(ctxInfluencerImageMode, "reference")
		defer discardUnacceptedInfluencerImages(c)
		imageRequest := influencerSheetRequest{Model: modelName, ImageMode: "reference", ImageURL: req.ImageURL, Size: strings.TrimSpace(req.Size)}
		out, _, code, err := buildInfluencerSheet(imageRequest)
		if err != nil {
			influencerError(c, http.StatusBadRequest, code, err.Error())
			c.Abort()
			return
		}
		if !prepareInfluencerImageInputs(c, imageRequest, out) {
			c.Abort()
			return
		}
	}
	if !prepareInfluencerSheetRouting(c, modelName, group, true) {
		c.Abort()
		return
	}
	if modelName == influencerImageModel {
		c.Next()
	}
}

// influencerReferenceChannelSupportsInput is deliberately narrower than a channel
// type: a compatible OpenAI transport must resolve to the reviewed supplier SKU.
func influencerReferenceChannelSupportsInput(ch *model.Channel, modelName string) bool {
	if modelName == influencerImageModel {
		return helper.ImageTaskChannelSupportsMode(ch, influencerImageModel, "reference")
	}
	if ch == nil || modelName != "gpt-image-2.5" {
		return false
	}
	if ch.Type == constant.ChannelTypeGoEasyAPI {
		return true
	}
	if ch.Type != constant.ChannelTypeOpenAI {
		return false
	}
	var other relaykitdto.ChannelOtherSettings
	if ch.OtherSettings != "" && common.UnmarshalJsonStr(ch.OtherSettings, &other) != nil {
		return false
	}
	// Static capability proof cannot authorize subsequent dynamic rewrites.
	if other.AdvancedCustom != nil && len(other.AdvancedCustom.ModelByParam) > 0 {
		return false
	}
	var overrides map[string]any
	if ch.ParamOverride != nil && strings.TrimSpace(*ch.ParamOverride) != "" {
		if common.UnmarshalJsonStr(*ch.ParamOverride, &overrides) != nil || len(overrides) != 0 {
			return false
		}
	}
	mapped := &gin.Context{}
	mapped.Set("model_mapping", ch.GetModelMapping())
	info := &relaycommon.RelayInfo{OriginModelName: modelName, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: modelName}}
	if helper.ModelMappedHelper(mapped, info, nil) != nil {
		return false
	}
	return sora.IsEvoLinkGPTImageModel(info.GetUpstreamModelName())
}

// influencerReferenceAutoGroups reuses native exclusions for every member, so a
// compatible channel never admits an incompatible neighbour in the same group.
func influencerReferenceAutoGroups(c *gin.Context, modelName string, groups []string) ([]string, error) {
	abilities, err := model.GetAllEnableAbilityWithChannels()
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0)
	for _, ability := range abilities {
		if ability.Model == modelName && !slices.Contains(ids, ability.ChannelId) {
			ids = append(ids, ability.ChannelId)
		}
	}
	var channels []model.Channel
	if len(ids) > 0 {
		if err := model.DB.Select("id", "type", "status", "model_mapping", "param_override", "settings", "setting").Where("id IN ?", ids).Find(&channels).Error; err != nil {
			return nil, err
		}
	}
	compatible := make(map[int]*model.Channel, len(channels))
	for i := range channels {
		ch := &channels[i]
		mode := c.GetString(ctxInfluencerImageMode)
		if mode == "" {
			mode = "reference"
		}
		if ch.Status == common.ChannelStatusEnabled && ((modelName == influencerImageModel && helper.ImageTaskChannelSupportsRequest(ch, influencerImageModel, mode, c.GetString(helper.ImageTaskRequestSizeContextKey), helper.ImageTaskFrozenFormats(c)...)) || (modelName != influencerImageModel && influencerReferenceChannelSupportsInput(ch, modelName))) {
			compatible[ch.Id] = ch
		} else {
			service.GetChannelConstraints(c).AddFilter(dto.ChannelFilter{Kind: dto.FilterExcludeChannels, ExcludeChannelIDs: []int{ch.Id}})
		}
	}
	available := make(map[string]bool)
	for _, ability := range abilities {
		if ability.Model != modelName || compatible[ability.ChannelId] == nil {
			continue
		}
		ch := compatible[ability.ChannelId]
		if ok, _ := model.ChannelSatisfiesFilters(ch, modelName, service.GetChannelConstraints(c).Filters); ok {
			available[ability.Group] = true
		}
	}
	kept := make([]string, 0, len(groups))
	for _, group := range groups {
		if available[group] {
			kept = append(kept, group)
		}
	}
	if len(kept) == 0 {
		return nil, errors.New("no compatible reference image route is available")
	}
	return kept, nil
}

func influencerReferenceAutoRouting(c *gin.Context) *model.GenerationRoutingPolicy {
	value, exists := c.Get(ctxInfluencerRouting)
	routing, ok := value.(*model.GenerationRoutingPolicy)
	if !exists || !ok || routing == nil || routing.Mode != model.GenerationRoutingAuto || c.GetString("influencer_reference_preflight") == "" {
		return nil
	}
	return routing
}

func influencerReferenceSelectedChannel(c *gin.Context) (*model.Channel, bool) {
	if model.DB == nil {
		return nil, false
	}
	ch, err := model.GetChannelById(common.GetContextKeyInt(c, constant.ContextKeyChannelId), true)
	if err != nil || ch == nil || ch.Status != common.ChannelStatusEnabled {
		return nil, false
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	modelName := "gpt-image-2.5"
	if c.GetString(ctxInfluencerImageMode) != "" {
		modelName = influencerImageModel
	}
	if routing := influencerReferenceAutoRouting(c); routing != nil {
		if selected := common.GetContextKeyString(c, constant.ContextKeyAutoGroup); selected != "" {
			group = selected
		}
		compatible := influencerReferenceChannelSupportsInput(ch, modelName)
		if modelName == influencerImageModel {
			compatible = helper.ImageTaskChannelSupportsRequest(ch, influencerImageModel, c.GetString(ctxInfluencerImageMode), c.GetString(helper.ImageTaskRequestSizeContextKey), helper.ImageTaskFrozenFormats(c)...)
		}
		if !slices.Contains(routing.Groups, group) || !compatible {
			return nil, false
		}
	} else if group != influencerReferenceGroup || ch.Type != constant.ChannelTypeGoEasyAPI {
		return nil, false
	}
	var bindings int64
	err = model.DB.Model(&model.Ability{}).Where(&model.Ability{Group: group, Model: modelName, ChannelId: ch.Id, Enabled: true}).Count(&bindings).Error
	return ch, err == nil && bindings > 0
}

// submitInfluencerReferenceTask keeps native accounting while forbidding duplicate edit POSTs.
func submitInfluencerReferenceTask(c *gin.Context) {
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to prepare reference task")
		return
	}
	ch, valid := influencerReferenceSelectedChannel(c)
	if !valid {
		influencerError(c, http.StatusServiceUnavailable, "reference_route_unavailable", "reference image route is unavailable")
		return
	}
	// Only the new, explicit native Auto reference intent may migrate. Legacy
	// group records retain their original single-attempt provider pin.
	if influencerReferenceAutoRouting(c) == nil {
		info.LockedChannel = ch
		service.GetChannelConstraints(c).AddPin(dto.ChannelPin{
			ChannelId: ch.Id,
			Source:    dto.ChannelPinSource("influencer_reference"),
			Rank:      dto.PinRankToken,
			RetryMode: dto.PinRetrySingleAttempt,
		})
	}
	outcome, taskErr := executeTaskSubmission(c, info)
	if taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	presentTaskSubmission(c, outcome)
}

// influencerParentImageArtifact accepts only server-projected image capabilities, not URL substrings.
func influencerParentImageArtifact(parent *model.Task, raw string) bool {
	candidate, err := url.Parse(raw)
	if err != nil || candidate.Scheme != "https" || candidate.User != nil || candidate.Fragment != "" || candidate.Opaque != "" {
		return false
	}
	artifacts, _, err := resolveTaskArtifacts(parent)
	if err != nil {
		return false
	}
	for _, artifact := range artifacts {
		if artifact.Type != "image" {
			continue
		}
		expected, err := url.Parse(artifact.ContentURL)
		if err != nil {
			continue
		}
		if candidate.Scheme == expected.Scheme && strings.EqualFold(candidate.Host, expected.Host) && candidate.EscapedPath() == expected.EscapedPath() && service.VerifyTaskArtifactAccess(candidate.Query().Get(service.TaskArtifactAccessQueryParameter), parent.TaskID, artifact.Key) {
			return true
		}
	}
	return false
}

// fork: workbench groups are explicit internal routing contracts, never provider fields.
func influencerModelGroup(raw string) (string, string) {
	name := strings.TrimSpace(raw)
	if index := strings.LastIndex(name, "@"); index > 0 && index < len(name)-1 {
		base, group := strings.TrimSpace(name[:index]), strings.TrimSpace(name[index+1:])
		if base != "" && group != "" {
			return base, group
		}
	}
	return name, ""
}

func influencerSheetGroup(c *gin.Context, bodyGroup string, suffixGroups ...string) (string, bool) {
	group := strings.TrimSpace(bodyGroup)
	headerGroup := c.GetHeader("X-Omnimux-Group")
	// Only a server-prepared route may translate its internal Auto header back
	// to the already validated quote group for the handler's second validation.
	if headerGroup == "auto" && c.GetString(ctxInfluencerQuoteGroup) != "" {
		headerGroup = c.GetString(ctxInfluencerQuoteGroup)
	}
	values := append(suffixGroups, headerGroup, c.GetHeader("New-Api-Group"))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if group != "" && group != value {
			influencerError(c, http.StatusBadRequest, "group_conflict", "all explicit group sources must agree")
			return "", false
		}
		group = value
	}
	if group == "" || group == "auto" {
		influencerError(c, http.StatusBadRequest, "explicit_group_required", "an explicit supplier-aligned group is required")
		return "", false
	}
	c.Request.Header.Set("X-Omnimux-Group", group)
	c.Request.Header.Del("New-Api-Group")
	return group, true
}
