# Dijital İkiz arayüz sözleşmesi

Amaç: Mail ekranından yanıt hazırlamayı açmak, kişi istisnasını fark etmek ve
hazırlanan taslağı incelemek. Referans: kullanıcının 1 Kasım 2026 tarihli
1000×600 ekran görüntülerinde mail anahtarı açıkken ayrı ana kapı kapalıydı;
kişinin `never` tercihi genel otomatik ayarı geçersiz kılıyordu.

- Sakin, sık kullanılan masaüstü ayar ekranı: mevcut Memo renkleri, tipografi ve
  ikonları. Altın renk seçili tercih/etkin işlem için; hata yalnız engel için.
- Birincil kontrol **E-posta yanıtlarını hazırla**. Açılması hem motoru hem mail
  kanalını atomik açar. Kapatılması diğer kanalı kapatmaz. Kullanıcının mevcut
  verisi veya kişi tercihi arayüz yenilenirken değiştirilmez.
- İlk görünümde açık/kapalı durumu, seçili yanıt modeli ve üç varsayılan tercih.
  Diğer kanallar/ses ikincil açılır bölümde. Gönderim henüz yok: kısa kalıcı not.
- Kişiler listesi kişi tercihini gösterir. Mailde seçili gönderenin yanıtı
  kapalıysa, kişi ayrıntısına doğrudan bağlantılı açıklama gösterilir.
- Hedefler: **1000×600**, **1440×900**, **480×800**. Büyükte tercihler yan yana,
  darda alt alta; içerik kaydırılır, başlık/kapatma/sekme alanı korunur.
- Yükleme: sabit kabuk ve ilerleme. Boş: yeni maille deneme/açma açıklaması.
  Hata: okunur hata ve tekrar deneme. Kısmi: model seçilmemişse açık açıklama.
  Başarı: kaydedilmiş taslak ve kullanıcı incelemesi durumu. Yetki: engel
  açıklaması ve geri dönme/kapatma. Uzaktan otomatik gönderim bu işin kapsamı değil.

ECC `frontend-design-direction`, `make-interfaces-feel-better` ve `error-handling`
becerileri uygulandı. Yerel ekran görüntüsü/JSON kanıtı:
`~/.codex/visualizations/2026/11/01/memo-twin/`.
Gerçek hesapta SMTP gönderimi veya kullanıcı ayarı değişikliği yapılmadı.

Manuel yanıt doğrulaması: SMTP alıcı zarfı yalnız posta adresi taşır; görünen
Türkçe ad MIME başlığında korunur. Virgüllü tırnaklı adlar tek alıcı kalır.
Yanıt konusu bir kez `Re:` alır; hata halinde yazılan metin kaybolmaz.
