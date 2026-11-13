package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/gin-gonic/gin"
)

func attachInfluencerImageInputs(c *gin.Context, task *model.Task) {
	value, exists := c.Get(ctxInfluencerImageInputs)
	inputs, ok := value.(*model.GenerationImageInputs)
	if !exists || !ok || inputs == nil {
		return
	}
	if task.PrivateData.GenerationQueue != nil {
		task.PrivateData.GenerationQueue.ImageInputs = inputs
	} else {
		task.PrivateData.ImageInputs = inputs
	}
}

const (
	influencerImageModel     = "gpt-image-2.5-sunburst"
	influencerImageGroup     = "gpt-image-2.5-sunburst-image"
	influencerImagePixels    = "1024x576"
	ctxInfluencerImageMode   = "influencer_image_mode"
	ctxInfluencerImageInputs = "influencer_image_inputs"
)

// fork: one canonical image contract is shared by all product entry points.
func buildInfluencerSunburstSheet(req influencerSheetRequest) (map[string]any, map[string]any, string, error) {
	if req.Seed != 0 {
		return nil, nil, "invalid_request", errors.New("seed is not supported by the reviewed image contract")
	}
	mode := strings.TrimSpace(req.ImageMode)
	imageURL, maskURL := strings.TrimSpace(req.ImageURL), strings.TrimSpace(req.MaskURL)
	if mode == "" {
		mode = "text"
		if imageURL != "" {
			mode = "reference"
		}
	}
	if mode != "text" && mode != "reference" && mode != "edit" && mode != "masked_edit" {
		return nil, nil, "invalid_request", errors.New("image_mode is unsupported")
	}
	if (mode == "text" && imageURL != "") || (mode != "text" && imageURL == "") || (mode == "masked_edit" && maskURL == "") || (mode != "masked_edit" && maskURL != "") {
		return nil, nil, "invalid_request", errors.New("image inputs do not match image_mode")
	}
	if (req.Size != "" && req.Size != "16:9" && req.Size != influencerImagePixels) || (req.Quality != "" && req.Quality != "high") || (req.OutputFormat != "" && req.OutputFormat != "png") || (req.ResponseFormat != "" && req.ResponseFormat != "url") || (req.N != nil && *req.N != 1) {
		return nil, nil, "invalid_request", errors.New("the reviewed image specification is high quality, 1024x576, one PNG")
	}
	if utf8.RuneCountInString(req.Brief) > 4000 || utf8.RuneCountInString(req.EditInstruction) > 4000 {
		return nil, nil, "invalid_request", errors.New("image instruction exceeds 4000 characters")
	}
	tier := req.Tier
	if tier == "" {
		tier = "normal"
	}
	selection, err := influencer.ValidateSelection(tier, req.Selection)
	if err != nil {
		return nil, nil, "invalid_selection", err
	}
	prompt := ""
	if mode == "edit" || mode == "masked_edit" {
		prompt = strings.TrimSpace(req.EditInstruction)
		if prompt == "" {
			return nil, nil, "invalid_request", errors.New("edit_instruction is required")
		}
	} else {
		if strings.TrimSpace(req.EditInstruction) != "" {
			return nil, nil, "invalid_request", errors.New("edit_instruction requires an edit mode")
		}
		if len(selection) == 0 && strings.TrimSpace(req.Brief) == "" && imageURL == "" && !influencer.IsAnimalTier(tier) {
			return nil, nil, "invalid_request", errors.New("pick at least one option or provide a brief")
		}
		prompt = influencer.BuildPrompt(tier, selection, req.Brief)
		if mode == "reference" {
			prompt = influencer.BuildReferencePrompt(tier, selection, req.Brief)
		}
	}
	if err := helper.ValidateGPTImagePrompt(influencerImageModel, prompt); err != nil {
		return nil, nil, "invalid_request", err
	}
	out := map[string]any{"model": influencerImageModel, "prompt": prompt, "image_mode": mode, "size": influencerImagePixels, "quality": "high", "n": 1, "output_format": "png", "response_format": "url"}
	meta := map[string]any{"tier": tier, "selection": selection, "brief": req.Brief, "size": "16:9", "image_mode": mode}
	if imageURL != "" {
		out["image"], meta["image_url"] = imageURL, imageURL
	}
	if maskURL != "" {
		out["mask_url"], meta["mask_url"] = maskURL, maskURL
	}
	if req.EditInstruction != "" {
		meta["edit_instruction"] = req.EditInstruction
	}
	return out, meta, "", nil
}

func prepareInfluencerImageInputs(c *gin.Context, req influencerSheetRequest, out map[string]any) bool {
	if req.Model != influencerImageModel {
		return true
	}
	size, err := helper.ImageTaskRequestSize(out)
	if err != nil {
		influencerError(c, 400, "invalid_request", err.Error())
		return false
	}
	if existing := c.GetString(helper.ImageTaskRequestSizeContextKey); existing != "" && existing != size {
		influencerError(c, 400, "invalid_request", "image preparation does not match the request")
		return false
	}
	c.Set(helper.ImageTaskRequestSizeContextKey, size)
	mode, _ := out["image_mode"].(string)
	c.Set(ctxInfluencerImageMode, mode)
	if existing, ok := c.Get(ctxInfluencerImageInputs); ok {
		inputs, valid := existing.(*model.GenerationImageInputs)
		if !valid || inputs == nil || inputs.Mode != mode {
			influencerError(c, 400, "invalid_request", "image preparation does not match the request")
			return false
		}
		applyInfluencerImageInputs(c, out, inputs)
		return true
	}
	inputs := &model.GenerationImageInputs{Version: 1, Mode: mode}
	prepared := make([]*service.PreparedInfluencerImage, 0, 2)
	for _, field := range []string{"image", "mask_url"} {
		raw, _ := out[field].(string)
		if raw == "" {
			continue
		}
		expiry, err := service.ResolveInfluencerImageSourceExpiry(c.Request.Context(), c.GetInt("id"), raw)
		if err != nil {
			for _, item := range prepared {
				item.Discard()
			}
			influencerError(c, http.StatusBadRequest, "invalid_reference_image", err.Error())
			return false
		}
		image, err := service.FreezeInfluencerImage(c.Request.Context(), c.GetInt("id"), raw, expiry, influencerReferenceHTTPClient)
		if err != nil {
			for _, item := range prepared {
				item.Discard()
			}
			influencerError(c, http.StatusBadRequest, "invalid_reference_image", err.Error())
			return false
		}
		prepared = append(prepared, &image)
		if field == "image" {
			inputs.Source = &image.InfluencerFrozenImage
		} else {
			inputs.Mask = &image.InfluencerFrozenImage
		}
	}
	if mode == "masked_edit" {
		if inputs.Source == nil || inputs.Mask == nil {
			for _, item := range prepared {
				item.Discard()
			}
			influencerError(c, 400, "invalid_reference_image", "source and mask are required")
			return false
		}
		if err := service.ValidateInfluencerFrozenImageMask(c.Request.Context(), c.GetInt("id"), *inputs.Source, *inputs.Mask); err != nil {
			for _, item := range prepared {
				item.Discard()
			}
			influencerError(c, 400, "invalid_reference_image", err.Error())
			return false
		}
	}
	c.Set(ctxInfluencerImageInputs, inputs)
	c.Set("influencer_image_prepared_files", prepared)
	applyInfluencerImageInputs(c, out, inputs)
	return true
}

func applyInfluencerImageInputs(c *gin.Context, out map[string]any, inputs *model.GenerationImageInputs) {
	if inputs.Source != nil {
		out["image"] = inputs.Source.URL
		c.Set("influencer_reference_preflight", inputs.Source.URL)
		c.Set("influencer_frozen_image", *inputs.Source)
	}
	if inputs.Mask != nil {
		out["mask_url"] = inputs.Mask.URL
		c.Set("influencer_frozen_mask", *inputs.Mask)
	}
}

func discardUnacceptedInfluencerImages(c *gin.Context) {
	if c.GetBool("influencer_image_inputs_retained") || c.GetBool("generation_queue_attempt_started") {
		return
	}
	if value, ok := c.Get("influencer_image_prepared_files"); ok {
		if files, valid := value.([]*service.PreparedInfluencerImage); valid {
			for _, image := range files {
				image.Discard()
			}
		}
	}
}

func validateInfluencerImageInputs(c *gin.Context, request *model.GenerationQueueRequest, body map[string]any) error {
	inputs := request.ImageInputs
	size, err := helper.ImageTaskRequestSize(body)
	if err != nil || inputs == nil || inputs.Version != 1 || body["image_mode"] != inputs.Mode {
		return errors.New("image request is outside its frozen contract")
	}
	for _, key := range []string{"images", "image_urls", "input_reference"} {
		if _, exists := body[key]; exists {
			return errors.New("additional image inputs are unsupported")
		}
	}
	_, hasSource := body["image"]
	_, hasMask := body["mask_url"]
	if inputs.Mode != "text" && inputs.Mode != "reference" && inputs.Mode != "edit" && inputs.Mode != "masked_edit" {
		return errors.New("frozen image mode is invalid")
	}
	if (inputs.Mode == "text" && (hasSource || inputs.Source != nil)) || (inputs.Mode != "text" && (!hasSource || inputs.Source == nil)) || (inputs.Mode == "masked_edit" && (!hasMask || inputs.Mask == nil)) || (inputs.Mode != "masked_edit" && (hasMask || inputs.Mask != nil)) {
		return errors.New("frozen image inputs do not match mode")
	}
	for _, item := range []struct {
		key string
		ref *model.GenerationImageReference
	}{{"image", inputs.Source}, {"mask_url", inputs.Mask}} {
		if item.ref == nil {
			continue
		}
		if body[item.key] != item.ref.URL {
			return errors.New("frozen image identity changed")
		}
		if _, err := service.ReadInfluencerFrozenImage(c.Request.Context(), c.GetInt("id"), *item.ref); err != nil {
			return fmt.Errorf("frozen image is unavailable: %w", err)
		}
	}
	if inputs.Source != nil && request.ReferencePreflight != inputs.Source.URL {
		return errors.New("frozen reference preflight changed")
	}
	if inputs.Source == nil && request.ReferencePreflight != "" {
		return errors.New("unexpected reference preflight")
	}
	if inputs.Mode == "masked_edit" {
		if err := service.ValidateInfluencerFrozenImageMask(c.Request.Context(), c.GetInt("id"), *inputs.Source, *inputs.Mask); err != nil {
			return err
		}
	}
	c.Set(ctxInfluencerImageInputs, inputs)
	c.Set(ctxInfluencerImageMode, inputs.Mode)
	c.Set(helper.ImageTaskRequestSizeContextKey, size)
	applyInfluencerImageInputs(c, body, inputs)
	return nil
}
