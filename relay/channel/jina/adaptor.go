package jina

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/common_handler"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode == constant.RelayModeReader {
		readerReq, ok := info.Request.(*dto.ReaderRequest)
		if !ok {
			return "", errors.New("invalid reader request")
		}
		return fmt.Sprintf("%s/%s", JinaReaderBaseURL, readerReq.Url), nil
	} else if info.RelayMode == constant.RelayModeRerank {
		return fmt.Sprintf("%s/v1/rerank", info.ChannelBaseUrl), nil
	} else if info.RelayMode == constant.RelayModeEmbeddings {
		return fmt.Sprintf("%s/v1/embeddings", info.ChannelBaseUrl), nil
	}
	return "", errors.New("invalid relay mode")
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if info.RelayMode == constant.RelayModeReader {
		return a.setupReaderRequestHeader(c, req, info)
	}
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	return nil
}

func (a *Adaptor) setupReaderRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	readerReq, ok := info.Request.(*dto.ReaderRequest)
	if !ok {
		return errors.New("invalid reader request")
	}

	switch readerReq.ReturnFormat {
	case "json":
		req.Set("Accept", "application/json")
	case "markdown", "":
		req.Set("Accept", "text/markdown")
	default:
		return fmt.Errorf("unsupported return_format: %s", readerReq.ReturnFormat)
	}

	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))

	if readerReq.RemoveSelector != "" {
		req.Set("X-Remove-Selector", readerReq.RemoveSelector)
	}
	if readerReq.TargetSelector != "" {
		req.Set("X-Target-Selector", readerReq.TargetSelector)
	}
	if readerReq.WaitForSelector != "" {
		req.Set("X-Wait-For-Selector", readerReq.WaitForSelector)
	}
	if readerReq.Timeout != "" {
		req.Set("X-Timeout", readerReq.Timeout)
	}
	if readerReq.RetainImages != "" {
		req.Set("X-Retain-Images", readerReq.RetainImages)
	}
	if readerReq.WithLinksSummary {
		req.Set("X-With-Links-Summary", "true")
	}
	if readerReq.WithImagesSummary {
		req.Set("X-With-Images-Summary", "true")
	}
	if readerReq.WithGeneratedAlt {
		req.Set("X-With-Generated-Alt", "true")
	}
	if readerReq.NoCache {
		req.Set("X-No-Cache", "true")
	}
	if readerReq.RespondWith != "" {
		req.Set("X-Respond-With", readerReq.RespondWith)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	request.EncodingFormat = ""
	return request, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeReader {
		return a.doReaderResponse(c, resp, info)
	} else if info.RelayMode == constant.RelayModeRerank {
		usage, err = common_handler.RerankHandler(c, info, resp)
	} else if info.RelayMode == constant.RelayModeEmbeddings {
		usage, err = openai.OpenaiHandler(c, info, resp)
	}
	return
}

// doReaderResponse 处理 r.jina.ai 的响应：原样透传响应，同时按输出 markdown
// 文本的 token 数统计 usage（与上游「按输出 token 计费」对齐）。
func (a *Adaptor) doReaderResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	content, upstreamTokens := extractReaderContent(resp, responseBody)
	// 上游按输出 token 计费；优先用 r.jina.ai 返回的精确 usage.tokens，
	// 缺失时才本地估算，保证扣费与上游成本一致。
	outputTokens := upstreamTokens
	if outputTokens <= 0 {
		outputTokens = service.CountTokenInput(content, info.OriginModelName)
	}

	c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(responseBody)

	return &dto.Usage{
		PromptTokens: outputTokens,
		TotalTokens:  outputTokens,
	}, nil
}

// extractReaderContent 从 reader 响应中取出计费依据：JSON 返回 data.content 和
// data.usage.tokens（上游精确输出 token），否则把整个响应体当 markdown 且 token 返回 0。
func extractReaderContent(resp *http.Response, body []byte) (string, int) {
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var jinaResp struct {
			Data struct {
				Content string `json:"content"`
				Usage   struct {
					Tokens int `json:"tokens"`
				} `json:"usage"`
			} `json:"data"`
		}
		if err := common.Unmarshal(body, &jinaResp); err == nil && jinaResp.Data.Content != "" {
			return jinaResp.Data.Content, jinaResp.Data.Usage.Tokens
		}
	}
	return string(body), 0
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
