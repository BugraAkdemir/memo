# Bilinen Sorunlar ve Teknik Riskler

Tam kod tabanı denetiminden kapsamlı hata dökümü (2026-06-03). Tam detay: `docs/tr/BILINEN_SORUNLAR.md`.

**Durum**: 37 hata tespit edildi → 37'si düzeltildi ✅

---

## 🔴 Kritik (Tümü Düzeltildi)

| ID | Sorun | Çözüm |
|----|-------|-------|
| C1 | `a.syncManager` veri yarışı — eşzamanlı okuma/yazma | `getSyncManager()` Lock/RLock ile (K16) |
| C2 | `a.store` startup'ta `storeMu` olmadan atanıyor | Lock/Unlock içine alındı |
| C6 | OAuth sunucu sızıntısı + `authWg` yarışı | `authSrv` alanı, `authDone` flag (K19) |
| C9 | Eski Gob migration indeks tutarsızlığı | Önce sil, sonra indeks güncelle (K20) |
| C10 | `UpdateSyncSettings` yetim goroutine'ler | Eski manager nil yapılır (K16) |
| C11 | Flutter: `Navigator.pop()` sonrası context | ScaffoldMessenger pop öncesi alınır (K21) |
| C12 | Flutter: Context menu async context | `if (!mounted) return;` (K21) |
| C13 | Flutter: TextEditingController build'de, dispose yok | StatefulWidget + dispose (K21) |
| C14 | Flutter: setState async sonrası mounted yok | Guard eklendi (K21) |
| C15 | Flutter: FocusNode async sonrası mounted yok | Guard eklendi (K21) |

## 🟠 Yüksek (Tümü Düzeltildi)

| ID | Sorun | Çözüm |
|----|-------|-------|
| H1 | OAuth callback duplicate Done panik | `authDone` flag (K19) |
| H2 | `callLLMStream` istemci koptuktan 5dk daha çalışır | `trySend()` select ile korumalı (K11+K17) |
| H3 | Eşzamanlı AddMessage sıra karışıklığı | Per-session mutex (K22) |
| H4 | `isAuthenticated` zaman aşımı yok | 10s timeout (K19) |
| H5 | Nil embeddingClient panic | Lock altında güvenli kopya (K23) |
| H6 | Hafıza sessizce devre dışı | Store nil atanmaz, event gönderilir (K23) |
| H7-H10 | Flutter sorunları | K21 düzeltmeleri |
| H11 | Ngrok erken dönüş — config kaydedilmez | Ngrok mode kontrolü eklendi |
| H12 | Ngrok çökmesi UI'a yansımaz | `LastError()` metodu eklendi |

## 🟡 Orta (Tümü Düzeltildi)

| ID | Sorun |
|----|-------|
| M1 | `retrieveMemory` context.Background() kullanır |
| M2 | `callLLM` context.Background() kullanır |
| M3 | Path traversal Layer 1 zayıf |
| M4 | HTTP handler'larda body sınırı yok |
| M5 | Geçici dosyalar yanlış dizine yazılır |
| M6 | Increment/TriggerNow yarışı — çift yedek |
| M7 | GitHub API timeout yok |
| M8 | Zip bomb koruması yok |
| M9-M12 | Flutter: init/build/state sorunları |
| M13 | Ngrok error field API'da yok |
| M14 | Ngrok bundled yolunu kontrol etmez |

## 🔵 Düşük (Tümü Düzeltildi)

| ID | Sorun |
|----|-------|
| L1 | `saveToken` hataları sessizce yutulur |
| L2 | `writeJSON` encode hatalarını yutar |
| L3 | `config.Save` validation bildirmez |
| L4 | STT binary dünya tarafından çalıştırılabilir |
| L5 | STT süreç grubu temizlenmez |
| L6 | Alt süreçler için Setpgid (zaten düzeltilmiş) |
| L7 | Flutter const constructor eksik (widespread) |
| L8 | Flutter boş catch blokları |
| L9 | Flutter connectionStatusProvider tek sorgu |
| L10 | Flutter hardcoded Türkçe string'ler |
| L11 | Ngrok UI start butonu ve token pre-fill |
| L12 | Ngrok bağlantı durumu otomatik yenilenmez |

## ⚪ Bilgi / Gözlemler

- B1: Eski GOB formatı (SQLite'e taşındı)
- B2: Etkileşim başına tek dosya tasarımı
- B3: Filepath.Walk hata yutma
- B4: Embedding istemcisi yeniden başlatma sonrası eski referans
- B5: Dosya adına göre model otomatik sınıflandırma
- B6: `unsanitizePath` repo ID'lerinden `/` enjekte edebilir
- B7: Llama sunucusu stderr'i uygulama loglarıyla karışıyor
- B8: App struct'ında context saklanması (anti-pattern)
- B9: Flutter L10n Riverpod yerine özel listener
- B10: Flutter hardcoded Türkçe string'ler
