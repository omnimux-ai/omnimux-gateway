package reasoning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderGeminiSeparatesHighAndMaximumBudget(t *testing.T) {
	for _, test := range []struct {
		model  string
		effort Effort
		budget int
	}{
		{"gemini-2.5-pro", EffortHigh, 24576},
		{"gemini-2.5-pro", EffortXHigh, 32768},
		{"gemini-2.5-pro", EffortMax, 32768},
		{"gemini-2.5-flash", EffortMax, 24576},
	} {
		t.Run(test.model+"/"+string(test.effort), func(t *testing.T) {
			rendered, err := RenderGemini(test.model, Intent{Mode: ModeEnabled, Effort: test.effort}, nil, 0)
			require.NoError(t, err)
			require.NotNil(t, rendered.Config)
			require.NotNil(t, rendered.Config.ThinkingBudget)
			assert.Equal(t, test.budget, *rendered.Config.ThinkingBudget)
		})
	}
}

func TestRenderClaudeDisabledThinkingKeepsAcceptedEffort(t *testing.T) {
	for _, test := range []struct {
		model    string
		thinking string
		effort   Effort
	}{
		{"claude-sonnet-5-5", "between_tools", EffortHigh},
		{"claude-haiku-5-5", "disabled", EffortHigh},
		{"claude-opus-5-5", "adaptive", EffortLow},
	} {
		t.Run(test.model, func(t *testing.T) {
			rendered, err := RenderClaude(test.model, Intent{Mode: ModeDisabled, Effort: EffortMax}, nil, 0)
			require.NoError(t, err)
			require.NotNil(t, rendered.Thinking)
			assert.Equal(t, test.thinking, rendered.Thinking.Type)
			assert.Equal(t, test.effort, rendered.OutputEffort)
			require.NotEmpty(t, rendered.Diagnostics)
		})
	}
}
