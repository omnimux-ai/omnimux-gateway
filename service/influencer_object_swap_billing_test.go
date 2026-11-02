package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectSwapUnderstandingTraceRemainsAdminOnly(t *testing.T) {
	plan := InfluencerObjectSwapPlan{
		Prompt: "private final editing instructions",
		Audit:  []byte(`{"target":"private observed material"}`),
		Trace: InfluencerObjectSwapPlanTrace{
			Model: "understanding-model", Group: "supplier-group", ChannelID: 42,
			RequestID: "supplier-private-id", Outcome: "received",
		},
	}
	details, err := common.Marshal(plan)
	require.NoError(t, err)
	task := &model.Task{TaskID: "public-task", Quota: 123}
	task.PrivateData.ObjectSwapPreparation = &model.GenerationObjectSwapPreparation{State: model.GenerationObjectSwapReady, Prompt: plan.Prompt, Details: details}
	other := taskBillingOther(task).Snapshot()
	assert.Equal(t, "public-task", other["task_id"])
	assert.NotContains(t, other, "object_swap_understanding")
	admin, ok := other["admin_info"].(map[string]any)
	require.True(t, ok)
	understanding, ok := admin["object_swap_understanding"].(map[string]any)
	require.True(t, ok)
	trace, ok := understanding["trace"].(InfluencerObjectSwapPlanTrace)
	require.True(t, ok)
	assert.Equal(t, plan.Trace, trace)
	assert.Nil(t, trace.Usage, "missing provider usage must remain unknown")
	assert.Equal(t, 123, task.Quota, "helper observation must not add a user charge")
	raw, err := common.Marshal(other)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), plan.Prompt)
	assert.NotContains(t, string(raw), "private observed material")
}
