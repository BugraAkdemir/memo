# ⚙️ Llama.cpp Integration

Memo uses `llama.cpp`, the world's most popular and highest-performing local engine for LLM inference.

## Integration Details
The Backend manages the `llama-server` binary as a subprocess.

### Intelligent Startup
When a model is started, Memo automatically sets these parameters:
- `--port`: Listening port.
- `--model`: GGUF file path.
- `--n-gpu-layers`: Number of layers to offload to the GPU based on VRAM capacity.
- `--ctx-size`: Context window size.
- `--embedding`: Added if starting as an embedding server.

## Automatic Installation (Llama Installer)
Users do not need to manually compile `llama.cpp`.
1. The system detects the operating system (Linux/Windows) and hardware (Cuda/CPU).
2. It downloads the most compatible pre-built binary from secure servers.
3. It places it in the `data/bin/` directory, making it ready for use.

## Performance Monitoring
Metrics from `llama.cpp` during chat are captured in real-time and presented to the user as `tokens per second` (t/s).

## Reliability Fixes

- **Stuck port auto-clear (v3.3.3):** if the embedding (or chat) model failed to start even once — usually a leftover process from an earlier crash still holding its port — it previously stayed broken until a full reboot, since every retry failed for the exact same reason. Memo now clears a stuck port automatically before every start attempt.
- **Embedding server defaults to CPU-only (v3.3.4):** it used to auto-start with GPU auto-detect as if it were the only model running, even while the chat model's own server already occupied most of the VRAM — the two together could push the chat model into partial CPU fallback, cutting local generation speed 4-5x. It now defaults to CPU (small enough to stay fast there); `embedding_gpu_layers` opts it back onto the GPU when there's real headroom.
- **Real max context read from the GGUF file (v3.3.3):** context size used to be an unbounded free-text field that could crash the model server; it's now clamped to the model's actual maximum.
- **Memo Swarm (Beta, v3.3.3):** for a model too large for one machine, `llama-server --rpc` on a Host plus `rpc-server` on Join machines pools compute across several PCs — see [[Memo Swarm]].

### Linked Notes:
- [[Model Management (The Factory)]]
- [[Backend (Go) Architecture]]
- [[Memo Swarm]]
