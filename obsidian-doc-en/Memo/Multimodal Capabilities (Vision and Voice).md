# 👁️ Multimodal Capabilities (Vision and Voice)

Memo is not limited to text only; it can see images and hear sounds.

## Vision Analysis
If the GGUF model you are using supports multimodality (e.g., `Llava`, `Moondream`, `BakLLaVA`):
- **Drag-and-Drop:** You can drag and drop images into the chat area for analysis.
- **Local Processing:** Images are locally converted to Base64 format and securely transmitted to the LLM. No images are uploaded to the cloud.

## Voice Command and Transcription (STT)
Memo includes a local Speech-to-Text (STT) engine:
- **Offline Recording:** You can record your voice using the microphone icon within the application.
- **Private Transcription:** Audio files are converted to text locally (with a Whisper or Vosk-based engine).
- **Low Latency:** As soon as the process finishes, the text is automatically written into the input field.

## File Contextualization
Not just media, but also code files (.go, .js, .py) or documents can be fed into the system. Memo reads the content of these files and uses them as instant context via the RAG mechanism.

### Linked Notes:
- [[Frontend (Flutter) Design]]
- [[RAG and Semantic Memory]]
