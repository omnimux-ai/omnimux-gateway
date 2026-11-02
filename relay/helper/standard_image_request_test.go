package helper

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/QuantumNous/new-api/tokenkit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func standardImageTestContext(t *testing.T, body map[string]any) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(encoded))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, &relaycommon.RelayInfo{OriginModelName: "gpt-image-2.5", UsingGroup: "gpt-image-2.5-economy", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2.5-flare"}}
}

func TestStandardImageRequestSupportsSevenReviewedSizesWithoutChangingInput(t *testing.T) {
	for ratio, pixels := range map[string]string{"1:1": "1024x1024", "2:3": "1024x1536", "3:2": "1536x1024", "3:4": "1024x1360", "4:3": "1360x1024", "9:16": "1088x1920", "16:9": "1920x1088"} {
		for _, size := range []string{ratio, pixels} {
			t.Run(size, func(t *testing.T) {
				body := map[string]any{"model": "gpt-image-2.5", "prompt": "a person", "size": size, "n": 1, "quality": "medium", "metadata": map[string]any{"seed": 23}}
				c, info := standardImageTestContext(t, body)
				require.NoError(t, ValidateGPTImageStandardRequest(c, info))
				actual, ok := GPTImageStandardPixels(size)
				assert.True(t, ok)
				assert.Equal(t, pixels, actual)
				var unchanged map[string]any
				require.NoError(t, common.UnmarshalBodyReusable(c, &unchanged))
				assert.Equal(t, size, unchanged["size"])
				assert.Equal(t, "medium", unchanged["quality"])
			})
		}
	}
	c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": "a person"})
	require.NoError(t, ValidateGPTImageStandardRequest(c, info))
}

func TestStandardImageRequestRejectsUnreviewedOrAmbiguousParameters(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"multiple output": {"n": 2}, "zero output": {"n": 0}, "negative output": {"n": -1}, "string output": {"n": "1"}, "fractional output": {"n": 1.5},
		"low": {"quality": "low"}, "high": {"quality": "high"}, "xhigh": {"quality": "xhigh"}, "max": {"quality": "max"}, "auto": {"quality": "auto"}, "legacy standard": {"quality": "standard"}, "null quality": {"quality": nil},
		"old 3:4 pixels": {"size": "1024x1365"}, "old 4:3 pixels": {"size": "1365x1024"}, "4K": {"size": "3840x2160"}, "arbitrary size": {"size": "1360x768"}, "null size": {"size": nil}, "aspect conflict": {"size": "16:9", "aspect_ratio": "1:1"},
		"resolution": {"resolution": "1K"}, "mask": {"mask_url": "https://8.8.8.8/mask.png"}, "transparent": {"background": "transparent"}, "compression": {"output_compression": 90}, "bad output format": {"output_format": "gif"}, "unreviewed JPEG output": {"output_format": "jpeg"}, "unreviewed WebP output": {"output_format": "webp"},
		"metadata count": {"metadata": map[string]any{"n": 2}}, "raw metadata": {"metadata": `{"quality":"high"}`}, "parameters": {"parameters": map[string]any{"n": 2}},
		"two aliases": {"image": "https://8.8.8.8/a.png", "image_urls": []string{"https://8.8.8.8/b.png"}}, "duplicate aliases": {"image": "https://8.8.8.8/a.png", "input_reference": "https://8.8.8.8/a.png"}, "two images": {"images": []string{"https://8.8.8.8/a.png", "https://8.8.8.8/b.png"}},
		"reference object with two URLs": {"image": map[string]any{"url": "https://8.8.8.8/a.png", "image_url": "https://8.8.8.8/b.png"}}, "reference false": {"image": false}, "null reference": {"image": nil}, "empty reference": {"image": " "}, "data reference": {"image": "data:image/png;base64,abc"}, "http reference": {"image": "http://8.8.8.8/a.png"},
	} {
		t.Run(name, func(t *testing.T) {
			body := map[string]any{"model": "gpt-image-2.5", "prompt": "a person", "size": "1024x1024"}
			for key, value := range extra {
				body[key] = value
			}
			c, info := standardImageTestContext(t, body)
			assert.Error(t, ValidateGPTImageStandardRequest(c, info))
		})
	}
}

func TestStandardImagePromptKeepsReviewedUnicodeAndUTF8Limits(t *testing.T) {
	for _, tc := range []struct {
		name, prompt string
		allowed      bool
	}{
		{"8192 ASCII", strings.Repeat("a", 8192), true}, {"8193 ASCII", strings.Repeat("a", 8193), false},
		{"both exact boundaries", strings.Repeat("字", 4096) + strings.Repeat("a", 4096), true},
		{"16384 UTF8 bytes above token limit", strings.Repeat("字", 5461) + "a", false}, {"16385 UTF8 bytes", strings.Repeat("字", 5461) + "aa", false},
		{"four byte code points at byte limit below token limit", strings.Repeat("😀", 4096), true}, {"four byte code points over byte limit", strings.Repeat("😀", 4096) + "a", false},
		{"leading and trailing whitespace counts", " " + strings.Repeat("a", 8191) + " ", false},
		{"historical public character limit", strings.Repeat("a", 32000), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": tc.prompt})
			err := ValidateGPTImageStandardRequest(c, info)
			if tc.allowed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestStandardImagePromptPreservesTheReviewedR2InputInFull(t *testing.T) {
	selection := influencer.Selection{
		"accessory": {"acc_headphones", "acc_glasses", "acc_jewelry"}, "aesthetic": {"theatrical"}, "age": {"mature"},
		"body_type": {"body_ultra"}, "distinctive": {"df_hetero", "df_ears"}, "ethnicity_origin_base": {"east_asian"},
		"eye_color": {"eye_ice_blue"}, "eye_shape": {"es_downturned"}, "facial_hair": {"fh_pencil"},
		"freak_face": {"fn_cheekbones", "fn_eyebags", "fn_thickbrows", "fn_mole"}, "freak_head": {"head_heart"}, "freak_neck": {"neck_normal"},
		"gender": {"non_binary"}, "hair": {"hair_short"}, "hair_colour": {"hc_chocolate"}, "height": {"h_average"},
		"proportions": {"pr_shoulders", "pr_longlimbs"}, "skin_tone": {"st_carbon"},
	}
	brief := []rune(strings.Repeat("角色穿着深蓝色棉质外套、浅灰色长裤与简洁运动鞋，衣服版型舒适自然，面料细节清楚，配色克制，配饰少量且彼此协调。保留成年角色平静自信的神态，头发整洁但有自然纹理，整体造型适合日常生活与简洁的人物展示。", 50))[:4000]
	prompt := influencer.BuildReferencePrompt("normal", selection, string(brief))
	require.Equal(t, 8053, len([]rune(prompt)))
	require.Equal(t, 16053, len(prompt))
	require.Equal(t, "0e31fbbe836e3f34ebe6f039932741702b49b3fbc1cedde57732a97ba1007bec", fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))))
	c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": prompt})
	require.NoError(t, ValidateGPTImageStandardRequest(c, info))
	var unchanged map[string]any
	require.NoError(t, common.UnmarshalBodyReusable(c, &unchanged))
	assert.Equal(t, prompt, unchanged["prompt"])
	// Four-byte user input can exceed the reviewed byte limit while the same
	// legacy 4000-code-point brief count remains valid; do not trim it to fit.
	prompt = influencer.BuildReferencePrompt("normal", selection, strings.Repeat("😀", 4000))
	require.Equal(t, 8053, len([]rune(prompt)))
	require.Equal(t, 20053, len(prompt))
	c, info = standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": prompt})
	require.Error(t, ValidateGPTImageStandardRequest(c, info))
}

func TestStandardImagePromptRejectsUnreviewedTokenCountsWithoutTruncation(t *testing.T) {
	for _, tc := range []struct {
		name, prompt string
		tokens       int
		allowed      bool
	}{
		{"5000 tokens", strings.Repeat("字", 5000), 5000, true},
		{"5001 tokens", strings.Repeat("字", 5001), 5001, false},
		{"high token count within character and byte limits", "Create one male character on a white background. Ignore the following decorative marks: " + strings.Repeat("\u07ff", 8100), 16216, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.tokens, tokenkit.Count("gpt-image-2.5", tc.prompt))
			c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": tc.prompt})
			err := ValidateGPTImageStandardRequest(c, info)
			if tc.allowed {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "5000 prompt tokens")
			}
			var unchanged map[string]any
			require.NoError(t, common.UnmarshalBodyReusable(c, &unchanged))
			assert.Equal(t, tc.prompt, unchanged["prompt"])
		})
	}
}

func TestStandardImageRouteUsesConcreteNativeGroupAndOriginalPublicID(t *testing.T) {
	c, info := standardImageTestContext(t, map[string]any{"prompt": "a person"})
	assert.True(t, IsGPTImageStandardRoute(c, info))
	info.OriginModelName = "gpt-image-2.5-flare"
	assert.False(t, IsGPTImageStandardRoute(c, info))
	info.OriginModelName, info.UsingGroup = "gpt-image-2.5", "other"
	assert.False(t, IsGPTImageStandardRoute(c, info))
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "gpt-image-2.5-economy")
	assert.True(t, IsGPTImageStandardRoute(c, info))
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "other")
	info.UsingGroup = "gpt-image-2.5-economy"
	assert.False(t, IsGPTImageStandardRoute(c, info), "the speculative or previous group cannot override the current selected group")
	info.OriginModelName = "rewritten-by-plugin"
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2.5")
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "gpt-image-2.5-economy")
	assert.True(t, IsGPTImageStandardRoute(c, info), "a plugin cannot change the original public identity of this standard request")
}

func TestStandardImageRequestRejectsChannelOverridesAndAllowsReviewedPNGOutput(t *testing.T) {
	c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": "a person", "output_format": "png"})
	require.NoError(t, ValidateGPTImageStandardRequest(c, info))
	for _, override := range []string{"parameters", "model by parameter"} {
		t.Run(override, func(t *testing.T) {
			c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": "a person"})
			if override == "parameters" {
				info.ParamOverride = map[string]any{"quality": "high"}
			} else {
				info.ChannelOtherSettings.AdvancedCustom = &dto.AdvancedCustomConfig{ModelByParam: map[string]dto.ParamModelRouting{"gpt-image-2.5": {Param: "size"}}}
			}
			require.ErrorContains(t, ValidateGPTImageStandardRequest(c, info), "parameter overrides")
		})
	}
}

type standardImageReferenceTransport struct {
	data  []byte
	calls *int
}

func (tr standardImageReferenceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*tr.calls++
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(tr.data)), Request: r}, nil
}

func TestStandardImageReferenceReusesExistingProtectedPreflight(t *testing.T) {
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	oldClient := standardImageReferenceHTTPClient
	t.Cleanup(func() { standardImageReferenceHTTPClient = oldClient })
	for _, preflight := range []string{"", "https://8.8.8.8/ref.png", "https://8.8.8.8/other.png"} {
		calls := 0
		standardImageReferenceHTTPClient = &http.Client{Transport: standardImageReferenceTransport{data.Bytes(), &calls}}
		c, info := standardImageTestContext(t, map[string]any{"model": "gpt-image-2.5", "prompt": "a person", "image_urls": []any{map[string]any{"image_url": "https://8.8.8.8/ref.png"}}})
		if preflight != "" {
			c.Set("influencer_reference_preflight", preflight)
		}
		require.NoError(t, ValidateGPTImageStandardRequest(c, info))
		require.NoError(t, ValidateGPTImageStandardRequest(c, info))
		if preflight == "https://8.8.8.8/ref.png" {
			assert.Zero(t, calls)
		} else {
			assert.Equal(t, 1, calls)
		}
	}
}

func TestStandardImageGroupRejectsMultipartBeforeParsingOrFetching(t *testing.T) {
	c, info := standardImageTestContext(t, map[string]any{"prompt": "a person"})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader("untrusted multipart body"))
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=local")
	require.ErrorContains(t, ValidateGPTImageStandardRequest(c, info), "application/json")
}

func TestGPTImagePromptSharedEnvelopeKeepsOldAndSunburstBoundaries(t *testing.T) {
	for _, modelName := range []string{"gpt-image-2.5", "gpt-image-2.5-sunburst"} {
		t.Run(modelName, func(t *testing.T) {
			require.NoError(t, ValidateGPTImagePrompt(modelName, strings.Repeat("字", 5000)))
			require.ErrorContains(t, ValidateGPTImagePrompt(modelName, strings.Repeat("字", 5001)), "5000 prompt tokens")
			require.ErrorContains(t, ValidateGPTImagePrompt(modelName, strings.Repeat("a", 8193)), "8192 Unicode")
			require.ErrorContains(t, ValidateGPTImagePrompt(modelName, strings.Repeat("😀", 4097)), "16384 UTF-8")
			require.Error(t, ValidateGPTImagePrompt(modelName, string([]byte{0xff})))
		})
	}
}
