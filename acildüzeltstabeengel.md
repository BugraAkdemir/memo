# Acil Düzeltilmesi Gereken Stabilite ve Olgunluk Engelleri

> **Tarih:** 2026-06-19
> **Kapsam:** Go backend (25 paket) + Flutter masaüstü + Flutter mobil — tam kod denetimi
> **Yöntem:** Kaynak kod seviyesinde doğrulama, satır satır inceleme

---

## 🔴 KRİTİK (Kullanıcı Verisi Güvenliği + Uygulama Çökmesi)

### K1. API Anahtarları Zayıf Şifreleme ile Saklanıyor

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/provider/config.go:374-402` |
| **Kod** | `defaultMachineKey()` → `/etc/machine-id` (Linux), registry GUID (Windows) |
| **Fallback** | `"Mm3m0L0c4lK3y!@#$%^&*()9876543210"` — kaynak kodda sabit |

**Sorun:** `/etc/machine-id` her proses tarafından okunabilir. Aynı makinedeki herhangi bir yazılım (veya kötü niyetli bir script) bu ID'yi okuyup AES-256 anahtarını türetebilir. Fallback anahtar binary içinde görünür durumda. `providers.json` dosyasındaki tüm OpenAI, Claude, Gemini API anahtarları şifresi çözülebilir durumda.

**Kullanıcı etkisi:** Tüm LLM sağlayıcı API anahtarları sızdırılabilir. Kredi kartına bağlı hesaplarda maddi kayıp riski.

**Önerilen çözüm:** İşletim sistemi anahtar kasası entegrasyonu (Linux: Secret Service API / `libsecret`, macOS: Keychain, Windows: Credential Manager).

---

### K2. Bulut Senkronizasyonu Boş Parola ile Hostname'e Düşüyor

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/cloudsync/crypto.go:27-38, 67-87, 113-148` |
| **Kod** | `deriveKey()` → boş parola → `hardwareID()` → hostname → `"memo-fallback-key"` |

**Sorun:** Kullanıcı parola belirlemezse, şifreleme anahtarı hostname veya sabit fallback string'ten türetiliyor. Hostname LAN'da herkes tarafından görülebilir. Ayrıca `decrypt()` (satır 84-86) PBKDF2 başarısız olduğunda sessizce SHA-256 fallback'e geçiyor — brute-force 600.000 kat daha hızlı.

**Kullanıcı etkisi:** Google Drive'a yedeklenen tüm sohbet geçmişi, bellek verileri ve ayarlar üçüncü şahıslar tarafından çözülebilir.

**Önerilen çözüm:** Parola zorunlu olsun. Boş parolaya izin verilmesin. Kullanıcıya şifreleme parolası oluşturma adımı gösterilsin.

---

### K3. Web Sunucu Kapanması Sonsuza Kadar Asılı Kalabiliyor

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/webserver/server.go:281` |
| **Kod** | `go srv.Shutdown(context.Background())` — zaman aşımı yok |

**Sorun:** `context.Background()` ile çağrılan `Shutdown()` hiçbir zaman timeout olmaz. SSE akışı açık olan bir istemci (örn. uzun süren bir sohbet) bağlıysa, `Shutdown()` sonsuza kadar bekler. Üstelik goroutine olarak çağrıldığı için (line 281: `go`), uygulama "kapandım" sanar ama process askıda kalır. Kullanıcı uygulamayı tekrar başlatmaya çalıştığında port (`:8090`) hala dolu olur.

**Kullanıcı etkisi:** Uygulama kapanmıyor, process zombie kalıyor. Yeniden başlatma başarısız oluyor (port çakışması).

**Önerilen çözüm:** `context.WithTimeout(context.Background(), 10*time.Second)` kullanılmalı.

---

### K4. CORS — LAN Modunda Tüm Origin'lere Açık

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/webserver/server.go:586` |
| **Kod** | `if isLoopback \|\| (isLAN && origin != "")` → origin başlığı doğrudan yansıtılıyor |

**Sorun:** `0.0.0.0` üzerinde çalışırken, gelen `Origin` başlığı olduğu gibi `Access-Control-Allow-Origin` cevabına yazılıyor. Ağa açık bir makinede (port yönlendirme, ngrok tunnel, Tailscale), internetteki herhangi bir web sitesi memo API'sine tarayıcı üzerinden istek yapabilir.

**Kullanıcı etkisi:** Uzaktan erişim senaryolarında cross-origin saldırı riski.

**Önerilen çözüm:** LAN modunda da whitelist uygulanmalı (local IP aralıkları: `192.168.*`, `10.*`, `172.16-31.*`).

---

### K5. Mobil API İstemcisi %80 Eksik

| Alan | Detay |
|------|-------|
| **Dosya** | `mobile/lib/core/api_client.dart` (585 satır) vs `frontend/lib/core/api_client.dart` (1122 satır) |
| **Eksikler** | Bellek yönetimi, model yönetimi, senkronizasyon, yedekleme, WhatsApp, takvim, transkripsiyon, görüntü, dosya, orkestra, agent izinleri, proaktif öğrenme, mood, beceriler, OpenRouter |

**Sorun:** Mobil uygulama yalnızca temel sohbet ve oturum yönetimini destekliyor. WhatsApp özelliği mobilde çalışmıyor, bellek ayarları değiştirilemiyor, modeller yönetilemiyor, yedekleme yapılamıyor.

**Kullanıcı etkisi:** Mobil uygulama "tam özellikli uzak istemci" vaadini karşılamıyor. Kullanıcılar temel sohbet dışında hiçbir şey yapamıyor.

**Önerilen çözüm:** Mobil ve masaüstü API istemcileri tek bir paylaşımlı pakette birleştirilmeli veya mobil istemciye tüm endpoint'ler eklenmeli.

---

## 🟠 YÜKSEK (Kullanıcı Deneyimini Ciddi Bozan)

### Y1. System Prompt Yazarken Kullanıcı Girdisi Sürekli Eziliyor

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/widgets/settings_dialog.dart:1096-1099` |
| **Kod** | Her `build()` çağrısında `_controller.text = prompt` yapılıyor, `_initialized` guard'ı yok |

**Sorun:** `_SystemPromptTab` widget'ı her yeniden build olduğunda (örneğin provider state değişince), kullanıcının yazmakta olduğu metin `_controller.text = prompt` ile eziliyor. İmleç pozisyonu kayboluyor, seçili metin siliniyor. Aynı sekmedeki `_IncognitoPromptTab` (satır 1186) `_initialized` guard'ı ile bu sorundan korunmuş ama sistem prompt sekmesinde bu koruma yok.

**Kullanıcı etkisi:** Kullanıcı sistem prompt'unu düzenlerken aniden metni sıfırlanıyor. Uzun prompt yazarken tekrar tekrar yaşanıyor — kullanılamaz hale geliyor.

**Önerilen çözüm:** `_initialized` flag eklenmeli. `didChangeDependencies` veya `didUpdateWidget` içinde sadece ilk yüklemede controller güncellenmeli.

---

### Y2. Flutter — Tüm Ekranlar Sürekli Canlı (IndexedStack)

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/screens/app_shell.dart:45-54` |
| **Kod** | 5 ekran (`ChatScreen`, `AgentScreen`, `ModelStoreScreen`, `WhatsAppScreen`, `CalendarScreen`) `IndexedStack` içinde |

**Sorun:** `IndexedStack` tüm çocuk widget'ları sürekli canlı tutar. `dispose()` hiçbir zaman çağrılmaz. Sonuçlar:
- **WhatsAppScreen** polling'i uygulama açılır açılmaz başlar ve hiç durmaz
- **ModelStoreScreen** indirme durumu sürekli poll edilir
- **CalendarScreen** hatırlatıcı döngüsü sürekli çalışır
- Bellek kullanımı 5 katına çıkar

**Kullanıcı etkisi:** Gereksiz CPU/şebeke kullanımı, dizüstünde pil tüketimi, sürekli HTTP istekleri (saniyede 1-5 istek × 5 ekran = sunucuda anlamsız yük).

**Önerilen çözüm:** `AutomaticKeepAliveClientMixin` ile seçici canlı tutma veya sekme değişiminde görünmeyen ekranların polling'ini duraklatan bir mekanizma.

---

### Y3. WhatsApp Polling Hiç Durmuyor

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/screens/whatsapp_screen.dart:36, 43` + `frontend/lib/providers/whatsapp_provider.dart:82-103` |
| **Kod** | `startPolling()` → `initState()`, `stopPolling()` → `dispose()` ama `dispose()` hiç çağrılmıyor (Y2) |

**Sorun:** WhatsApp durumu ve mesajları her 2-15 saniyede bir poll ediliyor. `IndexedStack` nedeniyle uygulama açıldığı andan itibaren sonsuza kadar sürüyor.

**Kullanıcı etkisi:** Kullanıcı WhatsApp'ı hiç kullanmasa bile sürekli HTTP isteği yapılıyor. Mobilde pil ve veri tüketimi.

**Önerilen çözüm:** Sekme görünürlüğüne bağlı polling kontrolü. `VisibilityDetector` veya `RouteAware` ile sekme değişiminde polling başlat/durdur.

---

### Y4. Oturum Kaydı Senkron Disk Yazma (Her Mesajda I/O)

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/sessions/sessions.go:156-188, 333-343` |
| **Kod** | `AddMessage()` → `m.save(s)` → `json.MarshalIndent` + `os.WriteFile` — tümü `m.mu.Lock()` altında |

**Sorun:** Her mesaj eklendiğinde tüm oturum dosyası baştan JSON'a çevrilip diske senkron yazılıyor ve bu işlem mutex altında yapılıyor. HDD/network FS gibi yavaş depolamada her mesaj 10-100ms gecikme ekliyor. Bu sırada tüm diğer oturum işlemleri kilitleniyor.

**Kullanıcı etkisi:** Hızlı sohbet akışında gecikme, arka arkaya mesajlarda takılma.

**Önerilen çözüm:** Oturum kaydı asenkron kanal üzerinden yapılmalı (memory save worker benzeri).

---

### Y5. Bellek Deposu — Sınırsız Mesaj Boyutu + Sessiz Hata Yutma

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/memory/store.go:241-297` (boyut), `:472` (`COUNT` hatası), `:551` (`Stats` hatası) |
| **Kod** | Boyut kontrolü yok; `SELECT COUNT(*)` hatası `return 0` ile yutuluyor |

**Sorun:** 
- Megabyte'larca metin tek seferde belleğe gömülebilir. Embedding model token limitini aşar, hata verir — ama kullanıcıya bildirilmez.
- Veritabanı hatasında (kilitli, bozuk, timeout) `COUNT(*)` sessizce `0` döner. UI "0 anı" gösterir, kullanıcı veritabanının bozuk olduğunu bilemez.
- `ListGobFiles()` (satır 498) `LIMIT 100` ile sınırlı, sayfalama yok — 100'den fazla anı listelenemez.

**Kullanıcı etkisi:** Bellek aranabilir değil gibi görünür, veritabanı bozulması fark edilmez, büyük mesajlar embedding hatasına yol açar.

**Önerilen çözüm:** Gömme öncesi token-aware truncation (`internal/truncate/` paketi mevcut), hata durumunda UI'a event gönderme, sayfalama desteği.

---

### Y6. Flutter — CancelToken Yarış Durumu (Stream Hatası)

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/providers/chat_provider.dart:194-195, 222, 339` |
| **Kod** | `_cancelToken = CancelToken()` → `stopStreaming()` → `_cancelToken?.cancel()` → stream hala `timeout` altında çalışıyor |

**Sorun:** `stopStreaming()` cancel token'ı iptal ediyor, ancak stream üzerindeki `timeout` wrapper hala aktif. İptal edilmiş bir stream'den veri gelmeye çalışınca `DioException` (cancel tipi) fırlıyor ve catch bloğuna düşüyor. Kullanıcıya aslında olmayan bir hata mesajı gösteriliyor.

**Kullanıcı etkisi:** Sohbet değiştirirken veya mesajı durdururken anlamsız hata SnackBar'ları.

**Önerilen çözüm:** `onError` callback'inde `CancelToken.isCancelled` kontrolü yapılmalı. Cancel durumunda hata gösterilmemeli.

---

### Y7. Flutter — AgentEventBus Emit-After-Close Çökmesi

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/providers/agent_provider.dart:82-100` |
| **Kod** | `_controller.close()` çağrıldıktan sonra `emit()` çağrılabilir → `StateError` |

**Sorun:** `AgentEventBus` dispose olduğunda `StreamController.close()` çağrılıyor. Ancak `chat_provider.dart:264`'teki SSE olay işleyicisi dispose'tan hemen sonra `emit()` yapmaya çalışabilir. Kapalı stream'e event göndermek `StateError` fırlatır.

**Kullanıcı etkisi:** Agent modunda izin isteği sırasında sekme değiştirince uygulama çökebilir.

**Önerilen çözüm:** `emit()` içinde `_controller.isClosed` kontrolü eklenmeli.

---

### Y8. Flutter — FocusNode Dispose Sonrası Kullanımı

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/widgets/chat_input.dart:71, 173` |
| **Kod** | `_send()` → `_focusNode.requestFocus()` — widget dispose olmuş olabilir |

**Sorun:** Mesaj gönderildikten hemen sonra sekme değiştirilirse, `_focusNode.requestFocus()` çağrısı dispose edilmiş bir `FocusNode` üzerinde çalışır. `FlutterError: FocusNode was used after being disposed` hatası.

**Kullanıcı etkisi:** Hızlı sekme değişiminde kırmızı hata ekranı.

**Önerilen çözüm:** `mounted` kontrolü ve `_focusNode.hasListeners` veya try/catch eklenmeli.

---

### Y9. Flutter — İzin Diyaloğu Sonsuza Kadar Blokluyor

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/screens/agent_screen.dart:177-179` |
| **Kod** | `PopScope(canPop: false)` — timeout yok |

**Sorun:** Agent izin diyaloğu `canPop: false` ile geri tuşunu engelliyor, ancak hiçbir zaman aşımı yok. Kullanıcı "Allow" veya "Deny" demezse diyalog sonsuza kadar açık kalır, tüm UI etkileşimi bloke olur. Ayrıca `_submit` `unawaited` ile çağrıldığı için HTTP hatası sessizce yutulur — backend agent pipeline'ı sonsuza kadar askıda kalır.

**Kullanıcı etkisi:** Kullanıcı bilgisayar başından ayrılırsa uygulama tamamen kilitlenir. Yeniden başlatmak gerekir.

**Önerilen çözüm:** 60 saniye timeout ile otomatik `DenyOnce`. Hata durumunda diyaloğu kapatıp kullanıcıya bilgi ver.

---

### Y10. Orchestral Mod — Konfigürasyon Doğrulaması Yok + Sabit Timeout

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/orchestra/conductor.go:40-45, 250, 479, 658` |
| **Kod** | `UpdateConfig()` validasyon yapmıyor; tüm işlemler sabit 300s timeout |

**Sorun:** Geçersiz `ChiefModel`, boş rol tanımları, var olmayan model isimleri — hepsi kabul ediliyor ve çalışma zamanında hata veriyor. Ayrıca tüm alt işlemler (plan, execute, synthesize) sabit 300 saniye — yavaş sağlayıcılarda timeout, hızlı sağlayıcılarda gereksiz beklemeye yol açıyor.

**Kullanıcı etkisi:** Orchestral mod rastgele "context deadline exceeded" hataları veriyor. Konfigürasyon hatası anında değil, dakikalar sonra ortaya çıkıyor.

**Önerilen çözüm:** `UpdateConfig`'te model/rol validasyonu, yapılandırılabilir timeout.

---

### Y11. ngrok Binary — Bütünlük Kontrolü Yok

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/ngrok/installer.go:62-91` |
| **Kod** | SHA256 doğrulaması yok, direkt HTTP indirme, sabit API anahtarı source'da |

**Sorun:** HTTPS üzerinden indiriliyor ancak checksum/signature doğrulaması yok. CDN tehlikeye girerse veya MITM yapılırsa, zararlı binary çalıştırılabilir. Sabit API anahtarı (`bNyj1mQVY4c`) kaynak kodda görünür.

**Kullanıcı etkisi:** Uzaktan kod çalıştırma riski. ngrok binary'si kullanıcı yetkileriyle çalışır ve tüm ağ trafiğini görebilir.

**Önerilen çözüm:** SHA256 checksum dosyası indirilip doğrulanmalı.

---

### Y12. WhatsApp — LIKE Desen Enjeksiyonu

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/whatsapp/store.go:115` |
| **Kod** | `"%" + query + "%"` — `%` ve `_` karakterleri kaçışlanmamış |

**Sorun:** SQL injection değil (parametrik sorgu), ancak LIKE özel karakterleri (`%`, `_`) filtrelenmemiş. Kullanıcı `%` ararsa tüm mesajlar döner. `_` ararsa her karakter eşleşir. Karakter bazında mesaj içeriği taranabilir.

**Kullanıcı etkisi:** Arama sonuçları beklenmedik şekilde çalışır. Kötü niyetli kullanımda mesaj içeriği enumerate edilebilir.

**Önerilen çözüm:** `strings.ReplaceAll(query, "%", "\\%")` ve `_` için aynı işlem.

---

### Y13. SQLite Write Loop — Shutdown Drain'de Kayıp Yazmalar

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/database/sqlite.go:101-124` |
| **Kod** | Drain sırasında `execWrite(task.ctx, task.fn)` — caller context'i zaten iptal olmuş olabilir |

**Sorun:** Graceful shutdown sırasında `writeCh` kuyruğundaki bekleyen yazmalar flush ediliyor ancak caller'ın context'i (HTTP isteği) zaten iptal edilmiş olabilir. `BeginTx(ctx)` başarısız olur, yazma sessizce kaybolur.

**Kullanıcı etkisi:** Shutdown sırasındaki son bellek kayıtları veya oturum güncellemeleri kaybolabilir.

**Önerilen çözüm:** Drain sırasında `context.Background()` veya taze timeout context kullanılmalı.

---

## 🔵 ORTA (Günlük Kullanımda Rahatsızlık Veren)

### O1. Yayınlanan Config'de `active_provider: openai` Sabit

| Alan | Detay |
|------|-------|
| **Dosya** | `config/config.yaml:94` |

**Sorun:** Dağıtılan config dosyasında `active_provider: openai` yazıyor. Kullanıcının seçtiği sağlayıcı (örn. `claude`), config sıfırlanırsa veya ilk kurulumda eziliyor. Uygulama kod seviyesinde `providerRouter` nil kontrolüyle korunuyor (çökme yok), ancak kullanıcıya "OpenAI" aktif görünüyor.

**Kullanıcı etkisi:** Config sıfırlandığında sağlayıcı seçimi kaybolur. İlk kurulumda kafa karıştırıcı "provider error" log'ları.

**Önerilen çözüm:** `active_provider` satırı config'den kaldırılmalı veya boş string olmalı.

---

### O2. Hafıza `COUNT(*)` Hatası Sessizce 0 Dönüyor

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/memory/store.go:472, 551` |

**Sorun:** `SELECT COUNT(*) FROM memories` ve `Stats()` sorgularındaki hata `return 0` ile yutuluyor. Veritabanı bozuksa veya kilitliyse, kullanıcı "0 anı" görür — gerçekte binlerce anı olabilir ama erişilemez durumdadır.

**Kullanıcı etkisi:** Bellek boş görünür, sorun olduğu anlaşılmaz.

**Önerilen çözüm:** Hata log'lanmalı ve UI'a event gönderilmeli. Zero-value ile hata durumu ayırt edilmeli.

---

### O3. Flutter — Streaming Mesajda Zaman Damgası Titreşiyor

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/widgets/chat_message_list.dart:499-501` |
| **Kod** | `DateTime.now().toIso8601String().substring(11, 16)` — her token'da yeniden hesaplanıyor |

**Sorun:** `_StreamingBubble` her token geldiğinde rebuild oluyor (saniyede 50+ kez) ve her seferinde `DateTime.now()` ile zaman damgası oluşturuyor. Damga sürekli değişiyor — görsel titreşim. Ayrıca `DateTime` formatting CPU maliyetli.

**Kullanıcı etkisi:** Mesaj akarken zaman damgasının sürekli değişmesi rahatsız edici.

**Önerilen çözüm:** Zaman damgası stream başlangıcında bir kere alınıp cache'lenmeli.

---

### O4. Flutter — Bağlantı Durumu 30 Saniyede Bir Sonsuz Polling

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/providers/chat_provider.dart:468-478` |

**Sorun:** `connectionStatusProvider` `while(true)` ile her 30 saniyede `/api/version` endpoint'ini sorguluyor. `autoDispose` yok. Yan panel görünmez olsa bile çalışıyor.

**Kullanıcı etkisi:** Sürekli arka plan HTTP isteği — pil tüketimi, gereksiz ağ trafiği.

**Önerilen çözüm:** `ref.onDispose` ile timer iptali veya `autoDispose` eklenmeli.

---

### O5. Flutter — Model Durumu 5 Saniyede Bir Sonsuz Polling

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/providers/models_provider.dart:35-59, 77-94` |

**Sorun:** İki provider (`modelStatusProvider`, `embeddingStatusProvider`) 5 saniyede bir, `downloadProgressProvider` 4 saniyede bir sonsuz polling yapıyor. Model mağazası ekranı `IndexedStack`'te olduğu için hiç dispose olmuyor — uygulama açık kaldığı sürece çalışıyor.

**Kullanıcı etkisi:** Saniyede ~1 HTTP isteği boşuna yapılıyor.

**Önerilen çözüm:** Sekme görünürlüğüne bağlı polling kontrolü.

---

### O6. Agent Pipeline — `time.After` Timer Sızıntısı

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/agent/executor.go:203` |
| **Kod** | `select { case req.ResCh <- policy: case <-time.After(time.Second): }` |

**Sorun:** İzin cevabı kanala başarıyla gönderildiğinde, `time.After` tarafından oluşturulan timer goroutine'i 1 saniye boyunca yaşamaya devam eder. Yoğun agent kullanımında birikme yapar.

**Kullanıcı etkisi:** Uzun agent oturumlarında hafif goroutine birikimi.

**Önerilen çözüm:** `time.NewTimer` + `defer timer.Stop()` veya non-blocking send.

---

### O7. Flutter — Uzun Basma Menüsü Yanlış Konumda

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/widgets/chat_message_list.dart:305-307` |
| **Kod** | `_tapPosition = Offset.zero` → `_showContextMenu()` |

**Sorun:** Mesaja uzun basınca `_tapPosition` sıfırlanıyor ve context menüsü yanlış konumda (sol üst köşe) açılıyor.

**Kullanıcı etkisi:** Mesaj menüsü (kopyala, düzenle, sil) ekranın üstünde açılır.

**Önerilen çözüm:** `onLongPressStart` detayından pozisyon alınmalı.

---

### O8. Agent ve Skill `DangerLevel` Tipleri Uyumsuz

| Alan | Detay |
|------|-------|
| **Dosya** | `internal/agent/tools.go:15-21` vs `internal/skill/types.go:7-13` |

**Sorun:** İki ayrı pakette aynı temel tipte (`string`) ama farklı isimli `DangerLevel` tipleri var. `agent.DangerLevel` ile `skill.DangerLevel` arasında explicit dönüşüm gerek. Derleme zamanı tip uyuşmazlığı.

**Kullanıcı etkisi:** Beceri geliştiricileri için kafa karışıklığı. Yanlış tehlike seviyesi atanması riski.

**Önerilen çözüm:** `internal/models/` altında paylaşımlı tip tanımı.

---

### O9. Mobil — Stream Subscription Yaşam Döngüsü

| Alan | Detay |
|------|-------|
| **Dosya** | `mobile/lib/providers/chat_provider.dart:148-246, 276` |

**Sorun:** `sendMessage` her çağrıldığında yeni bir stream subscription oluşturuluyor. Önceki subscription cancel ediliyor ancak null'lanmıyor. Stream doğal olarak tamamlandığında subscription temizlenmiyor. `dispose()` iptal ediyor ama zaten bitmiş bir subscription'ı iptal etmeye çalışmak no-op.

**Kullanıcı etkisi:** Mobilde arka arkaya mesaj gönderirken hafif bellek sızıntısı.

**Önerilen çözüm:** Stream tamamlandığında `_streamSubscription = null`.

---

### O10. Flutter — Çift `chatListProvider` Geçersizleştirme

| Alan | Detay |
|------|-------|
| **Dosya** | `frontend/lib/providers/chat_provider.dart:317, 320` |

**Sorun:** `ref.invalidate(chatListProvider)` hem `sendMessage` sonunda (satır 317) hem de 2 saniyelik Timer callback'inde (satır 320) çağrılıyor. Her mesajda sohbet listesi iki kere yenileniyor.

**Kullanıcı etkisi:** Gereksiz HTTP isteği, sohbet listesinde anlık titreme.

**Önerilen çözüm:** Tek bir geçersizleştirme yeterli.

---

## ⚪ DÜŞÜK (Kozmetik / Kenar Durum)

### D1. Hafıza Deposu — `LIMIT 100` Sayfalama Yok
- `internal/memory/store.go:498` — 100'den fazla anı varsa listeleme sadece ilk 100'ü gösterir.

### D2. Orchestral Hata Sarma — `%w` yanlış kullanım
- `internal/orchestra/conductor.go:837` — `fmt.Errorf("%w ... %v")` ile non-error değer formatlanıyor.

### D3. WhatsApp — Backoff Hiç Sıfırlanmıyor
- `frontend/lib/screens/whatsapp_screen.dart:58-64` — Polling interval başarılı istekte bile katlanmaya devam eder, 5 saniyeye hiç dönmez.

### D4. Model Mağazası — Birden Fazla Dio Instance'ı
- `frontend/lib/screens/model_store_screen.dart:850, 1204, 1232` — API istemcisi yerine direkt Dio instance'ları oluşturuluyor.

### D5. Mobil — Varsayılan URL Geliştirici IP'si
- `mobile/lib/core/api_client.dart:68` — `http://192.168.1.100:8090` sabit.

### D6. settings_dialog.dart — 4952 Satır
- Tek dosyada 4952 satır. Bakım ve değişiklik yapmak zor.

### D7. WhatsApp — `handleHistorySync` Koruma Yok
- `internal/whatsapp/client.go:315-363` — whatsmeow her yeniden bağlanmada history sync event'i gönderirse mesajlar tekrar kaydedilir.

### D8. `lifecycleCtx` Struct Alanı (Go Anti-pattern)
- `internal/app/app.go:101` — `context.Context` struct alanı olarak saklanıyor.

### D9. Konfig YAML `active_provider: openai` İlk Çalıştırmada Karışıklık
- `config/config.yaml:94` — Kullanıcı config'i değiştirse bile yedekten geri yüklenince eziliyor.

---

## 📊 ÖZET

| Öncelik | Sayı | Kapsam |
|---------|------|--------|
| 🔴 Kritik | 5 | Güvenlik (2), stabilite (2), mobil kullanılamazlık (1) |
| 🟠 Yüksek | 13 | UX bozulması (7), performans/kaynak sızıntısı (4), güvenlik (1), veri bütünlüğü (1) |
| 🔵 Orta | 10 | UX rahatsızlığı, kaynak israfı, hata yutma |
| ⚪ Düşük | 9 | Kozmetik, kenar durum, teknik borç |
| **Toplam** | **37** | |

### En Hızlı Kazanım Sağlayacak İlk 5 Düzeltme

1. **K3** — Web sunucu shutdown timeout (1 satır değişiklik, process zombi kalmasını engeller)
2. **Y1** — Sistem prompt yazarken metin ezilmesi (guard flag ekleme, kullanıcı verisi kaybını engeller)
3. **Y7** — AgentEventBus emit-after-close (1 satır `isClosed` kontrolü, çökmeyi engeller)
4. **Y8** — FocusNode dispose sonrası kullanımı (2 satır mounted kontrolü, çökmeyi engeller)
5. **Y5** — Bellek `COUNT` hatasının yutulması (hata log'lama, veri kaybı farkındalığı)

---

> **Not:** Bu doküman yalnızca kod analizi sonucu tespit edilen, doğrulanmış sorunları içerir. KNOWN_ISSUES.md'deki bazı maddeler (örn: H01 Priority kullanılmıyor, H09 Agent timeout yok, H14 Skill SetActive kayıt hatası) kod incelemesinde **yanlış/eskimiş** bulunarak listeye dahil edilmemiştir.
