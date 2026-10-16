package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	DefaultModel string  `json:"default_model,omitempty"`
	ModelsDir    string  `json:"models_dir,omitempty"`
	EnginePath   string  `json:"engine_path,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
	MaxTokens    int     `json:"max_tokens,omitempty"`
	RegistryURL  string  `json:"registry_url,omitempty"`
}

func Defaults() Config {
	return Config{
		ModelsDir:   DefaultModelsDir(),
		Temperature: 0.5,
		MaxTokens:   256,
	}
}

func DefaultModelsDir() string {
	if v := os.Getenv("MINI_MODELS_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "mini", "models")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "mini", "models")
		}
		return filepath.Join(home, "AppData", "Roaming", "mini", "models")
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "mini", "models")
		}
		return filepath.Join(home, ".local", "share", "mini", "models")
	}
}

func ConfigFilePath() string {
	if v := os.Getenv("MINI_CONFIG"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "mini", "config.json")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "mini", "config.json")
		}
		return filepath.Join(home, "AppData", "Roaming", "mini", "config.json")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "mini", "config.json")
		}
		return filepath.Join(home, ".config", "mini", "config.json")
	}
}

func Load() (Config, error) {
	cfg := Defaults()
	path := ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnv(cfg), nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.ModelsDir == "" {
		cfg.ModelsDir = DefaultModelsDir()
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.5
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 256
	}
	return applyEnv(cfg), nil
}

func applyEnv(cfg Config) Config {
	if v := os.Getenv("MINI_MODELS_DIR"); v != "" {
		cfg.ModelsDir = v
	}
	if v := os.Getenv("MINI_DEFAULT_MODEL"); v != "" {
		cfg.DefaultModel = v
	}
	if v := os.Getenv("MINI_ENGINE_PATH"); v != "" {
		cfg.EnginePath = v
	}
	return cfg
}

func Save(cfg Config) error {
	path := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
