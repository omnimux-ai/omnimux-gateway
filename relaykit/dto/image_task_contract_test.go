package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageTaskContractRejectsUnreviewedSpecificationDeclarations(t *testing.T) {
	for _, declaration := range []string{
		`"sizes":["1024x1024"]`,
		`"sizes":["1024x576","1024x576"]`,
		`"sizes":[""]`,
		`"delivery":"unknown"`,
		`"delivery":" B64_JSON "`,
	} {
		t.Run(declaration, func(t *testing.T) {
			var settings ChannelOtherSettings
			require.NoError(t, json.Unmarshal([]byte(`{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],`+declaration+`}}`), &settings))
			require.Error(t, settings.ValidateImageTaskContract())
		})
	}
}

func TestImageTaskContractRejectsUnrecognizedCapabilities(t *testing.T) {
	for name, contract := range map[string]*ImageTaskContract{
		"unknown protocol": {Protocol: "guess_from_host", Modes: []string{ImageTaskModeText}},
		"missing modes":    {Protocol: ImageTaskProtocolOpenAIMultipart},
		"unknown mode":     {Protocol: ImageTaskProtocolEvoLinkJSON, Modes: []string{"video"}},
		"duplicate mode":   {Protocol: ImageTaskProtocolOpenAIMultipart, Modes: []string{ImageTaskModeText, ImageTaskModeText}},
		"unknown format":   {Protocol: ImageTaskProtocolOpenAIMultipart, Modes: []string{ImageTaskModeReference}, InputFormats: []string{"gif"}},
		"duplicate format": {Protocol: ImageTaskProtocolOpenAIMultipart, Modes: []string{ImageTaskModeReference}, InputFormats: []string{"png", "png"}},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
		})
	}
	require.NoError(t, (*ChannelOtherSettings)(nil).ValidateImageTaskContract())
	require.NoError(t, (&ChannelOtherSettings{}).ValidateImageTaskContract())
}

func TestImageTaskContractRequiresExplicitMode(t *testing.T) {
	contract := &ImageTaskContract{Protocol: ImageTaskProtocolEvoLinkJSON, Modes: []string{ImageTaskModeText, ImageTaskModeReference}}
	require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
	assert.True(t, contract.SupportsMode(ImageTaskModeReference))
	assert.False(t, contract.SupportsMode(ImageTaskModeMaskedEdit))
	assert.False(t, (*ImageTaskContract)(nil).SupportsMode(ImageTaskModeText))
}

func TestImageTaskContractRejectsUnimplementedMultipartMask(t *testing.T) {
	for _, modes := range [][]string{
		{ImageTaskModeMaskedEdit},
		{ImageTaskModeText, ImageTaskModeReference, ImageTaskModeEdit, ImageTaskModeMaskedEdit},
	} {
		contract := &ImageTaskContract{Protocol: ImageTaskProtocolOpenAIMultipart, Modes: modes}
		require.Error(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
		contract.Protocol = ImageTaskProtocolEvoLinkJSON
		require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
	}
	contract := &ImageTaskContract{Protocol: ImageTaskProtocolOpenAIMultipart, Modes: []string{ImageTaskModeText, ImageTaskModeReference, ImageTaskModeEdit}}
	require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
}

func TestImageTaskContractInputFormatsDefaultNarrowly(t *testing.T) {
	contract := &ImageTaskContract{Protocol: ImageTaskProtocolOpenAIMultipart, Modes: []string{ImageTaskModeReference}}
	assert.True(t, contract.SupportsInputFormat("png"))
	assert.False(t, contract.SupportsInputFormat("jpeg"))
	assert.False(t, contract.SupportsInputFormat("webp"))
	assert.False(t, (*ImageTaskContract)(nil).SupportsInputFormat("png"))
	contract.InputFormats = []string{"png", "jpeg", "webp"}
	require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
	assert.True(t, contract.SupportsInputFormat("jpeg"))
	assert.True(t, contract.SupportsInputFormat("webp"))
	assert.False(t, contract.SupportsInputFormat("gif"))
}

func TestImageTaskContractSpecificationDefaultsNeverExpandLegacyQualification(t *testing.T) {
	contract := &ImageTaskContract{Protocol: ImageTaskProtocolOpenAIMultipart, Modes: []string{ImageTaskModeText}}
	require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
	assert.True(t, contract.SupportsSize("2048x1152"))
	assert.False(t, contract.SupportsSize("1024x576"))
	assert.False(t, contract.SupportsSize("16:9"))
	assert.Equal(t, "url", contract.DeliveryFormat())
	contract.Sizes, contract.Delivery = []string{"1024x576"}, "b64_json"
	require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
	assert.True(t, contract.SupportsSize("1024x576"))
	assert.False(t, contract.SupportsSize("2048x1152"))
	assert.Equal(t, "b64_json", contract.DeliveryFormat())
	contract.Sizes, contract.Delivery = []string{"1024x576", "2048x1152"}, "url"
	require.NoError(t, (&ChannelOtherSettings{ImageTaskContract: contract}).ValidateImageTaskContract())
	assert.True(t, contract.SupportsSize("1024x576"))
	assert.True(t, contract.SupportsSize("2048x1152"))
	assert.Equal(t, "url", contract.DeliveryFormat())
	assert.False(t, (*ImageTaskContract)(nil).SupportsSize("2048x1152"))
	assert.Empty(t, (*ImageTaskContract)(nil).DeliveryFormat())
}
