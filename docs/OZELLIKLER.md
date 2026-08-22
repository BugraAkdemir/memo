# Memo — Kapsamlı Özellik Kataloğu

Bu döküman, **Memo Yapay Zeka Hafıza Kabuğu** içinde entegre edilmiş olan her bir özelliğin detaylı dökümünü sunar. Arka plandaki ikili veri sürekliliğinden, multimodal (görsel/ses) yeteneklere kadar Memo'nun yerel yapay zeka deneyiminizi nasıl güçlendirdiğini keşfedin.

---

## 1. 🧠 Temel Zeka ve Hafıza

### Kalıcı RAG (Geri Getirme Destekli Nesil)

Memo sadece bir sohbet aracı değil, bir "İkinci Beyin"dir.

- **Anlamsal İndeksleme**: Her etkileşim otomatik olarak vektörleştirilir (embedding) ve yerel bir vektör veritabanında saklanır.
- **Hibrit Arama**: Arama, vektör benzerliğini FTS5 anahtar kelime aramasıyla birleştirir (Reciprocal Rank Fusion ile birleştirilir) — böylece kısa, kesin bir gerçek sadece "yeterince yakın" anlamsal benzerliğe bağlı kalmaz.
- **Çok-Konulu Soru Bölme**: Çok konulu bir soru ("adım, doğum günüm ve favori rengim ne") bağlaçlara göre bölünür, her konu tek bir harmanlanmış embedding'e sıkışmak yerine kendi aramasını alır.
- **Bağlamsal Hatırlama**: Her yanıttan önce Memo, en ilgili geçmiş konuşmaları geri getirmek için bu hibrit aramayı yapar (Top-K eşleşmesi).
- **Sabitlenmiş Gerçekler (2026-07-15)**: Kalıcı kişisel gerçekler (isim, doğum günü, evcil hayvan vb.) — ister `/remember` ile kaydedilsin ister normal sohbetten otomatik tespit edilsin — her promptta koşulsuz olarak enjekte edilir, arama sıralamasını tamamen atlar, böylece rutin sohbet arasında asla kaybolmaz.
- **Sonsuz Bağlam**: Uzun süreli hafıza, yapay zekanın haftalar veya aylar önceki detayları, modelin mevcut pencere sınırından bağımsız olarak hatırlamasını sağlar.

### Modelden Bağımsız Motor

- **Dahili Llama-Server**: Yüksek performanslı GGUF çıkarımı için `llama.cpp` tarafından desteklenir.
- **Özel Embedding Sunucusu**: Hafıza indeksleme için özel olarak çalışan ikinci bir dahili sunucu, ana sohbet performansını etkilemeden çalışır.
- **Harici Sağlayıcı Desteği**: LM-Studio veya herhangi bir OpenAI uyumlu yerel API'ye (Port 1234/8081) sorunsuz bağlanır.

---

## 2. 🏛️ Mimari ve Veri Sürekliliği

### SQLite + sqlite-vec Vektör Deposu

- **Yüksek Performans**: SQLite + sqlite-vec ile hızlı vektör arama ve ANN (Approximate Nearest Neighbor) indeksi.
- **Atomik Yazma**: SQLite'in ACID özellikleri sayesinde veri bütünlüğü garanti altındadır.
- **Gecikmeli Yükleme (Lazy Loading)**: Veriler yalnızca anlamsal olarak ilgili olduğunda diskten okunur, bu da yıllarca süren geçmişte bile RAM kullanımını minimumda tutar.

### Gizlilik ve Yerel İzolasyon

- **%100 Çevrimdışı**: Hiçbir veri bilgisayarınızdan dışarı çıkmaz. Telemetri yok, log gönderimi yok, bulut bağımlılığı yok.
- **Güvenli Yerel Depolama**: Zihniniz kendi donanımınızda kalır.

### `.memo` Yedekleme — Artık Gerçekten Eksiksiz

- Tam dışa aktarma artık takvim olayları, öğrenilen alışkanlıklar, rutinler, görev listeleri, agent tool izinleri, kurulu skill'ler **ve `machine.key`** dahil her şeyi içeriyor. `machine.key` eksikliği önceden bir yedeği başka bir makinede geri yüklediğinde tüm sağlayıcı API anahtarlarını kalıcı olarak çözülemez hale getiriyordu — düzeltildi.

### Uzaktan Erişim Sunucusu

- **Yerel Ağ Köprüsü**: Ayarlardan "Uzaktan Erişim"i etkinleştirerek, aynı Wi-Fi üzerindeki diğer cihazlardan (telefon, tablet vb.) yerel Memo'nuzla sohbet edebilirsiniz.
- **Tailscale artık Beta değil**: Tek tıkla giriş (auth key yapıştırmaya gerek yok), Funnel varsayılan açık, kopan bağlantıdan sonra otomatik yeniden bağlanma — Settings → Remote Access'te doğrudan, masaüstü ve mobilde.
- **Token zorunlu**: LAN/ngrok/Tailscale üzerinden yapılan her uzak istek artık Settings'te gösterilen erişim token'ını gerektiriyor — önceden isteğe bağlıydı, bu da aynı ağdaki herkesin API anahtarlarını okuyabilmesi gibi gerçek bir açıktı. Sadece yerel kullanım (varsayılan) etkilenmiyor.
- **Çoklu Hesap, Admin/Kullanıcı Rolleri**: Paylaşımlı bir self-hosted sunucu artık birden fazla girişi barındırabiliyor — bir admin ve istediğin kadar kullanıcı hesabı, her biri kendi şifresiyle. Settings → Accounts'tan veya SSH üzerinden `memo remote list-accounts`/`add-account`/`delete-account` ile yönetilir.
- **Hesap Bazlı Ayrıntılı İzinler**: Her hesap için 7 bağımsız izin anahtarı (Models, Memory, Agent, Calendar, WhatsApp, Telegram, Routines) — bir admin, mesela bir kullanıcının sohbet edip Agent araçlarını kullanmasına izin verirken Model Store/API Providers sekmelerini gizleyip hafıza yazmasını tamamen engelleyebilir. Sadece arayüzde gizlemekle kalmaz, backend'de de zorlanır; Settings → Accounts'ta checkbox arayüzü var.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Araması

- **Doğrudan Depo Erişimi**: Hugging Face üzerindeki modelleri uygulama içinden direkt arayın.
- **Repo ID Desteği**: Herhangi bir Hugging Face GGUF depo kimliğini yapıştırarak mevcut dosyaları anında listeleyin.

### Sistem Teşhisi

- **VRAM ve GPU Kontrolü**: Kullanılabilir NVIDIA/AMD VRAM miktarını otomatik algılar.
- **Uyumluluk Rozeti**: Modelleri indirmeden önce "GPU Uyumlu" veya "⚠️ VRAM Yetersiz" olarak işaretler.

### Arka Plan İndirme Yöneticisi

- **Paralel İndirme**: Artık aynı anda birden fazla GGUF indirmesi çalışabiliyor (önceden ikinci indirme reddediliyordu), toplam ilerleme motor durum çubuğunda; kaba bir süre tahmini de gösteriliyor.
- **Yaşam Döngüsü Kontrolü**: Yerel modeller için tek tıkla Başlat, Durdur ve Güncelle seçenekleri.
- **Donanıma Göre İlk Öneri**: Kurulum, RAM/GPU'nuzu okuyup eşleşen bir sohbet + hafıza model çiftini önerir, tek butonla ikisini birden indirmeye başlatır.
- **Güvenli Context Boyutu**: Context alanı artık modelin GGUF dosyasından okunan gerçek maksimumunu aşamıyor (önceden serbest metindi, motoru çökertebiliyordu).
- **Sade Dilli Hatalar ve İpuçları**: `llama: server failed to become ready within 120s` gibi ham hatalar yerine kısa, eyleme geçirilebilir mesajlar; donanım-uyum ve kuantizasyon rozetlerinde açıklayıcı ipuçları.
- **Discover Filtreleri**: Tools/Vision/Code/Embedding/Size filtreleri artık VE değil VEYA ile birleşiyor, çoklu-seçim dropdown'larına taşındı.

---

## 4. ⚡ Etkileşim ve Kullanıcı Deneyimi ▊

### Canlı Mod (Streaming)

- **Token-Bazlı Yazma**: Yapay zekanın yanıtlarını gerçek zamanlı olarak "yazmasını" izleyin.
- **Düşünme Durumu**: İlk token gelmeden önce yanıp sönen "Memo düşünüyor..." durumu görsel geri bildirim sağlar.
- **İmleç Tasarımı**: Akışı takip eden terminal tarzı bir imleç (`▊`).

### Gizli Mod (Incognito)

- **Sıfır Kalıcılık**: Hassas oturumlar için hafıza kaydını ve geçmiş loglarını devre dışı bırakan güvenli bir geçiş anahtarı.
- **Uçucu Bağlam**: Bağlam sadece o oturumda yaşar ve kapatıldığında tamamen silinir.

### Performans Paneli (HUD)

- **Gerçek Zamanlı İstatistikler**: Zaman damgasının üzerine gelerek üretim hızını (tok/s), toplam token miktarını ve süre metriklerini görün.

### Sesli Mod — Eller Serbest Konuşma (Beta)

- Sohbet kutusunun yanındaki küçük ses ikonu (Settings → Beta Features açıkken) — ayrı bir sidebar sekmesi değil. Konuşmanı dinler, ne zaman başlayıp bittiğini otomatik algılar, yerelde yazıya döker, normal bir sohbet mesajı olarak gönderir ve yanıtı sesli okur.
- **Varsayılan olarak tamamen yerel**: cihaz-üstü transkripsiyon + yerel **Piper** TTS. İstersen harici bir OpenAI TTS sağlayıcısı da yapılandırabilirsin; yerel Piper her zaman yedek olarak devrede kalır.
- **Çevrimdışı ses seçici**: küçük, elle seçilmiş bir Piper ses koleksiyonunu (TR/EN) indirip anında geçiş yapabilirsin.
- **Tek yönlü barge-in**: Memo konuşurken tekrar konuşursan durup seni dinler.
- **Bilinen sınırlama**: henüz yankı iptali yok — hoparlör kullanmak Memo'nun bazen kendi sesini kesme sanmasına yol açabilir.

### @ Dosya Bahsetme

- Herhangi bir sohbette `@` yazmak dosya adına göre arayıp referans gösterebileceğin bir liste açar — agent modunu tam yol yazmadan belirli bir dosyaya yönlendirmek için kullanışlı.

### WhatsApp Entegrasyonu

- **QR ile Eşleştirme**: Uygulama içinde gösterilen QR kod ile tam WhatsApp Web çoklu cihaz eşleştirmesi.
- **Çift Yönlü Mesajlaşma**: Kişi bilgisiyle birlikte mesaj gönderme ve alma.
- **Kişi Çözümleme**: Rehber senkronizasyonu, push name, yoksa telefon numarasına düşer.
- **Whitelist Dosya Transferi**: Güvenilir kişiler whitelist'teki dizinlerden dosya isteyebilir.
- **Agent Araçları**: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`.
- **Ayrı Sohbet Modu**: WhatsApp'a özel izole executor ve tool registry.
- **Kendine-Sohbet Asistanı**: QR girişinde eşleştirdiğin kendi WhatsApp numarana mesaj at, Memo tam bir asistan olarak yanıtlasın — sohbet, hafıza ve agent araçları hepsi Memo'yu açmadan telefonundaki WhatsApp'tan erişilebilir.
- **Sohbetten Rutin Oluşturma**: Düz dille iste ("her sabah 8'de bana hava durumunu hatırlat") ve Memo, uygulama içindeki aynı `create_routine`/`list_routines`/`cancel_routine` agent araçlarını kullanarak sohbetten doğrudan bir rutin oluşturur, listeler ya da iptal eder.
- **`/auto-perm`**: Kendine-sohbette araç çağrısı izin sorularını o konuşma için otomatik-izinli hale getiren bir slash komutu — sohbetten tetiklenen rutin/agent eylemleri, gelmeyecek bir masaüstü tıklamasını beklemek zorunda kalmasın diye.
- **Yerel Depolama**: Tüm WhatsApp mesajları izole bir SQLite veritabanında saklanır.

### Telegram Entegrasyonu

- **Bot Eşleştirme**: Settings → Telegram'da bir Telegram bot token'ı (@BotFather'dan) bağla; Memo yapılandırıldıktan sonra Bot API'yi long-polling ile dinlemeye başlar.
- **Sahip Kilidi**: Bir bot'un kullanıcı adını bulan herkes ona mesaj atabildiği için, Memo ilk mesaj atan kişiyi bot'un kalıcı sahibi olarak kilitler ve sonrasında herkesi sessizce yok sayar — bu entegrasyonun tüm erişim kontrolü bu kilide dayanır.
- **Asistan Sohbeti**: Bağlandıktan sonra sahip, WhatsApp kendine-sohbet ile aynı yeteneklere sahip tam bir asistan alır — sohbet, hafıza, agent araçları — bu sefer Telegram üzerinden.
- **Sohbetten Rutin Oluşturma**: Aynı `create_routine`/`list_routines`/`cancel_routine` araç akışı Telegram konuşmasından da çalışır.
- **Yerel Depolama**: Telegram mesajları, WhatsApp'tan bağımsız kendi izole SQLite veritabanında saklanır.

---

## 5. 🔌 Harici Sağlayıcı Desteği (External Providers)

### Çoklu Sağlayıcı Mimarisi
Memo, yerel modellerin yanında harici LLM API'lerine de bağlanır:
- **Desteklenen Sağlayıcılar:** OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Groq, Ollama, artı **OpenCode Zen** (kullandıkça öde, bazı modeller ücretsiz), **OpenCode Go** (abonelik) ve **Kilo Code** (app.kilo.ai — kullandıkça öde, bazı modeller ücretsiz) — üçü de gerçek, canlı model listesinden seçim yaptırır; ücretsiz modeller en üste sıralanır ve yeşil onay işaretiyle gösterilir.
- **Sohbet Sağlayıcısı Olarak Claude Code / Codex CLI (beta):** API çağrısı yerine Memo, kurulu `claude`/`codex` CLI'ını arka planda çalıştırır. Sohbet-bazlı (uygulama geneli değil), sabit zaman aşımı olmadan gerçek bir arka plan görevi olarak çalışır, CLI'ın kendi `/` komutları Memo'nun komut penceresinde görünür. Hafıza/kimlik bağlamı gönderilmez — CLI kendi oturumunu kendi yönetir.
- **Sağlayıcı Arayüzü:** Ortak `Provider` interface ile `ChatCompletion`, `ChatCompletionStream`, `ListModels`
- **Fallback Zinciri:** Router sağlayıcıları sırayla dener; 3 başarısızlıkta auto-disable; iyileşince health check ile tekrar aktifleştirme

### Şifreli Anahtar Yönetimi
- **AES-256-GCM Şifreleme:** API anahtarları makineye özel anahtar (`/etc/machine-id`) ile şifrelenir
- **Anahtar Depolama:** `data/providers.json` şifreli anahtar değerleriyle
- **Test Bağlantısı:** Kaydetmeden önce bağlantıyı test eden buton

### Frontend Sağlayıcı Arayüzü
- **API Providers Sekmesi:** Sağlayıcı ekleme/düzenleme için ayarlar sekmesi
- **Yapılandırma Dialog'u:** Sağlayıcı türü seçimi, API anahtarı girişi (maskeli), base URL, model dropdown
- **Aktif Sağlayıcı Seçimi:** Hangi sağlayıcının kullanılacağını seçme

---

## 6. 🧠 Ajan Modu (Agent Mode)

### Araç Çalıştırma Motoru
Memo, bilgisayarınızda işlem yapabilen bir AI ajanı olarak çalışır:
- **22 Yerleşik Araç:** dosya G/Ç (`read_file`, `write_file`, `edit_file`, `insert_line`, `delete_lines`, `delete_file`, `list_directory`, `get_file_info`, `search_files`), `run_command`, `read_env`, `web_search`, `self_clone`, `configure_provider`, `get_calendar_events`, rutinler (`create_routine`, `list_routines`, `cancel_routine`), WhatsApp (`whatsapp_send`/`search`/`latest`/`messages`)
- **Skill tool'ları artık gerçekten çalışıyor.** Bir skill'in `SKILL.md`'sinde tanımlanan `command:` alanı, yerleşik araçlarla aynı tool pipeline'ına ve izin-sorma arayüzüne bağlanıyor — önceden sadece deklaratifti, hiçbir şey çalıştırmıyordu.
- **Araç Kaydı:** JSON Schema parametre tanımlarıyla thread-safe kayıt sistemi
- **Tehlike Seviyesi:** `safe` (otomatik izin), `medium` (kullanıcıya sor), `dangerous` (kullanıcıya sor + 2sn gecikme)

### İzin Sistemi
- **6 Politika Türü:** PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever
- **Oturum Kalıcılığı:** İzinler `data/permissions.json` dosyasında saklanır
- **Argüman Hash'leme:** İzin eşleştirme için SHA-256 hash kullanılır

### Güvenlik Sandbox'ı
- **Path Traversal Koruması:** Symlink çözümleme, `..` engelleme, proje kök dizini sınırlaması
- **Komut Kara Listesi:** 23 tehlikeli pattern engellenir (`rm -rf /`, `sudo`, fork bomb, vb.)
- **Rate Limiting:** Dakikada 30 araç çağrısı, komut başına 5sn bekleme

### Ajan Pipeline'ı
- **LLM ↔ Araç Döngüsü:** Kullanıcı mesajı + araç tanımları LLM'e gönderilir, araç çağrıları çalıştırılır, sonuçlar LLM'e geri beslenir, nihai yanıta kadar döngü devam eder (max 20 iterasyon)
- **Olay Akışı:** Araç çalıştırma olayları SSE ile frontend'e iletilir
- **Denetim Günlüğü:** Son 1000 araç çalıştırması zaman damgasıyla kaydedilir

> **Not:** Ajan frontend UI'ı (izin dialog'ları, araç kartları, mod toggle) bir süredir tamamen canlı — toggle doğrudan sohbetin üst çubuğunda, web arama toggle'ının yanında; ayrı bir Agent ekranı gerekmiyor.

---

## 7. 🎵 Orkestra Modu (Multi-Model Orchestration)

### Konsept
Birden çok AI modeli bir ekip olarak çalışır:
1. **Şef Model** kullanıcı isteğini analiz eder, alt görevlere böler
2. **Uzman Roller** görevleri paralel olarak çalıştırır (frontend, backend, bug_fixer, vb.)
3. **Şef Model** sonuçları tek bir tutarlı cevapta birleştirir

### Yerleşik Roller
| Rol | Varsayılan Model | Amaç |
|------|-------------|---------|
| Planner | Claude | Yazılım mimarisi, görev dağılımı |
| Frontend | Grok | UI geliştirme |
| Backend | GPT-4o | API/sunucu mantığı |
| Bug Fixer | Gemini | Hata ayıklama, kök neden analizi |
| Reviewer | Claude | Kod kalite incelemesi |
| Security | GPT-4o | Güvenlik denetimi |
| DevOps | Grok | Altyapı/deploy |
| General | GPT-4o | Genel amaçlı yedek |

### Çalıştırma Modeli
- **Paralel Görevler:** Bağımsız görevler eşzamanlı çalışır (goroutine + WaitGroup)
- **Sıralı Görevler:** `depends_on` alanıyla bağımlılık çözümleme
- **Yeniden Deneme:** Rate-limit farkındalıklı üstel geri sarma (3 denemeye kadar)
- **Akış:** Her aşama için ilerleme güncellemeleri (plan → çalıştır → sentezle)

### Frontend Kontrolleri
- **Ayarlar Sekmesi:** Aç/kapa, şef model seçimi, rollere model atama
- **Yapılandırma Dialog'u:** Rol düzenleyici, model seçimi, system prompt düzenleme, özel rol desteği
- **Slash Komutu:** `/orchestra on`, `/orchestra off`, `/orchestra config`, `/orchestra status`

---

## 8. 👁️ Çoklu Modalite ve Duyular

### Görsel Destek (Multimodal)

- **Görsel Entegrasyonu**: Analiz için görselleri sürükleyip bırakın veya yükleyin (Llava veya Moondream gibi destekli modeller gerekir).
- **Yerel İşleme**: Güvenli ve yerel Base64 görüntü kodlama.

### Dosya Bağlamı

- **Döküman İndeksleme**: Yapay zekaya kod dosyaları (.go, .js, .py) veya dökümanlar (.md, .txt) ekleyerek belirli bir görev için anlık devasa bir bağlam kazandırın.

### Yerel STT (Sesden Metne)

- **Çevrimdışı Transkripsiyon**: Sesli mesajları doğrudan uygulama içinde kaydedin.
- **Entegre Motor**: Sıfır gecikmeli ve gizli transkripsiyon için paketlenmiş yerel ortamı (Vosk/Whisper muadili) kullanır.

---

## 9. ⏰ Rutinler ve Proaktif Zeka

### Rutinler (Zamanlanmış Otomasyonlar)

- Ne istediğini ve ne sıklıkla istediğini düz dille anlat; Memo bunu arka planda zamanında tetiklenen, basit bir prompt ya da tam bir agent görevi olabilen bir rutine çevirir.
- **Sadece Rutinler sekmesinden değil, sohbetten de oluştur**: normal bir sohbette ya da WhatsApp/Telegram kendine-sohbet asistanında düz dille bir rutin iste — `create_routine`/`list_routines`/`cancel_routine` agent araçları hallediyor, ayrı Rutinler ekranını açmana gerek yok.
- Bir rutin tetiklendiğinde nasıl oluşturulduğundan bağımsız olarak her zaman tam agent + web-arama araç erişimine sahiptir — eski bir hata bu erişimi oluşturma anındaki tek seferlik bir sınıflandırmaya bağlıyordu, bu yüzden zamanla sessizce "kapanabiliyordu"; artık koşulsuz.
- **Masaüstü ve mobilde** çalışır — mobil, uygulama açık olmasa bile gelen gerçek, önceden zamanlanmış yerel bildirimler kullanır.
- **Kendi cihazının saat diliminde** tetiklenir (oluşturulduğunda yakalanır, her yeniden bağlanmada güncellenir) — seyahat/DST değişikliği kendini düzeltir.

### Proaktif Öğrenme ve Ortam Uyarıları

- Memo kullanım desenlerini (belirttiğin bir alışkanlık veya belirli bir saatte yaptığın bir şey) fark edip kendiliğinden gündeme getirebilir — varsayılan olarak açık (subtle seviye).
- Doğrudan belirtilen bir alışkanlık ("her gece 21 gibi kod yazarım") hemen güvenilir; pasif gözlemlenen bir desenin önce istatistiksel olarak birikmesi gerekir.
- Bir hatırlatma normal bir yanıtın içine dokunarak veya masaüstünde bir öneri banner'ı (Evet / Şimdi Değil / Sormayı Bırak) olarak gelebilir.
- Gizli Mod'da tamamen kapalı; Minimal Mode'da özellikle yeniden açılmadıkça kapalı.

### Öz-İçgörü (`/insight`)

- Doğrudan sor, ya da haftalık bir Rutin sorsun — Memo son ruh hali/hafıza geçmişine bakıp gerçek bir desen varsa anlatır; yeterli sinyal yoksa uydurmak yerine bunu söyler.

### Minimal Mode (Settings → General)

- Mümkün olduğunca az ek yük ile yerel model çalıştırmak isteyenler için kişilik/mood/web-arama talimatlarını promptan tamamen çıkarır; hafıza da kapalıysa yazdığın mesajın dışında hiçbir şey eklenmez.
- Persona/sistem-promptu, yetenek açıklamaları, pasif-özellik açıklamaları ve proaktif öğrenme, Minimal Mode açıkken bile tek tek yeniden açılabilir.

### Memo'nun Kendi Kimliği

- Memo'yu kimin, neden yaptığı sorulduğunda artık tahmin yerine gerçek bir cevap veriyor — sadece sorulunca devreye giriyor, günlük davranışı değiştirmiyor, seçilen personadan bağımsız.

---

## 10. 🛠️ Geliştirici ve İleri Kullanıcı Özellikleri

### Developer API Gateway (Sidebar → Developer)

- Sadece Anthropic-uyumlu bir endpoint destekleyen araçların (en önemlisi **Claude Code**, `ANTHROPIC_BASE_URL` ile) Memo'nun yerel modelini veya yapılandırılmış herhangi bir sağlayıcı/anahtarını kullanmasını sağlayan yerel bir API.
- Model seçimi `type/model-id` formatında (`local/qwen2.5`, `openai/gpt-4o`, ...). openai/custom/local/groq/openrouter/grok/opencode-zen/opencode-go sağlayıcıları için tam agentic tool calling.
- İsteğe bağlı API key zorunluluğu (Remote Access token'ını paylaşır), isteğe bağlı hafıza entegrasyonu, canlı istek logu.

### Memo Swarm (Beta)

- Birden fazla PC'nin işlem gücünü (Settings → Beta Features → Swarm) tek bir makinenin RAM/VRAM'ine sığmayan büyük bir GGUF modeli çalıştırmak için havuzlar — bir Host model dosyasını tutar, diğerleri bir oda koduyla Join edip llama.cpp'nin `rpc-server`'ı üzerinden işlem gücü ödünç verir.
- Hedef hız değil kapasitedir. macOS'ta henüz yok.

### Kullanım İstatistikleri (Settings → Stats)

- KPI kartları (toplam istek, girdi/çıktı token, ortalama tok/s, en çok kullanılan model), son 30 günün günlük kullanım grafiği ve model-bazlı döküm — Gizli Mod hariç her tamamlanan tur (yerel, agent, orchestra, harici sağlayıcı) için kaydedilir.

### Hafızayı Başka Bir AI'dan İçe Aktar (Settings)

- Başka bir AI asistanından (ChatGPT, Gemini, Claude, ...) aldığın yapılandırılmış bir açıklamayı yapıştır; Memo bunu `/remember` ile aynı şekilde atomik gerçeklere böler, artı bir iletişim-tarzı özetini kendi sistem promptuna kalıcı olarak ekler.

### Hata Bildir (Settings)

- Tarayıcında önceden doldurulmuş bir GitHub issue açar (isteğe bağlı son 10 arka plan hata olayı eki ile) — sen GitHub'da gözden geçirip kendin göndermeden hiçbir şey hiçbir yere gitmez.

### Yeniden Düzenlenen Settings

- Settings yaklaşık 20 düz sekmeden, üstte arama kutusu olan, gruplanmış ve aranabilir bir rafa taşındı.

---

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm

- **Odak Odaklı UI**: Bilişsel yükü azaltmak için minimalist renk paleti.
- **Duyarlı Tasarım**: Hem geniş masaüstü hem de dar mobil görünümleri için optimize edilmiştir.
- **Kurulum Sihirbazı**: İsim, kişilik ve ilk teşhisler için rehberli kurulum süreci.

---
**Buğra tarafından geliştirildi.**
*Yapay Zekanı Kontrol Et. Hafızana Sahip Çık.*
