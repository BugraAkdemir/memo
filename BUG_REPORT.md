# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-10 (Session 19)
> **Not:** Bu dosya daha önce 1300+ satırlık, onlarca oturumun anlatısını ve 100 düzeltilmiş bug'ı içeren tarihsel bir arşivdi. Bu haliyle kullanılamaz hale gelmişti (görünüşte "27 açık bug" diyordu, gerçekte bunların çoğu zaten düzeltilmişti ama tablo hiç güncellenmemişti). Temizlendi — sadece hâlâ gerçekten açık olan 22 madde kaldı.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 2 |
| 🟠 HIGH | 7 |
| 🟡 MEDIUM | 10 |
| 🟢 LOW | 3 |
| **TOPLAM** | **22** |

---

## 🔴 CRITICAL — Stable'ı Doğrudan Bloklayan

### BUG-C1: Uzak erişim (ngrok/LAN) açıkken hiçbir API endpoint'i kimlik doğrulaması istemiyor

**[Doğrulandı — middleware zinciri kod okunarak teyit edildi]**

- **Dosya:** `internal/webserver/server.go:262`
- **Nedir:** Middleware zinciri `limitBodyMiddleware(rateLimitMiddleware(stopCleaner, corsMiddleware(mux)))` — sadece body-boyutu limiti, rate limit ve CORS. Hiçbir yerde token/şifre kontrolü yok. `internal/app/remote.go:97-98` bir `RemoteAccess.Token` üretip `GET /api/remote-access` ile arayüze döndürüyor (`handlers_flutter.go:596-597`), ama `internal/webserver/*.go` içinde bu token'ın **hiçbir handler'da hiç karşılaştırılmadığı** doğrulandı — tamamen kozmetik.
- **Kullanıcı etkisi:** `SetRemoteAccess` bind adresini `0.0.0.0`'a çevirdiğinde ya da ngrok açıldığında (`remote.go:128-132`), ngrok linkine ya da LAN'a erişimi olan **herkes** hiçbir kimlik bilgisi girmeden: `POST /api/wipe` ile tüm veriyi silebilir, `POST /api/agent/permission` ile bir agent aracına kalıcı izin verip `run_command` üzerinden host'ta rastgele shell komutu çalıştırabilir, `POST /api/import` ile keyfi veri enjekte edebilir, `POST /api/shutdown`/`/api/uninstall`/`/api/cli/remove` çağırabilir.
- **Düzeltme:** Üretilen `RemoteAccess.Token`'ı gerçekten kullanan bir auth middleware eklenmeli — örn. `Authorization: Bearer <token>` header kontrolü, sadece `remoteAccessEnabled` iken zorunlu (localhost-only modda mevcut davranış bozulmamalı).

### BUG-C2: Agent'ın `rm -rf /` güvenlik filtresi, tam olarak engellemesi gereken komutu engellemiyor

**[Doğrulandı — gerçek Go regex testiyle]**

- **Dosya:** `internal/agent/tools/command.go:27`
- **Nedir:** Kara liste regex'i `\brm\s+-rf\s+/\b`. `\b` kelime sınırı, `/`'den sonra bir kelime karakteri gelmesini gerektiriyor — boşluk, `;`, `*` ya da satır sonu geldiğinde sınır oluşmuyor. Gerçek Go `regexp` ile test edildi:
  ```
  "rm -rf /"              -> match=false
  "rm -rf /*"             -> match=false
  "sudo rm -rf /"         -> match=false
  "rm -rf /; echo done"   -> match=false
  "rm -rf /home/user/foo" -> match=true   (bu aslında göreceli güvenli bir yol)
  ```
  Filtre, engellemesi gereken **tam olarak `rm -rf /`'nin kendisini** hiç yakalamıyor. `\brm\s+-rf\s+~\b` ve `\brm\s+-rf\s+\.\b` desenleri de aynı hatadan muzdarip.
- **Geçmiş:** Bu, genel bir uyarı olarak zaten biliniyordu (agent tool onayı gerektirdiği için "tasarım gereği" düşük öncelikli sayılmıştı, blacklist'in "encoding tricks ile atlatılabileceği" not düşülmüştü) — ama bu oturumda somut, kanıtlanmış, tam bir bypass olduğu doğrulandı; ciddiyeti buna göre yükseltildi.
- **Kullanıcı etkisi:** Agent modunda bir prompt injection ya da modelin kendi halüsinasyonu `run_command` üzerinden tam olarak `rm -rf /` (ya da sonuna `*`/`;` eklenmiş hali) çalıştırırsa, "güvenlik için kara listede" hatası **hiç tetiklenmiyor** — komut gerçek kullanıcı olarak, hiçbir OS-seviyesi sandbox olmadan direkt çalışıyor.
- **Düzeltme:** `\b` yerine daha sağlam bir desen (örn. `(^|[\s;&|])rm\s+-rf\s+/($|[\s;&|*])`) ya da tamamen whitelist yaklaşımına geçiş.

---

## 🟠 HIGH

### BUG-H1: Sağlayıcı API anahtarları ve uzak-erişim tokenleri, kimlik doğrulaması olmadan düz GET ile okunabiliyor

- **Dosya:** `internal/webserver/handlers_flutter.go:1057-1065` (`GET /api/providers`), `internal/app/remote.go` (`GET /api/remote-access`)
- **Nedir:** `GET /api/providers`, tüm sağlayıcı yapılandırmasını döndürüyor — kodun kendi yorumu "API keys in plaintext" diyor (`internal/provider/config.go:151`). C1'deki auth eksikliğiyle birleşince tamamen açık.
- **Kullanıcı etkisi:** ngrok/LAN erişimi olan biri tek bir GET isteğiyle tüm bağlı OpenAI/Claude/Gemini/vb. hesaplarının API anahtarlarını ve ngrok token'ını ele geçirir.

### BUG-H2: Hassas SQLite dosyaları dünyaya-okunabilir (0644) — config.yaml/providers.json gibi 0600'e sertleştirilmemiş

- **Dosya:** `internal/memory/store.go`, `internal/mood/store.go`, `internal/observer/store.go`, `internal/calendar/store.go`, `internal/whatsapp/store.go`
- **Nedir:** Hiçbiri `sql.Open`/`sqlstore.New` sonrası `os.Chmod` çağırmıyor, dosyalar process umask'ına düşüyor. Diskte doğrulandı: `data/memory/memory.db`, `data/mood/mood.db`, `data/profile/observations.db`, `data/calendar/events.db`, `data/whatsapp/session.db` ve `messages.db` hepsi `-rw-r--r--`.
- **Kullanıcı etkisi:** Paylaşılan bir makinede başka bir yerel kullanıcı hesabı tüm sohbet hafızasını okuyabilir; `whatsapp/session.db` özellikle whatsmeow'un Signal/Noise oturum anahtarlarını tutuyor — WhatsApp oturumu tamamen ele geçirilebilir.
- **Düzeltme:** Diğer hassas dosyalarla aynı desen — açtıktan sonra `os.Chmod(path, 0600)`.

### BUG-H3: Agent dosya sandbox'ı, listelenmemiş bir mount noktasındaki mutlak yollarla aşılabiliyor

- **Dosya:** `internal/agent/sandbox.go:143-149`
- **Nedir:** `ValidatePath`, proje dizini (`BasePath`) dışındaki bir mutlak yolu, sadece elle yazılmış kısa bir `protectedPaths` listesinde (`/etc/`, `/usr/`, `/boot/`, `/dev/`, `/sys/`, `/proc/`, `/var/`, `/home/`, `/root/`, `/tmp/`, `/run/`, `/opt/`, `/mnt/`, `/media/`) değilse doğrudan izin veriyor.
- **Kullanıcı etkisi:** `/srv/`, ikinci bir disk, ya da listede olmayan herhangi bir mount noktası — agent'ın "proje dizini dışına çıkmaması" gereken sandbox'ı fiilen çalışmıyor.

### BUG-H4: Streaming/agent goroutine'lerinde hiçbir panic recovery yok — tek bir panic tüm backend'i çökertir

- **Dosya:** `internal/app/llm.go` (satır 123, 210, 511, 621, 732'deki `go func(){...}` bloklar), `internal/agent/pipeline.go:93` (`RunStream`)
- **Nedir:** Tüm repo'da `recover()` sadece `internal/taskloop/engine.go:104-111`'de var (kendi yorumunda "bir panic tüm uygulamayı çökertmemeli" diyor) — bu desen en çok kullanılan streaming/tool-execution yoluna hiç uygulanmamış. Go, kurtarılmamış bir panic'te hangi goroutine'de olursa olsun tüm process'i öldürür.
- **Kullanıcı etkisi:** Bir tool handler'da ya da provider yanıtı parse ederken tek bir nil-pointer/type-assertion/index-out-of-range hatası, o an aktif olan **herkesi** (tüm sohbetler, WhatsApp köprüsü, takvim hatırlatıcıları) aynı anda düşürür.
- **Düzeltme:** `taskloop/engine.go`'daki `recover()` deseni bu goroutine'lere de uygulanmalı — muhtemelen en yüksek kaldıraçlı tek düzeltme.

### BUG-H5: Stream ortasında sohbet değiştirilirse mesaj yanlış sohbete karışabiliyor

- **Dosya:** `internal/app/chat.go:210-217` (`sendMessageStreamInner`)
- **Nedir:** `buildMessages` o an aktif sohbetin geçmişini okuyor, ama kullanıcı mesajı ve yanıt daha sonra, `sm.GetActiveID()`/`sm.AddMessage()` çağrıldığı **o andaki** aktif sohbete yazılıyor. `/api/chats/switch` (`server.go:450`) `SwitchChat`'i doğrudan çağırıyor, `streamMu` ile hiç senkronize değil.
- **Kullanıcı etkisi:** Web arama/hafıza sorgusu sürerken kullanıcı başka bir sohbete geçerse, eski sohbetin bağlamıyla üretilen cevap yanlışlıkla yeni aktif sohbete eklenir.

### BUG-H6: Sohbet değiştirmek, hâlâ stream'de olan eski sohbetin Flutter notifier'ını dispose ediyor

- **Dosya:** `frontend/lib/providers/chat_provider.dart:87-108, 173-471`
- **Nedir:** `ActiveChatIdNotifier.switchTo`, `messagesProvider.notifier.stopStreaming()` çağırıp `ref.invalidate(messagesProvider)` ile notifier'ı dispose ediyor — ama stream döngüsündeki, post-stream finalize bloğundaki ve catch bloğundaki hiçbir `state = ...` yazımı "dispose edildi mi" kontrolü yapmıyor (sadece gecikmeli liste-yenileme timer'ı kontrol ediyor).
- **Kullanıcı etkisi:** A sohbetinde yanıt akarken B'ye geçilirse, A'nın notifier'ı ya dispose edilmiş bir state'e yazmaya çalışıp hataya düşüyor, ya da geç gelen yanıt kimsenin dinlemediği bir state nesnesine uygulanıyor — H5'in frontend karşılığı.

### BUG-H7: Backend otomatik-kapanma özelliği Windows'ta tamamen çalışmıyor

- **Dosya:** `internal/app/clients.go:136-142` (`selfShutdownSignal`)
- **Nedir:** `p.Signal(os.Interrupt)` ile kendine sinyal gönderme, Go'nun Windows implementasyonunda desteklenmiyor (`Process.Signal` sadece `Kill`'i destekliyor, diğerleri `EWINDOWS` hatasıyla dönüyor) — hata sessizce yutuluyor.
- **Kullanıcı etkisi:** Windows'ta, `spawnDetachedBackend`'in başlattığı her backend, son client ayrıldığında kendini kapatmayı deniyor ama başaramıyor — process arka planda süresiz birikiyor, elle kapatılana kadar.
- **Düzeltme:** Windows'ta `Process.Signal(os.Interrupt)` yerine platforma özgü bir yol (`taskkill`/`GenerateConsoleCtrlEvent`) kullanılmalı.

---

## 🟡 MEDIUM

### BUG-M1: `model_store_screen.dart` — 2500+ satır tek dosya

- **Dosya:** `frontend/lib/screens/model_store_screen.dart`
- **Kullanıcı etkisi:** Doğrudan bug değil ama bakım yapılamaz hale geliyor, değişikliklerde kırılma riski yüksek.

### BUG-M2: Mobile API client eksik

- **Dosya:** `mobile/lib/core/api_client.dart`
- **Kullanıcı etkisi:** Mobil uygulamada birçok backend endpoint'i kullanılamıyor.

### BUG-M3: `connectionStatusProvider` sonsuz polling

- **Dosya:** `frontend/lib/providers/chat_provider.dart:677-690`
- **Kullanıcı etkisi:** 30 saniyede bir `isAlive()` sorgusu, provider dispose olsa bile devam eder. Küçük bir performans sızıntısı — AGENTS.md'de "kabul edilebilir" diye not düşülmüş ama teknik olarak hâlâ açık.

### BUG-M4: Web arama / hafıza açık-kapalı ayarları kilitsiz okunup yazılıyor

- **Dosya:** `internal/app/settings.go:70` (`a.cfg.WebSearch.Enabled`), `internal/app/memory.go:284` (`a.cfg.Memory.MemoryEnabled`)
- **Nedir:** İkisi de hiçbir kilit olmadan yazılıyor; `internal/app/helpers.go:82,102` ve `chat.go` aynı alanları yine kilitsiz okuyor. Stream sırasında ayar değiştirilirse `-race` altında yakalanan gerçek bir race.

### BUG-M5: Minimal Mod, iki farklı yerden asenkron okunuyor

- **Dosya:** `internal/identity/identity.go` (`id.MinimalMode`), `internal/app/helpers.go:87-91` (`cfg.Identity.MinimalMode`)
- **Nedir:** Aynı anlama gelen iki alan, `buildMessages` içinde farklı zamanlarda okunuyor; `SetMinimalMode` ikisini de kilitsiz, atomik olmayan şekilde yazıyor.
- **Kullanıcı etkisi:** Minimal Mod toggle'ı ile aynı anda gelen bir mesaj, yarı-uygulanmış bir sistem promptu üretebilir.

### BUG-M6: Hafıza birleştirme (consolidation), embedding hatası olursa sessizce vektör aramadan düşüyor

- **Dosya:** `internal/memory/store.go:1607` (`saveMerged`)
- **Nedir:** Embedding hatası olursa birleşmiş anı yine kaydediliyor ama embedding'siz, hiçbir log satırı olmadan.
- **Kullanıcı etkisi:** Consolidation sonrası bazı anılar vector search'te bir daha asla bulunamıyor, kimse fark etmiyor.

### BUG-M7: İzin diyaloğu, gönderme başarısız olsa bile başarılıymış gibi kapanıyor

- **Dosya:** `frontend/lib/widgets/agent/permission_dialog.dart:50-58` (`_submit`)
- **Nedir:** `handleAgentPermission(...)` `unawaited(...)` ile, hiçbir try/catch olmadan ateşleniyor, ardından `Navigator.pop` koşulsuz çağrılıyor.
- **Kullanıcı etkisi:** POST başarısız olursa kullanıcı hiçbir hata görmüyor, backend'deki tool call kendi timeout'una kadar askıda kalıyor.

### BUG-M8: Hafıza/Minimal Mod anahtarına hızlı çift-tıklama, kullanıcının istediğinin tersi bir sonuç verebiliyor

- **Dosya:** `frontend/lib/providers/settings_provider.dart:327-337, 357-367`
- **Nedir:** `MemoryEnabledNotifier.toggle`/`MinimalModeNotifier.toggle` ikisi de bayat state okuyup `await` bitene kadar güncellemiyor; `Switch` widget'ları bu süre boyunca kendini disable etmiyor.
- **Kullanıcı etkisi:** Hızlı çift-tık, anahtarın iki tıklamanın üretmesi gereken durumun tersinde kilitli kalmasına yol açabiliyor.

### BUG-M9: Ayrılmış (detached) backend süreci, CLI hâlâ açıkken zombi kalabiliyor

- **Dosya:** `main.go:203` (`spawnDetachedBackend`)
- **Nedir:** `cmd.Process.Release()` kullanılıyor, `Wait()` değil. `Setsid`'e rağmen backend hâlâ CLI'ın OS-seviyesi çocuğu (double-fork yok).
- **Kullanıcı etkisi:** Backend kendi kendine ya da `/api/shutdown` ile kapanırsa ve CLI hâlâ açıksa, süreç CLI kapanana kadar reap edilmemiş (zombi) kalıyor.

### BUG-M10: Dışarıdan SIGTERM gelirse CLI'ın "hoşçakal" (unregister) çağrısı hiç çalışmıyor

- **Dosya:** `main.go` (sinyal dalı), `internal/replcli/repl.go:77-85`
- **Nedir:** `main()` `replDone` goroutine'ini beklemeden dönüyor, `Run()`'ın deferred `UnregisterClient` çağrısı hiç çalışmıyor.
- **Kullanıcı etkisi:** Backend'in bunu fark etmesi (heartbeat staleness sweep) 90 saniyeye kadar sürebiliyor, anlık değil.

---

## 🟢 LOW

### BUG-L1: İzin diyaloğu, stream durdurulsa/sohbet değiştirilse bile ekranda bayat kalabiliyor

- **Dosya:** `frontend/lib/screens/app_shell.dart:87-99`, `frontend/lib/widgets/agent/permission_dialog.dart`
- **Nedir:** Diyalog `barrierDismissible: false` ile bir `requestId`'ye bağlı açılıyor ama `stopStreaming()`/`switchTo()` ile hiç senkronize değil.
- **Kullanıcı etkisi:** Kullanıcı Stop'a basıp stream'i iptal etse bile diyalog ekranda kalıyor; sonunda Allow/Deny'e basınca artık var olmayan bir request için karar gönderiyor.

### BUG-L2: Bir API yanıtındaki `as List` cast'i, kardeş satırdaki gibi `is List` ile korunmuyor

- **Dosya:** `frontend/lib/core/api_client.dart:139-143`
- **Nedir:** `res.data['chats'] as List`, sadece `!= null` kontrolüyle korunuyor, `is List` değil — iki satır üstündeki root-list path'i `is List` kontrolü yapıp bozuk yanıtta `[]`'e düşerken bu yol doğrudan çöküyor.

### BUG-L3: Kapanma kararı, sinyal gerçekten teslim edilene kadar yeniden doğrulanmıyor

- **Dosya:** `internal/app/clients.go:94-129` (`UnregisterClient`, `sweepStaleClients`)
- **Nedir:** `shouldShutdown` kararı anlık registry durumuna bakıyor ve `selfShutdownSignal` çağrısı ile bu karar arasında dar bir zaman penceresi var.
- **Kullanıcı etkisi:** Nadir bir zamanlamada, tam o sırada `/gui` ile bağlanan yeni bir client, kapanmak üzere olan bir backend'e kaydolup hemen ardından o backend'in kapanmasıyla karşılaşabilir.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
