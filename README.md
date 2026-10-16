# Mini CLI (`mini`)

Go CLI to download, manage and chat with Mini locally. No account, no API key, no remote backend for inference.

- `mini chat` — interactive conversation (Bubble Tea). Works offline once the model is installed.
- `mini run "prompt"` — single prompt, stdout = answer, stderr = diagnostics (scriptable).
- `mini models list | info | pull` — local registry, sha256 verification, atomic install.

Real inference is done by `python/mini_engine.py` (PyTorch + `mini_llm`), spawned as a child process over JSON lines on stdio. The Go binary never reimplements the network and never converts weights blindly.

## Prerequisites

- Go >= 1.22 (tested with `go version`: 1.27.1)
- Python 3.11+ with the `mini-llm` venv for real inference:
  ```
  cd ../mini-llm
  uv sync
  ```
  Needs `torch`, `tokenizers`, `transformers`, `safetensors` (already in `mini-llm/pyproject.toml`).
- Model files: either
  - native: `models/16k/` (`*.pt` + `tok.json`), or
  - HF export: `exports/mini-llm-assistant-34M-hf/` (`config.json` + `model.safetensors` + `tokenizer.json`)

`mini chat --mock` / `mini run --mock` work without any model (deterministic fake replies, dev only).

## Install

```bash
cd Mini-CLI
go build -o mini ./cmd/mini
./mini --help
```

Set the engine + python for real inference:

```bash
export MINI_ENGINE_PATH="$PWD/python/mini_engine.py"
export MINI_PYTHON="$PWD/../mini-llm/.venv/bin/python"
# optional:
export MINI_MODELS_DIR="$HOME/.local/share/mini/models"  # linux default
export MINI_DEFAULT_MODEL="mini-llm-70m"
```

macOS defaults: `~/Library/Application Support/mini/models`, config `~/Library/Application Support/mini/config.json`.
Linux: `~/.local/share/mini/models`, `~/.config/mini/config.json`.
Windows: `%APPDATA%/mini/models`.

## Usage

```bash
# mock (no model needed)
./mini run --mock "Hello"
./mini chat --mock

# real local model (needs MINI_PYTHON + MINI_ENGINE_PATH)
./mini run --model ../mini-llm/models/16k "Hello"
./mini run --model ../mini-llm/exports/mini-llm-assistant-34M-hf "Hello" --max-tokens 64 --temperature 0.5

# models
./mini models list
./mini models info 16k
./mini models info ../mini-llm/exports/mini-llm-assistant-34M-hf
./mini models pull mini-llm-70m --registry https://example.com/models
```

Chat keys: `Enter` send, `/reset` clear history, `/quit` or `Ctrl+C` exit. History is in-memory only, never written to disk.

Conversation format is exactly the training format (`User: ...\nAssistant: ...\n`, see `../mini-llm/src/mini_llm/prompt.py`). The Python engine truncates oldest turns when the 512-token context is full.

## Configuration file

JSON at `ConfigFilePath()` (see `internal/config`). Example:

```json
{
  "default_model": "mini-llm-70m",
  "models_dir": "/data/mini/models",
  "engine_path": "/opt/mini/mini_engine.py",
  "temperature": 0.5,
  "max_tokens": 256,
  "registry_url": "https://example.com/models"
}
```

Env overrides: `MINI_MODELS_DIR`, `MINI_DEFAULT_MODEL`, `MINI_ENGINE_PATH`, `MINI_CONFIG`.

## How inference works

```
mini (Go: Cobra + Bubble Tea)
  └─ LocalGenerator (internal/inference/local.go)
       └─ python3 python/mini_engine.py  (JSONL stdin/stdout, logs on stderr)
            ├─ native .pt via mini_llm.checkpoints.load_model
            └─ HF dir via transformers AutoModelForCausalLM
```

- No TCP port, binds nowhere. The child is killed on `Close()`; no orphan process.
- Ready for streaming later: protocol can add `{"token": ...}` messages.

## Models storage

Each installed model is a directory:

```
$MINI_MODELS_DIR/mini-llm-70m/
  manifest.json
  model.safetensors / *.pt
  config.json / tok.json
  tokenizer.json
```

`manifest.json` (when present) lists `files[{path, sha256, size}]`. `models list` shows `id, format (hf/native-pt/unknown), size, verified|reason`. `models pull` requires a registry URL, downloads to `*.dl-*` tmp, verifies sha256 + sizes, then atomic `rename`. Refuses to overwrite an existing model and refuses `..` / absolute paths in manifests. No official download URL is bundled — `pull` errors clearly until `registry_url` is configured.

## Offline

Once the model directory is present, `mini chat` / `mini run` work offline (no network). First download obviously needs internet.

## Limitations

- Real generation needs Python + torch (~300 MB for 70M fp32). `go build` alone does not bundle them.
- Small 34M/70M models hallucinate; 512-token context; single-turn SFT, so long multi-turn drifts.
- `pull` needs a trusted registry; none is hardcoded.
- CPU is fine; CUDA used automatically by the engine when available.

## Development

```bash
go vet ./...
go test ./...
go build -o mini ./cmd/mini
```

Python side:

```bash
cd ../mini-llm
uv run pytest -q
echo '{"cmd":"load","model_dir":"models/16k"}...' | uv run python ../Mini-CLI/python/mini_engine.py
```

## Layout

```
cmd/mini/main.go
internal/cli/      Cobra commands (testable)
internal/chat/     Conversation + history (uses Generator)
internal/inference/ Generator interface, Mock, Local (stdio sidecar)
internal/model/    Registry, manifest verify, pull
internal/config/   OS paths, JSON config, env overrides
internal/tui/      Bubble Tea chat UI + Lip Gloss styles
python/mini_engine.py  sidecar
```
