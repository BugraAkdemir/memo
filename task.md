# v4.0.0 Yapılacaklar

## Amaç

Memo'yu sadece local modellere bağımlı olmaktan çıkarıp, harici API'ler (OpenAI, Google Gemini, Grok/xAI, Anthropic Claude vb.) üzerinden de çalışabilen, profesyonel bir AI agent sistemine dönüştürmek. Agent modu sayesinde kullanıcı bilgisayarında dosya açma, silme, terminal komutları çalıştırma gibi işlemleri yapabilecek — Claude Code benzeri bir deneyim.

---

> **⚠️ Sıralama:** Önce harici API desteği (1), ardından agent sistemi (2-13). Agent'ın anlamı güçlü modellerle artar.

---

## 0. Ön Koşul: Harici API Desteği (External Provider Support)

### 0.1 Neden Önce Bu?
- Local modeller agent kullanımı için çok yavaş ve yeteneksiz
- OpenAI, Claude, Gemini gibi modeller tool calling'de çok başarılı
- Kullanıcı locale muhtaç olmadan direkt kullanmaya başlayabilmeli

### 0.2 Provider Sistemi
- `internal/provider/` — yeni paket
- Ortak arayüz: `Provider` interface
  - `ChatCompletion(messages, tools)` → response
  - `ChatCompletionStream(messages, tools)` → stream
  - `Name()` → provider adı
  - `Models()` → kullanılabilir modeller
- Built-in provider'lar:
  - **OpenAI** — GPT-4o, GPT-4o-mini, o1, o3
  - **Google Gemini** — Gemini 2.0 Flash, Gemini 2.5 Pro
  - **xAI Grok** — Grok-2, Grok-3
  - **Anthropic Claude** — Claude 3.5 Sonnet, Claude 3 Opus
  - **OpenRouter** — tüm modellere tek API üzerinden erişim
  - **Ollama** — local modeller (mevcut llama.cpp'nin yanında alternatif)
  - **llama.cpp** — mevcut local engine (korunacak)

### 0.3 Provider Konfigürasyonu
- Ayarlar sayfasında "API Providers" bölümü
- Her provider için:
  - API Key input (maskeli, `****` gösterim)
  - Base URL (opsiyonel, varsayılan)
  - Model seçimi (dropdown)
  - "Test Connection" butonu
  - "Active" toggle — hangi provider'ın kullanılacağı
- API Key'ler şifrelenerek saklanmalı (`config.json` veya ayrı `providers.json`)
  - AES-GCM ile şifreleme (mevcut `crypto.go` benzeri)
  - Master key: `.machine-id` (mevcut)

### 0.4 Router / Fallback Sistemi
- `internal/provider/router.go` — provider yönlendirici
- Kullanıcı birden çok provider ekleyebilir, sıralı fallback çalışır:
  1. Birincil provider dene
  2. Başarısız olursa (rate limit, down, timeout) ikinciye geç
  3. Tümü başarısız olursa kullanıcıya hata göster
- Auto-fallback: Bir provider 3 kere üst üste hata verirse otomatik devre dışı bırakılır

### 0.5 Frontend: Provider Yönetimi
- Ayarlar'da "API Providers" sekmesi
- Provider listesi (kart görünümü):
  ```
  ┌──────────────────────────────────┐
  │ 🤖 OpenAI                        │
  │ Model: GPT-4o                    │
  │ Status: ✅ Connected             │
  │ [Configure] [Disable]            │
  └──────────────────────────────────┘
  ```
- Provider ekleme dialog'u:
  - Provider seçimi (dropdown)
  - API Key input
  - Model seçimi
  - "Test & Save" butonu
- Her provider için ayrı model parametreleri (temperature, top_p, max_tokens)

### 0.6 Mevcut Sistemle Entegrasyon
- Mevcut `callLLMStream` / `callLLM` fonksiyonları provider sistemine yönlendirilecek
- Local model varsa → llama.cpp kullanılır (mevcut)
- Harici API varsa → provider router kullanılır
- İkisi de yoksa → hata mesajı ("No provider configured")
- Provider seçimi UI'dan yapılabilir olmalı (sohbet ekranında dropdown)

### 0.7 Dosya Değişiklikleri
- Yeni: `internal/provider/` — provider interface + implementasyonlar
- Yeni: `internal/provider/openai.go`, `gemini.go`, `grok.go`, `claude.go`, `openrouter.go`, `ollama.go`
- Yeni: `internal/provider/router.go` — fallback yönlendirici
- Değişen: `app.go` — provider yöneticisi, `callLLMStream` refactor
- Değişen: `internal/webserver/handlers_flutter.go` — provider endpoint'leri
- Yeni: `frontend/lib/providers/provider_provider.dart` (provider provider 😄)
- Yeni: `frontend/lib/widgets/provider_config_dialog.dart`
- Değişen: `frontend/lib/widgets/settings_dialog.dart` — yeni sekme

---

## 1. Backend: Agent Execution Engine

### 1.1 Tool Definition Sistemi
- Her aracın tanımı JSON Schema formatında olmalı (OpenAI tool calling standardı)
- Tool tanımı: `name`, `description`, `parameters` (JSON Schema), `danger_level` (safe/medium/dangerous)
- Built-in tool'lar:
  - `read_file(path)` — dosya okuma (safe)
  - `write_file(path, content)` — dosya yazma/oluşturma (medium)
  - `delete_file(path)` — dosya silme (dangerous)
  - `list_directory(path)` — dizin listeleme (safe)
  - `run_command(command, cwd)` — terminal komutu çalıştırma (dangerous)
  - `search_files(pattern, path)` — dosya arama (safe)
  - `get_file_info(path)` — dosya meta bilgisi (safe)
  - `read_env()` — ortam değişkenlerini listele (medium)

### 1.2 Permission Manager
- `internal/agent/permissions.go` — yeni paket
- Permission tipleri:
  - `PromptAlways` — her seferinde sor
  - `AllowOnce` — bir kereye mahsus izin ver
  - `AllowSession` — bu oturum boyunca izin ver
  - `AllowForever` — kalıcı olarak izin ver (`.opencode/` dizinine kaydedilir)
  - `DenyOnce` — bir kere reddet
  - `DenyForever` — kalıcı reddet
- Permission kaydı: `data/permissions/` dizininde JSON dosyaları
- Her izin kaydı: `tool_name`, `args_hash` (argümanların hash'i), `policy`, `created_at`, `updated_at`

### 1.3 Tool Execution Sandbox
- `internal/agent/executor.go` — tool'ları çalıştırma
- `run_command` için:
  - Maksimum çalışma süresi (varsayılan 60 saniye, yapılandırılabilir)
  - Output boyut limiti (10MB)
  - Yasaklı komutlar listesi (rm -rf /, dd, mkfs, format, vb.)
  - `$PATH` güvenliği — sadece standart dizinler
- `write_file`/`delete_file` için:
  - Proje dizini dışına yazma/silme engeli (opsiyonel, ayarlanabilir)
  - Symlink saldırı koruması (mevcut `safePersistPath` benzeri)

### 1.4 Agent Pipeline
- `internal/agent/pipeline.go`:
  1. Kullanıcı mesajı → LLM'e gönder (tool tanımlarıyla birlikte)
  2. LLM tool call yanıtı dönerse → permission kontrolü
  3. İzin varsa → tool'u çalıştır, sonucu LLM'e geri gönder
  4. LLM nihai yanıtı üretir → kullanıcıya göster
- Loop desteği: LLM birden çok tool call yapabilir, her biri ayrı permission kontrolünden geçer

### 1.5 Streaming Support
- Tool execution sonuçları streaming ile kullanıcıya iletilmeli
- Uzun süren komutlarda (build, test) output anlık gösterilmeli
- Kullanıcı komutu iptal edebilmeli (cancel butonu)

---

## 2. Frontend: Permission UI

### 2.1 Permission Dialog
- `frontend/lib/widgets/agent/permission_dialog.dart`
- LLM bir tool çağırmak istediğinde açılan dialog
- İçerik:
  - **Tool adı** ve **açıklaması** (örn. "Run Command: `rm -rf /tmp/test`")
  - **Tehlikeli araç** uyarı banner'ı (kırmızı/kahverengi)
  - **Argümanlar** okunabilir formatta gösterilmeli
  - **Dosya önizleme** (read_file için ilk 20 satır)
- Butonlar:
  - "Allow Once" (birincil)
  - "Always Allow — this session" (ikincil)
  - "Always Allow — forever" (üçüncül, `.opencode/permissions.json`'a kaydedilir)
  - "Deny" (iptal)
  - "Deny Forever" (kırmızı, kalıcı red)
- Güvenlik: "Allow" butonlarına 2 saniyelik bekleme süresi (dangerous tool'larda)

### 2.2 Permission History Panel
- `frontend/lib/widgets/agent/permission_history.dart`
- Ayarlar'da "Permission History" bölümü
- Geçmiş tüm izin kararları listelenir
- Her kayıt: tool, args, policy, timestamp
- "Revoke" butonu — kalıcı izni iptal et
- "Clear All" — tüm kalıcı izinleri temizle

### 2.3 Agent Chat UI
- Normal sohbetten farklı bir görünüm (opsiyonel)
- Tool çağrıları görsel kart olarak gösterilmeli:
  ```
  ┌─────────────────────────────────┐
  │ 🔧 read_file("/etc/hosts")      │
  │ ✅ Completed (0.02s)            │
  │ 📄 [önizleme: 15 satır]         │
  └─────────────────────────────────┘
  ```
- Hata durumunda kart kırmızı:
  ```
  ┌─────────────────────────────────┐
  │ ❌ run_command("rm -rf /")      │
  │ ⛔ Permission denied            │
  └─────────────────────────────────┘
  ```
- Komut output'u varsa expandable/collapsible bölüm

### 2.4 Agent Mode Toggle
- Ana ekranda bir "Agent Mode" toggle (sohbet girişinin üstünde)
- Kapalıyken: normal sohbet (tool çağrısı yok)
- Açıkken: LLM tool çağırabilir, permission dialog'ları gösterilir
- Varsayılan: kapalı (güvenlik)

---

## 3. Güvenlik ve Kısıtlamalar

### 3.1 Danger Level Sistemi
| Seviye | Örnek | Varsayılan Policy |
|---|---|---|
| `safe` | read_file, search_files, list_directory | Allow (sessiz) |
| `medium` | write_file, read_env | Prompt |
| `dangerous` | delete_file, run_command | Prompt + 2sn gecikme |

### 3.2 Yasaklı İşlemler
- `run_command` için kara liste:
  - `rm -rf /`, `rm -rf ~`, `rm -rf .` (kök dizin koruması)
  - `dd`, `mkfs`, `format`, `fdisk`, `parted`
  - `chmod 777`, `chown` (sistem dosyalarında)
  - `sudo`, `su`, `pkexec` (yükseltilmiş yetki)
  - `:(){ :|:& };:` (fork bomb)
- `write_file` için:
  - `/etc/`, `/usr/`, `/bin/`, `/boot/`, `/dev/` dizinlerine yazma engeli
- `delete_file` için:
  - Aynı kara liste + proje `.git/` dizini koruması

### 3.3 Rate Limit ve Güvenlik
- Dakikada maksimum 30 tool call
- Aynı `run_command` çağrısı 5 saniyede 1'den fazla olamaz
- Tool call'ların tam log'u `data/agent-log/` dizinine yazılır (denetim için)

---

## 4. Yapısal Değişiklikler

### 4.1 Yeni Dizin Yapısı
```
internal/agent/
├── executor.go      # tool execution engine
├── permissions.go   # permission manager
├── pipeline.go      # LLM ↔ tool orchestration
├── tools.go         # tool definitions & registry
├── sandbox.go       # execution sandbox (security)
└── tools/
    ├── file.go      # read_file, write_file, delete_file, etc.
    ├── command.go   # run_command
    └── search.go    # search_files, get_file_info

frontend/lib/widgets/agent/
├── permission_dialog.dart
├── permission_history.dart
├── agent_chat_card.dart
├── agent_mode_toggle.dart
├── tool_result_view.dart
└── tool_call_bubble.dart
```

### 4.2 Mevcut Dosyalarda Değişiklik
- `app.go` — `Agent` yöneticisi eklenmeli, `SendMessageStream` agent mode'u desteklemeli
- `internal/webserver/handlers_flutter.go` — yeni endpoint'ler:
  - `POST /api/agent/tool-call` — tool call izni sonucu (allow/deny)
  - `GET /api/agent/permissions` — kalıcı izin listesi
  - `DELETE /api/agent/permissions/:id` — izni iptal et
- `internal/webserver/bridge.go` — `AgentBridge` eklenmeli
- `frontend/lib/providers/` — `agent_provider.dart`, `permission_provider.dart`
- `frontend/lib/screens/chat_screen.dart` — agent mode toggle, tool call bubble'ları

---

---

## 5. Multi-Step Planning (Planlama ve Çoklu Adım)

### 5.1 Plan Oluşturma
- LLM bir task için birden çok adımlı plan yapabilir
- Plan JSON formatında döner: `{"steps": [{"tool": "search_files", "args": {...}}, {"tool": "read_file", "args": {...}}]}`
- Her adım ayrı ayrı permission kontrolünden geçer
- Plan bir kerede gösterilir: "Şu adımları yapacağım:" + adım listesi

### 5.2 Toplu İzin
- Plan gösterildiğinde kullanıcıya seçenekler:
  - "Run All" — tüm adımları çalıştır (her adımda sorar)
  - "Run All & Auto-Allow Safe" — sadece medium/dangerous adımlarda sorar
  - "Cancel Plan" — tüm planı iptal et
  - "Modify Plan" — kullanıcı adımları düzenleyebilir

### 5.3 Plan Execution
- Adımlar sırayla çalıştırılır
- Her adımın sonucu bir sonraki adıma input olarak verilebilir
- Bir adım başarısız olursa: "Devam et" / "Durdur" seçeneği
- Plan ilerlemesi UI'da gösterilir (step 2/5 gibi)

---

## 6. File Edit (Satır Bazlı Düzenleme)

### 6.1 `edit_file` Tool
- `edit_file(path, old_string, new_string)` — dosyada belirtilen string'i değiştirir
- `edit_file(path, start_line, end_line, new_content)` — satır bazlı değiştirme
- `insert_line(path, line_number, content)` — belirli satıra ekleme
- `delete_lines(path, start_line, end_line)` — satır aralığı silme

### 6.2 Güvenlik
- Değişiklik öncesi otomatik yedek: `data/agent-backups/{path}.bak`
- Her `edit_file` öncesi diff gösterilmeli (unified diff formatı)
- "Apply" / "Discard" seçenekleri
- Geri alma desteği: son 10 düzenleme geri alınabilir

### 6.3 Önizleme
- Değişiklik öncesi: yeşil/kırmızı renkli diff görseli
- Syntax highlighting ile önizleme
- Satır numaraları gösterilmeli

---

## 7. Git Entegrasyonu

### 7.1 Git Tool'ları
- `git_status(path)` — çalışma dizini durumu
- `git_diff(path, file)` — değişiklikleri göster
- `git_log(path, max_count)` — commit geçmişi
- `git_commit(path, message)` — değişiklikleri commit'le
- `git_add(path, files)` — dosyaları stage'e ekle
- `git_branch(path)` — branch listesi
- `git_checkout(path, branch)` — branch değiştir

### 7.2 Güvenlik
- `git_commit` ve `git_push` medium danger
- `git_checkout` (değişiklik varsa) dangerous
- `git_push` öncesi değişiklik özeti gösterilmeli
- Commit mesajı kullanıcı onayına sunulmalı

### 7.3 UI
- Git durumu sohbet içinde görsel kart olarak:
  ```
  ┌─────────────────────────────────┐
  │ 📦 Git Status (memo)            │
  │ M  app.go                       │
  │ M  task.md                      │
  │ ?? frontend/tmp/                │
  └─────────────────────────────────┘
  ```
- Commit dialog'u: commit mesajı input + "Commit" butonu

---

## 8. Web Scraping

### 8.1 `fetch_url` Tool
- `fetch_url(url)` — URL'den içerik getir
- `fetch_url(url, selector)` — belirli CSS selector ile içerik
- `search_web(query)` — web araması (opsiyonel, API anahtarı gerektirir)

### 8.2 Kısıtlamalar
- Sadece HTTP/HTTPS protokolleri
- Maksimum 5MB yanıt boyutu
- 30 saniye timeout
- İçerik Markdown'a dönüştürülür (HTML→Markdown)
- Localhost/internal IP'lere istek engeli (SSRF koruması)

### 8.3 Rate Limit
- Dakikada maksimum 10 URL fetch
- Aynı URL'ye 30 saniyede 1'den fazla istek atılamaz

---

## 9. MCP Uyumluluğu (Model Context Protocol)

### 9.1 MCP Server
- `internal/agent/mcp/` — MCP implementasyonu
- Memo kendi MCP server'ı olarak çalışır
- Diğer MCP uyumlu araçlar (ör. GitHub API, veritabanı, dosya sistemi) entegre edilebilir
- Her MCP aracı ayrı bir permission kaydına sahip olur

### 9.2 MCP Client
- `internal/agent/mcp/client.go` — harici MCP server'lara bağlanma
- JSON-RPC üzerinden araç keşfi ve çağrısı
- Her harici araç için ayrı permission kontrolü

### 9.3 Tool Discovery
- Başlangıçta MCP server'lara bağlan, kullanılabilir araçları keşfet
- Keşfedilen araçlar LLM tool tanımlarına otomatik eklenir
- UI'da "Connected MCP Servers" listesi

---

## 10. Session Bazlı Context

### 10.1 Agent Context
- Her agent oturumu kendi context'ine sahip olur:
  - `cwd` — çalışma dizini (varsayılan: proje dizini)
  - `env` — ortam değişkenleri
  - `history` — önceki tool call sonuçları
  - `variables` — LLM'in hatırlaması gereken değerler

### 10.2 Context Yönetimi
- `set_cwd(path)` — çalışma dizinini değiştir
- `set_env(key, value)` — ortam değişkeni ekle
- `get_context()` — mevcut context'i göster
- Context değişiklikleri kullanıcıya bildirilir

---

## 11. Script Modu

### 11.1 Script Çalıştırma
- Kullanıcı bir script yazar (bash, python, node, vb.)
- Script güvenli bir sandbox'ta çalıştırılır
- Çıktı streaming ile gösterilir
- Maksimum çalışma süresi: 5 dakika

### 11.2 Sandbox
- Geçici dizin (`/tmp/memo-sandbox-*`)
- Ağ erişimi kapalı (varsayılan)
- Dosya sistemi erişimi sadece geçici dizin
- Tüm sistem çağrıları filtrelenir (seccomp)
- Kaynak limiti: 1 CPU, 512MB RAM

### 11.3 Script Editor
- Çok satırlı kod girişi (monospace editor)
- Dil seçimi: bash, python, node, go, rust
- "Run" butonu + output paneli
- "Save as tool" — sık kullanılan script'leri özel tool olarak kaydet

---

## 12. Test ve Hata Temizliği

### 12.1 Mevcut 31 Bilinen Hata
- BILINEN_SORUNLAR.md'deki tüm açık hatalar çözülecek
- Öncelik sırası: Kritik → Yüksek → Orta → Düşük

### 12.2 Test Altyapısı
- Go backend testleri:
  - `internal/agent/` — tüm tool'lar için unit test
  - `internal/agent/permissions_test.go` — permission mantığı
  - `internal/agent/sandbox_test.go` — sandbox güvenliği
- Flutter testleri:
  - `frontend/test/widgets/agent/` — permission dialog, agent chat card
  - `frontend/test/providers/agent_provider_test.dart`
- Entegrasyon testleri:
  - Tool call → permission → execution → response döngüsü

### 12.3 CI Entegrasyonu
- GitHub Actions (veya mevcut CI varsa genişletilir)
- `go test ./internal/agent/...`
- `flutter test`
- `dart analyze`
- Güvenlik taraması: `go vet`, `staticcheck`
