# 🔧 Advanced Settings

Memo offers power users the ability to fine-tune RAG and Model parameters.

## Memory (RAG) Settings
- **Top-K (Memory Count):** Determines how many past memories will be retrieved in each query. (Default: 5)
- **Similarity Threshold:** The minimum score required for a memory to be considered "relevant." (e.g., 0.75)
- **Min Similarity:** Used to prevent very irrelevant memories from cluttering the context.

## Model Parameters
- **Temperature:** Determines how "creative" or "consistent" the answers will be. (0.0 - 1.0)
- **Repeat Penalty:** Prevents the model from repeating the same words.
- **GPU Layers:** Number of layers to offload to the VRAM. Full performance is recommended by offloading all layers to the GPU.

## Network and Access
- **Remote Access:** When this setting is turned on, Memo opens for access from other devices on the local network (Wi-Fi).
- **Port Settings:** You can change the default 8090 port in case of a conflict.

### Linked Notes:
- [[Vector Search Logic]]
- [[API Documentation]]
