package inference

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type LocalGenerator struct {
	modelDir   string
	enginePath string
	pythonBin  string

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  *json.Encoder
	stdout *bufio.Scanner
	closed bool
}

type engineRequest struct {
	Cmd               string    `json:"cmd"`
	ModelDir          string    `json:"model_dir,omitempty"`
	Device            string    `json:"device,omitempty"`
	Messages          []Message `json:"messages,omitempty"`
	MaxTokens         int       `json:"max_tokens,omitempty"`
	Temperature       float64   `json:"temperature,omitempty"`
	TopK              int       `json:"top_k,omitempty"`
	RepetitionPenalty float64   `json:"repetition_penalty,omitempty"`
}

type engineResponse struct {
	Ok    bool   `json:"ok"`
	Text  string `json:"text,omitempty"`
	Model string `json:"model,omitempty"`
	Ctx   int    `json:"ctx,omitempty"`
	Error string `json:"error,omitempty"`
}

func NewLocalGenerator(modelDir, enginePath, pythonBin string) (*LocalGenerator, error) {
	if modelDir == "" {
		return nil, fmt.Errorf("model directory is required")
	}
	if enginePath == "" {
		var err error
		enginePath, err = ResolveEnginePath()
		if err != nil {
			return nil, err
		}
	}
	if pythonBin == "" {
		pythonBin = ResolvePythonBin()
	}
	g := &LocalGenerator{modelDir: modelDir, enginePath: enginePath, pythonBin: pythonBin}
	if err := g.start(); err != nil {
		return nil, err
	}
	if err := g.load(); err != nil {
		_ = g.Close()
		return nil, err
	}
	return g, nil
}

func (g *LocalGenerator) start() error {
	g.cmd = exec.Command(g.pythonBin, g.enginePath)
	env := os.Environ()
	if src := findMiniLLMSrc(g.enginePath); src != "" {
		env = prependPythonPath(env, src)
	}
	g.cmd.Env = env
	g.cmd.Stderr = os.Stderr
	in, err := g.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("engine stdin: %w", err)
	}
	outPipe, err := g.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("engine stdout: %w", err)
	}
	if err := g.cmd.Start(); err != nil {
		return fmt.Errorf("start engine %s: %w (install python deps: torch, tokenizers, transformers)", g.enginePath, err)
	}
	g.stdin = json.NewEncoder(in)
	g.stdout = bufio.NewScanner(outPipe)
	g.stdout.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	return nil
}

func (g *LocalGenerator) load() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.stdin.Encode(engineRequest{Cmd: "load", ModelDir: g.modelDir, Device: "auto"}); err != nil {
		return fmt.Errorf("engine load request: %w", err)
	}
	resp, err := g.nextResponseLocked()
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("engine load failed: %s", resp.Error)
	}
	return nil
}

func (g *LocalGenerator) nextResponseLocked() (engineResponse, error) {
	if !g.stdout.Scan() {
		err := g.stdout.Err()
		if err == nil {
			err = fmt.Errorf("engine exited (stderr may contain the cause)")
		}
		return engineResponse{}, fmt.Errorf("engine read: %w", err)
	}
	var resp engineResponse
	if err := json.Unmarshal(g.stdout.Bytes(), &resp); err != nil {
		return engineResponse{}, fmt.Errorf("engine bad JSON: %w", err)
	}
	return resp, nil
}

func (g *LocalGenerator) Generate(messages []Message, options GenerationOptions) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return "", fmt.Errorf("engine is closed")
	}
	if options.MaxTokens <= 0 {
		options.MaxTokens = 256
	}
	req := engineRequest{
		Cmd:               "generate",
		Messages:          messages,
		MaxTokens:         options.MaxTokens,
		Temperature:       options.Temperature,
		TopK:              options.TopK,
		RepetitionPenalty: options.RepetitionPenalty,
	}
	if req.Temperature == 0 {
		req.Temperature = 0.5
	}
	if req.TopK == 0 {
		req.TopK = 40
	}
	if req.RepetitionPenalty == 0 {
		req.RepetitionPenalty = 1.15
	}
	if err := g.stdin.Encode(req); err != nil {
		return "", fmt.Errorf("engine generate request: %w", err)
	}
	resp, err := g.nextResponseLocked()
	if err != nil {
		return "", err
	}
	if !resp.Ok {
		return "", fmt.Errorf("generation failed: %s", resp.Error)
	}
	return resp.Text, nil
}

func (g *LocalGenerator) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	g.closed = true
	_ = g.stdin.Encode(engineRequest{Cmd: "quit"})
	if g.cmd != nil && g.cmd.Process != nil {
		_ = g.cmd.Process.Kill()
		_ = g.cmd.Wait()
	}
	return nil
}

func ResolvePythonBin() string {
	if p := os.Getenv("MINI_PYTHON"); p != "" {
		return p
	}
	for _, c := range candidateVenvPythons() {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return "python3"
}

func candidateVenvPythons() []string {
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		add(filepath.Join(d, "..", "mini-llm", ".venv", "bin", "python"))
		add(filepath.Join(d, "..", "..", "mini-llm", ".venv", "bin", "python"))
		add(filepath.Join(d, "..", "..", "Mini", "mini-llm", ".venv", "bin", "python"))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "..", "mini-llm", ".venv", "bin", "python"))
		add(filepath.Join(cwd, "mini-llm", ".venv", "bin", "python"))
		add(filepath.Join(cwd, "..", "Mini", "mini-llm", ".venv", "bin", "python"))
		add(filepath.Join(cwd, "..", "mini-llm", ".venv", "bin", "python3"))
	}
	add("/Users/gautierpicon/Projets/Mini/mini-llm/.venv/bin/python")
	return out
}

func findMiniLLMSrc(enginePath string) string {
	candidates := []string{}
	if enginePath != "" {
		abs, _ := filepath.Abs(enginePath)
		candidates = append(candidates,
			filepath.Join(filepath.Dir(abs), "..", "..", "mini-llm", "src"),
			filepath.Join(filepath.Dir(abs), "..", "mini-llm", "src"),
		)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(d, "..", "mini-llm", "src"),
			filepath.Join(d, "..", "..", "mini-llm", "src"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "..", "mini-llm", "src"),
			filepath.Join(cwd, "mini-llm", "src"),
			filepath.Join(cwd, "..", "Mini", "mini-llm", "src"),
		)
	}
	candidates = append(candidates, "/Users/gautierpicon/Projets/Mini/mini-llm/src")
	for _, c := range candidates {
		clean := filepath.Clean(c)
		if st, err := os.Stat(filepath.Join(clean, "mini_llm")); err == nil && st.IsDir() {
			return clean
		}
	}
	return ""
}

func prependPythonPath(env []string, src string) []string {
	const key = "PYTHONPATH"
	for i, kv := range env {
		if len(kv) > len(key)+1 && kv[:len(key)+1] == key+"=" {
			old := kv[len(key)+1:]
			if old == "" {
				env[i] = key + "=" + src
			} else {
				env[i] = key + "=" + src + string(os.PathListSeparator) + old
			}
			return env
		}
	}
	return append(env, key+"="+src)
}

func ResolveEnginePath() (string, error) {
	if p := os.Getenv("MINI_ENGINE_PATH"); p != "" {
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		} else {
			return "", fmt.Errorf("MINI_ENGINE_PATH %q not found: %w", p, statErr)
		}
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "mini_engine.py"),
			filepath.Join(filepath.Dir(exe), "..", "python", "mini_engine.py"),
			filepath.Join(filepath.Dir(exe), "..", "..", "python", "mini_engine.py"),
		)
	}
	candidates = append(candidates,
		"python/mini_engine.py",
		"Mini-CLI/python/mini_engine.py",
		"../Mini-CLI/python/mini_engine.py",
	)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("mini_engine.py not found (set MINI_ENGINE_PATH). Searched near the mini binary and ./python/mini_engine.py")
}
