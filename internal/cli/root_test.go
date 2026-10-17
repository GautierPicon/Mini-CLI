package cli

import (
	"strings"
	"testing"

	"github.com/Mini-llm-lab/Mini-CLI/internal/config"
)

func TestRootHasExpectedCommands(t *testing.T) {
	root := NewRootCmd(config.Defaults())
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"chat", "run", "models"} {
		if !names[want] {
			t.Fatalf("missing %q (got %v)", want, names)
		}
	}
}

func TestModelsSubcommands(t *testing.T) {
	root := NewRootCmd(config.Defaults())
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "models" {
			found = true
			subs := map[string]bool{}
			for _, s := range c.Commands() {
				subs[s.Name()] = true
			}
			for _, want := range []string{"list", "info", "pull"} {
				if !subs[want] {
					t.Fatalf("models missing %q", want)
				}
			}
		}
	}
	if !found {
		t.Fatal("models command missing")
	}
}

func TestRunRequiresPrompt(t *testing.T) {
	root := NewRootCmd(config.Defaults())
	root.SetArgs([]string{"run"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("expected arg error, got %v", err)
	}
}
