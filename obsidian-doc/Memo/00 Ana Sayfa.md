# 🧠 Memo: Yerel Yapay Zeka Hafıza Kabuğu

> **Sürüm:** v3.1.0-beta | **Geliştirici:** Buğra | **Lisans:** MIT
> 
> *"Control your AI. Own your Memory."*

Memo'nun resmi teknik dökümantasyonuna ve bilgi bankasına hoş geldiniz. Bu kasa, projenin mimarisinden özelliklerine, geliştirme süreçlerinden kullanım kılavuzlarına kadar her ayrıntıyı barındıran kapsamlı bir **"Second Brain"** kaynağıdır.

---

## 📊 Proje İstatistikleri

| Metrik                     | Değer                                                             |
| -------------------------- | ----------------------------------------------------------------- |
| Go Backend                 | ~15.000 satır, 30+ paket                                          |
| Flutter Frontend           | ~8.000 satır, 20+ widget, 924 L10n anahtarı                       |
| Mobil Uygulama             | Flutter ince istemci (Android/iOS)                                |
| REST API                   | ~35 endpoint (sohbet, hafıza, modeller, sağlayıcılar, ajan, senk) |
| Veritabanı                 | SQLite + sqlite-vec (ANN vektör arama)                            |
| Harici Sağlayıcılar        | 7 (OpenAI, Gemini, Claude, Grok, Groq, OpenRouter, Ollama)        |
| Hata Düzeltmeleri (v3.1.0) | 61 belgelenmiş düzeltme (54 tespit → 46 düzeltildi, 8 açık)       |

---

## 🏛️ Mimari Yapı

Memo'nun nasıl çalıştığını, bileşenlerin birbiriyle nasıl konuştuğunu ve sistemin temel taşlarını keşfedin.

| Sayfa | Açıklama |
|-------|----------|
| [[Sistem Genel Bakış]] | Üst düzey mimari: Go backend ↔ Flutter frontend HTTP/JSON + SSE ile |
| [[Backend (Go) Mimarisi]] | App bridge deseni, modül haritası, veri akışı |
| [[Frontend (Flutter) Tasarımı]] | Riverpod state yönetimi, widget ağacı, 11 sekmeli ayarlar |
| [[Veri Katmanı ve Kalıcılık]] | SQLite + vec0, oturum JSON dosyaları, provider şifreleme |
| [[Teknik Derinlemesine]] | Bridge deseni, llama yaşam döngüsü, E2E senk, ajan motoru, orkestra iç yapısı |

```
┌─────────────────────────────────────────────────────────────┐
│  Flutter Masaüstü / Mobil Uygulama                          │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐  │
│  │ Sohbet  │ │ Ayarlar  │ │ Model    │ │ WhatsApp UI    │  │
│  │ (SSE)   │ │ (11 sek) │ │ Deposu   │ │ (QR + Sohbet)  │  │
│  └────▲────┘ └────▲─────┘ └────▲─────┘ └───────▲────────┘  │
│       │           │            │                │           │
├───────┼───────────┼────────────┼────────────────┼───────────┤
│       └───────────┼────────────┼────────────────┘           │
│                   │   HTTP/JSON + SSE (port 8090)           │
│  ┌────────────────┼────────────┼────────────────────────┐   │
│  │  Go Backend    │            │                        │   │
│  │  ┌─────────────▼────────────▼──────────────────┐    │   │
│  │  │           App (AppBridge)                    │    │   │
│  │  │  ┌────────┐ ┌──────────┐ ┌───────────────┐  │    │   │
│  │  │  │ LLM    │ │ Hafıza   │ │ Oturumlar     │  │    │   │
│  │  │  │ Router │ │ (SQLite  │ │ (JSON         │  │    │   │
│  │  │  │        │ │ + vec0)  │ │  dosyaları)   │  │    │   │
│  │  │  └───┬────┘ └──────────┘ └───────────────┘  │    │   │
│  │  │      │                                        │    │   │
│  │  │  ┌───┴────────┐ ┌──────────┐ ┌────────────┐  │    │   │
│  │  │  │ llama.cpp  │ │ Provider │ │ Ajan       │  │    │   │
│  │  │  │ (yerel)    │ │ Router   │ │ Motoru     │  │    │   │
│  │  │  │            │ │ (7 API)  │ │ (8 araç)   │  │    │   │
│  │  │  └────────────┘ └──────────┘ └────────────┘  │    │   │
│  │  │  ┌────────────┐ ┌──────────┐ ┌────────────┐  │    │   │
│  │  │  │ Orkestra   │ │ WhatsApp │ │ Bulut Sync │  │    │   │
│  │  │  │ (8 rol)    │ │(whatsmeow)│ │ (Drive)   │  │    │   │
│  │  │  └────────────┘ └──────────┘ └────────────┘  │    │   │
│  │  └───────────────────────────────────────────────┘    │   │
│  └────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🧠 Temel Özellikler

Memo'yu sadece bir sohbet arayüzü değil, gerçek bir **hafıza kabuğu** ve AI ajanı yapan özellikler.

### 💾 Hafıza ve Zeka

| Sayfa | Durum | Açıklama |
|-------|-------|----------|
| [[RAG ve Semantik Hafıza]] | ✅ Tam | Top-K benzerlik araması ile kalıcı vektör tabanlı RAG |
| [[Hafıza Deposu (SQLite + vec0)]] | ✅ Tam | SQLite + sqlite-vec ANN indeksi, ACID uyumlu, tek dosya |
| [[Gizli Mod (Incognito)]] | ✅ Tam | Geçici oturumlar — sıfır kalıcılık, uçucu bağlam |
| [[Multimodal Yetenekler (Görsel ve Ses)]] | ✅ Tam | Görsel yükleme, yerel STT, belge indeksleme |

### 🏭 Model Yönetimi

| Sayfa | Durum | Açıklama |
|-------|-------|----------|
| [[Model Yönetimi (Fabrika)]] | ✅ Tam | HuggingFace arama/indirme, arka plan indirme yöneticisi, sistem tanılama |
| [[Llama.cpp Entegrasyonu]] | ✅ Tam | Alt süreç yaşam döngüsü, sağlık kontrolleri, GPU offloading (NVIDIA/AMD) |

### 🌐 Dış Bağlantı

| Sayfa | Durum | Açıklama |
|-------|-------|----------|
| [[Harici Sağlayıcılar]] | ✅ Tam | 7 sağlayıcı, Router ile fallback zinciri, otomatik devre dışı, sağlık kontrolü |
| [[Bulut Senkronizasyonu]] | ✅ Tam | Google Drive E2E şifreli yedekleme (AES-256-GCM + PBKDF2) |
| [[WhatsApp Entegrasyonu]] | ✅ Tam | QR eşleştirme, çift yönlü mesajlaşma, 4 agent aracı |
| [[Yedekleme & Restore]] | ✅ Tam | `.memo` zip dışa/içe aktarma, tam silme, zip bomb koruması |
| [[Mobil Uygulama]] | ✅ Temel | İnce Flutter istemci (LAN/ngrok, token auth, akışlı sohbet) |

### 🧰 Gelişmiş AI

| Sayfa | Durum | Açıklama |
|-------|-------|----------|
| [[Ajan Modu]] | ✅ Backend | 8 araç, 6 izin politikası, yürütme sandbox'ı, hız sınırlaması |
| [[Orkestra Modu]] | ✅ Backend | 8 uzman rolü, Planla→Yürüt→Sentezle, SSE ilerleme |
| [[Özellik Kataloğu]] | ✅ Tam | Özellik-özellik tam liste |

---

## 🆕 v3.1.0 Özellikleri

Bu sürümde eklenen büyük yeni yetenekler.

| Özellik | Sayfa | Etki |
|---------|-------|------|
| 📱 WhatsApp | [[WhatsApp Entegrasyonu]] | Çift yönlü mesajlaşma, dosya transferi, agent araçları |
| 🔌 Harici Sağlayıcılar | [[Harici Sağlayıcılar]] | 7 LLM API'si akıllı yönlendirme ve fallback ile |
| 🤖 Ajan Modu | [[Ajan Modu]] | İzin sistemi ve sandbox ile AI araç çağırma |
| 🎵 Orkestra Modu | [[Orkestra Modu]] | Uzman rollerle çoklu model işbirliği |
| 📦 Yedekleme & Restore | [[Yedekleme & Restore]] | `.memo` formatı, Google Drive senk, tam silme |
| 📱 Mobil Uygulama | [[Mobil Uygulama]] | Android/iOS eşlikçi istemci |
| 🐛 Hata Düzeltmeleri | [[Çözülen Sorunlar]] | Tüm kod tabanında 61 belgelenmiş düzeltme |

---

## 🔧 Teknik Referans

Geliştiriciler ve ileri düzey kullanıcılar için derinlemesine teknik bilgiler.

| Sayfa | İçerik |
|-------|--------|
| [[API Dökümantasyonu]] | Tam REST API endpoint referansı (~35 endpoint) |
| [[Llama.cpp Entegrasyonu]] | Süreç yaşam döngüsü, sağlık kontrolleri, port yönetimi, GPU algılama |
| [[Vektör Arama Mantığı]] | Kosinüs benzerliği, paralel işçiler, Top-K, min_similarity |
| [[Gelişmiş Ayarlar]] | Model parametreleri, motor modları, yapılandırma referansı |
| [[CGO Bayrakları]] | CGO derleme gereksinimleri, sqlite-vec eklenti derlemesi |
| [[Varsayılan Sistem Promptu]] | Memo'nun kimlik yönergeleri ve anti-halüsinasyon kuralları |
| [[Bilinen Sorunlar]] | Kapsamlı hata denetimi — 37 hata, 37'si düzeltildi ✅ |
| [[Çözülen Sorunlar]] | 61 belgelenmiş düzeltme ve kod referansları |

---

## 🚀 Kılavuzlar

Memo'yu kurun, geliştirin ve kullanın.

| Kılavuz | Hedef Kitle | Sayfalar |
|---------|-------------|----------|
| [[Geliştirici Kurulum Rehberi]] | Geliştiriciler | Ortam kurulumu, hızlı başlangıç |
| [[Derleme ve Paketleme]] | Geliştiriciler | Çapraz platform derleme, release paketleme |
| [[Kullanım Kılavuzu]] | Son Kullanıcılar | Günlük kullanım |
| [[Sorun Giderme]] | Herkes | Yaygın sorunlar ve çözümleri |
| [[Katkıda Bulunma]] | Katkıda Bulunanlar | Nasıl katkı yapılır, kod standartları, kilit alanlar |

---

## 🗺️ Hızlı Navigasyon

| Bölüm | Sayfalar |
|-------|----------|
| 🏛️ Mimari | [[Sistem Genel Bakış]], [[Backend (Go) Mimarisi]], [[Frontend (Flutter) Tasarımı]], [[Veri Katmanı ve Kalıcılık]], [[Teknik Derinlemesine]] |
| 🧠 Özellikler | [[RAG ve Semantik Hafıza]], [[Hafıza Deposu (SQLite + vec0)]], [[Model Yönetimi (Fabrika)]], [[Gizli Mod (Incognito)]], [[Bulut Senkronizasyonu]], [[Multimodal Yetenekler (Görsel ve Ses)]], [[WhatsApp Entegrasyonu]], [[Yedekleme & Restore]], [[Mobil Uygulama]] |
| 🧰 Gelişmiş | [[Ajan Modu]], [[Orkestra Modu]], [[Harici Sağlayıcılar]], [[Özellik Kataloğu]] |
| 🔧 Referans | [[API Dökümantasyonu]], [[Llama.cpp Entegrasyonu]], [[Vektör Arama Mantığı]], [[Gelişmiş Ayarlar]], [[CGO Bayrakları]], [[Varsayılan Sistem Promptu]] |
| 📋 Operasyon | [[Bilinen Sorunlar]], [[Çözülen Sorunlar]], [[Sorun Giderme]], [[Katkıda Bulunma]], [[Yol Haritası]] |
| 🆕 v3.1.0 | [[v3.1.0 Özellikleri]] |

---

> **Felsefemiz:** *Control your AI. Own your Memory.*
> **Geliştirici:** Buğra
> **Güncel Sürüm:** v3.1.0-beta
