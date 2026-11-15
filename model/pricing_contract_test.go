package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fork: OmniMux — guards the contract field and the catalog publication gate
// on the public pricing row.

func TestPricingRowPublishesTheContractWhenPresent(t *testing.T) {
	contract, ok := modelcontracts.For("seedance-2-5")
	require.True(t, ok, "seedance-2-5 has no shipped contract")

	body, err := common.Marshal(Pricing{ModelName: "seedance-2-5", Contract: &contract})
	require.NoError(t, err)

	serialized := string(body)
	assert.Contains(t, serialized, `"contract"`)
	assert.Contains(t, serialized, `"model_id":"seedance-2-5"`)
	assert.Contains(t, serialized, `"durations_seconds"`)
}

func TestPricingRowWithoutAContractIsUnchanged(t *testing.T) {
	body, err := common.Marshal(Pricing{ModelName: "no-such-model"})
	require.NoError(t, err)
	assert.NotContains(t, string(body), "contract")
}

func prepareCatalogPricingFixtures(t *testing.T) {
	t.Helper()

	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 801, constant.ChannelTypeOpenAI, dto.ChannelOtherSettings{})
	for _, modelName := range []string{"seedance-2-5", "kling-v3", "muse-chat"} {
		insertPricingEndpointAbility(t, 801, modelName)
	}

	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
	})
}

func TestCatalogPricingRequiresLiveExecutableContract(t *testing.T) {
	prepareCatalogPricingFixtures(t)

	served := pricingByModel(GetPricing())
	require.Contains(t, served, "seedance-2-5")
	require.Contains(t, served, "kling-v3")
	require.Contains(t, served, "muse-chat")

	published := pricingByModel(GetCatalogPricing())
	require.Contains(t, published, "seedance-2-5")
	require.NotContains(t, published, "kling-v3")
	require.NotContains(t, published, "muse-chat")

	assert.Equal(t, []string{"kling-v3", "muse-chat"}, GetCatalogGaps())
}

func TestCatalogPricingKeepsLegacyBehaviorWhenGateDisabled(t *testing.T) {
	t.Run("self use mode", func(t *testing.T) {
		prepareCatalogPricingFixtures(t)
		operation_setting.SelfUseModeEnabled = true

		published := pricingByModel(GetCatalogPricing())
		require.Contains(t, published, "seedance-2-5")
		require.Contains(t, published, "kling-v3")
		require.Contains(t, published, "muse-chat")
		assert.Empty(t, GetCatalogGaps())
	})

	t.Run("env escape hatch", func(t *testing.T) {
		prepareCatalogPricingFixtures(t)
		t.Setenv("OMNIMUX_MODEL_CATALOG_ALLOW_UNCONTRACTED", "1")

		published := pricingByModel(GetCatalogPricing())
		require.Contains(t, published, "seedance-2-5")
		require.Contains(t, published, "kling-v3")
		require.Contains(t, published, "muse-chat")
		assert.Empty(t, GetCatalogGaps())
	})
}
