# WhatsApp Entegrasyonu

Memo, [`whatsmeow`](https://github.com/tulir/whatsmeow) kütüphanesi aracılığıyla WhatsApp Web multi-device protokolü ile entegre olur. Çift yönlü mesajlaşma, dosya transferi ve AI destekli otomasyon sağlar.

---

## ✨ Özellikler

| Özellik | Durum |
|---------|-------|
| QR kod ile eşleştirme | ✅ |
| Kişi adı çözümleme | ✅ |
| Mesaj gönderme | ✅ |
| Mesaj alma | ✅ |
| İlk eşleştirmede mesaj geçmişi | ✅ |
| Beyaz liste tabanlı dosya transferi | ✅ |
| Agent aracı entegrasyonu | ✅ (4 araç) |
| Otomatik yeniden bağlanma | ⚠️ (test edilmedi) |
| Yeniden bağlanmada geçmiş senkronizasyonu | ❌ (sadece ilk eşleştirmede) |
| QR yoklamasının durması | ❌ (hiç durmaz) |

## 🔐 Eşleştirme Süreci

1. WhatsApp servisini backend'den başlatın (Ayarlar → WhatsApp veya API)
2. Bir QR kodu oluşturulur, `GET /api/whatsapp/qr` ile yoklanabilir
3. WhatsApp mobil uygulaması ile QR kodu okutun (Ayarlar → Bağlı Cihazlar)
4. Eşleştirme tamamlandıktan sonra oturum `data/whatsapp/` içinde saklanır

## 🤖 Agent Araçları

| Araç | Açıklama | Tehlike Seviyesi |
|------|----------|-----------------|
| `SendWhatsApp` | Bir kişiye mesaj gönder | Orta |
| `SearchWhatsApp` | Mesaj geçmişinde ara | Güvenli |
| `LatestWhatsAppChats` | Son konuşmaları al | Güvenli |
| `GetWhatsAppMessages` | Belirli bir sohbetin mesajlarını al | Orta |

## 🗂️ Veri Depolama

WhatsApp verileri `data/whatsapp/` içinde izole bir SQLite veritabanında saklanır. Bu veritabanı ana Memo hafıza deposundan ayrıdır ve `.memo` yedekleme dışa aktarımlarına dahil edilir.

## ⚠️ Bilinen Sınırlamalar

- Geçmiş senkronizasyonu sadece ilk eşleştirmede çalışır
- QR kodu yoklaması hiç durmaz (uygulama ömrü boyunca çalışır)
- Oturum yeniden bağlanma davranışı iyi test edilmemiştir
- Mesajlar yerel olarak saklanır — masaüstü tarafında E2E şifreleme sağlanmaz
