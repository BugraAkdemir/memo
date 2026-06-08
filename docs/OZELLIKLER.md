# Memo — Kapsamlı Özellik Kataloğu

Bu döküman, **Memo Yapay Zeka Hafıza Kabuğu** içinde entegre edilmiş olan her bir özelliğin detaylı dökümünü sunar. Arka plandaki ikili veri sürekliliğinden, multimodal (görsel/ses) yeteneklere kadar Memo'nun yerel yapay zeka deneyiminizi nasıl güçlendirdiğini keşfedin.

---

## 1. 🧠 Temel Zeka ve Hafıza

### Kalıcı RAG (Geri Getirme Destekli Nesil)

Memo sadece bir sohbet aracı değil, bir "İkinci Beyin"dir.

- **Anlamsal İndeksleme**: Her etkileşim otomatik olarak vektörleştirilir (embedding) ve yerel bir vektör veritabanında saklanır.
- **Bağlamsal Hatırlama**: Her yanıttan önce Memo, en ilgili geçmiş konuşmaları geri getirmek için anlamsal bir benzerlik araması (Top-K eşleşmesi) yapar.
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

### Uzaktan Erişim Sunucusu

- **Yerel Ağ Köprüsü**: Ayarlardan "Uzaktan Erişim"i etkinleştirerek, aynı Wi-Fi üzerindeki diğer cihazlardan (telefon, tablet vb.) yerel Memo'nuzla sohbet edebilirsiniz.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Araması

- **Doğrudan Depo Erişimi**: Hugging Face üzerindeki modelleri uygulama içinden direkt arayın.
- **Repo ID Desteği**: Herhangi bir Hugging Face GGUF depo kimliğini yapıştırarak mevcut dosyaları anında listeleyin.

### Sistem Teşhisi

- **VRAM ve GPU Kontrolü**: Kullanılabilir NVIDIA/AMD VRAM miktarını otomatik algılar.
- **Uyumluluk Rozeti**: Modelleri indirmeden önce "GPU Uyumlu" veya "⚠️ VRAM Yetersiz" olarak işaretler.

### Arka Plan İndirme Yöneticisi

- **Paralel İndirme**: Gerçek zamanlı yüzde ve hız takibi ile yüksek hızlı GGUF indirme motoru.
- **Yaşam Döngüsü Kontrolü**: Yerel modeller için tek tıkla Başlat, Durdur ve Güncelle seçenekleri.

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

---

## 5. 🔌 Harici Sağlayıcı Desteği (External Providers)

### Çoklu Sağlayıcı Mimarisi
Memo, yerel modellerin yanında harici LLM API'lerine de bağlanır:
- **Desteklenen Sağlayıcılar:** OpenAI (GPT-4o, o1, o3), Google Gemini (2.0 Flash, 2.5 Pro), xAI Grok (2, 3), Anthropic Claude (3.5 Sonnet, 3 Opus), OpenRouter (tek API ile tüm modeller), Groq (hızlı çıkarım), Ollama (yerel alternatif)
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
- **8 Yerleşik Araç:** `read_file`, `write_file`, `delete_file`, `list_directory`, `run_command`, `search_files`, `get_file_info`, `read_env`
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

> **Not:** Ajan frontend UI'ı (izin dialog'ları, araç kartları, mod toggle) henüz uygulanmamıştır. Ajan sadece backend API üzerinden çalışır.

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

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm

- **Odak Odaklı UI**: Bilişsel yükü azaltmak için minimalist renk paleti.
- **Duyarlı Tasarım**: Hem geniş masaüstü hem de dar mobil görünümleri için optimize edilmiştir.
- **Kurulum Sihirbazı**: İsim, kişilik ve ilk teşhisler için rehberli kurulum süreci.

---
**Buğra tarafından geliştirildi.**
*Yapay Zekanı Kontrol Et. Hafızana Sahip Çık.*
