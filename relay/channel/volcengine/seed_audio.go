package volcengine

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

const (
	contextKeySeedAudioRequest = "volcengine_seed_audio_request"
)

// isSeedAudioModel 判断是否为 seed-audio 系列模型
func isSeedAudioModel(modelName string) bool {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(lower, "seed-audio") ||
		strings.HasPrefix(lower, "doubao-seed-audio") ||
		lower == ModelSeedAudio10
}

// buildSeedAudioV3Request 将 OpenAI 格式音频请求转换为火山引擎 V3 格式
func buildSeedAudioV3Request(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (*SeedAudioV3Request, error) {
	req := &SeedAudioV3Request{
		Model:      ModelSeedAudio10,
		TextPrompt: request.Input,
		AudioConfig: SeedAudioConfig{
			Format:     mapEncoding(request.ResponseFormat),
			SampleRate: 24000,
		},
	}

	if info != nil && info.ChannelMeta != nil && info.UpstreamModelName != "" {
		req.Model = info.UpstreamModelName
	} else if request.Model != "" {
		req.Model = request.Model
	}

	// 1. 如果有 Metadata，尝试先从中解析完整 SeedAudioV3Request
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, req); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to seed_audio request: %w", err)
		}
	}

	// 2. 检查顶层扩展字段 References
	if len(request.References) > 0 {
		var refs []SeedAudioReference
		if err := common.Unmarshal(request.References, &refs); err == nil && len(refs) > 0 {
			req.References = refs
		}
	}

	// 3. 检查顶层扩展字段 AudioConfig
	if len(request.AudioConfig) > 0 {
		var config SeedAudioConfig
		if err := common.Unmarshal(request.AudioConfig, &config); err == nil {
			if config.Format != "" {
				req.AudioConfig.Format = config.Format
			}
			if config.SampleRate > 0 {
				req.AudioConfig.SampleRate = config.SampleRate
			}
			if config.SpeechRate != nil {
				req.AudioConfig.SpeechRate = config.SpeechRate
			}
			if config.LoudnessRate != nil {
				req.AudioConfig.LoudnessRate = config.LoudnessRate
			}
			if config.PitchRate != nil {
				req.AudioConfig.PitchRate = config.PitchRate
			}
			if config.EnableSubtitle {
				req.AudioConfig.EnableSubtitle = true
			}
		}
	}

	// 4. 检查顶层扩展字段 Watermark
	if len(request.Watermark) > 0 {
		var watermark SeedAudioWatermark
		if err := common.Unmarshal(request.Watermark, &watermark); err == nil {
			req.Watermark = &watermark
		}
	}

	// 5. 如果没有指定 References 且传了 Voice，映射到 References[0].Speaker
	if len(req.References) == 0 && request.Voice != "" {
		req.References = []SeedAudioReference{
			{
				Speaker: mapVoiceType(request.Voice),
			},
		}
	}

	// 6. 如果用户传了 Speed 且 req.AudioConfig.SpeechRate 未被单独指定
	if request.Speed != nil && req.AudioConfig.SpeechRate == nil {
		req.AudioConfig.SpeechRate = request.Speed
	}

	// 保底确保 Format 与 SampleRate 规范
	if req.AudioConfig.Format == "" {
		req.AudioConfig.Format = "mp3"
	}
	if req.AudioConfig.SampleRate == 0 {
		req.AudioConfig.SampleRate = 24000
	}

	return req, nil
}
