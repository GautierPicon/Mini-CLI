# Mini CLI (`mini`)

Go CLI to download, manage and chat with Mini locally. No account, no API key, no
remote backend for inference.

- `mini chat` — interactive conversation (Bubble Tea). Works offline once the model is
  installed.
- `mini run "prompt"` — single prompt, stdout = answer, stderr = diagnostics
  (scriptable).
- `mini models list | info | pull` — local registry, sha256 verification, atomic
  install.

Real inference is done by `python/mini_engine.py` (PyTorch + `mini_llm`), spawned as a
child process over JSON lines on stdio. The Go binary never reimplements the network
and never converts weights blindly.

**Full documentation: [../Documentation/Mini-CLI](../Documentation/Mini-CLI/README.md)**

## Quickstart

Prerequisites:

- Go >= 1.22 (tested with 1.27.1).
- Python 3.11+ with the `mini-llm` venv for real inference:
  ```
  cd ../mini-llm
  uv sync
  ```
- Model files: native `models/<dir>/` (`*.pt` + `tok.json`) or an HF export
  (`config.json` + `model.safetensors` + `tokenizer.json`), both produced by
  [Mini-LLM](../mini-llm).

```
cd Mini-CLI
go build -o mini ./cmd/mini
./mini --help

# mock (no model needed)
./mini run --mock "Hello"

# real local model (the engine and python are auto-detected from ../mini-llm)
./mini run --model ../mini-llm/models/16k "What is 2+2?" --max-tokens 32
./mini chat --model ../mini-llm/models/16k
```

Usage, configuration file, paths, inference details and model storage live in the
[documentation](../Documentation/Mini-CLI/README.md).
