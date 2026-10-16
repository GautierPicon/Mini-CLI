package inference

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerationOptions struct {
	MaxTokens         int     `json:"max_tokens"`
	Temperature       float64 `json:"temperature"`
	TopP              float64 `json:"top_p,omitempty"`
	TopK              int     `json:"top_k,omitempty"`
	RepetitionPenalty float64 `json:"repetition_penalty,omitempty"`
}

func DefaultOptions() GenerationOptions {
	return GenerationOptions{
		MaxTokens:         256,
		Temperature:       0.5,
		TopK:              40,
		RepetitionPenalty: 1.15,
	}
}

type Generator interface {
	Generate(messages []Message, options GenerationOptions) (string, error)
	Close() error
}

func HistoryToPrompt(messages []Message) string {
	out := ""
	for _, m := range messages {
		switch m.Role {
		case "assistant":
			out += "Assistant: " + trimSpace(m.Content) + "\n"
		default:
			out += "User: " + trimSpace(m.Content) + "\n"
		}
	}
	out += "Assistant:"
	return out
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && isSpace(s[i]) {
		i++
	}
	for j > i && isSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '\r'
}
