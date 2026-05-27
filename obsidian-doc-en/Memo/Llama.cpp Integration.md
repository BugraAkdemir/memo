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

### Linked Notes:
- [[Model Management (The Factory)]]
- [[Backend (Go) Architecture]]
