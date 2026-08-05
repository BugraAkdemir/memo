# Memo Learning System — Proactive Intelligence Engine

> **Status:** Shipped — live in v3.3.3 as **Proactive Learning & Ambient Nudges**, on by default at a subtle level (Settings → General; also gated by Incognito and Minimal Mode). This document is kept as the original design doc/architecture reference — see `versinNote/v3.3.3.md` for the user-facing feature description and `§12.5 Implementation Progress` below for what actually shipped vs. this doc's original design.
> **Version:** v2.0 (design) → implemented v3.3.3
> **Related:** **Self-Insight** (`/insight`, or a weekly Routine) reuses this same mood/memory history to describe a pattern on request instead of proactively — see `versinNote/v3.3.3.md`.

---

## 1. Philosophy

Memo should not be a passive chatbot that waits for commands. It should **observe, learn, anticipate, and act** — like a real personal assistant who knows your habits, notices when they change, and adapts without being told.

### Core Principles

| Prensip | Açıklama |
|---------|----------|
| **Local-first** | Tüm öğrenme ve karar verme cihazda olur. Hiçbir veri dışarı çıkmaz |
| **Olasılıksal** | Pattern'ler binary değil, olasılık dağılımıdır. Zamanla değişir, adapte olur |
| **Unutabilen** | Eski pattern'ler decay ile ölür. Kullanıcı değişince sistem de değişir |
| **Şeffaf** | Kullanıcı tüm pattern'leri görebilir, düzeltebilir, silebilir |
| **Varsayılan kapalı** | Proactive Learning ayardan açılır. İzin yoksa sadece gözlem yapar, hiçbir şey söylemez |

---

## 2. Architecture — Orchestra-Driven Decision Engine

### The Key Insight

Proactive kararlar **hardcoded threshold'larla değil**, Orchestra Conductor/Chief ile verilir. Observer pattern'leri bulur, Proactive Engine eşleşen pattern'leri Chief'e gönderir, Chief LLM zekasıyla en doğru kararı verir.

### High-Level Data Flow

```mermaid
graph TB
    subgraph Observation["Observation Layer"]
        REC[Recorder] -->|her mesaj/session| OBS[(observations.db)]
    end

    subgraph Analysis["Analysis Layer (hourly)"]
        AN[Analyzer] -->|okur| OBS
        AN -->|zaman/sıklık/sıra analizi| PAT[(patterns.json)]
    end

    subgraph Proactive["Proactive Engine (30s)"]
        EN[Engine] -->|okur| PAT
        EN -->|saat + gün + context| MT[Matcher]
        MT -->|eşleşen pattern'ler| CHIEF[Orchestra Chief]
    end

    subgraph Decision["LLM Decision (Chief)"]
        CHIEF -->|system prompt ile karar verir| DEC[Decision: suggest/notify/auto/none]
    end

    subgraph Action["Action Layer"]
        DEC -->|suggest| CHAT[Chat Suggestion]
        DEC -->|notify| MOB[Mobile Notification]
        DEC -->|auto| AGENT[Agent Pipeline]
        DEC -->|none| END[Hiçbir şey]
    end

    subgraph Feedback["Feedback Loop"]
        FB[User Response] -->|"Evet" / "Hayır"| PAT
        FB -->|"Artık yapmıyorum"| PAT
    end

    User -->|messages| REC
    User -->|responses| FB
```

### Neden Chief?

| Yaklaşım | Sorun |
|-----------|-------|
| Hardcoded threshold (`if score > 0.5`) | Esnek değil, her kullanıcıya uymaz, yeni senaryolar için kod değişikliği gerek |
| **Orchestra Chief ile** | LLM bağlama göre karar verir. Kullanıcının ruh halini, saatini, geçmiş tepkilerini değerlendirir. Yeni entegrasyonlar için sadece Chief prompt'u güncellenir |

### Module Map

| Module | File | Responsibility |
|--------|------|---------------|
| `internal/observer/store.go` | SQLite CRUD for observations | Kaydet, sorgula, istatistik |
| `internal/observer/recorder.go` | app.go integration hook | Her mesajda/session'da kayıt |
| `internal/observer/analyzer.go` | Pattern detection engine | Zaman/sıklık/sıra analizi, confidence |
| `internal/proactive/engine.go` | Main loop goroutine | Her 30sn'de bir pattern'leri kontrol et |
| `internal/proactive/matcher.go` | Current time ↔ pattern match | Şu an hangi pattern'ler eşleşiyor? |
| `internal/proactive/prompt.go` | Chief system prompt builder | Proactive karar için prompt oluştur |
| `internal/orchestra/` | Existing — Conductor.Run() | Chief kararı verir |

---

## 3. Observation Layer

### What Gets Recorded

```go
type Observation struct {
    ID                int64     `json:"id"`
    Timestamp         time.Time `json:"timestamp"`
    DayOfWeek         int       `json:"day_of_week"`
    TimeOfDaySeconds  int       `json:"time_of_day_seconds"`
    SessionLengthMin  int       `json:"session_length_min"`
    Topic             string    `json:"topic"`
    ActivityType      string    `json:"activity_type"`
    WordCount         int       `json:"word_count"`
    Metadata          string    `json:"metadata"` // JSON blob — extensible
}
```

**Metadata extensibility:** Gelecekte Home Assistant, IoT, calendar event'leri gibi ek veriler `metadata` alanına JSON olarak yazılır. Observer'ın kendisi değişmez, sadece yeni Recorder hook'ları eklenir.

### Recording Triggers

| Trigger | Timing | Data |
|---------|--------|------|
| Message sent | User mesaj gönderdiğinde | timestamp, time_of_day, topic |
| Session start | Sohbet açıldığında | timestamp, day_of_week |
| Session end | Sohbet kapatıldığında | session_length |
| Agent mode run | Agent tool çağırdığında | activity_type="agent" |
| Orchestra run | Orchestra çalıştığında | activity_type="orchestra" |
| **Future: HA event** | Eve gelme sensörü | activity_type="home_arrival" |

---

## 4. Pattern Detection Algorithm

### 4.1 Time-Based Pattern Detection

Her aktivite tipi için saat bazlı bir **olasılık yoğunluk fonksiyonu** oluşturulur.

```go
type TimePattern struct {
    ID              string    `json:"id"`
    ActivityType    string    `json:"activity_type"`
    TimePeakSeconds int       `json:"time_peak_seconds"`
    StdDevSeconds   int       `json:"std_dev_seconds"`
    Confidence      float64   `json:"confidence"`
    DaysActive      [7]bool   `json:"days_active"`
    TotalCount      int       `json:"total_count"`
    FirstSeen       time.Time `json:"first_seen"`
    LastSeen        time.Time `json:"last_seen"`
    WeightedScore   float64   `json:"weighted_score"`
}
```

**Confidence = consistency × frequency × recency**

Consistency için dairesel istatistik (circular statistics) kullanılır — 23:59 ile 00:01 arası sorun oluşmaz.

### 4.2 Sequential & Day-of-Week Patterns

A aktivitesinden sonra B geliyor mu? Hangi günler aktif?

```
[Pzt, Sal, Çar, Per, Cum, Cmt, Paz]
[1,   1,   1,   1,   1,   0,   0 ]  → Hafta içi
```

---

## 5. Decay & Forgetting

### Sliding Window + Exponential Decay

Sadece son 30 günün verisi analiz edilir. Confidence her gün %5 azalır:

```go
func applyDecay(confidence float64, lastSeen time.Time, now time.Time) float64 {
    daysSince := now.Sub(lastSeen).Hours() / 24.0
    return confidence * math.Pow(0.95, daysSince)
}
```

30 gün sonra neredeyse silinir (`0.95^30 ≈ 0.21`). Kullanıcı "Artık yapmıyorum" derse pattern tamamen silinir.

---

## 6. Proactive Engine — Kalp Atışı

### Main Loop

```go
func (e *Engine) Start(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            e.tick(ctx)
        }
    }
}
```

### `tick()` — Her 30 Saniyede Bir

```go
func (e *Engine) tick(ctx context.Context) {
    now := time.Now()
    actives := e.store.LoadActivePatterns()

    // 1. Eşleşen pattern'leri bul
    var matches []MatchResult
    for _, p := range actives {
        score := e.matcher.Match(p, now)
        if score > 0.1 {
            matches = append(matches, MatchResult{Pattern: p, Score: score})
        }
    }
    if len(matches) == 0 {
        return
    }

    // 2. Zaten pending suggestion var mı? (tekrar gönderme)
    if e.store.HasPending() {
        return
    }

    // 3. Orchestra Chief'e sor: ne yapmalıyım?
    decision := e.askChief(ctx, matches, now)

    // 4. Kararı uygula
    e.execute(decision, matches)
}
```

### 6.1 Matching Logic

```go
func (m *Matcher) Match(pattern TimePattern, now time.Time) float64 {
    if !pattern.DaysActive[now.Weekday()] {
        return 0.0
    }
    currentSec := now.Hour()*3600 + now.Minute()*60
    diff := circularDistance(currentSec, pattern.TimePeakSeconds)
    score := gaussianPdf(diff, pattern.StdDevSeconds)
    return score * pattern.Confidence * pattern.WeightedScore
}
```

---

## 7. Orchestra Chief — Karar Verici (Proactive Loop)

Proactive Engine eşleşen pattern'leri **Orchestra Conductor'daki Chief model**'e (ana LLM) gönderir. Chief proaktif kararlar için özelleştirilmiş bir system prompt ile çalışır.

### Self-Correcting Loop

Chief'in ilk kararı her zaman doğru olmayabilir. Bu yüzden Proactive Engine bir **self-correcting loop** çalıştırır:

```
1. Chief: "Kod yazmaya başlayalım mı?" (suggest)
2. Kullanıcı: reddetti
3. Proactive Engine: Chief'e geri bildirim ver
   "Kullanıcı reddetti. Saat 21:00, pattern coding. Ne yapmalıyım?"
4. Chief: "Hiçbir şey yapma, kullanıcı meşgul. 2 saat sonra tekrar dene" (none)
5. 2 saat sonra:
   - Chief: "Saat 23:00, kullanıcı genelde bu saatte de kod yazıyor. Hatırlatayım" (notify)
6. Kullanıcı: mobile bildirim alır, "Evet" derse agent başlar
```

```go
func (e *Engine) askChief(ctx context.Context, matches []MatchResult, now time.Time,
    history []FeedbackEntry, attempt int) (Decision, error) {

    prompt := buildChiefPrompt(matches, now, history, attempt)

    // Orchestra Conductor'ı kullan — Chief model'e sor
    result, err := e.orchestra.Run(ctx, prompt)
    if err != nil {
        return Decision{}, err
    }

    decision := parseDecision(result)

    // Eğer karar "none" değilse ve attempt < 3 ise,
    // sonucu bekle ve gerekirse loop'u tekrarla
    if decision.Action != "none" && attempt < 3 {
        outcome := e.waitForOutcome(decision, 5*time.Minute)
        if outcome == OutcomeRejected || outcome == OutcomeError {
            history = append(history, FeedbackEntry{
                Decision: decision,
                Outcome:  outcome,
            })
            return e.askChief(ctx, matches, now, history, attempt+1)
        }
    }

    return decision, nil
}
```

### Chief Prompt (Proactive Mode)

```go
const ProactiveChiefPrompt = `Sen Memo'nun proaktif karar verme sistemisin.
Görevin: kullanıcının alışkanlıklarına göre ne zaman, nasıl ve hangi seviyede aksiyon alacağına karar vermek.

Girdi olarak şunları alırsın:
- Şu anki saat, gün, tarih
- Eşleşen pattern'ler (aktivite türü, güven skoru, standart sapma)
- Kullanıcının son 5 tepkisi (ne önerdin, ne dedi)
- Kullanıcının proaktivite seviyesi (off/subtle/normal/assertive)

Bu bir self-correcting loop'tur. İlk kararın yanlış olabilir:
- Kullanıcı reddederse veya hata alırsan, sistem sana tekrar soracak
- O zaman bir önceki hatadan öğrenip daha iyi bir karar vermelisin
- Max 3 deneme hakkın var

Karar tiplerin:
1. "none"       — Hiçbir şey yapma
2. "notify"     — Mobile push bildirim gönder
3. "suggest"    — Chat'e mesaj olarak yaz
4. "auto"       — Agent pipeline'ı başlat

Sadece JSON döndür:
{"decision": "suggest|notify|auto|none", "message": "kullanıcıya gösterilecek mesaj", "pattern_id": "id"}
`

```

### Neden Chief?

| Gelecekteki Entegrasyon | Değişiklik |
|------------------------|-----------|
| Home Assistant (eve gelince ışıkları aç) | Chief prompt'a "HA cihazlarını da kontrol edebilirsin" eklenir |
| IoT (kahve makinası) | Observer'a `activity_type="coffee_machine"` eklenir, Chief prompt'a "kahve saatini de öğrenebilirsin" eklenir |
| Takvim entegrasyonu | Observer takvim event'lerini de kaydeder, Chief prompt'a "toplantı öncesi hazırlık yapabilirsin" eklenir |
| Weather-aware | Observer hava durumunu da kaydeder, Chief prompt'a "yağmurlu günlerde farklı davran" eklenir |

**Hiçbir backend kodu değişmez.** Sadece Observer'a yeni kaynak eklenir (yeni recorder hook) ve Chief prompt'u güncellenir.

### Chief Prompt (Proactive Mode)

```go
const ProactiveChiefPrompt = `Sen Memo'nun proaktif karar verme sistemisin.
Görevin: kullanıcının alışkanlıklarına göre ne zaman, nasıl ve hangi seviyede aksiyon alacağına karar vermek.

Girdi olarak şunları alırsın:
- Şu anki saat, gün, tarih
- Eşleşen pattern'ler (aktivite türü, güven skoru, standart sapma)
- Kullanıcının son 5 tepkisi (ne önerdin, ne dedi)
- Kullanıcının proaktivite seviyesi (off/subtle/normal/assertive)

Karar tiplerin:
1. "none"       — Hiçbir şey yapma (eşik altı, kullanıcı meşgul)
2. "notify"     — Mobile push bildirim gönder
3. "suggest"    — Chat'e mesaj olarak yaz, kullanıcı cevaplasın
4. "auto"       — Agent pipeline'ı başlat, sonucu bildir

Kurallar:
- Kullanıcı son 5 öneriden 3'ünü reddettiyse → 1 hafta bekle
- Pattern güveni 0.85 üstü ve 30+ günlükse → "auto" düşün
- Pattern güveni 0.5-0.85 arası → "suggest" veya "notify"
- Pattern güveni 0.5 altı → "none"
- Kullanıcı "Artık yapmıyorum" dediyse → pattern'i sil
- Hafta sonu farklı karar verebilirsin (kullanıcı geç kalkıyordur)

Sadece JSON döndür:
{"decision": "suggest|notify|auto|none", "message": "kullanıcıya gösterilecek mesaj", "pattern_id": "id"}
`
```

### Neden Chief?

| Gelecekteki Entegrasyon | Değişiklik |
|------------------------|-----------|
| Home Assistant (eve gelince ışıkları aç) | Chief prompt'a "HA cihazlarını da kontrol edebilirsin" eklenir |
| IoT (kahve makinası) | Observer'a `activity_type="coffee_machine"` eklenir, Chief prompt'a "kahve saatini de öğrenebilirsin" eklenir |
| Takvim entegrasyonu | Observer takvim event'lerini de kaydeder, Chief prompt'a "toplantı öncesi hazırlık yapabilirsin" eklenir |
| Weather-aware | Observer hava durumunu da kaydeder, Chief prompt'a "yağmurlu günlerde farklı davran" eklenir |

**Hiçbir backend kodu değişmez.** Sadece Observer'a yeni kaynak eklenir (yeni recorder hook) ve Chief prompt'u güncellenir.

---

## 8. Action Types

| Action | Ne Olur |
|--------|---------|
| `none` | Hiçbir şey olmaz |
| `notify` | `/api/proactive/pending` endpoint'ine yazılır. Mobile app 30sn'de bir poll'lar. Bildirim gelir: "Kod yazma vaktin geldi ☕" |
| `suggest` | Chat'e mesaj gönderilir (kullanıcının aktif session'ı yoksa yeni session açılır) |
| `auto` | Agent pipeline tetiklenir. Background'da çalışır. Sonuç `/api/proactive/result`'a yazılır |

### Mobile Notification Flow

```
Desktop:
  Proactive Engine: "21:00 = coding match 0.89"
  → Chief: "suggest"
  → /api/proactive/pending'ye yaz: 
     {"id": "abc", "message": "Kod yazma vaktin. Başlayalım mı?", "pattern": "coding"}

Mobile (arka planda 30sn'de bir poll):
  GET /api/proactive/pending
  → {"id": "abc", "message": "Kod yazma vaktin. Başlayalım mı?"}
  → Yerel bildirim göster

Kullanıcı bildirime tıklar:
  → Mobil uygulama açılır
  → Sohbet ekranında Memo'nun mesajı gösterilir: "Kod yazma vaktin. Başlayalım mı?"
  → "Evet ✅" / "Hayır ❌" butonları

Kullanıcı "Evet" derse:
  → POST /api/proactive/respond {"id": "abc", "response": "yes"}
  → Backend: Agent pipeline başlatılır
  → Chat: "Tamam, kaldığın yerden devam ediyorum..."
```

---

## 9. Feedback Loop

| Tepki | Confidence Etkisi |
|-------|------------------|
| "Evet", "Harika" | **+0.10** |
| Yok sayar | **-0.03** |
| "Hayır", "Sonra" | **-0.15** |
| "Artık yapmıyorum" | **Pattern silinir** |

Chief ayrıca kullanıcının son 5 tepkisini de değerlendirir. Sürekli reddediyorsa 1 hafta bekleme süresine girer.

---

## 10. Cold Start

| Gün | Ne Olur |
|-----|---------|
| 1-3 | Sadece gözlem |
| 4-7 | Pattern oluşmaya başlar, sessiz |
| 8-14 | 3+ tekrar → sorabilir |
| 15-30 | Güven artar |
| 30+ | Oto-pilot'a geçebilir |

---

## 11. Privacy & Control

| Kontrol | Ne Yapar |
|---------|----------|
| `Settings > Proactive Learning` | Açar/kapatır |
| `Settings > Learning Profile` | Pattern'leri listeler |
| `"Şu pattern'i unut"` | Chat'ten siler |
| `"Tüm verilerimi sil"` | Temizler |

### Data Storage

```
data/profile/
├── observations.db    # SQLite — raw observation data
├── patterns.json      # Learned patterns with confidence scores
└── pending.json       # Pending proactive suggestions (for mobile poll)
```

---

## 12. Extensibility Membrane

Sistemin genişletilebilirliği iki noktada garanti altına alınmıştır:

```
Yeni Kaynak (örn: Home Assistant)
  → Observer'a yeni Recorder hook'u (1 dosya)
  → Chief prompt'u güncelle (1 string)
  → Tamam. Başka değişiklik yok.

Yeni Karar Mantığı
  → Chief prompt'u güncelle (1 string)
  → Tamam. Başka değişiklik yok.

Yeni Output Channel (örn: SMS, Slack)
  → Proactive Engine'e yeni executor (1 dosya)
  → Chief prompt'ta yeni action tipi belirt
  → Tamam.
```

---

## 12.5 Implementation Progress

| Faz | Kapsam | Durum |
|-----|--------|-------|
| **Faz 1** | **Observer + Storage** | ✅ **Tamamlandı** |
| **Faz 2** | **Analyzer (pattern detection)** | ✅ **Tamamlandı** |
| **Faz 3** | **Proactive Engine + Matcher** | ✅ **Tamamlandı** |
| **Faz 4** | **Chief decision loop + actions** | ✅ **Tamamlandı** |
| **Faz 5** | **Mobile poll + feedback loop** | ✅ **Tamamlandı** |

### Faz 1 — Tamamlanan İşler

- [x] `internal/observer/store.go` — SQLite `observations.db` CRUD (`Record`, `Query`, `Count`, `Prune`, `ClearAll`)
- [x] Schema + `activity_type` / `timestamp` index'leri, otomatik türetilen `day_of_week` & `time_of_day_seconds`
- [x] `internal/observer/recorder.go` — nil-safe, asenkron hook'lar (`RecordMessage`, `RecordAgentRun`, `RecordOrchestraRun`, `RecordSessionEnd`)
- [x] Yerel topic sınıflandırıcı (`ClassifyTopic` — coding/writing/research/planning/general)
- [x] `app.go` entegrasyonu: startup'ta store init, chat/agent/orchestra path'lerinde kayıt, incognito kayıt dışı, shutdown'da temiz kapanış
- [x] `internal/observer/store_test.go` — derived field, filtreleme, prune, sınıflandırma testleri (geçti)

> **Privacy notu:** Mesaj metni saklanmaz — yalnızca topic etiketi ve kelime sayısı türetilir. Incognito mesajları hiç kaydedilmez.

### Faz 2 — Tamamlanan İşler

- [x] `internal/observer/pattern.go` — `TimePattern` tipi (README §4.1), `ApplyDecay` (üstel sönüm §5), `PatternStore` (atomik `patterns.json` load/save + `LoadActive`)
- [x] `internal/observer/analyzer.go` — `AnalyzePatterns` saf çekirdeği: gözlemleri topic/aktivite anahtarına göre gruplar
- [x] **Dairesel istatistik** — saat dilimi dairede `cos/sin` ortalaması; gece yarısı sınırı (23:50 ↔ 00:10) doğru çözülür
- [x] **Confidence = consistency × frequency × recency** — circular resultant length (R), distinct-day sıklığı, son görülme sönümü
- [x] Dairesel standart sapmadan `StdDevSeconds` (10dk–6sa arası clamp), `DaysActive[7]`, `WeightedScore`, `TotalCount`
- [x] `MinObservations = 3` eşiği (cold start §10), 30 günlük kayan pencere
- [x] `app.go` zamanlayıcı: startup'tan 30sn sonra + saatlik analiz, pencere dışı gözlemleri prune ile temizleme, `ctx` ile temiz çıkış
- [x] `internal/observer/analyzer_test.go` — peak tespiti, gece yarısı sınırı, decay, gün-bazlı aktiflik, min-örnek, store round-trip (hepsi geçti)

---

## 13. Testing Strategy

| Test | Scope |
|------|-------|
| Pattern detection | Observation listesinden doğru pattern çıkıyor mu |
| Confidence calc | Farklı senaryolar |
| Decay | Zamanla confidence azalıyor mu |
| Circular stats | Gece yarısı sınırı |
| Chief prompt | LLM doğru JSON dönüyor mu |
| Feedback loop | Tepkiye göre confidence değişimi |
| Mobile poll | Pending suggestion doğru dönüyor mu |

---

### Faz 3–5 — Tamamlanan İşler

**Faz 3 — Proactive Engine + Matcher** (`internal/proactive/`)
- [x] `matcher.go` — `Match()`: dairesel mesafe + Gaussian çekirdek × confidence × weighted booster; gün aktif değilse 0
- [x] `engine.go` — 30sn'lik kalp atışı (`Start`/`tick`): aktif pattern'leri yükle, eşleştir, skor>0.1 filtrele, cooldown + tek pending kuralı
- [x] Seviye "off" iken tamamen sessiz (varsayılan) — canlı `levelFn` ile yeniden başlatmadan açılır

**Faz 4 — Chief decision loop + actions**
- [x] `prompt.go` — `ChiefSystemPrompt` + `BuildContextPrompt` (saat, eşleşmeler, son tepkiler, seviye, deneme)
- [x] `decision.go` — `Decision`/`Action`/`Level` tipleri, dengeli-parantez JSON çıkarımı (`ParseDecision`), bozuk yanıt → `none`
- [x] `engine.askChief` — enjekte edilen `Decider` ile Chief'e sorar (app `callLLM` → orchestra/provider/local)
- [x] `execute` — none/notify/suggest/auto; auto için opsiyonel `AutoRunner`

**Faz 5 — Mobile poll + feedback loop**
- [x] `pending.go` — `PendingStore` (atomik `pending.json`), TTL ile süre dolumu, `Set/Get/Respond/HasPending`
- [x] `feedback.go` — tepki→outcome eşlemesi (TR/EN), confidence delta'ları (+0.10/-0.03/-0.15), "artık yapmıyorum"→suppress
- [x] **Suppress mekanizması** — emekli edilen pattern `patterns.json`'da saklanır; analyzer yeniden öğrenmez (regression test'li)
- [x] `app_proactive.go` — `Decider`/`Emitter`/`levelFn` glue + bridge metotları; event JSON olarak yayınlanır
- [x] Webserver uçları: `/api/proactive/{settings,pending,respond,patterns,patterns/forget,clear}`
- [x] `config.ProactiveConfig` (varsayılan **off**, privacy-first), `config.Save` ile kalıcı
- [x] `proactive_test.go` — matcher, decision parse, feedback, pending yaşam döngüsü, engine uçtan-uca (suggest→accept, off→sessiz, stop→suppress)

> **Bilinen sınır:** Feedback ile yapılan confidence ince ayarı saatlik analizde davranıştan yeniden türetilir; kalıcı olan tek karar "suppress"tir. Bu tasarım gereğidir (sistem davranışı izler, sabit eşik tutmaz).
>
> **Faz 6 (v3.3.3'te tamamlandı) — Frontend entegrasyonu:** Settings → General'da Proactive Learning aç/kapa anahtarı (artık varsayılan olarak açık, subtle seviye) ve Minimal Mode'un alt-özellikler bazında bağımsız yeniden açılabilmesi; masaüstünde gerçek bir öneri banner'ı (Evet / Şimdi Değil / Sormayı Bırak); bir nudge artık Memo'nun normal bir yanıtının içine de dokunabiliyor (ayrı bir banner olmak zorunda değil) — kabul edilip edilmediği modelin kendisine sorularak değerlendiriliyor, dilden bağımsız. Gizli Mod'da tamamen kapalı.

---

> **Sonraki:** gerçek-kullanım kalibrasyonu, Home Assistant/IoT gibi ek Observer kaynakları (bkz. §12 Genişletilebilirlik)
> **Author:** Memo Team
> **Last updated:** 2026-08-05 (durum: shipped v3.3.3 — bkz. üstteki not; tasarım içeriği 2026-06-14'ten değişmedi)
