package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
)

func TestResolveCompletionRatioForJev(t *testing.T) {
	tests := []struct {
		name       string
		configured *float64
		wantRatio  float64
		wantLocked bool
	}{
		{name: "absent configuration uses free unlocked fallback", wantRatio: 0},
		{name: "explicit configuration overrides fallback", configured: floatPointer(2.5), wantRatio: 2.5},
		{name: "explicit zero remains free", configured: floatPointer(0), wantRatio: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResolveCompletionRatio("jev", test.configured)
			assert.Equal(t, test.wantRatio, got.Ratio)
			assert.Equal(t, test.wantLocked, got.Locked)
		})
	}
}

func floatPointer(value float64) *float64 {
	return &value
}

func TestFormatMatchingModelNameDoesNotStripBase(t *testing.T) {
	assert.Equal(t, "qwen3-max@thinking:on", FormatMatchingModelName("qwen3-max@thinking:on"))
	assert.Equal(t, "claude-3-7-sonnet-thinking", FormatMatchingModelName("claude-3-7-sonnet-thinking"))
	assert.Equal(t, "gemini-2.5-flash-thinking-*", FormatMatchingModelName("gemini-2.5-flash-thinking-8192"))
	assert.Equal(t, "gpt-4-gizmo-*", FormatMatchingModelName("gpt-4-gizmo-abc"))
}

func TestRoutingMatchModelNameStripsThenWildcards(t *testing.T) {
	assert.Equal(t, "qwen3-max", RoutingMatchModelName("qwen3-max@thinking:on@temperature:0.2"))
	assert.Equal(t, "claude-3-7-sonnet", RoutingMatchModelName("claude-3-7-sonnet-thinking"))
	assert.Equal(t, "gemini-2.5-flash-thinking-*", RoutingMatchModelName("gemini-2.5-flash-thinking-8192"))
	assert.Equal(t, "gpt-5.1-codex-max", RoutingMatchModelName("gpt-5.1-codex-max"))

	geminiSettings := model_setting.GetGeminiSettings()
	old := geminiSettings.ThinkingAdapterEnabled
	geminiSettings.ThinkingAdapterEnabled = true
	t.Cleanup(func() { geminiSettings.ThinkingAdapterEnabled = old })
	assert.Equal(t, "gemini-2.5-flash", RoutingMatchModelName("gemini-2.5-flash-thinking-8192"))
}

func TestRoutingMatchModelNamePreservesExemptAtName(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	original := append([]string(nil), settings.ThinkingModelBlacklist...)
	t.Cleanup(func() { settings.ThinkingModelBlacklist = original })
	settings.ThinkingModelBlacklist = append(original, "re:.*@sha256:.*")

	assert.Equal(t, "opaque@sha256:deadbeef", RoutingMatchModelName("opaque@sha256:deadbeef"))
	assert.Equal(t, "kimi-k2-thinking", RoutingMatchModelName("kimi-k2-thinking"))
}
