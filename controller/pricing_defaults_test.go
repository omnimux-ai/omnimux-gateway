package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterPricingDefaultsKeepsOnlyVisibleModelsWithGroups(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "gemini-3.8-flash", EnableGroup: []string{"default"}},
		{ModelName: "gpt-image-2.5", EnableGroup: []string{}},
		{ModelName: "seedance-2-5", EnableGroup: []string{"default"}},
	}
	defaults := map[string]string{
		"chat":           "gemini-3.8-flash",
		"text_to_image":  "gpt-image-2.5",
		"text_to_video":  "seedance-2-5",
		"text_to_music":  "suno",
		"speech_to_text": "whisper-1",
	}

	got := filterPricingDefaults(pricing, defaults)

	require.Equal(t, map[string]string{
		"chat":          "gemini-3.8-flash",
		"text_to_video": "seedance-2-5",
	}, got)
}

func TestFilterPricingDefaultsReturnsNilWhenNoRecommendationIsVisible(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "unrelated-model", EnableGroup: []string{"default"}},
	}
	defaults := map[string]string{
		"chat": "gemini-3.8-flash",
	}

	assert.Nil(t, filterPricingDefaults(pricing, defaults))
	assert.Nil(t, filterPricingDefaults(nil, defaults))
	assert.Nil(t, filterPricingDefaults(pricing, nil))
}
