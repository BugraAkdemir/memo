# Handoff — 2026-06-26

## Oturum Özeti

Bu oturumda Faz F (F1 + F2) tamamlandı. Değişiklikler henüz commit edilmedi.

---

## Yapılan Değişiklikler

### Faz F — Mobil Frontend Parity

**F1 — Mobil API client default URL'ini kaldır / discovery ekle**

- `mobile/lib/core/api_client.dart`: `MemoApiClient` default URL `''` (boş) oldu (önceki oturumda yapılmıştı, bu oturumda doğrulandı).
- `mobile/lib/providers/connection_provider.dart`:
  - `dart:async`, `dart:io`, `dio` import eklendi.
  - `ConnectionState.baseUrl` default `'http://192.168.1.100:8090'` → `''`.
  - `ConnectionState.discovering: bool` field eklendi.
  - `ConnectionNotifier.discoverUrl()` — `NetworkInterface.list()` ile cihazın IPv4 subnet'ini bulup host 1-254 arasında port 8090'e paralel istek atıyor. İlk yanıt veren URL'yi döndürüyor, 10s timeout.
  - `_scanLocalNetwork()` private helper eklendi.
- `mobile/lib/screens/connect_screen.dart`:
  - `_lanFields()` metoduna "TARA" butonu eklendi. Tararken `CircularProgressIndicator` gösteriyor.
  - `_scan()` async metodu eklendi — `discoverUrl()` çağırır, bulunan URL'yi `_urlCtrl.text`'e yazar.

**F2 — Mobil client'a eksik backend endpointlerini ekle**

- `mobile/lib/core/api_client.dart` — yeni endpoint grupları eklendi:
  - **Chat extras**: `renameChat`, `getActiveChatId`, `generateTitle`, `exportChat`, `deleteMessage`, `updateMessage`, `createAgentChat`
  - **Memory**: `getMemoryEnabled`, `setMemoryEnabled`, `getMemorySettings`, `updateMemorySettings`, `saveExplicitMemory`, `deleteExplicitMemory`, `exportMemories`, `importMemories`, `getMemoryStats`, `debugMemorySearch`
  - **System prompt / incognito**: `getSystemPrompt`, `setSystemPrompt`, `resetSystemPrompt`, `toggleIncognito`, `getIncognitoPrompt`, `setIncognitoPrompt`
  - **Mood**: `getMoodScore`, `getMoodEnabled`, `setMoodEnabled`
  - **Version / shutdown**: `getVersion`, `shutdown`
  - **Agent extras**: `undoAgentEdit`, `getAgentAutoPermission`, `setAgentAutoPermission`, `getAgentPermissions`, `revokeAgentPermission`, `clearAgentPermissions`
  - **WhatsApp**: `getWhatsAppStatus`, `startWhatsApp`, `stopWhatsApp`, `logoutWhatsApp`, `sendWhatsApp`, `searchWhatsApp`, `getWhatsAppChats`, `getWhatsAppMessages`, `whatsAppAvatarUrl`, `getWhatsAppStats`, `getWhatsAppChatMode`, `setWhatsAppChatMode`, `sendWhatsAppChatStream`
  - **Proactive**: `getProactiveSettings`, `setProactiveSettings`, `getProactivePatterns`, `forgetPattern`, `clearLearningData`, `getPendingSuggestion`, `respondToSuggestion`
  - **Self-interest / system management**: `getSelfInterestEnabled`, `setSelfInterestEnabled`, `getSystemManagementEnabled`, `setSystemManagementEnabled`
  - Yeni model class'lar: `MemoryStats`, `MemorySearchResult`

---

## Test Durumu

```bash
flutter analyze lib/   # 7 info (hepsi önceden vardı, yeni hata yok)
```

---

## Commit Edilmemiş Dosyalar

Önceki oturumdan stage'de bekleyenler (commit atılmadı):
```
frontend/lib/providers/chat_provider.dart
frontend/lib/providers/models_provider.dart
frontend/lib/providers/mood_provider.dart
frontend/lib/screens/app_shell.dart
frontend/lib/screens/calendar_screen.dart
frontend/lib/screens/whatsapp_screen.dart
internal/api/client.go
internal/app/app.go
internal/app/memory.go
internal/app/providers.go
internal/calendar/store.go
internal/cloudsync/crypto.go
internal/cloudsync/crypto_test.go
internal/cloudsync/sync_manager.go
internal/memory/store.go
internal/provider/claude.go
internal/provider/config.go
internal/provider/gemini.go
internal/whatsapp/client.go
build_releases.sh
build_releases.bat
yapılacaklar.md
```

Bu oturumda eklenenler (henüz staged değil):
```
mobile/lib/core/api_client.dart      (F1 default URL + F2 tüm eksik endpointler)
mobile/lib/providers/connection_provider.dart  (F1 discovery)
mobile/lib/screens/connect_screen.dart         (F1 TARA butonu)
yapılacaklar.md                                (F1+F2 checkbox'ları işaretlendi)
```

---

## Kalan Görevler

### RAG 3.1 — Embedding Auto-Setup

- Kullanıcı "Belleği Etkinleştir" toggle'ına bastığında `nomic-embed` otomatik inip başlamalı.
- `EmbeddingModelRepo` zaten var, sadece orkestrasyon yazılmadı.

### RAG 2.2 — Bellek Birleştirme (Ertelenmiş)

- Kosinüs benzerliği > 0.92 çiftleri LLM ile birleştir. LLM bağımlılığı nedeniyle ertelendi.
