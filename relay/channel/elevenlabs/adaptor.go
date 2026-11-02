package elevenlabs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	voiceID      string
	outputFormat string
	textInput    string
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return GetRequestURL(info, a.voiceID, a.outputFormat)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	apiKey := ""
	if info != nil && info.ChannelMeta != nil {
		apiKey = info.ApiKey
	}
	req.Set("xi-api-key", apiKey)
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "audio/mpeg")
	return nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("elevenlabs: unsupported audio relay mode")
	}

	voiceID := request.Voice
	if voiceID == "" {
		voiceID = DefaultVoice
	}
	a.voiceID = voiceID
	a.textInput = request.Input

	if request.ResponseFormat != "" {
		a.outputFormat = request.ResponseFormat
	}

	modelName := info.OriginModelName
	if modelName == "" {
		modelName = "eleven_multilingual_v2"
	}

	ttsReq := TTSRequest{
		Text:    request.Input,
		ModelID: modelName,
		VoiceSettings: &VoiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.75,
		},
	}

	if len(request.Metadata) > 0 {
		_ = common.Unmarshal(request.Metadata, &ttsReq)
	}

	jsonData, err := common.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: error marshalling request: %w", err)
	}

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("elevenlabs: image generation not supported")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("elevenlabs: claude request not supported")
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("elevenlabs: openai chat not supported")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("elevenlabs: gemini chat not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("elevenlabs: rerank not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("elevenlabs: embedding not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("elevenlabs: openai responses not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeAudioSpeech {
		return handleTTSResponse(c, resp, info, a.textInput)
	}
	return nil, types.NewError(errors.New("elevenlabs: unsupported relay mode"), types.ErrorCodeInvalidRequest)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
