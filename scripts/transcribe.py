#!/usr/bin/env python3
"""Transcribe audio file using faster-whisper. Prints text to stdout."""
import sys
import os
from faster_whisper import WhisperModel

_model_cache = {}

def get_model(model_size):
    if model_size not in _model_cache:
        _model_cache[model_size] = WhisperModel(model_size, device="auto", compute_type="auto")
    return _model_cache[model_size]

def main():
    if len(sys.argv) < 2:
        print("Usage: transcribe.py <audio_file> [model_size]", file=sys.stderr)
        sys.exit(1)

    audio_path = sys.argv[1]
    model_size = sys.argv[2] if len(sys.argv) > 2 else "base"

    print(f"Loading model '{model_size}'...", file=sys.stderr, flush=True)
    model = get_model(model_size)

    print("Transcribing...", file=sys.stderr, flush=True)
    segments, _ = model.transcribe(audio_path, beam_size=5)

    text = " ".join(seg.text.strip() for seg in segments)
    print(text, flush=True)

if __name__ == "__main__":
    main()
