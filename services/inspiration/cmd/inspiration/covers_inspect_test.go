package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"omnimux-inspiration/internal/media"
)

func readJSONFile(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

type fakeCoverInspectResult struct {
	image *media.CoverImage
	err   error
}

type fakeCoverInspectReader struct {
	results map[string]fakeCoverInspectResult
	calls   []string
}

func (r *fakeCoverInspectReader) Read(_ context.Context, ref string) (*media.CoverImage, error) {
	r.calls = append(r.calls, ref)
	result, ok := r.results[ref]
	if !ok {
		return nil, errors.New("unexpected read")
	}
	return result.image, result.err
}

func TestInspectCoversMixedResults(t *testing.T) {
	missingRef := "r2/publications/missing.jpg"
	invalidImageRef := "r2/publications/not-image.jpg"
	okRef := "r2/publications/ok.png"
	image := []byte("decoded-image-fixture")
	reader := &fakeCoverInspectReader{results: map[string]fakeCoverInspectResult{
		missingRef:      {err: media.ErrCoverNotFound},
		invalidImageRef: {err: media.ErrCoverInvalid},
		okRef:           {image: &media.CoverImage{Data: image, ContentType: "image/png"}},
	}}
	manifest := &coverInspectManifest{Items: []coverInspectManifestItem{
		{ID: "bad-ref", Ref: "https://example.com/cover.jpg"},
		{ID: "missing", Ref: missingRef},
		{ID: "invalid-image", Ref: invalidImageRef},
		{ID: "ok", Ref: okRef},
	}}

	got, err := inspectCovers(t.Context(), manifest, reader)
	if err != nil {
		t.Fatalf("inspect covers: %v", err)
	}
	if got.SchemaVersion != 1 || len(got.Items) != 4 {
		t.Fatalf("inspection = %+v", got)
	}
	if got.Items[0].Status != "invalid" || got.Items[0].ErrorCode != "INVALID_PARAM" {
		t.Fatalf("invalid ref result = %+v", got.Items[0])
	}
	if got.Items[1].Status != "invalid" || got.Items[1].ErrorCode != "COVER_NOT_FOUND" {
		t.Fatalf("missing result = %+v", got.Items[1])
	}
	if got.Items[2].Status != "invalid" || got.Items[2].ErrorCode != "COVER_INVALID" {
		t.Fatalf("invalid image result = %+v", got.Items[2])
	}
	sum := sha256.Sum256(image)
	if item := got.Items[3]; item.Status != "ok" || item.SHA256 != hex.EncodeToString(sum[:]) || item.Bytes != int64(len(image)) || item.ContentType != "image/png" || item.ErrorCode != "" {
		t.Fatalf("ok result = %+v", item)
	}
	wantCalls := []string{missingRef, invalidImageRef, okRef}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("reads = %v, want %v", reader.calls, wantCalls)
	}
}

func TestInspectCoversStopsOnSystemicError(t *testing.T) {
	okRef := "r2/publications/ok.jpg"
	timeoutRef := "r2/publications/timeout.jpg"
	unreadRef := "r2/publications/unread.jpg"
	reader := &fakeCoverInspectReader{results: map[string]fakeCoverInspectResult{
		okRef:      {image: &media.CoverImage{Data: []byte("ok"), ContentType: "image/jpeg"}},
		timeoutRef: {err: media.ErrCoverTimeout},
		unreadRef:  {image: &media.CoverImage{Data: []byte("must not read"), ContentType: "image/jpeg"}},
	}}
	manifest := &coverInspectManifest{Items: []coverInspectManifestItem{
		{ID: "ok", Ref: okRef}, {ID: "timeout", Ref: timeoutRef}, {ID: "unread", Ref: unreadRef},
	}}

	got, err := inspectCovers(t.Context(), manifest, reader)
	if err == nil || !strings.Contains(err.Error(), "COVER_STORAGE_TIMEOUT") {
		t.Fatalf("systemic error = %v", err)
	}
	if len(got.Items) != 2 || got.Items[1].Status != "unavailable" || got.Items[1].ErrorCode != "COVER_STORAGE_TIMEOUT" {
		t.Fatalf("partial inspection = %+v", got)
	}
	if want := []string{okRef, timeoutRef}; !reflect.DeepEqual(reader.calls, want) {
		t.Fatalf("reads = %v, want %v", reader.calls, want)
	}
}

func TestInspectCoversRejectsDuplicateIDsBeforeReads(t *testing.T) {
	reader := &fakeCoverInspectReader{}
	manifest := &coverInspectManifest{Items: []coverInspectManifestItem{
		{ID: "duplicate", Ref: "r2/publications/one.jpg"},
		{ID: "duplicate", Ref: "r2/publications/two.jpg"},
	}}

	if _, err := inspectCovers(t.Context(), manifest, reader); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate id error = %v", err)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("reader called before manifest validation: %v", reader.calls)
	}
}

func TestValidateCoverInspectManifestLimits(t *testing.T) {
	items := make([]coverInspectManifestItem, 200)
	for i := range items {
		items[i] = coverInspectManifestItem{ID: fmt.Sprintf("item-%d", i+1), Ref: "r2/publications/cover.jpg"}
	}
	if err := validateCoverInspectManifest(&coverInspectManifest{Items: items}); err != nil {
		t.Fatalf("200 items rejected: %v", err)
	}
	if err := validateCoverInspectManifest(&coverInspectManifest{}); err == nil {
		t.Fatal("empty manifest accepted")
	}
	items = append(items, coverInspectManifestItem{ID: "item-201", Ref: "r2/publications/cover.jpg"})
	if err := validateCoverInspectManifest(&coverInspectManifest{Items: items}); err == nil {
		t.Fatal("201 items accepted")
	}
}

func TestReadCoverInspectManifestRejectsInvalidShape(t *testing.T) {
	tests := map[string]string{
		"missing items":  `{}`,
		"null items":     `{"items":null}`,
		"missing ref":    `{"items":[{"id":"one"}]}`,
		"non-string ref": `{"items":[{"id":"one","ref":7}]}`,
		"unknown field":  `{"items":[{"id":"one","ref":"r2/publications/a.jpg","extra":true}]}`,
		"trailing value": `{"items":[{"id":"one","ref":"r2/publications/a.jpg"}]} {}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readCoverInspectManifest(path); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}

func TestCoversInspectCommandWritesSecureJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	outputPath := filepath.Join(dir, "inspection.json")
	envPath := filepath.Join(dir, "reader.env")
	ref := "r2/publications/source.jpg"
	if err := os.WriteFile(manifestPath, []byte(`{"items":[{"id":"source-1","ref":"`+ref+`"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := &fakeCoverInspectReader{results: map[string]fakeCoverInspectResult{
		ref: {image: &media.CoverImage{Data: []byte("image"), ContentType: "image/jpeg"}},
	}}
	var gotEnvPath string
	cmd := coversInspectCmdWithReader(func(path string) (coverInspectReader, error) {
		gotEnvPath = path
		return reader, nil
	})
	cmd.SetArgs([]string{"--manifest", manifestPath, "--out", outputPath, "--env-file", envPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute inspect: %v", err)
	}
	if gotEnvPath != envPath {
		t.Fatalf("env file = %q, want %q", gotEnvPath, envPath)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %#o, want 0600", info.Mode().Perm())
	}
	var got coverInspection
	if err := readJSONFile(outputPath, &got); err != nil {
		t.Fatalf("round-trip output: %v", err)
	}
	if got.SchemaVersion != 1 || len(got.Items) != 1 || got.Items[0].ID != "source-1" || got.Items[0].Status != "ok" {
		t.Fatalf("round-trip inspection = %+v", got)
	}
	if err := writeSecureJSON(outputPath, got); err == nil {
		t.Fatal("inspection output was clobbered")
	}
}

func TestCoversInspectCommandSavesPartialResultsBeforeFatalError(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	outputPath := filepath.Join(dir, "inspection.json")
	firstRef := "r2/publications/first.jpg"
	secondRef := "r2/publications/second.jpg"
	raw := `{"items":[{"id":"first","ref":"` + firstRef + `"},{"id":"second","ref":"` + secondRef + `"}]}`
	if err := os.WriteFile(manifestPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := &fakeCoverInspectReader{results: map[string]fakeCoverInspectResult{
		firstRef:  {image: &media.CoverImage{Data: []byte("image"), ContentType: "image/jpeg"}},
		secondRef: {err: media.ErrCoverUnavailable},
	}}
	cmd := coversInspectCmdWithReader(func(string) (coverInspectReader, error) { return reader, nil })
	cmd.SetArgs([]string{"--manifest", manifestPath, "--out", outputPath})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "COVER_STORAGE_UNAVAILABLE") {
		t.Fatalf("execute error = %v", err)
	}
	var got coverInspection
	if err := readJSONFile(outputPath, &got); err != nil {
		t.Fatalf("read partial output: %v", err)
	}
	if len(got.Items) != 2 || got.Items[0].Status != "ok" || got.Items[1].Status != "unavailable" {
		t.Fatalf("partial output = %+v", got)
	}
}

func TestCoversInspectCommandSavesUnavailableResultOnConfigurationFailure(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	outputPath := filepath.Join(dir, "inspection.json")
	if err := os.WriteFile(manifestPath, []byte(`{"items":[{"id":"source","ref":"r2/publications/source.jpg"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := coversInspectCmdWithReader(func(string) (coverInspectReader, error) {
		return nil, errors.New("configuration includes sensitive detail")
	})
	cmd.SetArgs([]string{"--manifest", manifestPath, "--out", outputPath})
	if err := cmd.Execute(); err == nil || err.Error() != "cover reader configuration unavailable" {
		t.Fatalf("execute error = %v", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %#o, want 0600", info.Mode().Perm())
	}
	var got coverInspection
	if err := readJSONFile(outputPath, &got); err != nil {
		t.Fatalf("read unavailable output: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Status != "unavailable" || got.Items[0].ErrorCode != "COVER_STORAGE_UNAVAILABLE" {
		t.Fatalf("unavailable output = %+v", got)
	}
}

func TestNewCoverInspectReaderRejectsMissingExplicitEnvFile(t *testing.T) {
	t.Setenv("INSPIRATION_TOKEN", "set-in-process")
	t.Setenv("INSPIRATION_COVER_R2_ENDPOINT", "https://account.r2.cloudflarestorage.com")
	t.Setenv("INSPIRATION_COVER_R2_BUCKET", "bucket")
	t.Setenv("INSPIRATION_COVER_R2_ACCESS_KEY", "access")
	t.Setenv("INSPIRATION_COVER_R2_SECRET_KEY", "secret")
	missing := filepath.Join(t.TempDir(), "missing.env")
	if _, err := newCoverInspectReader(missing); err == nil || !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("missing explicit env file error = %v", err)
	}
	malformed := filepath.Join(t.TempDir(), "malformed.env")
	if err := os.WriteFile(malformed, []byte("KEY='unterminated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newCoverInspectReader(malformed); err == nil || !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("malformed explicit env file error = %v", err)
	}
}
