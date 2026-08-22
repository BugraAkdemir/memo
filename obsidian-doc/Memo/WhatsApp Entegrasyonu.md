# WhatsApp Entegrasyonu

Memo, WhatsApp hesabına çoklu cihaz Web API'si (whatsmeow) üzerinden bağlanır. QR kod okut ve bağlan — WhatsApp Business API ücreti yok, telefon numarası kaydı yok.

## Özellikler

- **QR eşleştirme** — kodu okut, WhatsApp'ını bağla
- **Oku ve yanıtla** — mesajları gör, Memo arayüzünden yanıt gönder
- **Arama** — tüm sohbetler ve mesajlar arasında tam metin arama
- **AI asistan** — yanıt taslağı hazırlamasını veya konuşmayı özetlemesini iste
- **Ajan araçları** — ajana `whatsapp_send`, `whatsapp_search`, `whatsapp_latest`, `whatsapp_messages` araçları açık
- **Özel WhatsApp modu** — sadece WhatsApp için ayrı sohbet arayüzü
- **Profil fotoğrafları** — çekilir ve önbelleklenir, büyütmek ve indirmek için dokun
- **Otomatik yeniden bağlanma** — kopmada exponential backoff, yeniden başlatmada otomatik bağlantı
- **Anında gönderim** — iyimser arayüz, mesaj baloncukta hemen görünür
- **İsimle hitap** — "Berra'ya mesaj at" isimleri otomatik çözümler

## Güvenilirlik (v3.1.0 Cilalama)

| Özellik | Durum |
|---------|-------|
| QR polling | Adaptif: QR beklerken 2sn, bağlıyken 15sn |
| Geçmiş senk. | `INSERT OR IGNORE` — yeniden bağlanmalarda güvenli, çift kayıt yok |
| Yazma serileştirme | `sync.Mutex` `SaveMessage` + `SaveContact` üzerinde |
| Otomatik yeniden bağlanma | Exponential backoff (5sn, 10sn, 30sn, 60sn) |
| Çıkış zaman aşımı | 5sn üst sınır — yerel oturum her durumda temizlenir |
| Mesaj sıralaması | Düzeltildi: en yeni altta, en eski üstte |
| Caption'lı medya mesajları | Düzeltildi (v3.3.3): görsel/video/belge içeren canlı mesajlar, üzerlerinde bir caption varsa sessizce kaybolmuyor artık — önceden sadece düz metin mesajları okunuyordu |

## Kendine-Sohbet Asistanı (v3.9.0)

QR ile eşleştirdiğin kendi WhatsApp numarana mesaj at, Memo tam bir asistan olarak yanıtlasın — sohbet, hafıza ve agent araçları, hepsi Memo'yu açmadan telefonundaki normal WhatsApp uygulamasından erişilebilir. `POST /api/whatsapp/self-chat-assistant` ile beslenir, `internal/app/whatsapp.go` içindeki `handleWhatsAppSelfChatMessage` / `isSelfChatMessage`.

- **Sohbetten rutin oluşturma**: düz dille iste, Memo uygulama içindeki aynı `create_routine`/`list_routines`/`cancel_routine` agent araçlarını kullanarak bir rutin oluşturur, listeler ya da iptal eder — Rutinler sekmesini açmana gerek yok.
- **`/auto-perm`**: o konuşma için araç çağrısı izin sorularını otomatik-izinli yapan bir kendine-sohbet slash komutu — sohbetten tetiklenen rutin/agent eylemi geldiğinde tıklanacak bir masaüstü penceresi olmadığı için gerekli.
- **Yerelleştirilmiş yanıtlar**: kendine-sohbet komut yüzeyinin kendi küçük TR/EN metin tablosu var (`internal/app/whatsapp_l10n.go`) — dil, gelen mesajdan tespit edilir.

## Teknik

- **Kütüphane**: whatsmeow (Go, çoklu cihaz Web API)
- **Depo**: WAL modlu SQLite, yazmalarda `sync.Mutex`
- **Entegrasyon**: Mesajlar RAG hafızaya, proaktif gözlemciye, niyet çıkarıcıya ve duygu motoruna beslenir
- **Veri**: `data/whatsapp/` altında saklanır — mesajlar, kişiler, profil fotoğrafı önbelleği
