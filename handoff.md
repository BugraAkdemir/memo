# Handoff — 2026-06-27

## Oturum Özeti

Bu oturumda RAG bellek sisteminde kullanıcı deneyimini doğrudan etkileyen 3 bug bulunup düzeltildi. `go build ./...` temiz, `go test ./... -race` 29/29 pass.

---

## Yapılan Değişiklikler

### RAG Memory Bug Fixes

- `internal/memory/store.go` — **FTS5 escape injection**: `NOT`/`AND`/`OR` keyword'leri FTS5 operatörü sanılıp match kırılıyordu, `*`/`(`/`)` gereksiz siliniyordu, tek-karakter kelimeler atlanıyordu. Her kelime `"..."` içine alınarak FTS5 query syntax'ından izole edildi. Kod 12 satırdan 8 satıra indi.
- `internal/memory/store.go` — **`sqlFilteredFallback` similarity=0%**: Filtreleme fallback'e düşünce tüm sonuçlar UI'da "relevance=0%" görünüyordu. Varsayılan `Similarity = 0.5` atandı.
- `internal/memory/chunker.go` — **Son chunk çok kısa kalıyordu**: 300+ kelimelik mesajlarda son chunk 1-50 kelime arasına düşüyor, embedding kalitesiz oluyordu. Son chunk `< overlapWords*2` ise önceki chunka merge ediliyor.

---

## Yapılan Değişiklikler

### Cross-Platform Release Audit & Fixes

**CRITICAL — Uygulamayı tamamen kıran**

- `internal/database/sqlite.go` — `buildDSN()`: Windows absolute path (`C:\...`) `file:` URI'de bozuluyordu. `filepath.ToSlash` + leading `/` eklendi. Tüm SQLite DB'ler Windows'ta açılmıyordu.
- `internal/llama/sysproc_darwin.go` (YENİ) — macOS'ta `forceKillCmd` SIGKILL'i parent Memo process'ine gönderiyordu (child, parent'ın process group'unu inherit ediyordu). `Setpgid: true` ile child'a ayrı grup verildi.
- `internal/whisper/sysproc_darwin.go` (YENİ) — Aynı sorun whisper server için. Aynı fix.
- `internal/llama/sysproc_other.go` — Build tag `!linux` → `!linux && !darwin` (darwin artık kendi dosyasını kullanıyor)
- `internal/whisper/sysproc_other.go` — Aynı tag değişikliği

**HIGH — Önemli feature'ları kıran**

- `internal/fileutil/atomic.go` — `os.Rename` Windows'ta hedef dosya açıkken `ERROR_ACCESS_DENIED` döner. Copy-then-delete fallback eklendi.
- `internal/ngrok/proc_windows.go` — `killProcessTree` sadece parent'ı kill ediyordu; ngrok child'ları port 4040'ı tutuyordu. `taskkill /F /T /PID` ile tree kill yapılıyor.
- `build_releases.sh` + `build_releases.bat` — Graceful shutdown API port 8080 çağrılıyordu, backend 8090'da → hiç çalışmıyordu. 8080 → 8090 düzeltildi (4 ayrı call site).
- `internal/llama/llama.go` — macOS'ta `LD_LIBRARY_PATH` set ediliyordu; dyld bunu ignore eder. `DYLD_LIBRARY_PATH` + `DYLD_FALLBACK_LIBRARY_PATH` kullanılıyor.
- `internal/whisper/whisper.go` — Aynı DYLD fix.
- `internal/app/app.go` — `startSTTServer()` tanımlıydı ama hiç çağrılmıyordu → STT feature tüm platformlarda ölüydü. `go a.startSTTServer()` startup'a eklendi.
- `internal/cloudsync/sync_manager.go` — `restoreZip` rename failure'da partial restore bırakıyordu. `copyRestoreFile` helper + fallback eklendi.

**MEDIUM — Paketlenmiş build'de kırılan**

- `internal/config/config.go` — Seed config dosyası `filepath.Join("config", "config.yaml")` (CWD-relative) ile aranıyordu. Windows Start Menu shortcut'tan açılınca CWD yanlış oluyor. `os.Executable()` bazlı path'e geçildi.
- `internal/ngrok/manager.go` — ngrok binary aynı şekilde CWD-relative. `os.Executable()` dir'i önce aranıyor.
- `internal/webserver/server.go` — Rate-limit cleaner goroutine'in stop mekanizması yoktu; server her restart'ta yeni goroutine sızıyordu. `stopCleaner chan struct{}` eklendi, `Stop()` kapatıyor.
- `internal/agent/tools/command.go` — `filepath.EvalSymlinks` Windows junction point'lerde privilege hatası veriyordu, `filepath.Clean` fallback'e dönüldü. Windows `cmd.Env`'e `TEMP`, `TMP`, `COMSPEC`, `HOMEDRIVE`, `HOMEPATH` eklendi.
- `build_releases.bat` — `data\whatsapp` dizini Windows staging'de eksikti.
- `build_releases.sh` — macOS ZIP dosyası `arm64` hard-coded'dı; `uname -m` → `MAC_ARCH` ile gerçek arch kullanılıyor.

---

## Test Durumu

```
go build ./...                → temiz (0 hata)
go test ./... -race -count=1  → 29/29 PASS
```

---

## Commit Edilmemiş Dosyalar

```
internal/database/sqlite.go
internal/llama/sysproc_darwin.go   ← YENİ
internal/llama/sysproc_other.go
internal/llama/llama.go
internal/whisper/sysproc_darwin.go ← YENİ
internal/whisper/sysproc_other.go
internal/whisper/whisper.go
internal/fileutil/atomic.go
internal/ngrok/proc_windows.go
internal/ngrok/manager.go
internal/cloudsync/sync_manager.go
internal/config/config.go
internal/agent/tools/command.go
internal/app/app.go
internal/webserver/server.go
internal/memory/store.go
internal/memory/chunker.go
internal/memory/chunker_test.go
build_releases.sh
build_releases.bat
```

---

## Kalan Görevler

### Release için henüz yapılmayan (kod fix'i gerektirmeyen)

- **macOS GPU (Metal/Apple Silicon)** — `gpu.go`'da Apple Silicon tespiti yok; llama.cpp Metal destekli derlenmiş bile olsa `--n-gpu-layers 0` geçiliyor. Kullanıcı manuel ayarlamalı. Otomatik detection için `sysctl hw.optional.arm64` + darwin + arm64 kombinasyonu kontrol edilebilir.
- **Windows AMD GPU (DirectML)** — `detectAMD()` Linux-only (`rocm-smi`, sysfs). Windows AMD GPU kullanıcıları CPU mode'da kalıyor. DirectML veya HIP-for-Windows detection eklenebilir.
- **Whisper Windows binary** — `binaries/windows/cpu/whisper-server.exe` bundle'da yok. STT Windows'ta çalışmıyor (startSTTServer() fix'i sonrasında bile binary eksik).
- **Flutter embedding spinner** — Memory etkin ama embedding server çalışmıyorken perpetual spinner gösteriliyor. "Inactive" state ayrıştırılmalı.
- **Skill install dialog** — Windows'ta Unix path hint (`/home/...`) gösteriliyor.

### Önerilen sıradaki adımlar

1. Yukarıdaki dosyaları commit et
2. Whisper Windows binary'sini bundle'a ekle (eğer varsa)
3. macOS test makinasında build al, force-stop davranışını doğrula
4. Windows'ta SQLite DSN fix'ini doğrula (DB açılıyor mu?)
5. `git tag v3.1.0` + `build_releases.sh` ile binary üret
