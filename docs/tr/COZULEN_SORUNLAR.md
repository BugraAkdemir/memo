# Çözülen Sorunlar

Bu belge, Memo projesinde çözülmüş olan 61 hatayı listeler.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🟡 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- 🔵 **Düşük** — kozmetik, küçük iyileştirme veya uç durum

---

## 🔴 Kritik

### K1. Yetim SSE Bağlantıları — İstemci Koptuğunda LLM Çalışmaya Devam Ediyor
- **Dosya:** `internal/webserver/handlers_flutter.go:39-61`, `internal/api/streaming.go`, `app.go`, `internal/webserver/bridge.go`
- **Çözüm:** `AppBridge`/`FullBridge` stream metodlarına `ctx context.Context` parametresi eklendi. `handleSendStream`'den `r.Context()` tüm LLM çağrı zincirine (→ `SendMessageStream` → `callLLMStream` → `ChatCompletionStream`) iletiliyor. İstemci kopunca context cancel oluyor, LLM HTTP isteği iptal ediliyor, goroutine doğal olarak sonlanıyor.

### K2. Motor Modu / Yapılandırma Güncellemesi Tüm Llama Ayarlarını Sıfırlıyor
- **Dosya:** `app.go:1148-1151`
- **Çözüm:** `UpdateLlamaConfig` artık tüm struct'ı replace etmek yerine her alanı ayrı ayrı kontrol ediyor: sadece non-zero (boş olmayan) değerler mevcut config'e yazılıyor. Kısmi JSON body (`{"engine_mode": "cpu"}`) sadece `EngineMode` alanını güncelliyor, diğer ayarlar (port, ctx_size, binary_path vb.) korunuyor.

### K3. `/api/image` Üzerinden Keyfi Dosya Okuma
- **Dosya:** `app.go:865-872` (GetImageBase64), `internal/webserver/handlers_flutter.go:214-226` (handleImage)
- **Çözüm:** Çift katmanlı path doğrulama. Katman 1 (handler): `..` traversal ve absolute path engellenir. Katman 2 (`GetImageBase64`): `filepath.Abs` + `EvalSymlinks` çözümlemesi + `data/` dizini whitelist prefix kontrolü. Symlink saldırıları da engellenir.

### K4. Uzaktan Erişim Sunucusu — Kimlik Doğrulama Yok, Açık CORS
- **Dosya:** `internal/webserver/server.go:65-176`, `app.go:155-157`, `app.go:1048-1063`
- **Çözüm:** Uzaktan erişim v3.0.0'da tamamen devre dışı bırakıldı. `Start()` anında hata döndürür. `SetRemoteAccess` etkinleştirmeyi reddeder. CORS wildcard'ı `Origin` yankısı ile değiştirildi. Gelecek bir sürümde yeniden eklenecek.

### K5. `a.client` Değişkeninin Kilitsiz Yeniden Atanması
- **Dosya:** `app.go`
- **Çözüm:** `App` struct'ına `clientMu sync.RWMutex` eklendi. Tüm yazmalar `Lock/Unlock`, tüm okumalar `RLock/RUnlock` ile korunuyor. `a.client` ve `a.embeddingClient` tamamen güvence altına alındı.

### K6. `saveMemoryAsync` RLock→Lock Modeli (Kilitlenme Riski)
- **Dosya:** `app.go`
- **Çözüm:** Lock+goroutine modeli channel-based worker ile değiştirildi. `saveMemoryAsync` kanala yazar, anında döner. `memorySaveWorker` sırayla işleri `storeMu.Lock()` alarak yapar. RLock→Lock geçişi tamamen kalktı.

### K7. UI İş Parçacığı Performansı — Mesaj Başına AnimationController
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart`
- **Çözüm:** `_MessageBubble`'daki tüm entry animasyonları kaldırıldı. `SingleTickerProviderStateMixin`, `AnimationController`, `FadeTransition`, `SlideTransition` tamamen temizlendi.

---

### K8. Sohbet Değiştirince Stream İptal (Stream Cancel on Chat Switch)
- **Dosya:** `frontend/lib/providers/chat_provider.dart`
- **Çözüm:** `switchTo()` içinde `messagesProvider.notifier.stopStreaming()` çağrısı eklendi. Kullanıcı sohbet değiştirince önceki HTTP isteği iptal edilir.

### K9. Incognito Yarış Koşulu (Incognito Toggle Race)
- **Dosya:** `app.go`
- **Çözüm:** `incognitoMu sync.RWMutex` eklendi. Tüm okuma noktaları `RLock`/`RUnlock`, tüm yazma noktaları `Lock`/`Unlock` ile korundu. Hızlı art arda incognito toggle + mesaj gönderme işlemlerinde data race riski sıfır.

### K10. `processSSEStream` Watcher Goroutine Sızıntısı
- **Dosya:** `internal/api/streaming.go:49-53`
- **Çözüm:** `context.WithCancel` child context + `defer cancel()` eklendi. Fonksiyon dönünce `cancel()` çağrılır, watcher goroutine `watchCtx.Done()` alır ve çıkar. Uzun süreli kullanımda goroutine havuzu tükenmez.

### K11. `callLLMStream` Dolu Kanala Bloke Olma
- **Dosya:** `app.go`
- **Çözüm:** `trySend()` yardımcı fonksiyonu eklendi: `select { case outCh <- chunk: case <-ctx.Done(): }` ile context iptalinde bloke olmadan döner. İstemci kopunca goroutine saniyeler içinde temizlenir.

### K12. `memorySaveWorker` Shutdown'da Sızıntı
- **Dosya:** `app.go:309`
- **Çözüm:** `shutdown()` fonksiyonuna `close(a.memorySaveCh)` eklendi. Kanal kapanınca goroutine döngüden çıkar. Her uygulama kapanışında goroutine sızıntısı önlendi.

### K13. Eşzamanlı `writeIndexFile` İndeks Bozulması
- **Dosya:** `internal/memory/store.go:357-358`
- **Çözüm:** `go s.writeIndexFile(cp)` → `s.writeIndexFile(s.index)` (senkron). Zaten write lock altında çağrıldığı için async olmasına gerek yok. Hafıza indeksi her zaman tutarlı.

---

## 🟠 Yüksek

### Y1. SSE Bağlantı Kesintisinde Goroutine Sızıntısı
- **Dosya:** `internal/api/streaming.go`, `internal/api/client.go`
- **Çözüm:** `processSSEStream` artık `ctx context.Context` parametresi alıyor. Context iptal olduğunda body'i kapatan bir watcher goroutine ile scanner uyandırılıyor, goroutine doğal olarak sonlanıyor. `ChatCompletionStream` context'i `processSSEStream`'e iletiyor.

### Y2. Yapılandırma Dosyası Dünya-Tarafından Okunabilir (`0644`) — Sırlar İçeriyor
- **Dosya:** `internal/config/config.go:178`
- **Çözüm:** `os.WriteFile` çağrısında `0644` → `0600` olarak değiştirildi. Artık config dosyası sadece kullanıcının kendisi tarafından okunabilir.

### Y3. Senkronizasyon Şifrelemesi İçin Zayıf Anahtar Türetme
- **Dosya:** `internal/cloudsync/crypto.go:18-23`, `sync_manager.go`
- **Çözüm:** `encrypt`/`decrypt` fonksiyonları artık PBKDF2 (600.000 iterasyon) + rastgele 16-byte tuz kullanıyor. Tuz, şifrelenmiş verinin başına ekleniyor. Eski SHA-256 formatı, geriye dönük uyumluluk için `decrypt`'te fallback olarak korundu.

### Y4. Sabit Kodlanmış Geri Dönüş Şifreleme Anahtarı
- **Dosya:** `internal/cloudsync/sync_manager.go`
- **Çözüm:** `New()`'de parola boşsa, `persistDir/.machine-id` dosyasına yazılan UUID kullanılır. Bu şekilde parolası olmayan her makine benzersiz ve kalıcı bir anahtara sahip olur.

### Y5. `buildMessages` Oturum Geçmişini Kalıcı Olarak Değiştiriyor
- **Dosya:** `app.go:1358`
- **Çözüm:** `buildMessages` içinde `history`'nin savunma amaçlı kopyası eklendi: `history = append([]api.Message{}, history...)`. Artık `getSessionHistory()` dönüşü üzerinde yapılan mutasyonlar session verisini asla etkilemez. Sistem prompt'u her istekte tek sefer enjekte edilir, ikiye katlanmaz.

### Y6. `hash2hex` — SHA-256'nın Sadece 4 Baytı (Çakışma Riski)
- **Dosya:** `internal/memory/store.go:342-344`
- **Çözüm:** `hash[:4]` → `hash[:8]` olarak değiştirildi. Artık 8 bayt (16 hex karakter) kullanılıyor. Çakışma olasılığı %50'den ihmal edilebilir seviyeye düştü.

### Y7. `monitor()` Goroutine'inin `s.cmd`'ye Kilit Dışında Erişmesi
- **Dosya:** `internal/llama/llama.go:271-302`
- **Çözüm:** Nil kontrolü artık lock içinde yapılıp local kopyaya alınıyor (`cmd := s.cmd`). `Wait()` local kopya üzerinden çağrılıyor. `Stop()` `s.cmd = nil` yapsa bile `monitor()` kendi kopyası üzerinde çalıştığı için nil-pointer paniği mümkün değil.

### Y8. İndirme Hatalarında Geçici Dosya Temizlenmiyor
- **Dosya:** `internal/modelstore/modelstore.go:237-243`
- **Çözüm:** `os.Remove(tmpPath)` koşullu (`ctx.Err() != nil`) olmaktan çıkarılıp koşulsuz `defer` içine alındı. Artık ister context iptali, ister HTTP hatası, ister ağ zaman aşımı olsun — her durumda `.downloading` geçici dosyası temizlenir.

### Y9. `extractTarGzToBin`'de Dosya Tanıtıcı Sızıntısı
- **Dosya:** `internal/llama/installer.go:433,437`
- **Çözüm:** Tar çıkarma döngüsü yeniden yapılandırıldı. Her dosya için ayrı `extractFile()` fonksiyonu oluşturuldu, `defer out.Close()` ile dosyanın her koşulda kapanması garanti altına alındı. Manuel `out.Close()` çağrıları kaldırıldı.

### Y10. `nvidia-smi` Hataları Sessizce Geçiliyor → 0 VRAM → 0 GPU Katmanı
- **Dosya:** `internal/llama/gpu.go:62-101`
- **Çözüm:** Her `nvidia-smi` çağrısının hatası artık `log.Printf` ile loglanıyor: binary bulunamazsa, name sorgusu başarısız olursa, VRAM sorgusu başarısız olursa ve VRAM değeri parse edilemezse. Kullanıcı artık neden GPU kullanılamadığını görebilir.

### Y11. OAuth `authDone` Kanalı Yarışı
- **Dosya:** `internal/cloudsync/drive.go`
- **Çözüm:** `chan struct{}` + `authDoneClosed bool` yerine `sync.WaitGroup` kullanıldı. `StartAuthFlow` → `Add(1)`, `closeAuthDoneLocked` → `Done()`, `WaitForAuth` → goroutine ile `Wait()` yapar. Kanal takası olmadığı için yarış imkansız.

### Y12. `Shutdown(context.Background())` Süresiz Bloke Olabilir
- **Dosya:** `internal/webserver/server.go:286`
- **Çözüm:** `context.Background()` → `context.WithTimeout(10*time.Second)` olarak değiştirildi. Artık sunucu kapanırken bir handler takılı kalırsa 10 saniye sonra zaman aşımına uğrar ve kapanma tamamlanır.

### Y13. Oturum Kimliği 8 Hex Karaktere Kırpılmış
- **Dosya:** `internal/sessions/sessions.go:68`
- **Çözüm:** `uuid.New().String()[:8]` → `uuid.New().String()` olarak değiştirildi. Artık tam UUID (36 karakter) kullanılıyor. Çakışma olasılığı astronomik seviyede düşük.

### Y14. İndirme Yoklama Akışı Sonsuza Kadar Çalışıyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:66-79`
- **Çözüm:** `if (!progress.active) break;` eklendi — indirme tamamlandığında/boştayken döngü sonlanır. Hata durumunda da `break;`. Provider dispose edildiğinde Dart'ın async* mekanizması otomatik olarak iptal eder.

### Y15. Backend Hata Yönetimi: Bağlantı Hatasında "Kurulu" Gösteriliyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:97-104`
- **Çözüm:** Bağlantı hatasında `true` ("kurulu") döndüren `connectionError` kontrolü kaldırıldı. Artık her hata durumunda `false` döndürülür — backend ulaşılamaz olduğunda kullanıcı kurulum ekranını görür ve işlemi tetikleyebilir.

---

## 🟡 Orta

### O1. Arka Plan Hataları Arayüze Asla Ulaşmıyor (Bozuk Olay Sistemi)
- **Dosya:** `app.go`, `internal/webserver`
- **Çözüm:** `eventRing` (64 olaylık ring buffer) eklendi. `emitEvent` artık olayları ring buffer'a yazar ve log'lar. `GET /api/events` endpoint'i ile frontend olayları okuyabilir.

### O2. Oturum Dosyaları Dünya-Tarafından Okunabilir (`0644`)
- **Dosya:** `internal/sessions/sessions.go:236`
- **Çözüm:** `0644` → `0600` olarak değiştirildi.

### O3. `save()` Hataları Oturum Yöneticisinde Sessizce Atılıyor
- **Dosya:** `internal/sessions/sessions.go:75,155`
- **Çözüm:** `save()` hataları `log.Printf` ile loglanıyor.

### O4. `loadAll()` Bozuk Oturum Dosyalarını Sessizce Atlıyor
- **Dosya:** `internal/sessions/sessions.go:252-258`
- **Çözüm:** Okuma ve decode hataları `log.Printf` ile loglanıyor.

### O5. SSE `[DONE]` Parçasında `FinishReason` Eksik
- **Dosya:** `internal/api/streaming.go:73`
- **Çözüm:** `[DONE]` parçasına `FinishReason: "stop"` eklendi.

### O6. Ana Yolda Senkron Bloke Eden Yazmalar
- **Dosya:** `internal/sessions/sessions.go:155`, `internal/memory/store.go:105`
- **Çözüm:** Session yazmaları async yapıldı — `save()` artık `os.WriteFile`'ı bir goroutine'de çalıştırır. Memory yazmaları zaten `memorySaveWorker` (channel + goroutine) üzerinden async idi.

### O7. `LoadCache` Performansı — O(N) Başlangıç Süresi
- **Dosya:** `internal/memory/store.go`
- **Çözüm:** Tek dosyalı indeks (`memory_index.gob`) eklendi. Başlangıçta N tane `.gob` dosyası okumak yerine tek bir indeks dosyası okunur. İndeks her ekleme/silme sonrası async goroutine ile güncellenir. Eski format geriye dönük uyumlu (yoksa tek tek tara).

### O8. Kaba Kuvvet O(N) Vektör Arama
- **Dosya:** `internal/memory/retriever.go`, `store.go`
- **Çözüm:** `MemoryIndex.Norm` (önceden hesaplanmış L2 norm) eklendi — `cosineSimilarityFast` her iterasyonda norm hesaplamaz, ~2x hızlı. Paralel arama (worker goroutine) zaten mevcuttu.

### O9. `killByPort` `lsof` / `fuser`'a Bağımlı
- **Dosya:** `internal/llama/llama.go`, `process_unix.go`, `process_windows.go`
- **Çözüm:** `Server.portPid` alanı eklendi — başlatılan sürecin PID'i saklanır, `Stop()`'da önce doğrudan `killPID()` ile öldürülür, lsof/fuser sadece son çare. `lsof`/`fuser`/`netstat` hataları artık `log.Printf` ile loglanıyor.

### O10. Sabit Kodlanmış Windows Ses Aygıtı GUID'i
- **Dosya:** `app.go:737-775`
- **Çözüm:** Hardcoded GUID kaldırıldı. `getDefaultDshowDevice()` ffmpeg ile DirectShow aygıtlarını numaralandırır, ilk bulunan mikrofonu kullanır. Hiçbiri bulunamazsa `"default"` fallback'i.

### O11. Linux GPU Algılaması Sysfs Üzerinden Kırılgan
- **Dosya:** `internal/llama/gpu.go`
- **Çözüm:** `detectAMDLspci()` eklendi — önce `lspci` dene (konteyner dostu), bulunamazsa sysfs fallback. `bash` bağımlılığı kalktı, `filepath.Glob` + `os.ReadFile` kullanılıyor.

### O12. Geçmiş Okurken Otomatik Kaydırma Dibe Çekiyor
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:91-103`
- **Çözüm:** `_isNearBottom()` kontrolü eklendi — kullanıcı alttan 50px'den fazla uzaktaysa otomatik kaydırma atlanır.

### O13. Dışa Aktarma Hataları Sessizce Yutuluyor
- **Dosya:** `frontend/lib/screens/chat_screen.dart:203`
- **Çözüm:** Boş `catch (_) {}` → hata mesajı gösteren SnackBar eklendi.

---

## 🔵 Düşük

### D1. Yapılandırma Yükleme Hatası Sessizce Varsayılana Düşüyor
- **Dosya:** `app.go:168-171`
- **Çözüm:** Hata durumunda `a.emitEvent("config_load_error", err.Error())` ile frontend'e bildirim gönderiliyor. Varsayılan config ile devam ediliyor.

### D2. Hafıza Deposu / Oturum Yöneticisi Başlatma Hataları Sessizce Devre Dışı Bırakılıyor
- **Dosya:** `app.go:183-193`
- **Çözüm:** Hata durumunda `a.emitEvent("memory_store_error", ...)` ve `a.emitEvent("sessions_manager_error", ...)` ile frontend'e bildirim gönderiliyor.

### D3. `os.Executable()` Hatası Yok Sayıldığında Boş Yol
- **Dosya:** `app.go:916`
- **Çözüm:** `exePath, _` → `exePath, err`, hata durumunda `os.Args[0]`'a düşülüyor.

### D4. Boş Token Yolu Tüm Drive İşlemlerini Bozuyor
- **Dosya:** `internal/cloudsync/drive.go:44-46`
- **Çözüm:** `newDriveClient` artık `(*driveClient, error)` döndürüyor. `tokenPath` boşsa `"cloudsync: token path is empty"` hatası döndürülüyor. Çağrıcı (`sync_manager.go:81`) hatayı `log.Printf` ile logluyor.

### D5. Nil `embeddingFunc` ile `NewStore` Sessiz Çökme Yolu Oluşturuyor
- **Dosya:** `internal/memory/store.go:43-45`
- **Çözüm:** `embeddingFunc` nil ise `NewStore` artık `"memory.NewStore: embeddingFunc is nil"` hatası döndürüyor.

### D6. Hafıza İndeksi Tüm Embedding'leri `append` ile Kopyalıyor (2x RAM)
- **Dosya:** `internal/memory/store.go:104-108`
- **Çözüm:** `LoadCache` legacy path'inde `doc.Embedding`'e doğrudan referans veriliyor (doc scope dışına çıktığı için kopya gereksizdi). `SaveInteraction` path'indeki kopya korundu (chromem koleksiyonu referans tutabilir).

### D7. `DiscordWebhook` / Action URL Yazmaları Asla Kontrol Edilmiyor
- **Çözüm:** Discord/webhook/ActionURL özelliği kod tabanından kaldırıldığı için sorun kendiliğinden çözüldü.

### D8. OAuth Loopback Dinleyicisinde Tip Dönüşüm Panik
- **Dosya:** `internal/cloudsync/drive.go:103-107`
- **Çözüm:** `tcpAddr, ok := ln.Addr().(*net.TCPAddr)` güvenli dönüşümü, TCP değilse hata döndürülüyor.

### D9. `WakeOnLan` / `Precise` Dosya İşlemleri
- **Çözüm:** WakeOnLan/Precise özelliği kod tabanından kaldırıldığı için sorun kendiliğinden çözüldü.

### D10. Model İçe Aktarmada Boyut Sınırı Yok
- **Dosya:** `internal/modelstore/modelstore.go:396,406-409`
- **Çözüm:** `maxImportSize = 50 GiB` sabiti eklendi. `ImportLocalModel` kopyalamadan önce `info.Size() > maxImportSize` kontrolü yapıyor, limit aşımında hata döndürüyor.

### D11. `DeleteLocalModel`'de Sembolik Bağ Saldırısı
- **Dosya:** `internal/modelstore/modelstore.go:367-384`
- **Çözüm:** `filepath.EvalSymlinks` hem `absPath` hem `absModelsDir` üzerinde çağrılıyor. Karşılaştırma symlink-resolved yollar üzerinde yapılıyor.

### D12. `safePersistPath`'de TOCTOU Yarışı
- **Dosya:** `internal/memory/store.go:315-339`
- **Çözüm:** Dönen yolda `filepath.EvalSymlinks` çağrılıyor, symlink çözümlenmiş yol tekrar prefix kontrolünden geçiriliyor. TOCTOU penceresi daraltıldı.

### D13. `runCmdStream` Goroutine'leri Fonksiyondan Uzun Yaşayabilir
- **Dosya:** `internal/llama/installer.go:635-654`
- **Çözüm:** `sync.WaitGroup` ile stdout/stderr goroutine'leri `cmd.Wait()` sonrasında bekleniyor. Fonksiyon dönmeden önce tüm goroutine'lerin tamamlanması garanti altında.

### D14. Sohbet Girişi `/` Komutunun Görsel Göstergesi Yok
- **Dosya:** `frontend/lib/widgets/chat_input.dart:203`
- **Çözüm:** `hintText`'e `' (/)'` eklendi — kullanıcı `/` ile şablon açılabileceğini görür.

### D15. Her Build'de `FocusNode` Oluşturuluyor
- **Dosya:** `frontend/lib/widgets/chat_input.dart:19-21,39-42`
- **Çözüm:** `KeyboardListener` artık state'de saklanan `_kbFocusNode` kullanıyor, `dispose`'da temizleniyor.

### D16. Ayarlarda Eski Prompt Metni (Veri Değiştiğinde Güncellenmiyor)
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:321-323`
- **Çözüm:** `_initialized` flag'i kaldırıldı. `_controller.text != prompt` kontrolü ile prompt değiştiğinde controller otomatik güncelleniyor.

### D17. Hata Durumu Yalnızca Simge Gösteriyor — Hata Mesajı Yok
- **Dosya:** `frontend/lib/screens/chat_screen.dart:67-70`
- **Çözüm:** Sabit `'connection_error'` metni yerine `'$e'` hatanın kendisi gösteriliyor, `textAlign: TextAlign.center` ile.

### D18. Model Durdurma Düğmeleri Beklemeden Ateşleniyor
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:732-751, 804-827`
- **Çözüm:** `onPressed` artık `async` — `await ref.read(apiClientProvider).stopModel()` / `stopEmbeddingModel()`. Hata durumunda SnackBar gösteriliyor.

### D19. Bulut Senkronizasyonu ve Uzaktan Erişim Sekmeleri "Yapım Aşamasında"
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:788-958, 959-990`
- **Çözüm:** Cloud Sync sekmesi: Google Drive bağlantı durumu, OAuth bağlanma, şimdi senkronize et, bağlantıyı kes butonları eklendi. Remote Access: "v3.0.0'da devre dışı" bilgisi gösteriliyor.

### D20. Kurulum Sihirbazı Enterpolasyon Yerine Sabit `$name` Kullanıyor
- **Dosya:** `frontend/lib/widgets/setup_wizard_view.dart:87`
- **Çözüm:** `\$name` → `$name` (Dart string interpolation). Kullanıcının adı artık prompt'ta doğru şekilde yer alıyor.

---

> **Son güncelleme:** 2026-06-03  
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, tüm internal/ paketleri) ve Flutter frontend  
> **Toplam çözüm:** 61 (13 kritik, 15 yüksek, 13 orta, 20 düşük)
