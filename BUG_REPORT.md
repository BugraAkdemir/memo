# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-12 (Session 21, devam — AGENTS.md'deki açık teknik borç maddeleri "Teknik Borç" bölümü olarak buraya taşındı)
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
| 🟠 HIGH | 1 |
| 🟡 MEDIUM | 2 |
| 🟢 LOW | 1 |
| 🔧 TEKNİK BORÇ | 3 |
| **TOPLAM** | **7** |

---

## 🟠 HIGH

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

### BUG-L5: ngrok otomatik-başlatma, restart sonrası masaüstü Flutter GUI'sini token olmadan bırakabiliyor

- **Dosya:** `internal/app/app.go:356-369` (Startup — ngrok auto-start), `frontend/lib/core/api_client.dart` (yeni `_applyRemoteToken`)
- **Nedir:** BUG-C1'in düzeltmesiyle (`f5a579e`) gelen bilinen, dar kapsamlı bir yan etki. Masaüstü Flutter istemcisi token'ı yalnızca `getRemoteAccess()`/`setRemoteAccess()` yanıtlarından, o anki oturumda öğreniyor (bellekte tutuluyor, her yeniden başlatmada sıfırlanıyor). Ama `cfg.RemoteAccess.NgrokAutoStart` açıksa, backend `Startup()` sırasında sunucuyu **doğrudan** `0.0.0.0`'a bağlıyor — Flutter GUI henüz hiçbir istek atmadan, token'ı öğrenme fırsatı bulamadan. Bu durumda GUI'nin restart sonrası ilk isteği (örn. `/api/status`) 401 ile reddedilir.
- **Kullanıcı etkisi:** ngrok otomatik-başlatma açık olan bir kullanıcı, uygulamayı her yeniden başlattığında masaüstü GUI'nin "backend'e bağlanılamıyor" göstermesiyle karşılaşabilir — Ayarlar'dan Uzak Erişim'i kapatıp tekrar açmak (yeni bir `setRemoteAccess` çağrısı, token'ı taze bir şekilde yakalar) geçici çözüm.
- **Neden şimdi düzeltilmedi:** Gerçek çözüm ya ngrok'un ilettiği trafiği loopback'ten ayırt edecek güvenilir bir sinyal gerektiriyor (ngrok'un local agent'ı zaten `127.0.0.1:port`'a bağlanıyor — kaynak IP'ye güvenmek BUG-C1'i yeniden açar), ya da Tailscale tünelinin zaten kullandığı desene geçiş (`internal/tunnel/tailscale.go` — ana sunucu hep loopback'te kalır, önüne ayrı bir reverse-proxy dinleyici konur). İkisi de canlı bir ngrok/telefon testi gerektiren, bu oturumun kapsamının dışında bir mimari değişiklik.

---

## 🔧 Teknik Borç

> AGENTS.md'nin "Known Pitfalls & Technical Debt" / "Known Open Work" bölümlerinden buraya taşındı (2026-07-12) — bug değil ama açık mimari/bakım borcu, aynı formatta takip ediliyor.

### TD-1: Chat-ID refactor — tek-global-active-chat mimarisi hâlâ kısmen duruyor

- **Dosya:** `docs/plans/PLAN_chatid_refactor.md`, `internal/app/llm.go`/`chat.go`, `internal/sessions/sessions.go`
- **Nedir:** Backend'de tek global aktif sohbet var (`sessions.Manager.active`) — tüm gönderim çağrıları bunun üzerinden yürüyor. Faz 1 (session 21'de tamamlandı) ve Faz 2'nin dar kapsamlı kısmı (chat-switch race fix, eski BUG-H1/H2 — artık düzeltilmiş, git log'da) yapıldı, ama planın asıl istediği, dışarıdan explicit bir `chatID` kabul eden public `SendMessageStreamTo(ctx, chatID, userMsg)` API'si hâlâ yok.
- **Etki:** Task loop gibi otomatik/arka plan çağrılar hâlâ `SwitchChat` + `taskloopRunMu` workaround'una muhtaç; aktif olmayan bir sohbete arka plandan mesaj gönderme yeteneği yok.
- **Sıradaki adım:** Faz 3 (task loop workaround'unu kaldırma) başlamadan önce public `SendMessageStreamTo` API'si eklenmeli.

### TD-2: `skill.DangerLevel` / `agent.DangerLevel` ayrı named type'lar

- **Dosya:** `internal/skill/` ve `internal/agent/` içindeki `DangerLevel` tanımları
- **Nedir:** İki paket kendi `DangerLevel` tipini ayrı ayrı tanımlıyor — derleme zamanında birbirine atanamıyorlar.
- **Etki:** Bug değil ama iki sistem arasında danger-level bilgisi taşınacaksa elle dönüştürme gerekiyor; ortak bir tip veya dönüştürücü yok.

### TD-3: API versioning yok

- **Dosya:** `internal/webserver/server.go` (route tanımları)
- **Nedir:** Tüm endpoint'ler düz `/api/` prefix'i altında, versiyon stratejisi (örn. `/api/v1/`) yok.
- **Etki:** Gelecekte breaking bir API değişikliği yapılmak istendiğinde eski istemcileri (mobile, eski CLI sürümleri) kırmadan geçiş yolu yok.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
