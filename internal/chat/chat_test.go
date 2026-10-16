package chat

import (
	"testing"

	"github.com/GautierPicon/mini-cli/internal/inference"
)

func TestSendAppendsHistory(t *testing.T) {
	c := New(&inference.MockGenerator{}, inference.DefaultOptions())
	ans, err := c.Send("hello")
	if err != nil {
		t.Fatal(err)
	}
	if ans == "" || len(c.History) != 2 {
		t.Fatalf("got %q %+v", ans, c.History)
	}
	if c.History[0].Role != "user" || c.History[1].Role != "assistant" {
		t.Fatalf("%+v", c.History)
	}
	c.Reset()
	if len(c.History) != 0 {
		t.Fatal("reset failed")
	}
}

func TestSendEmptyRejected(t *testing.T) {
	c := New(&inference.MockGenerator{}, inference.DefaultOptions())
	if _, err := c.Send("   "); err == nil {
		t.Fatal("expected error")
	}
}
