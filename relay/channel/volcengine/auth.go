package volcengine

import "strings"

// extractVolcengineApiKey 提取火山引擎豆包语音 API Key。
// 兼容传统 "appid|token" 格式（取第二段）与纯 API Key 格式。
func extractVolcengineApiKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if _, after, ok := strings.Cut(trimmed, "|"); ok {
		token, _, _ := strings.Cut(after, "|")
		if token = strings.TrimSpace(token); token != "" {
			return token
		}
	}
	return trimmed
}
