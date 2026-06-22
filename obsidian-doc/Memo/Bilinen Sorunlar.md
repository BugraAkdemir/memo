# Bilinen Sorunlar ve Teknik Riskler

> Güncelleme: Haziran 2026 — `AGENTS.md` ve `docs/KNOWN_ISSUES.md` v3.1.0-polish durumuyla eşleşir.

**Özet**: 14 belgelenmiş sorun, 10'u düzeltildi, 4'ü kaldı. Çoğu tasarım seviyesinde teknik borç, bug değil.

---

## 🔴 Veri Yarışları

**a.client yeniden ataması streaming sırasında** (`clientMu` var ama streaming goroutine'leri model durdurulup başlatıldığında eski referansı tutabilir). Aynı pattern `providerRouter` için de geçerli.

**Durum**: Biliniyor, tolere ediliyor. Sadece yerel çalışan bir uygulama için düşük risk. Düzgün çözüm için connection pooling gerekir.

---

## 🟠 Hafıza / Vektör Deposu

- **Her başlangıçta tam yeniden inşa** — `LoadCache` O(N), artımlı indeks yok. Kişisel ölçekli kullanım için kabul edilebilir.
- **Embedding modeli manuel başlatma gerektiriyor** — yapılandırma tabanlı otomatik başlatma var ama ayarlanması gerekiyor.

**Durum**: Tasarım tercihi. Kırık değil, sadece büyük veri kümeleri için optimize edilmemiş.

---

## 🟡 Sağlayıcı / Ajan / Orkestra

| Sorun | Durum |
|-------|-------|
| `provider.Priority` alanı var ama router tarafından kullanılmıyor | Tasarım borcu — sıralama mantığı var, bağlanmamış |
| Orkestra `provider.Router`'ı atlıyor — sağlayıcıları doğrudan oluşturuyor, yedek zincir yok | Mimari kısıtlama |
| `orchestra/` paketi için test dosyası yok (~800 satır) | Kapsam boşluğu |
| Ajan frontend arayüzü (izin diyaloğu, araç çağrı kartları) tam olarak implemente edilmedi | Kısmi — temel diyalog var, streaming olayları render ediliyor |

---

## 🟢 Flutter

- `model_store_screen.dart` 2469 satır — bileşenlere bölünmeli
- Yaygın `const` constructor eksikliği (lint uyarıları)
- `connectionStatusProvider` ve indirme ilerleme polling'i sonsuza kadar çalışıyor (otomatik durma yok)

**Yakında düzeltildi**: `settings_dialog.dart` 5013 satırdan 15 dosyaya bölündü. ✓

---

## 🔵 Diğer

| Sorun | Durum |
|-------|-------|
| `skill.DangerLevel` ve `agent.DangerLevel` ayrı tipler | Derleme zamanı tip uyuşmazlığı — birleştirilmeli |
| API sürümleme stratejisi yok | Düz `/api/` prefix'i, `/v1/`, `/v2/` yok |
| Kademeli loglama geçişi | `webserver/` `logx` kullanıyor; diğer paketler hala `log.Printf` |

---

## ✅ Yakında Düzeltilenler (v3.1.0 Cilalama)

| Sorun | Orijinal Durum | Çözüm |
|-------|---------------|-------|
| Kaynak kodda gömülü şifreleme anahtarı | Güvenlik riski | `crypto/rand` + `data/machine.key` (0600) |
| İstek body boyutu sınırı yok | DoS vektörü | Tüm handler'larda 50MB `limitBodyMiddleware` |
| Yapılandırma dosyaları herkese açık | Gizlilik riski | Tüm hassas yazmalarda `0600` izinleri |
| WhatsApp store serileştirilmemiş yazmalar | Veri bozulması riski | `sync.Mutex` `SaveMessage` + `SaveContact` üzerinde |
| Takvim çift hatırlatma | UX hatası | `ClaimPendingReminders()` atomik transaction |
| ngrok otomatik kurtarma yok | Güvenilirlik | Çökmede 5sn otomatik yeniden başlatma |
| QR polling hiç durmuyor | UX sorunu | Adaptif: QR'da 2sn, bağlıyken 15sn |
| `handleHistorySync` sadece ilk eşleşmede | Veri boşluğu | `INSERT OR IGNORE` yeniden bağlanmalarda güvenli |
| Config'te sabit `active_provider: openai` | Geçersiz kılma hatası | Boş string varsayılan olarak değiştirildi |
| CI pipeline yok | Kalite | GitHub Actions: Go + Flutter push'ta otomatik test |
