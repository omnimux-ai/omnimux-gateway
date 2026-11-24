package modelcontracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryShippedSpecHasValidCanonicalBrand(t *testing.T) {
	fresh, err := Build(specDir)
	require.NoError(t, err)
	shipped := All()

	require.Equal(t, fresh.Brands, shipped.Brands, "shipped brands map drifted from specs; regenerate with `go run ./modelcontracts/gen`")
	require.NotEmpty(t, shipped.Brands)

	// Gate shipped specs against accidental vendor spelling drift: every real
	// governance spec must either declare an explicit brand_id or match
	// vendorBrandTable.
	entries, err := os.ReadDir(specDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(specDir, entry.Name()))
		require.NoError(t, err)
		var spec specFile
		require.NoError(t, common.Unmarshal(raw, &spec))
		if strings.TrimSpace(spec.BrandID) == "" {
			assert.True(t, IsMappedVendor(spec.Vendor), "shipped spec %s uses unmapped vendor %q; register in modelcontracts/brands.go or declare explicit brand_id", entry.Name(), spec.Vendor)
		}
	}

	for id, contract := range shipped.Models {
		require.NotEmpty(t, contract.BrandID, "model %s must resolve a canonical brand_id", id)
		require.NotEmpty(t, contract.BrandName, "model %s must resolve a canonical brand_name", id)
		require.NoError(t, ValidateBrandID(contract.BrandID), "model %s has invalid brand_id", id)

		meta, ok := shipped.Brands[contract.BrandID]
		require.True(t, ok, "brand %q referenced by %s missing from document brands map", contract.BrandID, id)
		assert.Equal(t, contract.BrandID, meta.ID)
		assert.Equal(t, contract.BrandName, meta.Name)
		assert.NoError(t, ValidateBrandIconPath(meta.IconPath))
	}

	assert.Equal(t, "openai", shipped.Models["gpt-image-2.5"].BrandID)
	assert.Equal(t, "OpenAI", shipped.Models["gpt-image-2.5"].BrandName)
	assert.Equal(t, "bytedance", shipped.Models["seedream-5-5"].BrandID)
	assert.Equal(t, "ByteDance", shipped.Models["seedream-5-5"].BrandName)
	assert.Equal(t, "bytedance", shipped.Models["seedance-2-5"].BrandID)
	assert.Equal(t, "google", shipped.Models["nano-banana-2"].BrandID)
	assert.Equal(t, "muse", shipped.Models["muse-image"].BrandID)
	assert.Equal(t, "indextts", shipped.Models["indextts-2"].BrandID)
	assert.Equal(t, "Index TTS2", shipped.Models["indextts-2"].BrandName)
	assert.Equal(t, "grok", shipped.Models["grok-4.6"].BrandID)
	assert.Equal(t, "xAI", shipped.Models["grok-4.6"].BrandName)
	assert.Equal(t, "typesafe-ai", shipped.Models["jev"].BrandID)
	assert.Equal(t, "TypeSafe AI", shipped.Models["jev"].BrandName)
}

func TestBrandFieldsDoNotChangeContractVersion(t *testing.T) {
	base := Contract{
		ModelID:       "probe-model",
		SchemaVersion: SchemaVersion,
		Family:        "image",
		Operations:    []Operation{},
	}
	v0, err := contractVersion(base)
	require.NoError(t, err)

	withBrand := base
	withBrand.BrandID = "acme-studio"
	withBrand.BrandName = "Acme Studio"
	v1, err := contractVersion(withBrand)
	require.NoError(t, err)
	assert.Equal(t, v0, v1, "adding brand_id/brand_name must not move capability ContractVersion")

	renamed := withBrand
	renamed.BrandName = "Acme AI Labs"
	v2, err := contractVersion(renamed)
	require.NoError(t, err)
	assert.Equal(t, v0, v2, "renaming brand_name must not move capability ContractVersion")
}

func TestValidateBrandIDAndIconPath(t *testing.T) {
	assert.NoError(t, ValidateBrandID("openai"))
	assert.NoError(t, ValidateBrandID("typesafe-ai"))
	assert.NoError(t, ValidateBrandID("x"))

	for _, bad := range []string{"", "OpenAI", "foo_bar", "foo bar", "-leading", "trailing-", "a--b", strings.Repeat("a", 65)} {
		assert.Error(t, ValidateBrandID(bad), "expected invalid brand_id: %q", bad)
	}

	assert.NoError(t, ValidateBrandIconPath(""))
	assert.NoError(t, ValidateBrandIconPath("M12 2L2 22h20L12 2z"))
	assert.NoError(t, ValidateBrandIconPath("m3.5,4.25 a6 6 0 1 0 12 0 c0-3-2.5-5-6-5z"))

	malicious := []string{
		`<svg><path d="M0 0h24v24H0z"/></svg>`,
		`M0 0"><script>alert(1)</script>`,
		`https://evil.example/logo.svg`,
		`javascript:alert(1)`,
		`url(https://evil.example/a.svg)`,
		`data:image/svg+xml;base64,AAAA`,
		`L10 10 Z`,
		"M0 0 " + strings.Repeat("L1 1 ", MaxBrandIconPathBytes),
	}
	for _, bad := range malicious {
		assert.Error(t, ValidateBrandIconPath(bad), "expected rejected icon_path: %q", bad)
	}
}

func TestBuildValidatesBrandMappingAndOptionalIconPath(t *testing.T) {
	t.Run("explicit brand with safe icon_path projects into document brands", func(t *testing.T) {
		doc := buildSpecDir(t, map[string]string{
			"acme-1.json": `{
				"model_id": "acme-1",
				"family": "image",
				"vendor": "Acme Labs",
				"brand_id": "acme-labs",
				"brand_name": "Acme Labs",
				"brand_icon_path": "M12 2L22 20H2L12 2Z",
				"operations": ["text_to_image"]
			}`,
		})
		contract := doc.Models["acme-1"]
		assert.Equal(t, "acme-labs", contract.BrandID)
		assert.Equal(t, "Acme Labs", contract.BrandName)
		assert.Equal(t, BrandMeta{
			ID:       "acme-labs",
			Name:     "Acme Labs",
			IconPath: "M12 2L22 20H2L12 2Z",
		}, doc.Brands["acme-labs"])
	})

	t.Run("runtime ASCII vendor without brand_id derives deterministic kebab-case slug", func(t *testing.T) {
		doc := buildSpecDir(t, map[string]string{
			"nova-1.json": `{"model_id":"nova-1","family":"llm","vendor":"Nova Labs"}`,
		})
		assert.Equal(t, "nova-labs", doc.Models["nova-1"].BrandID)
		assert.Equal(t, "Nova Labs", doc.Models["nova-1"].BrandName)
		assert.Equal(t, BrandMeta{ID: "nova-labs", Name: "Nova Labs"}, doc.Brands["nova-labs"])
	})

	t.Run("unmapped non-ASCII vendor without brand_id fails build", func(t *testing.T) {
		_, err := buildSpecDirResult(t, map[string]string{
			"unknown-1.json": `{"model_id":"unknown-1","family":"llm","vendor":"未知厂商"}`,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmapped vendor")
	})

	t.Run("conflicting brand_name for same brand_id fails build", func(t *testing.T) {
		_, err := buildSpecDirResult(t, map[string]string{
			"a.json": `{"model_id":"a","family":"llm","brand_id":"acme","brand_name":"Acme One"}`,
			"b.json": `{"model_id":"b","family":"llm","brand_id":"acme","brand_name":"Acme Two"}`,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting brand_name")
	})

	t.Run("unsafe brand_icon_path fails build", func(t *testing.T) {
		_, err := buildSpecDirResult(t, map[string]string{
			"bad.json": `{"model_id":"bad","family":"llm","brand_id":"acme","brand_name":"Acme","brand_icon_path":"<svg></svg>"}`,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "brand_icon_path")
	})
}
