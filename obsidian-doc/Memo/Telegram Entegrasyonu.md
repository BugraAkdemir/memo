# Telegram Entegrasyonu

> **Paket:** `internal/telegram/` (`client.go`, `store.go`)
> **API endpoint'leri:** `/api/telegram/status`, `/api/telegram/connect`, `/api/telegram/stop`, `/api/telegram/disconnect`
> **Eklendi:** v3.9.0

Memo, WhatsApp'ın yanında ikinci bir sohbet yüzeyi olarak bir Telegram bot'una bağlanabilir. WhatsApp'ın whatsmeow entegrasyonundan farklı olarak (o, tüm mevcut WhatsApp hesabına görünürlüğü olan tam bir WhatsApp Web istemcisini taklit eder), bir Telegram bot'u sadece kendisine doğrudan gönderilen mesajları görebilir — çok daha ağır olan MTProto kullanıcı API'si (telefon numarasıyla giriş, bot token değil) olmadan "diğer sohbetlerimi oku" gibi bir şey yoktur. Bu kapsam farkı bilinçli: bu paket, `@BotFather`'dan alınan bir bot token'ının Memo ile konuşmasına bir yol açmak için var, WhatsApp'ın kişi/grup/geçmiş genişliğini taklit etmek için değil.

## Kurulum

1. Telegram'da [@BotFather](https://t.me/BotFather) ile konuş, bir bot oluştur, token'ını al.
2. Token'ı Settings → Telegram'a yapıştır (ya da `POST /api/telegram/connect`).
3. Memo, yeni mesajlar için Bot API'yi long-polling ile dinlemeye başlar (`internal/telegram/client.go`).

## Sahip Kilidi

Bir bot'un kullanıcı adını bulan herkes ona mesaj atabildiği için, Memo'nun kendi erişim-kontrol sınırına ihtiyacı var — WhatsApp'taki gibi bir QR-eşleştirme adımı yok burada. `shouldReplyToTelegram` (`internal/app/telegram.go`) **ilk** mesajı atan kişiyi bot'un kalıcı sahibi olarak kilitler (`tgStore.SetOwner`), sonrasında diğer tüm gönderenleri sessizce yok sayar. Bu, entegrasyonun tüm yetkilendirme modelidir — paylaşımlı/çoklu-kullanıcı bir Telegram bot modu yoktur.

## Kendine-Sohbet Asistanı

Sahip bağlandıktan sonra, bot'a mesaj atmak WhatsApp'ın kendine-sohbetiyle aynı asistan yeteneğini verir: sohbet, hafıza ve agent araçları, Memo'yu açmadan. `handleTelegramMessage`/`handleTelegramCommand` (`internal/app/telegram.go`), `handleWhatsAppSelfChatMessage`/`handleWhatsAppSelfChatCommand`'ı yansıtır — baştaki slash komutu doğrudan işlenir, geri kalanı normal arka plan sohbet oturumundan geçer (`sm.NewBackgroundChat`, çalışma boyunca `a.tgSelfChatSessionID` olarak önbelleklenir).

- **Sohbetten rutin oluşturma**: düz dille iste, Memo uygulama içindeki aynı `create_routine`/`list_routines`/`cancel_routine` agent araçlarıyla bir rutin oluşturur/listeler/iptal eder.
- **İzin yanıtları**: `routeTelegramPermissionAnswer`, bekleyen bir agent-tool izin sorusuna Telegram mesajı olarak gelen yanıtı yakalar — WhatsApp'taki akışla aynı fikir.

## Teknik

- **Depolama**: `internal/telegram/store.go` — `OwnerChatID` (0 = henüz bağlanmadı) artı mesaj geçmişi, WhatsApp'ın kendi SQLite veritabanından izole.
- **Long-polling, webhook değil**: self-host etmesi daha basit (herkese açık HTTPS endpoint gerekmez), bedeli webhook push'a göre biraz daha yüksek gecikme.
- **Durum**: işlevsellik unit testlerle kapsanıyor (`client_test.go`, `store_test.go`); kullanıcının gerçek Telegram uygulamasında uçtan uca tıklayarak canlı bir bot'a karşı henüz doğrulamadığı bir özellik.

## Modeller, görseller ve okunabilir hatalar (v4.6.0)

WhatsApp kendine-sohbetiyle aynı üç ek (`internal/app/model_switch.go`, `selfchat_turn.go`, `selfchat_l10n.go` ortak, iki bot birbirinden ayrışamaz).

- **`/model`**: bir mesaj uzağındaki her modeli listeler (açık sağlayıcıların modeli, Abonelik hesaplarının canlı modelleri sağlayıcıya göre gruplu, yerel model), numaralı; aktif ✅, görsel modeli 🎨. `/model 3`, `/model gemini` ya da `/model <sağlayıcı> <model>` değiştirir; arama tam listenin numaralarını korur. `/status` modeli de söyler.
- **Gelen görsel:** `client.go` artık `photo` (en büyük boyut) ve görsel `document`'leri ile `caption`'ı okur; `Client.DownloadFile` `getFile` + dosya adresini kullanır (20 MB sınırı). Fotoğraf, uygulamanın görsel gönderiminin sohbet-kimlikli ikizi `App.SendMessageWithImageStreamTo`'ya gider; sohbet modeli anlatır, altındaki yazı değişiklik istiyorsa düzenlenir.
- **Giden görsel:** turun çizdiği görsel (`generated_image` işaretçisi) kasadan okunup `sendPhoto` ile gönderilir; Telegram fotoğrafı yeniden sıkıştırır ve bazı boyutları reddeder, ret olursa `sendDocument`'a düşülür.
- **`/image <açıklama>`** ifade nasıl olursa olsun görsel zorlar (`parseImageCommand`; `/img`, `/resim`, `/görsel` ve `@botadı` eki de tanınır). Fotoğrafla birlikte fotoğrafı düzenler. Görsel modeli yoksa bunu söyler.
- **Otomatik yönlendirme:** "bana bir kedi resmi çiz" / "draw me a cat" fark edilir (`image_intent.go`, temkinli) ve yalnızca o mesaj için bir görsel modeline gider; sonraki mesaj yine sohbettir. Abonelik hesabı kendi hesabının görsel modeliyle çizer; diğer sağlayıcılar varsayılan görsel modelini kullanır (Ayarlar › API Sağlayıcıları › Görsel üretimi).
- **Okunabilir hatalar:** turun hata metni gönderilmeden önce `App.FriendlyError`'dan geçer; 400/401/403/404/429/5xx bir cümle olarak okunur.
- **Kendi barındırdığın Bot API / testler:** `MEMO_TELEGRAM_API_BASE=http://127.0.0.1:8081` istemciyi başka bir Bot API sunucusuna yönlendirir (e2e takımındaki `FakeTelegram` bunu kullanır).

Sahte bir Bot API üzerinden uçtan uca doğrulandı (`internal/e2e/telegram_media_e2e_test.go`). **Gerçek Telegram uygulamasıyla doğrulanmadı.**

## Bağlantılı Notlar:
- [[WhatsApp Entegrasyonu]] — diğer kendine-sohbet yüzeyi, aynı asistan deseni
- [[Ajan Modu]] — her iki kendine-sohbet yüzeyinin de yönlendiği tool-calling döngüsü
- [[Backend (Go) Mimarisi]] — paket yapısı
