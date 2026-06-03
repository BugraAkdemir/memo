# Memo Roadmap

Strategic vision and release plan.

---

## v3.0.0 — "Agent" (Current)

**Theme:** Harici API desteği + profesyonel AI agent sistemi. Memo'yu local modele bağımlı olmaktan çıkarıp, Claude Code seviyesinde bir sistem yönetim aracına dönüştürmek.

### 0. Harici API Desteği (External Provider Support)
- OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Ollama entegrasyonu
- Provider interface + router + fallback sistemi
- API Key yönetimi (AES-GCM şifreli)
- Frontend: provider ekleme/yapılandırma UI'ı

### 1. Agent Execution Engine
- Tool definition sistemi (JSON Schema)
- Permission manager (AllowOnce/Session/Forever, Deny)
- Tool execution sandbox (run_command, file ops)
- Agent pipeline (LLM ↔ tool orchestration)
- Streaming support

### 2. Frontend: Permission UI
- Permission dialog (allow/deny seçenekleri, 2sn gecikme)
- Permission history panel
- Agent chat UI (tool call kartları)
- Agent mode toggle

### 3. Güvenlik ve Kısıtlamalar
- Danger level sistemi (safe/medium/dangerous)
- Yasaklı işlemler kara listesi
- Rate limit ve audit log

### 4. Yapısal Değişiklikler
- `internal/agent/` paketi
- `frontend/lib/widgets/agent/` widget'ları
- Yeni endpoint'ler ve bridge

### 5. Multi-Step Planning
- Plan oluşturma ve toplu izin
- Adım adım execution

### 6. File Edit
- Satır bazlı düzenleme (edit_file, insert_line, delete_lines)
- Diff önizleme ve geri alma

### 7. Git Entegrasyonu
- git_status, git_diff, git_commit, git_push tool'ları

### 8. Web Scraping
- fetch_url, search_web tool'ları
- SSRF koruması, rate limit

### 9. MCP Uyumluluğu
- MCP server + client
- Tool discovery

### 10. Session Bazlı Context
- cwd, env, history yönetimi

### 11. Script Modu
- Sandbox'ta script çalıştırma (bash/python/node)
- Script editor

### 12. Test ve Hata Temizliği
- 31 bilinen hatanın çözümü
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
- [ ] Cloud Sync ve Remote Access UI tamamlama

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
