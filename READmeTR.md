<div align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go" alt="Go 1.25"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-blue?style=for-the-badge" alt="Lisans AGPL v3"/>
  <img src="https://img.shields.io/badge/Sürüm-v3.1.0--beta-blue?style=for-the-badge" alt="v3.1.0-beta"/>
  <br/>
  <img src="https://img.shields.io/badge/llama.cpp-Entegre-orange?style=flat-square&logo=llama" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-Aktif-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-Entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Yedek-.memo_ZIP-blue?style=flat-square" alt="Yedek"/>
  <img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Çapraz Platform"/>
</div>

<h1 align="center">
  🧠 Memo — Yapay Zeka Hafıza Kabuğu
</h1>

<p align="center">
  <b>Yerel. Gizli. İkinci Beyniniz.</b><br/>
  <i>Kalıcı hafıza, WhatsApp entegrasyonu ve akıllı otomasyon ile gizlilik odaklı bir yapay zeka asistanı</i>
</p>

<p align="center">
  <a href="#-özellikler">Özellikler</a> •
  <a href="#-hızlı-başlangıç">Hızlı Başlangıç</a> •
  <a href="#-mimari">Mimari</a> •
  <a href="#-yol-haritası">Yol Haritası</a> •
  <a href="#-dokümantasyon">Dokümantasyon</a>
</p>

---

> **Güncel Sürüm:** v3.1.0-beta — RAG bellek, WhatsApp entegrasyonu, yerel embedding, yedekleme/geri yükleme. [Yol haritası →](docs/tr/ROADMAP.md)

---

## ✨ Özellikler

<table>
  <tr>
    <td width="50%">
      <h3>🧠 <b>Yerel RAG Motoru</b></h3>
      SQLite + sqlite-vec ANN vektör indeksi. Her etkileşim anlamsal olarak indekslenir. Her ölçekte O(log n) erişim. Bulut yok, üçüncü taraf embedding yok.
    </td>
    <td width="50%">
      <h3>💬 <b>WhatsApp Entegrasyonu</b></h3>
      QR ile tam WhatsApp Web eşleştirme. Mesaj gönderme/alma, kişi adı çözümleme, beyaz liste tabanlı dosya aktarımı, özel agent araçları. Sohbetleriniz yerel kalır.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🔒 <b>%100 Gizlilik</b></h3>
      Verileriniz makinenizden asla çıkmaz. Telemetri yok, konuşmalarınız üzerinde eğitim yok. İkinci beyniniz sadece size aittir.
    </td>
    <td width="50%">
      <h3>⚡ <b>Yerel llama.cpp</b></h3>
      Entegre llama-server yönetimi. Otomatik indirme, otomatik başlatma, VRAM tespiti ile GPU hızlandırma. Docker yok, konteyner yok, bulut bağımlılığı yok.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🔄 <b>Çapraz-Mod Mimarisi</b></h3>
      Sohbet için harici API sağlayıcıları (OpenAI, Claude, Gemini vb.) kullanırken embedding'ler için küçük bir yerel model çalıştırın. Güç + gizlilik bir arada.
    </td>
    <td width="50%">
      <h3>📦 <b>Yedekleme ve Geri Yükleme</b></h3>
      Tam .memo zip dışa/içe aktarma — oturumlar, yapılandırma, bellek, WhatsApp verisi, sağlayıcılar. Çift onaylı tüm verileri silme. Verileriniz tamamen taşınabilir.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🤗 <b>HuggingFace Entegrasyonu</b></h3>
      GGUF modellerini doğrudan HuggingFace Hub'dan arayın, indirin ve yönetin. Tek tıkla model değiştirme. Aramalı ve filtreli model mağazası.
    </td>
    <td width="50%">
      <h3>🎨 <b>Greige Tasarım</b></h3>
      Sıcak, minimal greige paleti ile premium Material 3 arayüz. Koyu mod dahil. Temiz, odaklı, uzun oturumlarda göz yormaz.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🛠️ <b>Model Bağımsız</b></h3>
      OpenAI uyumlu her yerel sunucu ile çalışır. llama.cpp, Ollama, LM Studio — kendi modelinizi getirin. Harici sağlayıcılar: OpenAI, Anthropic, Google, Grok, Groq, OpenRouter, Ollama.
    </td>
    <td width="50%">
      <h3>🖥️ <b>Çapraz Platform</b></h3>
      Linux, Windows, macOS desteği. Flutter ile native masaüstü uygulamaları. Inno Setup ile Windows kurulum. Asistanınız, çalıştırdığınız her yerde.
    </td>
  </tr>
</table>

---

## 🚀 Hızlı Başlangıç

### Gereksinimler
- **Go 1.25+** — [indir](https://go.dev/dl/)
- **Flutter 3.10+** — [kur](https://docs.flutter.dev/get-started/install)
- **llama.cpp** — hazır! Platformunuz için önceden derlenmiş binary'ler. Manuel kurulum gerekmez.

### Geliştirme
```bash
# Terminal 1 — Backend
git clone https://github.com/BugraAkdemir/memo.git && cd memo
go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

### Sürüm Paketi Oluşturma
```bash
# Linux
./build_releases.sh
# Çıktı: build_output/dist/Memo-linux-x64-v3.1.0.tar.gz / .AppImage / .deb

# Windows
.\build_releases.bat
# Çıktı: Memo-Setup-x64-v3.1.0.exe (Inno Setup) veya Memo-win-x64-v3.1.0.zip
```

---

## 🏛️ Mimari

```
┌──────────────────────────────────────────────────────┐
│                 Flutter Masaüstü İstemci               │
│  ┌──────────┐  ┌──────────┐  ┌────────┐  ┌────────┐│
│  │ Sohbet   │  │Ayarlar   │  │Model   │  │WhatsApp││
│  │ + Agent  │  │+ Yedek   │  │Mağazası│  │Ekranı  ││
│  └────┬─────┘  └────┬─────┘  └───┬────┘  └───┬────┘│
│       └──────────────┼────────────┼───────────┘     │
│                ┌─────┴────────────┴──────┐           │
│                │   Riverpod Sağlayıcılar  │           │
│                │  + SSE Stream İşleyici  │           │
│                └─────┬───────────────────┘           │
│                ┌─────┴───────────────────┐           │
│                │  MemoApiClient (Dio)     │           │
│                └─────┬───────────────────┘           │
└──────────────────────┼───────────────────────────────┘
                       │ REST + SSE (localhost:8090)
┌──────────────────────┼───────────────────────────────┐
│               Go Backend Sunucusu                     │
│  ┌────────────────────┴────────────────────┐          │
│  │        Web Sunucusu (server.go)          │          │
│  │    ~35 endpoint (handlers_flutter.go)    │          │
│  └────────────────────┬────────────────────┘          │
│  ┌────────────────────┴────────────────────┐          │
│  │         App Motoru (app.go)              │          │
│  └──┬──────────┬──────────┬──────────┬──────┘          │
│  ┌──┴──┐  ┌────┴────┐  ┌─┴─────────┐  ┌─┴─────────┐   │
│  │Bellek│  │Oturum  │  │Llama      │  │WhatsApp   │   │
│  │Deposu│  │Yönetici│  │Yöneticisi │  │İstemcisi  │   │
│  │vec0  │  │JSON    │  │llama.cpp  │  │whatsmeow  │   │
│  │SQLite│  │        │  │+ Emb Yön  │  │msg depo   │   │
│  └─────┘  └────────┘  └────────────┘  └────────────┘   │
│  ┌──────────┐  ┌──────────────┐  ┌──────────┐          │
│  │Sağlayıcı │  │Model Deposu  │  │Orkestra  │          │
│  │Yönlendir.│  │HF API+yerel  │  │Kondüktör │          │
│  │(6 tür)   │  │              │  │(8 rol)   │          │
│  └──────────┘  └──────────────┘  └──────────┘          │
└────────────────────────────────────────────────────────┘
```

**Derinlemesine:** [docs/architecture.md](docs/architecture.md)

---

## 🛣️ Yol Haritası

| Sürüm | Tema | Durum |
|-------|------|-------|
| **v3.1.0** | Hafıza — RAG, WhatsApp, Yedekleme, Yerel Embedding | ✅ Yayınlandı |
| **v3.2.0** | Zamanlanmış Zeka — Takvim, Cron, Ses, Akıllı Ev | 🚧 Geliştirme Aşamasında |
| **v3.3.0** | Mobil Yardımcı — İnce mobil istemci, uzaktan erişim | 🚧 Planlandı |
| **v3.4.0** | Kişisel Model — 1.2B modeli konuşmalarınızla ince ayar | 🔮 Gelecek |
| **v3.5.0** | Ekosistem — Eklentiler, Bilgi Grafiği, Çoklu Kullanıcı | 🔮 Gelecek |

[Tam yol haritası →](docs/tr/ROADMAP.md)

---

## 📚 Dokümantasyon

| Döküman | Açıklama |
|---------|----------|
| [🛣️ Yol Haritası](docs/tr/ROADMAP.md) | Tam stratejik vizyon ve sürüm planı |
| [🏛️ Mimari](docs/architecture.md) | Bileşenlerin teknik derinlemesine analizi |
| [📡 API Referansı](docs/API_REFERENCE.md) | Tüm REST endpoint'leri |
| [📖 Bilinen Sorunlar](docs/KNOWN_ISSUES.md) | Önceliklendirilmiş tam denetim |
| [🔧 Sorun Giderme](docs/TROUBLESHOOTING.md) | Sık karşılaşılan sorunlar ve çözümleri |
| [📝 Katkıda Bulunma](docs/CONTRIBUTING.md) | Nasıl katkıda bulunabilirsiniz |

---

## 🧪 Teknoloji Yığını

<div align="center">

| Katman | Teknoloji |
|--------|-----------|
| **Backend** | Go 1.25, http.ServeMux |
| **Frontend** | Flutter 3.10+, Riverpod 2.x, Dio, flutter_markdown |
| **LLM Çalışma Zamanı** | llama.cpp (gömülü), OpenAI uyumlu API |
| **Harici Sağlayıcılar** | OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama |
| **Vektör Deposu** | SQLite + sqlite-vec (vec0 ANN indeksi) |
| **WhatsApp** | whatsmeow (çoklu cihaz Web API) |
| **GPU** | nvidia-smi, rocm-smi, sysfs, Metal |
| **Derleme** | Go araç zinciri, Flutter build, shell scriptleri, Inno Setup |
| **Lisans** | GNU AGPL v3 |

</div>

---

## 🤝 Katkıda Bulunma

Katkılarınızı bekliyoruz! Bakmak isteyebileceğiniz kaynaklar:
- [Bilinen Sorunlar](docs/KNOWN_ISSUES.md) — üzerinde çalışmak için bir madde seçin
- [Yol Haritası](docs/tr/ROADMAP.md) — gelecek özellikleri görün
- [Katkıda Bulunma Rehberi](docs/CONTRIBUTING.md)

---

<div align="center">
  <h3>🧠 <i>Sizin Zihniniz. Sizin Veriniz. Sizin Bilgisayarınız.</i></h3>
  <p><b>Buğra Akdemir</b> tarafından ❤️ ile geliştirildi</p>
  <p>
    <a href="https://github.com/BugraAkdemir/memo/issues">Hata Bildir</a> •
    <a href="https://github.com/BugraAkdemir/memo/discussions">Tartışma</a> •
    <a href="README.md">English</a>
  </p>
</div>
