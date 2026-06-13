<div align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go" alt="Go 1.25"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-blue?style=for-the-badge" alt="Lisans AGPL v3"/>
  <img src="https://img.shields.io/badge/Sürüm-v3.1.0--beta-blue?style=for-the-badge" alt="v3.1.0-beta"/>
  <br/>
  <img src="https://img.shields.io/badge/llama.cpp-Entegre-orange?style=flat-square&logo=llama" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-Aktif-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-Entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Ajan-8_Araç-B08D57?style=flat-square" alt="Ajan"/>
  <img src="https://img.shields.io/badge/Yedek-.memo_ZIP-blue?style=flat-square" alt="Yedek"/>
  <img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Çapraz Platform"/>
</div>

<h1 align="center">
  🧠 Memo — Yapay Zeka Hafıza Kabuğu
</h1>

<p align="center">
  <b>Yerel. Gizli. İkinci Beyniniz.</b><br/>
  <i>Kalıcı RAG hafızası, harici sağlayıcılar, ajan motoru, WhatsApp ve premium bir masaüstü deneyimiyle gizlilik odaklı bir yapay zeka asistanı.</i>
</p>

<p align="center">
  <a href="#-neden-memo">Neden Memo</a> •
  <a href="#-özellikler">Özellikler</a> •
  <a href="#-tasarım">Tasarım</a> •
  <a href="#-hızlı-başlangıç">Hızlı Başlangıç</a> •
  <a href="#-mimari">Mimari</a> •
  <a href="#-yol-haritası">Yol Haritası</a> •
  <a href="README.md">English</a>
</p>

---

> **Güncel sürüm:** `v3.1.0-beta` — RAG hafıza, harici sağlayıcılar, ajan & orkestra motorları, WhatsApp, şifreli bulut senkronizasyonu, mobil yardımcı ve sıfırdan yapılan **"Pewter Study"** arayüz tasarımı. [Yol haritası →](docs/tr/ROADMAP.md)

---

## 🎯 Neden Memo

Çoğu yapay zeka asistanı konuşmalarınızı başkasının sunucularına gönderir. **Memo göndermez.** Modeli kendi makinenizde çalıştırır, her hafızayı yerel bir vektör veritabanında saklar ve asla "eve telefon etmez". Modelin, verinin ve onu barındıran diskin sahibi sizsiniz.

Ama yerel-öncelikli olmak, kullanması zor olmak demek değildir. Memo; gerçek bir RAG hafıza motorunu ve araç-çağıran bir ajanı, ilk kez açan birinin bile gezinebileceği bir arayüzle birleştirir — **tek tıkla model indir, indirmeden önce donanımına uygun mu gör ve neyin çalıştığını her an bil.**

- **🔒 Tasarımı gereği gizli** — telemetri yok, sohbetlerin üzerinde eğitim yok, bulut bağımlılığı yok. Şifreli yedekleme yalnızca *siz* açarsanız.
- **🧠 Gerçek hafıza** — her etkileşim vektörleştirilip indekslenir; her turda ilgili bağlam otomatik getirilir.
- **🤝 İki dünyanın en iyisi** — sohbeti güçlü bir harici API üzerinden çalıştırırken embedding'leri küçük bir yerel model yapar, ya da %100 çevrimdışı kal.
- **🖥️ Native, web sarmalayıcı değil** — Linux, Windows ve macOS'ta bir Flutter masaüstü uygulaması, ayrıca bir mobil yardımcı.

---

## ✨ Özellikler

<table>
  <tr>
    <td width="50%">
      <h3>🧠 <b>Yerel RAG Motoru</b></h3>
      SQLite + sqlite-vec (vec0 ANN indeksi). Her etkileşim anlamsal indekslenir; her ölçekte O(log n) erişim. Bulut yok, üçüncü taraf embedding yok.
    </td>
    <td width="50%">
      <h3>🤖 <b>Ajan Motoru</b></h3>
      8 yerleşik araçlı, araç-çağıran bir pipeline; 6 politikalı izin sistemi (bir kez/oturum/kalıcı izin-ret), çalıştırma kum havuzu, hız sınırlama ve denetim günlüğü.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🎵 <b>Orkestra Modu</b></h3>
      Çoklu model işbirliği: şef model görevi planlar ve böler, uzman roller paralel çalışır, şef sonucu sentezler. 8 yerleşik rol.
    </td>
    <td width="50%">
      <h3>🔄 <b>Çapraz-Mod Mimarisi</b></h3>
      Sohbet için harici API sağlayıcıları kullanırken embedding'ler için küçük bir yerel model çalıştır — güç ve gizlilik, bağımsız yapılandırılabilir.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>⚡ <b>Yerel llama.cpp</b></h3>
      Gömülü <code>llama-server</code> yaşam döngüsü yönetimi: otomatik indirme, otomatik başlatma, VRAM tespiti ile GPU hızlandırma (NVIDIA / AMD / Metal). Docker yok, konteyner yok.
    </td>
    <td width="50%">
      <h3>🏭 <b>Rehberli Model Mağazası</b></h3>
      Seçilmiş öneriler, tek tıkla indirme ve RAM/VRAM'ine göre bir <b>donanım-uygunluk rozeti</b>. Quantization otomatik seçilir — şifreli <code>Q4_K_M</code> tahmini yok.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>💬 <b>WhatsApp Entegrasyonu</b></h3>
      QR ile tam WhatsApp Web eşleştirme. Mesaj gönderme/alma, kişi adı çözümleme, beyaz liste tabanlı dosya aktarımı, özel ajan araçları. Sohbetleriniz yerel kalır.
    </td>
    <td width="50%">
      <h3>📦 <b>Yedekleme ve Geri Yükleme</b></h3>
      Tam <code>.memo</code> zip dışa/içe aktarma — oturumlar, yapılandırma, bellek, WhatsApp verisi, sağlayıcılar — ayrıca şifreli Google Drive senkronizasyonu (AES-256-GCM) ve çift onaylı tüm verileri silme.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🛠️ <b>Model ve Sağlayıcı Bağımsız</b></h3>
      OpenAI uyumlu her sunucu (llama.cpp, Ollama, LM Studio). Harici sağlayıcılar: OpenAI, Anthropic, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama — yedekli yönlendirici ile.
    </td>
    <td width="50%">
      <h3>📱 <b>Mobil ve Uzaktan</b></h3>
      LAN veya gömülü ngrok tüneli üzerinden ince bir Flutter mobil yardımcı (Android/iOS); token kimlik doğrulama ve akışlı sohbet.
    </td>
  </tr>
</table>

---

## 🎨 Tasarım

Memo'nun arayüzü **"Pewter Study"** — parlak bir light tema ile mağara-karası dark tema arasında bilinçli olarak duran bir **orta-ton** kimlik: sıcak grafit yüzeyler, metin için yumuşak kırık-beyaz mürekkep ve tek bir **dumanlı bronz** aksan. Neon yok, parlama yok — bir ikinci beyin için sakin, premium bir çalışma yüzeyi.

| İlke | Pratikte |
|------|----------|
| **Tek yüzey, üç katman** | Derinlik renkten değil, katmandan gelir. |
| **Aksanı tek yerde harca** | Bronz yalnız birincil aksiyon, aktif durum ve ilerleme için. |
| **Sade dil** | "Dengeli — önerilen", asla "Q4_K_M". |
| **Donanım-farkında** | Her model "cihazıma uyar mı?" sorusunu önden yanıtlar. |
| **Engine Strip** | Kalıcı bir alt şerit, çalışan sohbet + hafıza modellerini ve tek-tık durdurmayı gösterir. |

Tipografi **Schibsted Grotesk** (başlık), **Inter** (gövde) ve **JetBrains Mono** (kod) eşleşmesidir. İki tema gelir: **Pewter** (varsayılan) ve **Night** (daha koyu). Tam sistem: [frontend/DESIGN.md](frontend/DESIGN.md).

### Model indirme, yeniden hayal edilmiş

Eskiden model indirmek; HuggingFace'te arama yapmak, bir repo açmak ve `…Q4_K_M.gguf`, `…Q5_K_S.gguf`, `…fp16.gguf` gibi çoğu insan için anlamsız adlı dosyalardan birini seçmek demekti. Memo bunu rehberli bir akışla değiştirir:

1. **Keşfet**, seçilmiş ve çalıştığı bilinen modelleri tek cümlelik açıklama ve boyutla gösterir.
2. Tespit edilen **RAM ve VRAM**'ine göre bir **donanım-uygunluk rozeti** ("✓ Cihazına uygun — GPU'da hızlı" / "⚠ Yetersiz olabilir") hesaplanır.
3. **Tek tık**, en uygun quantization'ı otomatik seçip indirir.
4. İleri kullanıcılar yine de herhangi bir HuggingFace reposu için **Gelişmiş arama**yı açabilir; her quant sade dile çevrilmiştir.

---

## 🚀 Hızlı Başlangıç

### Gereksinimler
- **Go 1.25+** — [indir](https://go.dev/dl/)
- **Flutter 3.10+** — [kur](https://docs.flutter.dev/get-started/install)
- **llama.cpp** — gömülü. Platformunuz için önceden derlenmiş binary'ler; manuel kurulum gerekmez.

### Geliştirme
```bash
# Terminal 1 — Backend
git clone https://github.com/BugraAkdemir/memo.git && cd memo
go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

### Sürüm paketi oluşturma
```bash
# Linux  → tar.gz / AppImage / deb
./build_releases.sh

# Windows → Inno Setup kurulumu veya taşınabilir zip
.\build_releases.bat
```

---

## 🏛️ Mimari

```
┌──────────────────────────────────┐  ┌───────────────────────────────┐
│     Flutter Masaüstü İstemci      │  │     Flutter Mobil İstemci     │
│  ┌──────┐ ┌────────┐ ┌────────┐  │  │  ┌──────────┐ ┌──────────┐    │
│  │Sohbet│ │Ayarlar │ │ Model  │  │  │  │ Bağlan   │ │  Sohbet  │    │
│  │+Ajan │ │+Yedek  │ │Mağazası│  │  │  │  Ekranı  │ │  Ekranı  │    │
│  └──┬───┘ └───┬────┘ └───┬────┘  │  │  └────┬─────┘ └────┬─────┘    │
│  ┌──┴─────────┴──────────┴────┐  │  │  ┌────┴────────────┴────┐     │
│  │  Riverpod · SSE · Engine   │  │  │  │   Riverpod · Dio      │     │
│  │  Strip · MemoApiClient     │  │  │  │   MemoApiClient       │     │
│  └───────────┬────────────────┘  │  │  └───────────┬───────────┘     │
└──────────────┼────────────────────┘  └──────────────┼────────────────┘
               │ REST + SSE (:8090)                    │ LAN / ngrok / TLS
┌──────────────┼───────────────────────────────────────┼────────────────┐
│              └──────────────────┬─────────────────────┘                │
│                       Go Backend Sunucusu                              │
│  ┌──────────────────────────────┴──────────────────────────────┐      │
│  │   Web Sunucusu (server.go) · ~35 endpoint (handlers)         │      │
│  └──────────────────────────────┬──────────────────────────────┘      │
│  ┌──────────────────────────────┴──────────────────────────────┐      │
│  │                    App Motoru (app.go)                        │      │
│  └──┬─────────┬──────────┬──────────┬──────────┬──────────┬─────┘      │
│  ┌──┴──┐ ┌────┴───┐ ┌────┴────┐ ┌───┴────┐ ┌───┴────┐ ┌───┴─────┐    │
│  │Bellek│ │Oturum  │ │Llama +  │ │WhatsApp│ │Sağlayıc│ │ Ajan    │    │
│  │vec0 │ │ JSON   │ │Emb Yön. │ │whatsmeow│ │Yönlend.│ │ Motoru  │    │
│  │SQLite│ │       │ │+GPU/RAM │ │        │ │(7 API) │ │(8 araç) │    │
│  └─────┘ └────────┘ └─────────┘ └────────┘ └────────┘ └─────────┘    │
│  ┌──────────┐ ┌──────────────┐ ┌──────────┐ ┌──────────┐             │
│  │Orkestra  │ │Model Deposu  │ │Bulut Senk│ │ ngrok    │             │
│  │(8 rol)   │ │HF + yerel    │ │ (Drive)  │ │ Tüneli   │             │
│  └──────────┘ └──────────────┘ └──────────┘ └──────────┘             │
└──────────────────────────────────────────────────────────────────────┘
```

**Derinlemesine:** [docs/architecture.md](docs/architecture.md) · **API:** [docs/API_REFERENCE.md](docs/API_REFERENCE.md)

---

## 🛣️ Yol Haritası

| Sürüm | Tema | Durum |
|-------|------|-------|
| **v3.1.0** | Hafıza — RAG, WhatsApp, Yedekleme, Mobil, Uzaktan Erişim | ✅ Yayınlandı |
| **v3.2.0** | Zamanlanmış Zeka — Takvim, Ajan Arayüzü, Mobil Bildirimler | 🚧 Geliştiriliyor |
| **v3.3.0** | Mobil & Ses — Mobil v2 + Ses Asistanı | 🚧 Planlandı |
| **v3.4.0** | Eklenti & Web — Eklenti Sistemi + Web Arama | 🚧 Planlandı |
| **v3.5.0** | Daha Akıllı Memo — Bilgi Grafiği, Kendini Geliştiren Hafıza | 🔮 Gelecek |

[Tam yol haritası →](docs/tr/ROADMAP.md)

---

## 📚 Dokümantasyon

| Döküman | Açıklama |
|---------|----------|
| [🎨 Tasarım Sistemi](frontend/DESIGN.md) | "Pewter Study" token'ları, bileşenler ve ekranlar |
| [🏛️ Mimari](docs/architecture.md) | Her bileşenin teknik derinlemesine analizi |
| [📡 API Referansı](docs/API_REFERENCE.md) | Tüm REST endpoint'leri |
| [🛣️ Yol Haritası](docs/tr/ROADMAP.md) | Stratejik vizyon ve sürüm planı |
| [📱 Mobil README](mobile/README.md) | Mobil yardımcı uygulama dokümanları |
| [📖 Bilinen Sorunlar](docs/KNOWN_ISSUES.md) | Önceliklendirilmiş tam denetim |
| [🔧 Sorun Giderme](docs/TROUBLESHOOTING.md) | Sık karşılaşılan sorunlar ve çözümleri |
| [📝 Katkıda Bulunma](docs/CONTRIBUTING.md) | Nasıl katkıda bulunabilirsiniz |

---

## 🧪 Teknoloji Yığını

<div align="center">

| Katman | Teknoloji |
|--------|-----------|
| **Backend** | Go 1.25, `http.ServeMux`, SSE akış |
| **Frontend (Masaüstü)** | Flutter 3.10+, Riverpod 2.x, Dio, flutter_markdown, google_fonts |
| **Frontend (Mobil)** | Flutter 3.10+, Riverpod 2.x, Dio (Android · iOS · Web) |
| **LLM Çalışma Zamanı** | llama.cpp (gömülü), OpenAI uyumlu API |
| **Harici Sağlayıcılar** | OpenAI · Anthropic Claude · Google Gemini · xAI Grok · Groq · OpenRouter · Ollama |
| **Vektör Deposu** | SQLite + sqlite-vec (vec0 ANN indeksi) |
| **WhatsApp** | whatsmeow (çoklu cihaz Web API) |
| **Donanım Tespiti** | nvidia-smi · rocm-smi · Metal · sistem RAM (/proc, GlobalMemoryStatusEx, sysctl) |
| **Bulut Senkronizasyonu** | Google Drive OAuth2 + AES-256-GCM |
| **Derleme** | Go araç zinciri, Flutter build, shell scriptleri, Inno Setup |
| **Lisans** | GNU AGPL v3 |

</div>

---

## 🤝 Katkıda Bulunma

Katkılarınızı bekliyoruz:
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
