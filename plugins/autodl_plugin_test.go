package plugins_test

import (
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const autodlBaseUrl = "https://autodl.art"

func loadAutodlPlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("autodl")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "autodl"})
	require.NoError(t, err)
	return plugin
}

func autodlSubmitContext(model string, upstreamModel string, body map[string]any) map[string]any {
	return map[string]any{
		"requestBody":   body,
		"model":         model,
		"upstreamModel": upstreamModel,
		"baseUrl":       autodlBaseUrl,
		"apiKey":        "test-token",
		"action":        "",
	}
}

func autodlJSON(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	normalized := map[string]any{}
	require.NoError(t, common.Unmarshal(encoded, &normalized))
	return normalized
}

func callAutodlSubmit(t *testing.T, plugin *jsplugin.LoadedPlugin, ctx map[string]any) map[string]any {
	t.Helper()
	value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", ctx)
	require.NoError(t, err)
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	descriptor := map[string]any{}
	require.NoError(t, common.Unmarshal(encoded, &descriptor))
	return descriptor
}

func TestAutodlPluginManifestAndRoutes(t *testing.T) {
	plugin := loadAutodlPlugin(t)

	assert.Equal(t, "autodl", plugin.Meta.Key)
	assert.Equal(t, "AutoDL ComfyUI", plugin.Meta.Name)
	assert.Equal(t, autodlBaseUrl, plugin.Meta.BaseURL)
	assert.Equal(t, "per_task", plugin.Meta.FetchMode)
	assert.Equal(t, []string{"minimax-h3-video", "wan-2-2-animate", "indextts-2"}, plugin.Meta.Models)
	assert.Equal(t, "api_key", plugin.Meta.Auth.Type)
	require.NotNil(t, plugin.Meta.Author)
	assert.Equal(t, "OmniMux", plugin.Meta.Author.Name)
	assert.NotEmpty(t, plugin.Meta.Website)

	secondUnit, declared := plugin.Meta.UsageSchema["seconds"]
	require.True(t, declared, "usageSchema must declare seconds")
	assert.Equal(t, "number", secondUnit.Type)
	assert.Equal(t, "second", secondUnit.Unit)
	assert.NotEmpty(t, secondUnit.Description)

	resolution, declared := plugin.Meta.UsageSchema["resolution"]
	require.True(t, declared, "usageSchema must declare resolution")
	// 1088p/1440p are offered and priced by resolution in the task billing
	// expression (u("resolution")); 1080p stays unoffered (upstream cost
	// 0.10-0.11 CNY/s, twice the 768p rate) and fails closed.
	assert.Equal(t, []string{"480p", "736p", "768p", "1088p", "1440p"}, resolution.Enum)
	assert.NotEmpty(t, resolution.Description)

	require.NotEmpty(t, plugin.Meta.UsageExamples)
	for _, example := range plugin.Meta.UsageExamples {
		assert.Len(t, example.Facts, len(plugin.Meta.UsageSchema), example.Label)
	}

	generation := plugin.Engine
	for _, hook := range []string{"buildSubmitRequest", "parseSubmitResponse", "buildQueryRequest", "parseTaskResult", "extractUsage", "extractUsageOnComplete", "listArtifacts", "buildContentRequest"} {
		callable, err := generation.HasCallablePath(t.Context(), hook)
		require.NoError(t, err)
		assert.True(t, callable, hook)
	}

	routes := []struct {
		method string
		path   string
		route  jsplugin.RouteType
		render string
	}{
		{"POST", "/autodl/comfyui/workflow", jsplugin.RouteTypeSubmit, "taskCreated"},
		{"GET", "/autodl/comfyui/workflow/:task_id", jsplugin.RouteTypeQuery, "taskStatus"},
	}
	registry := jsplugin.NewRegistry()
	_, err := registry.RegisterFactory(autodlSource(t), jsplugin.Options{Key: "autodl"})
	require.NoError(t, err)
	for _, expected := range routes {
		binding, found := registry.Generation().LookupDeclaredRoute(expected.method, expected.path)
		require.True(t, found, expected.method+" "+expected.path)
		assert.Equal(t, "autodl", binding.Plugin.Meta.Key)
		assert.Equal(t, expected.route, binding.Route.Type)
		assert.Equal(t, expected.render, binding.Route.Render)
	}

	for _, model := range plugin.Meta.Models {
		binding, found := registry.Generation().LookupEndpoint("POST", "/v1/responses", model)
		require.True(t, found, model)
		assert.Equal(t, "openai_responses", binding.Protocol)
	}
	var responsesClaim jsplugin.ProtocolClaim
	for _, claim := range plugin.Meta.Protocols {
		if claim.Name == "openai_responses" {
			responsesClaim = claim
		}
	}
	assert.Equal(t, []string{"stream", "sync", "background"}, responsesClaim.Supports)
	for _, model := range []string{"minimax-h3-video", "wan-2-2-animate"} {
		_, found := registry.Generation().LookupEndpoint("POST", "/v1/videos", model)
		assert.True(t, found, model)
	}
	_, speechOnVideo := registry.Generation().LookupEndpoint("POST", "/v1/videos", "indextts-2")
	assert.False(t, speechOnVideo, "the speech model is not an OpenAI Video model")
}

func autodlSource(t *testing.T) string {
	t.Helper()
	source, err := builtinplugins.Source("autodl")
	require.NoError(t, err)
	return source
}

// One case per request shape, plus the tier pin and the workflowId pin.
func TestAutodlShapeRoutingDecisions(t *testing.T) {
	plugin := loadAutodlPlugin(t)
	image := "https://cdn.example/a.png"
	audio := "https://cdn.example/a.mp3"

	cases := []struct {
		name       string
		upstream   string
		body       map[string]any
		wantID     string
		wantAction string
		wantBody   map[string]any
	}{
		{
			name: "text-to-video", upstream: "minimax-h3-video",
			body:   map[string]any{"prompt": "a cat", "resolution": "480p", "duration": 15},
			wantID: "minimax_h3_lightx2v_no_pic", wantAction: "text_to_video",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "480p竖", "duration": 15},
		},
		{
			name: "image-reference", upstream: "minimax-h3-video",
			body:   map[string]any{"prompt": "a cat", "resolution": "768p", "images": []any{image}},
			// 802 (H3九图15s 画质增强版) is the purpose-built image-only workflow
			// and ties the incumbent on cost at every offered tier, so the
			// cost-then-workflowId order selects it.
			wantID: "minimax_h3_802", wantAction: "image_to_video",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "768p竖(768*1376)", "duration": 5, "ref_image_0": image},
		},
		{
			name: "first-last-frame", upstream: "minimax-h3-video",
			body:   map[string]any{"prompt": "a cat", "resolution": "768p", "first_frame": "https://cdn.example/f.png", "last_frame": "https://cdn.example/l.png"},
			wantID: "minimax_h3_lightx2v", wantAction: "first_last_frame",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "768p竖", "duration": 5, "first_frame": "https://cdn.example/f.png", "last_frame": "https://cdn.example/l.png"},
		},
		{
			name: "image-audio", upstream: "minimax-h3-video",
			body:   map[string]any{"prompt": "a cat", "resolution": "768p", "ref_image_0": image, "ref_audio_0": audio},
			// 803 (H3九图三音15s 画质增强版) serves image-audio at the incumbent's
			// price with a 15s ceiling instead of 10s.
			wantID: "minimax_h3_803", wantAction: "image_audio",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "768p竖(768*1376)", "duration": 5, "ref_image_0": image, "ref_audio_0": audio},
		},
		{
			name: "lip-sync", upstream: "minimax-h3-video",
			body:   map[string]any{"resolution": "768p", "ref_image_0": image, "ref_audio_0": audio},
			wantID: "minimax_h3_image_audio_to_video", wantAction: "lip_sync",
			wantBody: map[string]any{"resolution": "768p竖", "audio_duration": 5, "ref_image_0": image, "ref_audio_0": audio},
		},
		{
			name: "motion", upstream: "wan-2-2-animate",
			body:   map[string]any{"ref_image": image, "ref_video": "https://cdn.example/v.mp4"},
			wantID: "wan2.2animate-v4-motion_retargeting", wantAction: "motion_transfer",
			wantBody: map[string]any{"ref_image": image, "ref_video": "https://cdn.example/v.mp4"},
		},
		{
			name: "text-to-speech", upstream: "indextts-2",
			body:   map[string]any{"prompt_text": "你好世界", "prompt_simple": "https://cdn.example/voice.wav", "emo_happy": 0.5},
			wantID: "indextts2-v1", wantAction: "text_to_speech",
			wantBody: map[string]any{"prompt_text": "你好世界", "prompt_simple": "https://cdn.example/voice.wav", "emo_happy": 0.5},
		},
		{
			name: "tier pin pro", upstream: "pro",
			body:   map[string]any{"prompt": "a cat", "resolution": "768p", "ref_image_0": image},
			wantID: "minimax_h3_zm_u24", wantAction: "image_to_video",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "768p竖", "duration": 5, "ref_image_0": image},
		},
		{
			name: "tier pin fast", upstream: "fast",
			body:   map[string]any{"prompt": "a cat", "resolution": "768p", "ref_image_0": image},
			wantID: "minimax_h3_zm_u08", wantAction: "image_to_video",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "768p竖", "duration": 5, "ref_image_0": image},
		},
		{
			name: "workflowId pin", upstream: "minimax_h3_lightx2v_v5",
			body:   map[string]any{"prompt": "a cat", "resolution": "768p", "ref_image_0": image},
			wantID: "minimax_h3_lightx2v_v5", wantAction: "image_to_video",
			wantBody: map[string]any{"prompt": "a cat", "resolution": "768p竖", "duration": 5, "ref_image_0": image},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := testCase.body["model"]
			if model == nil {
				if testCase.upstream == "indextts-2" || testCase.upstream == "wan-2-2-animate" {
					model = testCase.upstream
				} else {
					model = "minimax-h3-video"
				}
				testCase.body["model"] = model
			}
			descriptor := callAutodlSubmit(t, plugin, autodlSubmitContext(model.(string), testCase.upstream, testCase.body))
			assert.Equal(t, autodlBaseUrl+"/api/v1/comfyui/comfyui_workflow/"+testCase.wantID, descriptor["url"])
			assert.Equal(t, "POST", descriptor["method"])
			assert.Equal(t, testCase.wantAction, descriptor["action"])
			headers, ok := descriptor["headers"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "test-token", headers["Authorization"])
			assert.Equal(t, autodlJSON(t, testCase.wantBody), descriptor["body"])
		})
	}
}

func TestAutodlCapabilityBoundaries(t *testing.T) {
	plugin := loadAutodlPlugin(t)
	image := "https://cdn.example/a.png"

	t.Run("the 12s workflow is trimmed above its ceiling", func(t *testing.T) {
		descriptor := callAutodlSubmit(t, plugin, autodlSubmitContext("minimax-h3-video", "minimax-h3-video", map[string]any{
			"prompt": "a cat", "resolution": "736p", "duration": 12, "ref_image_0": image,
		}))
		assert.Contains(t, descriptor["url"], "minimax_h3_b99_003_12s")

		_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", autodlSubmitContext("minimax-h3-video", "minimax-h3-video", map[string]any{
			"prompt": "a cat", "resolution": "736p", "duration": 13, "ref_image_0": image,
		}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "supports shape image-reference with resolution 736p and duration 13s")
		assert.Contains(t, err.Error(), "supported:")
	})

	t.Run("a 13s 480p image request resolves to a 15s workflow", func(t *testing.T) {
		descriptor := callAutodlSubmit(t, plugin, autodlSubmitContext("minimax-h3-video", "minimax-h3-video", map[string]any{
			"prompt": "a cat", "resolution": "480p", "duration": 13, "ref_image_0": image,
		}))
		// 802 is the cheapest 15s-capable image-reference workflow in the
		// standard tier, so the 10s-capped workflows are never in play here;
		// their outright rejection above 13s is asserted by the 736p ceiling
		// case, which has no 15s candidate to fall back to.
		assert.Contains(t, descriptor["url"], "minimax_h3_802")
	})

	t.Run("1080p is rejected for every model", func(t *testing.T) {
		cases := []struct {
			model string
			body  map[string]any
		}{
			{"minimax-h3-video", map[string]any{"prompt": "a cat", "resolution": "1080p"}},
			{"minimax-h3-video", map[string]any{"prompt": "a cat", "resolution": "1080p竖", "ref_image_0": image}},
			{"wan-2-2-animate", map[string]any{"ref_image": image, "ref_video": "https://cdn.example/v.mp4", "resolution": "1080p"}},
		}
		for _, testCase := range cases {
			_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", autodlSubmitContext(testCase.model, testCase.model, testCase.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "is not offered; supported resolutions are 480p, 736p, 768p")
		}
	})

	t.Run("a tier pin fails closed instead of crossing tiers", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", autodlSubmitContext("minimax-h3-video", "pro", map[string]any{"prompt": "a cat"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `no workflow in tier "pro"`)
	})

	t.Run("a workflowId pin still enforces its own capability", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", autodlSubmitContext("minimax-h3-video", "minimax_h3_lightx2v_v5", map[string]any{"prompt": "a cat"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pinned workflow minimax_h3_lightx2v_v5")
	})

	t.Run("a shape the model cannot serve fails closed", func(t *testing.T) {
		_, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", autodlSubmitContext("minimax-h3-video", "minimax-h3-video", map[string]any{"prompt": "a cat", "ref_video": "https://cdn.example/v.mp4"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "supports shape motion")
	})

	t.Run("an unmapped channel falls back to the default tier", func(t *testing.T) {
		descriptor := callAutodlSubmit(t, plugin, autodlSubmitContext("minimax-h3-video", "minimax-h3-video", map[string]any{"prompt": "a cat", "resolution": "480p"}))
		assert.Contains(t, descriptor["url"], "minimax_h3_lightx2v_no_pic")
	})
}

func TestAutodlUsageFacts(t *testing.T) {
	plugin := loadAutodlPlugin(t)

	t.Run("per-second workflows report seconds and resolution", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"requestBody": map[string]any{"prompt": "a cat", "resolution": "768p", "duration": 6},
			"model":       "minimax-h3-video", "upstreamModel": "minimax-h3-video",
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		facts := map[string]any{}
		require.NoError(t, common.Unmarshal(encoded, &facts))
		assert.Equal(t, float64(6), facts["seconds"])
		assert.Equal(t, "768p", facts["resolution"])
	})

	t.Run("per-second speech synthesis reports duration with 10s floor", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"requestBody": map[string]any{"prompt_text": "你好", "prompt_simple": "https://cdn.example/v.wav"},
			"model":       "indextts-2", "upstreamModel": "indextts-2",
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		facts := map[string]any{}
		require.NoError(t, common.Unmarshal(encoded, &facts))
		assert.Equal(t, float64(10), facts["seconds"])
	})

	t.Run("motion transfer reports seconds without a resolution tier", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"requestBody": map[string]any{"ref_image": "https://cdn.example/a.png", "ref_video": "https://cdn.example/v.mp4", "duration": 9},
			"model":       "wan-2-2-animate", "upstreamModel": "wan-2-2-animate",
		})
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		facts := map[string]any{}
		require.NoError(t, common.Unmarshal(encoded, &facts))
		assert.Equal(t, map[string]any{"seconds": float64(9)}, facts)
	})

	t.Run("completion bills the requested duration, not the upstream processing time", func(t *testing.T) {
		value, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete",
			map[string]any{"state": map[string]any{"billingMode": "per_second", "seconds": 5, "resolution": "768p"}},
			map[string]any{"status": "SUCCESS"},
			map[string]any{"code": 0, "data": map[string]any{"status": "SUCCESS", "duration": 123}},
		)
		require.NoError(t, err)
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		facts := map[string]any{}
		require.NoError(t, common.Unmarshal(encoded, &facts))
		assert.Equal(t, float64(5), facts["seconds"])
		assert.Equal(t, "768p", facts["resolution"])
	})
}

func TestAutodlQueryAndArtifacts(t *testing.T) {
	plugin := loadAutodlPlugin(t)

	value, err := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId": "task-123", "baseUrl": autodlBaseUrl, "apiKey": "test-token",
	})
	require.NoError(t, err)
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	descriptor := map[string]any{}
	require.NoError(t, common.Unmarshal(encoded, &descriptor))
	assert.Equal(t, autodlBaseUrl+"/api/v1/comfyui/comfyui_workflow/result/task-123", descriptor["url"])
	assert.Equal(t, "GET", descriptor["method"])

	parsed, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{},
		map[string]any{"code": 0, "data": map[string]any{"status": "SUCCESS", "duration": 8, "results": []any{"https://cdn.example/out.mp4?sig=1"}}},
		map[string]any{"status": 200, "headers": map[string]any{}},
	)
	require.NoError(t, err)
	encoded, err = common.Marshal(parsed)
	require.NoError(t, err)
	result := map[string]any{}
	require.NoError(t, common.Unmarshal(encoded, &result))
	assert.Equal(t, "SUCCESS", result["status"])
	assert.Equal(t, "https://cdn.example/out.mp4?sig=1", result["url"])

	parsedCompleted, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{},
		map[string]any{"code": 0, "data": map[string]any{"status": "completed", "results": []any{map[string]any{
			"url":         "https://cdn.example/ComfyUI_00010_.wav",
			"type":        "audio",
			"node_id":     "9",
			"file_type":   "wav",
			"output_type": "output",
		}}}},
		map[string]any{"status": 200, "headers": map[string]any{}},
	)
	require.NoError(t, err)
	encodedCompleted, err := common.Marshal(parsedCompleted)
	require.NoError(t, err)
	resultCompleted := map[string]any{}
	require.NoError(t, common.Unmarshal(encodedCompleted, &resultCompleted))
	assert.Equal(t, "SUCCESS", resultCompleted["status"])
	assert.Equal(t, "https://cdn.example/ComfyUI_00010_.wav", resultCompleted["url"])

	artifacts, err := plugin.Engine.Call(t.Context(), "listArtifacts", map[string]any{
		"taskId": "t1", "status": "SUCCESS", "action": "text_to_video", "producerVersion": "1.0.0",
		"data": map[string]any{"code": 0, "data": map[string]any{"status": "SUCCESS", "results": []any{"https://cdn.example/out.mp4"}}},
	})
	require.NoError(t, err)
	encoded, err = common.Marshal(artifacts)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"key":"video","type":"video","mimeType":"video/mp4"}]`, string(encoded))

	content, err := plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
		"artifactKey":   "video",
		"data":          map[string]any{"code": 0, "data": map[string]any{"status": "SUCCESS", "results": []any{"https://cdn.example/out.mp4"}}},
		"clientRequest": map[string]any{"method": "GET", "headers": map[string]any{}},
		"baseUrl":       autodlBaseUrl, "apiKey": "test-token",
	})
	require.NoError(t, err)
	encoded, err = common.Marshal(content)
	require.NoError(t, err)
	assert.JSONEq(t, `{"url":"https://cdn.example/out.mp4","method":"GET","credentialless":true}`, string(encoded))
}

func TestAutodlResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey:      "autodl",
		model:          "minimax-h3-video",
		requestBody:    map[string]any{"model": "minimax-h3-video", "input": "a cat walking on the beach"},
		wantAction:     "text_to_video",
		wantRequest:    map[string]any{"prompt": "a cat walking on the beach"},
		wantUsageKeys:  []string{"seconds", "resolution"},
		wantVendorName: "autodl",
	})
}

func TestAutodlFixtureGolden(t *testing.T) {
	data, err := os.ReadFile("tasks/autodl/fixtures/autodl.json")
	require.NoError(t, err)
	report, err := jsplugin.ReplayFixture(t.Context(), autodlSource(t), data)
	require.NoError(t, err)
	assert.Equal(t, report.Total, report.Passed)
	assert.Greater(t, report.Total, 20)
}

// The sandbox has no imports, so the capability matrix exists twice: the
// generated file and the marker block inside plugin.js. They must stay
// byte-identical modulo whitespace.
func TestAutodlGeneratedCapabilityMatrixMatchesPlugin(t *testing.T) {
	generated, err := os.ReadFile("tasks/autodl/cap.gen.js")
	require.NoError(t, err)
	plugin, err := os.ReadFile("tasks/autodl/plugin.js")
	require.NoError(t, err)

	generatedLiteral := autodlCapLiteral(t, string(generated), "export const CAP = ", "")
	pluginLiteral := autodlCapLiteral(t, string(plugin), "/* BEGIN autodl-cap (generated) */", "/* END autodl-cap (generated) */")
	assert.Equal(t, generatedLiteral, pluginLiteral, "rerun node scripts/ops/pricing-ledger/gen-autodl-cap.js")
}

func autodlCapLiteral(t *testing.T, source string, begin string, end string) string {
	t.Helper()
	body := source
	if end != "" {
		from := strings.Index(source, begin)
		require.GreaterOrEqual(t, from, 0, "missing %s", begin)
		to := strings.Index(source, end)
		require.Greater(t, to, from, "missing %s", end)
		body = source[from+len(begin) : to]
	} else {
		from := strings.Index(source, begin)
		require.GreaterOrEqual(t, from, 0, "missing %s", begin)
		body = source[from:]
	}
	from := strings.Index(body, "const CAP = ")
	require.GreaterOrEqual(t, from, 0, "missing CAP literal")
	literal := body[from+len("const CAP = "):]
	literal = strings.TrimSuffix(strings.TrimSpace(literal), ";")
	return strings.Join(strings.Fields(literal), " ")
}

// Every upstream endpoint in the price ledger must be present exactly once in
// the generated matrix: a new upstream workflow cannot slip in unreviewed.
func TestAutodlCapabilityMatrixCoversLedger(t *testing.T) {
	raw, err := os.ReadFile("../scripts/ops/pricing-ledger/official-autodl-prices.json")
	require.NoError(t, err)
	var ledger struct {
		Models []struct {
			UpstreamID  string `json:"upstream_id"`
			BillingMode string `json:"billing_mode"`
			Inputs      []struct {
				Name  string    `json:"name"`
				Range []float64 `json:"range"`
			} `json:"inputs"`
		} `json:"models"`
	}
	require.NoError(t, common.Unmarshal(raw, &ledger))
	require.NotEmpty(t, ledger.Models)

	generated, err := os.ReadFile("tasks/autodl/cap.gen.js")
	require.NoError(t, err)
	literal := autodlCapLiteral(t, string(generated), "export const CAP = ", "")
	var cap struct {
		EndpointCount int `json:"endpointCount"`
		Workflows     map[string]struct {
			BillingMode string `json:"billingMode"`
			MaxDuration *int   `json:"maxDuration"`
			Tier        string `json:"tier"`
		} `json:"workflows"`
	}
	require.NoError(t, common.Unmarshal([]byte(literal), &cap))
	require.Equal(t, len(ledger.Models), cap.EndpointCount)
	require.Len(t, cap.Workflows, len(ledger.Models))

	for _, model := range ledger.Models {
		record, declared := cap.Workflows[model.UpstreamID]
		require.True(t, declared, model.UpstreamID)
		assert.Equal(t, model.BillingMode, record.BillingMode, model.UpstreamID)
		assert.Contains(t, []string{"standard", "fast", "pro"}, record.Tier, model.UpstreamID)
		want := 0
		for _, input := range model.Inputs {
			if input.Name != "duration" && input.Name != "audio_duration" {
				continue
			}
			if len(input.Range) == 2 && int(input.Range[1]) > want {
				want = int(input.Range[1])
			}
		}
		if want == 0 {
			assert.Nil(t, record.MaxDuration, model.UpstreamID)
		} else {
			require.NotNil(t, record.MaxDuration, model.UpstreamID)
			assert.Equal(t, want, *record.MaxDuration, model.UpstreamID)
		}
	}
}
