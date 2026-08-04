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
- [x] **Tamamlandı 2026-07-12 (Faz 3 ile aynı oturumda):** `internal/app/chat.go`'a
  public `SendMessageStreamTo(ctx, chatID, userMsg) <-chan api.StreamChunk`
  eklendi — `sm.SessionExists(chatID)` doğrular, `sm.IsAgentChat(chatID)`'e göre
  o TEK çağrı için tool execution'ı zorlar (global `agentEnabled` bayrağına hiç
  dokunmadan — `routeStream`'e eklenen yeni `forceAgent bool` parametresiyle),
  ve session-scoped `sendMessageStreamInnerTo` üzerinden `chatID`'yi hiç
  "aktif sohbet" kavramına bakmadan uçtan uca taşır. `sendMessageStreamInner`
  (mevcut `SendMessageStream`'in çekirdeği) artık bunun ince bir sarmalayıcısı:
  `GetActiveID()`'i yakalayıp `forceAgent=false` ile aynı fonksiyonu çağırıyor
  — davranışı değişmedi.
- [ ] `SendMessageWithImageStream`/`SendMessageWithFileStream`'in de aynı
  şekilde **dışarıdan chatID kabul eden** public varyantları — şu an sadece
  içeride `GetActiveID()` yakalıyorlar, dışarıdan chatID parametresi almıyorlar.
  Task loop bunlara ihtiyaç duymuyor (sadece düz metin prompt gönderiyor), bu
  yüzden Faz 3'ün kapsamı dışında bırakıldı — ihtiyaç doğarsa eklenir.
- [ ] Non-stream `SendMessage`/`SendMessageWithImage`/`SendMessageWithFile` —
  hâlâ dokunulmadı, plan zaten bunu opsiyonel/ayrı commit olarak işaretlemişti.

### Faz 3 — task loop'u workaround'dan kurtar ✅ tamamlandı 2026-07-12

- [x] `internal/app/tasklist.go` `buildTaskLoopRunWorker`: `SwitchChat` +
  `agentEnabled` zorlama + geri alma bloğunu sildi, yerine
  `a.SendMessageStreamTo(ctx, chatID, prompt)` çağırıyor.
- [x] `taskloopRunMu` **korundu** (silinmedi) — artık "sohbet karışmasını
  önleme" değil, sadece **task-list turlarını birbirine karşı sıralama**
  amaçlı: `SendMessageStreamTo` artık hiçbir paylaşılan durumu (SwitchChat,
  global `agentEnabled`) değiştirmiyor, tek kalan paylaşılan kaynak
  `streamMu.TryLock()` (bloklamıyor, anında "lütfen bekleyin" hatasıyla
  reddediyor) — `taskloopRunMu` olmadan iki task list aynı anda koşarsa biri
  o turda boş çıktı hatası alırdı; mutex bunu düzgün sıraya sokuyor.
- [x] Yeni testler (`internal/app/chat_test.go`):
  `TestSendMessageStreamTo_UnknownChatID_ReturnsError`,
  `TestSendMessageStreamTo_TargetsGivenChatID_NotGloballyActiveChat` (asıl
  regresyon kanıtı: chatB aktifken `SendMessageStreamTo(chatA, ...)` mesajı
  chatA'nın geçmişine yazıyor, chatB'ye hiç sızmıyor, `GetActiveID()` chatB'de
  sabit kalıyor).
- [x] `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler
  yeşil. `GOOS=windows go vet ./...` → temiz.

### Faz 4 — HTTP + frontend netleştirme ✅ tamamlandı 2026-08-04

> Uzun süre "opsiyonel genişleme" sayılmıştı, ama gerçek bir kullanıcı
> raporuyla zorunlu hale geldi: GUI ve `internal/replcli` (terminal `memo`)
> aynı backend'e aynı anda bağlanabiliyor — biri chat değiştirince/yeni
> sohbet açınca diğerinin bir sonraki mesajı sessizce yanlış sohbete
> gidebiliyordu. commit `c60fab1` (backend), `4497f22` (frontend), `4460fde`
> (replcli).

- [x] `POST /api/send/stream` (handlers_flutter.go) body'sine opsiyonel
  `chat_id` alanı eklendi; verilmişse `SendMessageStreamTo`, verilmemişse
  eski davranış (`FullBridge.SendMessageStreamTo` eklendi). Eski
  istemciler kırılmadı — `chat_id` yoksa davranış birebir eskisi.
- [x] Flutter `chat_provider.dart`: `sendMessage()` zaten resolve ettiği
  `activeChatId`'yi artık `api.sendMessageStream(message, chatId: ...)`
  ile gönderiyor.
- [x] `internal/replcli`: `Client.SendStream` yeni `chatID` parametresi
  alıyor; `repl.go`'nun `sendMessage()`'ı `session.chatID`'yi, `main.go`'nun
  print-mode (`memo -p`) yolu kendi resolve ettiği `chatID`'yi geçiyor —
  ikisi de artık implicit-active'e hiç dokunmuyor.
- [x] Mobile client: yok (bu depoda mobile client'ın kendisi bulunmuyor,
  kapsam dışı bırakıldı).
- [x] Regresyon testleri: `handlers_send_stream_test.go` (backend, iki test),
  `api_client_test.dart` (frontend, üç test), `client_test.go` (replcli, iki
  test) — hepsi fix'i geri alınca gerçekten kırılıyor (runtime assertion ya
  da compile hatası), doğrulanmış.
- [x] Backend `-race` + `flutter analyze`/`flutter test` yeşil → commit
  (3 ayrı commit, backend/frontend/replcli).

## Dokunma / dikkat

- **HTTP API sözleşmesini kırma** — eski endpoint'ler ve gövdeler aynen çalışmalı.
- `streamMu`'nun "önceki cevap bitmeden yeni mesaj yok" kullanıcı-görünür
  davranışı bilinçli; onu gevşetmek bu planın kapsamı DIŞINDA.
  **2026-08-05'te yeniden gözden geçirildi:** bu not tek-client senaryolar
  (task loop vs. interaktif sohbet) düşünülerek yazılmıştı. Artık bilinen
  gerçek bir sonucu var: GUI ve `internal/replcli` aynı backend'e aynı anda
  bağlıyken (`streamMu` app-genelinde tek mutex, `chat.go`'daki her
  `TryLock()` çağrısı) biri stream halindeyken diğerinin mesajı "⏳ Lütfen
  önceki cevap tamamlanana kadar bekleyin." hatasıyla reddediliyor — farklı
  sohbetlerde olsalar bile. Kullanıcıya soruldu, **bilinçli olarak
  değiştirilmedi** (per-chat kilide çevirmek yerine mevcut davranış
  korunmasına karar verildi) — ileride tekrar gündeme gelirse bu not ve
  Faz 4'ün explicit-chatID altyapısı (`SendMessageStreamTo`) zaten per-chat
  bir kilide geçişi kolaylaştırır, sadece `streamMu`'yu `sync.Mutex`'ten
  `map[string]*sync.Mutex` (chat id başına) benzeri bir yapıya çevirmek
  yeterli olurdu.
- WhatsApp chat yolu (`sendWhatsAppChatStream`) ayrı pipeline — bu refactor'de
  dokunulmaz.
- Orchestra modu `callLLMStream` önceliğinin en üstünde; chatID taşırken
  orchestra dalının da aynı sessionID'yi aldığını test et.

## Bitti sayılma kriteri

İki ayrı sohbet: birinde task loop koşarken diğerinde elle mesaj at →
her mesaj kendi sohbetinde kalıyor, `GetActiveID()` kullanıcının seçtiği
sohbetten hiç oynamıyor, `-race` temiz.
