package model

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateIDRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "../x", "a/b", "a\\b", "-x"} {
		if ValidateID(bad) == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
	if err := ValidateID("mini-llm-70m"); err != nil {
		t.Fatal(err)
	}
}

func TestListAndInspect(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	writeFile(t, filepath.Join(dir, "m1", "tok.json"), "{}")
	writeFile(t, filepath.Join(dir, "m1", "model.pt"), "x")
	list, err := reg.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Format != "native-pt" {
		t.Fatalf("got %+v", list)
	}
	info, err := reg.Inspect(filepath.Join(dir, "m1"))
	if err != nil {
		t.Fatal(err)
	}
	if info.HasManifest {
		t.Fatal("should have no manifest")
	}
}

func TestResolveMissingGivesHelpfulError(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	_, err := reg.Resolve("nope")
	if err == nil {
		t.Fatal("expected error")
	}
	want := "mini models pull nope"
	if got := err.Error(); len(got) == 0 || !contains(got, want) {
		t.Fatalf("error should mention %q, got %q", want, got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
