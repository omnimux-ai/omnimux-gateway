package advancedcustom

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorXDirectTimelineContainerKeepsExistingRowRules(t *testing.T) {
	for _, scenario := range []struct {
		name, body string
		count      int
		failure    string
	}{
		{"parent and node identity", `{"data":{"timeline":[{"id":"fixture-parent","node":{"id":"fixture-child"}},{"node":{"tweet_id":"fixture-node"}},null,{}]}}`, 2, ""},
		{"existing ID only minimum", `{"data":{"timeline":[{"id":"fixture-id"},{"rest_id":0}]}}`, 2, ""},
		{"known empty collection", `{"data":{"timeline":[]}}`, 0, ""},
		{"unobserved v2 array remains unsupported", `{"data":{"timeline_v2":[{"id":"fixture-id"}]}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"nonempty no existing identity", `{"data":{"timeline":[{},null,false,"fixture-text",{"title":"fixture-title"}]}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"no new legacy identity", `{"data":{"timeline":[{"legacy":{"id":"fixture-id"}}]}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"no deeper node", `{"data":{"timeline":[{"node":{"node":{"id":"fixture-id"}}}]}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"object is not direct array", `{"data":{"timeline":{"items":[{"id":"fixture-id"}]}}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"nested data path is not direct", `{"data":{"data":{"timeline":[{"id":"fixture-id"}]}}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"nested user path is not direct", `{"data":{"user":{"timeline":[{"id":"fixture-id"}]}}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"bare array unchanged", `[{"id":"fixture-id"}]`, 0, "account monitoring upstream posts are unrecognized"},
		{"enveloped bare array unchanged", `{"data":[{"id":"fixture-id"}]}`, 0, "account monitoring upstream posts are unrecognized"},
		{"old empty collection wins", `{"data":{"items":[],"timeline":[{"id":"fixture-id"}]}}`, 0, ""},
		{"old valid collection wins", `{"data":{"items":[{"id":"fixture-old"}],"timeline":[{"id":"fixture-new-a"},{"id":"fixture-new-b"}]}}`, 1, ""},
		{"old invalid collection cannot fall back", `{"data":{"items":[{}],"timeline":[{"id":"fixture-id"}]}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"old invalid object cannot fall back", `{"data":{"items":{"unknown":"fixture"},"timeline":[{"id":"fixture-id"}]}}`, 0, "account monitoring upstream posts are unrecognized"},
		{"envelope failure wins", `{"success":false,"data":{"timeline":[{"id":"fixture-id"}]}}`, 0, "account monitoring upstream reported a failure"},
		{"nested envelope failure wins", `{"data":{"result":{"code":401},"timeline":[{"id":"fixture-id"}]}}`, 0, "account monitoring upstream reported a failure"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			summary, err := ValidateAccountMonitorResponseForModel("x-posts", "tikhub-posts", []byte(scenario.body))
			if scenario.failure != "" {
				require.EqualError(t, err, scenario.failure)
				assert.Equal(t, AccountMonitorResponseSummary{}, summary)
				return
			}
			require.NoError(t, err)
			assert.False(t, summary.ProfileRecognized)
			assert.Equal(t, scenario.count, summary.PostCount)
		})
	}
}

func TestAccountMonitorXTimelineSchemaThreeDoesNotSelectUnobservedV2Array(t *testing.T) {
	c, recorder := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "x-posts"))
	adaptor, info := monitorResponseAdaptor(t, "x-posts", "tikhub-posts")
	usage, apiErr := adaptor.DoResponse(c, shapeResponse(`{"data":{"timeline_v2":[{"id":"private-v2-sentinel"}]}}`), info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.Empty(t, recorder.Body.String())
	evidence := GetAccountMonitorShapeEvidence(c)
	require.NotNil(t, evidence)
	assert.Equal(t, 3, evidence.SchemaVersion)
	require.Len(t, evidence.Slots, 32)
	assert.Equal(t, "missing", evidence.Slots[15].Type)
	assert.Equal(t, "array", evidence.Slots[16].Type)
	assert.Equal(t, "missing", evidence.Slots[23].Type)
	assert.Equal(t, "missing", evidence.Slots[24].Type)
	encoded, err := common.Marshal(evidence)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "private-v2-sentinel")
}

func TestAccountMonitorXDirectTimelineContainerIsModelScoped(t *testing.T) {
	const body = `{"data":{"timeline":[{"id":"fixture-id"}]}}`
	_, err := ValidateAccountMonitorResponse("tikhub-posts", []byte(body))
	require.EqualError(t, err, "account monitoring upstream posts are unrecognized")
	for _, model := range []string{"", "tiktok-posts", "x-user", "tiktok-user", "other", "x-posts "} {
		summary, err := ValidateAccountMonitorResponseForModel(model, "tikhub-posts", []byte(body))
		require.EqualError(t, err, "account monitoring upstream posts are unrecognized", model)
		assert.Equal(t, AccountMonitorResponseSummary{}, summary)
	}
	_, err = ValidateAccountMonitorResponseForModel("x-posts", "tikhub-user", []byte(body))
	require.EqualError(t, err, "account monitoring upstream profile is unrecognized")
}

func TestAccountMonitorXTimelineFallbackChecksOnlyConsumerEnvelopeDepth(t *testing.T) {
	for _, scenario := range []struct {
		name, model, body, failure string
		count                      int
	}{
		{"user success false", "x-posts", `{"data":{"user":{"success":false},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"user explicit error", "x-posts", `{"data":{"user":{"error":"denied"},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"channel rejected code", "x-posts", `{"data":{"channel":{"code":401},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"author not ok", "x-posts", `{"data":{"author":{"ok":false},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"normal user envelope", "x-posts", `{"data":{"user":{"success":true,"nickname":"Fixture"},"timeline":[{"id":"fixture-id"}]}}`, "", 1},
		{"last checked result layer", "x-posts", `{"data":{"user":{"result":{"success":false}},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"last checked data layer", "x-posts", `{"data":{"user":{"data":{"success":false}},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"last checked user layer", "x-posts", `{"data":{"author":{"user":{"success":false}},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"last checked channel layer", "x-posts", `{"data":{"user":{"channel":{"success":false}},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"last checked author layer", "x-posts", `{"data":{"channel":{"author":{"ok":false}},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream reported a failure", 0},
		{"author beyond consumer depth is unchanged", "x-posts", `{"data":{"user":{"result":{"author":{"ok":false}}},"timeline":[{"id":"fixture-id"}]}}`, "", 1},
		{"data beyond consumer depth is unchanged", "x-posts", `{"data":{"user":{"result":{"data":{"success":false}}},"timeline":[{"id":"fixture-id"}]}}`, "", 1},
		{"unknown wrapper is not traversed", "x-posts", `{"data":{"unknown":{"user":{"success":false}},"timeline":[{"id":"fixture-id"}]}}`, "", 1},
		{"envelope array is not traversed", "x-posts", `{"data":{"user":[{"success":false}],"timeline":[{"id":"fixture-id"}]}}`, "", 1},
		{"row status is not an envelope", "x-posts", `{"data":{"timeline":[{"id":"fixture-id","status":401,"error":"denied"}]}}`, "", 1},
		{"old valid collection unchanged", "x-posts", `{"data":{"items":[{"id":"fixture-old"}],"user":{"success":false},"timeline":[{"id":"fixture-new"}]}}`, "", 1},
		{"old empty collection unchanged", "x-posts", `{"data":{"items":[],"user":{"success":false},"timeline":[{"id":"fixture-new"}]}}`, "", 0},
		{"old invalid collection unchanged", "x-posts", `{"data":{"items":[{}],"user":{"success":false},"timeline":[{"id":"fixture-new"}]}}`, "account monitoring upstream posts are unrecognized", 0},
		{"unselected timeline object unchanged", "x-posts", `{"data":{"user":{"success":false},"timeline":{"id":"fixture-id"}}}`, "account monitoring upstream posts are unrecognized", 0},
		{"other model unchanged", "tiktok-posts", `{"data":{"user":{"success":false},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream posts are unrecognized", 0},
		{"model-less unchanged", "", `{"data":{"user":{"success":false},"timeline":[{"id":"fixture-id"}]}}`, "account monitoring upstream posts are unrecognized", 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			summary, err := ValidateAccountMonitorResponseForModel(scenario.model, "tikhub-posts", []byte(scenario.body))
			if scenario.failure != "" {
				require.EqualError(t, err, scenario.failure)
				assert.Equal(t, AccountMonitorResponseSummary{}, summary)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, scenario.count, summary.PostCount)
		})
	}
}

func TestAccountMonitorXTimelineSchemaThreeProjectsOnlyExistingFirstRowSlots(t *testing.T) {
	const sentinel = "private-timeline-sentinel"
	for _, scenario := range []struct {
		name, body, firstType, rowIDType string
	}{
		{"observed direct array", `{"data":{"timeline":[{"id":null,"post_id":[],"tweet_id":{},"rest_id":false,"node":{"id":null,"rest_id":[]},"private-timeline-sentinel":"private-timeline-sentinel"},{"id":false,"rest_id":{}}]}}`, "object", "null"},
		{"old tweets candidate wins", `{"data":{"tweets":[null],"timeline":[{"id":{}}]}}`, "null", "missing"},
		{"last old candidate wins", `{"data":{"result":{"tweets":[null]},"timeline":[{"id":{}}]}}`, "null", "missing"},
		{"old empty candidate wins", `{"data":{"items":[],"posts":[{}],"timeline":[{"id":{}}]}}`, "missing", "missing"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			c, recorder := shapeContext()
			require.True(t, EnableAccountMonitorShapeDiagnostic(c, "x-posts"))
			adaptor, info := monitorResponseAdaptor(t, "x-posts", "tikhub-posts")
			usage, apiErr := adaptor.DoResponse(c, shapeResponse(scenario.body), info)
			require.NotNil(t, apiErr)
			assert.Nil(t, usage)
			assert.Empty(t, recorder.Body.String())
			evidence := GetAccountMonitorShapeEvidence(c)
			require.NotNil(t, evidence)
			assert.Equal(t, 3, evidence.SchemaVersion)
			require.Len(t, evidence.Slots, 32)
			assert.Equal(t, "array", evidence.Slots[15].Type)
			assert.Equal(t, "array", evidence.Slots[23].Type)
			assert.Equal(t, scenario.firstType, evidence.Slots[24].Type)
			assert.Equal(t, scenario.rowIDType, evidence.Slots[26].Type)
			if scenario.name == "observed direct array" {
				for i, kind := range []string{"object", "null", "array", "object", "boolean", "null", "array"} {
					assert.Equal(t, kind, evidence.Slots[25+i].Type)
				}
			}
			encoded, err := common.Marshal(evidence)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), sentinel)
			assert.NotContains(t, string(encoded), "timeline")
			assert.LessOrEqual(t, len(encoded), accountMonitorShapeMaxBytes)
		})
	}
}
