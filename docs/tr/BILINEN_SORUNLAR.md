# Bilinen Sorunlar ve Teknik Riskler

Bu belge, Memo projesindeki tüm açık hataları ve teknik riskleri takip eder.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🔵 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- ⚪ **Düşük** — kozmetik, küçük iyileştirme veya uç durum
- ⚫ **Bilgi** — tasarım notu, risk veya gözlem

---

## ✅ Düzeltilen Hatalar (2026-06-13)

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

---

## 🔴 Kritik

### K01. `DeleteLocalModel`'de TOCTOU Sembolik Bağlantı Yarışı
- **Dosya:** `internal/modelstore/modelstore.go:421-458`
- **Detay:** `filepath.EvalSymlinks` ile yol çözümleme (satır 431) ile `os.Remove` (satır 443) arasında bir zaman penceresi vardır. Kötü niyetli bir kullanıcı, modeller dizinine yazma erişimine sahipse, çözümlenmiş yolu kaldırma anında rastgele bir dosyaya sembolik bağlantı ile değiştirebilir. `strings.HasPrefix` koruması (satır 439) en basit saldırıları engeller ancak kararlı bir saldırgan bu zaman penceresinden yararlanabilir.
- **Risk:** Modeller dizini dışında rastgele dosya silme.
- **Kategori:** Güvenlik

### K02. HTTP İşleyicilerinde İstek Gövdesi Boyut Sınırı Yok
- **Dosya:** `internal/webserver/handlers_flutter.go` (~35 endpoint)
- **Detay:** Tüm HTTP işleyicileri `r.Body`'yi `http.MaxBytesReader` veya herhangi bir boyut sınırı olmadan okur. Bir istemci multi-gigabayt yükler göndererek tüm sunucu belleğini tüketebilir.
- **Risk:** Bellek tükenmesi DoS saldırısı.
- **Kategori:** Güvenlik

### K03. Veri Yarışı: `a.store` / `a.syncManager` / `a.client` İşaretçi Yazmaları
- **Dosya:** `app.go:143-186` (App struct), `app.go:240-263` (startup atamaları), `handlers_flutter.go` (yeniden yapılandırma işleyicileri)
- **Detay:** `a.store`, `a.syncManager` ve `a.client` alanları startup ve yeniden yapılandırma sırasında muteks koruması olmadan yeniden atanır (`a.client` için `clientMu` mevcut ancak `store` ve `syncManager` için eksik). Eşzamanlı istekler bu işaretçileri okurken kısmen başlatılmış veya nil değerler gözlemleyebilir.
- **Risk:** Yeniden yapılandırma sırasında eşzamanlı erişimde çökme veya veri bozulması.
- **Kategori:** Veri Yarışı

### K04. Agent Sandbox: Kabuk Yorumlayıcıları Kara Listede Değil
- **Dosya:** `internal/agent/tools/command.go:23-44` (kara liste regex'leri), `internal/agent/permissions.go` (izin sistemi)
- **Detay:** Komut kara listesi `rm`, `dd`, `mkfs`, `chmod`, `sudo` gibi tehlikeli araçları engeller ancak `sh`, `bash`, `zsh`, `dash` gibi kabuk yorumlayıcılarını engellemez. Bir saldırgan `sh -c "rm -rf /"` çağırarak `rm` kara listesini aşabilir.
- **Risk:** Kabuk yorumlayıcıları üzerinden tam sandbox kaçışı.
- **Kategori:** Güvenlik

### K05. Agent Araç Argümanlarında Yol Geçişi Koruması Yok
- **Dosya:** `internal/agent/sandbox.go:70-94` (`ValidatePath`), `internal/agent/tools/command.go` (`RunCommand` dosya argümanları)
- **Detay:** Sandbox, çalışma dizinini `strings.HasPrefix` ile doğrular ancak araç argümanlarındaki dosya yollarını (`read_file`, `write_file`, `edit_file` vb.) doğrulamaz. `write_file` aracına `../../etc/passwd` gibi göreli bir yol verildiğinde, sandbox dizini dışına çıkılabilir.
- **Risk:** Göreli yollar içeren araç argümanları ile sandbox kaçışı.
- **Kategori:** Güvenlik

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

### H06. Flutter: SSE Akışında Zaman Aşımı/Devamlılık Koruması Yok
- **Dosya:** `frontend/lib/core/api_client.dart:38-87`, `600-654`
- **Detay:** `sendMessageStream` ve `sendFileStream` yanıt akışında boşta kalma/duraklama zaman aşımı yoktur. Sunucu veri göndermeyi durdurursa `await for` süresiz bloke olur. Tek koruma Dio seviyesindeki 120s `receiveTimeout`'dur ancak hata vermeyen durmuş bir akış bağlantıyı sonsuza kadar tutar.

### H07. Flutter: Global Stil Önbelleği (`_styleCache`) Bellek Sızıntısı
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

### H14. `callLLMStream` Goroutine'u İstemci Bağlantı Kesintisinde 5 Dakika Yaşıyor
- **Dosya:** `app.go:931-1146`
- **Detay:** İstemci bağlantıyı kestiğinde HTTP işleyicisi döner ancak `callLLMStream` içindeki goroutine 300 saniyelik context timeout'una kadar yaşamaya devam eder. Bu, biriken goroutine'lere yol açar.
- **Risk:** Bağlantı kesilmesi başına ~5 dakika goroutine sızıntısı.

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

---

> **Son güncelleme:** 2026-06-13
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, tüm internal/ paketleri) ve Flutter frontend
> **Açık hatalar:** 30+ (🔴5, 🟠14, 🔵9, ⚪2)
> **Gözlemler:** 15
> **Düzeltilen:** 10
> **Bulunan toplam sorun sayısı:** 55+
