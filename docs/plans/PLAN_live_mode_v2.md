# Plan: Live Mode v2 — Çoklu Motor + İki Modelli Agent Delegasyonu

**İlgili plan dosyaları:** `docs/plans/PLAN_voice_live_mode.md` (orijinal Live
Mode fikir aşaması) → `faz1` (temel loop, VAD, yerel Piper TTS) → `faz2` (TTS
provider router + yerel ses store) → `faz4` (AEC, spike aşamasında kaldı).
**Bu plan o serinin devamı değil, yeni ve ayrı bir dosya** — faz1/2/4 tek bir
motoru (yerel whisper+piper) derinleştiriyordu; bu plan tamamen farklı bir
eksen ekliyor: çoklu motor seçimi (Google Live / OpenAI Realtime / ElevenLabs
/ Custom) ve iki modelli agent delegasyonu. Faz numaralandırmasını
sürdürmek iki farklı roadmap'i tek changelog'da karıştırırdı, o yüzden ayrı
dosya.

**Branch:** `feature/live-mode-v2` (bu plan `main`'de değil, bu branch'te
yürütülür — büyük/riskli değişiklik, AGENTS.md'nin branching kuralı gereği).

**AGENTS.md'ye bağlılık (her alt-adım için bağlayıcı):** Her oturum
`AGENTS.md` + bu dosyayı okuyarak başlar, `handoff.md`'ye yeni kayıt
ekleyerek biter. Her alt-adım kendi doğrulama komutlarını çalıştırıp
sonucu yapıştırmadan "bitti" denemez. Oturum başına 1-2 alt-adım. Flutter'a
dokunan her değişiklik için TR+EN L10n girdisi aynı commit'te (kural #8) ve
Rule #8 grep'i temiz olmalı. Her yeşil birim kendi commit'i — Conventional
Commits, İngilizce detaylı body, asla AI attribution satırı yok.

---

## Mevcut Durum (kod okunarak + resmi API dokümanları taranarak doğrulandı, 2026-08-26)

### Bugünkü beta Live Mode ("Hey Memo") — dokunulacak/üzerine inşa edilecek katman

- **STT:** `internal/whisper/whisper.go` — whisper.cpp'yi subprocess olarak
  sarar, `Transcribe(ctx, audioData)` tüm WAV'ı `/inference`'a POST eder,
  streaming/partial transcript yok. `internal/app/stt.go`'daki
  `TranscribeAudio(audioData []byte)` (satır 284) gerçek canlı yol;
  `StartRecording`/`StopRecordingAndTranscribe` (159-281) ölü kod, dokunulmaz.
  Config: `config.WhisperConfig` (`internal/config/config.go:172-179`).
- **TTS:** `internal/tts/tts.go` — Piper'ı her çağrıda taze subprocess olarak
  çalıştırır. `internal/tts/filler.go`'daki `FillerCache` kısa dolgu
  seslerini (Hmm/Mm/Ah) önceden yerel olarak üretir, external provider'a
  hiç uğramaz. `internal/app/tts.go`'daki `SynthesizeSpeech(text)` (satır 73)
  önce `a.ttsRouter`'ı (external), sonra yerel Piper'ı dener. Config:
  `config.TTSConfig` (`internal/config/config.go:189-193`).
- **TTS external-provider sistemi (Faz 2, genişletilecek desen):**
  `internal/tts/{provider.go,router.go,config.go}` — `internal/provider/`'ın
  birebir eşleniği (`TTSProvider` arayüzü, `Router` öncelik sıralı fallback
  zinciri, `ConfigManager` → `data/tts_providers.json`, API key'ler
  `provider.DefaultMachineKey()` ile aynı paylaşılan anahtarla AES-GCM
  şifreli). Bugün sadece OpenAI TTS gerçek implementasyona sahip
  (`internal/tts/openai.go`); `ProviderElevenLabs` sabiti tanımlı ama
  implementasyonsuz stub. `/api/tts/providers[/test]` var, `/models`
  endpoint'i **yok** — ses adı bugün serbest metin alanı, keşif yok.
  **Kurala uyulacak:** yerel motor (Piper/whisper) Router'a hiç girmiyor,
  çağrı noktasında ayrı, sabit son-çare fallback olarak kalıyor.
- **Yerel ses store (Faz 2.6):** `internal/tts/voice_store.go` —
  HuggingFace `rhasspy/piper-voices`'tan elle seçilmiş 3 sesin
  indirme/seçim akışı, API key gerektirmiyor.
- **LLM routing'e bağlantısı yok:** `internal/app/llm.go`'daki
  `callLLMStream`'de "voice"/"live" için sıfır referans — Live Mode bugün
  tamamen **frontend orkestrasyonu**: Flutter `VoiceModeNotifier`, normal
  metin sohbetinin kullandığı `messagesProvider.notifier.sendMessage(text)`'i
  çağırıyor (bu da normal 3'lü `callLLMStream` yönlendirmesini tetikliyor),
  sonra ayrıca son asistan cevabını `synthesizeSpeech`'e veriyor.
- **Frontend:** `frontend/lib/core/live_mode_controller.dart` (`vad`
  paketiyle VAD, PCM16→WAV), `frontend/lib/providers/voice_mode_provider.dart`
  (`voiceModeProvider`, idle/listening/thinking/speaking durumları,
  `_generation` sayaç deseniyle barge-in). Tek UI girişi:
  `frontend/lib/widgets/chat_input.dart` (~satır 1179-1221)'deki mikrofon
  ikonu, `if (ref.watch(betaFeaturesProvider))` ile kapılı. **Ayrı bir Live
  Mode ekranı artık yok** (Faz 1 sonrası kaldırıldı). Ayarlar bugün
  `beta_features_tab.dart` (371 satır) içine dağılmış: `_LiveModeVoiceTest`,
  `TTSVoiceSection`, `TTSProviderSection`.

### Beta'dan mezuniyet emsali — Tailscale/RemoteAccess

`internal/config/config.go:101-106`'daki `Beta` alanının doc comment'i
bunu açıkça söylüyor: Tailscale tüneli Beta'dan mezun oldu, kendi bağımsız
`RemoteAccessConfig.Enabled`/`TunnelMode` alanlarına ve kendi bağımsız
`remote_access_tab.dart` sekmesine (index 15, `settings_group_providers`
grubu, `Beta`/`betaFeaturesProvider`'a sıfır referans) kavuştu. Bu plan
Live Mode için **aynı deseni** izliyor.

### Agent-mode / Orchestra / routing mimarisi (Part B için)

- `internal/agent/` — `Executor.RunStream` (executor.go:188-284) her
  çağıranın (Flutter, WhatsApp, Telegram, task loop) geçtiği tek giriş
  noktası; `Pipeline.RunStream` tool-call loop'unu (max 40 iterasyon)
  `AgentProvider.ChatCompletion` + native function-calling ile yürütür;
  `PermissionManager` danger-level (`safe|medium|dangerous`) + policy
  (`prompt|allow_once|allow_session|allow_forever|deny_once|deny_forever`)
  kontrolü yapar. `NewWebSearchExecutor`/`NewWhatsAppExecutor` (executor.go
  85-151) — mevcut bir executor'ın sandbox/permission/audit-log'unu
  paylaşan "scoped executor" deseni, doğrudan referans alınacak.
- **SSE taşıma sözleşmesi:** her `api.StreamChunk`
  (`internal/api/types.go:170-177`) aynı `data: {...}` satırında akar, ayrı
  `event:` alanı yok — tür ayrımı `FinishReason` string'iyle yapılır:
  `""` = gerçek cevap metni (tek concat edilen, `chat.go:113-124`'teki
  `drainToReply` kuralı), `"agent_event"` = `agent.AgentEvent` JSON'ı,
  `"status"`/`"activity"`/`"memory_used"` = diğer tipler. Yeni bir
  discriminator eklemek (ilerleme bildirimi için) bu kurala uyacak.
- **`callLLMStream` 3'lü öncelik sırası:** Orchestra → external provider
  (`a.activeProviderName`/`a.providerRouter`) → yerel llama.cpp (`a.client`).
  Tüm `App` süreci için tek "aktif provider" — "ana model" bu demek zaten,
  Part B'nin "ana modele devret"i bu mevcut routing'i çağırmak demek,
  paralel bir provider-seçim mekanizması icat etmeye gerek yok.
- **Kritik eşzamanlılık tuzağı:** `App.streamMu` (app.go:267) TEK GLOBAL
  kilit — herhangi bir `SendMessage*` çağrısı, başka bir stream sürerken
  anında "lütfen bekleyin" hatası döner, **tüm app süreci için**. Live
  session'ın delegasyon çağrısı bundan asla etkilenmemeli.
  `SendMessageStreamToAsAgent`/`drainSelfChatReply`
  (`internal/app/selfchat_permission.go`) — WhatsApp/Telegram self-chat'in
  kullandığı, izin çözümlemesini **per-request-ID** yapan (paylaşılan
  `a.agentExecutor`'ın global `bypassPermissions`/`autoPermission`
  bayraklarına dokunmayan) desen — ama hâlâ aynı global `a.streamMu`'dan
  geçiyor, bu yüzden tek başına yeterli değil.
  **Asıl kopyalanacak emsal: `SendCLIMessageStream`**
  (`internal/app/cli_stream.go:180`, doc comment 155-179) — `a.streamMu`'yu
  bilinçli olarak atlıyor, per-chat exclusivity `a.cliJobsMu`/
  `a.cliJobs map[string]context.CancelFunc` (app.go:269-277) ile, lifecycle
  `a.lifecycleCtx`'e bağlı. Part B'nin concurrency modeli bunu kopyalıyor,
  tool-execution modeli ise `drainSelfChatReply`'yi kopyalıyor — ikisinin
  birleşimi yeni bir primitif.
- **Orchestra neden uygun değil:** `internal/orchestra` senkron, tek seferlik
  plan→execute→synthesize pipeline'ı, `modelType == "local"`'ı açıkça
  reddediyor (conductor.go:272-286), arka planda sürüp poll edilebilen bir
  iş kavramı yok. Sadece `CreateProviderForType` yardımcı fonksiyonu ilgi
  çekici ama Part B'nin buna bile ihtiyacı yok (ana model zaten mevcut
  routing).

### Sağlayıcı API araştırması (resmi dokümanlardan doğrulandı, 2026-08-26)

- **Google Live (Gemini Live API):** WS URL:
  `wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent?key=API_KEY`
  (ephemeral token varyantı da var: `access_token` query param). İlk mesaj
  `{"setup": {"model": "models/...", "responseModalities": ["AUDIO"],
  "systemInstruction": {...}, "tools": [...]}}`. Ses: giriş 16-bit PCM
  16kHz, çıkış 16-bit PCM 24kHz, little-endian. Function calling destekli
  (`setup.tools`'a `functionDeclarations`). Model keşfi: `models.list`
  REST endpoint'i, `supportedGenerationMethods` alanında
  `"bidiGenerateContent"` geçen modeller filtrelenir — **asla hardcode
  edilmeyecek**.
- **OpenAI Realtime:** WS URL: `wss://api.openai.com/v1/realtime?model=...`,
  `Authorization: Bearer <key>` header. `session.update` event'i
  model/instructions/audio-format/tools alanlarını taşır. Ses: base64 PCM
  24000Hz, `input_audio_buffer.append`/`response.output_audio.delta`.
  Function calling: `session.tools`'a tanım, model
  `response.function_call_arguments.done` event'i döner, cevap
  `conversation.item.create` + `function_call_output` ile geri gönderilir.
  Model keşfi: `GET /v1/models`, realtime-yetenekli ID'ler filtrelenir
  (tam filtre alanı implementasyon sırasında doğrulanacak — açık risk).
- **ElevenLabs:** `GET /v1/models` (`can_do_text_to_speech` ile filtrele),
  `GET /v1/voices`, `xi-api-key` header. STT: `POST /v1/speech-to-text`
  (multipart file + `model_id`, Scribe v2) — bu oturumda doğrulandı, önceki
  "belirsiz" notu kapandı. **Conversational-AI/Agents platformu (kendi
  `agent_id` + kendi LLM beyni) kullanılmayacak** — ElevenLabs sadece saf
  STT/TTS katmanı olarak ele alınıyor, kullanıcı kararıyla.

---

## Kilitlenmiş Tasarım Kararları (kullanıcıyla netleştirildi, sapılmayacak)

1. **Google Live / OpenAI Realtime** = gerçekten düşünen native ses-ses
   modelleri. Seçildiğinde bu motorun kendi modeli hem ses I/O'yu hem
   "live model" beynini üstleniyor — ayrı bir beyin seçimi yok. Native
   function-calling ile `delegate_to_main_model` aracı veriliyor.
2. **Local / ElevenLabs / Custom** = saf STT+TTS, kendi muhakemesi yok. Ayrı
   bir "live model" kavramı YOK — transkript direkt mevcut ana modele
   gidiyor (bugünkü `callLLMStream` routing'i, aynen), o hem sohbet ediyor
   hem iş yapıyor. Part B'nin yeni mimarisi sadece native iki motor için
   gerekli.
3. **Custom** = kullanıcı tanımlı **OpenAI-uyumlu STT/TTS REST endpoint**
   (`/v1/audio/speech`, `/v1/audio/transcriptions` şekli) — WebSocket/live
   motor değil.
4. **`WorkMode: "delegate" | "standalone"`** (sadece Google Live/OpenAI
   Realtime'da anlamlı, default `"delegate"`): `"delegate"`'te live model'in
   tek aracı `delegate_to_main_model`; `"standalone"`'da live model'e
   **ana modelin agent-mode'da kullandığı tam tool registry** doğrudan
   veriliyor, ikinci bir model hiç devreye girmiyor — kullanıcının bilinçli
   tercihiyle, küçük/hızlı modelin doğrudan dosya/komut erişimi riski
   UI'da uyarı metniyle açıkça belirtiliyor.

---

## Mimari

### 1. Motor seçimi yeni bir primitif, `tts.Router`'ın uzantısı değil

`tts.Router`/`ConfigManager` "birkaç TTS sağlayıcısı, otomatik öncelik
fallback, stateless `(text)->bytes`" modelliyor — ElevenLabs/Custom TTS için
doğru şekil, ama Google Live/OpenAI Realtime için yanlış (onlar uzun ömürlü,
stateful, tam-duplex, request/response şekli olmayan oturumlar). **Karar:**
yeni `internal/livemode` paketi tek-aktif-motor seçimini (fallback listesi
değil) yönetir; ElevenLabs ve Custom gerçekten stateless STT/TTS çağrısı
olduğu için **mevcut** `tts.Router`'a/yeni `internal/stt` Router'ına
değişmeden oturuyor. Yerel Piper/whisper.cpp aynen kalıyor, hiç router'a
girmiyor.

### 2. Yeni Go paket yapısı

```
internal/livemode/
    engine.go            // EngineType, EngineConfig{Type,APIKey,Model,Voice,BaseURL,Enabled,WorkMode}
    config.go             // ConfigManager — data/livemode_engines.json, AES-GCM (provider.DefaultMachineKey())
    session.go            // Session arayüzü (Start/SendAudio/Events/Close) — google + openai_realtime ortak sözleşmesi
    google/{client.go,models.go}
    openai_realtime/{client.go,models.go}
    delegate_tool.go      // WorkMode'a göre tool-set builder: [delegate_to_main_model] ya da tam agent registry, provider-native formata çeviri

internal/tts/
    elevenlabs.go, elevenlabs_models.go, custom.go   // mevcut stub'ları tamamlar, Router'a değişmeden oturur

internal/stt/            // YENİ — bugün sadece whisper.cpp var, arayüz katmanı yok
    provider.go, elevenlabs.go, custom.go, router.go, config.go   // internal/tts ile birebir aynı şekil
```

`config.yaml`: `RemoteAccessConfig`'in yanına, `Beta`'dan bağımsız yeni
`LiveModeConfig{Enabled, ActiveEngine, WorkMode, AgentPermissionPolicy}`.
`data/livemode_engines.json`: motor başına şifreli config, `tts_providers.json`
deseninin aynısı.

### 3. REST/WebSocket yüzeyi — backend her zaman proxy

Flutter hiçbir zaman ham provider API key tutmaz — bu kod tabanındaki her
external-provider entegrasyonuyla (`internal/provider`, `internal/tts`)
tutarlı. Yeni REST: `/api/livemode/engines[/test|/models]`,
`/api/livemode/active`. Yeni WS: `/api/livemode/session` — **sadece**
Google Live/OpenAI Realtime aktifken kullanılır; backend Flutter'ın ham PCM
sesini provider'ın WS'ine köprüler, iki sağlayıcının farklı wire formatlarını
tek bir Dart-tarafı çerçeve şekline çevirir. Local/ElevenLabs/Custom mevcut
discrete STT-dosya→sohbet→TTS-dosya akışını **değiştirmeden** kullanmaya
devam eder — bu endpoint ve Part B'nin tüm delegasyon makinesi **sadece**
iki native motor için gerekli.

`go.mod`'da `github.com/coder/websocket` zaten indirect bağımlılık
(Tailscale/gosearch üzerinden) — direct'e terfi ettirilecek, ikinci bir WS
kütüphanesi eklenmeyecek.

### 4. Part B delegasyonu — yeni primitif, Orchestra'nın yeniden kullanımı değil

Global `a.streamMu` yüzünden normal `SendMessage*` yolu kullanılamaz. Yeni
`internal/app/livemode_delegate.go`, yeni metod
`App.SendLiveDelegatedMessageStream`:

- **Concurrency modeli `SendCLIMessageStream`'den kopyalanır:** per-job
  exclusivity `a.liveJobsMu`/`a.liveJobs map[string]context.CancelFunc`
  (global `a.streamMu` DEĞİL). Job lifetime, live session'ın kendi
  context'ine bağlı (session WS açıldığında oluşur, kapandığında iptal
  olur) — CLI job'lardan farklı olarak `a.lifecycleCtx`'ten daha kısa ömürlü,
  bu bilinçli bir sapma, ileride "düzeltilmemeli".
- **Tool-execution modeli `SendMessageStreamToAsAgent` +
  `drainSelfChatReply`'den kopyalanır:** özel arka plan sohbeti
  (`sessions.Manager.NewBackgroundChat`, `a.liveModeChatID`), agent mode
  her çağrıda zorlanır, `permission_request` olayları **per-request-ID**
  `HandleAgentPermission` ile çözülür — paylaşılan `a.agentExecutor`'ın
  global bayraklarına dokunulmaz.

**Tool tanımı:** Google Live `setup.tools`'a `functionDeclarations` girdisi,
OpenAI Realtime `session.update.tools`'a `type: "function"` girdisi olarak
`delegate_to_main_model`'i alır. Tool-call event'inde
`SendLiveDelegatedMessageStream` çağrılır, `drainLiveDelegatedReply`
(`drainSelfChatReply`'nin aynası) ile drain edilir, tamamlanınca provider'ın
native function-result mesajı (`toolResponse`/`function_call_output`)
gönderilir.

**İlerleme anlatımı en büyük belirsizlik:** bir function call açıkken
provider'ın oturuma metin ekletme desteği dokümanlarda net değil. Önce
garanti-çalışan fallback (sadece sonucu anlat, model çağrı sürerken doğal
"düşünüyor" davranışına girer) — canlı doğrulama sonrası ilerleme enjeksiyonu
ayrı bir adımda denenecek.

**Sesli izin isteme** (`AgentPermissionPolicy`: `"voice_prompt"` default —
Medium/Dangerous araçlar sesli soruluyor, transkript edilen cevap
`HandleAgentPermission`'a bağlanıyor; `"auto_allow_once"` — sormadan
onaylar) — gerçekten yeni bir etkileşim deseni, mevcut UI'nin tekrar
kullanımı değil.

### 4b. `WorkMode: "delegate" | "standalone"`

`"standalone"`'da live session'a `agent.Executor`'ın tam tool registry'si
(`registry.ToOpenAITools()`) provider-native formatta veriliyor, tool-call'lar
**doğrudan** yürütülüyor — ikinci model yok. Bunun için `agent.Executor`'a
yeni, dar bir metod: `ExecuteToolCall(ctx, sandbox, toolName, argsJSON)
(result, error)` (Pipeline'ın iç döngüsünden çıkarılmış, tekrar yazılmamış).
Kendi `Sandbox`'ı var (`NewWebSearchExecutor` deseni), aynı per-request-ID
izin çözümlemesi (`voice_prompt`) — artık delegasyon sınırında değil, her
gerçek araçta. `delegate_tool.go` bu yüzden sabit tek araç değil, `WorkMode`'a
göre `[delegate_to_main_model]` ya da tam registry döndüren bir tool-set
builder'a dönüşüyor. Frontend'de `WorkMode` seçici + uyarı metni (yeni L10n
key) sadece Google Live/OpenAI Realtime seçiliyken görünür/etkin.

### 5. System prompt / RAG hafıza enjeksiyonu (native live oturumları için)

`App.buildMessagesForSession` (`internal/app/helpers.go:108-131`) her mesajda
taze çalışır — mesaja özel RAG araması + `identity.BuildSystemPrompt`.
Kalıcı, tek seferlik setup'lı bir realtime oturumunda bunun birebir eşleniği
yok. İki ayrı enjeksiyon anı:

1. **Oturum başlangıcı system prompt'u (statik, yüksek güven):** oturum
   açılırken `identity.BuildSystemPrompt` **bir kere**, live-mode'a özel
   parametrelerle çağrılır (geniş/güncel bir ilk hafıza taraması + live
   model'in gerçek yeteneklerini dürüstçe anlatan yeni bir bayrak — "elinde
   `delegate_to_main_model` dışında araç yok" ya da standalone modda tam
   registry) — `identity.BuildSystemPrompt`'un imzasına küçük bir uzantı ya
   da `internal/identity` içinde ince bir `BuildLiveModeSystemPrompt`
   sarmalayıcısı, aynı persona/mood mantığını tekrar yazmadan.
2. **Oturum-içi hafıza tazeleme (dinamik, ilerleme-anlatımıyla aynı riski
   paylaşıyor ama daha yüksek güvenilirlikte):** her iki sağlayıcı da native
   ses kullanılırken bile canlı transkript sağlıyor (Google:
   `inputTranscription`/`outputTranscription`; OpenAI:
   `conversation.item.input_audio_transcription`/
   `response.audio_transcript.delta`) — backend bunları dinler,
   `a.retrieveMemory(ctx, transcriptText)` çalıştırır, sonucu turlar arası
   context injection ile (§4'teki ilerleme-anlatımıyla **aynı** primitif)
   oturuma ekler. Turlar arası enjeksiyon, bir function call açıkken
   enjeksiyondan daha standart/belgeli bir uzatılabilirlik noktası —
   bu yüzden delegasyon primitifiyle aynı fazda güvenle inşa edilebilir,
   ilerleme-anlatımı kendi ayrı, riski izole edilmiş adımında kalır.

### 6. Frontend

Yeni bağımsız sekme `frontend/lib/widgets/settings/tabs/live_mode_tab.dart`,
`settings_dialog.dart`'a kayıt (doğrulandı: sekmeler bugün 0-23,
`settings_group_providers = [5, 6, 15, 22, 23]`, `remote_access` index
15'te — yeni sekme index 24, aynı gruba eklenir; `_tabIcons`/`_tabs`/
`_buildTabContent` güncellenir; `settings_dialog_test.dart`'ın coverage
testi aynı commit'te güncellenir). İçerik `tts_provider_section.dart`'ın
alan desenine (API key → canlı-çekilen model dropdown → test butonu)
benziyor ama tek-aktif-motor, çoklu-liste değil. `chat_input.dart`'taki
`betaFeaturesProvider` kapısı kaldırılır, `beta_features_tab.dart`'taki
ilgili widget'lar taşınır.

`voice_mode_provider.dart`'ın mevcut `VoiceModeNotifier`'ı (half-duplex,
discrete-turn) **değişmeden** Local/ElevenLabs/Custom'ı beslemeye devam
eder. Google Live/OpenAI Realtime için **ayrı, yeni** bir
`LiveRealtimeSessionNotifier` (full-duplex, sürekli akış, turn-taking
sağlayıcı tarafında) — iki şekli tek state machine'e sıkıştırmak yerine.
Yeni bağımlılık: `web_socket_channel` (bugün `pubspec.yaml`'da yok).

Her yeni UI string'i (motor seçici, model dropdown durumları, `WorkMode`
seçici + uyarı metni, izin politikası dropdown'ı — tahmini 15-25 key)
aynı commit'te hem TR hem EN `l10n.dart`'a girer (kural #8).

---

## Alt-Adımlar (her biri kendi commit'i + doğrulaması, oturum başına 1-2 madde)

**0 — Kurulum:** ✅ branch (`feature/live-mode-v2`) oluşturuldu, bu plan
dosyası yazıldı.

**1 — Sadece config mezuniyeti** (henüz yeni motor yok): `LiveModeConfig`
`config.yaml`'da; Flutter `liveModeEnabledProvider`, `chat_input.dart`'tan
Beta kapısı kaldırılır, mevcut yerel-motor widget'ları çıplak
`live_mode_tab.dart`'a taşınır, sekme kaydedilir, coverage test güncellenir.

**2 — `internal/stt` paketi + `internal/tts`'in ElevenLabs/Custom
stub'larının tamamlanması**, artı model/ses keşif endpoint'leri
(`GET /v1/models` `can_do_text_to_speech` filtreli, `GET /v1/voices`;
ElevenLabs STT `POST /v1/speech-to-text` + `model_id`).

**3 — `internal/livemode` iskeleti + motor config CRUD** (henüz realtime
oturum yok): engine.go/config.go, App wiring, REST CRUD
(`/api/livemode/engines`, `/api/livemode/active`), `/models` hariç.
Flutter: tam motor seçici + config formları, model/ses alanları için
placeholder metin kutusu.

**4 — Canlı model keşfi:** Google (`ListLiveModels`,
`supportedGenerationMethods` içinde `bidiGenerateContent` filtresi) + OpenAI
(`ListRealtimeModels`, `GET /v1/models`'tan realtime-yetenekli ID filtresi)
+ REST `/models` endpoint'i. Flutter: placeholder'lar gerçek dropdown'lara
döner.

**5 — Local/ElevenLabs/Custom'ın gerçek ses döngüsüne bağlanması:**
`TranscribeAudio`/`SynthesizeSpeech` aktif motorun provider'ı üzerinden
dispatch eder. Bu tek başına (1-5 fazları) Part A'yı 5 motordan 3'ü için
sıfır delegasyon karmaşıklığıyla tam olarak teslim eder.

**6 — WS köprü iskeleti** (`internal/livemode/session.go`,
`coder/websocket` direct bağımlılığa terfi, `/api/livemode/session` stub/echo
session ile) + Flutter `LiveRealtimeSessionNotifier`/`web_socket_channel` —
gerçek sağlayıcılara dokunmadan duplex taşımayı kanıtlar.

**7 — Google Live client'ı** (sadece setup/audio, henüz function-calling
yok), gerçek köprüye bağlanır.

**8 — OpenAI Realtime client'ı** (7'nin aynası, bağımsız mesaj şekilleriyle).

**9 — Part B delegasyon primitifi** (`"delegate"` modu, backend-only,
provider-agnostic): `SendLiveDelegatedMessageStream`, per-job concurrency
map, özel arka plan sohbeti, `drainLiveDelegatedReply`/izin çözümlemesi —
gerçek provider olmadan unit-test edilebilir, `selfchat_permission_test.go`
ile aynı desen. Aynı fazda: `"standalone"` modu için
`agent.Executor.ExecuteToolCall` tek-araç sarmalayıcısı.

**10 — Her iki `WorkMode`'un realtime client'lara bağlanması:** tool-set
builder + tool-call routing (delegate → `SendLiveDelegatedMessageStream`;
standalone → `ExecuteToolCall`) + sadece final-result anlatımı (henüz
ilerleme enjeksiyonu yok). Aynı fazda: oturum başlangıcı system prompt'u
(§5.1). Frontend: `WorkMode` seçici.

**11 — İlerleme anlatımı + oturum-içi hafıza tazeleme:** her iki API'ye
karşı canlı doğrulama (§5.2'nin iki tüketicisi — hafıza tazeleme daha
yüksek güvenilirlikte, delegasyon ilerlemesi daha düşük); hangisi
çalışıyorsa o gönderilir, sonuç ne olursa olsun belgelenir — Faz 10'un
zaten çalışan final-result delegasyonunu ve statik system prompt'u
bloklamayacak şekilde izole.

**12 — Sesli izin isteme** (`voice_prompt` politikası) uçtan uca.

**13 — Temizlik:** `beta_features_tab.dart`'ın akıbeti (muhtemelen
neredeyse boş kalacak — öyleyse kaldır, sekme index'lerini yeniden
numarala, coverage testi güncelle), plan dosyasının kapanış "Durum" bölümü
+ `handoff.md` kaydı.

---

## Açık Riskler (erken fazları bloklamasın, ama bilinçli takip edilsin)

- **Bu ortamda hiçbir gerçek provider API key'i yok** — Google Live/OpenAI
  Realtime/ElevenLabs'a dokunan her faz sadece `httptest` ile doğrulanabilir,
  gerçek API ile asla — her ilgili fazın durum kaydında açıkça belirtilecek
  (Faz 2'nin kendi kapanış notunun aynı dürüstlük geleneği).
- **OpenAI'nin WebRTC önerisine karşı WebSocket-üzerinden-Go-backend-relay
  seçimi** — mimari tutarlılık için WebSocket seçildi; Faz 8'in gerçek
  cihaz testinde gecikme kabul edilemez çıkarsa, belgelenmiş bir çıkış yolu
  var: Flutter, backend'in ürettiği kısa ömürlü ephemeral token ile OpenAI'a
  doğrudan WebRTC açar — sadece OpenAI'a özel bir sapma, genel politika
  değişikliği değil.
- **Live Mode'un kendi `perms.liveMode` hesap-izni kapısına ihtiyacı olup
  olmadığı** (WhatsApp/Telegram/remote_access'in zaten böyle bir kapısı var)
  — tutarlılık için eklenecek varsayılan, Faz 1'den sonra yanlış çıkarsa
  gözden geçirilecek.
- **`"standalone"` modu küçük/hızlı live model'e doğrudan dosya/komut erişimi
  veriyor** — kullanıcının bilinçli tercihi, `voice_prompt` izin politikası
  gerçek güvenlik ağı.
- **Native live oturumları mevcut per-turn RAG/identity enjeksiyon
  pipeline'ını olduğu gibi kullanamıyor** — §5 tasarımı bunu
  `identity.BuildSystemPrompt`'u tekrar yazmadan, oturum-başı + oturum-içi
  ikili bir bölünmeyle çözüyor; oturum-içi yarı, ilerleme-anlatımıyla aynı
  belirsizlik riskini taşıyor (Faz 11).
- **ElevenLabs Realtime STT'nin tam filtre alanı / OpenAI modeller
  endpoint'inin realtime-filtre alanı** implementasyon anında son kez
  doğrulanacak — mimariyi değiştirmez, sadece Faz 2/4'te bir doküman
  kontrolü gerektirir.

---

## Doğrulama (her Go/Dart'a dokunan faz için zorunlu)

```bash
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race

export PATH="$PATH:/home/bugra/Documents/flutter/bin"
cd frontend && flutter analyze lib/ && flutter test

git diff --name-only -- '*.dart' | xargs -r grep -nE "(Text|Tooltip|SnackBar|AlertDialog)\(\s*['\"][A-Za-zÇĞİÖŞÜçğıöşü]"
```

Gerçek provider key'leriyle uçtan uca sesli test bu ortamda mümkün değil —
her ilgili fazda "doğrulanmadı" olarak açıkça belirtilecek, sessizce
çalışıyor varsayılmayacak.

## Durum

- 2026-08-26: Plan onaylandı, `feature/live-mode-v2` branch'i açıldı, bu
  dosya yazıldı. Faz 0 tamam.
- 2026-08-26: Faz 1 tamam (backend `bb1d7fe`, frontend `d311d34`) — config
  mezuniyeti, yeni "Sesli Mod" sekmesi, beta kapısı kaldırıldı.
- 2026-08-26: Faz 2 tamam (`69a9dfe`) — `internal/stt` paketi (ElevenLabs +
  Custom STT), `internal/tts`'in ElevenLabs/Custom stub'ları tamamlandı,
  `GET /v1/models`+`GET /v1/voices` canlı keşif + `/api/tts/providers/
  models`/`voices` endpoint'leri, `/api/stt/providers` CRUD. Tüm yeni HTTP
  çağrı noktaları httptest ile doğrulandı (gerçek API key yok). Frontend
  tarafına henüz dokunulmadı — model/ses dropdown'ları Faz 3/4'te.
