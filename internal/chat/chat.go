package chat

import (
	"fmt"
	"strings"

	"github.com/GautierPicon/mini-cli/internal/inference"
)

// Conversation keeps in-memory history and delegates generation.
type Conversation struct {
	Generator inference.Generator
	Options   inference.GenerationOptions
	History   []inference.Message
}

// New creates a conversation with default generation options when empty.
func New(g inference.Generator, opts inference.GenerationOptions) *Conversation {
	if opts.MaxTokens == 0 {
		opts = inference.DefaultOptions()
	}
	return &Conversation{Generator: g, Options: opts}
}

// Send appends a user message, generates, appends the reply.
func (c *Conversation) Send(question string) (string, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", fmt.Errorf("empty question")
	}
	c.History = append(c.History, inference.Message{Role: "user", Content: q})
	answer, err := c.Generator.Generate(c.History, c.Options)
	if err != nil {
		c.History = c.History[:len(c.History)-1]
		return "", err
	}
	answer = strings.TrimSpace(answer)
	c.History = append(c.History, inference.Message{Role: "assistant", Content: answer})
	return answer, nil
}

// Reset clears in-memory history. History is never persisted to disk.
func (c *Conversation) Reset() {
	c.History = nil
}
