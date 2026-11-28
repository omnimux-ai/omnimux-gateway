package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViduResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "vidu",
		model:     "viduq2",
		requestBody: map[string]any{
			"model": "viduq2",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "move between frames"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/first.png"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/last.png"},
			}}},
			"seconds": 8,
			"size":    "720p",
		},
		wantAction: "first_tail_to_video",
		wantRequest: map[string]any{
			"model":    "viduq2",
			"prompt":   "move between frames",
			"images":   []any{"https://cdn.example/first.png", "https://cdn.example/last.png"},
			"duration": float64(8),
			"size":     "720p",
		},
		wantUsageKeys:       []string{"credits", "duration", "resolution"},
		wantSubmitUsageKeys: []string{"duration", "resolution"},
		wantVendorName:      "vidu",
	})
}

func TestViduMetadataBoundsAndUsageAlignment(t *testing.T) {
	source, err := builtinplugins.Source("vidu")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "vidu"})
	require.NoError(t, err)

	for _, invalid := range []any{99999, -5, 11} {
		t.Run("rejects invalid metadata.duration on viduq2", func(t *testing.T) {
			reqBody := map[string]any{
				"model":    "viduq2",
				"prompt":   "waves",
				"duration": 4,
				"metadata": map[string]any{"duration": invalid},
			}
			_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
				"model": "viduq2", "body": map[string]any{"kind": "json", "value": map[string]any{"model": "viduq2", "prompt": "waves", "seconds": 4, "metadata": map[string]any{"duration": invalid}}},
			})
			require.ErrorContains(t, err, "viduq2 duration must be between 1 and 10")

			_, err = plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
				"model": "viduq2", "body": map[string]any{"kind": "json", "value": map[string]any{"model": "viduq2", "input": "waves", "seconds": 4, "metadata": map[string]any{"duration": invalid}}},
			})
			require.ErrorContains(t, err, "viduq2 duration must be between 1 and 10")

			_, err = plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
				"baseUrl": "https://api.vidu.cn", "apiKey": "k", "upstreamModel": "viduq2", "requestBody": reqBody,
			})
			require.ErrorContains(t, err, "viduq2 duration must be between 1 and 10")

			_, err = plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
				"model": "viduq2", "upstreamModel": "viduq2", "requestBody": reqBody,
			})
			require.ErrorContains(t, err, "viduq2 duration must be between 1 and 10")
		})
	}

	t.Run("metadata.duration overrides seconds consistently across submit and usage", func(t *testing.T) {
		decodedVal, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
			"model": "viduq2",
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "viduq2", "prompt": "waves", "seconds": 4, "metadata": map[string]any{"duration": 10},
			}},
		})
		require.NoError(t, err)
		decoded := decodePluginValue(t, decodedVal)
		reqBody := decoded["requestBody"].(map[string]any)

		submitVal, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl": "https://api.vidu.cn", "apiKey": "k", "upstreamModel": "viduq2", "requestBody": reqBody,
		})
		require.NoError(t, err)
		submitBody := decodePluginValue(t, submitVal)["body"].(map[string]any)
		assert.Equal(t, float64(10), submitBody["duration"])

		usageVal, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"model": "viduq2", "upstreamModel": "viduq2", "requestBody": reqBody,
		})
		require.NoError(t, err)
		assert.Equal(t, float64(10), decodePluginValue(t, usageVal)["duration"])
	})

	t.Run("metadata.resolution aligns submit billing and vidu2.0 combo validation", func(t *testing.T) {
		decodedVal, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
			"model": "vidu2.0",
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "vidu2.0", "prompt": "waves", "seconds": 4, "size": "360p",
				"images":   []string{"https://cdn.example/frame.png"},
				"metadata": map[string]any{"resolution": "1080p"},
			}},
		})
		require.NoError(t, err)
		reqBody := decodePluginValue(t, decodedVal)["requestBody"].(map[string]any)
		submitVal, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl": "https://api.vidu.cn", "apiKey": "k", "upstreamModel": "vidu2.0", "requestBody": reqBody,
		})
		require.NoError(t, err)
		submitBody := decodePluginValue(t, submitVal)["body"].(map[string]any)
		assert.Equal(t, "1080p", submitBody["resolution"])

		usageVal, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"model": "vidu2.0", "upstreamModel": "vidu2.0", "requestBody": reqBody,
		})
		require.NoError(t, err)
		assert.Equal(t, "1080p", decodePluginValue(t, usageVal)["resolution"])

		// vidu2.0 duration=8 only allows 720p; metadata.resolution=1080p behind size=720p must fail.
		_, err = plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
			"model": "vidu2.0",
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "vidu2.0", "prompt": "waves", "seconds": 8, "size": "720p",
				"images":   []string{"https://cdn.example/frame.png"},
				"metadata": map[string]any{"resolution": "1080p"},
			}},
		})
		require.ErrorContains(t, err, "vidu2.0 duration 8 only allows resolution 720p")
		_ = common.Marshal
	})
}
