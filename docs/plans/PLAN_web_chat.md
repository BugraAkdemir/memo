# PLAN — Web sohbet sağlayıcıları (ChatGPT Web + claude.ai) — Beta

Amaç: Codex/Claude API kotasını yakmadan, kullanıcının KENDİ hesabının web sohbet kotasından yanıt almak.
Sadece düz metin; araç çağırma YOK. Beta kapısı + yasal uyarı onayı olmadan hiçbir şey çalışmaz.

## Kaynaklar (mantığı aldığımız, MIT)
- `MrBalourd/chatgpt-api-web` (`index.js`): composer seçici yedek listesi (`#prompt-textarea` → `textarea` →
  `contenteditable`), asistan mesajı `[data-message-author-role="assistant"]`, "üretiyor" tespiti
  `[data-testid="stop-button"]`, mesaj sayısı/metin farkı + `STABLE_MS` kararlılık bekleme, prefix-diff akışı.
- `yushangxiao/claude2api`: claude.ai tarafının oturum/konuşma akışı (referans; biz API'yi taklit etmiyoruz,
  gerçek sayfayı sürüyoruz).
Atıf: dosya başlıklarında ve `docs/`.

## İlkeler (kırılganlığı en aza indirme)
1. **Gerçek tarayıcı, taklit yok.** Memo'nun bulduğu Chromium'u görünür pencerede, Memo'ya ait profil klasöründe
   açar. Giriş/doğrulama/captcha kullanıcıda. Bot-tespit atlatma (stealth, PoW çözme, Camoufox…) YOK.
2. **Seçiciler veridir.** Her site için sıralı yedek listeleri (`site.go`); bir seçici kırılırsa sıradaki devreye girer.
   Tüm DOM etkileşimi tek enjekte JS içinde → 1 gidiş-dönüş, tek test noktası.
3. **Her başarısızlık sınıflanır.** `ErrKind` (NotInstalled, NeedsLogin, Challenge, PageChanged, Quota, Timeout,
   Busy, Disabled, NoConsent). Kullanıcıya sade cümle; ham chromedp/Go hatası yalnız log'a.
4. **İzolasyon.** Yeni paket; sağlayıcı yalnızca kullanıcı ekleyince var; varsayılan yedek zincirine girmez;
   kurucu asla hata vermez (tarayıcı tembel açılır); her goroutine `recover`; bir site bozulursa diğeri ve geri
   kalan Memo etkilenmez; bozulan oturum sıfırlanır, sonraki çağrı temiz başlar.
5. **Durumsuz konuşma.** Her tur yeni (geçici) sohbet + düzleştirilmiş transkript. Web'deki konuşma geçmişi
   ile Memo arasında durum kayması olmaz.
6. **Doğrulanamayan her şey handoff'ta "doğrulanmadı".** Gerçek hesapla deneme kullanıcıda.

## Adımlar
- [ ] 1. `internal/webchat`: `site.go` (ChatGPT, Claude seçici verisi), `errors.go`, `page.js` (enjekte JS),
        `browser.go` (Chromium yaşam döngüsü, kalıcı profil), `driver.go` (durum tespiti, gönder, akıt), testler
        (yerel sahte site + gerçek Chromium; Chromium yoksa skip).
- [ ] 2. `provider.go`: `web-chatgpt` / `web-claude` sağlayıcı tipleri, `RegisterConstructor`, transkript düzleştirme,
        Beta + onay kontrolü, kota/hata sınıflaması.
- [ ] 3. Onay + durum + bağlan/kopar REST uçları (`/api/webchat/*`), `adminOnly`.
- [ ] 4. Flutter: yasal uyarı penceresi (TR+EN L10n), bağlan düğmesi, durum satırı.
- [ ] 5. Belgeler (`docs/WEB_CHAT.md`, AGENTS.md), handoff.
