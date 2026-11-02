package relayconvert

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRequestGeminiToolSchemaUsesEffectiveModel(t *testing.T) {
	const model = "gemini-3.7-flash"
	schema := map[string]any{
		"type": "object", "properties": map[string]any{
			"enabled": map[string]any{"type": "boolean", "enum": []any{true, false}},
		},
	}
	text := "hi"
	requests := []struct {
		name    string
		request any
	}{
		{"chat", &dto.GeneralOpenAIRequest{
			Model: model, Messages: []dto.Message{{Role: "user", Content: text}},
			Tools: []dto.ToolCallRequest{{Type: "function", Function: dto.FunctionRequest{Name: "switch", Parameters: schema}}},
		}},
		{"responses", &dto.OpenAIResponsesRequest{
			Model: model, Input: mustMarshalRequestJSON(t, text),
			Tools: mustMarshalRequestJSON(t, []any{map[string]any{"type": "function", "name": "switch", "parameters": schema}}),
		}},
		{"claude", &dto.ClaudeRequest{
			Model: model, Messages: []dto.ClaudeMessage{{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "text", Text: &text}}}},
			Tools: []any{&dto.Tool{Name: "switch", InputSchema: schema}},
		}},
	}
	for _, request := range requests {
		for _, test := range []struct {
			name          string
			upstreamModel string
			wantJSON      bool
		}{
			{"standalone request model", "", true},
			{"attached supported model", model, true},
			{"attached unsupported model", "gemini-v1-model", false},
		} {
			t.Run(request.name+"/"+test.name, func(t *testing.T) {
				info := &convmeta.Values{
					UpstreamModelName: test.upstreamModel, ChannelMetaAttached: test.upstreamModel != "",
					Options: &convmeta.Options{Gemini: convmeta.GeminiOptions{
						SupportsJSONSchema: func(upstream string) bool { return upstream == model },
					}},
				}
				result, err := ConvertRequest(nil, info, types.RelayFormatGemini, request.request)
				require.NoError(t, err)
				converted := result.Value.(*dto.GeminiChatRequest)
				var groups []struct {
					Functions []map[string]any `json:"functionDeclarations"`
				}
				require.NoError(t, kitutil.Unmarshal(converted.Tools, &groups))
				require.Len(t, groups, 1)
				require.Len(t, groups[0].Functions, 1)
				function := groups[0].Functions[0]
				assert.Equal(t, "switch", function["name"])
				if test.wantJSON {
					assert.Equal(t, schema, function["parametersJsonSchema"])
					assert.NotContains(t, function, "parameters")
				} else {
					assert.NotContains(t, function, "parametersJsonSchema")
					parameters := function["parameters"].(map[string]any)
					assert.Equal(t, "OBJECT", parameters["type"])
					properties := parameters["properties"].(map[string]any)
					assert.Equal(t, map[string]any{"type": "BOOLEAN"}, properties["enabled"])
					var found bool
					for _, diagnostic := range result.Diagnostics {
						found = found || diagnostic.Code == "gemini_schema_keyword_dropped"
					}
					assert.True(t, found)
				}
			})
		}
	}
}

func TestConvertRequestGeminiResponseJSONSchemaKeepsClientKeyOrder(t *testing.T) {
	const schema = `{"type":"object","properties":{"z":{"type":"boolean","enum":[true,false]},"a":{"type":"string"}}}`
	request := &dto.OpenAIResponsesRequest{
		Model: "gemini-3.7-flash", Input: mustMarshalRequestJSON(t, "hi"),
		Text: []byte(`{"format":{"type":"json_schema","name":"answer","schema":` + schema + `}}`),
	}
	result, err := ConvertRequest(nil, &convmeta.Values{Options: &convmeta.Options{Gemini: convmeta.GeminiOptions{
		SupportsJSONSchema: func(model string) bool { return model == request.Model },
	}}}, types.RelayFormatGemini, request)
	require.NoError(t, err)
	config := result.Value.(*dto.GeminiChatRequest).GenerationConfig
	assert.Equal(t, schema, string(config.ResponseJsonSchema))
	assert.Nil(t, config.ResponseSchema)
	assert.Equal(t, "application/json", config.ResponseMimeType)
}
