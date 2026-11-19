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
	"github.com/QuantumNous/new-api/service"
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
	service.InitHttpClient()
	plugin := loadSoraPlugin(t)
	images := make([]string, 11)
	for index := range images {
		picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
		picture.Set(0, 0, color.RGBA{R: uint8(index + 1), A: 255})
		var buffer bytes.Buffer
		require.NoError(t, png.Encode(&buffer, picture))
		images[index] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())
	}
	mp4Header := base64.StdEncoding.EncodeToString([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"))
	for _, test := range []struct {
		name    string
		model   string
		mode    string
		inputs  []string
		wantErr string
	}{
		{name: "first frame", mode: "first_frame", inputs: images[:1]},
		{name: "one reference", inputs: images[:1], wantErr: "requires between 2 and 10 images"},
		{name: "reversed images", inputs: []string{images[1], images[0]}},
		{name: "three references", inputs: []string{images[2], images[0], images[1]}},
		{name: "ten references through mapped alias", model: "muse-public-alias", inputs: images[:10]},
		{name: "eleven references", inputs: images, wantErr: "requires between 2 and 10 images"},
		{name: "video data URL", inputs: []string{images[0], "data:video/mp4;base64," + mp4Header}, wantErr: "reference image 2"},
		{name: "video mislabeled as image", inputs: []string{images[0], "data:image/png;base64," + mp4Header}, wantErr: "reference image 2"},
		{name: "video through mapped alias", model: "muse-public-alias", inputs: []string{images[0], "data:video/mp4;base64," + mp4Header}, wantErr: "reference image 2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			bodies := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/v1/videos", r.URL.Path)
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				bodies <- body
				_, _ = io.WriteString(w, `{"id":"mock-task","status":"queued"}`)
			}))
			defer server.Close()
			publicModel := test.model
			if publicModel == "" {
				publicModel = "muse-video"
			}
			operation := test.mode
			if operation == "" {
				operation = "video_multi_ref"
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
			ctx.Set(pluginruntime.ContextKeyPinnedEndpoint, pluginruntime.PinnedEndpoint{Plugin: plugin, Protocol: "openai_video", Model: publicModel, MappedModel: "muse-video"})
			ctx.Set(pluginruntime.ContextKeyProtocolRequest, pluginruntime.ProtocolRequestContext{
				Protocol: "openai_video", Model: publicModel, UpstreamModel: "muse-video",
				RouteRequestContext: pluginruntime.RouteRequestContext{Body: map[string]any{"kind": "json", "value": map[string]any{
					"model": publicModel, "operation": operation, "image_urls": test.inputs, "prompt": "Create a video",
				}}},
			})
			info := &relaycommon.RelayInfo{
				OriginModelName: publicModel, TaskRelayInfo: &relaycommon.TaskRelayInfo{},
				ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: server.URL, UpstreamModelName: "muse-video"},
			}
			adaptor := taskplugin.New(plugin)
			adaptor.Init(info)
			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			if test.wantErr != "" {
				require.NotNil(t, taskErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
				assert.Contains(t, taskErr.Message, test.wantErr)
				assert.Empty(t, bodies)
				return
			}
			require.Nil(t, taskErr)
			reader, err := adaptor.BuildRequestBody(ctx, info)
			require.NoError(t, err)
			response, err := adaptor.DoRequest(ctx, info, reader)
			require.NoError(t, err)
			defer response.Body.Close()
			assert.Equal(t, http.StatusOK, response.StatusCode)
			require.Len(t, bodies, 1)
			var upstream map[string]any
			require.NoError(t, common.Unmarshal(<-bodies, &upstream))
			assert.NotContains(t, upstream, "operation")
			assert.NotContains(t, upstream, "image_urls")
			if operation == "first_frame" {
				assert.Equal(t, test.inputs[0], upstream["image"])
				assert.NotContains(t, upstream, "images")
			} else {
				assert.NotContains(t, upstream, "image")
				ordered := make([]any, len(test.inputs))
				for index, reference := range test.inputs {
					ordered[index] = reference
				}
				assert.Equal(t, ordered, upstream["images"])
			}
		})
	}
}
