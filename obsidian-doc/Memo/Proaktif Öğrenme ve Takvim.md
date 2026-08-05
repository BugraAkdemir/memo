# Proaktif Öğrenme ve Takvim

> **Durum:** ✅ Tam (backend + mobil + masaüstü ayar) | **Sürüm:** v3.1.1 (temel) → v3.3.3'te Routines, ambient nudge banner'ı ve Self-Insight ile genişledi
>
> Memo'nun öğrenme sistemi artık sadece *ne zaman* bir şey yaptığını değil, *ne yapacağını söylediğini* de anlıyor — hem normal sohbette hem de WhatsApp konuşmalarında. v3.3.3'ten itibaren bu öğrenme, kendiliğinden gündeme gelen nudge'lara, kullanıcı tanımlı zamanlanmış Routine'lere ve isteğe bağlı bir öz-değerlendirmeye (`/insight`) dönüşüyor.

İlgili sayfalar: [[WhatsApp Entegrasyonu]] · [[Orkestra Modu]] · [[Mobil Uygulama]] · [[Backend (Go) Mimarisi]]

---

## 🔁 Routines — Zamanlanmış Otomasyonlar (v3.3.3)

> **Paket:** `internal/routine/` (extractor, loop, store) · **UI:** Yan menü (masaüstü) / ayrı sekme (mobil) → **Rutinler**

Klasik takvim sistemi kullanıcının niyetini *tespit ediyordu* (aşağıya bkz.); Routines bunun tersini yapıyor — kullanıcı Memo'ya doğrudan ne yapmasını istediğini söylüyor ve bir zamanlama veriyor:

- **Doğal dilde tanım.** "Her sabah 8'de günü özetle" gibi bir cümle yeterli; `internal/routine/extractor.go` bunu yapılandırılmış bir zamanlamaya çevirir.
- **Basit prompt ya da tam agent.** Bir routine ya arka planda tek seferlik bir LLM çağrısı olarak, ya da (kullanıcı açarsa) araç kullanan tam bir agent çalışması olarak tetiklenebilir.
- **Masaüstü + Mobil.** Mobil uygulama routine hatırlatmalarını gerçek, önceden zamanlanmış yerel bildirimler olarak teslim eder — uygulama o an açık olmasa bile ulaşır.
- **Cihaz saat dilimi.** Routine'ler, oluşturuldukları cihazın saat diliminde tetiklenir; bu offset artık her (yeniden) bağlantıda otomatik resenkronize oluyor (`POST /api/routines/sync-offset`) — seyahat veya DST değişimi bir sonraki bağlantıda kendini düzeltiyor, donmuş offset sorunu pratikte kapandı.
- **Dile duyarlı üretim.** Routine'in kendi system prompt'u, "bugün için bir şey yok" dolgu metni ve bildirim başlıkları artık her zaman Türkçe yerine uygulama dilini takip ediyor.

## 🌱 Proaktif Öğrenme: Ambient Nudge'lar (v3.3.3)

Klasik gözlem tabanlı öğrenme sistemi artık kullanıcıya kendiliğinden geri dönebiliyor:

- **Varsayılan açık** (ince seviye). Ayarlar → Genel'den tamamen kapatılabilir, ya da (v3.3.3'te yeni) Minimal Mod'un sadece belirli parçaları — persona/sistem promptu, yetenek duyuruları, pasif-özellik duyuruları, proaktif öğrenme — birbirinden bağımsız olarak yeniden açılabilir.
- Doğrudan beyan edilen bir alışkanlık ("her gece 21:00 kodluyorum") istatistiksel olarak çok oturumda birikmesini beklemeden hemen güvenilir sayılır.
- Bir nudge artık Memo'nun kendi bir cevabına doğal biçimde dokunarak ("kodlama saatin gelmedi mi?") ya da ayrı bir öneri banner'ı olarak gelebilir — gerçekten gündeme gelip gelmediği ve kabul edilip edilmediği, belirli bir dilin kesin kalıbına bağlı olmadan modelin kendisine sorularak değerlendiriliyor.
- Gizli Mod'da tamamen kapalı; Minimal Mod'da (yukarıdaki gibi özel olarak açılmadıysa) da kapalı.
- **Masaüstünde gerçek bir öneri banner'ı** (Evet / Şimdi değil / Sorma) — önceden bekleyen bir öneriyi görecek/yanıtlayacak hiçbir UI yoktu.

## 🔎 Self-Insight — `/insight` (v3.3.3)

`/insight` yazıldığında (ya da bir Routine haftalık olarak sorduğunda), Memo son ruh hali geçmişine ve hafızasına bakıp gerçek bir örüntü varsa anlatır — açıkça, yeterli veri yoksa bir örüntü **uydurmaması** talimatıyla; bu durumda sadece bunu söyler.

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
| `POST`      | `/api/routines/sync-offset`      | (v3.3.3) İstemcinin güncel `DateTime.now().timeZoneOffset`'ini gönderir, backend tüm routine'lerin `UTCOffsetMinutes`'ını buna göre günceller |

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
