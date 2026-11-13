package volcengine

import (
	"strings"
)

// extractVolcengineApiKey 提取火山引擎 API Key。
// 兼容 "appid|token" 格式（取后半段 token）和纯 API Key 格式。
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
