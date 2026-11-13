package relayconvert

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRequestResponsesNamespacesPreserveCustomInputAndRetry(t *testing.T) {
	const input = "echo \"你好\"\n<file>"
	for _, target := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatGemini} {
		for _, namespace := range []string{"functions", "workspace"} {
			t.Run(string(target)+"/"+namespace, func(t *testing.T) {
				name := convmeta.NamespacedToolName(namespace, "exec")
				request := &dto.OpenAIResponsesRequest{
					Model:           "model-test",
					MaxOutputTokens: lo.ToPtr(uint(64)),
					Tools: mustMarshalRequestJSON(t, []any{map[string]any{
						"type": "namespace", "name": namespace,
						"tools": []any{map[string]any{"type": "custom", "name": "exec", "format": map[string]any{"type": "text"}}},
					}}),
					ToolChoice: mustMarshalRequestJSON(t, map[string]any{"type": "custom", "name": "exec", "namespace": namespace}),
					Input: mustMarshalRequestJSON(t, []any{
						map[string]any{"role": "user", "content": "Run the tool"},
						map[string]any{"type": "custom_tool_call", "call_id": "call_exec", "name": "exec", "namespace": namespace, "input": input},
						map[string]any{"type": "custom_tool_call_output", "call_id": "call_exec", "output": "done"},
					}),
				}
				info := &convmeta.Values{}
				result, err := ConvertRequest(nil, info, target, request)
				require.NoError(t, err)
				require.NotNil(t, info.ResponsesTools)
				assert.True(t, info.ResponsesTools.IsCustomTool(name))
				wantNamespace := namespace
				if namespace == "functions" {
					wantNamespace = ""
				}
				gotNamespace, gotName := info.ResponsesTools.ResponsesToolName(name)
				assert.Equal(t, wantNamespace, gotNamespace)
				assert.Equal(t, "exec", gotName)

				switch converted := result.Value.(type) {
				case *dto.GeneralOpenAIRequest:
					require.Len(t, converted.Tools, 1)
					assert.Equal(t, name, converted.Tools[0].Function.Name)
					assert.Equal(t, map[string]any{"type": "function", "function": map[string]any{"name": name}}, converted.ToolChoice)
					require.Len(t, converted.Messages, 3)
					calls := converted.Messages[1].ParseToolCalls()
					require.Len(t, calls, 1)
					assert.Equal(t, name, calls[0].Function.Name)
					var arguments map[string]any
					require.NoError(t, kitutil.Unmarshal([]byte(calls[0].Function.Arguments), &arguments))
					assert.Equal(t, map[string]any{"input": input}, arguments)
				case *dto.ClaudeRequest:
					tools, err := kitutil.Any2Type[[]dto.Tool](converted.Tools)
					require.NoError(t, err)
					require.Len(t, tools, 1)
					assert.Equal(t, name, tools[0].Name)
					assert.Equal(t, map[string]any{"type": "tool", "name": name}, converted.ToolChoice)
					require.Len(t, converted.Messages, 3)
					content, err := converted.Messages[1].ParseContent()
					require.NoError(t, err)
					require.Len(t, content, 1)
					assert.Equal(t, name, content[0].Name)
					assert.Equal(t, map[string]any{"input": input}, content[0].Input)
				case *dto.GeminiChatRequest:
					tools := converted.GetTools()
					require.Len(t, tools, 1)
					functions, err := kitutil.Any2Type[[]dto.FunctionRequest](tools[0].FunctionDeclarations)
					require.NoError(t, err)
					require.Len(t, functions, 1)
					assert.Equal(t, name, functions[0].Name)
					require.NotNil(t, converted.ToolConfig)
					assert.Equal(t, []string{name}, converted.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
					require.Len(t, converted.Contents, 3)
					require.Len(t, converted.Contents[1].Parts, 1)
					call := converted.Contents[1].Parts[0].FunctionCall
					require.NotNil(t, call)
					assert.Equal(t, name, call.FunctionName)
					assert.Equal(t, map[string]any{"input": input}, call.Arguments)
					require.NotNil(t, converted.Contents[2].Parts[0].FunctionResponse)
					assert.Equal(t, name, converted.Contents[2].Parts[0].FunctionResponse.Name)
				default:
					t.Fatalf("unexpected converted request %T", result.Value)
				}

				_, err = ConvertRequest(nil, info, target, &dto.OpenAIResponsesRequest{
					Model: "model-test", MaxOutputTokens: lo.ToPtr(uint(64)), Input: mustMarshalRequestJSON(t, "retry"),
				})
				require.NoError(t, err)
				assert.Nil(t, info.ResponsesTools, "a retry must discard both custom-tool and namespace records")
			})
		}
	}
}

func TestConvertResponseRestoresNamespacedFunctionAndCustomCalls(t *testing.T) {
	for _, namespace := range []string{"functions", "workspace"} {
		t.Run(namespace, func(t *testing.T) {
			customName := convmeta.NamespacedToolName(namespace, "exec")
			functionName := convmeta.NamespacedToolName(namespace, "lookup")
			tools := &convmeta.ResponsesToolState{CustomToolNames: map[string]struct{}{customName: {}}}
			wantNamespace := ""
			if namespace != "functions" {
				wantNamespace = namespace
				tools.Namespaces = map[string]string{customName: namespace, functionName: namespace}
			}
			info := &convmeta.Values{ResponsesTools: tools}
			message := dto.Message{Role: "assistant"}
			message.SetToolCalls([]dto.ToolCallRequest{
				{ID: "call_exec", Type: "function", Function: dto.FunctionRequest{Name: customName, Arguments: `{"input":"你好\n<file>"}`}},
				{ID: "call_lookup", Type: "function", Function: dto.FunctionRequest{Name: functionName, Arguments: `{"q":"x"}`}},
			})
			result, err := ConvertResponse(nil, info, types.RelayFormatOpenAIResponses, &dto.OpenAITextResponse{
				Id: "chatcmpl_1", Choices: []dto.OpenAITextResponseChoice{{Message: message, FinishReason: "tool_calls"}},
			})
			require.NoError(t, err)
			response := result.Value.(*dto.OpenAIResponsesResponse)
			require.Len(t, response.Output, 2)
			assert.Equal(t, "custom_tool_call", response.Output[0].Type)
			assert.Equal(t, "exec", response.Output[0].Name)
			assert.Equal(t, wantNamespace, response.Output[0].Namespace)
			var input string
			require.NoError(t, kitutil.Unmarshal(response.Output[0].Input, &input))
			assert.Equal(t, "你好\n<file>", input)
			encoded, err := kitutil.Marshal(response.Output[0])
			require.NoError(t, err)
			var serialized map[string]any
			require.NoError(t, kitutil.Unmarshal(encoded, &serialized))
			if wantNamespace == "" {
				assert.NotContains(t, serialized, "namespace")
			} else {
				assert.Equal(t, wantNamespace, serialized["namespace"])
			}
			assert.Equal(t, "function_call", response.Output[1].Type)
			assert.Equal(t, "lookup", response.Output[1].Name)
			assert.Equal(t, wantNamespace, response.Output[1].Namespace)

			state, err := NewResponseStreamState(types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, ResponseStreamOptions{ID: "resp_1"})
			require.NoError(t, err)
			index := 0
			_, err = ConvertStreamResponseChunk(nil, info, state, &dto.ChatCompletionsStreamResponse{
				Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{{Index: &index, ID: "call_exec", Type: "function", Function: dto.FunctionResponse{Name: customName, Arguments: `{"input":"你好\n<file>"}`}}},
				}}},
			})
			require.NoError(t, err)
			finals, err := FinalizeStreamResponse(nil, info, state)
			require.NoError(t, err)
			require.NotEmpty(t, finals)
			completed := finals[len(finals)-1].Value.(ChatToResponsesStreamEvent)
			require.NotNil(t, completed.Payload.Response)
			require.Len(t, completed.Payload.Response.Output, 1)
			assert.Equal(t, response.Output[0], completed.Payload.Response.Output[0])
		})
	}
}

func TestConvertRequestResponsesToolOutputMediaToGemini(t *testing.T) {
	const pngData = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	for _, model := range []string{"gemini-2.5-pro", "gemini-3-pro"} {
		t.Run(model, func(t *testing.T) {
			request := &dto.OpenAIResponsesRequest{
				Model: model,
				Input: mustMarshalRequestJSON(t, []any{
					map[string]any{"type": "function_call", "name": "screenshot", "call_id": "call_1", "arguments": "{}"},
					map[string]any{"type": "function_call_output", "call_id": "call_1", "output": []any{
						map[string]any{"type": "input_text", "text": "Screenshot taken"},
						map[string]any{"type": "input_image", "image_url": "data:image/png;base64," + pngData},
					}},
				}),
			}
			result, err := ConvertRequest(nil, nil, types.RelayFormatGemini, request)
			require.NoError(t, err)
			converted := result.Value.(*dto.GeminiChatRequest)
			require.Len(t, converted.Contents, 2)
			parts := converted.Contents[1].Parts
			require.NotEmpty(t, parts)
			require.NotNil(t, parts[0].FunctionResponse)
			response := parts[0].FunctionResponse
			assert.Equal(t, "screenshot", response.Name)
			assert.Equal(t, map[string]any{"content": "Screenshot taken"}, response.Response)
			var media dto.GeminiPart
			if model == "gemini-2.5-pro" {
				require.Len(t, parts, 2)
				assert.Empty(t, response.Parts)
				media = parts[1]
			} else {
				require.Len(t, parts, 1)
				var responseParts []dto.GeminiPart
				require.NoError(t, kitutil.Unmarshal(response.Parts, &responseParts))
				require.Len(t, responseParts, 1)
				media = responseParts[0]
			}
			require.NotNil(t, media.InlineData)
			assert.Equal(t, "image/png", media.InlineData.MimeType)
			// TestMain resolves media to this fixed payload; the converter must
			// carry that payload as media rather than putting it in text.
			assert.Equal(t, "aGVsbG8=", media.InlineData.Data)
		})
	}
}

func TestConvertRequestResponsesNamespaceCollisionKeepsFirstIdentity(t *testing.T) {
	namespaced := map[string]any{
		"type": "namespace", "name": "workspace", "tools": []any{map[string]any{
			"type": "function", "name": "exec", "description": "namespaced", "parameters": map[string]any{"type": "object"},
		}},
	}
	bare := map[string]any{"type": "function", "name": "workspace__exec", "description": "bare", "parameters": map[string]any{"type": "object"}}
	for _, target := range []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude, types.RelayFormatGemini} {
		for _, namespaceFirst := range []bool{true, false} {
			order := "bare-first"
			if namespaceFirst {
				order = "namespace-first"
			}
			t.Run(string(target)+"/"+order, func(t *testing.T) {
				tools := []any{bare, namespaced}
				wantDescription, wantName, wantNamespace := "bare", "workspace__exec", ""
				choice := map[string]any{"type": "function", "name": "workspace__exec"}
				if namespaceFirst {
					tools = []any{namespaced, bare}
					wantDescription, wantName, wantNamespace = "namespaced", "exec", "workspace"
					choice = map[string]any{"type": "function", "name": "exec", "namespace": "workspace"}
				}
				request := &dto.OpenAIResponsesRequest{
					Model: "model-test", MaxOutputTokens: lo.ToPtr(uint(64)), Input: mustMarshalRequestJSON(t, "Run the tool"),
					Tools: mustMarshalRequestJSON(t, tools), ToolChoice: mustMarshalRequestJSON(t, choice),
				}
				info := &convmeta.Values{}
				result, err := ConvertRequest(nil, info, target, request)
				require.NoError(t, err)
				var collision bool
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Code == "namespaced_tool_dropped" {
						collision = true
						assert.Equal(t, types.ConversionDiagnosticError, diagnostic.Severity)
					}
				}
				assert.True(t, collision, "both declaration orders must report the semantic loss")
				switch converted := result.Value.(type) {
				case *dto.GeneralOpenAIRequest:
					require.Len(t, converted.Tools, 1)
					assert.Equal(t, wantDescription, converted.Tools[0].Function.Description)
					assert.Equal(t, "workspace__exec", converted.Tools[0].Function.Name)
					assert.Equal(t, map[string]any{"type": "function", "function": map[string]any{"name": "workspace__exec"}}, converted.ToolChoice)
				case *dto.ClaudeRequest:
					functions, err := kitutil.Any2Type[[]dto.Tool](converted.Tools)
					require.NoError(t, err)
					require.Len(t, functions, 1)
					assert.Equal(t, wantDescription, functions[0].Description)
					assert.Equal(t, "workspace__exec", functions[0].Name)
					assert.Equal(t, map[string]any{"type": "tool", "name": "workspace__exec"}, converted.ToolChoice)
				case *dto.GeminiChatRequest:
					groups := converted.GetTools()
					require.Len(t, groups, 1)
					functions, err := kitutil.Any2Type[[]dto.FunctionRequest](groups[0].FunctionDeclarations)
					require.NoError(t, err)
					require.Len(t, functions, 1)
					assert.Equal(t, wantDescription, functions[0].Description)
					assert.Equal(t, "workspace__exec", functions[0].Name)
					require.NotNil(t, converted.ToolConfig)
					assert.Equal(t, []string{"workspace__exec"}, converted.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
				default:
					t.Fatalf("unexpected converted request %T", result.Value)
				}

				message := dto.Message{Role: "assistant"}
				message.SetToolCalls([]dto.ToolCallRequest{{
					ID: "call_exec", Type: "function", Function: dto.FunctionRequest{Name: "workspace__exec", Arguments: "{}"},
				}})
				response, err := ConvertResponse(nil, info, types.RelayFormatOpenAIResponses, &dto.OpenAITextResponse{
					Id: "chatcmpl_1", Choices: []dto.OpenAITextResponseChoice{{Message: message, FinishReason: "tool_calls"}},
				})
				require.NoError(t, err)
				output := response.Value.(*dto.OpenAIResponsesResponse).Output
				require.Len(t, output, 1)
				assert.Equal(t, wantName, output[0].Name)
				assert.Equal(t, wantNamespace, output[0].Namespace)

				_, err = ConvertRequest(nil, &convmeta.Values{Options: &convmeta.Options{
					ToolLossPolicy: types.ConversionLossPolicySafe,
				}}, target, request)
				var loss *types.ConversionLossError
				require.ErrorAs(t, err, &loss)
				collision = false
				for _, diagnostic := range loss.Diagnostics {
					if diagnostic.Code == "namespaced_tool_dropped" {
						collision = true
					}
				}
				assert.True(t, collision, "safe rejection must identify the namespace collision")
			})
		}
	}
}

func TestConvertRequestResponsesBareDuplicateNamesKeepExistingPolicy(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Model: "model-test", Input: mustMarshalRequestJSON(t, "Run the tool"),
		Tools: mustMarshalRequestJSON(t, []any{
			map[string]any{"type": "function", "name": "exec", "description": "first", "parameters": map[string]any{"type": "object"}},
			map[string]any{"type": "function", "name": "exec", "description": "second", "parameters": map[string]any{"type": "object"}},
		}),
	}
	result, err := ConvertRequest(nil, &convmeta.Values{Options: &convmeta.Options{
		ToolLossPolicy: types.ConversionLossPolicySafe,
	}}, types.RelayFormatOpenAI, request)
	require.NoError(t, err)
	converted := result.Value.(*dto.GeneralOpenAIRequest)
	require.Len(t, converted.Tools, 2)
	assert.Equal(t, "first", converted.Tools[0].Function.Description)
	assert.Equal(t, "second", converted.Tools[1].Function.Description)
	for _, diagnostic := range result.Diagnostics {
		assert.NotEqual(t, "namespaced_tool_dropped", diagnostic.Code)
	}
}
