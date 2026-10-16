package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manifest struct {
	ID           string      `json:"id"`
	Name         string      `json:"name,omitempty"`
	Version      string      `json:"version,omitempty"`
	Architecture string      `json:"architecture,omitempty"`
	Tokenizer    string      `json:"tokenizer,omitempty"`
	VocabSize    int         `json:"vocab_size,omitempty"`
	Context      int         `json:"context_length,omitempty"`
	Files        []FileEntry `json:"files,omitempty"`
	Source       string      `json:"source,omitempty"`
}

type FileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size,omitempty"`
}

type ModelInfo struct {
	ID          string
	Path        string
	HasManifest bool
	Valid       bool
	SizeBytes   int64
	Manifest    *Manifest
	Format      string
	Detail      string
}

type Registry struct {
	Dir string
}

func NewRegistry(dir string) *Registry { return &Registry{Dir: dir} }

func (r *Registry) List() ([]ModelInfo, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read models dir %s: %w", r.Dir, err)
	}
	var out []ModelInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := r.Inspect(filepath.Join(r.Dir, e.Name()))
		if err != nil {
			out = append(out, ModelInfo{ID: e.Name(), Path: filepath.Join(r.Dir, e.Name()), Detail: "unreadable: " + err.Error()})
			continue
		}
		out = append(out, *info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *Registry) Resolve(idOrPath string) (string, error) {
	if idOrPath == "" {
		return "", fmt.Errorf("no model specified")
	}
	if isPath(idOrPath) {
		abs, err := filepath.Abs(idOrPath)
		if err != nil {
			return "", err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("model path %q not found: %w", idOrPath, err)
		}
		if !st.IsDir() {
			return "", fmt.Errorf("model path %q is not a directory", idOrPath)
		}
		return abs, nil
	}
	if err := ValidateID(idOrPath); err != nil {
		return "", err
	}
	candidate := filepath.Join(r.Dir, idOrPath)
	st, err := os.Stat(candidate)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("model %q is not installed.\nInstall it with: mini models pull %s", idOrPath, idOrPath)
	}
	return candidate, nil
}

func (r *Registry) Inspect(dir string) (*ModelInfo, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	info := &ModelInfo{ID: filepath.Base(dir), Path: dir}
	info.SizeBytes = dirSize(dir)
	info.Format = detectFormat(dir)
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			info.Detail = "no manifest (manually placed?)"
			return info, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		info.Detail = "invalid manifest.json: " + err.Error()
		return info, nil
	}
	info.HasManifest = true
	info.Manifest = &m
	if m.ID != "" {
		info.ID = m.ID
	}
	if err := VerifyManifest(dir, &m); err != nil {
		info.Valid = false
		info.Detail = "validation failed: " + err.Error()
	} else {
		info.Valid = true
		info.Detail = "verified"
	}
	return info, nil
}

func isPath(s string) bool {
	return strings.Contains(s, "/") || strings.Contains(s, string(os.PathSeparator)) || strings.HasPrefix(s, ".")
}

func ValidateID(id string) error {
	if id == "" || id == "." || id == ".." {
		return fmt.Errorf("invalid model id %q", id)
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.ContainsRune(id, 0) {
		return fmt.Errorf("invalid model id %q: must be a plain name", id)
	}
	if strings.HasPrefix(id, "-") {
		return fmt.Errorf("invalid model id %q", id)
	}
	return nil
}

func detectFormat(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
		if _, err := os.Stat(filepath.Join(dir, "model.safetensors")); err == nil {
			return "hf"
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.pt"))
	if len(matches) > 0 {
		if _, err := os.Stat(filepath.Join(dir, "tok.json")); err == nil {
			return "native-pt"
		}
		return "native-pt"
	}
	if _, err := os.Stat(filepath.Join(dir, "tokenizer.json")); err == nil {
		return "hf"
	}
	return "unknown"
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func VerifyManifest(dir string, m *Manifest) error {
	for _, f := range m.Files {
		clean := filepath.Clean(f.Path)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("unsafe manifest path %q", f.Path)
		}
		full := filepath.Join(dir, clean)
		st, err := os.Stat(full)
		if err != nil {
			return fmt.Errorf("missing file %s: %w", f.Path, err)
		}
		if f.Size > 0 && st.Size() != f.Size {
			return fmt.Errorf("size mismatch for %s: got %d, want %d", f.Path, st.Size(), f.Size)
		}
		if f.SHA256 != "" {
			got, err := sha256File(full)
			if err != nil {
				return err
			}
			if !equalHex(got, f.SHA256) {
				return fmt.Errorf("sha256 mismatch for %s", f.Path)
			}
		}
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func equalHex(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
