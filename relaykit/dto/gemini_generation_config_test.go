package dto

import (
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiChatGenerationConfigPreservesExplicitZeroValuesCamelCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"topP":0,
			"topK":0,
			"maxOutputTokens":0,
			"candidateCount":0,
			"seed":0,
			"responseLogprobs":false
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	encoded, err := kitutil.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, kitutil.Unmarshal(encoded, &out))

	generationConfig, ok := out["generationConfig"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, generationConfig, "topP")
	assert.Contains(t, generationConfig, "topK")
	assert.Contains(t, generationConfig, "maxOutputTokens")
	assert.Contains(t, generationConfig, "candidateCount")
	assert.Contains(t, generationConfig, "seed")
	assert.Contains(t, generationConfig, "responseLogprobs")

	assert.Equal(t, float64(0), generationConfig["topP"])
	assert.Equal(t, float64(0), generationConfig["topK"])
	assert.Equal(t, float64(0), generationConfig["maxOutputTokens"])
	assert.Equal(t, float64(0), generationConfig["candidateCount"])
	assert.Equal(t, float64(0), generationConfig["seed"])
	assert.Equal(t, false, generationConfig["responseLogprobs"])
}

func TestGeminiChatGenerationConfigPreservesExplicitZeroValuesSnakeCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"top_p":0,
			"top_k":0,
			"max_output_tokens":0,
			"candidate_count":0,
			"seed":0,
			"response_logprobs":false
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	encoded, err := kitutil.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, kitutil.Unmarshal(encoded, &out))

	generationConfig, ok := out["generationConfig"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, generationConfig, "topP")
	assert.Contains(t, generationConfig, "topK")
	assert.Contains(t, generationConfig, "maxOutputTokens")
	assert.Contains(t, generationConfig, "candidateCount")
	assert.Contains(t, generationConfig, "seed")
	assert.Contains(t, generationConfig, "responseLogprobs")

	assert.Equal(t, float64(0), generationConfig["topP"])
	assert.Equal(t, float64(0), generationConfig["topK"])
	assert.Equal(t, float64(0), generationConfig["maxOutputTokens"])
	assert.Equal(t, float64(0), generationConfig["candidateCount"])
	assert.Equal(t, float64(0), generationConfig["seed"])
	assert.Equal(t, false, generationConfig["responseLogprobs"])
}

func TestGemini4ContinuationAndMediaProcessingAliases(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{
			name: "camelCase",
			raw: `{
				"continuationToken":{"cursor":"req-1"},
				"contents":[{"role":"user","parts":[{"text":"hi","mediaProcessing":{"mode":"fast"}}]}],
				"generationConfig":{"mediaProcessing":{"quality":"high"}}
			}`,
		},
		{
			name: "snake_case",
			raw: `{
				"continuation_token":{"cursor":"req-1"},
				"contents":[{"role":"user","parts":[{"text":"hi","media_processing":{"mode":"fast"}}]}],
				"generationConfig":{"media_processing":{"quality":"high"}}
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req GeminiChatRequest
			require.NoError(t, kitutil.Unmarshal([]byte(tc.raw), &req))
			assert.JSONEq(t, `{"cursor":"req-1"}`, string(req.ContinuationToken))
			require.Len(t, req.Contents, 1)
			require.Len(t, req.Contents[0].Parts, 1)
			assert.JSONEq(t, `{"mode":"fast"}`, string(req.Contents[0].Parts[0].MediaProcessing))
			assert.JSONEq(t, `{"quality":"high"}`, string(req.GenerationConfig.MediaProcessing))

			encoded, err := kitutil.Marshal(req)
			require.NoError(t, err)
			var out map[string]any
			require.NoError(t, kitutil.Unmarshal(encoded, &out))
			assert.Contains(t, out, "continuationToken")
		})
	}

	var resp GeminiChatResponse
	require.NoError(t, kitutil.Unmarshal([]byte(`{
		"candidates":[{"content":{"parts":[{"text":"partial"}]},"finishReason":"CONTINUATION","continuationToken":{"cursor":"cand-1"},"index":0}]
	}`), &resp))
	require.Len(t, resp.Candidates, 1)
	assert.JSONEq(t, `{"cursor":"cand-1"}`, string(resp.Candidates[0].ContinuationToken))
}
