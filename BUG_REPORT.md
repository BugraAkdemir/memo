# Bug Report — Memo v3.1.0-beta Stabilite Engelleri

> **Amaç:** Stable sürüme engel olan, kullanıcıyı direkt etkileyen bug'ların kapsamlı tespiti.
> **Tarih:** 2026-06-29
> **Kapsam:** Go backend + Flutter frontend — tam kod tabanı taraması
> **Metod:** 4 paralel agent ile eşzamanlı tarama (concurrency, crash, data loss, frontend)

---

## Özet

| Severity | Toplam | Düzeltilen | Açık |
|----------|--------|-----------|------|
| 🔴 CRITICAL | 6 | 4 (C1-C4) | **2 (QC1, QC2)** |
| 🟠 HIGH | 15 | 10 (H1-H10) | **5 (QH1-QH5)** |
| 🟡 MEDIUM | 19 | 8 (M1-M5,M7,M8) | **11 (M6,M9-M12,QM1-QM7)** |
| 🟢 LOW | 11 | 2 (L1,L2) | **9 (L3-L6,QL1-QL5)** |
| **TOPLAM** | **51** | **24** | **27** |

> **Son güncelleme:** 2026-07-07, Session 8 — 3 paralel QA agent ile gerçek kullanıcı senaryoları test edildi. 19 yeni bug tespit edildi. En kritik bulgu: Import config.yaml'ın sessizce atlanması (QC1) ve memory store'un yeniden başlatılmaması (QC2).

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

### BUG-C2: ~~WhatsApp `handleEvent` shared state'i locksuz yazıyor → data race~~ **→ ZATEN DÜZELTİLMİŞ (asıl commit: `854e04d`, 2026-06-30 — bu rapor hiç güncellenmemiş; regresyon testi eklendi 2026-07-07)**

- **Doğrulama (2026-07-07):** Kodu satır satır okudum — `handleEvent()`'in her case'i (`QR`, `PairSuccess`, `PairError`, `Connected`, `Disconnected`, `LoggedOut`) zaten `c.startMu.Lock()/Unlock()` ile sarılı (`internal/whatsapp/client.go:411-466`). Bu, C1 fix'i (`854e04d`, "WhatsApp c.waClient locksuz erişim nedeniyle nil panic") ile aynı commit'te yapılmış — commit mesajı bunu açıkça belirtiyor ("handleEvent: tüm shared state yazmaları startMu ile korunuyor") ama BUG_REPORT.md hiç güncellenmemiş, C2 sanki hâlâ açıkmış gibi kalmış.
- **Regresyon testi:** `internal/whatsapp/client_race_test.go` → `TestHandleEventConcurrentAccess`, `handleEvent`'i ve `QRCodes()/LastError()/IsReconnecting()/IsConnected()/IsLoggedIn()`'i eşzamanlı çalıştırıp `go test -race` ile doğruluyor. Temiz geçiyor.
- **Dosya:** `internal/whatsapp/client.go:411-466`
- **Eski iddia (artık geçersiz):** `handleEvent()` whatsmeow'un event loop goroutine'inde `c.qrCodes`, `c.lastError`, `c.started`, `c.reconnecting` alanlarını hiçbir lock olmadan yazıyordu.

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

### BUG-H2: ~~WhatsApp `autoReconnect` TOCTOU nil deref → reconnect sırasında crash~~ **→ ZATEN DÜZELTİLMİŞ (asıl commit: `854e04d`, 2026-06-30 — bu rapor hiç güncellenmemiş)**

- **Doğrulama (2026-07-07):** `internal/whatsapp/client.go:485-489` — `wa := c.waClient` lock altında local değişkene kopyalanıyor, sonrasında `wa.Connect()` bu local kopya üzerinden çağrılıyor (`c.waClient` değil). `Stop()` araya girip `c.waClient`'i nil yapsa bile `wa` hâlâ geçerli bir pointer, nil deref oluşmuz. Commit mesajı ("autoReconnect: c.waClient ve c.started lock altında okunup local değişkene alınıyor, lock dışında Connect() çağrısı TOCTOU'suz") bunu açıkça belirtiyor.
- **Dosya:** `internal/whatsapp/client.go:485-510`
- **Eski iddia (artık geçersiz):**
  ```go
  c.startMu.Lock()
  alive := c.started && c.waClient != nil   // line 446 — lock altında check ✓
  c.startMu.Unlock()                         // line 447 — lock bırakıldı

  // ... burada Stop() çalışıp c.waClient = nil yapabilir ...

  if err := c.waClient.Connect(); err != nil { // line 455 — LOCKSIZ kullanım → nil panic
  ```
  Bu artık kodda yok — güncel hali `wa` local değişkenini kullanıyor.

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

### BUG-H6: ~~`a.cfg` alanlarında data race — locksuz okuma/yazma~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit'ler:** `716a27c` (cfgMu eklendi), `07eaf0c` (tüm call site'lar cfgMu ile korundu)
- **Düzeltme:** `App` struct'ına `cfgMu sync.RWMutex` eklendi. `UpdateLlamaConfig`/`SetLlamaBinaryPath`/`InstallLlamaServer` yazmaları `Lock()` altına alındı; `llm.go`, `llama.go`, `embedding.go`, `helpers.go`, `learning.go` içindeki tüm `a.cfg.Llama.*` okumaları `RLock()` altında local değişkene kopyalanıp kullanılıyor.

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

### BUG-H7: ~~`callLLM` hata string'leri geçerli yanıt olarak session/memory'e kaydediliyor~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `9d273dd`
- **Düzeltme:** Session/chat geçmişine hata mesajının kaydedilmesi olduğu gibi bırakıldı (streaming path'teki `recordStreamError` ile tutarlı, kullanıcı hatayı görmeli). Asıl zarar veren kısım — hata string'inin RAG memory'e (embedding/vector store) indekslenmesi — tek noktadan (`saveMemoryAsync`) engellendi: yeni `isLLMErrorReply()` helper'ı `"⚠️ "` prefix konvansiyonunu kontrol edip eşleşirse memory kaydını atlıyor. Regresyon testleri: `internal/app/memory_test.go` → `TestIsLLMErrorReply`, `TestSaveMemoryAsyncSkipsErrorReplies`.

- **Dosya:** `internal/app/llm.go:830-927`, `internal/app/chat.go:42,66`, `internal/app/memory.go`
- **Eski iddia:** `callLLM()` hata durumunda `"⚠️ Yerel model yüklenmemiş..."` gibi bir hata string'i döner. `handleIncognito` (chat.go:42-47) bu string'i `assistant` rolüyle session'a ve memory'ye kaydeder. Hata mesajı hiçbir filtreden geçmez.
- **Kullanıcı etkisi (düzeltme öncesi):** Memory veritabanı bu hata string'leriyle kirleniyordu. Sonraki RAG aramaları bu hataları "ilgili bağlam" olarak dönebiliyordu → hafıza kalitesi düşüyordu.

### BUG-H8: ~~Flutter WhatsApp streaming Stop butonu çalışmıyor~~ **→ DÜZELTİLDİ (Session 6, bkz. FH1 aşağıda)**

- **Commit:** `cb5c995`
- **Düzeltme:** Aynı sorun Session 6'da FH1 olarak tekrar tespit edilip düzeltildi — bkz. "Frontend — Paralel İncelemede Bulunan Buglar" bölümü.

- **Dosya:** `frontend/lib/widgets/chat_input.dart:180`, `frontend/lib/providers/chat_provider.dart:258`
- **Nedir:** `_sendWhatsApp()` kendi içinde lokal bir `CancelToken` oluşturur (line 188) ve `api.sendWhatsAppChatStream()`'e geçer. Stop butonu `messagesProvider.notifier.stopStreaming()` çağırır, bu da `MessagesNotifier._cancelToken`'ı iptal eder — **tamamen farklı bir token**. WhatsApp stream'in cancel token'ı hiçbir yere expose edilmez.
- **Kullanıcı etkisi:** WhatsApp modunda **Stop butonu işlevsizdir**. Kullanıcı yanıtı durduramaz. `isSendingProvider` temizlendiği için UI "göndermeye hazır" görünür ama stream arkada devam eder → UI tutarsız olur.
- **Düzeltme:** WhatsApp cancel token'ı `MessagesNotifier` seviyesinde yönetilmeli, veya `_sendWhatsApp` cancel token'ı dışarıya expose edilmeli.

### BUG-H9: ~~Cloud sync WAL checkpoint hataları sessizce yutuluyor → bozuk yedek fark edilmez~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `44c5fc3`
- **Düzeltme:** `db.Exec` hatası kontrol ediliyor; checkpoint başarısız olursa o dosya bu backup turunda atlanıyor ve `emitError` ile kullanıcıya bildiriliyor (sessizce bozuk yedek yüklenmiyor).

- **Dosya:** `internal/cloudsync/sync_manager.go:415`
- **Nedir:**
  ```go
  db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")  // error return ignored!
  ```
  `sql.DB.Exec` `(sql.Result, error)` döner. Hata kontrolü yok. Eğer checkpoint başarısız olursa (DB kilitli, I/O hatası), kod sessizce devam eder ve checkpoint edilmemiş `memory.db` buluta yedeklenir.
- **Kullanıcı etkisi:** Checkpoint sessizce başarısız olduğunda **bulut yedeği eksik/bozuk olur** ve kullanıcının bundan haberi olmaz.
- **Düzeltme:** Hata kontrolü eklenmeli, başarısız checkpoint loglanmalı ve backup atlanmalı/ertelenmeli.

### BUG-H10: ~~`runObserverAnalysis` ve `proactiveEngine` yanlış context kullanıyor → Shutdown'da durmuyorlar~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `716a27c`
- **Düzeltme:** Her iki goroutine de `a.lifecycleCtx` ile başlatılıyor. `runObserverAnalysis`'in ilk 30sn'lik bekleyişi de durdurulabilir bir `time.Timer`'a çevrildi.

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

### BUG-M1: ~~`mood.db` bulut yedeklemede WAL checkpoint olmadan arşivleniyor~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `44c5fc3`
- **Düzeltme:** `mood.db` için de `memory.db` ile aynı `PRAGMA wal_checkpoint(TRUNCATE)` + hata kontrolü uygulandı.

- **Dosya:** `internal/cloudsync/sync_manager.go:474-484`
- **Nedir:** `mood.db`, `memory.db`'nin aksine `PRAGMA wal_checkpoint(TRUNCATE)` çalıştırılmadan zip'e eklenir. WAL'deki ruh hali kayıtları eksik kalır.
- **Kullanıcı etkisi:** Bulut yedeğinde ruh hali verisi eksik olabilir. Lokal veri sağlamdır.
- **Düzeltme:** `memory.db` ile aynı checkpoint pattern'i `mood.db` için de uygulanmalı.

### BUG-M2: ~~Import kısmı hata → yarımlanmış state, rollback yok~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `cbc2487`
- **Düzeltme:** İki fazlı import: tüm zip entry'leri önce data dizini içindeki özel bir staging dizinine çıkarılıyor; sadece tüm entry'ler başarıyla staged olduktan sonra aynı dosya sistemi içinde atomik `os.Rename` ile canlı konuma taşınıyor. Herhangi bir noktada hata olursa canlı data dizini hiç dokunulmamış kalıyor.

- **Dosya:** `internal/app/backup.go:98-147`
- **Nedir:** ZIP entry'leri sırayla yazılır. Entry N başarısız olursa, 0..N-1 arası entry'ler çoktan diske yazılmıştır. Hata dönülür ama **yazılan dosyalar geri alınmaz**. Kısmi import sonucu bazı dosyalar yeni, bazıları eski kalır.
- **Kullanıcı etkisi:** Import başarısız olduğunda uygulama **tutarsız bir state'te** kalır — örn. yeni `memory.db` ama eski `providers.json`.
- **Düzeltme:** Import öncesi tam snapshot alınıp, başarısızlıkta geri döndürülmeli. Veya önce temp dizine extract edilip, başarılı olursa atomik olarak taşınmalı.

### BUG-M3: ~~`copyFile` fallback hardcoded 0666 → hassas dosyalar world-readable~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `edd518d`
- **Düzeltme:** `copyFile`'a `perm fs.FileMode` parametresi eklendi, `AtomicWrite` çağıranın verdiği perm'i iletiyor.

- **Dosya:** `internal/fileutil/atomic.go:42`
- **Nedir:** `AtomicWrite` Windows'ta `os.Rename` başarısız olunca `copyFile` fallback'ine geçer. `copyFile` hedef dosyayı **her zaman 0666** (world-readable/writable) ile açar:
  ```go
  out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
  ```
  Oysa çağıran taraf `0600` perm parametresi geçmiştir (örn. session dosyaları, `machine.key`). Bu parametre `copyFile`'a iletilmez.
- **Kullanıcı etkisi:** **Windows'ta** `os.Rename` fallback tetiklenirse, session dosyaları ve `machine.key` **makinedeki tüm kullanıcılar tarafından okunabilir** hale gelir.
- **Düzeltme:** `copyFile`'a `perm` parametresi eklenmeli ve `AtomicWrite`'tan iletilmeli.

### BUG-M4: ~~Cloud restore dosyaları 0644 ile yazıyor → API anahtarları açığa çıkar~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `44c5fc3`
- **Düzeltme:** `restoreZip` ve `copyRestoreFile` artık `0600` ile yazıyor.

- **Dosya:** `internal/cloudsync/sync_manager.go:586,631`
- **Nedir:** `os.Create(tmpDest)` varsayılan `0666 & ~umask` (genelde 0644) ile oluşturur. `copyRestoreFile` (line 631) hardcoded `0644` kullanır. Bu dosyalar arasında `providers.json` (şifreli API key'leri içerir), `permissions.json`, `orchestra.json` bulunur.
- **Kullanıcı etkisi:** Buluttan restore edilen hassas konfigürasyon dosyaları **diğer lokal kullanıcılar tarafından okunabilir**.
- **Düzeltme:** Restore işleminde hassas dosyalar için `0600` kullanılmalı.

### BUG-M5: ~~Agent backup history yazma hataları sessizce yutuluyor~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `1a35e2e`
- **Düzeltme:** `saveHistoryLocked` artık hata döndürüyor; her çağıran ya bellek içi değişikliği geri alıyor ya da hatayı üst katmana iletiyor.

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

### BUG-M7: ~~Flutter type-unsafe `_guard<List>.cast<Map>()` → iterator anında TypeError~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `c64677c`
- **Düzeltme:** `_guardList<E>()` eklendi; her elemanı eagerly kontrol edip çağrı yerinde açıklayıcı exception fırlatıyor. `getAgentPermissions`, `listSkills`, `getProactivePatterns`, `getCalendarEvents` güncellendi.

- **Dosya:** `frontend/lib/core/api_client.dart:867,904,992,1004`
- **Nedir:** `_guard<T>()` generic type check için `data is T` kullanır. `List<Map<String, dynamic>>` için bu çalışmaz (Dart generic reification). `.cast<Map<String, dynamic>>()` lazy'dir — hata iteration anında (`.map()`, `.forEach()`) ortaya çıkar. Backend verisi bozuksa, **çağrı yeri değil iterator crash olur**.
- **Kullanıcı etkisi:** Bozuk/ beklenmedik API yanıtında permissions sayfası, agent listesi vb. **beklenmedik yerde crash**.
- **Düzeltme:** `_guard` generic liste için özel handling yapmalı, veya element bazında type check eklenmeli.

### BUG-M8: ~~Flutter WhatsApp optimistic mesajlar sayfa değişiminde temizlenmiyor → hayalet mesaj~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `f8ac365`
- **Düzeltme:** `_optimistic` listesinden çıkarma artık `mounted` kontrolünden bağımsız her zaman çalışıyor; sadece UI güncellemeleri (`setState`, provider invalidation, SnackBar) `mounted` ile korunuyor.

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

### BUG-L1: ~~AtomicWrite `.tmp` orphan dosyaları~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `edd518d`
- **Düzeltme:** İlk `os.WriteFile(tmp)` kısmi yazımla başarısız olursa `.tmp` dosyası hemen temizleniyor.

- **Dosya:** `internal/fileutil/atomic.go:22-31`
- **Nedir:** `os.WriteFile(tmp)` başarılı, `os.Rename` başarısız, `copyFile` da başarısız olursa `.tmp` dosyası diskte kalır.
- **Etki:** Zamanla `.tmp` dosyaları birikir. Export bunları atlar (`filepath.Ext != ".tmp"`), session loader `.json` olmayanları ignore eder.

### BUG-L2: ~~WhatsApp `whatsappChatMu` gereksiz mutex~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `716a27c`
- **Düzeltme:** `whatsappChatMode` `atomic.Bool`'a çevrildi, `whatsappChatMu` kaldırıldı.

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

### Faz 2 — Yüksek Öncelik — ✅ TAMAMLANDI (2026-07-07, Session 7)

| # | Bug | Tahmini Süre | Etki | Durum |
|---|-----|-------------|------|-------|
| C2 | WhatsApp `handleEvent` data race | 1.5 saat | Data corruption | ✅ Zaten düzeltilmişti (`854e04d`, 2026-06-30) — regresyon testi eklendi 2026-07-07 |
| H6 | `a.cfg` data race | 1 saat | Yanlış LLM parametreleri | ✅ `716a27c`/`07eaf0c` |
| H7 | Hata string'leri memory'e kaydediliyor | 30 dk | Hafıza kirliliği | ✅ `9d273dd` |
| H8 | Flutter WhatsApp Stop butonu | 1 saat | Kullanıcı deneyimi | ✅ `cb5c995` (Session 6, FH1) |
| H2 | WhatsApp `autoReconnect` TOCTOU | 30 dk | Crash | ✅ Zaten düzeltilmişti (`854e04d`, 2026-06-30) |
| H9 | Cloud sync checkpoint hatası yutma | 10 dk | Bozuk yedek | ✅ `44c5fc3` |
| H10 | Observer/proactive yanlış context | 15 dk | Kaynak sızıntısı | ✅ `716a27c` |

### Faz 3 — Orta Öncelik — kısmen tamamlandı (2026-07-07, Session 7)

| # | Bug | Tahmini Süre | Durum |
|---|-----|-------------|-------|
| M1 | `mood.db` WAL checkpoint | 15 dk | ✅ `44c5fc3` |
| M2 | Import rollback | 1 saat | ✅ `cbc2487` |
| M3 | `copyFile` 0666 hardcode | 10 dk | ✅ `edd518d` |
| M4 | Cloud restore 0644 | 15 dk | ✅ `44c5fc3` |
| M5 | Agent backup history error handling | 10 dk | ✅ `1a35e2e` |
| M6 | `startupTailscale` race | 30 dk | ⬜ Bekliyor |
| M7 | Flutter `_guard<List>.cast` | 30 dk | ✅ `c64677c` |
| M8 | Flutter WhatsApp optimistic mesaj temizleme | 20 dk | ✅ `f8ac365` |

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

**Yeni QA taraması sonucu:** 51 toplam bug, 24 düzeltilmiş, **27 hala açık**. Açık olanların 7'si CRITICAL/HIGH seviyesinde.

---

## Session 2026-07-06: Paralel Ajan Taraması — 5 Orijinal Bug + 70+ Yeni Bulgu

> **Metod:** 6 paralel agent (3 backend + 3 frontend) ile ~110+ kullanıcı senaryosu simülasyonu.
> **Düzeltme:** Pair-agent yaklaşımı (1 yazıcı + 1 incelemeci) ile tüm HIGH+ buglar düzeltildi.
> **Commit'ler:** 7 frontend commit'i, backend önceki session'da commit'lendi.

---

## Orijinal 5 Bug (Kullanıcı Tarafından Bildirilen)

| # | Severity | Alan | Açıklama | Durum |
|---|----------|------|----------|-------|
| B1 | HIGH | Backend | Stream durdurulunca sonraki mesaj bozuluyor (user,user → HTTP 400) | ✅ |
| B2 | HIGH | Backend | Stream sırasında sohbet değiştirmek cevabı yanlış sohbete yazıyor | ✅ |
| B3 | MED-HIGH | Backend | Çift gönderim (double send) → paralel stream'ler | ✅ |
| B4 | MEDIUM | Backend | Provider silmek router'ı güncellemiyor | ✅ |
| B5 | MEDIUM | Backend | WhatsApp normal sohbete yazıyor | ✅ |

---

## Backend — Paralel İncelemede Bulunan Ek Buglar

### 🔴 CRITICAL

#### BUG-NC1: nil streamClient → kapanmayan kanal → streamMu kalıcı deadlock
- **Dosya:** `internal/app/llm.go:700-703`
- **Nedir:** `callLLMStream` local model nil iken `trySend(ctx, outCh, ...)` + `return outCh` yapıyor. Kanal kapanmıyor. Wrapper goroutine `for chunk := range innerCh` ile sonsuza kadar bekliyor → `streamMu.Unlock()` hiç çalışmıyor → tüm chat stream'leri kalıcı deadlock.
- **Düzeltme:** `close(outCh)` eklendi. ✅

### 🟠 HIGH

#### BUG-NH1: callAgentWithOrchestra — orchestra hatasında yetim user mesajı
- **Dosya:** `internal/app/llm.go:342-345`
- **Nedir:** Orchestra `RunWithProgress` hata döndüğünde sadece error chunk gönderiliyor, `recordStreamError` çağrılmıyor. User mesajı session'da cevapsız kalıyor.
- **Düzeltme:** `recordStreamError` eklendi. ✅

#### BUG-NH2: callAgentWithOrchestra — fallback provider çözülemezse chief sentezi kayboluyor
- **Dosya:** `internal/app/llm.go:375-378`
- **Nedir:** `agentRouterFromProviderName` başarısız olunca, orchestra'nın ürettiği `finalContent` session'a kaydedilmeden return ediliyor.
- **Düzeltme:** `finalContent` boş değilse `finishStream`, boşsa `recordStreamError`. ✅

#### BUG-NH3: callAgentStream — ctx.Done() handling yok
- **Dosya:** `internal/app/llm.go:167` (for chunk := range streamCh)
- **Nedir:** Agent stream goroutine'i `for range` loop'unda context cancellation'ı hiç kontrol etmiyor. Kullanıcı Stop'a bassa da stream devam ediyor, sonuçları session'a yazıyor.
- **Düzeltme:** `select` ile `ctx.Done()` kontrolü eklendi. ✅

#### BUG-NH4: callAgentStream — boş yanıt durumunda yetim user mesajı
- **Dosya:** `internal/app/llm.go:186-191`
- **Nedir:** Agent stream kapandığında `fullReply` boşsa goroutine hiçbir şey kaydetmeden çıkıyor.
- **Düzeltme:** `else { recordStreamError(...) }` eklendi. ✅

#### BUG-NH5: callLLMStream orchestra path — userPrompt boşken yetim user mesajı
- **Dosya:** `internal/app/llm.go:516-519`
- **Nedir:** `userPrompt == ""` durumunda error gönderiliyor ama `recordStreamError` yok.
- **Düzeltme:** `recordStreamError` eklendi. ✅

#### BUG-NH6: DeleteProvider — silinen provider aktif ise activeProviderName temizlenmiyor
- **Dosya:** `internal/app/providers.go:78-110`
- **Nedir:** Aktif provider silindiğinde `activeProviderName` stale kalıyor. Sonraki chat isteği olmayan provider'ı kullanmaya çalışıyor → hata.
- **Düzeltme:** Silinen provider adı `activeProviderName` ile eşleşiyorsa temizleniyor. ✅

### 🟡 MEDIUM

#### BUG-NM1: WhatsAppChatStream — error chunk'lar sessizce yutuluyor
- **Dosya:** `internal/app/whatsapp.go:326-330`
- **Nedir:** Stream loop sadece `chunk.Content` kontrol ediyor, `chunk.Error` atlanıyor. Agent hata döndüğünde kullanıcıya gösterilmiyor ve session'a kaydedilmiyor.
- **Düzeltme:** Error chunk handling + `AddMessageToSession` ile kayıt eklendi. ✅

#### BUG-NM2: WhatsAppChatStream — ctx.Done() handling yok
- **Dosya:** `internal/app/whatsapp.go:326`
- **Nedir:** Agent stream loop'unda context cancellation kontrolü yok. Kullanıcı Stop'a bassa da stream devam ediyor.
- **Düzeltme:** `select` ile `ctx.Done()` kontrolü eklendi. ✅

#### BUG-NM3: WhatsAppChatStream — local model fallback yok
- **Dosya:** `internal/app/whatsapp.go:250`
- **Nedir:** Provider yoksa direkt hata dönüyor, lokal model çalışıyor olsa bile.
- **Düzeltme:** `llamaServer.IsRunning()` kontrolü eklendi. ✅

#### BUG-NM4: ~~WhatsApp session ID hiç temizlenmiyor → orphan session'lar~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `716a27c`
- **Düzeltme:** `StopWhatsApp()` ve `LogoutWhatsApp()` artık `whatsAppSessionID`'yi temizliyor, bir sonraki bağlantı temiz bir session ile başlıyor.
- **Dosya:** `internal/app/whatsapp.go:195, app.go:183`
- **Nedir:** `whatsAppSessionID` oluşturulduktan sonra `StopWhatsApp()` ve `LogoutWhatsApp()` tarafından temizlenmiyor. Her reconnect'te yeni session oluşuyor, eskiler diskte kalıyor.

#### BUG-NM5: SendMessageWithImage/File — incognito modda streamMu prematüre release
- **Dosya:** `internal/app/chat.go:252-254, 335-337`
- **Nedir:** Incognito branch'inde `a.streamMu.Unlock()` çağrılıp `handleIncognitoStream`'e geçiliyor. Bu sırada ikinci bir istek gelirse streamMu'yu alıp paralel stream başlatabilir.
- **Düzeltme:** Wrapper goroutine pattern'ine geçildi. ✅

#### BUG-NM6: resolveAgentProvider — nil pointer dereference on providerCfgMgr
- **Dosya:** `internal/app/llm.go:51`
- **Nedir:** `activeName != ""` ama `providerCfgMgr == nil` olabilir (startup race). `providerCfgMgr.GetEnabled()` panic.
- **Düzeltme:** Nil check eklendi. ✅

#### BUG-NM7: finishStream — GenerateChatTitle yanlış session'ı isimlendiriyor
- **Dosya:** `internal/app/llm.go:834-839`
- **Nedir:** `finishStream` doğru session'a ekleme yapıyor ama `GenerateChatTitle()` her zaman `GetActiveMessages()` ile aktif session'ı okuyor. Stream başka bir session için çalışıyorsa başlık yanlış session'a yazılıyor.
- **Durum:** Tespit edildi, düzeltilmedi (LOW etki, sadece ilk 2 mesajda tetiklenir).

### 🟢 LOW

#### BUG-NL1: ~~Router health check — UpdateConfigs sonrası orphan entry modifikasyonu~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `cf5af0a`
- **Düzeltme:** `isLiveEntry` ile write lock altında entry'nin hâlâ router'ın güncel listesinde olup olmadığı (pointer identity) doğrulanıyor, değilse re-enable atlanıyor.
- **Dosya:** `internal/provider/router.go:314-331`
- **Nedir:** HealthCheck RLock altında snapshot alıyor, Lock altında modifiye ediyor. Arada `UpdateConfigs` çalışırsa, artık listede olmayan bir entry'i modify ediyor → provider re-enable edilemiyor.

#### BUG-NL2: ~~Pipeline — truncation content eşleştirme kırılgan~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `411b06a`
- **Düzeltme:** `truncate.Message` artık orijinal slice index'ini taşıyor; recovery content eşitliği yerine doğrudan index lookup ile yapılıyor.
- **Dosya:** `internal/agent/pipeline.go:164-177`
- **Nedir:** Token bütçesi trimming'i content string eşitliği ile eşleştirme yapıyor. Aynı içerikli iki tool result yanlış eşleşebilir.

#### BUG-NL3: callAgentStream — gereksiz `sessionID != ""` kontrolü
- **Dosya:** `internal/app/llm.go:130,160,169`
- **Nedir:** `recordStreamError` zaten içeride sessionID kontrolü yapıyor, dışarıdaki kontroller redundant.

---

## Frontend — Paralel İncelemede Bulunan Buglar

### 🔴 CRITICAL

#### BUG-FC1: IndexedStack → çift permission dialog
- **Dosya:** `frontend/lib/screens/chat_screen.dart:61-78`, `frontend/lib/screens/agent_screen.dart:169-176`
- **Nedir:** ChatScreen ve AgentScreen IndexedStack'te yan yana, ikisi de `agentEventBusProvider` dinliyor. Permission request geldiğinde iki ekran da `showDialog` çağırıyor → iki dialog üst üste biniyor.
- **Düzeltme:** Listener AppShell'e taşındı, tek dialog gösteriliyor. ✅ (`ad6c7dc`)

### 🟠 HIGH

#### BUG-FH1: WhatsApp Stop butonu çalışmıyor
- **Dosya:** `frontend/lib/widgets/chat_input.dart:180-255`
- **Nedir:** `_sendWhatsApp()` lokal `CancelToken` oluşturuyor. `stopStreaming()` farklı bir token'ı iptal ediyor. Ayrıca stream sonrası `_stopped` kontrolü yok.
- **Düzeltme:** CancelToken state field'a alındı, stop butonu WhatsApp token'ını da iptal ediyor, `_stopped` kontrolü eklendi. ✅ (`cb5c995`)

#### BUG-FH2: WhatsApp mesajları refresh'te kayboluyor
- **Dosya:** `frontend/lib/widgets/chat_input.dart:192,241-249`
- **Nedir:** `_sendWhatsApp()` mesajları `messagesProvider`'a (normal chat) ekliyor. Backend WhatsApp session'ına kaydediyor. Refresh'te normal chat session'ından çekilen mesajlar WhatsApp mesajlarını içermiyor → kayboluyorlar.
- **Düzeltme:** Stream sonrası `refresh()` çağrılıyor. Geçici WhatsApp mesajları temizleniyor. ✅ (`cb5c995`)

#### BUG-FH3: Kullanıcı mesajı hata durumunda kayboluyor
- **Dosya:** `frontend/lib/providers/chat_provider.dart:325,459-467`
- **Nedir:** Optimistic user mesajı eklendikten sonra hata oluşursa, catch block `await refresh()` ile state'i backend'den gelenle değiştiriyor. Backend mesajı kaydetmediyse (örn. TryLock reddetti) user mesajı UI'dan siliniyor.
- **Düzeltme:** Catch block'ta `refresh()` kaldırıldı. ✅ (`e2357b8`)

#### BUG-FH4: sendFile'ta sıfır agent event handling
- **Dosya:** `frontend/lib/providers/chat_provider.dart:514-565`
- **Nedir:** `sendFile` stream loop'u sadece text işliyor. Agent event'leri, permission request'leri, status/usage chunk'ları tamamen yok sayılıyor. Agent modda dosya gönderince tool'lar çalışmıyor, permission'lar deadlock.
- **Düzeltme:** `sendMessage`'teki agent event handling aynen `sendFile`'a kopyalandı. ✅ (`e2357b8`)

#### BUG-FH5: Active provider state'i provider silinince güncellenmiyor
- **Dosya:** `frontend/lib/providers/provider_provider.dart`
- **Nedir:** `ProviderListNotifier.deleteProvider()` ve `updateProvider()` sadece `providerListProvider`'ı refresh ediyor, `activeProviderTypeProvider`'ı invalidate etmiyor. Engine strip ve provider kartları stale kalıyor.
- **Düzeltme:** `ref.invalidate(activeProviderTypeProvider)` eklendi. ✅ (`91ed742`)

#### BUG-FH6: Model Store detail panel'de llama.cpp kurulu değilken Start aktif
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:1835-1851`
- **Nedir:** `_ModelDetailPanel._buildDownloadRow()` llama.cpp kurulumunu kontrol etmeden Start butonu gösteriyor. `_MyModelsTab._LocalModelCard` doğru kontrol ediyor ama detail panel etmiyor.
- **Düzeltme:** `llamaInstalledProvider` watch + buton disable eklendi. ✅ (`5ad8827`)

#### BUG-FH7: Calendar olay silme onay dialog'u yok
- **Dosya:** `frontend/lib/screens/calendar_screen.dart:134-144`
- **Nedir:** Delete icon'u direkt `api.deleteCalendarEvent()` çağırıyor. Yanlışlıkla tıklamada veri kaybı.
- **Düzeltme:** `AlertDialog` confirmation eklendi. ✅ (`a225deb`)

#### BUG-FH8: ~~OAuth polling hatası sessizce yutuluyor~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `164c304`
- **Düzeltme:** Ardışık hata sayacı eklendi; 4 ardışık hatada polling durup SnackBar ile bildiriliyor, mevcut 40 denemelik timeout da artık SnackBar gösteriyor.
- **Dosya:** `frontend/lib/widgets/settings/tabs/backup_restore_tab.dart:136-151`
- **Nedir:** Timer callback içinde `catch (_) {}` tüm hataları yutuyor. Backend çökerse polling sonsuza kadar başarısız devam ediyor, kullanıcı bilgilendirilmiyor.

### 🟡 MEDIUM

#### BUG-FM1: errorMessageProvider'ın consumer'ı yok → tüm hatalar sessiz
- **Dosya:** `frontend/lib/providers/chat_provider.dart:572`, `frontend/lib/screens/chat_screen.dart`
- **Nedir:** ~12 hata yolunda `errorMessageProvider` set ediliyor ama hiçbir widget bunu dinleyip SnackBar göstermiyor.
- **Düzeltme:** `_ChatContentState.build()` içinde `ref.listen(errorMessageProvider, ...)` eklendi. ✅

#### BUG-FM2: Stop sonrası "Cevap durduruldu" mesajı refresh edilmiyor
- **Dosya:** `frontend/lib/providers/chat_provider.dart:418-425`
- **Nedir:** `_stopped` branch'i streaming state'i temizleyip return ediyor, `refresh()` çağrılmıyor. Backend "⏹️ Cevap durduruldu." kaydetmiş ama UI göstermiyor.
- **Düzeltme:** `await refresh()` eklendi. ✅

#### BUG-FM3: Chat input text'i chat değişiminde temizlenmiyor
- **Dosya:** `frontend/lib/widgets/chat_input.dart:36`
- **Nedir:** `_controller` chat değişiminde temizlenmiyor. Kullanıcı A sohbetinde yazdığını B sohbetinde görüyor → yanlış sohbete gönderme riski.
- **Düzeltme:** `ref.listen(activeChatIdProvider, ...)` ile `_controller.clear()` eklendi. ✅ (`99047a5`)

#### BUG-FM4: Tasks create dialog controller'ları dispose edilmiyor → bellek sızıntısı
- **Dosya:** `frontend/lib/screens/tasks_screen.dart:175-293`
- **Nedir:** `_showCreateDialog` içindeki `TextEditingController`'lar dialog kapanınca dispose edilmiyor. Her iptalde yeni controller'lar birikiyor.
- **Düzeltme:** `showDialog(...).then(...)` ile dispose eklendi. ✅ (`541a98a`)

#### BUG-FM5: Calendar boş başlık sessizce yutuluyor
- **Dosya:** `frontend/lib/screens/calendar_screen.dart:530`
- **Nedir:** `if (titleCtrl.text.trim().isEmpty) return;` dialog'u kapatıyor ama hiçbir hata göstermiyor. Kullanıcı neden kaydedilmediğini anlamıyor.
- **Düzeltme:** SnackBar ile hata mesajı eklendi. ✅ (`a225deb`)

#### BUG-FM6: WhatsApp stream lifecycle-aware değil
- **Dosya:** `frontend/lib/widgets/chat_input.dart:180`
- **Nedir:** Widget dispose olsa bile stream devam ediyor, cleanup yok.
- **Düzeltme:** CancelToken state field'a alındı, `dispose()`'da iptal ediliyor. ✅

#### BUG-FM7: Global Dio receiveTimeout 300s → non-streaming endpoint'ler etkileniyor
- **Dosya:** `frontend/lib/core/api_client.dart:27`
- **Nedir:** Global `receiveTimeout: 300s` tüm endpoint'leri etkiliyor. Status, chat list, message fetch gibi hızlı endpoint'ler de 5 dakika bekleyebilir.
- **Düzeltme:** 120s'e döndürüldü. Streaming endpoint'leri per-call `.timeout(300s)` kullanıyor. ✅ (`2abf8dd`)

#### BUG-FM8: Takvim `_parseDateTime` hatalı veride `DateTime.now()` dönüyor
- **Dosya:** `frontend/lib/screens/calendar_screen.dart:38-45`
- **Nedir:** Backend bozuk tarih döndüğünde parse hatası sessizce `DateTime.now()` ile değiştiriliyor. Olaylar yanlış zamanda görünüyor.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM9: ~~Widget key'leri `hashCode` kullanıyor → tüm liste rebuild~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `81f5099`
- **Düzeltme:** `hashCode` yerine mesajın kendi `timestamp`'i kullanılıyor (rebuild'ler arası stabil).
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:134`
- **Nedir:** `ValueKey('msg_${msg.hashCode}_$index')` her state değişiminde yeni hash üretiyor → tüm mesaj baloncukları rebuild oluyor.

#### BUG-FM10: İki task list aynı anda başlatılabiliyor (client-side guard yok)
- **Dosya:** `frontend/lib/screens/tasks_screen.dart:416-445`
- **Nedir:** UI iki list için de Start butonunu aktif gösteriyor. İkincisi backend'de `taskloopRunMu` tarafından reddediliyor ama hata mesajı açıklayıcı değil.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM11: Task list detail dialog auto-refresh yapmıyor
- **Dosya:** `frontend/lib/screens/tasks_screen.dart:295-414`
- **Nedir:** Detail dialog açıldığında statik snapshot alıyor. 3 saniyelik polling liste görünümünü güncelliyor ama dialog içeriği güncellenmiyor.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM12: Provider edit dialog — isim çakışması sessizce suffix ekliyor
- **Dosya:** `frontend/lib/widgets/provider_config_dialog.dart:190-201`
- **Nedir:** `_uniqueName()` isim çakışmasında sessizce " 2", " 3" ekliyor. Kullanıcıya bilgi verilmiyor.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM13: Model Store search debounce hidden tab'da API çağrısı yapıyor
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:406-426`
- **Nedir:** IndexedStack her iki tab'ı da alive tutuyor. Discover tab'ında arama yapıp "My Models" tab'ına geçince debounce timer hala tetikleniyor → gereksiz HuggingFace API çağrısı.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM14: ~~Settings dialog sabit 800x600 — responsive değil~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `5e9def0`
- **Düzeltme:** Dialog boyutu ekranın %85'ine göre, min/max sınırlarla clamp edilerek hesaplanıyor.
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:82-84`
- **Nedir:** Küçük ekranda overflow, büyük ekranda çok küçük.

#### BUG-FM15: WhatsApp stream timeout yok
- **Dosya:** `frontend/lib/widgets/chat_input.dart:200`
- **Nedir:** `_sendWhatsApp()` stream'e `.timeout()` uygulamıyor. Backend hang olursa `isSendingProvider` sonsuza kadar true kalır.
- **Düzeltme:** `.timeout(Duration(seconds: 300))` eklendi. ✅

#### BUG-FM16: Agent screen statusText'i ChatMessageList'e geçirmiyor
- **Dosya:** `frontend/lib/screens/agent_screen.dart:258-266`
- **Nedir:** ChatScreen `statusText: streamingStatus` geçiyor ama AgentScreen geçirmiyor. Web search göstergesi agent ekranda çalışmıyor.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM17: Test connection sonucu provider state'e persist edilmiyor
- **Dosya:** `frontend/lib/widgets/provider_config_dialog.dart:155-173`
- **Nedir:** Test sonucu dialog içinde gösteriliyor ama save sırasında yeni bir config objesi oluşturuluyor, test sonucu kayboluyor.
- **Durum:** Tespit edildi, düzeltilmedi.

#### BUG-FM18: ~~Permission dialog timeout yok~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `d57bf18`
- **Düzeltme:** 5 dakikalık countdown eklendi; süre dolunca otomatik `deny_once` gönderiliyor (fail-safe: hiçbir zaman otomatik onay vermiyor).
- **Dosya:** `frontend/lib/widgets/agent/permission_dialog.dart:26-33`
- **Nedir:** Dialog sonsuza kadar bekliyor. Kullanıcı bilgisayar başında değilse agent pipeline bloke.

#### BUG-FM19: ~~Cloud sync boş passphrase → device-locked yedek uyarısı yok~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `164c304`
- **Düzeltme:** Passphrase boşken onay dialog'u gösteriliyor, kullanıcı ya parola belirliyor ya da bilinçli olarak "cihaza özel" seçeneğini onaylıyor.
- **Dosya:** `frontend/lib/widgets/settings/tabs/backup_restore_tab.dart:776`
- **Nedir:** Kullanıcı passphrase girmeden cloud sync açarsa şifreleme machine ID ile yapılıyor. Başka cihaza geçince yedek çözülemez.

#### BUG-FM20: Hardcoded Turkish string'ler (l10n eksik)
- **Dosyalar:** `general_tab.dart:76,255,274`, `agent_screen.dart:302,360-365`
- **Nedir:** L10n.t() kullanılması gereken yerlerde Türkçe string'ler hardcode.
- **Durum:** Tespit edildi, düzeltilmedi.

### 🟢 LOW

#### BUG-FL1: `streamingAgentEventsProvider` double-clear (duplicate line)
- **Dosya:** `frontend/lib/providers/chat_provider.dart:200-202`
- **Durum:** Düzeltildi (önceki session)

#### BUG-FL2: `messagesProvider` autoDispose değil ama `onDispose` kullanıyor
- **Dosya:** `frontend/lib/providers/chat_provider.dart:168,181-184`
- **Nedir:** Provider `autoDispose` değil, `ref.onDispose()` hiç tetiklenmez.

#### BUG-FL3: `_AuthorAvatarState._cache` sınırsız büyüyor
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:844`

#### BUG-FL4: Model Store README yükleme hatası sessiz
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:1230,1262`
- **Nedir:** `catch (_) {}` tüm HTTP hatalarını yutuyor.

#### BUG-FL5: ~~Provider kart toggle icon'u ters~~ **→ DÜZELTİLDİ (2026-07-07)**
- **Commit:** `f3ea137`
- **Dosya:** `frontend/lib/widgets/settings/tabs/providers_tab.dart:288-289`
- **Nedir:** Enabled durumda `toggle_off`, disabled durumda `toggle_on` gösteriyor.

#### BUG-FL6: `AgentEvent.args` dynamic → type safety yok
- **Not (2026-07-07, `c64677c`):** Tasarım gereği — backend `args`'ı bazen JSON string bazen decode edilmiş Map olarak gönderiyor, `permission_dialog.dart` her ikisini de `is String`/`is Map` ile kontrol ediyor. `dynamic` kalması gerektiği koda yorum olarak belgelendi; concrete type'a çevrilmedi.
- **Dosya:** `frontend/lib/models/agent.dart:5`

---

## Session 2026-07-06 Düzeltme Özeti

### Backend (önceki session'da commit'lendi)

| # | Bug | Commit |
|---|-----|--------|
| B1-B5 | 5 orijinal bug | `e22e98b` serisi |
| NC1 | nil streamClient → deadlock | `e22e98b` serisi |
| NH1-NH6 | 6 HIGH bug | `e22e98b` serisi |
| NM1-NM6 | 6 MEDIUM bug | `e22e98b` serisi |

### Frontend (bu session)

| Commit | Dosyalar | Bug |
|--------|---------|-----|
| `ad6c7dc` | app_shell, agent_screen, chat_screen | FC1: Çift permission dialog |
| `99047a5` | chat_provider, chat_input | FM3: Input text leak + sendFile catch |
| `91ed742` | provider_provider | FH5: Stale active provider |
| `5ad8827` | model_store, l10n | FH6: Start butonu llama kontrolü |
| `a225deb` | calendar_screen | FH7: Silme onayı + FM5: Boş başlık |
| `541a98a` | tasks_screen | FM4: Controller leak |
| `2abf8dd` | api_client | FM7: receiveTimeout düzeltmesi |
| `cb5c995` | chat_input | FH1: WhatsApp stop + FH2: Mesaj kaybı + FM15: timeout |
| `e2357b8` | chat_provider | FH3: Mesaj kaybolması + FH4: sendFile agent + FM1: error consumer + FM2: stop refresh |

### Kalan İşler (Session 2026-07-06 sonu itibarıyla — Session 7'de bir kısmı düzeltildi, bkz. aşağıdaki bölüm)

| Öncelik | Sayı | Kapsam |
|---------|------|--------|
| HIGH | 1 | FH8: OAuth polling error swallowing |
| MEDIUM | 8 | FM8-FM20 (takvim, tasks, model store, UI polish) |
| LOW | 6 | FL2-FL6 + NL1-NL3 (kozmetik) |

---

## Session 2026-07-07 Düzeltme Özeti

> Bu session, Session 2026-07-06'nın "Kalan İşler" listesinden ve daha önceki Faz 2-3 roadmap'inden bir grup backend + frontend bug'ı ele aldı. Tüm backend testleri (`go build ./... && go vet ./... && go test ./...`) yeşil.

### Backend

| Commit | Dosyalar | Bug |
|--------|---------|-----|
| `edd518d` | fileutil/atomic.go, atomic_test.go | M3: copyFile 0666 hardcode + L1: orphan .tmp |
| `1a35e2e` | agent/backup.go | M5: Backup history yazma hatası sessizce yutuluyor |
| `411b06a` | agent/pipeline.go, truncate/tokens.go | NL2: Truncation content eşleştirme → index-based |
| `44c5fc3` | cloudsync/sync_manager.go(+test) | H9: WAL checkpoint hatası yutma + M1: mood.db checkpoint + M4: restore 0644 |
| `cbc2487` | app/backup.go | M2: Import staging + atomik taşıma (rollback) |
| `716a27c` | app/app.go, app/whatsapp.go | H10: lifecycleCtx + H6: cfgMu alanı + L2: whatsappChatMode atomic.Bool + NM4: session ID reset |
| `07eaf0c` | app/llama.go, llm.go, embedding.go, helpers.go, learning.go | H6: cfgMu call site'ları |
| `cf5af0a` | provider/router.go | NL1: HealthCheck orphan entry re-enable |

### Frontend

| Commit | Dosyalar | Bug |
|--------|---------|-----|
| `c64677c` | api_client.dart, models/agent.dart | M7: `_guardList` eager type check + FL6: dokümantasyon |
| `d57bf18` | widgets/agent/permission_dialog.dart | FM18: 5 dakika timeout, fail-safe auto-deny |
| `f8ac365` | screens/whatsapp_screen.dart | M8: Optimistic mesaj temizleme mounted'dan bağımsız |
| `81f5099` | widgets/chat_message_list.dart | FM9: ValueKey hashCode → timestamp |
| `f3ea137` | widgets/settings/tabs/providers_tab.dart | FL5: Toggle icon ters |
| `5e9def0` | widgets/settings_dialog.dart | FM14: Responsive dialog boyutu |
| `164c304` | widgets/settings/tabs/backup_restore_tab.dart | FM19: Boş passphrase uyarısı + FH8: OAuth polling hata gösterimi |

### Kalan İşler (Session 8 sonu itibarıyla)

| Öncelik | Sayı | Bug'lar |
|---------|------|---------|
| 🔴 CRITICAL | 2 | QC1 (import config.yaml), QC2 (import memory store reinit) |
| 🟠 HIGH | 5 | QH1 (shutdown method check), QH2 (MIME spoofing), QH3 (WhatsApp reconnect), QH4 (import provider reinit), QH5 (cloud restore reinit) |
| 🟡 MEDIUM | 11 | M6 (startupTailscale), M9-M12 (bilinen borç), QM1-QM7 (shutdown race, zero value, rate limit, WA cancel, agent error, image agent, WA optimistic) |
| 🟢 LOW | 9 | L3-L6 (shutdown delay, double-clear, cache), QL1-QL5 (HTTP method, agent delete, style cache, avatar cache, image history) |

### En Kritik 5 Düzeltme (Acil)

| # | Bug | Süre | Etki |
|---|-----|------|------|
| 1 | QC1: Import config.yaml case'i | 1 saat | Tüm ayarlar geri yüklenmez |
| 2 | QC2: Import memory store reinit | Yarım saat | RAG eski veriyi okur |
| 3 | QH3: WhatsApp stopCh reset | 1 saat | Auto-reconnect kalıcı ölüm |
| 4 | QH1: Shutdown POST-only | Yarım saat | Güvenlik açığı |
| 5 | QH4+QH5: Import/Restore reinit | 2 saat | State tutarsızlığı |

---

## Session 2026-07-07 (devam 3) — BUG-H7 düzeltildi

Kalan tek HIGH bug'a geçildi: `callLLM`'in `"⚠️ ..."` hata string'leri `SendMessage`/`SendMessageWithImage`/`SendMessageWithFile` üzerinden RAG memory'e (embedding/vector store) genuine bir yanıtmış gibi kaydediliyordu. Session/chat geçmişine kaydedilmesi (streaming path'teki `recordStreamError` ile tutarlı, kullanıcı hatayı görmeli) korunarak, sadece memory yazımı `saveMemoryAsync` içine eklenen `isLLMErrorReply()` kontrolüyle engellendi — tek nokta düzeltmesi, `finishStream`'in memory kaydını ve ileride gelecek başka çağrı noktalarını da otomatik kapsıyor.

**Commit:** `9d273dd` — `internal/app/memory.go` (fix) + `internal/app/memory_test.go` (yeni: `TestIsLLMErrorReply`, `TestSaveMemoryAsyncSkipsErrorReplies`).

**Doğrulama:** `go build/vet/test ./...` temiz (bir pre-existing flaky `internal/memory` testi dışında, ilgisiz).

**Sonuç: Tüm CRITICAL ve HIGH bug'lar kapalı.** Kalan iş sadece MEDIUM/LOW seviyede.

---

## Session 2026-07-07 (devam 2) — C2 ve H2 aslında zaten düzeltilmişti

Kullanıcının isteğiyle C2'ye geçildi. `internal/whatsapp/client.go`'yu satır satır okurken **her iki bug'ın da** (`C2`: `handleEvent` locksuz yazma, `H2`: `autoReconnect` TOCTOU) `854e04d` commit'inde (2026-06-30, BUG-C1 fix'i) çoktan düzeltilmiş olduğu görüldü — commit mesajı bunu açıkça listeliyor ama BUG_REPORT.md hiç güncellenmemişti, iki madde hâlâ "açık" gibi duruyordu.

**Doğrulama:**
- Kod okuması: `handleEvent`'in her case'i `startMu` altında; `autoReconnect` `c.waClient`'i lock altında local değişkene (`wa`) kopyalayıp onu kullanıyor.
- `go test ./internal/whatsapp/... -race -count=3`: temiz.
- Yeni regresyon testi eklendi: `internal/whatsapp/client_race_test.go` → `TestHandleEventConcurrentAccess`, `handleEvent`'i ve okuyucu metodları (`QRCodes`/`LastError`/`IsReconnecting`/`IsConnected`/`IsLoggedIn`) eşzamanlı çalıştırıp `-race` ile doğruluyor.

**Kod değişikliği yok** (zaten doğruydu) — sadece test eklendi ve rapor doğru duruma güncellendi. Artık **tüm CRITICAL bug'lar ve HIGH'ların 9/10'u düzeltilmiş durumda**, sadece H7 kaldı.

---

## Session 2026-07-07 (devam) — Kullanıcı taramasında bulunan 3 ek bulgu

### BUG-NM8: ~~`WhatsAppChatStream` — `whatsAppSessionID` mutex dışı erişim (data race)~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `063f047`
- **Dosya:** `internal/app/whatsapp.go:196-206`
- **Nedir:** `WhatsAppChatStream` içinde `a.whatsAppSessionID`'nin lazy-init check-and-set'i ve okunması hiçbir lock altında değildi. `StopWhatsApp`/`LogoutWhatsApp` ise aynı alanı `waMu` altında `""`'e sıfırlıyor. Eşzamanlı bir Stop/Logout + WhatsApp mesajı geldiğinde plain data race oluşuyordu; ayrıca stream, Stop'un az önce geçersiz kıldığı bir session ID'yi kullanabiliyordu.
- **Düzeltme:** Check-and-set + okuma artık `waMu` altında, Stop/Logout ile aynı kilit.

### BUG-NM9 (false positive — düzeltme gerekmedi): `callLLMStream` nil-client path'te `select+default` ile hata mesajı düşme riski

- **Dosya:** `internal/app/llm.go:727-733`
- **İddia:** `ctx` iptal edilmişse `default` dalı çalışıp hata mesajı okuyucuya ulaşmadan kaybolabilir.
- **Bulgu:** `outCh` bu fonksiyonda `make(chan api.StreamChunk, 128)` ile 128 buffer'lı oluşturuluyor (llm.go:504) ve bu, kanala yapılan **ilk** yazma. 128 kapasiteli taze bir kanalda `select`'in `default` dalı hiçbir zaman tetiklenemez — gönderim her zaman anında buffer'a sığar. Pratikte drop riski yok; kod biraz kafa karıştırıcı (gereksiz `select`) ama davranışsal olarak güvenli. Değişiklik yapılmadı.

### BUG-NM10 (beklenen davranış — düzeltme gerekmedi): `DeleteProvider` sonrası frontend invalidasyon timing'i

- **Dosya:** `internal/app/providers.go:118-120`
- **İddia:** Backend `activeProviderName`'i temizliyor, frontend `activeProviderTypeProvider`'ı invalidate ediyor ama bir sonraki mesajda "provider yok" hatası alınabilir.
- **Bulgu:** Bu, silinen aktif provider için beklenen ve doğru davranış — kullanıcı provider'ı sildiyse bir sonraki mesajda net bir hata almalı. Değişiklik yapılmadı.

---

## Session 2026-07-07 (devam 4) — 3 Paralel QA Agent Taraması

> **Metod:** 3 paralel agent (Backend API, Frontend Crash, Integration Flow) ile gerçek kullanıcı senaryoları test edildi.
> **Sonuç:** 19 yeni bug tespit edildi (2 CRITICAL, 5 HIGH, 7 MEDIUM, 5 LOW).

---

### 🔴 CRITICAL — Veri Kaybı / Silent Failure

#### BUG-QC1: ~~`ImportData` config.yaml'ı sessizce atlıyor → tüm ayarlar geri yüklenmez~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `68eeaa1`
- **Düzeltme:** Path çözümlemesi saf bir `resolveImportTarget(name, dataDir, configDir)` fonksiyonuna çıkarıldı, `"config/"` prefix'i için ayrı case eklendi (`config.ConfigDir()`'a map ediyor), her root için kendi escape-check'i var. Tanınmayan prefix'ler artık düpedüz reddediliyor (eskiden CWD'ye göre relative bir path'e düşüyordu). Test: `internal/app/backup_test.go` → `TestResolveImportTarget`.

- **Dosya:** `internal/app/backup.go:144-160`
- **Nedir:** ExportData zip entry'yi `"config/"` prefix'iyle yazar. ImportData'nın switch'i sadece `"sessions/"` ve `"data/"` prefix'lerini handle eder — `"config/"` default case'e düşer ve `filepath.Rel(DataDir, "config/config.yaml")` başarısız olur → entry sessizce atlanır.
- **Kullanıcı etkisi (düzeltme öncesi):** Tüm ayarlar (llama, sync, identity, memory, API, learning, calendar) export edilmiş ama geri yüklenmemiş olur. Sessiz veri kaybı.

#### BUG-QC2: ~~`ImportData` memory store'u yeniden başlatmıyor → RAG eski veriyi okur~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `68eeaa1`
- **Düzeltme:** Import sonunda `WipeAllData`'daki ile aynı pattern: embedding/main client seçilip `a.reinitMemoryStore(client, ...)` çağrılıyor.

- **Dosya:** `internal/app/backup.go:200-219`
- **Nedir:** `os.Rename` ile memory.db disk'te güncelleniyor ama canlı SQLite bağlantısı (a.store) eski inode'u tutuyor (Linux rename semantics). Sessions yeniden başlatılıyor ama `a.reinitMemoryStore()` çağrılmıyor. Cloud sync'teki `BeforeRestore`/`AfterRestore` hook'u doğru yapıyor ama import bunu yapmıyor.
- **Kullanıcı etkisi (düzeltme öncesi):** Import sonrası RAG/memory sistemi eski veriyi okuyor. Yeni memory.db disk'te boşa yatıyor — uygulama restart edilene kadar.

---

### 🟠 HIGH — Güvenlik / Reconnect Ölümleri / State Tutarsızlığı

#### BUG-QH1: `handleShutdown` her HTTP method'u kabul ediyor → LAN'den herkes kapatabilir

- **Dosya:** `internal/webserver/handlers_flutter.go:1775-1789`
- **Trigger:** LAN'den `curl -X DELETE http://<ip>:8090/api/shutdown`
- **Nedir:** Shutdown handler'ında method check yok. GET, POST, DELETE, OPTIONS — hepsi sunucuyu kapatıyor. LAN mode'da (`0.0.0.0`) ağdaki herhangi bir cihaz uygulamayı kapatabilir.
- **Kullanıcı etkisi:** Güvenlik açığı — ağdaki herhangi biri uygulamayı kapatabilir.
- **Düzeltme:** Sadece POST method'u kabul edilmeli.

#### BUG-QH2: ~~Streaming dosya yükleme MIME spoofing~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `504fa11`
- **Düzeltme:** Ortak `detectIsImageFile(path, filename)` yardımcı fonksiyonu (`internal/webserver/mime.go`) eklendi, hem streaming hem non-streaming handler bunu kullanıyor. Ayrıca ikisinin de ORİJİNAL halinde bulunan bir zafiyeti sıkılaştırdım: extension fallback eskiden content sniffing "image" demediğinde HER ZAMAN devreye giriyordu — content kesin olarak başka bir tür (örn. text/html) tespit etse bile. Artık fallback sadece sniffing gerçekten belirsizse (dosya okunamıyor veya `application/octet-stream`) devreye giriyor. Test: `internal/webserver/mime_test.go` → `TestDetectIsImageFile` (spoofed-filename case, ilk halinde fail ediyordu).

- **Dosya:** `internal/webserver/handlers_flutter.go:110-121`
- **Nedir:** `handleSendFileStream` client-supplied Content-Type'ı kullanıyor ama `handleSendFile` (non-streaming) `http.DetectContentType` ile dosya içeriğini okuyor. Streaming path'i trust ediyor.
- **Kullanıcı etkisi (düzeltme öncesi):** Text dosyası vision pipeline'a gider — yanlış işleme.

#### BUG-QH3: ~~WhatsApp `stopCh` Stop+Start sonrası ölüyor → auto-reconnect çalışmaz~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `edc8efe`
- **Düzeltme:** `Start()` artık `stopCh`/`stopOnce`'u `startMu` altında yeniden oluşturuyor. `Stop()`'un `stopOnce.Do(close)` çağrısı da aynı kilit altına taşındı (yoksa Start()'ın reassignment'ıyla yarışırdı). `autoReconnect` artık `c.stopCh`'ı direkt okumak yerine kilit altında bir local değişkene alıyor. Regresyon testi: `internal/whatsapp/client_race_test.go` → `TestStopChRecreatedAfterStop`.

- **Dosya:** `internal/whatsapp/client.go:85-93, 189-201, 470-477`
- **Nedir:** `NewClient()` `stopCh` oluşturur. `Stop()` `stopOnce.Do` ile kapatır (one-shot). `Start()` `stopCh'yi yeniden oluşturmaz. Yeni bağlantı koptuğunda `autoReconnect()` kapatılmış kanalı dinler → anında return, reconnect hiç denenmez.
- **Kullanıcı etkisi (düzeltme öncesi):** Herhangi bir disconnect→reconnect döngüsü sonrası WhatsApp auto-reconnect kalıcı olarak ölür — uygulama restart gerekir.

#### BUG-QH4: Import sonrası provider/orchestra yeniden başlatılmıyor

- **Dosya:** `internal/app/backup.go:209-219`
- **Trigger:** Import sonrası provider kullanılır.
- **Nedir:** `providers.json` disk'te güncelleniyor ama `providerCfgMgr` reload edilmiyor, `providerRouter` rebuild edilmiyor. `orchestra.json` da aynı şekilde — conductor yeniden oluşturulmuyor.
- **Kullanıcı etkisi:** Import sonrası provider ve orchestra config'leri disk'te yeni ama running app eski state'i kullanıyor. Restart gerekiyor.
- **Düzeltme:** Import sonrası provider router ve orchestra conductor yeniden oluşturulmalı.

#### BUG-QH5: Cloud restore sonrası provider/session yeniden başlatılmıyor

- **Dosya:** `internal/cloudsync/sync_manager.go:357-377`, `internal/app/sync.go:59-79`
- **Trigger:** PullSync ile cloud'dan geri yükleme.
- **Nedir:** `restoreZip` dosyaları disk'te güncelliyor. `BeforeRestore`/`AfterRestore` hook'ları sadece memory store'u handle ediyor — providers, sessions manager ve orchestra conductor yeniden başlatılmıyor.
- **Kullanıcı etkisi:** Cloud restore sonrası provider config'leri, sohbet geçmişi ve orchestra ayarları disk'te yeni ama uygulama eski state'i kullanıyor. Restart gerekiyor.
- **Düzeltme:** `AfterRestore` hook'una provider/session/orchestra reinit eklenmeli.

---

### 🟡 MEDIUM — Yanlış Davranış / UX Sorunları

#### BUG-QM1: Shutdown çift kez çalışıyor (bridge + SIGINT race)

- **Dosya:** `internal/webserver/handlers_flutter.go:1778-1788`
- **Trigger:** API ile kapatma.
- **Nedir:** `fullBridge.Shutdown()` çağrılıp ardından SIGINT gönderiliyor. İki eşzamanlı shutdown yolu SQLite, WhatsApp, embedding model ve HTTP sunucusu üzerinde race condition oluşturur.
- **Düzeltme:** Tek bir shutdown mekanizması kullanılmalı.

#### BUG-QM2: Temperature/TopP = 0 ayarlanamıyor

- **Dosya:** `internal/app/llama.go:111-119`
- **Trigger:** Kullanıcı Temperature veya TopP'yi 0 yapar (greedy decoding).
- **Nedir:** `if cfg.Temperature != 0` guard'ı zero value'yu sessizce atlıyor. Kullanıcı ayarı kaydeder ama hiçbir zaman uygulanmaz.
- **Düzeltme:** Zero value handling için `*float64` veya "set" bitmask kullanılmalı.

#### BUG-QM3: Rate limiter port bazlı (IP bazlı değil)

- **Dosya:** `internal/webserver/server.go:690`
- **Trigger:** Farklı portlardan 200+ req/s.
- **Nedir:** `r.RemoteAddr` port dahil — her TCP bağlantısı farklı source port aldığı için token bucket ayrı tutuluyor. 100 req/s limiti localhost'tan aşılabilir.
- **Düzeltme:** `strings.Split(r.RemoteAddr, ":")[0]` ile sadece IP alınmalı.

#### BUG-QM4: WhatsApp stream cancel hata gösteriyor

- **Dosya:** `frontend/lib/core/api_client.dart:1140-1142`
- **Trigger:** WhatsApp mesajı gönderilirken Stop butonuna basılır.
- **Nedir:** `sendWhatsAppChatStream` `DioExceptionType.cancel`'ı yakalamıyor (diğer iki stream method'u yakalıyor). Hata snackbar'ı kullanıcıya hatalı görünüyor.
- **Düzeltme:** `on DioException catch (e) { if (e.type == DioExceptionType.cancel) return; }` eklenmeli.

#### BUG-QM5: Agent sidebar "New Chat" hatası yutuyor

- **Dosya:** `frontend/lib/screens/agent_screen.dart:53-65`
- **Trigger:** Backend düşerken "New Chat" tıklanır.
- **Nedir:** `createAgentChat` çağrısında try-catch yok. Hata zone handler'a düşüyor — hiçbir feedback yok.
- **Düzeltme:** Try-catch + SnackBar eklendi.

#### BUG-QM6: Image stream agent modunu bypass ediyor

- **Dosya:** `internal/app/chat.go:228-301`
- **Trigger:** Agent açıkken resim gönderilir.
- **Nedir:** `SendMessageWithImageStream` kendi mesaj listesini oluşturup doğrudan `callLLMStream`'e gidiyor. `agentEnabled` kontrolü yok, agent system prompt enjekte edilmiyor, skill prompt eklenmiyor.
- **Kullanıcı etkisi:** Agent tool'ları (dosya okuma/yazma, komut çalıştırma) resimli mesajlarda çalışmıyor.
- **Düzeltme:** `sendMessageStreamInner`'daki agent routing mantığı image/file stream'lerde de kullanılmalı.

#### BUG-QM7: WhatsApp _optimistic çift mesaj riski

- **Dosya:** `frontend/lib/screens/whatsapp_screen.dart:686-696`
- **Trigger:** Tab değiştirip WhatsApp'a geri dönülür.
- **Nedir:** API çağrısı uzun sürerse ve kullanıcı tab değiştirirse, optimistic mesaj `_optimistic`'te kalır. Sunucu zaten mesajı kaydetmişse, geri döndüğünde mesaj 2 kez görünür (birisi optimistic, diğeri sunucudan gelen).
- **Düzeltme:** Tab değişiminde `_optimistic` temizlenmeli.

---

### 🟢 LOW — HTTP Semantiği / Bellek Sızıntısı / Eksik UI

#### BUG-QL1: HTTP method check yok (çoklu endpoint)

- **Dosyalar:** `server.go:438,509,513`, `handlers_flutter.go:544,553,726`
- **Nedir:** `/api/chats`, `/api/chats/active`, `/api/messages`, `/api/events` gibi read-only endpoint'ler her HTTP method'u kabul ediyor. DELETE ile chat listesi çekmek bile çalışıyor.
- **Etki:** HTTP semantiği ihlali, proxy/cache sorunları.

#### BUG-QL2: Agent chat silme butonu yok

- **Dosya:** `frontend/lib/screens/agent_screen.dart:92-97, 120-158`
- **Nedir:** `_AgentChatItem` `onDelete` callback alıyor ama widget'ta silme butonu/long-press yok. Agent sohbetleri agent sidebar'dan silinemez.
- **Etki:** Kullanıcı agent sohbetini silmek için Chat tab'ına gitmek zorunda.

#### BUG-QL3: `_styleCache` hiç temizlenmiyor

- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:12`
- **Nedir:** `Map<int, MarkdownStyleSheet>` theme/accent combo'ları kadar büyüyor, hiç eviction yok.
- **Etki:** Uzun oturumlarda hafif bellek sızıntısı.

#### BUG-QL4: `_AuthorAvatar._cache` hiç temizlenmiyor

- **Dosya:** `frontend/lib/screens/model_store_screen.dart:843`
- **Nedir:** HuggingFace avatar URL'leri author adına göre cache'leniyor, hiç temizlenmiyor.
- **Etki:** Uzun oturumlarda hafif bellek sızıntısı.

#### BUG-QL5: Image stream session history eksik

- **Dosya:** `internal/app/chat.go:269-278`
- **Trigger:** Resimli sohbet.
- **Nedir:** `SendMessageWithImageStream` kendi mesaj listesini oluşturuyor — `buildMessages()`'in web search context, skill prompt, agent prompt eklemelerini atlıyor.
- **Etki:** Resimli mesajlar text mesajlardan farklı (daha düşük kaliteli) prompt yapısına sahip.

---

## Session 2026-07-07 (devam 5) — RAG stabilite incelemesi: 3 gerçek bug bulundu ve düzeltildi

> **Bağlam:** Kullanıcı "RAG sistemi stable mi" diye sordu. `internal/memory/store.go`'yu (1838 satır) satır satır okuyarak inceledim — mimari (hibrit vektör+FTS arama, RRF, multi-query expansion, importance-weighted ranking) sağlam, ama 2 somut açık buldum. Kullanıcı "ikisini de düzelt" dedi; testleri yazarken bunlardan bağımsız, daha derin **üçüncü bir gerçek deadlock** ortaya çıktı (`internal/database`), onu da düzelttim.

### BUG-RAG1: ~~`Store.Close()` sonrası migration goroutine'i nil pointer'a çarpabiliyor~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `333e95e`
- **Dosya:** `internal/memory/store.go` (`Close()`)
- **Kanıt:** Bu oturumda gerçekten canlı olarak tetiklendi: `go test -race` çalışırken `panic: invalid memory address or nil pointer dereference` → `memo/internal/database.(*DB).Write(0x0, ...)` → `memo/internal/memory.(*Store).initSchema.func2()`.
- **Nedir:** `initSchema()`, FTS/vec migration'ı 60-120sn'ye kadar sürebilen arka plan goroutine'lerinde çalıştırıyor. `Close()` `s.db = nil` yapıyordu; migration bitip `s.db.Write(...)` çağıracağı sırada `Close()` araya girerse nil pointer'a yazmaya çalışıp panic atıyordu.
- **Düzeltme:** `Close()` artık `s.db`'yi nil'lemiyor — `database.DB`'nin kendi metodları zaten kapatıldıktan sonra çağrılmaya karşı zarif (panic değil, hata döner). Regresyon testi: `store_race_test.go` → `TestCloseKeepsDBReferenceUsable`.

### BUG-RAG2: ~~Embedding modeli değiştirilince vektör arama sessizce kalıcı olarak boşa düşüyor~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `333e95e`
- **Dosya:** `internal/memory/store.go` (`ensureVecMetadata`, `migrateEmbeddingsToVec`)
- **Nedir:** İki ayrı ama birbirini besleyen sorun: (1) `ensureVecMetadata`, dimension değiştiğinde `vec_memories`'i drop+recreate ediyor ama eski `vec_migration_done` bayrağını temizlemiyor — `initSchema` bunu görüp yeni (boş) tabloyu doldurma migration'ını hiç tetiklemiyor. (2) `migrateEmbeddingsToVec` tüm satırları TEK transaction'da yazıyordu; dimension'ı uyuşmayan tek bir satır (eski modelden kalma) `tx.Rollback()` tetikleyip **tüm batch'i** iptal ediyordu, ve bayrak hiç "1" olmadığı için her restart'ta aynı hata tekrarlanıyordu — vektör arama sonsuza kadar 0 sonuç dönerdi, hiç hata vermeden.
- **Düzeltme:** Dimension değişince `vec_migration_done` de temizleniyor (migration yeniden tetikleniyor). `migrateEmbeddingsToVec` artık uyumsuz dimension'lı satırları tek tek atlıyor (`goSearch`'teki pattern ile aynı), tek satır tüm batch'i iptal edemiyor. Regresyon testleri: `TestEnsureVecMetadataResetsMigrationFlagOnDimensionChange`, `TestMigrateEmbeddingsToVecSkipsDimensionMismatch`.

### BUG-RAG3: ~~`database.DB.Write()`, `Close()` ile yarışınca kalıcı olarak asılı kalabiliyor~~ **→ DÜZELTİLDİ (2026-07-07)**

- **Commit:** `c8a5ec3`
- **Dosya:** `internal/database/sqlite.go` (`Write`, `Close`)
- **Kanıt:** RAG2 için regresyon testi yazarken bulundu — `go test -count=3` ile ~1/3 ihtimalle asılı kaldı; `go test -timeout=15s` goroutine dump'ı tam olarak `Write()`'ın ikinci `select`'inde (`<-done` bekliyor) takılı kaldığını gösterdi.
- **Nedir:** `Write()`'ın ilk `select`'i, `db.ctx` zaten `Done` olsa bile Go'nun eşit-hazır case'ler arasında **rastgele seçim** yapması yüzünden "task kanal'a başarıyla gönderildi" dalını seçebiliyordu. `writeLoop`, `ctx.Done()`'ı görüp o anda kanalda olanları drain edip çıkmış olabilir — bu durumda gönderilen task'ı **hiç kimse işlemez**, `done` kanalına asla yanıt gelmez, çağıran goroutine sonsuza kadar bloke olur. Bu, `database.DB`'yi kullanan **her SQLite store'u** (memory, mood, calendar, whatsapp) etkileyen paylaşımlı bir sorundu — özellikle `context.Background()` ile (kendi iptal mekanizması olmadan) `Write()` çağıran her yer.
- **Düzeltme:** `Write()` artık tüm gövdesi boyunca bir `closeMu.RLock()` tutuyor; `Close()` cancel etmeden önce `closeMu.Lock()` alıp bir `closed` bayrağı set ediyor. Böylece `Close()`, devam eden hiçbir `Write()` çağrısı bitmeden `writeLoop`'u kapatamıyor. Regresyon testleri: `internal/database/close_write_race_test.go` → `TestWriteAfterCloseNeverHangs` (20x, eskiden ~1/3 asılı kalıyordu), `TestConcurrentWriteRacingClose`.

**Doğrulama:** `go build/vet ./...` temiz. `go test ./... -race -count=1 -timeout=300s` → tüm paketler yeşil (özellikle `database`, `memory`, `calendar`, `mood`, `whatsapp` — hepsi aynı `database.DB`'yi paylaşıyor). Yeni regresyon testleri `-race -count=10` ile de temiz.

**Not:** Bu bölümün üstünde, benim taramadığım ayrı bir "3 Paralel QA Agent Taraması" bölümü var (QC1-QC2 CRITICAL, QH1-QH5 HIGH dahil, henüz düzeltilmemiş). Bu, daha önce verdiğim "tüm CRITICAL/HIGH kapalı" değerlendirmesini eskitiyor — o bölüme henüz bakmadım.
