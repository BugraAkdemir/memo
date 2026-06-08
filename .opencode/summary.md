# Session — 2026-06-09

## Goal
WhatsApp entegrasyonu: Auto-connect, mesaj geçmişi senkronizasyonu, agent tools, chat mode toggle UI.

## Completed
### WhatsApp Go Backend
- **`initWhatsApp()`** artık `Start()`'ı goroutine'de çağırır (embedding model gibi auto-connect).
- **`Start()` idempotent + mutex**: `sync.Mutex` ile korumalı; ikinci çağrı no-op.
- **`_foreign_keys=on`**: whatsmeow sqlstore DSN'ine `&_foreign_keys=on` eklendi.
- **`handleHistorySync()`**: QR eşleşme sonrası geçmiş mesajları store'a aktarır.
- **Client store wrapper**: `SearchMessages()`, `GetChatList()`, `GetChatMessages()`.
- **Nil slice fix**: Tüm store metodları boşken `[]` döndürür (Flutter null-safety).

### Agent Tools
- **`internal/agent/tools/whatsapp.go`**: 4 tool (SendWhatsApp, SearchWhatsApp, LatestWhatsAppChats, GetWhatsAppMessages).
- **Tool registration**: `registerWhatsAppTools()`, `NewWhatsAppRegistry()`.
- **`syncc.Mutex` → `sync.Mutex`** typo fix.
- **`NewWhatsAppExecutor()`**: `executor.go`'ya eklendi, sadece WhatsApp tool'ları içerir.

### WhatsApp Chat Mode
- **`whatsappChatMode`**: `app.go`'da getter/setter ile toggle.
- **`WhatsAppChatStream()`**: Ayrı executor kullanır (normal agent tool'ları yok), WhatsApp system prompt.
- **`waToolAdapter`**: `*whatsapp.Client`'ı `tools.WhatsAppClient` interface'ine uyarlar.
- **`GetWhatsAppChatMode()`/`SetWhatsAppChatMode()`/`WhatsAppChatStream()`**: FullBridge'e eklendi.

### Flutter UI
- **`api_client.dart`**: `getWhatsAppChatMode()`, `setWhatsAppChatMode()`, `sendWhatsAppChatStream()`.
- **`whatsapp_provider.dart`**: `whatsAppChatModeProvider` (StateNotifier), `WhatsAppChatModeNotifier` (init/toggle).
- **`chat_screen.dart`**: `_ChatTopBar`'da WhatsApp toggle butonu (yeşil ikon, sadece bağlıyken görünür).
- **`chat_input.dart`**: `_send()` WhatsApp modunda `api.sendWhatsAppChatStream()`'e yönlenir.
- **`chat_provider.dart`**: `addMessage()` eklendi (WhatsApp mesajlarını listeye eklemek için).
- **`l10n.dart`**: `whatsapp_mode_on`/`whatsapp_mode_off` Türkçe + İngilizce.

### API Endpoints
- `GET/POST /api/whatsapp/chat-mode` — durum sorgulama/değiştirme.
- `POST /api/whatsapp/chat-stream` — SSE streaming ile WhatsApp sohbeti.
- FullBridge'de yeni metodlar: `GetWhatsAppChatMode`, `SetWhatsAppChatMode`, `WhatsAppChatStream`.

### Bug Fixes
- **Port 8082 leak**: `sysproc_linux.go`'da `Pdeathsig: SIGKILL` → `Setpgid: true`. Çocuk process kendi grubunda, forceKill process-group bazlı temizler.
- **`tools.go` duplicate import**: `sync` ve `memo/internal/whatsapp` import fix.

## Key Decisions
- **Auto-connect**: WhatsApp `Start()` goroutine'de çağrılır, embedding model gibi startup'ta bağlanır.
- **Ayrı executor**: WhatsApp chat mode ayrı registry kullanır; dosya/komut tool'ları WhatsApp modunda görünmez.
- **SSE streaming**: WhatsApp cevapları SSE ile Flutter'a iletilir.
- **Toggle UI**: Normal chat ekranında buton; sadece WhatsApp bağlıyken aktif.

## Next Steps
1. Flutter derleme (flutter CLI olan ortamda `flutter build linux --release`).
2. WhatsApp bağlantısının manuel testi (QR pairing → history sync → agent tools).
3. Takvim (Calendar) modülü (v3.3.0).

## Relevant Files
- `internal/whatsapp/client.go` — whatsmeow wrapper, history sync, store wrapper
- `internal/whatsapp/store.go` — SQLite mesaj deposu
- `internal/agent/tools/whatsapp.go` — 4 WhatsApp tool
- `internal/agent/tools.go` — tool registration
- `internal/agent/executor.go` — `NewWhatsAppExecutor()`
- `internal/llama/sysproc_linux.go` — `Setpgid: true`
- `app.go` — WhatsApp chat mode, stream, tool adapter
- `internal/webserver/handlers_flutter.go` — chat-mode + chat-stream handlers
- `internal/webserver/server.go` — route registration
- `internal/webserver/bridge.go` — FullBridge yeni metodlar
- `frontend/lib/core/api_client.dart` — WhatsApp streaming metodları
- `frontend/lib/providers/whatsapp_provider.dart` — chat mode provider
- `frontend/lib/providers/chat_provider.dart` — addMessage()
- `frontend/lib/screens/chat_screen.dart` — toggle butonu
- `frontend/lib/widgets/chat_input.dart` — WhatsApp send yönlendirmesi
- `frontend/lib/core/l10n.dart` — yeni string'ler
