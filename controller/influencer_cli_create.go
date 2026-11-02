package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/gin-gonic/gin"
)

type influencerCLICharacterRequest struct {
	RequestID string               `json:"request_id"`
	Tier      string               `json:"tier,omitempty"`
	Selection influencer.Selection `json:"selection"`
	Brief     string               `json:"brief,omitempty"`
	ImageURL  string               `json:"image_url,omitempty"`
}

type influencerCLIMultiViewRequest struct {
	RequestID string `json:"request_id"`
}

type influencerCLIVideoRequest struct {
	RequestID       string   `json:"request_id"`
	Mode            string   `json:"mode,omitempty"`
	CharacterTaskID string   `json:"character_task_id,omitempty"`
	ImageURLs       []string `json:"image_urls,omitempty"`
	VideoURL        string   `json:"video_url,omitempty"`
	VideoTaskID     string   `json:"video_task_id,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	AspectRatio     *string  `json:"aspect_ratio,omitempty"`
	Prompt          string   `json:"prompt,omitempty"`
	// DurationSec is the analysis-derived output length for mode=recreate. Other
	// modes derive their duration from the reference clip and ignore it.
	DurationSec int `json:"duration_sec,omitempty"`
}

// Product routes use enabled catalog bindings and the account's usable groups;
// no request-provided model, group or routing header participates in selection.
func influencerCLIRouteAvailable(c *gin.Context, modelName, group string) bool {
	if model.DB == nil || !service.IsUserSelectableGroup(c.GetString("group"), group) {
		return false
	}
	var bindings []model.Ability
	if err := model.DB.WithContext(c.Request.Context()).Where(&model.Ability{Model: modelName, Group: group, Enabled: true}).Find(&bindings).Error; err != nil || len(bindings) == 0 {
		return false
	}
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.ChannelId)
	}
	var channels []model.Channel
	if err := model.DB.WithContext(c.Request.Context()).Select("id", "status").Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return false
	}
	active := false
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		active = true
	}
	return active
}

// Capability reads use the same native preparation as submission, isolated from
// the caller's headers, channel exclusions, response and routing context.
func influencerCLIImageRouteAvailable(c *gin.Context, reference bool) bool {
	probe, _ := gin.CreateTestContext(httptest.NewRecorder())
	probe.Keys = c.Copy().Keys
	probe.Request = c.Request.Clone(c.Request.Context())
	if constraints, ok := common.GetContextKeyType[*dto.ChannelConstraints](c, constant.ContextKeyChannelConstraints); ok && constraints != nil {
		common.SetContextKey(probe, constant.ContextKeyChannelConstraints, &dto.ChannelConstraints{
			Pins:    append([]dto.ChannelPin(nil), constraints.Pins...),
			Filters: append([]dto.ChannelFilter(nil), constraints.Filters...),
		})
	}
	mode := "text"
	if reference {
		mode = "reference"
	}
	probe.Set(ctxInfluencerImageMode, mode)
	probe.Set(helper.ImageTaskRequestSizeContextKey, influencerImagePixels)
	return prepareInfluencerSheetRouting(probe, influencerImageModel, influencerImageGroup, reference)
}

func PostInfluencerCLICharacter(c *gin.Context) {
	defer discardUnacceptedInfluencerImages(c)
	var req influencerCLICharacterRequest
	if !decodeInfluencerCLIRequest(c, &req, "request_id", "tier", "selection", "brief", "image_url") {
		return
	}
	req.Tier, req.Brief, req.ImageURL = strings.TrimSpace(req.Tier), strings.TrimSpace(req.Brief), strings.TrimSpace(req.ImageURL)
	if req.Tier == "" {
		req.Tier = "normal"
	}
	if req.Selection == nil {
		req.Selection = influencer.Selection{}
	}
	intent := req
	intent.RequestID = ""
	payloadHash, ok := influencerCLIHash(c, req.RequestID, gin.H{"kind": "character", "input": intent})
	if !ok || replayInfluencerCLIRequest(c, req.RequestID, payloadHash) || !requireInfluencerCLIWriteAvailability(c) {
		return
	}
	request := influencerSheetRequest{Model: influencerImageModel, Tier: req.Tier, Selection: req.Selection, Brief: req.Brief, ImageURL: req.ImageURL}
	out, meta, code, err := buildInfluencerSheet(request)
	if err != nil {
		influencerError(c, http.StatusBadRequest, code, err.Error())
		return
	}
	if !prepareInfluencerImageInputs(c, request, out) {
		return
	}
	if !prepareInfluencerSheetRouting(c, influencerImageModel, influencerImageGroup, req.ImageURL != "") {
		return
	}
	path := "/pg/images/generations"
	if req.ImageURL != "" {
		path = "/pg/images/edits"
	}
	acceptInfluencerCLIRequest(c, req.RequestID, payloadHash, influencerImageGroup, path, ActionInfluencerSheet, out, meta)
}

func PostInfluencerCLIMultiView(c *gin.Context) {
	defer discardUnacceptedInfluencerImages(c)
	var req influencerCLIMultiViewRequest
	if !decodeInfluencerCLIRequest(c, &req, "request_id") {
		return
	}
	parentID := strings.TrimSpace(c.Param("task_id"))
	payloadHash, ok := influencerCLIHash(c, req.RequestID, gin.H{"kind": "multiview", "parent_task_id": parentID})
	if !ok || replayInfluencerCLIRequest(c, req.RequestID, payloadHash) || !requireInfluencerCLIWriteAvailability(c) {
		return
	}
	parent, ok := ownedInfluencerCLITask(c, parentID)
	if !ok {
		return
	}
	if !influencerCLIOriginalCharacter(parent) {
		influencerError(c, http.StatusBadRequest, "invalid_parent_character", "multiview requires a completed original character")
		return
	}
	parentMeta, _ := parseInfluencerTaskMeta(parent.Properties.Input)
	if parentMeta.Tier == "" {
		parentMeta.Tier = "normal"
	}
	selection, err := influencer.ValidateSelection(parentMeta.Tier, parentMeta.Selection)
	if err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_parent_character", "the saved character's options are unsupported")
		return
	}
	imageURL, ok := influencerCLIArtifactURL(c, parent, "image")
	if !ok {
		return
	}
	request := influencerSheetRequest{Model: influencerImageModel, Tier: parentMeta.Tier, Selection: selection, Brief: parentMeta.Brief, ImageURL: imageURL, ImageMode: "reference"}
	out, meta, code, err := buildInfluencerSheet(request)
	if err != nil {
		influencerError(c, http.StatusBadRequest, code, err.Error())
		return
	}
	out["prompt"] = influencer.BuildMultiViewPromptForTier(parentMeta.Tier, selection, parentMeta.Brief)
	if !prepareInfluencerImageInputs(c, request, out) {
		return
	}
	if !prepareInfluencerSheetRouting(c, influencerImageModel, influencerImageGroup, true) {
		return
	}
	meta["kind"], meta["parent_task_id"], meta["size"] = influencerTaskKindMultiView, parent.TaskID, influencerMultiViewSize
	if parentMeta.Seed > 0 {
		meta["seed"] = parentMeta.Seed
	}
	acceptInfluencerCLIRequest(c, req.RequestID, payloadHash, influencerImageGroup, "/pg/images/edits", ActionInfluencerSheet, out, meta)
}

func influencerCLIArtifactURL(c *gin.Context, task *model.Task, kind string) (string, bool) {
	if task.Status != model.TaskStatusSuccess {
		influencerError(c, http.StatusBadRequest, "creation_material_unavailable", "reference creation has not completed")
		return "", false
	}
	artifacts, err := influencerCLIArtifacts(task)
	if err != nil {
		influencerError(c, http.StatusServiceUnavailable, "artifact_projection_unavailable", "reference creation artifacts are temporarily unavailable")
		return "", false
	}
	for _, artifact := range artifacts {
		if artifact.Type == kind && artifact.ContentURL != "" {
			return artifact.ContentURL, true
		}
	}
	influencerError(c, http.StatusBadRequest, "creation_material_unavailable", "reference creation has no usable "+kind+" artifact")
	return "", false
}

func PostInfluencerCLIVideo(c *gin.Context) {
	var req influencerCLIVideoRequest
	if !decodeInfluencerCLIRequest(c, &req, "request_id", "mode", "character_task_id", "image_urls", "video_url", "video_task_id", "resolution", "aspect_ratio", "prompt", "duration_sec") {
		return
	}
	req.Mode, req.Resolution = strings.TrimSpace(req.Mode), strings.TrimSpace(req.Resolution)
	if req.Mode == "" {
		req.Mode = "motion"
	}
	if req.Resolution == "" {
		req.Resolution = "480p"
	}
	req.CharacterTaskID, req.VideoTaskID, req.VideoURL, req.Prompt = strings.TrimSpace(req.CharacterTaskID), strings.TrimSpace(req.VideoTaskID), strings.TrimSpace(req.VideoURL), strings.TrimSpace(req.Prompt)
	for i := range req.ImageURLs {
		req.ImageURLs[i] = strings.TrimSpace(req.ImageURLs[i])
		if req.ImageURLs[i] == "" {
			influencerError(c, http.StatusBadRequest, "invalid_request", "image_urls cannot contain empty images")
			return
		}
	}
	if req.AspectRatio != nil {
		value := strings.TrimSpace(*req.AspectRatio)
		req.AspectRatio = &value
	}
	intent := req
	intent.RequestID = ""
	payloadHash, ok := influencerCLIHash(c, req.RequestID, gin.H{"kind": "video", "input": intent})
	if !ok || replayInfluencerCLIRequest(c, req.RequestID, payloadHash) || !requireInfluencerCLIWriteAvailability(c) {
		return
	}
	// fork: viral recreation sends no reference clip to the video model, so the
	// clip is optional here; motion and replace still require exactly one source.
	if req.Mode != influencerMotionModeRecreate {
		if (req.VideoURL == "") == (req.VideoTaskID == "") {
			influencerError(c, http.StatusBadRequest, "invalid_request", "provide exactly one of video_url or video_task_id")
			return
		}
	} else if req.VideoURL != "" || req.VideoTaskID != "" || strings.TrimSpace(req.Prompt) == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "viral recreation takes a compiled prompt and images, not a reference video")
		return
	}
	group, ok := influencerMotionGroups[req.Resolution]
	if !ok {
		influencerError(c, http.StatusBadRequest, "invalid_request", "resolution must be 480p or 720p")
		return
	}
	// All three modes share one generation line and its resolution tier group.
	routeModel, routeGroup := influencerMotionModel, group
	if !influencerCLIRouteAvailable(c, routeModel, routeGroup) {
		influencerError(c, http.StatusServiceUnavailable, "creation_route_unavailable", "a compatible video route is currently unavailable")
		return
	}
	images := append([]string{}, req.ImageURLs...)
	if req.CharacterTaskID != "" {
		character, ok := ownedInfluencerCLITask(c, req.CharacterTaskID)
		if !ok {
			return
		}
		if !influencerCLIOriginalCharacter(character) {
			influencerError(c, http.StatusBadRequest, "invalid_source_character", "character_task_id must reference a completed original character")
			return
		}
		imageURL, ok := influencerCLIArtifactURL(c, character, "image")
		if !ok {
			return
		}
		images = append([]string{imageURL}, images...)
	}
	if req.VideoTaskID != "" {
		video, ok := ownedInfluencerCLITask(c, req.VideoTaskID)
		if !ok {
			return
		}
		if video.Action != ActionInfluencerMotion {
			influencerError(c, http.StatusBadRequest, "invalid_source_video", "video_task_id must reference a completed influencer video")
			return
		}
		var available bool
		req.VideoURL, available = influencerCLIArtifactURL(c, video, "video")
		if !available {
			return
		}
	}
	// The common path inspects/fixes media exactly once and carries all ordered images/audio.
	out, meta, prepared, ok := prepareInfluencerMotionRequest(c, influencerMotionRequest{Mode: req.Mode, Prompt: req.Prompt, SubjectImageURLs: images, SourceTaskID: req.CharacterTaskID, VideoURL: req.VideoURL, Resolution: req.Resolution, AspectRatio: req.AspectRatio, DurationSec: req.DurationSec}, true)
	if !ok {
		return
	}
	accepted := false
	defer func() {
		if !accepted {
			prepared.Discard()
		}
	}()
	// CLI references retain their 4-second minimum independently of web motion inputs.
	if req.Mode != influencerMotionModeRecreate && (prepared.Metadata.Duration < influencerMotionMinSeconds || (prepared.Metadata.VideoDuration != 0 && prepared.Metadata.VideoDuration < influencerMotionMinSeconds)) {
		influencerError(c, http.StatusBadRequest, "reference_video_invalid", "reference video must be between 4 and 30 seconds, including its actual visual duration")
		return
	}
	if req.VideoTaskID != "" {
		meta["video_task_id"] = req.VideoTaskID
	}
	accepted = acceptInfluencerCLIRequest(c, req.RequestID, payloadHash, routeGroup, "/pg/video/generations", ActionInfluencerMotion, out, meta)
}
