package inference

import "fmt"

// MockGenerator returns deterministic replies for dev and tests.
// It must never be presented as the real model.
type MockGenerator struct {
	Prefix string
	Calls  int
}

func (m *MockGenerator) Generate(messages []Message, options GenerationOptions) (string, error) {
	m.Calls++
	last := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			last = messages[i].Content
			break
		}
	}
	p := m.Prefix
	if p == "" {
		p = "[mock]"
	}
	return fmt.Sprintf("%s echo (%d): %s (max_tokens=%d)", p, m.Calls, last, options.MaxTokens), nil
}

func (m *MockGenerator) Close() error { return nil }
