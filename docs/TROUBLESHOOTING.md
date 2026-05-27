# Troubleshooting Guide

Common issues and their solutions for Memo.

## 1. Port Conflicts (Address already in use)
**Issue:** The backend fails to start with an error like `listen tcp :8090: bind: address already in use`.
**Solution:**
- Another instance of Memo or a different service is using port 8090.
- Kill the existing process: `fuser -k 8090/tcp` (Linux) or use a different port: `go run . --port 8091`.

## 2. VRAM / GPU Issues
**Issue:** Model fails to load or runs very slowly on CPU.
**Solution:**
- Check your VRAM availability. Large models (e.g., 7B+) might not fit in 4GB-6GB VRAM if `n_gpu_layers` is set too high.
- Ensure you have the correct `llama.cpp` binary (CUDA for NVIDIA, ROCm for AMD).
- Decrease `n_gpu_layers` in Settings to offload more to CPU if VRAM is tight.

## 3. Llama Server Not Found
**Issue:** Backend logs show `llama-server binary not found`.
**Solution:**
- Memo usually downloads this automatically. Check `data/bin/`.
- If it fails, manually download the `llama-server` binary for your OS from the official `llama.cpp` releases and place it in `data/bin/`.

## 4. Blank Frontend Screen
**Issue:** Flutter app opens but shows a white screen or doesn't load data.
**Solution:**
- Ensure the backend is running on the correct port (default 8090).
- Check `server.log` for any panic errors.
- Ensure your firewall isn't blocking local connections on port 8090.

## 5. Memory Retrieval Failures
**Issue:** The AI doesn't seem to remember past conversations.
**Solution:**
- Check if "Incognito Mode" is active (it disables memory).
- Ensure an embedding model is loaded. RAG requires an active embedding server (usually port 8082).
- Check `data/memory/` to see if `.gob` files are being generated.
