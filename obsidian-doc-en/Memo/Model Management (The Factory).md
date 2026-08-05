# 🏭 Model Management (The Factory)

Memo makes it easy to discover, download, and manage AI models from within the application.

## Hugging Face Integration
You can search for models in GGUF format on Hugging Face directly through the Model Store.
- **Automatic Filtering:** Only compatible GGUF formats are shown.
- **Repo ID Support:** Any Hugging Face URL can be pasted to start a download directly.

## Intelligent Diagnostics
Before downloading or starting a model, the system checks:
- **VRAM Check:** Checks the capacity of your graphics card.
- **GPU Compatibility Badge:** Indicates whether the model will work efficiently on your GPU.
- **Real max context from the GGUF file itself (v3.3.3):** context size used to be a free-text field with no upper bound — an unrealistic value could crash the model server. Memo now reads the model's actual maximum context from the file and won't let the slider exceed it.
- **Tool-calling / code-capability badges are based on the model's real chat template and tags (v3.3.3),** not a hardcoded list of "known" model families — more accurate for newer or less common models.
- **Brand logo reflects the actual creator (v3.3.3)**, even for a requantized/repackaged upload.

## Discovery & Downloads (v3.3.3 improvements)
- **First-run hardware-matched recommendation:** setup reads your RAM/GPU and suggests a matching chat + memory model pair, with one button to start both downloading.
- **Parallel downloads:** starting a second download used to be rejected outright; several can now run at once, with combined progress in the engine status bar.
- **Discover filters (Tools/Vision/Code/Embedding/Size) now combine with OR instead of AND** — selecting two used to often return nothing — grouped into multi-select dropdowns with an "N filters active · clear" indicator.
- Fixed: blank "?" author avatars, the download button getting stuck reading "Cancel" after a gated model's download failed with a 401. Gemma 3 4B/12B were removed from the recommended list after confirming they're access-gated and fail on an anonymous download.
- **Plain-language errors (v3.3.4):** model start/download failures now show a short, actionable message instead of a raw error string; hover tooltips explain the GGUF/quantization and hardware-fit badges; download progress shows a rough time estimate.

## Llama.cpp Engine
The high-performance `llama.cpp` runs in the background. Memo automatically downloads and configures the most up-to-date binary suitable for your operating system.

### Supported Features:
- **GPU Offloading:** Moving layers to VRAM to reduce CPU load.
- **Context Size:** Customizing the context window (e.g., 4096, 8192).

### Linked Notes:
- [[Llama.cpp Integration]]
- [[Backend (Go) Architecture]]
