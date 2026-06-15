<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="120"/>

  <h1>Memo</h1>
  <p><b>Alışkanlıklarını öğrenen ve sormadan önce harekete geçen yapay zeka asistanı.</b></p>
  <p>Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlılığı</p>

  <br/>

  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/>
  </a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers">
    <img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57" alt="Yıldızlar"/>
  </a>
  &nbsp;
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-blue?style=for-the-badge" alt="Lisans"/>
  &nbsp;
  <img src="https://img.shields.io/badge/Sürüm-v3.1_beta-blue?style=for-the-badge" alt="Sürüm"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-gömülü-orange?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_vec0-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-lightgrey?style=flat-square" alt="Platform"/>

</div>

---

<!-- SCREENSHOT: Ana ekran görüntüsü buraya gelecek -->
<!-- ![Memo Demo](docs/assets/demo.gif) -->

---

## Memo Nedir?

Çoğu yapay zeka asistanı bir sohbet kutusudur — yaz, cevap al, unut.

**Memo farklı.** Nasıl çalıştığını izler, alışkanlıklarını öğrenir ve *sen sormadan önce* yardıma gelir. Her akşam 21:00'da kod yazdığını bilir. Pazartesi sabahlarının planlama zamanı olduğunu bilir. Zamanı gelince karşına çıkar — önerir, telefonuna bildirim gönderir ya da ajanı kendi kendine başlatır.

Her şey senin bilgisayarında çalışır. Konuşmaların, hafızan, alışkanlıkların — hiçbiri bilgisayarından çıkmaz.

---

## Ekran Görüntüleri

<!-- SCREENSHOT: Proaktif öneri ekranı -->
<!-- ![Proaktif Öneri](docs/assets/proactive.gif) -->

<!-- SCREENSHOT: RAG hafıza arama -->
<!-- ![Hafıza Arama](docs/assets/memory.gif) -->

<!-- SCREENSHOT: WhatsApp entegrasyonu -->
<!-- ![WhatsApp](docs/assets/whatsapp.gif) -->

<!-- SCREENSHOT: Orchestra modu -->
<!-- ![Orchestra](docs/assets/orchestra.gif) -->

> 📸 *Ekran görüntüleri ve demo GIF'leri stable sürümle birlikte gelecek.*

---

## Özellikler

### 🧠 Proaktif Öğrenme Motoru
Memo kullanım alışkanlıklarını sessizce gözlemler. Birkaç gün sonra onları tanımaya başlar — ne zaman kod yazdığını, ne zaman yazdığını, ne zaman planladığını. Dahili proaktif motor, şu anki saati öğrenilen alışkanlıklarla eşleştirir ve ne yapması gerektiğine karar vermesi için bir Orchestra Chief LLM'e sorar: öner, telefona bildirim at, ya da ajanı otomatik başlat. Zayıflayan alışkanlıkları unutur; "artık yapmıyorum" dediğin anda o alışkanlığı tamamen siler. **Tamamen yerel, isteğe bağlı, şeffaf.**

```
1-7. gün   →  Sadece sessiz gözlem
8-14. gün  →  Alışkanlıklar oluşur, hafif öneriler başlar
30. gün+   →  Yüksek güvenli alışkanlıklar ajanı otomatik tetikleyebilir
```

### 💬 WhatsApp Entegrasyonu
QR koduyla tam WhatsApp Web eşleştirme. Mesaj gönder ve al, sohbet geçmişini ara, ajanın mesajları okumasına ve yanıtlamasına izin ver — hepsi whatsmeow aracılığıyla yerel olarak saklanır. WhatsApp API ücreti yok, verilerin cihazından çıkmaz.

### 🤖 Ajan Motoru
8 dahili araçlı araç-çağıran pipeline: dosya okuma/yazma, kabuk komutları, web araması, hafıza sorgusu ve daha fazlası. 6 politikalı izin sistemi (izin ver/reddet — bir kez / bu oturum / kalıcı) ajanın neye dokunabileceği konusunda tam kontrol sağlar.

### 🎵 Orkestra Modu
Çoklu model işbirliği. Bir Şef model görevi rollere böler, uzman modeller paralel çalışır, Şef sonucu sentezler. 8 dahili rol. Aynı pipeline'da yerel ve harici modellerin karışımını destekler.

### 🧩 Skill Sistemi
`data/skills/` dizinine bir `SKILL.md` dosyası bırak, Memo yeni bir yetenek kazanır — özel bir persona, alan uzmanı, özelleştirilmiş iş akışı. Kod yazmak gerekmez.

### ⚡ Yerel llama.cpp
Tam yaşam döngüsü yönetimi ile gömülü `llama-server`: otomatik başlatma, otomatik VRAM tespiti ile GPU hızlandırma (NVIDIA / AMD / Metal). Docker yok, konteyner yok, PATH yapılandırması yok.

### 🏪 Model Mağazası
Gerçek RAM ve VRAM'ine göre hesaplanan **donanım uygunluk rozeti** ile seçilmiş model önerileri. Tek tık indirir ve doğru quantization'ı otomatik seçer. Şifreli `Q4_K_M` tahmini yok.

### 🔌 Sağlayıcı Bağımsız
OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama — veya herhangi bir OpenAI uyumlu yerel sunucu. Sağlayıcıları karıştır: sohbet için güçlü bir harici API kullanırken embedding'ler için küçük bir yerel model çalıştır.

### 📱 Mobil Yardımcı
Bir Flutter mobil uygulaması (Android/iOS) LAN üzerinden veya dahili ngrok tüneli aracılığıyla bağlanır. Proaktif öneriler mobil bildirim olarak gelir. Telefonda tam akışlı sohbet.

### 📦 Yedekleme ve Senkronizasyon
Tam `.memo` ZIP dışa aktarma — oturumlar, hafıza, yapılandırma, WhatsApp verileri, sağlayıcılar. İsteğe bağlı şifreli Google Drive senkronizasyonu (AES-256-GCM). Çift onaylı veri silme.

### 🔒 Tasarımı Gereği Gizli
- Sıfır telemetri
- Konuşmalarında eğitim yok
- Bulut bağımlılığı yok (senkronizasyon yalnızca sen açarsan)
- Gizli mod: mesajlar saklanmaz, gözlemlenmez, vektörleştirilmez
- Gözlem katmanı yalnızca konu etiketlerini ve kelime sayılarını saklar — asla mesaj metni değil

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
<summary><b>Kaynaktan derleme (geliştiriciler)</b></summary>

**Gereksinimler:** Go 1.25+ · Flutter 3.10+

```bash
# Backend
git clone https://github.com/BugraAkdemir/memo.git
cd memo
go run . --port 8090

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
│   Flutter Masaüstü          │    │   Flutter Mobil          │
│   (Linux / Windows)         │    │   (Android / iOS)        │
│                             │    │                          │
│  Sohbet · Ajan · Orkestra   │    │  Sohbet · Bildirimler    │
│  Ayarlar · Model Mağazası   │    │  Uzak bağlantı           │
└──────────────┬──────────────┘    └───────────┬──────────────┘
               │  REST + SSE (:8090)            │  LAN / ngrok
               └──────────────┬─────────────────┘
┌─────────────────────────────┴──────────────────────────────────┐
│                        Go Backend                               │
│                                                                 │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │  Web Sunucu │  │  App Motoru  │  │  Proaktif Motor        │ │
│  │  ~35 route  │  │  (internal/  │  │  Gözlemci → Analizci   │ │
│  │  SSE stream │  │   app/)      │  │  → Şef → Eylem         │ │
│  └─────────────┘  └──────┬───────┘  └────────────────────────┘ │
│                           │                                     │
│  ┌────────┐ ┌──────────┐ ┌┴─────────┐ ┌──────────┐ ┌────────┐ │
│  │ Hafıza │ │ Oturumlar│ │ Llama +  │ │WhatsApp  │ │  Ajan  │ │
│  │ SQLite │ │          │ │ Embedding│ │whatsmeow │ │ Motoru │ │
│  │ vec0   │ │          │ │ GPU/RAM  │ │          │ │8 araç  │ │
│  └────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│                                                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │Orkestra  │ │  Model   │ │  Bulut   │ │  ngrok   │          │
│  │ 8 rol    │ │ Mağazası │ │  Senk    │ │  Tüneli  │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
└────────────────────────────────────────────────────────────────┘
```

**Belgeler:** [Mimari](docs/architecture.md) · [API Referansı](docs/API_REFERENCE.md) · [Tasarım Sistemi](frontend/DESIGN.md)

---

## Teknoloji Yığını

| Katman | Teknoloji |
|--------|-----------|
| **Backend** | Go 1.25, `net/http`, SSE akışı |
| **Masaüstü Frontend** | Flutter 3.10, Riverpod 2.x, Dio |
| **Mobil Frontend** | Flutter 3.10, Riverpod 2.x, Dio |
| **LLM Çalışma Zamanı** | llama.cpp (gömülü), OpenAI uyumlu API |
| **Harici Sağlayıcılar** | OpenAI · Anthropic · Gemini · Grok · Groq · OpenRouter · Ollama |
| **Vektör Deposu** | SQLite + sqlite-vec (vec0 ANN indeksi) |
| **WhatsApp** | whatsmeow (çoklu cihaz Web API) |
| **Öğrenme Motoru** | Özel gözlemci + dairesel istatistik analizci + Orchestra Chief |
| **Donanım Tespiti** | nvidia-smi · rocm-smi · /proc · GlobalMemoryStatusEx |
| **Bulut Senkronizasyonu** | Google Drive OAuth2 + AES-256-GCM |
| **Derleme** | Go araç zinciri · Flutter build · Inno Setup · AppImage |
| **Lisans** | GNU AGPL v3 |

---

## Yol Haritası

| Sürüm | Tema | Durum |
|-------|------|-------|
| **v3.1** | RAG hafıza · WhatsApp · Yedekleme · Mobil · Uzaktan erişim · Proaktif motor | ✅ Beta |
| **v3.2** | Stable sürüm · UI cilası · Proaktif arayüz · Mobil bildirimler | 🚧 Geliştiriliyor |
| **v3.3** | Ses asistanı · Mobil v2 · Takvim entegrasyonu | 📅 Planlandı |
| **v3.4** | Eklenti sistemi · Web araması · Tarayıcı uzantısı | 📅 Planlandı |
| **v3.5** | Bilgi grafiği · Kendini geliştiren hafıza | 🔮 Gelecek |

[Tam yol haritası →](docs/ROADMAP.md)

---

## Belgeler

| | |
|-|-|
| [🏛️ Mimari](docs/architecture.md) | Teknik derinlemesine analiz |
| [📡 API Referansı](docs/API_REFERENCE.md) | Tüm REST endpoint'leri |
| [🎨 Tasarım Sistemi](frontend/DESIGN.md) | "Pewter Study" UI token ve bileşenleri |
| [🛣️ Yol Haritası](docs/ROADMAP.md) | Sürüm planı |
| [📱 Mobil](mobile/README.md) | Mobil yardımcı belgeleri |
| [🔧 Sorun Giderme](docs/TROUBLESHOOTING.md) | Sık karşılaşılan sorunlar |
| [📝 Katkıda Bulunma](docs/CONTRIBUTING.md) | Nasıl katkıda bulunulur |

---

## Katkıda Bulunma

Memo AGPL-3.0 lisanslıdır ve katkılara açıktır.

- [Bilinen Sorunlar](docs/KNOWN_ISSUES.md)'a göz at — iyi bir başlangıç görevi seç
- [Yol Haritası](docs/ROADMAP.md)'nı incele — planlanan özellikler
- Fikirler için [Tartışma](https://github.com/BugraAkdemir/memo/discussions) aç

---

<div align="center">
  <br/>
  <p><b>Senin zihnin. Senin verin. Senin makineniz.</b></p>
  <p><a href="https://github.com/BugraAkdemir">Buğra Akdemir</a> tarafından geliştirildi</p>
  <br/>
  <a href="https://github.com/BugraAkdemir/memo/issues">Hata Bildir</a> ·
  <a href="https://github.com/BugraAkdemir/memo/discussions">Tartışma</a> ·
  <a href="README.md">English</a>
</div>
