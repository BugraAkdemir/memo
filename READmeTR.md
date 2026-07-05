<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="140"/>

  <h1>Memo</h1>

  <h3>Her şeyi hatırlayan ve sen sormadan harekete geçen yerel yapay zeka.</h3>

  <p><sub><b>%100 Yerel</b> · <b>Gizlilik Öncelikli</b> · <b>Sıfır Bulut Bağımlılığı</b> · <b>Tamamen Çevrimdışı</b></sub></p>

  <br/>

  <a href="https://memo.bugradev.com"><img src="https://img.shields.io/badge/⬇_Hemen_İndir-memo.bugradev.com-B08D57?style=for-the-badge&logoColor=white" alt="İndir"/></a>
  <a href="https://github.com/BugraAkdemir/memo/stargazers"><img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57&logo=github&logoColor=white" alt="Yıldız"/></a>
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-0a0a0a?style=for-the-badge" alt="Lisans"/>
  <img src="https://img.shields.io/badge/Sürüm-v3.1.1-B08D57?style=for-the-badge" alt="Sürüm"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-gömülü-F08705?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_+_vec0-0F9D58?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
   <img src="https://img.shields.io/badge/Platform-Linux_|_Windows_|_macOS-6e6e6e?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/CI-GitHub_Actions-2088FF?style=flat-square&logo=githubactions" alt="CI"/>

  <p><b><a href="README.md">🇬🇧 Click for English</a></b></p>

</div>

---

<div align="center">

### Diğer her yapay zeka, sekmeyi kapattığın an seni unutur.<br/>Memo unutmaz.

</div>

Tamamen **senin** makinende çalışır. Haftalar önceki konuşmaları hatırlar. *Ne zaman* çalıştığını öğrenir ve sen daha başlamadan sessizce hazırlanır. Ve gerçekten bir şey **yapmasını** istediğinde — bir dosyayı düzenle, komut çalıştır, WhatsApp'tan mesaj at — yapar.

Bulut yok. Abonelik yok. Telemetri yok. Zihnin sana kalır.

<div align="center">
  <br/>
  <h3>🎬 Ajan'ın işi gerçekten yapışını izle</h3>
  <video src="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/agent.mp4" controls width="80%"></video>
  <p><sub>Dosya okuyor, shell komutu çalıştırıyor, kod yazıyor — canlı, sandbox içinde, izin kontrollü.<br/><a href="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/agent.mp4">▶ Önizleme açılmazsa dokunup oynat</a></sub></p>
</div>

---

## Memo vs. Diğer Tüm Asistanlar

|  | 🤖 Tipik Yapay Zeka | 🧠 **Memo** |
|---|:---:|:---:|
| **Hafıza** | Sekme kapanınca unutur | **Haftalarca** hatırlar (yerel RAG) |
| **Verin** | Buluta gönderilir | **Makineni asla terk etmez** |
| **Çevrimdışı çalışır** | ❌ | ✅ |
| **Gerçek aksiyon alır** | Sadece metin üretir | Dosya düzenler, komut çalıştırır, mesaj atar |
| **Alışkanlıklarını öğrenir** | ❌ | ✅ Proaktif motor |
| **Maliyet** | Aylık abonelik | **Ücretsiz & açık kaynak** |

---

## Memo'yu Memo Yapan Üç Şey

<table>
<tr>
<td align="center" width="33%">
  <h3>🧠 Hatırlar</h3>
  <p>Her sohbet yerel bir vektör veritabanına işlenir. Üç hafta önceki bir projeden bahset, Memo bağlamı zaten bilir.</p>
  <sub><a href="#-ölümsüz-rag-hafızası">Nasıl →</a></sub>
</td>
<td align="center" width="33%">
  <h3>📊 Öğrenir</h3>
  <p>Arka plandaki gözlemci ritimlerini öğrenir. Bir hafta sonra ne zaman kod yazdığını, planladığını bilir — ve öngörür.</p>
  <sub><a href="#️-proaktif-öğrenme-motoru">Nasıl →</a></sub>
</td>
<td align="center" width="33%">
  <h3>⚡ Harekete Geçer</h3>
  <p>Sandbox içinde 8 gerçek araç: dosya oku/yaz, komut çalıştır, web'de ara, WhatsApp'tan mesaj at. Her adımı sen onaylarsın.</p>
  <sub><a href="#-ajan-agent-motoru">Nasıl →</a></sub>
</td>
</tr>
</table>

---

## ✨ Özellikler

### 💬 Yaşayan Bir Sohbet

<table>
<tr>
<td width="62%">

Token-token akan yanıtlar, tam Markdown — sözdizimi vurgulu kod blokları, tablolar, görseller. Bir dosya (metin, PDF, kod) veya görsel bırak, Memo okusun. `/` ile slash-komut paleti, ya da mikrofona basılı tutup **cihaz üzerinde** whisper.cpp ile sesli giriş.

Cümlenin ortasında dil değiştir, o da seninle değişir. Web aramayı aç — her yanıt canlı sonuçlarla zenginleşir, **API anahtarı yok, kurulum yok.**

</td>
<td align="center" width="38%">
  <img src="docs/assets/screen/chatscreen.png" alt="Sohbet Ekranı" width="100%"/>
</td>
</tr>
</table>

---

### 🧠 Ölümsüz RAG Hafızası

<table>
<tr>
<td width="62%">

Her mesajlaşma **768 boyutlu bir vektöre** dönüştürülür ve `sqlite-vec` ANN indekslemeli SQLite'da saklanır. Yeni bir şey sorduğunda Memo en alakalı geçmiş konuşmaları çeker ve prompt'a enjekte eder — model zaten biliyordur.

**Pinecone yok. Embedding API'si yok. Bulut vektör VT'si yok.** Sadece SQLite ve senin GPU'n. *Seni tanıyan* bir asistanla, her seferinde seninle ilk kez tanışan bir asistan arasındaki fark budur.

</td>
<td align="center" width="38%">
  <img src="docs/assets/screen/setting-sc.png" alt="Hafıza Ayarları" width="100%"/>
</td>
</tr>
</table>

> **🎬 Memo'nun seni — oturumlar arası — hatırlamasını izle:**

<div align="center">
  <video src="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/whats-my-name.mp4" controls width="80%"></video>
  <p><sub><a href="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/whats-my-name.mp4">▶ Önizleme açılmazsa dokunup oynat</a></sub></p>
</div>

---

### 🤖 Ajan (Agent) Motoru

<table>
<tr>
<td width="58%">

Memo'ya bir proje klasörü göster, konuşmayı bırakıp **yapmaya** başlasın — dosya oku, yaz, düzenle, sil; dizin listele; shell komutu çalıştır; web'de ara. Yol doğrulaması, symlink koruması ve komut kara listesi olan bir sandbox içinde **8 yerleşik araç.**

Her araç çağrısı önce sorar. **Bir kereliğine, oturum boyunca ya da kalıcı** izin ver veya reddet. Görev başına 20 iterasyon, araç başına 60sn zaman aşımı, istediğin an iptal. Claude Code ya da Cursor gibi hissettirir — minimal, bilgilendirici, asla gürültülü değil.

</td>
<td align="center" width="42%">
  <img src="docs/assets/screen/agent-sc.png" alt="Ajan Ekranı" width="100%"/>
</td>
</tr>
</table>

---

### 📱 Derin WhatsApp Entegrasyonu

<table>
<tr>
<td width="58%">

QR kod ile bağlan — WhatsApp Web ile aynı protokol, **Business API ücreti yok.** Mesajları oku, ara ve yanıtla — hepsi Memo'nun bronz temasına uyan arayüzünde.

Ajan mesaj gönderebilir ve kişileri isimle çözer — sadece *"Berra'ya mesaj at"* de, JID gerekmez. Ve her WhatsApp konuşması, normal sohbetle aynı hafıza, gözlemci ve takvim hatlarını besler.

</td>
<td align="center" width="42%">
  <img src="docs/assets/screen/whatsaap-qr-sc.png" alt="WhatsApp QR" width="100%"/>
</td>
</tr>
</table>

---

### 📅 Akıllı Takvim — Kendini Planlar

<table>
<tr>
<td width="58%">

Asla form doldurmazsın. İki aşamalı bir hat her mesajı zaman kalıpları için tarar (*"yarın"*, *"haftaya salı"*, *"saat 3'te"*) ve yalnızca eşleşenler, yapılandırılmış etkinlik çıkaran bir LLM'e ulaşır.

Hatırlatmalar masaüstü ve mobilde çalışır. *"Belki yarın?"* gibi belirsiz bir şey dediysen — Memo tek dokunuşla onaylayacağın bir belirsizlik etkinliği oluşturur. Atomik SQL işlemleri, bir hatırlatmanın iki kez tetiklenmemesini garanti eder.

</td>
<td align="center" width="42%">
  <img src="docs/assets/screen/clander-dc.png" alt="Takvim" width="100%"/>
</td>
</tr>
</table>

> **🎬 Sohbette bahsedilen bir plan, otomatik olarak takvim etkinliğine dönüşüyor:**

<div align="center">
  <video src="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/calendar.mp4" controls width="80%"></video>
  <p><sub><a href="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/calendar.mp4">▶ Önizleme açılmazsa dokunup oynat</a></sub></p>
</div>

---

### 👁️ Proaktif Öğrenme Motoru

Memo, *sana gelen* tek asistandır. Arka plandaki gözlemci *ne zaman* aktif olduğunu izler — asla ne söylediğini değil. **Dairesel (yönlü) istatistik** ile ritimleri bulur: *"Pazartesi 9–10, planlama"* ya da *"Her gün 21–23, kodlama."*

Güven eşiği aşıldığında karar verir: yararlı bir şey öner, bildirim gönder ya da — yüksek güvende — ajanı otomatik başlat. İsteğe bağlı, şeffaf ve zayıflayan kalıplar unutulur. Yalnızca **konu etiketleri ve zaman damgaları** saklanır — asla ham metnin değil.

> **🎬 Duygu Motoru & isteğe bağlı Öz-Çıkar protokolü iş başında:**

<div align="center">
  <video src="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/moods-and-ozc%C4%B1kar.mp4" controls width="80%"></video>
  <p><sub><a href="https://raw.githubusercontent.com/BugraAkdemir/memo/main/docs/assets/videos/moods-and-ozc%C4%B1kar.mp4">▶ Önizleme açılmazsa dokunup oynat</a></sub></p>
</div>

---

### 🏪 Küratörlü Model Mağazası

<table>
<tr>
<td width="62%">

Doğru modeli bulmak genelde HuggingFace repo'ları ve şifreli dosya adları labirentidir. Memo bunu çözer. GPU'unu ve RAM'ini tespit eder, her indirmeyi bir **donanım uygunluk rozetiyle** işaretler: *"Cihazına uygun — GPU'da hızlı"*, *"CPU'da çalışır"* ya da *"Çok büyük."*

Sade kalite etiketleri quant kodlarının yerini alır — `Q4_K_M` yerine *"Dengeli kalite"*. Gerçek şirket logoları, yetenek filtreleri (Araç / Görüntü / Kod) ve GPU offloading'i otomatik yapan tek tıklık Başlat. Artık tahmin yok.

</td>
<td align="center" width="38%">
  <img src="docs/assets/screen/models-discorver-sc.png" alt="Model Mağazası" width="100%"/>
  <br/><br/>
  <img src="docs/assets/screen/models-my-sc.png" alt="Modellerim" width="100%"/>
</td>
</tr>
</table>

---

### 🎵 Orkestra — Bir Model Ekibi

Bir **Şef** model karmaşık görevi parçalara böler, 8 uzman role dağıtır ve sonucu sentezler. Sağlayıcıları özgürce karıştır — mantık için Claude, hız için Gemini, kod için yerel llama.cpp. Bağımsız roller **paralel** koşar. Ajan ile birleştir: Şef planlar, Ajan adım adım uygular.

| Planlayıcı | Ön Yüz | Arka Yüz | Hata Düzeltici | Gözden Geçirici | Güvenlik | DevOps | Genel Uzman |
|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|

---

### 🔌 8 Sağlayıcı · 🎤 Ses · ☁️ Bulut Senk. · 🔒 Gizlilik

- **8 Sağlayıcı, Tek Arayüz** — OpenAI, Claude, Gemini, Grok, Groq, OpenRouter, Ollama ve gömülü `llama.cpp`. Hatada otomatik yedek, 3 hatada otomatik devre dışı, sohbet ortasında `/model` ile canlı geçiş. Anahtarlar AES-256-GCM ile şifreli.
- **🎤 Sesli Giriş** — cihaz üzerinde whisper.cpp. Bas, konuş, bırak. TR/EN otomatik algılar. Ses makineni terk etmez.
- **☁️ Bulut Senkronizasyonu** — isteğe bağlı uçtan uca şifreli Google Drive yedeği. AES-256-GCM + PBKDF2 (600K iterasyon). Yüklemeden *önce* şifrelenir — Google okuyamaz.
- **🔒 Tasarımı Gereği Gizli** — telemetri yok, analitik yok, çökme raporu yok. Yapılandırma dosyaları `0600`. Gizli mod sıfır iz bırakır. Gözlemci yalnız aktivite zaman damgalarını saklar, mesaj içeriğini değil.

---

## 🚀 Hızlı Başlangıç

**Terminal gerekmez. Derleme gerekmez. `llama.cpp` ve motor binary'leri gömülü.**

### Tek komutla kurulum

| Platform | Komut |
|----------|-------|
| **Linux / macOS** | `curl -fsSL https://download.bugradev.com/get-memo.sh \| bash` |
| **Windows** | `irm https://download.bugradev.com/get-memo.ps1 \| iex` |

<details>
<summary><b>📦 Kurulum ne yapıyor?</b></summary>
<br/>

- CLI'ı (`memo`) PATH'e ekler — herhangi bir terminalden çalıştırabilirsin
- Flutter masaüstü uygulamasını kurar — uygulama menüsünde **Memo** yazar
- Motor binary'lerini (`llama-server`, `vec0`) kopyalar — GPU'ya hazır
- Varsayılan config'leri oluşturur — mevcut ayarlarının üzerine asla yazmaz
- Tekrar çalıştırması güvenli — sadece binary'ler yenilenir

</details>

### Güncelleme / Kaldırma

```bash
# Kurulum script'ini tekrar çalıştır — mevcut kurulumu algılar, günceller
curl -fsSL https://download.bugradev.com/get-memo.sh | bash
# Ya da sadece güncelleyiciyi kullan
curl -fsSL https://download.bugradev.com/update.sh | bash
# Kaldır (istersen hafızanı yedekler)
curl -fsSL https://download.bugradev.com/uninstall.sh | bash
```

### Alternatif: manuel indirme

Son sürümü **[memo.bugradev.com](https://memo.bugradev.com)** adresinden indirip çalıştır:

| Platform | Nasıl |
|----------|-------|
| **Linux** | `tar xzf memo.tar.gz -d Memo && cd Memo && ./run_memo.sh` |
| **Windows** | `Memo-Setup.exe` çalıştır |
| **macOS** | `unzip memo-mac.zip -d Memo && cd Memo && ./run_memo.sh` |

**Her push'ta CI build'leri:**  
[![Build Linux](https://img.shields.io/badge/Build-Linux-B08D57?style=flat-square)](https://github.com/BugraAkdemir/memo/actions/workflows/build-linux.yml)
[![Build Windows](https://img.shields.io/badge/Build-Windows-B08D57?style=flat-square)](https://github.com/BugraAkdemir/memo/actions/workflows/build-windows.yml)
[![Build macOS](https://img.shields.io/badge/Build-macOS-B08D57?style=flat-square)](https://github.com/BugraAkdemir/memo/actions/workflows/build-macos.yml)

→ **Actions sekmesi** → workflow seç → **Artifact** indir.

<div align="center">
  <br/>
  <a href="https://memo.bugradev.com"><img src="https://img.shields.io/badge/⬇_Memo'yu_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/></a>
</div>

<details>
<summary><b>🛠 Geliştiriciler için — kaynaktan derleme</b></summary>
<br/>

**Gereksinimler:** Go 1.26+ · Flutter 3.10+ · SQLite geliştirme kütüphaneleri (CGO için)

```bash
git clone https://github.com/BugraAkdemir/memo.git
cd memo

# Terminal 1 — backend
CGO_ENABLED=1 go run . --port 8090

# Terminal 2 — frontend
cd frontend && flutter run -d linux
```

Sürüm paketleri:
```bash
./build_releases.sh     # Linux  → AppImage / deb / tar.gz
.\build_releases.bat    # Windows → Inno Setup kurulumu / zip
```
</details>

---

## 🏗 Mimari ve Teknoloji Yığını

İki ayrık süreç, `localhost:8090` üzerinden düz HTTP/SSE ile haberleşir. TLS yok (yalnız yerel), harici router yok — saf `net/http` ServeMux. Frontend; Riverpod, Dio SSE akışı ve flutter_markdown'lı tek sayfalık bir Flutter uygulamasıdır.

<details>
<summary><b>📐 Tam mimari diyagramını gör</b></summary>
<br/>

```
┌─────────────────────────────────┐    ┌──────────────────────────┐
│  Flutter Masaüstü (Linux/Win)    │    │  Flutter Mobil            │
│  Sohbet · Ajan · Orkestra        │    │  Sohbet · Bildirimler     │
│  Ayarlar · Model Mağazası        │    │  Uzak bağlantı            │
└──────────────┬───────────────────┘    └───────────┬──────────────┘
               │  REST + SSE (:8090)                 │  LAN / ngrok
               └──────────────┬──────────────────────┘
┌──────────────────────────────┴──────────────────────────────────┐
│               Go Backend — 25 paket, ~90 endpoint                │
│  ┌─────────┐ ┌──────┐ ┌──────┐ ┌────────┐ ┌──────┐ ┌────────┐  │
│  │ Hafıza  │ │Oturum│ │Llama │ │WhatsApp│ │Ajan  │ │Sağlayıc│  │
│  │ vec0    │ │JSON  │ │GPU   │ │whatsmeow│ │Pipe  │ │Router  │  │
│  └─────────┘ └──────┘ └──────┘ └────────┘ └──────┘ └────────┘  │
│  Orkestra · ModelMağaza · BulutSenk · Takvim · DuyguMotoru       │
│  ngrok · Tailscale · Whisper · Skill · Niyet · Gözlemci          │
└──────────────────────────────────────────────────────────────────┘
```
</details>

| | | | |
|---|---|---|---|
| **Backend** Go 1.26 | **Frontend** Flutter 3.10 | **Vektör VT** SQLite + vec0 | **Çıkarım** llama.cpp |
| **State** Riverpod 2.4 | **HTTP** Dio 5.4 / SSE | **Ses** whisper.cpp | **WhatsApp** whatsmeow |
| **Bulut** Drive + AES-256 | **GPU** nvidia/rocm/sysfs | **Lisans** AGPL v3 | **CI** GitHub Actions |

📚 **Derin dalış:** [Mimari](docs/architecture.md) · [API Referansı](docs/API_REFERENCE.md) · [Tasarım Sistemi](frontend/DESIGN.md) · [Yol Haritası](docs/ROADMAP.md) · [Değişiklik Günlüğü](versinNote/tr/v3.1.1.md)

---

## 🤝 Katkıda Bulunma

Memo **AGPL-3.0** lisanslıdır ve katkılar memnuniyetle karşılanır.

- 🛣️ Planlananlar için [Yol Haritası](docs/ROADMAP.md)'na göz at
- 🐛 İyi bir başlangıç için bir [Bilinen Sorun](docs/KNOWN_ISSUES.md) seç
- 💡 Bir fikri [Tartışmalar](https://github.com/BugraAkdemir/memo/discussions)'da paylaş

---

<div align="center">
  <br/>
  <h3>Senin zihnin. Senin verin. Senin makinen.</h3>
  <p><a href="https://github.com/BugraAkdemir">Buğra Akdemir</a> tarafından tutkuyla geliştirildi</p>
  <br/>
  <a href="https://memo.bugradev.com"><img src="https://img.shields.io/badge/⬇_İndir-B08D57?style=for-the-badge" alt="İndir"/></a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers"><img src="https://img.shields.io/badge/⭐_Bu_repoyu_yıldızla-0a0a0a?style=for-the-badge" alt="Yıldız"/></a>
  <br/><br/>
  <sub><a href="https://github.com/BugraAkdemir/memo/issues">Hata Bildir</a> · <a href="https://github.com/BugraAkdemir/memo/discussions">Tartışma</a> · <a href="README.md">English</a></sub>
</div>
