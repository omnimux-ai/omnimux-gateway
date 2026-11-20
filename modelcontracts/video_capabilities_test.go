package modelcontracts

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
	assert.True(t, rules.HasProtocol("muse-video", VideoProtocolOpenAI))
	assert.False(t, rules.HasProtocol("muse-video", VideoProtocolGenerations))
	assert.True(t, rules.UsesChannel(7))
	assert.False(t, rules.UsesChannel(8))
	assert.Equal(t, []int{7}, rules.ChannelIDs())

	identity, found := rules.ChannelIdentity("muse-video", 7)
	require.True(t, found)
	assert.Equal(t, VideoChannelIdentity{
		ChannelType: constant.ChannelTypeOpenAI, Profile: "openai_video", UpstreamModel: "muse-video",
		BaseURL: "https://example.com/video", Protocol: VideoProtocolOpenAI,
	}, identity)

	rule, found := rules.ChannelRule("muse-video", 7)
	require.True(t, found)
	require.Len(t, rule.Operations, 1)
	operation := rule.Operations[0]
	assert.Empty(t, operation.GatewayProtocol, "same-model protocol inherits the model contract")
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
	assert.False(t, empty.HasProtocol("seedance-2-5", VideoProtocolGenerations))
	assert.Empty(t, empty.NativeRequestModel("seedance-2-5"))
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
	assert.Equal(t, "seedance-2-5", rules.NativeRequestModel("seedance-2.5"))
	assert.Equal(t, VideoProtocolGenerations, rule.Operations[0].GatewayProtocol)
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
		{"missing native decoder", testVideoRuleDocument("muse-video", 7, "apimart_video", validMuse), "request_model must name a verified native request dialect"},
		{"unverified link credentials", strings.Replace(museFixture, "https://example.com/video/", "https://test@example.com/", 1), "base_url must be"},
		{"query string", strings.Replace(museFixture, "https://example.com/video/", "https://example.com/?", 1), "base_url must be"},
		{"unknown field", strings.Replace(museFixture, `"max":2`, `"maximum":2`, 1), "unrecognized field"},
		{"slot can exceed static cap", strings.Replace(museFixture, `"max":2`, `"max":11`, 1), "invalid upper count"},
		{"first item has a different role", strings.Replace(museFixture, `"max":2`, `"max":2,"firstItemRole":"last_frame"`, 1), "media semantics"},
		{"line MIME restriction cannot be enforced at selection", strings.Replace(museFixture, `"max":2`, `"max":2,"allowedMimes":["image/jpeg"]`, 1), "media semantics"},
		{"incompatible operation meaning", strings.Replace(museFixture, `"id":"video_multi_ref"`, `"id":"video_multi_ref","modeKind":"first_frame"`, 1), "conflicts with its model contract"},
		{"redundant format override", strings.Replace(museFixture, `"id":"video_multi_ref"`, `"id":"video_multi_ref","output":{"formats":["mp4"]}`, 1), "unrecognized field"},
		{"reference variant accepts zero assets", testVideoRuleDocument("seedance-2-5", 7, "apimart_video", `{"id":"video_multi_ref"}`), "requires a reference slot"},
		{"unsupported model protocol", `{"models":{"wan-3.0":[]}}`, "no verified video protocol"},
		{"non-video model", `{"models":{"gemini-3-flash":[]}}`, "no live video contract"},
		{"stub model", `{"models":{"kling-v3":[]}}`, "no live video contract"},
		{"unknown model", `{"models":{"unknown-video":[]}}`, "no live video contract"},
		{"unsupported channel profile", testVideoRuleDocument("muse-video", 7, "unknown", validMuse), "identify a verified video route"},
		{"duplicate aliases", `{"models":{"seedance-2-5":[],"seedance-2.5":[]}}`, "one channel array"},
		{"null is not an empty array", `{"models":{"seedance-2-5":null}}`, "one channel array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadVideoCapabilities(strings.NewReader(tc.document))
			require.ErrorContains(t, err, tc.errorText)
		})
	}
}

func TestVideoCapabilitiesEnrollNewModelThroughVerifiedProtocols(t *testing.T) {
	restore := All()
	t.Cleanup(func() { loaded, loadErr = restore, nil })
	const mixedID = "synthetic-mixed-video"
	const nativeID = "synthetic-native-video"
	document := buildSpecDir(t, map[string]string{
		mixedID + ".json": `{
			"model_id":"synthetic-mixed-video","family":"video","status":"documented",
			"gateway":{"profile":"openai_video","operations":["text_to_video","video_multi_ref"],
				"mode_kinds":{"text_to_video":"text_to_video","video_multi_ref":"image_reference"}},
			"reference_images":{"min":2,"max":6},"durations_seconds":{"min":4,"max":30,"allowed":[4,5,10,12,30]},
			"aspect_ratios":["16:9","9:16"],"output_formats":["mp4"]
		}`,
		nativeID + ".json": `{
			"model_id":"synthetic-native-video","family":"video","status":"documented",
			"gateway":{"profile":"apimart_video","request_model":"seedance-2-5","operations":["video_multi_ref"],
				"mode_kinds":{"video_multi_ref":"image_reference"}},
			"reference_images":{"min":1,"max":6},"durations_seconds":{"min":4,"max":30},
			"resolutions":["480p","720p"],"aspect_ratios":["16:9","9:16"],"output_formats":["mp4"]
		}`,
	})
	loaded, loadErr = *document, nil
	assert.Equal(t, VideoProtocolOpenAI, document.Models[mixedID].GatewayProtocol)
	assert.Equal(t, VideoProtocolGenerations, document.Models[nativeID].GatewayProtocol)
	assert.Contains(t, document.Models[nativeID].Operations[0].Parameters, Parameter{
		Name: "duration", Type: "integer", Required: true,
		Minimum: float64Pointer(4), Maximum: float64Pointer(30), Step: float64Pointer(1),
	})

	const mixedRules = `{"models":{"synthetic-mixed-video":[
		{"channel_id":10,"channel_type":%d,"profile":"openai_video","upstream_model":"muse-video",
			"base_url":"https://example.com/video","evidence":"openai fixture","operations":[{"id":"text_to_video"}]},
		{"channel_id":20,"channel_type":%d,"profile":"apimart_video","request_model":"seedance-2-5",
			"upstream_model":"synthetic-wire-model","base_url":"https://example.com/native",
			"evidence":"native fixture","operations":[{"id":"text_to_video","parameters":[{"name":"duration","fixed":10}]}]}
	]}}`
	config := fmt.Sprintf(mixedRules, constant.ChannelTypeOpenAI, constant.ChannelTypeApiMart)
	rules, err := ReadVideoCapabilities(strings.NewReader(config))
	require.NoError(t, err)
	assert.True(t, rules.Managed(mixedID))
	assert.True(t, rules.HasProtocol(mixedID, VideoProtocolOpenAI))
	assert.True(t, rules.HasProtocol(mixedID, VideoProtocolGenerations))
	assert.Equal(t, "seedance-2-5", rules.NativeRequestModel(mixedID))
	for id, protocol := range map[int]string{10: VideoProtocolOpenAI, 20: VideoProtocolGenerations} {
		rule, found := rules.ChannelRule(mixedID, id)
		require.True(t, found)
		require.Len(t, rule.Operations, 1)
		operation := rule.Operations[0]
		assert.Equal(t, protocol, operation.GatewayProtocol)
		encoded, err := common.Marshal(operation)
		require.NoError(t, err)
		assert.Contains(t, string(encoded), fmt.Sprintf(`"gateway_protocol":%q`, protocol))
		for _, privateField := range []string{"request_model", "upstream_model", "base_url", "evidence"} {
			assert.NotContains(t, string(encoded), privateField)
		}
	}
	request := VideoRequest{
		Operation: "text_to_video", Protocol: VideoProtocolOpenAI,
		Slots: map[string]int{"prompt": 1}, Parameters: map[string]any{"duration": 10},
	}
	assert.True(t, rules.Accepts(mixedID, 10, request))
	assert.False(t, rules.Accepts(mixedID, 20, request))
	request.Parameters["duration"] = 11
	assert.False(t, rules.Accepts(mixedID, 10, request))
	request.Parameters["duration"] = 10
	request.Protocol = VideoProtocolGenerations
	assert.False(t, rules.Accepts(mixedID, 10, request))
	assert.True(t, rules.Accepts(mixedID, 20, request))
	request.Parameters["duration"] = 12
	assert.False(t, rules.Accepts(mixedID, 20, request))
	delete(request.Parameters, "duration")
	assert.False(t, rules.Accepts(mixedID, 20, request))

	nativeRules := testVideoRuleDocument(nativeID, 30, "apimart_video", `{
		"id":"video_multi_ref",
		"inputs":[{"slot":"prompt"},{"slot":"reference_images","required":true,"max":4}],
		"parameters":[{"name":"resolution","fixed":"720p"},{"name":"duration"}]
	}`)
	rules, err = ReadVideoCapabilities(strings.NewReader(nativeRules))
	require.NoError(t, err)
	rule, found := rules.ChannelRule(nativeID, 30)
	require.True(t, found)
	require.Len(t, rule.Operations, 1)
	assert.Empty(t, rule.Operations[0].GatewayProtocol, "same-model protocol inherits the model contract")

	for _, tc := range []struct{ name, config, message string }{
		{
			"native cannot claim ordered first image",
			strings.Replace(config, `{"id":"text_to_video","parameters":[{"name":"duration","fixed":10}]}`,
				`{"id":"video_multi_ref"}`, 1),
			"incompatible media semantics",
		},
		{
			"new alias cannot claim auto billing",
			strings.Replace(config, `"fixed":10`, `"fixed":-1`, 1),
			"fixed value is outside",
		},
		{
			"native variant cannot omit duration",
			strings.Replace(config, `[{"name":"duration","fixed":10}]`, `[]`, 1),
			"requires a duration parameter",
		},
		{
			"native variant cannot default duration",
			strings.Replace(config, `"fixed":10`, `"default":10`, 1),
			"duration requires an explicit value",
		},
		{
			"unverified native dialect",
			strings.Replace(config, `"request_model":"seedance-2-5"`, `"request_model":"unknown"`, 1),
			"request_model must name a verified native request dialect",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadVideoCapabilities(strings.NewReader(tc.config))
			require.ErrorContains(t, err, tc.message)
		})
	}
}

func TestGatewaySpecRejectsUnsupportedNativeDecoder(t *testing.T) {
	_, err := buildSpecDirResult(t, map[string]string{
		"synthetic-video.json": `{
			"model_id":"synthetic-video","family":"video","status":"documented",
			"gateway":{"profile":"apimart_video","request_model":"wan-3.0","operations":["text_to_video"]}
		}`,
	})
	require.ErrorContains(t, err, "not a verified request dialect")
}

func TestGatewaySpecRejectsIncompatibleNativeOperations(t *testing.T) {
	const template = `{
		"model_id":%q,"family":"video","status":"documented",
		"gateway":{"profile":"apimart_video","request_model":"seedance-2-5","operations":[%q]}
		%s
	}`
	const nonAdaptive = `,"parameters":[{"name":"aspect_ratio","type":"enum","enum":["16:9"]}]`
	for _, tc := range []struct {
		name, model, operation, fields, message string
	}{
		{"missing duration", "synthetic-video", "text_to_video", "", "requires a duration parameter"},
		{"default duration", "synthetic-video", "text_to_video", `,"durations_seconds":{"min":4,"max":30,"default":10}`, "duration requires an explicit value"},
		{"extend ratio", "synthetic-video", "video_extend", `,"durations_seconds":{"min":4,"max":30}` + nonAdaptive, `parameter "aspect_ratio" exceeds`},
		{"legacy edit ratio", "seedance-2-5", "video_edit", nonAdaptive, `parameter "aspect_ratio" exceeds`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildSpecDirResult(t, map[string]string{
				tc.model + ".json": fmt.Sprintf(template, tc.model, tc.operation, tc.fields),
			})
			require.ErrorContains(t, err, tc.message)
		})
	}
}
