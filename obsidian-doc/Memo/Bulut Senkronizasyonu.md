# ☁️ Bulut Senkronizasyonu — Adım Adım Rehber

> **Amaç:** Memo'daki tüm verilerini (hafıza, sohbetler, ayarlar) Google Drive'ına otomatik yedekle. Bilgisayarın çökse de, yeni bilgisayara geçsen de hiçbir şey kaybolmaz. Üstelik Google bile verilerini okuyamaz — her şey senin belirlediğin şifreyle kilitlenir.

---

## 🤔 Bu Ne İşe Yarar?

Şöyle düşün: Memo'yu 3 aydır kullanıyorsun. Seni tanıyor, ne zaman çalıştığını biliyor, haftalar önceki konuşmaları hatırlıyor. Sonra bilgisayarın bozuldu. Bütün o hafıza gitti. İşte bunu engellemek için bulut senkronizasyonu var.

Her 50 mesajda bir (ayarlanabilir), Memo sessizce arka planda şunları yapar:
1. Verilerini bir pakete dönüştürür
2. Paketi senin şifrenle kilitler
3. Google Drive'ına yükler

Sonuç: Google Drive'ında şifreli bir yedek var. Kimse açamaz, sadece sen açabilirsin.

---

## 🔐 Şifreleme — En Basit Anlatım

```
Senin verin → [ŞİFRENLE KİTLENİR] → Google Drive
                                        ↓
                                   Google bunu göremez.
                                   Sadece şifreyi bilen açabilir.
```

- **Şifren olmadan:** Google Drive'daki dosya anlamsız, karışık bir veri yığınıdır. Google bile açamaz.
- **Şifrenle:** Dosya çözülür, Memo içindeki verileri okur, her şey eski haline döner.

> ⚠️ **Şifreni unutursan geri dönüş YOKTUR.** Bu şifreyi bir yere not et. Şifreyi kaybedersen buluttaki yedeklerin sonsuza kadar kilitli kalır.

---

## 📦 Neler Yedeklenir?

| Veri | İçerik |
|------|--------|
| **Hafıza (memory.db)** | Tüm RAG anıları, seninle ilgili bildiği her şey |
| **Sohbetler (sessions/)** | Tüm konuşma geçmişin |
| **Provider ayarları** | API anahtarların (zaten şifreli) |
| **Orchestra yapılandırması** | Hangi model hangi rolde |
| **Ajan izinleri** | Hangi araçlara izin verdiğin |
| **Öğrenme kalıpları** | Ne zaman çalıştığın, alışkanlıkların |
| **Mood (ruh hali)** | Duygu motorunun durumu |

---

## 🚀 Kurulum — Adım Adım

### Adım 1: Google Cloud Projesi Oluştur

Memo'nun senin Drive'ına yazabilmesi için Google'a "ben buyum" demesi gerek. Bunun için küçük bir proje oluşturacaksın. **Ücretsiz, 5 dakika sürer.**

1. [Google Cloud Console](https://console.cloud.google.com)'a git
2. Yeni proje oluştur (adı önemli değil, "Memo Sync" yaz geç)
3. Sol menüden **APIs & Services → Library**'ye git
4. **Google Drive API**'yi ara, etkinleştir
5. **Credentials** (Kimlik Bilgileri) sayfasına git
6. **Create Credentials → OAuth Client ID** seç
7. Application Type: **Desktop App**
8. Bir isim ver (örn. "Memo Desktop"), **Create**'e bas
9. Karşına **Client ID** ve **Client Secret** çıkacak — ikisini de kopyala

> Bu iki kod (Client ID + Client Secret), Memo'nun "Google Drive'ına yazmak için izin istiyorum" demesini sağlar. Başkasına verme, ama çok da gizli değil — uygulamanın kimliği sadece.

### Adım 2: Memo'ya Bilgileri Gir

1. Memo'yu aç
2. **Ayarlar → Cloud Sync** sekmesine git
3. Bu bilgileri doldur:
   - **Client ID:** Adım 1'den kopyaladığın uzun kod
   - **Client Secret:** Adım 1'den kopyaladığın diğer uzun kod
   - **Passphrase (Şifre):** Kendi belirleyeceğin güçlü bir şifre
     - En az 8 karakter
     - Büyük harf, küçük harf, rakam içersin
     - **Bu şifreyi unutma!** Not et, bir yere yaz.
4. **Save**'e bas

### Adım 3: Google'a Giriş Yap

1. **Login with Google** butonuna tıkla
2. Tarayıcın açılacak, Google hesabını seç
3. "Memo şu dosyalara erişmek istiyor" diyecek — **Allow/İzin Ver**
4. Tarayıcıyı kapat, Memo'ya dön
5. "Bağlandı" yazısını göreceksin

### Adım 4: İlk Yedeği Al

1. **Sync Now** (Şimdi Senkronize Et) butonuna tıkla
2. Bekle — arka planda verilerin şifrelenip Google Drive'a yükleniyor
3. "Sync complete" yazısını gördüğünde işlem tamamdır
4. **Bundan sonrası otomatik** — her 50 mesajda bir kendi kendine yedekler

---

## 🔄 Başka Bilgisayara Geri Yükleme

Yeni bilgisayara geçtin. Memo'yu kurdun. Eski hafızanı geri getirmek için:

1. **Aynı Client ID ve Client Secret'ı gir** (Adım 2'deki gibi)
2. **Aynı şifreyi (Passphrase) gir** — kesinlikle aynı olacak, yoksa açılmaz
3. **Login with Google** yap
4. **Pull from Cloud** (Buluttan Çek) butonuna tıkla
5. Memo buluttaki en son yedeği indirir, şifrenle açar, verilerini geri yükler
6. Her şey eskisi gibi — hafızan, sohbetlerin, ayarların geri geldi

---

## ⚙️ Ayarlar

| Ayar | Ne İşe Yarar | Varsayılan |
|------|-------------|------------|
| **Interval (messages)** | Kaç mesajda bir yedeklensin | 50 |
| **Passphrase** | Şifreleme şifren — kaybedersen veriler gider | (boş = makine anahtarı) |

> Şifre boş bırakılırsa, Memo otomatik bir makine anahtarı kullanır. Bu, aynı bilgisayarda çalışır ama **başka bilgisayara taşıyamazsın.** Şifre belirlemen şiddetle önerilir.

---

## ❓ Sık Sorulanlar

**S: Google verilerimi görebilir mi?**
Hayır. Veriler bilgisayarından çıkmadan önce şifrelenir. Google sadece şifreli bir dosya görür, içeriğini asla okuyamaz.

**S: İnternet yokken ne olur?**
Hiçbir şey. Senkronizasyon sadece internet varken çalışır. İnternet yokken atlar, bağlanınca devam eder.

**S: Şifremi değiştirmek istersem?**
Ayarlardan yeni şifre yaz, kaydet. Sonraki yedekler yeni şifreyle şifrelenir. Eski yedekler eski şifreyle kalır — onları geri yüklemek için eski şifreyi hatırlaman gerekir.

**S: Kaç yedek saklanır?**
Son 3 yedek. Eskiler otomatik silinir. Bu, Drive kotanı doldurmamak için.

---

## Bağlantılı Notlar:
- [[Veri Katmanı ve Kalıcılık]]
- [[Mimari Yapı]]
