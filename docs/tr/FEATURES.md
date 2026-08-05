# Memo — Kapsamlı Özellik Kataloğu

Bu döküman, **Memo Yapay Zeka Hafıza Kabuğu** içinde entegre edilmiş her bir özelliğin detaylı dökümünü sunar.

---

## 1. 🧠 Temel Zeka ve Hafıza

### Kalıcı RAG (Retrieval-Augmented Generation)
Memo sadece bir sohbet aracı değil; bir "İkinci Beyin"dir.
- **Anlamsal İndeksleme**: Her etkileşim otomatik olarak vektörleştirilir ve yerel bir vektör veritabanında saklanır.
- **Bağlamsal Hatırlama**: Her yanıttan önce Memo, en ilgili geçmiş konuşmaları geri getirmek için Top-K benzerlik araması yapar.
- **Sonsuz Bağlam**: Uzun süreli hafıza, modelin pencere sınırından bağımsız olarak haftalar/aylar önceki detayları hatırlamasını sağlar.

### Modelden Bağımsız Motor
- **Dahili Llama-Server**: Yüksek performanslı GGUF çıkarımı için `llama.cpp`.
- **Özel Embedding Sunucusu**: Hafıza indeksleme için ayrı sunucu, sohbet performansı etkilenmez.
- **Cross-Mode**: Harici API provider (OpenAI, Claude, Gemini) ile chat + yerel embedding aynı anda çalışır.
- **Harici Sağlayıcı Desteği**: OpenAI, Anthropic, Google, xAI, Groq, OpenRouter, Ollama.

### Uzaktan Erişim
- **ngrok Tüneli**: Dahili ngrok entegrasyonu, dünyanın her yerinden erişim.
- **Tailscale (artık Beta değil)**: Tek tıkla giriş, Funnel varsayılan açık, kopan bağlantıdan otomatik yeniden bağlanma.
- **Token Zorunlu**: Her uzak istek (LAN/ngrok/Tailscale) artık Settings'teki erişim token'ını gerektiriyor — önceden isteğe bağlıydı (v3.3.3 güvenlik düzeltmesi).

---

## 2. 🏛️ Mimari ve Kalıcılık

### SQLite + sqlite-vec Kalıcılığı
- **Birleşik Depolama**: Vektör gömmeleri ve metadata aynı SQLite veritabanında.
- **ANN İndeksleme**: `vec0` sanal tablosu ile O(log N) sorgu süresi.
- **ACID Uyumluluğu**: Atomik yazma ve veri bütünlüğü.
- **Go Fallback**: vec0 yoksa brute-force cosine similarity.

### Gizlilik ve Yerel İzolasyon
- **%100 Çevrimdışı**: Veriler cihazınızdan çıkmaz. Telemetri yok.
- **Şifreli Depolama**: API anahtarları AES-256-GCM ile şifrelenir.

### Yedekleme & Restore (.memo)
- **Tam Dışa Aktarım**: `POST /api/export` — zip arşivi (oturumlar, config, provider, orkestra, hafıza, WhatsApp, **artı takvim, alışkanlıklar, rutinler, görev listeleri, agent izinleri, skill'ler ve `machine.key`** — önceden eksikti; `machine.key` olmadan başka bir makinede geri yüklenen bir yedek tüm API anahtarlarını kalıcı olarak çözülemez bırakıyordu, düzeltildi).
- **Tam İçe Aktarım**: `POST /api/import` — .memo zip'ten geri yükleme.
- **Tüm Verileri Sil**: `POST /api/wipe` — çift onaylı, config dosyası kalır; Windows'ta başarısız olma sorunu düzeltildi (tüm iç veritabanları artık silmeden önce düzgünce kapatılıyor).

### Mobil Uygulama
- **İnce İstemci**: Android/iOS Flutter uygulama, LAN veya ngrok üzerinden bağlanır.
- **Sıfır İşlem**: Tüm AI masaüstünde — mobil sadece güvenli görüntüleyici.
- **Özellikler**: Sohbet (SSE), ayarlar (provider/model), oturum yönetimi, rutin bildirimleri, Tailscale uzaktan erişim.
- **Özellik eşitliği büyük ölçüde ulaşıldı**: backend'in 118 endpoint'inin 111'i artık mobilde var (kalan 7'si CLI-yönetimi/client-registry uçları, mobilde gerekmiyor). Biometrik auth ve çevrimdışı kuyruk hâlâ planlı.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Araması
- **Doğrudan Depo Erişimi**: Hugging Face modellerini uygulama içinden arayın.
- **Repo ID Desteği**: GGUF depo kimliği ile anında dosya getirme.

### Sistem Tanılama
- **VRAM/GPU Kontrolü**: NVIDIA/AMD VRAM otomatik algılama.
- **Uyumluluk Rozeti**: "GPU Uyumlu" / "Yetersiz VRAM" uyarıları.

### Arka Plan İndirme Yöneticisi
- **Paralel İndirme**: Artık birden fazla indirme aynı anda çalışabiliyor, gerçek zamanlı yüzde/hız takibi + kaba süre tahmini.
- **Yaşam Döngüsü**: Tek tıkla başlat/durdur/güncelle.
- **Donanıma Göre İlk Öneri**: Kurulum RAM/GPU'ya göre bir sohbet+hafıza model çifti önerir.
- **Güvenli Context Boyutu**: Slider artık GGUF dosyasından okunan gerçek maksimumu aşamıyor.

---

## 4. ⚡ Etkileşim ve Kullanıcı Deneyimi

### Akış (Streaming)
- **Token-Bazlı İşleme**: Gerçek zamanlı yanıt akışı.
- **Düşünme Durumu**: İlk token öncesi "Memo düşünüyor..." göstergesi.
- **Performans HUD**: tok/s, toplam token, süre metrikleri.

### Sesli Mod — Eller Serbest (Beta)
- Sohbet kutusunun yanındaki küçük ses ikonu (ayrı bir sekme değil) — dinler, otomatik konuşma başlangıç/bitiş algılar, yerelde yazıya döker, sesli yanıtlar.
- Varsayılan olarak tamamen yerel: cihaz-üstü transkripsiyon + yerel **Piper** TTS; isteğe bağlı harici OpenAI TTS.
- Tek yönlü barge-in; henüz yankı iptali yok (bilinen sınırlama).

### @ Dosya Bahsetme
- `@` yazarak dosya adına göre arayıp referans gösterme.

### Gizli Mod
- **Sıfır Kalıcılık**: Hafıza ve geçmiş kaydını devre dışı bırakır.
- **Uçucu Bağlam**: Oturum kapanınca her şey silinir.

### WhatsApp Entegrasyonu
- **QR Eşleştirme**: WhatsApp Web multi-device QR bağlantısı.
- **Çift Yönlü Mesajlaşma**: Kişi adı çözümlemeli gönder/al.
- **Beyaz Liste Dosya Aktarımı**: Güvenilir kişilerden dosya talebi.
- **Agent Araçları**: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`.
- **Yerel Depolama**: İzole SQLite veritabanı.

---

## 5. 🔌 Harici Sağlayıcı Desteği

### Çoklu Sağlayıcı Mimarisi
- **Desteklenenler:** OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Groq, Ollama, artı **OpenCode Zen** ve **OpenCode Go** (canlı model listesinden seçim)
- **Sohbet Sağlayıcısı Olarak Claude Code / Codex CLI (Beta):** kurulu CLI'ı arka planda çalıştırır, sohbet-bazlı, sabit zaman aşımı yok, hafıza/kimlik bağlamı gönderilmez
- **Ortak Arayüz:** `ChatCompletion`, `ChatCompletionStream`, `ListModels`
- **Fallback Zinciri:** Sırayla dener, 3 hata sonra devre dışı bırakır, sağlık kontrolü ile tekrar açar

### Şifreli Anahtar Yönetimi
- **AES-256-GCM**: Makine anahtarı (`/etc/machine-id`) ile şifreleme
- **Test Bağlantısı**: Kaydetmeden önce doğrulama

---

## 6. 🧠 Ajan Modu (Tool Calling)

### Araç Çalıştırma Motoru
- **19 Yerleşik Araç:** dosya G/Ç (`read_file`, `write_file`, `edit_file`, `insert_line`, `delete_lines`, `delete_file`, `list_directory`, `get_file_info`, `search_files`), `run_command`, `read_env`, `web_search`, `self_clone`, `configure_provider`, `get_calendar_events`, WhatsApp araçları
- **Skill tool'ları artık gerçekten çalışıyor** — bir skill'in `command:` alanı aynı pipeline/izin arayüzüne bağlanıyor
- **Tehlike Seviyesi:** `güvenli` (otomatik izin), `orta` (kullanıcı onayı), `tehlikeli` (onay + gecikme)

### İzin Sistemi
- **6 Politika:** PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever
- **Oturum Kalıcılığı:** `data/permissions.json`
- **Arg Hash**: SHA-256 ile izin eşleştirme

### Güvenlik Sandbox'ı
- **Yol Koruması**: Symlink çözümleme, `..` engelleme, proje kökü sınırlama
- **Komut Kara Listesi**: 23 tehlikeli desen engellenir
- **Hız Sınırlaması**: 30 araç çağrısı/dk, 5sn bekleme

> **Not:** Ajan frontend UI (izin diyalogları, tool call kartları, mod toggle) tamamen canlı — toggle sohbetin üst çubuğunda.

---

## 7. 🎵 Orkestra Modu (Multi-Model Orchestration)

### Konsept
1. **Şef Model** kullanıcı isteğini analiz eder, alt görevlere böler
2. **Uzman Roller** görevleri paralel çalıştırır (frontend, backend, bug_fixer vb.)
3. **Şef Model** sonuçları birleştirir

### Yerleşik Roller
| Rol | Varsayılan Model | Amaç |
|-----|-----------------|------|
| Planlayıcı | Claude | Mimari, görev dağılımı |
| Frontend | Grok | UI geliştirme |
| Backend | GPT-4o | API/sunucu |
| Hata Düzeltici | Gemini | Hata ayıklama |
| İncelemeci | Claude | Kod kalitesi |
| Güvenlik | GPT-4o | Güvenlik denetimi |
| DevOps | Grok | Altyapı/deploy |
| Genel | GPT-4o | Genel amaçlı |

### Yürütme
- **Paralel Görevler**: Bağımsız görevler eşzamanlı çalışır
- **Sıralı Görevler**: `depends_on` ile bağımlılık çözümü
- **Tekrar Deneme**: Rate-limit farkındalıklı üstel geri bildirim
- **Akış**: Planla → yürüt → sentezle aşamaları SSE ile iletilir

---

## 8. 👁️ Çoklu Modalite ve Duyular

### Görsel Desteği
- **Görsel Entegrasyonu**: Sürükle-bırak ile resim yükleme (multimodal GGUF gerekir).
- **Base64 İşleme**: Yerel, güvenli görsel kodlama.

### Dosya Bağlamsallaştırma
- **Belge İndeksleme**: Kod (.go, .js, .py) veya belge (.md, .txt) ekleyerek anlık bağlam sağlama.

### Yerel STT (Sesden Metne)
- **Çevrimdışı Transkripsiyon**: Ses kaydı, yerel transkripsiyon.

---

## 9. ⏰ Rutinler ve Proaktif Zeka

- **Rutinler**: Düz dille bir görev + zamanlama tanımla; masaüstü ve mobilde çalışır, cihazın kendi saat diliminde tetiklenir.
- **Proaktif Öğrenme**: Kullanım desenlerini fark edip kendiliğinden gündeme getirir (varsayılan açık, subtle seviye); masaüstünde öneri banner'ı (Evet/Şimdi Değil/Sormayı Bırak).
- **Öz-İçgörü (`/insight`)**: İstek üzerine veya haftalık bir Rutin ile son ruh hali/hafıza geçmişinden gerçek bir desen anlatır.
- **Minimal Mode**: Kişilik/mood/web-arama talimatlarını promptan çıkarır; alt-özellikler tek tek yeniden açılabilir.
- **Memo'nun Kendi Kimliği**: Kim yaptığı sorulduğunda artık gerçek bir cevap veriyor.

## 10. 🛠️ Geliştirici ve İleri Kullanıcı Özellikleri

- **Developer API Gateway**: Yerel Anthropic-uyumlu endpoint (Claude Code dahil araçlar için), `type/model-id` model seçimi.
- **Memo Swarm (Beta)**: Birden fazla PC'nin işlem gücünü tek büyük bir model için havuzlar (macOS'ta henüz yok).
- **Kullanım İstatistikleri**: İstek/token/hız KPI'ları, 30 günlük grafik, model bazlı döküm.
- **Hafızayı Başka AI'dan İçe Aktar**: Başka bir asistandan alınan bir açıklamayı atomik gerçeklere böler.
- **Hata Bildir**: Önceden doldurulmuş bir GitHub issue açar.
- **Yeniden Düzenlenen Settings**: ~20 düz sekme yerine aranabilir, gruplanmış bir raf.

---

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm
- **Önce Odak UI**: Minimalist renk paleti, azaltılmış bilişsel yük.
- **Kurulum Sihirbazı**: Rehberli kurulum, 6 karakter ön tanımlı prompt.
- **Türkçe/İngilizce**: Çift dil desteği, runtime dil değiştirme.

---

**Buğra tarafından geliştirildi.**
*Yapay Zekanı Kontrol Et. Hafızana Sahip Çık.*
