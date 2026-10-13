# Web sohbet (ChatGPT / claude.ai) — Beta

ChatGPT veya Claude'un **web** sohbetini, API yerine kendi hesabınla Memo'dan kullan. Memo siteyi kendi profil
klasörüyle (`data/webchat/<site>`) gerçek bir tarayıcı penceresinde açar, girişi **sen** yaparsın; Memo mesajı
sayfaya yazar ve cevabı markdown olarak geri okur. Harcanan kota web sohbetinin kotasıdır; API'nin ya da
Codex'in değil.

> **Önce bunu oku.** Sitelerin kullanım şartları web uygulamasının otomatik kullanımını kısıtlar. Hesabın
> sınırlanabilir veya kapatılabilir. Memo girişi ya da insan doğrulamasını aşmaz (onları pencerede sen
> yaparsın), özel API'leri taklit etmez ve bir tarayıcının sürüldüğünü gizlemez — ama bunların hiçbiri
> kullanımı izinli yapmaz. Özellik **deneyseldir**: sayfalar haber vermeden değişir ve çalışmayı bırakabilir.
> Yalnızca riskini göze aldığın bir hesapla kullan. Uygulama içi uyarı her site için ayrıca onaylanır.

## Kullanım

1. Ayarlar › Beta Özellikleri: Beta'yı aç.
2. Ayarlar › Web sohbet: site için **Uyarıyı oku ve etkinleştir** (kutuyu işaretle).
3. **Bağlan**: Memo tarayıcı penceresi açılır. Giriş yap, varsa insan doğrulamasını tamamla. Sayfa hazır
   olunca pencere küçülür.
4. **Modellerime ekle**; model seçicide *ChatGPT (Web)* / *Claude (Web)* seç.

Yalnızca metin: araç çağırma, dosya, görsel yok. Her tur yeni (ChatGPT'de geçici) bir konuşmadır; sistem istemi
ve o ana kadarki yazışma tek mesajda gider, böylece sitedeki duruma hiçbir şey bağlı kalmaz. Yardımcı çağrılar
(sohbet başlığı, olgu çıkarma, ruh hâli, proaktif kontroller…) ve "Bağlantıyı sına" web sohbetine **asla**
gönderilmez; başlık ilk sözcüklerinden üretilir.

## Kırılganlık nasıl sınırlandı

| Risk | Ne yapıldı |
|---|---|
| Sayfa değişir | Seçiciler veridir (`internal/webchat/site.go`), her listede yedek vardır; tüm DOM erişimi JSON döndüren ve hiç fırlatmayan tek bir betikte (`page.js`). |
| Çıkış yapılmış sayfaya yazmak | Çıkış durumu görünür yazı kutusundan önce gelir ve özniteliklerin yanında düğme **metniyle** ("Log in", "Continue with Google"…) tanınır. Giriş doğrulanmadan hiçbir şey yazılmaz. |
| Anlaşılmaz hatalar | Her hata birkaç türden biridir (kurulu değil, giriş gerek, insan doğrulaması, sayfa değişti, kota, zaman aşımı, meşgul, kapalı, onay yok) ve sade bir cümleyle gösterilir. Ham tarayıcı hatası yalnız log'a gider. |
| Hesabı yanlışlıkla harcamak | Beta **ve** uyarı onayı yoksa kapalı. Web sohbetleri hiçbir zaman yedek değildir (`provider.IsOptInOnly`) ve Auto Route adayı olmaz. |
| Memo'yu düşürmek | Sağlayıcıyı kurmak tarayıcı başlatmaz; panik yakalanır; ölü ya da tuhaf tarayıcı atılıp sonraki turda yeniden açılır; hata diğer sağlayıcılara dokunmaz. |
| Yarım markdown | Akarken bitmemiş son satır tutulur; gönderilen metin asla geri alınmak zorunda kalmaz. |

## Neler doğrulandı, neler doğrulanmadı

Yerel sahte sayfa + gerçek Chromium ile doğrulandı (`internal/webchat`, `internal/e2e`): tüm akış, hata
türleri, çıkış-yapılmış kuralı, tarayıcı ölünce yeniden başlama, onay kapısı, yardımcı çağrı olmaması.
Gerçek sayfalar çıkış yapılmış hâlde okunarak doğrulandı (2026-10-13): chatgpt.com `?temporary-chat=true`'yu
korur, kullanılabilir bir yazı kutusu ve test kimliği olmayan bir "Log in" düğmesi gösterir; claude.ai/new
`/login`'e yönlenir.

**Doğrulanmadı:** girişin arkasındaki her şey — iki sitenin gerçek yazı kutusu, gönder/durdur düğmeleri ve
asistan mesajı kancaları, gerçek cevapların markdown dönüşümü, gerçek kota ifadesi ve Cloudflare davranışı.
İki site de `Verified=false` taşır, arayüz giriş yapılmış bir hesapla kontrol edilene kadar *Deneysel*
etiketi gösterir (kontrol edilince `Verified` işaretlenir, farklı çıkan seçiciler düzeltilir).
