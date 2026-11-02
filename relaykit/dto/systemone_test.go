package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func TestSystemOneRequestTokenCountMetaIncludesSensitivePayload(t *testing.T) {
	request := &SystemOneRequest{
		Input:     map[string]any{"text": "sensitive input phrase"},
		State:     map[string]any{"approval": "pending"},
		Questions: []any{map[string]any{"instructions": "review this decision"}},
	}

	meta := request.GetTokenCountMeta()
	assert.Equal(t, types.TokenTypeTokenizer, meta.TokenType)
	assert.Contains(t, meta.CombineText, "sensitive input phrase")
	assert.Contains(t, meta.CombineText, "pending")
	assert.Contains(t, meta.CombineText, "review this decision")
}

func TestNilSystemOneRequestTokenCountMeta(t *testing.T) {
	var request *SystemOneRequest
	meta := request.GetTokenCountMeta()
	assert.Equal(t, types.TokenTypeTokenizer, meta.TokenType)
	assert.Empty(t, meta.CombineText)
}
