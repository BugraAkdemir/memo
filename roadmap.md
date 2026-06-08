# Memo Yol Haritası

## v3.2.0 — WhatsApp Entegrasyonu

### Amaç
Kullanıcı WhatsApp Web'e QR kod ile bağlanıp tüm mesaj geçmişini okuyabilsin, mesaj gönderebilsin, ve doğal dilde WhatsApp içeriği hakkında soru sorabilsin.

### Mimari

```
whatsmeow library (Go)
       │
       ▼
internal/whatsapp/
    ├── client.go         — whatsmeow bağlantı yönetimi (login/reconnect/pair)
    ├── qr.go             — QR kodu üretimi + Flutter'a SSE ile iletim
    ├── store.go          — Mesajları SQLite'a kaydet (ayrı veritabanı)
    ├── indexer.go        — Mesajları vektör index'ine ekle (RAG)
    ├── handler.go        — Gelen mesajları dinle + kaydet + indexle
    └── agent.go          — "whatsapp'ıma bak" → bağlam sorgulama
```

### Veri Akışı

```
Flutter UI → /api/whatsapp/pair → QR üret → kullanıcı cepten okutur
                         ↓
              whatsmeow oturum açar
                         ↓
              Gelen her mesaj → SQLite + vec0
                         ↓
              Kullanıcı sorar: "WhatsApp'ıma bak toplantı ne zamanmış"
                         ↓
              Agent: mesajları RAG'de ara → cevap üret
```

### Depolar
| Depo | Açıklama |
|------|----------|
| `data/whatsapp/messages.db` | Tüm mesajlar (SQLite, WAL) |
| `data/whatsapp/session.db` | WhatsApp oturum bilgisi (tekrar login gerekmez) |
| `data/whatsapp/contacts.db` | Rehber (isim → telefon) |
| Ana vec0 deposu | WhatsApp mesaj özetleri de memory'ye eklenebilir |

### API Uçları

| Endpoint | Method | Açıklama |
|----------|--------|----------|
| `/api/whatsapp/pair` | POST | QR kod başlat, SSE ile QR görseli gönder |
| `/api/whatsapp/status` | GET | Bağlantı durumu (bağlı/bağlı değil/taranıyor) |
| `/api/whatsapp/logout` | POST | Oturumu kapat |
| `/api/whatsapp/send` | POST | Mesaj gönder (`{to, text}`) |
| `/api/whatsapp/search` | GET | Mesajlarda ara (`?q=`) |
| `/api/whatsapp/stats` | GET | İstatistik (toplam mesaj, son 24 saat, vs) |

### Kullanıcı Akışı

1. Ayarlar → WhatsApp → "Bağla" → QR kod gösterilir
2. Kullanıcı cep telefonundan WhatsApp → Bağlı cihazlar → QR okutur
3. "✅ WhatsApp bağlandı" bildirimi
4. Geçmiş mesajlar otomatik indexlenir (ilk seferde uzun sürebilir, background)
5. Kullanıcı sohbette: "whatsapp'ıma bak kardeşim toplantı saat kaçta?"
6. Memo: WhatsApp mesajlarında RAG araması yapar, bağlamı bulur, cevap verir
7. Kullanıcı: "Ahmet'e yaz yarın akşam yemeğe çıkalım de"
8. Memo: `POST /whatsapp/send {to: "Ahmet", text: "..."}` → gönderir

### Teknik Notlar

- **Kütüphane**: `go.mau.fi/whatsmeow` (WhatsApp Web Multi-Device API)
- **QR iletimi**: SSE endpoint'inden base64 PNG olarak Flutter'a
- **Mesaj indexleme**: Her mesaj → embedding çıkar → vec0'a ekle (opsiyonel, kullanıcı etkinleştirebilir)
- **Gizlilik**: Tüm mesajlar local'de kalır, hiçbir şey dışarı çıkmaz
- **İlk sync**: İlk bağlantıda son N günlük mesaj çekilir (tüm geçmiş çok büyük olabilir)
- **Bağlantı kopması**: whatsmeow otomatik reconnect dener, session.db ile kalıcı oturum

---

## v3.3.0 — Dahili Takvim (Calendar)

### Amaç
Memo'nun kendi takvimi olsun. Kullanıcı doğal dilde olay ekleyebilsin, silebilsin, düzenleyebilsin. Memo planları otomatik takvime işleyebilsin.

### Mimari

```
internal/calendar/
    ├── store.go        — SQLite CRUD (events tablosu)
    ├── parser.go       — Doğal dil → Event yapısı (LLM + fallback regex)
    ├── reminder.go     — Bildirim/hatırlatıcı yönetimi
    └── handler.go      — "toplantımı kaydet" → bağlam çıkar + kaydet
```

### Depo
| Depo | Açıklama |
|------|----------|
| `data/calendar/events.db` | Takvim olayları (SQLite) |

### Event Yapısı
```go
type Event struct {
    ID          int64
    Title       string
    Description string
    StartTime   time.Time
    EndTime     time.Time
    AllDay      bool
    Category    string // "meeting", "task", "birthday", "reminder"
    Recurring   string // "", "daily", "weekly", "monthly", "yearly"
    CreatedBy   string // "user" veya "memo"
    CreatedAt   time.Time
}
```

### API Uçları

| Endpoint | Method | Açıklama |
|----------|--------|----------|
| `/api/calendar/events` | GET | Olayları listele (tarih aralığı filtresi) |
| `/api/calendar/events` | POST | Yeni olay ekle |
| `/api/calendar/events/:id` | PUT | Olay düzenle |
| `/api/calendar/events/:id` | DELETE | Olay sil |
| `/api/calendar/natural` | POST | Doğal dil → olay (`{"text": "yarın saat 3'te toplantı var"}`) |
| `/api/calendar/today` | GET | Bugünkü olayları getir |

### Kullanıcı Akışı

1. Kullanıcı: "haftaya pazartesi saat 10'da iş toplantısı var kaydet"
2. Memo: Doğal dil → Event → `/api/calendar/events` POST → "✅ Kaydedildi"
3. Kullanıcı panele girer, takvimi görür, düzenler/siler
4. Memo kendisi de plan yapabilir:
   - "Hatırlatıcı ayarladım, yarın saat 9'da ilaç içmeyi unutma"
   - Otomatik olay: `CreatedBy: "memo"`, kategori `reminder`

### Takvim Görünümü (Flutter)
- Aylık/genel bakış (takvim widget)
- Günlük/görünüm
- Olay ekleme/düzenleme formu
- Memo tarafından eklenen olaylar farklı renkte

---

## v4.0.0 — Uzun Vadeli (Fikir Aşaması)

| Özellik | Açıklama |
|---------|----------|
| **E-posta Entegrasyonu** | IMAP ile e-postaları oku, RAG'de ara, "email'lerime bak" |
| **Telefon Rehberi** | vCard/Contacts entegrasyonu, kişi bazlı bağlam |
| **SMTP ile e-posta gönderme** | "Ahmet'e mail at konuyu bildir" |
| **Sesli Asistan** | Wake word + STT + TTS, tamamen local |
| **Local Dosya OCR** | PDF/resim içindeki metni oku, indexle, soru sor |
| **Takvim Senkronizasyonu** | CalDAV (Nextcloud/iCloud/Google) ile çift yönlü sync |
| **Browser Eklentisi** | Web sayfalarını Memo'ya kaydet, soru sor |
| **Mobil Uygulama** | Flutter ile iOS/Android (arka plan sync) |

---

## Notlar

- WhatsApp ve Calendar modülleri birbirinden bağımsız, istenen sırada geliştirilebilir
- Her iki modül de local-first: veriler asla dışarı çıkmaz
- Flutter UI bileşenleri ayrı PR'larla eklenecek
- Öncelik sırası: WhatsApp entegrasyonu → Calendar → gerisi
- whatsmeow kütüphanesi Multi-Device API destekliyor, QR ile eşleşme yeterli
- Calendar için harici bir kütüphaneye gerek yok, in-house SQLite yeterli
