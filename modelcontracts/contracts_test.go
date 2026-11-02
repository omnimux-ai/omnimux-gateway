package modelcontracts

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specDir is read from the repository, which CI has and the runtime image does
// not. That asymmetry is the whole reason this package ships a generated copy.
const specDir = "../docs/model-governance/specs"

// sameContract compares two projections by the canonical JSON they encode,
// which is exactly what the shipped file carries. Comparing Go structs instead
// would report a difference for a declared-but-empty list, which the payload
// cannot represent distinctly anyway.
func sameContract(a, b Contract) bool {
	left, err := common.Marshal(a)
	if err != nil {
		return false
	}
	right, err := common.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(left, right)
}

func registered(t *testing.T, family string) Contract {
	t.Helper()
	models := All().Models
	keys := make([]string, 0, len(models))
	for k := range models {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		contract := models[k]
		if contract.Family == family && contract.DurationsSeconds != nil {
			return contract
		}
	}
	for _, k := range keys {
		contract := models[k]
		if contract.Family == family {
			return contract
		}
	}
	t.Fatalf("no %s contract in the shipped payload", family)
	return Contract{}
}

func TestShippedPayloadIsReadableAndNonEmpty(t *testing.T) {
	if err := LoadError(); err != nil {
		t.Fatalf("shipped payload is unreadable: %v", err)
	}
	if Count() == 0 {
		t.Fatal("shipped payload carries no contracts")
	}
}

func TestShippedPayloadMatchesGovernanceSpecs(t *testing.T) {
	fresh, err := Build(specDir)
	if err != nil {
		t.Fatalf("project %s: %v", specDir, err)
	}
	shipped := All()

	var missing, stale []string
	for id, want := range fresh.Models {
		got, ok := shipped.Models[id]
		switch {
		case !ok:
			missing = append(missing, id)
		case !sameContract(want, got):
			stale = append(stale, id)
		}
	}
	for id := range shipped.Models {
		if _, ok := fresh.Models[id]; !ok {
			stale = append(stale, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)

	if len(missing) > 0 {
		t.Errorf("governance specs with no shipped contract (regenerate with `go run ./modelcontracts/gen`): %s",
			strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		t.Errorf("shipped contracts that disagree with their spec (regenerate with `go run ./modelcontracts/gen`): %s",
			strings.Join(stale, ", "))
	}
}

func TestShippedPayloadCoversEverySpecFile(t *testing.T) {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Fatalf("read %s: %v", specDir, err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			files++
		}
	}
	if files != Count() {
		t.Errorf("%s holds %d specs but the payload carries %d contracts", specDir, files, Count())
	}
}

func TestStandardImagePromptPublishesOperationalLimitsWithoutChangingVendorIDs(t *testing.T) {
	standard, ok := For("gpt-image-2.5")
	require.True(t, ok)
	require.NotNil(t, standard.PromptMaxChars)
	assert.Equal(t, 8192, *standard.PromptMaxChars)
	assert.Equal(t, lineRestrictedBasis, standard.Basis)
	encoded, err := common.Marshal(standard)
	require.NoError(t, err)
	var published map[string]any
	require.NoError(t, common.Unmarshal(encoded, &published))
	assert.Equal(t, float64(16384), published["prompt_max_utf8_bytes"])
	assert.Equal(t, float64(5000), published["prompt_max_tokens"])
	for _, id := range []string{"gpt-image-2.5-flare"} {
		vendor, ok := For(id)
		require.True(t, ok)
		require.NotNil(t, vendor.PromptMaxChars)
		assert.Equal(t, 32000, *vendor.PromptMaxChars)
		encoded, err := common.Marshal(vendor)
		require.NoError(t, err)
		var independent map[string]any
		require.NoError(t, common.Unmarshal(encoded, &independent))
		assert.NotContains(t, independent, "prompt_max_utf8_bytes", "an unestablished vendor byte limit must remain absent")
		assert.NotContains(t, independent, "prompt_max_tokens", "our operational prompt-token limit must not change independent vendor contracts")
	}
}

func TestStandardImageContractPublishesOnlyReviewedSpecifications(t *testing.T) {
	standard, ok := For("gpt-image-2.5")
	require.True(t, ok)
	require.NotNil(t, standard.ReferenceImages)
	require.NotNil(t, standard.ReferenceImages.Max)
	assert.Equal(t, 1, *standard.ReferenceImages.Max)
	assert.Equal(t, []string{"jpeg", "png", "webp"}, standard.ReferenceImages.Formats)
	require.NotNil(t, standard.ReferenceImages.MaxBytesEach)
	assert.EqualValues(t, 5*1024*1024, *standard.ReferenceImages.MaxBytesEach)
	assert.Equal(t, []string{"1024x1024", "1024x1536", "1536x1024", "1024x1360", "1360x1024", "1088x1920", "1920x1088"}, standard.OutputSizes)
	assert.Equal(t, []string{"png"}, standard.OutputFormats)
	encoded, err := common.Marshal(standard)
	require.NoError(t, err)
	var published map[string]any
	require.NoError(t, common.Unmarshal(encoded, &published))
	serviceNotes, ok := published["service_notes"].(string)
	require.True(t, ok)
	for _, constraint := range []string{"JSON", "一张", "medium", "5MiB", "4000 万像素", "23170", "8192", "16384", "5000", "o200k_base", "不截断"} {
		assert.Contains(t, serviceNotes, constraint)
	}
	assert.NotContains(t, published, "notes", "governance provenance notes must stay private to the spec source")
	require.NotNil(t, standard.Billing)
	require.NotNil(t, standard.Billing.UnitPriceUSD)
	assert.Equal(t, 0.3, *standard.Billing.UnitPriceUSD)
	assert.Contains(t, standard.Billing.Notes["zh"], "每张的服务基准价")
	for _, id := range []string{"gpt-image-2.5-flare"} {
		vendor, ok := For(id)
		require.True(t, ok)
		require.NotNil(t, vendor.ReferenceImages)
		require.NotNil(t, vendor.ReferenceImages.Max)
		assert.Equal(t, 16, *vendor.ReferenceImages.Max)
		assert.Equal(t, []string{"png", "jpeg", "webp"}, vendor.OutputFormats)
		assert.Len(t, vendor.OutputSizes, 3)
		encoded, err := common.Marshal(vendor)
		require.NoError(t, err)
		var independent map[string]any
		require.NoError(t, common.Unmarshal(encoded, &independent))
		assert.NotContains(t, independent, "service_notes")
	}
}

func TestSunburstContractPublishesWorkbenchLimitsWithoutInventedPrice(t *testing.T) {
	contract, ok := For("gpt-image-2.5-sunburst")
	require.True(t, ok)
	assert.Equal(t, lineRestrictedBasis, contract.Basis)
	assert.Nil(t, contract.OutputSizes, "requested pixels must not be published as guaranteed original-image dimensions")
	assert.Equal(t, []string{"png"}, contract.OutputFormats)
	require.NotNil(t, contract.ReferenceImages)
	require.NotNil(t, contract.ReferenceImages.Max)
	assert.Equal(t, 1, *contract.ReferenceImages.Max)
	assert.Equal(t, []string{"jpeg", "png", "webp"}, contract.ReferenceImages.Formats)
	require.NotNil(t, contract.ReferenceImages.MaxBytesEach)
	assert.EqualValues(t, 5*1024*1024, *contract.ReferenceImages.MaxBytesEach)
	require.NotNil(t, contract.PromptMaxChars)
	assert.Equal(t, 8192, *contract.PromptMaxChars)
	require.NotNil(t, contract.PromptMaxUTF8Bytes)
	assert.Equal(t, 16384, *contract.PromptMaxUTF8Bytes)
	require.NotNil(t, contract.PromptMaxTokens)
	assert.Equal(t, 5000, *contract.PromptMaxTokens)
	for _, detail := range []string{"high", "新单仅请求", "1024x576", "客户成品 URL", "原图实际尺寸", "不保证", "精确 16:9", "历史已保存的 2048x1152 任务", "原始请求恢复和核对", "4000", "8192", "16384", "5000", "o200k_base", "PNG eXIf 数据块", "严格小于 4,000,000 字节", "最大 3,999,999 字节", "首发", "备用", "原单", "/v1/images/edits"} {
		assert.Contains(t, contract.ServiceNotes, detail)
	}
	require.NotNil(t, contract.Billing)
	assert.Equal(t, "per_image", contract.Billing.Unit)
	assert.Nil(t, contract.Billing.UnitPriceUSD, "a price not approved for this service must stay absent")
	assert.Contains(t, contract.Billing.Notes["zh"], "以提交前报价为准")
	encoded, err := common.Marshal(contract)
	require.NoError(t, err)
	var published map[string]any
	require.NoError(t, common.Unmarshal(encoded, &published))
	assert.NotContains(t, published, "output_sizes", "an unestablished output-pixel guarantee must remain absent")
	assert.NotContains(t, published, "sources", "private supplier provenance must not become a runtime promise")
	assert.NotContains(t, published, "notes")
	billing, ok := published["billing"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, billing, "unit_price_usd", "an absent sale quote is not zero or a stale historical amount")
}

func TestServiceNotesAreExplicitAndDoNotPublishGovernanceNotes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ordinary.json"), []byte(`{"model_id":"ordinary","notes":"private provenance"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "restricted.json"), []byte(`{"model_id":"restricted","service_notes":"public service limits","notes":"private audit detail"}`), 0o600))
	doc, err := Build(dir)
	require.NoError(t, err)
	for _, id := range []string{"ordinary", "restricted"} {
		encoded, err := common.Marshal(doc.Models[id])
		require.NoError(t, err)
		var published map[string]any
		require.NoError(t, common.Unmarshal(encoded, &published))
		assert.NotContains(t, published, "notes")
		if id == "restricted" {
			assert.Equal(t, "public service limits", published["service_notes"])
		} else {
			assert.NotContains(t, published, "service_notes")
		}
	}
}

func TestVideoContractCarriesTheDeclaredEnvelope(t *testing.T) {
	video := registered(t, "video")
	if len(video.Operations) == 0 {
		t.Error("video contract declares no operations")
	}
	if video.DurationsSeconds == nil {
		t.Error("video contract declares no duration envelope")
	}
	if video.ReferenceImages == nil && video.ReferenceVideos == nil && video.ReferenceAudios == nil {
		t.Error("video contract declares no reference-media limits at all")
	}
}

func TestAbsentAxesStayAbsentInsteadOfDefaulting(t *testing.T) {
	// A family whose specs declare no duration or resolution vocabulary must
	// arrive without one; defaulting here would invent the very contract this
	// package exists to publish.
	data := registered(t, "data")
	if data.DurationsSeconds != nil {
		t.Errorf("data contract carries an invented duration envelope: %+v", data.DurationsSeconds)
	}
	if data.Resolutions != nil {
		t.Errorf("data contract carries an invented resolution list: %v", data.Resolutions)
	}
	if data.AudioSupported != nil {
		t.Errorf("data contract carries an invented audio flag: %v", *data.AudioSupported)
	}
}

func TestForResolvesTheDottedSpellingOfAHyphenatedId(t *testing.T) {
	// The catalog spells some ids with a dot and others with a hyphen, so the
	// retry exists for a caller holding the dotted spelling of a hyphenated key.
	// `seedance-2-5` and `seedance-2.5` are the pair the frontend already has to
	// reconcile.
	models := All().Models
	if _, ok := models["seedance-2-5"]; !ok {
		t.Skip("seedance-2-5 is not in the shipped payload")
	}
	if _, twin := models["seedance-2.5"]; twin {
		t.Skip("both spellings exist as models; an exact match correctly wins")
	}

	got, ok := For("seedance-2.5")
	if !ok {
		t.Fatal(`For("seedance-2.5") found nothing; the hyphenated key is in the payload`)
	}
	if got.ModelID != "seedance-2-5" {
		t.Errorf(`For("seedance-2.5") resolved to %q, want "seedance-2-5"`, got.ModelID)
	}
}

func TestForReportsAbsenceForUnknownModels(t *testing.T) {
	if _, ok := For("no-such-model-id"); ok {
		t.Error("unknown model resolved to a contract")
	}
}

func TestLineRestrictedSpecsPublishTheirBasis(t *testing.T) {
	// A reverse-engineered line is narrower than the vendor model it serves, and
	// a consumer must be able to tell the two apart without reading the spec's
	// prose. Whatever a spec marks must survive into the published payload.
	fresh, err := Build(specDir)
	if err != nil {
		t.Fatalf("build from %s: %v", specDir, err)
	}

	marked := 0
	for id, contract := range fresh.Models {
		if contract.Basis == "" {
			continue
		}
		marked++
		if contract.Basis != lineRestrictedBasis {
			t.Errorf("%s: basis = %q, want one of the declared values", id, contract.Basis)
		}
		if shipped, ok := All().Models[id]; !ok || shipped.Basis != contract.Basis {
			t.Errorf("%s: basis %q did not survive into the published payload", id, contract.Basis)
		}
	}
	if marked == 0 {
		t.Fatal("no spec declares a basis, so the field is untested")
	}
}

func TestApplySpecDirServesTheProjectionFromDisk(t *testing.T) {
	// Swapping the document is process-wide state, so a test that left it swapped
	// would poison every later test in this package.
	restore := All()
	t.Cleanup(func() {
		loaded, loadErr = restore, nil
	})

	dir := t.TempDir()
	spec := `{"model_id":"temp-probe","family":"video","status":"documented",` +
		`"durations_seconds":{"min":7,"max":7,"allowed":[7]}}`
	if err := os.WriteFile(filepath.Join(dir, "temp-probe.json"), []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if err := ApplySpecDir(dir); err != nil {
		t.Fatalf("ApplySpecDir(%s): %v", dir, err)
	}
	contract, ok := For("temp-probe")
	if !ok {
		t.Fatal("the projection built from disk did not become the served document")
	}
	if contract.DurationsSeconds == nil || contract.DurationsSeconds.Min == nil || *contract.DurationsSeconds.Min != 7 {
		t.Errorf("served contract lost the declared envelope: %+v", contract.DurationsSeconds)
	}
	if _, ok := For("seedance-2-5"); ok {
		t.Error("the embedded payload is still served, so the override did not take effect")
	}
}

func TestApplySpecDirRefusesADirectoryWithNoSpecs(t *testing.T) {
	restore := All()
	t.Cleanup(func() { loaded, loadErr = restore, nil })

	if err := ApplySpecDir(t.TempDir()); err == nil {
		t.Fatal("an empty directory must not replace the served contracts")
	}
	if _, ok := For("seedance-2-5"); !ok {
		t.Error("a refused directory must leave the embedded contracts in place")
	}
}
