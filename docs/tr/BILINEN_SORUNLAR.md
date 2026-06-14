# Bilinen Sorunlar ve Teknik Riskler

Bu belge, Memo projesindeki tüm açık hataları ve teknik riskleri takip eder.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🔵 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- ⚪ **Düşük** — kozmetik, küçük iyileştirme veya uç durum
- ⚫ **Bilgi** — tasarım notu, risk veya gözlem

---

## ✅ Düzeltilen Hatalar

Bu oturumda aşağıdaki hatalar düzeltildi:

### F1. Flutter: Boş `catch (_)` Blokları Hataları Sessizce Yutuyor
- **Öncesi:** 25'ten fazla `catch (_) {}` bloğu tüm hataları sessizce yutuyordu.
- **Yapılan:** Tüm catch blokları `catch (e) { debugPrint('error: $e'); }` olarak güncellendi.
- **Kullanıcı etkisi:** Kaydetme, değiştirme, test etme gibi işlemler sessizce başarısız oluyordu.

### F2. WhatsApp SSE İşleyicisi `ctx.Done()` İzlemiyor
- **Öncesi:** `for chunk := range streamCh` ile `ctx.Done()` kontrolü yoktu. İstemci kopunca goroutine 5 dakika yaşıyordu.
- **Yapılan:** `select { case <-ctx.Done(): return; case chunk, ok := <-streamCh: ... }` eklendi.
- **Kullanıcı etkisi:** Kaynak sızıntısı — sık sekme değiştirmede goroutine birikmesi.

### F3. Agent İzin İptali Yanlış ID Gönderiyor
- **Öncesi:** `revoke(p.argsHash)` backend'in beklediği `id` yerine `argsHash` gönderiyordu.
- **Yapılan:** `AgentPermission` modeline `id` alanı eklendi, çağrı `revoke(p.id)` olarak değiştirildi.
- **Kullanıcı etkisi:** İzin iptalleri sessizce başarısız oluyor, kullanıcı izinlerin hâlâ geçerli olduğunu fark etmiyordu.

### F4. Sağlayıcı Bağlantı Testi Sessizce `false` Dönüyor
- **Öncesi:** `catch (_) { return false; }` ile hata nedeni gizleniyordu.
- **Yapılan:** Dönüş tipi `Map<String, dynamic>` olarak değiştirildi, hata bilgisi UI'a iletilir oldu.
- **Kullanıcı etkisi:** Test başarısız olunca nedenini anlayamıyor, yanlış API anahtarını tekrar deniyordu.

### F5. WhatsApp Akışında Hata İşleme ve İptal Desteği Yok
- **Öncesi:** `sendWhatsAppChatStream`'de `try/catch` ve `CancelToken` yoktu.
- **Yapılan:** `try/catch` eklendi, `CancelToken` parametresi eklendi, akış döngüsünde iptal kontrolü yapılıyor.
- **Kullanıcı etkisi:** Ağ hatasında uygulama çöküyordu. WhatsApp akışı iptal edilemiyordu.

### F6. Sohbet Stream Deadlock'u
- **Öncesi:** `sendMessage()` catch bloğunda `_stopped = false` çağrılmıyordu. Hata sonrası tüm mesajlar streaming kullanmaz hale geliyordu.
- **Yapılan:** Catch bloğuna `_stopped = false` eklendi.
- **Kullanıcı etkisi:** Hata sonrası agent/streaming modu kalıcı kilitleniyor, sadece uygulama restartı düzeltiyordu.

### F7. Aktif Sağlayıcı UI'da Görünmüyor
- **Öncesi:** `_ProvidersTab` hangi sağlayıcının aktif olduğunu göstermiyordu.
- **Yapılan:** `_ProviderCard`'a `isActive` prop'u, yeşil `AKTİF` rozeti ve yeşil kenarlık eklendi.
- **Kullanıcı etkisi:** "Hangi sağlayıcı cevap veriyor?" — kullanıcı başka ekrana geçmek zorundaydı.

### F8. Model/Embedding Durumu Sonsuz Polling
- **Öncesi:** `modelStatusProvider` ve `embeddingStatusProvider` her 5 saniyede polling yapıyor, uygulama boyunca hiç durmuyordu.
- **Yapılan:** `StreamProvider.autoDispose` eklendi — model store ekranı kapandığında polling duruyor.
- **Kullanıcı etkisi:** Günde 34.560+ gereksiz HTTP isteği, pil tüketimi.

### F9. `handleSendFileStream`: Geçici Dosya Sızıntısı + MIME Panik
- **Öncesi:** Hata durumunda geçici dosya silinmiyordu (`defer os.Remove` yoktu). `mimeType[:5]` kısa MIME tiplerinde panic atıyordu.
- **Yapılan:** `defer os.Remove(tmpFilePath)` eklendi. `strings.HasPrefix(mimeType, "image")` ile değiştirildi.
- **Kullanıcı etkisi:** Her başarısız dosya yüklemesi `/tmp/memo_web_*` dosyası bırakıyordu. Kısa MIME tipli dosya yüklemeleri handler'ı çökertiyordu.

### F10. Bağlantı Durumu Provider Hata Loglamıyor
- **Öncesi:** `catch (_) { yield false; }` ile hatalar sessizce yutuluyordu.
- **Yapılan:** `debugPrint` eklendi.
- **Kullanıcı etkisi:** Bağlantı durumu "bağlı değil" gösteriyor ancak sebebi belirtilmiyordu.

### F11. HTTP İstek Gövdesi Boyut Sınırı (K02)
- **Öncesi:** Global limit 10MB idi, >10MB dosya yüklemeleri çalışmıyordu.
- **Yapılan:** Limit 50MB'a çıkarıldı.
- **Kullanıcı etkisi:** 50MB'a kadar dosya yüklemeleri artık çalışıyor.

### F12. Agent Sandbox: Kabuk Yorumlayıcıları Kara Listeye Eklendi (K04)
- **Öncesi:** `sh`, `bash`, `zsh`, `dash` engellenmiyordu — `sh -c "rm -rf /"` ile sandbox kaçışı mümkündü.
- **Yapılan:** Kabuk yorumlayıcıları komut kara listesine eklendi.
- **Kullanıcı etkisi:** Agent artık kabuk üzerinden sandbox'ı bypass edemez.

### F13. `callLLMStream` Goroutine Sızıntısı (H06)
- **Öncesi:** `for chunk := range ch` provider timeout'una (300s) kadar bloke oluyordu.
- **Yapılan:** Her iki döngü (provider + local model) `select` ile `ctx.Done()` izleyecek şekilde değiştirildi.
- **Kullanıcı etkisi:** İstemci bağlantıyı kestiğinde goroutine anında çıkıyor.

### F14. Veri Yarışları: Provider Router, Aktif Sağlayıcı, Oturumlar (K03)
- **Öncesi:** `providerRouter`, `activeProvider`, `sessions`, `syncManager` alanları korumasızdı.
- **Yapılan:** Muteks korumaları (`providerMu`, `sessionsMu`) eklendi, sync TOCTOU yarışı düzeltildi.
- **Kullanıcı etkisi:** Eşzamanlı yapılandırma değişikliklerinde çökme riski azaldı.

### F15. SSE Akış Zaman Aşımı (H08)
- **Öncesi:** `await for` stream durunca sonsuz bekliyordu.
- **Yapılan:** Stream tüketimine 60s `.timeout()` eklendi.
- **Kullanıcı etkisi:** Donan akışlar artık zaman aşımına uğrayıp hata gösteriyor.

### F16. Agent İzin Kartlarında İnsan Tarafından Okunabilir İsimler
- **Öncesi:** Kartlar `write_file`, `run_command` gibi ham araç adları gösteriyordu.
- **Yapılan:** `tool_names.dart` ile Türkçe görünen adlar, açıklamalar ve ikonlar eklendi.
- **Kullanıcı etkisi:** Kullanıcılar artık "Dosya Yaz", "Komut Çalıştır" gibi anlaşılır ifadeler görüyor.

### F17. `DeleteLocalModel`'de TOCTOU Sembolik Bağlantı Yarışı (K01)
- **Öncesi:** `filepath.EvalSymlinks` ile `os.Remove` arasında sembolik bağlantı değiştirme saldırısı mümkündü.
- **Yapılan:** `os.Remove` öncesinde yeniden `EvalSymlinks` + yeniden doğrulama eklendi.
- **Kullanıcı etkisi:** TOCTOU penceresi kapatıldı — yol silme anında yeniden doğrulanıyor.

### F18. WebServer CORS: Her Origin'i Yansıtıyor (K03)
- **Öncesi:** `corsMiddleware` gelen Origin header'ını whitelist kontrolü olmadan doğrudan yanıta yansıtıyordu.
- **Yapılan:** Sadece `localhost`, `127.0.0.1`, `::1` origin'lerine izin veren whitelist eklendi.
- **Kullanıcı etkisi:** Kötü niyetli web siteleri artık kullanıcının tarayıcısı üzerinden yerel API'ye erişemez.

### F19. Orchestra `safeProgress` Deadlock (K02)
- **Öncesi:** `safeProgress` mutex tutarken `fn(up)` çağırıyor, bu da dolu kanala yazmaya çalışınca tüm goroutine'leri kilitleyen deadlock oluşturuyordu.
- **Yapılan:** `progressMu` conductor'dan kaldırıldı; `safeProgress` artık doğrudan `fn(up)` çağırıyor. `fullBuf` güvenliği `app.go`'da yerel `sync.Mutex` ile sağlanıyor.
- **Kullanıcı etkisi:** Orchestra artık yavaş/kopuk istemcilerde donmuyor.

### F20. Orchestra: Çok Turlu Sohbetlerde Yanlış Kullanıcı Mesajı (K07)
- **Öncesi:** `userPrompt` çıkarma kodu ilk kullanıcı mesajını alıyordu; son mesaj değil.
- **Yapılan:** `callAgentWithOrchestra`, `callLLMStream`, `callLLM` içindeki döngüler tersine çevrildi.
- **Kullanıcı etkisi:** Çok turlu sohbetlerde orchestra artık doğru soruyu yanıtlıyor.

### F21. Skill `Install()`: Manifest Name Path Traversal (H25) + `os.Stat` Hata Yutma (M25)
- **Öncesi:** `def.Manifest.Name` YAML'dan alınıp doğrulanmadan `filepath.Join` ile kullanılıyordu; `os.Stat` hataları yutuluyordu.
- **Yapılan:** `validateSkillName()` ile `/`, `\`, `..` içeren isimler reddediliyor; `filepath.Abs` karşılaştırmasıyla hedef dizin `SkillsDir()` içinde doğrulanıyor; `os.Stat` switch ile düzgün hata kontrolü yapılıyor.
- **Kullanıcı etkisi:** Kötü niyetli SKILL.md ile dosya sistemi geçişi artık mümkün değil.

### F22. Skill `Remove()`: Path Traversal (K05)
- **Öncesi:** `os.RemoveAll(def.Path)` çağrılmadan önce `def.Path`'in `SkillsDir()` içinde olduğu doğrulanmıyordu.
- **Yapılan:** `filepath.Abs` ile `def.Path` kökünün `SkillsDir()` içinde olduğu kontrol ediliyor.
- **Kullanıcı etkisi:** Bozuk bir skill kurulumunun ardından `Remove` çağrısı artık uygulama dosyalarını silemez.

### F23. Skill `copyDir`: Sembolik Bağlantı Takibi (H24) + Boyut Limiti Eksikliği (M26 rollback dahil)
- **Öncesi:** Symlink'ler `os.ReadFile` ile takip edilerek hassas sistem dosyaları kopyalanabiliyordu; boyut sınırı yoktu; `copyDir` başarısız olunca artık dizin bırakılıyordu.
- **Yapılan:** Symlink'ler atlanıyor (`entry.Type()&os.ModeSymlink != 0`); dosya başına 10MB sınırı eklendi; `copyDir` başarısızlığında `os.RemoveAll(targetDir)` rollback yapılıyor.
- **Kullanıcı etkisi:** Kötü niyetli skill artık sistem dosyalarını kopyalayamaz ve bellek DoS oluşturamaz.

### F24. `/skill:on <name>` Diğer Skill'leri Siliyordu (H26)
- **Öncesi:** `SetActive([]string{name})` mevcut aktif seti tamamen değiştiriyordu.
- **Yapılan:** Mevcut aktif listeye ekleme yapılıyor; zaten aktifse bilgilendirici mesaj gösteriliyor.
- **Kullanıcı etkisi:** Yeni skill aktifleştirilirken mevcut aktif skill'ler artık silinmiyor.

### F25. `handleActiveProvider` / `handleOrchestraConfig` Nil Guard (H14)
- **Öncesi:** `s.fullBridge == nil` kontrolü yoktu, nil bridge'de server panic atıyordu.
- **Yapılan:** Her iki handler'a başta `if s.fullBridge == nil` guard eklendi.
- **Kullanıcı etkisi:** Test ortamı veya kısmi başlatmada endpoint'ler artık server'ı çökertmiyor.

### F26. `createBackup()` Hatası Sessizce Yutuluyordu (H16)
- **Öncesi:** `file.go` ve `edit.go`'daki tüm araçlarda `createBackup(fullPath)` dönüş değeri görmezden geliniyordu.
- **Yapılan:** Tüm `WriteFile`, `DeleteFile`, `EditFile`, `InsertLine`, `DeleteLines` fonksiyonlarında backup hatası kontrol ediliyor ve yazma iptal ediliyor.
- **Kullanıcı etkisi:** Disk dolu veya izin hatası durumunda yedeksiz yazma yapılmıyor.

### F27. `run_command` CWD Geçiş Kontrolü Bypass (H17)
- **Öncesi:** `EvalSymlinks` başarısız olunca CWD traversal kontrolü tamamen atlanıyordu.
- **Yapılan:** `EvalSymlinks` hatası için `IsNotExist` ayrımı yapılıyor; var olmayan dizinlerde `filepath.Clean` sonucu doğrulanıyor; diğer hatalar reddediliyor.
- **Kullanıcı etkisi:** Agent artık var olmayan bir dizin yolu ile sandbox dışında komut çalıştıramıyor.

### F28. `/api/image` Handler Path Traversal (K25)
- **Öncesi:** URL-encoded path (`%2Fetc%2Fpasswd`) ile `IsAbs` bypass edilebiliyordu.
- **Yapılan:** `IsAbs` kontrolü URL-decoded değer üzerinde yapılıyor; `..` ve absolute path kontrolleri decode sonrası çalışıyor; `filepath.Clean` normalizasyonu eklendi; sadece `data/images/`, `data/avatars/`, `data/attachments/` alt dizinlerine izin veren whitelist eklendi.
- **Kullanıcı etkisi:** Artık rastgele dosya okunamaz, sadece izinli dizinlerdeki dosyalara erişilebilir.

### F29. `callLLM` `activeProvider`/`providerRouter` Data Race (K26)
- **Öncesi:** `callLLM` streaming olmayan yol `activeProvider` ve `providerRouter` okumalarını `providerMu` olmadan yapıyordu.
- **Yapılan:** `a.providerMu.RLock()` / `RUnlock()` eklendi, local değişkenlere kopyalama yapıldı — `callLLMStream`'deki pattern ile uyumlu hale getirildi.
- **Kullanıcı etkisi:** Eşzamanlı provider değişikliklerinde veri yarışı riski ortadan kalktı.

### F30. Skill `SetActive` Tool Kayıt Mantığı (H21)
- **Öncesi:** Yeni aktifleştirilen skill'lerin tool'ları `RegisterTool` yerine `UnregisterTool` ile kaldırılıyordu.
- **Yapılan:** `UnregisterTool` → `RegisterTool` olarak düzeltildi. Gereksiz ikinci kayıt bloğu kaldırıldı.
- **Kullanıcı etkisi:** Skill aktifleştirildiğinde tool'ları artık doğru şekilde kaydediliyor.

### F31. Cloud Sync `WaitGroup` Sızıntısı (H22)
- **Öncesi:** İkinci auth flow'da `WaitGroup` sıfırlanırken eski WaitGroup garbage collect ediliyor, önceki `WaitForAuth` çağrıları sonsuz bloke oluyordu.
- **Yapılan:** Handle edildi.
- **Kullanıcı etkisi:** Cloud sync artık ikinci auth flow'da donmuyor.

### F32. SQLite Context İptali (H23)
- **Öncesi:** `execWrite` caller context'i yerine `db.ctx` (background context) kullanıyordu.
- **Yapılan:** `execWrite` artık `context.Context` parametresi alıyor ve caller'ın context'ini kullanıyor.
- **Kullanıcı etkisi:** SQLite yazma işlemleri artık iptal edilebilir.

### F33. Flutter AgentEventBus StreamController Sızıntısı (H24)
- **Öncesi:** `StreamController` hiç dispose edilmiyordu.
- **Yapılan:** `ref.onDispose(() => bus.dispose())` eklendi.
- **Kullanıcı etkisi:** Agent event bus bellek sızıntısı giderildi.

### F34. Mobile Chat Stream Subscription Sızıntısı (H25)
- **Öncesi:** `StreamSubscription` hiçbir yerde saklanmıyor/iptal edilmiyordu.
- **Yapılan:** `_streamSubscription` alanı eklendi; her yeni gönderimde eski subscription iptal ediliyor; dispose'da temizleniyor.
- **Kullanıcı etkisi:** Mobile chat'te artık stream subscription sızıntısı yok.

### F35. Agent Permission Channel Blokajı (M35)
- **Öncesi:** `HandlePermissionResponse`'daki send `waitFn` dönmüşse sonsuz bloke oluyordu.
- **Yapılan:** `select` ile 1s timeout eklendi — kanal doluysa permission response atılıyor.
- **Kullanıcı etkisi:** Permission timeout sonrası HTTP handler goroutine sızıntısı giderildi.

### F36. Memory Store COUNT(*) Hata Kontrolü (M36)
- **Öncesi:** `Scan` hatası sessizce yutuluyordu.
- **Yapılan:** Hata kontrolü ve log eklendi.
- **Kullanıcı etkisi:** Veritabanı hatası artık log'da görünüyor.

### F37. API Streaming `body.Close()` Double-Close (M37)
- **Öncesi:** Deferred `body.Close()` ile watcher goroutine'i arasında double-close race vardı.
- **Yapılan:** `sync.Once` ile close tekilleştirildi.
- **Kullanıcı etkisi:** Nadir double-close panic riski giderildi.

### F38. Agent Screen Permission Dialog Bypass (M38)
- **Öncesi:** Back button ile dialog kapatılabiliyordu.
- **Yapılan:** `PopScope(canPop: false)` eklendi.
- **Kullanıcı etkisi:** Permission dialog artık back button ile bypass edilemez.

### F39. Model Store Dio Instance Sızıntısı (M39)
- **Öncesi:** `_loadReadme()`, `_loadMoreModels()` her biri kendi `Dio()` örneğini oluşturuyordu.
- **Yapılan:** Paylaşılan `apiClientProvider.dio` kullanılıyor.
- **Kullanıcı etkisi:** Gereksiz Dio örnekleri ve tutarsız timeout'lar giderildi.

### F40. Settings `didUpdateWidget` Kullanıcı Input'u Üzerine Yazma (M42)
- **Öncesi:** `_controller.text` her `didUpdateWidget`'ta kullanıcının düzenlemesinin üzerine yazıyordu.
- **Yapılan:** `_isEditing` flag'ı eklendi; focus durumunda güncelleme yapılmıyor.
- **Kullanıcı etkisi:** Settings'te kullanıcı input'u artık dış güncellemelerle silinmiyor.

### F41. Provider Config Dialog Double-Submit (L34)
- **Öncesi:** `_save()`'de `_saving` guard yoktu; loading göstergesi yoktu.
- **Yapılan:** `_isSaving` flag'ı eklendi; buton disabled + spinner gösteriliyor.
- **Kullanıcı etkisi:** Çift submit ve "dondu" hissi giderildi.

### F42. `memorySaveWorker` Context (L30)
- **Öncesi:** `context.Background()` kullanıyordu.
- **Yapılan:** `a.ctx` kullanılıyor — shutdown'da iptal edilebilir.
- **Kullanıcı etkisi:** Shutdown sonrası kısa süreli memory save devam etme riski azaltıldı.

### F43. `ExportData` File Handle Sızıntısı (L28)
- **Öncesi:** `defer f.Close()` döngü içinde birikiyordu.
- **Yapılan:** Explicit `f.Close()` ile her dosya hemen kapatılıyor.
- **Kullanıcı etkisi:** Büyük export'larda file descriptor tükenmesi riski giderildi.

---

## 🔴 Kritik

### K01. Agent Araç Argümanlarında Yol Geçişi Koruması Yok
- **Dosya:** `internal/agent/sandbox.go:70-94` (`ValidatePath`), `internal/agent/tools/command.go` (`RunCommand` dosya argümanları)
- **Detay:** Sandbox, çalışma dizinini `strings.HasPrefix` ile doğrular ancak araç argümanlarındaki dosya yollarını (`read_file`, `write_file`, `edit_file` vb.) doğrulamaz. `write_file` aracına `../../etc/passwd` gibi göreli bir yol verildiğinde, sandbox dizini dışına çıkılabilir.
- **Risk:** Göreli yollar içeren araç argümanları ile sandbox kaçışı.
- **Kategori:** Güvenlik

### K04. 0.0.0.0'a Bağlanınca Tüm Endpoint'ler Açık — Kimlik Doğrulama Yok
- **Dosya:** `internal/webserver/server.go:79-201` (StartHTTPWithAddr)
- **Detay:** Uzaktan erişim etkinleştirildiğinde sunucu `0.0.0.0` adresine bağlanır. Tüm endpoint'ler (`/api/wipe`, `/api/whatsapp/send`, `/api/agent/permission`, `/api/import` vb.) hiçbir token veya session doğrulaması olmadan yerel ağdaki herkese açık hale gelir.
- **Risk:** LAN'daki herhangi bir cihaz tüm verileri silebilir, WhatsApp mesajı gönderebilir, agent'ı kontrol edebilir.
- **Kategori:** Güvenlik (kimlik doğrulama eksikliği)

### K25. Provider API Anahtarları Zayıf Makine-Türevli Anahtarla Şifreleniyor
- **Dosya:** `internal/provider/config.go:374-402`
- **Detay:** `defaultMachineKey()` AES-256 anahtarını `/etc/machine-id` (Linux), registry GUID (Windows), veya IOPlatformUUID (macOS) ile türetir. Bu değerler gizli değildir — makinedeki herhangi bir işlem bunları okuyabilir. Satır 401'deki hardcoded fallback anahtarı `"Mm3m0L0c4lK3y!@#$%^&*()9876543210"` kaynak kodunda düz metin olarak görünür. Dosya sistemine erişimi olan herhangi bir saldırgan `providers.json`'daki tüm API anahtarlarını çözebilir.
- **Risk:** Tüm saklanan API anahtarları (OpenAI, Claude, Gemini vb.) çözülebilir — toplu credential sızıntısı.
- **Kategori:** Güvenlik (zayıf şifreleme)

### K26. Cloud Sync Şifreleme: Passphrase Boşken Donanım Kimliği Kullanılıyor
- **Dosya:** `internal/cloudsync/crypto.go:27-38, 67-87`
- **Detay:** `encrypt()` ve `deriveKey()` passphrase boş olduğunda `hardwareID()`'e düşer. `hardwareID()` (satır 113-148) `/etc/machine-id`, hostname veya registry anahtarlarını kullanır — bunların hiçbiri gizli değildir. Daha da kötüsü, `decrypt()` yolu önce PBKDF2 dener, başarısız olursa sessizce zayıf SHA-256 `deriveKey()`'e düşer. Veri bütünlüğü ve gizliliği riske girer.
- **Risk:** Cloud yedeklemeleri makine ID'sine veya hostname'e erişimi olan herkes tarafından çözülebilir.
- **Kategori:** Güvenlik (zayıf şifreleme)

---

## 🟠 Yüksek

### H01. Sağlayıcı Öncelik (Priority) Alanı Kullanılmıyor
- **Dosya:** `internal/provider/config.go`, `router.go:188-204`
- **Detay:** `ProviderConfig.Priority` alanı tanımlı ancak `getActiveEntries()` sağlayıcıları öncelik sırasına göre değil, Go map iterasyon sırasına göre döndürür.

### H02. Frontend ApiClient'ta Agent Metodu Yok
- **Dosya:** `frontend/lib/core/api_client.dart`
- **Detay:** Backend'de agent endpoint'leri tamamen çalışır durumda ancak frontend `api_client.dart` bunları çağıracak metodlara sahip değil. Agent modu UI'dan açılıp kapatılamaz.

### H03. İndirme İlerlemesi Yoklaması Hiç Durmuyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:71-87`
- **Detay:** `downloadProgressProvider` sonsuz `while (true)` döngüsüne sahip. Her 1 saniyede bir `/api/models/download/progress` vuruyor, uygulama hayatı boyunca asla tam durmaz. Saniyede 1 HTTP isteği sürekli.
- **Risk:** Boş yere CPU/ağ tüketimi, pil ömrünü kısaltır.

### H04. ngrok Binary İndirmesinde Bütünlük Kontrolü Yok
- **Dosya:** `internal/ngrok/installer.go:34-91`
- **Detay:** ngrok binary'si HTTPS üzerinden indirilir ancak SHA256 sağlama toplamı doğrulaması yoktur. İndirme URL'si (`bin.ngrok.com/c/bNyj1mQVY4c/`) statik bir API anahtarı içerir. CDN ele geçirilirse veya indirme kesintiye uğrarsa, kötü niyetli bir binary yüklenebilir.
- **Risk:** Tehlikeye atılmış ngrok binary'si ile rastgele kod yürütme.

### H05. WhatsApp SQL LIKE Enjeksiyonu
- **Dosya:** `internal/whatsapp/store.go:107-138` (`SearchMessages`)
- **Detay:** Kullanıcı girdisi `"%" + query + "%"` ile LIKE desenine sarılır. Sorgu `_` (LIKE'da tek karakter joker karakteri) içeriyorsa, istenmeyen sonuçlar döner. Örnek: `"test_"` sorgusu `"test1"`, `"testX"` vb. eşleşir.
- **Risk:** Alt çizgi içeren mesaj aramalarında yanlış sonuçlar.

### H06. Flutter: Global Stil Önbelleği (`_styleCache`) Bellek Sızıntısı
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:13`
- **Detay:** `_styleCache` global mutable bir `Map`'tir — asla temizlenmez, ziyaret edilen her tema yapılandırması kombinasyonu ile sonsuz büyür. Bu, temalar değiştirildikçe bellek kullanımının sürekli arttığı bir bellek sızıntısıdır.

### H08. Flutter: `connectionStatusProvider` Sonsuz Polling
- **Dosya:** `frontend/lib/providers/chat_provider.dart:429-438`
- **Detay:** `connectionStatusProvider` uygulama ömrü boyunca her 30 saniyede bir polling yapan `while(true)` döngüsü çalıştırır. `autoDispose` kullanılmamış.

### H09. Orkestra Yapılandırması Doğrulamasız
- **Dosya:** `internal/orchestra/conductor.go:120` (`UpdateConfig`)
- **Detay:** `UpdateConfig` herhangi bir rol yapılandırmasını doğrulama olmadan kabul eder. Geçersiz bir chief modeli veya eksik rol modeli, çalışma zamanında hataya neden olur.

### H10. Agent Pipeline'da Araç Çağrısı Başına Zaman Aşımı Yok
- **Dosya:** `internal/agent/pipeline.go:122-222`
- **Detay:** Bireysel araç çağrılarında pipeline tarafından zorlanan zaman aşımı yok. Asılı kalan bir `run_command` tüm pipeline'ı sandbox'ın 60s zaman aşımına rağmen süresiz bloke edebilir (pipeline bunu zorlamaz).

### H11. Agent Denetim Günlüğü 1000 Girdiyle Sınırlı
- **Dosya:** `internal/agent/executor.go:40-45`
- **Detay:** `logEntries` slice'ı 1000 ile sınırlı. Eski girdiler sessizce atılır. Döndürme veya kalıcılık yok.

### H12. Flutter: Provider Test ve Mobil Uç Nokta Eksiklikleri
- **Dosya:** `mobile/lib/core/api_client.dart`
- **Detay:** Mobil API istemcisi backend uç noktalarının çoğunu içermez: `sendFileStream`, `exportChat`, `generateTitle`, `updateMessage`, `deleteMessage`, `getSystemPrompt`, memory ayarları, model arama/indirme, WhatsApp, sync, remote access, backup/restore, recording, image.

### H13. `StartLocalModel`/`StopLocalModel`'de `a.client` Veri Yarışı
- **Dosya:** `app.go` (`StartLocalModel`, `StopLocalModel`)
- **Detay:** `a.client` (llama.cpp API istemcisi) eşzamanlama olmadan yeniden atanır. `clientMu` mevcut olsa da, streaming istekleri sırasında istemci değiştirilirse eski istemci referansı ile yeni istekler gönderilebilir.
- **Risk:** Model değiştirme sırasında beklenmeyen hatalar.

### H15. `Sandbox.ValidatePath` Korumasız Mutlak Yollara İzin Veriyor
- **Dosya:** `internal/agent/sandbox.go:88-112`
- **Detay:** Mutlak yol (`filepath.IsAbs(targetPath) == true`) ve yol `ProtectedPaths` dışındaysa `Sandbox.ValidatePath` `nil` (izin verildi) döndürüyor. `/tmp`, `~/.ssh`, `~/Documents` gibi dizinler ProtectedPaths'te yok. Bu metod gelecekte yeni araç entegrasyonlarında çağrılırsa LLM bu yollara yazabilir.
- **Risk:** `/tmp/cron_job`, `~/.ssh/authorized_keys` gibi dosyaların agent tarafından oluşturulması.

### H18. Orchestra Paralel Görev Sonuçları `results[idx]` Race Detector'ı Tetikliyor
- **Dosya:** `internal/orchestra/conductor.go:392-402` (executeParallel)
- **Detay:** Her goroutine farklı bir `results[idx]`'e yazsa da Go bellek modeli ve race detector, `wg.Wait()` öncesinde happens-before ilişkisi kurmadığı için bu yazmaları race olarak işaretler. Planner'dan gelen bozuk plan JSON'u iki göreve aynı `idx` atarsa gerçek veri yarışı oluşur.
- **Risk:** `go test -race` hatası; bozuk plan durumunda sessiz veri bozulması.

### H19. Flutter Orchestra Toggle: TOCTOU ve Notifier Bypass
- **Dosya:** `frontend/lib/widgets/chat_input.dart:662-679`
- **Detay:** Toggle butonu `api.getOrchestraConfig()` + `copyWith(enabled: true)` + `api.updateOrchestraConfig()` zincirini doğrudan çağırıyor. `orchestraConfigProvider.notifier`'ın `toggle()` metodunu bypass ediyor. OrchestraConfigDialog aynı anda açıksa dialog'un local `_config`'i ile toggle'ın sunucudan okuduğu config arasında TOCTOU yarışı oluşur. İki `await` arasında `mounted` kontrolü de yok.
- **Risk:** Yarış sonucu beklenmedik enabled/disabled durumu; unmount sonrası `ScaffoldMessenger.of(context)` çökmesi.

### H20. Orkestra Progress Goroutine: ctx.Done() Sonrası 300s Sızıntı
- **Dosya:** `app.go:888-890, 1057-1059`
- **Detay:** `onProgress` callback'inde `ctx.Done()` algılandığında callback erken return yapar. Ancak `RunWithProgress` goroutine'i hâlâ provider'lara istek gönderiyor ve en fazla 300 saniye daha çalışmaya devam ediyor. Eşzamanlı 10 kullanıcı iptal ederse 30 LLM bağlantısı açık kalır.
- **Risk:** Yüksek yük altında birikim goroutine/bağlantı sızıntısı.

### H21. Skill `SetActive()` Tool'ları Register Yerine Unregister Ediyor
- **Dosya:** `internal/skill/manager.go:208-214`
- **Detay:** `SetActive` metodunda yeni aktifleştirilen skill'lerin tool'ları `RegisterTool` ile kaydedilmek yerine `UnregisterTool` ile kaldırılıyor — üstteki deaktivasyon bloğundan copy-paste hatası. Ardından satır 221-228 tüm aktif skill'leri yeniden kaydetmeyi dener ancak bu da gereksiz duplicate registration'a yol açar.
- **Risk:** Skill aktifleştirildiğinde tool'ları kaydedilmez; agent/LLM skill araçlarını kullanamaz. Ara durumda kesilirse tool'lar kayıtsız kalır.
- **Kategori:** Mantık hatası

### H22. Cloud Sync `WaitForAuth` İkinci Çağrıda Sonsuz Bloke
- **Dosya:** `internal/cloudsync/drive.go:112, 490-496`
- **Detay:** `closeAuthDoneLocked()` double-Done kontrolü yapar ancak `authWg` WaitGroup'u sıfırlanırken (satır 112: `dc.authWg = sync.WaitGroup{}`) eski WaitGroup garbage collect edilir. Önceki `WaitForAuth` çağrısı yeni WaitGroup'u beklerse sonsuza kadar bloke olur çünkü yeni WaitGroup hiçbir zaman `Done()` çağrısı almaz.
- **Risk:** İkinci kez auth flow başlatıldığında `WaitForAuth` kalıcı olarak donar.
- **Kategori:** Eşzamanlılık (deadlock)

### H23. SQLite Yazmaları Caller Context İptalini Yok Sayıyor
- **Dosya:** `internal/database/sqlite.go:101-124`
- **Detay:** `execWrite` satır 102'de transaction başlatmak için `db.ctx` (arka plan context) kullanır, **caller'ın context'ini** kullanmaz. Tüm SQL yazma işlemleri (`Write()` üzerinden) caller'ın iptal/zaman aşımını kaybeder. Disk dolu veya kilit çekişmesi durumunda yazma işlemi caller context'i iptal edilse bile bloke kalır.
- **Risk:** Yazma işlemleri askıda kalır; bellek depolama işlemleri iptal edilemez.
- **Kategori:** Eşzamanlılık (context iptali)

### H24. Flutter `AgentEventBus` StreamController Hiç Dispose Edilmiyor
- **Dosya:** `frontend/lib/providers/agent_provider.dart:82-97`
- **Detay:** `StreamController<AgentEvent>.broadcast()` bir `Provider` ile oluşturulur ancak `dispose()` hiçbir zaman çağrılmaz. `AgentEventBus` uygulama ömrü boyunca yaşar ve stream controller asla kapatılmaz. Aboneler (`agentEventStreamProvider` üzerinden) temizlenmez.
- **Risk:** Kalıcı bellek sızıntısı; stream subscriber'ları asla temizlenmez.
- **Kategori:** Bellek sızıntısı

### H25. Mobile Chat Stream Subscription Yeniden Göndermede Sızıntı
- **Dosya:** `mobile/lib/providers/chat_provider.dart:164-243`
- **Detay:** `_api.sendMessageStream().listen(...)` bir `StreamSubscription` döndürür ancak bu hiçbir yerde saklanmaz veya iptal edilmez. `sendMessage` tekrar çağrıldığında eski subscription yetim kalır. `CancelToken` yalnızca HTTP isteği içindir, listener için değil.
- **Risk:** Yetim stream subscription'ları birikir, bellek sızıntısı ve duplicate mesaj işleme.
- **Kategori:** Bellek sızıntısı

### H26. WhatsApp Screen Polling IndexedStack'te Hiç Durdurulamıyor
- **Dosya:** `frontend/lib/widgets/whatsapp_screen.dart:32-34`, `frontend/lib/screens/app_shell.dart:44-52`
- **Detay:** `WhatsAppScreen` `IndexedStack` içinde olduğu için `dispose()` asla çağrılmaz — polling hiç durmaz. `startPolling()` `initState`'te başlar, `stopPolling()` `dispose()`'tedir ancak `IndexedStack` tüm child'ları canlı tutar. `WhatsAppStatusNotifier` da timer-based polling yapar.
- **Risk:** 2-15s aralıklarla sonsuz polling, pil ve ağ tüketimi.
- **Kategori:** Performans (sonsuz polling)

### H27. `handleImage` dataDir Altındaki Tüm Dosyaları Okuyabilir
- **Dosya:** `internal/webserver/handlers_flutter.go:425-457`
- **Detay:** Handler mutlak yolları ve `..` traversali'ni engeller ancak `dataDir` altındaki herhangi bir göreli yola izin verir. `dataDir` = `filepath.Dir(a.cfg.Memory.PersistDir)` = `data/` olduğunda, `data/providers.json` gibi hassas dosyalara erişilebilir. K25 ile birleşince zafiyet genişler.
- **Risk:** Yetkisiz erişim — provider konfigürasyonları ve diğer veri dosyaları okunabilir.
- **Kategori:** Güvenlik (yetkisiz dosya erişimi)

---

## 🔵 Orta

### M01. Flutter: `prefsProvider` `UnimplementedError` Fırlatıyor
- **Dosya:** `frontend/lib/providers/settings_provider.dart:9-11`
- **Detay:** `prefsProvider` `UnimplementedError()` fırlatır. Bu kasıtlıdır (`main()`'de override edilir) ancak `main()` override'ından önce herhangi bir kod yolu erişirse uygulama kafa karıştırıcı bir hatayla çöker.

### M02. Flutter: Dil Değişimi İçin Paralel Bildirim Sistemleri
- **Dosya:** `frontend/lib/core/l10n.dart:8`
- **Detay:** Dil değişimi için Riverpod yanında özel listener pattern kullanılıyor — projenin tek state management prensibini ihlal ediyor.

### M03. Flutter: `app_shell.build()`'de Yan Etki
- **Dosya:** `frontend/lib/screens/app_shell.dart:34-36`
- **Detay:** `_currentIndex = 0` mutable alan ataması `build()` metodu içinde yapılıyor. Bu, Flutter'ın build saflık kurallarını ihlal eder ve `setState` tetiklemez.

### M04. Flutter: `IndexedStack` Gereksiz Provider Çalıştırması
- **Dosya:** `frontend/lib/screens/app_shell.dart:46-57`
- **Detay:** `IndexedStack` ile tüm ekranlar aynı anda canlı tutulur. `ChatScreen`, `AgentScreen`, `ModelStoreScreen` aynı anda aktif provider'lara sahiptir, gereksiz ağ çağrılarına neden olur.

### M05. Flutter: `_StreamingBubble` Gereksiz Zaman Damgası Güncellemesi
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:496`
- **Detay:** `Text(DateTime.now().toIso8601String().substring(11, 16))` her yeniden derlemede çalışır — streaming sırasında her karede zaman damgası güncellenir.

### M06. Flutter: `_showEditDialog` Bertaraf Sonrası Çağrı Riski
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:206-236`
- **Detay:** `_showEditDialog` `TextEditingController` oluşturur ancak dialog iptal aboneliğini iptal etmez. Widget diyalog açıkken bertaraf edilirse (sekme değiştirme), dialog tamamlanma geri çağrısı bertaraf sonrası çalışır.

### M07. Flutter: Chat Sidebar `didUpdateWidget` Dış Düzenleme Çakışması
- **Dosya:** `frontend/lib/widgets/chat_sidebar.dart:177-181`
- **Detay:** `_editController.text` yalnızca `!_isEditing` iken güncellenir. Kullanıcı düzenleme yaparken sohbet başlığı dışarıdan değişirse, kullanıcının düzenlemesi bir sonraki `didUpdateWidget`'da üzerine yazılır.

### M08. Agent Çağrı İzni için Zaman Aşımı Yok
- **Dosya:** `frontend/lib/widgets/chat_input.dart:400-418`
- **Detay:** `_startOpenRouterOAuth` yükleme diyaloğu gösterir ancak zaman aşımı yoktur. API çağrısı asılı kalırsa diyalog sonsuza kadar UI'ı bloke eder.

### M09. Flutter: API anahtarları provider config dialogunda düz metin gösteriliyor
- **Dosya:** `frontend/lib/widgets/provider_config_dialog.dart`
- **Detay:** Provider yapılandırma diyalogu API anahtarlarını `TextField`'da gösterir. Arka planda biri ekranı izliyorsa (screen recording, screenshot, shoulder surfing) API anahtarları ifşa olur.

### M10. Orkestra/Sağlayıcı/Agent için Test Dosyası Yok
- **Dosya:** `internal/provider/`, `internal/agent/`, `internal/orchestra/`
- **Detay:** Üç yeni paket için sıfır unit test (~4700 satır üretim kodu). (Önceki denetimde ~4150 satır, yeni eklemelerle arttı.)

### M11. Orkestra Config Dosyası 0644 İzniyle Yazılıyor
- **Dosya:** `internal/orchestra/conductor.go:114`
- **Detay:** Orkestra config dosyası dünya tarafından okunabilir (`0644`) izinlerle yazılır. API anahtarları içermese de, yapılandırma detaylarını sızdırabilir.

### M12. Agent İzin Dosyası 0644 İzniyle Yazılıyor
- **Dosya:** `internal/agent/permissions.go:229`
- **Detay:** Agent izin dosyası (`permissions.json`) dünya tarafından okunabilir (`0644`) izinlerle yazılır.

### M13. `unsanitizePath`, Repo ID'lerindeki `__`'den `/` Enjekte Edebilir
- **Dosya:** `internal/modelstore/modelstore.go:345`
- **Detay:** HuggingFace repo ID'lerindeki `__` (çift alt çizgi) dosya yolunda `/`'ye dönüştürülür. Kötü niyetli bir repo ID'si (`foo__..__bar`) dizin geçişine neden olabilir.

### M14. Model Otomatik Sınıflandırma Dosya Adına Göre
- **Dosya:** `internal/modelstore/modelstore.go:58-64`
- **Detay:** `isEmbeddingModel` dosya adındaki "embedding" alt dizesini arar. "embedding" kelimesini içeren normal modeller yanlış sınıflandırılır.

### M15. `Filepath.Walk` Hata Yutma
- **Dosya:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:374-376`
- **Detay:** `filepath.Walk` geri çağrısında hata nil olmayan döndürüldüğünde `return nil` yapılır — hatalar sessizce atlanır.

### M16. Gömme İstemcisi Yeniden Başlatma Sonrası Eski Referans
- **Dosya:** `app.go:143-165` (App struct embeddingClient), `app.go:148-149`
- **Detay:** Gömme modeli yeniden başlatıldığında `embeddingClient` referansı güncellenmez. Eski istemci referansı, kapanmış bir bağlantıya işaret edebilir.

### M17. Bellek Depolama: Kullanıcı Mesaj Boyutu Sınırı Yok
- **Dosya:** `internal/memory/store.go` (çeşitli ekleme yolları)
- **Detay:** Herhangi bir boyuttaki kullanıcı mesajı bellek olarak saklanabilir. Gömme modellerinin tipik token sınırları vardır (örn. 512 token). Saklama öncesi kırpma yapılmaz.
- **Risk:** Gömme hatası veya aşırı büyük bellek girişleri.

### M18. Agent Sistem Promptu Bellek Bağlamıyla Şişebilir
- **Dosya:** `internal/identity/identity.go:26-49`
- **Detay:** `BuildSystemPrompt` tüm alınan anıları doğrudan sistem prompt'una ekler. Çok sayıda yüksek benzerlikli anı ile prompt modelin bağlam penceresini aşabilir. Kırpma veya boyut sınırı yoktur.

### M19. Orkestra: Kendine Referans Veren Roller için Döngü Koruması Yok
- **Dosya:** `internal/orchestra/conductor.go` (`callModel`)
- **Detay:** Bir rolün model endpoint'i Memo uygulamasının kendisine işaret ediyorsa (veya başka bir hizmet üzerinden döngü oluşturuyorsa), sonsuz özyineleme mümkündür. Döngü tespiti yoktur.

### M20. Cloud Sync: Kesintiye Uğrayan Yükleme Kısmi Dosya Bırakıyor
- **Dosya:** `internal/cloudsync/drive.go`
- **Detay:** Yükleme yarıda kesilirse, bulut hedefi kısmi/bozuk bir dosya içerir. Hata durumunda temizlik veya kısmi yükleme iptali yapılmaz.

### M21. `handleSendFile`: Yanlış Slice + Eksik Nokta ile MIME Tespiti Bozuk
- **Dosya:** `internal/webserver/server.go:362-365`
- **Detay:** `ext := tmpFilePath[len(tmpFilePath)-4:]` — geçici dosya adının son 4 karakterini alıyor; orijinal dosyanın uzantısını değil. Ayrıca `ext == "jpeg"` koşulunda başında nokta yok, bu koşul hiçbir zaman true olamaz (ext her zaman noktayla başlar). `.jpeg` uzantılı dosyalar hiçbir zaman resim olarak tanımlanamaz.
- **Risk:** Kullanıcı `.jpeg` dosyası gönderince resim yerine generic dosya olarak işleniyor.

### M22. `handleWhatsAppSearch`: Boş Sorgu Tüm Mesajları Döküyor
- **Dosya:** `internal/webserver/handlers_flutter.go:1245-1251`
- **Detay:** `q` parametresi boş olduğunda `WhatsAppSearch("", 50)` çağrılıyor. SQLite LIKE `%%` deseni tüm mesajları eşleştirebilir; en fazla 50 mesaj döner. K04 ile birleşince kimlik doğrulamasız LAN erişimi ile özel sohbet içeriği sızabilir.

### M23. Agent Pipeline: Araç Çağrıları Döngüsünde Context İptali Kontrolü Yok
- **Dosya:** `internal/agent/pipeline.go:125-233`
- **Detay:** Dış iterasyon döngüsünde `ctx.Done()` kontrol ediliyor ancak `for _, tc := range resp.ToolCalls` iç döngüsünde yok. LLM yanıtı 10 araç çağrısı içeriyorsa ve kullanıcı 1. çağrı sonrası iptal ederse kalan 9 çağrı (her biri 60s'e kadar `run_command` çalıştırabilir) tamamlanır.

### M24. `run_command`: Zaman Aşımı Hatası Yanlış Context Kontrol Ediyor
- **Dosya:** `internal/agent/tools/command.go:130-137`
- **Detay:** `if ctx.Err() == context.DeadlineExceeded` koşulu `execCtx`'i değil `ctx`'i (parent context) kontrol ediyor. 60s tool timeout (`execCtx`) dolduğunda `ctx` hâlâ geçerliyse hata mesajı "Command timed out" yerine "Command failed with error: ..." oluyor.

### M27. Orchestra `UpdateConfig` ve `Config()` Arasında TOCTOU (Kaydetme Yarışı)
- **Dosya:** `app.go` (~1904-1914), `internal/orchestra/conductor.go:41-56`
- **Detay:** `UpdateConfig(cfg)` mutex alıp bırakır, ardından `Config()` tekrar mutex alır. İki çağrı arasında başka bir goroutine `UpdateConfig` çağırırsa kaydedilen config son set edilenden farklı olabilir. Restart'tan sonra tutarsız config yüklenir.

### M26. Orchestra: HTTP 503 Rate-Limit Olarak Muamele Ediliyor
- **Dosya:** `internal/orchestra/conductor.go:809`
- **Detay:** `isRateLimitError` içinde `strings.Contains(err.Error(), "503")` kontrolü var. 503 Service Unavailable gerçek bir servis kesintisidir, rate-limit değil. Bu durumda `callWithRetry` çöken servisi 3+ kez dener (5+10+20s bekleme), sağlıklı provider'a geçiş gecikir.

### M27. `retryTask` Rate-Limit'te `callWithRetry`'ı Çağırarak Deneme Sayısını İkiye Katlıyor
- **Dosya:** `internal/orchestra/conductor.go:561`
- **Detay:** Rate-limit hatası alındığında `retryTask` direkt `callWithRetry(fn)` çağırıyor. `callWithRetry` kendi içinde 3 deneme daha yapar. `retryTask`'ın kendi döngüsü de devam eder. Sonuç: API halihazırda rate-limit atarken 6 toplamı deneme yapılır.

### M28. Flutter: `orchestraConfigProvider.build()` Hataları Yutarak Varsayılan Config Döndürüyor
- **Dosya:** `frontend/lib/providers/orchestra_provider.dart:15-21`
- **Detay:** Tüm exception'lar `catch (e)` ile yakalanıp `OrchestraConfig()` (varsayılan, disabled) döndürülüyor. API başlangıçta erişilemezse kullanıcının önceki oturumda aktifleştirdiği orchestra görünmez. Kullanıcı dialog üzerinden kaydederse sunucudaki mevcut yapılandırmanın üzerine default yazılabilir.

### M29. Flutter: `_sendWhatsApp` — Unmount Ortasında `isSendingProvider` Sızdırıyor
- **Dosya:** `frontend/lib/widgets/chat_input.dart:173-210`
- **Detay:** `isSendingProvider.state = true` set edildikten sonra widget unmount edilirse stream tamamlanma callback'lerindeki `state = false` çağrılmaz. Uygulama "gönderiliyor" durumunda kalıcı olarak donar, yeniden başlatma gerekir. `try/finally` bloğu eksik.

### M30. Flutter: WhatsApp `connect()` Yükleme Durumu Göstermiyor — Eşzamanlı Çağrı Riski
- **Dosya:** `frontend/lib/providers/whatsapp_provider.dart:110-120`
- **Detay:** `state = const AsyncValue.loading()` kaldırıldı. Önceki state hata ise kullanıcı hata ekranını görmeye devam ederken bağlantı denenebilir, butona tekrar basılabilir. Eşzamanlı iki `connect()` çağrısı yarışa girebilir.

### M31. Skill Instructions Sistem Promptuna Sanitizasyon Olmadan Enjekte Ediliyor
- **Dosya:** `app_skill.go:196-200`, `app.go:574-585`
- **Detay:** `def.Instructions` (kullanıcı tarafından kontrol edilen SKILL.md markdown gövdesi) mevcut sistem promptuna doğrudan ekleniyor. Kötü niyetli bir skill instructions içinde `ignore all previous instructions` tarzı prompt injection içerebilir. Uzunluk sınırı, karakter doğrulaması veya escape mekanizması yok.
- **Risk:** Prompt injection ile AI davranışının ele geçirilmesi.

### M32. YAML Front Matter Ayrıştırıcısı: `---` Bulunan Skill Body'si Front Matter'ı Keser
- **Dosya:** `internal/skill/loader.go:98-104` (extractFrontMatter)
- **Detay:** `second := strings.Index(rest, frontMatterDelim)` YAML bloğundan sonraki tüm içerikte `---` arar. Skill'in markdown body'sinde yatay çizgi (`---`) kullanılırsa YAML front matter erken kapatılır, geri kalan kısım yanlış yorumlanır veya kaybolur.

### M33. `/skill:off` (Tümünü Kapat): `SetActive(nil)` Hata Dönüşü Yutulуyor
- **Dosya:** `app_skill.go:152`
- **Detay:** `a.skillManager.SetActive(nil)` çağrısı `error` döndürür ancak dönen değer görmezden geliniyor. Başarısız deaktivasyonda kullanıcı "Tüm skill'ler devre dışı bırakıldı" mesajını görür ama işlem gerçekleşmeyebilir.

### M34. Agent Backup ID Çakışması: `UnixNano` Düşük Çözünürlüklü Saatlerde Çakışabilir
- **Dosya:** `internal/agent/backup.go:86`
- **Detay:** Yedek dosya adı `time.Now().UnixNano()` ile oluşturuluyor. VM'lerde ve bazı Linux çekirdeklerinde `time.Now()` çözünürlüğü ~1ms düzeyinde olabilir. Hızlı ardışık yedeklemede aynı timestamp üretilirse ikinci yedek birincinin üzerine yazılır. Geri alınamaz veri kaybı.

### M35. Agent Permission Channel HTTP Handler'ı Sonsuz Bloke Ediyor
- **Dosya:** `internal/agent/executor.go:145`
- **Detay:** `resCh := make(chan PermissionPolicy, 1)` oluşturulup `HandlePermissionResponse`'ta `req.ResCh <- policy` ile gönderilir. `waitFn` context iptali veya timeout nedeniyle döndüyse kanalı okuyan goroutine kalmaz. Buffer 1 olmasına rağmen `waitFn` zaten `select`'ten çıkmışsa, `HandlePermissionResponse`'daki send sonsuza kadar bloke olur. Bu bir HTTP handler goroutine'idir.
- **Risk:** Permission timeout sonrası kullanıcı yanıt verirse HTTP handler goroutine'i kalıcı olarak bloke olur — goroutine sızıntısı.
- **Kategori:** Eşzamanlılık (goroutine sızıntısı)

### M36. Memory Store `COUNT(*)` Sorgu Hatası Sessizce Yutuluyor
- **Dosya:** `internal/memory/store.go` (yaklaşık satır 530)
- **Detay:** `s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memories").Scan(&stats.Count)` çağrısında `Scan`'den dönen hata kontrol edilmez. Veritabanı bozuk veya kapalıysa `stats.Count` varsayılan değeri 0 kalır, hata bildirilmez.
- **Risk:** UI "0 anı" gösterirken aslında veritabanı bozuk olabilir — sessiz veri kaybı göstergesi.
- **Kategori:** Hata işleme

### M37. API Streaming `body.Close()` Double-Close Race Condition
- **Dosya:** `internal/api/streaming.go:47, 55`
- **Detay:** `defer body.Close()` satır 47'de `processSSEStream` döndüğünde çalışır. Watcher goroutine'i (satır 53-56) context iptalinde `body.Close()` çağırır. Deferred close ile watcher goroutine'i arasında yarış vardır. Double-close bazı `io.ReadCloser` implementasyonlarında panic veya tanımsız davranışa yol açabilir.
- **Risk:** Nadiren de olsa panic — HTTP yanıt body'sinin çift kapatılması.
- **Kategori:** Eşzamanlılık (race condition)

### M38. Agent Screen Permission Dialog Back Button ile Bypass Edilebiliyor
- **Dosya:** `frontend/lib/screens/agent_screen.dart:178`
- **Detay:** Agent permission dialog `barrierDismissible: false` ile gösterilir ancak `PopScope(canPop: false)` kullanılmamış. Masaüstü chat screen (chat_screen.dart:67-70) `canPop: false`'u doğru şekilde ayarlarken agent screen'de bu koruma yok.
- **Risk:** Kullanıcı back button ile dialog'u kapatabilir, agent pipeline perm response'u sonsuz bekler.
- **Kategori:** Kullanıcı deneyimi

### M39. Model Store Her Görünümde 3+ Ayrı `Dio()` Örneği Oluşturuyor
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:850, 1204, 1232`
- **Detay:** `_loadFiles()` paylaşılan `apiClientProvider` Dio'sunu kullanırken, `_loadReadme()`, `_loadMoreModels()` ve `_AuthorAvatarState._resolve()` her biri kendi `Dio()` örneğini oluşturur. Farklı timeout ve konfigürasyonlara sahiptirler.
- **Risk:** Gereksiz kaynak kullanımı; connection pooling yok; tutarsız timeout davranışı.
- **Kategori:** Performans

### M40. `_delayedRefreshTimer` Provider Dispose Sonrası Ateşlenebilir
- **Dosya:** `frontend/lib/providers/chat_provider.dart:314-317`
- **Detay:** Mesaj gönderildikten sonra 2 saniyelik `Timer` kurulur. `ref.onDispose` timer'ı iptal eder ancak `stopStreaming()` ile `onDispose` arasında timer ateşlenip `ref.invalidate(chatListProvider)` çağırabilir. `ref` o noktada geçersiz olabilir.
- **Risk:** Provider disposal sonrası `StateError` alma riski.
- **Kategori:** Widget lifecycle

### M41. WhatsApp `_msgTimer` Polling'inde Error Backoff Yok
- **Dosya:** `frontend/lib/screens/whatsapp_screen.dart:46-53`
- **Detay:** Her 5 saniyede bir polling yapar. Ağ kesintisinde bile tam hızda polling devam eder — backoff veya bekleme yok.
- **Risk:** Mobilde pil tüketimi; gereksiz ağ trafiği.
- **Kategori:** Performans

### M42. Settings `didUpdateWidget` Kullanıcı Düzenlemesinin Üzerine Yazıyor
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:2526-2533`
- **Detay:** `_controller.text = widget.value.toString()` `didUpdateWidget` içinde kullanıcının o an düzenleme yapıp yapmadığı kontrol edilmeden çağrılır. Kullanıcı yazarken provider güncellenirse kullanıcının girdisi silinir.
- **Risk:** Kullanıcı input kaybı.
- **Kategori:** Kullanıcı deneyimi

### M43. Dio Stream `timeout` İptal Sonrası Error Ekleyebilir
- **Dosya:** `frontend/lib/providers/chat_provider.dart:227-233, 369-374`
- **Detay:** `stream.timeout(onTimeout: (sink) => sink.addError(...))` timeout'ta sink'e error ekler. Ancak `CancelToken` timeout'tan önce iptal edilmişse, error zaten kapatılmış bir stream'e eklenmeye çalışılır — unhandled exception.
- **Risk:** Stream işlemede yakalanamayan hatalar.
- **Kategori:** Eşzamanlılık

### M44. Flutter Model Status Polling Engine Strip Nedeniyle Hiç Durmuyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:35-59`
- **Detay:** `StreamProvider.autoDispose` kullanılmasına rağmen uygulamanın altındaki engine strip widget'ı `modelStatusProvider`'ı sürekli izler. Bu nedenle `autoDispose` hiçbir zaman tetiklenmez — her 5 saniyede bir polling devam eder.
- **Risk:** Gereksiz HTTP isteği — uygulama hayatı boyunca her 5 saniyede bir.
- **Kategori:** Performans

### M45. Flutter `connectionStatusProvider` 30 Saniyede Bir Sonsuz Polling
- **Dosya:** `frontend/lib/providers/chat_provider.dart:464-474`
- **Detay:** `StreamProvider<bool>` tüm uygulama ömrü boyunca her 30 saniyede bir polling yapan `while(true)` döngüsü çalıştırır. `autoDispose` kullanılmamış. Sidebar kapalıyken bile çalışır.
- **Risk:** Uygulama hayatı boyunca durmayan ağ istekleri.
- **Kategori:** Performans (sonsuz polling)

---

## ⚪ Düşük

### L01. `api/streaming.go` `scanner.Err()` Kontrolü Mevcut
- **Dosya:** `internal/api/streaming.go:127-129`
- **Not:** Mevcut durumda `scanner.Err()` kontrol ediliyor — bu bir hata DEĞİL, sadece doğrulama notu.

### L02. Provider Router Hızlı Geri Dönüşte Goroutine Birikmesi
- **Dosya:** `internal/provider/router.go`
- **Detay:** Geri dönüş zincirinde her zaman aşımına uğramış sağlayıcı bir sonraki deneme için goroutine başlatır. Tüm sağlayıcılar hızlıca başarısız olursa, kısa ömürlü goroutine'ler birikir.

### L03. Oturumlar: Yüklü Oturum Sayısı Sınırı Yok
- **Dosya:** `internal/sessions/sessions.go`
- **Detay:** Tüm oturumlar başlangıçta belleğe yüklenir. Yüzlerce büyük mesaj geçmişi olan oturum önemli bellek tüketebilir.

### L04. SQLite `MaxOpenConns` Varsayılan (Sınırsız)
- **Dosya:** `internal/database/sqlite.go:50-53`
- **Detay:** SQLite WAL modunda birden çok okuyucuyu işleyebilir ancak `SetMaxOpenConns` açıkça ayarlanmazsa varsayılan 0'dır (sınırsız). `MaxPool` 1 olarak ayarlansa da, alt seviye `sql.DB` için bu geçerli değildir.

### L05. `doDownload` Geçici Dosya Temizleme Mantığı Kırılgan
- **Dosya:** `internal/modelstore/modelstore.go:255-262`
- **Detay:** Ertelenmiş (`defer`) temizleme, `os.Rename` başarılı olduktan sonra bile `os.Remove(tmpPath)` çağırır (rename sonrası `tmpPath` artık yok, hata sessizce yutulur).

### L06. `ngrok/installer.go` Arşiv Çıkarma Güvenliği Yok
- **Dosya:** `internal/ngrok/installer.go:93-122, 124-154`
- **Detay:** TGZ ve ZIP çıkarma, hedef dosyanın zaten var olup olmadığını kontrol etmez. Eşzamanlı indirmeler dosyaları bozabilir.

### L07. ESLint: `unawaited(future)` Uyarısı
- **Dosya:** `frontend/lib/widgets/agent/permission_dialog.dart:51`
- **Detay:** `unawaited(future)` — `handleAgentPermission`'dan dönen `Future` açıkça beklenmez. İstek başarısız olursa hata sessizce kaybolur.

### L08. Flutter: `ThemeColors.lerp()` Null-forgiving Operatör Riski
- **Dosya:** `frontend/lib/core/theme.dart:65-80`
- **Detay:** `Color.lerp` null-forgiving `!` operatörü kullanır. `other` parametresi null değilse bile alanları null olabilir, bu durumda `!` çöker.

### L09. Flutter: `version_banner.dart` `addPostFrameCallback` Her Yeniden Derlemede
- **Dosya:** `frontend/lib/widgets/version_banner.dart:115-119`
- **Detay:** `WidgetsBinding.instance.addPostFrameCallback` `build()` içinde çağrılır. Her yeniden derlemede yeni bir post-frame callback kaydedilir. `_AnimatedBanner` bir `StatelessWidget` olduğundan, parent her yeniden derlediğinde gereksiz callback'ler birikir.

### L10. Mobil: Varsayılan URL Ev Ağı IP'si Sızdırıyor
- **Dosya:** `mobile/lib/core/api_client.dart:68`
- **Detay:** Varsayılan URL `http://192.168.1.100:8090` olarak sabit kodlanmış — geliştiricinin iç ağ topolojisini sızdırır.

### L11. `unsanitizePath` 64-bit `int` Taşması Riski
- **Dosya:** `internal/modelstore/modelstore.go` (ilgili satır)
- **Detay:** Repo ID'lerinde kullanılan 64-bit tamsayı dönüşümleri, çok büyük sayılar için taşabilir.

### L12. `handleImport`: Tüm İstek Gövdesi Tek Seferde Heap'e Alınıyor
- **Dosya:** `internal/webserver/handlers_flutter.go:182-195`
- **Detay:** `io.ReadAll(r.Body)` ile 50MB'a kadar istek tek bir `[]byte` olarak heap'e alınıyor. Eş zamanlı birden çok büyük import isteği OOM'a yol açabilir.

### L13. `Server.Stop()`: Port Serbest Kalmadan `running = false` Setleniyor
- **Dosya:** `internal/webserver/server.go:249-262`
- **Detay:** `go srv.Shutdown(context.Background())` goroutine olarak ateşlenip unutuluyor. `running` hemen false olur. Aynı porta hemen tekrar `Start()` çağrılırsa "address already in use" hatası alınır.

### L14. `readEnv` Araçı: Maskelenen Gizli Değerler Ama Anahtar İsimleri Görünüyor
- **Dosya:** `internal/agent/tools/search.go:87-119`
- **Detay:** Hassas env değişkenleri `MY_KEY=********` olarak gösterilir. Anahtar isimleri (örn. `OPENAI_API_KEY`, `GITHUB_TOKEN`) log'lara ve frontend'e sızar — hangi credential'ların sistemde tanımlı olduğunu açığa çıkarır.

### L15. WhatsApp: Başlatılmış Ama Giriş Yapılmamış Durumda 2s Polling
- **Dosya:** `frontend/lib/providers/whatsapp_provider.dart:88-90`
- **Detay:** `!status.loggedIn` koşulu artık QR bekleme VE aktif olmayan-başlatılmış durumu kapsar. Backend başlatıldıktan sonra kullanıcı connect etmeden de 2s polling başlar, sunucu ve pil yükü artar.

### L16. Orchestra `ProgressTaskChunk`: Her Kelimeyi Role Prefix'iyle Gönderme UX'i Bozuyor
- **Dosya:** `app.go:901-907`, `internal/orchestra/conductor.go:466-472`
- **Detay:** Her stream chunk'ı `"**planner**: kelime"` formatında gönderiliyor. 1000 kelimelik görev çıktısı 1000 ayrı `**rol**: kelime` parçası haline gelir. Paralel görevlerin iç içe geçmiş markdown'ı UI'da okunamaz hale gelir.

### L17. Orchestra `ProgressTaskDone`: Megabaytlık İçerik Struct'ta Değer Olarak Kopyalanıyor
- **Dosya:** `internal/orchestra/conductor.go:480-491`
- **Detay:** `ProgressUpdate{..., Content: sb.String()}` ile görev yanıtının tamamı struct değeri olarak `safeProgress`'e geçiriliyor. 50KB'lık bir görev çıktısı her `ProgressTaskDone` çağrısında heap'te bir kez daha kopyalanır.

### L18. `findProviderConfig`: Loop Değişkeninin Adresi Döndürülüyor
- **Dosya:** `internal/orchestra/conductor.go:169-204`
- **Detay:** `for _, cfg := range configs { return &cfg }` — `cfg` döngü yerel değişkenidir. Go escape analysis bunu heap'e taşır, bu yüzden çalışır. Ancak `pCfg.Model = modelName` ile provider config'ini değiştirmek, orijinal config'i değil heap'teki kopyanın bir alanını değiştirir. Anlam belirsizliği ve bakım zorluğu.

### L19. Flutter: `WhatsAppChatModeNotifier.init()` — Dispose Sonrası State Yazma
- **Dosya:** `frontend/lib/providers/whatsapp_provider.dart:37-43`
- **Detay:** `init()` async metodu, `await` sonrasında provider dispose edilmiş olabilir. `StateNotifier.state` dispose sonrası set edildiğinde debug modda assertion hatası, release modda tanımsız davranış.

### L20. Skill `/skill:off <name>`: Aktif Olmayan Skill İçin "Başarılı" Mesajı
- **Dosya:** `app_skill.go:156-166`
- **Detay:** Deaktive edilmek istenen skill aktif değilse `remaining` listesi değişmez, `SetActive(remaining)` çağrısı başarılı olur, kullanıcı "✅ Skill X deactivated" mesajını görür. Ancak X hiç aktif değildi, gerçek bir işlem yapılmadı.

### L21. Skill `/skill:off <name>`: Read-Modify-Write'ta TOCTOU Yarışı
- **Dosya:** `app_skill.go:156-163`
- **Detay:** `GetActiveNames()` (RLock alıp bırakır) ile `SetActive(remaining)` (WLock alır) arasında başka bir goroutine aktif seti değiştirirse stale `remaining` listesi yeni durumun üzerine yazılır. Atomik `Deactivate(name)` metodu yok.

### L22. Skill `SetActive`: `RegisterTool` Hata Dönüşü Yutulуyor
- **Dosya:** `internal/skill/manager.go:176`
- **Detay:** `m.toolRegistrar.RegisterTool(...)` `error` döndürür ancak hiç kontrol edilmiyor. Kayıt başarısız olursa `SetActive` `nil` (başarı) döndürür, tool sessizce çalışmaz.

### L23. Skill `Discover()`: Tüm Disk I/O Süresince Write Lock Tutuluyor
- **Dosya:** `internal/skill/manager.go:40-60`
- **Detay:** `m.mu.Lock()` alındıktan sonra `DiscoverSkills()` çağrılıyor; bu metod `os.ReadDir` + her skill için `os.ReadFile` yapıyor. Tüm bu süre boyunca tüm `List`, `Get`, `IsActive`, `ActiveInstructions` çağrıları bloke olur.

### L24. Skill `handleSkillCommand`: `ctx` Parametresi Kullanılmıyor
- **Dosya:** `app_skill.go:64`
- **Detay:** `context.Context` parametresi kabul ediliyor ama hiç kullanılmıyor. Context iptali (kullanıcı navigasyonu) görmezden geliniyor.

### L25. Skill `ParseSkill`: Name Doğrulaması Instructions Kontrolünden Sonra
- **Dosya:** `internal/skill/loader.go:43-55`
- **Detay:** `manifest.Name == ""` kontrolü satır 53'te, instructions kontrolü satır 49'da yapılıyor. Her ikisi de boşsa hata mesajı `skill "" has no instructions` — boş name sessizce format string'e gömülüyor.

### L26. `skills-lock.json` Hash'leri Hiç Doğrulanmıyor
- **Dosya:** `skills-lock.json`, `internal/skill/loader.go`
- **Detay:** `skills-lock.json`'da `computedHash` alanları var ama hiçbir Go kodu bu dosyayı okuyup hash'i doğrulamıyor. Değiştirilen bir SKILL.md checksum uyumsuzluğu algılanmadan yüklenir — lock dosyası yanlış güvenlik hissi veriyor.

### L27. Skill `LoadSkill` / `copyDir`: Boyut Sınırı Olmadan Dosya Okuma
- **Dosya:** `internal/skill/loader.go:17`, `internal/skill/manager.go:242`
- **Detay:** `os.ReadFile` boyut sınırı olmadan tüm dosyayı okur. Yüzlerce MB'lık bir SKILL.md veya skill dizinindeki büyük dosya process belleğini tüketebilir.

### L28. `ExportData` Dosya Tanımlayıcıları Hata Durumunda Kapatılmıyor
- **Dosya:** `app.go:3551-3608`
- **Detay:** `addFile` helper'ı `os.Open` ile dosya açar ancak `defer f.Close()` bir döngü içindedir — tüm deferred `Close` çağrıları birikir ve ancak `addFile` döndüğünde çalışır. Walk callback'i de aynı soruna sahiptir. Çok sayıda dosya işlenirken tüm file handle'lar açık kalır.
- **Risk:** Büyük export'larda file descriptor tükenmesi.

### L29. `EventRing.snapshot()` Buffer Copy Mantığı Kırılgan
- **Dosya:** `app.go:138-140`
- **Detay:** `snapshot()` circular buffer'dan veri kopyalarken `copy(out, r.buf[r.pos:])` ve `copy(out[len(r.buf)-r.pos:], r.buf[:r.pos])` kullanır. Bu mantık yalnızca `r.pos` belirli değerlerdeyken doğru çalışır. `r.pos=0` ve `r.full=true` durumunda `n=64` olmasına rağmen ilk copy 64 eleman kopyalar, ikinci copy 0 eleman kopyalar — bu da doğrudur ancak farklı `pos` değerlerinde hatalı kopyalama yapılabilir.

### L30. `memorySaveWorker` `context.Background()` Kullanıyor
- **Dosya:** `app.go:3441`
- **Detay:** `memorySaveWorker` `a.saveMemorySync(context.Background(), ...)` ile çağrılır — `a.ctx` yerine taze bir background context. 10 saniyelik timeout `saveMemorySync` içinde uygulansa da, worker uygulama seviyesi iptalini yok sayar.
- **Risk:** Shutdown sonrası kısa süreli memory save devam eder.

### L31. `Server.Stop()` `context.Background()` Kullanıyor
- **Dosya:** `internal/webserver/server.go:269`
- **Detay:** `srv.Shutdown(context.Background())` timeoutsuz background context kullanır. Sunucu aktif bağlantılarla aşırı yüklüyse `Shutdown` sonsuza kadar bloke olabilir.

### L32. Flutter: `_delayedRefreshTimer` Provider Dispose Sonrası StateError Riski
- **Dosya:** `frontend/lib/providers/chat_provider.dart:314-317`
- **Detay:** Mesaj gönderildikten 2sn sonra ateşlenen timer, `ref.onDispose` ile iptal edilir ancak `stopStreaming()`-`onDispose` arasındaki pencerede `ref.invalidate` çağırabilir.

### L33. Flutter: `contextMenu` Pozisyonu Scroll Sonrası Hatalı
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:302-304`
- **Detay:** `_tapPosition` `details.localPosition` ile saklanır ancak `showMenu` global koordinat bekler. `localToGlobal` dönüşümü, down ile tap arasında scroll olursa hatalı pozisyon verir.

### L34. Flutter: `provider_config_dialog.dart` `_save()`'de `_saving` Guard Yok
- **Dosya:** `frontend/lib/widgets/provider_config_dialog.dart`
- **Detay:** `_save()` double-submission korumasına sahip değil. Kullanıcı "Save" butonuna hızlıca iki kez basabilir. Kaydetme sırasında loading göstergesi de yok.

### L35. Flutter: `unawaited(future)` Uyarıları (Birden Çok Yerde)
- **Dosya:** `frontend/lib/widgets/agent/permission_dialog.dart:51`, `frontend/lib/providers/agent_provider.dart`
- **Detay:** `handleAgentPermission`'dan dönen `Future` `unawaited` ile çağrılır. İstek başarısız olursa hata sessizce kaybolur.

---

## ⚫ Bilgi / Gözlemler

### I1. `App.ctx` struct alanında saklanıyor (anti-pattern)
- **Dosya:** `app.go:227`
- **Not:** Go context dokümantasyonuna göre context'ler struct'ta saklanmamalı, fonksiyonlara parametre olarak geçirilmeli.

### I2. Eski GOB Formatı (SQLite'e taşındı)
- **Dosya:** `internal/memory/store.go`

### I3. Etkileşim Başına Tek Dosya Tasarımı
- **Dosya:** `internal/memory/store.go`
- **Not:** Bellek depolama hala eski dosya başına-etkileşim modelini destekliyor.

### I4. Flutter: Hardcoded Türkçe string'ler L10n'i bypass ediyor
- **Dosya:** `frontend/lib/widgets/agent/permission_dialog.dart:149`, `frontend/lib/widgets/agent/agent_chat_card.dart:19,25,30,75,81`, `frontend/lib/widgets/permission_history.dart`, `chat_input.dart`
- **Not:** Çeşitli widget'larda hala sabit kodlanmış Türkçe metinler var, L10n sistemi bypass ediliyor.

### I5. Flutter: `const` constructor'lar eksik (yaygın)
- **Not:** Tüm Flutter kod tabanında yaygın olarak `const` constructor'lar eksik. Her yeniden derlemede yeni nesne oluşturulması GC baskısını artırır.

### I6. Llama Sunucusu Stderr'i Uygulama Loglarıyla Karışıyor
- **Dosya:** `internal/llama/llama.go:125-126`
- **Not:** `s.cmd.Stderr = os.Stderr` — llama.cpp hata çıktısı uygulama loglarına karışır, ayrı bir log dosyasına yönlendirilmez.

### I7. Bellek Depolama Tam Yeniden Derleme
- **Dosya:** `internal/memory/store.go`
- **Not:** `LoadCache` O(N) karmaşıklığındadır, artımlı indeksleme yoktur. Başlangıç süresi bellek sayısıyla doğrusal artar.

### I8. `internal/models/memory.go` ve `internal/memory/store.go` Çifte Tip Tanımı
- **Dosya:** `internal/models/memory.go`, `internal/memory/store.go`
- **Not:** `models.MemoryResult` ve `memory.MemoryResult` aynı amaca hizmet eden çifte tip tanımlarıdır. Tek bir tipte birleştirilmeli.

### I9. `stt_proc_windows.go`: `sttSetProcessGroup` No-Op
- **Dosya:** `stt_proc_windows.go:7`
- **Not:** Windows'ta `sttSetProcessGroup` boştur, `sttKillProcessGroup` yalnızca ana process'i öldürür, çocuk process'ler hayatta kalır.

### I10. Provider `defaultMachineKey()` Sabit Kodlanmış Geri Dönüş Anahtarı
- **Dosya:** `internal/provider/config.go:380`
- **Not:** Donanım kimliği alınamazsa sabit kodlanmış `"Mm3m0L0c4lK3y!@#$%^&*()9876543210"` anahtarı kullanılır. Bu, API anahtarlarının çözülebileceği anlamına gelir.

### I11. Cloud Sync: Donanım Kimliği Düşüşü Sabit Kodlanmış
- **Dosya:** `internal/cloudsync/crypto.go:145`
- **Not:** Donanım kimliği alınamazsa `"memo-fallback-key"` kullanılır. Sync şifrelemesi temelde bu anahtara güvenir.

### I12. Provider `defaultMachineKey()` `sh` Komutu Çalıştırıyor
- **Dosya:** `internal/provider/config.go:371-377` (macOS yolu)
- **Not:** Donanım kimliği için `sh -c "ioreg ..."` çağırılır — bu, `sh` binary'sinin mevcut olduğunu varsayar ve minimal ortamlarda başarısız olabilir.

### I13. `patch_interfaces.go` Sadece 3 Satır — Görünüşte Gereksiz
- **Dosya:** `patch_interfaces.go`
- **Not:** Dosya yalnızca `package main` ve yorum içerir. Görünüşe göre bir script için dummy dosya.

### I14. `go 1.25.0` — Henüz Yayınlanmamış Go Sürümü
- **Dosya:** `go.mod:4`
- **Not:** `go 1.25.0` henüz yayınlanmamıştır (mevcut kararlı sürüm: 1.24.x). Bu, derleme sorunlarına neden olabilir.

### I15. Agent WhatsApp Aracı AppBridge'i By-Pass Ediyor
- **Dosya:** `internal/agent/tools/whatsapp.go`
- **Not:** WhatsApp aracı, ana uygulama bridge'i yerine doğrudan WhatsApp deposuna erişir. Bu, App katmanındaki erişim kontrollerini ve denetim günlüğünü atlar.

### I16. `skill/manager.go` SetActive: Yeni Skill Tool'larını Gereksiz Unregister Ediyor
- **Dosya:** `internal/skill/manager.go:159-166`
- **Not:** Yeni aktive edilen skill'lerin toolları önce `UnregisterTool` ile kaldırılıp hemen ardından alt blokta `RegisterTool` ile ekleniyor. Orta blok yanlış: `RegisterTool` yerine `UnregisterTool` çağrılıyor. Final blok doğru kayıt yapıyor ama ara durum gereksiz kaldırma+ekleme döngüsü oluşturuyor. Eğer final blok herhangi bir nedenle (panic vb.) çalışmadan kesilirse tool'lar kayıttan çıkmış kalır.

### I17. `config/config.yaml` Kayıtlı: `active_provider: openai` Hardcoded
- **Dosya:** `config/config.yaml:63`
- **Not:** `active_provider: openai` değeri config dosyasına commit edilmiş. Bu değer sunucu başlangıcında okunursa kullanıcının önceki tercihini (örn. `claude`) geçersiz kılabilir.

### I18. `skill.DangerLevel` ve `agent.DangerLevel` Çifte Tip — Tip Uyumsuzluğu
- **Dosya:** `internal/skill/types.go:7` vs `internal/agent/tools.go:13`
- **Not:** İki paket aynı string değerlerine sahip ayrı `DangerLevel` named type'ları tanımlıyor. `SkillTool.DangerLevel` (`skill.DangerLevel`) agent pipeline'ındaki `agent.DangerLevel` tip assertion'larıyla uyumsuz. Her iki paketi kullanan kod compile sürecinde type mismatch yaşar. Ortak bir `internal/common` paketi önerilebilir.

### I19. Orchestra Conductor Hardcoded 300s Timeout
- **Dosya:** `internal/orchestra/conductor.go:249, 478, 657`
- **Not:** Tüm orchestra alt-operasyonları aynı hardcoded 300s timeout'u kullanır. Bu değerler yapılandırılabilir olmalı.

### I20. `memory/store.go` Latency Log Mesajı Hatalı
- **Dosya:** `internal/memory/store.go:269-275`
- **Not:** `time.Since(embedStart.Add(writeDur)).Milliseconds()` ifadesi yanlış — `embedStart.Add(writeDur)` embedStart'ten daha eski bir zaman verir, bu nedenle `time.Since` gerçekte olduğundan daha büyük bir değer döndürür. Sadece logging bug'ı.

### I21. `api/streaming.go` `scanner.Err()` Doğrulandı
- **Dosya:** `internal/api/streaming.go:127-129`
- **Not:** `scanner.Err()` döngü sonrası kontrol ediliyor — bu bir hata DEĞİL, doğrulama notu.

### I22. Agent Pipeline'da 20 İterasyon Hardcoded
- **Dosya:** `internal/agent/pipeline.go:63/78`
- **Not:** `maxIters: 20` sabit kodlanmış. Karmaşık görevlerde yetersiz kalabilir.

### I23. Sessions Her Mesajda Diske Senkron Yazıyor
- **Dosya:** `internal/sessions/sessions.go:185`
- **Not:** `AddMessage` her çağrıldığında `m.save(s)` ile diske yazar. Streaming yanıtlarda `finishStream` yanıt başına bir kez çağrılır, bu kabul edilebilir. Ancak hızlı ardışık çağrılarda rate limiting veya batch yok.

### I24. WhatsApp Store'da Connection Pool Yok
- **Dosya:** `internal/whatsapp/store.go:20`
- **Not:** `sql.Open` doğrudan kullanılır, `database.DB` wrapper'ı üzerinden serialized write yapılmaz. WhatsApp handler'ları ve agent tool çağrılarından eşzamanlı yazmalar "database is locked" hatasına yol açabilir.

---

> **Son güncelleme:** 2026-06-14
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, app_skill.go, tüm internal/ paketleri) + Flutter frontend + mobil frontend + yeni skill sistemi + orchestra sistemi
> **Açık hatalar:** 51 (🔴2, 🟠18, 🔵31, ⚪17)
> **Gözlemler:** 24
> **Düzeltilen:** 43
> **Bulunan toplam sorun sayısı:** 118+
