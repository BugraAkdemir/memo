# Handoff — 2026-06-28

## Oturum Özeti

Bu oturumda macOS Apple Silicon Metal GPU detection + Windows AMD GPU detection + kapsamlı test yazımı + profesyonel CI/CD pipeline kurulumu yapıldı. Tüm testler geçiyor, CI/CD production-ready.

---

## Yapılan Değişiklikler

### macOS Metal GPU Detection

- `internal/llama/gpu.go` — **`GPUTypeMetal`** constant'ı eklendi, **`detectAppleSilicon()`** fonksiyonu:
  - `sysctl hw.optional.arm64` ile Apple Silicon tespiti
  - `machdep.cpu.brand_string` ile chip adı (M1/M2/M3 Pro/Max...)
  - `hw.memsize` ile unified memory boyutu
  - `GPULayers: 999` (tüm katmanları Metal'e offload — unified memory avantajı)
  - Priority: NVIDIA → AMD → Apple Silicon → CPU
- `internal/llama/installer.go` — `assetPrefs["darwin"]`'e `GPUTypeMetal: {"metal", "macos"}` eklendi. Installer artık `-metal-` tagged release asset'lerini tercih eder.
- `internal/llama/llama.go` — `Start()`'e `mode == "metal"` kontrolü eklendi.
- `internal/config/config.go` — `engine_mode` comment'ine `"metal"` eklendi.
- `config/config.yaml.example` — `engine_mode` comment'ine `"metal"` eklendi.
- `frontend/lib/widgets/settings/tabs/gpu_config_tab.dart` — "Apple Silicon (Metal)" dropdown seçeneği eklendi.
- `frontend/lib/core/l10n.dart` — `'engine_metal'` localization anahtarı TR/EN eklendi.

### Windows AMD GPU Detection

- `internal/llama/gpu.go` — **`detectAMDWindows()`** eklendi:
  - PowerShell/WMI (`Get-CimInstance Win32_VideoController`) ile AMD/Radeon GPU sorgulama
  - AdapterRAM'den VRAM okuma, `recommendLayers()` ile optimal layer sayısı
  - Önceden Windows AMD kullanıcıları hiç tespit edilemiyor, CPU'da kalıyordu

### Test Altyapısı

- `internal/webserver/server_test.go` (YENİ, 590 satır) — **Temel REST API handler'ları için kapsamlı testler:**
  - Mock `AppBridge` ile handler izolasyonu
  - Tüm basic API endpoint'leri: send, chats, new/switch/delete/rename chat, messages, update/delete message, status, incognito, transcribe
  - Middleware testleri: CORS (loopback validation, OPTIONS preflight), body limit
  - Error case'ler: bad JSON, method not allowed, bridge error
  - `writeJSON` utility testi
- `internal/app/app_test.go` (YENİ, 239 satır) — **App orchestrator metodları için 20+ test:**
  - Config, system prompt (set/reset), incognito prompt
  - Memory ayarları, web search, mood, self-interest, system management
  - Agent, active provider, listen address, events, active chat ID
  - Tümü existing pattern'lerle uyumlu: stdlib testing, table-driven, hand-written mocks

### Profesyonel CI/CD Pipeline

- `.github/workflows/ci.yml` (YENİ) — **Tam CI/CD pipeline:**
  - **`lint`** — `go vet` + `golangci-lint` (50+ linter, `.golangci.yml` ile yapılandırılmış)
  - **`test`** — `go test -race -coverprofile` → Codecov upload
  - **`flutter`** — `flutter analyze` + `flutter test --coverage` → Codecov upload
  - **`security`** — `govulncheck` ile Go güvenlik taraması
  - **`build-linux`**, **`build-windows`**, **`build-macos`** — tag/manual trigger'da matrix build + artifact upload
  - **`release`** — tag push'te otomatik GitHub Release oluşturma, prerelease detection (beta/alpha/rc)
  - Go module + Flutter pub cache ile hızlı build
- `.github/dependabot.yml` (YENİ) — Haftalık Go/Flutter/Actions dependency update
- `.golangci.yml` (YENİ) — 50+ linter, test dosyası exclusion'ları, revive/stlyecheck kuralları

---

## Test Durumu

```
go build ./...                → temiz (0 hata)
go vet ./...                  → temiz (0 hata)
go test ./... -race -count=1  → 30/30 PASS (22 paket + race detector)
```

### Coverage Özeti

| Paket | Coverage | Durum |
|-------|----------|-------|
| internal/webserver | **5.8%** | 0%'den geldi, en kritik handler'lar testli |
| internal/app | **3.8%** | 1.8%'den geldi, getter/setter'lar testli |
| internal/orchestra | 77.1% | ✅ |
| internal/config | 85.3% | ✅ |
| internal/truncate | 86.5% | ✅ |

---

## Commit Geçmişi

```
4edfd67 ci(professional): complete CI/CD pipeline with lint, test, security, build matrix, and release
2c62944 test(app): add comprehensive test suite for App orchestrator methods
cadcb62 test(webserver): add comprehensive test suite for REST API handlers
937d48b fix(windows): add AMD GPU detection via PowerShell/WMI
11f35a9 feat(flutter): add Apple Silicon (Metal) option to GPU config dropdown
aaa232f docs(config): add 'metal' as valid engine_mode value
e27c6c8 feat(macos): add Apple Silicon Metal GPU detection and installer support
```

---

## Kalan Görevler

### Test Edilmeyen Paketler (hala 0 test dosyası)

- `internal/webserver/handlers_flutter.go` — 110+ Flutter endpoint, FullBridge mock gerektirir
- `internal/whatsapp` — WhatsApp bridge
- `internal/whisper` — STT whisper
- `internal/ngrok` / `internal/tunnel` — Remote access
- `internal/fileutil` / `internal/logx` / `internal/models` — Utility paketleri

### Diğer

- Flutter JDK build hatası — `sudo apt install default-jdk`
- macOS GPU için `frontend/macos/` Flutter platform projesi oluşturulmamış (başka branch'ta varmış)
- Whisper-server Metal variant'ı `resolveBinary()`'de desteklenmiyor
- Skill install dialog Windows path hint düzeltmesi

### Önerilen sıradaki adımlar

1. En kritik: handoff.md'yi bu oturuma göre güncelle (yapıldı)
2. Flutter JDK sorununu çöz → `sudo apt install default-jdk`
3. Kanban/issue açılıp task takibi başlat
