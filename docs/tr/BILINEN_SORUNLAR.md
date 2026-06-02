# Bilinen Sorunlar ve Teknik Riskler (Kapsamlı Denetim)

Bu belge, Memo projesindeki tüm tespit edilen hataları, mimari kısıtlamaları ve uç durumları takip eder. Derinlemesine yapılan kod denetimi sonrası güncellenmiştir.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🟡 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- 🔵 **Düşük** — kozmetik, küçük iyileştirme veya uç durum
- ⚪ **Bilgi** — tasarım notu, risk veya gözlem

---

## 🔴 Kritik

### ~~K1. Yetim SSE Bağlantıları — İstemci Koptuğunda LLM Çalışmaya Devam Ediyor~~ ✅ Çözüldü
- **Dosya:** `internal/webserver/handlers_flutter.go:39-61`, `internal/api/streaming.go`, `app.go`, `internal/webserver/bridge.go`
- **Çözüm:** `AppBridge`/`FullBridge` stream metodlarına `ctx context.Context` parametresi eklendi. `handleSendStream`'den `r.Context()` tüm LLM çağrı zincirine (→ `SendMessageStream` → `callLLMStream` → `ChatCompletionStream`) iletiliyor. İstemci kopunca context cancel oluyor, LLM HTTP isteği iptal ediliyor, goroutine doğal olarak sonlanıyor.

### ~~K2. Motor Modu / Yapılandırma Güncellemesi Tüm Llama Ayarlarını Sıfırlıyor~~ ✅ Çözüldü
- **Dosya:** `app.go:1148-1151`
- **Çözüm:** `UpdateLlamaConfig` artık tüm struct'ı replace etmek yerine her alanı ayrı ayrı kontrol ediyor: sadece non-zero (boş olmayan) değerler mevcut config'e yazılıyor. Kısmi JSON body (`{"engine_mode": "cpu"}`) sadece `EngineMode` alanını güncelliyor, diğer ayarlar (port, ctx_size, binary_path vb.) korunuyor.

### ~~K3. `/api/image` Üzerinden Keyfi Dosya Okuma~~ ✅ Çözüldü
- **Dosya:** `app.go:865-872` (GetImageBase64), `internal/webserver/handlers_flutter.go:214-226` (handleImage)
- **Çözüm:** Çift katmanlı path doğrulama. Katman 1 (handler): `..` traversal ve absolute path engellenir. Katman 2 (`GetImageBase64`): `filepath.Abs` + `EvalSymlinks` çözümlemesi + `data/` dizini whitelist prefix kontrolü. Symlink saldırıları da engellenir.

### ~~K4. Uzaktan Erişim Sunucusu — Kimlik Doğrulama Yok, Açık CORS~~ ✅ Çözüldü
- **Dosya:** `internal/webserver/server.go:65-176`, `app.go:155-157`, `app.go:1048-1063`
- **Çözüm:** Uzaktan erişim v3.0.0'da tamamen devre dışı bırakıldı. `Start()` anında hata döndürür. `SetRemoteAccess` etkinleştirmeyi reddeder. CORS wildcard'ı `Origin` yankısı ile değiştirildi. Gelecek bir sürümde yeniden eklenecek.

### ~~K5. `a.client` Değişkeninin Kilitsiz Yeniden Atanması~~ ✅ Çözüldü
- **Dosya:** `app.go`
- **Çözüm:** `App` struct'ına `clientMu sync.RWMutex` eklendi. Tüm yazmalar `Lock/Unlock`, tüm okumalar `RLock/RUnlock` ile korunuyor. `a.client` ve `a.embeddingClient` tamamen güvence altına alındı.

### ~~K6. `saveMemoryAsync` RLock→Lock Modeli (Kilitlenme Riski)~~ ✅ Çözüldü
- **Dosya:** `app.go`
- **Çözüm:** Lock+goroutine modeli channel-based worker ile değiştirildi. `saveMemoryAsync` kanala yazar, anında döner. `memorySaveWorker` sırayla işleri `storeMu.Lock()` alarak yapar. RLock→Lock geçişi tamamen kalktı.

### ~~K7. UI İş Parçacığı Performansı — Mesaj Başına AnimationController~~ ✅ Çözüldü
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart`
- **Çözüm:** `_MessageBubble`'daki tüm entry animasyonları kaldırıldı. `SingleTickerProviderStateMixin`, `AnimationController`, `FadeTransition`, `SlideTransition` tamamen temizlendi.

---

## 🟠 Yüksek

### ~~Y1. SSE Bağlantı Kesintisinde Goroutine Sızıntısı~~ ✅ Çözüldü
- **Dosya:** `internal/api/streaming.go`, `internal/api/client.go`
- **Çözüm:** `processSSEStream` artık `ctx context.Context` parametresi alıyor. Context iptal olduğunda body'i kapatan bir watcher goroutine ile scanner uyandırılıyor, goroutine doğal olarak sonlanıyor. `ChatCompletionStream` context'i `processSSEStream`'e iletiyor.

### ~~Y2. Yapılandırma Dosyası Dünya-Tarafından Okunabilir (`0644`) — Sırlar İçeriyor~~ ✅ Çözüldü
- **Dosya:** `internal/config/config.go:178`
- **Çözüm:** `os.WriteFile` çağrısında `0644` → `0600` olarak değiştirildi. Artık config dosyası sadece kullanıcının kendisi tarafından okunabilir.

### Y3. Senkronizasyon Şifrelemesi İçin Zayıf Anahtar Türetme
- **Dosya:** `internal/cloudsync/crypto.go:18-23`
- **Sorun:** `deriveKey`, `sha256.Sum256([]byte("memo-sync-v1:" + passphrase))` kullanır — sabit bir tuz ile tek SHA-256 iterasyonu. Zayıf parolalar için (çoğu kullanıcının seçtiği gibi) kaba kuvvetle kolayca kırılabilir. Endüstri standardı PBKDF2, bcrypt veya argon2id'dir.
- **Etki:** Senkronizasyon verilerinin gizliliği zayıf bir KDF'ye dayanır.
- **Çözüm:** `golang.org/x/crypto/pbkdf2` veya argon2id kullanın, verilerle birlikte saklanan rastgele bir tuz ile.

### Y4. Sabit Kodlanmış Geri Dönüş Şifreleme Anahtarı
- **Dosya:** `internal/cloudsync/crypto.go:59-62`
- **Sorun:** `hardwareID()` makine kimliği belirleyemediğinde (örn. hostname'siz konteyner), sabit `"memo-fallback-key"` string'ine düşer. Parola veya makine kimliği olmayan her makine **aynı** şifreleme anahtarını kullanır.
- **Etki:** Bu tür tüm makineler birbirlerinin senkronizasyon verilerini çözebilir.
- **Çözüm:** Net bir parola gerektirin veya ilk senkronizasyonda rastgele bir anahtar oluşturup yapılandırmada saklayın.

### ~~Y5. `buildMessages` Oturum Geçmişini Kalıcı Olarak Değiştiriyor~~ ✅ Çözüldü
- **Dosya:** `app.go:1358`
- **Çözüm:** `buildMessages` içinde `history`'nin savunma amaçlı kopyası eklendi: `history = append([]api.Message{}, history...)`. Artık `getSessionHistory()` dönüşü üzerinde yapılan mutasyonlar session verisini asla etkilemez. Sistem prompt'u her istekte tek sefer enjekte edilir, ikiye katlanmaz.

### ~~Y6. `hash2hex` — SHA-256'nın Sadece 4 Baytı (Çakışma Riski)~~ ✅ Çözüldü
- **Dosya:** `internal/memory/store.go:342-344`
- **Çözüm:** `hash[:4]` → `hash[:8]` olarak değiştirildi. Artık 8 bayt (16 hex karakter) kullanılıyor. Çakışma olasılığı %50'den ihmal edilebilir seviyeye düştü.

### ~~Y7. `monitor()` Goroutine'inin `s.cmd`'ye Kilit Dışında Erişmesi~~ ✅ Çözüldü
- **Dosya:** `internal/llama/llama.go:271-302`
- **Çözüm:** Nil kontrolü artık lock içinde yapılıp local kopyaya alınıyor (`cmd := s.cmd`). `Wait()` local kopya üzerinden çağrılıyor. `Stop()` `s.cmd = nil` yapsa bile `monitor()` kendi kopyası üzerinde çalıştığı için nil-pointer paniği mümkün değil.

### ~~Y8. İndirme Hatalarında Geçici Dosya Temizlenmiyor~~ ✅ Çözüldü
- **Dosya:** `internal/modelstore/modelstore.go:237-243`
- **Çözüm:** `os.Remove(tmpPath)` koşullu (`ctx.Err() != nil`) olmaktan çıkarılıp koşulsuz `defer` içine alındı. Artık ister context iptali, ister HTTP hatası, ister ağ zaman aşımı olsun — her durumda `.downloading` geçici dosyası temizlenir.

### ~~Y9. `extractTarGzToBin`'de Dosya Tanıtıcı Sızıntısı~~ ✅ Çözüldü
- **Dosya:** `internal/llama/installer.go:433,437`
- **Çözüm:** Tar çıkarma döngüsü yeniden yapılandırıldı. Her dosya için ayrı `extractFile()` fonksiyonu oluşturuldu, `defer out.Close()` ile dosyanın her koşulda kapanması garanti altına alındı. Manuel `out.Close()` çağrıları kaldırıldı.

### ~~Y10. `nvidia-smi` Hataları Sessizce Geçiliyor → 0 VRAM → 0 GPU Katmanı~~ ✅ Çözüldü
- **Dosya:** `internal/llama/gpu.go:62-101`
- **Çözüm:** Her `nvidia-smi` çağrısının hatası artık `log.Printf` ile loglanıyor: binary bulunamazsa, name sorgusu başarısız olursa, VRAM sorgusu başarısız olursa ve VRAM değeri parse edilemezse. Kullanıcı artık neden GPU kullanılamadığını görebilir.

### Y11. OAuth `authDone` Kanalı Yarışı
- **Dosya:** `internal/cloudsync/drive.go:99-103`
- **Sorun:** `StartAuthFlow` yeni bir `authDone` kanalı oluşturur (`make(chan struct{})`). `WaitForAuth` önceki kanalı eşzamanlı okuyorsa, takas eski (zaten kapalı) kanalda sonsuza kadar beklemesine neden olurken yeni kanal asla kapatılmaz.
- **Etki:** Hızlı OAuth akışı başlatmalarında nadir takılma.
- **Çözüm:** `sync.WaitGroup` veya mutex ile sıfırlanan tek bir paylaşımlı kanal kullanın.

### ~~Y12. `Shutdown(context.Background())` Süresiz Bloke Olabilir~~ ✅ Çözüldü
- **Dosya:** `internal/webserver/server.go:286`
- **Çözüm:** `context.Background()` → `context.WithTimeout(10*time.Second)` olarak değiştirildi. Artık sunucu kapanırken bir handler takılı kalırsa 10 saniye sonra zaman aşımına uğrar ve kapanma tamamlanır.

### ~~Y13. Oturum Kimliği 8 Hex Karaktere Kırpılmış~~ ✅ Çözüldü
- **Dosya:** `internal/sessions/sessions.go:68`
- **Çözüm:** `uuid.New().String()[:8]` → `uuid.New().String()` olarak değiştirildi. Artık tam UUID (36 karakter) kullanılıyor. Çakışma olasılığı astronomik seviyede düşük.

### Y14. İndirme Yoklama Akışı Sonsuza Kadar Çalışıyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:66-79`
- **Sorun:** `downloadProgressProvider` akışı, her 1–3 saniyede bir backend'i yoklayan bir `while (true)` döngüsü içerir. Bu döngü asla iptal edilmez — uygulama ömrü boyunca çalışır. İndirme tamamlandıktan sonra bile (ve ilerleme dialog'u kapatıldığında), yoklama her 3 saniyede bir gereksiz HTTP isteği yapmaya devam eder.
- **Etki:** Gereksiz ağ trafiği ve pil tüketimi.
- **Çözüm:** İndirme tamamlandığında veya provider dispose edildiğinde akış aboneliğini iptal edin.

### ~~Y15. Backend Hata Yönetimi: Bağlantı Hatasında "Kurulu" Gösteriliyor~~ ✅ Çözüldü
- **Dosya:** `frontend/lib/providers/models_provider.dart:97-104`
- **Çözüm:** Bağlantı hatasında `true` ("kurulu") döndüren `connectionError` kontrolü kaldırıldı. Artık her hata durumunda `false` döndürülür — backend ulaşılamaz olduğunda kullanıcı kurulum ekranını görür ve işlemi tetikleyebilir.

---

## 🟡 Orta

### O1. Arka Plan Hataları Arayüze Asla Ulaşmıyor (Bozuk Olay Sistemi)
- **Dosya:** `app.go` (emitEvent pasif), tüm çağrı noktaları
- **Sorun:** `emitEvent` Wails için tasarlanmıştı ve Flutter için no-op haline geldi. Arka plan hataları (bulut senkronizasyon hataları, gömme modeli yükleme hataları, indirme ilerlemesi, otomatik başlatma hataları) yalnızca `server.log`'a yazılır ve kullanıcıya asla gösterilmez.
- **Etki:** Sessiz hatalar; senkronizasyon bozulduğunda veya gömme başarısız olduğunda kullanıcı hiçbir geri bildirim almaz.
- **Çözüm:** Arka plan durumu için bir sunucu-olay akışı endpoint'i veya yoklama endpoint'i uygulayın.

### O2. Oturum Dosyaları Dünya-Tarafından Okunabilir (`0644`)
- **Dosya:** `internal/sessions/sessions.go:236`
- **Sorun:** Sohbet oturumu JSON dosyaları `0644` izinleriyle yazılır. Tam sohbet geçmişi herhangi bir yerel kullanıcı tarafından okunabilir.
- **Etki:** Çok kullanıcılı sistemlerde gizlilik sızıntısı.
- **Çözüm:** `0600` ile yazın.

### O3. `save()` Hataları Oturum Yöneticisinde Sessizce Atılıyor
- **Dosya:** `internal/sessions/sessions.go:75,155`
- **Sorun:** `newSession` ve `AddMessage` `m.save(s)` çağırır ancak dönen hatayı yok sayar. Oturum verileri diske sessizce yazılamaz.
- **Etki:** Disk dolu veya izin hatası koşullarında hiçbir uyarı olmadan sohbet geçmişi kaybı.
- **Çözüm:** Hatayı loglayın ve/veya çağrı sahibine döndürün.

### O4. `loadAll()` Bozuk Oturum Dosyalarını Sessizce Atlıyor
- **Dosya:** `internal/sessions/sessions.go:252-258`
- **Sorun:** `loadAll` içinde, bireysel dosya okuma hataları ve JSON decode hataları `continue` ile atlanır — hiçbir hata loglanmaz ve kullanıcı bazı oturumların kaybolduğunu asla bilemez.
- **Etki:** Bozulmada sessiz veri kaybı.
- **Çözüm:** Her atlanan dosyayı loglayın.

### O5. SSE `[DONE]` Parçasında `FinishReason` Eksik
- **Dosya:** `internal/api/streaming.go:65`
- **Sorun:** `[DONE]` sentinel parçası `finish_reason` alanı olmadan gönderilir. Frontend'in normal tamamlama, maksimum token'a ulaşma veya durdurma dizisini ayırt etme yolu yoktur.
- **Etki:** Frontend "maksimum token'a ulaşıldı" veya "durduruldu" göstergeleri gösteremez.
- **Çözüm:** `[DONE]` parçasında `finish_reason` gönderin.

### O6. Ana Yolda Senkron Bloke Eden Yazmalar
- **Dosya:** `internal/sessions/sessions.go:155`, `internal/memory/store.go:105`
- **Sorun:** Her mesaj, oturumlar için senkron `json.MarshalIndent` + `os.WriteFile` ve hafıza için senkron gömme hesaplaması + gob yazma tetikler. Bunlar LLM yanıt yolunu bloke eder.
- **Etki:** Yavaş disklerde veya gömme hesaplaması sırasında artan yanıt gecikmesi.
- **Çözüm:** Yazmaları debounce zamanlayıcı / async worker ile tamponlayın.

### O7. `LoadCache` Performansı — O(N) Başlangıç Süresi
- **Dosya:** `internal/memory/store.go:72-90`
- **Sorun:** `LoadCache` başlangıçta diskteki her `.gob` dosyasını okur ve tüm embedding'leri RAM'de saklar. 10.000+ hafıza girişinde başlangıç süresi ve bellek kullanımı doğrusal olarak artar.
- **Etki:** Yavaş başlangıç; büyük hafıza depoları için aşırı RAM kullanımı.
- **Çözüm:** Sayfalama, tembel yükleme veya disk tabanlı indeks (SQLite/bolt) uygulayın.

### O8. Kaba Kuvvet O(N) Vektör Arama
- **Dosya:** `internal/memory/retriever.go`
- **Sorun:** Hafıza araması RAM'deki tüm embedding vektörlerini doğrusal olarak tarar. 10.000 girişin ötesinde arama gecikmesi belirgin şekilde artar.
- **Çözüm:** Bir yaklaşık en yakın komşu (ANN) indeksi veya vektör veritabanı kullanın.

### O9. `killByPort` `lsof` / `fuser`'a Bağımlı
- **Dosya:** `internal/llama/llama.go:244-253`
- **Sorun:** `killByPort`, `lsof` veya `fuser`'a kabuk çağrısı yapar. Minimal konteynerlerde, gömülü sistemlerde veya bu araçların olmadığı Windows'ta fonksiyon sessizce başarısız olur ve porta bağlı bir süreç bırakır.
- **Etki:** Sonraki model başlatmalarında "Adres kullanımda" hataları.
- **Çözüm:** Port tabanlı keşif yerine alt süreç PID'lerini takip edin ve doğrudan öldürün.

### O10. Sabit Kodlanmış Windows Ses Aygıtı GUID'i
- **Dosya:** `app.go:739` (StartRecording üzerinden)
- **Sorun:** Windows'ta kayıt komutu, mikrofon için sabit bir `@device_cm_{...}` GUID'i kullanır. Bu GUID yalnızca bir donanım yapılandırmasına özgüdür. Çoğu Windows makinesinde kayıt sessizce başarısız olur.
- **Etki:** STT, çoğu kullanıcı için Windows'ta bozuktur.
- **Çözüm:** Başlangıçta ses aygıtlarını numaralandırın veya varsayılan kayıt aygıtını kullanın.

### O11. Linux GPU Algılaması Sysfs Üzerinden Kırılgan
- **Dosya:** `internal/llama/gpu.go:167`
- **Sorun:** GPU algılaması `bash -c "cat /sys/class/drm/card*/device/vendor"` çalıştırır. Bu, `/sys`'in mevcut olmasına (Docker'da `--privileged` gerekir), `bash`'in bulunmasına ve DRM aygıtlarının belirli adlandırma modeline bağlıdır.
- **Etki:** Konteynerlerde veya standart dışı ortamlarda GPU algılaması sessizce başarısız olur.
- **Çözüm:** Yedek olarak `lspci` ayrıştırması kullanın veya `hwmon`/`drm` bilgilerini daha sağlam okuyun.

### O12. Geçmiş Okurken Otomatik Kaydırma Dibe Çekiyor
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:23-33`
- **Sorun:** `didUpdateWidget`, mesajlar değiştiğinde `_scrollToBottom()` çağırır. Kullanıcı önceki mesajları okumak için yukarı kaydırdıysa, yeni bir token gelmesi onu zorla alta götürür.
- **Etki:** Akış etkinken geçmiş okunamaz.
- **Çözüm:** Yalnızca kullanıcı dibe yakınsa (örn. 50px) otomatik kaydırma yapın.

### O13. Dışa Aktarma Hataları Sessizce Yutuluyor
- **Dosya:** `frontend/lib/screens/chat_screen.dart:170-178`
- **Sorun:** Sohbet dışa aktarma boş bir `catch (_) {}` bloğuna sahiptir. Dışa aktarma başarısız olursa (aktif sohbet yok, izin reddi, yazma hatası), kullanıcı sıfır geri bildirim alır.
- **Etki:** Kullanıcılar dışa aktarma sessizce başarısız olduğunda başarılı olduğunu düşünür.
- **Çözüm:** Hata durumunda SnackBar veya dialog gösterin.

---

## 🔵 Düşük

### D1. Yapılandırma Yükleme Hatası Sessizce Varsayılana Düşüyor
- **Dosya:** `app.go:115-119`
- **Sorun:** `config.Load()` başarısız olursa (bozuk YAML, izinler), uygulama hatayı loglar ve varsayılan yapılandırmayı kullanır. Kullanıcının özel ayarları sessizce yok sayılır.
- **Çözüm:** Hatayı main'e döndürün ve başlamayı reddedin (veya en azından bloke eden bir dialog gösterin).

### D2. Hafıza Deposu / Oturum Yöneticisi Başlatma Hataları Sessizce Devre Dışı Bırakılıyor
- **Dosya:** `app.go:126-137`
- **Sorun:** `NewStore` ve `sessions.NewManager`'dan gelen hatalar yalnızca loglanır. `a.store` ve `a.sessions` nil olarak ayarlanır. Uygulama hafızasız ve oturum kalıcılığı olmadan çalışmaya devam eder — her ikisi de çalışıyor gibi görünür ancak hiçbir şey kaydetmez.
- **Çözüm:** Bu hataları kullanıcıya gösterin veya başlamayı reddedin.

### D3. `os.Executable()` Hatası Yok Sayıldığında Boş Yol
- **Dosya:** `app.go:813`
- **Sorun:** `exePath, _ := os.Executable()` — `/proc/self/exe`'nin mevcut olmadığı sistemlerde `exePath` boş string'dir. Bu, geçerli çalışma dizinine çözümlenen göreli yollara yayılır.
- **Çözüm:** Hatayı kontrol edin ve `os.Args[0]`'a düşün.

### D4. Boş Token Yolu Tüm Drive İşlemlerini Bozuyor
- **Dosya:** `internal/cloudsync/drive.go:115`
- **Sorun:** `dc.tokenPath` boşsa (yapılandırma yok), `os.ReadFile("")` geçerli dizini okur ve başarısız olur. Kullanıcıya net bir hata gösterilmez.
- **Çözüm:** `tokenPath`'i başlatmada doğrulayın ve net bir hata döndürün.

### D5. Nil `embeddingFunc` ile `NewStore` Sessiz Çökme Yolu Oluşturuyor
- **Dosya:** `internal/memory/store.go:38-57`
- **Sorun:** `embeddingFunc` nil ise `NewStore` başarılı olur ancak herhangi bir `SaveInteraction` çağrısı nil fonksiyon çağrısı ile panikler.
- **Çözüm:** `NewStore`'dan `embeddingFunc` nil olduğunda hata döndürün.

### D6. Hafıza İndeksi Tüm Embedding'leri `append` ile Kopyalıyor (2x RAM)
- **Dosya:** `internal/memory/store.go:84`
- **Sorun:** `MemoryIndex` her embedding vektörünü `append([]float32(nil), doc.Embedding...)` ile kopyalar. `LoadCache` sırasında bu, hafıza verileri için RAM kullanımını ikiye katlar.
- **Çözüm:** Kaynak mutasyona uğramayacaksa embedding'lere doğrudan referans verin.

### D7. `DiscordWebhook` / Action URL Yazmaları Asla Kontrol Edilmiyor
- **Dosya:** `app.go:463-514`
- **Sorun:** Discord webhook ve aksiyon URL yazmalarının (satır 483-494 ve 503) sonuçları asla kontrol edilmez. Bu entegrasyonların sessiz başarısızlığı.
- **Çözüm:** En azından hatayı loglayın.

### D8. OAuth Loopback Dinleyicisinde Tip Dönüşüm Panik
- **Dosya:** `internal/cloudsync/drive.go:109`
- **Sorun:** `port := ln.Addr().(*net.TCPAddr).Port` sert tip dönüşümü kullanır. Dinleyici TCP değilse (bazı Go ağ uygulamalarında olası), bu panikler.
- **Çözüm:** Virgül-ok tip dönüşümü kullanın.

### D9. `WakeOnLan` / `Precise` Dosya İşlemleri
- **Dosya:** `app.go:1150-1178`
- **Sorun:** Çeşitli geçici dosya yazma ve flag dosyası işlemleri hataları kontrol etmez. Disk dolu veya izinler yanlışsa işlemler başarılı görünür ancak gerçekleşmez.
- **Çözüm:** Tüm `os.WriteFile` ve `os.Remove` çağrılarını kontrol edin.

### D10. Model İçe Aktarmada Boyut Sınırı Yok
- **Dosya:** `internal/modelstore/modelstore.go:399-433`
- **Sorun:** `ImportLocalModel`, kullanıcı tarafından belirtilen bir kaynak yolundan dosyayı boyut sınırı olmadan kopyalar. Çok terabaytlık bir dosya seçimi diski doldurabilir.
- **Çözüm:** Kopyalamadan önce dosya boyutunu kontrol edin ve/veya limitli `io.CopyN` kullanın.

### D11. `DeleteLocalModel`'de Sembolik Bağ Saldırısı
- **Dosya:** `internal/modelstore/modelstore.go:370-397`
- **Sorun:** Yol doğrulaması `strings.HasPrefix(absPath, absModelsDir)` kullanır. `/data/models/evil` gibi çözümlenmiş bir yol, `evil` `/etc/passwd`'ye sembolik bağ olsa bile geçebilir ve keyfi dosya silmeye yol açar.
- **Çözüm:** Karşılaştırmadan önce her iki yolda `filepath.EvalSymlinks` kullanın.

### D12. `safePersistPath`'de TOCTOU Yarışı
- **Dosya:** `internal/memory/store.go:262-278`
- **Sorun:** Yol doğrulama ve dosya işlemi atomik değildir. Kötü niyetli bir süreç, doğrulanmış dosyayı kontrol ve işlem arasında bir sembolik bağ ile değiştirebilir.
- **Çözüm:** Dosyayı doğrulamadan önce açın (`syscall.Open` ile `O_NOFOLLOW`).

### D13. `runCmdStream` Goroutine'leri Fonksiyondan Uzun Yaşayabilir
- **Dosya:** `internal/llama/installer.go:628-633`
- **Sorun:** Stdout/stderr okuyucu goroutine'leri `go func()` ile başlatılır. `cmd.Wait()` goroutine'ler okumayı bitirmeden dönerse (kısa ömürlü komut), logger'a fonksiyon döndükten sonra yazabilirler.
- **Çözüm:** Goroutine'lerin tamamlanmasını sağlamak için `sync.WaitGroup` kullanın.

### D14. Sohbet Girişi `/` Komutunun Görsel Göstergesi Yok
- **Dosya:** `frontend/lib/widgets/chat_input.dart:29-32`
- **Sorun:** `/` tuşu bir prompt şablonu açılır penceresi tetikler ancak hiçbir UI ipucu yoktur (yer tutucu metin, araç ipucu yok). Kullanıcılar bunu tesadüfen keşfetmelidir.
- **Çözüm:** Bir ipucu metni veya simge düğmesi ekleyin.

### D15. Her Build'de `FocusNode` Oluşturuluyor
- **Dosya:** `frontend/lib/widgets/chat_input.dart:185-193`
- **Sorun:** `KeyboardListener`, build metodunda `FocusNode()` kullanır ve her yeniden build'de yeni bir nesne oluşturur. Eski node garbage collect edilir.
- **Çözüm:** FocusNode'u state'de saklayın.

### D16. Ayarlarda Eski Prompt Metni (Veri Değiştiğinde Güncellenmiyor)
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:343-345`
- **Sorun:** Sistem prompt `TextEditingController`'ı ilk veri gelişinde bir kez başlatılır. Prompt harici olarak değişirse (başka cihaz, API çağrısı), görüntülenen metin eskidir.
- **Çözüm:** Provider'a bir dinleyici ekleyin ve controller'ı güncelleyin.

### D17. Hata Durumu Yalnızca Simge Gösteriyor — Hata Mesajı Yok
- **Dosya:** `frontend/lib/screens/chat_screen.dart:53`
- **Sorun:** Sohbet listesi için yükleme/hata durumu yalnızca genel bir hata simgesi gösterir. Gerçek hata nesnesi görüntülenmez.
- **Çözüm:** Hata mesajı metnini gösterin.

### D18. Model Durdurma Düğmeleri Beklemeden Ateşleniyor
- **Dosya:** `frontend/lib/screens/model_store_screen.dart:606-608, 662-665`
- **Sorun:** Model durdurma düğmeleri API'yi `await` olmadan ve sonucu kontrol etmeden çağırır. API çağrısı başarısız olursa, düğme durumu ("durduruldu") gerçeklikle eşleşmez.
- **Çözüm:** Çağrıyı bekleyin ve hata durumunda UI durumunu geri alın.

### D19. Bulut Senkronizasyonu ve Uzaktan Erişim Sekmeleri "Yapım Aşamasında" Gösteriyor
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:766-823`
- **Sorun:** Bulut Senkronizasyonu ve Uzaktan Erişim ayar sekmeleri "Yapım aşamasında..." gösterirken backend her iki özellik için de tam uygulamaya sahiptir.
- **Çözüm:** UI sekmelerini backend işlevselliğiyle eşleşecek şekilde uygulayın.

### D20. Kurulum Sihirbazı Enterpolasyon Yerine Sabit `$name` Kullanıyor
- **Dosya:** `frontend/lib/widgets/setup_wizard_view.dart:87`
- **Sorun:** Oluşturulan sistem prompt'u `\$name` (kaçışlı, sabit `$name` metni) içerirken backend `%s` format string'leri kullanır. İsim asla yerine konulmaz.
- **Çözüm:** `$name`'i prompt'tan kaldırın veya uygun enterpolasyon uygulayın.

---

## ⚪ Bilgi / Gözlemler

### B1. GOB Kodlaması ve İleri Uyumluluk
- **Dosya:** `internal/memory/store.go:302-306`
- **Not:** `chromem.Document`, Go'nun `gob` kodlaması ile serileştirilir. Gob, struct alan değişikliklerine duyarlıdır: gelecek bir sürümde alan eklemek, kaldırmak veya yeniden adlandırmak mevcut tüm hafıza dosyalarını okunamaz hale getirecektir. Kendi kendini tanımlayan bir format (JSON, CBOR veya protobuf) düşünün.

### B2. Etkileşim Başına Tek Dosya Tasarımı
- **Dosya:** `internal/memory/store.go`
- **Not:** Her hafıza etkileşimi ayrı bir `.gob` dosyasıdır. Bu tasarım silmeyi (`os.Remove`) basitleştirir ancak şunlar için patolojik davranış yaratır:
  - Başlangıç: O(N) dosya okuma
  - Bulut senkronizasyonu: senkronizasyon başına O(N) API çağrısı
  - Dosya tanıtıcı kullanımı
  - HDD'lerde disk arama süreleri

### B3. Filepath.Walk Hata Yutma
- **Dosya:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`
- **Not:** Birçok `filepath.Walk` geri çağrısı tüm hatalar için `nil` döndürür. İzin reddedilen dizinler ve G/Ç hataları kullanıcıya görünmez.

### B4. Gömme İstemcisi Yeniden Başlatma Sonrası Eski Referans
- **Dosya:** `app.go:148-149`, `app.go:124-125`
- **Not:** `a.client` değiştirildiğinde (yeni LLM endpoint'i), `a.store`'daki gömme fonksiyonu hala eski client'ı referans alır. Store, `reinitMemoryStore` çağrılana kadar önceki endpoint'i kullanmaya devam eder.

### B5. Dosya Adına Göre Model Otomatik Sınıflandırma
- **Dosya:** `internal/modelstore/modelstore.go:58-64`
- **Not:** `isEmbeddingModel`, dosya adı veya repo ID'sinin gömme ile ilgili anahtar kelimeler (bge, e5, vb.) içerip içermediğini kontrol eder. Bu buluşsal yöntem, adında bu string'leri bulunan ancak aslında sohbet modeli olan modelleri yanlış sınıflandırır.

### B6. `unsanitizePath`, Repo ID'lerindeki `__`'den `/` Enjekte Edebilir
- **Dosya:** `internal/modelstore/modelstore.go:345`
- **Not:** `unsanitizePath`, `__`'yi `/` ile değiştirir. Bir HuggingFace repo ID'si doğal olarak `__` içeriyorsa, bu beklenmeyen dizin yapıları oluşturur. Yol geçişi `filepath.Join` normalizasyonu ile önlenir ancak dizin düzeni kullanıcıları şaşırtabilir.

### B7. Llama Sunucusu Stderr'i Uygulama Loglarıyla Karışıyor
- **Dosya:** `internal/llama/llama.go:118-119`
- **Not:** Alt süreç stdout/stderr'i `os.Stdout`/`os.Stderr`'e ayarlanmıştır. Llama.cpp'nin tanılama çıktısı (prompt işleme istatistikleri, zamanlama, uyarılar) ön ek veya filtreleme olmadan doğrudan uygulamanın çıktı akışında görünür.

### B8. `UpdateSyncSettings` ve `ensureSyncManager` Arasında Yarış
- **Dosya:** `app.go:1505-1510`, `app.go:1627-1652`
- **Not:** `ensureSyncManager()`, `a.syncManager`'ı kilit olmadan okur. `UpdateSyncSettings`, `syncManager = nil` ayarlar ve senkronizasyon olmadan yeni bir örnek oluşturur. Eşzamanlı çağrılar güncel olmayan veya çift başlatmaya neden olabilir.

---

> **Son güncelleme:** 2026-06-02  
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, tüm internal/ paketleri) ve Flutter frontend  
> **Toplam sorun:** 55 (7 kritik ✅, 15 yüksek → 11 ✅ / 4 ⬜, 13 orta, 20 düşük, 8 bilgi notu)
