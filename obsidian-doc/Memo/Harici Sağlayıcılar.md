# 🔌 Harici Sağlayıcılar

> **Paket:** `internal/provider/` (10+ dosya, ~1700 satır), CLI provider'lar için ayrıca `internal/agentcli/`
> **Yapılandırma dosyası:** `data/providers.json`
> **API endpoint'leri:** `/api/providers`, `/api/providers/test`, `/api/providers/active`

Memo, yerel llama.cpp motorunun yanında harici LLM API'lerini de destekler. Bu, GPT-4o, Claude, Gemini ve Grok gibi güçlü modellere yerel GPU gerektirmeden erişim sağlar.

---

## Mimari Bakış

```
┌─────────────────────────────────────────────────────────┐
│                    app.go                               │
│  ┌──────────────────────────────────────────────────┐   │
│  │  callLLMStream()                                  │   │
│  │  1. Orkestra modu?   → orchestra.Conductor        │   │
│  │  2. Harici sağlayıcı? → provider.Router           │   │
│  │  3. Yerel llama.cpp   → api.Client                │   │
│  └──────────────────────────────────────────────────┘   │
│                    │                                     │
│                    ▼                                     │
│  ┌──────────────────────────────────────────────────┐   │
│  │           provider.Router                         │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐         │   │
│  │  │ OpenAI   │ │ Gemini   │ │ Claude   │  ...     │   │
│  │  └──────────┘ └──────────┘ └──────────┘         │   │
│  │  Fallback: 3 başarısızlıkta auto-disable          │   │
│  │  Health check: iyileşince re-enable (5dk)        │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

---

## Sağlayıcı Arayüzü

Tüm sağlayıcılar `internal/provider/provider.go` içinde tanımlı ortak bir arayüzü uygular:

```go
type Provider interface {
    Name() ProviderType
    DisplayName() string
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
    ListModels(ctx context.Context) ([]string, error)
}
```

---

## Desteklenen Sağlayıcılar (10 tür + 2 CLI provider)

### 1. OpenAI (`openai.go`, 353 satır)
- **Uyumlu API'ler:** OpenAI, OpenAI uyumlu tüm endpoint'ler
- **Auth:** `Authorization` header'ında Bearer token
- **Özellikler:** Chat tamamlama, SSE streaming, model listeleme
- **Varsayılan modeller:** GPT-4o, GPT-4o-mini, o1, o3
- **Özel base URL:** LM Studio, yerel proxy desteği

### 2. Google Gemini (`gemini.go`, 351 satır)
- **API Stili:** OpenAI uyumlu değil, özel implementasyon
- **Auth:** API anahtarı query parameter (`?key=...`)
- **Implementasyon:** `:generateContent` (non-streaming), `:streamGenerateContent?alt=sse` (streaming)
- **Rol eşleme:** `system`/`developer` → `SystemInstruction`, `assistant` → `model`

### 3. Anthropic Claude (`claude.go`, 368 satır)
- **API Stili:** OpenAI uyumlu değil, özel implementasyon
- **Auth:** `x-api-key` header (Bearer değil), `anthropic-version: 2023-06-01`
- **Implementasyon:** POST `/messages` ile özel SSE event ayrıştırma

### 4. xAI Grok (`grok.go`, 29 satır)
- **API Stili:** OpenAI uyumlu (`openAIProvider` wrapper)
- **Base URL:** `https://api.x.ai/v1`

### 5. Groq (`groq.go`, 62 satır)
- **API Stili:** OpenAI uyumlu, özel `ListModels`
- **Base URL:** `https://api.groq.com/openai/v1`

### 6. OpenRouter (`openrouter.go`, 28 satır)
- **API Stili:** OpenAI uyumlu
- **Base URL:** `https://openrouter.ai/api/v1`
- **Değer:** Tek API ile 200+ model

### 7. Ollama (`ollama.go`, 28 satır)
- **API Stili:** OpenAI uyumlu
- **Base URL:** `http://localhost:11434/v1`
- **Değer:** Yerel açık kaynak modeller

### 8. OpenCode Zen (v3.3.3)
- **API Stili:** OpenAI uyumlu
- **Model seçimi:** OpenRouter gibi elle model adı yazılmaz — provider dialog'unda "Seç" ile sağlayıcının canlı model listesinden seçilir
- **Fiyatlandırma:** Pay-as-you-go, bazı modeller ücretsiz

### 9. OpenCode Go (v3.3.3)
- **API Stili:** OpenAI uyumlu
- **Model seçimi:** OpenCode Zen ile aynı — canlı liste, elle yazım yok
- **Fiyatlandırma:** Abonelik tabanlı

### 10. Kilo Code (v3.9.0)
- **API Stili:** OpenAI uyumlu
- **Base URL:** app.kilo.ai
- **Model seçimi:** OpenCode Zen/Go ile aynı desen — canlı model listesinden seçim, elle yazım yok; ücretsiz modeller listenin başında yeşil onay işaretiyle
- **Fiyatlandırma:** Pay-as-you-go, bazı modeller ücretsiz

> **Not:** llama.cpp bir provider olarak uygulanmamıştır. Yerel modeller `api.Client` ile ayrıca yönetilir.

### 11-12. Claude Code CLI ve Codex CLI (`internal/agentcli/`, CLI tabanlı — v3.3.4)

Diğer sağlayıcılardan mimari olarak tamamen farklı: bir HTTP API'ye istek atmak yerine bilgisayarda kurulu bir komut satırı aracını subprocess olarak çalıştırır — Claude Code için `claude -p --output-format stream-json --dangerously-skip-permissions [--resume <id>]`, Codex için `codex exec --json --dangerously-bypass-approvals-and-sandbox [-C <dir>] [resume <thread-id>]`. Import cycle'a girmeden `provider.Provider` arayüzünü uygular — `internal/provider`, `internal/agentcli`'yi doğrudan import etmez; `provider.RegisterConstructor` ile `agentcli`'nin kendi `init()`'i (her iki dosya, `claude_code.go` ve `codex.go`, ayrı ayrı) kendini kaydeder (`database/sql` driver deseni). Codex'in stream-json çıktısı Claude Code'unkinden farklı: metin delta'lar halinde değil, her `item.completed` (`type:"agent_message"`) olayında turun tam metnini tek parça olarak verir; oturum kimliği `session_id` değil `thread_id` alanında gelir, ve `resume` alt-komutu (fresh-run'ın aksine) `-C` bayrağını kabul etmez — orijinal oturumun çalışma dizinini kendisi hatırlar.

- **Sohbet-bazlı, uygulama geneli değil.** `sessions.Session`'a eklenen `CLIProvider`/`CLISessionIDs`/`CLIWorkdir` alanları — her chat kendi CLI provider'ını, kendi CLI oturum id'sini ve kendi çalışma dizinini taşır.
- **`App.streamMu`'dan bağımsız.** Normal `SendMessageStreamTo` tek bir global stream kilidi kullanır (aynı anda sadece bir chat stream edebilir); CLI görevleri bunun yerine `App.cliJobs` (`map[chatID]context.CancelFunc`) ile chat-bazlı kilitlenir — farklı chat'ler birbirini bloklamaz.
- **`a.lifecycleCtx`'e bağlı, HTTP isteğine değil.** `SendCLIMessageStream`'in ctx'i request'ten değil `App.lifecycleCtx`'ten türetilir — kullanıcı chat değiştirse/pencereyi kapatsa bile subprocess çalışmaya devam eder, sadece gerçek backend shutdown'ı (`lifecycleCancel`) öldürür (`exec.CommandContext` zincirleme cascade).
- **`ChatRequest.ResumeSessionID`/`WorkDir` ve `StreamChunk.CLISessionID`** — bu iki alan sadece CLI provider'lar tarafından kullanılır, diğer 7 provider görmezden gelir.
- Yeni endpoint'ler: `GET /api/cli/status?type=`, `POST /api/chats/cli-provider`, `POST /api/chats/cli-workdir`, `POST /api/send/cli-stream`, `GET /api/cli/running`, `GET /api/cli/commands?type=&chat_id=`.
- **Slash komutları** (`internal/agentcli/commands.go`): `ListCommands` her CLI'ın kendi komut dizinlerini okur — Claude Code için `.claude/commands/*.md` + `.claude/skills/*/SKILL.md`, Codex için `.codex/prompts/*.md` — hem proje (chat'in çalışma dizini) hem kullanıcı (home) seviyesinde; isim çakışmasında proje kazanır. Açıklama YAML frontmatter'daki `description:`'dan, yoksa ilk düz metin satırından gelir. Yerleşik komutlar bilinçli olarak kısa bir seçki: `/clear`, `/model`, `/compact` gibi çoğu yerleşik komut sadece interaktif oturum durumunu değiştirir, Memo'nun sohbetinde karşılığı yoktur.
- **Çalıştırma farkı (2026-08-02'de gerçek binary'lerle doğrulandı):** `claude -p "/komut"` slash komutunu gerçekten çalıştırır (init olayı `slash_commands` listesini bile döndürür) — pass-through yeterli. `codex exec "/komut"` ise **çalıştırmaz**; codex `~/.codex/prompts`'u sadece kendi TUI'sinde çözer, exec modunda metin modele düz geçer ve model uydurur. Bu yüzden `CodexCLI.ChatCompletionStream` komutu kendisi açar (`ExpandCommand`): frontmatter atılır, `$ARGUMENTS`/`$1..$9` doldurulur, placeholder yoksa argümanlar sona eklenir (kullanıcının yazdığı sessizce kaybolmasın diye). Bilinmeyen komut olduğu gibi gönderilir.

---

## Router ve Fallback Sistemi

**Dosya:** `internal/provider/router.go` (282 satır)

### Provider Arayüzü (Go)

```go
type Provider interface {
    Name() ProviderType
    DisplayName() string
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
    ListModels(ctx context.Context) ([]string, error)
}
```

```go
type ChatRequest struct {
    Model     string
    Messages  []Message
    Stream    bool
    MaxTokens int
    Options   map[string]interface{} // temperature, top_p, etc.
}

type StreamChunk struct {
    Content      string
    FinishReason string
    Err          error
}
```

### Şifreleme Implementasyonu

Anahtar türetme (`config.go`):

```go
func deriveKey() []byte {
    id, err := os.ReadFile("/etc/machine-id")
    // veya fallback: data/.machine-id'den UUID
    return sha256.Sum256(bytes.TrimSpace(id))
}
```

Şifreleme (`encryptAPIKey`):
1. 12-byte rastgele nonce oluştur
2. AES-256-GCM ile şifrele (nonce eklenir)
3. base64 encode → `data/providers.json`'a yaz

Şifre çözme: base64 decode → nonce ayır → AES-256-GCM decrypt

### Çalışma Prensibi

```
İstek → Router.getActiveEntries() → Sağlayıcı 1'i dene
                                        │
                                   Başarılı? → Yanıtı döndür
                                        │
                                      Hayır → recordFailure()
                                        │
                                   failCount ≥ 3? → sağlayıcıyı auto-disable et
                                        │
                                   Sağlayıcı 2'yi dene
                                        │
                                   (tümü tükenene kadar tekrarla)
```

### Özellikler

| Özellik | Implementasyon |
|---------|---------------|
| **Sıralama** | Sağlayıcılar ekleme sırasına göre döner (`Priority` alanı kullanılmaz) |
| **Auto-disable** | 3 ardışık başarısızlıkta sağlayıcı otomatik devre dışı bırakılır |
| **Health check** | Arka plan goroutine (5 dk aralıkla) disabled sağlayıcıları test eder |
| **Hata sınıflandırma** | Rate limiting (429), auth (401/403), timeout — hepsi fallback tetikler |

---

## Şifreli Konfigürasyon Yönetimi

**Dosya:** `internal/provider/config.go` (369 satır)

### Şifreleme Detayları

| Parametre | Değer |
|-----------|-------|
| Algoritma | AES-256-GCM |
| Anahtar türetme | `/etc/machine-id` (veya persistent UUID fallback) |
| Nonce | Her şifreleme için 12-byte random |
| Saklama formatı | `base64(nonce + ciphertext)` |
| Taşınabilirlik | **Taşınabilir değil** — makine ID'sine bağlı |

### Varsayılan Konfigürasyonlar

```go
func defaultConfigs() []ProviderConfig {
    return []ProviderConfig{}
}
```

Önceden 7 disabled placeholder config döndürüyordu (her yerleşik provider türü için biri), böylece yeni bir kurulumda Providers sekmesi kullanıcı hiçbir şey eklemeden önce her provider'ı "Disabled" olarak gösteriyordu. v3.9.0'daki UI düzeltme turunda değiştirildi: bu, kullanıcının hiç kullanmayacağı provider'larla sekmeyi kalabalıklaştırıyordu — artık boş dönüyor, sadece kullanıcının gerçekten eklediği provider'lar görünüyor. Bu değişiklikten önceki kurulumlarda diskte kalmış placeholder satırları, yıkıcı olmayan bir frontend filtresiyle gizleniyor.

---

## Ön Yüz Entegrasyonu

### Sağlayıcı Yapılandırma Dialog'u

```
┌─────────────────────────────────────┐
│  Sağlayıcı Ekle                      │
│                                      │
│  Sağlayıcı Türü: [OpenAI        ▼]   │
│  Görünen Ad:    [My OpenAI       ]   │
│  API Anahtarı:  [****************]   │
│  Base URL:      [api.openai.com/v1]  │
│  Model:         [gpt-4o          ]   │
│                                      │
│  [✓] Aktif                           │
│                                      │
│  [Test Bağlantısı]  [İptal] [Kaydet] │
└─────────────────────────────────────┘
```

### Sağlayıcı Kartı (Ayarlar Sekmesi)

```
┌──────────────────────────────────────┐
│ 🤖 OpenAI                            │
│ Model: GPT-4o                        │
│ Durum: ✅ Bağlandı                   │
│         [Yapılandır] [Devre Dışı]    │
└──────────────────────────────────────┘
```

---

## LLM Yönlendirme Önceliği

`callLLMStream()` içinde (app.go):

1. **Orkestra modu** (aktifse) → [[Orkestra Modu]]
2. **Harici sağlayıcı** (aktifse) → `provider.Router.ChatCompletionStream`
3. **Yerel llama.cpp** (fallback) → `api.Client`

Hiçbir sağlayıcı yapılandırılmamışsa ve yerel model çalışmıyorsa hata döner.

---

## Bilinen Sorunlar

| Sorun | Detay |
|-------|-------|
| **llama.cpp provider yok** | Yerel motor provider arayüzü üzerinden değil, ayrıca yönetilir |
| ~~**Priority alanı kullanılmıyor**~~ | ✅ Düzeltildi — router artık `Priority`'ye göre sıralıyor, UI'da gösteriyor |
| ~~**Test dosyası yok**~~ | ✅ Düzeltildi — `router_test.go` mevcut, 48 test `-race` ile geçiyor; `claude_test.go`/`openai_test.go` de eklendi |
| ~~**Orkestra router'ı bypass eder**~~ | ✅ Düzeltildi — `tryFallbackProviders` ile Orkestra'ya da yedek zincir eklendi |
| **Makineye bağlı şifreleme** | `providers.json` makineler arası taşınamaz |
| **CLI görev iptali tam değil** | `App.streamMu`'nun global "tek stream aynı anda" koruması CLI için yok — aynı chat'e iki mesaj birden gitmesin diye ayrı bir `cliJobs` kilidi var ama chat'ler arası hiç engelleme yok, kasıtlı |
| ~~**Claude provider'ı boş model alanı gönderiyordu**~~ | ✅ Düzeltildi (`fd6fdd2`, 2026-07-22, CRITICAL) — `ChatRequest.Model` boşsa hesaplanan fallback hiç kullanılmıyordu; Claude aktif provider'ken **her normal sohbet mesajı** Anthropic API'sine boş `"model": ""` gönderiyordu. Gemini/OpenAI etkilenmemişti. Regresyon testiyle kapatıldı. |

---

### Bağlantılı Notlar:
- [[Mimari Yapı]] — Sistem modül haritası
- [[Orkestra Modu]] — Provider'ların orkestrasyonda kullanımı
- [[Ajan Modu]] — Provider modellerle araç çağırma
- [[Geliştirici API Ağ Geçidi]] — Bu sağlayıcıları dışarıdan Anthropic/OpenAI-uyumlu bir araçla (Claude Code gibi) kullanma
- [[API Dökümantasyonu]] — Provider endpoint'leri
