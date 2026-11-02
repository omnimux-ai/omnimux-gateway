package volcengine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const (
	contextKeyASRRequest = "volcengine_asr_request"
)

// resolveASRResourceId 根据模型名称与用户入参确定火山 Resource ID
func resolveASRResourceId(request dto.AudioRequest, info *relaycommon.RelayInfo) string {
	if request.ResourceId != "" {
		return request.ResourceId
	}
	model := getUpstreamModelName(info)
	if model == "" {
		model = request.Model
	}
	if model == "" && info != nil {
		model = info.OriginModelName
	}
	lower := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(lower, "bigasr") || lower == ModelBigASRAUC {
		return ResourceBigASRAUC
	}
	return ResourceSeedASRAUC
}

// inferAudioFormat 从 URL 或入参推断音频格式
func inferAudioFormat(request dto.AudioRequest, rawURL string) string {
	if request.Format != "" {
		return strings.ToLower(strings.TrimSpace(request.Format))
	}
	cleanURL := strings.Split(rawURL, "?")[0]
	ext := strings.ToLower(filepath.Ext(cleanURL))
	switch ext {
	case ".wav":
		return "wav"
	case ".mp3":
		return "mp3"
	case ".ogg", ".oga":
		return "ogg"
	case ".m4a":
		return "m4a"
	case ".aac":
		return "aac"
	case ".flac":
		return "flac"
	case ".mp4":
		return "mp4"
	case ".pcm":
		return "pcm"
	default:
		return "mp3" // 默认兜底 mp3
	}
}

// buildASRSubmitRequest 构建火山引擎 ASR Submit 请求体
func buildASRSubmitRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (*VolcengineASRSubmitRequest, error) {
	audioURL := request.GetURL()
	if audioURL == "" && c != nil {
		audioURL = c.Query("url")
		if audioURL == "" {
			audioURL = c.PostForm("url")
		}
		if audioURL == "" {
			audioURL = c.PostForm("audio_url")
		}
	}

	if audioURL == "" {
		return nil, errors.New("audio url is required for Auc BigModel ASR: please provide 'url' or 'audio_url' (URL-first mode)")
	}

	format := inferAudioFormat(request, audioURL)

	submitReq := &VolcengineASRSubmitRequest{
		Audio: VolcengineAudioInfo{
			URL:     audioURL,
			Format:  format,
			Rate:    16000,
			Bits:    16,
			Channel: 1,
		},
		Request: VolcengineASRRequestConfig{
			ModelName:      "bigmodel",
			EnableITN:      true,
			EnablePunc:     true,
			EnableDDC:      true,
			ShowUtterances: true,
		},
	}

	if request.EnableITN != nil {
		submitReq.Request.EnableITN = *request.EnableITN
	}
	if request.EnablePunc != nil {
		submitReq.Request.EnablePunc = *request.EnablePunc
	}
	if request.EnableDDC != nil {
		submitReq.Request.EnableDDC = *request.EnableDDC
	}
	if request.EnableSpeakerInfo != nil {
		submitReq.Request.EnableSpeakerInfo = *request.EnableSpeakerInfo
		if submitReq.Request.EnableSpeakerInfo && request.SSDVersion == "" {
			submitReq.Request.SSDVersion = "300"
		}
	}
	if request.SSDVersion != "" {
		submitReq.Request.SSDVersion = request.SSDVersion
	}
	if request.EnableEmotionDetection != nil {
		submitReq.Request.EnableEmotionDetection = *request.EnableEmotionDetection
	}
	if request.Prompt != "" {
		submitReq.Request.Corpus = &VolcengineASRCorpus{
			Context: request.Prompt,
		}
	}

	return submitReq, nil
}

// HandleASR 核心处理协调函数：提交任务、轮询等待、格式转换并生成 Usage 计量
func HandleASR(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (any, *types.NewAPIError) {
	submitReq, err := buildASRSubmitRequest(c, info, request)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest)
	}

	baseURL := ""
	if info != nil && info.ChannelMeta != nil {
		baseURL = info.ChannelBaseUrl
	}
	if baseURL == "" || baseURL == channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngine] {
		baseURL = "https://openspeech.bytedance.com"
	}

	resourceId := resolveASRResourceId(request, info)
	client := NewVolcengineASRClient(baseURL, nil)

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	apiKey := ""
	if info != nil && info.ChannelMeta != nil {
		apiKey = info.ApiKey
	}

	submitResp, err := client.SubmitTask(ctx, apiKey, resourceId, submitReq)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("submit asr task failed: %w", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	poller := NewASRPoller(nil)
	result, err := poller.Poll(ctx, client, apiKey, resourceId, submitResp.TaskId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, 499)
		}
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("poll asr task failed: %w", err), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	originModel := ""
	if info != nil {
		originModel = info.OriginModelName
	}
	formatter := GetASRFormatter(request.ResponseFormat)
	data, contentType, err := formatter.Format(result, originModel)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("format asr response failed: %w", err), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	if c != nil {
		c.Header("Content-Type", contentType)
		c.Data(http.StatusOK, contentType, data)
	}

	// 计算音频时长（从最后一个 utterance 的 EndTime 计算）
	var durationSec float64
	if len(result.Utterances) > 0 {
		lastEndTimeMs := result.Utterances[len(result.Utterances)-1].EndTime
		durationSec = float64(lastEndTimeMs) / 1000.0
	}

	audioTokens := 0
	if durationSec > 0 {
		audioTokens = int(math.Ceil(durationSec / 60.0 * 1000.0))
	} else if len(result.Text) > 0 {
		// 兜底若无分句时间戳但有识别文本，至少计 1000 tokens (1分钟)
		audioTokens = 1000
	}

	usage := &dto.Usage{
		PromptTokens: audioTokens,
		TotalTokens:  audioTokens,
		PromptTokensDetails: dto.InputTokenDetails{
			AudioTokens: audioTokens,
		},
	}

	return usage, nil
}
