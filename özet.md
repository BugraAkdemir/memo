# Memo — Proje Özeti

> **Son güncelleme:** 2026-06-05
> **Sürüm:** v3.0.0-beta
> **Mimari:** Decoupled (Go Backend + Flutter Frontend)

---

## 1. Proje Nedir?

Memo, yerel ve harici LLM'lerle çalışan, gizlilik odaklı bir **AI Hafıza Kabuğu (Memory Shell)**'dur. Sohbet arayüzünün ötesinde, RAG (Retrieval-Augmented Generation) ile kalıcı hafıza, çoklu model orkestrasyonu, AI ajan (tool calling) ve harici API desteği sunar.

---

## 2. Mimari Yapı

### 2.1 İki Süreç Ayrışması

```
┌──────────────────────────────┐     HTTP/JSON + SSE     ┌──────────────────────────┐
│  Flutter Desktop Client       │ ◄────────────────────► │  Go Backend (localhost)  │
│  (Linux/Windows/macOS)       │    localhost:8090       │  Headless REST API       │
│  Riverpod + Dio              │                         │  http.ServeMux           │
└──────────────────────────────┘                         └──────────────────────────┘
```

- **Backend:** Headless Go REST API sunucusu, port 8090
- **Frontend:** Flutter masaüstü uygulaması (Riverpod state, Dio HTTP/SSE)
- **İletişim:** Plain HTTP/JSON + SSE streaming, TLS yok (localhost)

### 2.2 Bridge Pattern

```
Web Server (handlers_flutter.go)
    │
    ▼ AppBridge (interface)
    │
    ▼ app.go (central orchestrator, 2409 satır)
    │
    ├── internal/memory/     (vector store)
    ├── internal/llama/      (llama.cpp lifecycle)
    ├── internal/sessions/   (chat persistence)
    ├── internal/provider/   (external LLM APIs)
    ├── internal/agent/      (tool calling engine)
    ├── internal/orchestra/  (multi-model orchestration)
    ├── internal/cloudsync/  (Google Drive backup)
    ├── internal/identity/   (system prompt & persona)
    ├── internal/modelstore/ (HF model search/download)
    └── internal/api/        (OpenAI-compatible client)
```

`AppBridge` base interface + `FullBridge` (Flutter handler'ları için genişletilmiş). `App` struct'ı her iki interface'i de implemente eder.

---

## 3. LLM Yönlendirme Önceliği (`callLLMStream`)

```
Kullanıcı Mesajı
    │
    ▼
1. Orchestra Mode aktif mi?
    │ Evet → orchestra.Conductor.RunWithProgress()
    │        (Chief → plan → experts → synthesize)
    │
    Hayır ▼
2. Agent Mode aktif + active provider var mı?
    │ Evet → callAgentStream() → agent.Executor.RunStream()
    │        (LLM → tool call → permission → execute → loop)
    │
    Hayır ▼
3. Active provider (external) set edilmiş mi?
    │ Evet → provider.Router.ChatCompletionStream()
    │        (fallback chain: try provider1 → provider2 → ...)
    │
    Hayır ▼
4. Local llama.cpp çalışıyor mu?
    │ Evet → api.Client → local llama-server
    │
    Hayır ▼
5. Hata: "No provider configured"
```

---

## 4. Backend Modülleri

### 4.1 `internal/webserver/` — REST API Katmanı

| Dosya | Satır | Görev |
|-------|-------|-------|
| `server.go` | ~600 | http.ServeMux router, ~45 route, StartHTTP/Start, SSE handler |
| `handlers_flutter.go` | ~960 | Tüm Flutter REST handler'ları |
| `bridge.go` | ~115 | AppBridge + FullBridge interface |

**Kritik bilgi:** Gin framework kullanılmaz, standart `http.ServeMux`. Remote access v3.0.0'da devre dışı bırakıldı.

### 4.2 `internal/llama/` — llama.cpp Yönetimi

| Dosya | Satır | Görev |
|-------|-------|-------|
| `llama.go` | ~462 | Subprocess lifecycle (start/stop/monitor/wait-ready), port conflict resolution |
| `installer.go` | ~646 | Otomatik binary download (GitHub releases), git clone + build fallback |
| `gpu.go` | ~232 | GPU detection (NVIDIA nvidia-smi, AMD lspci/sysfs, Apple Metal), VRAM hesaplama |

**Bilinen sorunlar:** `monitor()` goroutine'i `s.cmd`'ye lock dışında erişir (nil-pointer panic riski, kısmen fixed). `nvidia-smi` hataları loglanır (eskiden sessiz geçilirdi).

### 4.3 `internal/memory/` — Vektör Deposu (RAG)

| Dosya | Satır | Görev |
|-------|-------|-------|
| `store.go` | ~350 | Chromem-go + .gob file CRUD, index management (memory_index.gob) |
| `retriever.go` | ~100 | Cosine similarity search (pre-computed L2 norm), parallel workers |
| `embedder.go` | ~50 | OpenAI-compatible embedding API client (port 8082) |

**Depolama formatı:**
- Her etkileşim ayrı `.gob` dosyası (`data/memory/{collection_id}/{hash}.gob`)
- `memory_index.gob` ile O(1) startup (eski: O(N) per-file scan)
- Embedding: 768/1536 boyutlu float32 vektörler

**Bilinen sorunlar:** Collision risk `hash[:8]` ile minimize edildi (eski: `hash[:4]`). Embedding model ayrıca başlatılmalı (auto-start atlanabilir).

### 4.4 `internal/sessions/` — Sohbet Oturumları

| Dosya | Satır | Görev |
|-------|-------|-------|
| `sessions.go` | ~262 | JSON CRUD, auto-title generation, rename/delete |

**Depolama:** `data/sessions/{uuid}.json` (tam UUID, 36 karakter). Her mesajda async yazma.

### 4.5 `internal/provider/` — Harici API Sağlayıcıları (YENİ)

| Dosya | Satır | Görev |
|-------|-------|-------|
| `provider.go` | ~237 | Provider interface (ChatCompletion, ChatCompletionStream, ListModels), tipler, factory |
| `config.go` | ~369 | ConfigManager: AES-256-GCM şifreli API key saklama, data/providers.json |
| `router.go` | ~282 | Multi-provider router: sıralı fallback, auto-disable (3 hata), health check (5dk) |
| `openai.go` | ~353 | OpenAI-compatible implementasyon (ChatCompletion + SSE + ListModels) |
| `gemini.go` | ~351 | Google Gemini özel implementasyon (generateContent, streamGenerateContent) |
| `claude.go` | ~368 | Anthropic Claude özel implementasyon (x-api-key, Messages API, SSE events) |
| `grok.go` | ~29 | xAI Grok (OpenAI-compatible wrapper) |
| `groq.go` | ~62 | Groq (OpenAI-compatible + custom ListModels) |
| `openrouter.go` | ~28 | OpenRouter (OpenAI-compatible wrapper) |
| `ollama.go` | ~28 | Ollama (OpenAI-compatible wrapper) |

**Şifreleme:** AES-256-GCM, master key `/etc/machine-id`'den türetilir (taşınabilir değil).

**Router özellikleri:**
- Aktif provider'lar sırayla denenir
- 3 ardışık hata → auto-disable (429/401/403/timeout)
- Health check goroutine disabled provider'ları periyodik test eder
- Manuel re-enable: `ReenableProvider()`, `ReenableAllProviders()`

**Bilinen sorunlar:** `Priority` alanı tanımlı ama kullanılmıyor. Test dosyası yok.

### 4.6 `internal/agent/` — AI Ajan Motoru (YENİ)

| Dosya | Satır | Görev |
|-------|-------|-------|
| `tools.go` | ~149 | ToolDef, DangerLevel (Safe/Medium/Dangerous), ToolRegistry, 8 built-in tool |
| `permissions.go` | ~241 | 6 policy: PromptAlways, AllowOnce/Session/Forever, DenyOnce/Forever, SHA-256 hashing |
| `sandbox.go` | ~137 | Path validation, rate limiting (30/dk), 23-pattern command blacklist |
| `pipeline.go` | ~226 | LLM ↔ tool loop (max 20 iterasyon), event streaming |
| `executor.go` | ~171 | Top-level orchestrator, audit log (1000 entry) |
| `tools/file.go` | ~267 | read_file, write_file, delete_file, list_directory, get_file_info |
| `tools/command.go` | ~131 | run_command (bash -c, 60s timeout, 10MB output limit) |
| `tools/search.go` | ~121 | search_files (glob, 30s, max 100), read_env (sensitive key masking) |

**8 built-in tool:**
| Tool | Danger Level | Açıklama |
|------|-------------|----------|
| `read_file` | Safe | Dosya okuma, max 1MB |
| `write_file` | Medium | Dosya yazma, `.bak` yedek |
| `delete_file` | Dangerous | Dosya/dizin silme, `.git` korumalı |
| `list_directory` | Safe | Dizin listeleme, max 1000 |
| `run_command` | Dangerous | Komut çalıştırma, 60s timeout |
| `search_files` | Safe | Glob arama, 30s timeout |
| `get_file_info` | Safe | Dosya meta bilgisi |
| `read_env` | Medium | Env listeleme, hassas maskeli |

**Pipeline akışı:**
1. User message + 8 tool definition → LLM (temp=0.2)
2. LLM tool_calls döndürürse → her call için:
   a. Tool registry kontrolü
   b. Rate limit kontrolü
   c. Permission check (Safe→auto, Medium/Dangerous→prompt)
   d. Execute (sandbox'ta)
   e. Sonucu `role: "tool"` olarak conversation'a ekle
3. Nihai yanıt üretilene kadar döngü (max 20)

**Command blacklist (23 pattern):**
`rm -rf /`, `rm -rf ~`, `rm -rf .`, `dd`, `mkfs`, `format`, `fdisk`, `parted`, `chmod 777`, `chown` (system files), `sudo`, `su`, `pkexec`, `:(){ :|:& };:`, `nc -e`, `bash -i`, `mkfifo`, `shutdown`, `reboot`, `halt`, `poweroff`

**Bilinen sorunlar:** Frontend UI yok (permission dialog, tool call cards, mode toggle). Non-streaming ChatCompletion kullanır (UI bloke olur). Audit log persisted değil. Test yok.

### 4.7 `internal/orchestra/` — Çoklu Model Orkestrasyonu (YENİ)

| Dosya | Satır | Görev |
|-------|-------|-------|
| `conductor.go` | ~680 | Conductor: plan → execute → synthesize pipeline |
| `types.go` | ~150 | OrchestraConfig, OrchestraTask, OrchestraPlan, OrchestraResult, ProgressUpdate |
| `roles.go` | ~169 | 8 built-in role system prompts (Turkish) |

**Akış:**
1. **Plan:** Chief model (ör. Claude) kullanıcı isteğini analiz eder, JSON plan döndürür (`{"tasks": [{"role":"frontend",...}], "parallel": true}`)
2. **Execute:** Görevler paralel (goroutine + WaitGroup) veya sıralı (DAG dependency resolution) çalıştırılır. Her görev için ayrı provider instance'ı oluşturulur (120s timeout, 2 retry)
3. **Synthesize:** Chief tüm sonuçları alır, tek bir tutarlı cevap üretir

**8 built-in role:**
| Role | Default Model | Default Enabled |
|------|--------------|-----------------|
| planner | Claude | ✅ |
| frontend | Grok | ✅ |
| backend | GPT-4o | ✅ |
| bug_fixer | Gemini | ✅ |
| reviewer | Claude | ❌ |
| security | GPT-4o | ❌ |
| devops | Grok | ❌ |
| general | GPT-4o | ✅ |

**Rate limit koruması:** `callWithRetry` — 3 retry, 5s/10s/20s exponential backoff + jitter, `try again in Xs` parse.

**Önemli not:** Router'ı bypass eder — provider'ları doğrudan factory ile oluşturur (fallback zinciri yok).

**Frontend:** Settings'de Orchestra tab, config dialog (chief model + role assignments), `/orchestra` slash command.

### 4.8 `internal/cloudsync/` — Google Drive Yedek

| Dosya | Satır | Görev |
|-------|-------|-------|
| `sync_manager.go` | ~470 | Periodic/triggered pull/push/full-sync pipeline |
| `drive.go` | ~260 | OAuth2 loopback auth, Google Drive API |
| `crypto.go` | ~100 | AES-256-GCM + PBKDF2 (600K iterasyon) |

**Özellikler:**
- AES-256-GCM E2E encryption with PBKDF2 key derivation
- Random salt per encryption (legacy SHA-256 fallback)
- Machine-ID based key (hardcoded fallback kaldırıldı)
- Zip-based archive format

### 4.9 `internal/identity/` — Sistem Prompt & Persona

System prompt yönetimi, kullanıcı adı ve kişilik bilgisi. RAG memory injection burada yapılır.

### 4.10 `internal/modelstore/` — HuggingFace Model Mağazası

| Dosya | Satır | Görev |
|-------|-------|-------|
| `modelstore.go` | ~458 | HF model search/download, local model import/delete/list |

**Özellikler:** GGUF model arama, paralel indirme (progress tracking), 50 GiB import limit, model auto-classification via filename.

### 4.11 `internal/api/` — OpenAI- Uyumlu API Client

| Dosya | Satır | Görev |
|-------|-------|-------|
| `types.go` | ~50 | ChatCompletionRequest/Response, Message, StreamChunk |
| `client.go` | ~200 | HTTP client for llama-server communication |
| `streaming.go` | ~80 | SSE parsing with thinking/reasoning extraction |

---

## 5. Frontend (Flutter)

### 5.1 Teknik Stack
- **Framework:** Flutter 3.10+ (Linux/Windows/macOS)
- **State Management:** Riverpod 2.x (AsyncNotifierProvider)
- **HTTP Client:** Dio 5.4 with SSE interceptor
- **Markdown:** flutter_markdown 0.6

### 5.2 Module Map

```
AppShell
  ├── ChatScreen
  │   ├── ChatMessageList → MessageBubble
  │   └── ChatInput
  │       └── PromptTemplates (slash commands)
  ├── ModelStoreScreen
  └── SettingsDialog
      ├── General
      ├── Identity
      ├── Memory
      ├── Model Parameters
      ├── API Providers → ProviderConfigDialog
      ├── Orchestra → OrchestraConfigDialog
      ├── Cloud Sync
      ├── Remote Access
      └── About
```

### 5.3 State Providers

| Provider | Dosya | Sorumluluk |
|----------|-------|------------|
| `ChatProvider` | `chat_provider.dart` | Message state, stream handling |
| `ModelsProvider` | `models_provider.dart` | Model list, download progress |
| `SettingsProvider` | `settings_provider.dart` | App settings, llama config |
| `ProviderListNotifier` | `provider_provider.dart` | External provider CRUD |
| `ActiveProviderNotifier` | `provider_provider.dart` | Active provider type |
| `OrchestraConfigNotifier` | `orchestra_provider.dart` | Orchestra config |

### 5.4 Önemli Widget'lar

| Widget | Dosya | Satır | Görev |
|--------|-------|-------|-------|
| `ChatMessageList` | `chat_message_list.dart` | ~450 | Message rendering, markdown, context menu (edit/delete) |
| `ChatInput` | `chat_input.dart` | ~283 | Text input, file attach, STT, slash commands |
| `SettingsDialog` | `settings_dialog.dart` | ~2129 | 8-tab settings |
| `ProviderConfigDialog` | `provider_config_dialog.dart` | ~264 | Provider add/edit |
| `OrchestraConfigDialog` | `orchestra_config_dialog.dart` | ~330 | Orchestra config |
| `ModelStoreScreen` | `model_store_screen.dart` | ~1223 | HF model search/download |
| `SetupWizardView` | `setup_wizard_view.dart` | ~296 | First-run wizard |

### 5.5 Feature List

| # | Feature | Durum |
|---|---------|-------|
| F1 | Memory Toggle | ✅ |
| F2 | Smart Chat Titles | ✅ |
| F3 | External Provider System | ✅ (Backend + Frontend) |
| F4 | Session Management UI (Rename/Delete) | ✅ |
| F5 | Model Parameters UI | ✅ |
| F6 | Message Edit/Delete | ✅ |
| F7 | Stop Streaming | ✅ |
| F8 | Dark Mode | ✅ |
| F9 | Streaming Toggle | ✅ |
| F10 | User Message Markdown | ✅ |
| F11 | Orchestra Mode | ✅ (Backend + Frontend) |
| F12 | Slash Commands Rework | ✅ |
| F13 | Model Store Visual Improvements | ✅ |
| F14 | Agent Execution Engine | ✅ (Backend only, Frontend UI missing) |

---

## 6. REST API Endpoints (Tümü)

### 💬 Chat
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| POST | `/api/send` | Non-streaming message |
| POST | `/api/send/stream` | SSE streaming message |
| POST | `/api/send_file` | File/image (Multipart) |
| GET | `/api/chats` | List sessions |
| POST | `/api/chats/new` | Create session |
| POST | `/api/chats/switch` | Switch session |
| POST | `/api/chats/delete` | Delete session |
| POST | `/api/chats/rename` | Rename session |
| GET | `/api/messages` | Get active chat history |
| POST | `/api/messages/update` | Edit message |
| POST | `/api/messages/delete` | Delete message |

### 🧠 Memory
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| GET | `/api/status` | System + memory count |
| POST | `/api/incognito` | Toggle incognito |
| GET/PUT | `/api/system-prompt` | Get/update system prompt |
| GET/DELETE | `/api/memory/files` | List/delete memory files |
| POST | `/api/memory/clear` | Clear all memory |
| GET/PUT | `/api/memory/enabled` | Get/set memory enabled |

### 🏭 Models
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| GET/DELETE | `/api/models/local` | List/delete local models |
| POST | `/api/models/start` | Start model |
| POST | `/api/models/stop` | Stop model |
| GET | `/api/models/status` | Model status |
| GET | `/api/gpu` | GPU detection |
| POST | `/api/models/search` | HF search |
| POST | `/api/models/download` | Start download |
| GET | `/api/models/download/progress` | Download progress |
| GET | `/api/models/llama/check` | Check llama binary |
| GET/PUT | `/api/config/llama` | Llama config |
| POST | `/api/embed/start` | Start embedding server |
| POST | `/api/embed/stop` | Stop embedding server |

### 🔌 Providers
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| GET/PUT/DELETE | `/api/providers` | Provider CRUD |
| POST | `/api/providers/test` | Test connection |
| GET/PUT | `/api/providers/active` | Get/set active provider |

### 🤖 Agent
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| GET/PUT | `/api/agent/enabled` | Get/set agent mode |
| POST | `/api/agent/permission` | Permission response |
| GET | `/api/agent/permissions` | List permissions |
| DELETE | `/api/agent/permissions` | Revoke/clear permissions |

### 🎵 Orchestra
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| GET/PUT | `/api/orchestra/config` | Get/update config |

### ☁️ Cloud Sync & Config
| Method | Endpoint | Açıklama |
|--------|----------|----------|
| GET/PUT | `/api/sync/settings` | Sync settings |
| POST | `/api/image` | Read image (restricted) |
| GET | `/api/events` | Background events |

---

## 7. Depolama Katmanı

| Veri | Format | Konum | Mekanizma |
|------|--------|-------|-----------|
| Memory (vectors) | `.gob` | `data/memory/{collection}/{hash}.gob` | Chromem-go + index file |
| Sessions | JSON | `data/sessions/{uuid}.json` | Async write |
| Provider config | JSON | `data/providers.json` | AES-256-GCM encrypted |
| Orchestra config | JSON | `data/orchestra.json` | Plain JSON |
| Permissions | JSON | `data/permissions.json` | SHA-256 hashed args |
| Agent logs | Memory | (last 1000 entries) | Not persisted |
| App config | YAML | `config/config.yaml` | 0600 permissions |
| Models | GGUF | `data/models/{name}.gguf` | Downloaded files |
| Sync | Zip + AES | Google Drive appdata | E2E encrypted |
| Machine ID | Text | `data/.machine-id` | Persistent UUID |

---

## 8. Konfigürasyon

### `config/config.yaml`
```yaml
api:
  base_url: "http://127.0.0.1:8082/v1"
  embedding_model: ""
  timeout: 120
identity:
  user_name: ""
  assistant_name: "Memo"
  system_prompt: "..."
memory:
  enabled: true
llama:
  engine_mode: "auto"
  binary_path: ""
  port: 8081
  ctx_size: 4096
  n_gpu_layers: 0
  temperature: 0.7
  top_p: 1.0
  max_tokens: 0
  # ...
sync:
  enabled: false
```

### `data/providers.json`
7 provider entry (AES-256-GCM encrypted API keys):
- openai, gemini, grok, groq, claude, openrouter, ollama

### `data/orchestra.json`
Orchestra config: enabled, chief_type, chief_model, roles[]

---

## 9. Test Altyapısı

| Paket | Test Sayısı |
|-------|-------------|
| `internal/config` | 9 |
| `internal/api` | 16 |
| `internal/cloudsync` | 10 |
| `internal/sessions` | 19 |
| `internal/identity` | 13 |
| `internal/modelstore` | 17 |
| `internal/llama` | 6 |
| `internal/memory` | 2 |
| Flutter (Dart) | ~40 |
| **Toplam Go** | **95** |
| **Toplam (Go + Dart)** | **~135** |

**Test edilmeyen:** `internal/provider/`, `internal/agent/`, `internal/orchestra/` — sıfır test.

---

## 10. Sürüm Geçmişi & Yol Haritası

| Sürüm | Durum | İçerik |
|-------|-------|--------|
| **v3.0.0-beta** | **Current** | External providers + Agent backend + Orchestra + Memory toggle + Dark mode + Message edit/delete + Stop streaming + Slash commands + Model params UI + Session rename/delete + Smart titles + Markdown render |
| **v3.0.0** | Planned | Agent frontend UI (permission dialog, tool cards, mode toggle), Multi-step planning, File edit, Git tools, Web scraping |
| **v4.0.0** | Future | SQLite migration, UI overhaul, Remote Access, STT re-enable |
| **v5.0.0** | Future | Knowledge graph, plugins, mobile, autonomy |

---

## 11. Bilinen Sorunlar (Özet, 54 bug + 11 observation)

### Critical (12)
- C1: `a.syncManager` data race
- C2: `a.store` assigned without lock at startup
- C6: OAuth server leak + authWg race
- C9: DeleteGobFile index inconsistency
- C10: Orphaned syncManager goroutines
- C11-C15: Flutter crash bugs (context after pop, setState after async, etc.)
- C16: Orchestra bypasses router (no fallback)
- C17: Agent pipeline non-streaming (blocks UI)

### High (14)
- H1-H10: OAuth duplicate Done, 5-min goroutine, message reordering, nil embeddingClient, etc.
- H11: Provider Priority field unused
- H12: Active provider not visible in UI
- H13: No agent API methods in frontend
- H14: Agent permission dialog not implemented

### Medium (15)
- M1-M12: Context.Background usage, path traversal, no body limits, temp file location, etc.
- M13: Orchestra config no validation
- M14: Agent pipeline no per-tool timeout
- M15: Agent audit log limited to 1000

### Low (10)
- L1-L10: Silent error swallowing, missing const constructors, hardcoded Turkish strings, etc.

### Observations (11)
- I1-I10: GOB compatibility, single-file design, Flutter L10n issues, etc.
- I11: No tests for provider/agent/orchestra

---

## 12. Geliştirme Komutları

```bash
# Backend
go run . --port 8090
go build -o memo .
go test ./...                    # 95 test
go test ./internal/provider/...  # 0 test (TODO)

# Frontend
cd frontend && flutter run -d linux
cd frontend && flutter build linux --release
cd frontend && flutter test      # ~40 test
cd frontend && flutter analyze

# Package
./package_linux.sh               # build_output/memo-linux-x64/
./build_releases.sh              # tar.gz, AppImage, deb
```

---

> **Detaylı dokümantasyon:**
> - Mimari: `architecture.md`
> - API: `docs/API_REFERENCE.md`
> - Özellikler: `docs/FEATURES.md`
> - Bilinen sorunlar: `docs/KNOWN_ISSUES.md`
> - Çözülen sorunlar: `docs/RESOLVED_ISSUES.md`
> - Yol haritası: `docs/ROADMAP.md`
> - Task list: `task.md`
> - Sürüm notları: `versinNote/V3.0.0.md`
> - Obsidian (EN): `obsidian-doc-en/Memo/`
> - Obsidian (TR): `obsidian-doc/Memo/`
