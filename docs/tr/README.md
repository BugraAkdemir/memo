<div align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=for-the-badge&logo=go" alt="Go 1.26"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-blue?style=for-the-badge" alt="Lisans AGPL v3"/>
  <img src="https://img.shields.io/badge/Durum-v3.3.3--Açık_Beta-blue?style=for-the-badge" alt="v3.3.3"/>
  <br/>
  <img src="https://img.shields.io/badge/llama.cpp-Entegre-orange?style=flat-square&logo=llama" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-Aktif-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/Google_Drive_Bulut_Yedek-Entegre-blue?style=flat-square&logo=googledrive" alt="Google Drive"/>
  <img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Çapraz Platform"/>
</div>

<h1 align="center">
  🧠 Memo — Yapay Zeka Hafıza Kabuğu
</h1>

<p align="center">
  <b>Yerel. Gizli. Senin İkinci Beynin.</b><br/>
  <i>Yerel LLM'ler için yüksek performanslı, gizlilik odaklı bir Hafıza Kabuğu</i>
</p>

<p align="center">
  <a href="#-özellikler">Özellikler</a> •
  <a href="#-hızlı-başlangıç">Hızlı Başlangıç</a> •
  <a href="#-mimari">Mimari</a> •
  <a href="#-ekran-görüntüleri">Ekran Görüntüleri</a> •
  <a href="../../README.md">English</a>
</p>

---

> **⚠️ Güncel Durum:** v3.3.3 açık beta (v3.3.4 geliştirmede) — Agent modu, Orchestra, WhatsApp entegrasyonu, bulut senkronizasyonu, takvim, rutinler, proaktif öğrenme, (beta) Sesli Mod, (beta) Claude Code/Codex CLI sağlayıcıları ve Developer API Gateway aktif. [Bilinen sorunlar →](./BILINEN_SORUNLAR.md) · Güncel açık bug sayısı: `BUG_REPORT.md`'ye bakın (0)

---

## ✨ Özellikler

<table>
  <tr>
    <td width="50%">
      <h3>🧠 <b>Yerel RAG Motoru</b></h3>
      Her etkileşim anlamsal olarak indekslenir. Her yanıttan önce geçmiş bağlamınızı hatırlar — nasıl düşündüğünüzü öğrenir.
    </td>
    <td width="50%">
      <h3>🔒 <b>%100 Gizlilik</b></h3>
      Hiçbir veri makinenizden çıkmaz. Bulut yok, telemetri yok, konuşmalarınız eğitim için kullanılmaz. İkinci beyniniz sadece size ait.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>⚡ <b>Yerel llama.cpp</b></h3>
      Entegre llama-server yönetimi. Otomatik indir, otomatik başlat, GPU hızlandırma ile VRAM algılama. Docker yok, container yok.
    </td>
    <td width="50%">
      <h3>💬 <b>Gerçek Zamanlı Sohbet</b></h3>
      SSE token akışı ile anlık mesajlaşma. Düşünme/akıl yürütme metni ayrıştırma. Multimodal modeller (metin + görsel) desteği.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🤗 <b>HuggingFace Entegrasyonu</b></h3>
      HuggingFace Hub'dan GGUF modellerini arayın, indirin ve yönetin. Tek tıkla model değiştirme.
    </td>
    <td width="50%">
      <h3>☁️ <b>Şifreli Bulut Yedek</b></h3>
      AES-256-GCM uçtan uca şifreleme ile isteğe bağlı Google Drive yedekleme. Senin parolan, senin anahtarın.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🎨 <b>Greige Tasarım</b></h3>
      Sıcak, minimal bej-gri palet ile premium Material 3 arayüz. Karanlık mod dahil. Göz yormaz.
    </td>
    <td width="50%">
      <h3>🛠️ <b>Model Bağımsız</b></h3>
      OpenAI uyumlu herhangi bir yerel sunucu ile çalışır. llama.cpp, Ollama, LM Studio — modelini kendin getir.
    </td>
  </tr>
</table>

---

## 🚀 Hızlı Başlangıç

### Gereksinimler
- **Go 1.26+** — [indir](https://go.dev/dl/)
- **Flutter 3.10+** — [kur](https://docs.flutter.dev/get-started/install)
- **llama.cpp** — gömülü! Uygulama, platformunuz için önceden derlenmiş llama-server binary'leri ile birlikte gelir. Elle kurulum gerekmez.

### Hızlı Başlangıç (Linux)
```bash
# Projeyi klonla ve backend'i çalıştır
git clone https://github.com/bugra/memo.git && cd memo
go run . --port 8090

# Başka bir terminalde frontend'i çalıştır
cd frontend && flutter run -d linux
```

İlk modeli başlattığınızda uygulama, `binaries/` klasöründeki gömülü llama-server'ı kullanır. GGUF modelleri doğrudan HuggingFace üzerinden arayüzden indirilir.

### Taşınabilir Sürüm Derleme
```bash
./scripts/build_releases.sh
# Çıktı: build_output/dist/
#   Memo-linux-x64-v3.5.0.tar.gz
#   Memo-linux-x64-v3.5.0.AppImage
#   Memo-linux-x64-v3.5.0.deb
```

---

## 🏛️ Mimari

```
┌─────────────────────────────────────────────────┐
│            Flutter Masaüstü İstemci               │
│  ┌─────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ Sohbet  │  │ Ayarlar  │  │ Model Deposu  │  │
│  └────┬────┘  └────┬─────┘  └──────┬────────┘  │
│       └────────────┼───────────────┘            │
│              ┌─────┴──────┐                     │
│              │  Riverpod  │                     │
│              │  Providers │                     │
│              └─────┬──────┘                     │
│              ┌─────┴──────┐                     │
│              │ ApiClient  │                     │
│              │ (Dio/SSE)  │                     │
│              └─────┬──────┘                     │
└────────────────────┼────────────────────────────┘
                     │ REST + SSE (localhost:8090)
┌────────────────────┼────────────────────────────┐
│          Go Arka Uç Sunucusu                     │
│  ┌─────────────────┴─────────────────┐          │
│  │          Web Sunucusu             │          │
│  │  (server.go + handlers_flutter)   │          │
│  └─────────────────┬─────────────────┘          │
│  ┌─────────────────┴─────────────────┐          │
│  │          app.go (Motor)           │          │
│  └──┬──────────┬──────────┬──────────┘          │
│  ┌──┴──┐  ┌────┴────┐  ┌─┴─────────┐           │
│  │Hafıza│  │Oturum  │  │Llama Yön. │           │
│  │Deposu│  │Yönetici│  │(alt süreç)│           │
│  │vec0  │  │JSON    │  │llama.cpp  │           │
│  │SQLite│  │         │  │           │           │
│  └─────┘  └─────────┘  └───────────┘           │
│  ┌──────────┐  ┌──────────────┐                 │
│  │Bulut Sync│  │Model Deposu │                  │
│  │Drive+AES │  │HF API+yerel │                  │
│  └──────────┘  └──────────────┘                 │
└─────────────────────────────────────────────────┘
```

**Detaylı mimari:** [../../architecture.md](../../architecture.md)

---

## 🖼️ Ekran Görüntüleri

<div align="center">
  <p><i>(Ekran görüntüleri yakında — v3.0.0 sürümü ile)</i></p>
  <table>
    <tr>
      <td align="center"><b>💬 Sohbet</b></td>
      <td align="center"><b>⚙️ Ayarlar</b></td>
      <td align="center"><b>🤗 Model Deposu</b></td>
    </tr>
    <tr>
      <td><img src="https://via.placeholder.com/300x200/e8ddd3/333?text=Sohbet+Ekrani" alt="Sohbet Ekranı"/></td>
      <td><img src="https://via.placeholder.com/300x200/e8ddd3/333?text=Ayarlar" alt="Ayarlar"/></td>
      <td><img src="https://via.placeholder.com/300x200/e8ddd3/333?text=Model+Deposu" alt="Model Deposu"/></td>
    </tr>
  </table>
</div>

---

## 📚 Dokümantasyon

| Belge | Açıklama |
|---|---|
| [📖 Bilinen Sorunlar](./BILINEN_SORUNLAR.md) | Tam denetim — bilinen sorunlar |
| [🏛️ Mimari](../../architecture.md) | Teknik derinlemesine analiz |
| [📡 API Referansı](./API_REFERENCE.md) | Tüm REST endpoint'leri |
| [🔧 Sorun Giderme](./TROUBLESHOOTING.md) | Sık karşılaşılan sorunlar ve çözümleri |
| [🤝 Katkıda Bulunma](./CONTRIBUTING.md) | Nasıl katkıda bulunabilirsiniz |

---

## 🧪 Teknoloji Yığını

<div align="center">

| Katman | Teknoloji |
|---|---|---|
| **Arka Uç** | Go 1.26, http.ServeMux |
| **Ön Yüz** | Flutter 3.10+, Riverpod 2.x, Dio |
| **LLM** | llama.cpp, OpenAI uyumlu API |
| **Vektör** | SQLite + sqlite-vec (vec0 ANN indeksi) |
| **Senkronizasyon** | Google Drive API, AES-256-GCM |
| **GPU** | nvidia-smi, rocm-smi, sysfs, Metal |
| **Derleme** | Go toolchain, Flutter build, shell script |

</div>

---

## 🤝 Katkıda Bulunma

Katkılarınızı bekliyoruz! Şuraya göz atın:
- [Bilinen Sorunlar](./BILINEN_SORUNLAR.md) — 🔴 veya 🟠 bir madde seçin
- [Görev Listesi](../../task.md) — frontend'e özel görevler
- [Katkıda Bulunma Rehberi](./CONTRIBUTING.md)

---

<div align="center">
  <h3>🧠 <i>Senin Zihnin. Senin Verin. Senin Bilgisayarın.</i></h3>
  <p>❤️ ile <b>Buğra</b> tarafından yapıldı</p>
  <p>
    <a href="https://github.com/bugra/memo/issues">Hata Bildir</a> •
    <a href="https://github.com/bugra/memo/discussions">Tartışma</a> •
    <a href="../../README.md">English</a>
  </p>
</div>
