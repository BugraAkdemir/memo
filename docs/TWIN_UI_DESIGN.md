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
  Diğer kanallar/ses ikincil açılır bölümde. Otomatik gönderim henüz yok: kısa kalıcı not.
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

## Mail simplification and owner review — 2 November

Reference: the user could not find where to send a generated draft, while account
connection panels and technical copy occupied the main inbox. Keep Memo's quiet
desktop palette/type, but prioritize the daily mail workflow:

- Header: Mail, new message, synchronize, add account and a settings menu.
- Primary sections: Inbox / Replies, with a pending count. Connection cards only
  appear when there is no account. Details use one pane with Back below 850px.
- Replies: selected account → status filters → recipient/subject → draft preview
  → Review and send. Inbox search is not shown in Replies because it searches
  the inbox. Review fixes recipient/account, edits subject/body, and keeps the
  Send action distinct from Close. No unattended send is introduced.
- Target viewports remain 1000×600, 1440×900 and 480×800. Cards/list content scroll;
  compact headers constrain long copy. The inbox activity strip is bounded and
  avoids repeating the full draft; Replies uses the full card instead.
- Loading: status/progress while activity loads. Empty: explanatory reply view.
  Partial: draft remains reviewable with an unavailable decision score. Error:
  uncertain delivery keeps edits and disables retry, directing the owner to Sent.
  Success: sent state/filter. Permission: failed access explains the owner-only boundary
  with settings/refresh controls; no draft data is exposed by the denied request.
- Rendered fixture evidence: `~/.codex/visualizations/2026/11/02/memo-mail/`.
  Uses production theme/fonts and synthetic messages; local images only.
  Keyboard Tab traversal, visible focus, long text/narrow layout, icon labels,
  desktop button hit areas and reduced-motion configuration are checked there.

The earlier “delivery unavailable” note now refers specifically to unattended
sending. Manual owner delivery and its limitations: `MAIL_REPLY_REVIEW.md`.
