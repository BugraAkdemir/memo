<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="160"/>

  <h1>Memo</h1>

  <h3>Alışkanlıklarını öğrenen ve sen sormadan<br>harekete geçen yapay zeka asistanı.</h3>

  <p>
    <sub>Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı</sub>
  </p>

  <br/>

  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_İndir-memo.bugradev.com-B08D57?style=for-the-badge&logoColor=white" alt="İndir"/>
  </a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers">
    <img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57&logo=github&logoColor=white" alt="Yıldız"/>
  </a>
  &nbsp;
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-0a0a0a?style=for-the-badge" alt="Lisans"/>
  &nbsp;
  <img src="https://img.shields.io/badge/Sürüm-v3.1.0_beta-B08D57?style=for-the-badge" alt="Sürüm"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-gömülü-F08705?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_+_vec0-0F9D58?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-6e6e6e?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/CI-geçiyor-0F9D58?style=flat-square&logo=githubactions" alt="CI"/>

</div>

---

## Memo Neden Farklı

Çoğu yapay zeka asistanı bir modele bağlı basit bir metin kutusundan ibarettir. Sen yazarsın, o yanıtlar. Sohbet biter. Hiçbir şey kalıcı olmaz, hiçbir şey öğrenilmez, hiçbir şey harekete geçmez.

Memo bu kalıbı kırar.

<table>
<tr>
<td align="center" width="33%">
  <h3>🧠 Hatırlar</h3>
  <p>Her konuşma yerel bir vektör veritabanına işlenir. Haftalar sonra bile Memo ne konuştuğunuzu hatırlar — bulut gerekmez.</p>
  <sub><a href="#-rag-hafıza">Detay →</a></sub>
</td>
<td align="center" width="33%">
  <h3>📊 Öğrenir</h3>
  <p>Arka plandaki gözlemci aktivite ritimlerini tespit eder. Bir hafta sonra Memo ne zaman kod yazdığını, planlama yaptığını bilir — ve öngörmeye başlar.</p>
  <sub><a href="#-proaktif-öğrenme-motoru">Detay →</a></sub>
</td>
<td align="center" width="33%">
  <h3>⚡ Harekete Geçer</h3>
  <p>8 gerçek araçlı ajan motoru: dosya oku/yaz, komut çalıştır, web'de ara, WhatsApp'tan mesaj gönder. Hepsi sandbox içinde, hepsi izin kontrollü.</p>
  <sub><a href="#-ajan-motoru">Detay →</a></sub>
</td>
</tr>
</table>

> **Bu bir chatbot değil. İzleyen, hatırlayan, öğrenen ve harekete geçen bir asistan — hepsi senin donanımında, sıfır telemetri ile.**

---

## Özellikler

### 💬 Sohbet

<table>
<tr>
<td width="65%">

Token-token akan yanıtlar. Tam Markdown desteği — sözdizimi vurgulu kod blokları, tablolar, görseller. Dosya (metin, PDF, kod) ve görsel eki (görüntü destekli modeller görebilir). Gizli mod sıfır iz bırakır. Web arama düğmesi her yanıtı canlı DuckDuckGo sonuçlarıyla zenginleştirir. WhatsApp modu aynı arayüzden WhatsApp hesabınla sohbet etmeni sağlar.

`/` yazınca slash-komut paleti. Mikrofona basılı tutarak cihaz üzerinde whisper.cpp ile sesli giriş. Mesajları düzenle veya sil — bağlam buna göre güncellenir. Tek tıkla Markdown olarak dışa aktar.

</td>
<td align="center" width="35%">
  <img src="docs/assets/screen/chatscreen.png" alt="Sohbet Ekranı" width="100%"/>
</td>
</tr>
</table>

---

### 🧠 RAG Hafıza

<table>
<tr>
<td width="65%">

Her kullanıcı+asistan mesajlaşması, yerel bir embedding modeliyle 768 boyutlu vektöre dönüştürülür ve sqlite-vec ANN indekslemeli SQLite'da saklanır. Yeni bir soru sorduğunda, Memo anlamsal olarak en yakın geçmiş konuşmaları getirir ve sistem prompt'una enjekte eder — böylece modelin bağlamı zaten hazırdır.

Pinecone yok. Embedding API'si yok. Bulut vektör veritabanı yok. Sadece SQLite ve senin GPU'n.

</td>
<td align="center" width="35%">
  <img src="docs/assets/screen/setting-sc.png" alt="Hafıza Ayarları" width="100%"/>
</td>
</tr>
</table>

---

### 🤖 Ajan Motoru

<table>
<tr>
<td width="60%">

Ajan <em>konuşmaktan</em> <em>yapmaya</em> geçiştir. Bir proje klasörü seç, ajan dosyaları okusun, yazsın, düzenlesin, silsin; dizinleri listelesin; shell komutu çalıştırsın; web'de arama yapsın. 8 yerleşik araç; yol doğrulaması, symlink koruması ve komut kara listesi olan bir sandbox içinde çalışır.

Her araç çağrısı izin diyaloğu tetikler — bir kereliğine, oturum boyunca veya kalıcı olarak izin ver veya reddet. Pipeline başına 20 iterasyon, araç başına 60sn zaman aşımı. İstediğin an iptal et.

<details>
<summary>🎬 <b>Demoyu izle</b></summary>
<br/>
<video src="docs/assets/videos/agent.mp4" controls width="100%"></video>
</details>

</td>
<td align="center" width="40%">
  <img src="docs/assets/screen/agent-sc.png" alt="Ajan Ekranı" width="100%"/>
</td>
</tr>
</table>

---

### 🎵 Orkestra — Çoklu Model

<table>
<tr>
<td width="65%">

Bir Şef model karmaşık görevleri analiz eder, 8 uzman role dağıtır (Planlayıcı, Ön Yüz, Arka Yüz, Hata Düzeltici, Gözden Geçirici, Güvenlik Denetçisi, DevOps, Genel Uzman) ve sonuçları sentezler. Her role farklı sağlayıcı ve model ata — mantık için Claude, hız için Gemini, kod üretimi için yerel llama.cpp.

Bağımsız roller paralel çalışır. Orkestra + Ajan = Şef planlar, Ajan adım adım uygular.

</td>
<td align="center" width="35%">

| Rol | En İyisi |
|-----|----------|
| Planlayıcı | Yapılandırılmış plan |
| Ön Yüz | UI, stil |
| Arka Yüz | API, veritabanı |
| Gözden Geçirici | Kod kalitesi |
| Güvenlik | Açık taraması |
| DevOps | CI/CD, Docker |
| Hata Düzeltici | Hata ayıklama |
| Genel Uzman | Diğer her şey |

</td>
</tr>
</table>

---

### 📱 WhatsApp Entegrasyonu

<table>
<tr>
<td width="60%">

QR kod ile bağlan — WhatsApp Web ile aynı protokol. Business API ücreti yok. Eşleştikten sonra: mesajları oku, geçmişte ara, yanıtla — hepsi Memo içinden. Profil fotoğrafları önbelleklenir, arayüz Memo'nun bronz temasına uyar.

Ajan WhatsApp'tan mesaj gönderebilir, konuşmalarda arama yapabilir, kişi adlarını çözümleyebilir — "Berra'ya mesaj at" de, JID bilmene gerek yok. WhatsApp mesajları RAG hafızaya, proaktif gözlemciye ve takvim niyet çıkarıcısına beslenir.

<details>
<summary>🎬 <b>Demoyu izle</b></summary>
<br/>
<video src="docs/assets/videos/whats-my-name.mp4" controls width="100%"></video>
</details>

</td>
<td align="center" width="40%">
  <img src="docs/assets/screen/whatsaap-qr-sc.png" alt="WhatsApp QR" width="100%"/>
</td>
</tr>
</table>

---

### 📅 Akıllı Takvim

<table>
<tr>
<td width="60%">

Elle doldurduğun bir takvim değil — otomatik yakalama sistemi. İki aşamalı niyet pipeline'ı: hızlı anahtar kelime filtresi her mesajı zaman kalıpları için tarar ("yarın", "salı", "saat 3'te"), sadece eşleşen mesajlar yapılandırılmış etkinlik çıkarımı için LLM'e gönderilir.

Masaüstü ve mobilde hatırlatmalar. Emin olunamayan etkinlikler ("belki yarın?") → belirsizlik etkinliği, tek tıkla onayla/sil. Aynı hatırlatmanın iki kez tetiklenmesini atomik SQL transaction'ı engeller.

<details>
<summary>🎬 <b>Demoyu izle</b></summary>
<br/>
<video src="docs/assets/videos/calendar.mp4" controls width="100%"></video>
</details>

</td>
<td align="center" width="40%">
  <img src="docs/assets/screen/clander-dc.png" alt="Takvim" width="100%"/>
</td>
</tr>
</table>

---

### 🧠 Proaktif Öğrenme Motoru

Memo <em>sana gelen</em> tek yapay zeka asistanıdır. Gözlemci <strong>ne zaman</strong> aktif olduğunu izler — ne söylediğini değil. Dairesel (yönlü) istatistik ritimleri tespit eder: "Pazartesi sabahları 9-10, planlama" veya "Her gün 21-23, kodlama."

Güven eşiği aşıldığında, proaktif motor bir LLM'e danışır ve karar verir: yararlı bir şey öner, bildirim gönder veya (yüksek güvende) ajanı otomatik başlat. İsteğe bağlı, şeffaf ve zayıflayan pattern'ler unutulur. Sadece konu etiketleri ve kelime sayıları saklanır — asla ham mesaj metni değil.

<details>
<summary>🎬 <b>Demoyu izle</b></summary>
<br/>
<video src="docs/assets/videos/moods-and-ozcıkar.mp4" controls width="100%"></video>
</details>

---

### 🏪 Model Mağazası

<table>
<tr>
<td width="65%">

Gerçek RAM ve VRAM'ine göre hesaplanmış <strong>donanım uygunluk rozetli</strong> küratörlü modeller — statik liste değil. "Cihazına uygun — GPU'da hızlı" ya da "CPU'da çalışır" ya da "Çok büyük."

Ham quantization kodları yerine sade dilde kalite etiketleri: "Dengeli kalite" (Q4_K_M), "Yüksek kalite" (Q5_K_M). HuggingFace'ten otomatik çekilen gerçek şirket logoları. Boyuta (1-8B, 8-14B, 14B+) ve yeteneğe (Araç, Görüntü, Kod) göre filtrele. Gerçek zamanlı ilerlemeyle indir, iptal edilebilir. Tek tıkla Başlat GPU offloading'i otomatik yapılandırır.

</td>
<td align="center" width="35%">
  <img src="docs/assets/screen/models-discorver-sc.png" alt="Model Mağazası Keşfet" width="100%"/>
  <br/>
  <img src="docs/assets/screen/models-my-sc.png" alt="Modellerim" width="100%"/>
</td>
</tr>
</table>

---

### 🔌 8 Sağlayıcı, Tek Arayüz

OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama ve herhangi bir yerel llama.cpp sunucusu.

**Production-grade sağlayıcı sistemi:** hatada otomatik yedekleme, 3 ardışık hatada otomatik devre dışı bırakma, 5 dakikada bir arka plan sağlık kontrolü, sohbet ortasında `/model` ile canlı sağlayıcı değiştirme. Model başına yapılandırılabilir bağlam penceresi. API anahtarları makinede üretilen rastgele anahtarla AES-256-GCM şifreli.

---

### 🎤 Sesli Giriş · ☁️ Bulut Senk. · 🔒 Gizlilik

- **Sesli Giriş** — cihaz üzerinde whisper.cpp ile konuşma-metne çevrimi. Basılı tut ve konuş. TR/EN otomatik algılama. Ses bilgisayarından asla çıkmaz.
- **Bulut Senkronizasyonu** — Google Drive'a E2E şifreli yedekleme. AES-256-GCM + PBKDF2 (600K iterasyon). Yüklemeden önce şifrelenir — Google verilerini okuyamaz.
- **Tasarımı Gereği Gizli** — telemetri yok, analitik yok, çökme raporlaması yok. Yapılandırma dosyaları 0600 izinli. Gizli mod. Gözlemci sadece aktivite zaman damgalarını saklar, mesaj içeriğini asla.

---

## Hızlı Başlangıç

**Terminal yok. Derleme yok. Tek tık.**

| Platform | İndirme | Nasıl |
|----------|---------|-------|
| **Windows** | `Memo-Setup.exe` | Kurulumu çalıştır |
| **Linux** | `.AppImage` | `chmod +x` → başlat |
| **Linux** | `.deb` | `sudo dpkg -i` |

llama.cpp gömülü gelir. Uygulamayı aç, **Model Mağazası**'na git, bir model seç, sohbete başla.

<div align="center">
  <br/>
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Memo'yu_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/>
  </a>
</div>

<details>
<summary><b>Kaynaktan derleme</b></summary>
<br/>

**Gereksinimler:** Go 1.26+ · Flutter 3.10+ · SQLite geliştirme kütüphaneleri (CGO için)

```bash
git clone https://github.com/BugraAkdemir/memo.git
cd memo
CGO_ENABLED=1 go run . --port 8090          # Backend
cd frontend && flutter run -d linux          # Frontend
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
┌─────────────────────────────────┐    ┌──────────────────────────┐
│  Flutter Masaüstü                │    │  Flutter Mobil            │
│  (Linux / Windows)               │    │  (Android / iOS)          │
│                                  │    │                           │
│  Sohbet · Ajan · Orkestra        │    │  Sohbet · Bildirimler     │
│  Ayarlar · Model Mağazası        │    │  Uzak bağlantı            │
└──────────────┬───────────────────┘    └───────────┬───────────────┘
               │  REST + SSE (:8090)                 │  LAN / ngrok
               └──────────────┬──────────────────────┘
┌──────────────────────────────┴──────────────────────────────────┐
│               Go Backend — 25 paket, ~90 endpoint                │
│                                                                  │
│  ┌─────────┐ ┌──────────┐ ┌────────────────┐                    │
│  │ HTTP    │ │ App      │ │ Proaktif       │                    │
│  │ ServeMux│ │ Motoru   │ │ Gözlemci→Eylem │                    │
│  └─────────┘ └────┬─────┘ └────────────────┘                    │
│                    │                                             │
│  ┌────────┐ ┌──────┐ ┌──────┐ ┌────────┐ ┌──────┐ ┌────────┐  │
│  │ Hafıza │ │Oturum│ │Llama │ │WhatsApp│ │Ajan  │ │Sağlayıc│  │
│  │ vec0   │ │JSON  │ │GPU   │ │whatsmeow│ │Pipe  │ │Router  │  │
│  └────────┘ └──────┘ └──────┘ └────────┘ └──────┘ └────────┘  │
│                                                                  │
│  Orkestra · ModelMağazası · BulutSenk · Takvim · DuyguMotoru    │
│  ngrok · Tailscale · Whisper · Skill · Niyet · Gözlemci         │
└──────────────────────────────────────────────────────────────────┘
```

İki süreç `localhost:8090` üzerinden düz HTTP ile haberleşir. TLS yok (yerel). Harici router yok — saf `net/http` ServeMux.

**Belgeler:** [Mimari](docs/architecture.md) · [API Referansı](docs/API_REFERENCE.md) · [Tasarım Sistemi](frontend/DESIGN.md)

---

## Teknoloji Yığını

<table>
<tr>
<th>Katman</th><th>Teknoloji</th><th>Notlar</th>
</tr>
<tr>
<td>Backend</td>
<td><img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt=""/></td>
<td>SQLite için CGO gerekli</td>
</tr>
<tr>
<td>HTTP</td>
<td><code>net/http</code> ServeMux</td>
<td>Harici router bağımlılığı yok</td>
</tr>
<tr>
<td>Akış</td>
<td>SSE (Server-Sent Events)</td>
<td>Token-token sohbet akışı</td>
</tr>
<tr>
<td>Masaüstü Frontend</td>
<td><img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt=""/></td>
<td>Linux + Windows</td>
</tr>
<tr>
<td>State</td>
<td>Riverpod 2.4</td>
<td>AsyncNotifierProvider</td>
</tr>
<tr>
<td>HTTP İstemcisi</td>
<td>Dio 5.4</td>
<td>SSE ayrıştırma, interceptor'lar</td>
</tr>
<tr>
<td>Markdown</td>
<td>flutter_markdown 0.6</td>
<td>Kod blokları, tablolar, görseller</td>
</tr>
<tr>
<td>LLM Çalışma Zamanı</td>
<td>llama.cpp</td>
<td>Gömülü binary, alt süreç yönetimi</td>
</tr>
<tr>
<td>Vektör VT</td>
<td>SQLite + sqlite-vec</td>
<td>vec0 ANN, 768 boyut embedding</td>
</tr>
<tr>
<td>WhatsApp</td>
<td>whatsmeow</td>
<td>Çoklu cihaz Web API</td>
</tr>
<tr>
<td>Ses-Metne</td>
<td>whisper.cpp</td>
<td>Cihaz üzerinde, gömülü binary</td>
</tr>
<tr>
<td>Bulut Senk.</td>
<td>Google Drive API v3</td>
<td>AES-256-GCM + PBKDF2</td>
</tr>
<tr>
<td>GPU Tespiti</td>
<td>nvidia-smi, rocm-smi, sysfs</td>
<td>Otomatik VRAM ölçümü</td>
</tr>
<tr>
<td>Loglama</td>
<td><code>internal/logx</code></td>
<td>slog wrapper</td>
</tr>
<tr>
<td>CI/CD</td>
<td><img src="https://img.shields.io/badge/GitHub_Actions-geçiyor-0F9D58?style=flat-square&logo=githubactions" alt=""/></td>
<td>Go vet+test+build, Flutter analyze+test</td>
</tr>
<tr>
<td>Lisans</td>
<td>GNU AGPL v3</td>
<td>Özgür yazılım</td>
</tr>
</table>

---

## Belgeler

| | |
|-|-|
| [Mimari](docs/architecture.md) | Paket haritası, veri akışı, modül sınırları |
| [API Referansı](docs/API_REFERENCE.md) | 90+ REST endpoint'i istek/yanıt şemalarıyla |
| [Tasarım Sistemi](frontend/DESIGN.md) | "Pewter Study" tema token'ları, renk paleti, tipografi |
| [Yol Haritası](docs/ROADMAP.md) | Sürümlü yayın planı |
| [Mobil](mobile/README.md) | Flutter mobil yardımcı kurulumu ve tünel yapılandırması |
| [Sorun Giderme](docs/TROUBLESHOOTING.md) | GPU kurulumu, port çakışmaları, sık hatalar |
| [Katkı](docs/CONTRIBUTING.md) | Geliştirici kurulumu, kod stili, PR süreci |
| [Değişiklik Günlüğü](versinNote/tr/v3.1.0.md) | Tam v3.1.0 özellik listesi, hata düzeltmeleri |

---

## Katkıda Bulunma

Memo AGPL-3.0 lisanslıdır. Katkılar memnuniyetle karşılanır.

- [Yol Haritası](docs/ROADMAP.md)'nı incele — planlanan özellikler
- [Bilinen Sorunlar](docs/KNOWN_ISSUES.md)'a göz at — iyi başlangıç görevleri
- Fikirler için [Tartışma](https://github.com/BugraAkdemir/memo/discussions) aç

---

<br/>

<div align="center">
  <p><b>Senin zihnin. Senin verin. Senin makinen.</b></p>
  <p><a href="https://github.com/BugraAkdemir">Buğra Akdemir</a> tarafından geliştirildi</p>
  <br/>
  <a href="https://github.com/BugraAkdemir/memo/issues">Hata Bildir</a> ·
  <a href="https://github.com/BugraAkdemir/memo/discussions">Tartışma</a> ·
  <a href="README.md">English</a>
</div>
