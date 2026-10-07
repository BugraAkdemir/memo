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

## 11. Subscriptions: empty model list, sign-in page, usage limit
**Issue:** After signing in under Settings › Subscriptions the model list is empty, or no browser page opens, or a chat stops with a "usage limit reached" card.
**Cause / Solution:**
- *Empty list right after sign-in* — the bundled CLIProxyAPI helper lists no models for about 30 seconds after it starts. Memo registers the previous session's model immediately and the tab keeps re-reading until the list arrives; wait a moment, the picker re-fetches on every open.
- *Sign-in page never opens* — fixed in v4.6.0 (a relative data folder and a hanging `xdg-open` under KDE). Memo opens the browser itself and also returns the sign-in address so it can be copied by hand. Update if you see it on an older build.
- *"Usage limit reached" card* — the account's allowance for that model ran out. The card counts down to the refill and, with "automatically continue when it resets" on (the default), types `continue` into the chat by itself — only while Memo is open. Pick another model in the top-right selector to carry on immediately.
- *No remaining-percentage badge* — Antigravity and Codex report one; Claude's parser is built from the endpoint's known shape and was not verified against a live Claude sign-in, so a different shape just means no badge.
- *An image model returns an error* — Antigravity's own image model currently answers 500 from Google on the account it was tried with; Codex's image models work.
- Signing out deletes the credential file but does not revoke the vendor grant (the same login may be shared with the vendor's own CLI).

## 12. A long answer shows "no word from the server"
**Issue:** The progress line under the chat turns orange after 30 seconds.
**Cause:** Nothing at all (not even the backend's 10-second `heartbeat`) has arrived for 30 seconds — a dropped connection to a remote Memo, or a stalled backend. A slow but alive model does not trigger it, because the heartbeat keeps the line green.
**Solution:** Check that the backend is running and reachable. A reply is only cut after 300 seconds of true silence (30 minutes in total at most); it is then marked as a timeout and the text received so far is kept.

## 13. The browser panel stays blank or Memo cannot open it
**Issue:** The live browser panel shows nothing, or `browser_*` tools fail.
**Cause:** The browser panel drives an isolated Chromium that is an optional download. Pages are also refused if they are not `http`, `https` or a blank page (`file://` is blocked on purpose).
**Solution:** Install Chromium from the Browser Engine section of Settings › General (the *Download Chromium* button). Agent permission is required for the panel's session endpoints; with Agent Mode off you can still drive the panel by hand if your account has the agent permission.

## 14. "The provider's safety filter refused this request" / other provider messages
**Issue:** The chat shows a sentence like "The provider can't answer right now (HTTP 500)" or "…safety filter refused this request" instead of a raw vendor error.
**Cause:** Since v4.6.0 Memo translates provider failures into plain sentences in your language; the number in brackets is the HTTP status the provider returned. The vendor's original text is in the backend log (search for `ERROR shown to the user as`).
**Solution:** Follow the sentence: reword a refused message; wait and retry on a rate limit or outage; check the key under Settings › API Providers (Subscriptions: sign in again under Settings › Subscriptions); pick another model if the model is unknown or not allowed for the account; start a new chat if it says the conversation no longer fits (Memo normally summarizes at 90% of the window by itself — see the context ring at the foot of the chat).
