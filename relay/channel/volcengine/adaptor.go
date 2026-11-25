package volcengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Adaptor struct {
	ttsVoiceID       string
	ttsFormat        string
	ttsBillableChars int
	ttsRequest       dto.AudioRequest
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	if _, ok := channelconstant.ChannelSpecialBases[info.ChannelBaseUrl]; ok {
		adaptor := claude.Adaptor{}
		return adaptor.ConvertClaudeRequest(c, info, req)
	}
	adaptor := openai.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func getUpstreamModelName(info *relaycommon.RelayInfo) string {
	if info == nil || info.ChannelMeta == nil {
		return ""
	}
	return info.UpstreamModelName
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode == constant.RelayModeAudioTranscription {
		c.Set(contextKeyASRRequest, request)
		submitReq, err := buildASRSubmitRequest(c, info, request)
		if err != nil {
			return nil, err
		}
		jsonData, err := common.Marshal(submitReq)
		if err != nil {
			return nil, fmt.Errorf("error marshalling asr submit request: %w", err)
		}
		return bytes.NewReader(jsonData), nil
	}

	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	if !dto.IsVolcengineTTS3Model(getUpstreamModelName(info)) &&
		!dto.IsVolcengineTTS3Model(request.Model) &&
		!dto.IsVolcengineTTS3Model(info.OriginModelName) {
		return nil, fmt.Errorf("unsupported volcengine speech model %q: only %s is supported", request.Model, ModelSeedTTS30)
	}

	tts3Req, voiceID, outputFormat, billableChars, err := buildTTS3Request(info, request)
	if err != nil {
		return nil, err
	}

	a.ttsVoiceID = voiceID
	a.ttsFormat = outputFormat
	a.ttsBillableChars = billableChars
	a.ttsRequest = request
	info.IsStream = false

	jsonData, err := common.Marshal(tts3Req)
	if err != nil {
		return nil, fmt.Errorf("error marshalling volcengine tts 3.0 request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	switch info.RelayMode {
	case constant.RelayModeImagesGenerations:
		return request, nil
	// 根据官方文档,并没有发现豆包生图支持表单请求:https://www.volcengine.com/docs/82379/1824121
	//case constant.RelayModeImagesEdits:
	//
	//	var requestBody bytes.Buffer
	//	writer := multipart.NewWriter(&requestBody)
	//
	//	writer.WriteField("model", request.Model)
	//
	//	formData := c.Request.PostForm
	//	for key, values := range formData {
	//		if key == "model" {
	//			continue
	//		}
	//		for _, value := range values {
	//			writer.WriteField(key, value)
	//		}
	//	}
	//
	//	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
	//		return nil, errors.New("failed to parse multipart form")
	//	}
	//
	//	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
	//		var imageFiles []*multipart.FileHeader
	//		var exists bool
	//
	//		if imageFiles, exists = c.Request.MultipartForm.File["image"]; !exists || len(imageFiles) == 0 {
	//			if imageFiles, exists = c.Request.MultipartForm.File["image[]"]; !exists || len(imageFiles) == 0 {
	//				foundArrayImages := false
	//				for fieldName, files := range c.Request.MultipartForm.File {
	//					if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
	//						foundArrayImages = true
	//						for _, file := range files {
	//							imageFiles = append(imageFiles, file)
	//						}
	//					}
	//				}
	//
	//				if !foundArrayImages && (len(imageFiles) == 0) {
	//					return nil, errors.New("image is required")
	//				}
	//			}
	//		}
	//
	//		for i, fileHeader := range imageFiles {
	//			file, err := fileHeader.Open()
	//			if err != nil {
	//				return nil, fmt.Errorf("failed to open image file %d: %w", i, err)
	//			}
	//			defer file.Close()
	//
	//			fieldName := "image"
	//			if len(imageFiles) > 1 {
	//				fieldName = "image[]"
	//			}
	//
	//			mimeType := detectImageMimeType(fileHeader.Filename)
	//
	//			h := make(textproto.MIMEHeader)
	//			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileHeader.Filename))
	//			h.Set("Content-Type", mimeType)
	//
	//			part, err := writer.CreatePart(h)
	//			if err != nil {
	//				return nil, fmt.Errorf("create form part failed for image %d: %w", i, err)
	//			}
	//
	//			if _, err := io.Copy(part, file); err != nil {
	//				return nil, fmt.Errorf("copy file failed for image %d: %w", i, err)
	//			}
	//		}
	//
	//		if maskFiles, exists := c.Request.MultipartForm.File["mask"]; exists && len(maskFiles) > 0 {
	//			maskFile, err := maskFiles[0].Open()
	//			if err != nil {
	//				return nil, errors.New("failed to open mask file")
	//			}
	//			defer maskFile.Close()
	//
	//			mimeType := detectImageMimeType(maskFiles[0].Filename)
	//
	//			h := make(textproto.MIMEHeader)
	//			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="mask"; filename="%s"`, maskFiles[0].Filename))
	//			h.Set("Content-Type", mimeType)
	//
	//			maskPart, err := writer.CreatePart(h)
	//			if err != nil {
	//				return nil, errors.New("create form file failed for mask")
	//			}
	//
	//			if _, err := io.Copy(maskPart, maskFile); err != nil {
	//				return nil, errors.New("copy mask file failed")
	//			}
	//		}
	//	} else {
	//		return nil, errors.New("no multipart form data found")
	//	}
	//
	//	writer.Close()
	//	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	//	return bytes.NewReader(requestBody.Bytes()), nil

	default:
		return request, nil
	}
}

func detectImageMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		if strings.HasPrefix(ext, ".jp") {
			return "image/jpeg"
		}
		return "image/png"
	}
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := info.ChannelBaseUrl
	if baseUrl == "" {
		baseUrl = channelconstant.GetChannelBaseURL(channelconstant.ChannelTypeVolcEngine)
	}
	specialPlan, hasSpecialPlan := channelconstant.ChannelSpecialBases[baseUrl]

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		if hasSpecialPlan && specialPlan.ClaudeBaseURL != "" {
			return fmt.Sprintf("%s/v1/messages", specialPlan.ClaudeBaseURL), nil
		}
		if strings.HasPrefix(info.UpstreamModelName, "bot") {
			return fmt.Sprintf("%s/api/v3/bots/chat/completions", baseUrl), nil
		}
		return fmt.Sprintf("%s/api/v3/chat/completions", baseUrl), nil
	default:
		switch info.RelayMode {
		case constant.RelayModeChatCompletions:
			if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
				return fmt.Sprintf("%s/chat/completions", specialPlan.OpenAIBaseURL), nil
			}
			if strings.HasPrefix(info.UpstreamModelName, "bot") {
				return fmt.Sprintf("%s/api/v3/bots/chat/completions", baseUrl), nil
			}
			return fmt.Sprintf("%s/api/v3/chat/completions", baseUrl), nil
		case constant.RelayModeEmbeddings:
			return fmt.Sprintf("%s/api/v3/embeddings", baseUrl), nil
		//豆包的图生图也走generations接口: https://www.volcengine.com/docs/82379/1824121
		case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
			return fmt.Sprintf("%s/api/v3/images/generations", baseUrl), nil
		//case constant.RelayModeImagesEdits:
		//	return fmt.Sprintf("%s/api/v3/images/edits", baseUrl), nil
		case constant.RelayModeRerank:
			return fmt.Sprintf("%s/api/v3/rerank", baseUrl), nil
		case constant.RelayModeResponses:
			return fmt.Sprintf("%s/api/v3/responses", baseUrl), nil
		case constant.RelayModeAudioSpeech:
			if !dto.IsVolcengineTTS3Model(getUpstreamModelName(info)) && !dto.IsVolcengineTTS3Model(info.OriginModelName) {
				return "", fmt.Errorf("unsupported volcengine speech model %q: only %s is supported", info.OriginModelName, ModelSeedTTS30)
			}
			voiceID := a.ttsVoiceID
			if voiceID == "" {
				voiceID = defaultTTS3VoiceID
			}
			return buildTTS3UpstreamURL(info.ChannelBaseUrl, voiceID), nil
		case constant.RelayModeAudioTranscription:
			if baseUrl == channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine] || baseUrl == "" {
				return "https://openspeech.bytedance.com" + PathAUCBigModelSubmit, nil
			}
			return fmt.Sprintf("%s%s", strings.TrimRight(baseUrl, "/"), PathAUCBigModelSubmit), nil
		default:
		}
	}
	return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	if info.RelayMode == constant.RelayModeAudioTranscription {
		apiKey := extractVolcengineApiKey(info.ApiKey)
		req.Set("X-Api-Key", apiKey)
		req.Set("Content-Type", "application/json")
		return nil
	} else if info.RelayMode == constant.RelayModeAudioSpeech {
		apiKey := extractVolcengineApiKey(info.ApiKey)
		req.Set("X-Api-Key", apiKey)
		req.Set("Content-Type", "application/json")
		req.Set("X-Request-Id", uuid.New().String())
		return nil
	} else if info.RelayMode == constant.RelayModeImagesEdits {
		req.Set("Content-Type", gin.MIMEJSON)
	}

	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	if !model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) &&
		!model_setting.ShouldPreserveThinkingSuffix(info.UpstreamModelName) &&
		strings.HasSuffix(info.UpstreamModelName, "-thinking") &&
		strings.HasPrefix(info.UpstreamModelName, "deepseek") {
		info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
		request.Model = info.UpstreamModelName
		request.THINKING = json.RawMessage(`{"type": "enabled"}`)
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == constant.RelayModeAudioTranscription {
		return nil, nil
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayFormat == types.RelayFormatClaude {
		if _, ok := channelconstant.ChannelSpecialBases[info.ChannelBaseUrl]; ok {
			adaptor := claude.Adaptor{}
			return adaptor.DoResponse(c, resp, info)
		}
	}

	if info.RelayMode == constant.RelayModeAudioTranscription {
		var request dto.AudioRequest
		if reqVal, exists := c.Get(contextKeyASRRequest); exists {
			if r, ok := reqVal.(dto.AudioRequest); ok {
				request = r
			}
		}
		return HandleASR(c, info, request)
	}

	if info.RelayMode == constant.RelayModeAudioSpeech {
		return handleTTS3Response(c, resp, info, a.ttsRequest, a.ttsFormat, a.ttsBillableChars)
	}

	adaptor := openai.Adaptor{}
	usage, err = adaptor.DoResponse(c, resp, info)
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
