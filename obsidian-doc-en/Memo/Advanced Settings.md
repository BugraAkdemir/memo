# 🔧 Advanced Settings

Memo offers power users the ability to fine-tune RAG and Model parameters.

> Settings itself was reorganized in v3.3.4: what used to be ~20 flat tabs is now a grouped, searchable rail — type a few letters of what you're looking for instead of scanning every tab.

## Memory (RAG) Settings
- **Top-K (Memory Count):** Determines how many past memories will be retrieved in each query. (Default: 5)
- **Similarity Threshold:** The minimum score required for a memory to be considered "relevant." (e.g., 0.75)
- **Min Similarity:** Used to prevent very irrelevant memories from cluttering the context.
- **Memory context token budget:** The block of retrieved memories + pinned facts injected into every prompt is capped at 4096 tokens (v3.3.4) — previously an unenforced 16K "ceiling," which let worst-case prompt bloat grow unbounded as pinned facts accumulated over a long-lived session.
- **`embedding_gpu_layers`:** The dedicated embedding server now defaults to CPU-only (v3.3.4) to stop it fighting the chat model for VRAM — this option opts it back onto the GPU if you have real headroom to spare.
- **Import Memory From Another AI** (Settings → Import Memory, v3.3.3): paste a description another AI assistant (ChatGPT, Gemini, Claude, ...) gives back about you, and Memo breaks it into atomic facts saved the same way `/remember` does, plus a standing "how you like to be talked to" summary folded into the system prompt.
- **Minimal Mode:** a toggle for running a local model with as little prompt overhead as possible — skips personality/mood/web-search instructions entirely. As of v3.3.3, four pieces (persona/system-prompt, capability disclosures, passive-feature disclosures, proactive learning) can each be re-enabled independently instead of all-or-nothing.

## Model Parameters
- **Temperature:** Determines how "creative" or "consistent" the answers will be. (0.0 - 1.0)
- **Repeat Penalty:** Prevents the model from repeating the same words.
- **GPU Layers:** Number of layers to offload to the VRAM. Full performance is recommended by offloading all layers to the GPU.

## Network and Access
- **Remote Access:** When this setting is turned on, Memo opens for access from other devices on the local network (Wi-Fi). Every remote request now requires the access token (v3.3.3 security fix) — previously anyone reachable on the same network or ngrok link could read provider API keys or run commands with zero credentials.
- **Port Settings:** You can change the default 8090 port in case of a conflict.
- **Tailscale** (Settings → Remote Access) graduated out of Beta in v3.3.4: one-click interactive login (no auth key to paste anymore), Funnel on by default, and auto-reconnect after a dropped connection or a killed listener.
- **Developer API Gateway** (sidebar → Developer, not inside Settings): an Anthropic-compatible `/v1/messages` endpoint for tools like Claude Code — see [[Developer API Gateway]].

## Beta Features (Settings → Beta Features)
- Single master switch for experimental features (previously nested under Remote Access; Tailscale itself moved back out of Beta in v3.3.4).
- **On:** unlocks [[Memo Swarm]] and Live Mode (see [[Multimodal Capabilities (Vision and Voice)]], v3.3.4) — hands-free voice chat, plus its own local voice picker and TTS provider config.
- **Off:** Swarm nav and Live Mode's icon stay hidden / disabled.
- Each beta feature is configured on its own screen (Swarm → sidebar; Live Mode → the icon next to the chat input, configured from Beta Features).

## Report a Bug (Settings → Report Bug, v3.3.3)
Describe what happened, optionally attach your last 10 background error events, and Memo opens a prefilled GitHub issue in your browser — nothing is sent anywhere until you review and submit it yourself on GitHub, with your own account.

### Linked Notes:
- [[Vector Search Logic]]
- [[API Documentation]]
- [[Memo Swarm]]
- [[Developer API Gateway]]
