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
- **Token Kimlik Doğrulama**: İsteğe bağlı `X-Memo-Token` ile güvenli bağlantı.

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
- **Tam Dışa Aktarım**: `GET /api/export` — zip arşivi (oturumlar, config, provider, orkestra, hafıza, WhatsApp).
- **Tam İçe Aktarım**: `POST /api/import` — .memo zip'ten geri yükleme.
- **Tüm Verileri Sil**: `POST /api/wipe` — çift onaylı, config dosyası kalır.

### Mobil Uygulama
- **İnce İstemci**: Android/iOS Flutter uygulama, LAN veya ngrok üzerinden bağlanır.
- **Sıfır İşlem**: Tüm AI masaüstünde — mobil sadece güvenli görüntüleyici.
- **Özellikler**: Sohbet (SSE), ayarlar (provider/model), oturum yönetimi.
- **Planlanan (v3.3.0)**: Tam özellik eşitliği, biometrik auth, çevrimdışı kuyruk.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Araması
- **Doğrudan Depo Erişimi**: Hugging Face modellerini uygulama içinden arayın.
- **Repo ID Desteği**: GGUF depo kimliği ile anında dosya getirme.

### Sistem Tanılama
- **VRAM/GPU Kontrolü**: NVIDIA/AMD VRAM otomatik algılama.
- **Uyumluluk Rozeti**: "GPU Uyumlu" / "Yetersiz VRAM" uyarıları.

### Arka Plan İndirme Yöneticisi
- **Paralel İndirme**: Gerçek zamanlı yüzde ve hız takibi.
- **Yaşam Döngüsü**: Tek tıkla başlat/durdur/güncelle.

---

## 4. ⚡ Etkileşim ve Kullanıcı Deneyimi

### Canlı Mod (Akış)
- **Token-Bazlı İşleme**: Gerçek zamanlı yanıt akışı.
- **Düşünme Durumu**: İlk token öncesi "Memo düşünüyor..." göstergesi.
- **Performans HUD**: tok/s, toplam token, süre metrikleri.

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
- **Desteklenenler:** OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Groq, Ollama
- **Ortak Arayüz:** `ChatCompletion`, `ChatCompletionStream`, `ListModels`
- **Fallback Zinciri:** Sırayla dener, 3 hata sonra devre dışı bırakır, sağlık kontrolü ile tekrar açar

### Şifreli Anahtar Yönetimi
- **AES-256-GCM**: Makine anahtarı (`/etc/machine-id`) ile şifreleme
- **Test Bağlantısı**: Kaydetmeden önce doğrulama

---

## 6. 🧠 Ajan Modu (Tool Calling)

### Araç Çalıştırma Motoru
- **8 Yerleşik Araç:** `read_file`, `write_file`, `delete_file`, `list_directory`, `run_command`, `search_files`, `get_file_info`, `read_env`
- **Tehlike Seviyesi:** `güvenli` (otomatik izin), `orta` (kullanıcı onayı), `tehlikeli` (onay + gecikme)

### İzin Sistemi
- **6 Politika:** PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever
- **Oturum Kalıcılığı:** `data/permissions.json`
- **Arg Hash**: SHA-256 ile izin eşleştirme

### Güvenlik Sandbox'ı
- **Yol Koruması**: Symlink çözümleme, `..` engelleme, proje kökü sınırlama
- **Komut Kara Listesi**: 23 tehlikeli desen engellenir
- **Hız Sınırlaması**: 30 araç çağrısı/dk, 5sn bekleme

> **Not:** Ajan frontend UI (izin diyalogları, tool call kartları, mod toggle) v3.2.0'da planlanmıştır.

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

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm
- **Önce Odak UI**: Minimalist renk paleti, azaltılmış bilişsel yük.
- **Kurulum Sihirbazı**: Rehberli kurulum, 6 karakter ön tanımlı prompt.
- **Türkçe/İngilizce**: Çift dil desteği, runtime dil değiştirme.

---

**Buğra tarafından geliştirildi.**
*Yapay Zekanı Kontrol Et. Hafızana Sahip Çık.*
