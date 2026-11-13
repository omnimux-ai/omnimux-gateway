package service

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	httpstd "net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// fork: influencer viral-recreation planning. A user supplies a reference clip
// that is used ONLY as decomposition material plus the reference images that
// define their own product or character. Three internal turns later the workbench
// has a Seedance prompt and a human-readable breakdown; the generation request
// later carries images and the prompt and never the clip. The clip therefore
// never reaches the video model, which is the product promise of this surface.

// ErrInfluencerRecreateTimeout marks a plan that ran past the product request
// budget. The handler turns it into a retryable answer instead of a proxy timeout.
var ErrInfluencerRecreateTimeout = errors.New("参考视频分析超时，请稍后重试")

//go:embed influencer_recreate_prompts/analysis.txt
var influencerRecreateAnalysisSystem string

//go:embed influencer_recreate_prompts/image.txt
var influencerRecreateImageSystem string

//go:embed influencer_recreate_prompts/compose.txt
var influencerRecreateComposeSystem string

const (
	// InfluencerRecreateMaxVideoBytes caps the decomposition download. The clip is
	// no longer sent inline: the server extracts its key frames, so the bound only
	// protects the download and the temporary file.
	InfluencerRecreateMaxVideoBytes = 64 * 1024 * 1024
	// InfluencerRecreateMaxPromptRunes is the same public prompt ceiling the
	// generation route enforces, counted in runes.
	InfluencerRecreateMaxPromptRunes = 4000
	// InfluencerRecreateMaxImages bounds one recreation image set. The mode now
	// shares the motion line (seedance-2-5), whose reference ceiling is 30.
	InfluencerRecreateMaxImages = 30
	// InfluencerRecreateMinSeconds and InfluencerRecreateMaxSeconds bound the
	// generated clip. The output length follows the reference clip instead of a
	// caller choice; the shared seedance-2-5 line accepts 4-30 whole seconds.
	InfluencerRecreateMinSeconds = 4
	InfluencerRecreateMaxSeconds = 30

	influencerRecreateImageBytes = 8 * 1024 * 1024

	// The three turns run in the generation queue worker, not in a proxied request,
	// so they may take minutes; key frames still keep the vision turn cheap. This
	// deadline bounds one job so a stuck supplier cannot hold the account slot.
	influencerRecreatePlanTimeout = 300 * time.Second

	// Key-frame extraction budget: 512 px long side keeps the vision turn cheap,
	// and the count follows the clip length.
	influencerRecreateFrameBox        = 512
	influencerRecreateFrameMinCount   = 6
	influencerRecreateFrameMaxCount   = 12
	influencerRecreateFrameSecondsPer = 1.2
)

// InfluencerRecreatePlanInput is everything the recreation planner may read.
// VideoURL is decomposition material only; ImageURLs keep the caller's order.
type InfluencerRecreatePlanInput struct {
	VideoURL   string
	Resolution string
	ImageURLs  []string
	ImageTags  []string
	UserPrompt string
}

// InfluencerRecreatePlan is the planner's accepted result. AnalysisJSON is the
// structured breakdown the workbench renders next to the prompt; ImageJSON keeps
// the per-image reading so the product can show what the model locked.
type InfluencerRecreatePlan struct {
	Prompt           string
	AnalysisJSON     string
	ImageJSON        string
	ShotCount        int
	DurationSec      int
	ReferenceSeconds float64
	Hook             string
	RiskFlags        []string
}

// influencerRecreateAnalysis is the subset of the breakdown the planner gates on
// and reports to the workbench. Unknown fields stay in AnalysisJSON.
type influencerRecreateAnalysis struct {
	Evidence struct {
		Grade string `json:"grade"`
		Media struct {
			AnalysisStatus string  `json:"analysis_status"`
			DurationS      float64 `json:"duration_s"`
		} `json:"media"`
	} `json:"evidence"`
	Shots []struct {
		ShotID       string  `json:"shot_id"`
		StartS       float64 `json:"start_s"`
		EndS         float64 `json:"end_s"`
		NarrativeJob string  `json:"narrative_job"`
	} `json:"shots"`
	Hook struct {
		Mechanism        []string `json:"mechanism"`
		WhyItStopsScroll string   `json:"why_it_stops_scroll"`
	} `json:"hook"`
}

// influencerRecreateCompose is the subset of the compose answer the planner
// accepts; the full answer still travels to the workbench as the risk summary.
type influencerRecreateComposeAnswer struct {
	Prompt    string `json:"prompt"`
	RiskFlags []struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
		Note     string `json:"note"`
	} `json:"risk_flags"`
	Meta struct {
		PromptCharCount int `json:"prompt_char_count"`
	} `json:"meta"`
}

// PlanInfluencerRecreatePrompt runs analysis, reference-image reading and prompt
// composition. Any gate failure returns an error and the product shows it; it
// never substitutes a guessed prompt, because a wrong prompt still bills a video.
func PlanInfluencerRecreatePrompt(parent context.Context, in InfluencerRecreatePlanInput) (InfluencerRecreatePlan, error) {
	ctx, cancel := context.WithTimeout(parent, influencerRecreatePlanTimeout)
	defer cancel()
	if !strings.HasPrefix(strings.TrimSpace(in.VideoURL), "https://") {
		return InfluencerRecreatePlan{}, errors.New("只能拆解已上传的参考视频")
	}
	if len(in.ImageURLs) == 0 {
		return InfluencerRecreatePlan{}, errors.New("至少需要一张参考图来锁定生成主体")
	}
	if len(in.ImageURLs) > InfluencerRecreateMaxImages {
		return InfluencerRecreatePlan{}, fmt.Errorf("参考图最多 %d 张", InfluencerRecreateMaxImages)
	}
	if in.Resolution != "480p" && in.Resolution != "720p" {
		in.Resolution = "480p"
	}

	video, err := influencerMotionFetchMedia(ctx, in.VideoURL, InfluencerRecreateMaxVideoBytes)
	if err != nil {
		return InfluencerRecreatePlan{}, errors.New("参考视频无法读取，请重新上传 4 到 30 秒的短视频")
	}
	// The generated clip does not take a caller-chosen length: it follows the
	// reference clip. The same temporary file feeds the probe and the key frames,
	// so the clip is downloaded and written exactly once.
	clip, err := influencerRecreatePrepareClip(ctx, video)
	if err != nil {
		return InfluencerRecreatePlan{}, influencerRecreatePlanFailure(ctx, err)
	}
	durationSec, referenceSeconds := clip.DurationSec, clip.ReferenceSeconds

	session := newInfluencerMotionPlanSession()
	analysis, err := influencerRecreateAnalyseVideo(ctx, clip, in, session)
	if err != nil {
		return InfluencerRecreatePlan{}, influencerRecreatePlanFailure(ctx, err)
	}
	parsed, err := influencerRecreateAnalysisGate(analysis)
	if err != nil {
		return InfluencerRecreatePlan{}, err
	}
	imageJSON, err := influencerRecreateReadImages(ctx, in, session)
	if err != nil {
		return InfluencerRecreatePlan{}, influencerRecreatePlanFailure(ctx, err)
	}
	prompt, risks, err := influencerRecreateCompose(ctx, analysis, imageJSON, in, durationSec, len(in.ImageURLs), session)
	if err != nil {
		return InfluencerRecreatePlan{}, err
	}
	hook := ""
	if len(parsed.Hook.Mechanism) > 0 {
		hook = strings.Join(parsed.Hook.Mechanism, ",")
	}
	return InfluencerRecreatePlan{
		Prompt:           prompt,
		AnalysisJSON:     analysis,
		ImageJSON:        imageJSON,
		ShotCount:        len(parsed.Shots),
		DurationSec:      durationSec,
		ReferenceSeconds: referenceSeconds,
		Hook:             hook,
		RiskFlags:        risks,
	}, nil
}

// influencerRecreateAnalyseVideo runs the decomposition turn on the clip's key
// frames. Frames keep the vision turn short enough for the synchronous product
// request and still carry timestamps, so the breakdown can place every beat.
func influencerRecreateAnalyseVideo(ctx context.Context, clip influencerRecreateClip, in InfluencerRecreatePlanInput, session *influencerMotionPlanSession) (string, error) {
	intent := strings.TrimSpace(in.UserPrompt)
	if intent == "" {
		intent = "未指定（按结构 1:1 迁移到参考图主体）"
	}
	content := []map[string]any{
		{"type": "text", "text": fmt.Sprintf("参考视频关键帧共 %d 张，按时间戳升序；时间区间与镜头切分只根据这些帧判断。只做结构拆解，不识别身份。", len(clip.Frames))},
	}
	for index, frame := range clip.Frames {
		timestamp := 0.0
		if index < len(clip.FrameTimes) {
			timestamp = clip.FrameTimes[index]
		}
		content = append(content,
			map[string]any{"type": "text", "text": fmt.Sprintf("关键帧 %.1fs：", timestamp)},
			map[string]any{"type": "image_url", "image_url": map[string]any{
				"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame),
			}},
		)
	}
	content = append(content,
		map[string]any{"type": "text", "text": "用户意图：" + intent},
		map[string]any{"type": "text", "text": fmt.Sprintf("目标平台：tiktok；目标时长：%d 秒；内容语言：跟随视频。", clip.DurationSec)},
	)
	reply, err := InfluencerMotionChat(ctx, influencerRecreateAnalysisSystem, content, influencerRecreateAnalysisReplyUsable, session)
	if err != nil {
		return "", fmt.Errorf("参考视频拆解失败: %w", err)
	}
	return influencerMotionStripFence(reply), nil
}

// influencerRecreateAnalysisReplyUsable rejects an answer that is not a usable
// breakdown so the transport can try the next audited line instead of treating a
// prose answer as structure.
func influencerRecreateAnalysisReplyUsable(reply string) error {
	var parsed influencerRecreateAnalysis
	if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &parsed); err != nil {
		return errors.New("no usable breakdown json")
	}
	if len(parsed.Shots) == 0 {
		return errors.New("breakdown has no shots")
	}
	return nil
}

// influencerRecreateAnalysisGate enforces the product gates on an accepted
// breakdown: the model must have been able to read the clip, and its shot list
// must be a real timeline.
func influencerRecreateAnalysisGate(analysis string) (influencerRecreateAnalysis, error) {
	var parsed influencerRecreateAnalysis
	if err := common.UnmarshalJsonStr(analysis, &parsed); err != nil {
		return influencerRecreateAnalysis{}, errors.New("参考视频拆解结果无法解析，请换一段视频重试")
	}
	if strings.EqualFold(parsed.Evidence.Media.AnalysisStatus, "BLOCKED") {
		return influencerRecreateAnalysis{}, errors.New("这段参考视频无法拆解，请上传可正常播放的 4 到 30 秒短视频")
	}
	if len(parsed.Shots) == 0 {
		return influencerRecreateAnalysis{}, errors.New("参考视频没有可用分镜，请换一段视频重试")
	}
	return parsed, nil
}

// influencerRecreateReadImages describes the caller's own references by upload
// order. The texts are the compose turn's only source of subject appearance.
func influencerRecreateReadImages(ctx context.Context, in InfluencerRecreatePlanInput, session *influencerMotionPlanSession) (string, error) {
	content := []map[string]any{
		{"type": "text", "text": fmt.Sprintf("参考图共 %d 张，按上传顺序输出 JSON 数组。", len(in.ImageURLs))},
	}
	for i, url := range in.ImageURLs {
		tag := ""
		if i < len(in.ImageTags) {
			tag = strings.TrimSpace(in.ImageTags[i])
		}
		if tag == "" {
			tag = "未标注"
		}
		image, err := influencerMotionFetchMedia(ctx, url, influencerRecreateImageBytes)
		if err != nil {
			return "", fmt.Errorf("第 %d 张参考图无法读取", i+1)
		}
		content = append(content,
			map[string]any{"type": "text", "text": fmt.Sprintf("参考图 %d（标签：%s）：", i+1, tag)},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + httpstd.DetectContentType(image) + ";base64," + base64.StdEncoding.EncodeToString(image)}},
		)
	}
	reply, err := InfluencerMotionChat(ctx, influencerRecreateImageSystem, content, influencerRecreateImageReplyUsable(len(in.ImageURLs)), session)
	if err != nil {
		return "", fmt.Errorf("参考图分析失败: %w", err)
	}
	return influencerMotionStripFence(reply), nil
}

// influencerRecreateImageReplyUsable requires one entry per uploaded image so the
// compose turn can never reference an angle the user did not provide.
func influencerRecreateImageReplyUsable(count int) func(string) error {
	return func(reply string) error {
		var items []struct {
			ImageIndex int    `json:"image_index"`
			Kind       string `json:"kind"`
			Summary    string `json:"summary"`
		}
		if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &items); err != nil {
			return errors.New("no usable image reading json")
		}
		if len(items) != count {
			return fmt.Errorf("image reading covered %d of %d images", len(items), count)
		}
		for _, item := range items {
			if item.ImageIndex < 1 || item.ImageIndex > count || strings.TrimSpace(item.Summary) == "" {
				return errors.New("image reading is out of order")
			}
		}
		return nil
	}
}

// influencerRecreateCompose runs the compile turn and enforces the prompt gates.
// One corrective retry is allowed; a second failure stops before any video is
// billed.
func influencerRecreateCompose(ctx context.Context, analysis, imageJSON string, in InfluencerRecreatePlanInput, durationSec, imageCount int, session *influencerMotionPlanSession) (string, []string, error) {
	intent := strings.TrimSpace(in.UserPrompt)
	if intent == "" {
		intent = "未指定（按结构 1:1 迁移到参考图主体）"
	}
	base := []map[string]any{
		{"type": "text", "text": "参考视频结构化拆解（JSON）：\n" + analysis},
		{"type": "text", "text": "参考图分析（按上传顺序，image 1 起）：\n" + imageJSON},
		{"type": "text", "text": "用户意图：" + intent},
		{"type": "text", "text": fmt.Sprintf("目标参数：duration_sec=%d（跟随参考视频），aspect_ratio=9:16，resolution=%s，image_count=%d，platform=tiktok，prompt_max_chars=%d。", durationSec, in.Resolution, imageCount, InfluencerRecreateMaxPromptRunes)},
	}
	var correction string
	for attempt := 0; attempt < 2; attempt++ {
		content := append([]map[string]any{}, base...)
		if correction != "" {
			content = append(content, map[string]any{"type": "text", "text": correction})
		}
		reply, err := InfluencerMotionChat(ctx, influencerRecreateComposeSystem, content, influencerRecreateComposeReplyUsable, session)
		if err != nil {
			return "", nil, fmt.Errorf("提示词编译失败: %w", err)
		}
		composed, err := influencerRecreateParseCompose(influencerMotionStripFence(reply))
		if err != nil {
			return "", nil, err
		}
		gate := InfluencerRecreatePromptGate(composed.Prompt, imageCount)
		if gate == "" {
			risks := make([]string, 0, len(composed.RiskFlags))
			for _, flag := range composed.RiskFlags {
				if strings.TrimSpace(flag.Code) != "" {
					risks = append(risks, flag.Code)
				}
			}
			return composed.Prompt, risks, nil
		}
		correction = "上一次输出违反了产品门禁（" + gate + "）。请只输出修正后的完整 JSON。"
	}
	return "", nil, errors.New("提示词编译未通过门禁，请调整意图后重试")
}

// influencerRecreateComposeReplyUsable requires the compose shape; a prose answer
// must never become a billed prompt.
func influencerRecreateComposeReplyUsable(reply string) error {
	var parsed influencerRecreateComposeAnswer
	if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &parsed); err != nil {
		return errors.New("no usable compose json")
	}
	if strings.TrimSpace(parsed.Prompt) == "" {
		return errors.New("compose answer has no prompt")
	}
	return nil
}

func influencerRecreateParseCompose(reply string) (influencerRecreateComposeAnswer, error) {
	var parsed influencerRecreateComposeAnswer
	if err := common.UnmarshalJsonStr(reply, &parsed); err != nil {
		return influencerRecreateComposeAnswer{}, errors.New("提示词编译结果无法解析，请重试")
	}
	if strings.TrimSpace(parsed.Prompt) == "" {
		return influencerRecreateComposeAnswer{}, errors.New("提示词编译结果为空，请重试")
	}
	return parsed, nil
}

var (
	// influencerRecreateVideoRefs rejects any wording that would make the video
	// model look for a clip, a source edit or shared footage. This mode never
	// sends one, so a prompt that names one is malformed by construction.
	influencerRecreateVideoRefs = regexp.MustCompile(`(?i)\b(video|footage|original clip|reference clip)\b|原视频|参考视频|原片`)
	// influencerRecreateAdditiveText rejects prompts that ask the model to draw
	// captions, titles or logos instead of leaving them to post-production.
	influencerRecreateAdditiveText = regexp.MustCompile(`(?i)\b(add|adding|include|including|overlay|overlaying|with)\b[^.\n]{0,24}\b(captions?|subtitles?|title cards?|text overlay|logos?)\b`)
	// influencerRecreateCleanFrame requires the positive no-text constraint.
	influencerRecreateCleanFrame = regexp.MustCompile(`(?i)(free of|no)\s+[^.\n]{0,40}(captions?|lettering|text|logos?|watermarks?)`)
	influencerRecreateImageRef   = regexp.MustCompile(`(?i)\bimage\s*([0-9]{1,2})\b`)
)

// InfluencerRecreatePromptGate returns a short reason when a composed prompt is
// unusable, or an empty string when it may be billed.
func InfluencerRecreatePromptGate(prompt string, imageCount int) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "提示词为空"
	}
	if len([]rune(trimmed)) > InfluencerRecreateMaxPromptRunes {
		return fmt.Sprintf("提示词超过 %d 字符", InfluencerRecreateMaxPromptRunes)
	}
	if influencerRecreateVideoRefs.MatchString(trimmed) {
		return "提示词引用了视频素材"
	}
	if influencerRecreateAdditiveText.MatchString(trimmed) {
		return "提示词要求生成字幕或 logo"
	}
	if !influencerRecreateCleanFrame.MatchString(trimmed) {
		return "提示词缺少无字幕约束"
	}
	refs := influencerRecreateImageRef.FindAllStringSubmatch(trimmed, -1)
	if len(refs) == 0 {
		return "提示词没有引用参考图"
	}
	seenFirst := false
	for _, ref := range refs {
		index := 0
		for _, ch := range ref[1] {
			index = index*10 + int(ch-'0')
		}
		if index < 1 || index > imageCount {
			return "提示词引用了不存在的参考图"
		}
		if index == 1 {
			seenFirst = true
		}
	}
	if !seenFirst {
		return "提示词没有把 image 1 作为主体"
	}
	return ""
}

// influencerRecreateClip is the decomposition material: the clip-derived output
// length, the observed seconds and the extracted key frames.
type influencerRecreateClip struct {
	DurationSec      int
	ReferenceSeconds float64
	Frames           [][]byte
	FrameTimes       []float64
}

// influencerRecreatePlanFailure turns a plan error into the retryable timeout the
// handler answers with, so a slow supplier never becomes a proxy timeout.
func influencerRecreatePlanFailure(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return ErrInfluencerRecreateTimeout
	}
	return err
}

// influencerRecreatePrepareClip writes the downloaded bytes once, reads the clip's
// own picture duration (the generated length follows it; callers cannot choose it)
// and extracts timestamped key frames for the vision turn.
func influencerRecreatePrepareClip(parent context.Context, video []byte) (influencerRecreateClip, error) {
	directory, err := os.MkdirTemp("", "influencer-recreate-*")
	if err != nil {
		return influencerRecreateClip{}, errors.New("参考视频暂时无法读取，请重试")
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "reference")
	if err := os.WriteFile(path, video, 0o600); err != nil {
		return influencerRecreateClip{}, errors.New("参考视频暂时无法读取，请重试")
	}
	metadata, err := influencerVideoPreparationProbe(parent, path)
	if err != nil {
		return influencerRecreateClip{}, errors.New("参考视频无法解析，请换一段 4 到 30 秒、能正常播放的 mp4")
	}
	seconds := metadata.VideoDuration
	if seconds <= 0 {
		seconds = metadata.Duration
	}
	if seconds <= 0 {
		return influencerRecreateClip{}, errors.New("参考视频没有可读的画面时长，请换一段 4 到 30 秒的 mp4")
	}
	rounded := int(math.Round(seconds))
	if rounded < InfluencerRecreateMinSeconds {
		return influencerRecreateClip{}, fmt.Errorf("参考视频只有 %.1f 秒，请上传至少 %d 秒的视频", seconds, InfluencerRecreateMinSeconds)
	}
	if rounded > InfluencerRecreateMaxSeconds {
		return influencerRecreateClip{}, fmt.Errorf("参考视频有 %.1f 秒，当前最多复刻 %d 秒，请先剪辑到 %d 秒以内", seconds, InfluencerRecreateMaxSeconds, InfluencerRecreateMaxSeconds)
	}
	frames, times, err := influencerRecreateExtractFrames(parent, path, seconds)
	if err != nil {
		return influencerRecreateClip{}, err
	}
	return influencerRecreateClip{DurationSec: rounded, ReferenceSeconds: seconds, Frames: frames, FrameTimes: times}, nil
}

// influencerRecreateExtractFrames renders evenly spaced JPEG key frames. It
// tolerates a few unreadable positions (late keyframe or seek edge) and only
// fails when no frame is usable, because the breakdown needs at least a few.
func influencerRecreateExtractFrames(ctx context.Context, path string, seconds float64) ([][]byte, []float64, error) {
	tool, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, nil, errors.New("视频分析服务暂不可用，请稍后重试")
	}
	count := int(math.Round(seconds * influencerRecreateFrameSecondsPer))
	if count < influencerRecreateFrameMinCount {
		count = influencerRecreateFrameMinCount
	}
	if count > influencerRecreateFrameMaxCount {
		count = influencerRecreateFrameMaxCount
	}
	frames := make([][]byte, 0, count)
	times := make([]float64, 0, count)
	for index := 0; index < count; index++ {
		if ctx.Err() != nil {
			return nil, nil, context.DeadlineExceeded
		}
		timestamp := (float64(index) + 0.5) * seconds / float64(count)
		command := exec.CommandContext(ctx, tool,
			"-nostdin", "-v", "error",
			"-i", path,
			"-ss", strconv.FormatFloat(timestamp, 'f', 3, 64),
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", influencerRecreateFrameBox, influencerRecreateFrameBox),
			"-c:v", "mjpeg", "-q:v", "4",
			"-f", "image2pipe", "pipe:1",
		)
		output, runErr := command.Output()
		if runErr != nil || len(output) < 1024 {
			continue
		}
		frames = append(frames, output)
		times = append(times, timestamp)
	}
	if len(frames) == 0 {
		return nil, nil, errors.New("参考视频关键帧提取失败，请换一段视频重试")
	}
	return frames, times, nil
}
