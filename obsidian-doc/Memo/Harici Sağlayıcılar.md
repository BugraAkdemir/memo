# 🔌 Harici Sağlayıcılar

> **Paket:** `internal/provider/` (10 dosya, ~1700 satır)
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

## Desteklenen Sağlayıcılar (7 tür)

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

> **Not:** llama.cpp bir provider olarak uygulanmamıştır. Yerel modeller `api.Client` ile ayrıca yönetilir.

---

## Router ve Fallback Sistemi

**Dosya:** `internal/provider/router.go` (282 satır)

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
    // 7 disabled sağlayıcı döndürür:
    // - openai:    gpt-4o
    // - gemini:    gemini-2.0-flash
    // - grok:      grok-2
    // - groq:      mixtral-8x7b-32768
    // - claude:    claude-sonnet-4-20250514
    // - openrouter: openai/gpt-4o
    // - ollama:    llama3
}
```

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
| **Priority alanı kullanılmıyor** | Router sıralamada Priority'yi dikkate almaz |
| **Test dosyası yok** | `internal/provider/` için sıfır test |
| **Orkestra router'ı bypass eder** | Orkestra doğrudan provider oluşturur, fallback zinciri yok |
| **Makineye bağlı şifreleme** | `providers.json` makineler arası taşınamaz |

---

### Bağlantılı Notlar:
- [[Mimari Yapı]] — Sistem modül haritası
- [[Orkestra Modu]] — Provider'ların orkestrasyonda kullanımı
- [[Ajan Modu]] — Provider modellerle araç çağırma
- [[API Dökümantasyonu]] — Provider endpoint'leri
