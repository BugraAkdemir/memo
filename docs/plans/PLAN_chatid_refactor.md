# PLAN — Chat-ID Refactor: tek global "aktif sohbet" mimarisini kaldırmak

> **Kaynak:** handoff.md Session 13'te "bilinçli kapsam dışı" bırakılan iki
> mimari kısıtın gerçek çözümü. **Boyut: BÜYÜK.** Tek oturumda bitmeye
> çalışma — fazlar halinde, her faz sonunda tüm testler yeşilken commit at.
>
> **Bu plan 2026-07-06'da kod okunarak yazıldı.** Satır numaraları o günkü
> duruma göredir; önce grep'le doğrula, sonra değiştir.

## Problem

Uygulamada **tek bir global aktif sohbet** (`sessions.Manager` içinde) ve
**tek bir global agent-mode bayrağı** (`App.agentEnabled`, `app.go:160`) var.
`App.SendMessageStream(ctx, userMsg)` hangi sohbete yazacağını parametreden
değil, o anki global durumdan alıyor. Bunun iki bilinen sonucu:

1. Task loop çalışırken kullanıcı başka sohbette yazışırsa mesajlar **çapraz
   karışabilir** (loop `SwitchChat` yapınca kullanıcının aktif sohbeti kayar).
2. Birden çok görev listesi gerçekte paralel çalışamaz — `taskloopRunMu`
   (app.go) tüm loop'ları sıralı hale getiriyor; bu bir düzeltme değil,
   global duruma çarpmamak için konmuş bir emniyet kilidi.

## Mevcut durumun haritası (2026-07-06)

- `internal/sessions/sessions.go` — **iyi haber:** session-scoped API zaten
  kısmen var: `AddMessageToSession(sessionID, ...)` (satır 246),
  `GetActiveMessagesForSession(sessionID)` (satır 280), `SessionExists`,
  `IsAgentChat`. Global olanlar: `AddMessage`, `GetActiveMessages`,
  `GetActiveID`, `SwitchChat`, `GetHistoryForAPI`, `GetHistoryForAPITokenAware`.
- `internal/app/llm.go` — **iyi haber:** cevap persist eden kod zaten çift
  yollu: `sessionID != ""` ise `AddMessageToSession` (satır 828, 864),
  boşsa global `AddMessage` (satır 830, 869). Yani stream pipeline'ının alt
  yarısı sessionID taşımayı biliyor.
- `internal/app/chat.go` — **sorunlu katman:** tüm `SendMessage*` girişleri
  (satır 51, 78, 228, 305, 393, 440) kullanıcı mesajını global
  `sm.AddMessage(...)` ile yazıyor (satır 64, 187, 284, 358, 423, 468) ve
  history'yi global aktif sohbetten kuruyor.
- `internal/app/tasklist.go` — workaround burada: satır 83-99,
  `taskloopRunMu` altında `SwitchChat(chatID)` + `agentEnabled=true` zorla +
  iş bitince eskiye döndür.
- Dış çağıranlar: `internal/webserver/bridge.go` ve `handlers_flutter.go`
  (HTTP katmanı), `internal/app/tasklist.go`. Flutter ve REPL, HTTP
  üzerinden geldiği için Go imza değişikliklerinden etkilenmez — HTTP API
  sözleşmesi korunacak.

## Hedef tasarım

Çekirdek metotlar **explicit chatID** alır; "aktif sohbet" yalnızca UI'nin
bir kavramı olarak kalır (HTTP handler'ları aktif sohbeti çözüp explicit ID
geçirir). `agentEnabled` global bayrağı yerine sohbetin kendisinden türetme
(`IsAgentChat(chatID)`) esas alınır.

```go
// yeni çekirdek imza (örnek)
func (a *App) SendMessageStreamTo(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk
```

## Fazlar

### Faz 1 — sessions.Manager'ı tamamla (küçük, risksiz) ✅ tamamlandı (commit `a632873`)

- [x] Eksik session-scoped eşdeğerleri ekle:
  - `GetHistoryForAPIForSession(sessionID string, maxMessages int)`
  - `GetHistoryForAPITokenAwareForSession(sessionID string, maxTokens int)`
  - `UpdateMessage`/`DeleteMessage` için session-scoped varyant: **eklenmedi, bilinçli** — Faz 2'nin `SendMessageStreamTo`'su bunlara ihtiyaç duymuyor (sadece `AddMessageToSession` + `...ForSession` history fonksiyonlarını kullanıyor); Update/Delete zaten kullanıcının o an açık olan, tek bir sohbette elle tetiklediği düzenleme/silme aksiyonları, otomatik stream pipeline'ının parçası değil. Gerçekten ihtiyaç doğarsa (ör. task loop bir mesajı düzenlemek isterse) o zaman eklenir.
- [x] Global varyantları bu yenilerin `GetActiveID()` ile çağrılan sarmalayıcısı
  yap — kod tekilleşsin, davranış değişmesin.
- [x] Tablolu unit test: aynı manager'da iki session, birine yaz, öbürünün
  history'sinin değişmediğini doğrula (`TestSessionScopedHistory_IsolatedBetweenSessions`, `TestGetHistoryForAPI_MatchesActiveSessionVariant`).
- [x] `CGO_ENABLED=1 go test ./... -race` yeşil → commit.

### Faz 2 — chat.go: SendMessageStreamTo (asıl iş) — ⚠️ kısmen tamamlandı (commit `f00197f` backend, `d18f99e` frontend), 2026-07-12

Kullanıcı doğrudan BUG-H1/BUG-H2'yi (chat-switch race) istedi; bu ikisi
**tamamen düzeltildi ve testle doğrulandı**, ama Faz 2'nin planladığı public
API şekliyle DEĞİL — daha dar bir mekanizmayla:

- [x] `sendMessageStreamInner`/`SendMessageWithImageStream`/
  `SendMessageWithFileStream` artık her biri kendi başında
  `chatID := sm.GetActiveID()`'i **bir kez, en başta** yakalayıp
  `buildMessagesForSession(ctx, chatID, ...)`, `sm.AddMessageToSession(chatID, ...)`
  ve `routeStream(..., chatID)` boyunca aynı değeri kullanıyor — call içinde bir
  switch olursa artık history/user-mesajı/reply hep AYNI (çağrının başında
  yakalanan) sohbete gidiyor.
- [x] `buildMessages`/`getSessionHistoryTokenAware` → `buildMessagesForSession`/
  `getSessionHistoryTokenAwareForSession`'ın ince sarmalayıcısı (WhatsApp'ın
  ayrı pipeline'ı ve non-stream `SendMessage`/`SendMessageWithImage`/
  `SendMessageWithFile` hâlâ bunları kullanıyor, davranışları değişmedi).
- [x] Frontend: `MessagesNotifier.sendMessage`/`sendFile`/`refresh`, her
  `await`'ten sonra `_disposed` kontrolü yapıyor (BUG-H2) — dispose olmuş bir
  chat'in stream'i artık paylaşılan `isSendingProvider`'ı klobberlamıyor.
- [x] `-race` ile tüm backend testleri + `flutter analyze`/`flutter test`
  (99/99) yeşil → commit.
- [ ] **Eksik kalan (Faz 3 öncesi gerçek gereksinim):** Plan'ın istediği
  `SendMessageStreamTo(ctx, chatID, userMsg)` **public, dışarıdan explicit
  chatID kabul eden** API hâlâ yok — mevcut düzeltme sadece "çağrı sırasında
  aktif olan sohbeti sabitler," dışarıdan (ör. task loop'tan) *aktif olmayan*
  bir sohbete mesaj göndermeyi sağlamaz. Faz 3'ün "`SwitchChat` zorlamadan
  `a.SendMessageStreamTo(ctx, chatID, prompt)` çağır" planı bu yüzden hâlâ
  gerçekleştirilemez durumda — Faz 3'e başlamadan önce bu public API'nin asıl
  şekliyle eklenmesi gerekiyor.
- [ ] `SendMessageWithImageStream`/`SendMessageWithFileStream`'in de aynı
  şekilde **dışarıdan chatID kabul eden** public varyantları — şu an sadece
  içeride `GetActiveID()` yakalıyorlar, dışarıdan chatID parametresi almıyorlar.
- [ ] Non-stream `SendMessage`/`SendMessageWithImage`/`SendMessageWithFile` —
  hâlâ dokunulmadı, plan zaten bunu opsiyonel/ayrı commit olarak işaretlemişti.

### Faz 3 — task loop'u workaround'dan kurtar

- [ ] `internal/app/tasklist.go` `buildTaskLoopRunWorker`:
  `SwitchChat` + `agentEnabled` zorlama + geri alma bloğunu (satır ~83-99) sil,
  yerine `a.SendMessageStreamTo(ctx, chatID, prompt)` çağır.
- [ ] `taskloopRunMu`'yu **hemen silme** — önce şunu doğrula: LLM katmanında
  (`a.client`, `providerRouter`, `streamMu`) eşzamanlı iki stream güvenli mi?
  `streamMu` zaten tekilleştiriyorsa mutex'i kaldır, stream'ler sıraya kendiliğinden
  girer; değilse mutex kalır ama artık **sohbet karışması değil, sadece
  sıralılık** kısıtı olur (dokümante et).
- [ ] Session 13'ün testleri (`internal/taskloop/`) + yeni bir test: loop bir
  sohbete yazarken global aktif sohbet **değişmiyor** (GetActiveID sabit).
- [ ] `-race` yeşil → commit.

### Faz 4 — HTTP + frontend netleştirme (opsiyonel genişleme)

- [ ] `POST /api/chat/stream` (handlers_flutter.go) body'sine opsiyonel
  `chat_id` alanı ekle; verilmişse `SendMessageStreamTo`, verilmemişse eski
  davranış. Eski Flutter sürümleri kırılmaz.
- [ ] Flutter `chat_provider.dart`: istek atarken o an bağlı olduğu sohbetin
  ID'sini gönder (`activeChatIdProvider`'a "körü körüne güvenme" kuralının
  gerçek çözümü). Mobile client'a da aynı alanı ekle.
- [ ] `flutter analyze lib/` + `flutter test` yeşil → commit.

## Dokunma / dikkat

- **HTTP API sözleşmesini kırma** — eski endpoint'ler ve gövdeler aynen çalışmalı.
- `streamMu`'nun "önceki cevap bitmeden yeni mesaj yok" kullanıcı-görünür
  davranışı bilinçli; onu gevşetmek bu planın kapsamı DIŞINDA.
- WhatsApp chat yolu (`sendWhatsAppChatStream`) ayrı pipeline — bu refactor'de
  dokunulmaz.
- Orchestra modu `callLLMStream` önceliğinin en üstünde; chatID taşırken
  orchestra dalının da aynı sessionID'yi aldığını test et.

## Bitti sayılma kriteri

İki ayrı sohbet: birinde task loop koşarken diğerinde elle mesaj at →
her mesaj kendi sohbetinde kalıyor, `GetActiveID()` kullanıcının seçtiği
sohbetten hiç oynamıyor, `-race` temiz.
