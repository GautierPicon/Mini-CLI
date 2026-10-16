package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestServer(t *testing.T, manifest Manifest, files map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// manifest
		if len(r.URL.Path) > 0 && endsWith(r.URL.Path, "/manifest.json") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(manifest)
			return
		}
		// files: /<id>/<path>
		parts := splitPath(r.URL.Path)
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		rel := joinPath(parts[1:])
		data, ok := files[rel]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	return httptest.NewServer(mux)
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func splitPath(p string) []string {
	var out []string
	cur := ""
	for _, c := range p {
		if c == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func joinPath(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "/"
		}
		out += p
	}
	return out
}

func shaOf(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestPullSuccess(t *testing.T) {
	content := []byte("hello model")
	m := Manifest{ID: "demo", Files: []FileEntry{{Path: "model.bin", SHA256: shaOf(content), Size: int64(len(content))}}}
	srv := newTestServer(t, m, map[string][]byte{"model.bin": content})
	defer srv.Close()

	reg := NewRegistry(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dest, err := Pull(ctx, reg, "demo", srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "model.bin"))
	if err != nil || string(got) != string(content) {
		t.Fatalf("bad install: %v %q", err, got)
	}
	if _, err := os.Stat(filepath.Join(dest, "manifest.json")); err != nil {
		t.Fatal("manifest not installed")
	}
}

func TestPullRefusesOverwrite(t *testing.T) {
	content := []byte("x")
	m := Manifest{ID: "demo", Files: []FileEntry{{Path: "f.bin", SHA256: shaOf(content), Size: 1}}}
	srv := newTestServer(t, m, map[string][]byte{"f.bin": content})
	defer srv.Close()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Pull(ctx, reg, "demo", srv.URL, srv.Client()); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}

func TestPullDetectsCorruptDownload(t *testing.T) {
	good := []byte("good content")
	m := Manifest{ID: "demo", Files: []FileEntry{{Path: "f.bin", SHA256: shaOf([]byte("different")), Size: int64(len(good))}}}
	srv := newTestServer(t, m, map[string][]byte{"f.bin": good})
	defer srv.Close()

	reg := NewRegistry(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Pull(ctx, reg, "demo", srv.URL, srv.Client()); err == nil {
		t.Fatal("expected sha mismatch")
	}
}

func TestPullRejectsUnsafePath(t *testing.T) {
	m := Manifest{ID: "demo", Files: []FileEntry{{Path: "../evil.bin", SHA256: "abc"}}}
	srv := newTestServer(t, m, map[string][]byte{})
	defer srv.Close()

	reg := NewRegistry(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Pull(ctx, reg, "demo", srv.URL, srv.Client()); err == nil {
		t.Fatal("expected unsafe path error")
	}
}

func TestPullNeedsRegistry(t *testing.T) {
	reg := NewRegistry(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Pull(ctx, reg, "demo", "", nil); err == nil {
		t.Fatal("expected no-registry error")
	}
}
