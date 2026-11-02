package volcvoice

// 火山引擎豆包语音「音色训练」任务适配器。
//
// 官方接口（文档编号 2534906，https://docs.volcengine.com/docs/DoubaoVoice/tone-training-http）：
//
//	提交训练   POST {base}/api/v3/tts/voice_clone
//	查询状态   POST {base}/api/v3/tts/get_voice
//
// 官方状态机：0 NotFound / 1 Training / 2 Success / 3 Failed / 4 Active；
// status 为 2 或 4 时方可调用合成接口。demo_audio 仅 1 小时有效。
//
// 本适配器只负责协议翻译与状态映射：预扣费、结算与失败退款全部交给原生任务
// 计费管道（EstimateBilling → AdjustBillingOnSubmit → AdjustBillingOnComplete），
// 不引入任何本适配器私有的账目结构。

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	// PublicModelVoiceClone 是音色训练的公开模型 ID（按次计费，独立于按秒的合成模型）。
	PublicModelVoiceClone = "volc-voice-clone"

	submitPath = "/api/v3/tts/voice_clone"
	queryPath  = "/api/v3/tts/get_voice"

	// 官方自定义音色代号命名约束。
	customIDMinLen = 8
	customIDMaxLen = 256

	// VoiceNamespacePrefix 是平台自建音色的专属命名空间。平台生成的音色一律带此前缀，
	// 合成侧据此用一次前缀判断即可跳过官方预置音色（509 款）的归属查询，
	// 不给既有配音流量增加任何开销。
	VoiceNamespacePrefix = "omnimux_"

	// 官方音频上限 10MB。
	maxAudioBytes = 10 * 1024 * 1024
)

// 官方训练状态枚举。
const (
	upstreamStatusNotFound = 0
	upstreamStatusTraining = 1
	upstreamStatusSuccess  = 2
	upstreamStatusFailed   = 3
	upstreamStatusActive   = 4
)

var supportedAudioFormats = map[string]bool{
	"wav": true, "mp3": true, "ogg": true, "m4a": true, "aac": true, "pcm": true,
}

// upstreamErrorHints 把官方错误码翻译成可读处置建议，避免用户只看到裸数字。
var upstreamErrorHints = map[int]string{
	45001109: "参考文本与实际录音差异过大（WERError）：请让录音内容与 text 逐字一致，或省略 text 由服务自行识别",
	45001110: "音频质量不达标（噪声/静音/过短）：建议更换干净录音，或开启 extra_params.enable_audio_denoise",
	45001111: "音色代号已被占用或与官方音色冲突：请更换 custom_speaker_id",
	45001112: "音色槽位不足或未开通「声音复刻」服务：请在豆包语音控制台确认服务状态与音色槽位",
}

// TaskAdaptor 实现 channel.TaskAdaptor。
type TaskAdaptor struct {
	taskcommon.BaseBilling

	ChannelType int
	baseURL     string
	apiKey      string

	// 提交阶段解析出的请求，供 BuildRequestBody 复用，避免重复读取请求体。
	parsed *voiceCloneRequest
}

// ---------------------------------------------------------------------------
// 请求契约
// ---------------------------------------------------------------------------

// voiceCloneRequest 是面向客户端暴露的训练提交请求。
//
// 兼容两种用法：audio_data 直接内联 base64 音频，或 audio_url 由服务端拉取。
type voiceCloneRequest struct {
	Model        string         `json:"model,omitempty"`
	Action       string         `json:"action,omitempty"`
	Audio        string         `json:"audio,omitempty"`      // 等价于 audio_data
	AudioData    string         `json:"audio_data,omitempty"` // base64 音频字节
	AudioURL     string         `json:"audio_url,omitempty"`
	Format       string         `json:"format,omitempty"`
	Text         string         `json:"text,omitempty"` // 参考文本，用于发音准确率比对
	Language     *int           `json:"language,omitempty"`
	SpeakerID    string         `json:"custom_speaker_id,omitempty"`
	DemoText     string         `json:"demo_text,omitempty"`
	Denoise      *bool          `json:"enable_audio_denoise,omitempty"`
	NoVolumeNorm *bool          `json:"disable_volume_normalization,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// upstreamVoiceCloneBody 是发给火山的训练请求体，字段与官方文档一一对应。
type upstreamVoiceCloneBody struct {
	SpeakerID       string               `json:"speaker_id"`
	CustomSpeakerID string               `json:"custom_speaker_id,omitempty"`
	Audio           upstreamAudio        `json:"audio"`
	Text            string               `json:"text,omitempty"`
	Language        *int                 `json:"language,omitempty"`
	ExtraParams     *upstreamExtraParams `json:"extra_params,omitempty"`
}

type upstreamAudio struct {
	Data   string `json:"data"`
	Format string `json:"format,omitempty"`
}

type upstreamExtraParams struct {
	DemoText                   string `json:"demo_text,omitempty"`
	EnableAudioDenoise         *bool  `json:"enable_audio_denoise,omitempty"`
	DisableVolumeNormalization *bool  `json:"disable_volume_normalization,omitempty"`
}

// upstreamVoiceCloneResponse 是训练提交/查询的响应。
type upstreamVoiceCloneResponse struct {
	Code                   int             `json:"code"`
	Message                string          `json:"message"`
	SpeakerID              string          `json:"speaker_id"`
	Status                 *int            `json:"status"`
	DemoAudio              string          `json:"demo_audio"`
	AvailableTrainingTimes *int            `json:"available_training_times"`
	CreateTime             *int64          `json:"create_time"`
	SpeakerStatus          []speakerStatus `json:"speaker_status"`
}

type speakerStatus struct {
	ModelType *int `json:"model_type"`
}

// ---------------------------------------------------------------------------
// 适配器生命周期
// ---------------------------------------------------------------------------

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = extractVolcengineApiKey(info.ApiKey)
}

// extractVolcengineApiKey 与合成侧保持一致的鉴权兼容：
// 渠道密钥若是传统 "appid|token" 形态则取后半段，纯 API Key 直接使用。
func extractVolcengineApiKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if strings.Contains(trimmed, "|") {
		parts := strings.Split(trimmed, "|")
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[1])
		}
	}
	return trimmed
}

// ---------------------------------------------------------------------------
// 校验
// ---------------------------------------------------------------------------

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	req, taskErr := parseVoiceCloneRequest(c)
	if taskErr != nil {
		return taskErr
	}

	if strings.TrimSpace(info.OriginModelName) == "" && strings.TrimSpace(req.Model) != "" {
		info.OriginModelName = strings.TrimSpace(req.Model)
	}
	if strings.TrimSpace(info.OriginModelName) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "invalid_request", http.StatusBadRequest)
	}

	if taskErr := validateAudioPayload(req); taskErr != nil {
		return taskErr
	}

	if err := validateDemoText(req.DemoText); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_demo_text", http.StatusBadRequest)
	}

	req.SpeakerID = resolveSpeakerID(req.SpeakerID)
	if err := validateCustomSpeakerID(req.SpeakerID); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_custom_speaker_id", http.StatusBadRequest)
	}

	a.parsed = req
	info.Action = constant.TaskActionVoiceClone
	return nil
}

func parseVoiceCloneRequest(c *gin.Context) (*voiceCloneRequest, *taskdto.TaskError) {
	var req voiceCloneRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return nil, service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	// audio 与 audio_data 互为别名，取其一。
	if strings.TrimSpace(req.AudioData) == "" && strings.TrimSpace(req.Audio) != "" {
		req.AudioData = strings.TrimSpace(req.Audio)
	}
	return &req, nil
}

func validateAudioPayload(req *voiceCloneRequest) *taskdto.TaskError {
	if strings.TrimSpace(req.AudioData) == "" && strings.TrimSpace(req.AudioURL) == "" {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("audio_data (base64) or audio_url is required"),
			"audio_required", http.StatusBadRequest)
	}

	if data := strings.TrimSpace(req.AudioData); data != "" {
		// base64 编码后长度约为原始字节的 4/3，用解码后估算值做上限判断。
		if approx := len(data) / 4 * 3; approx > maxAudioBytes {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("audio exceeds the official 10MB limit"),
				"audio_too_large", http.StatusBadRequest)
		}
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format != "" && !supportedAudioFormats[format] {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("unsupported audio format %q (wav, mp3, ogg, m4a, aac, pcm)", req.Format),
			"unsupported_audio_format", http.StatusBadRequest)
	}
	return nil
}

func validateDemoText(demoText string) error {
	trimmed := strings.TrimSpace(demoText)
	if trimmed == "" {
		return nil
	}
	if n := len([]rune(trimmed)); n < 4 || n > 300 {
		return fmt.Errorf("demo_text must be 4~300 characters, got %d", n)
	}
	return nil
}

// resolveSpeakerID 在客户端未指定代号时生成一个符合官方命名规范的唯一代号。
func resolveSpeakerID(given string) string {
	if trimmed := strings.TrimSpace(given); trimmed != "" {
		return trimmed
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// 极端情况下退回时间戳，仍满足命名规范。
		return VoiceNamespacePrefix + "voice_" + strconv.FormatInt(common.GetTimestamp(), 36)
	}
	return VoiceNamespacePrefix + hex.EncodeToString(buf)
}

// officialConflictPattern 是官方文档给出的音色代号冲突/格式拦截正则，原样采用，
// 以保证平台侧判定与上游一致（官方前缀 S_/ICL_/MIX_/DiT_/BV、官方资源前缀、
// 官方后缀 _bigtts/_tob/_streaming 等）。
var officialConflictPattern = regexp.MustCompile(
	`^((?i:S_|ICL_|MIX_|DiT_|BV)|[a-z]{2}_|(?i:(wvae|moon|mercury|venus|earth|mars|jupiter|saturn|uranus|neptune|pluto|umm)_)).*|.*_(?i:bigtts|bigtts_cc|tob|cs_tob|streaming)$|^[^a-zA-Z]|.*[-_]$|^.{0,7}$|^.{257,}$|.*[^a-zA-Z0-9_-].*`)

// validateCustomSpeakerID 复刻官方命名规范：
// 8~256 字符、仅字母数字中划线下划线、必须以英文字母开头、首尾不可为符号，
// 且不得与官方音色冲突（官方保留前缀/后缀）。
func validateCustomSpeakerID(id string) error {
	if len(id) < customIDMinLen || len(id) > customIDMaxLen {
		return fmt.Errorf("custom_speaker_id must be %d~%d characters, got %d", customIDMinLen, customIDMaxLen, len(id))
	}
	first := id[0]
	if !(first >= 'a' && first <= 'z' || first >= 'A' && first <= 'Z') {
		return fmt.Errorf("custom_speaker_id must start with an English letter")
	}
	last := id[len(id)-1]
	if last == '-' || last == '_' {
		return fmt.Errorf("custom_speaker_id must not end with '-' or '_'")
	}
	for _, r := range id {
		isAlphaNum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !isAlphaNum && r != '-' && r != '_' {
			return fmt.Errorf("custom_speaker_id may only contain letters, digits, '-' and '_'")
		}
	}
	if officialConflictPattern.MatchString(id) {
		return fmt.Errorf("custom_speaker_id conflicts with official voice naming (reserved prefix/suffix)")
	}
	// 平台音色必须留在专属命名空间内：命名空间之外的代号无法被合成侧的归属校验
	// 识别，等于绕过所有权约束。
	if !strings.HasPrefix(id, VoiceNamespacePrefix) {
		return fmt.Errorf("custom_speaker_id must start with the platform namespace %q", VoiceNamespacePrefix)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 计费
// ---------------------------------------------------------------------------
//
// 训练是按次固定计价：预扣即最终价，无需按量纲倍率调整，失败由任务框架的原生
// 退款路径处理。因此本适配器直接复用 taskcommon.BaseBilling 的默认实现
// （EstimateBilling/AdjustBillingOnSubmit 返回 nil，AdjustBillingOnComplete 返回 0），
// 不引入任何本适配器私有的计费逻辑。

// ---------------------------------------------------------------------------
// 提交
// ---------------------------------------------------------------------------

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	}
	if a.baseURL == "" {
		a.baseURL = "https://openspeech.bytedance.com"
	}
	return a.baseURL + submitPath, nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", a.apiKey)
	req.Header.Set("X-Api-Request-Id", newRequestID())
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	if a.parsed == nil {
		req, taskErr := parseVoiceCloneRequest(c)
		if taskErr != nil {
			return nil, taskErr.Error
		}
		a.parsed = req
	}

	body := upstreamVoiceCloneBody{
		// 官方规定：使用自定义音色时 speaker_id 必须传固定值。
		SpeakerID:       "custom_speaker_id",
		CustomSpeakerID: a.parsed.SpeakerID,
		Audio: upstreamAudio{
			Data:   strings.TrimSpace(a.parsed.AudioData),
			Format: strings.ToLower(strings.TrimSpace(a.parsed.Format)),
		},
		Text:     strings.TrimSpace(a.parsed.Text),
		Language: a.parsed.Language,
	}

	extra := upstreamExtraParams{
		DemoText:                   strings.TrimSpace(a.parsed.DemoText),
		EnableAudioDenoise:         a.parsed.Denoise,
		DisableVolumeNormalization: a.parsed.NoVolumeNorm,
	}
	if extra.DemoText != "" || extra.EnableAudioDenoise != nil || extra.DisableVolumeNormalization != nil {
		body.ExtraParams = &extra
	}

	encoded, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(encoded), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "read_response_failed", http.StatusInternalServerError)
	}

	var upstream upstreamVoiceCloneResponse
	if err := common.Unmarshal(respBody, &upstream); err != nil {
		return nil, service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
	}

	if resp.StatusCode != http.StatusOK || (upstream.Code != 0 && upstream.Code != 200) {
		return nil, service.TaskErrorWrapperLocal(
			fmt.Errorf("%s", describeUpstreamError(upstream.Code, upstream.Message)),
			"upstream_voice_clone_failed", http.StatusBadRequest)
	}

	speakerID := strings.TrimSpace(upstream.SpeakerID)
	if speakerID == "" {
		speakerID = a.parsed.SpeakerID
	}

	taskData, err := common.Marshal(voiceCloneTaskData{
		SpeakerID:              speakerID,
		CustomSpeakerID:        a.parsed.SpeakerID,
		Status:                 upstream.Status,
		DemoAudio:              upstream.DemoAudio,
		AvailableTrainingTimes: upstream.AvailableTrainingTimes,
		CreateTime:             upstream.CreateTime,
		ModelType:              firstModelType(upstream.SpeakerStatus),
	})
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "marshal_task_data_failed", http.StatusInternalServerError)
	}

	submit := &channel.TaskSubmitResponse{
		UpstreamTaskID: speakerID,
		TaskData:       taskData,
	}

	// 上游可能在提交阶段就返回终态（训练很快或复刻 2.0 即时生效），
	// 此时直接给出终态，避免多轮无意义轮询。
	if info := mapUpstreamStatus(&upstream); info != nil {
		submit.Immediate = info
	}

	return submit, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{PublicModelVoiceClone}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "volcengine-voice-clone"
}

// ---------------------------------------------------------------------------
// 轮询
// ---------------------------------------------------------------------------

// FetchTask 按音色代号回查训练状态。官方查询接口只接受 speaker_id。
func (a *TaskAdaptor) FetchTask(baseUrl, key string, task *model.Task, proxy string) (*http.Response, error) {
	body := map[string]string{"speaker_id": task.TaskID}
	encoded, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseUrl, "/") + queryPath
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", extractVolcengineApiKey(key))
	req.Header.Set("X-Api-Request-Id", newRequestID())

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(task *model.Task, resp *http.Response, respBody []byte) (*relaycommon.TaskInfo, error) {
	var upstream upstreamVoiceCloneResponse
	if err := common.Unmarshal(respBody, &upstream); err != nil {
		return nil, err
	}

	if upstream.Code != 0 && upstream.Code != 200 {
		return relaycommon.FailTaskInfo(describeUpstreamError(upstream.Code, upstream.Message)), nil
	}

	info := &relaycommon.TaskInfo{
		TaskID: task.TaskID,
		Code:   upstream.Code,
	}
	if upstream.Status == nil {
		return relaycommon.FailTaskInfo("upstream returned no training status"), nil
	}

	switch *upstream.Status {
	case upstreamStatusSuccess, upstreamStatusActive:
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		info.Url = strings.TrimSpace(upstream.DemoAudio)
		info.RemoteUrl = info.Url
	case upstreamStatusFailed:
		return relaycommon.FailTaskInfo("voice training failed upstream"), nil
	case upstreamStatusTraining:
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	default:
		info.Status = string(model.TaskStatusNotStart)
		info.Progress = taskcommon.ProgressSubmitted
	}
	return info, nil
}

// ---------------------------------------------------------------------------
// 内部工具
// ---------------------------------------------------------------------------

// voiceCloneTaskData 是持久化到任务记录里的训练结果快照。
// 归属（用户 ↔ 音色）由任务记录原生的 user_id 承载，无需额外账目表。
type voiceCloneTaskData struct {
	SpeakerID              string `json:"speaker_id"`
	CustomSpeakerID        string `json:"custom_speaker_id"`
	Status                 *int   `json:"status,omitempty"`
	DemoAudio              string `json:"demo_audio,omitempty"`
	AvailableTrainingTimes *int   `json:"available_training_times,omitempty"`
	CreateTime             *int64 `json:"create_time,omitempty"`
	ModelType              *int   `json:"model_type,omitempty"`
}

// mapUpstreamStatus 在上游已给出终态时直接返回终态信息。
func mapUpstreamStatus(upstream *upstreamVoiceCloneResponse) *relaycommon.TaskInfo {
	if upstream.Status == nil {
		return nil
	}
	switch *upstream.Status {
	case upstreamStatusSuccess, upstreamStatusActive:
		demo := strings.TrimSpace(upstream.DemoAudio)
		return &relaycommon.TaskInfo{
			Status:    string(model.TaskStatusSuccess),
			Progress:  taskcommon.ProgressComplete,
			Url:       demo,
			RemoteUrl: demo,
		}
	case upstreamStatusFailed:
		return relaycommon.FailTaskInfo("voice training failed upstream")
	default:
		return nil
	}
}

func firstModelType(list []speakerStatus) *int {
	for _, item := range list {
		if item.ModelType != nil {
			return item.ModelType
		}
	}
	return nil
}

func describeUpstreamError(code int, message string) string {
	base := fmt.Sprintf("upstream error %d: %s", code, strings.TrimSpace(message))
	if hint, ok := upstreamErrorHints[code]; ok {
		return base + " — " + hint
	}
	return base
}

func newRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(common.GetTimestamp(), 10)
	}
	return hex.EncodeToString(buf)
}

var _ channel.TaskAdaptor = (*TaskAdaptor)(nil)
var _ = constant.ChannelTypeVolcEngine
