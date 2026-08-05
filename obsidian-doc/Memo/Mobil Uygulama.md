# 📱 Memo Mobil Uygulama — Adım Adım Rehber

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

### 2. Mobili Derle ve Çalıştır

```bash
cd mobile
flutter run
```

Telefonun USB ile bağlı ve geliştirici modu açık olmalı.

### 3. Bağlantı Ekranı

Uygulama açıldığında karşına bir bağlantı ekranı gelir:

| Alan | Ne Yazmalısın |
|------|--------------|
| **Sunucu Adresi** | Bilgisayarının IP'si + port: `http://192.168.1.42:8090` |
| **Token (isteğe bağlı)** | Ayarlar'da belirlediysen token'ı gir |

**LAN Otomatik Keşif:** IP adresini bilmiyorsan **Tara** butonuna bas. Telefon ağdaki tüm adresleri tarar, Memo'yu bulur, adresi otomatik doldurur.

4. **Bağlan**'a tıkla.

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

Mobil uygulamada bir takvim sekmesi var. Bu sekme:

- **Aylık görünüm** gösterir — etkinlik olan günlerde nokta var
- Güne dokununca o günün etkinliklerini listeler
- Yeni etkinlik ekleyebilirsin (manuel)
- Etkinliğe uzun basınca silebilirsin
- Hatırlatma süresini değiştirebilirsin

---

## 🔁 Rutinler Sekmesi (v3.3.3)

Routines masaüstü kadar mobilde de tam çalışır:

- Rutin hatırlatmaları **gerçek, önceden zamanlanmış yerel bildirimler** olarak gelir — uygulama o an açık olmasa bile ulaşır.
- Rutinler, oluşturuldukları cihazın saat diliminde tetiklenir; bu offset her (yeniden) bağlantıda otomatik güncellenir (seyahat/DST değişimi kendini düzeltir).
- Rutin bildirim başlıkları ve metinleri artık uygulama dilini (TR/EN) takip eder.

## 🌍 Tam Yerelleştirme (v3.3.3)

Mobil uygulama artık masaüstüyle aynı şekilde tam TR/EN iki dilli — dil değiştirici hem **Ayarlar**'da hem de **eşleştirme öncesi bağlantı ekranında** mevcut.

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
