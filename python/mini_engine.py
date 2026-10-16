#!/usr/bin/env python3

import glob
import json
import os
import sys
from pathlib import Path

def log(*a):
    print(*a, file=sys.stderr, flush=True)

STATE = {"model": None, "tok": None, "eos": None, "ctx": 512, "kind": None, "dev": "cpu"}

def format_conversation(messages):
    parts = []
    for m in messages:
        role = m.get("role", "user")
        content = (m.get("content") or "").strip()
        if role == "assistant":
            parts.append(f"Assistant: {content}\n")
        else:
            parts.append(f"User: {content}\n")
    parts.append("Assistant:")
    return "".join(parts)

def detect_model_dir(d: Path):
    if (d / "model.safetensors").exists() and (d / "config.json").exists():
        return "hf"
    pts = sorted(glob.glob(str(d / "*.pt")))
    pts = [p for p in pts if not p.endswith(".state.pt")]
    if pts and (d / "tok.json").exists():
        return "native-pt"
    return "unknown"

def load_native(model_dir: Path, device: str):
    try:
        from mini_llm.checkpoints import load_model, load_tokenizer
        from mini_llm.config import resolve_device
    except ImportError as e:
        raise RuntimeError(
            "mini_llm not importable. Run with MINI_PYTHON pointing at the mini-llm uv venv "
            "(uv run -- python Mini-CLI/python/mini_engine.py) or set PYTHONPATH to mini-llm/src. "
            f"({e})"
        )
    dev = resolve_device(device if device else "auto")
    cands = sorted(model_dir.glob("mini-llm-assistant-*.pt"))
    cands = [c for c in cands if not c.name.endswith(".state.pt")]
    if cands:
        ckpt = max(cands, key=lambda p: p.stat().st_mtime)
    else:
        allpts = sorted(model_dir.glob("*.pt"))
        allpts = [c for c in allpts if not c.name.endswith(".state.pt")]
        if not allpts:
            raise RuntimeError(f"no .pt checkpoint in {model_dir}")
        ckpt = max(allpts, key=lambda p: p.stat().st_mtime)
    model, cfg = load_model(str(ckpt), dev)
    model.eval()
    tok = load_tokenizer(str(ckpt))
    eos = tok.token_to_id("<eos>")
    STATE.update(model=model, tok=tok, eos=eos, ctx=cfg.ctx, kind="native-pt", dev=dev)
    return ckpt.name, cfg.ctx

def load_hf(model_dir: Path, device: str):
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer

    tok = AutoTokenizer.from_pretrained(str(model_dir), trust_remote_code=True)
    dtype = torch.float32
    model = AutoModelForCausalLM.from_pretrained(
        str(model_dir), trust_remote_code=True, dtype=dtype
    )
    if device == "cuda":
        try:
            model = model.to("cuda")
            dev = "cuda"
        except Exception:
            dev = "cpu"
    else:
        dev = "cpu"
    model.eval()
    eos = tok.eos_token_id
    if eos is None:
        eos = 0
    ctx = getattr(getattr(model, "config", None), "max_position_embeddings", 512) or 512
    STATE.update(model=model, tok=tok, eos=eos, ctx=ctx, kind="hf", dev=dev)
    return model_dir.name, ctx

def do_load(model_dir: str, device: str):
    d = Path(model_dir)
    if not d.is_dir():
        raise RuntimeError(f"model dir not found: {model_dir}")
    kind = detect_model_dir(d)
    if kind == "native-pt":
        return load_native(d, device or "auto")
    if kind == "hf":
        return load_hf(d, device or "auto")
    raise RuntimeError(
        f"unknown model format in {model_dir}: need (config.json + model.safetensors) or (*.pt + tok.json)"
    )

def do_generate(messages, max_tokens, temperature, top_k, rep_penalty):
    if STATE["model"] is None:
        raise RuntimeError("no model loaded (send load first)")
    prompt = format_conversation(messages or [])
    kind = STATE["kind"]
    if kind == "native-pt":
        import torch

        tok, model, eos, dev, ctx = STATE["tok"], STATE["model"], STATE["eos"], STATE["dev"], STATE["ctx"]
        ids = tok.encode(prompt).ids
        if len(ids) > ctx - 1:
            ids = ids[-(ctx - 1):]
        with torch.no_grad():
            out = model.generate(
                torch.tensor([ids], device=dev),
                n=int(max_tokens or 256),
                temp=float(temperature or 0.5),
                top_k=int(top_k or 40),
                eos=eos,
                rep_penalty=float(rep_penalty or 1.15),
            )
        return tok.decode(out[0, len(ids):].tolist()).strip()
    else:
        import torch

        tok, model, eos, ctx = STATE["tok"], STATE["model"], STATE["eos"], STATE["ctx"]
        ids = tok.encode(prompt)
        if len(ids) > ctx - 1:
            ids = ids[-(ctx - 1):]
        x = torch.tensor([ids])
        try:
            dev = next(model.parameters()).device
            x = x.to(dev)
        except Exception:
            pass
        with torch.no_grad():
            out = model.generate(
                x,
                max_new_tokens=int(max_tokens or 256),
                do_sample=True,
                temperature=float(temperature or 0.5),
                top_k=int(top_k or 40),
                repetition_penalty=float(rep_penalty or 1.15),
                eos_token_id=eos,
                pad_token_id=eos,
            )
        gen = out[0, len(ids):].tolist()
        try:
            text = tok.decode(gen, skip_special_tokens=True)
        except TypeError:
            text = tok.decode(gen)
        text = text.strip()
        if text.endswith("<eos>"):
            text = text[: -len("<eos>")].strip()
        return text

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except Exception as e:
            print(json.dumps({"ok": False, "error": f"bad JSON: {e}"}), flush=True)
            continue
        cmd = req.get("cmd")
        try:
            if cmd == "load":
                name, ctx = do_load(req.get("model_dir", ""), req.get("device", "auto"))
                print(json.dumps({"ok": True, "model": name, "ctx": ctx}), flush=True)
            elif cmd == "generate":
                text = do_generate(
                    req.get("messages", []),
                    req.get("max_tokens", 256),
                    req.get("temperature", 0.5),
                    req.get("top_k", 40),
                    req.get("repetition_penalty", 1.15),
                )
                print(json.dumps({"ok": True, "text": text}), flush=True)
            elif cmd == "quit":
                print(json.dumps({"ok": True}), flush=True)
                break
            else:
                print(json.dumps({"ok": False, "error": f"unknown cmd {cmd!r}"}), flush=True)
        except BrokenPipeError:
            break
        except Exception as e:
            log(f"engine error: {e}")
            print(json.dumps({"ok": False, "error": str(e)}), flush=True)

if __name__ == "__main__":
    main()
