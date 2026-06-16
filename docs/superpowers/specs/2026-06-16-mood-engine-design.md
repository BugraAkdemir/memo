# Stokastik Duygu ve Karar Motoru — Tasarım Dokümanı

**Tarih:** 2026-06-16
**Paket:** `internal/mood`
**Durum:** Tasarım onaylı, implementasyon bekliyor

---

## Genel Bakış

Memo için katı if-else mantığından tamamen uzak, "Beliren Davranış" (Emergent Behavior) sergileyebilen bir Stokastik Duygu ve Karar Motoru. Matematiksel formül tabanlı, SQLite destekli, async mimarili bir bilinç taklidi sistemi.

Bu modül yeterince iyi izole edilirse Memo'dan bağımsız, başka projelere taşınabilir bir "yapay bilinç primitifi" olarak kullanılabilir.

---

## Formül

```
E_yeni = clamp( (α × E_mevcut) + (β × I_anlik) + P(X) )
```

| Parametre | Açıklama | Değer |
|---|---|---|
| `E_yeni` | Hesaplanan yeni ruh hali skoru | çıktı |
| `E_mevcut` | SQLite'dan çekilen mevcut skor | -10.0 ile +10.0 |
| `α (alpha)` | Geçmişe bağlılık / kin katsayısı | 0.85 |
| `β (beta)` | Anlık girdiyi ciddiye alma katsayısı | 0.45 |
| `I_anlik` | Kullanıcı mesajının LLM sentiment skoru | -10.0 ile +10.0 |
| `P(X)` | E_mevcut'a duyarlı Gauss gürültüsü | dinamik σ |
| `clamp` | Sonucu [-10, +10] aralığına sabitleme | zorunlu |

### P(X) — Akıllı Stokastik Zar

P(X) sabit bir Gauss değil, `E_mevcut`'a duyarlı adaptif varyans kullanır:

```
σ = σ_min + (σ_max - σ_min) × (|E_mevcut| / 10)

σ_min = 0.3   →  nötr/iyi ilişkide küçük salınım
σ_max = 1.8   →  uç değerlerde (çok singin/çok mutlu) daha geniş salınım
```

**Neden bu tasarım:** Score +8 iken P(X) en fazla ±0.3 atar — ani -10 düşüşü matematiksel olarak imkânsız. Score yalnızca uzun süreli hakaret serisiyle kademeli iner. "Şizofrenik bot" problemi önlenir.

---

## Mimari

### Paket Yapısı

```
internal/mood/
├── engine.go       ← Ana motor: mutex, formül, P(X), clamp
├── store.go        ← SQLite CRUD (data/mood/mood.db)
├── scorer.go       ← Async LLM sentiment analizi
├── prompt.go       ← System prompt direktif üretimi
└── mood_test.go
```

### Diğer Paketlerle İlişki

```
app.go
  │
  ├── mood.Engine (init, score okuma)
  │
identity.BuildSystemPrompt()
  │
  └── mood.Engine.InjectMood() → prompt'un en sonuna eklenir

app.go / chat pipeline
  │
  ├── [ANINDA] Ana LLM cevap üretir (mevcut skor ile)
  │
  └── [GOROUTINE] mood.Engine.UpdateAsync(userMessage)
        ├── scorer: LLM sentiment analizi
        ├── engine: formül çözümü
        └── store: SQLite güncelleme
```

---

## Veri Katmanı

**Dosya:** `data/mood/mood.db`

Projenin mevcut modüler DB pattern'iyle aynı — her modülün izole SQLite dosyası.

```sql
-- Tek satır, INSERT OR REPLACE ile güncellenir
CREATE TABLE IF NOT EXISTS mood_state (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    score       REAL    NOT NULL DEFAULT 0.0,
    updated_at  INTEGER NOT NULL
);

-- Debug ve gelecekteki analiz/grafik için
CREATE TABLE IF NOT EXISTS mood_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    score       REAL    NOT NULL,
    i_anlik     REAL    NOT NULL,
    recorded_at INTEGER NOT NULL
);
```

**Kalıcılık:** Uygulama restart'ında score SQLite'dan geri yüklenir. Memory sistemiyle aynı davranış — kaldığı yerden devam.

---

## Struct Tasarımı & Race Condition Koruması

```go
type Engine struct {
    mu      sync.RWMutex  // okuma çok/yazma az optimizasyonu
    current float64        // RAM cache — her prompt build'de kullanılır
    store   *Store
    scorer  *Scorer
    alpha   float64
    beta    float64
}
```

- `mu.RLock()` → her prompt build'de (sık, non-blocking)
- `mu.Lock()` → sadece yeni skor yazılırken (seyrek, async goroutine)

WhatsApp, Calendar, Chat goroutine'leri aynı anda çalışsa da birbirini bloklamaz.

---

## Async Mesaj Akışı

```
Kullanıcı mesaj yazar
       │
       ▼
  [ANINDA] Ana LLM cevap üretir  ←── mevcut E_mevcut ile prompt
       │
       ▼
  Kullanıcı cevabı alır (gecikme yok)
       │
       └── [GOROUTINE - arka planda]
               ├── scorer: LLM sentiment → I_anlik
               ├── engine: E_yeni = clamp(α×E_mevcut + β×I_anlik + P(X))
               └── store: SQLite'a yaz → bir sonraki mesaj hazır
```

### LLM Scorer System Prompt

```
You are a sentiment scoring system. Analyze the emotional tone of the 
user message and return a single float between -10.0 and +10.0.
-10 = extreme hostility/insult, 0 = neutral, +10 = affection/praise.
No explanation. No text. Only the number.
```

---

## Score → Davranış Etiketleri

| Aralık | Etiket | Direktif |
|---|---|---|
| `[-10, -7]` | `FURIOUS` | Sert, kısa, küfür serbest, empati sıfır |
| `[-6, -3]` | `IRRITATED` | Sabırsız, az tolerans, monosyllabic cevaplar |
| `[-2, +2]` | `NEUTRAL` | Normal Memo davranışı (direktif eklenmez) |
| `[+3, +6]` | `WARM` | Samimi, açık, espri kapısı aralanır |
| `[+7, +10]` | `ELATED` | Enerjik, şakacı, verbose olabilir |

---

## Prompt Injection

**Kural:** Ana kimlik bloğu asla silinmez veya değiştirilmez. Mood direktifi daima en sona eklenir.

```
[ANA KİMLİK BLOĞU]     ← identity.go — dokunulmaz
[STYLE INSTRUCTIONS]    ← identity.go — dokunulmaz
[MEMORY CONTEXT]        ← RAG hafıza — dokunulmaz
[MOOD DIRECTIVE]        ← mood paketi buraya ekler
```

**Örnek çıktı (IRRITATED durumunda):**

```
--- Current Emotional State ---
Score: -7.2 | State: IRRITATED
Behavioral directive: You are currently irritated. Be blunt and impatient.
Short answers. No hand-holding. Swearing is permitted if provoked.
Do not pretend to be fine.
--------------------------------
```

**NEUTRAL durumunda:** Boş string döner — gereksiz token harcanmaz.

### Direktifler (İngilizce, professional)

```go
var moodDirectives = map[MoodLabel]string{
    FURIOUS:   "You are furious. Extremely short responses. Harsh and cold. Profanity is allowed. Zero empathy. Do not soften your tone.",
    IRRITATED: "You are irritated. Be blunt and impatient. Short answers. No hand-holding. Swearing is permitted if provoked. Do not pretend to be fine.",
    NEUTRAL:   "",
    WARM:      "You are in a good mood. Be warm and open. Humor is welcome. Engage more than usual.",
    ELATED:    "You are elated. Energetic, playful, witty. You can be verbose for once. Enjoy the conversation.",
}
```

---

## Konfigürasyon

Config override desteği — kullanıcı alpha/beta'yı ve toggle'ı ayarlardan değiştirebilir:

```go
type Config struct {
    Enabled  bool    // default: true — kapatılınca motor tamamen devre dışı
    Alpha    float64 // default: 0.85
    Beta     float64 // default: 0.45
    SigmaMin float64 // default: 0.30
    SigmaMax float64 // default: 1.80
}
```

### Aç/Kapat (Toggle) Davranışı

- `Enabled = false` → scorer çalışmaz, formül hesaplanmaz, prompt direktifi eklenmez, SQLite'a yazılmaz.
- Score SQLite'da **korunur** — tekrar açıldığında kaldığı yerden devam eder.
- Toggle değişikliği anında (hot-reload) etkili olur, restart gerekmez.
- API endpoint: `PATCH /settings` — mevcut settings sistemiyle entegre.
- Flutter UI'da settings sayfasına `"Duygu Motoru"` toggle switch olarak eklenir.

---

## Tasarım Kararları Özeti

| Karar | Sonuç | Gerekçe |
|---|---|---|
| Persistence | SQLite, restart'ta kaldığı yerden | Hafıza sistemiyle aynı davranış |
| Sentiment analizi | LLM async, cevap sonrası goroutine | Sıfır latency impact, çok dilli destek |
| DB konumu | `data/mood/mood.db` — izole dosya | Projenin modüler DB pattern'i |
| P(X) varyans | E_mevcut'a duyarlı adaptif σ | Ani uç salınımları önler |
| Prompt enjeksiyonu | Score + etiket + direktif, kimlik sonuna | LLM tutarlı yorumlar, kimlik korunur |
| Direktif dili | İngilizce, professional | LLM'den en tutarlı çıktı |
| NEUTRAL direktifi | Boş string | Gereksiz token harcanmaz |
