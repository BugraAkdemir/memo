# Özellik Kataloğu

Memo'nun özellik-özellik tam listesi. Tam detay: `docs/tr/FEATURES.md`.

---

## 🧠 Zeka ve Hafıza

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Kalıcı RAG | ✅ | Her etkileşimin otomatik vektörleştirilmesi |
| Bağlamsal Hatırlama | ✅ | Her yanıttan önce Top-K benzerlik araması |
| Sonsuz Bağlam | ✅ | Model pencere sınırından bağımsız uzun süreli hafıza |
| Cross-Mode | ✅ | Harici sağlayıcı sohbeti + yerel embedding eşzamanlı |
| Gizli Mod | ✅ | Geçici oturumlar, sıfır kalıcılık |

## 🏭 Model Yönetimi (Fabrika)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Yerel llama-server | ✅ | Yüksek performanslı GGUF çıkarımı |
| Özel Embedding Sunucusu | ✅ | Ayrı sunucu, sohbet performansı etkilenmez |
| HuggingFace Arama | ✅ | Uygulama içi model tarayıcısı |
| Arka Plan İndirme Yöneticisi | ✅ | Gerçek zamanlı ilerleme, başlat/durdur/güncelle |
| Sistem Tanılama | ✅ | NVIDIA/AMD VRAM algılama, uyumluluk rozetleri |

## 🔌 Harici Sağlayıcılar

| Sağlayıcı | Durum | Kimlik Doğrulama |
|-----------|-------|-----------------|
| OpenAI | ✅ | API anahtarı |
| Google Gemini | ✅ | API anahtarı |
| xAI Grok | ✅ | API anahtarı |
| Anthropic Claude | ✅ | API anahtarı |
| OpenRouter | ✅ | API anahtarı |
| Groq | ✅ | API anahtarı |
| Ollama | ✅ | URL |

Router özellikleri: fallback zinciri, 3 hatada otomatik devre dışı bırakma, sağlık kontrolü goroutine'i.

## 🧑‍💻 Geliştirici Araçları (v3.3.3)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Kullanım İstatistikleri | ✅ | Ayarlar → İstatistikler: token/hız/model dağılımı, 30 günlük grafik (fl_chart) |
| Geliştirici API Ağ Geçidi | ✅ | Yan menüde ayrı bir ekran (Ayarlar içinde değil): Claude Code'u (`ANTHROPIC_BASE_URL`) ya da OpenAI-uyumlu bir aracı Memo'daki yerel/harici modele bağla, canlı istek/yanıt günlüğü dahil — bkz. [[Geliştirici API Ağ Geçidi]] |

## 🧰 Ajan Modu (Tool Calling)

| Özellik                       | Durum      |
| ----------------------------- | ---------- |
| 8 yerleşik araç               | ✅          |
| 3 seviyeli tehlike            | ✅          |
| 6 izin politikası             | ✅          |
| Yürütme sandbox'ı             | ✅          |
| Hız sınırlaması (30 çağrı/dk) | ✅          |
| Komut kara listesi (23 desen) | ✅          |
| Denetim izi (1000 kayıt)      | ✅          |
| Ajan frontend UI              | ❌ (v3.2.0) |

## 🎵 Orkestra Modu (Multi-Model)

| Özellik | Durum |
|---------|-------|
| 8 uzman rolü | ✅ |
| Planla → Yürüt → Sentezle | ✅ |
| Paralel görev yürütme | ✅ |
| `depends_on` ile sıralı | ✅ |
| SSE ilerleme akışı | ✅ |
| Üstel geri bildirim ile yeniden deneme | ✅ |

## 💬 WhatsApp Entegrasyonu

| Özellik | Durum |
|---------|-------|
| QR eşleştirme | ✅ |
| Çift yönlü mesajlaşma | ✅ |
| Kişi adı çözümleme | ✅ |
| Beyaz liste dosya transferi | ✅ |
| 4 agent aracı | ✅ |
| Yerel SQLite depolama | ✅ |

## 🔐 Uzaktan Erişim ve Yedekleme

| Özellik | Durum |
|---------|-------|
| ngrok tüneli | ✅ |
| Token kimlik doğrulama | ✅ |
| `.memo` dışa/içe aktarma | ✅ |
| Tam silme (wipe) | ✅ |
| Google Drive E2E senkronizasyon | ✅ |
| AES-256-GCM şifreleme | ✅ |

## 🎨 UI ve Kullanıcı Deneyimi

| Özellik | Durum |
|---------|-------|
| Akışlı SSE yanıtları | ✅ |
| Markdown işleme | ✅ |
| Görsel ekleme (vision) | ✅ |
| Dosya bağlamı ekleme | ✅ |
| Mesaj düzenleme/silme/dışa aktarma | ✅ |
| Gizli mod açma/kapama | ✅ |
| Kurulum sihirbazı (6 kişilik) | ✅ |
| Çoklu dil (TR/EN) | ✅ (924 anahtar) |
| Greige teması, Material 3 | ✅ |
| Mobil eşlikçi uygulama | ✅ (temel) |
| Karanlık mod | ✅ |

## 🎵 Ses ve Multimodal

| Özellik | Durum |
|---------|-------|
| Yerel STT (sesden metne) | ✅ |
| Görsel yükleme (multimodal GGUF) | ✅ |
| Belge indeksleme | ✅ |
