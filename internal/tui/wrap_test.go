package tui

import (
	"strings"
	"testing"
)

func TestWordWrapBasic(t *testing.T) {
	got := wordWrap("hello world foo bar", 10)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 10 {
			t.Fatalf("line too long %q", line)
		}
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "bar") {
		t.Fatalf("lost words: %q", got)
	}
}

func TestWordWrapPreservesNewlines(t *testing.T) {
	got := wordWrap("a\n\nb", 20)
	if got != "a\n\nb" {
		t.Fatalf("got %q", got)
	}
}

func TestWordWrapBreaksLongWord(t *testing.T) {
	long := strings.Repeat("x", 30)
	got := wordWrap(long, 10)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 10 {
			t.Fatalf("line too long %q", line)
		}
	}
	joined := strings.ReplaceAll(got, "\n", "")
	if joined != long {
		t.Fatal("content changed")
	}
}

func TestWrapMessageWidth(t *testing.T) {
	got := wrapMessage("AI  > ", strings.Repeat("hello ", 20), 30)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 30 {
			t.Fatalf("line too long (%d): %q", len(line), line)
		}
	}
}
