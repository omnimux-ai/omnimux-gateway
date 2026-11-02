package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// fork: influencer motion prompt planning. Before a queued motion task is sent,
// a vision model reads the reference clip (reverse) and then writes the final
// Seedance prompt from that breakdown, the labelled reference images and the
// caller's own words (rewrite). The caller's intent ranks first; the system only
// completes what the caller left out and keeps one fixed section structure.
// Any failure returns an error and the caller keeps its fixed framing.

//go:embed influencer_motion_prompts/reverse.txt
var influencerMotionReverseSystem string

//go:embed influencer_motion_prompts/rewrite.txt
var influencerMotionRewriteSystem string

const (
	// InfluencerMotionPlannerModel and InfluencerMotionPlannerGroup name the
	// internal vision call. Internal model requests always carry an explicit
	// supplier-aligned group. That group binds the audited Gemini 4 Argon lines
	// and its channel priority is the cheap-first order, so the selector's retry
	// index is the line order (0 = cheapest).
	InfluencerMotionPlannerModel = "gemini-4-argon"
	InfluencerMotionPlannerGroup = "gemini-4-argon-multimodal"

	// A native Argon turn with an inline reference clip is measured in minutes,
	// not seconds (the audited production acceptance took ~117 s for one video
	// turn), and the cheapest line can be much slower. Planning runs two turns in
	// the queue after the receipt, and a failed tier is remembered per plan so the
	// rewrite does not pay for it again. The user is not blocked on this budget
	// because the worker, not the submit request, waits.
	influencerMotionPlanTimeout   = 600 * time.Second
	influencerMotionMaxFrameBytes = 8 * 1024 * 1024
	influencerMotionMaxReplyBytes = 2 * 1024 * 1024

	// influencerMotionPlannerMaxRoutes bounds the audited four-line policy. The
	// planner never invents a cheaper line: the group priority already is the
	// price order.
	influencerMotionPlannerMaxRoutes       = 4
	influencerMotionPlannerMaxOutputTokens = 8000
)

// InfluencerMotionPlanInput is everything the planner may read. VideoURL is
// the frozen reference clip; images keep the caller's order (image 1 = 图片1).
type InfluencerMotionPlanInput struct {
	VideoURL    string
	DurationSec int
	ImageURLs   []string
	ImageTags   []string
	UserPrompt  string
}

// InfluencerMotionPlan is the planner's accepted result.
type InfluencerMotionPlan struct {
	Prompt          string
	NegationRemoved bool
}

// influencerMotionPlanSession remembers the priority tiers that failed within
// one plan, so the rewrite turn does not spend its budget on a line the reverse
// turn already rejected.
type influencerMotionPlanSession struct {
	failed map[int64]bool
}

func newInfluencerMotionPlanSession() *influencerMotionPlanSession {
	return &influencerMotionPlanSession{failed: make(map[int64]bool)}
}

// InfluencerMotionChat sends one planner turn through the native content
// interface and returns the assistant text. The optional check rejects an
// unusable answer so the transport can try the next audited line. It is a
// variable so tests can supply a local fake instead of a provider.
var InfluencerMotionChat = influencerMotionChatViaChannel

// influencerMotionSelectChannel picks one planner line; the retry index selects
// the priority tier (cheapest first). Tests replace it to stay provider-free.
var influencerMotionSelectChannel = model.GetRandomSatisfiedChannel

// influencerMotionFetchMedia downloads one reference; tests replace it.
var influencerMotionFetchMedia = influencerMotionFetch

// influencerMotionReverseReplyUsable rejects a reply with a different shape so
// the transport can try the next audited line instead of using it as a breakdown.
func influencerMotionReverseReplyUsable(reply string) error {
	var parsed map[string]any
	if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &parsed); err != nil || parsed["timeline"] == nil {
		return errors.New("no usable breakdown")
	}
	return nil
}

// influencerMotionRewriteReplyUsable requires a non-empty planned prompt.
func influencerMotionRewriteReplyUsable(reply string) error {
	var rewrite struct {
		FinalPrompt string `json:"final_prompt"`
	}
	if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &rewrite); err != nil || strings.TrimSpace(rewrite.FinalPrompt) == "" {
		return errors.New("no usable prompt")
	}
	return nil
}

// PlanInfluencerMotionPrompt runs reverse then rewrite and checks the result.
func PlanInfluencerMotionPrompt(parent context.Context, in InfluencerMotionPlanInput) (InfluencerMotionPlan, error) {
	ctx, cancel := context.WithTimeout(parent, influencerMotionPlanTimeout)
	defer cancel()
	if !strings.HasPrefix(in.VideoURL, "https://") || len(in.ImageURLs) == 0 {
		return InfluencerMotionPlan{}, errors.New("motion plan input is incomplete")
	}
	video, err := influencerMotionFetchMedia(ctx, in.VideoURL, InfluencerVideoMaxBytes)
	if err != nil {
		return InfluencerMotionPlan{}, err
	}
	reverseContent := []map[string]any{
		{"type": "text", "text": fmt.Sprintf("参考视频文件，时长 %d 秒。动作、时间和节拍以视频为准。", in.DurationSec)},
		{"type": "file", "file": map[string]any{"filename": "reference.mp4", "file_data": "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(video)}},
	}
	session := newInfluencerMotionPlanSession()
	breakdown, err := InfluencerMotionChat(ctx, influencerMotionReverseSystem, reverseContent, influencerMotionReverseReplyUsable, session)
	if err != nil {
		return InfluencerMotionPlan{}, fmt.Errorf("motion reverse failed: %w", err)
	}
	breakdown = influencerMotionStripFence(breakdown)
	var parsed map[string]any
	if err := common.UnmarshalJsonStr(breakdown, &parsed); err != nil || parsed["timeline"] == nil {
		return InfluencerMotionPlan{}, errors.New("motion reverse returned no usable breakdown")
	}

	rewriteContent := []map[string]any{
		{"type": "text", "text": "参考视频结构化拆解：\n" + breakdown},
		{"type": "text", "text": "生成时的视频素材：视频1 = 参考原视频本身（含原音）。参考中的字幕和后期叠层不属于场景元素，禁止写进最终提示词。"},
	}
	for i, url := range in.ImageURLs {
		tag := "未标注"
		if i == 0 {
			tag = "角色"
		}
		if i < len(in.ImageTags) && strings.TrimSpace(in.ImageTags[i]) != "" {
			tag = strings.TrimSpace(in.ImageTags[i])
		}
		image, err := influencerMotionFetchMedia(ctx, url, influencerMotionMaxFrameBytes)
		if err != nil {
			return InfluencerMotionPlan{}, err
		}
		rewriteContent = append(rewriteContent,
			map[string]any{"type": "text", "text": fmt.Sprintf("图片%d（标签：%s）：", i+1, tag)},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + http.DetectContentType(image) + ";base64," + base64.StdEncoding.EncodeToString(image)}},
		)
	}
	userPrompt := strings.TrimSpace(in.UserPrompt)
	if userPrompt == "" {
		userPrompt = "无（按规则补全）"
	}
	rewriteContent = append(rewriteContent, map[string]any{"type": "text", "text": "用户要求：" + userPrompt})
	reply, err := InfluencerMotionChat(ctx, influencerMotionRewriteSystem, rewriteContent, influencerMotionRewriteReplyUsable, session)
	if err != nil {
		return InfluencerMotionPlan{}, fmt.Errorf("motion rewrite failed: %w", err)
	}
	var rewrite struct {
		FinalPrompt string `json:"final_prompt"`
	}
	if err := common.UnmarshalJsonStr(influencerMotionStripFence(reply), &rewrite); err != nil {
		return InfluencerMotionPlan{}, errors.New("motion rewrite returned no usable prompt")
	}
	prompt, removed, err := GuardInfluencerMotionPrompt(rewrite.FinalPrompt)
	if err != nil {
		return InfluencerMotionPlan{}, err
	}
	return InfluencerMotionPlan{Prompt: prompt, NegationRemoved: removed}, nil
}

var (
	// influencerMotionNegation matches clauses that name and negate a belonging
	// ("不佩戴X", "无X配饰"): naming a forbidden item re-introduces it.
	influencerMotionNegation  = regexp.MustCompile(`[，,；;、]?\s*(?:全身)?(?:不佩戴|不戴|不穿|无)[^，,。；;\n【】]*(?:墨镜|眼镜|配饰|饰品|帽|首饰)[^，,。；;\n【】]*`)
	influencerMotionEditWords = []string{"替换", "换成", "改成", "修改", "删除", "去掉", "编辑", "原来的"}
)

// GuardInfluencerMotionPrompt removes named negations and rejects prompts that
// would read as a video edit or break the fixed section order.
func GuardInfluencerMotionPrompt(prompt string) (string, bool, error) {
	cleaned := influencerMotionNegation.ReplaceAllString(strings.TrimSpace(prompt), "")
	removed := cleaned != strings.TrimSpace(prompt)
	if cleaned == "" || len([]rune(cleaned)) > 6000 {
		return "", removed, errors.New("motion prompt is empty or too long")
	}
	for _, word := range influencerMotionEditWords {
		if strings.Contains(cleaned, word) {
			// Name the matched word: the prompt text itself is user-derived and
			// stays out of the log, but the word is what makes a fallback
			// diagnosable without reproducing the plan.
			return "", removed, fmt.Errorf("motion prompt contains edit wording %q", word)
		}
	}
	subject, scene := strings.Index(cleaned, "【主体】"), strings.Index(cleaned, "【场景】")
	if subject < 0 || scene < 0 || subject > scene {
		return "", removed, errors.New("motion prompt sections are out of order")
	}
	return cleaned, removed, nil
}

func influencerMotionStripFence(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	return strings.TrimSpace(strings.TrimSuffix(text, "```"))
}

func influencerMotionFetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	if err := ValidateSSRFProtectedFetchURL(url); err != nil {
		return nil, errors.New("motion reference address is not allowed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return nil, errors.New("motion reference is unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("motion reference is unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(data) == 0 || int64(len(data)) > limit {
		return nil, errors.New("motion reference is empty or too large")
	}
	return data, nil
}

// influencerPlannerChatResult is one native planner exchange: the answer text
// plus the admin-only delivery evidence both planners record.
type influencerPlannerChatResult struct {
	Text         string
	ChannelID    int
	RequestID    string
	Usage        json.RawMessage
	FinishReason string
	Outcome      string
}

// influencerMotionChatViaChannel is the motion planner's thin wrapper over the
// shared native planner transport; it keeps motion's own model, group and shape
// checks unchanged.
func influencerMotionChatViaChannel(ctx context.Context, system string, content []map[string]any, check func(string) error, session *influencerMotionPlanSession) (string, error) {
	result, err := influencerPlannerChatViaChannel(ctx, InfluencerMotionPlannerModel, InfluencerMotionPlannerGroup, system, content, check, session)
	return result.Text, err
}

// influencerPlannerChatViaChannel calls one planner model on the first line of
// its audited group that can take it. Lines are bound to the audited Gemini 4
// Argon group, whose channel priority is the cheap-first order, so attempt 0 is
// the cheapest line. A hand-off happens when the attempt definitely produced no
// plan: a locally unusable key, a complete authentication rejection, a gateway
// rejection (429/5xx), or a 200 reply that fails the caller's shape check. These
// internal calls are unbilled and produce no user-visible side effect, so a retry
// only risks absorbed supplier cost. Transport failures and timeouts stay unsafe
// to replay and stop the plan; the caller then keeps its fixed framing.
func influencerPlannerChatViaChannel(ctx context.Context, modelName, group, system string, content []map[string]any, check func(string) error, session *influencerMotionPlanSession) (influencerPlannerChatResult, error) {
	result := influencerPlannerChatResult{Outcome: "not_sent"}
	parts, err := influencerMotionNativeParts(content)
	if err != nil {
		return result, err
	}
	var lastErr error
	seenPriority := make(map[int64]bool, influencerMotionPlannerMaxRoutes)
	for attempt := 0; attempt < influencerMotionPlannerMaxRoutes; attempt++ {
		channel, selectErr := influencerMotionSelectChannel(group, modelName, attempt, nil)
		if selectErr != nil {
			return result, selectErr
		}
		if channel == nil {
			break
		}
		priority := channel.GetPriority()
		if seenPriority[priority] {
			// The selector clamps past the last tier; no cheaper line is left.
			break
		}
		seenPriority[priority] = true
		if session != nil && session.failed[priority] {
			continue
		}
		result.ChannelID = channel.Id
		key, _, keyErr := channel.GetNextEnabledKey()
		if keyErr != nil || key == "" {
			lastErr = errors.New("planner channel has no usable key")
			if session != nil {
				session.failed[priority] = true
			}
			continue
		}
		started := time.Now()
		exchange, handOff, callErr := influencerPlannerNativeGenerate(ctx, modelName, channel, key, system, parts)
		if callErr == nil {
			if check != nil {
				if checkErr := check(exchange.Text); checkErr != nil {
					// A line that answers 200 with the wrong shape never becomes the
					// plan; try the next audited line. Only absorbed supplier cost is
					// at risk for this internal call.
					lastErr = fmt.Errorf("planner reply on channel %d was unusable: %w", channel.Id, checkErr)
					result = exchange
					if session != nil {
						session.failed[priority] = true
					}
					continue
				}
			}
			// Ops evidence: which audited line produced the plan and how long the
			// native turn took. Without it a success leaves no trace because these
			// internal calls are not billed or logged as relay requests.
			common.SysLog(common.LogText("influencer planner used channel %d in %s", channel.Id, time.Since(started).Round(time.Millisecond)))
			return exchange, nil
		}
		result = exchange
		lastErr = callErr
		if !handOff {
			return result, callErr
		}
		if session != nil {
			session.failed[priority] = true
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no planner channel is available")
	}
	return result, lastErr
}

// influencerPlannerNativeGenerate sends one native generateContent request to the
// selected line and reports whether its failure is safe to hand off. The exchange
// evidence is filled from the native response so both planners can record it.
func influencerPlannerNativeGenerate(ctx context.Context, modelName string, channel *model.Channel, key, system string, parts []map[string]any) (influencerPlannerChatResult, bool, error) {
	result := influencerPlannerChatResult{ChannelID: channel.Id, Outcome: "not_sent"}
	body, err := common.Marshal(map[string]any{
		"systemInstruction": map[string]any{"parts": []map[string]any{{"text": system}}},
		"contents":          []map[string]any{{"role": "user", "parts": parts}},
		"generationConfig": map[string]any{
			"maxOutputTokens": influencerMotionPlannerMaxOutputTokens,
			// Low effort keeps the planner inside the queue's time budget; thinking
			// tokens still count against maxOutputTokens.
			"thinkingConfig": map[string]any{"thinkingLevel": "low"},
		},
	})
	if err != nil {
		return result, false, err
	}
	endpoint := strings.TrimRight(channel.GetBaseURL(), "/") + "/v1beta/models/" + modelName + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, false, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("x-goog-api-key", key)
	req.Header.Set("Content-Type", "application/json")
	client, err := GetHttpClientWithProxySettings(channel.GetSetting().Proxy, channel.GetSetting())
	if err != nil {
		return result, false, err
	}
	// No retry: after handing off the request, a transport error is result-unknown.
	result.Outcome = "unknown"
	resp, err := client.Do(req)
	if err != nil {
		return result, false, fmt.Errorf("planner request failed on channel %d: %w", channel.Id, err)
	}
	defer resp.Body.Close()
	result.RequestID = resp.Header.Get("X-Request-Id")
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, influencerMotionMaxReplyBytes))
	if readErr != nil {
		return result, false, fmt.Errorf("planner reply is unreadable: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		// A gateway rejection (rate limit or 5xx) returns before a usable answer and
		// is safe to hand off for an internal, side-effect-free planner call. Other
		// 4xx responses mean the request shape itself is wrong, so they stop.
		handOff := isArgonNativeRejection(resp.StatusCode, raw) ||
			resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode >= http.StatusInternalServerError
		result.Outcome = "rejected"
		return result, handOff, fmt.Errorf("planner answered status %d on channel %d", resp.StatusCode, channel.Id)
	}
	reply := influencerPlannerNativeReply(raw)
	result.Outcome = "received"
	if reply.ResponseID != "" {
		result.RequestID = reply.ResponseID
	}
	result.Usage = reply.Usage
	result.FinishReason = reply.FinishReason
	if reply.Text == "" {
		return result, false, errors.New("planner returned no content")
	}
	result.Text = reply.Text
	return result, false, nil
}

type influencerPlannerNativeReplyBody struct {
	Text         string
	FinishReason string
	ResponseID   string
	Usage        json.RawMessage
}

// influencerPlannerNativeReply reads the answer text and delivery evidence of
// the first candidate. Thinking parts are excluded so the planner only reads the
// final answer.
func influencerPlannerNativeReply(raw []byte) influencerPlannerNativeReplyBody {
	var reply struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		ResponseID    string          `json:"responseId"`
		UsageMetadata json.RawMessage `json:"usageMetadata"`
	}
	var result influencerPlannerNativeReplyBody
	if common.Unmarshal(raw, &reply) != nil || len(reply.Candidates) == 0 {
		return result
	}
	result.ResponseID = reply.ResponseID
	result.FinishReason = reply.Candidates[0].FinishReason
	if len(reply.UsageMetadata) != 0 && string(reply.UsageMetadata) != "null" {
		result.Usage = reply.UsageMetadata
	}
	var text strings.Builder
	for _, part := range reply.Candidates[0].Content.Parts {
		if part.Thought {
			continue
		}
		text.WriteString(part.Text)
	}
	result.Text = strings.TrimSpace(text.String())
	return result
}

// influencerMotionNativeParts translates the planner's neutral blocks into
// native Gemini parts. Callers keep building text, the clip and labelled images;
// only the wire shape differs from a chat completion.
func influencerMotionNativeParts(content []map[string]any) ([]map[string]any, error) {
	parts := make([]map[string]any, 0, len(content))
	for _, block := range content {
		switch block["type"] {
		case "text":
			text, _ := block["text"].(string)
			parts = append(parts, map[string]any{"text": text})
		case "file":
			file, _ := block["file"].(map[string]any)
			raw, _ := file["file_data"].(string)
			part, err := influencerMotionInlinePart(raw)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case "image_url":
			image, _ := block["image_url"].(map[string]any)
			raw, _ := image["url"].(string)
			part, err := influencerMotionInlinePart(raw)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		default:
			return nil, errors.New("planner content block is unsupported")
		}
	}
	if len(parts) == 0 {
		return nil, errors.New("planner content is empty")
	}
	return parts, nil
}

// influencerMotionInlinePart turns one inline data URL into a native inlineData
// part. Only image and video payloads reach the planner.
func influencerMotionInlinePart(raw string) (map[string]any, error) {
	const prefix = "data:"
	if !strings.HasPrefix(raw, prefix) {
		return nil, errors.New("planner media is not inline")
	}
	header, data, found := strings.Cut(raw[len(prefix):], ",")
	if !found || !strings.HasSuffix(header, ";base64") {
		return nil, errors.New("planner media is not base64 encoded")
	}
	mimeType := strings.TrimSuffix(header, ";base64")
	if !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "video/") {
		return nil, errors.New("planner media type is unsupported")
	}
	return map[string]any{"inlineData": map[string]any{"mimeType": mimeType, "data": data}}, nil
}
