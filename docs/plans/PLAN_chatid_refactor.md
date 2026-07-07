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

### Faz 1 — sessions.Manager'ı tamamla (küçük, risksiz)

- [ ] Eksik session-scoped eşdeğerleri ekle:
  - `GetHistoryForAPIForSession(sessionID string, maxMessages int)`
  - `GetHistoryForAPITokenAwareForSession(sessionID string, maxTokens int)`
  - (varsa `UpdateMessage`/`DeleteMessage` için de session-scoped varyant)
- [ ] Global varyantları bu yenilerin `GetActiveID()` ile çağrılan sarmalayıcısı
  yap — kod tekilleşsin, davranış değişmesin.
- [ ] Tablolu unit test: aynı manager'da iki session, birine yaz, öbürünün
  history'sinin değişmediğini doğrula.
- [ ] `CGO_ENABLED=1 go test ./... -race` yeşil → commit.

### Faz 2 — chat.go: SendMessageStreamTo (asıl iş)

- [ ] `SendMessageStream`'in gövdesini `sendMessageStreamTo(ctx, chatID, userMsg)`
  özel fonksiyonuna taşı. İçeride:
  - kullanıcı mesajı: `sm.AddMessage` → `sm.AddMessageToSession(chatID, ...)`
  - history: Faz 1'deki `...ForSession(chatID, ...)` fonksiyonları
  - llm.go pipeline'ına zaten var olan `sessionID` yolunu **her zaman** doldurarak gir
    (boş-string dalı sadece geriye uyumluluk için kalır)
  - agent modu: global `a.agentEnabled` yerine
    `a.agentEnabled && sm.IsAgentChat(chatID)` ya da tamamen chatID'den türet —
    incognito/skill-command özel yolları aynı kalsın.
- [ ] Public API:
  - `SendMessageStreamTo(ctx, chatID, userMsg)` — yeni, explicit
  - `SendMessageStream(ctx, userMsg)` — `GetActiveID()` ile yenisine delege
    eden ince sarmalayıcı (HTTP sözleşmesi bozulmaz)
- [ ] Aynı işlemi `SendMessageWithImageStream` (228) ve
  `SendMessageWithFileStream` (305) için tekrarla. Non-stream `SendMessage`
  (51), `SendMessageWithImage` (393), `SendMessageWithFile` (440) —
  bunlar da aynı çekirdeğe delege edilebilir; kapsam büyürse ayrı commit.
- [ ] `-race` ile tüm testler + mevcut chat testleri yeşil → commit.

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
