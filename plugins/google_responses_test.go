package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "google",
		model:     "veo-3.1-fast-generate-preview",
		requestBody: map[string]any{
			"model": "veo-3.1-fast-generate-preview",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "animate this frame"},
				map[string]any{"type": "input_image", "image_url": "data:image/png;base64,aGVsbG8="},
			}}},
			"seconds": 8,
			"size":    "1280x720",
		},
		wantAction: "image_to_video",
		wantRequest: map[string]any{
			"model":    "veo-3.1-fast-generate-preview",
			"prompt":   "animate this frame",
			"images":   []any{"data:image/png;base64,aGVsbG8="},
			"duration": float64(8),
			"size":     "1280x720",
			"metadata": map[string]any{},
		},
		wantUsageKeys:  []string{"resolution", "seconds"},
		wantVendorName: "gemini",
	})
}

func TestGoogleDurationAndMetadataBounds(t *testing.T) {
	source, err := builtinplugins.Source("google")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "google"})
	require.NoError(t, err)

	for _, invalid := range []any{99999, -1, 16} {
		t.Run("rejects invalid metadata.durationSeconds across entrypoints", func(t *testing.T) {
			reqBody := map[string]any{"model": "veo-3.1-fast-generate-preview", "prompt": "waves", "metadata": map[string]any{"durationSeconds": invalid}}
			_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
				"model": "veo-3.1-fast-generate-preview",
				"body":  map[string]any{"kind": "json", "value": reqBody},
			})
			require.ErrorContains(t, err, "seconds must be one of 4, 6, or 8")

			_, err = plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
				"model": "veo-3.1-fast-generate-preview",
				"body":  map[string]any{"kind": "json", "value": map[string]any{"model": "veo-3.1-fast-generate-preview", "input": "waves", "metadata": map[string]any{"durationSeconds": invalid}}},
			})
			require.ErrorContains(t, err, "seconds must be one of 4, 6, or 8")

			_, err = plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
				"model": "veo-3.1-fast-generate-preview", "upstreamModel": "veo-3.1-fast-generate-preview", "requestBody": reqBody,
			})
			require.ErrorContains(t, err, "seconds must be one of 4, 6, or 8")

			_, err = plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
				"baseUrl": "https://generativelanguage.googleapis.com", "apiKey": "k", "upstreamModel": "veo-3.1-fast-generate-preview", "requestBody": reqBody,
			})
			require.ErrorContains(t, err, "seconds must be one of 4, 6, or 8")
		})
	}

	t.Run("metadata.durationSeconds=4 aligns extractUsage and buildSubmitRequest", func(t *testing.T) {
		reqBody := map[string]any{"model": "veo-3.1-fast-generate-preview", "prompt": "waves", "metadata": map[string]any{"durationSeconds": 4}}
		usageVal, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"model": "veo-3.1-fast-generate-preview", "upstreamModel": "veo-3.1-fast-generate-preview", "requestBody": reqBody,
		})
		require.NoError(t, err)
		assert.Equal(t, float64(4), decodePluginValue(t, usageVal)["seconds"])

		submitVal, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl": "https://generativelanguage.googleapis.com", "apiKey": "k", "upstreamModel": "veo-3.1-fast-generate-preview", "requestBody": reqBody,
		})
		require.NoError(t, err)
		params := decodePluginValue(t, submitVal)["body"].(map[string]any)["parameters"].(map[string]any)
		assert.Equal(t, float64(4), params["durationSeconds"])
	})

	t.Run("openai_responses rejects invalid seconds and resolution", func(t *testing.T) {
		_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
			"model": "veo-3.1-fast-generate-preview",
			"body":  map[string]any{"kind": "json", "value": map[string]any{"model": "veo-3.1-fast-generate-preview", "input": "waves", "seconds": 12}},
		})
		require.ErrorContains(t, err, "seconds must be one of 4, 6, or 8")

		_, err = plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
			"model": "veo-3.1-fast-generate-preview",
			"body":  map[string]any{"kind": "json", "value": map[string]any{"model": "veo-3.1-fast-generate-preview", "input": "waves", "resolution": "8k"}},
		})
		require.ErrorContains(t, err, "resolution must be one of 720p, 1080p, or 4k")
	})
}
