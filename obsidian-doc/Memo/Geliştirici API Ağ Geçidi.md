# 🧩 Geliştirici API Ağ Geçidi

> **Paket:** `internal/anthropicapi/` (wire-format çevirisi), `internal/app/devgateway.go` (yönlendirme), `internal/webserver/devgateway_handlers.go` (HTTP)
> **Yapılandırma:** `config.DevGatewayConfig` (`require_api_key`, `use_memory`) — ikisi de varsayılan kapalı
> **API endpoint'leri:** `GET/PUT /api/dev-gateway/config`, `GET /api/dev-gateway/models`, `POST /v1/messages`
> **Ayarlar sekmesi:** Ayarlar → Geliştirici

Memo'yu, sadece Anthropic-uyumlu bir adres kabul eden dış araçlarla (en başta **Claude Code**'un kendisi, `ANTHROPIC_BASE_URL` üzerinden) kullanılabilir hale getirir. Amaç: Claude Code'u Memo'ya bağlayıp, arkada aslında kendi yerel modelini ya da kendi OpenAI/Gemini/vb. API anahtarını kullanmak.

---

## Neden var

Claude Code gibi araçlar sadece Anthropic'in Messages API formatını (`POST /v1/messages`) konuşur — başka bir sağlayıcıyı doğrudan kullanamazsın. Bu ağ geçidi, Memo'yu bu iki tarafın arasına koyar: dışarıdan Anthropic formatında bir istek gelir, Memo bunu kendi iç temsiline çevirir, hangi backend'e (yerel model ya da tanımlı bir sağlayıcı) gideceğine karar verir, cevabı alır ve tekrar Anthropic formatına çevirip geri yollar.

`internal/provider/claude.go` Memo'nun gerçek Anthropic API'sine **istemci** olarak konuştuğu yer — bu paket (`internal/anthropicapi`) bunun tam tersi: Memo'nun kendisi bir Anthropic **sunucusu** gibi davranıyor.

---

## Model seçimi: `type/model-id`

İsteğin `"model"` alanı `<tip>/<model-id>` formatında olmalı:

| Örnek            | Ne olur                                                                                                                        |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `local/qwen2.5`  | Yüklü olan yerel llama.cpp modeli kullanılır (model-id kısmı sadece etiket, gerçek model zaten yüklü olan)                     |
| `openai/gpt-4o`  | Ayarlar → API Providers'da **tipi `openai` olan ve etkin (enabled)** ilk sağlayıcı kullanılır, model `gpt-4o` olarak ayarlanır |
| `custom/qwen2.5` | Tipi `custom` (kendi OpenAI-uyumlu endpoint'in — LM Studio, vLLM, vb.) olan etkin sağlayıcı                                    |

Aynı tipten birden fazla sağlayıcı tanımlıysa **etkin (enabled) olan** kullanılır — hangisinin kullanılacağını seçmek için ayrı bir arayüz yok (bilinçli basitleştirme).

`GET /api/dev-gateway/models` şu an hangi `type/model-id`'lerin kullanılabilir olduğunu listeler — Ayarlar → Geliştirici sekmesi bunu kopyalanabilir bir liste olarak gösterir.

---

## Kimlik doğrulama

`DevGateway.RequireAPIKey` **varsayılan kapalı** — tıpkı localhost erişiminin zaten kimlik doğrulaması istemediği gibi (bkz. `remoteAuthOK`). Açılırsa:

- İstek `x-api-key` header'ını taşımalı (gerçek Anthropic istemcilerinin — Claude Code dahil — otomatik gönderdiği header) ya da `Authorization: Bearer <token>` (alternatif).
- Token, Uzaktan Erişim'in kullandığı **aynı token** (`RemoteAccess.Token`) — Ayarlar → Geliştirici sekmesinde kopyalanabilir gösterilir.
- Bu kontrol, mevcut `remoteAuthMiddleware`'den **bağımsız**: o sadece Memo `0.0.0.0`'a bağlıyken devreye girer, bu ise local/uzak fark etmeksizin her zaman `RequireAPIKey` ayarına göre çalışır — amaç, aynı makinedeki başka bir sürecin izinsiz bu portu kullanmasını engellemek.

---

## Hafıza entegrasyonu

`DevGateway.UseMemory` **varsayılan kapalı**. Açılırsa:

- İstek, Memo'nun RAG hafızasından ilgili bilgi bloğu ile zenginleştirilir (kullanıcının son mesajına göre `retrieveMemory` + `memory.FormatMemoriesForPrompt`) — dış aracın kendi sistem promptunun geri kalanına (persona, yetenek duyuruları) **dokunulmaz**, sadece hatırlanan gerçekler eklenir.
- Tur bittiğinde `saveMemoryAsync` ile hafızaya kaydedilir.
- **Ama hiçbir zaman görünür bir sohbet oturumu oluşturmaz** — Claude Code üzerinden yapılan bir kodlama sohbeti, Memo'nun sohbet geçmişinde asla görünmez. Bu bilinçli bir tasarım kararı: ağ geçidi trafiği ile gerçek sohbetler birbirine karışmasın diye.

Kapalıyken (varsayılan): istekler tamamen izole, hafızaya hiç dokunulmaz.

---

## Araç çağırma (Tool Use) — tam agentic destek

Claude Code'un asıl gücü olan araç çağırma (dosya okuma/yazma, komut çalıştırma) **çalışır**: isteğin `"tools"` alanı, Anthropic'in `tool_use`/`tool_result` blokları ve çok turlu devam (bir aracın sonucunu bir sonraki mesajda geri gönderme) hepsi çevriliyor.

**Nasıl çalışıyor:**
- Anthropic'in `input_schema`'sı zaten OpenAI'ın `parameters` alanının beklediği JSON Schema ile aynı — neredeyse doğrudan eşleniyor.
- Asistanın önceki bir `tool_use` bloğu → OpenAI'ın `tool_calls` alanına çevriliyor; kullanıcının `tool_result` bloğu → ayrı bir `role: "tool"` mesajına çevriliyor (OpenAI araç sonuçlarını kullanıcı mesajının içine gömülü değil, ayrı bir mesaj olarak bekliyor).
- **Önemli format detayı:** Anthropic'in `tool_use.input`'u gerçek bir JSON nesnesi, OpenAI'ın `function.arguments`'ı ise o nesnenin metnini taşıyan bir JSON **string**'i — ikisi karıştırılırsa (örn. doğrudan aynı bytes'ı kullanmak) ya çifte kodlama ya da Claude Code'un `input` alanında bir nesne yerine düz metin görmesi gibi sinsi bir hata oluşuyor. `anthropicInputToOpenAIArguments`/`openAIArgumentsToJSONText` bu ikisi arasında tam ters çeviriyi yapıyor — canlı bir uçtan uca testte bu hata gerçekten yakalandı ve düzeltildi.
- Araç çağıran istekler her zaman **non-streaming** olarak backend'e gidiyor (`DevGatewayChat`) — Memo'nun kendi agent pipeline'ı (`internal/agent/pipeline.go`) da araç çağırma kararını hep non-streaming `ChatCompletion` ile alıyor, hiçbir sağlayıcının streaming tarafı `tool_calls` delta'larını çözmüyor zaten. İstemci streaming istediyse, tamamlanmış cevap tek seferde Anthropic'in SSE event dizisi olarak "yeniden oynatılıyor".

**Bilinen sınırlama:** `gemini`, `claude`, `ollama` tipi sağlayıcılar için araç çağırma henüz desteklenmiyor — bu üçünün `internal/provider` içindeki kendi implementasyonları Tools/ToolCalls'ı hiç çözmüyor (ağ geçidinden bağımsız, önceden var olan bir eksiklik). Bu tiplerden birine araç tanımlı bir istek gelirse, sessizce araçları düşürmek yerine açık bir hata dönülür.

Token sayıları **tahmini** (kelime sayısına dayalı) — gerçek sağlayıcının raporladığı kesin sayılar değil, kod tabanının geri kalanındaki canlı sayaçla aynı yaklaşım.

---

## İlgili sayfalar

- [[Harici Sağlayıcılar]] — ağ geçidinin yönlendirdiği sağlayıcı sistemi
- [[RAG ve Semantik Hafıza]] — hafıza entegrasyonunun dayandığı sistem
- [[API Dokümantasyonu]] — tüm REST endpoint'leri
