package volcengine

import "strings"

const (
	// TTS 3.0 Models & Endpoint
	ModelSeedTTS30                = "seed-tts-3.0"
	UpstreamModelDoubaoSeedTTS30  = "doubao-seed-tts-3.0"
	VolcengineTTS3BaseURL         = "https://voice.cn-beijing.volces.com"
	VolcengineTTS3PathTemplate    = "/api/v3/text-to-speech/%s/streaming"
	VolcengineLegacySpeechBaseURL = "https://openspeech.bytedance.com"

	// ASR Models
	ModelDoubaoASRBigModel = "doubao-asr-bigmodel"
	ModelSeedASRAUC        = "seedasr-auc"
	ModelBigASRAUC         = "bigasr-auc"

	// Resource IDs
	ResourceSeedASRAUC = "volc.seedasr.auc" // 豆包 2.0
	ResourceBigASRAUC  = "volc.bigasr.auc"  // 豆包 1.0

	// API Paths
	PathAUCBigModelSubmit = "/api/v3/auc/bigmodel/submit"
	PathAUCBigModelQuery  = "/api/v3/auc/bigmodel/query"

	// Status Codes
	StatusSuccess    = "20000000"
	StatusProcessing = "20000001"
	StatusQueued     = "20000002"
)

var ModelList = []string{
	"Doubao-pro-128k",
	"Doubao-pro-32k",
	"Doubao-pro-4k",
	"Doubao-lite-128k",
	"Doubao-lite-32k",
	"Doubao-lite-4k",
	"Doubao-embedding",
	"doubao-seedream-4-0-250828",
	"seedream-4-0-250828",
	"doubao-seedance-1-0-pro-250528",
	"seedance-1-0-pro-250528",
	"doubao-seed-1-6-thinking-250715",
	"seed-1-6-thinking-250715",
	ModelSeedTTS30,
	ModelDoubaoASRBigModel,
	ModelSeedASRAUC,
	ModelBigASRAUC,
}

var ChannelName = "volcengine"

// isASRModel 辅助判断是否为火山 ASR 识别模型
func isASRModel(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	if lower == "" {
		return false
	}
	return lower == ModelDoubaoASRBigModel ||
		lower == ModelSeedASRAUC ||
		lower == ModelBigASRAUC ||
		lower == "doubao-asr" ||
		strings.HasPrefix(lower, "seedasr") ||
		strings.HasPrefix(lower, "bigasr") ||
		strings.Contains(lower, "doubao-asr")
}
