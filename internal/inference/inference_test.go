package inference

import "testing"

func TestMockGeneratorDeterministic(t *testing.T) {
	m := &MockGenerator{}
	opts := DefaultOptions()
	a, err := m.Generate([]Message{{Role: "user", Content: "hello"}}, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Generate([]Message{{Role: "user", Content: "hello"}}, opts)
	if a == b {
		t.Fatal("calls should differ (counter)")
	}
	if m.Calls != 2 {
		t.Fatalf("calls=%d", m.Calls)
	}
}

func TestHistoryToPromptFormat(t *testing.T) {
	got := HistoryToPrompt([]Message{
		{Role: "user", Content: " hi "},
		{Role: "assistant", Content: "hello"},
		{Role: "user", Content: "bye"},
	})
	want := "User: hi\nAssistant: hello\nUser: bye\nAssistant:"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
