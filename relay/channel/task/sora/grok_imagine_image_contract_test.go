package sora

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func explicitImageInfo(protocol string, modes ...string) *relaycommon.RelayInfo {
	info := newImageRelayInfo("gpt-image-2.5-sunburst", constant.TaskActionGenerate)
	info.ChannelOtherSettings.ImageTaskContract = &relaykitdto.ImageTaskContract{Protocol: protocol, Modes: modes}
	return info
}

const explicitTextBody = `{"model":"gpt-image-2.5-sunburst","image_mode":"text","prompt":"a blue kettle","quality":"high","size":"2048x1152","n":1,"output_format":"png","response_format":"url","group":"supplier-group"}`

func TestExplicitImageTaskMappedAliasPreservesPublicQuoteAndBody(t *testing.T) {
	for _, upstream := range []string{"gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst-1k"} {
		for _, protocol := range []string{relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskProtocolEvoLinkJSON} {
			t.Run(upstream+"/"+protocol, func(t *testing.T) {
				ctx := newJSONImageContext(t, "/api/influencer/sheet", explicitTextBody)
				mapping, err := common.Marshal(map[string]string{"gpt-image-2.5-sunburst": upstream})
				require.NoError(t, err)
				ctx.Set("model_mapping", string(mapping))
				info := explicitImageInfo(protocol, relaykitdto.ImageTaskModeText)
				info.PriceData.ModelPrice = 0.075
				info.PriceData.Quota = 37500
				quote := info.PriceData
				require.NoError(t, helper.ModelMappedHelper(ctx, info, nil))
				adaptor := &TaskAdaptor{baseURL: "https://supplier.example"}
				require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
				reader, err := adaptor.BuildRequestBody(ctx, info)
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, common.DecodeJson(reader, &body))
				assert.Equal(t, upstream, body["model"])
				// Alias recognition does not approve a lower output specification.
				assert.Equal(t, "2048x1152", body["size"])
				assert.Equal(t, "high", body["quality"])
				assert.Equal(t, "png", body["output_format"])
				assert.Equal(t, "gpt-image-2.5-sunburst", info.OriginModelName)
				assert.Equal(t, quote, info.PriceData)
				var original map[string]any
				require.NoError(t, common.UnmarshalBodyReusable(ctx, &original))
				assert.Equal(t, "gpt-image-2.5-sunburst", original["model"])
				assert.Equal(t, "supplier-group", original["group"])
				url, err := adaptor.BuildRequestURL(info)
				require.NoError(t, err)
				assert.Equal(t, "https://supplier.example/v1/images/generations", url)
			})
		}
	}
}

func TestExplicitImageTaskRejectsUnrecognizedUpstreamAliases(t *testing.T) {
	for _, upstream := range []string{
		"gpt-image-2.5", "gpt-image-2.5-1k", "gpt-image-2.5-sunburst-1k-preview",
		"gpt-image-2.5-sunburst-1K", "gpt-image-2.5-sunburst ", " gpt-image-2.5-sunburst-1k",
	} {
		t.Run(upstream, func(t *testing.T) {
			info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
			info.UpstreamModelName = upstream
			ctx := newJSONImageContext(t, "/api/influencer/sheet", explicitTextBody)
			adaptor := &TaskAdaptor{}
			failure := adaptor.ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, failure)
			assert.Equal(t, "invalid_image_task_contract", failure.Code)
			reader, err := adaptor.BuildRequestBody(ctx, info)
			require.Error(t, err)
			assert.Nil(t, reader)
		})
	}
}

func TestExplicitImageTaskAliasDoesNotGrantUndeclaredEditMode(t *testing.T) {
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
	info.UpstreamModelName = "gpt-image-2.5-sunburst-1k"
	ctx := newJSONImageContext(t, "/api/influencer/sheet", strings.Replace(explicitTextBody, `"image_mode":"text"`, `"image_mode":"edit"`, 1))
	adaptor := &TaskAdaptor{}
	failure := adaptor.ValidateRequestAndSetAction(ctx, info)
	require.NotNil(t, failure)
	assert.Equal(t, http.StatusBadRequest, failure.StatusCode)
	assert.Equal(t, "invalid_request", failure.Code)
	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.Error(t, err)
	assert.Nil(t, reader)
}

func TestExplicitImageTaskTextPreservesReviewedSpecAndOriginalBody(t *testing.T) {
	ctx := newJSONImageContext(t, "/api/influencer/sheet", explicitTextBody)
	adaptor := &TaskAdaptor{baseURL: "https://unrelated-provider.example/", apiKey: "test-key"}
	for _, protocol := range []string{relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskProtocolEvoLinkJSON} {
		info := explicitImageInfo(protocol, relaykitdto.ImageTaskModeText)
		require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
		reader, err := adaptor.BuildRequestBody(ctx, info)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, common.DecodeJson(reader, &got))
		assert.Equal(t, map[string]any{"model": "gpt-image-2.5-sunburst", "prompt": "a blue kettle", "quality": "high", "size": "2048x1152", "n": float64(1), "output_format": "png", "response_format": "url"}, got)
		request, err := http.NewRequest(http.MethodPost, "https://unrelated-provider.example", nil)
		require.NoError(t, err)
		require.NoError(t, adaptor.BuildRequestHeader(ctx, request, info))
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
		url, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://unrelated-provider.example/v1/images/generations", url)
	}
	var original map[string]any
	require.NoError(t, common.UnmarshalBodyReusable(ctx, &original))
	assert.Equal(t, "text", original["image_mode"])
	assert.Equal(t, "supplier-group", original["group"])
}

func TestExplicitImageTaskRejectsUnsupportedModeAndSpecBeforePreparation(t *testing.T) {
	for name, replacement := range map[string]map[string]any{
		"unsupported mode":  {"image_mode": "masked_edit", "image": "https://media.example/input.png", "mask_url": "https://media.example/mask.png"},
		"lower quality":     {"quality": "medium"},
		"multiple images":   {"n": float64(2)},
		"unexpected size":   {"size": "2688x1520"},
		"wrong format":      {"output_format": "jpeg"},
		"inline response":   {"response_format": "b64_json"},
		"mask without mode": {"mask_url": "https://media.example/mask.png"},
		"untyped mask":      {"mask_url": float64(1)},
	} {
		t.Run(name, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, common.Unmarshal([]byte(explicitTextBody), &body))
			for key, value := range replacement {
				body[key] = value
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			ctx := newJSONImageContext(t, "/api/influencer/sheet", string(encoded))
			info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
			failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, failure)
			assert.Equal(t, http.StatusBadRequest, failure.StatusCode)
			assert.False(t, failure.NoTaskCreated)
			reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
			require.Error(t, err)
			assert.Nil(t, reader)
		})
	}
}

func TestExplicitImageTaskRejectsMissingFrozenInputAndUsesConfiguredEndpoint(t *testing.T) {
	const body = `{"model":"gpt-image-2.5-sunburst","image_mode":"edit","prompt":"change the cup","image":"https://media.example/input.png","quality":"high","size":"2048x1152","n":1,"output_format":"png","response_format":"url"}`
	for _, protocol := range []string{relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskProtocolEvoLinkJSON} {
		ctx := newJSONImageContext(t, "/api/influencer/sheet", body)
		info := explicitImageInfo(protocol, relaykitdto.ImageTaskModeEdit)
		info.Action = constant.TaskActionImageEdit
		adaptor := &TaskAdaptor{baseURL: "https://arbitrary.example"}
		failure := adaptor.ValidateRequestAndSetAction(ctx, info)
		require.NotNil(t, failure)
		assert.Equal(t, http.StatusBadRequest, failure.StatusCode)
		url, err := adaptor.BuildRequestURL(info)
		require.NoError(t, err)
		expectedPath := "/v1/images/generations"
		if protocol == relaykitdto.ImageTaskProtocolOpenAIMultipart {
			expectedPath = "/v1/images/edits"
		}
		assert.Equal(t, "https://arbitrary.example"+expectedPath, url)
		reader, err := adaptor.BuildRequestBody(ctx, info)
		require.Error(t, err)
		assert.Nil(t, reader)
	}
}

func TestExplicitImageTaskRequiresExactMappedModelWithoutChangingLegacyModels(t *testing.T) {
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
	info.UpstreamModelName = "gpt-image-2.5-flare"
	ctx := newJSONImageContext(t, "/api/influencer/sheet", explicitTextBody)
	failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
	require.NotNil(t, failure)
	assert.Equal(t, "invalid_image_task_contract", failure.Code)
	_, err := (&TaskAdaptor{}).BuildRequestURL(info)
	require.Error(t, err)
	// The same channel's existing Flare contract must not acquire the new
	// public Sunburst specification or supported-mode restrictions.
	info.OriginModelName = "gpt-image-2.5-flare"
	contract, err := imageTaskContractFor(info)
	require.NoError(t, err)
	assert.Nil(t, contract)
}

func TestExplicitImageTaskCannotReportInlineOnlyAsCompleted(t *testing.T) {
	for name, body := range map[string]string{
		"inline only":        `{"data":[{"b64_json":"aW1hZ2U="}]}`,
		"multiple results":   `{"data":[{"url":"https://media.example/a.png"},{"url":"https://media.example/b.png"}]}`,
		"invalid result URL": `{"data":[{"url":"http://media.example/a.png"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := newJSONImageContext(t, "/api/influencer/sheet", explicitTextBody)
			info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
			info.PublicTaskID = "task_explicit"
			result, failure := (&TaskAdaptor{}).ParseResponse(ctx, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}, info)
			require.NotNil(t, failure)
			assert.Nil(t, result)
			assert.True(t, failure.NoRetry)
			assert.False(t, failure.NoTaskCreated, "a successful supplier response never proves no work was created")
		})
	}
}

func TestExplicitImageTaskSyncURLKeepsNativeArtifactSource(t *testing.T) {
	ctx := newJSONImageContext(t, "/api/influencer/sheet", explicitTextBody)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
	info.PublicTaskID = "task_explicit_result"
	body := `{"created":1792150000,"data":[{"url":"https://media.example/one.png?original=1"}]}`
	result, failure := (&TaskAdaptor{}).ParseResponse(ctx, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(body))}, info)
	require.Nil(t, failure)
	require.NotNil(t, result.Immediate)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Immediate.Status)
	assert.Equal(t, "https://media.example/one.png?original=1", result.Immediate.Url)
	assert.Equal(t, "task_explicit_result", result.UpstreamTaskID)
	responseBody, ok := result.ClientResponse.([]byte)
	require.True(t, ok)
	assert.NotContains(t, string(responseBody), "media.example")
	assert.Contains(t, string(responseBody), "/v1/images/task_explicit_result/content")
}

func frozenImageTaskFixture(t *testing.T, sourceFormats ...string) (service.InfluencerFrozenImage, service.InfluencerFrozenImage, []byte, []byte) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.File{}, &model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "image-task-owner", Status: 1}).Error)
	t.Setenv("FILE_PUBLIC_BASE_URL", "https://8.8.8.8/frozen-test")
	driver := storage.NewLocalStorageDriver(t.TempDir())
	oldDB, oldDriver := model.DB, storage.GetStorageDriver()
	fetchSetting := system_setting.GetFetchSetting()
	oldFetchSetting := *fetchSetting
	*fetchSetting = system_setting.FetchSetting{}
	model.DB = db
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() {
		model.DB = oldDB
		storage.SetGlobalDriver(oldDriver)
		*fetchSetting = oldFetchSetting
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	var snapshots []service.InfluencerFrozenImage
	var contents [][]byte
	for _, name := range []string{"source", "mask"} {
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: 120, G: 180, B: 200, A: 255})
			}
		}
		if name == "mask" {
			img.SetNRGBA(0, 0, color.NRGBA{A: 0})
		}
		var content bytes.Buffer
		format := "png"
		if name == "source" && len(sourceFormats) > 0 {
			format = sourceFormats[0]
		}
		switch format {
		case "jpeg":
			require.NoError(t, jpeg.Encode(&content, img, nil))
		case "webp":
			data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
			require.NoError(t, err)
			_, err = content.Write(data)
			require.NoError(t, err)
		default:
			require.NoError(t, png.Encode(&content, img))
		}
		data := content.Bytes()
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		require.NoError(t, err)
		extension := format
		if extension == "jpeg" {
			extension = "jpg"
		}
		fileName := name + "." + extension
		key := "influencer-image/frozen/" + fileName
		url, err := driver.Upload(context.Background(), key, data, "image/"+format)
		require.NoError(t, err)
		expiry := time.Now().Add(time.Hour).Truncate(time.Second)
		file := &model.File{FileId: "file_" + name, UserId: 41, FileName: fileName, OriginalName: fileName, FileSize: int64(len(data)), MimeType: "image/" + format, StorageKey: key, UploadPath: "influencer-image/frozen", FileUrl: url, CreatedAt: time.Now(), ExpiresAt: expiry, Status: model.FileStatusActive}
		require.NoError(t, db.Create(file).Error)
		digest := sha256.Sum256(data)
		snapshots = append(snapshots, service.InfluencerFrozenImage{URL: url, FileID: file.FileId, SHA256: fmt.Sprintf("%x", digest), Width: config.Width, Height: config.Height, Format: format, ExpiresAt: expiry.Unix()})
		contents = append(contents, data)
	}
	return snapshots[0], snapshots[1], contents[0], contents[1]
}

func TestExplicitImageTaskEvoLinkUsesOnlyVerifiedFrozenInputs(t *testing.T) {
	source, mask, _, _ := frozenImageTaskFixture(t)
	for _, mode := range []string{relaykitdto.ImageTaskModeReference, relaykitdto.ImageTaskModeEdit, relaykitdto.ImageTaskModeMaskedEdit} {
		t.Run(mode, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, common.Unmarshal([]byte(explicitTextBody), &body))
			body["image_mode"], body["image"] = mode, source.URL
			if mode == relaykitdto.ImageTaskModeMaskedEdit {
				body["mask_url"] = mask.URL
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			ctx := newJSONImageContext(t, "/api/influencer/sheet", string(encoded))
			ctx.Set("influencer_frozen_image", source)
			ctx.Set("influencer_frozen_mask", mask)
			info := explicitImageInfo(relaykitdto.ImageTaskProtocolEvoLinkJSON, mode)
			info.UserId = 41
			reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, common.DecodeJson(reader, &got))
			assert.Equal(t, []any{source.URL}, got["image_urls"])
			assert.NotContains(t, got, "image")
			assert.NotContains(t, got, "image_mode")
			if mode == relaykitdto.ImageTaskModeMaskedEdit {
				assert.Equal(t, mask.URL, got["mask_url"])
			} else {
				assert.NotContains(t, got, "mask_url")
			}
			info.UserId = 42
			reader, err = (&TaskAdaptor{}).BuildRequestBody(ctx, info)
			require.Error(t, err)
			assert.Nil(t, reader)
		})
	}
}

func TestExplicitImageTaskPublicDefaultsDoNotMutateCanonicalInput(t *testing.T) {
	const raw = `{"model":"gpt-image-2.5-sunburst","prompt":"a blue kettle"}`
	ctx := newJSONImageContext(t, "/v1/images/generations", raw)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
	require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info))
	reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.DecodeJson(reader, &got))
	assert.Equal(t, "high", got["quality"])
	assert.Equal(t, "2048x1152", got["size"])
	assert.Equal(t, float64(1), got["n"])
	assert.Equal(t, "png", got["output_format"])
	assert.Equal(t, "url", got["response_format"])
	var original map[string]any
	require.NoError(t, common.UnmarshalBodyReusable(ctx, &original))
	assert.Len(t, original, 2)
	bad := newJSONImageContext(t, "/v1/images/generations", `{"model":"gpt-image-2.5-sunburst","prompt":"a kettle","quality":null}`)
	require.NotNil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(bad, info))
	longPrompt := newJSONImageContext(t, "/v1/images/generations", `{"model":"gpt-image-2.5-sunburst","prompt":"`+strings.Repeat("a", 8193)+`"}`)
	require.NotNil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(longPrompt, info))
}

func TestExplicitImageTaskPublicModeInferenceReusesFrozenInputs(t *testing.T) {
	source, mask, _, _ := frozenImageTaskFixture(t)
	for _, mode := range []string{relaykitdto.ImageTaskModeReference, relaykitdto.ImageTaskModeMaskedEdit} {
		t.Run(mode, func(t *testing.T) {
			body := map[string]any{"model": "gpt-image-2.5-sunburst", "prompt": "preserve and edit", "image": source.URL}
			if mode == relaykitdto.ImageTaskModeMaskedEdit {
				body["mask_url"] = mask.URL
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
			ctx.Set("influencer_frozen_image", source)
			ctx.Set("influencer_frozen_mask", mask)
			info := explicitImageInfo(relaykitdto.ImageTaskProtocolEvoLinkJSON, mode)
			info.UserId = 41
			for attempt := 0; attempt < 2; attempt++ {
				require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info))
				assert.Equal(t, constant.TaskActionImageEdit, info.Action)
				reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
				require.NoError(t, err)
				var got map[string]any
				require.NoError(t, common.DecodeJson(reader, &got))
				assert.Equal(t, []any{source.URL}, got["image_urls"])
				assert.Equal(t, "high", got["quality"])
				if mode == relaykitdto.ImageTaskModeMaskedEdit {
					assert.Equal(t, mask.URL, got["mask_url"])
				}
			}
			var count int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
			assert.EqualValues(t, 2, count, "an existing frozen input is never frozen again on retry")
		})
	}
}

type imageTaskInputFetch func(*http.Request) (*http.Response, error)

func (fetch imageTaskInputFetch) RoundTrip(request *http.Request) (*http.Response, error) {
	return fetch(request)
}

func TestExplicitImageTaskRawSourceIsFrozenOnceAcrossAttempts(t *testing.T) {
	_, _, original, _ := frozenImageTaskFixture(t)
	raw := `{"model":"gpt-image-2.5-sunburst","prompt":"edit an image","image":"https://8.8.4.4/external.png"}`
	ctx := newJSONImageContext(t, "/v1/images/generations", raw)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolEvoLinkJSON, relaykitdto.ImageTaskModeReference)
	info.UserId = 41
	calls := 0
	client := &http.Client{Transport: imageTaskInputFetch(func(request *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Empty(t, request.Header.Get("Authorization"))
		assert.Empty(t, request.Header.Get("Cookie"))
		data := original
		if calls > 1 {
			data = []byte("changed original")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}
	var rawBody map[string]any
	require.NoError(t, common.UnmarshalBodyReusable(ctx, &rawBody))
	body, err := normalizedImageTaskInput(rawBody)
	require.NoError(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		require.NoError(t, freezeExplicitImageTaskInputs(ctx, info, body, relaykitdto.ImageTaskModeReference, client))
		reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
		require.NoError(t, err)
		var got map[string]any
		require.NoError(t, common.DecodeJson(reader, &got))
		assert.NotEqual(t, []any{"https://8.8.4.4/external.png"}, got["image_urls"])
	}
	assert.Equal(t, 1, calls)
	snapshot := ctx.MustGet("influencer_frozen_image").(service.InfluencerFrozenImage)
	audit := ctx.MustGet("influencer_image_inputs").(*model.GenerationImageInputs)
	assert.Equal(t, 1, audit.Version)
	assert.Equal(t, relaykitdto.ImageTaskModeReference, audit.Mode)
	require.NotNil(t, audit.Source)
	assert.Equal(t, snapshot, *audit.Source)
	assert.Nil(t, audit.Mask)
	retained, err := service.ReadInfluencerFrozenImage(context.Background(), 41, snapshot)
	require.NoError(t, err)
	assert.Equal(t, original, retained)
}

func TestExplicitImageTaskInvalidRawMaskCleansOnlyNewFrozenFiles(t *testing.T) {
	_, _, original, _ := frozenImageTaskFixture(t)
	ctx := newJSONImageContext(t, "/v1/images/generations", `{}`)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolEvoLinkJSON, relaykitdto.ImageTaskModeMaskedEdit)
	info.UserId = 41
	client := &http.Client{Transport: imageTaskInputFetch(func(request *http.Request) (*http.Response, error) {
		// A fully opaque source supplied as a mask has no editable pixels.
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), ContentLength: int64(len(original)), Body: io.NopCloser(bytes.NewReader(original))}, nil
	})}
	body := map[string]any{"image": "https://8.8.4.4/source.png", "mask_url": "https://8.8.4.4/mask.png"}
	require.Error(t, freezeExplicitImageTaskInputs(ctx, info, body, relaykitdto.ImageTaskModeMaskedEdit, client))
	var count int64
	require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
	assert.EqualValues(t, 2, count, "pre-existing files remain, newly rejected source and mask are discarded")
	_, sourceExists := ctx.Get("influencer_frozen_image")
	_, maskExists := ctx.Get("influencer_frozen_mask")
	assert.False(t, sourceExists)
	assert.False(t, maskExists)
}

func TestExplicitImageTaskPreservesAndVerifiesExistingPrivateInputAudit(t *testing.T) {
	source, mask, _, _ := frozenImageTaskFixture(t)
	ctx := newJSONImageContext(t, "/v1/images/generations", `{}`)
	original := &model.GenerationImageInputs{Version: 1, Mode: relaykitdto.ImageTaskModeMaskedEdit, Source: &source, Mask: &mask}
	ctx.Set("influencer_image_inputs", original)
	equivalentSource, equivalentMask := source, mask
	require.NoError(t, retainImageTaskInputAudit(ctx, &model.GenerationImageInputs{Version: 1, Mode: relaykitdto.ImageTaskModeMaskedEdit, Source: &equivalentSource, Mask: &equivalentMask}))
	assert.Same(t, original, ctx.MustGet("influencer_image_inputs"))
	equivalentMask.SHA256 = strings.Repeat("0", 64)
	require.Error(t, retainImageTaskInputAudit(ctx, &model.GenerationImageInputs{Version: 1, Mode: relaykitdto.ImageTaskModeMaskedEdit, Source: &equivalentSource, Mask: &equivalentMask}))
	assert.Same(t, original, ctx.MustGet("influencer_image_inputs"))
}

func TestExplicitImageTaskMultipartPreservesOriginalFileAndRebuildsEachAttempt(t *testing.T) {
	for _, format := range []string{"png", "jpeg", "webp"} {
		t.Run(format, func(t *testing.T) {
			source, _, original, _ := frozenImageTaskFixture(t, format)
			for _, mode := range []string{relaykitdto.ImageTaskModeReference, relaykitdto.ImageTaskModeEdit} {
				t.Run(mode, func(t *testing.T) {
					body := map[string]any{"model": "gpt-image-2.5-sunburst", "prompt": "保留原物体并换背景", "image_mode": mode, "image": source.URL, "group": "reviewed-group"}
					encoded, err := common.Marshal(body)
					require.NoError(t, err)
					ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
					ctx.Set("influencer_frozen_image", source)
					info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, mode)
					info.ChannelOtherSettings.ImageTaskContract.InputFormats = []string{format}
					info.UserId = 41
					adaptor := &TaskAdaptor{baseURL: "https://selected.example"}
					lastBoundary := ""
					for attempt := 0; attempt < 2; attempt++ {
						upstream := "gpt-image-2.5-sunburst"
						if attempt == 1 {
							upstream += "-1k"
						}
						info.UpstreamModelName = upstream
						require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
						reader, err := adaptor.BuildRequestBody(ctx, info)
						require.NoError(t, err)
						request, err := http.NewRequest(http.MethodPost, "https://selected.example", nil)
						require.NoError(t, err)
						require.NoError(t, adaptor.BuildRequestHeader(ctx, request, info))
						mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
						require.NoError(t, err)
						assert.Equal(t, "multipart/form-data", mediaType)
						require.NotEmpty(t, params["boundary"])
						assert.NotEqual(t, lastBoundary, params["boundary"])
						lastBoundary = params["boundary"]
						form, err := multipart.NewReader(reader, params["boundary"]).ReadForm(1 << 20)
						require.NoError(t, err)
						assert.Equal(t, map[string][]string{"model": {upstream}, "prompt": {"保留原物体并换背景"}, "size": {"2048x1152"}, "quality": {"high"}, "n": {"1"}, "output_format": {"png"}, "response_format": {"url"}}, form.Value)
						require.Len(t, form.File, 1)
						require.Len(t, form.File["image"], 1)
						file := form.File["image"][0]
						extension := format
						if format == "jpeg" {
							extension = "jpg"
						}
						assert.Equal(t, "source."+extension, file.Filename)
						assert.Equal(t, "image/"+format, file.Header.Get("Content-Type"))
						content, err := file.Open()
						require.NoError(t, err)
						data, err := io.ReadAll(content)
						require.NoError(t, err)
						require.NoError(t, content.Close())
						assert.Equal(t, original, data, "source bytes must never be resized or re-encoded")
						require.NoError(t, form.RemoveAll())
						url, err := adaptor.BuildRequestURL(info)
						require.NoError(t, err)
						assert.Equal(t, "https://selected.example/v1/images/edits", url)
					}
					// A later JSON supplier must not inherit a file boundary or the
					// prior supplier's file fields on the same canonical request.
					info.ChannelOtherSettings.ImageTaskContract.Protocol = relaykitdto.ImageTaskProtocolEvoLinkJSON
					reader, err := adaptor.BuildRequestBody(ctx, info)
					require.NoError(t, err)
					var got map[string]any
					require.NoError(t, common.DecodeJson(reader, &got))
					assert.Equal(t, "gpt-image-2.5-sunburst-1k", got["model"])
					assert.Equal(t, []any{source.URL}, got["image_urls"])
					request, err := http.NewRequest(http.MethodPost, "https://selected.example", nil)
					require.NoError(t, err)
					require.NoError(t, adaptor.BuildRequestHeader(ctx, request, info))
					assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
					assert.Equal(t, "application/json", ctx.Request.Header.Get("Content-Type"))
					var unchanged map[string]any
					require.NoError(t, common.UnmarshalBodyReusable(ctx, &unchanged))
					assert.Equal(t, body, unchanged)
				})
			}
		})
	}
}

func TestExplicitImageTaskMultipartMaskRemainsClosed(t *testing.T) {
	source, mask, _, _ := frozenImageTaskFixture(t)
	body := map[string]any{"model": "gpt-image-2.5-sunburst", "prompt": "change selected pixels", "image": source.URL, "mask_url": mask.URL}
	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	ctx := newJSONImageContext(t, "/v1/images/edits", string(encoded))
	ctx.Set("influencer_frozen_image", source)
	ctx.Set("influencer_frozen_mask", mask)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeMaskedEdit)
	info.UserId = 41
	failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
	require.NotNil(t, failure)
	assert.Equal(t, "invalid_image_task_contract", failure.Code)
	reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
	require.Error(t, err)
	assert.Nil(t, reader)
	// Retain the final refusal even if a caller bypasses declaration validation.
	reader, err = buildExplicitImageTaskBody(ctx, info, info.ChannelOtherSettings.ImageTaskContract)
	require.ErrorContains(t, err, "multipart mask contract is not enabled")
	assert.Nil(t, reader)
}

func TestExplicitImageTaskUnreviewedSourceFormatFailsBeforeSubmission(t *testing.T) {
	for _, format := range []string{"jpeg", "webp"} {
		t.Run(format, func(t *testing.T) {
			source, _, _, _ := frozenImageTaskFixture(t, format)
			body := map[string]any{"model": "gpt-image-2.5-sunburst", "prompt": "edit a kettle", "image": source.URL}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			for _, protocol := range []string{relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskProtocolEvoLinkJSON} {
				ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
				ctx.Set("influencer_frozen_image", source)
				info := explicitImageInfo(protocol, relaykitdto.ImageTaskModeReference)
				info.UserId = 41
				failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
				require.NotNil(t, failure)
				assert.Equal(t, http.StatusBadRequest, failure.StatusCode)
				assert.Contains(t, failure.Message, "source image format")
				_, retained := ctx.Get("influencer_image_inputs")
				assert.False(t, retained)
				reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
				require.ErrorContains(t, err, "source image format")
				assert.Nil(t, reader)
			}
		})
	}
}

func TestExplicitImageTaskDoesNotSilentlyDropUnsupportedFeatures(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"wrong aspect":            {"aspect_ratio": "9:16"},
		"conflicting aspect size": {"aspect_ratio": "16:9", "size": "1024x1024"},
		"mask alias":              {"mask": "https://8.8.4.4/mask.png"},
		"custom strength":         {"strength": float64(0.5)},
		"background":              {"background": "transparent"},
		"metadata feature":        {"metadata": map[string]any{"strength": float64(0.5)}},
		"explicit zero seed":      {"seed": float64(0)},
		"explicit seed":           {"seed": float64(123)},
		"metadata zero seed":      {"metadata": map[string]any{"seed": float64(0)}},
		"metadata seed":           {"metadata": map[string]any{"seed": float64(123)}},
	} {
		t.Run(name, func(t *testing.T) {
			body := map[string]any{"model": "gpt-image-2.5-sunburst", "prompt": "a kettle"}
			for key, value := range extra {
				body[key] = value
			}
			encoded, err := common.Marshal(body)
			require.NoError(t, err)
			ctx := newJSONImageContext(t, "/v1/images/generations", string(encoded))
			info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
			failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, failure)
			assert.Equal(t, http.StatusBadRequest, failure.StatusCode)
			assert.False(t, failure.NoTaskCreated)
			_, hasInput := ctx.Get("influencer_image_inputs")
			assert.False(t, hasInput)
		})
	}
	ctx := newJSONImageContext(t, "/v1/images/generations", `{"model":"gpt-image-2.5-sunburst","prompt":"a kettle","aspect_ratio":"16:9"}`)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
	require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info))
	reader, err := (&TaskAdaptor{}).BuildRequestBody(ctx, info)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.DecodeJson(reader, &got))
	assert.Equal(t, "2048x1152", got["size"])
	assert.NotContains(t, got, "aspect_ratio")
	seeded := newJSONImageContext(t, "/v1/images/edits", `{"model":"gpt-image-2.5-sunburst","prompt":"edit a kettle","image":"https://8.8.4.4/source.png","seed":123}`)
	seedInfo := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeReference)
	failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(seeded, seedInfo)
	require.NotNil(t, failure)
	assert.Contains(t, failure.Message, "seed is not supported")
}

func TestExplicitImageTaskPollCannotCompleteWithoutSingleHTTPSArtifact(t *testing.T) {
	task := &model.Task{Properties: model.Properties{OriginModelName: "gpt-image-2.5-sunburst"}}
	task.PrivateData.ImageInputs = &model.GenerationImageInputs{Version: 1, Mode: relaykitdto.ImageTaskModeText}
	for name, body := range map[string]string{
		"empty completed":            `{"id":"upstream-task","status":"completed"}`,
		"inline only":                `{"data":{"id":"upstream-task","status":"completed","result":{"b64_json":"iVBORw0KGgoAAAANS"}}}`,
		"URL masquerading as inline": `{"data":{"id":"upstream-task","status":"completed","result":{"b64_json":"https://media.example/not-a-url-field.png"}}}`,
		"data URI":                   `{"data":{"id":"upstream-task","status":"completed","result":{"url":"data:image/png;base64,aW1hZ2U="}}}`,
		"userinfo":                   `{"id":"upstream-task","status":"completed","results":["https://user:pass@media.example/image.png"]}`,
		"fragment":                   `{"id":"upstream-task","status":"completed","results":["https://media.example/image.png#fragment"]}`,
		"empty fragment":             `{"id":"upstream-task","status":"completed","results":["https://media.example/image.png#"]}`,
		"two images":                 `{"id":"upstream-task","status":"completed","results":["https://media.example/a.png","https://media.example/b.png"]}`,
		"two nested images":          `{"data":{"id":"upstream-task","status":"completed","result":{"images":[{"url":["https://media.example/a.png","https://media.example/b.png"]}]}}}`,
		"conflicting collections":    `{"id":"upstream-task","status":"completed","results":["https://media.example/a.png"],"images":["https://media.example/b.png"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult(task, nil, []byte(body))
			require.Error(t, err)
			assert.Nil(t, result, "invalid completed result remains a parse error, never an invented failure/refund")
		})
	}
	for name, body := range map[string]string{
		"flat":          `{"id":"upstream-task","status":"completed","results":["https://media.example/image.png?original=1"]}`,
		"nested":        `{"data":{"id":"upstream-task","status":"completed","result":{"images":[{"url":["https://media.example/image.png?original=1"]}]}}}`,
		"scalar result": `{"data":{"id":"upstream-task","status":"completed","result":{"url":"https://media.example/image.png?original=1"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := (&TaskAdaptor{}).ParseTaskResult(task, nil, []byte(body))
			require.NoError(t, err)
			assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
			assert.Equal(t, "https://media.example/image.png?original=1", result.Url)
		})
	}
}

func TestExplicitImageTaskPollDoesNotRewriteLegacySemantics(t *testing.T) {
	body := []byte(`{"id":"legacy-task","status":"completed"}`)
	for _, task := range []*model.Task{
		nil,
		{Properties: model.Properties{OriginModelName: "gpt-image-2.5-sunburst"}},
		{Properties: model.Properties{OriginModelName: "gpt-image-2.5-flare"}, PrivateData: model.TaskPrivateData{ImageInputs: &model.GenerationImageInputs{Version: 1}}},
	} {
		result, err := (&TaskAdaptor{}).ParseTaskResult(task, nil, body)
		require.NoError(t, err)
		assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
	}
	result, err := (&TaskAdaptor{}).ParseTaskResultBody(body)
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
}

func TestExplicitImageTaskPollAlsoProtectsAcceptedQueueBeforeHandoff(t *testing.T) {
	task := &model.Task{Properties: model.Properties{OriginModelName: "gpt-image-2.5-sunburst"}}
	task.PrivateData.GenerationQueue = &model.GenerationQueueRequest{ImageInputs: &model.GenerationImageInputs{Version: 1, Mode: relaykitdto.ImageTaskModeText}}
	result, err := (&TaskAdaptor{}).ParseTaskResult(task, nil, []byte(`{"id":"accepted-upstream-task","status":"completed"}`))
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, task.PrivateData.ImageInputs, "poll parsing must not invent or mutate a completed handoff")
}

func inlineExplicitImageFixture(t *testing.T) (*gin.Context, *relaycommon.RelayInfo, []byte) {
	t.Helper()
	originalSecret, originalAddress, originalPublic := common.CryptoSecret, system_setting.ServerAddress, system_setting.TaskPublicAddress
	previousDriver := storage.GetStorageDriver()
	common.CryptoSecret = "inline-adaptor-secret"
	system_setting.ServerAddress = "https://images.example"
	system_setting.TaskPublicAddress = ""
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
	t.Cleanup(func() {
		common.CryptoSecret = originalSecret
		system_setting.ServerAddress = originalAddress
		system_setting.TaskPublicAddress = originalPublic
		storage.SetGlobalDriver(previousDriver)
	})
	body := strings.Replace(explicitTextBody, "2048x1152", "1024x576", 1)
	c := newJSONImageContext(t, "/v1/images/generations", body)
	c.Set("id", 7)
	info := explicitImageInfo(relaykitdto.ImageTaskProtocolOpenAIMultipart, relaykitdto.ImageTaskModeText)
	info.UserId = 7
	info.ChannelId = 41
	info.PublicTaskID = "task_inline_adaptor"
	info.UpstreamModelName = "gpt-image-2.5-sunburst-1k"
	info.ChannelOtherSettings.ImageTaskContract.Sizes = []string{"1024x576"}
	info.ChannelOtherSettings.ImageTaskContract.Delivery = "b64_json"
	inputs := &model.GenerationImageInputs{Version: 1, Mode: relaykitdto.ImageTaskModeText}
	c.Set("influencer_image_inputs", inputs)
	c.Set("influencer_image_mode", inputs.Mode)
	c.Set("generation_queue_reserved_task", &model.Task{ID: 1, TaskID: info.PublicTaskID, UserId: 7, Action: "influencer_sheet", Group: "gpt-image-2.5-sunburst-image", Status: model.TaskStatusDispatching, Properties: model.Properties{OriginModelName: "gpt-image-2.5-sunburst"}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{ClaimOwner: "inline-scanner", SendStartedAt: 1, ImageInputs: inputs, Routing: &model.GenerationRoutingPolicy{MaxAttempts: 2, Attempts: []model.GenerationRouteAttempt{{ChannelID: 41, Group: "gpt-image-2.5-sunburst-image", Outcome: model.GenerationAttemptPending}}}}}})
	c.Set("generation_queue_scanner", &model.SystemTask{TaskID: "inline-scanner"})
	c.Set("generation_queue_runner", "inline-runner")
	c.Set("generation_queue_attempt_started", true)
	var original bytes.Buffer
	require.NoError(t, png.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 3, 2))))
	return c, info, original.Bytes()
}

func TestExplicitInlineImagePersistsOriginalAndRemovesSupplierPayload(t *testing.T) {
	c, info, original := inlineExplicitImageFixture(t)
	body, err := common.Marshal(map[string]any{"id": "untrusted-upstream-id", "task_id": "different-parent-id", "created": 123, "data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(original)}}})
	require.NoError(t, err)
	result, failure := (&TaskAdaptor{}).ParseResponse(c, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, info)
	require.Nil(t, failure)
	require.NotNil(t, result)
	require.NotNil(t, result.Immediate)
	assert.Equal(t, info.PublicTaskID, result.UpstreamTaskID)
	assert.Equal(t, string(model.TaskStatusSuccess), result.Immediate.Status)
	assert.Contains(t, result.Immediate.Url, "https://images.example/v1/tasks/"+info.PublicTaskID+"/artifacts/image/content")
	assert.NotContains(t, string(result.TaskData), "b64_json")
	assert.NotContains(t, string(result.TaskData), "untrusted-upstream-id")
	assert.NotContains(t, string(result.TaskData), "different-parent-id")
	var normalized map[string]any
	require.NoError(t, common.Unmarshal(result.TaskData, &normalized))
	data := normalized["data"].([]any)[0].(map[string]any)
	assert.Equal(t, float64(3), data["width"])
	assert.Equal(t, float64(2), data["height"])
	stored, _, err := service.ReadTaskArtifact(t.Context(), &model.Task{TaskID: info.PublicTaskID}, "image", service.MaxInlineImageTaskBytes)
	require.NoError(t, err)
	assert.Equal(t, original, stored)
	_, _, err = service.ReadTaskArtifact(t.Context(), &model.Task{TaskID: "untrusted-upstream-id"}, "image", service.MaxInlineImageTaskBytes)
	require.Error(t, err)
}

func TestExplicitInlineImageRebuildsSupplierDeliveryFromSameCanonicalInput(t *testing.T) {
	c, info, _ := inlineExplicitImageFixture(t)
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	for _, delivery := range []string{"b64_json", "url"} {
		info.ChannelOtherSettings.ImageTaskContract.Delivery = delivery
		reader, err := adaptor.BuildRequestBody(c, info)
		require.NoError(t, err)
		var sent map[string]any
		require.NoError(t, common.DecodeJson(reader, &sent))
		assert.Equal(t, delivery, sent["response_format"])
		assert.Equal(t, "1024x576", sent["size"])
		assert.Equal(t, "gpt-image-2.5-sunburst-1k", sent["model"])
	}
	var canonical map[string]any
	require.NoError(t, common.UnmarshalBodyReusable(c, &canonical))
	assert.Equal(t, "url", canonical["response_format"])
	assert.Equal(t, "gpt-image-2.5-sunburst", canonical["model"])
}

func TestExplicitInlineImageRejectsUntrustedIdentityAndSentFailures(t *testing.T) {
	for _, name := range []string{"no public id", "foreign public id", "foreign user", "parent id", "missing send fence", "wrong live attempt", "released", "bad image", "multiple images", "alternate URL"} {
		t.Run(name, func(t *testing.T) {
			c, info, original := inlineExplicitImageFixture(t)
			value, _ := c.Get("generation_queue_reserved_task")
			task := value.(*model.Task)
			encoded := base64.StdEncoding.EncodeToString(original)
			switch name {
			case "no public id":
				info.PublicTaskID = ""
			case "foreign public id":
				info.PublicTaskID = "task_other"
			case "foreign user":
				info.UserId = 8
			case "parent id":
				info.OriginTaskID = info.PublicTaskID
			case "missing send fence":
				task.PrivateData.GenerationQueue.SendStartedAt = 0
			case "wrong live attempt":
				task.PrivateData.GenerationQueue.Routing.Attempts[0].ChannelID = 42
			case "released":
				task.PrivateData.GenerationQueue.ReleaseStartedAt = 1
			case "bad image":
				encoded = base64.StdEncoding.EncodeToString([]byte("invalid PNG"))
			}
			items := []map[string]any{{"b64_json": encoded}}
			if name == "multiple images" {
				items = append(items, items[0])
			}
			if name == "alternate URL" {
				items[0]["url"] = "https://untrusted.example/one.png"
			}
			body, err := common.Marshal(map[string]any{"data": items})
			require.NoError(t, err)
			result, failure := (&TaskAdaptor{}).ParseResponse(c, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, info)
			require.NotNil(t, failure)
			assert.Nil(t, result)
			assert.True(t, failure.NoRetry)
			assert.False(t, failure.NoTaskCreated)
			stored, err := service.GetTaskArtifactStore().Resolve(task, "image")
			require.NoError(t, err)
			assert.Nil(t, stored)
		})
	}
}

type oversizedInlineResponseReader struct {
	remaining int64
	reads     int64
}

func (r *oversizedInlineResponseReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	for i := range p {
		p[i] = 'A'
	}
	r.remaining -= int64(len(p))
	r.reads += int64(len(p))
	return len(p), nil
}
func TestExplicitInlineImageBoundsResponseBeforeParsing(t *testing.T) {
	c, info, _ := inlineExplicitImageFixture(t)
	reader := &oversizedInlineResponseReader{remaining: service.MaxInlineImageTaskResponseBytes + 100}
	result, failure := (&TaskAdaptor{}).ParseResponse(c, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(reader)}, info)
	require.NotNil(t, failure)
	assert.Nil(t, result)
	assert.True(t, failure.NoRetry)
	assert.False(t, failure.NoTaskCreated)
	assert.Equal(t, service.MaxInlineImageTaskResponseBytes+1, reader.reads)
}

func TestExplicitInlineImageRequiresHTTPSDestinationBeforeSupplierBuild(t *testing.T) {
	c, info, _ := inlineExplicitImageFixture(t)
	system_setting.ServerAddress = "http://images.example"
	failure := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.NotNil(t, failure)
	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.Error(t, err)
	assert.Nil(t, reader)
}

type inlineAdaptorFaultDriver struct {
	storage.StorageDriver
	corrupt   bool
	failWrite bool
	cancel    context.CancelFunc
	writes    int
}

func (d *inlineAdaptorFaultDriver) Upload(ctx context.Context, key string, data []byte, mime string) (string, error) {
	d.writes++
	if d.failWrite {
		return "", fmt.Errorf("injected storage failure")
	}
	result, err := d.StorageDriver.Upload(ctx, key, data, mime)
	if d.cancel != nil {
		d.cancel()
	}
	return result, err
}
func (d *inlineAdaptorFaultDriver) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	reader, mime, size, err := d.StorageDriver.Get(ctx, key)
	if err != nil || !d.corrupt {
		return reader, mime, size, err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", 0, err
	}
	data[len(data)/2] ^= 1
	return io.NopCloser(bytes.NewReader(data)), mime, size, nil
}
func TestExplicitInlineImageStorageFailuresRemainUnknownAndDoNotChangeFunding(t *testing.T) {
	for _, name := range []string{"write failure", "readback corruption", "canceled after save"} {
		t.Run(name, func(t *testing.T) {
			c, info, original := inlineExplicitImageFixture(t)
			ctx, cancel := context.WithCancel(c.Request.Context())
			defer cancel()
			c.Request = c.Request.WithContext(ctx)
			driver := &inlineAdaptorFaultDriver{StorageDriver: storage.NewLocalStorageDriver(t.TempDir()), failWrite: name == "write failure", corrupt: name == "readback corruption"}
			if name == "canceled after save" {
				driver.cancel = cancel
			}
			storage.SetGlobalDriver(driver)
			info.PriceData.ModelPrice = 0.075
			info.PriceData.Quota = 37500
			quote := info.PriceData
			body, err := common.Marshal(map[string]any{"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(original)}}})
			require.NoError(t, err)
			result, failure := (&TaskAdaptor{}).ParseResponse(c, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, info)
			require.NotNil(t, failure)
			assert.Nil(t, result)
			assert.True(t, failure.NoRetry)
			assert.False(t, failure.NoTaskCreated)
			assert.Equal(t, quote, info.PriceData)
			assert.Equal(t, 1, driver.writes)
			value, _ := c.Get("generation_queue_reserved_task")
			task := value.(*model.Task)
			assert.Equal(t, model.TaskStatusDispatching, task.Status)
			assert.Nil(t, task.PrivateData.GenerationAcceptedResult)
			assert.Equal(t, model.GenerationAttemptPending, task.PrivateData.GenerationQueue.Routing.Attempts[0].Outcome)
		})
	}
}
