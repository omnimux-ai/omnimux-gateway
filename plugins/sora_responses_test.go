package plugins_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	taskplugin "github.com/QuantumNous/new-api/relay/channel/task/jsplugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSoraResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "sora",
		model:     "sora-2-pro",
		requestBody: map[string]any{
			"model":   "sora-2-pro",
			"input":   "waves at sunset",
			"seconds": 8,
			"size":    "1792x1024",
		},
		wantAction: "text_to_video",
		wantRequest: map[string]any{
			"model":   "sora-2-pro",
			"prompt":  "waves at sunset",
			"seconds": float64(8),
			"size":    "1792x1024",
		},
		wantUsageKeys:  []string{"seconds", "size"},
		wantVendorName: "sora",
	})
}

func loadSoraPlugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("sora")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "sora"})
	require.NoError(t, err)
	return plugin
}

func TestSoraAsyncImageTaskSupport(t *testing.T) {
	plugin := loadSoraPlugin(t)
	adaptor := taskplugin.New(plugin)

	// 1. buildQueryRequest for image models targets /v1/tasks/:id
	queryReqVal, err := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"baseUrl": "https://api.evolink.ai",
		"taskId":  "task-123",
		"apiKey":  "key-abc",
		"model":   "seedream-5-0-pro",
	})
	require.NoError(t, err)
	queryReqMap, ok := queryReqVal.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://api.evolink.ai/v1/tasks/task-123", queryReqMap["url"])

	// 2. parseTaskResult extracts status and url for completed image task
	pollBody := map[string]any{
		"id":      "task-123",
		"status":  "completed",
		"results": []any{"https://tos.example.com/image.jpg"},
	}
	pollJSON, _ := json.Marshal(pollBody)
	task := &model.Task{TaskID: "task_pub_123", Status: model.TaskStatusInProgress, Properties: model.Properties{OriginModelName: "seedream-5-0-pro"}}
	taskInfo, err := adaptor.ParseTaskResult(task, &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)}, pollJSON)
	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", taskInfo.Status)
	assert.Equal(t, "https://tos.example.com/image.jpg", taskInfo.Url)

	// 3. listArtifacts returns image artifact
	task.Status = model.TaskStatusSuccess
	task.SetData(pollBody)
	artifacts, err := adaptor.ListArtifacts(task)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	assert.Equal(t, "image", artifacts[0].Key)
	assert.Equal(t, "image", artifacts[0].Type)

	// 4. buildContentRequest returns image url
	contentVal, err := plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
		"artifactKey": "image",
		"data":        pollBody,
		"clientRequest": map[string]any{
			"method": "GET",
		},
	})
	require.NoError(t, err)
	contentMap, ok := contentVal.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://tos.example.com/image.jpg", contentMap["url"])
}
