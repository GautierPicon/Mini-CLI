package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultModelsDirRespectsEnv(t *testing.T) {
	t.Setenv("MINI_MODELS_DIR", "/tmp/mini-models-test")
	if got := DefaultModelsDir(); got != "/tmp/mini-models-test" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINI_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("MINI_MODELS_DIR", filepath.Join(dir, "models"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxTokens != 256 || cfg.Temperature != 0.5 {
		t.Fatalf("bad defaults: %+v", cfg)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINI_CONFIG", filepath.Join(dir, "config.json"))
	os.Unsetenv("MINI_MODELS_DIR")
	cfg := Defaults()
	cfg.DefaultModel = "mini-llm-70m"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	back, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if back.DefaultModel != "mini-llm-70m" {
		t.Fatalf("got %+v", back)
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINI_CONFIG", filepath.Join(dir, "config.json"))
	t.Setenv("MINI_MODELS_DIR", filepath.Join(dir, "env-models"))
	t.Setenv("MINI_DEFAULT_MODEL", "env-model")
	t.Setenv("MINI_ENGINE_PATH", "/tmp/fake_engine.py")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultModel != "env-model" {
		t.Fatalf("got %q", cfg.DefaultModel)
	}
	if cfg.ModelsDir != filepath.Join(dir, "env-models") {
		t.Fatalf("got %q", cfg.ModelsDir)
	}
	if cfg.EnginePath != "/tmp/fake_engine.py" {
		t.Fatalf("got %q", cfg.EnginePath)
	}
}
