package controller

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

var influencerObjectSwapPlanner = service.PlanInfluencerObjectSwapPrompt

type influencerVideoPlanMetadata struct {
	Mode             string   `json:"mode"`
	Prompt           string   `json:"prompt"`
	DurationSec      int      `json:"duration_sec"`
	ImageTags        []string `json:"subject_image_tags"`
	VerifiedIdentity string   `json:"verified_subject_identity"`
}

// fork: both product entrances plan from the same frozen task before video submission.
func prepareInfluencerVideoPrompt(ctx context.Context, task *model.Task, out map[string]any, scanner *model.SystemTask, runnerID string) error {
	var meta influencerVideoPlanMetadata
	if task == nil || common.UnmarshalJsonStr(task.Properties.Input, &meta) != nil {
		return errors.New("对象替换信息不可读取，视频生成未开始")
	}
	switch meta.Mode {
	case "":
		// Historical queued tasks predate mode metadata and retain their
		// accepted request. New object-swap tasks always persist replace.
		return nil
	case "motion":
		planInfluencerMotionPrompt(ctx, task, out)
		return nil
	case "recreate":
		// fork: viral recreation carries the already-compiled prompt from the
		// analysis step and has no reference clip to plan from.
		return nil
	case "replace":
	default:
		return errors.New("视频生成类型不可读取，任务未开始")
	}

	videos, videoOK := out["videos"].([]any)
	images, imageOK := out["images"].([]any)
	if !videoOK || len(videos) != 1 || !imageOK || len(images) == 0 {
		return errors.New("对象替换素材不完整，视频生成未开始")
	}
	in := service.InfluencerObjectSwapPlanInput{DurationSec: meta.DurationSec, ImageTags: meta.ImageTags, UserPrompt: meta.Prompt, VerifiedIdentity: meta.VerifiedIdentity}
	in.VideoURL, _ = videos[0].(string)
	if strings.TrimSpace(in.VideoURL) == "" {
		return errors.New("对象替换视频不可读取，视频生成未开始")
	}
	for _, value := range images {
		url, ok := value.(string)
		if !ok || strings.TrimSpace(url) == "" {
			return errors.New("对象替换图片不可读取，视频生成未开始")
		}
		in.ImageURLs = append(in.ImageURLs, url)
	}
	preparation, begun, err := model.BeginGenerationObjectSwapPreparation(ctx, task, scanner, runnerID)
	if err != nil {
		return errors.New("素材理解准备状态不可确认，视频生成未开始")
	}
	if !begun {
		if preparation != nil && preparation.State == model.GenerationObjectSwapReady && strings.TrimSpace(preparation.Prompt) != "" {
			out["prompt"] = preparation.Prompt
			return nil
		}
		return errors.New("素材理解未完成或结果待核对，视频生成未开始，不会自动重复理解")
	}

	plan, planErr := influencerObjectSwapPlanner(ctx, in)
	state := model.GenerationObjectSwapReady
	if planErr != nil || strings.TrimSpace(plan.Prompt) == "" {
		state = model.GenerationObjectSwapFailed
		if plan.Trace.Outcome == "unknown" {
			state = model.GenerationObjectSwapUnknown
		}
	}
	details, err := common.Marshal(plan)
	if err != nil {
		return errors.New("素材理解结果无法保存，视频生成未开始")
	}
	result := model.GenerationObjectSwapPreparation{State: state, Details: details}
	if state == model.GenerationObjectSwapReady {
		result.Prompt = plan.Prompt
	}
	if err := model.FinishGenerationObjectSwapPreparation(ctx, task, scanner, runnerID, result); err != nil {
		return errors.New("素材理解结果未确认保存，视频生成未开始")
	}
	if state != model.GenerationObjectSwapReady {
		// The understanding call never consumed the clip, so any plan it returns
		// describes a scene the media does not contain.
		if errors.Is(planErr, service.ErrInfluencerObjectSwapUnobserved) {
			return errors.New("素材理解未观察到参考视频，视频生成未开始")
		}
		// Provider bodies may contain signed media URLs or account details.
		// Only the credential-free trace is retained in private task data.
		if state == model.GenerationObjectSwapUnknown {
			return errors.New("素材理解结果待核对，视频生成未开始，不会自动重复理解")
		}
		var audit struct {
			Issue struct {
				Code string `json:"code"`
			} `json:"issue"`
		}
		if common.Unmarshal(plan.Audit, &audit) == nil {
			switch audit.Issue.Code {
			case "unreadable_media":
				return errors.New("参考素材无法完整读取，视频生成未开始")
			case "no_match":
				return errors.New("参考素材与要求无法形成合理的对象替换，视频生成未开始")
			case "out_of_scope":
				return errors.New("自定义要求超出对象替换范围，视频生成未开始")
			}
		}
		return errors.New("素材理解失败或未完成，视频生成未开始")
	}
	out["prompt"] = plan.Prompt
	return nil
}
