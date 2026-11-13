package volcengine

// VolcengineASRSubmitRequest 火山引擎 ASR 提交任务请求体
type VolcengineASRSubmitRequest struct {
	Audio   VolcengineAudioInfo        `json:"audio"`
	Request VolcengineASRRequestConfig `json:"request"`
	User    *VolcengineUserMeta        `json:"user,omitempty"`
}

// VolcengineAudioInfo 音频信息对象
type VolcengineAudioInfo struct {
	URL     string `json:"url"`               // 音频公网可访问 URL
	Format  string `json:"format"`            // wav, mp3, ogg, pcm, aac, m4a 等
	Codec   string `json:"codec,omitempty"`   // 编码格式
	Rate    int    `json:"rate,omitempty"`    // 采样率，默认 16000
	Bits    int    `json:"bits,omitempty"`    // 位深，默认 16
	Channel int    `json:"channel,omitempty"` // 声道数，默认 1
}

// VolcengineASRRequestConfig ASR 大模型任务控制参数
type VolcengineASRRequestConfig struct {
	ModelName              string               `json:"model_name"`                         // 固定值 "bigmodel"
	EnableITN              bool                 `json:"enable_itn"`                         // 文本标准化，默认 true
	EnablePunc             bool                 `json:"enable_punc"`                        // 标点符号预测，默认 true
	EnableDDC              bool                 `json:"enable_ddc"`                         // 顺滑（口吃、语气词过滤），默认 true
	ShowUtterances         bool                 `json:"show_utterances"`                    // 输出分句结果，必须为 true
	EnableSpeakerInfo      bool                 `json:"enable_speaker_info,omitempty"`      // 说话人分离
	SSDVersion             string               `json:"ssd_version,omitempty"`              // 说话人分离模型版本
	EnableEmotionDetection bool                 `json:"enable_emotion_detection,omitempty"` // 情绪检测
	Corpus                 *VolcengineASRCorpus `json:"corpus,omitempty"`                   // 上下文提示词
}

// VolcengineASRCorpus 上下文/热词配置
type VolcengineASRCorpus struct {
	Context string `json:"context,omitempty"`
}

// VolcengineUserMeta 用户元数据
type VolcengineUserMeta struct {
	UID string `json:"uid,omitempty"`
}

// VolcengineASRSubmitResponse 火山引擎 Submit API 响应
type VolcengineASRSubmitResponse struct {
	TaskId     string `json:"task_id"`
	StatusCode string `json:"status_code,omitempty"`
	Message    string `json:"message,omitempty"`
}

// VolcengineASRQueryResponse 火山引擎 Query API 响应
type VolcengineASRQueryResponse struct {
	StatusCode string              `json:"status_code,omitempty"`
	Message    string              `json:"message,omitempty"`
	Result     VolcengineASRResult `json:"result"`
}

// VolcengineASRResult ASR 识别聚合结果
type VolcengineASRResult struct {
	Text       string                   `json:"text"`                 // 全文合并纯文本
	Utterances []VolcengineASRUtterance `json:"utterances,omitempty"` // 分句识别明细列表
}

// VolcengineASRUtterance 句级识别结果
type VolcengineASRUtterance struct {
	Text      string                 `json:"text"`                // 分句文本
	StartTime int64                  `json:"start_time"`          // 起始时间（毫秒）
	EndTime   int64                  `json:"end_time"`            // 结束时间（毫秒）
	Words     []VolcengineASRWord    `json:"words,omitempty"`     // 词级详情
	Additions VolcengineASRAdditions `json:"additions,omitempty"` // 说话人与情绪等附加信息
}

// VolcengineASRWord 词级时间戳明细
type VolcengineASRWord struct {
	Text          string `json:"text"`
	StartTime     int64  `json:"start_time"` // 毫秒
	EndTime       int64  `json:"end_time"`   // 毫秒
	BlankDuration int64  `json:"blank_duration,omitempty"`
}

// VolcengineASRAdditions 附加识别属性
type VolcengineASRAdditions struct {
	Speaker    string  `json:"speaker,omitempty"`
	Emotion    string  `json:"emotion,omitempty"`
	SpeechRate float64 `json:"speech_rate,omitempty"`
	Speed      float64 `json:"speed,omitempty"` // 兼容 speed 属性
	Volume     float64 `json:"volume,omitempty"`
}
