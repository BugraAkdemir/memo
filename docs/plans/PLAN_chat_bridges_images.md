# PLAN — Telegram/WhatsApp modelleri, görseller, otomatik görsel yönlendirme, şifreli görseller (2026-10-08, gece)

## İstek (kullanıcıdan, kelimesi kelimesine)

> tamam senden isteiğim şu önce telegram whatssap ve wahttsap için model değiştirme yani /model ilen ekli modeler arasında değişim yapabieyim 2. oalrka ta telegram ve whatsap üzeründen img2img veya text2img olarka kulalnabilemek analdınkı yada img2text gibi analadnmı isteiğimi ve birde 400 500 404 gibi bilnen hata mesajlarını kullanıya saçma şekilde vermek yerine açıklayıcı şekilde bas chatte ve auto route desteği getir ama bu aboleklikerde şu şekilde olacak
>
> örnek codex in chat text modelini kulalnıyorum ve dedimki resim üret gibi dediğimde ben elimi sürmeden codex in image modeline geçecek ve tekara text yazdıgımda text den devam edecek bu antigraty codex için aynı
>
> api proverdalar için ise ayarlara uygun bir yere varsayılan image modeli ekle onuda o şekilde yap örnek kullancı opencode go kulalnıyor şu resmi yap dedi varsyılan image modeline istek atacak analdınmı ama bak abolikte varsayılana değil o aboliklteki image modeline istek aatcak anladınmı birde image çıktıarı için resimi büyütme incelleme indirme özellikeri ekle ama ilk önce kesinklike hızlı şekilde telegram ve whatssap işini hallet ben uyuyorum aklında soru varsa sor yoksa sabaha kadar gelene kdar herşeyi kuralalra uygun bir biçimde yap
>
> artık uyuyorum tüm yetkin var proje üstünde ve birde üretilen ve gönderilen img ler şifreli olsun yani örnek ben memo ya bir resim düzetterdim çıktıyı ve girdi resmi şifreli olsun bu şifre aşırı güçlü yani dosya sıza bile resim görünemesin bunuda unutma dediklerimin hepsi sabaha bitsin
>
> kesinklie push yapma

## Sınırlar
- **Push yok** (hiçbir remote'a, hiçbir etiket). Commit'ler yerelde.
- Gerçek `data/` ve gerçek satıcı hesapları (Codex/Antigravity kimlik bilgileri) okunur-dokunulmaz: token yenilemesi orijinali bozabilir. Doğrulama sahte Bot API / sahte sağlayıcı / sahte sidecar ile.
- Önce Telegram + WhatsApp, sonra gerisi.

## Yapılacaklar
- [x] `/model` — Telegram + WhatsApp (liste, numara, ad, `<sağlayıcı> <model>`)
- [x] Telegram + WhatsApp: gelen fotoğraf → anlat (img2text) / düzenle (img2img); sözle görsel → çiz (text2img); giden görsel
- [x] `/image` ile zorla; görsel modeli yoksa açıkla
- [x] 400/500/404… hataları sohbette sade cümle (botlar, akışsız uç noktalar, Canlı Mod)
- [x] Otomatik yönlendirme: tek tur görsel modeline gider, sonraki mesaj yine metin modeli (tüm yüzeyler)
- [x] Abonelik: o hesabın kendi görsel modeli (önce kullanılan modelin sağlayıcısı); varsayılana gitmez
- [x] API sağlayıcılar: Ayarlar'da varsayılan görsel modeli (+ otomatik yönlendirme anahtarı) — Flutter + REST
- [x] Görsel büyütme / inceleme / indirme (Flutter)
- [x] Gönderilen ve üretilen görseller diskte şifreli (XChaCha20-Poly1305, dosya başına anahtar, anahtar veri klasörü dışında); yükleme temp'e düz yazılmıyor
- [x] Doküman (EN/TR, iki Obsidian kasası, v4.6.0 notları), AGENTS.md tuzakları, handoff.md

## Doğrulanmadı (dürüstçe)
Gerçek Telegram/WhatsApp uygulaması, gerçek satıcı görsel üretimi, Flutter'da gözle bakış, kaydet penceresi. Ayrıntı: `handoff.md` üst girdi.
