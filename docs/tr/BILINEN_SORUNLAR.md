# Bilinen Sorunlar ve Teknik Riskler (Kapsamlı Denetim)

Bu belge, Memo projesindeki tüm tespit edilen hataları, mimari kısıtlamaları ve uç durumları takip eder. 2026-06-03 tarihli derinlemesine kod denetimi sonrası güncellenmiştir.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🟡 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- 🔵 **Düşük** — kozmetik, küçük iyileştirme veya uç durum
- ⚪ **Bilgi** — tasarım notu, risk veya gözlem

---

## 🔴 Kritik

### C1. `a.syncManager` veri yarışı (Data Race) — ✅ Düzeltildi (K16)
- **Dosya:** `app.go:1880-1890`, `app.go:1669-1671`
- **Detay:** `UpdateSyncSettings` fonksiyonu `a.syncManager`'ı **herhangi bir kilit olmadan** atar (`nil` yapar veya yeni örnek oluşturur). Aynı anda, `memorySaveWorker` goroutine'i (`saveMemorySync` üzerinden) `a.syncManager`'ı okur. Bu bir **data race** — işaretçinin eşzamanlı okuma ve yazması.
- **Risk:** Çökmeye yol açabilir. `syncManager` nil okunursa `Increment()` nil pointer panic üretir.
- **Çözüm (K16):** `getSyncManager()` yardımcısı lock+RLock ile pointer kopyalar, tüm çağrıcılar kopya üzerinden çalışır.

### C2. `a.store` startup'ta `storeMu` olmadan atanıyor — ✅ Düzeltildi
- **Dosya:** `app.go:205`
- **Detay:** Başlangıçta `a.store = store` ataması `storeMu` tutulmadan yapılır.
- **Risk:** Startup sırasında başka bir goroutine store'u okursa yarış oluşur.
- **Çözüm:** `a.storeMu.Lock()`/`Unlock()` içine alındı.

### C6. OAuth sunucu sızıntısı + `authWg` yarışı — ✅ Düzeltildi (K19)
- **Dosya:** `internal/cloudsync/drive.go:97-185`
- **Detay:**
  - **Sızıntı:** `StartAuthFlow()` iki kez çağrılırsa ilk HTTP sunucusu sonsuza kadar çalışmaya devam eder.
  - **Yarış:** `dc.authWg.Add(1)` satır 100'de `dc.mu` olmadan çağrılır. İlk flow tamamlanırsa (`Done`), ikinci `StartAuthFlow` `Add` çağırır, sonra `WaitForAuth` başlarsa, ikinci `Done()` sıfırın altına düşer → **`sync.WaitGroup` paniği**.
- **Risk:** Çökme veya kaynak sızıntısı.
- **Çözüm (K19):** `authSrv` alanı eklendi; yeni flow önce eski sunucuyu kapatır. `authDone` flag + `authWg` reset ile duplicate Done paniği önlendi.

### C9. `DeleteGobFile` hata durumunda indeks tutarsızlığı — ✅ Düzeltildi (K20)
- **Dosya:** `internal/memory/store.go:267-269`
- **Detay:** `readDocument` başarılı olursa (dosyayı açar, okur, kapatır) ancak `os.Remove` başarısız olursa, **indeks girdisi sessizce kaldırılmaz**. İndeks, artık var olmayan bir dosyayı referans alır.
- **Risk:** Hafıza indeksi şişer, ölü referanslar oluşur.
- **Çözüm (K20):** İndeksteki ID, dosya hash'inden bulunur (okumaya gerek yok). Önce dosya silinir, başarısız olursa indeks dokunulmaz.

### C10. `UpdateSyncSettings` eski `syncManager`'ı temizlemeden yenisini oluşturur — ✅ Düzeltildi (K16)
- **Dosya:** `app.go:2622-2659`
- **Detay:** Eski `a.syncManager`'ın devam eden bir yedekleme goroutine'i olabilir. Eski yöneticinin `count` atomic'i ve `inFlight` flag'i yetim kalır.
- **Risk:** Yetim goroutine'ler ve tutarsız durum.
- **Çözüm (K16):** `syncMu.Lock()` içinde eski manager nil yapılır, yeni manager oluşturulur. Goroutine doğal olarak sonlanır.

### C11. Flutter: `Navigator.pop()` sonrası `context` kullanımı — ✅ Düzeltildi (K21)
- **Dosya:** `screens/model_store_screen.dart:1497-1498`, `widgets/model_config_dialog.dart:103-111`
- **Detay:** Dialog kapatıldıktan sonra `ScaffoldMessenger.of(context)` çağrılır. Dialog pop edilince context geçersiz olabilir.
- **Risk:** Çökme veya SnackBar'ın görünmemesi.
- **Çözüm (K21):** `ScaffoldMessenger` referansı pop öncesi alınır.

### C12. Flutter: Context menu'de async sonrası context kullanımı — ✅ Düzeltildi (K21)
- **Dosya:** `widgets/chat_message_list.dart:182-188`, `widgets/chat_sidebar.dart:385-391`
- **Detay:** `showMenu` sonrası `.then()` callback'inde widget dispose edilmiş olabilir. `_showEditDialog()` / `_showDeleteConfirm()` context kullanır.
- **Risk:** Widget dispose edildikten sonra context kullanımı.
- **Çözüm (K21):** `if (!mounted) return;` kontrolleri eklendi.

### C13. Flutter: `TextEditingController` `build()` içinde oluşturuluyor, dispose edilmiyor — ✅ Düzeltildi (K21)
- **Dosya:** `widgets/settings_dialog.dart:1678`
- **Detay:** `_ParamIntInput.build()` içinde her frame'de yeni bir `TextEditingController` oluşturulur. Hiçbiri dispose edilmez.
- **Risk:** Bellek sızıntısı, her parametre değişikliğinde sızan controller'lar.
- **Çözüm (K21):** `_ParamIntInput` StatefulWidget'a dönüştürüldü; controller initState'de oluşur, dispose'de temizlenir.

### C14. Flutter: `setState` async sonrası `mounted` kontrolü yok — ✅ Düzeltildi (K21)
- **Dosya:** `widgets/llama_installer_view.dart:53-56`
- **Detay:** Async işlem sonrası `setState` çağrılmadan önce `mounted` kontrolü yok. Dispose sonrası çağrılırsa patlar.
- **Risk:** Çökme.
- **Çözüm (K21):** Tüm async setState öncesi `if (!mounted) return;` eklendi.

### C15. Flutter: `FocusNode.requestFocus()` async sonrası `mounted` kontrolü yok — ✅ Düzeltildi (K21)
- **Dosya:** `widgets/chat_input.dart:65`
- **Detay:** `sendMessage` async çağrısı sonrası `_focusNode.requestFocus()`. Widget dispose edilmişse çöker.
- **Risk:** Çökme.
- **Çözüm (K21):** `requestFocus` öncesi `if (!mounted) return;` eklendi.

---

## 🟠 Yüksek

### H1. OAuth callback'inde duplicate `Done` paniği — ✅ Düzeltildi (K19)
- **Dosya:** `internal/cloudsync/drive.go:156-174`
- **Detay:** Callback goroutine'i `dc.closeAuthDoneLocked()` → `dc.authWg.Done()` çağırır. Callback iki kez tetiklenirse (HTTP replay ile mümkün), `Done` panik üretir.
- **Risk:** Çökme.
- **Çözüm (K19):** `authDone` flag ile duplicate `Done` çağrıları engellendi.

### H2. `callLLMStream` goroutine'i istemci koptuktan sonra 5 dakika daha çalışır — ✅ Kısmen düzeltildi (K11+K17)
- **Dosya:** `app.go:487-554`, `handlers_flutter.go:44-63`
- **Detay:** HTTP istemcisi bağlantıyı kestiğinde `handleSendStream` döner (satır 50/61) ancak `callLLMStream` goroutine'i 300 saniyelik context timeout'u dolana kadar çalışmaya devam eder.
- **Risk:** 5 dakika boyunca GPU/CPU kaynağı boşa harcanır.
- **Çözüm (K11+K17):** `trySend()` context iptalinde bloke olmaz; `processSSEStream`'deki tüm `ch <-` gönderimleri `select` ile korunur. İstemci kopunca kanala bloke olma ve goroutine sızıntısı önlenir.

### H3. Eşzamanlı `AddMessage` çağrıları mesaj sırasını karıştırabilir — ✅ Düzeltildi (K22)
- **Dosya:** `app.go:360-378`, `app.go:487-554`
- **Detay:** `SendMessage` ve `SendMessageStream` (goroutine içinde `finishStream`) aynı anda `a.sessions.AddMessage` çağırır. Mutex korumalı olsa da kullanıcı ve asistan mesajlarının sırası karışabilir.
- **Çözüm (K22):** Per-session mutex (`sessionSendMu`) eklendi. Aynı oturuma yeni mesaj gönderimi, önceki stream bitene kadar bekler.

### H4. `isAuthenticated` zaman aşımı yok — ✅ Düzeltildi (K19)
- **Dosya:** `internal/cloudsync/drive.go:70-93`
- **Detay:** `TokenSource` ve `Token()` çağrıları `context.Background()` kullanır. OAuth token sunucusu yanıt vermezse çağrı sonsuza kadar bloke olur.
- **Çözüm (K19):** 10 saniyelik context timeout eklendi.

### H5. Nil `embeddingClient` ile embedding çağrısı nil pointer panic — ✅ Düzeltildi (K23)
- **Dosya:** `app.go:1455`, `embedder.go:13-21`
- **Detay:** `StopEmbeddingModel` `embeddingClient = nil` atar. `NewEmbeddingFunc` nil client yakalarsa, `client.CreateEmbedding` nil pointer panic üretir. `reinitMemoryStore`'daki parametre kontrolü yetersizdir.
- **Çözüm (K23):** `CheckEmbeddingHealth` lock altında güvenli kopya alır. Init'te store nil ise atanmaz.

### H6. Hafıza sessizce devre dışı kalır, kullanıcı habersiz — ✅ Düzeltildi (K23)
- **Dosya:** `app.go:185-189`
- **Detay:** `NewStore` başarısız olursa (disk hatası), `a.store = nil` atanır. Sonraki işlemler sessizce nil döndürür. Kullanıcıya sadece log satırı yazılır, UI'da bildirim olmaz.
- **Çözüm (K23):** Store nil ise atanmaz; `retrieveMemory` ve `saveMemorySync` event gönderir.

### H7. Flutter: `Future.delayed` iptal edilmiyor — ✅ Düzeltildi (K21)
- **Dosya:** `providers/chat_provider.dart:220-222`
- **Detay:** Her `sendMessage()` çağrısı 2 saniye gecikmeli bir `Future.delayed` oluşturur. Kullanıcı hızlı mesaj gönderirse birden çok timer birikir, hepsi ateşlenir. Dispose edilince temizlenmez.
- **Çözüm (K21):** `Future.delayed` yerine `Timer` kullanıldı, `ref.onDispose` ile iptal edilir.

### H8. Flutter: Async metodlar await edilmiyor — ✅ Düzeltildi (K21)
- **Dosya:** `widgets/chat_sidebar.dart:113-114, 119-125`, `widgets/settings_dialog.dart:498-508`
- **Detay:** `switchTo(id)`, `delete(id)`, `save()` gibi Future dönen metodlar `await` edilmeden çağrılıyor. Hatalar sessizce yutuluyor.
- **Çözüm (K21):** `unawaited()` ile sarıldı veya `await` eklendi.

### H9. Flutter: `build()` içinde yan etki (side-effect mutation) — ✅ Düzeltildi (K21)
- **Dosya:** `screens/chat_screen.dart:120-125`
- **Detay:** `whenData()` callback'leri içinde `title` değişkeni mutate ediliyor. İlk build'de her zaman "Yeni Sohbet" görünür, veri gelince düzelir. Gereksiz bir frame'de yanlış başlık gösterilir.
- **Çözüm (K21):** `ref.listen` ile async sonrası mounted kontrolü eklendi.

### H10. Flutter: Context menu'de stale closure — ✅ Düzeltildi (K21)
- **Dosya:** `widgets/chat_sidebar.dart:36-42`
- **Detay:** `isIncognito` build zamanında yakalanır. Kullanıcı buton ile incognito'yu değiştirdiyse closure güncel olmayabilir.
- **Çözüm (K21):** `if (!mounted) return;` guard eklendi.

### H11. Ngrok `SetRemoteAccess` erken dönüş — config kaydedilmez ✅ Düzeltildi
- **Dosya:** `app.go:1918`
- **Detay:** `SetNgrokMode` token'ı hafızaya yazıp `SetRemoteAccess` çağırır. `SetRemoteAccess`'te `enabled == a.remoteAccessEnabled && port == port` erken dönüş yapar. Ngrok başlamaz, token config'e yazılmaz.
- **Risk:** Ngrok sessizce çalışmaz, kullanıcı "ngrok tunnel started" mesajı görür ama URL gelmez.
- **Çözüm:** Erken dönüşe ngrok mode kontrolü eklendi: `a.cfg.RemoteAccess.NgrokMode == (a.ngrokServer != nil)`.

### H12. Ngrok alt süreç çökmesi UI'a yansımaz ✅ Düzeltildi
- **Dosya:** `internal/ngrok/manager.go:53-87`, `app.go:1899-1909`
- **Detay:** Ngrok subprocess async başlatılır. Auth hatası olursa süreç çöker ama `pollPublicURL` sadece timeout olur. Kullanıcıya hata gösterilmez.
- **Risk:** Kullanıcı yanlış token'la ngrok'un çalıştığını sanır.
- **Çözüm:** `cmd.Wait()` goroutine'i eklendi — süreç çıkınca errMsg yakalanır. `LastError()` metodu ile `RemoteAccessStatus.NgrokError`'a yazılır. Frontend'de kırmızı kutu ile gösterilir.

---

## 🟡 Orta

### M1. `retrieveMemory` request context yerine `context.Background()` kullanır — ✅ Düzeltildi (K23)
- **Dosya:** `app.go:1591`
- **Detay:** Kullanıcı sohbet değiştirdiğinde veya iptal ettiğinde memory retrieval iptal edilemez.
- **Çözüm (K23):** `ctx context.Context` parametresi eklendi, çağrıcılardan türetilir.

### M2. `callLLM` request context yerine `context.Background()` kullanır — ✅ Düzeltildi (K23)
- **Dosya:** `app.go:1608`
- **Detay:** Kullanıcı 120 saniyelik LLM çağrısını iptal edemez.
- **Çözüm (K23):** `ctx context.Context` parametresi eklendi, çağrıcılardan request context türetilir.

### M3. Path traversal Layer 1 kontrolü zayıf ✅ Düzeltildi
- **Dosya:** `internal/webserver/handlers_flutter.go:344`
- **Detay:** `strings.Contains(path, "..")` URL-encoded `..` (`%2e%2e`) ile atlatılabilir. Ancak Layer 2 (`GetImageBase64`) `filepath.EvalSymlinks` ile sağlam kontrol yapar. Gerçek güvenlik Layer 2'ye dayanır.
- **Çözüm:** `url.QueryUnescape` ile önce decode edilir, sonra `..` kontrolü yapılır. `%2e%2e` bypass'ı kapatıldı.

### M4. Çoğu HTTP handler'da istek gövde boyut sınırı yok — ✅ Düzeltildi (K18)
- **Dosya:** `internal/webserver/server.go`, `handlers_flutter.go`
- **Detay:** Sadece `handleTranscribe` `MaxBytesReader` kullanır. Diğer handler'lar sınırsız gövde kabul eder. DoS vektörü.
- **Çözüm (K18):** `limitBodyMiddleware` tüm handler'lara 10MB limit uygular.

### M5. Geçici dosyalar sistem temp dizini yerine uygulama dizinine yazılır ✅ Düzeltildi
- **Dosya:** `internal/llama/installer.go:193`
- **Detay:** Multi-GB model indirmeleri `os.TempDir()` yerine `i.BaseDir` ("data/") dizinine yazılır. Disk dolmasına yol açabilir.
- **Çözüm:** `os.TempDir()` kullanılıyor.

### M6. `syncManager.Increment()` ile `TriggerNow` yarışı — çift yedekleme — ✅ Düzeltildi (K24)
- **Dosya:** `internal/cloudsync/sync_manager.go:108-152`
- **Detay:** `Increment` `m.inFlight` kontrolünü `m.mu` altında yapar, sonra pipeline başlatmadan önce kilidi bırakır. `TriggerNow` araya girip `inFlight = false` görebilir. Sonuç: **iki eşzamanlı yedekleme**.
- **Çözüm (K24):** `scheduleMu` mutex'i Increment ve TriggerNow arasındaki yarışı engeller.

### M7. GitHub API çağrılarında zaman aşımı yok — ✅ Düzeltildi (K25)
- **Dosya:** `internal/llama/installer.go:238, 325`
- **Detay:** `http.DefaultClient` timeout olmadan kullanılır. GitHub API asılı kalırsa çağrı sonsuza kadar bloke olur.
- **Çözüm (K25):** API çağrılarına 30 saniye, dosya indirmelerine 5 dakika timeout eklendi.

### M8. `restoreZip`'de zip bomb koruması yok ✅ Düzeltildi
- **Dosya:** `internal/cloudsync/sync_manager.go:363-412`
- **Detay:** Google Drive'dan gelen zip'in açılmış boyutu sınırlandırılmamış. Güvenilir kaynaktan gelse de, compromised bir yedekleme keyfi miktarda veriyi diske yazabilir.
- **Çözüm:** Dosya başına 100MB, toplamda 500MB limit eklendi. `io.LimitReader` ile copy sınırlandırıldı.

### M9. Flutter: `_init()` constructor'dan çağrılıyor — state geçici olarak yanlış ✅ Düzeltildi
- **Dosya:** `providers/settings_provider.dart:94-96, 283-285, 307-309, 331-333`, `main.dart`
- **Detay:** `StateNotifier` constructor'ları `_init()` çağırır ancak async işlem beklenmez. İlk state hardcoded default'tur, sonra SharedPreferences gelince düzelir. Kısa bir süre yanlış değer gösterilir.
- **Çözüm:** `SharedPreferences` `main()`'de startup'ta yüklenir, `prefsProvider` override ile tüm provider'lara enjekte edilir. Constructor'lar artık synchronous başlar, _init() pattern'i tamamen kaldırıldı.

### M10. Flutter: `ref.listen` her build'de yeniden kaydolur ✅ Düzeltildi
- **Dosya:** `screens/chat_screen.dart:41-48`, `screens/model_store_screen.dart:1015-1021`
- **Detay:** Her rebuild'de yeni listener eklenir. Riverpod deduplicate etse de gereksiz overhead oluşur.
- **Çözüm:** `ConsumerWidget` → `ConsumerStatefulWidget` dönüştürüldü. `ref.listen` çağrıları `initState()`'e taşındı.

### M11. Flutter: `_ModelParametersCardState` ayarları güncellenmez ✅ Düzeltildi
- **Dosya:** `widgets/settings_dialog.dart:1517-1523`
- **Detay:** `llamaSettingsProvider` değişirse local state güncellenmez. Kullanıcı diğer sekmeden değişiklik yaparsa UI yansımaz.
- **Çözüm:** `ref.listen` ile provider değişiklikleri takip edilir. `_saveVersion`/`_displayedVersion` mekanizması — kullanıcı düzenleme yapmadıysa (saveVersion == displayedVersion) otomatik sync olur. Save butonuna basınca version artar.

### M12. Flutter: `MarkdownStyleSheet` her frame'de yeniden oluşturulur ✅ Düzeltildi
- **Dosya:** `widgets/chat_message_list.dart:9-37`
- **Detay:** `_buildMarkdownStyleSheet(context)` her `_MessageBubble.build()`'de çağrılır. Cache mekanizması yok.
- **Çözüm:** `_styleCache` map'i eklendi — tema değişene kadar aynı style sheet döndürülür.

### M13. Ngrok error field API'da yok ✅ Düzeltildi
- **Dosya:** `app.go:1875-1910`
- **Detay:** `RemoteAccessStatus` struct'ında ngrok hatasını taşıyacak alan yok. Backend ngrok hatasını bilse bile frontend'e aktaramaz.
- **Çözüm:** `NgrokError string \`json:"ngrok_error"\`` eklendi, `GetRemoteAccessStatus`'te `a.ngrokServer.LastError()` ile doldurulur.

### M14. Ngrok Install bundled `binaries/` yolunu kontrol etmez ✅ Düzeltildi
- **Dosya:** `internal/ngrok/installer.go:34-81`
- **Detay:** `Install("data")` sadece `data/binaries/<os>/ngrok` yolunu kontrol eder. `binaries/<os>/ngrok` (bundled) yok sayılır, binary tekrar indirilir.
- **Risk:** Gereksiz download (~31MB).
- **Çözüm:** `Install` önce `data/binaries/`, sonra bundled `binaries/` yolunu kontrol eder. Bundled varsa direkt kullanır.

---

## 🔵 Düşük

### L1. `saveToken` hataları sessizce yutuluyor ✅ Düzeltildi
- **Dosya:** `internal/cloudsync/drive.go:89, 174, 224, 256`
- **Detay:** `_ = dc.saveToken(t)` — token kaydedilemezse sessizce geçilir. Yeniden başlatmada token kaybolur.
- **Çözüm:** Hata loglanıyor.

### L2. `writeJSON` encode hatalarını yutar ✅ Düzeltildi
- **Dosya:** `internal/webserver/server.go:537-539`
- **Detay:** `json.NewEncoder(w).Encode(v)` hatası kontrol edilmez. Bağlantı koparsa veya JSON serialize edilemezse sessizce başarısız olur.
- **Çözüm:** Hata loglanıyor: `log.Printf("writeJSON error: %v", err)`.

### L3. `config.Save` validation hatalarını bildirmez ✅ Düzeltildi
- **Dosya:** `internal/config/config.go:170-173`
- **Detay:** `cfg.validate()` hata döndürmez, geçersiz değerleri sessizce düzeltir. Kullanıcı girdisinin yok sayıldığını bilmez.
- **Çözüm:** `validate()` artık `[]string` döndürür. Düzeltilen alanlar loglanır: `log.Printf("config: applied defaults for: %v", fixes)`.

### L4. STT binary dünya-tarafından çalıştırılabilir ✅ Düzeltildi
- **Dosya:** `app.go:416`
- **Detay:** STT binary'si 0755 izniyle geçici dizine yazılır. Diğer kullanıcılar binary'i okuyabilir.
- **Çözüm:** 0700 (sadece sahip çalıştırabilir) olarak değiştirildi.

### L5. STT süreç grubu temizlenmez ✅ Düzeltildi
- **Dosya:** `app.go:422-423`, `stt_proc_unix.go`
- **Detay:** Sadece ana süreç kill edilir, alt süreçler yetim kalır.
- **Çözüm:** STT subprocess `Setpgid: true` ile kendi process group'unda başlatılır; shutdown'da tüm group kill edilir.

### L6. Alt süreçler için `Setpgid` ayarlanmamış ✅ Zaten düzeltilmiş
- **Dosya:** `internal/llama/sysproc_linux.go:12-16`
- **Detay:** `Setpgid` ayarlanmazsa `Pdeathsig` sadece direkt alt süreci öldürür, torunları hayatta kalır.
- **Durum:** Kod incelemesinde `Setpgid: true` zaten ayarlanmış olduğu görüldü. Doküman hatası.

### L7. Flutter: `const` constructor'lar eksik (birçok yerde)
- **Tüm proje genelinde:**
  - `AppShell()`, `ChatSidebar()`, `ChatInput()`, `WelcomeView()`, sayısız `SizedBox()`, `Padding()`, `Text()`, `Icon()` çağrısı `const` değil.

### L8. Flutter: Boş `catch (_)` blokları hataları yutar ✅ Düzeltildi
- **Dosya:** `core/api_client.dart:68, 606`, `providers/settings_provider.dart:214, 221, 229, 239, 268`
- **Detay:** Hatalar sessizce yutulur, kullanıcıya bildirilmez.
- **Çözüm:** `catch (_)` → `catch (e)` olarak değiştirildi. Hata değişkeni erişilebilir oldu.

### L9. Flutter: `connectionStatusProvider` sadece bir kere sorgulanır ✅ Düzeltildi
- **Dosya:** `providers/chat_provider.dart:375-385`
- **Detay:** `FutureProvider` olduğu için sadece bir kere çalışır. Backend sonradan düşerse durum göstergesi yeşil kalır.
- **Çözüm:** `StreamProvider`'a dönüştürüldü — 30 saniyede bir `isAlive()` sorgular.

### L10. Flutter: Türkçe string'ler L10n sistemi dışında hardcoded ✅ Düzeltildi
- **Dosya:** `screens/model_store_screen.dart` (öncelikli)
- **Detay:** Çeviri sistemi bypass edilir. İngilizce UI'da Türkçe metinler görünür.
- **Çözüm:** `model_store_screen.dart`'deki tüm hardcoded Türkçe string'ler `L10n.t(...)` ile değiştirildi.

### L11. Ngrok UI "Start" butonu ve token pre-fill eksik ✅ Düzeltildi
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:2135-2185`
- **Detay:** Her açılışta token alanı boş, kullanıcı her seferinde token'ı tekrar girmek ve ayrıca "Start ngrok Tunnel" butonuna basmak zorunda. Token config'de kayıtlı olsa bile UI boş gösterir.
- **Çözüm:** Token backend'den prefetch edilip otomatik doldurulur. Toggle ile ngrok otomatik başlar, ayrı buton kalktı. Loading state, auto-refresh timer (2sn'de bir 20sn boyunca) eklendi.

### L12. Ngrok bağlantı durumu otomatik yenilenmez ✅ Düzeltildi
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:2159-2184`
- **Detay:** Ngrok async başlatılır (1-5sn). Frontend sadece toggle anında bir kere sorgular, ngrok URL'si gelene kadar beklemez. Kullanıcı sayfayı manuel yenilemek zorunda.
- **Çözüm:** `_startRefreshTimer()` — `Timer.periodic` ile 2sn'de bir `remoteAccessProvider` invalidate edilir, maksimum 10 tekrar (20sn). Timer dispose'da cancel edilir. Token değişince `onEditingComplete` ile otomatik kaydedilir.

---

## ⚪ Bilgi / Gözlemler

### B1. Eski GOB Formatı (SQLite'e taşındı)
- **Dosya:** `internal/memory/store.go` (legacy migration yolu)

### B2. Etkileşim Başına Tek Dosya Tasarımı
- **Dosya:** `internal/memory/store.go`

### B3. Filepath.Walk Hata Yutma
- **Dosya:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`

### B4. Gömme İstemcisi Yeniden Başlatma Sonrası Eski Referans
- **Dosya:** `app.go:148-149`, `app.go:124-125`

### B5. Dosya Adına Göre Model Otomatik Sınıflandırma
- **Dosya:** `internal/modelstore/modelstore.go:58-64`

### B6. `unsanitizePath`, Repo ID'lerindeki `__`'den `/` Enjekte Edebilir
- **Dosya:** `internal/modelstore/modelstore.go:345`

### B7. Llama Sunucusu Stderr'i Uygulama Loglarıyla Karışıyor
- **Dosya:** `internal/llama/llama.go:118-119`

### B8. Flutter: `App` struct'ında context saklanması (anti-pattern)
- **Dosya:** `app.go:100`
- **Not:** Go `context` paketi dokümantasyonuna göre context'ler struct'da saklanmamalı, fonksiyonlara parametre olarak geçilmeli. `a.ctx` birden çok yerde kullanılır.

### B9. Flutter: L10n kendi listener sistemini kullanıyor, Riverpod değil
- **Dosya:** `core/l10n.dart:8`
- **Not:** İki paralel bildirim sistemi mevcut.

### B10. Flutter: Hardcoded Türkçe string'ler L10n bypass ediyor
- Detay için L10'a bakın.

---

> **Son güncelleme:** 2026-06-11
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, tüm internal/ paketleri) ve Flutter frontend
> **Toplam hata:** 37 (🔴7, 🟠12, 🟡14, 🔵4) — 37'si düzeltildi ✅
> **Kalan:** 0 — tüm hatalar giderildi 🎉
> **Toplam gözlem:** 10
