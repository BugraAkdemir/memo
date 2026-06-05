# Memo Roadmap

Strategic vision and release plan.

---

## v3.0.0 — "Agent" (Current)

**Theme:** Harici API desteği + profesyonel AI agent sistemi. Memo'yu local modele bağımlı olmaktan çıkarıp, Claude Code seviyesinde bir sistem yönetim aracına dönüştürmek.

### ✅ 0. Harici API Desteği (External Provider Support) — DONE
- OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Groq, Ollama entegrasyonu
- Provider interface + router + fallback sistemi (auto-disable, health check)
- API Key yönetimi (AES-256-GCM şifreli, machine-ID derived key)
- Frontend: provider ekleme/yapılandırma UI'ı, test connection

### ✅ 0.8 Orchestra Mode (Çoklu Model Orkestrasyonu) — DONE
- Chief → plan → parallel/sequential execution → synthesis pipeline
- 8 built-in expert roles with configurable models
- Streaming progress updates per phase
- Frontend: settings tab, config dialog, `/orchestra` slash command

### ✅ 1. Agent Execution Engine — DONE (Backend)
- Tool definition sistemi (8 built-in tools, JSON Schema, DangerLevel)
- Permission manager (6 policy types, session + permanent, SHA-256 hashing)
- Tool execution sandbox (path validation, rate limiting, command blacklist)
- Agent pipeline (LLM ↔ tool loop, 20 max iterations, event streaming)
- `internal/agent/` package with executor, pipeline, permissions, sandbox

### ❌ 2. Frontend: Permission UI — NOT STARTED
- Permission dialog (allow/deny seçenekleri, 2sn gecikme)
- Permission history panel
- Agent chat UI (tool call kartları)
- Agent mode toggle
- **Blocker:** Agent backend çalışıyor ama frontend UI olmadığı için kullanılamıyor

### ✅ 3. Güvenlik ve Kısıtlamalar — DONE (Backend)
- Danger level sistemi (safe/medium/dangerous)
- Yasaklı işlemler kara listesi (23 pattern)
- Rate limit ve audit log (30 calls/min, 1000 entry log)

### ✅ 4. Yapısal Değişiklikler — DONE
- `internal/agent/` paketi (8 dosya, ~1450 satır)
- `internal/provider/` paketi (10 dosya, ~1700 satır)
- `internal/orchestra/` paketi (3 dosya, ~1000 satır)
- Yeni endpoint'ler: provider CRUD, agent toggle/permissions, orchestra config

### ❌ 5. Multi-Step Planning — NOT STARTED
- Plan oluşturma ve toplu izin
- Adım adım execution

### ❌ 6. File Edit — NOT STARTED
- Satır bazlı düzenleme (edit_file, insert_line, delete_lines)
- Diff önizleme ve geri alma

### ❌ 7. Git Entegrasyonu — NOT STARTED
- git_status, git_diff, git_commit, git_push tool'ları

### ❌ 8. Web Scraping — NOT STARTED
- fetch_url, search_web tool'ları
- SSRF koruması, rate limit

### ❌ 9. MCP Uyumluluğu — NOT STARTED
- MCP server + client
- Tool discovery

### ❌ 10. Session Bazlı Context — NOT STARTED
- cwd, env, history yönetimi

### ❌ 11. Script Modu — NOT STARTED
- Sandbox'ta script çalıştırma (bash/python/node)
- Script editor

### ❌ 12. Test ve Hata Temizliği — NOT STARTED
- 54 bilinen hatanın çözümü
- Test altyapısı ve CI entegrasyonu

---

## v4.0.0 — "Refresh" (Future)

**Theme:** Architectural improvements, UI overhaul, mobile companion.

### Storage Overhaul
- [ ] Migrate memory store from `.gob` to SQLite (CGO-free)
- [ ] One-time migration script
- [ ] ANN index for vector search (FTS5 + vector extension)
- [ ] Lazy loading / pagination

### UI/UX Overhaul
- [ ] Custom design system (brand identity)
- [ ] Remote Access UI completion

### Reliability
- [ ] Config validation at startup
- [ ] STT yeniden aktifleştirme

### Mobile
- [ ] Mobile companion app — secure tunnel to local memory

---

## v5.0.0 — "Evolve" (Future)

**Theme:** New capabilities, ecosystem, and autonomy.

- [ ] Knowledge Graph (Obsidian-style graph view)
- [ ] Plugin system (Go plugins)
- [ ] Autonomous memory pruning
- [ ] Self-improving system prompt
- [ ] Multi-user support with isolation
- [ ] Import/Export wizards (Notion, Obsidian, Google Keep)

---

> **Detaylı task listesi:** [task.md](../task.md) (13 bölüm, 0-12)
