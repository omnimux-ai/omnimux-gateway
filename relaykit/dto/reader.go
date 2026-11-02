package dto

import (
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// ReaderRequest 对应 OmniMux 的「URL → LLM 友好文本」读取能力，上游是
// Jina Reader（https://r.jina.ai）。请求无标准 OpenAI 兼容语义，独立成 DTO。
type ReaderRequest struct {
	// Model 是 OmniMux 侧模型名（如 jina-reader-v1），用于渠道路由与计费。
	Model string `json:"model"`
	// Url 是要抓取的目标 URL（http/https），会拼在 r.jina.ai 之后。
	Url string `json:"url"`
	// ReturnFormat 控制响应格式：markdown（默认）或 json。
	ReturnFormat string `json:"return_format,omitempty"`

	// 以下为可选的 Jina Reader 请求头透传项，为空则不设置。
	RemoveSelector     string `json:"remove_selector,omitempty"`      // X-Remove-Selector
	TargetSelector     string `json:"target_selector,omitempty"`      // X-Target-Selector
	WaitForSelector    string `json:"wait_for_selector,omitempty"`    // X-Wait-For-Selector
	Timeout            string `json:"timeout,omitempty"`              // X-Timeout（秒）
	RetainImages       string `json:"retain_images,omitempty"`        // X-Retain-Images（none 移除图片）
	WithLinksSummary   bool   `json:"with_links_summary,omitempty"`   // X-With-Links-Summary
	WithImagesSummary  bool   `json:"with_images_summary,omitempty"`  // X-With-Images-Summary
	WithGeneratedAlt   bool   `json:"with_generated_alt,omitempty"`   // X-With-Generated-Alt
	NoCache            bool   `json:"no_cache,omitempty"`             // X-No-Cache
	RespondWith        string `json:"respond_with,omitempty"`         // X-Respond-With（readerlm-v2）
}

func (r *ReaderRequest) IsStream(c *http.Request) bool {
	return false
}

func (r *ReaderRequest) GetTokenCountMeta() *types.TokenCountMeta {
	// reader 的输入只有 URL，预扣阶段按 URL 长度估算；实际结算按上游返回的
	// markdown 输出 token 计费（见 jina adaptor 的 reader DoResponse）。
	return &types.TokenCountMeta{
		CombineText: r.Url,
	}
}

func (r *ReaderRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}
