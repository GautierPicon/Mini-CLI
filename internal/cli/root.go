package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/GautierPicon/mini-cli/internal/chat"
	"github.com/GautierPicon/mini-cli/internal/config"
	"github.com/GautierPicon/mini-cli/internal/inference"
	"github.com/GautierPicon/mini-cli/internal/model"
	"github.com/GautierPicon/mini-cli/internal/tui"
)

func Execute() error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		cfg = config.Defaults()
	}
	root := NewRootCmd(cfg)
	return root.Execute()
}

func NewRootCmd(cfg config.Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "mini",
		Short: "Mini: run a small language model locally",
		Long: `Mini CLI — download, manage and chat with a small local model.

Inference runs offline once model files are installed. No account, no API key.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newChatCmd(cfg), newRunCmd(cfg), newModelsCmd(cfg))
	return root
}

func resolveGenerator(cfg config.Config, modelFlag string, useMock bool) (inference.Generator, string, func(), error) {
	if useMock {
		return &inference.MockGenerator{}, "mock (dev only, not the real model)", func() {}, nil
	}
	reg := model.NewRegistry(cfg.ModelsDir)
	idOrPath := modelFlag
	if idOrPath == "" {
		idOrPath = cfg.DefaultModel
	}
	if idOrPath == "" {
		return nil, "", func() {}, fmt.Errorf("no model configured.\nInstall one with: mini models pull <id>\nOr point at a directory: mini chat --model ./path/to/model")
	}
	dir, err := reg.Resolve(idOrPath)
	if err != nil {
		return nil, "", func() {}, err
	}
	pythonBin := os.Getenv("MINI_PYTHON")
	enginePath := cfg.EnginePath
	gen, err := inference.NewLocalGenerator(dir, enginePath, pythonBin)
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("cannot start local engine: %w\nTried python=%q. Fix: export MINI_PYTHON=<mini-llm venv python> (e.g. ../mini-llm/.venv/bin/python)%s", err, inference.ResolvePythonBin(), engineHint())
	}
	return gen, dir, func() { _ = gen.Close() }, nil
}

func engineHint() string {
	return " (see Mini-CLI README: pip install torch tokenizers transformers, or uv sync in mini-llm)"
}

func generationOptions(cfg config.Config, maxTokens int, temperature float64) inference.GenerationOptions {
	opts := inference.DefaultOptions()
	if cfg.MaxTokens > 0 {
		opts.MaxTokens = cfg.MaxTokens
	}
	if cfg.Temperature != 0 {
		opts.Temperature = cfg.Temperature
	}
	if maxTokens > 0 {
		opts.MaxTokens = maxTokens
	}
	if temperature > 0 {
		opts.Temperature = temperature
	}
	return opts
}

func newChatCmd(cfg config.Config) *cobra.Command {
	var modelFlag string
	var maxTokens int
	var temperature float64
	var mock bool
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive conversation",
		RunE: func(cmd *cobra.Command, args []string) error {
			gen, label, cleanup, err := resolveGenerator(cfg, modelFlag, mock)
			if err != nil {
				return err
			}
			defer cleanup()
			if mock {
				fmt.Fprintln(os.Stderr, "mock mode: replies are fake and for development only")
			}
			opts := generationOptions(cfg, maxTokens, temperature)
			conv := chat.New(gen, opts)
			title := fmt.Sprintf("Mini [%s]  (T=%.2g, max_tokens=%d)", filepath.Base(label), opts.Temperature, opts.MaxTokens)
			p := tea.NewProgram(tui.New(title, conv), tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
	cmd.Flags().StringVar(&modelFlag, "model", "", "model id or directory")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "max new tokens")
	cmd.Flags().Float64Var(&temperature, "temperature", 0, "sampling temperature")
	cmd.Flags().BoolVar(&mock, "mock", false, "use deterministic fake replies (dev only)")
	return cmd
}

func newRunCmd(cfg config.Config) *cobra.Command {
	var modelFlag string
	var maxTokens int
	var temperature float64
	var mock bool
	cmd := &cobra.Command{
		Use:   "run <prompt>",
		Short: "Run a single prompt and print the reply",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")
			gen, _, cleanup, err := resolveGenerator(cfg, modelFlag, mock)
			if err != nil {
				return err
			}
			defer cleanup()
			opts := generationOptions(cfg, maxTokens, temperature)
			text, err := gen.Generate([]inference.Message{{Role: "user", Content: prompt}}, opts)
			if err != nil {
				return err
			}
			fmt.Println(strings.TrimSpace(text))
			return nil
		},
	}
	cmd.Flags().StringVar(&modelFlag, "model", "", "model id or directory")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "max new tokens")
	cmd.Flags().Float64Var(&temperature, "temperature", 0, "sampling temperature")
	cmd.Flags().BoolVar(&mock, "mock", false, "use deterministic fake replies (dev only)")
	return cmd
}

func newModelsCmd(cfg config.Config) *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "Manage installed models"}
	cmd.AddCommand(
		newModelsListCmd(cfg),
		newModelsInfoCmd(cfg),
		newModelsPullCmd(cfg),
	)
	return cmd
}

func newModelsListCmd(cfg config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed models",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := model.NewRegistry(cfg.ModelsDir)
			list, err := reg.List()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Printf("No models installed in %s\nInstall one with: mini models pull <id>\n", cfg.ModelsDir)
				return nil
			}
			for _, m := range list {
				status := m.Detail
				fmt.Printf("%-28s %-8s %10.1f MB  %s\n  %s\n", m.ID, m.Format, float64(m.SizeBytes)/1e6, status, m.Path)
			}
			return nil
		},
	}
}

func newModelsInfoCmd(cfg config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "info <id|path>",
		Short: "Show model metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := model.NewRegistry(cfg.ModelsDir)
			dir, err := reg.Resolve(args[0])
			if err != nil {
				return err
			}
			info, err := reg.Inspect(dir)
			if err != nil {
				return err
			}
			fmt.Printf("id:      %s\npath:    %s\nformat:  %s\nsize:    %.1f MB\nstatus:  %s\n", info.ID, info.Path, info.Format, float64(info.SizeBytes)/1e6, info.Detail)
			if info.Manifest != nil {
				m := info.Manifest
				fmt.Printf("name:    %s\nversion: %s\narch:    %s\ntokenizer: %s\n", m.Name, m.Version, m.Architecture, m.Tokenizer)
				if m.VocabSize > 0 {
					fmt.Printf("vocab:   %d\n", m.VocabSize)
				}
				if m.Context > 0 {
					fmt.Printf("context: %d\n", m.Context)
				}
			} else {
				fmt.Printf("(no manifest.json: showing only detected fields)\n")
			}
			return nil
		},
	}
}

func newModelsPullCmd(cfg config.Config) *cobra.Command {
	var registryURL string
	return &cobra.Command{
		Use:   "pull <id>",
		Short: "Download a model from the configured registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if registryURL == "" {
				registryURL = cfg.RegistryURL
			}
			reg := model.NewRegistry(cfg.ModelsDir)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			client := &http.Client{Timeout: 60 * time.Second}
			dest, err := model.Pull(ctx, reg, id, registryURL, client)
			if err != nil {
				return err
			}
			fmt.Printf("Installed %s -> %s\n", id, dest)
			return nil
		},
	}
}
