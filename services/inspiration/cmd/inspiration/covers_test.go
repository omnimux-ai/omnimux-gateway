package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSecureJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	want := map[string]string{"operationId": "fixture"}
	if err := writeSecureJSON(path, want); err != nil {
		t.Fatalf("write secure JSON: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("output mode = %#o, want 0600", got)
	}
	var got map[string]string
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if got["operationId"] != want["operationId"] {
		t.Fatalf("output = %#v", got)
	}
	if err := writeSecureJSON(path, want); err == nil {
		t.Fatal("existing output was overwritten")
	}
}

func TestReceiptOutputReservedBeforeTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	output, err := reserveReceiptOutput(path, "operation", "digest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reserveReceiptOutput(path, "another", "digest"); err == nil {
		t.Fatal("existing receipt reservation accepted")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var pending map[string]string
	if err := json.Unmarshal(raw, &pending); err != nil || pending["state"] != "PREPARED" {
		t.Fatalf("pending receipt: %s %v", raw, err)
	}
	if err := output.save(map[string]string{"state": "AFTER"}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	if err := output.save(map[string]string{"state": "AFTER"}); err == nil {
		t.Fatal("replaced reservation reused")
	}
}

func TestSecureJSONDoesNotOverwriteSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "link.json")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := writeSecureJSON(link, map[string]string{"value": "replacement"}); err == nil {
		t.Fatal("symlink overwritten")
	}
	raw, _ := os.ReadFile(target)
	if string(raw) != "original" {
		t.Fatal("target changed")
	}
}
