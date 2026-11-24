package model

import (
	"os"
	"path/filepath"
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

	body, err := common.Marshal(Pricing{
		ModelName: "seedance-2-5",
		BrandID:   contract.BrandID,
		BrandName: contract.BrandName,
		Contract:  &contract,
	})
	require.NoError(t, err)

	serialized := string(body)
	assert.Contains(t, serialized, `"contract"`)
	assert.Contains(t, serialized, `"model_id":"seedance-2-5"`)
	assert.Contains(t, serialized, `"brand_id":"bytedance"`)
	assert.Contains(t, serialized, `"brand_name":"ByteDance"`)
	assert.Contains(t, serialized, `"durations_seconds"`)
}

func TestPricingRowWithoutAContractIsUnchanged(t *testing.T) {
	body, err := common.Marshal(Pricing{ModelName: "no-such-model"})
	require.NoError(t, err)
	assert.NotContains(t, string(body), "contract")
	assert.NotContains(t, string(body), "brand_id")
	assert.NotContains(t, string(body), "brand_name")
}

func TestEffectiveVideoVersionIgnoresBrandDisplayChanges(t *testing.T) {
	buildVideoContract := func(brandID, brandName, iconPath string) modelcontracts.Contract {
		dir := t.TempDir()
		raw := `{
			"model_id": "probe-video",
			"family": "video",
			"status": "documented",
			"vendor": "` + brandName + `",
			"brand_id": "` + brandID + `",
			"brand_name": "` + brandName + `",
			"brand_icon_path": "` + iconPath + `",
			"gateway": {
				"profile": "apimart_video",
				"request_model": "seedance-2-5",
				"operations": ["video_multi_ref"],
				"mode_kinds": {"video_multi_ref": "image_reference"}
			},
			"reference_images": {"min": 1, "max": 6},
			"durations_seconds": {"min": 4, "max": 30},
			"resolutions": ["480p", "720p"],
			"aspect_ratios": ["16:9", "9:16"],
			"output_formats": ["mp4"]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "probe-video.json"), []byte(raw), 0o600))
		doc, err := modelcontracts.Build(dir)
		require.NoError(t, err)
		c, ok := doc.Models["probe-video"]
		require.True(t, ok)
		return c
	}

	c0 := buildVideoContract("acme", "Acme", "")
	c1 := buildVideoContract("acme-labs", "Acme Labs", "M12 2L22 20H2Z")
	require.Equal(t, c0.ContractVersion, c1.ContractVersion, "brand display changes must not alter ContractVersion")

	p0 := Pricing{
		ModelName: "probe-video",
		BrandID:   c0.BrandID,
		BrandName: c0.BrandName,
		Contract:  &c0,
		videoOperationsByGroup: map[string][]modelcontracts.Operation{
			"default": c0.Operations,
		},
	}
	p1 := Pricing{
		ModelName: "probe-video",
		BrandID:   c1.BrandID,
		BrandName: c1.BrandName,
		Contract:  &c1,
		videoOperationsByGroup: map[string][]modelcontracts.Operation{
			"default": c1.Operations,
		},
	}
	v0 := p0.VideoCapabilitiesForGroups(map[string]float64{"default": 1})
	v1 := p1.VideoCapabilitiesForGroups(map[string]float64{"default": 1})
	require.NotNil(t, v0)
	require.NotNil(t, v1)
	require.NotEmpty(t, v0.Version)
	assert.Equal(t, v0.Version, v1.Version, "brand display changes must not alter effective_video.version")
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
