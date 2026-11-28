package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTypeSafePlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("typesafe")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "typesafe"})
	require.NoError(t, err)
	return plugin
}

func roundTripTypeSafeValue(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, common.Unmarshal(encoded, &result))
	return result
}

func TestEmbeddedTypeSafePluginContract(t *testing.T) {
	plugin := loadTypeSafePlugin(t)

	assert.Equal(t, "1.2.0", plugin.Meta.Version)
	assert.Equal(t, []string{"jev-1.14.0", "jev-1.13.0", "jev-latest", "jev-preview"}, plugin.Meta.Models)
	assert.Equal(t, "https://api.typesafe.ai", plugin.Meta.BaseURL)
	assert.True(t, plugin.Meta.SupportsUpstream(jsplugin.UpstreamKindVendor))
	assert.True(t, plugin.Meta.SupportsUpstream(jsplugin.UpstreamKindNewAPI))
	assert.True(t, plugin.Meta.PreservesJSONOrder())

	mediaType, iconBytes, ok := builtinplugins.Icon("typesafe")
	require.True(t, ok)
	assert.Equal(t, "image/png", mediaType)
	require.NoError(t, jsplugin.ValidateIconImage(mediaType, iconBytes))

	baseRequest := map[string]any{
		"model": "jev-1.14.0",
		"state": map[string]any{"zeta": "1", "alpha": "2"},
		"questions": map[string]any{
			"q1": map[string]any{"type": "noul", "instructions": "Is this valid?"},
		},
	}

	t.Run("buildSubmitRequest targets vendor and new_api upstreams", func(t *testing.T) {
		vendorDesc, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl":        "https://api.typesafe.ai/v1/",
			"apiKey":         "ts-key",
			"model":          "jev",
			"upstreamModel":  "jev-latest",
			"requestBody":    baseRequest,
			"requestHeaders": map[string]any{},
			"files":          []any{},
			"upstream":       map[string]any{"kind": "vendor"},
		})
		require.NoError(t, err)
		desc := roundTripTypeSafeValue(t, vendorDesc)
		assert.Equal(t, "https://api.typesafe.ai/v1/systemone", desc["url"])
		assert.Equal(t, "POST", desc["method"])
		headers := desc["headers"].(map[string]any)
		assert.Equal(t, "Bearer ts-key", headers["Authorization"])
		assert.Equal(t, "application/json", headers["Content-Type"])
		assert.NotContains(t, headers, "TypeSafe-Danger-Zone-Images")
		assert.NotContains(t, headers, "TypeSafe-Danger-Zone-Long-Context")
		assert.Equal(t, "jev-latest", desc["body"].(map[string]any)["model"])

		gatewayDesc, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl":       "https://gateway.example",
			"apiKey":        "sk-gw",
			"model":         "jev-1.14.0",
			"upstreamModel": "jev-1.14.0",
			"requestBody": map[string]any{
				"model":     "jev-1.14.0",
				"state":     "photo",
				"questions": map[string]any{"find": map[string]any{"type": "box"}},
				"assets":    map[string]any{"photo": map[string]any{"type": "image", "format": "png", "part": "img1"}},
			},
			"requestHeaders": map[string]any{"Typesafe-Danger-Zone-Long-Context": "true"},
			"files":          []any{map[string]any{"ref": "request_file:img1", "field": "img1", "filename": "a.png", "mimeType": "image/png", "size": 8}},
			"upstream":       map[string]any{"kind": "new_api"},
		})
		require.NoError(t, err)
		gw := roundTripTypeSafeValue(t, gatewayDesc)
		assert.Equal(t, "https://gateway.example/typesafe/v1/systemone", gw["url"])
		assert.Equal(t, "multipart", gw["bodyType"])
		gwHeaders := gw["headers"].(map[string]any)
		assert.Equal(t, "true", gwHeaders["TypeSafe-Danger-Zone-Images"])
		assert.Equal(t, "true", gwHeaders["TypeSafe-Danger-Zone-Long-Context"])
		parts := gw["parts"].([]any)
		require.Len(t, parts, 2)
		assert.Equal(t, "request", parts[0].(map[string]any)["name"])
		assert.Equal(t, "img1", parts[1].(map[string]any)["name"])
		assert.Equal(t, "request_file:img1", parts[1].(map[string]any)["fileRef"])
	})

	t.Run("extractUsage and extractUsageOnComplete enforce token budgets", func(t *testing.T) {
		ratios, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"usagePurpose":   "billing_ratios",
			"model":          "jev-1.14.0",
			"upstreamModel":  "jev-1.14.0",
			"requestBody":    baseRequest,
			"requestHeaders": map[string]any{},
		})
		require.NoError(t, err)
		assert.Nil(t, ratios)

		defaultFacts, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"usagePurpose":   "facts",
			"model":          "jev-1.14.0",
			"upstreamModel":  "jev-1.14.0",
			"requestBody":    baseRequest,
			"requestHeaders": map[string]any{},
		})
		require.NoError(t, err)
		assert.EqualValues(t, 65792, roundTripTypeSafeValue(t, defaultFacts)["input_tokens"])

		longFacts, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"usagePurpose":   "facts",
			"model":          "jev-1.14.0",
			"upstreamModel":  "jev-1.14.0",
			"requestBody":    baseRequest,
			"requestHeaders": map[string]any{"Typesafe-Danger-Zone-Long-Context": "true"},
		})
		require.NoError(t, err)
		assert.EqualValues(t, 262400, roundTripTypeSafeValue(t, longFacts)["input_tokens"])

		settled, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete",
			map[string]any{},
			map[string]any{"status": "SUCCESS"},
			map[string]any{"usage": map[string]any{"input_tokens": 1234, "output_tokens": 99}},
		)
		require.NoError(t, err)
		assert.EqualValues(t, 1234, roundTripTypeSafeValue(t, settled)["input_tokens"])

		for _, badBody := range []any{
			map[string]any{"usage": map[string]any{}},
			map[string]any{"usage": map[string]any{"input_tokens": -1}},
			map[string]any{"usage": map[string]any{"input_tokens": 262401}},
		} {
			_, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete", map[string]any{}, map[string]any{"status": "SUCCESS"}, badBody)
			require.Error(t, err)
		}
	})

	t.Run("native error unwraps TypeSafe detail JSON and wraps plain errors", func(t *testing.T) {
		upstreamErr, err := plugin.Engine.CallPath(t.Context(), "native", []string{"error"},
			map[string]any{},
			map[string]any{"message": `{"detail":{"error_type":"max_tokens_exceeded"}}`},
		)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"detail": map[string]any{"error_type": "max_tokens_exceeded"}}, roundTripTypeSafeValue(t, upstreamErr))

		plainErr, err := plugin.Engine.CallPath(t.Context(), "native", []string{"error"},
			map[string]any{},
			map[string]any{"message": "model is required"},
		)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"detail": "model is required"}, roundTripTypeSafeValue(t, plainErr))
	})
}
