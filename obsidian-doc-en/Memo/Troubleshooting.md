# Troubleshooting Guide

Common issues and their solutions for Memo.

---

## 1. Port Conflicts (Address already in use)

**Issue**: Backend fails with `listen tcp :8090: bind: address already in use`.

**Solution**:
- Kill existing process: `fuser -k 8090/tcp` (Linux)
- Or use a different port: `go run . --port 8091`

## 2. VRAM / GPU Issues

**Issue**: Model fails to load or runs very slowly on CPU.

**Solution**:
- Large models (7B+) may not fit in 4GB-6GB VRAM with high `n_gpu_layers`
- Ensure correct `llama.cpp` binary (CUDA for NVIDIA, ROCm for AMD)
- Decrease `n_gpu_layers` in Settings to offload more to CPU

## 3. Llama Server Not Found

**Issue**: Backend logs show `llama-server binary not found`.

**Solution**:
- Check `data/bin/` — Memo downloads this automatically
- If failed, manually download `llama-server` from official `llama.cpp` releases and place in `data/bin/`

## 4. Blank Frontend Screen

**Issue**: Flutter app opens but shows white screen.

**Solution**:
- Ensure backend is running on port 8090
- Check `server.log` for panic errors
- Check firewall isn't blocking localhost:8090

## 5. Memory Retrieval Failures

**Issue**: AI doesn't seem to remember past conversations.

**Solution**:
- "Incognito Mode" disables memory — check if active
- Ensure an embedding model is loaded (RAG requires active embedding server, usually port 8082)
- Check memory store is active — uses SQLite (sqlite-vec) for vector storage

## 6. External Provider Connection Issues

**Issue**: "Failed to connect" or "Authentication error" with providers.

**Solution**:
- Verify API key in Settings → API Providers
- Use "Test Connection" before saving
- Check provider base URL (especially for custom endpoints)
- Rate limits: some providers (Grok, Gemini free tier) have strict limits
- Router auto-disables after 3 failures — re-enable from provider settings

## 7. Agent Mode Not Working

**Issue**: Agent mode doesn't respond or tool calls fail.

**Solution**:
- Agent mode requires active external provider (local llama.cpp doesn't support tool calling reliably)
- Enable via API: `PUT /api/agent/enabled {"enabled": true}`
- Agent frontend UI is not yet implemented — use backend API directly
- Check `data/permissions.json` if policies are blocking tools
