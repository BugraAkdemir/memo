# Bug Report — Memo v3.1.0-beta Stabilite Engelleri

> **Amaç:** Stable sürüme engel olan, kullanıcıyı direkt etkileyen bug'ların kapsamlı tespiti.
> **Tarih:** 2026-06-29
> **Kapsam:** Go backend + Flutter frontend — tam kod tabanı taraması
> **Metod:** 4 paralel agent ile eşzamanlı tarama (concurrency, crash, data loss, frontend)

---

## Özet

| Severity | Adet | Düzeltilenler | En Kritik Alan |
|----------|------|--------------|---------------|
| 🔴 CRITICAL | 4 | C1, C3, C4 ~~fixed~~ | WhatsApp nil panic, import'ta memory.db bozulması, cloud key kaybı |
| 🟠 HIGH | 10 | H1, H3, H4, H5 ~~fixed~~ | Shutdown panic, data race, config kaybı, eksik yedek, Flutter Stop |
| 🟡 MEDIUM | 12 | — | Goroutine sızıntısı, hata kirliliği, kısmi import, güvenlik |
| 🟢 LOW | 6 | — | Cache sızıntısı, error swallowing, UI double-rebuild |

> **Son güncelleme:** 2026-06-30, Session 5 — 7 stable-blocking bug düzeltildi.

---

## 🔴 CRITICAL — Uygulama Çökmesi / Kalıcı Veri Kaybı

### BUG-C1: ~~WhatsApp `c.waClient` 30+ yerde locksuz okunuyor → nil pointer panic~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `854e04d`
- **Düzeltme:** Tüm `c.waClient`, `c.qrCodes`, `c.lastError`, `c.started`, `c.reconnecting` erişimleri `startMu` ile korundu. `handleEvent` lock altında yazıyor, `autoReconnect` local değişkene kopyalıyor.

- **Dosya:** `internal/whatsapp/client.go:234-650`
- **Nedir:** `c.waClient` alanı sadece `Start()` ve `Stop()` içinde `startMu` altında set ediliyor, ancak **tüm okumalar locksuz**:
  - `IsConnected():234` → `c.waClient != nil && c.waClient.IsConnected()`
  - `IsLoggedIn():239` → `c.waClient != nil && c.waClient.IsLoggedIn()`
  - `SendMessage():245→252` → TOCTOU: 245'te nil check, 252'de `c.waClient.SendMessage()` — arada `Stop()` girerse nil deref
  - `GetProfilePicture():309→339` — aynı TOCTOU
  - `handleHistorySync():477`, `handleMessage():562`, `resolveDisplayName():596`, `importContacts():618`, `importGroups():650` — hepsi locksuz
- **Kullanıcı etkisi:** WhatsApp bağlantısı kesilirken/sonlandırılırken gelen herhangi bir HTTP isteği (mesaj gönderme, durum sorgulama, profil resmi) **uygulamayı panic ile çökertir**.
- **Düzeltme:** Tüm bu fonksiyonlarda `c.startMu.RLock()` / `c.startMu.Lock()` ile `c.waClient` okuması korunmalı. Alternatif: `c.waClient`'i `atomic.Value` ile sar.

### BUG-C2: WhatsApp `handleEvent` shared state'i locksuz yazıyor → data race

- **Dosya:** `internal/whatsapp/client.go:388-430`
- **Nedir:** `handleEvent()` whatsmeow'un event loop goroutine'inde çalışır. `c.qrCodes`, `c.lastError`, `c.started`, `c.reconnecting` alanlarını **hiçbir lock olmadan** yazar:
  - `:391` → `c.qrCodes = v.Codes`
  - `:394` → `c.qrCodes = nil`
  - `:398` → `c.lastError = v.Error.Error()`
  - `:405` → `c.reconnecting = false`
  - `:409` → `c.lastError = "connection lost"`
  - `:415` → `if c.started {` (okuma, locksuz)
  - `:420-422` → `c.lastError`, `c.qrCodes`, `c.started` yazma
- **Kullanıcı etkisi:** Aynı alanlar `Start()`/`Stop()` tarafından lock altında yazılırken, `QRCodes()`/`LastError()`/`IsReconnecting()` tarafından locksuz okunur. Slice corruption (qrCodes), boolean tear, kayıp hata mesajları. **QR kod gösterimi bozulabilir, bağlantı durumu yanlış raporlanır.**
- **Düzeltme:** `handleEvent` içindeki tüm `c.qrCodes`, `c.lastError`, `c.started`, `c.reconnecting` yazma/okumaları `startMu.Lock()` altına alınmalı.

### BUG-C3: ~~Import sırasında `os.Create` direkt hedef dosyayı kesiyor → memory.db kalıcı bozulur~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `b00b800`
- **Düzeltme:** Temp-file (`*.importtmp`) + `os.Rename` pattern kullanıldı. Crash'te orijinal dosya korunur.

- **Dosya:** `internal/app/backup.go:135`
- **Nedir:** `ImportData()` fonksiyonu:
  ```go
  out, err := os.Create(target)   // hedef dosyayı anında sıfırlar
  io.Copy(out, rc)                // crash olursa yarım dosya kalır
  ```
  Temp-file + atomic rename pattern'i yok. `os.Create` çağrıldığı anda eski dosya yok olur. Eğer `io.Copy` sırasında disk dolarsa, process crash olursa veya I/O hatası alınırsa, **hedef dosya sıfırlanmış ama eksik yazılmış halde kalır**. Bu özellikle `memory.db` (SQLite) için ölümcüldür — bozuk bir SQLite dosyası geri döndürülemez.
- **Kullanıcı etkisi:** `.memo` dosyasından import sırasında crash/disk-full → **tüm RAG hafıza kalıcı olarak bozulur**. Pre-import snapshot (`backups/pre_import_*.zip`) var ama manuel recovery gerekir.
- **Düzeltme:** `fileutil.AtomicWrite` veya temp-file + `os.Rename` pattern'i kullanılmalı.

### BUG-C4: ~~`.machine-id` memory dizininde → WipeAllData bulut yedekleme anahtarını yok eder~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `b006911`
- **Düzeltme:** `.machine-id` `data/` dizinine taşındı, eski konumdan migrasyon eklendi, `wipePreserve` listesine `.machine-id` eklendi.

- **Dosya:** `internal/cloudsync/sync_manager.go:106-120`, `internal/app/backup.go:237-285`
- **Nedir:** `loadOrCreateMachineID(persistDir)`, `.machine-id` dosyasını `data/memory/` altında oluşturur. Bu ID, bulut yedekleme şifreleme anahtarının türetilmesinde kullanılır (passphrase boşken fallback). `WipeAllData()`, `data/memory/` dahil tüm data dizinlerini siler — ama `wipePreserve` listesinde `memory` yoktur:
  ```go
  var wipePreserve = map[string]bool{
      "models": true, "bin": true, "binaries": true,
      "tmp_llama": true, "tailscale": true,
      // "memory" YOK!
  }
  ```
- **Kullanıcı etkisi:** Fabrika ayarlarına döndürme (`WipeAllData`) sonrası **tüm eski bulut yedekleri kalıcı olarak geri döndürülemez** — şifreleme anahtarı değiştiği için decrypt imkansız hale gelir. Kullanıcı tüm cloud backup'larını kaybeder.
- **Düzeltme:** `.machine-id`, `data/` altında memory'den bağımsız bir konuma taşınmalı (örn. `data/.machine-id`) veya `wipePreserve` listesine `memory` eklenmeli (ama bu wipe'ın amacına aykırı). En doğrusu: `persistDir` yerine `dataDir` kullanmak.

---

## 🟠 HIGH — Veri Kaybı / Stabilite / Kaynak Sızıntısı

### BUG-H1: ~~`close(memorySaveCh)` sonrası kanala yazma → shutdown sırasında panic~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `86a0045`
- **Düzeltme:** `close(memorySaveCh)` → `webSrv.Stop()` sonrasına taşındı; `memorySaveWg` ile worker sync; store worker bittikten sonra kapanıyor.

- **Dosya:** `internal/app/app.go:470`, `internal/app/memory.go:37-42`
- **Nedir:** `Shutdown()` içinde `close(a.memorySaveCh)` çağrılır (line 470). Ancak aynı anda LLM stream goroutine'leri hala çalışıyor olabilir (HTTP request context'ine bağlılar, `lifecycleCtx`'e değil). `finishStream` → `saveMemoryAsync` zinciri, kapatılmış kanala `memorySaveCh <- saveTask{...}` gönderirse **Go runtime panic** oluşur:
  ```go
  // memory.go:38
  case a.memorySaveCh <- saveTask{userMsg: userMsg, reply: reply}:
  ```
  `select` + `time.After` bu durumu engellemez — kapalı kanala send her zaman panictir.
- **Kullanıcı etkisi:** Uygulama kapanırken (özellikle aktif chat varsa) **panic ile çöker**. Loglar kaybolur, graceful shutdown başarısız olur.
- **Düzeltme:** Kanalı kapatmadan önce tüm stream goroutine'lerinin bitmesi beklenmeli (WaitGroup). Veya send öncesi kanalın nil olup olmadığı kontrol edilmeli.

### BUG-H2: WhatsApp `autoReconnect` TOCTOU nil deref → reconnect sırasında crash

- **Dosya:** `internal/whatsapp/client.go:444-456`
- **Nedir:**
  ```go
  c.startMu.Lock()
  alive := c.started && c.waClient != nil   // line 446 — lock altında check ✓
  c.startMu.Unlock()                         // line 447 — lock bırakıldı

  // ... burada Stop() çalışıp c.waClient = nil yapabilir ...

  if err := c.waClient.Connect(); err != nil { // line 455 — LOCKSIZ kullanım → nil panic
  ```
  Line 446'daki nil check ile line 455'teki kullanım arasında lock yok. `Stop()` araya girip `c.waClient = nil` yaparsa, `c.waClient.Connect()` nil dereference panic üretir.
- **Kullanıcı etkisi:** WhatsApp otomatik yeniden bağlanma sırasında uygulama kapanıyorsa **crash**.
- **Düzeltme:** `c.waClient`'i lock altında local değişkene alıp onu kullanmak, veya tüm reconnect gövdesini lock altında tutmak.

### BUG-H3: ~~Export WAL checkpoint olmadan memory.db kopyalar → eksik `.memo` yedeği~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `be4d6ae`
- **Düzeltme:** Export öncesi `PRAGMA wal_checkpoint(TRUNCATE)` çalıştırılıyor (cloudsync ile aynı pattern).

- **Dosya:** `internal/app/backup.go:73`
- **Nedir:** `ExportData()`, `data/memory/` dizinini olduğu gibi zip'e ekler. Ancak **WAL checkpoint yapılmaz**. SQLite WAL modda olduğu için, henüz ana dosyaya yazılmamış (checkpoint edilmemiş) transaction'lar `-wal` dosyasındadır. Sadece `memory.db` kopyalandığında bu transaction'lar **eksik kalır**. Karşılaştırma: `sync_manager.go:414` bulut yedeklemede DOĞRU şekilde `PRAGMA wal_checkpoint(TRUNCATE)` çalıştırır.
- **Kullanıcı etkisi:** `.memo` export'u **son hafıza kayıtlarını içermez**. Kullanıcı yedeğin tam olduğunu sanır ama en son konuşmaların hafıza kayıtları eksiktir.
- **Düzeltme:** Export öncesi `PRAGMA wal_checkpoint(TRUNCATE)` çalıştırılmalı (cloud sync'teki gibi).

### BUG-H4: ~~Config.yaml atomic olmayan yazım → crash'te tüm ayarlar kaybolur~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `1384e52`
- **Düzeltme:** `os.WriteFile` → `fileutil.AtomicWrite` (tmp + Rename). Crash'te orijinal config.yaml korunur.

- **Dosya:** `internal/config/config.go:412`
- **Nedir:** `saveToFile()`:
  ```go
  return os.WriteFile(path, data, 0600)
  ```
  `os.WriteFile` önce dosyayı truncate eder, sonra yazar. Crash/disk-full durumunda **config.yaml sıfır bayt veya eksik kalır**. Sonraki başlatmada `yaml.Unmarshal` başarısız olur → `Load()` hata döner → uygulama ya başlamaz ya da fresh defaults oluşturur.
- **Kullanıcı etkisi:** **Tüm kullanıcı ayarları, provider yapılandırması, kişiselleştirmeler tek seferde kaybolur.** Bu en kritik konfigürasyon dosyasıdır.
- **Düzeltme:** `fileutil.AtomicWrite` kullanılmalı (zaten projede mevcut).

### BUG-H5: ~~Provider config atomic olmayan yazım → crash'te API anahtarları kaybolur~~ **→ DÜZELTİLDİ (2026-06-30)**

- **Commit:** `41cc723`
- **Düzeltme:** `os.WriteFile` → `fileutil.AtomicWrite`. Crash'te orijinal providers.json korunur.

- **Dosya:** `internal/provider/config.go:146`
- **Nedir:** `saveLocked()`:
  ```go
  if err := os.WriteFile(cm.filePath, data, 0600); err != nil {
  ```
  BUG-H4 ile aynı pattern. `providers.json` direkt yazılır, crash'te bozulur.
- **Kullanıcı etkisi:** Tüm AI provider yapılandırması (OpenAI, Gemini, Claude, Grok, vb.) ve **şifrelenmiş API anahtarları** kaybolur. Kullanıcı tüm sağlayıcıları yeniden yapılandırmak zorunda kalır.
- **Düzeltme:** `fileutil.AtomicWrite` kullanılmalı.

### BUG-H6: `a.cfg` alanlarında data race — locksuz okuma/yazma

- **Dosya:** `internal/app/llama.go:82-111` (yazma), `internal/app/llm.go:619-621,699,886-888` (okuma)
- **Nedir:** `UpdateLlamaConfig()` (llama.go), `a.cfg.Llama.Temperature`, `TopP`, `MaxTokens` gibi alanları **hiçbir lock olmadan** yazar:
  ```go
  // llama.go:104
  a.cfg.Llama.Temperature = cfg.Temperature  // WRITE — no lock
  ```
  Aynı anda `callLLMStream()` (llm.go:619) bu alanları okur:
  ```go
  // llm.go:619
  Temperature: a.cfg.Llama.Temperature,  // READ — no lock
  ```
  App struct'ında `cfgMu` diye bir mutex yok. `clientMu`, `providerMu`, `storeMu` vb. var ama config için yok.
- **Kullanıcı etkisi:** Kullanıcı ayarları değiştirirken (örn. sıcaklık, token limiti) eşzamanlı LLM isteğinde **yanlış/bozuk değerler** kullanılabilir. String alanlarda (örn. `EngineMode`, `BinaryPath`) partial read riski.
- **Düzeltme:** `a.cfg` için `sync.RWMutex` eklenmeli veya `atomic.Value` ile sarmalanmalı.

### BUG-H7: `callLLM` hata string'leri geçerli yanıt olarak session/memory'e kaydediliyor

- **Dosya:** `internal/app/llm.go:830-927`, `internal/app/chat.go:42,66`
- **Nedir:** `callLLM()` hata durumunda `"⚠️ Yerel model yüklenmemiş..."` gibi bir hata string'i döner. `handleIncognito` (chat.go:42-47) bu string'i `assistant` rolüyle session'a ve memory'ye kaydeder. Hata mesajı hiçbir filtreden geçmez.
- **Kullanıcı etkisi:** Chat geçmişi `"⚠️"` ile başlayan hata mesajlarıyla dolar. Memory veritabanı bu hata string'leriyle kirlenir. Sonraki RAG aramaları bu hataları "ilgili bağlam" olarak dönebilir → **hafıza kalitesi düşer**.
- **Düzeltme:** `callLLM` hata durumunda boş string dönmeli, hata ayrı bir kanaldan (error return) iletilmeli. Veya `handleIncognito` hata mesajlarını filtrelemeli.

### BUG-H8: Flutter WhatsApp streaming Stop butonu çalışmıyor

- **Dosya:** `frontend/lib/widgets/chat_input.dart:180`, `frontend/lib/providers/chat_provider.dart:258`
- **Nedir:** `_sendWhatsApp()` kendi içinde lokal bir `CancelToken` oluşturur (line 188) ve `api.sendWhatsAppChatStream()`'e geçer. Stop butonu `messagesProvider.notifier.stopStreaming()` çağırır, bu da `MessagesNotifier._cancelToken`'ı iptal eder — **tamamen farklı bir token**. WhatsApp stream'in cancel token'ı hiçbir yere expose edilmez.
- **Kullanıcı etkisi:** WhatsApp modunda **Stop butonu işlevsizdir**. Kullanıcı yanıtı durduramaz. `isSendingProvider` temizlendiği için UI "göndermeye hazır" görünür ama stream arkada devam eder → UI tutarsız olur.
- **Düzeltme:** WhatsApp cancel token'ı `MessagesNotifier` seviyesinde yönetilmeli, veya `_sendWhatsApp` cancel token'ı dışarıya expose edilmeli.

### BUG-H9: Cloud sync WAL checkpoint hataları sessizce yutuluyor → bozuk yedek fark edilmez

- **Dosya:** `internal/cloudsync/sync_manager.go:415`
- **Nedir:**
  ```go
  db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")  // error return ignored!
  ```
  `sql.DB.Exec` `(sql.Result, error)` döner. Hata kontrolü yok. Eğer checkpoint başarısız olursa (DB kilitli, I/O hatası), kod sessizce devam eder ve checkpoint edilmemiş `memory.db` buluta yedeklenir.
- **Kullanıcı etkisi:** Checkpoint sessizce başarısız olduğunda **bulut yedeği eksik/bozuk olur** ve kullanıcının bundan haberi olmaz.
- **Düzeltme:** Hata kontrolü eklenmeli, başarısız checkpoint loglanmalı ve backup atlanmalı/ertelenmeli.

### BUG-H10: `runObserverAnalysis` ve `proactiveEngine` yanlış context kullanıyor → Shutdown'da durmuyorlar

- **Dosya:** `internal/app/app.go:299,311`
- **Nedir:**
  ```go
  go a.runObserverAnalysis(ctx)        // ctx = Startup()'a gelen parametre
  go a.proactiveEngine.Start(ctx)      // aynı
  ```
  `Shutdown()` sadece `a.lifecycleCancel()` ile `a.lifecycleCtx`'i iptal eder. Bu iki goroutine `lifecycleCtx` yerine ana `ctx`'i dinlediği için **Shutdown tarafından durdurulamaz**. Sadece process exit ile sonlanırlar.
- **Kullanıcı etkisi:** Uygulama kapandıktan sonra observer analizi ve proaktif öneri motoru arka planda çalışmaya devam eder. CPU/LLM kaynağı tüketir, gereksiz embedding hesaplamaları yapar.
- **Düzeltme:** `a.lifecycleCtx` kullanılmalı: `go a.runObserverAnalysis(a.lifecycleCtx)`.

---

## 🟡 MEDIUM — İşlevsellik Bozukluğu / Veri Bütünlüğü Riski

### BUG-M1: `mood.db` bulut yedeklemede WAL checkpoint olmadan arşivleniyor

- **Dosya:** `internal/cloudsync/sync_manager.go:474-484`
- **Nedir:** `mood.db`, `memory.db`'nin aksine `PRAGMA wal_checkpoint(TRUNCATE)` çalıştırılmadan zip'e eklenir. WAL'deki ruh hali kayıtları eksik kalır.
- **Kullanıcı etkisi:** Bulut yedeğinde ruh hali verisi eksik olabilir. Lokal veri sağlamdır.
- **Düzeltme:** `memory.db` ile aynı checkpoint pattern'i `mood.db` için de uygulanmalı.

### BUG-M2: Import kısmı hata → yarımlanmış state, rollback yok

- **Dosya:** `internal/app/backup.go:98-147`
- **Nedir:** ZIP entry'leri sırayla yazılır. Entry N başarısız olursa, 0..N-1 arası entry'ler çoktan diske yazılmıştır. Hata dönülür ama **yazılan dosyalar geri alınmaz**. Kısmi import sonucu bazı dosyalar yeni, bazıları eski kalır.
- **Kullanıcı etkisi:** Import başarısız olduğunda uygulama **tutarsız bir state'te** kalır — örn. yeni `memory.db` ama eski `providers.json`.
- **Düzeltme:** Import öncesi tam snapshot alınıp, başarısızlıkta geri döndürülmeli. Veya önce temp dizine extract edilip, başarılı olursa atomik olarak taşınmalı.

### BUG-M3: `copyFile` fallback hardcoded 0666 → hassas dosyalar world-readable

- **Dosya:** `internal/fileutil/atomic.go:42`
- **Nedir:** `AtomicWrite` Windows'ta `os.Rename` başarısız olunca `copyFile` fallback'ine geçer. `copyFile` hedef dosyayı **her zaman 0666** (world-readable/writable) ile açar:
  ```go
  out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
  ```
  Oysa çağıran taraf `0600` perm parametresi geçmiştir (örn. session dosyaları, `machine.key`). Bu parametre `copyFile`'a iletilmez.
- **Kullanıcı etkisi:** **Windows'ta** `os.Rename` fallback tetiklenirse, session dosyaları ve `machine.key` **makinedeki tüm kullanıcılar tarafından okunabilir** hale gelir.
- **Düzeltme:** `copyFile`'a `perm` parametresi eklenmeli ve `AtomicWrite`'tan iletilmeli.

### BUG-M4: Cloud restore dosyaları 0644 ile yazıyor → API anahtarları açığa çıkar

- **Dosya:** `internal/cloudsync/sync_manager.go:586,631`
- **Nedir:** `os.Create(tmpDest)` varsayılan `0666 & ~umask` (genelde 0644) ile oluşturur. `copyRestoreFile` (line 631) hardcoded `0644` kullanır. Bu dosyalar arasında `providers.json` (şifreli API key'leri içerir), `permissions.json`, `orchestra.json` bulunur.
- **Kullanıcı etkisi:** Buluttan restore edilen hassas konfigürasyon dosyaları **diğer lokal kullanıcılar tarafından okunabilir**.
- **Düzeltme:** Restore işleminde hassas dosyalar için `0600` kullanılmalı.

### BUG-M5: Agent backup history yazma hataları sessizce yutuluyor

- **Dosya:** `internal/agent/backup.go:74-77`
- **Nedir:**
  ```go
  if err == nil {
      os.WriteFile(bm.historyFile, data, 0600)  // write error ignored
  }
  ```
  Write hatası kontrol edilmez. Bellek içi history güncellenir ama diske yazılamazsa, **disk ve bellek state'i birbirinden kopar**.
- **Kullanıcı etkisi:** Undo işlemi, diske yazılamamış ama bellekte var olan bir backup'a referans verip **dosya bulunamadı hatası** alabilir.
- **Düzeltme:** Write hatası kontrol edilmeli, başarısız olursa in-memory state de geri alınmalı.

### BUG-M6: `startupTailscale` goroutine'i web sunucusu kurulmadan çalışıyor → hiç çalışmaz

- **Dosya:** `internal/app/app.go:342`, `internal/app/remote_tailscale.go:126-143`, `main.go:26-30`
- **Nedir:** `Startup()` içinde `go a.startupTailscale()` çağrılır (app.go:342). Ancak `StartWebServerHTTP()` **daha sonra** main.go:30'da çağrılır. `startupTailscale()` içinde `a.getWebServer()` nil döner, `ws != nil` kontrolü başarısız olur ve fonksiyon sessizce return eder. Tailscale otomatik başlatma **hiçbir zaman tetiklenmez**.
- **Kullanıcı etkisi:** "Tailscale auto-start" ayarı açık olsa bile **çalışmaz**. Kullanıcı manuel başlatmak zorunda kalır.
- **Düzeltme:** `startupTailscale()` çağrısı `StartWebServerHTTP` sonrasına alınmalı, veya web server set edilene kadar retry mekanizması eklenmeli.

### BUG-M7: Flutter type-unsafe `_guard<List>.cast<Map>()` → iterator anında TypeError

- **Dosya:** `frontend/lib/core/api_client.dart:867,904,992,1004`
- **Nedir:** `_guard<T>()` generic type check için `data is T` kullanır. `List<Map<String, dynamic>>` için bu çalışmaz (Dart generic reification). `.cast<Map<String, dynamic>>()` lazy'dir — hata iteration anında (`.map()`, `.forEach()`) ortaya çıkar. Backend verisi bozuksa, **çağrı yeri değil iterator crash olur**.
- **Kullanıcı etkisi:** Bozuk/ beklenmedik API yanıtında permissions sayfası, agent listesi vb. **beklenmedik yerde crash**.
- **Düzeltme:** `_guard` generic liste için özel handling yapmalı, veya element bazında type check eklenmeli.

### BUG-M8: Flutter WhatsApp optimistic mesajlar sayfa değişiminde temizlenmiyor → hayalet mesaj

- **Dosya:** `frontend/lib/screens/whatsapp_screen.dart:654-701`
- **Nedir:** `_sendMessage()` optimistic mesajı `_optimistic` listesine ekler (line 671). Başarıda (line 679) veya hatada (line 687) `mounted` kontrolü ile siler. Ancak kullanıcı **async işlem tamamlanmadan önce sohbet değiştirir veya ekrandan çıkarsa**, `mounted == false` olur ve silme atlanır. Mesaj `_optimistic` listesinde **sonsuza kadar** kalır. Aynı sohbete dönüldüğünde `_optimistic.where((m) => m.chatJid == _selectedJid)` bu hayalet mesajı gösterir.
- **Kullanıcı etkisi:** WhatsApp'ta **mükerrer/hayalet mesaj** görünümü. Gönderilmiş gibi görünen ama aslında gönderilmemiş mesajlar.
- **Düzeltme:** `dispose()` içinde `_optimistic.clear()`. Veya screen'den çıkarken pending optimistic mesajları temizleyen bir mekanizma.

### BUG-M9: `bash -c` ile command injection riski (mevcut, önceden biliniyor)

- **Dosya:** `internal/agent/tools/command.go:164`
- **Nedir:** Blacklist tabanlı komut filtreleme. Encoding tricks ile atlatılabilir.
- **Not:** Tasarım gereği — agent tool onayı gerektirir. Yine de hardening yapılmalı.
- **Kullanıcı etkisi:** Agent modunda zararlı komut çalıştırma riski (onay dialog'u olsa da).
- **Düzeltme:** Whitelist yaklaşımı veya sandbox.

### BUG-M10: `model_store_screen.dart` — 2500+ satır tek dosya (mevcut, önceden biliniyor)

- **Dosya:** `frontend/lib/screens/model_store_screen.dart`
- **Kullanıcı etkisi:** Doğrudan bug değil ama bakım yapılamaz hale geliyor, değişikliklerde kırılma riski yüksek.

### BUG-M11: Mobile API client eksik (mevcut, önceden biliniyor)

- **Dosya:** `mobile/lib/core/api_client.dart`
- **Kullanıcı etkisi:** Mobil uygulamada birçok backend endpoint'i kullanılamaz.

### BUG-M12: `connectionStatusProvider` sonsuz polling (mevcut, önceden biliniyor)

- **Dosya:** `frontend/lib/providers/chat_provider.dart:677-690`
- **Kullanıcı etkisi:** 30 saniyede bir `isAlive()` sorgusu, provider dispose olsa bile devam eder. Performans sızıntısı.

---

## 🟢 LOW — Küçük Kusurlar

### BUG-L1: AtomicWrite `.tmp` orphan dosyaları

- **Dosya:** `internal/fileutil/atomic.go:22-31`
- **Nedir:** `os.WriteFile(tmp)` başarılı, `os.Rename` başarısız, `copyFile` da başarısız olursa `.tmp` dosyası diskte kalır.
- **Etki:** Zamanla `.tmp` dosyaları birikir. Export bunları atlar (`filepath.Ext != ".tmp"`), session loader `.json` olmayanları ignore eder.

### BUG-L2: WhatsApp `whatsappChatMu` gereksiz mutex

- **Dosya:** `internal/app/whatsapp.go:167-175`
- **Nedir:** `whatsappChatMode` boolean'ı için tam `sync.Mutex` kullanılıyor. `atomic.Bool` daha uygun olur.

### BUG-L3: `runObserverAnalysis` ilk turda shutdown'a 30sn gecikmeli yanıt

- **Dosya:** `internal/app/app.go:549-553`
- **Nedir:** İlk `select` bloğunda 30 saniyelik `time.After`, shutdown sinyalini geciktirir.

### BUG-L4: Flutter double-reset `streamingAgentEventsProvider`

- **Dosya:** `frontend/lib/providers/chat_provider.dart:264-266`
- **Nedir:** `stopStreaming()` içinde `streamingAgentEventsProvider` iki kez sıfırlanıyor (copy-paste):
  ```dart
  ref.read(streamingAgentEventsProvider.notifier).state = [];     // line 264
  ref.read(streamingStatusProvider.notifier).state = '';          // line 265
  ref.read(streamingAgentEventsProvider.notifier).state = [];     // line 266 — duplicate
  ```
- **Etki:** Gereksiz bir state notification ve widget rebuild.

### BUG-L5: Flutter `_AuthorAvatarState` cache sınırsız büyüme

- **Dosya:** `frontend/lib/screens/model_store_screen.dart:844`
- **Nedir:** `static final _cache = <String, String?>` — process lifetime, hiç expire olmaz.
- **Etki:** Çok uzun oturumlarda hafif bellek büyümesi (~200 byte/author).

### BUG-L6: Flutter `_loadReadme` sessizce tüm HTTP hatalarını yutar

- **Dosya:** `frontend/lib/screens/model_store_screen.dart:1230`
- **Nedir:** `catch (_)` tüm exception'ları yakalar, kullanıcıya hiçbir hata gösterilmez.
- **Etki:** README yüklenemediğinde kullanıcı sebebini bilemez (ağ hatası mı, yok mu?).

---

## Daha Önce Düzeltilmiş Bug'lar (Referans — 35 adet)

| # | Madde | Session |
|---|-------|---------|
| 1 | AgentStatusBar.events.last crash | 2 |
| 2 | qrCodes.first crash | 2 |
| 3 | Unsafe casts (6 location) | 2 |
| 4 | as RenderBox crash | 2 |
| 5 | Gemini API key URL'de sızıyor | 2 |
| 6 | Goroutine leak (4 adet) | 3 |
| 9 | Provider priority UI eksik | — |
| 10 | Orchestra fallback kullanmıyor | — |
| 11 | Nil client dereference (3 yer) | 2 |
| 12 | HTTP client timeout yok | 2 |
| 13 | Logging migration eksik | — |
| 16 | Mounted check eksik | 2 |
| 17 | const constructor eksik | — |
| 18 | Silent error ignore | 2 |
| 19 | Skill dialog Windows path | 2 |
| 24 | os.Exit(42) — veri kaybı | 3 |
| 25 | Health check goroutine leak | 3 |
| 26 | store.Close() çağrılmıyor | 3 |
| 27 | 28+ unsafe as cast (Flutter) | 3 |
| 28 | Data race: whisperServer | 3 |
| 29 | Data race: webServer | 3 |
| 30 | WhatsApp autoReconnect leak | 3 |
| 31 | resolveAgentProvider race | 3 |
| 32 | incrementRetrieveCounts UAC | 3 |
| 33 | Observer unbounded goroutine | 3 |
| 34 | Ngrok stopCh'siz sleep | 3 |
| 35 | GetEnabled() priority sırasız | 3 |
| 37 | whatsapp_provider.dart unsafe cast | 3 |
| 38 | **C1** — WhatsApp nil panic (30+ locksuz) | 5 |
| 39 | **C3** — Import atomic write | 5 |
| 40 | **C4** — .machine-id wipe koruması | 5 |
| 41 | **H1** — memorySaveCh shutdown panic | 5 |
| 42 | **H3** — Export WAL checkpoint | 5 |
| 43 | **H4** — Config.yaml atomic write | 5 |
| 44 | **H5** — Provider config atomic write | 5 |

---

## Düzeltme Öncelik Sırası (Roadmap)

### Faz 1 — Acil (Stable sürüm için şart) ✅ TAMAMLANDI (2026-06-30)

| # | Bug | Tahmini Süre | Etki | Durum |
|---|-----|-------------|------|-------|
| C1 | WhatsApp `c.waClient` locksuz okuma (30+ yer) | 2 saat | Crash | ✅ `854e04d` |
| C3 | Import atomic write | 30 dk | Veri kaybı | ✅ `b00b800` |
| C4 | `.machine-id` wipe koruması | 15 dk | Cloud key kaybı | ✅ `b006911` |
| H4 | Config atomic write | 15 dk | Ayar kaybı | ✅ `1384e52` |
| H5 | Provider config atomic write | 15 dk | API key kaybı | ✅ `41cc723` |
| H1 | `memorySaveCh` close sonrası panic | 1 saat | Shutdown crash | ✅ `86a0045` |
| H3 | Export WAL checkpoint | 20 dk | Eksik yedek | ✅ `be4d6ae` |

### Faz 2 — Yüksek Öncelik

| # | Bug | Tahmini Süre | Etki |
|---|-----|-------------|------|
| C2 | WhatsApp `handleEvent` data race | 1.5 saat | Data corruption |
| H6 | `a.cfg` data race | 1 saat | Yanlış LLM parametreleri |
| H7 | Hata string'leri memory'e kaydediliyor | 30 dk | Hafıza kirliliği |
| H8 | Flutter WhatsApp Stop butonu | 1 saat | Kullanıcı deneyimi |
| H2 | WhatsApp `autoReconnect` TOCTOU | 30 dk | Crash |
| H9 | Cloud sync checkpoint hatası yutma | 10 dk | Bozuk yedek |
| H10 | Observer/proactive yanlış context | 15 dk | Kaynak sızıntısı |

### Faz 3 — Orta Öncelik

| # | Bug | Tahmini Süre |
|---|-----|-------------|
| M1 | `mood.db` WAL checkpoint | 15 dk |
| M2 | Import rollback | 1 saat |
| M3 | `copyFile` 0666 hardcode | 10 dk |
| M4 | Cloud restore 0644 | 15 dk |
| M5 | Agent backup history error handling | 10 dk |
| M6 | `startupTailscale` race | 30 dk |
| M7 | Flutter `_guard<List>.cast` | 30 dk |
| M8 | Flutter WhatsApp optimistic mesaj temizleme | 20 dk |

### Faz 4 — Düşük Öncelik / Mevcut Borç

| # | Bug | Tahmini Süre |
|---|-----|-------------|
| M9 | `bash -c` hardening | 30 dk |
| M10 | `model_store_screen` refactor | 2 saat |
| M11 | Mobile API client | 4 saat |
| M12 | `connectionStatusProvider` polling | 10 dk |
| L1-L6 | Düşük öncelikli kusurlar | 1 saat |

---

## Test Coverage Durumu

```
go build ./...   ✅ (0 hata)
go vet ./...     ✅ (0 uyarı)
go test ./... -race -count=1  → tüm paketler PASS
```

**Not:** `go test -race` mevcut testlerde data race bulamaz çünkü testler senkron/kısa ömürlü. Yukarıdaki race condition'lar (C1, C2, H2, H6) production yükü altında veya eşzamanlı HTTP isteklerinde ortaya çıkar.
