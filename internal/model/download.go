package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Pull downloads a model from a trusted registry base URL into the registry.
// No pull happens without an explicit allowlisted registry URL and a manifest.
func Pull(ctx context.Context, reg *Registry, id, registryURL string, client *http.Client) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	if registryURL == "" {
		return "", fmt.Errorf("no registry configured.\nSet registry_url in the config file or pass --registry <base-url>.\nNo official download source is bundled; refusing to guess a URL")
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	manifestURL := registryURL + "/" + id + "/manifest.json"
	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("registry returned %s for model %q", resp.Status, id)
	}
	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", fmt.Errorf("bad manifest: %w", err)
	}
	if m.ID != "" && m.ID != id {
		return "", fmt.Errorf("manifest id mismatch: got %q, want %q", m.ID, id)
	}
	m.ID = id

	dest := filepath.Join(reg.Dir, id)
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("model %q already installed at %s (refusing to overwrite)", id, dest)
	}

	tmp, err := os.MkdirTemp(filepath.Dir(dest), id+".dl-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	for _, f := range m.Files {
		if err := downloadFile(ctx, client, registryURL+"/"+id+"/"+f.Path, filepath.Join(tmp, f.Path), f); err != nil {
			return "", err
		}
	}
	manifestBytes, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), manifestBytes, 0o644); err != nil {
		return "", err
	}
	if err := VerifyManifest(tmp, &m); err != nil {
		return "", fmt.Errorf("verification after download: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("atomic install: %w", err)
	}
	return dest, nil
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string, want FileEntry) error {
	clean := filepath.Clean(want.Path)
	if filepath.IsAbs(clean) || len(clean) >= 2 && clean[:2] == ".." {
		return fmt.Errorf("unsafe manifest path %q", want.Path)
	}
	const maxBytes = 2 << 30 // 2 GiB per file
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", want.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: registry returned %s", want.Path, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	h := sha256.New()
	n, err := io.Copy(out, io.LimitReader(io.TeeReader(resp.Body, h), maxBytes+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", want.Path, err)
	}
	if n > maxBytes {
		return fmt.Errorf("file %s exceeds 2 GiB limit", want.Path)
	}
	if want.Size > 0 && n != want.Size {
		return fmt.Errorf("size mismatch for %s: got %d, want %d", want.Path, n, want.Size)
	}
	if want.SHA256 != "" && hex.EncodeToString(h.Sum(nil)) != want.SHA256 {
		// case-insensitive compare done by caller normally; keep strict here then retry message
		return fmt.Errorf("sha256 mismatch for %s (incomplete or corrupt download)", want.Path)
	}
	return nil
}
