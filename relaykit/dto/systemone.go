package dto

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// SystemOneRequest 对应 TypeSafe AI System One（Jev）决策/判定专用协议请求。
// 原生交互形态为 POST /v1/systemone，包含 input、state 与 questions 等字段。
type SystemOneRequest struct {
	Model     string `json:"model,omitempty"`
	Input     any    `json:"input,omitempty"`
	State     any    `json:"state,omitempty"`
	Questions any    `json:"questions,omitempty"`
}

func (s *SystemOneRequest) IsStream(c *http.Request) bool {
	return false
}

func (s *SystemOneRequest) GetTokenCountMeta() *types.TokenCountMeta {
	if s == nil {
		return &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer}
	}
	parts := make([]string, 0, 3)
	for _, value := range []any{s.Input, s.State, s.Questions} {
		if value == nil {
			continue
		}
		encoded, err := json.Marshal(value)
		if err == nil {
			parts = append(parts, string(encoded))
		}
	}
	return &types.TokenCountMeta{
		TokenType:   types.TokenTypeTokenizer,
		CombineText: strings.Join(parts, "\n"),
	}
}

func (s *SystemOneRequest) SetModelName(modelName string) {
	if s != nil && modelName != "" {
		s.Model = modelName
	}
}
