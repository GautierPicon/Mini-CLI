package inference

import (
	"os"
	"path/filepath"
	"testing"
)

const fakeEngine = `import sys, json
for line in sys.stdin:
    line=line.strip()
    if not line:
        continue
    try:
        req=json.loads(line)
    except Exception as e:
        print(json.dumps({"ok": False, "error": str(e)}), flush=True)
        continue
    cmd=req.get("cmd")
    if cmd=="load":
        print(json.dumps({"ok": True, "model": "fake", "ctx": 512}), flush=True)
    elif cmd=="generate":
        msgs=req.get("messages", [])
        last=""
        for m in reversed(msgs):
            if m.get("role")=="user":
                last=m.get("content","")
                break
        print(json.dumps({"ok": True, "text": "fake:"+last}), flush=True)
    elif cmd=="quit":
        print(json.dumps({"ok": True}), flush=True)
        break
    else:
        print(json.dumps({"ok": False, "error": "unknown cmd"}), flush=True)
`

func newFakeEngine(t *testing.T) (modelDir, enginePath string) {
	t.Helper()
	modelDir = t.TempDir()
	enginePath = filepath.Join(t.TempDir(), "fake_engine.py")
	if err := os.WriteFile(enginePath, []byte(fakeEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	return modelDir, enginePath
}

func TestLocalGeneratorWithFakeEngine(t *testing.T) {
	modelDir, engine := newFakeEngine(t)
	g, err := NewLocalGenerator(modelDir, engine, "python3")
	if err != nil {
		t.Skipf("python3 unavailable: %v", err)
	}
	defer g.Close()
	text, err := g.Generate([]Message{{Role: "user", Content: "hello"}}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if text != "fake:hello" {
		t.Fatalf("got %q", text)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Generate([]Message{{Role: "user", Content: "x"}}, DefaultOptions()); err == nil {
		t.Fatal("expected error after Close")
	}
}

func TestLocalGeneratorRequiresModelDir(t *testing.T) {
	if _, err := NewLocalGenerator("", "/nonexistent.py", "python3"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveEnginePathEnvMissing(t *testing.T) {
	t.Setenv("MINI_ENGINE_PATH", "/definitely/not/here.py")
	if _, err := ResolveEnginePath(); err == nil {
		t.Fatal("expected error")
	}
}
