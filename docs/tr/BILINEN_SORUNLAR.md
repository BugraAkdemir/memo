# Bilinen Sorunlar ve Teknik Riskler

Bu belge, Memo projesindeki tüm açık hataları ve teknik riskleri takip eder.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🟡 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- 🔵 **Düşük** — kozmetik, küçük iyileştirme veya uç durum
- ⚪ **Bilgi** — tasarım notu, risk veya gözlem

---

## 🟠 Yüksek

### H13. Sağlayıcı Öncelik (Priority) Alanı Kullanılmıyor
- **Dosya:** `internal/provider/config.go`, `router.go:40-55`
- **Detay:** `ProviderConfig.Priority` alanı tanımlı ancak `getActiveEntries()` sağlayıcıları öncelik sırasına göre değil, ekleme sırasına göre (Go map iterasyonu) döndürür.

### H14. Aktif Sağlayıcı Ayarlar UI'ında Görünmüyor
- **Dosya:** `frontend/lib/widgets/settings_dialog.dart:199-281`
- **Detay:** `_ProvidersTab` sağlayıcı kartlarını gösterir ancak hangisinin aktif olduğunu belirtmez. Kullanıcı aktif sağlayıcıyı görmek için başka bir ekrana gitmek zorunda.

### H15. Frontend ApiClient'ta Agent Metodu Yok
- **Dosya:** `frontend/lib/core/api_client.dart`
- **Detay:** Backend'de agent endpoint'leri tamamen çalışır durumda ancak frontend `api_client.dart` bunları çağıracak metodlara sahip değil. Agent modu UI'dan açılıp kapatılamaz.

### H16. İndirme İlerlemesi Yoklaması Hiç Durmuyor
- **Dosya:** `frontend/lib/providers/models_provider.dart:69-81`
- **Detay:** `downloadProgressProvider` sonsuz `while (true)` döngüsüne sahip. Her 1 saniyede bir `/api/models/download/progress` vuruyor, uygulama hayatı boyunca asla durmaz. Hiç indirme yokken bile.
- **Risk:** Boş yere CPU/ağ tüketimi, dizüstü bilgisayarda pil ömrünü kısaltır.

---

## 🟡 Orta

### M15. Orkestra Yapılandırması Doğrulamasız
- **Dosya:** `internal/orchestra/conductor.go:115-120`
- **Detay:** `UpdateConfig` herhangi bir rol yapılandırmasını kabul eder. Geçersiz bir chief modeli veya eksik rol modeli, çalışma zamanında hataya neden olur.

### M16. Agent'te Araç Çağrısı Başına Zaman Aşımı Yok
- **Dosya:** `internal/agent/pipeline.go:120-150`
- **Detay:** Bireysel araç çağrılarında zaman aşımı yok. Asılı kalan bir `run_command` tüm pipeline'ı süresiz bloke eder (sandbox 60s time out'a sahip ancak pipeline bunu zorlamaz).

### M17. Agent Denetim Günlüğü 1000 Girdiyle Sınırlı
- **Dosya:** `internal/agent/executor.go:40-45`
- **Detay:** `logEntries` slice'ı 1000 ile sınırlı. Eski girdiler sessizce atılır. Döndürme veya kalıcılık yok.

### M18. 19 Boş `catch (_)` Bloğu Hataları Yutuyor
- **Dosya:** `frontend/lib/` genelinde — `providers/chat_provider.dart`, `providers/models_provider.dart`, `providers/agent_provider.dart`, `providers/whatsapp_provider.dart`, `providers/orchestra_provider.dart`, `providers/provider_provider.dart`, `widgets/chat_input.dart`, `widgets/agent/permission_dialog.dart`, `widgets/agent/agent_chat_card.dart`, `widgets/setup_wizard_view.dart`
- **Detay:** `catch (_) {}` tüm hataları sessizce yutar. Bir provider çağrısı başarısız olduğunda kullanıcı hiçbir hata mesajı görmez.
- **Risk:** Kullanıcı işlemler başarısız olduğunda geri bildirim alamaz (yapılandırma kaydetme, model listeleme, agent değiştirme vb.).

---

## ⚪ Bilgi / Gözlemler

### B1. `App.ctx` struct alanında saklanıyor (anti-pattern)
- **Dosya:** `app.go:227`
- **Not:** Go context dokümantasyonuna göre context'ler struct'ta saklanmamalı, fonksiyonlara parametre olarak geçirilmeli.

### B2. Flutter: L10n Riverpod yerine özel listener kullanıyor
- **Dosya:** `core/l10n.dart:8`
- **Not:** Dil değişimi için iki paralel bildirim sistemi.

### B3. Flutter: Hardcoded Türkçe string'ler L10n'i bypass ediyor

### B4. Flutter: `const` constructor'lar eksik (yaygın)

### B5. Provider/Agent/Orchestra için test dosyası yok
- **Dosya:** `internal/provider/`, `internal/agent/`, `internal/orchestra/`
- **Not:** Üç yeni paket için sıfır unit test (~4150 satır kod).

### B6. Eski GOB Formatı (SQLite'e taşındı)
- **Dosya:** `internal/memory/store.go`

### B7. Etkileşim Başına Tek Dosya Tasarımı
- **Dosya:** `internal/memory/store.go`

### B8. Filepath.Walk Hata Yutma
- **Dosya:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`

### B9. Gömme İstemcisi Yeniden Başlatma Sonrası Eski Referans
- **Dosya:** `app.go:148-149`, `app.go:124-125`

### B10. Dosya Adına Göre Model Otomatik Sınıflandırma
- **Dosya:** `internal/modelstore/modelstore.go:58-64`

### B11. `unsanitizePath`, Repo ID'lerindeki `__`'den `/` Enjekte Edebilir
- **Dosya:** `internal/modelstore/modelstore.go:345`

### B12. Llama Sunucusu Stderr'i Uygulama Loglarıyla Karışıyor
- **Dosya:** `internal/llama/llama.go:118-119`

---

> **Son güncelleme:** 2026-06-12
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, tüm internal/ paketleri) ve Flutter frontend
> **Açık hatalar:** 7 (🟠4, 🟡3)
> **Gözlemler:** 12
