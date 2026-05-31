# Known Issues & Technical Risks (Exhaustive Audit)

This document tracks all identified bugs, architectural limitations, and edge cases in the Memo project, updated after a deep codebase audit.

## 🧵 Concurrency & Race Conditions
- **Session Manager Mutex Scope:** In `internal/sessions/sessions.go`, while `mu` is used, some operations like `newSession` called during `NewManager` are done without explicit locks. Rapid switching and adding messages could lead to rare race conditions if not careful.
- **Llama Process Monitoring:** The `waitDone` channel in `internal/llama/llama.go` is closed in a `monitor` goroutine. While mitigated with a `select`, rapid start/stop cycles could theoretically lead to timing issues if the process is killed externally while `monitor` is running.
- **Memory Store Re-init:** In `app.go`, `reinitMemoryStore` replaces the `store` pointer. While protected by `storeMu`, `saveMemoryAsync` spawns a goroutine that *then* tries to acquire the lock. There is a small window where it might refer to an old store state or wait unnecessarily long if a re-init is happening.

## 🚀 Performance & I/O
- **Memory Fragmentation (One File Per Interaction):** Every interaction is saved as a separate `.gob` file. This results in hundreds or thousands of small files.
  - *Risk:* Startup time (`LoadCache`) increases linearly.
  - *Risk:* Cloud sync efficiency drops significantly due to thousands of API calls for small files.
  - *Risk:* OS file handle limits or disk performance (i.e., on HDD) will degrade.
- **Brute-Force Vector Search:** Memory search uses O(N) linear scanning of all embeddings in RAM. Even with parallel workers, this will degrade as memory grows beyond 10,000+ interactions.
- **Synchronous Persistence:** Every message triggers a synchronous write to both GOB (memory) and JSON (sessions) on the main goroutine (for sessions). This causes "UI stutter" or lag in response time on slow disks.
- **Large Directory Walking:** `ListGobFiles` in `internal/memory/store.go` walks the entire directory. This will make the Settings -> Memory screen load slowly with thousands of entries.

## 🛡️ Error Handling & Reliability
- **Broken Event System (Flutter Migration):** The `emitEvent` function in `app.go` is currently disabled for Flutter.
  - *Critical:* Background errors (Cloud Sync failures, Auto-start embedding model errors, Llama installer progress) are **never reported to the UI**. They only exist in `server.log`.
- **Silent STT Dependency Failures:** `StartRecording` depends on `ffmpeg`, `sox`, or `arecord` being in the PATH. If missing, it returns a generic error that may not help the user install the requirement.
- **Brittle LLM Error Detection:** Error handling for unsupported features (like Vision) depends on matching specific English error strings from `llama.cpp`. This breaks if `llama.cpp` updates its error messages or uses a different language.
- **WaitReady Hard Timeout:** The 180s timeout for model loading is hardcoded. Large models (70B+) or slow CPUs/HDDs will fail to load even if the process is working fine.
- **SSE Connection Orphans:** If a user closes the chat or disconnects during a streaming response, the backend LLM request continues until completion (up to 300s). This wastes CPU/GPU resources and power.

## 📡 Hardware & OS Compatibility
- **AMD Windows Detection:** Automated VRAM detection for AMD on Windows is unreliable and often falls back to CPU mode because `rocm-smi` is rarely in the system PATH on Windows.
- **Hardcoded Windows Audio:** The Windows recording command uses a hardcoded GUID for the audio device (`@device_cm_{...}`). This will fail for users with different hardware configurations or multiple microphones.
- **Linux Sysfs Paths:** GPU detection assumes `/sys/class/drm/card*` naming. Non-standard drivers or containerized environments (Docker) will break this.
- **Port Kill Aggression:** `killByPort` is used to clear ports, but it might fail on systems without `lsof`/`fuser` or without sufficient permissions, leading to "address already in use" errors.

## 📱 Frontend (Flutter)
- **State Desync:** The frontend does not poll for backend state changes initiated outside the current request (e.g., if a model is stopped via the API or another client).
- **Memory Invalidation:** Deleting a memory file from the UI removes the file from disk/RAM but doesn't immediately invalidate the LLM's current internal context window.

## 🔐 Security & Privacy
- **Temp File Leakage:** Web file uploads and STT binaries are stored in the system's global temporary directory. On multi-user systems, this could lead to data exposure if permissions are not strictly set by the OS.
- **Insecure Default Bind:** The web server binds to `0.0.0.0` for remote access, which is dangerous if the user has no firewall and a simple passphrase.
