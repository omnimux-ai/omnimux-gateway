package typesafe

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := ""
	if info != nil && info.ChannelMeta != nil {
		baseUrl = info.ChannelMeta.ChannelBaseUrl
	}
	if baseUrl == "" {
		baseUrl = "https://api.typesafe.ai"
	}
	return fmt.Sprintf("%s/v1/systemone", baseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if c != nil && c.Request != nil {
		channel.SetupApiRequestHeader(info, c, req)
	}
	apiKey := ""
	if info != nil && info.ChannelMeta != nil {
		apiKey = info.ApiKey
	}
	req.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Set("Content-Type", "application/json")
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("typesafe does not support openai chat completions, please use /v1/systemone")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	// 解析 TypeSafe System One 响应中的 token usage
	var typeSafeResp struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := common.Unmarshal(responseBody, &typeSafeResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	total := typeSafeResp.Usage.TotalTokens
	if total == 0 {
		total = typeSafeResp.Usage.InputTokens + typeSafeResp.Usage.OutputTokens
	}
	if typeSafeResp.Usage.InputTokens <= 0 || total <= 0 {
		return nil, types.NewOpenAIError(errors.New("missing or invalid usage in TypeSafe response"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// 原样透传给调用客户端
	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(responseBody)

	return &dto.Usage{
		PromptTokens:     typeSafeResp.Usage.InputTokens,
		CompletionTokens: typeSafeResp.Usage.OutputTokens,
		TotalTokens:      total,
	}, nil
}

func (a *Adaptor) GetModelList() []string {
	return []string{"jev"}
}

func (a *Adaptor) GetChannelName() string {
	return "typesafe"
}
