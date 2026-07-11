# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-12 (Session 21, devam)
> **Not:** Bu dosya daha önce 1300+ satırlık, onlarca oturumun anlatısını ve 100 düzeltilmiş bug'ı içeren tarihsel bir arşivdi. Bu haliyle kullanılamaz hale gelmişti (görünüşte "27 açık bug" diyordu, gerçekte bunların çoğu zaten düzeltilmişti ama tablo hiç güncellenmemişti). Temizlendi — sadece hâlâ gerçekten açık olan maddeler kaldı.
>
> **İkinci geçiş (aynı gün):** Kalan eski maddeler tek tek koda karşı yeniden doğrulandı. Sonuç: "Mobile API client eksik" iddiası artık geçersizdi (118 backend endpoint'inin 111'i zaten destekleniyor, eksik 7'si de mobile'a hiç uygun değil — kaldırıldı). `AGENTS.md`'nin "Known Pitfalls" bölümü de tarandı: iki madde ("data race" olarak işaretlenen `a.client`/`providerRouter` reassignment'ları) meğerse zaten kilitli imiş — gerçek risk daha dar (BUG-L4), "memory full rebuild O(N)" notu ise referans verdiği `LoadCache` fonksiyonu artık kodda hiç yok, tamamen bayat — hiç eklenmedi.
>
> **Session 20:** İki kritik madde (BUG-C1, BUG-C2) düzeltildi — bkz. commit `de4450e`, `f5a579e`. Ardından sırayla eski BUG-H1/H2/H3 (auth yan etkisi, SQLite izinleri, dead-code sandbox, panic recovery) ve eski MEDIUM listesindeki 7 madde (websearch-memory race, Minimal Mod dual-source, consolidation sessiz hata, izin diyaloğu sessiz kapanma, toggle çift-tık race'i, detached backend zombi süreci, SIGTERM'de unregister eksikliği — son ikisi `4f364f4`/`14e545f`) düzeltildi. Kalan 3 HIGH maddesi (chat-switch race — mimari refactor gerektiriyor; Windows auto-shutdown — bu ortamda test edilemez) ve M1/M2 (dosya boyutu notu, kabul edilmiş polling) bilinçli olarak atlandı. Tam detay için `git log`.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 3 |
| 🟡 MEDIUM | 2 |
| 🟢 LOW | 4 |
| **TOPLAM** | **9** |

---

## 🟠 HIGH

### BUG-H1: Stream ortasında sohbet değiştirilirse mesaj yanlış sohbete karışabiliyor

- **Dosya:** `internal/app/chat.go:210-217` (`sendMessageStreamInner`)
- **Nedir:** `buildMessages` o an aktif sohbetin geçmişini okuyor, ama kullanıcı mesajı ve yanıt daha sonra, `sm.GetActiveID()`/`sm.AddMessage()` çağrıldığı **o andaki** aktif sohbete yazılıyor. `/api/chats/switch` (`server.go:450`) `SwitchChat`'i doğrudan çağırıyor, `streamMu` ile hiç senkronize değil.
- **Kullanıcı etkisi:** Web arama/hafıza sorgusu sürerken kullanıcı başka bir sohbete geçerse, eski sohbetin bağlamıyla üretilen cevap yanlışlıkla yeni aktif sohbete eklenir.

### BUG-H2: Sohbet değiştirmek, hâlâ stream'de olan eski sohbetin Flutter notifier'ını dispose ediyor

- **Dosya:** `frontend/lib/providers/chat_provider.dart:87-108, 173-471`
- **Nedir:** `ActiveChatIdNotifier.switchTo`, `messagesProvider.notifier.stopStreaming()` çağırıp `ref.invalidate(messagesProvider)` ile notifier'ı dispose ediyor — ama stream döngüsündeki, post-stream finalize bloğundaki ve catch bloğundaki hiçbir `state = ...` yazımı "dispose edildi mi" kontrolü yapmıyor (sadece gecikmeli liste-yenileme timer'ı kontrol ediyor).
- **Kullanıcı etkisi:** A sohbetinde yanıt akarken B'ye geçilirse, A'nın notifier'ı ya dispose edilmiş bir state'e yazmaya çalışıp hataya düşüyor, ya da geç gelen yanıt kimsenin dinlemediği bir state nesnesine uygulanıyor — H1'in frontend karşılığı.

### BUG-H3: Backend otomatik-kapanma özelliği Windows'ta tamamen çalışmıyor

- **Dosya:** `internal/app/clients.go:136-142` (`selfShutdownSignal`)
- **Nedir:** `p.Signal(os.Interrupt)` ile kendine sinyal gönderme, Go'nun Windows implementasyonunda desteklenmiyor (`Process.Signal` sadece `Kill`'i destekliyor, diğerleri `EWINDOWS` hatasıyla dönüyor) — hata sessizce yutuluyor.
- **Kullanıcı etkisi:** Windows'ta, `spawnDetachedBackend`'in başlattığı her backend, son client ayrıldığında kendini kapatmayı deniyor ama başaramıyor — process arka planda süresiz birikiyor, elle kapatılana kadar.
- **Düzeltme:** Windows'ta `Process.Signal(os.Interrupt)` yerine platforma özgü bir yol (`taskkill`/`GenerateConsoleCtrlEvent`) kullanılmalı.

---

## 🟡 MEDIUM

### BUG-M1: `model_store_screen.dart` — 2600+ satır tek dosya

- **Dosya:** `frontend/lib/screens/model_store_screen.dart` (doğrulandı: 2612 satır)
- **Kullanıcı etkisi:** Doğrudan bug değil ama bakım yapılamaz hale geliyor, değişikliklerde kırılma riski yüksek.

### BUG-M2: `connectionStatusProvider` sonsuz polling

- **Dosya:** `frontend/lib/providers/chat_provider.dart:671`
- **Kullanıcı etkisi:** 30 saniyede bir `isAlive()` sorgusu, provider dispose olsa bile devam eder. Küçük bir performans sızıntısı — AGENTS.md'de "kabul edilebilir" diye not düşülmüş ama teknik olarak hâlâ açık.

---

## 🟢 LOW

### BUG-L1: İzin diyaloğu, stream durdurulsa/sohbet değiştirilse bile ekranda bayat kalabiliyor

- **Dosya:** `frontend/lib/screens/app_shell.dart:87-99`, `frontend/lib/widgets/agent/permission_dialog.dart`
- **Nedir:** Diyalog `barrierDismissible: false` ile bir `requestId`'ye bağlı açılıyor ama `stopStreaming()`/`switchTo()` ile hiç senkronize değil.
- **Kullanıcı etkisi:** Kullanıcı Stop'a basıp stream'i iptal etse bile diyalog ekranda kalıyor; sonunda Allow/Deny'e basınca artık var olmayan bir request için karar gönderiyor.

### BUG-L3: Kapanma kararı, sinyal gerçekten teslim edilene kadar yeniden doğrulanmıyor

- **Dosya:** `internal/app/clients.go:94-129` (`UnregisterClient`, `sweepStaleClients`)
- **Nedir:** `shouldShutdown` kararı anlık registry durumuna bakıyor ve `selfShutdownSignal` çağrısı ile bu karar arasında dar bir zaman penceresi var.
- **Kullanıcı etkisi:** Nadir bir zamanlamada, tam o sırada `/gui` ile bağlanan yeni bir client, kapanmak üzere olan bir backend'e kaydolup hemen ardından o backend'in kapanmasıyla karşılaşabilir.

### BUG-L4: Model/sağlayıcı değişimi tam stream ortasına denk gelirse, o stream durdurulmuş/değiştirilmiş bir client ile konuşmaya devam ediyor

- **Dosya:** `internal/app/llm.go:714-716,965-967` (`a.client` okuması), `internal/app/providers.go` (`a.providerRouter` okuması)
- **Nedir:** `AGENTS.md`'nin "Data Races" olarak listelediği bu iki madde aslında **veri yarışı değil** — `clientMu`/`providerMu` hem okuma hem yazma tarafında düzgün kilitleniyor, doğrulandı. Gerçek kalan risk daha dar: bir stream başlarken `a.client`'i kilit altında local bir değişkene kopyalıyor (`streamClient := a.client`), ama stream saniyelerce sürebiliyor — bu sırada kullanıcı modeli değiştirirse (`StopLocalModel`/`StartLocalModel`), o an akan stream hâlâ **eski, artık durdurulmuş** client'ı kullanmaya devam ediyor.
- **Kullanıcı etkisi:** Model swap tam bir mesaj akarken yapılırsa, o mesaj muhtemelen "connection refused" ile başarısız olur — veri bozulması ya da çökme değil, ama şaşırtıcı bir hata.
- **Not:** `AGENTS.md`'deki orijinal not düzeltilmeli — mevcut "data race" ifadesi yanlış, kod zaten kilitli.

### BUG-L5: ngrok otomatik-başlatma, restart sonrası masaüstü Flutter GUI'sini token olmadan bırakabiliyor

- **Dosya:** `internal/app/app.go:356-369` (Startup — ngrok auto-start), `frontend/lib/core/api_client.dart` (yeni `_applyRemoteToken`)
- **Nedir:** BUG-C1'in düzeltmesiyle (`f5a579e`) gelen bilinen, dar kapsamlı bir yan etki. Masaüstü Flutter istemcisi token'ı yalnızca `getRemoteAccess()`/`setRemoteAccess()` yanıtlarından, o anki oturumda öğreniyor (bellekte tutuluyor, her yeniden başlatmada sıfırlanıyor). Ama `cfg.RemoteAccess.NgrokAutoStart` açıksa, backend `Startup()` sırasında sunucuyu **doğrudan** `0.0.0.0`'a bağlıyor — Flutter GUI henüz hiçbir istek atmadan, token'ı öğrenme fırsatı bulamadan. Bu durumda GUI'nin restart sonrası ilk isteği (örn. `/api/status`) 401 ile reddedilir.
- **Kullanıcı etkisi:** ngrok otomatik-başlatma açık olan bir kullanıcı, uygulamayı her yeniden başlattığında masaüstü GUI'nin "backend'e bağlanılamıyor" göstermesiyle karşılaşabilir — Ayarlar'dan Uzak Erişim'i kapatıp tekrar açmak (yeni bir `setRemoteAccess` çağrısı, token'ı taze bir şekilde yakalar) geçici çözüm.
- **Neden şimdi düzeltilmedi:** Gerçek çözüm ya ngrok'un ilettiği trafiği loopback'ten ayırt edecek güvenilir bir sinyal gerektiriyor (ngrok'un local agent'ı zaten `127.0.0.1:port`'a bağlanıyor — kaynak IP'ye güvenmek BUG-C1'i yeniden açar), ya da Tailscale tünelinin zaten kullandığı desene geçiş (`internal/tunnel/tailscale.go` — ana sunucu hep loopback'te kalır, önüne ayrı bir reverse-proxy dinleyici konur). İkisi de canlı bir ngrok/telefon testi gerektiren, bu oturumun kapsamının dışında bir mimari değişiklik.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
