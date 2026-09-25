# 📱 Telefonda Memo — Adım Adım Rehber

> **2026-09'da değişti:** artık ayrı bir mobil uygulama yok. Bağımsız
> `mobile/` projesi emekliye ayrıldı ve masaüstüyle **aynı** Flutter istemcisi
> (`frontend/`) Android ve iOS hedeflerine sahip oldu. Yani telefonda gerçek
> ekranlar var — sohbet, ajan modu, takvim, rutinler, model deposu, ayarlar —
> dar ekran için yerleştirilmiş halde. Aşağıdaki her şey o istemciyi anlatıyor.

> **Ne bu?** Telefonundan Memo'ya bağlan. Yaz, sohbet et, takvime bak. Tüm AI işlemleri masaüstünde çalışır — telefon sadece bir "uzaktan kumanda" gibidir. Telefonun ısınmaz, şarjı bitmez, internet kotaları şişmez.

---

## 🤔 Bu Ne İşe Yarar?

Şöyle düşün: Bilgisayarın odanda, Memo masaüstünde çalışıyor. Sen mutfaktasın, canın bir şey sormak istedi. Bilgisayara gitmeden, telefonundan yazıyorsun. Memo bilgisayarında cevabı hazırlıyor, telefonuna gönderiyor.

Ya da dışarıdasın. ngrok/Tailscale ile Memo'ya uzaktan bağlanıyorsun. Takvimine bakıyorsun, bir etkinlik ekliyorsun, hatırlatma alıyorsun.

Telefon sadece yazdığını ve aldığın cevabı gösterir. Tüm ağır iş — LLM çıkarımı, RAG araması, embedding — masasüstünde olur.

---

## 📱 Kurulum — Adım Adım

### Ön Koşullar

- Memo masaüstü uygulaması **çalışıyor olmalı**
- Telefon ve bilgisayar **aynı Wi-Fi ağında** olmalı (LAN bağlantısı için)
- Veya bilgisayarda ngrok/Tailscale **aktif** olmalı (uzaktan bağlantı için)

### 1. Masaüstünde Hazırlık

Memo masaüstünde hiçbir ek ayar yapmana gerek yok. Sadece çalışıyor olsun. IP adresini öğrenmek için:

```bash
# Linux
ip addr show | grep "inet " | grep -v 127

# Windows
ipconfig
```

Örnek çıktı: `192.168.1.42` — bu senin bilgisayarının IP adresi.

Eğer uzaktan bağlanacaksan, Ayarlar → Uzaktan Erişim'den ngrok veya Tailscale'i aç.

### 2. Telefon İçin Derle ve Çalıştır

```bash
cd frontend
flutter run          # telefon USB ile bağlı, geliştirici modu açık
# ya da kurulabilir bir debug APK üretmek için:
flutter build apk --debug
```

CI de her push'ta iki hedefi derliyor (`build-android.yml` debug APK artifact'ı
üretiyor; `build-ios.yml` iOS'u imzasız derliyor — bu projenin imzalama
sertifikası yok).

### 3. İlk Açılış — Sunucu Adresi

Telefonda setup wizard ilk adım olarak **Sunucuya Bağlan**'ı gösteriyor, çünkü
bir telefon Memo'nun backend'ini kendisi çalıştırmaz, dolayısıyla varsayılan
alacak bir adres yok:

| Alan | Ne Yazmalısın |
|------|--------------|
| **Sunucu adresi** | Bilgisayarının IP'si + port: `192.168.1.42:8090` (şemayı senin için tamamlıyor) |
| **Erişim anahtarı** | Ayarlar → Uzaktan Erişim'de bir token belirlediysen o |

**Bağlantıyı test et**'e bas — yeşil satır Memo'nun cevap verdiği anlamına
geliyor; wizard'ın kalanı (karakter, model, tercihler) o zaman normal çalışıyor.

Tailscale adresi doğrudan yazılabilir: `*.ts.net` biten bir host tanınıyor ve
`https://` ile port eklenmeden kullanılıyor — Funnel standart 443 üzerinden
hizmet veriyor.

**LAN otomatik keşif yok.** Emekliye ayrılan istemcide alt ağdaki her adresi
yoklayan bir "Tara" butonu vardı; taşınmadı. Adresi elle yaz ya da değişmeyen
bir Tailscale adı kullan.

---

## 🏠 Aynı Wi-Fi'de Bağlanma (LAN)

En basit yöntem. Telefon ve bilgisayar aynı ağda olsun yeter.

```
Telefon ←──── Wi-Fi ────→ Bilgisayar (Memo çalışıyor)
   ↓                          ↓
 192.168.1.100            192.168.1.42:8090
```

1. Bilgisayarın IP'sini öğren (yukarıdaki komutla)
2. Mobil uygulamada bu IP'yi ve `:8090` portunu gir
3. Bağlan

> Bu yöntem **sadece aynı ev/ofis ağında çalışır.** Dışarıdan bağlanamazsın.

---

## 🌍 Dışarıdan Bağlanma (ngrok / Tailscale)

Evde değilsin, Memo'ya ulaşmak istiyorsun.

### Seçenek 1: ngrok (en kolay)

1. [ngrok.com](https://ngrok.com)'a git, ücretsiz hesap aç
2. Auth token'ını kopyala
3. Memo'da **Ayarlar → Uzaktan Erişim → Ngrok**
4. Token'ı yapıştır, **Ngrok Aktif**'i aç
5. Memo sana bir URL verecek: `https://abc123.ngrok.io`
6. Bu URL'yi mobil uygulamaya gir

```
Telefon ←──── İnternet ────→ ngrok sunucusu ────→ Senin bilgisayarın
```

> Ücretsiz ngrok'ta URL her başlatmada değişir. Her seferinde yeni URL'yi mobil uygulamaya girmen gerekir.

### Seçenek 2: Tailscale (kararlı URL)

> **v3.3.4 (geliştirme aşamasında):** Tailscale artık Beta özelliği değil, doğrudan Uzaktan Erişim'in içinde. Bağlanmak artık key yapıştırmayı gerektirmiyor — tek tıkla, key gerektirmeyen interaktif bir giriş akışı var. Ayrıca kopan bir bağlantıdan sonra otomatik yeniden bağlanıyor ve mobil uygulama soğuk başlangıçta kayıtlı URL ile otomatik yeniden bağlanıyor.

1. [Tailscale](https://tailscale.com)'e üye ol
2. Memo'da **Ayarlar → Uzaktan Erişim → Tailscale**
3. Tek tıkla giriş yap (auth key elle yapıştırmak artık gerekmiyor)
4. Hostname belirle (örn. `memo-ev`)
5. Telefonuna da Tailscale uygulamasını kur
6. Mobil uygulamada adresi `http://memo-ev:8090` olarak gir

```
Telefon ←── Tailscale ağı ──→ Bilgisayar
(Tailscale app)              (Memo + gömülü Tailscale)
```

> Tailscale ile URL **hep aynı kalır.** Bir kere kur, hep aynı adresle bağlan.

---

## 🎯 Takvim Sekmesi

Takvim sekmesi:

- **Aylık görünüm** gösterir — etkinlik olan günlerde nokta var
- Güne dokununca o günün etkinliklerini listeler
- Yeni etkinlik ekleyebilirsin (manuel)
- Etkinliğe uzun basınca silebilirsin
- Hatırlatma süresini değiştirebilirsin

---

## 🔔 Bildirimler

**Takvim hatırlatmaları gerçek bildirim olarak geliyor** — bir bağlantı
üzerinden push edilmiyor, doğrudan işletim sistemine zamanlanıyor. Yani Memo
kapalıyken ve telefon o günden beri backend'i hiç duymamışken bile ateşleniyor.
Takvim her yüklendiğinde ve uygulama ön plana döndüğünde yeniden kuruluyor.
İzin, ilk açılışta sessizce değil, setup wizard'ın tercihler adımından isteniyor.

Bilinen bir boşluk: takvim sekmesini hiç açmadığın sırada asistanın eklediği bir
etkinlik, sen sekmeyi açana kadar zamanlanmıyor (bkz. KNOWN_ISSUES M32).

**Rutin bildirimleri yok** — o yol, yokladığı `/api/routines/mobile-ready`
endpoint'iyle birlikte v3.9.0'da kaldırıldı. Rutinler artık **WhatsApp ya da
Telegram kendine-sohbet** üzerinden oluşturuluyor, yönetiliyor ve teslim
ediliyor (bkz. [[WhatsApp Entegrasyonu]]).

## 🌍 Yerelleştirme

Tam TR/EN iki dilli — masaüstü build'inin kullandığı `l10n.dart`'ın aynısı,
dolayısıyla ikisi birbirinden ayrışamıyor.

## 🚫 Telefonda Olmayanlar

- **Live Mode'un native realtime motorları** (Google Live, OpenAI Realtime) —
  akış halindeki ses çalma yolları sadece Linux'ta var. Sesli düğme, çalışan
  ayrık kaydet → yazıya çevir → yanıtla → seslendir döngüsüne düşüyor.
- **Masaüstü maskotu** ve **sistem tepsisi** — telefonda ikisi de yok.
- **CLI kurulumu** — kabuk yok, kurulacak bir home dizini yok.

Masaüstüne özelmiş gibi görünüp aslında sorunsuz çalışanlar (hepsi sunucu
taraflı REST çağrısı): model deposu (GGUF'u sunucu kendi diskine indiriyor),
GPU bilgisi (sunucunun GPU'sunu bildiriyor) ve llama.cpp kurulumu.

---

## 🔐 Token Koruması

İstersen bağlantıya şifre koyabilirsin:

1. Masaüstü Memo → **Ayarlar → Uzaktan Erişim**
2. **Access Token** alanına bir şifre yaz
3. Mobil uygulamada bağlanırken aynı token'ı gir

Token yoksa bağlantı reddedilir. Özellikle ngrok ile dışarı açtıysan bunu yapman şiddetle önerilir.

---

## ❓ Sık Sorulanlar

**S: İnternet yokken çalışır mı?**
LAN bağlantısı internet istemez. Aynı Wi-Fi'de olman yeter. İnternet sadece ngrok/Tailscale ile uzaktan bağlantı için gerekir.

**S: Telefonumda model çalıştırmam gerekir mi?**
Hayır! Tüm AI işlemleri masaüstünde olur. Telefon sadece metni gönderir ve cevabı gösterir. Telefonun eski, yavaş, pili bitik olabilir — fark etmez.

**S: Aynı anda hem masaüstünden hem telefondan sohbet edebilir miyim?**
Evet. Aynı oturuma iki yerden bağlanabilirsin. Ama aynı anda ikisinden de mesaj yazarsan karışabilir — sırayla kullan.

---

## Bağlantılı Notlar:
- [[Uzaktan Erişim]]
- [[Proaktif Öğrenme ve Takvim]]
- [[Mimari Yapı]]
