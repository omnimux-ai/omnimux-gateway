package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/influencer"

	"github.com/gin-gonic/gin"
)

// ActionInfluencerSheet marks image tasks submitted through the influencer
// workbench so history/recreate can distinguish them.
const ActionInfluencerSheet = "influencer_sheet"

// influencerSheetRequest is the structured payload the workbench posts.
type influencerSheetRequest struct {
	Model           string               `json:"model"`
	Group           string               `json:"group"`
	Tier            string               `json:"tier"`
	Selection       influencer.Selection `json:"selection"`
	Brief           string               `json:"brief"`
	Seed            int64                `json:"seed"`
	Size            string               `json:"size"`
	ImageURL        string               `json:"image_url"`
	ImageMode       string               `json:"image_mode"`
	MaskURL         string               `json:"mask_url"`
	EditInstruction string               `json:"edit_instruction"`
	Quality         string               `json:"quality"`
	OutputFormat    string               `json:"output_format"`
	ResponseFormat  string               `json:"response_format"`
	N               *int                 `json:"n"`
	Extras          map[string]any       `json:"-"`
}

func influencerError(c *gin.Context, status int, code, message string) {
	if c.GetBool("influencer_creation_api") {
		c.JSON(status, gin.H{"success": false, "code": code, "message": message})
		return
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    code,
		},
	})
}

// GetInfluencerTaxonomy returns the embedded option dataset (18 categories x
// 170 options x 3 tiers) so the builder renders entirely from server data.
func GetInfluencerTaxonomy(c *gin.Context) {
	tax := influencer.Taxonomy()
	c.Header("Cache-Control", "private, max-age=300")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tax,
		"etag":    influencer.ETag(),
	})
}

// influencerCaptureWriter buffers the downstream response so the task id can
// be recovered for the post-submit properties patch without changing the
// shared relay code path.
type influencerCaptureWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *influencerCaptureWriter) Write(p []byte) (int, error) {
	w.body = append(w.body, p...)
	return len(p), nil
}

func (w *influencerCaptureWriter) WriteString(s string) (int, error) {
	w.body = append(w.body, s...)
	return len(s), nil
}

func (w *influencerCaptureWriter) flush() {
	if len(w.body) > 0 {
		_, _ = w.ResponseWriter.Write(w.body)
	}
}

// PostInfluencerSheet validates a character-sheet payload, assembles the
// prompt server-side, rewrites the request body into the shared task
// submission shape and hands off to the standard RelayTask chain — so billing
// (precharge/settle/refund), task queueing and result proxying stay native.
func PostInfluencerSheet(c *gin.Context) {
	var req influencerSheetRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "model is required")
		return
	}
	modelName, suffixGroup := influencerModelGroup(req.Model)
	req.Model = modelName
	if _, ok := influencerSheetGroup(c, req.Group, suffixGroup); !ok {
		return
	}
	out, meta, code, err := buildInfluencerSheet(req)
	if err != nil {
		influencerError(c, http.StatusBadRequest, code, err.Error())
		return
	}
	if !prepareInfluencerImageInputs(c, req, out) {
		return
	}
	hasImage := strings.TrimSpace(req.ImageURL) != ""
	if hasImage {
		if req.Model != influencerImageModel && c.GetString("influencer_reference_preflight") != strings.TrimSpace(req.ImageURL) {
			PrepareInfluencerSheet(c)
			if c.IsAborted() {
				return
			}
		}
		if _, valid := influencerReferenceSelectedChannel(c); !valid {
			influencerError(c, http.StatusServiceUnavailable, "reference_route_unavailable", "reference image route is unavailable")
			return
		}
	}
	prepared, _ := c.Get(ctxInfluencerRouting)
	if routing, ok := prepared.(*model.GenerationRoutingPolicy); ok && routing != nil && routing.Mode == model.GenerationRoutingAuto {
		// Preserve the workbench's established pixels across suppliers. Its
		// public composition remains 16:9; provider ratio defaults differ.
		if req.Model != influencerImageModel {
			out["size"] = influencerImageLandscapePixels
		}
	}

	influencerTaskSubmit(c, out, hasImage, meta)
}

// buildInfluencerSheet is the shared product normalization for the workbench and CLI.
func buildInfluencerSheet(req influencerSheetRequest) (map[string]any, map[string]any, string, error) {
	if strings.TrimSpace(req.Model) == influencerImageModel {
		return buildInfluencerSunburstSheet(req)
	}
	if req.ImageMode != "" || req.MaskURL != "" || req.EditInstruction != "" {
		return nil, nil, "invalid_request", fmt.Errorf("image editing requires the reviewed Sunburst route")
	}
	if utf8.RuneCountInString(req.Brief) > 4000 {
		return nil, nil, "invalid_request", fmt.Errorf("brief exceeds 4000 characters")
	}
	selection, verr := influencer.ValidateSelection(req.Tier, req.Selection)
	if verr != nil {
		return nil, nil, "invalid_selection", verr
	}
	hasImage := strings.TrimSpace(req.ImageURL) != ""
	if len(selection) == 0 && strings.TrimSpace(req.Brief) == "" && !hasImage && !influencer.IsAnimalTier(req.Tier) {
		return nil, nil, "invalid_request", fmt.Errorf("pick at least one option or provide a brief")
	}

	prompt := influencer.BuildPrompt(req.Tier, selection, req.Brief)
	if hasImage {
		prompt = influencer.BuildReferencePrompt(req.Tier, selection, req.Brief)
	}

	// Rebuild the wire body for the shared task chain. The path is rewritten
	// to the canonical image endpoints so platform adaptors pick generations
	// vs edits (reference image) exactly like the regular playground submit.
	out := map[string]any{
		"model":  strings.TrimSpace(req.Model),
		"prompt": prompt,
	}
	// fork: new character sheets use the fixed landscape dual-view layout.
	// Accept legacy Size values without letting them override this composition.
	size := "16:9"
	out["size"] = size
	if hasImage {
		out["image"] = strings.TrimSpace(req.ImageURL)
	}
	if req.Seed > 0 {
		out["metadata"] = map[string]any{"seed": req.Seed}
	}

	// snapshot of what to persist on the task record
	meta := map[string]any{
		"tier":      req.Tier,
		"selection": selection,
		"brief":     req.Brief,
		"size":      size,
	}
	if req.Seed > 0 {
		meta["seed"] = req.Seed
	}
	if hasImage {
		meta["image_url"] = strings.TrimSpace(req.ImageURL)
	}

	return out, meta, "", nil
}

// patchInfluencerTask writes the structured sheet params into the created
// task's properties.input and tags it with the influencer action so history
// can filter and Recreate can rehydrate the form.
func patchInfluencerTask(c *gin.Context, respBody []byte, metaJSON []byte, action string) {
	if c.Writer.Status() < 200 || c.Writer.Status() >= 300 {
		return
	}
	var resp struct {
		TaskID string `json:"task_id"`
		ID     string `json:"id"`
		Data   struct {
			TaskID string `json:"task_id"`
			ID     string `json:"id"`
		} `json:"data"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return
	}
	taskID := resp.TaskID
	if taskID == "" {
		taskID = resp.ID
	}
	if taskID == "" {
		taskID = resp.Data.TaskID
	}
	if taskID == "" {
		taskID = resp.Data.ID
	}
	if taskID == "" {
		return
	}
	var t model.Task
	if err := model.DB.Where("task_id = ? AND user_id = ?", taskID, c.GetInt("id")).First(&t).Error; err != nil {
		common.SysError("influencer task patch: task not found " + taskID)
		return
	}
	t.Properties.Input = string(metaJSON)
	t.Action = action
	if err := t.Update(); err != nil {
		common.SysError("influencer task patch failed: " + err.Error())
	}
}

const (
	// influencerTaskKindMultiView marks a task as a turnaround sheet derived
	// from another task. A row without a kind is a plain character sheet, which
	// keeps rows written before this action existed readable.
	influencerTaskKindMultiView = "multiview"
	// influencerMultiViewSize is the panel aspect the turnaround layout needs.
	influencerMultiViewSize        = "16:9"
	influencerImageLandscapePixels = "1920x1088"
)

// influencerTaskMeta is the structured payload the workbench persists in a
// task's properties.input.
type influencerTaskMeta struct {
	Kind            string               `json:"kind"`
	Tier            string               `json:"tier"`
	Selection       influencer.Selection `json:"selection"`
	Brief           string               `json:"brief"`
	Seed            int64                `json:"seed"`
	Size            string               `json:"size"`
	ImageURL        string               `json:"image_url"`
	ImageMode       string               `json:"image_mode,omitempty"`
	MaskURL         string               `json:"mask_url,omitempty"`
	EditInstruction string               `json:"edit_instruction,omitempty"`
	Parent          string               `json:"parent_task_id"`
}

// parseInfluencerTaskMeta reads a persisted payload. It only reports whether the
// stored JSON parsed; callers decide what a usable payload must contain.
func parseInfluencerTaskMeta(raw string) (influencerTaskMeta, bool) {
	var meta influencerTaskMeta
	if strings.TrimSpace(raw) == "" {
		return meta, false
	}
	var fields map[string]any
	if err := common.Unmarshal([]byte(raw), &fields); err != nil || len(fields) == 0 {
		return meta, false
	}
	if value, exists := fields["tier"]; exists {
		tier, ok := value.(string)
		if !ok || strings.TrimSpace(tier) == "" {
			return meta, false
		}
	}
	if value, exists := fields["selection"]; exists && value == nil {
		return meta, false
	}
	if err := common.Unmarshal([]byte(raw), &meta); err != nil {
		return meta, false
	}
	if meta.Tier == "" && meta.Kind != influencerTaskKindMultiView && len(meta.Selection) == 0 && strings.TrimSpace(meta.Brief) == "" && strings.TrimSpace(meta.ImageURL) == "" {
		return meta, false
	}
	return meta, true
}

// influencerMultiViewRequest references an existing sheet task instead of
// carrying form state, so the character description is always the persisted one.
type influencerMultiViewRequest struct {
	Model        string `json:"model"`
	Group        string `json:"group"`
	ParentTaskID string `json:"parent_task_id"`
	ImageURL     string `json:"image_url"`
	Size         string `json:"size"`
}

// PostInfluencerMultiView starts a turnaround sheet derived from an existing
// character sheet. The parent task is the authoritative source of the character
// description — the caller supplies only the parent id and the reference image —
// so this endpoint cannot be used to submit an arbitrary prompt on the billed
// task chain, and the reference image must be the parent's own artifact.
func PostInfluencerMultiView(c *gin.Context) {
	var req influencerMultiViewRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "model is required")
		return
	}
	parentTaskID := strings.TrimSpace(req.ParentTaskID)
	if parentTaskID == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "parent_task_id is required")
		return
	}
	imageURL := strings.TrimSpace(req.ImageURL)
	if imageURL == "" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "image_url is required")
		return
	}

	var parent model.Task
	if err := model.DB.Where("task_id = ? AND user_id = ?", parentTaskID, c.GetInt("id")).First(&parent).Error; err != nil {
		influencerError(c, http.StatusNotFound, "task_not_found", "parent task not found")
		return
	}
	if parent.Action != ActionInfluencerSheet {
		influencerError(c, http.StatusBadRequest, "invalid_request", "parent task is not a character sheet")
		return
	}
	parentMeta, ok := parseInfluencerTaskMeta(parent.Properties.Input)
	if !ok {
		influencerError(c, http.StatusBadRequest, "invalid_request", "parent task carries no sheet parameters")
		return
	}
	if parentMeta.Kind == influencerTaskKindMultiView {
		influencerError(c, http.StatusBadRequest, "invalid_request", "a multi-view board cannot be the parent of another")
		return
	}
	if parentMeta.ImageMode == "edit" || parentMeta.ImageMode == "masked_edit" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "a multi-view board requires an original character sheet")
		return
	}
	if parent.Status != model.TaskStatusSuccess {
		influencerError(c, http.StatusBadRequest, "invalid_request", "parent task has not finished")
		return
	}
	if parentMeta.Tier == "" {
		parentMeta.Tier = "normal"
	}
	if !influencer.ValidTier(parentMeta.Tier) {
		influencerError(c, http.StatusBadRequest, "invalid_selection", "parent character type is unsupported")
		return
	}
	if influencer.IsAnimalTier(parentMeta.Tier) {
		selection, err := influencer.ValidateSelection(parentMeta.Tier, parentMeta.Selection)
		if err != nil {
			influencerError(c, http.StatusBadRequest, "invalid_selection", err.Error())
			return
		}
		parentMeta.Selection = selection
	}
	if len(parentMeta.Selection) == 0 && strings.TrimSpace(parentMeta.Brief) == "" && strings.TrimSpace(parentMeta.ImageURL) == "" && !influencer.IsAnimalTier(parentMeta.Tier) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "parent task carries no sheet parameters")
		return
	}
	onlyImageParent := len(parentMeta.Selection) == 0 && strings.TrimSpace(parentMeta.Brief) == "" && strings.TrimSpace(parentMeta.ImageURL) != ""
	prepared, _ := c.Get(ctxInfluencerRouting)
	routing, _ := prepared.(*model.GenerationRoutingPolicy)
	autoReference := routing != nil && routing.Mode == model.GenerationRoutingAuto
	if onlyImageParent || autoReference {
		if !influencerParentImageArtifact(&parent, imageURL) {
			influencerError(c, http.StatusBadRequest, "invalid_request", "image_url must be a valid image artifact of the parent task")
			return
		}
		var suffixGroup string
		modelName, suffixGroup = influencerModelGroup(modelName)
		group, ok := influencerSheetGroup(c, req.Group, suffixGroup)
		if !ok {
			return
		}
		if !autoReference && group != common.GetContextKeyString(c, constant.ContextKeyUsingGroup) {
			influencerError(c, http.StatusBadRequest, "group_conflict", "requested and distributed groups must agree")
			return
		}
		if (modelName != "gpt-image-2.5" || group != influencerReferenceGroup) && (modelName != influencerImageModel || group != influencerImageGroup) {
			influencerError(c, http.StatusBadRequest, "reference_group_unsupported", "reference image requires the reviewed economy image route")
			return
		}
		if modelName == influencerImageModel {
			request := influencerSheetRequest{Model: modelName, Tier: parentMeta.Tier, Selection: parentMeta.Selection, Brief: parentMeta.Brief, ImageURL: imageURL, ImageMode: "reference"}
			prepared, _, code, err := buildInfluencerSheet(request)
			if err != nil {
				influencerError(c, 400, code, err.Error())
				return
			}
			if !prepareInfluencerImageInputs(c, request, prepared) {
				return
			}
		} else {
			if err := service.ValidateInfluencerReference(c.Request.Context(), imageURL, influencerReferenceHTTPClient); err != nil {
				influencerError(c, http.StatusBadRequest, "invalid_reference_image", err.Error())
				return
			}
			c.Set("influencer_reference_preflight", imageURL)
		}
		if _, valid := influencerReferenceSelectedChannel(c); !valid {
			influencerError(c, http.StatusServiceUnavailable, "reference_route_unavailable", "reference image route is unavailable")
			return
		}
	} else if influencer.IsAnimalTier(parentMeta.Tier) && !influencerParentImageArtifact(&parent, imageURL) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "image_url must be a valid image artifact of the parent task")
		return
	} else if !influencer.IsAnimalTier(parentMeta.Tier) && !strings.Contains(imageURL, parentArtifactPath(parentTaskID)) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "image_url must be an artifact of the parent task")
		return
	}

	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = influencerMultiViewSize
	}
	wireSize := size
	if autoReference {
		if size != influencerMultiViewSize {
			influencerError(c, http.StatusBadRequest, "invalid_request", "size must be 16:9")
			return
		}
		wireSize = influencerImageLandscapePixels
	}

	out := map[string]any{
		"model":  modelName,
		"prompt": influencer.BuildMultiViewPromptForTier(parentMeta.Tier, parentMeta.Selection, parentMeta.Brief),
		"image":  imageURL,
		"size":   wireSize,
	}
	if modelName == influencerImageModel {
		out["image_mode"], out["size"], out["quality"], out["n"], out["output_format"], out["response_format"] = "reference", influencerImagePixels, "high", 1, "png", "url"
		if inputs, ok := c.Get(ctxInfluencerImageInputs); ok {
			applyInfluencerImageInputs(c, out, inputs.(*model.GenerationImageInputs))
		}
	}
	if group := strings.TrimSpace(req.Group); group != "" && !onlyImageParent && !autoReference {
		out["group"] = group
	}

	meta := map[string]any{
		"kind":           influencerTaskKindMultiView,
		"parent_task_id": parentTaskID,
		"size":           size,
		"image_url":      imageURL,
	}
	meta["tier"] = parentMeta.Tier
	if modelName == influencerImageModel {
		meta["image_mode"] = "reference"
	}
	if parentMeta.Selection == nil {
		parentMeta.Selection = influencer.Selection{}
	}
	meta["selection"] = parentMeta.Selection
	meta["brief"] = parentMeta.Brief
	if parentMeta.Seed > 0 {
		meta["seed"] = parentMeta.Seed
	}
	influencerTaskSubmit(c, out, true, meta)
}

// parentArtifactPath is the public artifact route a reference image must come
// from. Pinning it keeps a derived action bound to its own parent render.
func parentArtifactPath(taskID string) string {
	return "/v1/tasks/" + taskID + "/artifacts/"
}

// influencerTaskSubmit rewrites the request into the canonical image-submit
// shape, hands off to the native RelayTask chain, then tags the created task
// with the structured payload the workbench reads back.
func influencerTaskSubmit(c *gin.Context, out map[string]any, hasImage bool, meta map[string]any) {
	path := "/pg/images/generations"
	if hasImage {
		path = "/pg/images/edits"
	}
	influencerRelaySubmit(c, path, ActionInfluencerSheet, out, meta)
}

// influencerRelaySubmit rewrites the request body to out, points it at the
// canonical playground path, runs the native task chain and then tags the
// created task with action and the workbench payload.
func influencerRelaySubmit(c *gin.Context, path, action string, out map[string]any, meta map[string]any) {
	if err := influencerRewriteBody(c, out); err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to build upstream request")
		return
	}
	c.Request.URL.Path = path
	if enqueueInfluencerGeneration(c, path, action, out, meta) {
		return
	}

	metaJSON, _ := common.Marshal(meta)

	capture := &influencerCaptureWriter{ResponseWriter: c.Writer}
	c.Writer = capture
	defer func() {
		capture.flush()
	}()

	if c.GetString("influencer_reference_preflight") != "" {
		submitInfluencerReferenceTask(c)
	} else {
		RelayTaskPluginEndpoint(c, RelayTask)
	}

	patchInfluencerTask(c, capture.body, metaJSON, action)
}

// influencerRewriteBody replaces the request body with out as JSON, so every
// later reader (distribution, the task chain) sees the rewritten request.
func influencerRewriteBody(c *gin.Context, out map[string]any) error {
	outBody, err := common.Marshal(out)
	if err != nil {
		return err
	}
	if storage, sErr := common.CreateBodyStorage(outBody); sErr == nil {
		c.Set(common.KeyBodyStorage, storage)
		c.Request.Body = io.NopCloser(storage)
	} else {
		c.Request.Body = io.NopCloser(strings.NewReader(string(outBody)))
	}
	c.Request.ContentLength = int64(len(outBody))
	c.Request.Header.Set("Content-Type", "application/json")
	return nil
}
