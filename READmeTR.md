<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="120"/>

  <h1>Memo</h1>
  <p><b>Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.</b></p>
  <p>Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı çalışabilir</p>

  <br/>

  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/>
  </a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers">
    <img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57" alt="Yıldız"/>
  </a>
  &nbsp;
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-blue?style=for-the-badge" alt="Lisans"/>
  &nbsp;
  <img src="https://img.shields.io/badge/Sürüm-v3.1.0_beta-blue?style=for-the-badge" alt="Sürüm"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-gömülü-orange?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_vec0-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-lightgrey?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/CI-geçiyor-success?style=flat-square&logo=githubactions" alt="CI"/>

</div>

---

## Memo Nedir?

Çoğu yapay zeka asistanı bir sohbet kutusudur — yaz, cevap al, unut.

**Memo farklı.** Nasıl çalıştığını izler, alışkanlıklarını öğrenir ve *sen sormadan önce* yardıma gelir. Her akşam 21:00'da kod yazdığını bilir. Pazartesi sabahlarının planlama zamanı olduğunu bilir. Zamanı gelince karşına çıkar — önerir, telefonuna bildirim gönderir ya da ajanı kendi kendine başlatır.

Her şey senin bilgisayarında çalışır. Bulut yok, telemetri yok. Konuşmaların, hafızan, alışkanlıkların — hiçbiri bilgisayarından çıkmaz. Tamamen çevrimdışı, yerel modellerle kullanabilir ya da API anahtarlarını girip bulut sağlayıcılara bağlanabilirsin — seçim senin.

---

## Özelliklere Genel Bakış

| Kategori | Ne yapar |
|----------|---------|
| **Sohbet** | Akışlı çok turlu AI sohbeti, markdown render, görsel/dosya eki, gizli mod. Tamamen TR/EN iki dilli. |
| **RAG Hafıza** | SQLite + sqlite-vec vektör deposu konuşmalarını hatırlar, alakalı bağlamı otomatik getirir. |
| **Ajan Motoru** | 8 yerleşik araç (dosya, shell, web arama, WhatsApp). 6 politikali izin sistemi. Araç başına 60sn zaman aşımı. |
| **Orkestra** | Çoklu model iş akışı: Şef model görevi böler, 8 uzman rol paralel çalışır, sonuç sentezlenir. |
| **WhatsApp** | QR eşleştirme ile tam WhatsApp Web (whatsmeow). Oku, yanıtla, ara, özetle — API ücreti yok, tamamen yerel. |
| **Takvim** | Konuşmalardan niyet çıkarımı → otomatik etkinlik → hatırlatma bildirimi. Manuel etkinlik de eklenebilir. |
| **Proaktif** | 7 gün sessiz gözlem → pattern tespiti → hafif öneriler → yüksek güvenle otomatik ajan tetikleme. |
| **Skill Sistemi** | `data/skills/` dizinine `SKILL.md` bırak, özel persona, alan uzmanı veya iş akışı ekle. Kod yazmak gerekmez. |
| **Model Mağazası** | Donanım uygunluk rozetli (RAM/VRAM) HuggingFace tarayıcısı. Tek tık indirme, akıllı quantization seçimi. |
| **Ses (Whisper)** | Cihaz üzerinde whisper.cpp ile konuşma-metne çevrimi. TR/EN/karışık otomatik algılama. İnternet gerekmez. |
| **Bulut Senkron.** | İsteğe bağlı E2E şifreli Google Drive yedekleme (AES-256-GCM, PBKDF2 600K iterasyon). Çoklu cihaz. |
| **Uzaktan Erişim** | Dahili ngrok + Tailscale (tsnet) tünelleri. Otomatik CORS'lu LAN erişimi. Tünel üzerinden mobil bağlantı. |
| **Onboarding** | Kurulum sihirbazı → launchpad özellik kartları → spotlight ikon turu → açıklamalı boş ekranlar. |
| **GPU Algılama** | NVIDIA (CUDA), AMD (ROCm/Vulkan), Metal. Otomatik VRAM tespiti. Önerilen motor modu. |
| **8 Sağlayıcı** | OpenAI, Anthropic, Gemini, Grok, Groq, OpenRouter, Ollama, llama.cpp — akıllı yedek zinciri ile. |
| **Çift Dil** | 300+ L10n anahtarı ile tam TR/EN desteği. Takvim, Ayarlar, Sohbet, Onboarding hepsi iki dilli. |
| **Production** | Rate limiting (IP başına 100 istek/sn), 50MB body limit, 0600 dosya izinleri, `crypto/rand` anahtar türetme, CI/CD. |

---

## Hızlı Başlangıç

**Terminal yok. Derleme yok. Tek tık.**

| Platform | İndirme | Nasıl |
|----------|---------|-------|
| **Windows** | `Memo-Setup.exe` | Kurulumu çalıştır → bitti |
| **Linux** | `.AppImage` | `chmod +x` → başlat |
| **Linux** | `.deb` | `sudo dpkg -i` → bitti |

llama.cpp gömülü gelir. İlk başlatmada her şey `~/.memo` altına kopyalanır. Uygulamayı aç, **Model Mağazası**'na git, bir model seç, sohbete başla.

<div align="center">
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Memo'yu_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/>
  </a>
</div>

<details>
<summary><b>Kaynaktan derleme</b></summary>

**Gereksinimler:** Go 1.26+ · Flutter 3.10+ · SQLite geliştirme kütüphaneleri (CGO için)

```bash
# Backend
git clone https://github.com/BugraAkdemir/memo.git
cd memo
CGO_ENABLED=1 go run . --port 8090

# Frontend (ayrı terminal)
cd frontend
flutter run -d linux
```

Sürüm paketleri:
```bash
./build_releases.sh     # Linux  → AppImage / deb / tar.gz
.\build_releases.bat    # Windows → Inno Setup kurulumu / zip
```
</details>

---

## Mimari

```
┌─────────────────────────────┐    ┌──────────────────────────┐
│  Flutter Masaüstü            │    │  Flutter Mobil            │
│  (Linux / Windows)           │    │  (Android / iOS)          │
│                              │    │                           │
│  Sohbet · Ajan · Orkestra    │    │  Sohbet · Bildirimler     │
│  Ayarlar · Model Mağazası    │    │  Uzak bağlantı            │
└──────────────┬───────────────┘    └───────────┬───────────────┘
               │  REST + SSE (:8090)             │  LAN / ngrok
               └──────────────┬──────────────────┘
┌──────────────────────────────┴──────────────────────────────────┐
│                    Go Backend (25 paket)                          │
│                                                                  │
│  Web Sunucu ──── App Motoru ──── Proaktif Motor                  │
│  (~55 route)       (app/)           Gözlemci → Analizci → Eylem  │
│       │                │                                         │
│  ┌────┴────┬───────────┼──────────┬──────────┬──────────┐       │
│  │ Hafıza  │ Sağlayıcı │ Llama    │ Ajan     │ WhatsApp │       │
│  │ vec0    │ Router    │ GPU      │ Pipeline │ whatsmeow│       │
│  └─────────┴───────────┴──────────┴──────────┴──────────┘       │
│                                                                  │
│  Orkestra  ·  Model Mağazası  ·  Bulut Senk  ·  Takvim          │
│  ngrok     ·  Tailscale       ·  Whisper     ·  Skill'ler       │
└──────────────────────────────────────────────────────────────────┘
```

**İki süreçli, düz HTTP.** Frontend backend ile `localhost:8090` üzerinden REST + SSE akışı ile haberleşir. TLS yok (sadece yerel). Router framework'ü yok — saf `net/http` ServeMux.

**Belgeler:** [Mimari](docs/architecture.md) · [API Referansı](docs/API_REFERENCE.md) · [Tasarım Sistemi](frontend/DESIGN.md)

---

## Teknoloji Yığını

| Katman | Teknoloji |
|--------|-----------|
| **Backend** | Go 1.26, `net/http`, CGO (SQLite için) |
| **Masaüstü Frontend** | Flutter 3.10, Riverpod 2.4, Dio 5.4, flutter_markdown 0.6 |
| **Mobil Frontend** | Flutter 3.10, Riverpod, Dio |
| **LLM Çalışma Zamanı** | llama.cpp (gömülü binary), OpenAI uyumlu HTTP API |
| **Vektör Deposu** | SQLite + sqlite-vec (vec0 ANN indeks, 768 boyut) |
| **WhatsApp** | whatsmeow (Go, çoklu cihaz Web API) |
| **Ses-Metne** | whisper.cpp (gömülü binary) |
| **Bulut Senk.** | Google Drive API v3, OAuth2, AES-256-GCM, PBKDF2 |
| **GPU Tespiti** | nvidia-smi, rocm-smi, sysfs, GlobalMemoryStatusEx |
| **Loglama** | `internal/logx` — seviyeli yapılandırılmış slog wrapper |
| **CI/CD** | GitHub Actions: Go vet + test + build, Flutter analyze + test |
| **Lisans** | GNU AGPL v3 |

---

## Nasıl Çalışır

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Memo ile    │ ──→ │  RAG Hafıza  │ ──→ │  Model tam    │
│  sohbet      │     │  bağlam ekler│     │  resmi görür  │
└──────────────┘     └──────────────┘     └──────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Gözlemci    │ ──→ │  Proaktif    │ ──→ │  Ajan iş      │
│  izler       │     │  eşleştirir  │     │  yapar/önerir │
└──────────────┘     └──────────────┘     └──────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Takvim      │ ──→ │  Niyet       │ ──→ │  Hatırlatma   │
│  saklar      │     │  çıkarımı    │     │  tetiklenir   │
└──────────────┘     └──────────────┘     └──────────────┘
```

---

## İlk Kullanım Deneyimi

1. **Kurulum Sihirbazı** — dil, tema ve asistan kişiliği seç (6 hazır seçenek veya özel)
2. **Launchpad** — 5 özellik kartı: Sohbet, Ajan, Orkestra, WhatsApp, Takvim'i açıklar
3. **Spotlight Tur** — 4 adımda kenar çubuğu ikonlarını ışıklandırarak tanıtır
4. **Açıklamalı Boş Ekranlar** — her sekme boşken ne işe yaradığını anlatır

Hepsi Ayarlar → Genel'den tekrar başlatılabilir. Sonraki açılışlarda gösterilmez.

---

## Güvenlik ve Gizlilik

| Alan | Uygulama |
|------|---------|
| **API anahtarları** | `crypto/rand` ile üretilen makine anahtarıyla AES-256-GCM şifreli (`data/machine.key`, 0600) |
| **Yapılandırma dosyaları** | `0600` izinleriyle yazılır (sadece sahibi okuyabilir) |
| **Bulut senk.** | PBKDF2 (600K iterasyon) → AES-256-GCM. Yüklemeden *önce* şifrelenir. |
| **Rate limiting** | IP başına token bucket: 100 istek/sn, 200 burst. Aşım → 429. |
| **Body limitleri** | Tüm handler'larda 50MB `MaxBytesReader` |
| **Gizli mod** | Oturum kaydı yok, hafızaya yazma yok, gözlem yok |
| **Gözlem** | Sadece konu etiketleri ve kelime sayıları saklanır — asla ham mesaj metni değil |
| **Telemetri** | Yok. Sıfır. |

---

## Yol Haritası

| Sürüm | Tema | Durum |
|-------|------|-------|
| **v3.1** | RAG hafıza · WhatsApp · Yedekleme · Mobil · Proaktif · Onboarding · Production sertleştirme | ✅ Beta |
| **v3.2** | Stable sürüm · UI cilası · Proaktif arayüz · Mobil bildirimler | 🚧 Geliştiriliyor |
| **v3.3** | Ses asistanı · Mobil v2 · Takvim v2 | 📅 Planlandı |
| **v3.4** | Eklenti sistemi · Web araması v2 · Tarayıcı uzantısı | 📅 Planlandı |

[Tam yol haritası →](docs/ROADMAP.md) · [Değişiklik günlüğü →](versinNote/tr/v3.1.0.md)

---

## Belgeler

| | |
|-|-|
| [🏛️ Mimari](docs/architecture.md) | Paket haritası, veri akışı, modül sorumlulukları |
| [📡 API Referansı](docs/API_REFERENCE.md) | 55+ REST endpoint'i istek/yanıt formatlarıyla |
| [🎨 Tasarım Sistemi](frontend/DESIGN.md) | "Pewter Study" tema token'ları ve bileşen kalıpları |
| [🛣️ Yol Haritası](docs/ROADMAP.md) | Sürümlü yayın planı |
| [📱 Mobil](mobile/README.md) | Flutter mobil yardımcı kurulumu ve tünel yapılandırması |
| [🔧 Sorun Giderme](docs/TROUBLESHOOTING.md) | Sık karşılaşılan sorunlar, GPU kurulumu, port çakışmaları |
| [📝 Katkı](docs/CONTRIBUTING.md) | Kurulum, kod stili, PR süreci |
| [📋 Değişiklik Günlüğü](versinNote/tr/v3.1.0.md) | Tam v3.1.0 özellik listesi ve hata düzeltmeleri |

---

## Katkıda Bulunma

Memo AGPL-3.0 lisanslıdır ve katkılara açıktır.

- [Yol Haritası](docs/ROADMAP.md)'nı incele — planlanan özellikler
- [Bilinen Sorunlar](docs/KNOWN_ISSUES.md)'a göz at — iyi başlangıç görevleri
- Fikirler için [Tartışma](https://github.com/BugraAkdemir/memo/discussions) aç

---

<div align="center">
  <br/>
  <p><b>Senin zihnin. Senin verin. Senin makinen.</b></p>
  <p><a href="https://github.com/BugraAkdemir">Buğra Akdemir</a> tarafından geliştirildi</p>
  <br/>
  <a href="https://github.com/BugraAkdemir/memo/issues">Hata Bildir</a> ·
  <a href="https://github.com/BugraAkdemir/memo/discussions">Tartışma</a> ·
  <a href="README.md">English</a>
</div>
