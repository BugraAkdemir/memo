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
- External providers work reliably; local llama.cpp support depends on the model's own tool-calling capability — starting a model that doesn't support it now shows a warning up front (v3.3.4)
- Toggle it from Chat's top bar (next to web search), or via API: `PUT /api/agent/enabled {"enabled": true}`
- Check `data/permissions.json` if policies are blocking tools
- **Fails on a one-word message with a local model + small context?** Fixed in v3.3.4 — agent mode's tool definitions weren't being counted against the context budget, so even "hi" could trip a "request exceeds the available context size" error. Update to v3.3.4+, or raise the model's context size / lower tool count in the meantime.

## 8. macOS: "Connection Error" on Launch

**Issue**: The desktop app opens but immediately shows a connection error talking to the local backend — macOS only.

**Solution**:
- Fixed in commit `420e6a5` (see [[Build and Packaging]]): the macOS App Sandbox build was missing the `network.client` entitlement, which silently blocked the Flutter frontend's Dio calls to `localhost:8090`. Update to a build that includes this fix.
- The same commit also fixes mic access (`device.audio-input`, needed for voice input / Live Mode) and file-picker access (`files.user-selected.read-write`, needed for backup/restore and chat attachments) being silently blocked for the same reason.
- If building from source on macOS, check `frontend/macos/Runner/Release.entitlements` / `DebugProfile.entitlements` include all three, and that `Info.plist` has `NSMicrophoneUsageDescription`.

## 9. Windows: Fails to Launch With a Missing `msvcp140.dll` Error

**Issue**: A clean Windows machine without the Visual C++ Runtime already installed can't start Memo at all.

**Solution**: Fixed in v3.3.4 — the installer now bundles the Visual C++ Redistributable and installs it silently. Update to v3.3.4+, or install the [Microsoft VC++ Redistributable](https://learn.microsoft.com/cpp/windows/latest-supported-vc-redist) manually in the meantime.

## 10. Windows: "Delete All Data" Fails

**Issue**: Settings → Backup's factory reset works on Linux but fails on Windows.

**Solution**: Fixed in v3.3.4 — several internal databases (memory, usage stats, calendar, mood, WhatsApp) were still open when the wipe tried to delete their files, which Windows refuses for a file still in use by the same process. Update to v3.3.4+.
