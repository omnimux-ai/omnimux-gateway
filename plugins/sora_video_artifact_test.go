package plugins_test

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	taskplugin "github.com/QuantumNous/new-api/relay/channel/task/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSoraVideoContentPreservesAuthenticatedFallback(t *testing.T) {
	plugin := loadSoraPlugin(t)
	for _, tc := range []struct {
		name     string
		data     map[string]any
		upstream map[string]any
	}{
		{"official without delivery", map[string]any{"object": "video"}, nil},
		{"metadata is user input", map[string]any{"metadata": map[string]any{"url": "https://user.example/not-a-video"}}, nil},
		{"gateway requires its token", map[string]any{"url": "https://gateway.example/v1/videos/upstream/content"}, map[string]any{"kind": "new_api"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{"baseUrl": "https://provider.example", "apiKey": "provider-key", "upstreamTaskId": "upstream-task", "artifactKey": "video", "data": tc.data, "upstream": tc.upstream, "clientRequest": map[string]any{"method": "HEAD"}})
			require.NoError(t, err)
			request, ok := value.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "https://provider.example/v1/videos/upstream-task/content", request["url"])
			assert.Equal(t, map[string]any{"Authorization": "Bearer provider-key"}, request["headers"])
			assert.Equal(t, "HEAD", request["method"])
		})
	}
}

func TestSoraGatewayImageKeepsImageArtifact(t *testing.T) {
	plugin := loadSoraPlugin(t)
	adaptor := taskplugin.New(plugin)
	task := &model.Task{TaskID: "image-task", Status: model.TaskStatusSuccess}
	task.SetData(map[string]any{"object": "video", "model": "gpt-image-2.5", "results": []string{"https://cdn.example/image.png"}})
	artifacts, err := adaptor.ListArtifacts(task)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	assert.Equal(t, "image", artifacts[0].Key)
}

func TestSoraCompletedVideoUsesPersistedDeliveryURL(t *testing.T) {
	for _, field := range []string{"url", "video_url", "result_url", "data", "outer", "cover"} {
		t.Run(field, func(t *testing.T) {
			delivery := "https://cdn.example/video.mp4?signature=delivery"
			data := map[string]any{"id": "upstream-task", "object": "video", "status": "completed"}
			if field == "cover" {
				data["video_url"] = delivery
				data["images"] = []string{"https://cdn.example/cover.jpg"}
			} else if field == "data" {
				data = map[string]any{"data": map[string]any{"object": "video", "status": "completed", "url": delivery}}
			} else if field == "outer" {
				data = map[string]any{"data": map[string]any{"object": "video", "status": "completed"}, "url": delivery}
			} else {
				data[field] = delivery
			}
			plugin := loadSoraPlugin(t)
			adaptor := taskplugin.New(plugin)
			adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: 1, ChannelBaseUrl: "https://provider.example", ApiKey: "provider-key"}})
			task := &model.Task{TaskID: "task_public", Status: model.TaskStatusSuccess, Action: "influencer_motion", PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-task"}}
			task.SetData(data)
			artifacts, err := adaptor.ListArtifacts(task)
			require.NoError(t, err)
			require.Len(t, artifacts, 1)
			assert.Equal(t, "video", artifacts[0].Key)
			body, err := common.Marshal(data)
			require.NoError(t, err)
			result, err := adaptor.ParseTaskResult(task, &http.Response{StatusCode: 200, Header: make(http.Header)}, body)
			require.NoError(t, err)
			assert.Equal(t, delivery, result.Url)
			request, err := adaptor.BuildContentRequest(t.Context(), task, "video", relaychannel.TaskArtifactClientRequest{Method: http.MethodGet})
			require.NoError(t, err)
			assert.Equal(t, delivery, request.URL)
			assert.True(t, request.Credentialless)
			assert.Empty(t, request.Headers, "provider credentials must not reach the delivery host")
		})
	}
}
