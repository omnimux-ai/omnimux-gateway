package controller

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// fork: influencer motion video — the workbench's video surface. The handler
// validates the form, picks the price tier group from the resolution, cuts the
// reference clip's sound into a reference audio, rewrites the body to the
// canonical video submit shape and re-enters the native task chain, so billing,
// queueing and the result proxy stay native.

// ActionInfluencerMotion marks video tasks submitted from the workbench.
const ActionInfluencerMotion = "influencer_motion"

const (
	influencerMotionModel        = "seedance-2-5"
	influencerMotionAspectRatio  = "9:16"
	influencerMotionMinSeconds   = 4
	influencerMotionMaxSeconds   = 30
	influencerMotionMaxPromptLen = 4000
	influencerMotionMaxImages    = 30
	influencerMotionMaxTagLen    = 20
	influencerMotionKind         = "motion"
	// fork: viral recreation — the reference clip is decomposition material only and
	// never enters the submit body; the caller sends images plus a compiled prompt.
	influencerMotionModeRecreate = "recreate"
	// The tier group travels in the X-Omnimux-Group header on this route and is
	// also written into the request body for the native chain. It must not be
	// forwarded to the provider: some upstreams reject an unknown top-level
	// "group" field with "Unsupported request field".
	influencerMotionGroupField = "group"
)

// influencerMotionGroups maps the output resolution to the price tier group the
// upstream line is bound to. A request always carries one of these groups.
var influencerMotionGroups = map[string]string{
	"480p": "seedance-2-5-480p",
	"720p": "seedance-2-5-720p",
}

// influencerMotionOutputRatios is the output framing the bound upstream lines
// are evidenced to accept per resolution. It is an output contract only: it
// says nothing about which aspect ratio a reference video may carry. The
// verified 2026-10-12 supplier catalogs declare 16:9/9:16/3:4 on the 480p
// stable line and only 16:9/9:16 on the 720p J1 line, so 720p 3:4 is not
// offered even though the generic vendor model lists it. Other ordinary output
// choices stay out until verified, including automatic source-preserving edits.
var influencerMotionOutputRatios = map[string][]string{
	"480p": {"16:9", "9:16", "3:4"},
	"720p": {"16:9", "9:16"},
}

// influencerMotionPrompts frames the user's instruction for each mode. Motion
// uses image 1 as its subject; object swap lets the video model assign image roles.
//
// fork: each framing carries three things the bare instruction left implicit.
// The motion framing names the exact part of the clip to reuse (movement,
// gestures, timing and camera movement) and then describes the setting
// positively, because the video endpoints only document negative phrasing for
// subtitles and audio. influencerMotionScene must hold for any clip, so it
// names no clip-specific objects (windows, seats, ...) and states principles
// instead: the structure of the shot and every object around the person stay
// as they are, because those objects usually carry the clip's joke or hook,
// and only surface design varies. Light and time of day stay too, and nothing
// new may compete with the character for attention. Asking for "a different
// place" with new lighting and window views made the engine drop the props
// that carried the joke and invent sunsets and onlookers. Edit trigger words
// (replace/modify/change/add/remove/delete/edit) stay out of motion framings,
// since the upstream treats them as a video edit that locks the source frames.
// The replace framing preserves everything outside the chosen edit targets.
// No framing ships a separate audio reference: the frozen video input carries
// its own track, so every framing must state the sound contract explicitly.
const influencerMotionScene = "The setting is a lightly redesigned version of the place in reference video 1, so a viewer instantly recognises the same situation. Keep the same type of location, the same spatial layout, the same shot size, camera angle and camera height, the same light direction, brightness and time of day, and every object around the person in the same number, position, size and distance, above all anything the person touches, leans on, is squeezed or blocked by, or is framed by, because those objects carry the joke or hook of the clip. If other people appear in the background, keep the same number of them in the same places doing the same things, with different faces, hair and clothes. Vary only surface design: colours, materials, fabric patterns, finishes and small decorative details, so the place reads as a different instance of the same setting. Keep the set free of any people, objects, light sources or scenery beyond those in reference video 1, so the main character stays the only focal point. No captions, on-screen text, logos or watermarks."

// influencerMotionAudioRule keeps the generated sound anchored to the frozen
// reference. No separate audio reference is submitted: the provider video input
// already carries its own audio track, so the instruction alone defines the
// audio contract for every motion framing. It avoids upstream edit trigger
// words so the source frames stay unlocked.
const influencerMotionAudioRule = " Keep the audio consistent with reference video 1: preserve its original sound and its synchronisation with the picture exactly, and keep every sound as it is."

var influencerMotionPrompts = map[string]string{
	"motion":  "Reference image 1 is the main character. Use reference video 1 only as a motion reference: follow its full-body movement, gestures, timing and camera movement in the same order. " + influencerMotionScene + " Keep the person's face, hair, body and outfit, including footwear, identical to reference image 1. Natural fabric movement, believable weight and contact, sharp focus on the subject." + influencerMotionAudioRule,
	"replace": "编辑视频1。综合全部参考图片的核心内容及其与原片的语义、功能和交互关系，自主确定合理的替换目标与范围。图片顺序不预设用途，用户补充的明确意图优先。\n\n参考图片定义替换后目标的完整外观与可辨识特征；原视频定义事件进程、动作时序、空间关系和镜头组织。按确定的对应关系完整实现替换，消除与参考目标冲突的原有特征，不引入图片中无关内容。\n\n只修改完成替换所必需的部分，其余画面与原音沿用原片。在保持目标定义的前提下，允许自然融合所需的透视、遮挡、接触、光影和运动响应调整，保持原有交互逻辑与全片连续性。\n\n多张图片可互补同一目标或分别对应不同目标，对应关系贯穿全片稳定一致；不重复增添或混合冲突形象，缺乏合理对应的内容不强行加入。\n\n声音与参考视频1保持一致：保留视频1原音及其与画面的同步，不新增、不替换、不重写任何声音。",
}

// influencerCharacterMotionPrompt adapts the primary reference without guessing uploaded species.
func influencerCharacterMotionPrompt(mode, characterType string) string {
	if mode == "replace" {
		return influencerMotionPrompts[mode]
	}
	if characterType == "human" {
		return influencerMotionPrompts[mode]
	}
	if characterType == "cat" || characterType == "dog" {
		subject := "the anthropomorphic " + characterType + " from reference image 1"
		identity := " Keep the same " + characterType + " identity, animal face, muzzle, ears, fur colour and markings, tail, paws, upright two-legged body and outfit as reference image 1. Adapt the gestures to its paws and preserve its animal anatomy rather than turning it into a human or a four-legged pet. Use believable fur and fabric movement, weight and grounded contact."
		return "Animate " + subject + ". Use reference video 1 only as a motion reference: follow its body movement, gestures, timing and camera movement in the same order. " + influencerMotionScene + identity + influencerMotionAudioRule
	}
	return "Animate the character from reference image 1. Use reference video 1 only as a motion reference: follow its movement, gestures, timing and camera movement in the same order. " + influencerMotionScene + " Preserve the character's existing identity, species, body structure, colours and outfit; adapt the movement to that structure rather than altering its species. Keep believable weight, material movement and grounded contact." + influencerMotionAudioRule
}

type influencerMotionRequest struct {
	Mode                 string                           `json:"mode"`
	Prompt               string                           `json:"prompt"`
	SubjectImageURL      string                           `json:"subject_image_url"`
	SubjectImageURLs     []string                         `json:"subject_image_urls"`
	SubjectImageTags     []string                         `json:"subject_image_tags"`
	SubjectCharacterType string                           `json:"subject_character_type"`
	SourceTaskID         string                           `json:"source_task_id"`
	VideoURL             string                           `json:"video_url"`
	DurationSec          int                              `json:"duration_sec"`
	Resolution           string                           `json:"resolution"`
	AspectRatio          *string                          `json:"aspect_ratio,omitempty"`
	VideoMetadata        *service.InfluencerVideoMetadata `json:"video_metadata,omitempty"`
}

const (
	ctxInfluencerMotionOut        = "influencer_motion_out"
	ctxInfluencerMotionMeta       = "influencer_motion_meta"
	ctxInfluencerMotionSubmitting = "influencer_motion_submitting"
)

// PrepareInfluencerMotion validates the form and rewrites the body to the
// canonical video submit shape before channel distribution, which reads the
// model and tier group from the body. A rejected form never reaches billing.
func PrepareInfluencerMotion(c *gin.Context) {
	var req influencerMotionRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerAbort(c, "invalid JSON body")
		return
	}
	out, meta, prepared, ok := prepareInfluencerMotionRequest(c, req, false)
	if !ok {
		return
	}
	defer func() {
		if !c.GetBool(ctxInfluencerMotionSubmitting) {
			prepared.Discard()
		}
	}()
	if err := influencerRewriteBody(c, out); err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to build upstream request")
		c.Abort()
		return
	}
	c.Request.Header.Set("X-Omnimux-Group", out["group"].(string))
	c.Request.Header.Del("New-Api-Group")
	c.Set(ctxInfluencerMotionOut, out)
	c.Set(ctxInfluencerMotionMeta, meta)
	c.Next()
}

// prepareInfluencerMotionRequest keeps the product API compatibility path while
// the workbench follows browser-declared reference metadata without server inspection.
func prepareInfluencerMotionRequest(c *gin.Context, req influencerMotionRequest, inferMetadata bool) (map[string]any, map[string]any, *service.PreparedInfluencerVideo, bool) {
	_, ok := influencerMotionPrompts[req.Mode]
	if !ok && req.Mode != influencerMotionModeRecreate {
		influencerAbort(c, "mode must be motion, replace or recreate")
		return nil, nil, nil, false
	}
	characterType := strings.TrimSpace(req.SubjectCharacterType)
	switch characterType {
	case "", "human", "cat", "dog":
	default:
		influencerAbort(c, "subject_character_type must be human, cat or dog")
		return nil, nil, nil, false
	}
	resolution := strings.TrimSpace(req.Resolution)
	group, ok := influencerMotionGroups[resolution]
	if !ok {
		influencerAbort(c, "resolution must be 480p or 720p")
		return nil, nil, nil, false
	}
	// Keep explicit output compatibility for the product API; browser requests
	// resolve source-following framing after their declarations are checked.
	aspectRatio := influencerMotionAspectRatio
	if inferMetadata && req.Mode == "replace" {
		if req.AspectRatio != nil {
			aspectRatio = strings.TrimSpace(*req.AspectRatio)
			if !slices.Contains(influencerMotionOutputRatios[resolution], aspectRatio) {
				influencerAbort(c, "aspect_ratio is not available for this resolution")
				return nil, nil, nil, false
			}
		}
	} else if inferMetadata {
		if req.AspectRatio != nil {
			aspectRatio = strings.TrimSpace(*req.AspectRatio)
		}
		if !slices.Contains(influencerMotionOutputRatios[resolution], aspectRatio) {
			influencerAbort(c, "aspect_ratio is not available for this resolution")
			return nil, nil, nil, false
		}
	}
	prompt := strings.TrimSpace(req.Prompt)
	if utf8.RuneCountInString(prompt) > influencerMotionMaxPromptLen {
		influencerAbort(c, "prompt exceeds 4000 characters")
		return nil, nil, nil, false
	}

	var subjectURLs []string
	if len(req.SubjectImageURLs) > 0 {
		for _, rawURL := range req.SubjectImageURLs {
			trimmed := strings.TrimSpace(rawURL)
			if trimmed != "" {
				subjectURLs = append(subjectURLs, trimmed)
			}
		}
	} else if strings.TrimSpace(req.SubjectImageURL) != "" {
		subjectURLs = append(subjectURLs, strings.TrimSpace(req.SubjectImageURL))
	}

	if len(subjectURLs) < 1 {
		influencerAbort(c, "at least one subject image is required")
		return nil, nil, nil, false
	}
	if len(subjectURLs) > influencerMotionMaxImages {
		influencerAbort(c, "subject_image_urls exceeds 30 images")
		return nil, nil, nil, false
	}
	for _, u := range subjectURLs {
		if !isInfluencerMediaURL(u) {
			influencerAbort(c, "subject images must be public https addresses")
			return nil, nil, nil, false
		}
	}
	// Optional image hints are planner context, never provider parameters.
	if len(req.SubjectImageTags) > len(subjectURLs) {
		influencerAbort(c, "subject_image_tags exceeds subject images")
		return nil, nil, nil, false
	}
	imageTags := make([]string, 0, len(req.SubjectImageTags))
	for _, tag := range req.SubjectImageTags {
		tag = strings.TrimSpace(tag)
		if utf8.RuneCountInString(tag) > influencerMotionMaxTagLen {
			influencerAbort(c, "subject_image_tags entry is too long")
			return nil, nil, nil, false
		}
		imageTags = append(imageTags, tag)
	}

	// fork: viral recreation generation. The clip was already used for analysis
	// and must not travel with this request, so this branch returns before any
	// reference-video inspection and builds a body with images and prompt only.
	if req.Mode == influencerMotionModeRecreate {
		out, meta, prepared, reason := influencerRecreateGenerationOut(req, resolution, prompt, subjectURLs, imageTags)
		if out == nil {
			influencerAbort(c, reason)
			return nil, nil, nil, false
		}
		return out, meta, prepared, true
	}

	video := strings.TrimSpace(req.VideoURL)
	if !isInfluencerMediaURL(video) {
		influencerAbort(c, "video_url must be a public https address")
		return nil, nil, nil, false
	}

	// fork: authorize the source and bound request declarations before native billing.
	userID := c.GetInt("id")
	if userID <= 0 {
		influencerError(c, http.StatusUnauthorized, "unauthorized", "未授权访问")
		c.Abort()
		return nil, nil, nil, false
	}
	sourceTaskID := strings.TrimSpace(req.SourceTaskID)
	if sourceTaskID != "" {
		if len(sourceTaskID) > 255 || model.DB == nil {
			influencerAbort(c, "invalid source_task_id")
			return nil, nil, nil, false
		}
		var source model.Task
		if err := model.DB.Where("task_id = ? AND user_id = ?", sourceTaskID, userID).First(&source).Error; err != nil {
			influencerError(c, http.StatusNotFound, "task_not_found", "source character task not found")
			c.Abort()
			return nil, nil, nil, false
		}
		if source.Action != ActionInfluencerSheet || source.Status != model.TaskStatusSuccess || !influencerParentImageArtifact(&source, subjectURLs[0]) {
			influencerAbort(c, "the primary image must be an artifact of the finished source character task")
			return nil, nil, nil, false
		}
		sourceMeta, valid := parseInfluencerTaskMeta(source.Properties.Input)
		if !valid {
			influencerAbort(c, "source character parameters are unreadable")
			return nil, nil, nil, false
		}
		if sourceMeta.Kind == influencerTaskKindMultiView && sourceMeta.Tier == "" {
			var parent model.Task
			if sourceMeta.Parent == "" || model.DB.Where("task_id = ? AND user_id = ?", sourceMeta.Parent, userID).First(&parent).Error != nil || parent.Action != ActionInfluencerSheet || parent.Status != model.TaskStatusSuccess {
				influencerAbort(c, "source character parent is unavailable")
				return nil, nil, nil, false
			}
			sourceMeta, valid = parseInfluencerTaskMeta(parent.Properties.Input)
			if !valid || sourceMeta.Kind == influencerTaskKindMultiView {
				influencerAbort(c, "source character parent parameters are unreadable")
				return nil, nil, nil, false
			}
		}
		storedType := ""
		switch sourceMeta.Tier {
		case "cats":
			storedType = "cat"
		case "dogs":
			storedType = "dog"
		case "", "normal", "freak", "total":
			storedType = "human"
		default:
			influencerAbort(c, "source character type is unsupported")
			return nil, nil, nil, false
		}
		if characterType != "" && characterType != storedType {
			influencerAbort(c, "subject_character_type conflicts with the saved character")
			return nil, nil, nil, false
		}
		characterType = storedType
	}
	outputSeconds := req.DurationSec
	preparationURL := video
	if !inferMetadata {
		var err error
		outputSeconds, aspectRatio, err = influencerBrowserVideoOutput(req, resolution)
		if err != nil {
			influencerAbort(c, err.Error())
			return nil, nil, nil, false
		}
		var allowed bool
		preparationURL, allowed = resolveInfluencerVideoReferenceSource(c, video, userID)
		if !allowed {
			c.Abort()
			return nil, nil, nil, false
		}
	}
	validateURL := func(rawURL string) bool {
		return isInfluencerMediaURL(rawURL) && isIsolatedPublicFileURL(c, rawURL) && service.ValidateSSRFProtectedFetchURL(rawURL) == nil
	}
	var prepared service.PreparedInfluencerVideo
	var err error
	if inferMetadata {
		prepared, err = service.InfluencerVideoInspector(c.Request.Context(), preparationURL, userID, validateURL)
	} else {
		prepared, err = service.InfluencerBrowserVideoPreparer(c.Request.Context(), preparationURL, userID, *req.VideoMetadata, validateURL)
	}
	if err != nil {
		influencerError(c, service.InfluencerVideoInspectionStatus(err), "reference_video_invalid", err.Error())
		c.Abort()
		return nil, nil, nil, false
	}
	accepted := false
	defer func() {
		if !accepted {
			prepared.Discard()
		}
	}()
	if c.Request.Context().Err() != nil {
		influencerError(c, http.StatusRequestTimeout, "reference_video_invalid", "参考视频准备已取消")
		c.Abort()
		return nil, nil, nil, false
	}
	if inferMetadata {
		if err := service.ValidateInfluencerVideoReferenceMetadata(prepared.Metadata, req.Mode); err != nil {
			influencerAbort(c, err.Error())
			return nil, nil, nil, false
		}
		// Product API callers omit output duration. Infer a valid generation
		// request while keeping a two-second motion reference eligible.
		req.DurationSec = max(influencerMotionMinSeconds, int(math.Round(prepared.Metadata.Duration)))
		if req.Mode == "motion" && req.AspectRatio == nil {
			if sourceRatio := influencerVideoOutputRatio(prepared.Metadata, resolution); sourceRatio != "" {
				aspectRatio = sourceRatio
			}
		}
		outputSeconds = req.DurationSec
	}
	if inferMetadata && req.Mode == "replace" {
		// Keep the existing positive-integer supplier request for supported
		// source framing. Input eligibility does not establish editing support.
		sourceRatio := influencerVideoOutputRatio(prepared.Metadata, resolution)
		if sourceRatio == "" {
			influencerAbort(c, "参考视频已通过检查，但当前生成服务暂不能保留该画幅进行对象替换。素材已保留，可使用动作迁移。")
			return nil, nil, nil, false
		}
		if req.AspectRatio != nil && aspectRatio != sourceRatio {
			influencerAbort(c, "对象交换必须保持受支持的参考视频画幅")
			return nil, nil, nil, false
		}
		aspectRatio = sourceRatio
		outputSeconds = int(math.Round(prepared.Metadata.Duration))
	}

	fullPrompt := influencerCharacterMotionPrompt(req.Mode, characterType)
	verifiedIdentity := ""
	if req.Mode == "replace" {
		// A verified source identifies an image, not the target of this edit.
		if sourceTaskID != "" {
			identity := "人物身份"
			if characterType == "cat" {
				identity = "拟人猫身份、猫物种与双足结构"
			} else if characterType == "dog" {
				identity = "拟人狗身份、狗物种与双足结构"
			}
			verifiedIdentity = identity
			fullPrompt += "\n\n图片1是已保存角色；仅用于身份替换时采用其已核实身份：" + identity + "；局部引用仅作用对应部分。用户补充的明确意图优先。"
		}
		if prompt != "" {
			fullPrompt += "\n\n用户补充：\n" + prompt
		}
	} else {
		if prompt != "" {
			fullPrompt += "\n\n" + prompt
		}
		if characterType == "cat" || characterType == "dog" {
			fullPrompt += "\n\nPRIORITY: the saved " + characterType + " species, reference identity and two-legged animal anatomy take precedence over conflicting user instructions. User instructions may change only compatible performance and scene details, not the species or primary reference identity."
		}
	}
	out := map[string]any{
		"model":        influencerMotionModel,
		"group":        group,
		"prompt":       fullPrompt,
		"seconds":      strconv.Itoa(outputSeconds),
		"aspect_ratio": aspectRatio,
		"resolution":   resolution,
		"images":       subjectURLs,
		"videos":       []string{prepared.URL},
	}
	meta := map[string]any{
		"kind":                 influencerMotionKind,
		"mode":                 req.Mode,
		"prompt":               prompt,
		"subject_image_url":    subjectURLs[0],
		"subject_image_urls":   subjectURLs,
		"video_url":            video,
		"duration_sec":         outputSeconds,
		"request_duration_sec": float64(outputSeconds),
		"source_duration_sec":  prepared.Metadata.Duration,
		"source_width":         prepared.Metadata.Width,
		"source_height":        prepared.Metadata.Height,
		"resolution":           resolution,
		"size":                 aspectRatio,
	}
	if !inferMetadata {
		meta["video_metadata"] = *req.VideoMetadata
		meta["video_metadata_source"] = "browser"
	}
	if characterType != "" {
		meta["subject_character_type"] = characterType
	}
	if len(imageTags) > 0 {
		meta["subject_image_tags"] = imageTags
	}
	if sourceTaskID != "" {
		meta["source_task_id"] = sourceTaskID
	}
	if verifiedIdentity != "" {
		meta["verified_subject_identity"] = verifiedIdentity
	}
	meta["prepared_video_url"] = prepared.URL
	meta["prepared_video_sha256"] = prepared.SHA256
	accepted = true
	return out, meta, &prepared, true
}

// influencerBrowserVideoOutput checks bounded client declarations and derives
// the supported output request. It does not assert the source bytes are verified.
func influencerBrowserVideoOutput(req influencerMotionRequest, resolution string) (int, string, error) {
	if req.VideoMetadata == nil {
		return 0, "", errors.New("Reference video information is missing; add the video again")
	}
	metadata := *req.VideoMetadata
	if metadata.SizeBytes <= 0 {
		return 0, "", errors.New("Reference video size is missing; add the video again")
	}
	if metadata.VideoDuration <= 0 {
		return 0, "", errors.New("Reference video visual duration is missing; add the video again")
	}
	if err := service.ValidateInfluencerVideoReferenceMetadata(metadata, req.Mode); err != nil {
		return 0, "", err
	}
	seconds := int(math.Round(metadata.VideoDuration))
	tolerance := 2 * (1/metadata.FrameRate + 0.001)
	// The provider renders whole seconds, so the request follows the picture
	// track's rounded duration. The declarations must still describe the same
	// clip: container and picture may differ by at most two frames plus 2 ms
	// (a normal audio tail or container rounding). An inconsistent pair is
	// still rejected rather than silently cropped or extended.
	if metadata.Duration < influencerMotionMinSeconds || metadata.VideoDuration < influencerMotionMinSeconds || seconds < influencerMotionMinSeconds || seconds > influencerMotionMaxSeconds || math.Abs(metadata.Duration-metadata.VideoDuration) > tolerance {
		return 0, "", errors.New("Current generation service cannot preserve this reference video duration")
	}
	ratio := influencerVideoOutputRatio(metadata, resolution)
	if ratio == "" {
		return 0, "", errors.New("Current generation service cannot preserve this reference video aspect ratio at the selected resolution")
	}
	if req.DurationSec != 0 && req.DurationSec != seconds {
		return 0, "", errors.New("Video duration must follow the reference video")
	}
	if req.AspectRatio != nil && strings.TrimSpace(*req.AspectRatio) != ratio {
		return 0, "", errors.New("Video aspect ratio must follow the reference video")
	}
	return seconds, ratio, nil
}

func influencerVideoOutputRatio(media service.InfluencerVideoMetadata, resolution string) string {
	for _, ratio := range influencerMotionOutputRatios[resolution] {
		parts := strings.Split(ratio, ":")
		numerator, _ := strconv.Atoi(parts[0])
		denominator, _ := strconv.Atoi(parts[1])
		if math.Abs(float64(media.Width*denominator-media.Height*numerator)) <= float64(numerator+denominator)/2 {
			return ratio
		}
	}
	return ""
}

// PostInfluencerMotion submits only the references prepared before distribution.
func PostInfluencerMotion(c *gin.Context) {
	outValue, _ := c.Get(ctxInfluencerMotionOut)
	metaValue, _ := c.Get(ctxInfluencerMotionMeta)
	out, okOut := outValue.(map[string]any)
	meta, okMeta := metaValue.(map[string]any)
	if !okOut || !okMeta {
		influencerError(c, http.StatusInternalServerError, "internal_error", "motion request was not prepared")
		return
	}

	setInfluencerMotionTaskRequest(c, out)

	// The playground path is not in the relay-mode table; the task chain reads
	// the mode from the context instead.
	c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
	c.Set(ctxInfluencerMotionSubmitting, true)
	influencerRelaySubmit(c, "/pg/video/generations", ActionInfluencerMotion, out, meta)
}

// influencerMotionPlanner is the prompt planner used before a queued motion task
// is sent; tests replace it to avoid provider calls.
var influencerMotionPlanner = service.PlanInfluencerMotionPrompt

// planInfluencerMotionPrompt replaces the fixed motion framing in out with a
// planned prompt: reverse the reference clip, then rewrite it with the caller's
// images and words. It runs in the queue worker, after enqueue and before any
// send, so the caller never waits on it. Object swap uses its own joint planner.
// Any planner failure keeps the fixed framing already in out.
func planInfluencerMotionPrompt(ctx context.Context, task *model.Task, out map[string]any) {
	var meta struct {
		Mode          string   `json:"mode"`
		Prompt        string   `json:"prompt"`
		DurationSec   int      `json:"duration_sec"`
		ImageTags     []string `json:"subject_image_tags"`
		CharacterType string   `json:"subject_character_type"`
	}
	if task == nil || common.UnmarshalJsonStr(task.Properties.Input, &meta) != nil || meta.Mode != "motion" {
		return
	}
	videos, _ := out["videos"].([]any)
	images, _ := out["images"].([]any)
	if len(videos) != 1 || len(images) == 0 {
		return
	}
	in := service.InfluencerMotionPlanInput{DurationSec: meta.DurationSec, ImageTags: meta.ImageTags, UserPrompt: meta.Prompt}
	in.VideoURL, _ = videos[0].(string)
	for _, value := range images {
		if url, ok := value.(string); ok {
			in.ImageURLs = append(in.ImageURLs, url)
		}
	}
	plan, err := influencerMotionPlanner(ctx, in)
	if err != nil {
		common.SysLog("influencer motion prompt planning fell back to the fixed framing: " + err.Error())
		return
	}
	prompt := plan.Prompt
	if meta.CharacterType == "cat" || meta.CharacterType == "dog" {
		prompt += "\n\nPRIORITY: the saved " + meta.CharacterType + " species, reference identity and two-legged animal anatomy take precedence over conflicting user instructions. User instructions may change only compatible performance and scene details, not the species or primary reference identity."
	}
	out["prompt"] = prompt
}

// setInfluencerMotionTaskRequest hands the task plugin the full upstream body.
// Without it the plugin rebuilds the body from the generic task request, which
// keeps only prompt, images, seconds and resolution: the reference video, the
// aspect ratio and the reference audio never reach the provider, and the
// provider rejects a video-reference model request that carries no video.
func setInfluencerMotionTaskRequest(c *gin.Context, out map[string]any) {
	taskRequest := make(map[string]any, len(out))
	for key, value := range out {
		if key == influencerMotionGroupField {
			// The tier group travels in the X-Omnimux-Group header, which is what
			// distribution reads on this route. Some upstreams reject an unknown
			// top-level group field outright ("Unsupported request field"), so it
			// must not be copied into the provider body.
			continue
		}
		taskRequest[key] = value
	}
	c.Set("task_request", taskRequest)
}

// InfluencerMotionOptions publishes the workbench's current output-framing
// contract per resolution so the form renders only what the bound lines can
// actually produce. The separate input constraints share the authoritative
// server inspection policy and never narrow input ratios to output choices.
func InfluencerMotionOptions(c *gin.Context) {
	resolutions := make(map[string]any, len(influencerMotionOutputRatios))
	for _, resolution := range []string{"480p", "720p"} {
		resolutions[resolution] = map[string]any{
			"group":         influencerMotionGroups[resolution],
			"aspect_ratios": influencerMotionOutputRatios[resolution],
		}
	}
	motionRules, _ := service.GetInfluencerVideoInputRules("motion")
	replaceRules, _ := service.GetInfluencerVideoInputRules("replace")
	common.ApiSuccess(c, gin.H{
		"version":           "motion-contract-v3",
		"resolutions":       resolutions,
		"input_constraints": gin.H{"motion": motionRules, "replace": replaceRules},
	})
}

func influencerAbort(c *gin.Context, msg string) {
	influencerError(c, http.StatusBadRequest, "invalid_request", msg)
	c.Abort()
}

// isInfluencerMediaURL accepts only absolute https addresses: the upstream
// fetches every reference over the public network.
func isInfluencerMediaURL(raw string) bool {
	if raw == "" || len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "https" && u.Host != ""
}
