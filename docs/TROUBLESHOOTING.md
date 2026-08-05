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
- Check if memory store is active and embedding model is loaded. The memory store uses SQLite (sqlite-vec) for vector storage.

## 6. External Provider Connection Issues
**Issue:** "Failed to connect" or "Authentication error" when using external providers.
**Solution:**
- Verify your API key is correct in Settings → API Providers.
- Use "Test Connection" to diagnose the issue before saving.
- Check if the provider's base URL is correct (especially for custom endpoints).
- Rate limits: Some providers (e.g., Grok, Gemini free tier) have strict rate limits. Wait and retry.
- The router auto-disables providers after 3 consecutive failures. Re-enable from provider settings.

## 7. Agent Mode Not Working
**Issue:** Agent mode doesn't respond or tool calls fail.
**Solution:**
- Toggle Agent Mode directly in Chat's top bar, next to the web-search toggle — no separate screen or API call needed.
- With a local model, make sure it actually supports tool calling and has enough context: the default local context was raised 4096 → 8192, and a very small context could previously fail even on a one-word message because tool definitions weren't counted against the budget (fixed).
- Check `data/permissions.json` if permission policies are blocking tools.

## 8. macOS: "Connection Error" on Launch
**Issue:** The desktop app opens but immediately shows a connection error talking to its own local backend, or the microphone/file picker silently doesn't work.
**Cause:** The macOS App Sandbox entitlements were missing `network.client` (blocks Dio's calls to `localhost:8090`), `device.audio-input` (blocks the `record` package for Live Mode/voice input), and `files.user-selected.read-write` (blocks `file_picker`); `Info.plist` was also missing `NSMicrophoneUsageDescription`.
**Solution:** Fixed in commit `420e6a5` (`frontend/macos/Runner/Release.entitlements`, `DebugProfile.entitlements`, `Info.plist`). Update to a build that includes this fix.

## 9. Turning On Memory Makes Local Generation Very Slow
**Issue:** Enabling memory/RAG on a local model tanks generation speed (e.g. ~10 tok/s down to 2-3).
**Cause (fixed in v3.3.4):** The embedding server was auto-sizing itself onto the GPU as if it were the only model running, oversubscribing VRAM alongside the chat model's own server and pushing it into partial CPU fallback.
**Solution:** Update to a build with the fix — the embedding server now defaults to CPU-only. If you have real VRAM headroom to spare, `embedding_gpu_layers` in config lets you opt it back onto the GPU.

## 10. Windows: Missing `msvcp140.dll` on Launch
**Issue:** A clean Windows install (common on fresh VMs) fails to start Memo at all with a missing DLL error.
**Solution:** Fixed in v3.3.4 — the installer now bundles and silently installs the Visual C++ Redistributable. Update to a current installer, or install the VC++ Redistributable manually as a workaround on an older build.
