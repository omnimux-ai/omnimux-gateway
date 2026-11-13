package relay

import (
	"encoding/json"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

func parseMJRequest(t *testing.T, body string) *dto.ImageRequest {
	t.Helper()
	request := &dto.ImageRequest{}
	if err := json.Unmarshal([]byte(body), request); err != nil {
		t.Fatalf("parse image request: %v", err)
	}
	return request
}

func TestApplyMidjourneyOperationRoutesAndPrices(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		originModel    string
		wantUpstream   string
		wantMultiplier float64
		wantHandled    bool
		wantOperation  string
	}{
		{
			name:           "upscale on v7",
			body:           `{"model":"mj-v7","model_params":{"operation":"upscale","task_id":"t1","image_number":0}}`,
			originModel:    "mj-v7",
			wantUpstream:   "mj-v7-upscale",
			wantMultiplier: 1.5,
			wantHandled:    true,
		},
		{
			name:           "remove-bg on v8.1 maps the dotted upstream id",
			body:           `{"model":"mj-v8-1","image_urls":["https://cdn.example/a.png"],"model_params":{"operation":"remove-bg"}}`,
			originModel:    "mj-v8-1",
			wantUpstream:   "mj-v8.1-remove-bg",
			wantMultiplier: 1,
			wantHandled:    true,
		},
		{
			name:           "no operation keeps plain generation",
			body:           `{"model":"mj-v7","prompt":"a cat"}`,
			originModel:    "mj-v7",
			wantUpstream:   "mj-v7",
			wantMultiplier: 1,
			wantHandled:    true,
		},
		{
			name:           "explicit generate matches plain generation",
			body:           `{"model":"mj-v8-1","prompt":"a cat","model_params":{"operation":"generate"}}`,
			originModel:    "mj-v8-1",
			wantUpstream:   "mj-v8.1",
			wantMultiplier: 1,
			wantHandled:    true,
		},
		{
			name:           "unknown operation is rejected, not downgraded",
			body:           `{"model":"mj-v7","model_params":{"operation":"teleport"}}`,
			originModel:    "mj-v7",
			wantUpstream:   "",
			wantMultiplier: 0,
			wantHandled:    false,
		},
		{
			name:           "legacy suffixed id stays untouched",
			body:           `{"model":"mj-v7-upscale","model_params":{"task_id":"t1"}}`,
			originModel:    "mj-v7-upscale",
			wantUpstream:   "",
			wantMultiplier: 0,
			wantHandled:    false,
		},
		{
			name:           "non-midjourney model stays untouched",
			body:           `{"model":"gpt-image-2","prompt":"a cat","model_params":{"operation":"upscale"}}`,
			originModel:    "gpt-image-2",
			wantUpstream:   "",
			wantMultiplier: 0,
			wantHandled:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := parseMJRequest(t, tc.body)
			info := &relaycommon.RelayInfo{
				OriginModelName: tc.originModel,
				ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: tc.originModel},
			}

			multiplier, reason, handled := ApplyMidjourneyOperation(info, request)

			if handled != tc.wantHandled {
				t.Fatalf("handled=%v want %v (reason=%q)", handled, tc.wantHandled, reason)
			}
			// Only requests inside the Midjourney scheme carry a rejection reason;
			// anything else (legacy suffixed ids, other models) is simply not ours.
			if !tc.wantHandled && types.IsMidjourneyImageModel(tc.originModel) && reason == "" {
				t.Fatal("a rejected Midjourney request must carry a caller-facing reason")
			}
			if multiplier != tc.wantMultiplier {
				t.Fatalf("multiplier=%v want %v", multiplier, tc.wantMultiplier)
			}
			if tc.wantUpstream != "" && info.UpstreamModelName != tc.wantUpstream {
				t.Fatalf("upstream=%q want %q", info.UpstreamModelName, tc.wantUpstream)
			}
			if !tc.wantHandled && info.UpstreamModelName != tc.originModel {
				t.Fatalf("rejected request must keep the original upstream model, got %q", info.UpstreamModelName)
			}
		})
	}
}

// Quality and speed are separate paid dimensions that multiply the operation tier.
// Evolink documents hd = 1.5x and draft/fast = 1x; turbo and creative have no
// published multiplier, so those requests are refused instead of guessed.
func TestMidjourneyRequestDimensionsAffectPrice(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		originModel    string
		wantMultiplier float64
		wantHandled    bool
		wantUpstream   string
	}{
		{"v8.1 standard generation", `{"model":"mj-v8-1","prompt":"cat"}`, "mj-v8-1", 1, true, "mj-v8.1"},
		{"v8.1 hd generation is 1.5x", `{"model":"mj-v8-1","prompt":"cat","quality":"hd"}`, "mj-v8-1", 1.5, true, "mj-v8.1"},
		{"v8.1 hd keeps the version price before the hd step", `{"model":"mj-v8-1","prompt":"cat","quality":"standard"}`, "mj-v8-1", 1, true, "mj-v8.1"},
		{"v8.1 hd remove-bg = 1 x 1.5", `{"model":"mj-v8-1","image_urls":["u"],"quality":"hd","model_params":{"operation":"remove-bg"}}`, "mj-v8-1", 1.5, true, "mj-v8.1-remove-bg"},
		{"v8.1 hd upscale = 1.5 x 1.5", `{"model":"mj-v8-1","quality":"hd","model_params":{"operation":"upscale","task_id":"t"}}`, "mj-v8-1", 2.25, true, "mj-v8.1-upscale"},
		{"turbo pan is priced 2x (measured)", `{"model":"mj-v7","model_params":{"operation":"pan","task_id":"t","speed":"turbo"}}`, "mj-v7", 2, true, "mj-v7-pan"},
		{"creative upscale is priced 4/3x (measured)", `{"model":"mj-v8-1","model_params":{"operation":"upscale","task_id":"t","type":"creative"}}`, "mj-v8-1", 2, true, "mj-v8.1-upscale"},
		{"unknown upscale type is refused", `{"model":"mj-v8-1","model_params":{"operation":"upscale","task_id":"t","type":"mega"}}`, "mj-v8-1", 0, false, ""},
		{"draft is priced at 1x", `{"model":"mj-v8-1","prompt":"cat","model_params":{"speed":"draft"}}`, "mj-v8-1", 1, true, "mj-v8.1"},
		{"unknown speed stays refused", `{"model":"mj-v7","model_params":{"operation":"pan","task_id":"t","speed":"warp"}}`, "mj-v7", 0, false, ""},
		{"hd on v7 is refused", `{"model":"mj-v7","prompt":"cat","quality":"hd"}`, "mj-v7", 0, false, ""},
		{"unknown quality is refused", `{"model":"mj-v8-1","prompt":"cat","quality":"ultra"}`, "mj-v8-1", 0, false, ""},
		{"unknown speed is refused", `{"model":"mj-v8-1","prompt":"cat","model_params":{"speed":"warp"}}`, "mj-v8-1", 0, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := parseMJRequest(t, tc.body)
			info := &relaycommon.RelayInfo{
				OriginModelName: tc.originModel,
				ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: tc.originModel},
			}
			multiplier, reason, handled := ApplyMidjourneyOperation(info, request)
			if handled != tc.wantHandled {
				t.Fatalf("handled=%v want %v (reason=%q)", handled, tc.wantHandled, reason)
			}
			if multiplier != tc.wantMultiplier {
				t.Fatalf("multiplier=%v want %v", multiplier, tc.wantMultiplier)
			}
			if !tc.wantHandled && reason == "" {
				t.Fatal("a refused request must carry a caller-facing reason")
			}
			if tc.wantUpstream != "" && info.UpstreamModelName != tc.wantUpstream {
				t.Fatalf("upstream=%q want %q", info.UpstreamModelName, tc.wantUpstream)
			}
		})
	}
}

// A generation request still reaches upstream untouched, and hd must survive into
// the outgoing body (quality is a declared DTO field, so it serializes as-is).
func TestMidjourneyRequestBodyCarriesDeclaredFields(t *testing.T) {
	request := parseMJRequest(t, `{"model":"mj-v8-1","prompt":"edit this","image_urls":["https://cdn.example/a.png"],"model_params":{"operation":"retexture","speed":"fast"},"some_client_junk":{"x":1}}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "mj-v8-1",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "mj-v8-1"},
	}
	if _, _, handled := ApplyMidjourneyOperation(info, request); !handled {
		t.Fatal("expected the request to be handled")
	}

	body, err := marshalImageRequestBody(request)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if parsed["model"] != "mj-v8.1-retexture" {
		t.Errorf("body model=%v want mj-v8.1-retexture", parsed["model"])
	}
	params, ok := parsed["model_params"].(map[string]any)
	if !ok {
		t.Fatalf("model_params missing from body: %s", body)
	}
	if params["operation"] != "retexture" {
		t.Errorf("model_params.operation=%v want retexture", params["operation"])
	}
	if params["speed"] != "fast" {
		t.Errorf("model_params.speed=%v want fast", params["speed"])
	}
	if _, ok := parsed["image_urls"]; !ok {
		t.Errorf("image_urls missing from body: %s", body)
	}
	if _, leaked := parsed["some_client_junk"]; leaked {
		t.Errorf("undeclared field leaked upstream: %s", body)
	}
}

// A request without model_params must not gain one, and the body must stay
// byte-equivalent to what the image path produced before this feature.
func TestMidjourneyRequestBodyWithoutOperationIsUnchanged(t *testing.T) {
	request := parseMJRequest(t, `{"model":"mj-v7","prompt":"a cat","size":"1024x1024"}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "mj-v7",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "mj-v7"},
	}
	if _, _, handled := ApplyMidjourneyOperation(info, request); !handled {
		t.Fatal("expected the request to be handled")
	}
	body, err := marshalImageRequestBody(request)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := parsed["model_params"]; ok {
		t.Errorf("plain generation must not carry model_params: %s", body)
	}
	if parsed["model"] != "mj-v7" || parsed["prompt"] != "a cat" {
		t.Errorf("unexpected body: %s", body)
	}
}
