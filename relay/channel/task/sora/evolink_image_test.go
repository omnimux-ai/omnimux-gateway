package sora

import (
	"io"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvoLinkGPTImageMappedRequestPreservesInputs(t *testing.T) {
	for _, upstream := range []string{"gpt-image-2.5-flare", "gpt-image-2.5-sunburst"} {
		for _, action := range []string{constant.TaskActionGenerate, constant.TaskActionImageEdit} {
			t.Run(upstream+"/"+action, func(t *testing.T) {
				info := newImageRelayInfo("gpt-image-2.5", action)
				info.UpstreamModelName = upstream
				adaptor := &TaskAdaptor{baseURL: "https://api.evolink.example/"}
				url, err := adaptor.BuildRequestURL(info)
				require.NoError(t, err)
				assert.Equal(t, "https://api.evolink.example/v1/images/generations", url)

				body := `{"model":"gpt-image-2.5","prompt":"Preserve every reference in order","images":["https://media.example/front.png",{"url":"https://media.example/back.png"}],"image_urls":["https://media.example/side.png"],"image":{"image_url":"https://media.example/detail.png"},"input_reference":"https://media.example/last.png","n":2,"size":"1920x1088","resolution":"2K","quality":"high","mask_url":"https://media.example/mask.png","background":"transparent","output_format":"webp","output_compression":0}`
				reader, err := adaptor.BuildRequestBody(newJSONImageContext(t, "/v1/images/edits", body), info)
				require.NoError(t, err)
				encoded, err := io.ReadAll(reader)
				require.NoError(t, err)
				var got map[string]any
				require.NoError(t, common.Unmarshal(encoded, &got))
				assert.Equal(t, map[string]any{
					"model": upstream, "prompt": "Preserve every reference in order",
					"image_urls": []any{"https://media.example/front.png", "https://media.example/back.png", "https://media.example/side.png", "https://media.example/detail.png", "https://media.example/last.png"},
					"n":          float64(2), "size": "1920x1088", "resolution": "2K", "quality": "high",
					"mask_url": "https://media.example/mask.png", "background": "transparent", "output_format": "webp", "output_compression": float64(0),
				}, got)
			})
		}
	}
}

func TestEvoLinkGPTImageReferenceValidationPreservesObjectAliases(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newImageRelayInfo("gpt-image-2.5", "")
	info.UpstreamModelName = "gpt-image-2.5-sunburst"
	ctx := newJSONImageContext(t, "/v1/images/edits", `{"model":"gpt-image-2.5","prompt":"preserve the face","image_urls":[{"url":"https://media.example/face.png"}]}`)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	assert.Equal(t, constant.TaskActionImageEdit, info.Action)
	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	gotBytes, err := io.ReadAll(reader)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.Unmarshal(gotBytes, &got))
	assert.Equal(t, []any{"https://media.example/face.png"}, got["image_urls"])
}

func TestEvoLinkGPTImageRejectsMalformedReferencesAndQuality(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"reference without URL": {"image_urls": []any{map[string]any{"caption": "face"}}},
		"non URL reference":     {"image_urls": []any{false}},
		"null quality":          {"quality": nil},
		"object quality":        {"quality": map[string]any{"level": "high"}},
		"number quality":        {"quality": 1},
	} {
		t.Run(name, func(t *testing.T) {
			body := map[string]any{"model": "gpt-image-2.5", "prompt": "preserve the face"}
			for key, value := range extra {
				body[key] = value
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
			info := newImageRelayInfo("gpt-image-2.5", "")
			info.UpstreamModelName = "gpt-image-2.5-flare"
			adaptor := &TaskAdaptor{}
			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			assert.False(t, taskErr.NoTaskCreated, "invalid client input must not gain a retry grant")
			reader, err := adaptor.BuildRequestBody(ctx, info)
			require.Error(t, err)
			assert.Nil(t, reader)
		})
	}
}

func TestEvoLinkGPTImageContractUsesSelectedUpstream(t *testing.T) {
	info := newImageRelayInfo("gpt-image-2.5-flare", constant.TaskActionImageEdit)
	info.UpstreamModelName = "gpt-image-2.5"
	adaptor := &TaskAdaptor{baseURL: "https://api.example"}
	url, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example/v1/images/edits", url)
	ctx := newJSONImageContext(t, "/v1/images/edits", `{"model":"gpt-image-2.5-flare","prompt":"a red apple","image":"https://media.example/apple.png","quality":"auto"}`)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	gotBytes, err := io.ReadAll(reader)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.Unmarshal(gotBytes, &got))
	assert.Equal(t, "auto", got["quality"])
	assert.Equal(t, []any{"https://media.example/apple.png"}, got["images"])
	assert.NotContains(t, got, "image_urls")
}

func TestEvoLinkGPTImageQualityContract(t *testing.T) {
	for _, quality := range []string{"", "low", "medium", "high", "xhigh", "max", "auto", "ultra"} {
		t.Run(quality, func(t *testing.T) {
			body := map[string]any{"model": "gpt-image-2.5", "prompt": "a red apple", "size": "1024x1024", "n": 1}
			if quality != "" {
				body["quality"] = quality
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			info := newImageRelayInfo("gpt-image-2.5", "")
			info.UpstreamModelName = "gpt-image-2.5-flare"
			ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
			adaptor := &TaskAdaptor{}
			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			reader, buildErr := adaptor.BuildRequestBody(ctx, info)
			if quality == "auto" || quality == "ultra" {
				require.NotNil(t, taskErr)
				assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
				assert.Equal(t, "invalid_request", taskErr.Code)
				require.ErrorContains(t, buildErr, "quality must be one of low, medium, high, xhigh, max")
				assert.Nil(t, reader)
				return
			}
			require.Nil(t, taskErr)
			require.NoError(t, buildErr)
			gotBytes, err := io.ReadAll(reader)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, common.Unmarshal(gotBytes, &got))
			assert.Equal(t, "gpt-image-2.5-flare", got["model"])
			assert.Equal(t, "1024x1024", got["size"])
			assert.Equal(t, float64(1), got["n"])
			if quality == "" {
				assert.NotContains(t, got, "quality", "an omitted quality retains the supplier's documented medium default")
			} else {
				assert.Equal(t, quality, got["quality"])
			}
		})
	}
}

func TestEvoLinkGPTImagePreparationDoesNotContaminateAnotherAttempt(t *testing.T) {
	const body = `{"model":"gpt-image-2.5","prompt":"a red apple","image":"https://media.example/reference.png","size":"1920x1088"}`
	ctx := newJSONImageContext(t, "/v1/images/edits", body)
	adaptor := &TaskAdaptor{}
	for _, upstream := range []string{"gpt-image-2.5-flare", "gpt-image-2.5"} {
		info := newImageRelayInfo("gpt-image-2.5", constant.TaskActionImageEdit)
		info.UpstreamModelName = upstream
		reader, err := adaptor.BuildRequestBody(ctx, info)
		require.NoError(t, err)
		gotBytes, err := io.ReadAll(reader)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, common.Unmarshal(gotBytes, &got))
		assert.Equal(t, upstream, got["model"])
		assert.Equal(t, "1920x1088", got["size"])
		assert.NotContains(t, got, "quality")
		if upstream == "gpt-image-2.5-flare" {
			assert.Equal(t, []any{"https://media.example/reference.png"}, got["image_urls"])
			assert.NotContains(t, got, "images")
		} else {
			assert.Equal(t, []any{"https://media.example/reference.png"}, got["images"])
			assert.NotContains(t, got, "image_urls")
		}
	}
}

func TestEvoLinkGPTImageNamesRouteOnlyExactVariants(t *testing.T) {
	for _, name := range []string{"gpt-image-2.5-flare", "gpt-image-2.5-sunburst"} {
		assert.True(t, IsEvoLinkGPTImageModel(name), name)
		assert.True(t, IsAsyncImageModel(name), name)
		info := newImageRelayInfo(name, constant.TaskActionGenerate)
		url, err := (&TaskAdaptor{baseURL: "https://api.evolink.example"}).BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://api.evolink.example/v1/images/generations", url)
	}
	for _, name := range []string{"gpt-image-2-beta", "gpt-image-2.5-flare-preview", "not-gpt-image-2.5-sunburst"} {
		assert.False(t, IsEvoLinkGPTImageModel(name), name)
		assert.False(t, IsAsyncImageModel(name), name)
	}
	assert.False(t, IsEvoLinkGPTImageModel("gpt-image-2.5"), "the public ID alone does not establish upstream capability")
}

func TestEvoLinkStandardGroupConvertsReviewedRatiosWithoutLoweringPixels(t *testing.T) {
	for ratio, pixels := range map[string]string{"1:1": "1024x1024", "2:3": "1024x1536", "3:2": "1536x1024", "3:4": "1024x1360", "4:3": "1360x1024", "9:16": "1088x1920", "16:9": "1920x1088"} {
		t.Run(ratio, func(t *testing.T) {
			info := newImageRelayInfo("gpt-image-2.5", constant.TaskActionGenerate)
			info.UpstreamModelName = "gpt-image-2.5-flare"
			info.UsingGroup = "gpt-image-2.5-economy"
			encoded, err := common.Marshal(map[string]any{"model": "gpt-image-2.5", "prompt": "a person", "size": ratio, "n": 1})
			require.NoError(t, err)
			ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
			reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, common.DecodeJson(reader, &got))
			assert.Equal(t, pixels, got["size"])
			assert.NotContains(t, got, "quality")
			var original map[string]any
			require.NoError(t, common.UnmarshalBodyReusable(ctx, &original))
			assert.Equal(t, ratio, original["size"], "supplier preparation must not mutate the frozen input")
		})
	}
	for _, scope := range []struct{ origin, group string }{
		{"gpt-image-2.5-flare", "gpt-image-2.5-economy"},
		{"gpt-image-2.5", "other"},
	} {
		info := newImageRelayInfo(scope.origin, constant.TaskActionGenerate)
		info.UpstreamModelName = "gpt-image-2.5-flare"
		info.UsingGroup = scope.group
		ctx := newJSONImageContext(t, "/v1/images/generations", `{"model":"gpt-image-2.5","prompt":"a person","size":"16:9","quality":"high"}`)
		reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, common.DecodeJson(reader, &got))
		assert.Equal(t, "16:9", got["size"], "other public IDs and groups retain their existing supplier contract")
		assert.Equal(t, "high", got["quality"])
	}
}
