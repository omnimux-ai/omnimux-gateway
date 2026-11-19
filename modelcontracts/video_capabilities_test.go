package modelcontracts

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testVideoRuleDocument(model string, channel int, profile, variants string) string {
	channelType := constant.ChannelTypeOpenAI
	if profile == "apimart_video" {
		channelType = constant.ChannelTypeApiMart
	}
	return fmt.Sprintf(`{"models":{%q:[{
		"channel_id":%d,"channel_type":%d,"profile":%q,"upstream_model":%q,
		"base_url":"https://example.com/video/","evidence":"verified fixture",
		"operations":[%s]
	}]}}`, model, channel, channelType, profile, model, variants)
}

func TestVideoCapabilitiesPreserveReferenceModeAcrossChannelLimits(t *testing.T) {
	const variants = `{"id":"video_multi_ref","inputs":[{"slot":"reference_images","max":2},{"slot":"prompt"}]}`
	rules, err := ReadVideoCapabilities(strings.NewReader(testVideoRuleDocument("muse-video", 7, "openai_video", variants)))
	require.NoError(t, err)
	require.True(t, rules.Managed("muse-video"))
	assert.False(t, rules.Managed("seedance-2-5"))
	assert.True(t, rules.UsesChannel(7))
	assert.False(t, rules.UsesChannel(8))
	assert.Equal(t, []int{7}, rules.ChannelIDs())

	identity, found := rules.ChannelIdentity("muse-video", 7)
	require.True(t, found)
	assert.Equal(t, VideoChannelIdentity{
		ChannelType: 1, Profile: "openai_video", UpstreamModel: "muse-video",
		BaseURL: "https://example.com/video",
	}, identity)

	rule, found := rules.ChannelRule("muse-video", 7)
	require.True(t, found)
	require.Len(t, rule.Operations, 1)
	operation := rule.Operations[0]
	require.Len(t, operation.Inputs, 2)
	assert.Equal(t, "image_reference", operation.ModeKind)
	assert.Equal(t, "prompt", operation.Inputs[0].Slot)
	assert.Equal(t, "first_frame", operation.Inputs[1].FirstItemRole)
	require.NotNil(t, operation.Inputs[1].Max)
	assert.Equal(t, 2, *operation.Inputs[1].Max)

	for _, tc := range []struct {
		images   int
		accepted bool
	}{{1, false}, {2, true}, {10, false}} {
		request := VideoRequest{Operation: "video_multi_ref", Slots: map[string]int{"prompt": 1, "reference_images": tc.images}}
		assert.Equal(t, tc.accepted, rules.Accepts("muse-video", 7, request))
	}

	*rule.Operations[0].Inputs[1].Max = 10
	rule.Operations[0].Inputs[1].AllowedMimes[0] = "text/plain"
	rule.Operations[0].Parameters[1].Enum[0] = "bad"
	original, found := rules.ChannelRule("muse-video", 7)
	require.True(t, found)
	assert.Equal(t, 2, *original.Operations[0].Inputs[1].Max)
	assert.NotContains(t, original.Operations[0].Inputs[1].AllowedMimes, "text/plain")
	assert.NotContains(t, original.Operations[0].Parameters[1].Enum, "bad")

	empty, err := ReadVideoCapabilities(strings.NewReader(`{"models":{"seedance-2.5":[]}}`))
	require.NoError(t, err)
	assert.True(t, empty.Managed("seedance-2-5"))
	assert.True(t, empty.Managed("SEEDANCE-2.5"))
	_, found = empty.ChannelRule("seedance-2-5", 7)
	assert.False(t, found)
}

func TestVideoCapabilitiesMatchCompleteSeedanceVariants(t *testing.T) {
	const variants = `
		{
			"id":"video_multi_ref",
			"inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true}],
			"parameters":[{"name":"duration","fixed":-1},{"name":"resolution","fixed":"480p"},{"name":"generate_audio","fixed":false},{"name":"output_format","enum":["mov"]}]
		},
		{
			"id":"video_multi_ref",
			"inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":2}],
			"parameters":[{"name":"duration"},{"name":"resolution","fixed":"720p","default":"720p"},{"name":"generate_audio"}]
		},
		{"id":"text_to_video","parameters":[{"name":"output_format","fixed":"mov","default":"mov"}]}`
	rules, err := ReadVideoCapabilities(strings.NewReader(testVideoRuleDocument("seedance-2-5", 7, "apimart_video", variants)))
	require.NoError(t, err)
	rule, found := rules.ChannelRule("seedance-2.5", 7)
	require.True(t, found)
	assert.Equal(t, "multimodal_reference", rule.Operations[0].ModeKind)
	assert.True(t, rule.Operations[0].Parameters[0].Required)
	assert.True(t, rule.Operations[0].Parameters[1].Required)
	assert.True(t, rule.Operations[0].Parameters[2].Required, "static UI default must not authorize omission")
	assert.Equal(t, []string{"mov"}, rule.Operations[0].Output.Formats)
	assert.ElementsMatch(t, []string{"mp4", "mov"}, rule.Operations[1].Output.Formats)
	assert.Equal(t, []string{"mov"}, rule.Operations[2].Output.Formats)
	assert.True(t, rules.Accepts("seedance-2.5", 7, VideoRequest{
		Operation: "text_to_video",
		Slots:     map[string]int{"prompt": 1},
	}), "a verified default permits format omission")

	for _, tc := range []struct {
		name       string
		images     int
		parameters map[string]any
		accepted   bool
	}{
		{"auto with explicit false and allowed format", 10, map[string]any{"duration": -1, "resolution": "480p", "generate_audio": false, "output_format": "mov"}, true},
		{"wrong format", 10, map[string]any{"duration": -1, "resolution": "480p", "generate_audio": false, "output_format": "mp4"}, false},
		{"missing narrowed format", 10, map[string]any{"duration": -1, "resolution": "480p", "generate_audio": false}, false},
		{"duration gap", 10, map[string]any{"duration": 0, "resolution": "480p", "generate_audio": false, "output_format": "mov"}, false},
		{"mixed channel axes", 10, map[string]any{"duration": 10, "resolution": "720p"}, false},
		{"missing narrowed boolean", 10, map[string]any{"duration": -1, "resolution": "480p", "output_format": "mov"}, false},
		{"missing all references", 0, map[string]any{"duration": 10, "resolution": "720p"}, false},
		{"verified omitted resolution", 2, map[string]any{"duration": 10, "output_format": "mov"}, true},
		{"fallback format stays available", 2, map[string]any{"duration": 10, "output_format": "mp4"}, true},
		{"omission with no format restriction", 2, map[string]any{"duration": 10}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := VideoRequest{
				Operation:  "video_multi_ref",
				Slots:      map[string]int{"prompt": 1, "reference_images": tc.images},
				Parameters: tc.parameters,
			}
			assert.Equal(t, tc.accepted, rules.Accepts("seedance-2.5", 7, request))
		})
	}
}

func TestVideoCapabilitiesPublishMuseGatewayDuration(t *testing.T) {
	const variants = `
		{"id":"video_multi_ref","parameters":[{"name":"duration","minimum":5}]},
		{"id":"first_frame","parameters":[{"name":"duration","fixed":10}]}
	`
	rules, err := ReadVideoCapabilities(strings.NewReader(testVideoRuleDocument("muse-video", 7, "openai_video", variants)))
	require.NoError(t, err)
	rule, found := rules.ChannelRule("muse-video", 7)
	require.True(t, found)
	inherited := rule.Operations[0].Parameters[0]
	assert.Equal(t, 5, inherited.Default)
	assert.False(t, inherited.Required)
	fixed := rule.Operations[1].Parameters[0]
	assert.Nil(t, fixed.Default)
	assert.True(t, fixed.Required)

	for _, tc := range []struct{ variant, message string }{
		{`{"id":"text_to_video","parameters":[{"name":"aspect_ratio"}]}`, "requires a duration parameter"},
		{`{"id":"text_to_video","parameters":[{"name":"duration","default":6}]}`, "duration default conflicts"},
	} {
		_, err = ReadVideoCapabilities(strings.NewReader(testVideoRuleDocument("muse-video", 7, "openai_video", tc.variant)))
		require.ErrorContains(t, err, tc.message)
	}
}

func TestVideoCapabilitiesRejectUnverifiedClaims(t *testing.T) {
	const validMuse = `{"id":"video_multi_ref","inputs":[{"slot":"prompt"},{"slot":"reference_images","max":2}]}`
	museFixture := testVideoRuleDocument("muse-video", 7, "openai_video", validMuse)
	for _, tc := range []struct {
		name      string
		document  string
		errorText string
	}{
		{"empty channel decision", testVideoRuleDocument("muse-video", 7, "openai_video", ""), "at least one operation"},
		{"wrong profile", testVideoRuleDocument("muse-video", 7, "apimart_video", validMuse), "identify the pilot route"},
		{"unverified link credentials", strings.Replace(museFixture, "https://example.com/video/", "https://test@example.com/", 1), "base_url must be"},
		{"query string", strings.Replace(museFixture, "https://example.com/video/", "https://example.com/?", 1), "base_url must be"},
		{"unknown field", strings.Replace(museFixture, `"max":2`, `"maximum":2`, 1), "unrecognized field"},
		{"slot can exceed static cap", strings.Replace(museFixture, `"max":2`, `"max":11`, 1), "invalid upper count"},
		{"first item has a different role", strings.Replace(museFixture, `"max":2`, `"max":2,"firstItemRole":"last_frame"`, 1), "media semantics"},
		{"line MIME restriction cannot be enforced at selection", strings.Replace(museFixture, `"max":2`, `"max":2,"allowedMimes":["image/jpeg"]`, 1), "media semantics"},
		{"incompatible operation meaning", strings.Replace(museFixture, `"id":"video_multi_ref"`, `"id":"video_multi_ref","modeKind":"first_frame"`, 1), "conflicts with its model contract"},
		{"redundant format override", strings.Replace(museFixture, `"id":"video_multi_ref"`, `"id":"video_multi_ref","output":{"formats":["mp4"]}`, 1), "unrecognized field"},
		{"reference variant accepts zero assets", testVideoRuleDocument("seedance-2-5", 7, "apimart_video", `{"id":"video_multi_ref"}`), "requires a reference slot"},
		{"unknown pilot model", `{"models":{"wan-3.0":[]}}`, "outside the pilot"},
		{"duplicate aliases", `{"models":{"seedance-2-5":[],"seedance-2.5":[]}}`, "one channel array"},
		{"null is not an empty array", `{"models":{"seedance-2-5":null}}`, "one channel array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadVideoCapabilities(strings.NewReader(tc.document))
			require.ErrorContains(t, err, tc.errorText)
		})
	}
}
