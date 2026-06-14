# Voice Keyboard — Design Spec

> **Tarih:** 2026-06-14
> **Versiyon:** v1.0
> **Hedef Sürüm:** v3.2.0-beta

## Overview

Memo'ya sesli klavye (speech-to-text) özelliği eklenir. Kullanıcı chat input'taki mic butonuna tıklayarak konuşur, konuşması whisper.cpp tarafından transkribe edilir ve text input'a yazılır.

**Temel prensipler:**
- 0 dış bağımlılık (internet yokken de çalışır)
- Tamamen gömülü (model + engine `binaries/` altında paketlenir)
- On-demand lifecycle (sadece kayıt anında çalışır, normalde 0 RAM)
- Toggle interaction (tıkla → konuş → tıkla → transkribe ol)

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Flutter Frontend                                │
│  ┌──────────────────────────────┐               │
│  │ ChatInput                     │               │
│  │ [img] [file] [🎤] [orch] ... │               │
│  └──────────┬───────────────────┘               │
│             │ POST /api/transcribe (WAV)         │
└─────────────┼───────────────────────────────────┘
              │
┌─────────────┼───────────────────────────────────┐
│  Go Backend                                     │
│  ┌──────────▼───────────────────────────────┐   │
│  │ internal/whisper/                         │   │
│  │  ├── whisper.go    (Server)               │   │
│  │  └── installer.go  (binary/model resolve) │   │
│  └──────────┬───────────────────────────────┘   │
│             │ HTTP POST (WAV)                   │
│  ┌──────────▼───────────────────────────────┐   │
│  │ whisper-server (subprocess, on-demand)     │   │
│  │  --model ggml-small.bin --port 9877       │   │
│  └──────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

## Components

### 1. `internal/whisper/` — Whisper Server Manager

`llama.Server` pattern'inin birebir aynısı.

**`whisper.go` — Server struct:**
```
Server
  mu        sync.RWMutex
  cmd       *exec.Cmd
  port      int
  modelPath string
  language  string
  stopping  bool
  waitDone  chan struct{}
  portPid   int

Start(binaryPath, modelPath string, language string, port int) error
Stop() error                              // SIGTERM + 5sn timeout + force kill
IsRunning() bool
WaitReady(timeout time.Duration) error    // poll /v1/audio/transcriptions health
Transcribe(ctx context.Context, audioData []byte) (string, error)
monitor()                                 // background goroutine, watches cmd.Wait()
resolveBinary(configuredPath string) string  // bundled binaries/ path
resolveModel(configuredPath string) string   // bundled model path
```

**`installer.go` — Binary/model resolution:**
- `resolveBinary()` — Finds `whisper-server` in `binaries/{os}/{gpu}/`, `data/bin/`, `$PATH`
- `resolveModel()` — Finds `ggml-small.bin` in `binaries/{os}/{gpu}/`, `data/models/`
- No download — everything is bundled in `binaries/`

**Platform çoklama (llama.go pattern'i):**
- `process_unix.go` / `process_windows.go` — ProcessGroup yönetimi (Setpgid, Kill)
- `sysproc_linux.go` — `Setpgid: true`

### 2. Config (`internal/config/config.go`)

```go
type WhisperConfig struct {
    BinaryPath string `yaml:"binary_path" json:"binary_path"`
    ModelPath  string `yaml:"model_path" json:"model_path"`
    Language   string `yaml:"language" json:"language"`   // "auto", "tr", "en"
    Port       int    `yaml:"port" json:"port"`           // default 9877
    Enabled    bool   `yaml:"enabled" json:"enabled"`     // default true
}
```

`AppConfig`'a `Whisper WhisperConfig` field eklenir.

### 3. `app.go` Değişiklikleri

| Eski (STT) | Yeni (whisper) |
|------------|-----------------|
| `sttServer *exec.Cmd` | `whisperServer *whisper.Server` |
| `startSTTServer()` | `startWhisperServer()` |
| `shutdown()` → `sttKillProcessGroup(a.sttServer)` | `a.whisperServer.Stop()` |
| `TranscribeAudio(audioData)` → POST `127.0.0.1:9876/transcribe` | `a.whisperServer.Transcribe(ctx, audioData)` |
| `StartRecording()`, `StopRecordingAndTranscribe()` | **Kaldırılır** (kayıt Flutter'da) |
| `//go:embed binaries/stt_server_*` | Kaldırılır |

**`startWhisperServer()`:**
1. Resolve binary (`whisper-server`) from `binaries/{os}/{gpu}/`
2. Resolve model (`ggml-small.bin`) from same dir
3. Create `whisper.Server`, call `Start(binary, model, language, port)`
4. `WaitReady(10s)`

**`shutdown()`:**
- `a.whisperServer.Stop()` eklenir (sttServer kill yerine)

### 4. Flutter

#### Yeni Dependency (`pubspec.yaml`)
```yaml
record: ^5.2.0
```

#### Yeni Provider (`frontend/lib/providers/recording_provider.dart`)
```dart
enum RecordingState { idle, recording, transcribing }

class RecordingNotifier extends StateNotifier<RecordingState> {
  final Ref ref;
  AudioRecorder? _recorder;
  
  Future<void> start() async  // state = recording, start audio capture
  Future<void> stop() async   // state = transcribing, send to /api/transcribe
  // On response: state = idle, emit transcribed text via callback
}
```

Audio format: WAV 16kHz 16-bit mono

#### ChatInput Mikrofon Butonu (`chat_input.dart`)

Toggle pattern, basınca kayıt başlar, tekrar basınca durur.

```dart
// Mevcut _InputIconButton widget'ı kullanılır
_InputIconButton(
  icon: state == RecordingState.idle ? Icons.mic_outlined 
      : state == RecordingState.recording ? Icons.mic
      : Icons.hourglass_top,
  tooltip: 'Ses Kaydet',
  iconColor: state == RecordingState.recording ? ThemeColors.red : null,
  onTap: state == RecordingState.idle 
      ? ref.read(recordingProvider.notifier).start
      : state == RecordingState.recording
          ? ref.read(recordingProvider.notifier).stop
          : null, // transcribing'de disabled
)
```

**Buton pozisyonu:** `[image] [attach_file] [🎤 mic] [orchestra]` — orchestra butonunun hemen solunda.

**Animasyonlar (AnimatedContainer):**
- **idle → recording:** Mic icon kırmızıya döner, container border'a subtle glow eklenir (boxShadow + AnimatedContainer ile renk geçişi)
- **recording → transcribing:** Loading spinner (`CircularProgressIndicator` boyut 18)
- **transcribing → idle:** Sonuç direkt text input'a yazılır, mic normale döner

#### API Client (`api_client.dart`)
Mevcut `transcribeAudio(audioData)` endpoint'i zaten var — doğrudan kullanılır:
```dart
Future<String> transcribeAudio(Uint8List audioData) async {
  final res = await _dio.post('/api/transcribe', data: audioData);
  return res.data['text'] as String? ?? '';
}
```

#### L10n (`l10n.dart`)
```dart
// TR
'mic_recording': 'Kaydediliyor...',
'mic_transcribing': 'Yazıya dökülüyor...',
'mic_start': 'Ses kaydı başlat',
'mic_stop': 'Ses kaydını durdur',

// EN
'mic_recording': 'Recording...',
'mic_transcribing': 'Transcribing...',
'mic_start': 'Start voice recording',
'mic_stop': 'Stop voice recording',
```

### 5. Binary Layout

```
binaries/
├── linux/
│   ├── cpu/
│   │   ├── llama-server          (mevcut)
│   │   ├── whisper-server        (yeni)
│   │   └── ggml-small.bin        (yeni ~500MB)
│   ├── amd/
│   │   ├── llama-server          (mevcut)
│   │   └── whisper-server        (yeni)
│   ├── nvidia/
│   │   ├── llama-server          (mevcut)
│   │   └── whisper-server        (yeni)
│   ├── vec0.so                   (mevcut)
│   └── ngrok                     (mevcut)
├── windows/
│   ├── cpu/
│   │   ├── llama-server.exe      (mevcut)
│   │   ├── whisper-server.exe    (yeni)
│   │   └── ggml-small.bin        (yeni ~500MB)
│   ├── amd/  (aynı şekilde)
│   ├── nvidia/ (aynı şekilde)
│   ├── vec0.dll                  (mevcut)
│   └── ngrok.exe                 (mevcut)
└── placeholder                   (mevcut)
```

### 6. Build Scripts

**`build_releases.sh`:**
- Mevcut `cp -r binaries/* "$STAGEDIR/binaries/"` satırı whisper dosyalarını da otomatik kapsar → **DEĞİŞİKLİK YOK**
- `data/bin/stt_server` kopyalama satırı (69-70) **kaldırılır**
- `LD_LIBRARY_PATH` satırında `data/bin` referansı kalabilir

**`build_releases.bat`:**
- Mevcut `xcopy binaries\windows` whisper dosyalarını da kapsar → **DEĞİŞİKLİK YOK**

**`run_memo.sh`:**
- İlk çalıştırmada `MEMO_HOME/binaries/` altına kopyalanan `binaries/*` whisper dosyalarını da içerir → **DEĞİŞİKLİK YOK**
- `$MEMO_HOME/data/bin` dizin yapısı ve STT ile ilgili hiçbir kod kalkar

**`run_memo.bat`:**
- Aynı şekilde → **DEĞİŞİKLİK YOK**

### 7. API Endpoints

| Endpoint | Değişiklik |
|----------|------------|
| `POST /api/transcribe` | **Mevcut, değişmez.** Body: raw WAV bytes → Response: `{"text": "..."}` |
| `POST /api/recording/start` | **KALDIRILIR** (kayıt Flutter'da) |
| `POST /api/recording/stop` | **KALDIRILIR** (kayıt Flutter'da) |
| `GET /api/config/whisper` | Yeni — whisper ayarlarını getir |
| `PUT /api/config/whisper` | Yeni — whisper ayarlarını güncelle |

### 8. Silinecek Kod

| Dosya | Not |
|-------|-----|
| `stt_proc_unix.go` | `sttSetProcessGroup`, `sttKillProcessGroup` → whisper.go içinde kendi versiyonu olur |
| `stt_proc_windows.go` | Aynı şekilde |
| `internal/webserver/handlers_flutter.go` → `handleRecordingStart`, `handleRecordingStop` | Kaldırılır |
| `internal/webserver/bridge.go` → `StartRecording()`, `StopRecordingAndTranscribe()` | FullBridge'den kaldırılır |
| `app.go` → `startSTTServer()`, `TranscribeAudio()` eski implementasyon | whisper ile replace edilir |
| `app.go` → `StartRecording()`, `StopRecordingAndTranscribe()` metodları | Kaldırılır |
| `frontend/lib/core/api_client.dart` → `startRecording()`, `stopRecording()` | Kaldırılır |

### 9. Error Handling

- **whisper-server binary bulunamazsa:** `startWhisperServer()` log atar, `TranscribeAudio()` `"whisper sunucusu başlatılamadı"` hatası döner, Flutter UI'da `debugPrint` ile loglanır
- **ggml-small.bin model bulunamazsa:** Aynı şekilde
- **Kayıt sırasında hata:** `record` paketi exception'ı `debugPrint` + state `idle`'a döner
- **Transkripsiyon timeout:** whisper sunucusu cevap vermezse 30sn timeout, hata mesajı UI'da gösterilir
- **Mikrofon izni reddedilirse:** `record` paketi `hasPermission()` kontrolü, izin reddedilmişse Flutter'da `SnackBar` ile uyarı

### 10. whisper.cpp Subprocess Komutu

```bash
./whisper-server \
  --model ggml-small.bin \
  --port 9877 \
  --host 127.0.0.1 \
  --language auto
```

`POST /v1/audio/transcriptions` endpoint'i ile transkripsiyon (multipart audio upload).

---

## Implementation Order

| Sıra | Faz | Dosyalar |
|------|-----|----------|
| 1 | **`internal/whisper/` paketi** | `whisper.go`, `installer.go`, platform dosyaları |
| 2 | **Config** | `internal/config/config.go` → `WhisperConfig` |
| 3 | **`app.go`** entegrasyonu | `startWhisperServer()`, shutdown, `TranscribeAudio()` |
| 4 | **Eski STT temizliği** | `stt_proc_*.go`, `handlers_flutter.go` recording handler'ları, `bridge.go` |
| 5 | **Binary/model ekleme** | `binaries/linux/cpu/whisper-server`, `ggml-small.bin`, Windows |
| 6 | **Build scripts** | `build_releases.sh` → stt_server satırı sil, `.bat` güncelle |
| 7 | **Flutter mic button** | `recording_provider.dart`, `chat_input.dart`, `l10n.dart`, `pubspec.yaml` |
| 8 | **Test & verify** | End-to-end test |
