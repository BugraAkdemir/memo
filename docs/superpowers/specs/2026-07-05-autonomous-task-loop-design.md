# Otonom Görev Döngüsü (Task Loop) — Tasarım

**Tarih:** 2026-07-05
**Kapsam:** Backend (`internal/taskloop`, `internal/app`, `internal/webserver`) + Flutter masaüstü (`frontend/`). Mobil, aynı REST API'yi kullanan ayrı ve daha küçük bir sonraki adım — bu spec'in kapsamı dışında.

## Problem

Kullanıcı ajana çok maddeli bir görev listesi veriyor ("gece yatarken"). Ajan bugün bir maddeyi bitirince duruyor — devam etmesi için kullanıcının tekrar mesaj yazması gerekiyor. İstenen: kullanıcı uykudayken/uzaktayken Memo listedeki **tüm maddeleri kendi kendine, sırayla, kimse tetiklemeden** bitirsin.

**Revizyon (v2):** İlk taslakta işçi ajan kendi çıktısını kendi kendine "bitti" diye işaretliyordu (self-report sentinel). Kullanıcı bunu düzeltti: istenen **iki bağımsız rol** — bir CEO/Şef (chief) modeli işçinin çıktısını **bağımsız olarak** denetler, eksik/yanlış bulursa somut geri bildirimle işçiye geri gönderir. Bu, kod tabanında zaten var olan **Orchestra Mode**'un chief/worker rol atama sistemine oturuyor.

## Karar Özeti (brainstorming'de netleşenler)

| Soru | Karar |
|---|---|
| Tetikleme | Hem sohbette doğal dille yazılan liste algılanır, hem de ayrı bir "Görevler" ekranından elle liste girilebilir |
| Döngü mekaniği | **İki rol:** İşçi (worker) araç kullanarak maddeyi yapar → CEO (chief) çıktıyı bağımsız inceler, onaylar ya da eksikleri belirtip işçiye geri yollar |
| Madde başına tavan | En fazla ~5 inceleme turu (işçi dener → CEO inceler → gerekirse tekrar). Aşılırsa madde "tıkandı" sayılır, sıradakine geçilir |
| Tıkanma davranışı | Tıkanan/onaylanmayan madde atlanır, listenin geri kalanı çalışmaya devam eder |
| İzinler | Döngü başlatılırken **tek seferlik** bir onay alınır ("bu liste boyunca işçinin tüm araç çağrıları otomatik onaylanacak"); onaydan sonra gece boyu hiç izin sorusu çıkmaz |
| CEO/işçi modeli | Mevcut **Orchestra Mode** ayarlarındaki Chief ve worker rol/model ataması kullanılır — yeni bir model ayarı icat edilmiyor |
| Kapsam sırası | Bu spec: backend motoru + masaüstü arayüz. Mobil arayüz ayrı, daha küçük bir sonraki iş |

## Mevcut Altyapıda Neyi Kullanıyoruz (kod okuması ile doğrulandı)

Bu özellik sıfırdan bir "agent loop" ya da "chief/worker" icat etmiyor — iki ayrı mevcut mekanizmayı, her birini en güçlü olduğu iş için kullanarak birbirine bağlıyor:

**İşçi tarafı — araç kullanan gerçek ajan:**
1. **`agent.Pipeline.RunStream`** (`internal/agent/pipeline.go:90`) — tek bir "tur"un ne zaman bittiğini zaten belirliyor: model araç çağrısı yapmayı bırakınca (`EventFinalResponse`), 40 iterasyon sınırına takılınca, ya da context iptal olunca.
2. **`App.SendMessageStream(ctx, userMsg) <-chan api.StreamChunk`** (`internal/app/chat.go:78`) — HTTP handler'ın çağırdığı fonksiyonun ta kendisi, herhangi bir goroutine'den de çağrılabilir. İşçi rolü bunu doğrudan çağıracak.
3. **`agent.Executor.SetBypassPermissions(bool)`** (`internal/agent/executor.go:222`) — Mood "Sistem Yönetimi" modu için zaten var olan, tüm izin sorularını atlayan global bayrak. Döngü başlarken `true`, bitince eski değerine.

**CEO tarafı — bağımsız, araçsız denetleyici:**
4. **`orchestra.Conductor.createProviderForType(cfg.ChiefType, cfg.ChiefModel)`** (`internal/orchestra/conductor.go:230`) — Orchestra ayarlarında tanımlı Chief modelini/sağlayıcısını döndüren, zaten var olan fonksiyon. CEO incelemesi bu sağlayıcı üzerinden, **araçsız düz bir `provider.Message` çağrısı** olarak yapılır (`executeSingleTask`, conductor.go:474, worker çağrılarının da yaptığı gibi — Orchestra'nın kendi worker'ları zaten araçsız, sadece bu satırdaki *çağırma deseni* ödünç alınıyor; gerçek iş yine yukarıdaki agent pipeline'da oluyor).
5. **Önemli önkoşul kontrolü:** Orchestra hiç yapılandırılmamışsa (`cfg.Enabled == false` ya da Chief provider çözülemiyorsa), CEO rolü **aktif sohbet modeline** düşer (aynı model, ayrı bir "sen şimdi bir denetleyicisin" sistem promptuyla çağrılır). Böylece özellik Orchestra kurulu olmasa da çalışır, kuruluysa gerçekten bağımsız/farklı bir modelle denetler.

**Ortak altyapı:**
6. **`proactive.Engine`** (`internal/proactive/engine.go`) — canlı kullanıcı mesajı olmadan ajanı tetikleyebilen tek mevcut örnek (`AutoRunner`, satır 36-38, 98). Döngü motorumuzun goroutine yapısı bunun bir varyasyonu.
7. **`sessions.Manager`**'ın dosya-başına-JSON kayıt deseni (`internal/sessions/sessions.go`) — görev listelerini aynı şekilde `~/.memo/data/tasklists/<id>.json` olarak saklıyoruz.

**Önemli kısıtlama 1:** `SetBypassPermissions` **global** bir bayrak (tek bir paylaşılan `Executor` örneği var, sohbet başına değil). Döngü çalışırken kullanıcı elle başka bir sohbette ajanı kullanırsa, o da izin sormadan çalışır. Bu, kullanıcıya döngüyü başlatmadan önce **açıkça söylenecek**.

**Önemli kısıtlama 2:** Aynı anda **birden fazla görev listesi** çalışıyor olabilir. Bayrak global olduğu için `Engine`, açık liste sayısını bir referans sayaç (`activeCount int`) ile tutar; bayrağı sadece `activeCount` 0'dan 1'e çıkarken `true` yapar, sadece 1'den 0'a inerken eski değerine döndürür.

## Yeni Bileşenler

### 1. `internal/taskloop/store.go` — veri modeli + kalıcılık

```go
type TaskItem struct {
    ID         string
    Text       string    // kullanıcının yazdığı orijinal madde metni
    Status     string    // "pending" | "running" | "done" | "stuck" | "skipped"
    Note       string    // CEO'nun son eksik-bulma gerekçesi ya da tıkanma nedeni
    Rounds     int       // bu madde için kaç işçi→CEO inceleme turu harcandı
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

`sessions.Manager` ile birebir aynı desen: `dir` altında dosya başına bir JSON, `mu sync.RWMutex`, `Load/Save/Delete`.

### 2. `internal/taskloop/engine.go` — döngü motoru

```go
// RunWorker: işçi rolü — araç kullanarak maddeyi dener, kendi son yanıt metnini döndürür.
type RunWorker func(ctx context.Context, chatID, prompt string) (workerOutput string, err error)

// ReviewChief: CEO rolü — orijinal madde + işçinin çıktısını görür, bağımsız karar verir.
type ReviewChief func(ctx context.Context, itemText, workerOutput string) (approved bool, feedback string, err error)

type Engine struct {
    store       *Store
    runWorker   RunWorker    // App.SendMessageStream'i saran enjeksiyon (proactive.Decider ile aynı desen)
    reviewChief ReviewChief  // orchestra.Conductor'ın chief-çağırma desenini saran enjeksiyon
    mu          sync.Mutex
    activeCount int
    active      map[string]context.CancelFunc // o an çalışan liste ID'leri
}

func (e *Engine) Start(ctx context.Context, listID string) error
func (e *Engine) Stop(listID string)
```

`run()` döngüsü, her `pending` madde için:

1. Madde `running` olarak işaretlenir, `tasklist:item_started` olayı yayılır.
2. **İşçi turu:** `runWorker(ctx, list.ChatID, prompt)` çağrılır.
   - 1. tur: maddenin ham metni.
   - Sonraki turlar (CEO onaylamadıysa): madde metni + CEO'nun son geri bildirimi (*"CEO şunu eksik/yanlış buldu: `<feedback>`. Bunu düzelt."*).
3. **CEO incelemesi:** `reviewChief(ctx, item.Text, workerOutput)` çağrılır — araçsız, bağımsız bir çağrı. Chief'ten **yapılandırılmış bir yanıt** istenir (Orchestra'nın `createPlan`'da zaten yaptığı gibi JSON-in-text ayrıştırma): `{"approved": bool, "feedback": string}`.
4. `approved == true` → madde `done`, sıradaki maddeye geç.
   `approved == false` → `Rounds++`; 5 tur sınırı aşılmadıysa işçiye geri bildirimle birlikte 2. adıma dön; aşıldıysa madde `stuck` + CEO'nun son geri bildirimi not olarak, sıradaki maddeye geç.
5. Tüm maddeler bitince liste `done` olur, `tasklist:finished` yayılır.

### 3. `internal/app/tasklist.go` — App seviyesi ince sarmalayıcı

`internal/app/sessions.go` ile aynı stil: `CreateTaskList`, `StartTaskList` (izin bypass'ını açıp motoru tetikler), `StopTaskList` (bypass'ı eski haline döndürür), `ListTaskLists`, `GetTaskList`. `StartTaskList`, `reviewChief` enjeksiyonunu kurarken önce `a.orchestraConductor.Config()` üzerinden Chief provider'ı çözmeye çalışır; başarısızsa aktif sohbet modeline düşer (yukarıdaki "önkoşul kontrolü").

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
- Başlatmadan önce tek bir onay diyaloğu: *"Bu liste bitene kadar (ya da tıkanana kadar) işçinin tüm araç izinleri otomatik onaylanacak — bu arada başka açık sohbetlerdeki araç çağrıları da izin sormadan çalışır. Devam edilsin mi?"*
- Çalışırken: `frontend/lib/widgets/agent/activity_panel.dart`'taki pending/running/done/error rozet dilini yeniden kullanan bir liste görünümü. Her madde için işçi turu ile CEO'nun onay/red geri bildirimi ayrı ayrı, kısa bir "günlük" olarak gösterilir (kaç tur harcandığı dahil).
- Sohbette liste algılandığında: aynı `activity_panel` diliyle küçük bir onay banner'ı.

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
     ┌──────────────────────────────────────┴───────────────────────────────┐
     │  her pending madde için:                                             │
     │                                                                      │
     │   İşçi (agent.Pipeline, araçlı) ──── çıktı ────▶  CEO (Chief, araçsız)│
     │        ▲                                              │              │
     │        │            onaylanmadı + geri bildirim        │              │
     │        └──────────────────────────────────────────────┘              │
     │                                (max 5 tur)                           │
     │                                     │ onaylandı ya da tur doldu      │
     │                                     ▼                                │
     │                          done | stuck → sıradaki madde               │
     └──────────────────────────────────────┬───────────────────────────────┘
                                             ▼
                        tüm maddeler bitti → TaskList (done)
                        bypassPermissions eski haline döner
                                             │
                                             ▼
                    Masaüstü, poll ile ilerlemeyi zaten göstermişti;
                    sabah kullanıcı "stuck" maddelere ve CEO notlarına bakar.
```

## Hata Yönetimi

- İşçi turu context iptaliyle dönerse (backend kapanıyor) → madde `pending`'e geri alınır (yarım "running" olarak kalmaz), liste `paused`; backend yeniden açıldığında kaldığı yerden devam edebilir.
- İşçi ya da CEO çağrısı hata dönerse → o madde `stuck` + hata mesajı not olarak, döngü durmaz.
- CEO'nun yapılandırılmış yanıtı (JSON) ayrıştırılamazsa → güvenli taraf: `approved=false`, feedback="CEO yanıtı anlaşılamadı, tekrar deneniyor" — sonsuz döngüye girmemesi için bu da tur sayacına dahildir.
- Aynı anda aynı listeyi iki kere başlatma isteği → `Engine.active` map'inde zaten varsa no-op.
- Liste `running` durumdayken masaüstünden madde ekleme/silme/düzenleme **kilitlenir** (sadece görüntülenebilir). Düzenlemek isteyen önce `stop` çağırmalı.

## Test Yaklaşımı

- `internal/taskloop`: sahte `RunWorker` + `ReviewChief` enjekte edilerek — onaylandı/reddedildi/max-tur/context-iptal/CEO-parse-hatası senaryoları unit test.
- `internal/app/tasklist_test.go`: bypass-permissions'ın start'ta açılıp stop/finish'te eski haline döndüğü, Chief provider çözülemeyince aktif sohbet modeline düştüğü doğrulanır.
- `internal/webserver`: yeni endpoint'ler için httptest, mevcut `/api/chats` testleriyle aynı desen.
- Flutter: yeni ekran için widget test yok denecek kadar az altyapı var projede (mevcut desene bakılarak) — manuel `/run` doğrulaması yeterli.

## Bilinçli Olarak Kapsam Dışı Bırakılanlar (YAGNI)

- Mobil arayüz (ayrı, daha küçük bir sonraki spec).
- Sohbet-dışı bir yerden onay verme — masaüstü/mobil açık olmalı.
- Madde başına farklı izin politikası (hepsi ya da hiçbiri; ince taneli seçim yok).
- İşçi tarafına birden fazla worker rolü/model ataması (tek işçi rolü, listenin tamamı için sabit) — çoklu worker koordinasyonu Orchestra'nın kendi işi, bu özellik onu çoğaltmıyor.
- Görev listeleri arası bağımlılık/sıralama grafiği — basit, sıralı liste yeterli.
