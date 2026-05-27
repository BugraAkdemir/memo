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

## Llama.cpp Engine
The high-performance `llama.cpp` runs in the background. Memo automatically downloads and configures the most up-to-date binary suitable for your operating system.

### Supported Features:
- **GPU Offloading:** Moving layers to VRAM to reduce CPU load.
- **Context Size:** Customizing the context window (e.g., 4096, 8192).

### Linked Notes:
- [[Llama.cpp Integration]]
- [[Backend (Go) Architecture]]
