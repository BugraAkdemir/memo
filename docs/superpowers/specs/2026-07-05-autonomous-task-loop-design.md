# Otonom Görev Döngüsü (Task Loop) — Tasarım

**Tarih:** 2026-07-05
**Kapsam:** Backend (`internal/taskloop`, `internal/app`, `internal/webserver`) + Flutter masaüstü (`frontend/`). Mobil, aynı REST API'yi kullanan ayrı ve daha küçük bir sonraki adım — bu spec'in kapsamı dışında.

## Problem

Kullanıcı ajana çok maddeli bir görev listesi veriyor ("gece yatarken"). Ajan bugün bir maddeyi bitirince duruyor — devam etmesi için kullanıcının tekrar mesaj yazması gerekiyor. İstenen: kullanıcı uykudayken/uzaktayken Memo listedeki **tüm maddeleri kendi kendine, sırayla, kimse tetiklemeden** bitirsin.

## Karar Özeti (brainstorming'de netleşenler)

| Soru | Karar |
|---|---|
| Tetikleme | Hem sohbette doğal dille yazılan liste algılanır, hem de ayrı bir "Görevler" ekranından elle liste girilebilir |
| Döngü mekaniği | Ajan turu (mevcut 40 araç-çağrısı sınırına takılınca ya da madde bitince) kendi kendine yeni bir tur başlatarak devam eder |
| Madde başına tavan | En fazla ~5 tur (≈200 araç çağrısı hakkı). Aşılırsa madde "tıkandı" sayılır, sıradakine geçilir |
| Tıkanma davranışı | Tıkanan/blocked madde atlanır, listenin geri kalanı çalışmaya devam eder |
| İzinler | Döngü başlatılırken **tek seferlik** bir onay alınır ("bu liste boyunca tüm araç çağrıları otomatik onaylanacak"); onaydan sonra gece boyu hiç izin sorusu çıkmaz |
| Kapsam sırası | Bu spec: backend motoru + masaüstü arayüz. Mobil arayüz ayrı, daha küçük bir sonraki iş |

## Mevcut Altyapıda Neyi Kullanıyoruz (kod okuması ile doğrulandı)

Bu özellik sıfırdan bir "agent loop" icat etmiyor — üç mevcut mekanizmayı birbirine bağlıyor:

1. **`agent.Pipeline.RunStream`** (`internal/agent/pipeline.go:90`) — tek bir "tur"un ne zaman bittiğini zaten belirliyor: model araç çağrısı yapmayı bırakınca (`EventFinalResponse`), 40 iterasyon sınırına takılınca, ya da context iptal olunca. Tur, tek bir HTTP isteğine bağlı yaşıyor ve kendi kendine devam edemiyor — döngü motoru bu turu **tekrar tekrar tetikleyen** dış katman olacak.
2. **`App.SendMessageStream(ctx, userMsg) <-chan api.StreamChunk`** (`internal/app/chat.go:78`) — HTTP handler'ın çağırdığı fonksiyonun ta kendisi, herhangi bir goroutine'den de çağrılabilir. Döngü motoru bunu doğrudan çağıracak; yeni bir "arka planda ajan çalıştır" mekanizması icat etmeye gerek yok.
3. **`agent.Executor.SetBypassPermissions(bool)`** (`internal/agent/executor.go:222`) — Mood "Sistem Yönetimi" modu için zaten var olan, **tüm izin sorularını atlayan** global bayrak. Döngü başlarken bunu `true` yapıp bitince eski değerine geri döndüreceğiz — yeni bir izin mekanizması icat etmiyoruz.
4. **`proactive.Engine`** (`internal/proactive/engine.go`) — canlı kullanıcı mesajı olmadan ajanı tetikleyebilen tek mevcut örnek (`AutoRunner`, satır 36-38, 98). Döngü motorumuzun goroutine/ticker yapısı bunun küçük bir varyasyonu.
5. **`sessions.Manager`**'ın dosya-başına-JSON kayıt deseni (`internal/sessions/sessions.go`) — görev listelerini de aynı şekilde `~/.memo/data/tasklists/<id>.json` olarak saklayacağız.

**Önemli kısıtlama:** `SetBypassPermissions` **global** bir bayrak (tek bir paylaşılan `Executor` örneği var, sohbet başına değil). Yani döngü çalışırken kullanıcı elle başka bir sohbette ajanı kullanırsa, o da izin sormadan çalışır. Bu, kullanıcıya döngüyü başlatmadan önce **açıkça söylenecek** ("bu süre boyunca diğer sohbetlerdeki araç çağrıları da otomatik onaylanacak").

Bununla bağlantılı ikinci bir ayrıntı: aynı anda **birden fazla görev listesi** çalışıyor olabilir (iki farklı sohbette). Bayrak global olduğu için `Engine`, açık liste sayısını bir referans sayaç (`activeCount int`) ile tutar; bayrağı sadece `activeCount` 0'dan 1'e çıkarken `true` yapar, sadece 1'den 0'a inerken eski değerine döndürür. Tek bir listenin durdurulması, hâlâ çalışan başka bir listenin iznini iptal etmez.

## Yeni Bileşenler

### 1. `internal/taskloop/store.go` — veri modeli + kalıcılık

```go
type TaskItem struct {
    ID         string
    Text       string    // kullanıcının yazdığı orijinal madde metni
    Status     string    // "pending" | "running" | "done" | "stuck" | "skipped"
    Note       string    // tıkanma nedeni ya da kısa sonuç özeti
    Turns      int       // bu madde için kaç kendi-kendine-devam turu harcandı
    StartedAt  string
    FinishedAt string
}

type TaskList struct {
    ID        string
    ChatID    string      // hangi sohbete bağlı (agent-mode + proje bağlamı oradan gelir)
    Title     string
    Items     []TaskItem
    Status    string      // "idle" | "running" | "paused" | "done"
    CreatedAt string
    UpdatedAt string
}
```

`sessions.Manager` ile birebir aynı desen: `dir` altında dosya başına bir JSON, `mu sync.RWMutex`, `Load/Save/Delete`. Yeni bir kalıcılık deseni icat edilmiyor.

### 2. `internal/taskloop/engine.go` — döngü motoru

```go
type RunTurn func(ctx context.Context, chatID, prompt string) (finalText string, err error)

type Engine struct {
    store   *Store
    runTurn RunTurn          // App.SendMessageStream'i saran, dekuple bir enjeksiyon (proactive.Decider ile aynı desen)
    mu      sync.Mutex
    active  map[string]context.CancelFunc // o an çalışan liste ID'leri
}

func (e *Engine) Start(ctx context.Context, listID string) error
func (e *Engine) Stop(listID string)
```

`run()` döngüsü, her `pending` madde için:

1. Madde `running` olarak işaretlenir, `tasklist:item_started` olayı yayılır.
2. `runTurn(ctx, list.ChatID, prompt)` çağrılır:
   - 1. tur: maddenin ham metni.
   - Sonraki turlar (madde bitmediyse): *"Önceki maddeye devam et, henüz bitirmedin."* şeklinde sentetik bir devam mesajı — asıl bağlam zaten sohbet geçmişinden geliyor, yeniden anlatmaya gerek yok.
3. Modelin **son satırı** deterministik bir sinyal içerecek şekilde, **sadece bu sentetik `runTurn` çağrısına özel** bir ek talimat gönderilir: `TASK_DONE` ya da `TASK_BLOCKED: <sebep>`. Bu talimat sohbetin kalıcı sistem promptunu değiştirmez — normal (döngü dışı) sohbet turlarını etkilemez, sadece motorun kendi turlarına eklenen tek seferlik bir ek mesajdır. Serbest metni yorumlamaya çalışmak yerine bu sabit sinyali arıyoruz — daha güvenilir.
4. `TASK_DONE` görülürse → madde `done`, sıradaki maddeye geç.
   `TASK_BLOCKED` görülürse ya da 5 tur sınırı aşılırsa → madde `stuck` + not, sıradaki maddeye geç.
5. Tüm maddeler bitince liste `done` olur, `tasklist:finished` yayılır.

### 3. `internal/app/tasklist.go` — App seviyesi ince sarmalayıcı

`internal/app/sessions.go` ile aynı stil: `CreateTaskList`, `StartTaskList` (izin bypass'ını açıp motoru tetikler), `StopTaskList` (bypass'ı eski haline döndürür), `ListTaskLists`, `GetTaskList`.

### 4. Yeni REST uç noktaları (`internal/webserver`)

Mevcut `/api/chats` ailesiyle aynı desen:

- `GET /api/tasklists` — tüm listeleri özet olarak listeler
- `POST /api/tasklists` — `{chat_id, title, items: []string}` → yeni liste oluşturur
- `GET /api/tasklists/{id}` — tek liste + madde durumları (masaüstü bunu birkaç saniyede bir poll eder)
- `POST /api/tasklists/{id}/start` — `{auto_approve: true}` (izin bypass'ını kabul ettiğini teyit eder) → döngüyü başlatır
- `POST /api/tasklists/{id}/stop` — duraklatır
- `DELETE /api/tasklists/{id}`

### 5. Sohbette liste algılama (`internal/app/chat.go`)

`SendMessageStream` içine, LLM'e gitmeden önce hafif bir regex kontrolü: mesaj ≥2 satır `1.`/`-`/`•` gibi liste kalıbıyla başlıyorsa, bunu normal bir sohbet turu olarak göndermek yerine:
1. Bekleyen bir `TaskList` oluştur (henüz başlatma).
2. `tasklist:detected` olayını yay + sohbete "N maddelik bir görev listesi gibi duruyor, gece boyu otomatik çalıştırayım mı?" diye tek satır bir onay mesajı yaz — **takvimin belirsiz-tarih onay akışıyla birebir aynı desen** (README: "Memo creates an ambiguity event you confirm with one tap").
3. Kullanıcı "evet/başlat" derse `StartTaskList` tetiklenir.

### 6. Masaüstü Flutter — "Görevler" ekranı

- Yeni bir sekme: madde ekle/düzenle + "Başlat" butonu.
- Başlatmadan önce tek bir onay diyaloğu: *"Bu liste bitene kadar (ya da tıkanana kadar) tüm araç izinleri otomatik onaylanacak — bu arada başka açık sohbetlerdeki araç çağrıları da izin sormadan çalışır. Devam edilsin mi?"*
- Çalışırken: `frontend/lib/widgets/agent/activity_panel.dart`'taki pending/running/done/error rozet dilini yeniden kullanan bir liste görünümü — ama tek bir turun geçici adımları yerine kalıcı, kullanıcının yazdığı maddeler için.
- Sohbette liste algılandığında: aynı `activity_panel` diliyle küçük bir onay banner'ı (takvimin belirsizlik onayı UI'ıyla tutarlı).

## Veri Akışı (özet)

```
Kullanıcı sohbete liste yazar  ──▶  regex algılama ──▶ TaskList (idle) + onay mesajı
                                                              │ kullanıcı onaylar
                                                              ▼
Masaüstü "Görevler" ekranından  ──▶  POST /tasklists  ──▶  TaskList (idle)
elle liste girilir                        │
                                            ▼
                                  POST /tasklists/{id}/start
                                            │
                                            ▼
                        Engine.Start: bypassPermissions=true, goroutine başlar
                                            │
                    ┌───────────────────────┴────────────────────────┐
                    │  her pending madde için:                       │
                    │  runTurn → SendMessageStream → TASK_DONE/BLOCKED│
                    │  (max 5 tur) → done|stuck → sıradaki madde      │
                    └───────────────────────┬────────────────────────┘
                                            ▼
                        tüm maddeler bitti → TaskList (done)
                        bypassPermissions eski haline döner
                                            │
                                            ▼
                    Masaüstü, poll ile ilerlemeyi zaten göstermişti;
                    sabah kullanıcı "stuck" maddelere bakar.
```

## Hata Yönetimi

- `runTurn` context iptaliyle dönerse (backend kapanıyor) → madde `pending`'e geri alınır (yarım "running" olarak kalmaz), liste `paused`; backend yeniden açıldığında kaldığı yerden devam edebilir (dosyaya her adım sonrası kayıt, `sessions.Manager` deseninde olduğu gibi).
- LLM/agent hatası dönerse → o madde `stuck` + hata mesajı not olarak, döngü durmaz.
- Aynı anda aynı listeyi iki kere başlatma isteği → `Engine.active` map'inde zaten varsa no-op.
- Liste `running` durumdayken masaüstünden madde ekleme/silme/düzenleme **kilitlenir** (sadece görüntülenebilir) — çalışırken listenin altını oymamak için. Düzenlemek isteyen önce `stop` çağırmalı.

## Test Yaklaşımı

- `internal/taskloop`: sahte bir `RunTurn` enjekte edilerek — done/blocked/max-turns/context-iptal senaryoları unit test.
- `internal/app/tasklist_test.go`: bypass-permissions'ın start'ta açılıp stop/finish'te eski haline döndüğü doğrulanır.
- `internal/webserver`: yeni endpoint'ler için httptest, mevcut `/api/chats` testleriyle aynı desen.
- Flutter: yeni ekran için widget test yok denecek kadar az altyapı var projede (mevcut desene bakılarak) — manuel `/run` doğrulaması yeterli.

## Bilinçli Olarak Kapsam Dışı Bırakılanlar (YAGNI)

- Mobil arayüz (ayrı, daha küçük bir sonraki spec).
- Sohbet-dışı bir yerden ("push notification ile onayla") onay verme — masaüstü/mobil açık olmalı.
- Madde başına farklı izin politikası (hepsi ya da hiçbiri; ince taneli seçim yok).
- Görev listeleri arası bağımlılık/sıralama grafiği (orchestra mode'daki gibi) — basit, sıralı liste yeterli.
