package sora

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJSONImageContext builds the request context the relay hands the adaptor for
// a JSON image submission on the given path.
func newJSONImageContext(t *testing.T, path, body string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

// newImageRelayInfo builds the relay state the adaptor reads: ChannelMeta and
// TaskRelayInfo are embedded pointers, so the literals must allocate them.
func newImageRelayInfo(model, action string) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{OriginModelName: model}
	info.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: model}
	info.TaskRelayInfo = &relaycommon.TaskRelayInfo{Action: action}
	return info
}

// A video submission reaches BuildRequestBody without a TaskRelayInfo, so the
// edit check must answer false rather than dereference the embedded pointer.
func TestIsImageEditRequestToleratesMissingTaskRelayInfo(t *testing.T) {
	assert.False(t, IsImageEditRequest(nil))
	assert.False(t, IsImageEditRequest(&relaycommon.RelayInfo{OriginModelName: "sora-2"}))

	info := &relaycommon.RelayInfo{OriginModelName: "sora-2"}
	info.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: "sora-2"}
	assert.False(t, IsImageEditRequest(info))

	assert.True(t, IsImageEditRequest(newImageRelayInfo("gpt-image-2.5", constant.TaskActionImageEdit)))
}

func TestIsImageEditRequestPath(t *testing.T) {
	cases := map[string]bool{
		"/v1/images/edits":       true,
		"/pg/images/edits":       true,
		"/v1/edits":              true,
		"/v1/images/generations": false,
		"/pg/images/generations": false,
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, want, isImageEditRequestPath(newJSONImageContext(t, path, `{}`)))
		})
	}
	assert.False(t, isImageEditRequestPath(nil))
}

// Every accepted spelling must survive as reference material, because the
// frontend and the public API do not agree on one field name.
func TestReferenceImageURLsReadsEveryAcceptedSpelling(t *testing.T) {
	refs := referenceImageURLs(relaycommon.TaskSubmitReq{
		Images:         []string{" https://a/1.png ", ""},
		Image:          "https://a/2.png",
		InputReference: "https://a/3.png",
	})
	assert.Equal(t, []string{"https://a/1.png", "https://a/2.png", "https://a/3.png"}, refs)

	assert.Empty(t, referenceImageURLs(relaycommon.TaskSubmitReq{Prompt: "a cat"}))
}

func TestReferenceURLsFromBodyReadsEveryAcceptedSpelling(t *testing.T) {
	body := map[string]any{
		"image_urls":      []any{"https://a/1.png"},
		"image":           "https://a/2.png",
		"input_reference": []any{"https://a/3.png"},
		"prompt":          "a cat",
	}
	assert.Equal(t, []string{"https://a/1.png", "https://a/2.png", "https://a/3.png"}, referenceURLsFromBody(body))
	assert.Empty(t, referenceURLsFromBody(map[string]any{"prompt": "a cat"}))
}

// The upstream must see one canonical field: two spellings of the same reference
// is an ambiguity it would have to resolve itself.
func TestNormalizeImageEditBodyCollapsesOntoImages(t *testing.T) {
	body := map[string]any{
		"model":           "gpt-image-2.5",
		"prompt":          "make it blue",
		"image_urls":      []any{"https://a/1.png"},
		"input_reference": "https://a/2.png",
		"size":            "1024x1024",
	}
	normalizeImageEditBody(body)

	assert.Equal(t, []string{"https://a/1.png", "https://a/2.png"}, body["images"])
	assert.NotContains(t, body, "image_urls")
	assert.NotContains(t, body, "input_reference")
	assert.NotContains(t, body, "image")
	assert.Equal(t, "make it blue", body["prompt"])
	assert.Equal(t, "1024x1024", body["size"])
}

func TestValidateGrokImageRequestMarksEditWhenReferencePresent(t *testing.T) {
	info := newImageRelayInfo("gpt-image-2.5", "")
	c := newJSONImageContext(t, "/pg/images/edits", `{"model":"gpt-image-2.5","prompt":"make it blue","images":["https://a/1.png"]}`)

	require.Nil(t, validateGrokImageRequest(c, info))
	assert.Equal(t, constant.TaskActionImageEdit, info.Action)
}

func TestValidateGrokImageRequestRejectsEditWithoutReference(t *testing.T) {
	info := newImageRelayInfo("gpt-image-2.5", "")
	c := newJSONImageContext(t, "/v1/images/edits", `{"model":"gpt-image-2.5","prompt":"make it blue"}`)

	taskErr := validateGrokImageRequest(c, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	// A rejected request never reaches the adaptor, so it is left unmarked.
	assert.Empty(t, info.Action)
}

// A reference sent to the generate path still reaches the upstream: the caller
// may not know the edit surface exists.
func TestValidateGrokImageRequestAcceptsReferenceOnGeneratePath(t *testing.T) {
	info := newImageRelayInfo("gpt-image-2.5", "")
	c := newJSONImageContext(t, "/v1/images/generations", `{"model":"gpt-image-2.5","prompt":"a cat","images":["https://a/1.png"]}`)

	require.Nil(t, validateGrokImageRequest(c, info))
	assert.Equal(t, constant.TaskActionGenerate, info.Action)
}

func TestBuildRequestURLSelectsEditSurfaceOnlyForEdits(t *testing.T) {
	a := &TaskAdaptor{baseURL: "https://api.example.com/"}
	image := newImageRelayInfo("gpt-image-2.5", "")

	url, err := a.BuildRequestURL(image)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1/images/generations", url)

	edit := newImageRelayInfo("gpt-image-2.5", constant.TaskActionImageEdit)
	url, err = a.BuildRequestURL(edit)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1/images/edits", url)
}

func TestBuildRequestBodyNormalizesEditAndLeavesGenerateUntouched(t *testing.T) {
	a := &TaskAdaptor{}

	generateBody := `{"model":"gpt-image-2.5","prompt":"a cat","image_urls":["https://a/1.png"],"size":"1024x1024"}`
	generateInfo := newImageRelayInfo("gpt-image-2.5", constant.TaskActionGenerate)
	reader, err := a.BuildRequestBody(newJSONImageContext(t, "/v1/images/generations", generateBody), generateInfo)
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	var generateDecoded map[string]any
	require.NoError(t, common.Unmarshal(got, &generateDecoded))
	assert.Equal(t, map[string]any{
		"model":      "gpt-image-2.5",
		"prompt":     "a cat",
		"image_urls": []any{"https://a/1.png"},
		"size":       "1024x1024",
	}, generateDecoded)

	editBody := `{"model":"gpt-image-2.5","prompt":"make it blue","image_urls":["https://a/1.png"],"size":"1024x1024"}`
	editInfo := newImageRelayInfo("gpt-image-2.5", constant.TaskActionImageEdit)
	reader, err = a.BuildRequestBody(newJSONImageContext(t, "/v1/images/edits", editBody), editInfo)
	require.NoError(t, err)
	got, err = io.ReadAll(reader)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, common.Unmarshal(got, &decoded))
	assert.Equal(t, []any{"https://a/1.png"}, decoded["images"])
	assert.NotContains(t, decoded, "image_urls")
	assert.Equal(t, "make it blue", decoded["prompt"])
	assert.Equal(t, "gpt-image-2.5", decoded["model"])
}
