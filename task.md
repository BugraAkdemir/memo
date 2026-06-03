# v4.0.0 Yapılacaklar — Sistem Yönetimi (AI Agent Mode)

## Amaç

Kullanıcının bilgisayarında dosya açma, dosya silme, terminal komutları çalıştırma gibi işlemleri yapabilen profesyonel bir AI agent sistemi. Claude Code benzeri bir yapı — kullanıcıya her işlem öncesi izin sorulacak (allow once, always allow, deny, deny forever gibi seçeneklerle).

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

## 5. İleri Seviye Özellikler (Sonraki Versiyonlar)

- **Multi-step planning:** LLM birden çok adımlı plan yapabilir, her adımı permission kontrolünden geçer
- **File edit:** Sadece yazma değil, belirli satırları değiştirme (`edit_file(path, old_string, new_string)`)
- **Git işlemleri:** `git status`, `git diff`, `git commit` gibi işlemler için özel araçlar
- **Web scraping:** LLM'in URL'den içerik okuması
- **MCP uyumluluğu:** Model Context Protocol desteği — harici araçların entegrasyonu
- **Session bazlı context:** Agent'ın çalışma dizini, ortam değişkenleri oturum boyunca korunur
- **Script modu:** Kullanıcının yazdığı script'i güvenli bir sandbox'ta çalıştırma

---

> **Not:** v4.0.0 öncesi mevcut 31 bilinen hata (BILINEN_SORUNLAR.md) ve zayıf test altyapısı ele alınmalı mı? Yoksa doğrudan agent sistemine mi geçilmeli? Bu karar verilmeli.
