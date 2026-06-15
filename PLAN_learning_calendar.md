# Plan: Intent-Based Learning + Calendar System + Single Model Mode

**Kapsam:** v3.2 — stable sonrası ilk büyük özellik paketi  
**Tahmini büyüklük:** ~4000 satır Go + ~800 satır Flutter  
**Bağımlılık:** `flutter_local_notifications` (Flutter), mevcut tüm paketler yeterli (Go)

---

## Genel Mimari

```
Mesaj Kaynakları
  ├── Normal Sohbet (chat)
  └── WhatsApp (gelen + giden)
          │
          ▼
  IntentExtractor (internal/intent/)
    1. KeywordFilter (hızlı, LLM yok)
    2. LLM Parse → IntentResult
          │
    ┌─────┴──────┐
    │            │
    ▼            ▼
Calendar      Observer
(etkinlik)    (alışkanlık/beyan)
    │
    ▼
ReminderLoop → AppEvent → Flutter Local Notification
```

---

## Faz 1 — Intent Extraction Engine (`internal/intent/`)

### 1.1 Tipler (`result.go`)

```go
package intent

import "time"

type Source string
const (
    SourceChat     Source = "chat"
    SourceWhatsApp Source = "whatsapp"
)

type IntentResult struct {
    HasIntent       bool
    Summary         string     // "Yarın saat 11'de halısaha oynayacak"
    IsCalendarEvent bool
    IsHabit         bool       // "Her gün saat 21'de kod yazacağım"
    EventTitle      string     // "Halısaha"
    EventTime       *time.Time // nil = belirsiz zaman
    HabitTime       *time.Time // nil = belirsiz zaman
    HabitDays       []int      // boş = her gün
    Source          Source
    ContactName     string     // WhatsApp'tan geliyorsa karşı tarafın adı
    RawSummary      string     // LLM'den gelen ham özet (debug için)
}
```

### 1.2 Keyword Pre-Filter (`filter.go`)

LLM'i her mesajda çağırmamak için önce basit tarama. Eşleşme yoksa direkt geç.

```go
var intentKeywords = []string{
    // Zaman bildiren kelimeler
    "yarın", "bugün", "saat", "sabah", "akşam", "öğle",
    "pazartesi", "salı", "çarşamba", "perşembe", "cuma", "cumartesi", "pazar",
    "hafta", "ay", "tomorrow", "tonight", "monday", "tuesday",
    // Niyet bildiren kelimeler
    "başlayacağım", "yapacağım", "gideceğim", "olacak", "planlıyorum",
    "buluşalım", "gidelim", "yapalım", "toplantı", "randevu",
    "every day", "her gün", "düzenli",
}

func MightHaveIntent(text string) bool // hızlı substring scan
```

### 1.3 LLM Extractor (`extractor.go`)

```go
type Extractor struct {
    decide Decider // func(ctx, system, user) (string, error) — aynı interface, proactive ile paylaşılır
}

const extractorSystemPrompt = `Sen Memo'nun niyet analiz sistemisin.
Görevin: verilen mesajın bir etkinlik planı veya alışkanlık beyanı içerip içermediğini belirlemek.

Kural:
- Etkinlik: Belirli bir zamanda olacak tek seferlik bir şey ("yarın 11'de halısaha")
- Alışkanlık: Tekrarlayan niyet ("her gün saat 21'de kod yazacağım")
- Hiçbiri: Genel sohbet, soru, şikayet vb.

Sadece JSON döndür — başka hiçbir şey yazma:
{
  "has_intent": true|false,
  "is_calendar_event": true|false,
  "is_habit": true|false,
  "summary": "kullanıcının niyetinin kısa Türkçe özeti (max 100 karakter)",
  "event_title": "etkinlik başlığı veya boş string",
  "event_time_iso": "ISO 8601 veya boş string",
  "habit_time_iso": "sadece saat kısmı HH:MM veya boş string",
  "habit_days": [0,1,2,3,4,5,6] // boş = her gün, 0=Pazar
}
`

func (e *Extractor) Extract(ctx context.Context, text string, source Source, contact string, now time.Time) (IntentResult, error)
```

**Not:** `now time.Time` parametre olarak geçilir; LLM "yarın" dediğinde `now+1gün` hesaplar. Ekstraktör kendi `time.Now()` çağırmaz → test edilebilir.

### 1.4 Decider Interface

Proactive engine ile aynı `Decider` interface'i kullanır. Learning ayarındaki "single model mode" buraya da uygulanır — tek ortak factory:

```go
// internal/intent/decider.go
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
```

---

## Faz 2 — Observer Genişletmesi

### 2.1 Yeni ActivityType'lar (`internal/observer/store.go`)

```go
const (
    ActivityChat        = "chat"
    ActivityAgent       = "agent"
    ActivityOrchestra   = "orchestra"
    ActivitySession     = "session"
    ActivityIntent      = "intent"      // YENİ: beyan/niyet
    ActivityWhatsApp    = "whatsapp"    // YENİ: whatsapp kökenli
)
```

### 2.2 Metadata Şeması

`Observation.Metadata` JSON blob'una eklenenler:

```json
{
  "summary": "Yarın 11'de halısaha oynayacak",
  "source": "whatsapp",
  "contact_name": "Ahmet",
  "is_habit": false,
  "habit_time": "21:00",
  "habit_days": []
}
```

Ham mesaj metni **hiçbir zaman** saklanmaz — sadece özet.

### 2.3 Recorder Yeni Metotlar (`internal/observer/recorder.go`)

```go
// RecordIntent beyan edilen niyeti kaydeder. Alışkanlık için TimeOfDaySeconds
// beyan edilen saate (habit_time) set edilir — gerçek zamana değil.
func (r *Recorder) RecordIntent(result intent.IntentResult, now time.Time)

// RecordWhatsAppMessage gelen/giden bir WhatsApp mesajından intent kaydeder.
func (r *Recorder) RecordWhatsAppMessage(result intent.IntentResult, now time.Time)
```

---

## Faz 3 — Calendar Sistemi (`internal/calendar/`)

### 3.1 Event Tipi (`event.go`)

```go
package calendar

import "time"

type Event struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`
    StartTime    time.Time `json:"start_time"`
    EndTime      *time.Time `json:"end_time,omitempty"`
    Description  string    `json:"description,omitempty"`
    Source       string    `json:"source"`        // "chat" | "whatsapp" | "manual"
    ContactName  string    `json:"contact_name,omitempty"`
    CreatedAt    time.Time `json:"created_at"`
    ReminderSent bool      `json:"reminder_sent"`
}
```

### 3.2 Store (`store.go`)

SQLite tabanlı, `internal/database` paketini kullanır.

```
data/calendar/
└── events.db
```

```go
type Store struct { db *database.DB }

func NewStore(dir string) (*Store, error)
func (s *Store) Add(ctx context.Context, e Event) error
func (s *Store) List(ctx context.Context, from, to time.Time) ([]Event, error)
func (s *Store) Delete(ctx context.Context, id string) error
func (s *Store) MarkReminderSent(ctx context.Context, id string) error
func (s *Store) PendingReminders(ctx context.Context, before time.Time) ([]Event, error)
```

Tablo şeması:
```sql
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    start_time INTEGER NOT NULL,   -- Unix timestamp
    end_time INTEGER,
    description TEXT,
    source TEXT,
    contact_name TEXT,
    created_at INTEGER NOT NULL,
    reminder_sent INTEGER DEFAULT 0
);
CREATE INDEX idx_events_start ON events(start_time);
```

### 3.3 Reminder Loop (`reminder.go`)

```go
type ReminderConfig struct {
    LeadMinutes int // kullanıcı ayarından gelir, default 30
}

type ReminderLoop struct {
    store   *Store
    cfg     func() ReminderConfig // canlı config, restart gerektirmez
    emit    func(Event)           // AppEvent sistemine yazar
}

// Start her dakika kontrol eder. Etkinlik zamanı - LeadMinutes içindeyse:
// 1. emit() çağırır → "calendar:reminder" AppEvent
// 2. MarkReminderSent() çağırır → tekrar göndermez
func (r *ReminderLoop) Start(ctx context.Context)
```

**AppEvent formatı:**
```json
{
  "name": "calendar:reminder",
  "data": "{\"id\":\"abc\",\"title\":\"Halısaha\",\"start_time\":\"2026-06-16T11:00:00Z\",\"minutes_until\":28}"
}
```

Flutter bu event'i alınca `flutter_local_notifications` ile yerel bildirim gösterir.

### 3.4 Extractor Entegrasyonu (`extractor_bridge.go`)

```go
// AddFromIntent bir IntentResult'tan otomatik Event oluşturur ve store'a ekler.
// Dönüş: eklenen event ID (Flutter'a "eklendi" bildirimi için)
func AddFromIntent(ctx context.Context, s *Store, r intent.IntentResult) (string, error)
```

---

## Faz 4 — WhatsApp + Chat Hook

### 4.1 WhatsApp (`internal/app/whatsapp.go`)

Mevcut mesaj okuma goroutine'ine ekleme:

```go
go func() {
    for msg := range a.waClient.MessageChannel() {
        // Mevcut: store'a kaydet
        // YENİ: intent analizi
        if a.intentExtractor != nil && intent.MightHaveIntent(msg.Text) {
            go a.processMessageIntent(msg.Text, intent.SourceWhatsApp, msg.SenderName, msg.Timestamp)
        }
    }
}()
```

```go
func (a *App) processMessageIntent(text string, source intent.Source, contact string, ts time.Time) {
    result, err := a.intentExtractor.Extract(a.ctx, text, source, contact, ts)
    if err != nil || !result.HasIntent {
        return
    }
    // 1. Observer'a kaydet (alışkanlık öğrenimi)
    a.observerRecorder.RecordIntent(result, ts)
    // 2. Takvime ekle (etkinlik ise)
    if result.IsCalendarEvent && result.EventTime != nil && a.calendarStore != nil {
        id, err := calendar.AddFromIntent(a.ctx, a.calendarStore, result)
        if err == nil {
            a.emitEvent("calendar:added", id)
        }
    }
}
```

### 4.2 Chat (`internal/app/chat.go` veya `send.go`)

`SendMessage` / `Chat` fonksiyonunda kullanıcı mesajı gönderildikten sonra aynı `processMessageIntent` çağrısı.

---

## Faz 5 — API Endpoints

### Calendar Endpoints (`internal/webserver/handlers_calendar.go`)

```
GET    /api/calendar/events?from=ISO&to=ISO   → []Event
POST   /api/calendar/events                    → Event (manuel ekleme)
DELETE /api/calendar/events/:id                → 204
GET    /api/calendar/settings                  → CalendarConfig
PUT    /api/calendar/settings                  → CalendarConfig
```

### Learning Settings Endpoints (mevcut `handlers_flutter.go`'ya eklenir)

```
GET    /api/learning/settings   → LearningConfig
PUT    /api/learning/settings   → LearningConfig
```

---

## Faz 6 — Config Genişletmesi

### Yeni Config Alanları (`internal/config/config.go`)

```go
type LearningConfig struct {
    // Single model mode: Orchestra yerine tek model kullan
    SingleModelEnabled bool   `yaml:"single_model_enabled" json:"single_model_enabled"`
    // Hangi model kullanılacak (model ID / name — provider'a göre değişir)
    ModelID            string `yaml:"model_id" json:"model_id"`
}

type CalendarConfig struct {
    // Hatırlatma kaç dakika önce gitsin
    ReminderLeadMinutes int `yaml:"reminder_lead_minutes" json:"reminder_lead_minutes"`
}
```

`AppConfig`'e eklenir:

```go
type AppConfig struct {
    // ... mevcut alanlar ...
    Learning  LearningConfig  `yaml:"learning" json:"learning"`
    Calendar  CalendarConfig  `yaml:"calendar" json:"calendar"`
}
```

Default değerler:
```go
Learning: LearningConfig{SingleModelEnabled: false, ModelID: ""},
Calendar: CalendarConfig{ReminderLeadMinutes: 30},
```

---

## Faz 7 — Single Model Mode (`internal/intent/decider_factory.go`)

Hem Intent Extractor hem Proactive Engine aynı Decider interface'ini kullanır. Factory fonksiyon ikisini de besler:

```go
// BuildDecider, config'e göre Orchestra veya tek model Decider döner.
// App startup'ta ve ayar değişince çağrılır.
func BuildDecider(
    cfg config.LearningConfig,
    orchestra func(ctx context.Context, prompt string) (string, error),
    singleCall func(ctx context.Context, modelID, system, user string) (string, error),
) Decider {
    if cfg.SingleModelEnabled && cfg.ModelID != "" {
        return func(ctx context.Context, system, user string) (string, error) {
            return singleCall(ctx, cfg.ModelID, system, user)
        }
    }
    return func(ctx context.Context, system, user string) (string, error) {
        combined := system + "\n\n" + user
        return orchestra(ctx, combined)
    }
}
```

Ayar değişince (PUT /api/learning/settings):
1. Config kaydedilir
2. `BuildDecider()` yeniden çağrılır
3. `a.intentExtractor.decide` ve `a.proactiveEngine.decide` (inject edilmiş) güncellenir

---

## Faz 8 — App Struct Güncellemesi

```go
type App struct {
    // ... mevcut alanlar ...

    // Intent extraction
    intentExtractor *intent.Extractor  // YENİ

    // Calendar
    calendarStore  *calendar.Store     // YENİ
    calendarRemind *calendar.ReminderLoop // YENİ
}
```

---

## Faz 9 — Flutter Tarafı

### 9.1 Yeni Bağımlılık (`pubspec.yaml`)

```yaml
flutter_local_notifications: ^17.0.0
```

### 9.2 Yeni Dosyalar

```
mobile/lib/
├── screens/
│   └── calendar_screen.dart        # Takvim ekranı
├── models/
│   └── calendar_event.dart         # Event modeli
├── providers/
│   └── calendar_provider.dart      # Riverpod provider
└── core/
    └── notification_service.dart   # flutter_local_notifications wrapper
```

### 9.3 `calendar_screen.dart` Özellikleri

- Aylık görünüm (basit grid — harici lib gerektirmez)
- Güne tıklanınca o günün etkinlikleri listelenir
- Etkinliğe uzun basınca sil butonu
- Sağ üst: manuel etkinlik ekleme butonu

### 9.4 `notification_service.dart`

```dart
class NotificationService {
  static Future<void> init();
  static Future<void> showReminder(String title, String body);
  // SSE event "calendar:reminder" gelince çağrılır
}
```

### 9.5 `calendar_provider.dart`

```dart
// events: belirli tarih aralığındaki etkinlikler
// addEvent: POST /api/calendar/events
// deleteEvent: DELETE /api/calendar/events/:id
// calendarSettings: GET/PUT /api/calendar/settings
```

### 9.6 Navigation

`chat_screen.dart`'taki bottom nav'a takvim ikonu eklenir (veya ayrı tab).

### 9.7 SSE Event Dinleme

Mevcut SSE/polling mekanizması `calendar:reminder` ve `calendar:added` event'lerini dinler:

```dart
// calendar:added → "Halısaha eklendi" SnackBar
// calendar:reminder → flutter_local_notifications ile yerel bildirim
```

---

## Faz 10 — Learning Settings UI (Flutter)

`settings_screen.dart`'a yeni "Öğrenme" bölümü:

```
┌─ Öğrenme Sistemi ──────────────────────────┐
│                                            │
│  Tek Model Modu        [Toggle OFF→ON]     │
│                                            │
│  Model: [gpt-4o-mini              ▼]       │
│  (Proaktif kararlar ve niyet analizi       │
│   için bu model kullanılır)                │
│                                            │
│  Takvim Hatırlatma: [30 dk    ▼]           │
│  (Etkinlikten kaç dakika önce bildirim)    │
│                                            │
└────────────────────────────────────────────┘
```

---

## Dosya Listesi (Yeni + Değişen)

### Yeni Go dosyaları

```
internal/intent/
├── result.go           # IntentResult tipi
├── filter.go           # Keyword pre-filter
├── extractor.go        # LLM extraction + prompt
└── decider_factory.go  # BuildDecider() factory

internal/calendar/
├── event.go            # Event tipi
├── store.go            # SQLite CRUD
├── reminder.go         # ReminderLoop goroutine
└── extractor_bridge.go # AddFromIntent()
```

### Değişen Go dosyaları

```
internal/config/config.go           # LearningConfig, CalendarConfig ekle
internal/observer/store.go          # Yeni ActivityType sabitleri
internal/observer/recorder.go       # RecordIntent, RecordWhatsAppMessage
internal/app/app.go                 # intentExtractor, calendarStore, calendarRemind alanları
internal/app/whatsapp.go            # Intent hook (processMessageIntent)
internal/app/chat.go                # Intent hook (kullanıcı mesajında)
internal/app/proactive.go           # Decider'ı BuildDecider ile oluştur
internal/webserver/handlers_calendar.go   # YENİ: calendar API
internal/webserver/handlers_flutter.go   # learning settings endpoint ekle
internal/webserver/server.go        # Yeni route'ları kaydet
```

### Yeni Flutter dosyaları

```
mobile/lib/screens/calendar_screen.dart
mobile/lib/models/calendar_event.dart
mobile/lib/providers/calendar_provider.dart
mobile/lib/core/notification_service.dart
```

### Değişen Flutter dosyaları

```
mobile/pubspec.yaml                  # flutter_local_notifications ekle
mobile/lib/main.dart                 # NotificationService.init() çağır
mobile/lib/screens/settings_screen.dart  # Learning bölümü ekle
mobile/lib/core/api_client.dart      # Calendar endpoint'leri ekle
```

---

## Implementasyon Sırası

1. `internal/intent/` → bağımsız, test edilebilir
2. `internal/calendar/store.go` + `event.go` → bağımsız
3. Config genişletmesi → diğerleri buna bağlı
4. `calendar/reminder.go` + AppEvent entegrasyonu
5. `intent/decider_factory.go` + App struct güncellemesi
6. WhatsApp hook (`app/whatsapp.go`)
7. Chat hook (`app/chat.go`)
8. API endpoint'leri
9. Observer genişletmesi
10. Flutter: notification_service → calendar_model → provider → screen → settings UI

---

## Kritik Tasarım Kararları

| Karar | Seçilen Yaklaşım | Neden |
|-------|-----------------|-------|
| LLM çağrı maliyeti | Keyword pre-filter → LLM | Her mesajda LLM pahalı; %90 mesaj filtrede düşer |
| Ham metin saklama | Asla — sadece özet | Privacy-first felsefe korunur |
| Onay akışı | Direkt ekle + "eklendi" bildirimi | Kullanıcı isterse siler; friction azaltılır |
| Reminder mekanizması | Backend loop → SSE → Flutter local notif | Push altyapısı gerektirmez, LAN'da çalışır |
| Single model Decider | Factory pattern, inject edilmiş | Engine yeniden başlatmadan değişir |
| Calendar storage | Ayrı SQLite DB | Observer DB'sinden izole, bağımsız migrate edilebilir |

---

## Test Planı

| Paket | Test |
|-------|------|
| `intent` | Keyword filter precision/recall; LLM mock ile extract; zaman parse (yarın, saat 11, hafta sonu) |
| `calendar/store` | CRUD, zaman aralığı sorgusu, PendingReminders |
| `calendar/reminder` | Tick: LeadMinutes içindeki etkinlik emit eder; dışındaki etmez; ikinci kez atmaz |
| `observer` (güncelleme) | RecordIntent alışkanlık zamanını doğru set ediyor mu |
| Entegrasyon | WhatsApp mesajı → intent → calendar add → reminder event zinciri |

---

> **Son güncelleme:** 2026-06-15  
> **Durum:** Plan — implementasyon başlamadı
