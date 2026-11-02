package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const objectSwapReady = `{"status":"ready","issue":{"code":"","message":""},"edits":[{"images":[1],"target":"原片左侧的容器","scope":"该容器的整体外观","appearance":"参考图中可见的外观","remove":"原容器的冲突外观","physics":"保持原有接触与遮挡"}],"unused_images":[],"preserve":"原音、动作、镜头及右侧容器保持不变","prompt":"编辑@视频1，将原片左侧容器整体替换为@图片1提供的外观，保留右侧容器与原音，维持原有接触及遮挡。"}`

func TestPlanInfluencerObjectSwapJointMediaAndOriginalIntent(t *testing.T) {
	previousChat, previousFetch := InfluencerObjectSwapChat, influencerMotionFetchMedia
	t.Cleanup(func() { InfluencerObjectSwapChat, influencerMotionFetchMedia = previousChat, previousFetch })
	stubObjectSwapFrames(t, nil, nil, errors.New("frame rendering unavailable"))
	media := map[string][]byte{
		"https://files.example/source.mp4": []byte("the actual original video"),
		"https://files.example/a.png":      []byte("\x89PNG\r\n\x1a\nfirst-image"),
		"https://files.example/b.png":      []byte("\x89PNG\r\n\x1a\nsecond-image"),
	}
	var fetched []string
	influencerMotionFetchMedia = func(_ context.Context, address string, _ int64) ([]byte, error) {
		fetched = append(fetched, address)
		return media[address], nil
	}
	userIntent := "只替换左边的容器，\n保留图上的\"文字\"。"
	var calls int
	InfluencerObjectSwapChat = func(_ context.Context, system string, content []map[string]any) (InfluencerObjectSwapChatResult, error) {
		calls++
		assert.Contains(t, system, "边界内，以用户明确要求")
		assert.Contains(t, system, "不得因为常见歧义而回避决策")
		require.Len(t, content, 7)
		file := content[1]["file"].(map[string]any)
		assert.Equal(t, "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString(media["https://files.example/source.mp4"]), file["file_data"])
		for i, address := range []string{"https://files.example/a.png", "https://files.example/b.png"} {
			image := content[3+i*2]["image_url"].(map[string]any)
			assert.Equal(t, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(media[address]), image["url"])
		}
		assert.Contains(t, content[2]["text"], "@图片1")
		assert.NotContains(t, content[2]["text"], "角色")
		text := content[6]["text"].(string)
		var intent struct {
			UserPrompt       string `json:"user_prompt"`
			VerifiedIdentity struct {
				Image       int    `json:"image"`
				Description string `json:"description"`
			} `json:"verified_identity"`
		}
		require.NoError(t, common.UnmarshalJsonStr(text[strings.Index(text, "\n")+1:], &intent))
		assert.Equal(t, userIntent, intent.UserPrompt)
		assert.Equal(t, 1, intent.VerifiedIdentity.Image)
		assert.Equal(t, "已核实身份资料", intent.VerifiedIdentity.Description)
		assert.Contains(t, text, "只核实来源，不赋予本次身份替换用途")
		assert.Contains(t, text, "不覆盖用户明确要求")
		result := strings.Replace(objectSwapReady, `"unused_images":[]`, `"unused_images":[{"image":2,"reason":"未用于本次替换"}]`, 1)
		return InfluencerObjectSwapChatResult{Content: result, Trace: InfluencerObjectSwapPlanTrace{Outcome: "received", RequestID: "joint-1", Usage: json.RawMessage(`{"total_tokens":123}`)}}, nil
	}
	plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{
		VideoURL: "https://files.example/source.mp4", DurationSec: 5,
		ImageURLs: []string{"https://files.example/a.png", "https://files.example/b.png"},
		ImageTags: []string{"", "自定义标签"}, UserPrompt: userIntent, VerifiedIdentity: "已核实身份资料",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Len(t, fetched, 3)
	assert.Contains(t, plan.Prompt, "编辑@视频1")
	assert.Contains(t, string(plan.Audit), "unused_images")
	assert.Equal(t, "joint-1", plan.Trace.RequestID)
	assert.JSONEq(t, `{"total_tokens":123}`, string(plan.Trace.Usage))
}

func TestObjectSwapWithoutVerifiedIdentityDoesNotInventImageRole(t *testing.T) {
	stubMotionMedia(t)
	stubObjectSwapFrames(t, nil, nil, errors.New("frame rendering unavailable"))
	previousChat := InfluencerObjectSwapChat
	t.Cleanup(func() { InfluencerObjectSwapChat = previousChat })
	userPrompt := "只使用图片中的衣服，不替换人物。"
	InfluencerObjectSwapChat = func(_ context.Context, _ string, content []map[string]any) (InfluencerObjectSwapChatResult, error) {
		text := content[len(content)-1]["text"].(string)
		var intent map[string]json.RawMessage
		require.NoError(t, common.UnmarshalJsonStr(text[strings.Index(text, "\n")+1:], &intent))
		assert.Equal(t, "null", string(intent["verified_identity"]))
		var originalPrompt string
		require.NoError(t, common.Unmarshal(intent["user_prompt"], &originalPrompt))
		assert.Equal(t, userPrompt, originalPrompt)
		return InfluencerObjectSwapChatResult{Content: objectSwapReady, Trace: InfluencerObjectSwapPlanTrace{Outcome: "received"}}, nil
	}
	_, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{
		VideoURL: "https://files.example/a.mp4", ImageURLs: []string{"https://files.example/a.png"}, UserPrompt: userPrompt,
	})
	require.NoError(t, err)
}

func TestObjectSwapPlanValidationProtectsMediaAccountingAndScope(t *testing.T) {
	tests := map[string]func(map[string]any){
		"unknown top field":              func(v map[string]any) { v["confidence"] = 0.5 },
		"missing field":                  func(v map[string]any) { delete(v, "preserve") },
		"null field":                     func(v map[string]any) { v["preserve"] = nil },
		"missing edit appearance":        func(v map[string]any) { delete(v["edits"].([]any)[0].(map[string]any), "appearance") },
		"unreadable null edit field":     func(v map[string]any) { v["edits"].([]any)[0].(map[string]any)["remove"] = nil },
		"out of range image":             func(v map[string]any) { v["edits"].([]any)[0].(map[string]any)["images"] = []int{2} },
		"image zero":                     func(v map[string]any) { v["edits"].([]any)[0].(map[string]any)["images"] = []int{0} },
		"fractional image":               func(v map[string]any) { v["edits"].([]any)[0].(map[string]any)["images"] = []float64{1.5} },
		"duplicate within edit":          func(v map[string]any) { v["edits"].([]any)[0].(map[string]any)["images"] = []int{1, 1} },
		"used and unused":                func(v map[string]any) { v["unused_images"] = []any{map[string]any{"image": 1, "reason": "冲突"}} },
		"unmentioned used image":         func(v map[string]any) { v["prompt"] = "编辑@视频1，将原片左侧容器替换。" },
		"wrong image mention":            func(v map[string]any) { v["prompt"] = "编辑@视频1，采用@图片2替换。" },
		"invented video":                 func(v map[string]any) { v["prompt"] = "编辑@视频1，采用@图片1和@视频2。" },
		"unresolved generic instruction": func(v map[string]any) { v["prompt"] = "自行决定替换目标" },
		"too long":                       func(v map[string]any) { v["prompt"] = "编辑@视频1 @图片1" + strings.Repeat("字", 6000) },
		"ready with issue":               func(v map[string]any) { v["issue"] = map[string]any{"code": "no_match", "message": "没有对应"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			require.NoError(t, common.UnmarshalJsonStr(objectSwapReady, &value))
			mutate(value)
			raw, err := common.Marshal(value)
			require.NoError(t, err)
			_, err = validateInfluencerObjectSwapPlan(string(raw), 1)
			assert.Error(t, err)
		})
	}
	_, err := validateInfluencerObjectSwapPlan(objectSwapReady, 2)
	assert.Error(t, err, "all actually supplied images must be accounted for")
	_, err = validateInfluencerObjectSwapPlan("Here is the plan: "+objectSwapReady, 1)
	assert.Error(t, err, "arbitrary prose around JSON must not become the submitted video prompt")
	_, err = validateInfluencerObjectSwapPlan(objectSwapReady, 1)
	assert.NoError(t, err)
	var repeated map[string]any
	require.NoError(t, common.UnmarshalJsonStr(objectSwapReady, &repeated))
	repeated["edits"] = append(repeated["edits"].([]any), repeated["edits"].([]any)[0])
	raw, err := common.Marshal(repeated)
	require.NoError(t, err)
	_, err = validateInfluencerObjectSwapPlan(string(raw), 1)
	assert.NoError(t, err, "one reference may legitimately support multiple targets")
}

func TestPlanInfluencerObjectSwapNeverFallsBackOrRepairsAfterError(t *testing.T) {
	previousChat, previousFetch := InfluencerObjectSwapChat, influencerMotionFetchMedia
	t.Cleanup(func() { InfluencerObjectSwapChat, influencerMotionFetchMedia = previousChat, previousFetch })
	stubObjectSwapFrames(t, nil, nil, errors.New("frame rendering unavailable"))
	influencerMotionFetchMedia = func(_ context.Context, address string, _ int64) ([]byte, error) {
		if strings.HasSuffix(address, ".mp4") {
			return []byte("video"), nil
		}
		return []byte("\x89PNG\r\n\x1a\nimage"), nil
	}
	for name, response := range map[string]struct {
		content string
		err     error
		outcome string
	}{
		"provider result unknown":         {err: errors.New("unknown"), outcome: "unknown"},
		"bad model shape":                 {content: `{"prompt":"fallback"}`, outcome: "received"},
		"blocked":                         {content: `{"status":"blocked","issue":{"code":"no_match","message":"无法按用户明确要求形成替换"},"edits":[],"unused_images":[],"preserve":"","prompt":""}`, outcome: "received"},
		"ordinary ambiguity cannot block": {content: `{"status":"blocked","issue":{"code":"low_confidence","message":"请用户点选"},"edits":[],"unused_images":[],"preserve":"","prompt":""}`, outcome: "received"},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			InfluencerObjectSwapChat = func(context.Context, string, []map[string]any) (InfluencerObjectSwapChatResult, error) {
				calls++
				return InfluencerObjectSwapChatResult{Content: response.content, Trace: InfluencerObjectSwapPlanTrace{Outcome: response.outcome, RequestID: "trace-stays", Usage: json.RawMessage(`{"total_tokens":40}`)}}, response.err
			}
			plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{VideoURL: "https://files.example/a.mp4", ImageURLs: []string{"https://files.example/a.png"}})
			require.Error(t, err)
			assert.Empty(t, plan.Prompt)
			assert.Equal(t, 1, calls)
			assert.Equal(t, "trace-stays", plan.Trace.RequestID)
			assert.Equal(t, response.outcome, plan.Trace.Outcome)
			assert.JSONEq(t, `{"total_tokens":40}`, string(plan.Trace.Usage))
			if name == "blocked" {
				assert.Contains(t, string(plan.Audit), "no_match")
			}
		})
	}
}

func stubObjectSwapLines(t *testing.T, lines []*model.Channel) *[]int {
	t.Helper()
	previous := influencerMotionSelectChannel
	t.Cleanup(func() { influencerMotionSelectChannel = previous })
	attempts := make([]int, 0)
	influencerMotionSelectChannel = func(group, modelName string, retry int, _ []appdto.ChannelFilter) (*model.Channel, error) {
		assert.Equal(t, InfluencerObjectSwapPlannerGroup, group)
		assert.Equal(t, InfluencerObjectSwapPlannerModel, modelName)
		attempts = append(attempts, retry)
		if retry >= len(lines) {
			return nil, nil
		}
		return lines[retry], nil
	}
	return &attempts
}

func objectSwapNativeResponse(t *testing.T, text, responseID string) string {
	t.Helper()
	body, err := common.Marshal(map[string]any{
		"responseId": responseID,
		"candidates": []any{map[string]any{
			"content":      map[string]any{"parts": []any{map[string]any{"text": text}}},
			"finishReason": "STOP",
		}},
		"usageMetadata": json.RawMessage(`{"promptTokenCount":50,"candidatesTokenCount":20,"totalTokenCount":70}`),
	})
	require.NoError(t, err)
	return string(body)
}

func TestObjectSwapPlannerUsesGemini4MultimodalGroup(t *testing.T) {
	assert.Equal(t, "gemini-4-argon", InfluencerObjectSwapPlannerModel)
	assert.Equal(t, "gemini-4-argon-multimodal", InfluencerObjectSwapPlannerGroup)
}

func TestPlanInfluencerObjectSwapUsesNativeGemini4Line(t *testing.T) {
	stubMotionMedia(t)
	stubObjectSwapFrames(t, nil, nil, errors.New("frame rendering unavailable"))
	var path, authorization, apiKey string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, authorization, apiKey = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-goog-api-key")
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "header-id")
		_, _ = io.WriteString(w, objectSwapNativeResponse(t, "```json\n"+objectSwapReady+"\n```", "native-response-id"))
	}))
	defer server.Close()
	stubObjectSwapLines(t, []*model.Channel{plannerLine(100, server.URL, "cheap-key")})

	plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{
		VideoURL: "https://files.example/source.mp4", DurationSec: 6,
		ImageURLs: []string{"https://files.example/a.png"}, UserPrompt: "只替换左边的容器",
	})
	require.NoError(t, err)
	assert.Contains(t, plan.Prompt, "编辑@视频1")
	assert.JSONEq(t, objectSwapReady, string(plan.Audit), "the stored audit is the stripped JSON")
	assert.Equal(t, InfluencerObjectSwapPlannerModel, plan.Trace.Model)
	assert.Equal(t, InfluencerObjectSwapPlannerGroup, plan.Trace.Group)
	assert.Equal(t, 100, plan.Trace.ChannelID)
	assert.Equal(t, "native-response-id", plan.Trace.RequestID)
	assert.Equal(t, "STOP", plan.Trace.FinishReason)
	assert.Equal(t, "received", plan.Trace.Outcome)
	assert.JSONEq(t, `{"promptTokenCount":50,"candidatesTokenCount":20,"totalTokenCount":70}`, string(plan.Trace.Usage))

	assert.Equal(t, "/v1beta/models/gemini-4-argon:generateContent", path)
	assert.Equal(t, "Bearer cheap-key", authorization)
	assert.Equal(t, "cheap-key", apiKey)
	system, _ := body["systemInstruction"].(map[string]any)
	systemParts, _ := system["parts"].([]any)
	require.Len(t, systemParts, 1)
	assert.Equal(t, influencerObjectSwapSystem, systemParts[0].(map[string]any)["text"])
	contents, _ := body["contents"].([]any)
	require.Len(t, contents, 1)
	userParts, _ := contents[0].(map[string]any)["parts"].([]any)
	var mimeTypes []string
	for _, part := range userParts {
		inline, ok := part.(map[string]any)["inlineData"].(map[string]any)
		if !ok {
			continue
		}
		mimeTypes = append(mimeTypes, inline["mimeType"].(string))
	}
	assert.Equal(t, []string{"video/mp4", "image/png"}, mimeTypes)
	generation, _ := body["generationConfig"].(map[string]any)
	assert.EqualValues(t, influencerMotionPlannerMaxOutputTokens, generation["maxOutputTokens"])
}

func TestInfluencerObjectSwapChatHandsOffUnusableNativeReply(t *testing.T) {
	var hits []string
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "cheapest")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, objectSwapNativeResponse(t, "您的额度已用完", "first-line"))
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "next")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, objectSwapNativeResponse(t, objectSwapReady, "native-response-id"))
	}))
	defer second.Close()
	stubObjectSwapLines(t, []*model.Channel{plannerLine(100, first.URL, "k1"), plannerLine(90, second.URL, "k2")})

	result, err := influencerObjectSwapChatViaChannel(context.Background(), "object swap rules", []map[string]any{{"type": "text", "text": "joint media"}})
	require.NoError(t, err)
	assert.Equal(t, objectSwapReady, result.Content)
	assert.Equal(t, []string{"cheapest", "next"}, hits, "a 200 reply without a JSON object never becomes the plan")
	assert.Equal(t, 90, result.Trace.ChannelID)
	assert.Equal(t, "native-response-id", result.Trace.RequestID)
	assert.Equal(t, "received", result.Trace.Outcome)
}

func TestInfluencerObjectSwapChatHandsOffGatewayRejection(t *testing.T) {
	for name, status := range map[string]int{"rate limited": http.StatusTooManyRequests, "unavailable": http.StatusServiceUnavailable} {
		t.Run(name, func(t *testing.T) {
			var hits []string
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits = append(hits, "first")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"code":500,"message":"upstream failed","status":"INTERNAL"}}`)
			}))
			defer first.Close()
			second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits = append(hits, "second")
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, objectSwapNativeResponse(t, objectSwapReady, "native-response-id"))
			}))
			defer second.Close()
			stubObjectSwapLines(t, []*model.Channel{plannerLine(100, first.URL, "k1"), plannerLine(90, second.URL, "k2")})

			result, err := influencerObjectSwapChatViaChannel(context.Background(), "object swap rules", []map[string]any{{"type": "text", "text": "joint media"}})
			require.NoError(t, err)
			assert.Equal(t, objectSwapReady, result.Content)
			assert.Equal(t, []string{"first", "second"}, hits, "a gateway rejection reaches the next audited line")
		})
	}
}

func TestInfluencerObjectSwapChatTransportErrorStops(t *testing.T) {
	var hits int
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.Copy(io.Discard, r.Body)
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			_ = conn.Close()
		}
	}))
	defer first.Close()
	var reached bool
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	defer second.Close()
	stubObjectSwapLines(t, []*model.Channel{plannerLine(100, first.URL, "k1"), plannerLine(90, second.URL, "k2")})

	result, err := influencerObjectSwapChatViaChannel(context.Background(), "object swap rules", []map[string]any{{"type": "text", "text": "joint media"}})
	require.Error(t, err)
	assert.Equal(t, 1, hits)
	assert.False(t, reached, "a transport failure is an unknown outcome, not an unsent rejection")
	assert.Equal(t, "unknown", result.Trace.Outcome)
	assert.Empty(t, result.Content)
}

func TestInfluencerObjectSwapChatTraceFallsBackToRequestHeaderID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "header-fallback-id")
		_, _ = io.WriteString(w, objectSwapNativeResponse(t, objectSwapReady, ""))
	}))
	defer server.Close()
	stubObjectSwapLines(t, []*model.Channel{plannerLine(100, server.URL, "k1")})

	result, err := influencerObjectSwapChatViaChannel(context.Background(), "object swap rules", []map[string]any{{"type": "text", "text": "joint media"}})
	require.NoError(t, err)
	assert.Equal(t, "header-fallback-id", result.Trace.RequestID)
}

func TestPlanInfluencerObjectSwapRejectsProseNativeReply(t *testing.T) {
	stubMotionMedia(t)
	stubObjectSwapFrames(t, nil, nil, errors.New("frame rendering unavailable"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, objectSwapNativeResponse(t, "Here is the plan: "+objectSwapReady, "prose-id"))
	}))
	defer server.Close()
	stubObjectSwapLines(t, []*model.Channel{plannerLine(100, server.URL, "k1")})

	plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{
		VideoURL: "https://files.example/source.mp4", DurationSec: 6,
		ImageURLs: []string{"https://files.example/a.png"}, UserPrompt: "只替换左边的容器",
	})
	require.Error(t, err)
	assert.Empty(t, plan.Prompt)
	assert.Empty(t, plan.Audit)
	assert.Equal(t, "received", plan.Trace.Outcome)
}

func TestObjectSwapInvalidMediaNeverStartsUnderstanding(t *testing.T) {
	previousChat, previousFetch := InfluencerObjectSwapChat, influencerMotionFetchMedia
	t.Cleanup(func() { InfluencerObjectSwapChat, influencerMotionFetchMedia = previousChat, previousFetch })
	var modelCalls int
	InfluencerObjectSwapChat = func(context.Context, string, []map[string]any) (InfluencerObjectSwapChatResult, error) {
		modelCalls++
		return InfluencerObjectSwapChatResult{}, nil
	}
	influencerMotionFetchMedia = func(context.Context, string, int64) ([]byte, error) { return nil, errors.New("media unavailable") }
	for _, input := range []InfluencerObjectSwapPlanInput{
		{VideoURL: "http://files.example/a.mp4", ImageURLs: []string{"https://files.example/a.png"}},
		{VideoURL: "https://files.example/a.mp4", ImageURLs: []string{"http://files.example/a.png"}},
		{VideoURL: "https://files.example/a.mp4"},
		{VideoURL: "https://files.example/a.mp4", ImageURLs: []string{"https://files.example/a.png"}},
	} {
		plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), input)
		require.Error(t, err)
		assert.Empty(t, plan.Prompt)
		assert.Equal(t, "not_sent", plan.Trace.Outcome)
	}
	assert.Zero(t, modelCalls)
}

// stubObjectSwapFrames keeps the understanding call tool-free and lets a test
// choose the media transport it wants to exercise.
func stubObjectSwapFrames(t *testing.T, frames [][]byte, times []float64, err error) {
	t.Helper()
	previous := influencerObjectSwapFrameExtractor
	t.Cleanup(func() { influencerObjectSwapFrameExtractor = previous })
	influencerObjectSwapFrameExtractor = func(context.Context, []byte, int) ([][]byte, []float64, error) {
		return frames, times, err
	}
}

func objectSwapUsage(modalities ...string) json.RawMessage {
	details := make([]map[string]any, 0, len(modalities))
	for _, modality := range modalities {
		details = append(details, map[string]any{"modality": modality, "tokenCount": 258})
	}
	raw, err := common.Marshal(map[string]any{"promptTokensDetails": details})
	if err != nil {
		panic(err)
	}
	return raw
}

// TestObjectSwapUnderstandingRequiresTheMediaItSent locks the guard that task
// 907 needed: an 11-second 8.5 MiB clip reached the planner as a request with no
// VIDEO tokens, the reply still passed as "ready", and its invented "dark stage
// with purple lighting" was submitted as the edit instruction.
func TestObjectSwapUnderstandingRequiresTheMediaItSent(t *testing.T) {
	tests := map[string]struct {
		frames    [][]byte
		times     []float64
		frameErr  error
		usage     json.RawMessage
		wantErr   bool
		wantMedia bool
		transport string
	}{
		"frames need image tokens": {
			frames: [][]byte{[]byte("frame")}, times: []float64{1.5},
			usage: objectSwapUsage("TEXT", "IMAGE"), wantMedia: true, transport: influencerObjectSwapTransportFrames,
		},
		"frames without image tokens fail": {
			frames: [][]byte{[]byte("frame")}, times: []float64{1.5},
			usage: objectSwapUsage("TEXT"), wantErr: true, wantMedia: true, transport: influencerObjectSwapTransportFrames,
		},
		"video transport needs video tokens": {
			frameErr: errors.New("frame rendering unavailable"),
			usage:    objectSwapUsage("TEXT", "VIDEO", "IMAGE"), wantMedia: true, transport: influencerObjectSwapTransportVideo,
		},
		"video transport without video tokens fails": {
			frameErr: errors.New("frame rendering unavailable"),
			usage:    objectSwapUsage("TEXT", "IMAGE"), wantErr: true, wantMedia: true, transport: influencerObjectSwapTransportVideo,
		},
		"unreported modalities stay unknown": {
			frameErr: errors.New("frame rendering unavailable"),
			usage:    json.RawMessage(`{"total_tokens":40}`), transport: influencerObjectSwapTransportVideo,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			stubMotionMedia(t)
			stubObjectSwapFrames(t, test.frames, test.times, test.frameErr)
			previousChat := InfluencerObjectSwapChat
			t.Cleanup(func() { InfluencerObjectSwapChat = previousChat })
			InfluencerObjectSwapChat = func(context.Context, string, []map[string]any) (InfluencerObjectSwapChatResult, error) {
				return InfluencerObjectSwapChatResult{
					Content: objectSwapReady,
					Trace:   InfluencerObjectSwapPlanTrace{Outcome: "received", RequestID: "guarded", Usage: test.usage},
				}, nil
			}
			plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{
				VideoURL: "https://files.example/source.mp4", DurationSec: 11,
				ImageURLs: []string{"https://files.example/a.png"},
			})
			assert.Equal(t, test.transport, plan.Trace.Transport)
			if test.wantMedia {
				assert.NotEmpty(t, plan.Trace.Media)
			} else {
				assert.Empty(t, plan.Trace.Media, "an unreported breakdown stays unknown")
			}
			if test.wantErr {
				require.ErrorIs(t, err, ErrInfluencerObjectSwapUnobserved)
				assert.Empty(t, plan.Prompt)
				assert.Empty(t, plan.Audit)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, plan.Prompt)
		})
	}
}

func TestObjectSwapSendsSampledFramesWithTimestamps(t *testing.T) {
	stubMotionMedia(t)
	frames := [][]byte{[]byte("first-frame"), []byte("second-frame"), []byte("third-frame")}
	stubObjectSwapFrames(t, frames, []float64{1.25, 3.75, 6.25}, nil)
	var captured []map[string]any
	previousChat := InfluencerObjectSwapChat
	t.Cleanup(func() { InfluencerObjectSwapChat = previousChat })
	InfluencerObjectSwapChat = func(_ context.Context, _ string, content []map[string]any) (InfluencerObjectSwapChatResult, error) {
		captured = content
		return InfluencerObjectSwapChatResult{
			Content: objectSwapReady,
			Trace:   InfluencerObjectSwapPlanTrace{Outcome: "received", Usage: objectSwapUsage("TEXT", "IMAGE")},
		}, nil
	}
	plan, err := PlanInfluencerObjectSwapPrompt(context.Background(), InfluencerObjectSwapPlanInput{
		VideoURL: "https://files.example/source.mp4", DurationSec: 8,
		ImageURLs: []string{"https://files.example/a.png"},
	})
	require.NoError(t, err)
	assert.Equal(t, influencerObjectSwapTransportFrames, plan.Trace.Transport)
	for _, block := range captured {
		assert.NotEqual(t, "file", block["type"], "the sampled-frame transport must not inline the whole clip")
	}
	assert.Contains(t, captured[0]["text"], "抽样的 3 帧")
	for index, frame := range frames {
		label := captured[1+index*2]
		image := captured[2+index*2]
		assert.Equal(t, "text", label["type"])
		assert.Contains(t, label["text"], fmt.Sprintf("第%d帧", index+1))
		assert.Equal(t, "image_url", image["type"])
		assert.Equal(t,
			"data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(frame),
			image["image_url"].(map[string]any)["url"])
	}
}

func TestObjectSwapSampleCountStaysBounded(t *testing.T) {
	assert.Equal(t, InfluencerObjectSwapFrameMinCount, influencerObjectSwapSampleCount(2))
	assert.Equal(t, 5, influencerObjectSwapSampleCount(5))
	assert.Equal(t, InfluencerObjectSwapFrameMaxCount, influencerObjectSwapSampleCount(30))
}

// The system rule keeps the edit instruction from asserting scene attributes it
// never observed, so a bad observation can no longer rewrite the environment.
func TestObjectSwapPromptKeepsPreserveGeneric(t *testing.T) {
	assert.Contains(t, influencerObjectSwapSystem, "抽样")
	assert.Contains(t, influencerObjectSwapSystem, "只能依据实际看到的画面")
	assert.Contains(t, influencerObjectSwapSystem, "场景属性不复述")
	assert.Contains(t, influencerObjectSwapSystem, "替换范围之外的一切保持与原片一致")
	assert.Contains(t, influencerObjectSwapSystem, "不得出现未经观察确认的场景属性词")
	assert.NotContains(t, influencerObjectSwapSystem, "保留@视频1的")
}
