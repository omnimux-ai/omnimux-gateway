package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func modelByParamConfig() *dto.AdvancedCustomConfig {
	return &dto.AdvancedCustomConfig{
		ModelByParam: map[string]dto.ParamModelRouting{
			"gpt-image-2.5": {
				Param: "size",
				Rules: []dto.ParamModelRule{
					{AnyOf: []string{"2048x2048", "3840x2160"}, Model: "image2.5-pro"},
				},
			},
		},
	}
}

// 渠道配置了 model_by_param 时，命中的请求按规则改写上游名与请求体里的模型名；
// 计费口径（OriginModelName）保持不变。
func TestModelMappedHelperAppliesModelByParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName:    "gpt-image-2.5",
			ChannelOtherSettings: dto.ChannelOtherSettings{AdvancedCustom: modelByParamConfig()},
		},
	}
	request := &dto.ImageRequest{Model: "gpt-image-2.5", Prompt: "a cat", Size: "2048x2048"}

	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.Equal(t, "image2.5-pro", info.UpstreamModelName, "命中的请求应改写到规则里的上游模型")
	assert.Equal(t, "image2.5-pro", request.Model, "请求体里的模型名也必须改写，否则上游收到的还是旧名字")
	assert.Equal(t, "gpt-image-2.5", info.OriginModelName, "计费口径必须保持公开 ID 不变")
}

func TestModelMappedHelperModelByParamInactive(t *testing.T) {
	newInfo := func(cfg *dto.AdvancedCustomConfig, originModel string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			OriginModelName: originModel,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName:    "gpt-image-2.5",
				ChannelOtherSettings: dto.ChannelOtherSettings{AdvancedCustom: cfg},
			},
		}
	}
	cases := []struct {
		name    string
		origin  string
		cfg     *dto.AdvancedCustomConfig
		request dto.Request
	}{
		{"未配置渠道 → 不改写", "gpt-image-2.5", nil, &dto.ImageRequest{Model: "gpt-image-2.5", Prompt: "a cat", Size: "2048x2048"}},
		{"尺寸未命中规则 → 不改写", "gpt-image-2.5", modelByParamConfig(), &dto.ImageRequest{Model: "gpt-image-2.5", Prompt: "a cat", Size: "1024x1024"}},
		{"模型不在配置里 → 不改写", "nano-banana-pro", modelByParamConfig(), &dto.ImageRequest{Model: "nano-banana-pro", Prompt: "a cat", Size: "2048x2048"}},
		{"非图片请求 → 不改写", "gpt-image-2.5", modelByParamConfig(), &dto.GeneralOpenAIRequest{Model: "gpt-image-2.5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := newInfo(tc.cfg, tc.origin)
			info.ChannelMeta.UpstreamModelName = tc.origin
			require.NoError(t, ModelMappedHelper(c, info, tc.request))
			assert.Equal(t, tc.origin, info.UpstreamModelName, "未命中时上游名必须保持原样")
		})
	}
}

// 渠道同时配了 model_mapping 与 model_by_param 时，参数规则优先（更具体）。
func TestModelMappedHelperModelByParamWinsOverMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("model_mapping", `{"gpt-image-2.5":"image2.5-base"}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName:    "gpt-image-2.5",
			ChannelOtherSettings: dto.ChannelOtherSettings{AdvancedCustom: modelByParamConfig()},
		},
	}
	request := &dto.ImageRequest{Model: "gpt-image-2.5", Prompt: "a cat", Size: "3840x2160"}

	require.NoError(t, ModelMappedHelper(c, info, request))
	assert.Equal(t, "image2.5-pro", info.UpstreamModelName)
	assert.Equal(t, "image2.5-pro", request.Model)
}

// 任务链路走 lookup 版本（拿不到请求体对象），行为必须与同步链路一致。
func TestApplyModelByParamLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName:    "gpt-image-2.5",
			ChannelOtherSettings: dto.ChannelOtherSettings{AdvancedCustom: modelByParamConfig()},
		},
	}
	sizes := map[string]string{"size": "2048x2048"}
	upstream, changed := ApplyModelByParamLookup(info, func(p string) string { return sizes[p] })
	assert.True(t, changed)
	assert.Equal(t, "image2.5-pro", upstream)
	assert.Equal(t, "image2.5-pro", info.UpstreamModelName)
	assert.Equal(t, "gpt-image-2.5", info.OriginModelName, "计费口径不变")

	// 未命中：不改写
	other := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName:    "gpt-image-2.5",
			ChannelOtherSettings: dto.ChannelOtherSettings{AdvancedCustom: modelByParamConfig()},
		},
	}
	if _, changed := ApplyModelByParamLookup(other, func(string) string { return "1024x1024" }); changed {
		t.Fatal("未命中时不得改写")
	}
	// 未配置渠道：完全不生效
	plain := &relaycommon.RelayInfo{OriginModelName: "gpt-image-2.5", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-image-2.5"}}
	if _, changed := ApplyModelByParamLookup(plain, func(string) string { return "2048x2048" }); changed {
		t.Fatal("未配置渠道不得改写")
	}
}
