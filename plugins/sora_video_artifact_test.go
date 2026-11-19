package plugins_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	taskplugin "github.com/QuantumNous/new-api/relay/channel/task/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
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

func TestMuseVideoReferencesReachSubmitInOrder(t *testing.T) {
	plugin := loadSoraPlugin(t)
	first, second := "https://example.com/first.png", "https://example.com/second.png"
	for _, test := range []struct {
		name    string
		request map[string]any
		want    map[string]any
		wantErr string
	}{
		{name: "page order", request: map[string]any{"operation": "video_multi_ref", "image_urls": []string{first, second}}, want: map[string]any{"images": []any{first, second}}},
		{name: "page reversed", request: map[string]any{"operation": "video_multi_ref", "image_urls": []string{second, first}}, want: map[string]any{"images": []any{second, first}}},
		{name: "API native order", request: map[string]any{"images": []any{map[string]any{"url": second}, first}}, want: map[string]any{"images": []any{second, first}}},
		{name: "page first frame", request: map[string]any{"operation": "first_frame", "image_with_roles": []map[string]any{{"role": "first_frame", "url": first}}}, want: map[string]any{"image": first}},
		{name: "API single input", request: map[string]any{"input_reference": first}, want: map[string]any{"image": first}},
		{name: "text mode", request: map[string]any{"operation": "text_to_video"}, want: map[string]any{}},
		{name: "missing image", request: map[string]any{"operation": "video_multi_ref", "image_urls": []string{first}}, wantErr: "exactly two images"},
		{name: "too many images", request: map[string]any{"images": []string{first, second, first}}, wantErr: "exactly two images"},
		{name: "empty image", request: map[string]any{"operation": "video_multi_ref", "image_urls": []string{first, " "}}, wantErr: "must be non-empty"},
		{name: "mixed image fields", request: map[string]any{"image": first, "image_urls": []string{first, second}}, wantErr: "only one set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.request["prompt"] = "Create a video"
			test.request["aspect_ratio"] = "16:9"
			decoded, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
				"model": "muse-video", "body": map[string]any{"kind": "json", "value": test.request},
			})
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			intent, ok := decoded.(map[string]any)
			require.True(t, ok)
			descriptor, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
				"baseUrl": "https://example.com", "upstreamModel": "muse-video", "requestBody": intent["requestBody"], "action": intent["action"],
			})
			require.NoError(t, err)
			request, ok := descriptor.(map[string]any)
			require.True(t, ok)
			body, ok := request["body"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "1280x720", body["size"])
			assert.NotContains(t, body, "image_urls")
			assert.NotContains(t, body, "image_with_roles")
			assert.NotContains(t, body, "operation")
			for _, field := range []string{"image", "images"} {
				assert.Equal(t, test.want[field], body[field])
			}
		})
	}
}

func TestMuseVideoSubmitSnapshotsOnlyImageBytes(t *testing.T) {
	plugin := loadSoraPlugin(t)
	images := make([]string, 2)
	for index, pixel := range []color.RGBA{{R: 255, A: 255}, {B: 255, A: 255}} {
		picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
		picture.Set(0, 0, pixel)
		var buffer bytes.Buffer
		require.NoError(t, png.Encode(&buffer, picture))
		images[index] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())
	}
	mp4Header := base64.StdEncoding.EncodeToString([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"))
	for _, test := range []struct {
		name      string
		inputs    []string
		wantError bool
	}{
		{name: "reversed images", inputs: []string{images[1], images[0]}},
		{name: "video data URL", inputs: []string{images[0], "data:video/mp4;base64," + mp4Header}, wantError: true},
		{name: "video mislabeled as image", inputs: []string{images[0], "data:image/png;base64," + mp4Header}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
			ctx.Set(pluginruntime.ContextKeyPinnedEndpoint, pluginruntime.PinnedEndpoint{Plugin: plugin, Protocol: "openai_video", Model: "muse-video"})
			ctx.Set(pluginruntime.ContextKeyProtocolRequest, pluginruntime.ProtocolRequestContext{
				Protocol: "openai_video", Model: "muse-video",
				RouteRequestContext: pluginruntime.RouteRequestContext{Body: map[string]any{"kind": "json", "value": map[string]any{
					"model": "muse-video", "operation": "video_multi_ref", "image_urls": test.inputs, "prompt": "Create a video",
				}}},
			})
			info := &relaycommon.RelayInfo{
				OriginModelName: "muse-video", TaskRelayInfo: &relaycommon.TaskRelayInfo{},
				ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://provider.example", UpstreamModelName: "muse-video"},
			}
			adaptor := taskplugin.New(plugin)
			adaptor.Init(info)
			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			if test.wantError {
				require.NotNil(t, taskErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
				assert.Contains(t, taskErr.Message, "reference image 2")
				return
			}
			require.Nil(t, taskErr)
			reader, err := adaptor.BuildRequestBody(ctx, info)
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			var upstream struct {
				Images []string `json:"images"`
			}
			require.NoError(t, common.Unmarshal(data, &upstream))
			assert.Equal(t, test.inputs, upstream.Images)
		})
	}
}
