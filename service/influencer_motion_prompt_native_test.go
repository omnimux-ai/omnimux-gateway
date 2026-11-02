package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	appdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const plannerAuthRejection = `{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`

const plannerNativeSuccess = `{"candidates":[{"content":{"role":"model","parts":[{"text":"thinking words","thought":true},{"text":"planned answer"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3,"totalTokenCount":15}}`

// plannerLine builds one supplier-route fixture: a native Gemini line with the
// given priority (higher is served first) and key.
func plannerLine(priority int64, baseURL, key string) *model.Channel {
	weight := uint(0)
	return &model.Channel{
		Id:       int(priority),
		Type:     59,
		Name:     "planner fixture",
		Key:      key,
		BaseURL:  &baseURL,
		Priority: &priority,
		Weight:   &weight,
		Status:   common.ChannelStatusEnabled,
	}
}

// stubPlannerLines replaces the channel selector and records the retry indexes
// the planner asked for, so line order is observable without a database.
func stubPlannerLines(t *testing.T, lines []*model.Channel) *[]int {
	t.Helper()
	previous := influencerMotionSelectChannel
	t.Cleanup(func() { influencerMotionSelectChannel = previous })
	attempts := make([]int, 0)
	influencerMotionSelectChannel = func(group, modelName string, retry int, filters []appdto.ChannelFilter) (*model.Channel, error) {
		assert.Equal(t, InfluencerMotionPlannerGroup, group)
		assert.Equal(t, InfluencerMotionPlannerModel, modelName)
		attempts = append(attempts, retry)
		if retry >= len(lines) {
			return nil, nil
		}
		return lines[retry], nil
	}
	return &attempts
}

func plannerContent() []map[string]any {
	return []map[string]any{
		{"type": "text", "text": "reverse this clip"},
		{"type": "file", "file": map[string]any{"filename": "reference.mp4", "file_data": "data:video/mp4;base64,VklERU8="}},
		{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,SU1BR0U="}},
	}
}

func TestInfluencerMotionNativePartsTranslateInlineMedia(t *testing.T) {
	parts, err := influencerMotionNativeParts(plannerContent())
	require.NoError(t, err)
	require.Len(t, parts, 3)
	assert.Equal(t, "reverse this clip", parts[0]["text"])
	assert.Equal(t, map[string]any{"inlineData": map[string]any{"mimeType": "video/mp4", "data": "VklERU8="}}, parts[1])
	assert.Equal(t, map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "SU1BR0U="}}, parts[2])

	for name, content := range map[string][]map[string]any{
		"remote url":  {{"type": "image_url", "image_url": map[string]any{"url": "https://files.example/a.png"}}},
		"pdf inline":  {{"type": "file", "file": map[string]any{"file_data": "data:application/pdf;base64,QQ=="}}},
		"empty":       {},
		"unknown op":  {{"type": "audio"}},
		"plain bytes": {{"type": "file", "file": map[string]any{"file_data": "VklERU8="}}},
	} {
		_, err := influencerMotionNativeParts(content)
		assert.Error(t, err, name)
	}
}

func TestInfluencerMotionChatUsesNativeArgonLine(t *testing.T) {
	var path, authorization, apiKey string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, authorization, apiKey = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-goog-api-key")
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer server.Close()
	stubPlannerLines(t, []*model.Channel{plannerLine(100, server.URL, "cheap-key")})

	text, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "planned answer", text, "thinking parts never reach the planner result")
	assert.Equal(t, "/v1beta/models/gemini-4-argon:generateContent", path)
	assert.Equal(t, "Bearer cheap-key", authorization)
	assert.Equal(t, "cheap-key", apiKey)

	system, _ := body["systemInstruction"].(map[string]any)
	systemParts, _ := system["parts"].([]any)
	require.Len(t, systemParts, 1)
	assert.Equal(t, "system rules", systemParts[0].(map[string]any)["text"])

	contents, _ := body["contents"].([]any)
	require.Len(t, contents, 1)
	userParts, _ := contents[0].(map[string]any)["parts"].([]any)
	require.Len(t, userParts, 3)
	inline, _ := userParts[1].(map[string]any)["inlineData"].(map[string]any)
	assert.Equal(t, "video/mp4", inline["mimeType"])

	generation, _ := body["generationConfig"].(map[string]any)
	assert.EqualValues(t, influencerMotionPlannerMaxOutputTokens, generation["maxOutputTokens"])
	thinking, _ := generation["thinkingConfig"].(map[string]any)
	assert.Equal(t, "low", thinking["thinkingLevel"])
}

func TestInfluencerMotionChatHandsOffOnDefiniteAuthRejection(t *testing.T) {
	var hits []string
	cheap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "cheapest")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, plannerAuthRejection)
	}))
	defer cheap.Close()
	next := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "next")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer next.Close()
	stubPlannerLines(t, []*model.Channel{plannerLine(100, cheap.URL, "k1"), plannerLine(90, next.URL, "k2")})

	text, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "planned answer", text)
	assert.Equal(t, []string{"cheapest", "next"}, hits)
}

func TestInfluencerMotionChatHandsOffOnGatewayRejection(t *testing.T) {
	for name, status := range map[string]int{"rate limited": http.StatusTooManyRequests, "bad gateway": http.StatusBadGateway, "unavailable": http.StatusServiceUnavailable} {
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
				_, _ = io.WriteString(w, plannerNativeSuccess)
			}))
			defer second.Close()
			stubPlannerLines(t, []*model.Channel{plannerLine(100, first.URL, "k1"), plannerLine(90, second.URL, "k2")})

			text, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
			require.NoError(t, err)
			assert.Equal(t, "planned answer", text)
			assert.Equal(t, []string{"first", "second"}, hits, "a gateway rejection reaches the next audited line")
		})
	}
}

func TestInfluencerMotionChatStopsOnAmbiguousOrClientFailure(t *testing.T) {
	for name, status := range map[string]int{"malformed auth rejection": http.StatusUnauthorized, "bad request": http.StatusBadRequest} {
		t.Run(name, func(t *testing.T) {
			var hits []string
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits = append(hits, "first")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == http.StatusUnauthorized {
					_, _ = io.WriteString(w, `{"error":{"code":401,"message":"quota exhausted","status":"UNAUTHENTICATED"}}`)
					return
				}
				_, _ = io.WriteString(w, `{"error":{"code":400,"message":"bad field","status":"INVALID_ARGUMENT"}}`)
			}))
			defer first.Close()
			var reached bool
			second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				_, _ = io.WriteString(w, plannerNativeSuccess)
			}))
			defer second.Close()
			stubPlannerLines(t, []*model.Channel{plannerLine(100, first.URL, "k1"), plannerLine(90, second.URL, "k2")})

			_, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
			assert.Error(t, err)
			assert.False(t, reached, "only gateway rejection and strict auth rejection may hand off")
			assert.Equal(t, []string{"first"}, hits)
		})
	}
}

func TestInfluencerMotionChatSkipsLineWithoutUsableKey(t *testing.T) {
	var cheapHits int
	cheap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cheapHits++
	}))
	defer cheap.Close()
	next := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer next.Close()

	emptyLine := plannerLine(100, cheap.URL, "")
	emptyLine.ChannelInfo.IsMultiKey = true
	stubPlannerLines(t, []*model.Channel{emptyLine, plannerLine(90, next.URL, "k2")})

	text, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "planned answer", text)
	assert.Zero(t, cheapHits, "a locally unusable key never sends a request")
}

func TestInfluencerMotionChatStopsAtLastDistinctLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, plannerAuthRejection)
	}))
	defer server.Close()

	attempts := stubPlannerLines(t, []*model.Channel{plannerLine(100, server.URL, "k1")})
	influencerMotionSelectChannel = func(group, modelName string, retry int, filters []appdto.ChannelFilter) (*model.Channel, error) {
		*attempts = append(*attempts, retry)
		// A real selector clamps past the last tier; the planner must not replay it.
		return plannerLine(100, server.URL, "k1"), nil
	}

	_, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
	assert.Error(t, err)
	assert.Equal(t, []int{0, 1}, *attempts, "the clamped tier is never replayed")
}

func TestInfluencerMotionChatKeepsTimeoutUnsafeToHandOff(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer slow.Close()
	var reached bool
	next := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer next.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	stubPlannerLines(t, []*model.Channel{plannerLine(100, slow.URL, "k1"), plannerLine(90, next.URL, "k2")})

	_, err := influencerMotionChatViaChannel(ctx, "system rules", plannerContent(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "planner request failed", "the logged cause keeps the transport error")
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.False(t, reached, "a timed-out line is an unknown outcome, not an unsent rejection")
}

func TestInfluencerMotionChatRetriesLineWhoseAnswerFailsTheCheck(t *testing.T) {
	var hits []string
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "cheapest")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"\u60a8\u7684\u989d\u5ea6\u5df2\u7528\u5b8c"}]},"finishReason":"STOP"}]}`)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, "next")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer second.Close()
	stubPlannerLines(t, []*model.Channel{plannerLine(100, first.URL, "k1"), plannerLine(90, second.URL, "k2")})

	check := func(reply string) error {
		if reply != "planned answer" {
			return assert.AnError
		}
		return nil
	}
	text, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), check, nil)
	require.NoError(t, err)
	assert.Equal(t, "planned answer", text)
	assert.Equal(t, []string{"cheapest", "next"}, hits, "a 200 reply with the wrong shape does not become the plan")
}

func TestInfluencerMotionPlanSessionSkipsLineRejectedByEarlierTurn(t *testing.T) {
	badHits, goodHits := 0, 0
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badHits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, plannerAuthRejection)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, plannerNativeSuccess)
	}))
	defer good.Close()
	stubPlannerLines(t, []*model.Channel{plannerLine(100, bad.URL, "k1"), plannerLine(90, good.URL, "k2")})
	session := newInfluencerMotionPlanSession()

	_, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, session)
	require.NoError(t, err)
	_, err = influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, session)
	require.NoError(t, err)
	assert.Equal(t, 1, badHits, "a tier rejected earlier in the plan is not paid for again")
	assert.Equal(t, 2, goodHits)
}

func TestInfluencerMotionChatPropagatesSelectorFailure(t *testing.T) {
	previous := influencerMotionSelectChannel
	t.Cleanup(func() { influencerMotionSelectChannel = previous })
	influencerMotionSelectChannel = func(group, modelName string, retry int, filters []appdto.ChannelFilter) (*model.Channel, error) {
		return nil, assert.AnError
	}
	_, err := influencerMotionChatViaChannel(context.Background(), "system rules", plannerContent(), nil, nil)
	assert.ErrorIs(t, err, assert.AnError)
}
