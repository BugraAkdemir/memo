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
| OpenCode Zen (v3.3.3) | ✅ | API anahtarı — pay-as-you-go, bazı modeller ücretsiz; ücretsiz-sıralı model tarayıcı (v3.9.0) |
| OpenCode Go (v3.3.3) | ✅ | API anahtarı — abonelik tabanlı |
| Kilo Code (v3.9.0) | ✅ | API anahtarı — app.kilo.ai, pay-as-you-go, bazı modeller ücretsiz, ücretsiz modeller en üstte canlı model tarayıcı |
| Claude Code CLI (Beta, v3.3.4) | ✅ | Yok — kurulu `claude` CLI'ını subprocess olarak çalıştırır |
| Codex CLI (Beta, v3.3.4) | ✅ | Yok — kurulu `codex` CLI'ını subprocess olarak çalıştırır |

Router özellikleri: fallback zinciri, 3 hatada otomatik devre dışı bırakma, sağlık kontrolü goroutine'i. OpenCode Zen/Go, OpenRouter gibi model adını elle yazmak yerine sağlayıcının canlı model listesinden seçtiriyor. Claude Code/Codex CLI mimari olarak bambaşka — bkz. [[Harici Sağlayıcılar]].

## 🔁 Routines — Zamanlanmış Otomasyonlar (v3.3.3)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Doğal dilde tanım | ✅ | "Her sabah 8'de günü özetle" gibi bir cümle yeterli |
| Masaüstü + Mobil | ✅ | Mobilde gerçek, önceden zamanlanmış yerel bildirimler |
| Cihaz saat dilimi | ✅ | Oluşturulduğu cihazın saat diliminde tetiklenir, her (yeniden) bağlantıda resenkronize olur |
| Basit prompt / tam agent | ✅ | İsteğe bağlı olarak araç kullanan tam bir agent çalışması olarak da tetiklenebilir |
| Dil desteği | ✅ | Routine metinleri (sistem promptu, bildirim başlıkları) artık uygulama dilini takip ediyor |

## 🌱 Proaktif Öğrenme, Ambient Nudge'lar ve Self-Insight (v3.3.3)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Ambient nudge | ✅ | Varsayılan açık (ince seviye); bir örüntüyü kendiliğinden gündeme getirebilir |
| Öneri banner'ı | ✅ | Masaüstünde gerçek bir UI: Evet / Şimdi değil / Sorma |
| Doğrudan beyan edilen alışkanlık | ✅ | "Her gece 21:00 kodluyorum" gibi bir cümle istatistiksel örüntü beklemeden hemen güvenilir |
| `/insight` | ✅ | Ruh hali/hafıza geçmişinden gerçek bir örüntü varsa açıklar, yoksa uydurmaz |
| Minimal Mod / Gizli Mod etkileşimi | ✅ | Gizli Mod'da tamamen kapalı; Minimal Mod'da parça parça yeniden açılabilir |

## 🧘 Minimal Mod (v3.3.3)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Ayarlar → Genel | ✅ | Kişilik/ruh hali/web arama talimatlarını tamamen atlar — sadece hafıza (açıksa) modele gider |
| Parça parça yeniden açma | ✅ | Persona/sistem promptu, yetenek duyuruları, pasif-özellik duyuruları, proaktif öğrenme ayrı ayrı yeniden açılabilir |

## 🧑‍💻 Geliştirici Araçları (v3.3.3)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Kullanım İstatistikleri | ✅ | Ayarlar → İstatistikler: token/hız/model dağılımı, 30 günlük grafik (fl_chart) |
| Geliştirici API Ağ Geçidi | ✅ | Yan menüde ayrı bir ekran (Ayarlar içinde değil): Claude Code'u (`ANTHROPIC_BASE_URL`) ya da OpenAI-uyumlu bir aracı Memo'daki yerel/harici modele bağla, canlı istek/yanıt günlüğü dahil — bkz. [[Geliştirici API Ağ Geçidi]] |

## 🎙️ Sesli Mod / Live Mode (Beta, v3.3.4, geliştirme aşamasında)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Eller serbest sesli sohbet | ✅ Beta | Sohbet kutusunun yanında küçük bir ikon (yan menü sekmesi DEĞİL) |
| Yerel Piper TTS | ✅ | Varsayılan — çevrimdışı, hiçbir şey makineden çıkmak zorunda değil |
| Harici TTS (OpenAI) | ✅ Opsiyonel | Ayarlar → Beta Özellikler'den yapılandırılır, Piper her zaman yedek |
| Tek yönlü barge-in | ✅ | Memo konuşurken tekrar konuşman onu keser |
| Gömülü VAD modeli | ✅ | Ses-aktivite algılama artık uygulamayla birlikte geliyor, çalışma zamanı CDN indirmesi yok |
| Bilinen sınırlama | ⚠️ | Henüz echo cancellation yok — hoparlörle kullanımda Memo kendi sesini duyabilir |

## 🖥️ Sohbet Sağlayıcısı Olarak Claude Code / Codex CLI (Beta, v3.3.4)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Gerçek arka plan agent görevi | ✅ Beta | Kurulu `claude`/`codex` CLI'ı subprocess olarak çalıştırır, dosya okur/yazar, komut çalıştırır |
| Sohbet-bazlı | ✅ | Her sohbet kendi CLI sağlayıcısını/oturumunu/çalışma dizinini taşır |
| Sabit zaman aşımı yok | ✅ | Diğer yanıtların 5 dakikalık bütçesine tabi değil |
| CLI'ın kendi `/` komutları | ✅ | `.claude/commands` / `.codex/prompts`, skill'ler ve yerleşik komutlar |
| İnceleme arayüzü | ❌ | Dosya düzenlemelerini/komutları son metnin ötesinde gözden geçirecek UI henüz yok |

Detay: [[Harici Sağlayıcılar]]

## 🐝 Memo Swarm (Beta)

| Özellik | Durum | Açıklama |
|---------|-------|----------|
| Host / Katıl odası | ✅ Beta | Yan menü → Swarm; oda kodu ile birden fazla PC | 
| rpc-server (Join) | ✅ Beta | Katılan makinede model dosyası gerekmez |
| llama-server `--rpc` (Host) | ✅ Beta | Model host’ta; yük katmanları makineler arasında (llama.cpp RPC) |
| Beta anahtarı | ✅ | Ayarlar → **Beta Özellikler** (eski: Uzaktan Erişim altındaydı) |
| macOS | ❌ | UI gizli; RPC binary paketlenmiyor |

Sade dil + kurulum: [[Memo Swarm]].

## 🧰 Ajan Modu (Tool Calling)

| Özellik                       | Durum      |
| ----------------------------- | ---------- |
| 22 yerleşik araç              | ✅          |
| `create_routine`/`list_routines`/`cancel_routine` | ✅ (v3.9.0) — normal sohbetten ya da WhatsApp/Telegram kendine-sohbet asistanından kullanılabilir |
| 3 seviyeli tehlike            | ✅          |
| 6 izin politikası             | ✅          |
| Yürütme sandbox'ı             | ✅          |
| Hız sınırlaması (30 çağrı/dk) | ✅          |
| Komut kara listesi (23 desen) | ✅          |
| Denetim izi (1000 kayıt)      | ✅          |
| Ajan frontend UI (izin dialog'u, sohbet üst çubuğunda toggle) | ✅ |

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
| Kendine-sohbet asistanı (kendine yaz, tam asistan al) | ✅ (v3.9.0) |
| Sohbetten rutin oluşturma (`create_routine`/`list_routines`/`cancel_routine`) | ✅ (v3.9.0) |
| `/auto-perm` slash komutu | ✅ (v3.9.0) |

## ✈️ Telegram Entegrasyonu (v3.9.0)

| Özellik | Durum |
|---------|-------|
| Bot token eşleştirme (`@BotFather`) | ✅ |
| Sahip kilidi (ilk mesajı atan kalıcı sahip olur) | ✅ |
| Kendine-sohbet asistanı | ✅ |
| Sohbetten rutin oluşturma | ✅ |
| Yerel SQLite depolama, WhatsApp'tan izole | ✅ |
| `telegram_send` agent aracı | ❌ — WhatsApp'ın send/search/latest/messages araçlarının eşdeğeri henüz yok |

Detay: [[Telegram Entegrasyonu]]

## 🔐 Uzaktan Erişim ve Yedekleme

| Özellik | Durum |
|---------|-------|
| ngrok tüneli | ✅ |
| Tailscale tüneli | ✅ (v3.3.4: artık Beta değil — tek tıkla giriş, Funnel varsayılan açık, otomatik yeniden bağlanma) |
| Token kimlik doğrulama (zorunlu) | ✅ (v3.3.3 güvenlik düzeltmesi — önceden token olmadan da erişilebiliyordu) |
| Çoklu hesap, admin/kullanıcı rolleri | ✅ (v3.5.5, Faz 5.1) — Settings → Accounts ya da `memo remote add-account` |
| Hesap bazlı ayrıntılı izinler (7 checkbox) | ✅ (v3.9.0, Faz 5.1.1) — backend'de zorlanır, sadece arayüzde gizlenmez |
| `.memo` dışa/içe aktarma (eksiksiz) | ✅ (v3.3.3: takvim/rutin/görev/izin/skill + `machine.key` artık dahil) |
| Tam silme (wipe) | ✅ (v3.3.4: Windows'ta dosya kilidi nedeniyle başarısız olma sorunu düzeltildi) |
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
| Mobil eşlikçi uygulama | ✅ (v3.3.3: tam TR/EN yerelleştirme + bağlantı ekranında dil seçici) |
| Karanlık mod | ✅ |
| Sohbette `@` dosya bahsetme | ✅ (v3.3.4) |
| Aktif model/sağlayıcı pill'i | ✅ (v3.3.4) — sohbet üst çubuğunda tıkla-değiştir |
| Ayarlar aranabilir raf | ✅ (v3.3.4) — 20 düz sekme yerine gruplanmış, aranabilir liste |
| Agent modu toggle'ı (sohbet üst çubuğu) | ✅ (v3.3.3) |

## 🎵 Ses ve Multimodal

| Özellik | Durum |
|---------|-------|
| Yerel STT (sesden metne) | ✅ |
| Görsel yükleme (multimodal GGUF) | ✅ |
| Belge indeksleme | ✅ |
