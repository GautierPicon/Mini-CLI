package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyMissingFile(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Files: []FileEntry{{Path: "a.bin", SHA256: "abc"}}}
	if err := VerifyManifest(dir, m); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestVerifySizeMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Files: []FileEntry{{Path: "a.bin", Size: 999}}}
	if err := VerifyManifest(dir, m); err == nil {
		t.Fatal("expected size mismatch")
	}
}

func TestVerifyUnsafePath(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"../x.bin", "/abs.bin"} {
		m := &Manifest{Files: []FileEntry{{Path: p}}}
		if err := VerifyManifest(dir, m); err == nil {
			t.Fatalf("expected unsafe error for %q", p)
		}
	}
}

func TestInspectDetectsHFFormat(t *testing.T) {
	dir := t.TempDir()
	mdir := filepath.Join(dir, "m-hf")
	if err := os.MkdirAll(mdir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config.json", "model.safetensors"} {
		if err := os.WriteFile(filepath.Join(mdir, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg := NewRegistry(dir)
	info, err := reg.Inspect(mdir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "hf" {
		t.Fatalf("got %q", info.Format)
	}
}
