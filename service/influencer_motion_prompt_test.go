package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const plannedPrompt = "【素材分工】视频1只继承动作。\n【主体】主体与图片1一致，身穿衬衫，全身不佩戴墨镜。\n【场景】客厅。\n【锁定项】无字幕。"

func TestGuardInfluencerMotionPromptRemovesNamedNegation(t *testing.T) {
	cleaned, removed, err := GuardInfluencerMotionPrompt(plannedPrompt)
	require.NoError(t, err)
	assert.True(t, removed)
	assert.NotContains(t, cleaned, "墨镜")
	assert.Contains(t, cleaned, "身穿衬衫。")
}

func TestGuardInfluencerMotionPromptRejectsEditWordsAndSectionOrder(t *testing.T) {
	for name, prompt := range map[string]string{
		"edit wording":         "【主体】主体。【场景】把座椅换成红色。",
		"scene before subject": "【场景】客厅。【主体】主体。",
		"missing subject":      "【场景】客厅。",
		"empty":                "  ",
	} {
		_, _, err := GuardInfluencerMotionPrompt(prompt)
		assert.Error(t, err, name)
	}
}

func TestGuardInfluencerMotionPromptNamesMatchedEditWord(t *testing.T) {
	_, _, err := GuardInfluencerMotionPrompt("【主体】主体。【场景】把座椅换成红色。")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"换成"`)
}

// influencerMotionRewriteSystem mandates sentences the model must copy into
// final_prompt. A mandated sentence that itself contains a banned edit word
// makes every plan fail the guard, so the two contracts must stay disjoint.
func TestRewriteSystemPromptMandatesAvoidEditWords(t *testing.T) {
	var mandates []string
	for _, line := range strings.Split(influencerMotionRewriteSystem, "\n") {
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		start := strings.Index(line, "写“")
		if start < 0 {
			continue
		}
		rest := line[start+len("写“"):]
		end := strings.Index(rest, "”")
		if end < 0 {
			continue
		}
		mandates = append(mandates, rest[:end])
	}
	require.NotEmpty(t, mandates)

	foundSound := false
	for _, sentence := range mandates {
		if strings.Contains(sentence, "保留视频1原音") {
			foundSound = true
		}
		for _, word := range influencerMotionEditWords {
			assert.NotContains(t, sentence, word, "mandated sentence %q uses banned edit word %q", sentence, word)
		}
		content := "【主体】主体与图片1一致。\n【场景】客厅。\n【声音】" + sentence
		_, _, err := GuardInfluencerMotionPrompt(content)
		assert.NoError(t, err, "the guard must accept mandated sentence %q", sentence)
	}
	assert.True(t, foundSound, "the sound mandate must stay in the rewrite system prompt")
}

func TestSystemPromptsStayGeneric(t *testing.T) {
	// Reusable rules must not carry nouns from any single test clip.
	for _, noun := range []string{"尖鞋", "尖发", "飞机", "机舱", "座椅", "舷窗", "墨镜", "灯泡", "仓库", "凉鞋", "长颈"} {
		assert.NotContains(t, influencerMotionReverseSystem, noun)
		assert.NotContains(t, influencerMotionRewriteSystem, noun)
	}
	assert.Contains(t, influencerMotionRewriteSystem, "人像图是主体形象的唯一来源")
	assert.Contains(t, influencerMotionRewriteSystem, "参考原视频本身")
	assert.Contains(t, influencerMotionRewriteSystem, "保留视频1原音")
}

func stubMotionMedia(t *testing.T) {
	t.Helper()
	previous := influencerMotionFetchMedia
	t.Cleanup(func() { influencerMotionFetchMedia = previous })
	influencerMotionFetchMedia = func(_ context.Context, url string, _ int64) ([]byte, error) {
		if strings.HasSuffix(url, ".mp4") {
			return []byte("video-bytes"), nil
		}
		return []byte("\x89PNG\r\n\x1a\nimage"), nil
	}
}

func TestPlanInfluencerMotionPromptSendsVideoUserIntentAndLabels(t *testing.T) {
	stubMotionMedia(t)
	previous := InfluencerMotionChat
	t.Cleanup(func() { InfluencerMotionChat = previous })
	var rewriteText string
	InfluencerMotionChat = func(_ context.Context, system string, content []map[string]any, _ func(string) error, _ *influencerMotionPlanSession) (string, error) {
		if system == influencerMotionReverseSystem {
			require.Equal(t, "file", content[1]["type"], "the reference clip is sent inline as a file part")
			return "```json\n{\"timeline\":[{\"t\":\"开场\"}]}\n```", nil
		}
		var text strings.Builder
		for _, part := range content {
			if s, ok := part["text"].(string); ok {
				text.WriteString(s)
			}
		}
		rewriteText = text.String()
		return `{"final_prompt":"` + strings.ReplaceAll(plannedPrompt, "\n", `\n`) + `"}`, nil
	}
	plan, err := PlanInfluencerMotionPrompt(context.Background(), InfluencerMotionPlanInput{
		VideoURL:    "https://files.example/a.mp4",
		DurationSec: 4,
		ImageURLs:   []string{"https://files.example/face.png", "https://files.example/sofa.png"},
		ImageTags:   []string{"", "场景"},
		UserPrompt:  "沙发改成复古绿丝绒",
	})
	require.NoError(t, err)
	assert.Contains(t, rewriteText, "用户要求：沙发改成复古绿丝绒")
	assert.Contains(t, rewriteText, "图片1（标签：角色）")
	assert.Contains(t, rewriteText, "图片2（标签：场景）")
	assert.True(t, plan.NegationRemoved)
	assert.NotContains(t, plan.Prompt, "墨镜")
}

func TestPlanInfluencerMotionPromptFailsWithoutUsableBreakdown(t *testing.T) {
	stubMotionMedia(t)
	previous := InfluencerMotionChat
	t.Cleanup(func() { InfluencerMotionChat = previous })
	InfluencerMotionChat = func(context.Context, string, []map[string]any, func(string) error, *influencerMotionPlanSession) (string, error) {
		return "not json", nil
	}
	_, err := PlanInfluencerMotionPrompt(context.Background(), InfluencerMotionPlanInput{VideoURL: "https://files.example/a.mp4", ImageURLs: []string{"https://files.example/b.png"}})
	assert.Error(t, err)
}

func TestPlanInfluencerMotionPromptFailsWhenPlannerFails(t *testing.T) {
	stubMotionMedia(t)
	previous := InfluencerMotionChat
	t.Cleanup(func() { InfluencerMotionChat = previous })
	InfluencerMotionChat = func(context.Context, string, []map[string]any, func(string) error, *influencerMotionPlanSession) (string, error) {
		return "", errors.New("upstream down")
	}
	_, err := PlanInfluencerMotionPrompt(context.Background(), InfluencerMotionPlanInput{VideoURL: "https://files.example/a.mp4", ImageURLs: []string{"https://files.example/b.png"}})
	assert.Error(t, err)
}

func TestPlanInfluencerMotionPromptRefusesNonHTTPSBeforeModelCall(t *testing.T) {
	previous := InfluencerMotionChat
	t.Cleanup(func() { InfluencerMotionChat = previous })
	InfluencerMotionChat = func(context.Context, string, []map[string]any, func(string) error, *influencerMotionPlanSession) (string, error) {
		t.Fatal("no model call for invalid input")
		return "", nil
	}
	_, err := PlanInfluencerMotionPrompt(context.Background(), InfluencerMotionPlanInput{VideoURL: "http://files.example/a.mp4", ImageURLs: []string{"https://files.example/b.png"}})
	assert.Error(t, err)
}
