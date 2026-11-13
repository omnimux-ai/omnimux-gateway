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

// buildSpecDir writes specs into a temporary directory and projects them.
func buildSpecDir(t *testing.T, files map[string]string) *Document {
	t.Helper()
	doc, err := buildSpecDirResult(t, files)
	require.NoError(t, err)
	return doc
}

func buildSpecDirResult(t *testing.T, files map[string]string) (*Document, error) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	return Build(dir)
}

func operationByID(t *testing.T, contract Contract, operationID string) Operation {
	t.Helper()
	for _, operation := range contract.Operations {
		if operation.ID == operationID {
			return operation
		}
	}
	t.Fatalf("%s has no operation %q", contract.ModelID, operationID)
	return Operation{}
}

func TestEveryBuiltContractCarriesSchemaAndContentVersion(t *testing.T) {
	doc, err := Build(specDir)
	require.NoError(t, err)
	require.Equal(t, SchemaVersion, doc.SchemaVersion)
	require.NotEmpty(t, doc.Models)

	for id, contract := range doc.Models {
		assert.Equal(t, SchemaVersion, contract.SchemaVersion, id)
		assert.Regexp(t, `^[0-9a-f]{16}$`, contract.ContractVersion, id)

		// The version must be reproducible from the contract itself with the
		// field excluded, and no other field may change it accidentally.
		recomputed, err := contractVersion(contract)
		require.NoError(t, err)
		assert.Equal(t, recomputed, contract.ContractVersion, id)
	}
}

func TestContractVersionChangesWhenTheContractChanges(t *testing.T) {
	contract := Contract{
		ModelID:       "probe",
		SchemaVersion: SchemaVersion,
		Family:        "llm",
		Operations:    []Operation{},
	}
	first, err := contractVersion(contract)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	firstAgain, err := contractVersion(contract)
	require.NoError(t, err)
	assert.Equal(t, first, firstAgain, "version must be deterministic")

	contract.ServiceNotes = "changed"
	second, err := contractVersion(contract)
	require.NoError(t, err)
	assert.NotEqual(t, first, second, "a content change must move the contract version")
}

func TestEveryOperationHasIDOutputAndUniqueIDs(t *testing.T) {
	doc, err := Build(specDir)
	require.NoError(t, err)
	for id, contract := range doc.Models {
		require.NotEmpty(t, contract.Operations, "%s carries no operation", id)
		seen := make(map[string]bool, len(contract.Operations))
		for _, operation := range contract.Operations {
			assert.NotEmpty(t, operation.ID, "%s has an operation without an id", id)
			assert.False(t, seen[operation.ID], "%s repeats operation %q", id, operation.ID)
			seen[operation.ID] = true
			assert.NotEmpty(t, operation.Output.Type, "%s/%s has no output type", id, operation.ID)
			assert.NotNil(t, operation.Inputs, "%s/%s inputs must be an array", id, operation.ID)
			assert.NotNil(t, operation.Parameters, "%s/%s parameters must be an array", id, operation.ID)
		}
		require.Len(t, contract.OperationIDs, len(contract.Operations))
		for index, operation := range contract.Operations {
			assert.Equal(t, operation.ID, contract.OperationIDs[index])
		}
	}
}

func TestUnknownsRecordNullFamilyAxesAndOperationGaps(t *testing.T) {
	video := buildSpecDir(t, map[string]string{
		"seedance-2-5.json": `{
			"model_id": "seedance-2-5",
			"family": "video",
			"status": "documented",
			"operations": ["text_to_video", "first_frame"],
			"gateway": {"profile": "apimart_video", "operations": ["text_to_video", "first_frame"]},
			"durations_seconds": {"min": null, "max": null},
			"resolutions": null,
			"aspect_ratios": null,
			"prompt_max_chars": null
		}`,
	})
	contract := video.Models["seedance-2-5"]
	for _, path := range []string{
		"durations_seconds.min",
		"durations_seconds.max",
		"resolutions",
		"aspect_ratios",
		"prompt_max_chars",
		"operations.first_frame.parameters.duration",
	} {
		assert.Contains(t, contract.Unknowns, path)
	}

	image := buildSpecDir(t, map[string]string{
		"probe-image.json": `{
			"model_id": "probe-image",
			"family": "image",
			"status": "documented",
			"operations": ["image_to_image"],
			"reference_images": {"max": null, "min": 0, "formats": ["png"], "max_bytes_each": null},
			"output_sizes": null
		}`,
	})
	assert.Contains(t, image.Models["probe-image"].Unknowns, "reference_images.max")
	assert.Contains(t, image.Models["probe-image"].Unknowns, "reference_images.max_bytes_each")
	assert.Contains(t, image.Models["probe-image"].Unknowns, "output_sizes")
}

func TestApimartVideoProfileExpandsExecutableOperations(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"seedance-2-5.json": `{
			"model_id": "seedance-2-5",
			"family": "video",
			"status": "documented",
			"operations": ["text_to_video", "first_frame", "first_last_frame", "video_multi_ref", "video_edit", "video_extend"],
			"gateway": {
				"profile": "apimart_video",
				"operations": ["text_to_video", "first_frame", "first_last_frame", "video_multi_ref", "video_edit", "video_extend"]
			},
			"durations_seconds": {"min": 4, "max": 30},
			"resolutions": ["480p", "720p", "1080p"],
			"aspect_ratios": ["16:9", "9:16", "adaptive"],
			"reference_images": {"min": 1, "max": 30, "formats": ["image/jpeg", "image/png"], "max_bytes_each": 31457280},
			"reference_videos": {"min": 0, "max": 10, "formats": ["video/mp4"], "max_bytes_each": 209715200},
			"reference_audios": {"min": 0, "max": 10, "formats": ["audio/wav"], "max_bytes_each": 15728640},
			"output_formats": ["mp4", "mov"]
		}`,
	})
	contract := doc.Models["seedance-2-5"]
	require.Equal(t, []string{"text_to_video", "first_frame", "first_last_frame", "video_multi_ref", "video_edit", "video_extend"}, contract.OperationIDs)
	for _, operation := range contract.Operations {
		assert.True(t, operation.Executable, operation.ID)
		assert.Equal(t, "video", operation.Output.Type)
	}

	firstFrame := operationByID(t, contract, "first_frame")
	require.Len(t, firstFrame.Inputs, 2)
	assert.Equal(t, "prompt", firstFrame.Inputs[0].Slot)
	assert.Equal(t, "first_frame", firstFrame.Inputs[1].Slot)
	assert.True(t, firstFrame.Inputs[1].Required)
	require.NotNil(t, firstFrame.Inputs[1].Min)
	require.NotNil(t, firstFrame.Inputs[1].Max)
	assert.Equal(t, 1, *firstFrame.Inputs[1].Min)
	assert.Equal(t, 1, *firstFrame.Inputs[1].Max)
	assert.Equal(t, []string{"image/jpeg", "image/png"}, firstFrame.Inputs[1].AllowedMimes)
	require.NotNil(t, firstFrame.Inputs[1].MaxSizeMb)
	assert.Equal(t, 30.0, *firstFrame.Inputs[1].MaxSizeMb)

	multiRef := operationByID(t, contract, "video_multi_ref")
	require.Len(t, multiRef.Inputs, 4)
	assert.Equal(t, "reference_images", multiRef.Inputs[1].Slot)
	require.NotNil(t, multiRef.Inputs[1].Max)
	assert.Equal(t, 30, *multiRef.Inputs[1].Max)

	edit := operationByID(t, contract, "video_edit")
	var editDuration *Parameter
	var editAspect *Parameter
	for index := range edit.Parameters {
		switch edit.Parameters[index].Name {
		case "duration":
			editDuration = &edit.Parameters[index]
		case "aspect_ratio":
			editAspect = &edit.Parameters[index]
		}
	}
	require.NotNil(t, editDuration)
	require.NotNil(t, editDuration.Minimum)
	require.NotNil(t, editDuration.Maximum)
	assert.Equal(t, -1.0, *editDuration.Minimum)
	assert.Equal(t, -1.0, *editDuration.Maximum)
	assert.Equal(t, -1, editDuration.Default)
	require.NotNil(t, editAspect)
	assert.Equal(t, "adaptive", editAspect.Fixed)
}

func TestFalH3VideoProfileExpandsExecutableOperations(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-fal.json": `{
			"model_id": "probe-fal",
			"family": "video",
			"status": "documented",
			"operations": ["text_to_video", "first_frame", "video_multi_ref"],
			"gateway": {"profile": "fal_h3_video", "operations": ["text_to_video", "first_frame", "video_multi_ref"]},
			"durations_seconds": {"min": 5, "max": 10, "allowed": [5, 10]},
			"resolutions": ["480P", "768P"],
			"aspect_ratios": ["16:9", "9:16"],
			"reference_images": {"min": 0, "max": 9},
			"reference_videos": {"min": 0, "max": 3},
			"reference_audios": {"min": 0, "max": 3}
		}`,
	})
	contract := doc.Models["probe-fal"]
	assert.Equal(t, []string{"text_to_video", "first_frame", "video_multi_ref"}, contract.OperationIDs)
	firstFrame := operationByID(t, contract, "first_frame")
	require.Len(t, firstFrame.Inputs, 2)
	require.NotNil(t, firstFrame.Inputs[1].Min)
	require.NotNil(t, firstFrame.Inputs[1].Max)
	assert.Equal(t, 1, *firstFrame.Inputs[1].Min)
	assert.Equal(t, 1, *firstFrame.Inputs[1].Max)
	multiRef := operationByID(t, contract, "video_multi_ref")
	assert.Len(t, multiRef.Inputs, 4, "prompt plus image, video and audio reference slots")
}

func TestAutoDLH3VideoProfileExpandsItsOwnWorkflows(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"minimax-h3-video.json": `{
			"model_id": "minimax-h3-video",
			"family": "video",
			"status": "documented",
			"operations": ["text_to_video", "first_last_frame", "video_multi_ref", "digital_human"],
			"gateway": {"profile": "autodl_h3_video", "operations": ["text_to_video", "first_last_frame", "video_multi_ref", "digital_human"]},
			"durations_seconds": {"min": 1, "max": 15},
			"resolutions": ["480p", "736p", "768p", "1088p", "1440p"],
			"aspect_ratios": ["16:9", "9:16", "1:1"],
			"reference_images": {"min": 0, "max": 9},
			"reference_audios": {"min": 0, "max": 3}
		}`,
	})
	contract := doc.Models["minimax-h3-video"]
	assert.Equal(t, []string{"text_to_video", "first_last_frame", "video_multi_ref", "digital_human"}, contract.OperationIDs)

	multiRef := operationByID(t, contract, "video_multi_ref")
	assert.Len(t, multiRef.Inputs, 3, "prompt, reference images and reference audios; this line has no reference videos")
	for _, input := range multiRef.Inputs {
		assert.NotEqual(t, "reference_videos", input.Slot)
	}

	digitalHuman := operationByID(t, contract, "digital_human")
	require.Len(t, digitalHuman.Inputs, 2)
	assert.Equal(t, "reference_image", digitalHuman.Inputs[0].Slot)
	assert.Equal(t, "reference_audio", digitalHuman.Inputs[1].Slot)
	assert.True(t, digitalHuman.Inputs[0].Required)
	assert.True(t, digitalHuman.Inputs[1].Required)
	for _, input := range digitalHuman.Inputs {
		assert.NotEqual(t, "prompt", input.Slot, "the AutoDL lip-sync shape has no prompt slot")
		require.NotNil(t, input.Min)
		require.NotNil(t, input.Max)
		assert.Equal(t, 1, *input.Min)
		assert.Equal(t, 1, *input.Max)
	}

	// The shipped spec must resolve minimax-h3-video to its actual AutoDL
	// supply, not the fal profile that was once recorded by mistake.
	shipped, err := Build(specDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"text_to_video", "first_last_frame", "video_multi_ref", "digital_human"}, shipped.Models["minimax-h3-video"].OperationIDs)
}

func TestFalH3LipSyncProfileExpandsImageAndAudioOnly(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-lip-sync.json": `{
			"model_id": "probe-lip-sync",
			"family": "video",
			"status": "documented",
			"operations": ["lip_sync"],
			"gateway": {"profile": "fal_h3_lip_sync", "operations": ["lip_sync"]},
			"resolutions": ["480P", "768P", "1080P", "2K"],
			"reference_images": {"min": 1, "max": 1},
			"reference_audios": {"min": 1, "max": 1}
		}`,
	})
	contract := doc.Models["probe-lip-sync"]
	assert.Equal(t, "live", contract.ExecutionStatus)
	operation := operationByID(t, contract, "lip_sync")
	require.Len(t, operation.Inputs, 2, "one portrait image and one audio track")
	assert.Equal(t, "reference_image", operation.Inputs[0].Slot)
	assert.Equal(t, "reference_audio", operation.Inputs[1].Slot)
	assert.True(t, operation.Inputs[0].Required)
	assert.True(t, operation.Inputs[1].Required)
	for _, input := range operation.Inputs {
		assert.NotEqual(t, "prompt", input.Slot, "fal lip-sync does not consume a text prompt")
	}
	for _, parameter := range operation.Parameters {
		assert.NotEqual(t, "duration", parameter.Name, "lip-sync length follows the audio")
	}
}

func TestImplicitFamilyProfilesUseRealOperationIDs(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-image.json": `{
			"model_id": "probe-image",
			"family": "image",
			"status": "documented",
			"operations": ["text_to_image", "image_to_image"],
			"reference_images": {"max": 2, "min": 1, "formats": ["image/png"], "max_bytes_each": 5242880},
			"output_sizes": ["1024x1024"],
			"output_formats": ["png"]
		}`,
		"probe-llm.json": `{
			"model_id": "probe-llm",
			"family": "llm",
			"status": "documented",
			"operations": ["chat_completion", "streaming"],
			"input_modalities": ["text", "image", "pdf"],
			"reference_images": {"formats": ["image/jpeg"]},
			"context_window_tokens": 128000
		}`,
		"probe-data.json": `{
			"model_id": "probe-data",
			"family": "data",
			"status": "documented",
			"operations": ["fetch_post", "search_products"],
			"endpoints": [{"name": "客户入口", "method": "POST", "path": "/v1/chat/completions"}],
			"auth": "bearer"
		}`,
		"probe-asr.json": `{
			"model_id": "probe-asr",
			"family": "audio",
			"status": "documented",
			"operations": ["transcription", "translation"],
			"audio_kind": "asr",
			"input_formats": ["wav", "mp3"],
			"max_input_bytes": 10485760,
			"max_input_seconds": 300,
			"output_formats": ["text"]
		}`,
		"probe-tts.json": `{
			"model_id": "probe-tts",
			"family": "audio",
			"status": "documented",
			"operations": ["text_to_speech", "voice_clone", "emotion_control"],
			"audio_kind": "tts",
			"reference_audios": {"min": 0, "max": 2, "formats": ["wav"]},
			"output_formats": ["wav"]
		}`,
	})

	image := doc.Models["probe-image"]
	assert.Equal(t, []string{"text_to_image", "image_to_image"}, image.OperationIDs)
	textToImage := operationByID(t, image, "text_to_image")
	assert.Len(t, textToImage.Inputs, 1, "text_to_image consumes only the prompt")
	imageToImage := operationByID(t, image, "image_to_image")
	require.Len(t, imageToImage.Inputs, 2)
	assert.Equal(t, "reference_image", imageToImage.Inputs[1].Slot)
	require.NotNil(t, imageToImage.Inputs[1].Min)
	require.NotNil(t, imageToImage.Inputs[1].Max)
	assert.Equal(t, 1, *imageToImage.Inputs[1].Min)
	assert.Equal(t, 2, *imageToImage.Inputs[1].Max)

	llm := doc.Models["probe-llm"]
	assert.Equal(t, []string{"chat"}, llm.OperationIDs)
	chat := llm.Operations[0]
	assert.Equal(t, "text", chat.Output.Type)
	var slots []string
	for _, input := range chat.Inputs {
		slots = append(slots, input.Slot)
	}
	assert.Equal(t, []string{"prompt", "image", "pdf"}, slots)

	data := doc.Models["probe-data"]
	assert.Equal(t, []string{"fetch_post", "search_products"}, data.OperationIDs)
	for _, operation := range data.Operations {
		assert.Equal(t, "text", operation.Output.Type)
	}

	asr := doc.Models["probe-asr"]
	assert.Equal(t, []string{"speech_to_text", "translation"}, asr.OperationIDs)
	for _, operation := range asr.Operations {
		assert.Equal(t, "text", operation.Output.Type)
	}
	require.Len(t, asr.Operations[0].Inputs, 1)
	assert.Equal(t, "audio", asr.Operations[0].Inputs[0].Slot)

	tts := doc.Models["probe-tts"]
	assert.Equal(t, []string{"text_to_speech", "voice_clone", "emotion_control"}, tts.OperationIDs)
	for _, operation := range tts.Operations {
		assert.Equal(t, "audio", operation.Output.Type)
	}
}

func TestVideoWithoutGatewayFallsBackToNonExecutableCanonicalOperations(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-video.json": `{
			"model_id": "probe-video",
			"family": "video",
			"status": "documented",
			"operations": ["text_to_video", "image_to_video", "reference_to_video", "first_last_frame_to_video"],
			"durations_seconds": {"min": 3, "max": 15},
			"resolutions": ["720p"]
		}`,
	})
	contract := doc.Models["probe-video"]
	assert.Equal(t, []string{"text_to_video", "first_frame", "video_multi_ref", "first_last_frame"}, contract.OperationIDs)
	for _, operation := range contract.Operations {
		assert.False(t, operation.Executable, operation.ID)
		assert.Empty(t, operation.Inputs)
		assert.Empty(t, operation.Parameters)
		assert.Equal(t, "video", operation.Output.Type)
	}
	assert.Contains(t, contract.Unknowns, "operations.first_frame.inputs")
}

func TestBuildRejectsUnknownGatewayProfile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{
		"model_id": "seedance-2-5",
		"family": "video",
		"status": "documented",
		"operations": ["text_to_video"],
		"gateway": {"profile": "not_a_profile", "operations": ["text_to_video"]}
	}`), 0o600))
	_, err := Build(dir)
	require.Error(t, err)
	var specErr *SpecError
	require.ErrorAs(t, err, &specErr)
	assert.Equal(t, "bad.json", specErr.File)
	assert.Contains(t, err.Error(), "unknown gateway profile")
}

func TestBuildRejectsUnknownAndDisallowedOperations(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown operation",
			body: `{
				"model_id": "seedance-2-5",
				"family": "video",
				"status": "documented",
				"operations": ["text_to_video"],
				"gateway": {"profile": "apimart_video", "operations": ["text_to_video", "no_such_operation"]}
			}`,
			want: "unknown operation",
		},
		{
			name: "operation not allowed for model",
			body: `{
				"model_id": "seedance-2-0",
				"family": "video",
				"status": "documented",
				"operations": ["text_to_video"],
				"gateway": {"profile": "apimart_video", "operations": ["text_to_video", "video_edit"]}
			}`,
			want: "not allowed",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte(testCase.body), 0o600))
			_, err := Build(dir)
			require.Error(t, err)
			var specErr *SpecError
			require.ErrorAs(t, err, &specErr)
			assert.Equal(t, "bad.json", specErr.File)
			assert.Contains(t, err.Error(), testCase.want)
		})
	}
}

func TestBuildRejectsDuplicateGatewayOperations(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{
		"model_id": "seedance-2-5",
		"family": "video",
		"status": "documented",
		"operations": ["text_to_video"],
		"gateway": {"profile": "apimart_video", "operations": ["text_to_video", "text_to_video"]}
	}`), 0o600))
	_, err := Build(dir)
	require.Error(t, err)
	var specErr *SpecError
	require.ErrorAs(t, err, &specErr)
	assert.Equal(t, "bad.json", specErr.File)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestOperationJSONPublishesEmptyArraysNotNull(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-video.json": `{
			"model_id": "probe-video",
			"family": "video",
			"status": "documented",
			"operations": ["text_to_video"],
			"durations_seconds": {"min": 1, "max": 2},
			"resolutions": ["720p"]
		}`,
	})
	body, err := common.Marshal(doc.Models["probe-video"].Operations[0])
	require.NoError(t, err)
	var published map[string]any
	require.NoError(t, common.Unmarshal(body, &published))
	assert.Equal(t, []any{}, published["inputs"])
	assert.Equal(t, []any{}, published["parameters"])
}

func TestNewModelsInAnEstablishedFamilyNeedOnlyASpec(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"newcomer-llm.json": `{
			"model_id": "newcomer-llm",
			"family": "llm",
			"status": "documented",
			"operations": ["chat_completion"],
			"input_modalities": ["text"],
			"context_window_tokens": 64000
		}`,
		"newcomer-image.json": `{
			"model_id": "newcomer-image",
			"family": "image",
			"status": "documented",
			"operations": ["text_to_image"],
			"reference_images": {"max": 1, "min": 0, "formats": ["png"], "max_bytes_each": 5242880},
			"output_sizes": ["1024x1024"]
		}`,
	})
	llm, ok := doc.Models["newcomer-llm"]
	require.True(t, ok)
	assert.Equal(t, []string{"chat"}, llm.OperationIDs)
	assert.NotEmpty(t, llm.ContractVersion)

	image, ok := doc.Models["newcomer-image"]
	require.True(t, ok)
	assert.Equal(t, []string{"text_to_image"}, image.OperationIDs)
	assert.NotEmpty(t, image.ContractVersion)
}

func TestToolFieldsSurviveProjectionUnderTheirPublishedTypes(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-llm.json": `{
			"model_id": "probe-llm",
			"family": "llm",
			"status": "documented",
			"operations": ["chat_completion"],
			"input_modalities": ["text"],
			"context_window_tokens": 128000,
			"tool_calling": true,
			"streaming": false,
			"prompt_caching": true
		}`,
		"probe-audio.json": `{
			"model_id": "probe-audio",
			"family": "audio",
			"status": "documented",
			"operations": ["text_to_speech"],
			"audio_kind": "tts",
			"input_formats": ["wav", "mp3"],
			"max_input_bytes": 10485760,
			"max_input_seconds": 30,
			"max_output_seconds": 120,
			"voices": 3,
			"languages": ["English", "Chinese"],
			"output_formats": ["wav"]
		}`,
		"probe-data.json": `{
			"model_id": "probe-data",
			"family": "data",
			"status": "documented",
			"operations": ["fetch_post"],
			"endpoints": [{"name": "客户入口", "method": "POST", "path": "/v1/chat/completions"}],
			"auth": "bearer",
			"rate_limit": null,
			"pagination": "cursor",
			"response_fields": ["id", "url"]
		}`,
	})

	llm := doc.Models["probe-llm"]
	require.NotNil(t, llm.ToolCalling)
	require.NotNil(t, llm.Streaming)
	assert.True(t, *llm.ToolCalling)
	assert.False(t, *llm.Streaming, "a declared false must survive as a pointer, not be omitted")
	require.NotNil(t, llm.PromptCaching)
	assert.True(t, *llm.PromptCaching)

	audio := doc.Models["probe-audio"]
	assert.Equal(t, "tts", audio.AudioKind)
	assert.Equal(t, []string{"wav", "mp3"}, audio.InputFormats)
	require.NotNil(t, audio.MaxInputBytes)
	assert.EqualValues(t, 10485760, *audio.MaxInputBytes)
	require.NotNil(t, audio.MaxInputSeconds)
	assert.Equal(t, 30, *audio.MaxInputSeconds)
	require.NotNil(t, audio.MaxOutputSeconds)
	assert.Equal(t, 120, *audio.MaxOutputSeconds)
	require.NotNil(t, audio.Voices)
	assert.Equal(t, 3, *audio.Voices)
	assert.Equal(t, []string{"English", "Chinese"}, audio.Languages)

	data := doc.Models["probe-data"]
	require.Len(t, data.Endpoints, 1)
	assert.Equal(t, "POST", data.Endpoints[0].Method)
	assert.Equal(t, "bearer", data.Auth)
	assert.Equal(t, "cursor", data.Pagination)
	assert.Equal(t, []string{"id", "url"}, data.ResponseFields)
}

func TestNonNumericVoicesAndNonStringLanguagesStayUnpublished(t *testing.T) {
	doc := buildSpecDir(t, map[string]string{
		"probe-audio.json": `{
			"model_id": "probe-audio",
			"family": "audio",
			"status": "documented",
			"operations": ["text_to_speech"],
			"audio_kind": "tts",
			"voices": "支持男女声智能匹配",
			"languages": "提示词支持中、英等多语言描述",
			"output_formats": ["wav"]
		}`,
	})
	contract := doc.Models["probe-audio"]
	assert.Nil(t, contract.Voices, "a prose voice description is not a numeric voice count")
	assert.Nil(t, contract.Languages, "a prose language description is not a string array")
}

func TestShippedPayloadStaysUnderSizeBudget(t *testing.T) {
	info, err := os.Stat("contracts.json")
	require.NoError(t, err)
	const budget = 1536 * 1024 // 1.5 MiB
	assert.LessOrEqual(t, info.Size(), int64(budget), "contracts.json must stay at or below 1.5 MB")
}

func TestVersionReturnsAnAlreadyComputedDocumentVersion(t *testing.T) {
	doc := &Document{
		SchemaVersion: SchemaVersion,
		Version:       "0123456789abcdef",
		Models: map[string]Contract{
			"unrelated": {ModelID: "unrelated", SchemaVersion: SchemaVersion},
		},
	}
	assert.Equal(t, "0123456789abcdef", Version(doc), "the fast path must not re-hash a loaded document")

	fresh := &Document{SchemaVersion: SchemaVersion, Models: doc.Models}
	assert.NotEmpty(t, Version(fresh))
}

func TestEveryContractPublishesExecutionStatus(t *testing.T) {
	doc, err := Build(specDir)
	require.NoError(t, err)
	valid := map[string]bool{"live": true, "stub": true, "unavailable": true}
	for id, contract := range doc.Models {
		require.True(t, valid[contract.ExecutionStatus], "%s has invalid execution_status %q", id, contract.ExecutionStatus)
		hasExecutable := false
		for _, operation := range contract.Operations {
			if operation.Executable {
				hasExecutable = true
				break
			}
		}
		if contract.ExecutionStatus == "live" {
			assert.True(t, hasExecutable, "%s is live without an executable operation", id)
		} else {
			assert.NotEmpty(t, contract.ExecutionReason, "%s is non-live without an execution reason", id)
		}
	}
}

func TestExecutionStatusContradictionsAreRefused(t *testing.T) {
	t.Run("live without executable operation", func(t *testing.T) {
		_, err := buildSpecDirResult(t, map[string]string{
			"probe-video.json": `{
				"model_id": "probe-video",
				"family": "video",
				"status": "documented",
				"operations": ["text_to_video"],
				"execution": {"status": "live"}
			}`,
		})
		require.ErrorContains(t, err, "execution.status=live requires at least one executable operation")
	})

	t.Run("stub with executable operation", func(t *testing.T) {
		_, err := buildSpecDirResult(t, map[string]string{
			"probe-video.json": `{
				"model_id": "probe-video",
				"family": "video",
				"status": "documented",
				"operations": ["text_to_video"],
				"gateway": {"profile": "fal_h3_video", "operations": ["text_to_video"]},
				"execution": {"status": "stub", "reason": "not deployed"}
			}`,
		})
		require.ErrorContains(t, err, "execution.status=stub contradicts executable operations")
	})
}

func TestProjectDefaultsAggregatesRecommendationsAndRejectsConflicts(t *testing.T) {
	defaults, err := projectDefaults([]specFile{
		{ModelID: "gemini-3.8-flash", Family: "llm", DefaultFor: []string{"chat", "vision_chat"}},
		{ModelID: "gpt-image-2.5", Family: "image", DefaultFor: []string{"text_to_image"}},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"chat":          "gemini-3.8-flash",
		"vision_chat":   "gemini-3.8-flash",
		"text_to_image": "gpt-image-2.5",
	}, defaults)

	_, err = projectDefaults([]specFile{
		{ModelID: "a", Family: "llm", DefaultFor: []string{"chat"}},
		{ModelID: "b", Family: "llm", DefaultFor: []string{"chat"}},
	})
	require.ErrorContains(t, err, "claimed by both")

	_, err = projectDefaults([]specFile{
		{ModelID: "a", Family: "llm", DefaultFor: []string{"unknown"}},
	})
	require.ErrorContains(t, err, "unknown default_for")

	_, err = projectDefaults([]specFile{
		{ModelID: "a", Family: "image", DefaultFor: []string{"chat"}},
	})
	require.ErrorContains(t, err, "requires family")
}

func TestReasoningEffortsProjectAndValidate(t *testing.T) {
	contract, err := projectContract(specFile{
		ModelID:                "probe-llm",
		Family:                 "llm",
		Status:                 "documented",
		Operations:             []string{"chat"},
		ReasoningEfforts:       map[string]string{"low": "low", "high": "high"},
		DefaultReasoningEffort: "high",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"low": "low", "high": "high"}, contract.ReasoningEfforts)
	assert.Equal(t, "high", contract.DefaultReasoningEffort)

	_, err = projectContract(specFile{
		ModelID:                "probe-llm",
		Family:                 "llm",
		Status:                 "documented",
		Operations:             []string{"chat"},
		ReasoningEfforts:       map[string]string{"low": "low"},
		DefaultReasoningEffort: "high",
	})
	require.ErrorContains(t, err, "not a key in reasoning_efforts")

	_, err = projectContract(specFile{
		ModelID:                "probe-llm",
		Family:                 "llm",
		Status:                 "documented",
		Operations:             []string{"chat"},
		DefaultReasoningEffort: "high",
	})
	require.ErrorContains(t, err, "not a key in reasoning_efforts")
}

func TestShippedDefaultsReferenceShippedContracts(t *testing.T) {
	doc, err := Build(specDir)
	require.NoError(t, err)
	require.NotEmpty(t, doc.Defaults)
	for operation, modelID := range doc.Defaults {
		_, ok := doc.Models[modelID]
		assert.True(t, ok, "default_for %s references unknown model %s", operation, modelID)
	}
	shipped := All()
	assert.Equal(t, doc.Defaults, shipped.Defaults, "shipped defaults must match the governance specs")
}

func TestVersionIncludesDefaults(t *testing.T) {
	first := &Document{
		SchemaVersion: "2.0",
		Models:        map[string]Contract{},
		Defaults:      map[string]string{"chat": "a"},
	}
	second := &Document{
		SchemaVersion: "2.0",
		Models:        map[string]Contract{},
		Defaults:      map[string]string{"chat": "b"},
	}
	require.NotEqual(t, Version(first), Version(second))
}
