# Proaktif Öğrenme ve Takvim

> **Durum:** ✅ Tam (backend + mobil + masaüstü ayar) | **Sürüm:** v3.1.1
>
> Memo'nun öğrenme sistemi artık sadece *ne zaman* bir şey yaptığını değil, *ne yapacağını söylediğini* de anlıyor — hem normal sohbette hem de WhatsApp konuşmalarında.

İlgili sayfalar: [[WhatsApp Entegrasyonu]] · [[Orkestra Modu]] · [[Mobil Uygulama]] · [[Backend (Go) Mimarisi]]

---

## 🎯 Genel Bakış

Klasik öğrenme sistemi (bkz. `docs/learning-system`) gözlem tabanlıydı: kullanıcının *davranışını* izler, dairesel istatistikle saat örüntüsü çıkarırdı. Bu sürümde üç yeni katman eklendi:

1. **Niyet Çıkarımı (Intent Extraction)** — mesajdaki planı/alışkanlığı anlama
2. **Takvim Sistemi** — tespit edilen etkinlikleri kaydetme ve hatırlatma
3. **Tek Model Modu** — öğrenme kararları için Orchestra yerine tek model

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
    ▼            ▼
Takvim        Observer
(etkinlik)    (alışkanlık/beyan)
    │
    ▼
ReminderLoop → AppEvent → Mobil Yerel Bildirim
```

---

## 🧩 Niyet Çıkarımı — `internal/intent/`

Her mesajı LLM'e göndermek pahalı olurdu. Bu yüzden iki aşamalı bir işleme kullanılır.

### Aşama 1 — Anahtar Kelime Ön Filtresi (`filter.go`)
`MightHaveIntent(text)` hızlı bir substring taraması yapar. Zaman/niyet/alışkanlık anahtar kelimeleri (yarın, saat, her gün, toplantı, "i will", "every day"…) içermeyen mesajlar **hiç LLM çağırmadan** elenir.

### Aşama 2 — LLM Çıkarımı (`extractor.go`)
Filtreyi geçen mesaj `Extractor.Extract()` ile modele gönderilir. Model sadece JSON döner:

```json
{
  "has_intent": true,
  "is_calendar_event": true,
  "is_habit": false,
  "summary": "Yarın 11:00'de halısaha oynayacak",
  "event_title": "Halısaha",
  "event_time_iso": "2026-06-16T11:00:00",
  "habit_time_hhmm": "",
  "habit_days": []
}
```

- **`now` parametresi enjekte edilir** — "yarın" gibi göreli ifadeler test edilebilir şekilde çözülür; extractor kendi `time.Now()`'unu çağırmaz.
- **String-aware JSON çıkarımı** — `extractJSON` artık string değerleri içindeki süslü parantezleri (`"yarın :)}"`) yok sayar; LLM yanıtı düz metne sarılmış olsa bile JSON ayıklanır.

### Gizlilik
**Ham mesaj metni hiçbir zaman saklanmaz.** Yalnızca kısa özet, konu ve (alışkanlıklar için) beyan edilen saat tutulur — öğrenme sisteminin gizlilik-öncelikli felsefesiyle tutarlı.

---

## 📅 Takvim Sistemi — `internal/calendar/`

| Dosya | Sorumluluk |
|-------|-----------|
| `event.go` | `Event` tipi, `ReminderPayload`, `AddedPayload` |
| `store.go` | SQLite CRUD (`data/calendar/events.db`), `PendingReminders` |
| `reminder.go` | Her dakika çalışan `ReminderLoop`, `calendar:reminder` olayı yayar |
| `bridge.go` | `AddFromIntent` — niyeti etkinliğe çevirir |

### Otomatik Ekleme
Bir etkinlik tespit edildiğinde (örn. WhatsApp'ta "yarın 11'de halısaha" deyip kabul edince) anında takvime eklenir. Kullanıcıya **eklendi bildirimi** gider; yanlışsa silebilir. Onay sorulmaz — friction azaltılır.

### Hatırlatma Döngüsü
`ReminderLoop` her dakika `PendingReminders(now, lead)` sorgular. Etkinlik, `ReminderLeadMinutes` penceresine girince:
1. `calendar:reminder` AppEvent yayınlanır (JSON payload),
2. `reminder_sent=1` işaretlenir (tekrar gönderilmez).

Lead süresi kullanıcı tarafından seçilir: **10 dk / 15 dk / 30 dk / 1 saat / 2 saat**.

### Mobil Takvim Sekmesi
Mobil uygulamada ([[Mobil Uygulama]]) yeni **Takvim** sekmesi:
- Etkinlik noktalı aylık ızgara görünümü
- Güne dokununca o günün etkinlikleri
- Manuel etkinlik ekleme
- Silmek için silme butonu
- `calendar:reminder` olayında `flutter_local_notifications` ile yerel bildirim

---

## 🤖 Tek Model Modu

Öğrenme sistemi (niyet çıkarımı + proaktif kararlar) eskiden hep [[Orkestra Modu]] üzerinden çalışırdı; Orchestra farklı işler için farklı modeller atar. Kullanıcı her şey için tek model kullanmak isterse:

- **Ayarlar → Öğrenme (Learning)** sekmesinde "Tek Model Modu" açılır ve model ID seçilir.
- `intent.BuildDecider` factory'si tek bir `Decider` üretir; bu hem `Extractor`'a hem proaktif motora enjekte edilir.
- **Anlık geçiş** — ayar değişince extractor yeniden kurulur, yeniden başlatma gerekmez (`learningMu` ile korunur).

```go
func BuildDecider(cfg config.LearningConfig, orchestra OrchestraFn, single SingleCallFn) Decider {
    if cfg.SingleModelEnabled && cfg.ModelID != "" && single != nil {
        return func(ctx, system, user) (string, error) {
            return single(ctx, cfg.ModelID, system, user)
        }
    }
    // aksi halde Orchestra'ya yönlendir
}
```

---

## 🔌 API Uçları

| Metot       | Uç                               | Açıklama                       |
| ----------- | -------------------------------- | ------------------------------ |
| `GET`       | `/api/calendar/events?from=&to=` | Zaman aralığındaki etkinlikler |
| `POST`      | `/api/calendar/events`           | Manuel etkinlik ekle           |
| `DELETE`    | `/api/calendar/events/{id}`      | Etkinlik sil                   |
| `GET`/`PUT` | `/api/calendar/settings`         | Hatırlatma süresi (dakika)     |
| `GET`/`PUT` | `/api/learning/settings`         | Tek model modu + model ID      |

Bkz. [[API Dökümantasyonu]].

---

## 🗄️ Veri Akışı ve Entegrasyon

| Katman | Dosya | Ne yapar |
|--------|-------|----------|
| WhatsApp | `internal/app/whatsapp.go` | `runWhatsAppIntentLoop` — gelen/giden her mesajı işler |
| Sohbet | `internal/app/chat.go` | Kullanıcı mesajında `processMessageIntent` |
| Glue | `internal/app/learning.go` | init, decider factory, takvim köprü metotları |
| Gözlem | `internal/observer/recorder.go` | `RecordIntent`, `RecordWhatsAppMessage` |

> **Önemli tasarım kararı:** Alışkanlık için beyan edilen saat (örn. "her gün 21:00"), mesajın gönderildiği saate değil, **niyetteki saate** kaydedilir. `Store.Record` zaman alanlarını timestamp'ten türettiği için, `RecordIntent` habit saatini doğrudan timestamp'e gömer — böylece analizör doğru örüntüyü öğrenir.

---

## 🧪 Test Kapsamı

| Paket | Test |
|-------|------|
| `intent` | Keyword filter precision, extract (etkinlik/alışkanlık), string-aware JSON, prose içinde JSON |
| `calendar` | CRUD, zaman aralığı, `PendingReminders`, reminder loop emit + tek seferlik |
| `observer` | `RecordIntent` habit saatini koruyor mu, metadata round-trip |

Tümü `-race` ile geçer.

---

## 🔭 Sonraki Adımlar (Vizyon)

- WhatsApp'tan öğrenilen niyetlerle doğal proaktif sohbet ("kanka spora başladın mı?")
- Tüm sosyal medya kaynaklarının aynı niyet hattına bağlanması
- Takvim ↔ proaktif motor entegrasyonu (etkinlik öncesi hazırlık önerisi)
