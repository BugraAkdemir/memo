# Memo Yol Haritası — Stratejik Vizyon

Gizlilik odaklı, yerel öncelikli bir yapay zeka asistanı. Tüm özellikler temel prensibe bağlı kalır: **verileriniz izniniz olmadan cihazınızdan asla çıkmaz.**

---

## ✅ v3.1.1 — Açık Beta (Güncel, 2026-07-04)

v3.1 serisinin ilk açık beta sürümü, Windows ve Linux için 2-3 hafta içinde hedeflenen stable sürüm öncesi geri bildirim toplama turu. Aşağıdaki her şeyi ve zaten koda göre doğrulanmış v3.2.0 maddelerini içeriyor (o bölümdeki nota bakın).

---

## ✅ v3.1.0 — "Hafıza" (Yayınlandı)

**Tema:** Kalıcı bellek, yerel embedding, çapraz-mod mimari, WhatsApp temeli, mobil uygulama, uzaktan erişim.

### Temel Özellikler
- SQLite + sqlite-vec vektör deposu (ANN index ile, chromem-go kaldırıldı)
- Cross-mode: harici API sohbet + yerel embedding bağımsız çalışır
- WhatsApp entegrasyonu: QR eşleştirme, çift yönlü mesajlaşma, kişi adı çözümleme, beyaz liste dosya aktarımı, agent araçları
- Yedekleme/Restore (.memo): tam dışa/içe aktarma, silme desteği
- Mobil uygulama: Android/iOS için ince Flutter istemci (LAN/ngrok bağlantısı, temel sohbet, sağlayıcı/model kontrolü)
- Uzaktan erişim: ngrok tünel desteği
- Windows desteği: 8 uyumluluk düzeltmesi, Inno Setup installer
- Kurulum sihirbazı yenilendi: 6 karakter ön tanımlı prompt
- Hafıza hata düzeltmeleri: dosya boyutları, L10n, hata yönetimi

---

## 🚧 v3.2.0 — "Zamanlanmış Zeka"

**Tema:** Takvim, hatırlatıcı, cron zamanlama ve tam donanımlı agent arayüzü ile proaktif otomasyon.

### 📅 Takvim & Hatırlatıcılar

**Amaç:** Memo'nun kendi takvimi olsun. Kullanıcı doğal dil ile olay ekleyebilsin, silebilsin, düzenleyebilsin. Memo otonom olarak hatırlatıcı ve tekrarlı görevler oluşturabilsin.

#### Doğal Dil ile Olay Ayrıştırma
- Herhangi bir sohbet bağlamından tarih/saat çıkarma (WhatsApp dahil): *"Memo, yarın saat 10'da dişçi randevum var"* → olay oluşturma
- Yapay zeka destekli ayrıştırma + regex fallback
- Tekrarlı desen desteği: *"her hafta pazartesi 9'da toplantı"*

#### Takvim Deposu (`internal/calendar/`)
- SQLite tabanlı olay depolama (`data/calendar/events.db`)
- Tam CRUD: oluşturma, okuma, güncelleme, silme
- Zengin olay modeli: başlık, açıklama, başlangıç/bitiş, tüm gün, kategori (toplantı/görev/doğum günü/hatırlatıcı), tekrarlama kuralları, kaynak (kullanıcı/memo)
- Tekrarlanan olay genişletme (günlük, haftalık, aylık, yıllık)

#### Cron Motoru (`internal/scheduler/`)
- Doğal konuşma ile tekrarlı görev tanımlama: *"Memo, her gün saat 10'da günaydın yaz"*
- WhatsApp mesajları, sistem komutları veya özel agent eylemleri zamanlama
- Tam cron ifade desteği + hazır ön tanımlı kalıplar
- Uygulama yeniden başlatmalarında görev kalıcılığı
- Yürütme günlükleri ve hata bildirimleri
- Masaüstü bildirimleri (libnotify/Windows toast)

#### Takvim API
| Endpoint | Method | Açıklama |
|----------|--------|----------|
| `/api/calendar/events` | GET | Olayları listele (tarih aralığı filtresi) |
| `/api/calendar/events` | POST | Yeni olay ekle |
| `/api/calendar/events/:id` | PUT | Olay düzenle |
| `/api/calendar/events/:id` | DELETE | Olay sil |
| `/api/calendar/natural` | POST | Doğal dil → olay |
| `/api/calendar/today` | GET | Bugünkü olaylar |
| `/api/scheduler/tasks` | GET/POST/DELETE | Cron görev yönetimi |

#### Flutter Takvim Arayüzü
- Aylık genel bakış widget'ı (olay göstergeli)
- Günlük/haftalık liste görünümü
- Olay ekleme/düzenleme formu
- Memo tarafından eklenen olaylar farklı renkte
- Sohbet içinden hızlı olay oluşturma

---

### 🤖 Agent Arayüzü Tamamlama

**Amaç:** Agent motoru backend'te tamamen çalışıyor — şimdi kullanıcıların doğal bir şekilde etkileşime girebileceği bir frontend arayüzü gerekiyor.

#### Agent Modu Açma/Kapama
- Sohbet ekranı başlığında kalıcı toggle (açık/kapalı)
- Agent modu aktifken görsel gösterge
- Klavye kısayolu ile hızlı geçiş
- Varsayılan: kapalı (isteğe bağlı)

#### İzin Diyaloğu
- Agent bir araç çalıştırmak istediğinde gerçek zamanlı popup
- Net gösterim: araç adı, yapılacak işlem, tehlike seviyesi (Güvenli/Orta/Tehlikeli)
- Eylem butonları: Bir Kez İzin Ver, Bu Oturumda İzin Ver, Sürekli İzin Ver, Bir Kez Reddet, Sürekli Reddet
- Çalıştırılmadan önce tam komut/argüman gösterimi
- Zaman aşımlı otomatik reddetme (30 saniye)

#### Araç Çağrı Kartları
- Araç yürütme ilerlemesini gösteren gerçek zamanlı kartlar
- Her araç çağrısı daraltılabilir kart olarak:
  - Araç ikonu + adı
  - Durum göstergesi (bekliyor/çalışıyor/başarılı/başarısız/reddedildi)
  - Argüman önizleme (genişletilebilir)
  - Yürütme çıktısı (stdout/stderr, genişletilebilir)
  - Süre rozeti
- Akıcı kart geçiş animasyonları

#### Agent Sohbet Modu
- Normal sohbetten ayrı görsel mod
- Agent modundayken sistem promptu görünür
- Bağlam: seçili proje dizini gösterilir
- Token/oturum kullanım istatistikleri

#### API Client Entegrasyonu
- `api_client.dart` — tüm agent endpoint'leri
- `agent_enabled`, `agent_permissions`, `agent/respond` endpoint'leri
- Gerçek zamanlı araç yürütme için SSE olay akışı
- Hata yönetimi ve yeniden bağlanma mantığı

#### Kurulum Sihirbazı Entegrasyonu
- İsteğe bağlı "Agent Modu Etkinleştir" adımı
- Varsayılan izin politikası seçimi
- Proje dizini yapılandırması

---

### 📱 Mobil Bildirimler

**Amaç:** Takvim hatırlatıcıları ve zamanlanmış görevler mobil uygulamaya push bildirim gönderebilsin.

#### Push Bildirim Aktarımı
- Masaüstü → mobil bildirim aktarımı (WebSocket veya polling)
- Takvim hatırlatıcılarının telefona iletilmesi
- Zamanlanmış görev tamamlanma/başarısızlık bildirimleri
- Kategori bazlı yapılandırılabilir bildirimler (hatırlatıcı, görev, sistem)
- Sessiz/öncelikli mod

#### Mobil Arayüz Güncellemeleri
- Mobil ayarlarda bildirim geçmişi görünümü
- Dokunarak ertele veya kapat
- Uygulama ikonunda rozet sayısı

---

## 🚧 v3.3.0 — "Mobil & Ses" *(Planlanan)*

**Tema:** Tam donanımlı mobil deneyim ve eller-serbest sesli etkileşim. Tüm AI işlemleri masaüstünde kalır.

### Mobil Uygulama v2

**Amaç:** Mobil uygulama ince bir görüntüleyiciden tam yetenekli bir uzak istemciye dönüşüyor.

#### Özellik Eşitliği
- **RAG Bellek Tarama** — hafıza dosyalarını mobilde görüntüle, ara, sil
- **WhatsApp Arayüzü** — QR eşleştirme, mesaj gönder/al, kişi listesi, sohbet görünümü
- **Takvim Görünümü** — aylık genel bakış, olay oluşturma, hatırlatıcı yönetimi
- **Agent Kontrolü** — izin yanıtları, araç yürütme takibi, mod açma/kapama
- **Ayarlar** — masaüstünden tam ayar senkronizasyonu (sağlayıcılar, modeller, orkestra, agent)

#### Mobil Kullanıcı Deneyimi
- **Biyometrik Kimlik Doğrulama** — Face ID / parmak izi ile uygulama erişimi
- **Çevrimdışı Mesaj Kuyruğu** — çevrimdışıyken mesaj yaz, bağlantı gelince otomatik gönder
- **Push Bildirim Aktarımı** — tüm olay türleri mobile iletilir (hatırlatıcı, görev, mesaj)
- **Bağlantı Durumu** — kalıcı gösterge, otomatik yeniden bağlanma

#### Uzaktan Erişim
- **Tailscale-native bağlantı** — Tailscale kullanıcıları için sıfır yapılandırma
- **TLS + parola** — Tailscale olmayan uzak erişim için
- **Uçtan uca şifreli kanal** — mobil ve masaüstü arası tüm iletişim
- **Üstel geri bildirimli otomatik yeniden bağlanma**

### 🎤 Sesli Asistan

**Amaç:** Memo ile eller-serbest etkileşim — "Hey Memo" uyandırma sözcüğü, konuşmadan metne, metinden konuşmaya.

#### Uyandırma Sözcüğü (Wake Word)
- Yapılandırılabilir "Hey Memo" ile hassasiyet ayarı
- Yerel wake word motoru — bulut bağımlılığı yok
- Düşük güçte arka plan dinleme, mikrofon soğuma süresi
- Dinleme aktifken görsel gösterge

#### Konuşmadan Metne (STT)
- **whisper.cpp** entegrasyonu ile tamamen yerel ses tanıma
- İsteğe bağlı bulut STT yedeği (OpenAI Whisper API)
- Türkçe dil optimizasyonu (whisper small/turbo)
- Sohbette gerçek zamanlı akış transkripsiyonu
- Mobil için sesli mesaj kaydı (ses gönder → masaüstünde çözümle)

#### Metinden Konuşmaya (TTS)
- Yerel TTS motoru ile sesli yanıtlar
- Yapılandırılabilir ses ve hız ayarı
- Gelen mesajlar için otomatik okuma seçeneği
- Hands-free modu: sor → dinle → işle → yanıtla döngüsü

#### Ses Pipeline'ı
- Uçtan uca: "Hey Memo, Buğra'ya mesaj gönder akşam yemeğe geliyorum de"
- Uyandır → dinle → yazıya çevir → LLM işle → çalıştır → TTS yanıtla
- Ses modunda tam konuşma belleği
- Google Asistan / Siri tarzı eller-serbest etkileşim modeli

---

## 🚧 v3.4.0 — "Eklenti & Web" *(Planlanan)*

**Tema:** Topluluk araçları için eklenti mimarisi + local modeller için canlı web arama.

### 🔌 Eklenti Sistemi

**Amaç:** Herkes Memo'nun çekirdeğini değiştirmeden kendi araçlarını yazabilsin.

#### Mimari
- **Go plugin interface**:
  ```go
  type PluginTool interface {
      Name() string
      Description() string
      Execute(ctx PluginContext, args map[string]any) (string, error)
      DangerLevel() agent.DangerLevel
  }
  ```
- Plugin'ler `.so` olarak derlenir (`go build -buildmode=plugin`)
- `plugins/` dizininden startup'ta otomatik keşfedilir
- Agent tool registry'sine otomatik kaydolur
- Permission sistemi out-of-the-box çalışır

#### Go Plugin Modu (v1)
- Ana binary ile aynı Go version
- Paylaşılan dependency kısıtı (minimal bağımlılık)
- İsteğe bağlı hot-reload
- Built-in örnek plugin'ler: `echo.so`, `hello.so`

#### gRPC Plugin Modu (v2 — gelecek)
- Plugin ayrı process'te çalışır
- mTLS ile gRPC haberleşmesi
- Dil bağımsız — Python, Rust, Node.js ile yazılabilir
- Process başına kaynak limiti (CPU, memory, süre)
- Plugin health check + crash'te otomatik restart

#### Plugin Yönetimi
- `GET /api/plugins` — yüklü plugin'leri listele
- `POST /api/plugins/install` — URL'den plugin indir
- `DELETE /api/plugins/:name` — plugin kaldır
- Plugin manifest: `plugin.json` (isim, versiyon, yazar, açıklama, tehlike seviyesi)
- Topluluk plugin index URL'si (yapılandırılabilir)

#### Market (gelecek)
- GitHub'da topluluk plugin dizini
- Tek komut kurulum: `/plugin install github/user/memo-plugin-name`
- İmzalı plugin doğrulaması (isteğe bağlı GPG)

---

### 🌐 Local Model için Web Arama

**Amaç:** Local modellerin agent tool calling ile gerçek zamanlı internette arama yapabilmesi.

#### Nasıl Çalışır
- Local model güncel bilgi gerektiren bir soru alır: *"Bugün dolar ne kadar?"*
- Model `web_search` tool'unu çağırır
- Tool bir arama API'siyle sorguyu çalıştırır
- Sonuçlar modele context olarak döner
- Model kaynaklarıyla birlikte doğal bir cevap üretir

#### Mimari
- `internal/agent/tools/websearch.go` — yeni built-in tool
- Yapılandırılabilir arama sağlayıcıları:
  - **Bing Search API** (birincil, ucuz, Türkçe iyi)
  - **SerpAPI** (Google sonuçları)
  - **DuckDuckGo** (ücretsiz, API anahtarı gerekmez)
  - **SearXNG** (kendi sunucun, gizlilik odaklı)
- Config: `search_provider`, `api_key`, `max_results`, `region`
- Rate limiting ve cache ile gereksiz sorguları önleme
- Sonuç formatı: başlık, snippet, URL

#### Kullanıcı Deneyimi
- Web arama **Agent Mode** içinde çalışır — model ne zaman arayacağına karar verir
- Manuel tetik: `/search bugün dolar ne kadar`
- Sonuçlar agent UI'da tool call kartı olarak gösterilir
- Doğrulama için kaynak URL'leri cevaplarda görünür

#### Agent Entegrasyonu
- `web_search` varsayılan tool registry'ye kayıtlı
- Permission sistemi uygulanır (auto-allow yapılabilir)
- Çok turlu araştırma için takip sorguları
- Web arama sonuçları hafıza bağlamından ayrı tutulur

---

## 🔮 v3.5.0 — "Smarter Memo" *(Gelecek)*

**Tema:** Memo pasif bir asistan olmaktan çıkıp kendini geliştiren, bağlamı anlayan, bilgileri arasında iliki kuran ve proaktif hareket eden bir zeka haline geliyor.

### 🧠 Bilgi Grafiği

**Amaç:** Düz hafıza kayıtlarını birbirine bağlı bir bilgi ağına dönüştürmek. Memo konuşmalar, kişiler, projeler ve kavramlar arasındaki ilişkileri anlar.

#### Grafik Motoru (`internal/knowledge/`)
- **Varlık çıkarımı** — konuşmalardan otomatik olarak kişi, yer, proje, teknoloji, tarih çıkarma (local NER veya LLM tabanlı)
- **İlişki haritalama** — varlıklar arasında anlamsal bağlantı keşfi: "Buğra → çalışıyor → Memo", "Memo → kullanıyor → sqlite-vec"
- **Grafik veritabanı** — hafif SQLite tabanlı grafik deposu (adjacency list + node properties)
- **Artımlı indeksleme** — yeni konuşmalar gerçek zamanlı grafiği günceller, tam yeniden inşa gerekmez

#### Kullanıcı Etkileşimi
- **Grafik görselleştirme** — Obsidian tarzı interaktif grafik (masaüstü ve mobil)
  - Düğüm boyutları varlık önemini yansıtır (frekans + güncellik)
  - Kenar kalınlığı ilişki gücünü yansıtır
  - Düğüme tıkla → ilgili konuşmaları göster
  - Varlık türüne, tarih aralığına, alaka düzeyine göre filtrele
- **Doğal dil grafik sorguları**: *"Memo, Buğra'nın üzerinde çalıştığı projeleri göster"* → grafik vurguları + liste
- **Konuşma bağlamı** — bir konuyu tartışırken Memo grafikten ilgili varlıkları yüzeye çıkarır

#### Hafıza Geliştirme
- Grafik RAG getirimini zenginleştirir: hafıza ararken sorgu konusuyla bağlantılı varlıkları dahil et
- *Örnek:* kullanıcı "vektör deposu" hakkında sorar → grafik "sqlite-vec", "ANN index", "embedding model" ile ilgili anıları getirir
- Konuşmalara çıkarılan varlıklara göre otomatik etiket oluşturma

---

### 🤖 Kendini Geliştiren Zeka

**Amaç:** Memo kullanıcı etkileşimlerinden öğrenir ve manuel yapılandırma olmadan zamanla iyileşir.

#### Geri Bildirimle Öğrenme
- **Açık geri bildirim**: kullanıcı düzeltir → "hayır yanlış oldu" → Memo düzeltmeyi kaydeder, aynı hatayı tekrarlamaz
- **Örtük geri bildirim**: kullanıcı mesaj düzenler → Memo düzenlemeyi analiz eder, neyin yanlış olduğunu anlar
- **Tekrar eden desen tespiti**: kullanıcı sürekli belirli önerileri reddediyorsa Memo onları yapmayı bırakır
- **Düzeltme deposu** (`data/feedback/`): SQLite tabanlı, düzeltmeleri bağlamıyla saklar

#### Sistem Promptu Otomatik İyileştirme
- Memo hangi cevap stillerinin olumlu tepki aldığını analiz eder
- Periyodik olarak prompt iyileştirmeleri önerir: *"Son konuşmalara bakınca daha kısa cevap vermeni tercih ediyorsun. Stili güncellememi ister misin?"*
- Düşük riskli etkileşimlerde A/B testi
- Sürümlenmiş prompt geçmişi ve geri alma desteği

#### Kullanım Deseni Analizi
- Kullanıcı rutinlerini tanımlar: *"Her pazartesi saat 9'da kod yazmaya başlıyorsun"*
- Kullanıcının iş akışına göre proaktif optimizasyon
- Gözlemlenen davranışa dayalı otomasyon önerileri

---

### 🧹 Akıllı Hafıza

**Amaç:** Hafıza pasif bir kayıttan çıkıp kendi kendini organize eden bir bilgi tabanına dönüşür.

#### Otomatik Hafıza Budama
- Eski hafıza tespiti: X günden eski ve hiç getirilmemiş kayıtlar → otomatik arşiv
- Kopya tespiti: benzer konuşmalar tek bir kayıtta birleştirilir
- Düşük değerli filtreleme: "merhaba", "teşekkürler" gibi mesajlar hafızaya kaydedilmez
- Yapılandırılabilir saklama politikaları: yaşa, alaka düzeyine, depolama kotasına göre

#### Hafıza Birleştirme
- Haftalık birleştirme: benzer anılar özet kayıtlarda birleştirilir
- *Örnek:* "proje mimarisi" hakkında 5 ayrı konuşma → tek bir birleştirilmiş kayıt
- Orijinal kayıtlar arşivlenir, birleştirilmiş kayıt getirimde öne çıkar
- Kullanıcı birleştirilmiş ve ham hafıza arasında geçiş yapabilir

#### Anlamsal Organizasyon
- Otomatik kategorizasyon: anılar konuya, projeye, kişiye göre gruplanır
- Konu hiyerarşisi: "Memo > Geliştirme > Backend > SQLite" tarzı ayrıştırma
- Tüm hafıza metaverisi üzerinde tam metin arama (konular, varlıklar, kategoriler)
- Masaüstü ve mobil hafıza tarayıcısı

---

### 💡 Proaktif Öneriler

**Amaç:** Memo sorulmayı beklemez — ihtiyaçları öngörür ve kullanıcı sormadan yardım sunar.

#### Desen Tanıma
- Zaman bazlı desenler: *"Her cuma akşamı haftalık rapor yazıyorsun — başlayalım mı?"*
- Konuşma bazlı desenler: *"Geçen sefer deploy sonrası hata almıştın, testleri koşayım mı?"*
- Olay bazlı desenler: *"Takvimde 10 dakika sonra toplantın var, hazırlık notlarını özetleyeyim mi?"*

#### Bildirim Kanalları
- Sohbet içi öneriler (rahatsız etmeyen, tek kapatmalık)
- Masaüstü bildirimi (yapılandırılabilir)
- Mobil push bildirimi
- Yapılandırılabilir proaktivite seviyesi: Kapalı / Hafif / Normal / Israrlı

#### Öneri Türleri
- **Görev hatırlatıcıları**: konuşma geçmişinden tespit edilen tekrarlı görevler
- **Bilgi yüzeye çıkarma**: "Geçen ay benzer bir sorunu çözmüştün, çözümü hatırlatayım mı?"
- **Otomasyon önerileri**: "Her push'ta test koşuyorsun, bunu CI'a eklemek ister misin?"
- **Öğrenme önerileri**: "Son 3 konuşmanda aynı hatayı düzelttin, bunu kalıcı olarak öğreneyim mi?"

---

### 🧩 Çok Adımlı Akıl Yürütme

**Amaç:** Memo karmaşık, çok parçalı soruları planlayarak, alt görevleri çalıştırarak ve sonuçları sentezleyerek tek bir konuşma adımında yanıtlar.

#### Akıl Yürütme Pipeline'ı
- **Plan aşaması**: LLM soruyu alt adımlara böler (*"önce hava durumunu bul, sonra etkinlik öner"*)
- **Yürütme aşaması**: her alt adım uygun aracı çağırır (web arama, hafıza, kod çalıştırma)
- **Sentez aşaması**: tüm alt adımlardan gelen sonuçlar tutarlı bir cevapta birleştirilir
- **Doğrulama aşaması**: model cevabın orijinal soruyu karşıladığını kontrol eder

#### Entegrasyon
- Agent Mode içinde çalışır (tool calling)
- Alt adım sonuçları agent UI'da genişletilebilir kartlar olarak gösterilir
- Kullanıcı akıl yürütme zincirini görebilir ve her adımı doğrulayabilir
- Bağımsız alt adımlar paralel çalıştırılır
- Zaman aşımı ve kısmi sonuç yönetimi

#### Kullanım Senaryoları
- *"İstanbul'da bu hafta sonu ne yapabilirim?"* → hava durumuna bak → etkinlik ara → hafta sonu planı oluştur
- *"Memo'nun performansını nasıl optimize ederim?"* → config kontrol et → hafıza boyutunu analiz et → iyileştirme öner
- *"Bu hatayı nasıl çözerim?"* → hafızada ara → web'de ara → çözümleri birleştir

---

## Temel İlkeler

| İlke | Açıklama |
|------|----------|
| **Yerel öncelikli** | Her özellik çevrimdışı çalışır. Bulut bağımlılığı yoktur. |
| **Gizlilik tasarım gereği** | Veriler, siz açıkça izin vermediğiniz sürece cihazınızdan çıkmaz. |
| **Kullanıcı sahipliği** | Verilerinizin, modelinizin ve ince ayarınızın kontrolü sizdedir. |
| **Aşamalı karmaşıklık** | Özellikler, asistanla birlikte büyüdükçe kendini gösterir. |
| **Açık kaynak** | AGPL v3 — inceleyin, değiştirin, yeniden dağıtın. |

---

> **Lejant:** ✅ Yayınlandı | 🚧 Geliştirme Aşamasında | 🔮 Gelecek  
> **Güncel sürüm:** v3.1.0-beta  
> **Depo:** [github.com/BugraAkdemir/memo](https://github.com/BugraAkdemir/memo)
