package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func imageTaskAliasChannel(t *testing.T, upstream string, contract *relaykitdto.ImageTaskContract) *model.Channel {
	t.Helper()
	settings, err := common.Marshal(relaykitdto.ChannelOtherSettings{ImageTaskContract: contract})
	require.NoError(t, err)
	mapping, err := common.Marshal(map[string]string{"gpt-image-2.5-sunburst": upstream})
	require.NoError(t, err)
	mappingText := string(mapping)
	return &model.Channel{Type: constant.ChannelTypeOpenAI, OtherSettings: string(settings), ModelMapping: &mappingText}
}

func TestImageTaskChannelSupportsExactSunburstAliases(t *testing.T) {
	for _, tc := range []struct {
		name, public, upstream string
		want                   bool
	}{
		{"original identity", "gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst", true},
		{"exact 1k alias", "gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst-1k", true},
		{"different public model", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst-1k", false},
		{"alias is not public identity", "gpt-image-2.5-sunburst-1k", "gpt-image-2.5-sunburst-1k", false},
		{"public whitespace", " gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst-1k", false},
		{"different upstream model", "gpt-image-2.5-sunburst", "gpt-image-2.5", false},
		{"different 1k model", "gpt-image-2.5-sunburst", "gpt-image-2.5-1k", false},
		{"unknown suffix", "gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst-1k-preview", false},
		{"case variant", "gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst-1K", false},
		{"upstream whitespace", "gpt-image-2.5-sunburst", "gpt-image-2.5-sunburst-1k ", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := imageTaskAliasChannel(t, tc.upstream, &relaykitdto.ImageTaskContract{Protocol: relaykitdto.ImageTaskProtocolOpenAIMultipart, Modes: []string{relaykitdto.ImageTaskModeText}})
			assert.Equal(t, tc.want, ImageTaskChannelSupportsMode(ch, tc.public, relaykitdto.ImageTaskModeText))
		})
	}
}

func TestImageTaskAliasKeepsChannelCapabilityGuards(t *testing.T) {
	for _, tc := range []struct {
		name, mode, format string
		change             func(*model.Channel, *relaykitdto.ChannelOtherSettings)
		want               bool
	}{
		{name: "declared text", mode: relaykitdto.ImageTaskModeText, want: true},
		{name: "undeclared edit", mode: relaykitdto.ImageTaskModeEdit},
		{name: "default PNG", mode: relaykitdto.ImageTaskModeReference, format: "png", change: func(_ *model.Channel, s *relaykitdto.ChannelOtherSettings) {
			s.ImageTaskContract.Modes = []string{relaykitdto.ImageTaskModeReference}
		}, want: true},
		{name: "undeclared JPEG", mode: relaykitdto.ImageTaskModeText, format: "jpeg"},
		{name: "declared JPEG", mode: relaykitdto.ImageTaskModeText, format: "jpeg", change: func(_ *model.Channel, s *relaykitdto.ChannelOtherSettings) {
			s.ImageTaskContract.InputFormats = []string{"jpeg"}
		}, want: true},
		{name: "missing contract", mode: relaykitdto.ImageTaskModeText, change: func(_ *model.Channel, s *relaykitdto.ChannelOtherSettings) { s.ImageTaskContract = nil }},
		{name: "invalid protocol", mode: relaykitdto.ImageTaskModeText, change: func(_ *model.Channel, s *relaykitdto.ChannelOtherSettings) {
			s.ImageTaskContract.Protocol = "unreviewed"
		}},
		{name: "multipart mask declaration", mode: relaykitdto.ImageTaskModeText, change: func(_ *model.Channel, s *relaykitdto.ChannelOtherSettings) {
			s.ImageTaskContract.Modes = []string{relaykitdto.ImageTaskModeText, relaykitdto.ImageTaskModeMaskedEdit}
		}},
		{name: "wrong channel type", mode: relaykitdto.ImageTaskModeText, change: func(ch *model.Channel, _ *relaykitdto.ChannelOtherSettings) { ch.Type = constant.ChannelTypeAnthropic }},
		{name: "parameter override", mode: relaykitdto.ImageTaskModeText, change: func(ch *model.Channel, _ *relaykitdto.ChannelOtherSettings) {
			value := `{"size":"1024x1024"}`
			ch.ParamOverride = &value
		}},
		{name: "empty parameter override", mode: relaykitdto.ImageTaskModeText, change: func(ch *model.Channel, _ *relaykitdto.ChannelOtherSettings) { value := `{}`; ch.ParamOverride = &value }, want: true},
		{name: "chained mapping", mode: relaykitdto.ImageTaskModeText, change: func(ch *model.Channel, _ *relaykitdto.ChannelOtherSettings) {
			value := `{"gpt-image-2.5-sunburst":"internal-reviewed-alias","internal-reviewed-alias":"gpt-image-2.5-sunburst-1k"}`
			ch.ModelMapping = &value
		}, want: true},
		{name: "parameter model routing", mode: relaykitdto.ImageTaskModeText, change: func(_ *model.Channel, s *relaykitdto.ChannelOtherSettings) {
			s.AdvancedCustom = &relaykitdto.AdvancedCustomConfig{ModelByParam: map[string]relaykitdto.ParamModelRouting{"gpt-image-2.5-sunburst": {Param: "size"}}}
		}},
		{name: "malformed mapping", mode: relaykitdto.ImageTaskModeText, change: func(ch *model.Channel, _ *relaykitdto.ChannelOtherSettings) { value := `{`; ch.ModelMapping = &value }},
		{name: "mapping cycle", mode: relaykitdto.ImageTaskModeText, change: func(ch *model.Channel, _ *relaykitdto.ChannelOtherSettings) {
			value := `{"gpt-image-2.5-sunburst":"gpt-image-2.5-sunburst-1k","gpt-image-2.5-sunburst-1k":"gpt-image-2.5-sunburst"}`
			ch.ModelMapping = &value
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := imageTaskAliasChannel(t, "gpt-image-2.5-sunburst-1k", &relaykitdto.ImageTaskContract{Protocol: relaykitdto.ImageTaskProtocolOpenAIMultipart, Modes: []string{relaykitdto.ImageTaskModeText}})
			var settings relaykitdto.ChannelOtherSettings
			require.NoError(t, common.UnmarshalJsonStr(ch.OtherSettings, &settings))
			if tc.change != nil {
				tc.change(ch, &settings)
			}
			encoded, err := common.Marshal(settings)
			require.NoError(t, err)
			ch.OtherSettings = string(encoded)
			var formats []string
			if tc.format != "" {
				formats = []string{tc.format}
			}
			assert.Equal(t, tc.want, ImageTaskChannelSupportsMode(ch, "gpt-image-2.5-sunburst", tc.mode, formats...))
		})
	}
	assert.False(t, ImageTaskChannelSupportsMode(nil, "gpt-image-2.5-sunburst", relaykitdto.ImageTaskModeText))
}

func TestImageTaskCandidateRequiresTheFrozenRequestSpecification(t *testing.T) {
	for _, tc := range []struct {
		name, size, delivery string
		declared             []string
		want                 bool
	}{
		{name: "legacy explicit 2K", size: "2048x1152", want: true},
		{name: "legacy cannot imply 1K", size: "1024x576"},
		{name: "new inline 1K", size: "1024x576", declared: []string{"1024x576"}, delivery: "b64_json", want: true},
		{name: "1K cannot downgrade legacy", size: "2048x1152", declared: []string{"1024x576"}, delivery: "b64_json"},
		{name: "reviewed URL 1K", size: "1024x576", declared: []string{"1024x576", "2048x1152"}, delivery: "url", want: true},
		{name: "unknown size declaration", size: "1024x576", declared: []string{"1024x576", "1024x1024"}},
		{name: "unknown delivery", size: "1024x576", declared: []string{"1024x576"}, delivery: "inline"},
		{name: "missing canonical size", declared: []string{"1024x576", "2048x1152"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contract := &relaykitdto.ImageTaskContract{Protocol: relaykitdto.ImageTaskProtocolOpenAIMultipart, Modes: []string{"text"}, Sizes: tc.declared, Delivery: tc.delivery}
			ch := imageTaskAliasChannel(t, "gpt-image-2.5-sunburst-1k", contract)
			assert.Equal(t, tc.want, ImageTaskChannelSupportsRequest(ch, "gpt-image-2.5-sunburst", "text", tc.size))
			assert.False(t, ImageTaskChannelSupportsRequest(ch, "gpt-image-2.5-sunburst", "edit", tc.size), "size never substitutes for a mode qualification")
		})
	}
}

func TestImageTaskCanonicalSpecificationRequiresEveryFrozenField(t *testing.T) {
	canonical := map[string]any{"model": "gpt-image-2.5-sunburst", "size": "2048x1152", "quality": "high", "n": 1, "output_format": "png", "response_format": "url"}
	for _, size := range []string{"2048x1152", "1024x576"} {
		canonical["size"] = size
		data, err := common.Marshal(canonical)
		require.NoError(t, err)
		actual, err := ParseImageTaskRequestSize(data)
		require.NoError(t, err)
		assert.Equal(t, size, actual)
	}
	for field, invalid := range map[string]any{"model": "gpt-image-2.5-sunburst-1k", "size": "16:9", "quality": "medium", "n": 2, "output_format": "jpeg", "response_format": "b64_json"} {
		t.Run(field, func(t *testing.T) {
			body := make(map[string]any, len(canonical))
			for key, value := range canonical {
				body[key] = value
			}
			body[field] = invalid
			_, err := ImageTaskRequestSize(body)
			require.Error(t, err)
			delete(body, field)
			_, err = ImageTaskRequestSize(body)
			require.Error(t, err, "missing fields must not be recovered using today's defaults")
		})
	}
	for _, body := range []string{`null`, `{`, `[]`, `{"model":"gpt-image-2.5-sunburst","size":"1024x576","quality":"high","n":true,"output_format":"png","response_format":"url"}`} {
		_, err := ParseImageTaskRequestSize([]byte(body))
		assert.Error(t, err)
	}
}
