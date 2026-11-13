package advancedcustom

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountMonitorXRootDiagnosticSchemaThree(t *testing.T) {
	for _, tt := range []struct {
		name, body                        string
		sourceType, firstType, rowIDType  string
		nodeType, nodeIDType, tweetIDType string
	}{
		{"tweets hypothesis not success", `{"data":{"tweets":[{"tweet_id":"private-x-shape-sentinel","legacy":{"full_text":"private-x-shape-sentinel"}}],"private-x-shape-sentinel":{"nested":"private-x-shape-sentinel"}}}`, "array", "object", "missing", "missing", "missing", "string"},
		{"first item only", `{"data":{"tweets":[null,{"id":"private-x-shape-sentinel","tweet_id":"private-x-shape-sentinel"}]}}`, "array", "null", "missing", "missing", "missing", "missing"},
		{"data tweets", `{"data":{"data":{"tweets":[{"node":{"id":"private-x-shape-sentinel"}}]}}}`, "array", "object", "missing", "object", "string", "missing"},
		{"first array priority", `{"data":{"items":[null],"tweets":[{"id":"private-x-shape-sentinel"}]}}`, "array", "null", "missing", "missing", "missing", "missing"},
		{"first empty array priority", `{"data":{"items":[],"posts":[{}],"tweets":[{"id":"private-x-shape-sentinel"}]}}`, "array", "missing", "missing", "missing", "missing", "missing"},
		{"wrong root type not selected", `{"data":{"tweets":{"id":"private-x-shape-sentinel"}}}`, "missing", "missing", "missing", "missing", "missing", "missing"},
		{"unknown wrapper not searched", `{"data":{"private-x-shape-sentinel":{"tweets":[{"id":"private-x-shape-sentinel"}]}}}`, "missing", "missing", "missing", "missing", "missing", "missing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := shapeContext()
			require.True(t, EnableAccountMonitorShapeDiagnostic(c, "x-posts"))
			adaptor, info := monitorResponseAdaptor(t, "x-posts", "tikhub-posts")
			usage, apiErr := adaptor.DoResponse(c, shapeResponse(tt.body), info)
			require.NotNil(t, apiErr)
			assert.Nil(t, usage)
			assert.Empty(t, recorder.Body.String())
			evidence := GetAccountMonitorShapeEvidence(c)
			require.NotNil(t, evidence)
			require.Equal(t, 3, evidence.SchemaVersion)
			require.Len(t, evidence.Slots, 32)
			assert.Equal(t, tt.sourceType, evidence.Slots[23].Type)
			assert.Equal(t, tt.firstType, evidence.Slots[24].Type)
			assert.Equal(t, tt.nodeType, evidence.Slots[25].Type)
			assert.Equal(t, tt.rowIDType, evidence.Slots[26].Type)
			assert.Equal(t, tt.tweetIDType, evidence.Slots[28].Type)
			assert.Equal(t, tt.nodeIDType, evidence.Slots[30].Type)
			encoded, err := common.Marshal(evidence)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), "private-x-shape-sentinel")
			assert.LessOrEqual(t, len(encoded), accountMonitorShapeMaxBytes)
		})
	}
}

func TestAccountMonitorXSchemaThreeRejectsOldVersionAndWrongSlotCount(t *testing.T) {
	for _, tt := range []struct{ version, slots int }{{1, 27}, {1, 32}, {2, 32}, {3, 31}, {3, 33}, {4, 32}} {
		evidence := &AccountMonitorShapeEvidence{SchemaVersion: tt.version, Model: "x-posts", Status: accountMonitorShapeObserved}
		for i := 0; i < tt.slots; i++ {
			evidence.Slots = append(evidence.Slots, AccountMonitorShapeSlot{Type: "missing"})
		}
		actual := boundAccountMonitorShapeEvidence(evidence)
		require.NotNil(t, actual)
		assert.Equal(t, 3, actual.SchemaVersion)
		assert.Equal(t, accountMonitorShapeLimit, actual.Status)
		assert.Empty(t, actual.Slots)
	}
}

func TestAccountMonitorXRootSlotsStayVersionedAndFixed(t *testing.T) {
	const body = `{"data":{"items":null,"posts":12,"edges":false,"data":{"user":{"result":{"unknown":"private-x-root-sentinel"}},"tweets":[]},"result":{"tweets":null},"list":"private-x-root-sentinel","media":{"unknown":"private-x-root-sentinel"},"user":{"result":{"timeline_v2":{"timeline":{"unknown":"private-x-root-sentinel"}},"timeline":{"unknown":"private-x-root-sentinel"}}},"timeline":{"unknown":"private-x-root-sentinel"},"timeline_v2":{"timeline":{"unknown":"private-x-root-sentinel"}},"tweets":"private-x-root-sentinel"}}`
	c, recorder := shapeContext()
	require.True(t, EnableAccountMonitorShapeDiagnostic(c, "x-posts"))
	adaptor, info := monitorResponseAdaptor(t, "x-posts", "tikhub-posts")
	usage, apiErr := adaptor.DoResponse(c, shapeResponse(body), info)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.Empty(t, recorder.Body.String())
	evidence := GetAccountMonitorShapeEvidence(c)
	require.NotNil(t, evidence)
	assert.Equal(t, 3, evidence.SchemaVersion)
	require.Len(t, evidence.Slots, 32)
	for i, kind := range []string{
		"null", "number", "boolean", "object", "object", "string", "object", "object", "object",
		"object", "object", "object", "object", "object", "object", "object", "object", "string", "array", "null",
	} {
		assert.Equal(t, kind, evidence.Slots[3+i].Type, "schema 3 root slot %d", 3+i)
	}
	assert.Equal(t, "array", evidence.Slots[23].Type)
	require.NotNil(t, evidence.Slots[23].Nonempty)
	assert.False(t, *evidence.Slots[23].Nonempty)
	assert.Equal(t, "missing", evidence.Slots[24].Type)
	data, err := common.Marshal(evidence)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "private-x-root-sentinel")
	assert.NotContains(t, string(data), "unknown")
}
