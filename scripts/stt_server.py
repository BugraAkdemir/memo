#!/usr/bin/env python3
"""Lightweight STT HTTP server using Vosk. Model stays in memory."""
import sys
import json
import wave
import tempfile
import os
from http.server import HTTPServer, BaseHTTPRequestHandler
from vosk import Model, KaldiRecognizer, SetLogLevel

SetLogLevel(-1)

LANG = sys.argv[1] if len(sys.argv) > 1 else "tr"
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 9876

MODEL_MAP = {
    "tr": "vosk-model-small-tr-0.3",
    "en": "vosk-model-small-en-us-0.15",
}

model_name = MODEL_MAP.get(LANG, f"vosk-model-small-{LANG}")
print(f"[STT] Loading Vosk model '{model_name}'...", flush=True)
model = Model(model_name=model_name)
print(f"[STT] Model loaded. Server on port {PORT}", flush=True)


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        audio_data = self.rfile.read(length)

        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
            f.write(audio_data)
            tmp_path = f.name

        try:
            wf = wave.open(tmp_path, "rb")
            rec = KaldiRecognizer(model, wf.getframerate())
            rec.SetWords(False)

            while True:
                data = wf.readframes(4000)
                if len(data) == 0:
                    break
                rec.AcceptWaveform(data)

            result = json.loads(rec.FinalResult())
            text = result.get("text", "")
            wf.close()
        except Exception as e:
            text = ""
            print(f"[STT] Error: {e}", flush=True)
        finally:
            os.unlink(tmp_path)

        resp = json.dumps({"text": text}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(resp)))
        self.end_headers()
        self.wfile.write(resp)

    def log_message(self, fmt, *args):
        pass  # quiet


if __name__ == "__main__":
    server = HTTPServer(("127.0.0.1", PORT), Handler)
    print(f"[STT] Ready at http://127.0.0.1:{PORT}", flush=True)
    server.serve_forever()
