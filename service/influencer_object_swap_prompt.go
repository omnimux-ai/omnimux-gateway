package service

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

//go:embed influencer_motion_prompts/object_swap.txt
var influencerObjectSwapSystem string

const (
	InfluencerObjectSwapPlannerModel = "gemini-4-argon"
	InfluencerObjectSwapPlannerGroup = "gemini-4-argon-multimodal"
	InfluencerObjectSwapPromptLimit  = 6000
	// fork: the understanding line reads ordered sample frames instead of one
	// inline clip. An 8.5 MiB 11-second clip becomes ~11 MiB of base64, and the
	// audited lines silently dropped inline video that large (task 907: the
	// usage breakdown carried no VIDEO tokens and the plan invented the scene,
	// including lighting the clip never had). Frames are small images, which
	// these lines do ingest.
	InfluencerObjectSwapFrameMaxCount = 8
	InfluencerObjectSwapFrameMinCount = 4
	InfluencerObjectSwapFrameBox      = 512
)

// Object-swap media transports. Frames are the default; the single inline clip
// is the fallback when frame rendering is unavailable.
const (
	influencerObjectSwapTransportFrames = "frames"
	influencerObjectSwapTransportVideo  = "video"
)

// ErrInfluencerObjectSwapUnobserved rejects a plan the media never reached: the
// provider's usage breakdown must name the modality actually sent. A plan built
// without the clip invents scene attributes, and the video model then follows
// that invented scene instead of the reference.
var ErrInfluencerObjectSwapUnobserved = errors.New("object swap understanding did not observe the reference video")

// InfluencerObjectSwapPlanInput keeps the original media order. VerifiedIdentity
// is optional trusted saved-character context for image 1; it does not assign
// that image an identity-replacement role in this particular edit.
type InfluencerObjectSwapPlanInput struct {
	VideoURL         string
	ImageURLs        []string
	ImageTags        []string
	UserPrompt       string
	DurationSec      int
	VerifiedIdentity string
}

// InfluencerObjectSwapPlanTrace is admin-only preparation evidence. A missing
// Usage means unknown, never zero. Outcome describes delivery, not billing.
type InfluencerObjectSwapPlanTrace struct {
	Model        string          `json:"model"`
	Group        string          `json:"group"`
	ChannelID    int             `json:"channel_id,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
	Outcome      string          `json:"outcome"`
	Transport    string          `json:"transport,omitempty"`
	Media        []string        `json:"media,omitempty"`
	Usage        json.RawMessage `json:"usage,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
}

type InfluencerObjectSwapPlan struct {
	Prompt string                        `json:"prompt,omitempty"`
	Audit  json.RawMessage               `json:"audit,omitempty"`
	Trace  InfluencerObjectSwapPlanTrace `json:"trace"`
}

type InfluencerObjectSwapChatResult struct {
	Content string
	Trace   InfluencerObjectSwapPlanTrace
}

// InfluencerObjectSwapChat is a one-call boundary, also used by offline tests.
var InfluencerObjectSwapChat = influencerObjectSwapChatViaChannel

// PlanInfluencerObjectSwapPrompt sends the actual video and every image in one
// request. Any failure leaves Prompt empty; callers must not submit a fallback.
func PlanInfluencerObjectSwapPrompt(parent context.Context, in InfluencerObjectSwapPlanInput) (InfluencerObjectSwapPlan, error) {
	plan := InfluencerObjectSwapPlan{Trace: InfluencerObjectSwapPlanTrace{
		Model: InfluencerObjectSwapPlannerModel, Group: InfluencerObjectSwapPlannerGroup, Outcome: "not_sent",
	}}
	if !strings.HasPrefix(in.VideoURL, "https://") || len(in.ImageURLs) == 0 || utf8.RuneCountInString(in.UserPrompt) > 4000 {
		return plan, errors.New("object swap plan input is incomplete or invalid")
	}
	for _, address := range in.ImageURLs {
		if !strings.HasPrefix(address, "https://") {
			return plan, errors.New("object swap reference address is not allowed")
		}
	}
	ctx, cancel := context.WithTimeout(parent, influencerMotionPlanTimeout)
	defer cancel()
	video, err := influencerMotionFetchMedia(ctx, in.VideoURL, InfluencerVideoMaxBytes)
	if err != nil {
		return plan, errors.New("object swap video is unavailable")
	}
	content, transport := influencerObjectSwapMediaBlocks(ctx, video, in.DurationSec)
	for i, address := range in.ImageURLs {
		image, err := influencerMotionFetchMedia(ctx, address, influencerMotionMaxFrameBytes)
		if err != nil {
			return plan, errors.New("object swap image is unavailable")
		}
		mime := http.DetectContentType(image)
		if !strings.HasPrefix(mime, "image/") {
			return plan, errors.New("object swap reference is not an image")
		}
		tag := ""
		if i < len(in.ImageTags) {
			tag = in.ImageTags[i]
		}
		label, err := common.Marshal(map[string]any{"image": i + 1, "label": tag})
		if err != nil {
			return plan, err
		}
		content = append(content,
			map[string]any{"type": "text", "text": fmt.Sprintf("@图片%d；附加标签仅供参考、不预设用途：%s", i+1, label)},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)}},
		)
	}
	var verifiedIdentity any
	if strings.TrimSpace(in.VerifiedIdentity) != "" {
		verifiedIdentity = map[string]any{"image": 1, "description": in.VerifiedIdentity}
	}
	intent, err := common.Marshal(map[string]any{
		"user_prompt":       in.UserPrompt,
		"verified_identity": verifiedIdentity,
	})
	if err != nil {
		return plan, err
	}
	content = append(content, map[string]any{"type": "text", "text": "用户原始要求与可选已核实身份如下。user_prompt 在对象替换范围内优先；verified_identity 的 image 明确标识身份资料所属图片，只核实来源，不赋予本次身份替换用途，也不覆盖用户明确要求。仅在该图片实际用于身份替换时补充，局部引用不扩大到身份替换：\n" + string(intent)})
	reply, err := InfluencerObjectSwapChat(ctx, influencerObjectSwapSystem, content)
	plan.Trace = reply.Trace
	if plan.Trace.Model == "" {
		plan.Trace.Model = InfluencerObjectSwapPlannerModel
	}
	if plan.Trace.Group == "" {
		plan.Trace.Group = InfluencerObjectSwapPlannerGroup
	}
	if plan.Trace.Outcome == "" {
		plan.Trace.Outcome = "unknown"
	}
	plan.Trace.Transport = transport
	if err != nil {
		return plan, err
	}
	// fork: a reply that never consumed the clip describes a scene the media
	// does not contain, and the submitted video model obeys that description.
	// Fail closed when the provider reports modalities without the one we sent.
	modalities, reported := influencerObjectSwapObservedModalities(plan.Trace.Usage)
	plan.Trace.Media = modalities
	if reported && !influencerObjectSwapSawMedia(modalities, transport) {
		return plan, ErrInfluencerObjectSwapUnobserved
	}
	auditRaw := influencerMotionStripFence(reply.Content)
	audit, err := validateInfluencerObjectSwapPlan(auditRaw, len(in.ImageURLs))
	if err != nil {
		return plan, err
	}
	plan.Audit = json.RawMessage(auditRaw)
	if audit.Status == "blocked" {
		return plan, fmt.Errorf("object swap planning blocked: %s", audit.Issue.Code)
	}
	plan.Prompt = audit.Prompt
	return plan, nil
}

// influencerObjectSwapMediaBlocks renders the reference clip for joint review.
// Frames keep the request small enough for the audited lines to ingest and carry
// their own timestamps, so the plan can anchor observations; a deployment
// without ffmpeg keeps the single inline clip and the modality gate above then
// requires the provider to report video tokens.
func influencerObjectSwapMediaBlocks(ctx context.Context, video []byte, seconds int) ([]map[string]any, string) {
	if frames, times, err := influencerObjectSwapFrameExtractor(ctx, video, seconds); err == nil && len(frames) > 0 && len(frames) == len(times) {
		blocks := []map[string]any{{
			"type": "text",
			"text": fmt.Sprintf("@视频1：待编辑原视频，时长 %d 秒；原音随原视频保留。以下为按时间顺序均匀抽样的 %d 帧，只代表这些时刻的画面，不是完整片段。以下全部素材须联合观察。", seconds, len(frames)),
		}}
		for index, frame := range frames {
			blocks = append(blocks,
				map[string]any{"type": "text", "text": fmt.Sprintf("@视频1 第%d帧（约 %.2f 秒）：", index+1, times[index])},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame)}},
			)
		}
		return blocks, influencerObjectSwapTransportFrames
	}
	return []map[string]any{
		{"type": "text", "text": fmt.Sprintf("@视频1：待编辑原视频，时长 %d 秒；原音随原视频保留。以下全部素材须联合观察。", seconds)},
		{"type": "file", "file": map[string]any{"filename": "reference.mp4", "file_data": "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(video)}},
	}, influencerObjectSwapTransportVideo
}

// influencerObjectSwapFrameExtractor renders ordered JPEG sample frames from
// the fetched clip. Tests replace it to stay tool-free.
var influencerObjectSwapFrameExtractor = influencerObjectSwapExtractFrames

// influencerObjectSwapSampleCount keeps roughly one frame per second, within
// four to eight frames, so a short edit still shows its beats.
func influencerObjectSwapSampleCount(seconds int) int {
	count := seconds
	if count < InfluencerObjectSwapFrameMinCount {
		count = InfluencerObjectSwapFrameMinCount
	}
	if count > InfluencerObjectSwapFrameMaxCount {
		count = InfluencerObjectSwapFrameMaxCount
	}
	return count
}

// influencerObjectSwapExtractFrames decodes the clip once into evenly spaced
// frames. An unreadable position is skipped; only a clip with no usable frame
// fails, which lets the caller fall back to the inline clip.
func influencerObjectSwapExtractFrames(ctx context.Context, video []byte, seconds int) ([][]byte, []float64, error) {
	if seconds <= 0 || len(video) == 0 {
		return nil, nil, errors.New("reference video is empty")
	}
	tool, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, nil, errors.New("video frame rendering is unavailable")
	}
	file, err := os.CreateTemp("", "omnimux-object-swap-*.mp4")
	if err != nil {
		return nil, nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err = file.Write(video); err != nil {
		file.Close()
		return nil, nil, err
	}
	if err = file.Close(); err != nil {
		return nil, nil, err
	}
	count := influencerObjectSwapSampleCount(seconds)
	frames := make([][]byte, 0, count)
	times := make([]float64, 0, count)
	for index := 0; index < count; index++ {
		if ctx.Err() != nil {
			return nil, nil, context.DeadlineExceeded
		}
		timestamp := (float64(index) + 0.5) * float64(seconds) / float64(count)
		command := exec.CommandContext(ctx, tool,
			"-nostdin", "-v", "error",
			"-i", path,
			"-ss", strconv.FormatFloat(timestamp, 'f', 3, 64),
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", InfluencerObjectSwapFrameBox, InfluencerObjectSwapFrameBox),
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
		return nil, nil, errors.New("reference video frames are unreadable")
	}
	return frames, times, nil
}

// influencerObjectSwapObservedModalities reads the provider's own breakdown of
// what it consumed. reported=false means the line returned no modality list, so
// coverage stays unknown instead of being claimed either way.
func influencerObjectSwapObservedModalities(usage json.RawMessage) ([]string, bool) {
	if len(usage) == 0 {
		return nil, false
	}
	var parsed struct {
		PromptTokensDetails []struct {
			Modality   string `json:"modality"`
			TokenCount int    `json:"tokenCount"`
		} `json:"promptTokensDetails"`
	}
	if common.Unmarshal(usage, &parsed) != nil || len(parsed.PromptTokensDetails) == 0 {
		return nil, false
	}
	modalities := make([]string, 0, len(parsed.PromptTokensDetails))
	for _, detail := range parsed.PromptTokensDetails {
		name := strings.ToUpper(strings.TrimSpace(detail.Modality))
		if name == "" || detail.TokenCount <= 0 {
			continue
		}
		modalities = append(modalities, name)
	}
	return modalities, true
}

// influencerObjectSwapSawMedia reports whether the reported modalities include
// the transport we actually sent.
func influencerObjectSwapSawMedia(modalities []string, transport string) bool {
	required := "IMAGE"
	if transport == influencerObjectSwapTransportVideo {
		required = "VIDEO"
	}
	for _, modality := range modalities {
		if modality == required {
			return true
		}
	}
	return false
}

type influencerObjectSwapAudit struct {
	Status string `json:"status"`
	Issue  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"issue"`
	Edits []struct {
		Images     []int  `json:"images"`
		Target     string `json:"target"`
		Scope      string `json:"scope"`
		Appearance string `json:"appearance"`
		Remove     string `json:"remove"`
		Physics    string `json:"physics"`
	} `json:"edits"`
	UnusedImages []struct {
		Image  int    `json:"image"`
		Reason string `json:"reason"`
	} `json:"unused_images"`
	Preserve string `json:"preserve"`
	Prompt   string `json:"prompt"`
}

// objectSwapJSONShape rejects missing, extra, null and wrongly typed fields.
// The values name JSON types, keeping validation on the host JSON codec.
func objectSwapJSONShape(raw []byte, fields map[string]string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := common.Unmarshal(raw, &object); err != nil || len(object) != len(fields) {
		return nil, errors.New("object swap planner returned an invalid result shape")
	}
	for key, kind := range fields {
		value, ok := object[key]
		if !ok || common.GetJsonType(value) != kind {
			return nil, errors.New("object swap planner returned an invalid result shape")
		}
	}
	return object, nil
}

var objectSwapImageReference = regexp.MustCompile(`@图片([0-9]+)`)
var objectSwapVideoReference = regexp.MustCompile(`@视频([0-9]+)`)

func validateInfluencerObjectSwapPlan(raw string, imageCount int) (influencerObjectSwapAudit, error) {
	var result influencerObjectSwapAudit
	invalid := errors.New("object swap planner returned no usable prompt")
	object, err := objectSwapJSONShape([]byte(raw), map[string]string{
		"status": "string", "issue": "object", "edits": "array", "unused_images": "array", "preserve": "string", "prompt": "string",
	})
	if err != nil {
		return result, err
	}
	if _, err = objectSwapJSONShape(object["issue"], map[string]string{"code": "string", "message": "string"}); err != nil {
		return result, err
	}
	var edits, unused []json.RawMessage
	if common.Unmarshal(object["edits"], &edits) != nil || common.Unmarshal(object["unused_images"], &unused) != nil {
		return result, invalid
	}
	for _, edit := range edits {
		if _, err = objectSwapJSONShape(edit, map[string]string{"images": "array", "target": "string", "scope": "string", "appearance": "string", "remove": "string", "physics": "string"}); err != nil {
			return result, err
		}
	}
	for _, entry := range unused {
		if _, err = objectSwapJSONShape(entry, map[string]string{"image": "number", "reason": "string"}); err != nil {
			return result, err
		}
	}
	if err = common.UnmarshalJsonStr(raw, &result); err != nil {
		return result, invalid
	}
	if result.Status == "blocked" {
		if result.Prompt != "" || result.Preserve != "" || len(result.Edits) != 0 || len(result.UnusedImages) != 0 || strings.TrimSpace(result.Issue.Message) == "" {
			return result, invalid
		}
		switch result.Issue.Code {
		case "unreadable_media", "no_match", "out_of_scope":
			return result, nil
		default:
			return result, invalid
		}
	}
	if result.Status != "ready" || result.Issue.Code != "" || result.Issue.Message != "" || len(result.Edits) == 0 || strings.TrimSpace(result.Preserve) == "" || !strings.HasPrefix(result.Prompt, "编辑@视频1") || utf8.RuneCountInString(result.Prompt) > InfluencerObjectSwapPromptLimit {
		return result, invalid
	}
	used, omitted := make(map[int]bool), make(map[int]bool)
	for _, edit := range result.Edits {
		if len(edit.Images) == 0 || strings.TrimSpace(edit.Target) == "" || strings.TrimSpace(edit.Scope) == "" || strings.TrimSpace(edit.Appearance) == "" {
			return result, invalid
		}
		withinEdit := make(map[int]bool)
		for _, image := range edit.Images {
			if image <= 0 || image > imageCount || withinEdit[image] {
				return result, invalid
			}
			withinEdit[image], used[image] = true, true
		}
	}
	for _, entry := range result.UnusedImages {
		if entry.Image <= 0 || entry.Image > imageCount || strings.TrimSpace(entry.Reason) == "" || used[entry.Image] || omitted[entry.Image] {
			return result, invalid
		}
		omitted[entry.Image] = true
	}
	if len(used)+len(omitted) != imageCount {
		return result, invalid
	}
	referenced := make(map[int]bool)
	for _, match := range objectSwapImageReference.FindAllStringSubmatch(result.Prompt, -1) {
		image, err := strconv.Atoi(match[1])
		if err != nil || !used[image] {
			return result, invalid
		}
		referenced[image] = true
	}
	if len(referenced) != len(used) {
		return result, invalid
	}
	for _, match := range objectSwapVideoReference.FindAllStringSubmatch(result.Prompt, -1) {
		if match[1] != "1" {
			return result, invalid
		}
	}
	return result, nil
}

// influencerObjectSwapReplyUsable rejects a reply that is not a JSON object so
// the transport can try the next audited line instead of failing the plan. A
// single fenced JSON reply is usable; arbitrary prose around the JSON is not.
func influencerObjectSwapReplyUsable(reply string) error {
	var object map[string]json.RawMessage
	if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &object); err != nil || len(object) == 0 {
		return errors.New("no usable object swap result")
	}
	return nil
}

// influencerObjectSwapChatViaChannel routes the joint understanding request
// through the shared native Gemini transport. The dedicated object-swap route
// uses the same audited supplier group as the motion planner's multimodal calls.
func influencerObjectSwapChatViaChannel(ctx context.Context, system string, content []map[string]any) (InfluencerObjectSwapChatResult, error) {
	exchange, err := influencerPlannerChatViaChannel(ctx, InfluencerObjectSwapPlannerModel, InfluencerObjectSwapPlannerGroup, system, content, influencerObjectSwapReplyUsable, nil)
	result := InfluencerObjectSwapChatResult{
		Content: exchange.Text,
		Trace: InfluencerObjectSwapPlanTrace{
			Model:        InfluencerObjectSwapPlannerModel,
			Group:        InfluencerObjectSwapPlannerGroup,
			ChannelID:    exchange.ChannelID,
			RequestID:    exchange.RequestID,
			Usage:        exchange.Usage,
			FinishReason: exchange.FinishReason,
			Outcome:      exchange.Outcome,
		},
	}
	if result.Trace.Outcome == "" {
		result.Trace.Outcome = "unknown"
	}
	return result, err
}
