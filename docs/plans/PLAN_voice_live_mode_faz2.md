# Plan: Voice Live Mode — Faz 2 (TTS Provider Router + Store)

**Üst plan:** `docs/plans/PLAN_voice_live_mode.md` (Faz 2 tanımı: "TTS Store +
Provider Router: yerel/API TTS seçimi, dile göre model önerisi").
**Önceki faz:** `docs/plans/PLAN_voice_live_mode_faz1.md` — Faz 1 "prototip
olarak koda döküldü" durumunda, iki bilinçli açık maddesi var (barge-in yok,
VAD modeli hâlâ CDN'den iniyor). Kullanıcı kararıyla bunlar kapatılmadan
Faz 2'ye geçiliyor.

---

## Mevcut Durum (kod okunarak doğrulandı, `codebase-memory` + doğrudan okuma — 2026-07-29)

### Faz 1'den kalan TTS altyapısı (dokunulmayacak, üzerine inşa edilecek)

- `internal/tts.Synthesizer` (`tts.go`) — Piper'ı her çağrıda taze bir
  subprocess olarak çalıştırır, `Synthesize(ctx, text) ([]byte, error)`
  (WAV döner). Kalıcı server/port yok.
- `config.TTSConfig` (`internal/config/config.go:157`) — tek, sabit yerel
  motor: `BinaryPath`, `ModelPath` (.onnx), `Enabled`. **Bu adı/şekli
  değişmiyor** — yeni external-provider config'i ayrı bir alan olacak (bkz.
  2.1).
- `App.initTTS()`/`App.SynthesizeSpeech(text)` (`internal/app/tts.go`) — tek
  bir `a.ttsSynthesizer *tts.Synthesizer` alanını sarar, direkt Piper'a gider.
  Faz 2.4 bu fonksiyonu genişletecek.
- `FullBridge.SynthesizeSpeech` → `POST /api/tts/synthesize`
  (`handlers_flutter.go:1895`) → Flutter `api_client.dart`'ın
  `synthesizeSpeech(text)`'i → Beta Features sekmesindeki test widget'ı.
  Bu zincir **değişmeden** kalır (imzası aynı, sadece içi genişler).

### LLM tarafının aynı problemi nasıl çözdüğü — birebir örnek alınacak desen

`internal/provider/`:
- `Provider` arayüzü (`provider.go:34`) — `Name()`, `DisplayName()`,
  `ChatCompletion`, `ChatCompletionStream`, `ListModels`.
- `ProviderConfig` (`provider.go:149`) — `Type`, `Name`, `APIKey`, `BaseURL`,
  `Model`, `Enabled`, `Priority`, bağlantı durumu (`Connected`/`Error`,
  persist edilmiyor).
- `Router` (`router.go:15`) — `[]*providerEntry` (config + `failCount` +
  `disabled`), `Priority`'ye göre azalan sıra, 3 ardışık hatada
  `auto-disable`, `HealthCheck` ile periyodik geri-deneme, `UpdateConfigs`
  ile tam yeniden inşa.
- `ConfigManager` (`config.go:26`, `NewConfigManager(filePath, masterKey)`)
  — `providers.json`'a persist, API key'ler `machine.key` ile şifreli.
- `App` tarafında: `a.providerCfgMgr *provider.ConfigManager`,
  `a.providerRouter *provider.Router` (+ `providerMu`), `UpdateProvider`/
  `DeleteProvider`/`GetProviders` (`internal/app/providers.go`) config'i
  kaydedip router'ı yeniden kuruyor.
- REST: `/api/providers`, `/api/providers/test`, `/api/providers/models`,
  `/api/providers/active` (`server.go:221-224`).
- **Önemli mimari ayrım — TTS'e de aynen taşınacak:** LLM tarafında yerel
  llama.cpp, `provider.Router`'ın **içinde bir entry değil** — ayrı,
  üçüncü bir katman (`callLLMStream`'in kendi 3'lü öncelik sırası: Orchestra
  → external Router → local llama.cpp, `internal/app/llm.go`). Faz 2 TTS
  için de aynısı yapılacak: Faz 1'in `tts.Synthesizer` (yerel Piper)
  **hiç değişmeden**, yeni `tts.Router`'ın dışında, çağrı noktasında
  (`SynthesizeSpeech`, adım 2.4) son fallback olarak kalacak — yerelı de
  Router'ın bir "provider"ı yapıp Faz 1 kodunu yeniden yazmaya gerek yok.

### Model indirme deseni (Faz 2.6 "TTS Store" için referans)

`internal/modelstore/modelstore.go` — HuggingFace arama/indirme/yerel liste
(GGUF sohbet modelleri için). Piper ses modelleri de HF'de
(`rhasspy/piper-voices` reposu) barındırılıyor, aynı `SearchModels`/
`DownloadModel`/`ListLocalModels` deseni **dil ekseniyle** (donanım değil)
yeniden kullanılabilir — ama bu, kendi başına büyük bir alt-adım (2.6),
bu oturumun kapsamına girmiyor, sadece referans olarak not düşülüyor.

---

## Faz 2 Alt-Adımları (Faz 1'in kuralı: her biri kendi commit'i + doğrulaması, oturum başına 1-2 madde)

### 2.1 — `internal/tts`: `TTSProvider` arayüzü + `Router` (external-only)

`internal/provider/{provider.go,router.go}`'nun birebir eşleniği, TTS'e
özel alan farklarıyla:

```go
// internal/tts/provider.go
type ProviderType string
const (
    ProviderOpenAI     ProviderType = "openai"      // https://api.openai.com/v1/audio/speech
    ProviderElevenLabs ProviderType = "elevenlabs"   // 2.2'de sadece OpenAI yapılacak, ElevenLabs iskeleti (Validate + tip) burada, implementasyonu ayrı bir adım
)

type TTSProvider interface {
    Name() ProviderType
    DisplayName() string
    Synthesize(ctx context.Context, text, voice string) ([]byte, error) // WAV/MP3 döner
}

type ProviderConfig struct {
    Type     ProviderType `json:"type"`
    Name     string       `json:"name"`
    APIKey   string       `json:"api_key,omitempty"`
    Voice    string       `json:"voice"`    // sağlayıcıya özel ses adı (örn. "alloy")
    Enabled  bool         `json:"enabled"`
    Priority int          `json:"priority"`
    Connected bool        `json:"connected,omitempty"`
    Error     string      `json:"error,omitempty"`
}
func (c ProviderConfig) Validate() error { ... } // Type, Voice zorunlu; APIKey provider'a göre zorunlu

func NewProvider(cfg ProviderConfig) (TTSProvider, error) { ... } // 2.1'de sadece switch iskeleti, case'ler 2.2+'da eklenir
```

`internal/tts/router.go` — `provider.Router`'ın birebir kopyası
(`providerEntry`, `failCount`/`disabled`, `Priority` sıralaması,
`UpdateConfigs`, `Synthesize` (ChatCompletion'ın yerine), `HealthCheck` YOK
— TTS çağrıları LLM kadar sık değil, periyodik health-check bu fazda
gereksiz karmaşıklık, `recordFailure`/`ReenableProvider` yeterli).

**ChatCompletion → Synthesize fallback döngüsü birebir aynı mantık**
(sırayla dene, hata → `failCount++`, 3'te disable, context iptali ayrı
ele alınır).

Test: `internal/tts/router_test.go`, `provider/router_test.go`'nun aynı
test iskeletini (sahte/stub provider'larla fallback, auto-disable,
re-enable) TTS'e uyarlar.

**Bu adımda henüz gerçek bir external provider implementasyonu yok** —
sadece arayüz + router + boş `NewProvider` switch'i. Faz 1'in
`tts.Synthesizer`'ına dokunulmuyor.

### 2.2 — İlk gerçek external provider: OpenAI TTS

`internal/tts/openai.go` — `POST https://api.openai.com/v1/audio/speech`,
body `{"model":"tts-1","input":text,"voice":cfg.Voice}`, `Authorization:
Bearer <APIKey>`, cevap ham audio byte (varsayılan format mp3). Mevcut
`internal/provider/openai.go`'nun HTTP-çağrı iskeletini (client, header,
hata mesajı unwrap — `provider.ExtractErrorMessage` **aynı fonksiyon
tekrar kullanılabilir**, TTS'e özel değil) örnek alır, streaming yok
(TTS cevabı tek parça byte, chat'in SSE'siyle karışmaz).

`NewProvider`'daki `ProviderOpenAI` case'i buraya bağlanır. Test:
`openai_test.go`, `httptest` ile sahte `/v1/audio/speech` endpoint'i.

**ElevenLabs veya başka bir ikinci external provider bu adıma dahil
değil** — kapsam bilinçli olarak tek provider'a sınırlı, ikinci
sağlayıcı ayrı bir küçük adım olarak sonradan eklenebilir (Router zaten
çoklu provider'ı destekliyor, ekleme maliyeti düşük).

### 2.3 — Persistence + `App` wiring + REST

`provider.ConfigManager`'ın (`internal/provider/config.go`) TTS eşleniği:
`internal/tts/config.go`, `data/tts_providers.json`'a persist, aynı
`machine.key` (`a.machineKey`, zaten LLM provider'ları için kullanılan
alan — **yeni bir anahtar üretilmiyor**, mevcut olan paylaşılıyor) ile
API key şifreleme.

`App`: `a.ttsProviderCfgMgr *tts.ConfigManager`, `a.ttsRouter *tts.Router`
(+ `ttsRouterMu`), `internal/app/tts_providers.go` (yeni dosya) —
`GetTTSProviders`/`UpdateTTSProvider`/`DeleteTTSProvider`/
`TestTTSProviderConnection`, `providers.go`'daki eşlenik fonksiyonların
birebir kopyası.

REST (`server.go`, mevcut `/api/providers*` bloğunun hemen altına):
`/api/tts/providers`, `/api/tts/providers/test`. (`/api/tts/providers/models`
yok — TTS'te "model listesi" kavramı yok, ses adı sabit/elle girilen bir
alan, `ListModels`'in eşleniği gereksiz.) `FullBridge`'e eklenir (Flutter-only,
`ImportMemoryFromText`/`SynthesizeSpeech` ile aynı seviye), `AppBridge`'e
değil.

### 2.4 — Çağrı noktası: `SynthesizeSpeech` external→local öncelik sırası

`internal/app/tts.go`'nun `SynthesizeSpeech` fonksiyonu, `callLLMStream`'in
3'lü önceliğinin sadeleştirilmiş (Orchestra yok) 2'li versiyonu:

```go
func (a *App) SynthesizeSpeech(text string) ([]byte, error) {
    if router := a.ttsRouter; router != nil && router.HasActiveProvider() {
        if audio, err := router.Synthesize(ctx, text); err == nil {
            return audio, nil
        }
        // düş, yerel Piper'a devam et — external tamamen yoksa da aynı yol
    }
    // mevcut Piper kodu, değişmeden
}
```

Yerel Piper hâlâ **son, garanti fallback** — hiç external provider
yapılandırılmamışsa (Faz 1'deki gibi) davranış birebir aynı kalır,
regresyon riski yok. Test: `tts_test.go`'ya external-router-hatası →
local-fallback senaryosu (mevcut `llm_test.go`'daki benzer external→local
fallback testlerinin deseniyle).

### 2.5 — Flutter: TTS provider ayar ekranı

`frontend/lib/widgets/provider_config_dialog.dart`'ın küçültülmüş bir
eşleniği (`tts_provider_config_dialog.dart`) — Type dropdown (şimdilik
sadece OpenAI), API key alanı, Voice text field, Priority, Enabled toggle,
Test butonu. Beta Features'taki mevcut `_LiveModeVoiceTest` widget'ının
yanına/Ayarlar'a yeni bir "Sesli Yanıt Sağlayıcıları" girişi. `api_client.dart`'a
`getTTSProviders`/`updateTTSProvider`/`deleteTTSProvider`/`testTTSProvider`
(mevcut provider eşleniklerinin birebir kopyası). Yeni L10n anahtarları
TR+EN (AGENTS.md kural #8).

### 2.6 — "TTS Store": dile göre ses modeli önerisi/indirme (kapsamı büyük, ayrı planlanmalı)

Üst planın "TTS Store" kısmı — `internal/modelstore` deseninin
`rhasspy/piper-voices` HF reposuna, dil ekseniyle uyarlanması (donanım
yerine `Whisper.Language`/`Identity.UILanguage`'a göre öneri). **Bu adım
2.1-2.5'ten bağımsız çalışır** (Piper'ın hangi ses dosyasını kullandığı
zaten `config.TTS.ModelPath` ile elle ayarlanıyor, Store sadece bunu
otomatikleştiriyor) — kendi dosya-bazlı alt-planını hak edecek kadar büyük,
bu dosyanın kapsamına şimdilik sadece madde olarak düşülüyor, detaylandırma
2.1-2.5 bittikten sonra ayrı bir oturumda yapılacak.

---

## Durum (2026-07-29, güncellendi) — 2.1-2.5 TAMAMLANDI

Kullanıcının açık talebiyle ("komple çalışır hale getir") tek oturumda
2.1'den 2.5'e kadar tamamlandı — her alt-adım (ve bazı alt-adımlar kendi
içinde de, kritik dosya gruplarına göre) kendi ayrı commit'iyle:

- **2.1** — `internal/tts/{provider,router}.go` + testleri.
- **2.2** — `internal/tts/openai.go` (gerçek OpenAI TTS implementasyonu,
  `response_format=wav` sabitlenmiş çünkü `handleTTSSynthesize` her yanıtı
  `Content-Type: audio/wav` ile işaretliyor).
- **2.3** — üç ayrı commit'e bölündü:
  - `provider.DefaultMachineKey()` export (internal/tts'in aynı machine.key'i
    paylaşması için, ikinci bir anahtar dosyası icat etmeden).
  - `internal/tts/config.go` (`ConfigManager` — `data/tts_providers.json`,
    AES-256-GCM şifreli API key'ler).
  - `App` wiring (`internal/app/tts_providers.go` + `app.go`'daki struct
    alanları/Startup çağrısı) ve REST katmanı (`bridge.go`,
    `handlers_flutter.go`, `server.go`, `swarm_stub_bridge_test.go`) — ayrı
    commit'ler.
- **2.4** — `SynthesizeSpeech` artık external router'ı önce deniyor, hata/
  yapılandırılmamışsa yerel Piper'a düşüyor (`callLLMStream`'in
  external→local önceliğiyle aynı desen).
- **2.5** — Flutter: `TTSProviderConfig` modeli + `api_client.dart` CRUD
  metodları (ayrı commit), `TTSProviderSection` widget'ı + Beta
  Features'a bağlanması + 19 yeni L10n anahtarı TR+EN (ayrı commit).

**2.6 de aynı oturumda tamamlandı** (kullanıcının "asıl mesele local/offline
çalışması, TTS Store'u atladın" geri bildirimi üzerine, önce yanlışlıkla
sadece external/API-key'li sağlayıcı tarafı bitmiş gibi rapor edilmişti —
bu düzeltildi): `internal/tts/voice_store.go`'nun `VoiceStore`'u,
`rhasspy/piper-voices` HF reposundan küçük, elle seçilmiş bir katalogla
(tr_TR-fahrettin-medium, en_US-lessac-medium, en_US-amy-medium) **hiç API
anahtarı gerektirmeden** ses modeli indirip yerel Piper motorunu
(`config.TTS.ModelPath`) otomatik yapılandırıyor. App wiring
(`tts_voices.go`, `GetSelectedTTSVoicePath` dahil), REST
(`/api/tts/voices`, `/download`, `/select`) ve Flutter UI
(`TTSVoiceSection`, Beta Features'ta `TTSProviderSection`'dan **önce**
gösteriliyor — local-first öncelik sırasını yansıtmak için) hepsi ayrı
commit'lerle tamamlandı.

**Hâlâ kapsam dışı:** ElevenLabs sağlayıcısı implementasyonsuz (tip
tanımlı, seçilirse net hata). Gerçek bir external TTS API anahtarıyla
canlı uçtan uca test yapılmadı — sadece `httptest` ile sahte sunucuya
karşı. Curated ses kataloğunun ötesinde (upstream reponun tamamını
arama/gözatma) bilinçli olarak yapılmadı — küçük, güvenilir bir liste
tercih edildi.

## Doğrulama

Her commit kendi başına: `go build -tags "sqlite_fts5" ./...`,
`go vet -tags "sqlite_fts5" ./...`, ilgili paketlerde `go test ... -race`.
Flutter tarafı: `flutter analyze lib/` (sadece 4 önceden var olan, alakasız
info), `flutter test` 114/114. Gerçek bir external TTS API anahtarı bu
ortamda yok — her external provider testi `httptest` ile sahte HTTP
sunucusuna karşı yazıldı, gerçek API'ye canlı çağrı yapılmadı. Gerçek
Piper/VAD binary'si de yok (Faz 1'den kalma sınırlama) — Live Mode ekranının
gerçek bir sesli döngüsü bu ortamda hiç çalıştırılmadı.
