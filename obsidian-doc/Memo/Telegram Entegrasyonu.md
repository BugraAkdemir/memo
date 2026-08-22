# Telegram Entegrasyonu

> **Paket:** `internal/telegram/` (`client.go`, `store.go`)
> **API endpoint'leri:** `/api/telegram/status`, `/api/telegram/connect`, `/api/telegram/stop`, `/api/telegram/disconnect`
> **Eklendi:** v3.9.0

Memo, WhatsApp'ın yanında ikinci bir sohbet yüzeyi olarak bir Telegram bot'una bağlanabilir. WhatsApp'ın whatsmeow entegrasyonundan farklı olarak (o, tüm mevcut WhatsApp hesabına görünürlüğü olan tam bir WhatsApp Web istemcisini taklit eder), bir Telegram bot'u sadece kendisine doğrudan gönderilen mesajları görebilir — çok daha ağır olan MTProto kullanıcı API'si (telefon numarasıyla giriş, bot token değil) olmadan "diğer sohbetlerimi oku" gibi bir şey yoktur. Bu kapsam farkı bilinçli: bu paket, @BotFather'dan alınan bir bot token'ının Memo ile konuşmasına bir yol açmak için var, WhatsApp'ın kişi/grup/geçmiş genişliğini taklit etmek için değil.

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

## Bağlantılı Notlar:
- [[WhatsApp Entegrasyonu]] — diğer kendine-sohbet yüzeyi, aynı asistan deseni
- [[Ajan Modu]] — her iki kendine-sohbet yüzeyinin de yönlendiği tool-calling döngüsü
- [[Backend (Go) Mimarisi]] — paket yapısı
