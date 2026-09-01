# Memo — Proje Dosya Haritası

Bu dosya, projedeki her kaynak dosyanın ne işe yaradığını kısaca açıklayan bir
referans ağaçtır. Elle güncel tutulur — büyük bir refactor sonrası tekrar
oluşturulması gerekebilir.

---

## 1. Go Backend

### Kök dizin

```
main.go          — CLI giriş noktası: headless REST API sunucusu ya da interaktif terminal REPL'i başlatır
embed.go         — go:embed ile binaries/ placeholder'ı ve version dosyasını binary'ye gömer
```

### cmd/

```
cmd/proactive-demo/
  main.go — sahte bir alışkanlık (habit) uydurup tüm proaktif öğrenme pipeline'ını çalıştıran CLI demo
```

### internal/agent/ — Agent/tool çalıştırma motoru

```
internal/agent/
  executor.go        — Executor: registry, permissions, sandbox'ı birbirine bağlayan üst düzey orkestrasyon
  pipeline.go         — AgentEvent tipleri ve tool-calling döngüsünü event olarak akıtan pipeline (maxIters=40)
  execute_tool_call.go — Executor.ExecuteToolCall: tek bir tool çağrısının izin-kontrolü+çalıştırma+event-yayınlama akışı (pipeline.go'dan çıkarıldı)
  sandbox.go          — SandboxConfig: rate limit, timeout, korumalı yollar (tool güvenliği)
  permissions.go      — PermissionManager: tool+argüman bazlı izin politikalarını diske kaydeder
  permissions_test.go — izin politikası wildcard eşleşmesi ve tek seferlik reddin temizlenmesi testleri
  tools.go            — ToolDef/ToolRegistry ve DangerLevel sınıflandırması; registerBuiltins()'te 27 yerleşik tool kayıtlı (bkz. Ajan Modu doc'undaki tam liste)
  backup.go           — BackupManager: agent dosya düzenlemelerinden önce anlık yedek alır, geri yükler
  backup_test.go      — BackupManager oluşturma/geri yükleme round-trip testleri

internal/agent/tools/  — 27 yerleşik tool'un implementasyonları (internal/agent/tools.go'daki registerBuiltins() kayıtlarının ExecuteFn'leri), aşağıdaki dosyalara dağılmış
  file.go             — read_file/write_file/delete_file/list_directory/get_file_info tool'ları: sandbox'lanmış taban yol içinde dosya G/Ç
  edit.go             — edit_file/insert_line/delete_lines tool'ları: mevcut dosyalara string/satır aralığı değişikliği uygular
  edit_test.go        — EditFile satır ve string-replace davranışı testleri
  changedir.go        — change_directory tool'u: ajanın sandbox tabanını sohbetin geri kalanı için değiştirir
  search.go           — search_files tool'u: proje dizininde desen/glob arama
  command.go          — run_command tool'u: 43 desenlik kara liste + timeout korumasıyla shell komutu çalıştırır; RunCapturingOutput (pipe değil geçici dosya ile çıktı yakalama — arkaplana atılan süreçler artık takılmıyor)
  sysproc_unix.go / sysproc_windows.go — platform-özel process-group kurulumu/öldürme (command.go'nun arkaplan süreç desteği için)
  websearch.go        — web_search tool'u: internal/websearch'ü agent'tan çağrılabilir hale getirir
  fetchpage.go        — fetch_page tool'u: bir URL'nin tam içeriğini Markdown'a çevirir
  fetchbudget.go       — WithFetchBudget: bir istekte fetch_page'in en fazla 5 farklı domain deneyebilmesini sınırlayan context bütçesi
  whatsapp.go         — whatsapp_send/search/latest/messages tool bağlamaları, enjekte edilen client arayüzü üzerinden (ayrı, kapsamlandırılmış registry'de)
  calendar.go         — get_calendar_events tool'u
  routine.go          — create_routine/list_routines/cancel_routine tool'ları
  provider.go         — configure_provider tool'u: agent'ın kendi sağlayıcı ayarlarını değiştirmesine izin verir
  selfclone.go        — self_clone tool'u: çalışan Memo binary'sini/projeyi başka dizine kopyalar
  selfclone_test.go   — SelfClone'un kendine/alt dizinine klonlamaya karşı korumalarının testi
  sendfile.go         — share_file tool'u: bir dosya/klasörü (klasörse zip'leyerek) bu konuşmaya geri gönderir
  taskmd_tools.go     — create_task_md/edit_task_md tool'ları
  taskstatus.go       — get_task_status/pause_task/resume_task tool'ları
  selfdrivingtask.go  — start_self_driving_task tool'u
  sandboxctx.go        — WithSandboxSetter/SandboxSetterFromContext: change_directory'nin sandbox'ı çalışma zamanında değiştirebilmesi için context köprüsü
  l10n.go              — SetUILanguage/T: bu paketin kendi TR/EN metin yardımcıları (tools.go'nun `internal/app`'e bağımlı olmadan yerelleştirme yapabilmesi için)
  calendar_test.go, command_test.go, file_test.go, provider_test.go, readonly_allowlist_test.go — testler
```

### internal/agentcli/ — Claude Code / Codex CLI'ı sohbet sağlayıcısına çeviren köprü (beta)

```
internal/agentcli/
  claude_code.go          — Claude Code CLI'ı provider.Provider arayüzüne saran, alt-süreç olarak çalıştıran implementasyon
  codex.go                — Codex CLI için aynı köprünün karşılığı
  commands.go              — CLI'ın kendi `/` komutlarını (.claude/commands, .codex/prompts, skill'ler) keşfeder
  commands_reported.go     — komut listesini Memo'nun `/` popup'ına raporlar (proje/kişisel/skill/yerleşik etiketiyle)
  commands_reported_test.go, commands_test.go, models_test.go, claude_code_test.go, codex_test.go — testler
  models.go                — CLI sağlayıcı için sahte/statik model listesi
  sysproc_unix.go/sysproc_windows.go — alt süreç grubu kurulumu (process-group kill için)
```

> `internal/provider`'ın kendisi değil — bunlar HTTP çağrısı değil, dosya düzenleyebilen/komut çalıştırabilen durumlu (stateful) yerel süreçler, kendi oturum/auth modelleriyle.

### internal/anthropicapi/ — Developer API Gateway (Sidebar → Developer)

```
internal/anthropicapi/
  anthropicapi.go          — Anthropic Messages API'sinin (ANTHROPIC_BASE_URL) sunucu tarafı implementasyonu; internal/provider/claude.go'nun (istemci tarafı) aynadaki karşılığı — Claude Code gibi araçların Memo'yu gerçek Anthropic API'si sanıp yerel modele/kendi API key'lerine yönlendirmesini sağlar
  anthropicapi_test.go     — testler
```

### internal/routine/ — Rutinler (zamanlanmış otomasyonlar)

```
internal/routine/
  types.go        — Routine veri tipi ve zamanlama alanları
  extractor.go     — doğal dil açıklamasından bir Routine yapılandırması çıkarır (LLM tabanlı)
  loop.go          — zamanlanmış rutinleri tetikleyen arka plan döngüsü, cihaz saat dilimi ofsetiyle
  store.go         — data/routines/*.json — rutin başına bir dosya
  *_test.go        — testler
```

### internal/stats/ — Kullanım İstatistikleri (Settings → Stats)

```
internal/stats/
  store.go       — her tamamlanan sohbet turunu (yerel/agent/orchestra/harici sağlayıcı) data/usage.db'ye kaydeder; incognito'da kayıt yok
  store_test.go  — testler
```

### internal/swarm/ — Memo Swarm (beta)

```
internal/swarm/
  room.go       — Host/Join oda kodu, katılımcı listesi, pay (yüzde) yönetimi
  worker.go     — llama.cpp rpc-server ile bir makinenin işlem gücünü havuza katma
  *_test.go     — testler
```

> macOS'ta henüz yok (yardımcı binary orada paketlenmiyor).

### internal/taskloop/ — Self-Driving otonom çok-adımlı görev döngüsü motoru (v4.4.0'da büyük genişleme — bu bölüm o genişlemeyle güncellendi, önceki hâli sadece 3 dosya listeliyordu)

```
internal/taskloop/
  engine.go          — worker/review-chief döngüsü: processItem bir maddeyi çalıştırır (busy-chat/rate-limit/transient/auth hata sınıflandırmasıyla), chief'e onaylatır, sıradakine geçer; planlayıcı/uygulayıcı modunu da yönetir
  schema.go           — Task.md formatının tek insan/LLM-yüzlü tanımı: TaskMdSchemaDoc() (model+kullanıcıya gösterilen spec), TaskMdTemplate(), RenderTaskMd() — taskmd.go'daki parser'la schema_parse_sync_test.go üzerinden senkron tutulur
  taskmd.go           — ParseTaskMd: Task.md'yi header/intro/madde/not olarak ayrıştıran gerçek parser; NotifyLevel (# bildirim: header'ından)
  plan.go             — Plan: planlayıcı modunun çıktısı (küçük, tek tek doğrulanabilir adımlar + kabul kontrolleri + bağımlılık DAG'ı), Plan.md render/parse
  runtime.go           — RunningTaskInfo: `task_list` komutu ve GET /api/tasks/running için çalışan bir görevin canlı görünümü (store + engine'in bellek-içi durumu birleşimi)
  notify.go            — Notification: bir taskloop olayının kullanıcının bildirim kanallarına yönlendirilmesi (NotifyBus)
  retry.go             — RetryScheduler: "waiting-limit" durumundaki listeler için tek seferlik zamanlayıcılar (rate-limit sonrası aynı maddeden devam)
  provider_err.go      — providerFailKind: worker-turu hatasının en-iyi-çaba sınıflandırması (auth/rate-limit/transient) — döngünün bekle mi/sağlayıcı değiştir mi/vazgeç mi kararını besler; ErrChatBusy + IsChatBusyErr de burada
  subagent.go          — SubRole ve SubAgentOrchestrator.Spawn: büyük bir madde için 1 yazma-yetkili coder + paralel en fazla 3 salt-okunur analyzer/reviewer/test-runner alt-ajanı
  store.go            — görev listesi kalıcılığı (JSON store)
  *_test.go            — testler (engine_chatbusy/concurrency/mirror/planexec/planning/selfheal_test.go dahil, canlı senaryoları taklit eden geniş bir test seti)
```

### internal/tts/ — Live Mode metin-konuşma (Piper + opsiyonel OpenAI TTS)

```
internal/tts/
  tts.go             — Piper binary'sini saran Synthesizer (yerel, çevrimdışı, varsayılan)
  openai.go           — opsiyonel harici OpenAI TTS sağlayıcısı
  router.go           — hangi TTS motorunun kullanılacağına karar verir, hata durumunda yerel Piper'a düşer
  filler.go            — cevap üretilirken çalan kısa "düşünme" sesi (hmm/mm/ah)
  voice_store.go       — Hugging Face'ten küçük, elle seçilmiş Piper ses koleksiyonunu indirip yöneten yerel katalog
  *_test.go            — testler
```

### internal/shutdown/, internal/gguf/, internal/jsonutil/, internal/browseropen/ — küçük paylaşılan yardımcı paketler

```
internal/shutdown/shutdown.go     — main()'in seçtiği (select) süreç-geneli graceful-shutdown sinyali; eski os.Signal-tabanlı yöntemin Windows'ta sessiz no-op olması sorununu çözer
internal/gguf/gguf.go             — bir GGUF dosyasının sadece metadata bölümünü okuyup gerçek maksimum context ve tool-calling desteğini tespit eder (tensor verisine hiç dokunmaz)
internal/jsonutil/jsonutil.go     — paylaşılan JSON yardımcıları
internal/browseropen/browseropen.go — varsayılan tarayıcıda URL açma (CLI'ın --github/--bugreport/--docs bayrakları ve Tailscale giriş akışı tarafından kullanılır)
```

### internal/api/ — LLM API istemcisi (OpenAI uyumlu)

```
internal/api/
  client.go     — OpenAI uyumlu chat completion API'si için HTTP Client (yerel/uzak)
  streaming.go  — SSE stream decoder ve <think> etiketi ayrıştırıcısı (reasoning/normal metin ayrımı)
  types.go      — Message/ContentPart/ImageURL istek-cevap tip tanımları
  api_test.go   — Message constructor'larının (metin ve multimodal) testleri
```

### internal/app/ — Merkezi orkestratör (en büyük paket)

```
internal/app/
  app.go              — App struct'ının merkezi tanımı, Startup/Shutdown ile tüm alt sistemleri bağlar
  app_test.go         — App'in temel getter'larının (config, stiller, sistem promptu) testleri
  chat.go             — SendMessage(Stream) giriş noktaları, gizli mod, görsel/dosya ekli sohbet
  llm.go              — Sohbet çağrılarını agent/orchestra/provider/yerel LLM arasında yönlendirip akıtır
  llama.go            — Yerel llama.cpp sohbet modeli yaşam döngüsü: başlat/durdur/durum/config/kurulum
  embedding.go        — Ayrılmış embedding llama-server'ının başlat/durdur/durum yönetimi
  memory.go           — RAG hafıza kaydet/getir/birleştir, hafıza ayarları, elle hafıza içe/dışa aktarma
  models.go           — HuggingFace model arama/indirme ve yerel model yönetimi köprüsü
  providers.go        — Harici LLM sağlayıcıları için CRUD ve bağlantı testi
  orchestra.go        — Orchestra (çoklu-model şef) yapılandırmasını al/güncelle
  proactive.go        — Proaktif öğrenme motorunun Decider'ını ve öneri yayıcısını App'e bağlar
  learning.go         — Intent extractor ve takvim deposunu App'e bağlar (initLearning)
  sessions.go         — Sohbet oturumu yaşam döngüsü: yeni sohbet/agent sohbeti, session manager erişimi
  agent.go            — Agent modu aç/kapa ve izin-cevabı köprüsü (Executor'a)
  cliadmin.go         — `memo` terminal CLI wrapper'ını kurar/kaldırır (~/.memo, ~/.local/bin)
  cliadmin_test.go    — RemoveCLI'ın CLI giriş dosyalarını sildiğinin testi
  settings.go         — kimlik/mood/self-interest/web-search/sistem-yönetimi ayar get/set'leri
  skill.go            — Skill kurulum/liste/kaldırma ve aktif skill prompt enjeksiyonu
  stt.go              — whisper konuşma-metin sunucu kontrolü ve ses kayıt/transkripsiyon
  sync.go             — Google Drive bulut senkronizasyon auth, tetikleme, ayarlar, bağlantı kesme köprüsü
  remote.go           — Uzak erişim durumu/config: web sunucu portu, tokenlar, ngrok anahtarları
  remote_tailscale.go — Yerel web sunucusuna gömülü Tailscale tünelini başlatır/durdurur
  whatsapp.go         — WhatsApp eşleştirme, sohbet akışı, arama ve durum köprü metodları
  version.go          — Uygulama versiyonu getter'ı ve versiyon endpoint'ine karşı güncelleme kontrolü
  backup.go           — Tam veri dışa/içe aktarma (.memo zip), içe aktarma öncesi anlık görüntü, tüm veriyi silme
  helpers.go          — mesaj/context builder'ları, token bütçesi, dosya indirme/kopyalama yardımcıları
  helpers_test.go     — buildMessages'in mood-disabled sistem promptu temizleme testi
  shutdown_test.go    — Shutdown'ın zamanında biten temizlik için zorla çıkış yapmadığının testi
  file_mentions.go    — ListProjectFiles: sohbette `@dosya` anmak için proje dosyalarını arar
  install_id.go       — InstallID: kuruluma özel rastgele kimlik (BUG-ONB10 fix'i, data dizini silinince yok olur)
  onboarding.go       — GetOnboardingComplete: ilk-çalıştırma sihirbazının tamamlanıp tamamlanmadığı
  insight.go          — GenerateSelfInsight (`/insight`): ruh hali/hafıza geçmişinden gerçek bir örüntü varsa anlatır
  routine.go          — Rutinler alt sisteminin App'e bağlanması: initRoutines, Decider — bkz. internal/routine/
  proactive_ambient.go — Proaktif öneri (ambient nudge) penceresinin aktif olup olmadığı ve prompt'a eklenecek blok
  stats.go            — Kullanım istatistikleri alt sisteminin App'e bağlanması — bkz. internal/stats/
  telegram.go / telegram_l10n.go — Telegram bot köprüsü durum/mesajlaşma metodları + TR/EN şablon metinleri
  server_browse.go    — BrowseServerPath: sunucu tarafı dosya sistemi tarayıcısı (dosya seçme dialog'u için)
  sendfile.go         — share_file agent aracının arkası: bir indirme linki (outbox) üretip süreli olarak sunar
  swarm.go            — Memo Swarm (beta) alt sisteminin App'e bağlanması — bkz. internal/swarm/
  stt_providers.go / tts_providers.go — data/stt_providers.json + data/tts_providers.json'dan STT/TTS sağlayıcı listesini yükler
  tts.go / tts_voices.go — Piper synthesizer kurulumu (initTTS) + indirilebilir ses kataloğu (GetTTSVoiceCatalog)
  clients.go          — Bağlı istemcilerin (client_id) canlı/durgun takibi — bir istemci durgunlaşınca temizlenir
  remote_auth.go      — RemoteDeviceInfo: istemcilere gösterilen cihaz listesi DTO'su
  claudecodecli.go / cli_status.go / cli_stream.go — Claude Code/Codex CLI sağlayıcı köprüsü: bağlantı durumu, sohbet-bazlı CLI provider/workdir ayarları, CLI stream başlatma
  devgateway.go / devgatewaylog.go — Geliştirici API Ağ Geçidi (Sidebar → Developer) config'i + canlı istek/yanıt günlüğü
  agent_chat_context.go / selfchat_context.go / selfchat_permission.go — hangi ajan sohbetinin/self-chat yüzeyinin (WhatsApp/Telegram) şu an aktif turu işlediğini taşıyan context anahtarları + self-chat'e özel izin/yanıt akışı
  chat_locks.go       — Sohbet-bazlı stream kilidi: busyStreamChan, withChatLockWait — bir Self-Driving turu meşgul bir sohbeti kuyruğa alıp bekleyebilir
  retry.go            — retryWithBackoff: genel amaçlı, anında+tekrarlı geri çağırma yardımcı fonksiyonu
  livemode.go / livemode_delegate.go / livemode_session.go / livemode_session_wrapper.go / livemode_voice.go — Live Mode v2 (native audio-to-audio): motor/mod/izin doğrulama, delegate sohbeti, gerçek livemode.Session kurulumu, sesli izin-onay sarmalayıcısı, motor bazlı sentezleme/transkripsiyon
  memory_import.go    — "Hafızayı İçe Aktar": başka bir AI'ın özetini atomik gerçeklere bölen JSON çıkarma yardımcıları
```

**Self-Driving görev döngüsü — App köprü katmanı** (`internal/taskloop`'u backend'e bağlayan dosyalar, v4.4.0'da eklendi):

```
internal/app/
  tasklist.go           — CreateTaskListFromTaskMd: bir Task.md dosyasını ayrıştırıp bir görev listesini tohumlar
  tasklist_run.go        — taskRunConfig: çalışan bir görev listesinin hangi sağlayıcı/model'i kullandığının özel görünümü
  tasklist_stepexec.go   — runPlanStep (planlayıcı modu adım çalıştırıcı) + emitToolActivity/toolVerbs (canlı görev kartı aktivite satırları)
  tasklist_stream.go     — streamIdleTimeout: bir worker turu için "bu kadar süre yeni token yok → bırak" bütçesi
  tasklist_planner.go    — planTask: "planlayıcı" modu için WithPlanner callback'i, Plan.md üretimi
  tasklist_planconfig.go — planTaskConfig: WithPlanConfig callback'i, liste başına bir kere self-configuration
  tasklist_roleconfig.go — roleModels: planlayıcı/kodlayıcı/doğrulayıcı rolleri için çözümlenmiş model
  tasklist_selfheal.go   — healTaskProvider: WithSelfHeal callback'i, sağlayıcı hatasına tepki (geçiş mi/bekle mi)
  tasklist_escalate.go   — escalateStep: WithEscalator callback'i, takılan bir adımın hedefli yeniden-planlanması
  tasklist_settings.go   — GetTaskLoopSettings: kalıcı task-loop config'i
  task_control.go        — taskSurface: bir kontrol mesajının hangi köprüden (chat/API) geldiğini tanımlar
  task_events.go         — taskChatEvent: bir Self-Driving olayının, güncel ilerleme bilgisiyle zenginleştirilmiş hâli
  task_notify.go         — taskNotifyQueueSize/taskNotifySendTimeout: bildirim pompasının sınırları (engine goroutine'ini bloklamaz)
  taskmd_tool.go         — taskMdEditorAdapter: create_task_md / edit_task_md agent araçlarının arkası
  taskstatus_tool.go     — taskStatusToolAdapter: get_task_status agent aracının arkası
  selfdriving_task_tool.go — selfDrivingTaskToolAdapter: start_self_driving_task agent aracının arkası
  subagent_runner.go     — appSubAgentRunner: taskloop.SubAgentRunner'ı uygular, her Run bir efemer alt-ajan turu
```

### internal/calendar/

```
internal/calendar/
  event.go          — paket dokümantasyonu, Event struct'ı ve Source sabitleri (sohbet/whatsapp/elle)
  store.go          — SQLite tabanlı Store: takvim olayları için şema ve CRUD
  reminder.go       — ReminderLoop: yaklaşan olayları yoklar, süre öncesi bildirim üretir
  bridge.go         — AddFromIntent: bir intent.IntentResult'ı kalıcı bir takvim Event'ine çevirir
  calendar_test.go  — store kurulum/kapatma yardımcı fonksiyonu ve olay kalıcılığı testleri
```

### internal/cloudsync/

```
internal/cloudsync/
  crypto.go            — Parola/donanım ID'sinden PBKDF2 ile AES-256-GCM şifreleme/çözme
  crypto_test.go       — şifrele/çöz round-trip doğruluğu testi
  drive.go             — Google Drive API istemcisi: appDataFolder'da yedek yükle/listele/budama
  sync_manager.go      — Manager: etkileşimleri sayar, her N mesajda zip+şifrele+yükle yapar
  sync_manager_test.go — SQLite WAL yardımcı dosyalarının arşive dahil edildiğinin testi
```

### internal/config/

```
internal/config/
  config.go       — AppConfig struct'ı, YAML yükle/kaydet ve platforma duyarlı DataDir() çözümü
  config_test.go  — Default() config değerlerinin (API, hafıza, llama varsayılanları) testi
```

### internal/database/

```
internal/database/
  sqlite.go          — DB wrapper: sqlite3 üzerinde yazmaları tek bir worker kanalından geçirerek serileştirir
  vec_register.go     — sqlite-vec eklentisi driver'ını vektör arama desteği için kaydeder
  sqlite_test.go      — Open'ın DB dosyası için eksik üst dizinleri oluşturduğunun testi
```

### internal/fileutil/

```
internal/fileutil/
  atomic.go       — AtomicWrite: tmp dosya + rename ile çökmeye dayanıklı dosya yazma (Windows fallback'iyle)
  atomic_test.go  — AtomicWrite'ın doğru içerik yazıp geri okuduğunun testi
```

### internal/identity/

```
internal/identity/
  identity.go       — Identity struct'ı: isim/stil/rol ve hafızalardan sistem promptunu inşa eder
  identity_test.go  — Identity'nin varsayılan ve özel-rol değerleriyle inşasının testi
  styles.go         — styleMap: hazır iletişim stili prompt metinleri (resmi, samimi, teknik, yaratıcı)
```

### internal/intent/

```
internal/intent/
  result.go             — paket dokümantasyonu ve IntentResult struct'ı (takvim olayı vs alışkanlık vs yok)
  filter.go             — regex tabanlı, dile bağımsız zaman/tarih anahtar kelime ön-filtresi (MightHaveIntent)
  extractor.go          — Extractor: iki aşamalı pipeline (anahtar kelime filtresi sonra LLM), IntentResult üretir
  decider_factory.go    — BuildDecider: intent LLM çağrılarını Orchestra ya da tek model üzerinden yönlendirir
  intent_test.go        — MightHaveIntent'in Türkçe/İngilizce zaman ifadeleri üzerindeki testi
```

### internal/llama/ — llama.cpp süreç yönetimi

```
internal/llama/
  llama.go             — Server: llama-server alt sürecinin yaşam döngüsünü ve durumunu yönetir
  installer.go         — Mevcut platform için llama-server binary release'ini indirir/açar
  gpu.go               — GPU üretici/VRAM tespiti (NVIDIA/AMD/Metal/CPU), katman-offload önerisi için
  llama_test.go        — GPU tipi sabitleri ve katman-önerisi mantığının testi
  process_unix.go      — unix süreç kontrolü: SIGTERM, canlılık kontrolü, süreç-grubu zorla öldürme
  process_windows.go   — Windows süreç kontrolü: Kill ve tasklist tabanlı canlılık kontrolü
  sysproc_darwin.go    — macOS SysProcAttr: alt süreç grubunu izole etmek için Setpgid
  sysproc_linux.go     — Linux SysProcAttr: alt süreç grubunu izole etmek için Setpgid
  sysproc_other.go     — diğer unix-benzeri platformlar için yedek SysProcAttr
  sysram_darwin.go     — macOS'ta sysctl hw.memsize ile toplam RAM tespiti
  sysram_linux.go      — Linux'ta /proc/meminfo ayrıştırarak toplam RAM tespiti
  sysram_other.go      — desteklenmeyen platformlarda 0 dönen RAM tespit stub'ı
  sysram_windows.go    — GlobalMemoryStatusEx Win32 API'siyle toplam RAM tespiti
```

### internal/logx/

```
internal/logx/
  logx.go       — slog tabanlı yapılandırılmış logger; seviye kontrolü ve Printf uyumlu API, SetOutput ile yönlendirme
  logx_test.go  — seviye ayarlama, debug modu ve Debug/Info/Warn/Error çıktısı testleri
```

### internal/lora/

```
internal/lora/  — .go dosyası yok; sadece önceden derlenmiş bir LoRA eğitim binary'si/CMake build dizini içeriyor
```

### internal/memory/ — RAG vektör hafızası

```
internal/memory/
  store.go         — SQLite tabanlı vektör/FTS hafıza deposu: etkileşim kaydet, getir, birleştir (en büyük dosya, ~1800 satır)
  embedder.go      — api.Client.CreateEmbedding'i store için bir EmbeddingFunc'a sarar
  chunker.go       — uzun metni örtüşen kelime-tabanlı parçalara böler (embedding için)
  chunker_test.go  — chunkText'in kısa/uzun/örtüşme davranışı testleri
  store_test.go    — etkileşim kaydetme ve benzer bağlam getirme testleri
```

### internal/models/

```
internal/models/
  memory.go — paylaşılan Memory/MemoryResult/MemoryFileInfo/SearchParams veri tipleri
```

### internal/modelstore/

```
internal/modelstore/
  modelstore.go       — Hugging Face model arama, GGUF indirme/içe aktarma/silme, yerel model listeleme
  modelstore_test.go  — isEmbeddingModel sezgiselinin ve sanitizePath/unsanitizePath testleri
```

### internal/mood/

```
internal/mood/
  engine.go     — mood skorlama motoru: config, skor güncellemeleri, açma/kapama, stokastik gürültü
  scorer.go     — kullanıcı metninden çıplak sayısal skor üreten LLM tabanlı duygu skorlayıcı
  prompt.go     — mood etiketlerini sistem promptu için İngilizce davranışsal direktif metnine eşler
  store.go      — mevcut mood skoru ve geçmişi için SQLite şema/kalıcılık
  sysinfo.go    — self-interest özellikleri için sistem anlık görüntüsü (hostname, OS, uptime)
  mood_test.go  — motor kırpma, skorlama ve geçici DB üzerinden kalıcılık testleri
```

### internal/ngrok/

```
internal/ngrok/
  installer.go     — mevcut OS/mimari için ngrok binary'sini indirir ve açar
  manager.go       — ngrok sürecini başlatır/durdurur, yerel API'den genel tünel URL'sini okur
  proc_unix.go     — ngrok alt süreci için Unix süreç-grubu kurulumu ve killProcessTree
  proc_windows.go  — Windows taskkill tabanlı süreç ağacı sonlandırma
  ngrok_test.go    — NewManager varsayılanları ve installer arşiv açma testleri
```

### internal/observer/ — kullanım deseni öğrenme

```
internal/observer/
  store.go               — paket dokümantasyonu + ham kullanıcı-aktivite Observation'ları için SQLite deposu
  recorder.go            — Observation'ları arka planda, bloklamadan biriktirip yazan hook
  pattern.go             — TimePattern struct'ı, güven azalması (decay) ve JSON dosya PatternStore CRUD'u
  analyzer.go            — ham gözlemleri periyodik olarak azalan/ağırlıklı TimePattern'lere çevirir
  analyzer_test.go       — sentetik gözlem dizilerinden desen tespiti testi
  pattern_test.go        — PatternStore'un paralel erişim altında concurrency/race testi
  store_test.go          — Record'un türetilmiş gün/günün-saati alanlarını doğru doldurduğunun testi
  intent_record_test.go  — beyan edilen alışkanlık saatlerinin (mesaj gönderme zamanı değil) kaydedildiğinin testi
```

### internal/orchestra/ — çoklu-model "şef" orkestrasyonu

```
internal/orchestra/
  conductor.go                 — "şef + roller" orkestrasyonu: planla, çalıştır, tekrar dene, sentezle
  roles.go                     — her orchestra rolü için yerleşik Türkçe sistem promptları (planlayıcı, frontend, …)
  types.go                     — ProgressUpdate/OrchestraTask/OrchestraConfig tipleri ve config Sanitize()
  types_test.go                — Sanitize'ın model id'lerindeki fazla boşlukları temizlediğinin regresyon testi
  conductor_config_test.go     — MergeRoles'un yerleşik rolleri koruduğu ve config yükle/kaydet testleri
  conductor_exec_test.go       — mock sağlayıcılara karşı sıralı görev çalıştırma testi
  conductor_helpers_test.go    — orchestra testleri için paylaşılan mockProvider/mockFactory yardımcıları
  conductor_plan_test.go       — extractJSON'ın plan cevaplarını (çıplak/kod-bloklu JSON) ayrıştırma testi
  conductor_retry_test.go      — retryTask başarı/tümü-başarısız tekrar deneme davranışı testi
  conductor_run_test.go        — RunWithProgress iptal ve ilerleme akışı testi
  conductor_synthesis_test.go  — synthesize()'ın görev sonuçlarını nihai cevaba birleştirme testi
```

### internal/proactive/ — proaktif öneri motoru

```
internal/proactive/
  matcher.go            — paket dokümantasyonu + bir deseni "şimdi" ile Gaussian zaman-of-day skorlaması
  engine.go              — desenleri şimdiki zamanla eşleştiren ve Şef'e harekete geçilip geçilmeyeceğini soran ticker
  decision.go            — Şef LLM çıktısını bir Action/Decision'a ayrıştırır (yok/bildir/öner/otomatik)
  feedback.go            — kullanıcı cevap metnini Outcome ve güven-delta ayarına eşler
  pending.go             — tek seferlik PendingSuggestion ve TTL'i için JSON dosya deposu
  prompt.go              — ana modeli proaktif karar-verici yapan Türkçe sistem promptu
  proactive_test.go      — Match, feedback ayrıştırma ve karar ayrıştırma için birim testleri
  integration_test.go    — desen eşleştir → öner → kabul et pipeline'ının uçtan uca testi
```

### internal/provider/ — harici LLM sağlayıcıları

```
internal/provider/
  provider.go       — ProviderType enum'u ve ortak Provider arayüzü/istek-cevap tipleri
  config.go         — sağlayıcı API anahtarlarını saklamak/yüklemek için şifrelenmiş ConfigManager
  router.go         — hata durumunda yedeğe geçen, yapılandırılmış sağlayıcılar arasında seçim yapan Router
  router_test.go    — Router sağlayıcı seçimi ve mock ChatCompletion davranışı testleri
  openai.go         — OpenAI API sağlayıcı implementasyonu, diğer OpenAI-uyumlularca paylaşılır/yeniden kullanılır
  claude.go         — Anthropic Claude API sağlayıcısı (sohbet + streaming)
  gemini.go         — Google Gemini API sağlayıcısı (sohbet + streaming)
  grok.go           — xAI Grok sağlayıcısı, OpenAI-uyumlu istemcinin ince sarmalayıcısı
  groq.go           — Groq sağlayıcısı, OpenAI-uyumlu istemcinin ince sarmalayıcısı
  ollama.go         — Ollama sağlayıcısı, OpenAI-uyumlu istemcinin ince sarmalayıcısı
  openrouter.go     — OpenRouter sağlayıcısı, OpenAI-uyumlu istemcinin ince sarmalayıcısı
  llamacpp.go       — OpenAI-uyumlu endpoint'i üzerinden yerel llama-server sağlayıcısı
  custom_anthropic.go — Özel (Anthropic uyumlu) sağlayıcı: *claudeProvider'ı saran ince sarmalayıcı, kullanıcının kendi Anthropic-şekilli proxy'si için (v4.4.0'da eklendi)
  opencode_zen.go / opencode_go.go — OpenCode Zen/Go sağlayıcıları, canlı model listeli gateway'ler
  kilo.go           — Kilo Code sağlayıcısı (app.kilo.ai), canlı model listeli gateway
  effort.go         — GeminiThinkingBudgetForLevel + EffortLevelsFor*: "effort level" (minimal/low/medium/high/max) etiketlerinin vendor-özel parametrelere çevrimi (Gemini thinkingBudget, Claude adaptive thinking)
```

### internal/replcli/ — terminal REPL (`memo` CLI)

```
internal/replcli/
  repl.go                  — ana interaktif terminal sohbet döngüsü (Run), backend REST API'sine bağlı
  repl_test.go             — Run()'ın senaryolu bir SSE test sunucusuna karşı uçtan uca testi
  client.go                — Memo REST API'sini saran minimal HTTP istemcisi
  client_test.go           — Client.Status ve diğer HTTP çağrılarının test sunucusuna karşı testleri
  models_client.go         — yerel model liste/durum endpoint'leri için REPL-tarafı istemci
  models_client_test.go    — ListLocalModels'ın mock sunucuya karşı testi
  download_client.go       — Hugging Face model arama/indirme endpoint'leri için REPL-tarafı istemci
  download_client_test.go  — SearchModels HTTP çağrısı ve cevap çözme testi
  sse.go                   — bir Server-Sent-Events satırını api.StreamChunk'a ayrıştırır
  sse_test.go              — ParseSSELine'ın içerik/bitiş/finish-reason durumları testi
  agent_event.go           — terminal REPL için agent_event SSE payload'larını çözen AgentEvent struct'ı
  agent_event_test.go      — AgentEvent JSON çözmenin tam payload testleri
  commands.go              — REPL slash komutlarını uygular (/help, /models, /model, /connect, /gui, /model-download …)
  commands_test.go         — slash komut handler'larının mock models/providers sunucusuna karşı testleri
  menu.go                  — ok tuşlarıyla gezilen interaktif terminal seçim menüsü
  color.go                 — bağımsız-kütüphanesiz ANSI renk/biçimlendirme yardımcıları, banner, progress bar
  color_test.go             — progressBar'ın çeşitli yüzdelerde render edilmesi testi
  spinner.go               — ilk cevap byte'ı beklenirken gösterilen animasyonlu "düşünüyor..." spinner'ı ve daktilo efekti
  spinner_test.go          — daktilo metin gösterimi ve spinner başlat/durdur testleri
```

### internal/sessions/

```
internal/sessions/
  sessions.go       — sohbet oturumlarını/mesajları diske kalıcı kılan Manager, aktif-sohbet değiştirme
  sessions_test.go  — oturum oluşturma, kalıcılık ve diskten yeniden yükleme testleri
```

### internal/skill/ — eklenti benzeri skill sistemi

```
internal/skill/
  types.go          — skill sistemi için SkillTool/SkillManifest/DangerLevel veri tipleri
  loader.go         — SKILL.md ön-madde (front-matter) bilgisini bir SkillDefinition'a ayrıştırır/yükler
  loader_test.go    — ParseSkill'in OpenCode formatlı SKILL.md içeriğine karşı testi
  manager.go        — diskte skill'leri keşfeder, aktive/deaktive eder, skill tool'larını kaydeder
  manager_test.go   — Manager'ın diskteki skill keşfi testi
```

### internal/truncate/

```
internal/truncate/
  tokens.go       — kaba token tahmini ve mesaj listesini bir token bütçesine sığdırma
  tokens_test.go  — EstimateTokens ve TruncateMessages uç durum testleri
```

### internal/tunnel/

```
internal/tunnel/
  tailscale.go       — paket dokümantasyonu + web sunucusunu uzaktan erişime açan gömülü tsnet Tailscale node'u
  tailscale_test.go  — NewTailscale varsayılanları (çalışmıyor, boş genel URL) testi
```

### internal/websearch/

```
internal/websearch/
  ddg.go       — web_search tool'u için DuckDuckGo HTML arama sonuçlarını kazır (scrape)
  ddg_test.go  — DDG HTML sonuç ayrıştırmasının kaydedilmiş örnek sayfaya karşı testi
```

### internal/webserver/ — REST API (Flutter'ın kullandığı ~45+ endpoint)

```
internal/webserver/
  server.go              — HTTP sunucu kurulumu, routing, self-signed TLS, CORS/rate-limit middleware, temel sohbet/oturum handler'ları
  bridge.go              — FullBridge arayüzü: Flutter REST API'sinin ihtiyaç duyduğu tüm App metodları
  handlers_flutter.go    — en büyük handler dosyası: sohbet streaming, hafıza, modeller, sağlayıcılar, orchestra, agent, WhatsApp, skill endpoint'leri
  handlers_calendar.go   — takvim olayları, ayarlar ve öğrenme-sistemi ayarları için REST handler'ları
  handlers_mood.go       — mood skoru ve mood/self-interest/sistem-yönetimi ayarları için REST handler'ları
  handlers_oauth.go      — OpenRouter OAuth bağlanma akışı ve model liste/anahtar doğrulama handler'ları
  handlers_proactive.go  — proaktif ayarlar, bekleyen öneriler ve desenler için REST handler'ları
  server_test.go         — temel webserver handler'larının bir mockBridge'e karşı testleri
```

### internal/whatsapp/

```
internal/whatsapp/
  client.go      — whatsmeow tabanlı WhatsApp istemcisi: bağlan, gönder/al, kişiler, yeniden bağlanma
  store.go       — WhatsApp mesajları ve sohbet metadata'sı için SQLite kalıcılığı
  store_test.go  — Store mesaj kalıcılığı ve getirme testleri
  util_test.go   — güvenli dosya sistemi/anahtar kullanımı için sanitizeJID yardımcısının testi
```

### internal/whisper/

```
internal/whisper/
  whisper.go            — yerel whisper.cpp sunucu süreç yaşam döngüsünü ve ses transkripsiyonunu yönetir
  process_unix.go       — whisper.cpp sunucusu için Unix süreç sinyalleme/canlılık/zorla öldürme yardımcıları
  process_windows.go    — whisper.cpp sunucusu için Windows süreç sinyalleme/canlılık yardımcıları
  sysproc_darwin.go     — whisper sunucusunu başlatmak için macOS SysProcAttr (Setpgid)
  sysproc_linux.go      — whisper sunucusunu başlatmak için Linux SysProcAttr (Setpgid)
  sysproc_other.go      — diğer Unix-benzerleri için yedek SysProcAttr
  whisper_test.go       — NewServer varsayılanları ve çalışma-durumu kontrolleri testi
```

---

## 2. Flutter Masaüstü Uygulaması (`frontend/`)

```
frontend/lib/
  main.dart — uygulama giriş noktası; SharedPreferences, ProviderScope, MaterialApp/tema/dil kurulumu

frontend/lib/core/
  api_client.dart — Go backend'in REST API'sini saran Dio tabanlı MemoApiClient
  l10n.dart        — Türkçe/İngilizce i18n anahtar-değer sözlüğü, dil değişikliği dinleyicileriyle
  theme.dart       — ThemeColors extension + MemoTheme: Pewter/Night/Glass Light renk temaları

frontend/lib/models/
  chat.dart             — ChatMessage, StreamChunk, ChatSession modelleri (Go sessions paketini yansıtır)
  agent.dart            — Agent tool-call event'leri ve izin istekleri için AgentEvent, AgentPermission modelleri
  activity_step.dart    — orchestra plan görevlerini ve agent tool çağrılarını tek bir zaman çizelgesinde birleştiren ActivityStep
  provider_config.dart  — harici LLM sağlayıcıları (OpenAI/Claude/Gemini/vb) için ProviderConfig modeli ve varsayılanlar
  local_model.dart      — diskteki ve HuggingFace'teki model verisi için LocalModel, HFModelResult, GGUFFile, DownloadProgress
  curated_models.dart   — Discover sekmesi için elle seçilmiş GGUF modelleri ve donanım-uygunluğu sezgiselleri
  gpu_info.dart         — donanım ve sunucu durumu için GPUInfo, ServerStatus, MemoryFileInfo/Stats/SearchResult
  orchestra_config.dart — çoklu-rol "orchestra" agent modu yapılandırması için OrchestraConfig/RoleConfig modelleri
  token_usage.dart      — üst barda gösterilen canlı tur-başı token sayımı için TokenUsage modeli
  whatsapp.dart         — WhatsApp entegrasyonu için WhatsAppMessage, WhatsAppChatSummary, WhatsAppStatus modelleri
  task_list.dart        — Self-Driving görev listesi modelleri: ChatTaskState (working/stalled eşikleri), görev/madde/plan durumları (v4.4.0)

frontend/lib/providers/ (Riverpod state)
  chat_provider.dart      — çekirdek sohbet durumu: API istemcisi, sohbet listesi, aktif sohbet, streaming içerik/durum
  agent_provider.dart     — agent modu anahtarı, izin listesi, agent event bus/stream durumu
  models_provider.dart    — yerel model listesi, model/embedding sunucu durumu, GPU bilgisi, indirme ilerlemesi, model arama
  provider_provider.dart  — harici sağlayıcı listesi, aktif sağlayıcı seçimi, sağlayıcı logo/ikon yardımcıları
  orchestra_provider.dart — orchestra modu yapılandırma durumunu yöneten OrchestraConfigNotifier
  settings_provider.dart  — SharedPreferences tabanlı ayarlar: hafıza/llama ayarları, kurulum/tur bayrakları, sistem promptu
  mood_provider.dart      — mood skorunu ve mood-takip bayrağını 10sn'de bir backend'den akıtır
  learning_provider.dart  — proaktif öğrenme ayarları ve öğrenilen desenler için LearnedPattern modeli + provider'lar
  whatsapp_provider.dart  — WhatsApp bağlantı durumu, sohbet listesi, mesajlar, arama, sohbet-modu anahtarı
  skill_provider.dart     — SkillDefinition modeli ve kurulu agent skill'lerini listeleyen SkillListNotifier
  version_provider.dart   — periyodik olarak yeni uygulama versiyonu olup olmadığını kontrol eden VersionCheckNotifier
  recording_provider.dart — sesli-metin girişi için mikrofon kayıt durumunu kontrol eden RecordingNotifier
  tasklist_provider.dart  — Self-Driving görev listelerinin durumu, SSE üzerinden canlı aktivite akışı (v4.4.0)

frontend/lib/screens/
  app_shell.dart        — NavRail sekme değiştirici (sohbet/agent/model/whatsapp/takvim) ve global kısayollarla ana kabuk
  chat_screen.dart       — standart sohbet ekranı: kenar çubuğu, mesaj listesi, giriş, token sayacı, üst bar
  agent_screen.dart      — agent modu ekranı: agent sohbetleri kenar çubuğu, tool aktivitesiyle sohbet içeriği, izin dialogları
  model_store_screen.dart — Model Mağazası/Keşfet ekranı: seçilmiş model gezinme, yerel modeller, HF arama, donanım rozetleri (en büyük dart dosyası)
  whatsapp_screen.dart   — WhatsApp ekranı: sohbet listesi, mesaj görünümü, profil fotoğraflı avatarlar, gönder/ara arayüzü
  calendar_screen.dart   — /api/calendar/events'ten olayları çeken ve gösteren takvim ekranı
  task_detail_screen.dart — Self-Driving görev detay ekranı: canlı faz/ilerleme, plan onay kartı (_PlanApprovalSection), escalation banner'ı (v4.4.0)

frontend/lib/widgets/
  settings_dialog.dart      — settings/tabs/* içeriğine dikey sekme navigasyonuyla giden ayarlar dialog kabuğu
  chat_input.dart           — kompozitör widget'ı: metin girişi, model değiştirici dialog, OpenRouter model seçici, kayıt butonu
  chat_message_list.dart    — kaydırılabilir markdown mesaj listesi, streaming/düşünme balonu render'ıyla
  chat_sidebar.dart         — yeni-sohbet butonu, gizli mod anahtarı ve durum çubuğuyla sohbet listesi kenar çubuğu
  prompt_templates.dart     — slash-komut (/) açılır penceresi: prompt şablonları, model değiştir, orchestra değiştir, skill seç
  model_config_dialog.dart  — yerel bir modeli yapılandırıp başlatma dialog'u (context boyutu, GPU katmanları, port)
  provider_config_dialog.dart — harici sağlayıcı ekleme/düzenleme dialog'u (API anahtarı, model tarayıcı)
  orchestra_config_dialog.dart — orchestra modu rollerini, sistem promptlarını, rol-başı model seçimini yapılandırma dialog'u
  skill_config_dialog.dart  — agent skill'lerini listeleme, aktif/deaktif etme, kurma, kaldırma dialog'u
  setup_wizard_view.dart    — donanım tanılaması ve ilk yapılandırmadan geçiren ilk-çalıştırma kurulum sihirbazı katmanı
  llama_installer_view.dart — llama.cpp binary kurulumu/indirme ilerlemesini yöneten katman ekranı
  launchpad_view.dart       — Memo'nun yeteneklerini tanıtan özellik kartlarıyla ilk-çalıştırma launchpad ekranı
  welcome_view.dart         — dokunulabilir sohbet başlatıcı önerileri gösteren boş-sohbet karşılama ekranı
  spotlight_tour.dart       — UI elemanlarını delik-açma efektiyle vurgulayan spotlight/onboarding turu widget'ı
  version_banner.dart       — yeni Memo versiyonu bildiren, kapatılabilir sağ-alt banner
  engine_strip.dart         — şu an çalışan sohbet/hafıza modelini durdur kontrolüyle gösteren alt durum şeridi
  gpu_badge.dart            — GPU tespit edildi mi yoksa sadece CPU mu olduğunu gösteren küçük rozet
  mood_gauge.dart           — özel painter'lı kompakt ve genişletilmiş mood göstergesi (nokta/emoji/bar)
  glass_surface.dart        — Glass Light teması için Apple-tarzı buzlu cam panelleri render eden GlassBlur/GlassSurface

frontend/lib/widgets/agent/
  agent_mode_toggle.dart    — agent modunu aç/kapa anahtarı
  agent_chat_card.dart      — bir agent sohbeti girdisini özetleyen kart (agent kenar çubuğu listesinde kullanılır)
  activity_panel.dart       — orchestra görevlerini ve agent tool adımlarını canlı gösteren sağ taraf aktivite zaman çizelgesi
  permission_dialog.dart    — istenen bir agent tool iznini onayla/reddet dialog'u
  permission_history.dart   — geçmiş agent izin kararlarının liste görünümü
  task_activity_block.dart  — sohbet içi canlı Self-Driving görev kartı: tool/alt-ajan aktivite satırları, working/stalled durumları, "model üretiyor" göstergesi (v4.4.0)

frontend/lib/widgets/settings/tabs/
  general_tab.dart           — genel tercihler + CLI yeniden yükleme/kaldırma yönetimi sekmesi
  memory_tab.dart            — RAG hafıza yapılandırması sekmesi: istatistikler, benzerlik/top-k ayarları, hafıza dosyaları
  providers_tab.dart         — yapılandırılmış harici LLM sağlayıcılarını kart olarak listeleyen sekme
  orchestra_tab.dart         — orchestra (çoklu-rol agent) modunu yapılandırma sekmesi giriş noktası
  agent_permissions_tab.dart — agent tool izin kurallarını inceleme/yönetme sekmesi
  skills_tab.dart            — kurulu agent skill'lerini listeleme/yönetme sekmesi
  gpu_config_tab.dart        — GPU/model başlatma parametreleri sekmesi (context boyutu, GPU katmanları, slider'lar)
  learning_tab.dart          — proaktif öğrenme profili sekmesi: ayarlar, model yönlendirme, öğrenilen desen kartları
  mood_tab.dart              — onay dialogları ve doğrulama adımlarıyla mood-takip özelliği sekmesi
  system_prompt_tab.dart     — varsayılan sohbet sistem promptunu düzenleme sekmesi
  incognito_prompt_tab.dart  — gizli sohbet modunda kullanılan sistem promptunu düzenleme sekmesi
  remote_access_tab.dart     — backend'e uzak/ağ erişimini yapılandırma sekmesi
  backup_restore_tab.dart    — uygulama verisini yedekleme/geri yükleme sekmesi (bulut metin alanı config dahil)
  about_tab.dart             — uygulama versiyonu, emek verenler ve hakkında bilgisi sekmesi

frontend/lib/utils/
  tool_names.dart — agent tool çağrı tipleri için yerelleştirilmiş görünen isim/açıklama/ikon/tehlike etiketleri
```

---

## 3. Flutter Mobil Uygulaması (`mobile/`)

Masaüstü backend'ine ağ/Tailscale/ngrok tüneli üzerinden bağlanan uzaktan-kumanda istemcisi.

```
mobile/lib/
  main.dart — uygulama giriş noktası; provider'ları kurar, event polling yapar, alt gezinme kabuğu (Sohbet/Takvim)

mobile/lib/core/
  api_client.dart          — masaüstü backend REST API'siyle konuşan HTTP/SSE istemcisi (dio)
  notification_service.dart — takvim hatırlatmaları için flutter_local_notifications sarmalayıcısı
  theme.dart               — masaüstü uygulamasıyla paylaşılan "Pewter Study" tasarım tokenları/renkler/tipografi

mobile/lib/models/
  calendar_event.dart — JSON (de)serileştirmeli CalendarEvent veri modeli

mobile/lib/providers/
  chat_provider.dart       — sohbet mesajları, streaming ve agent-enabled bayrağı için Riverpod durumu
  calendar_provider.dart   — takvim olayları/ayarları/yükleme durumu için Riverpod state notifier
  connection_provider.dart — backend bağlantısı (LAN/Tailscale/ngrok) ve uzak erişim için Riverpod durumu

mobile/lib/screens/
  connect_screen.dart  — ilk bağlanma/eşleştirme ekranı (LAN, Tailscale, uzak/ngrok modları)
  chat_screen.dart      — mesaj listesi, izin dialogları, oturum çekmecesiyle ana sohbet sekmesi
  calendar_screen.dart  — takvim sekmesi arayüzü (ay görünümü, olay listesi, olay ekle/düzenle)
  settings_screen.dart  — bağlantı/uygulama tercihleri için 4 sekmeli ayarlar ekranı

mobile/lib/widgets/
  branding.dart      — MemoLogo widget'ı — uygulama içinde çizilen bronz "M" simgesi
  chat_bubble.dart   — markdown render eden, panoya kopyalamalı sohbet mesajı balonu
  message_input.dart — gönder/streaming durdur kontrollü sohbet metin giriş çubuğu
  session_drawer.dart — sohbet oturumlarını ve bağlantı/sunucu bilgisini listeleyen yan çekmece
```

---

## 4. Paketleme / Build / Kurulum Script'leri (`scripts/`)

Root'un kalabalıklaşmaması için `.sh`/`.ps1`/`.bat` script'lerinin çoğu
`scripts/` altında toplandı (2026-08-09) — hiçbiri kendi dosya konumuna
göre `cd` yapmıyor, hepsi repo kökünden çalıştırılmayı varsayıyor. İstisna:
ana release betiği `build_releases.sh` repo **kökünde** (`./build_releases.sh`).
Ayrıntı ve kullanım örnekleri için `scripts/README.md`.

```
build_releases.sh    — (repo KÖKÜNDE) Linux release paketleme (.deb/.AppImage/.tar.gz) — Go backend + Flutter frontend
build_releases.bat   — build_releases.sh'in Windows karşılığı
build_releases_arm.sh — build_releases.sh'in arm64 (Raspberry Pi) karşılığı
package_linux.sh      — Go backend + Flutter frontend derler, bir Linux release paketi hazırlar
package_windows.sh    — Go backend'i Windows için çapraz derler, bir Windows release paketi hazırlar
macrelease.sh         — macOS'a özel paketleme (.app/.tar.gz/.dmg üretir, Xcode/cgo gerektirir)
download_binaries.sh  — bundle için llama.cpp llama-server binary'lerini indirir (Linux CPU/NVIDIA/AMD)
patch.sh              — birkaç App metodunun dönüş tipini interface{}'e çeviren tek seferlik sed patch'i
install.sh            — Linux kullanıcı-yerel yükleyici; paketi ~/.local/share'e kopyalar, masaüstü girdisi ekler
uninstall.sh          — Linux kullanıcı-yerel kurulumu kaldırır (uygulama dizini, masaüstü dosyası, ikon)
uninstall-arm.sh      — uninstall.sh'in arm64 karşılığı; hem native hem Docker kurulumlarını algılar
update.sh             — mevcut kurulumu veri/config'e dokunmadan günceller
run_memo.sh           — yerel geliştirme kısayolu (backend+frontend'i birlikte başlatır)
get-memo.sh           — Linux/macOS için tek satır curl yükleyicisi (stable); binary'leri + PATH wrapper'ını kurar
get-memo-beta.sh      — get-memo.sh'in beta kanalı (her main push'unda güncellenir)
get-memo.ps1          — Windows için tek satır yükleyici; Inno Setup yükleyicisini indirip çalıştırır
get-memo-beta.ps1     — get-memo.ps1'in beta kanalı
get_memo_arm.sh       — Linux arm64 (Raspberry Pi) için tam kurulum (CLI+backend+masaüstü)
get-memo-server.sh    — self-hosted, sadece sunucu kurulumu (masaüstü yok), stable kanal
get-memo-server-beta.sh — get-memo-server.sh'in beta kanalı
```

`installer.iss` (Windows Inno Setup script'i) bilinçli olarak kök dizinde
bırakıldı — `build-windows.yml` CI workflow'u onu sabit bir yoldan
referans alıyor, taşımak CI'a ek bir değişiklik gerektirirdi.

## 5. CI/CD (`.github/workflows/`)

```
ci.yml             — main'e her push/PR'da Go testlerini çalıştırır (SQLite bağımlılığıyla)
build-linux.yml     — push/PR/dispatch'te Linux x86_64 release'i derler (Go backend + Flutter)
build-macos.yml     — push/PR/dispatch'te macOS arm64+x86_64 release'i derler
build-windows.yml   — push/PR/dispatch'te Windows x86_64 release'i derler
upload-r2.yml       — Linux/macOS/Windows release'lerini derleyip Cloudflare R2'ye yükleyen elle-tetiklenen workflow
```

## 6. Yapılandırma, Yardımcı Script'ler, Skill'ler

```
config/
  config.yaml          — yerel (gitignore'lu) canlı config: API endpoint, kimlik/persona, hafıza, uzak erişim
  config.yaml.example  — config.yaml için tüm ayarlanabilir anahtarları belgeleyen şablon

scripts/
  stt_server.py   — hafif, bellek-içi Vosk konuşma-metin HTTP sunucusu (tr/en modelleri)
  transcribe.py   — faster-whisper ile bir ses dosyasını metne çevirir, stdout'a yazdırır

skills/
  memo/SKILL.md          — "memo-rag-memory" skill'i: RAG/semantik hafıza alt sistemini güvenle değiştirme iş akışı
  memo-project/SKILL.md  — "memo-project" skill'i: mimari/konvansiyonlar için hızlı üst-düzey harita
```

## 7. Üst Düzey Dokümantasyon

```
AGENTS.md                 — AI kodlama ajanları için Memo'nun teknoloji yığını/mimarisi özeti (ana referans doküman)
BLUEPRINT.md               — v4.0.0'ı hedefleyen spekülatif ekosistem/iş planı (memo-proxy API iş modeli) — henüz uygulanmadı
BUG_REPORT.md              — aktif olarak bakımı yapılan açık bug takipçisi (bu yazının yazıldığı tarihte 0 açık madde, her seviyede)
GEMINI.md                  — proje genel bakış dokümanı ("Memory Shell" konsepti)
README.md / READmeTR.md    — ana proje README'si (EN) ve Türkçe çevirisi
roadmap.md                 — docs/ROADMAP.md (EN) ve docs/tr/ROADMAP.md'ye yönlendiren stub
handoff.md                 — oturum devir teslim notları, ters-kronolojik (en yeni en üstte)
plan.md                    — özellik kaybetmeden ilk-çalıştırma onboarding/UX iyileştirme planı
PLAN_learning_calendar.md  — v3.2 intent-tabanlı öğrenme + takvim + tek-model-modu planı (tamamlandı)
yapılacaklar.md            — Türkçe TODO listesi: stable-release engelleri + RAG hafıza yol haritası

docs/                — superpowers/specs (tasarım dokümanları) + superpowers/plans (uygulama planları),
                        learning-system/, task/, plans/, tr/ (API_REFERENCE, BILINEN_SORUNLAR,
                        TROUBLESHOOTING vb. Türkçe teknik dokümantasyon seti)
obsidian-doc/, obsidian-doc-en/ — Obsidian vault olarak dışa aktarılmış TR/EN kullanıcı/geliştirici dokümantasyonu
versinNote/           — sürüm notları (V1.0.0 → v3.3.3 yayınlandı, v3.3.4 geliştirmede) ve tr/ altında Türkçe çevirileri
```

## 8. Çalışma Zamanı / Üretilen Dizinler (kaynak kodu değil, açıklanmadı)

```
data/          — çalışma zamanı verisi: hafıza DB, sohbet oturumları, modeller, takvim, mood, WhatsApp, rutinler, kullanım istatistikleri (usage.db), Piper sesleri (tts/voices/), config anahtarları
binaries/      — platforma özel llama.cpp/vec0/whisper binary'leri (bundling için)
build/, build_output/ — derleme çıktıları ve paketleme sahne alanları
frontend/build/, frontend/.dart_tool/, mobile/build/, mobile/.dart_tool/ — Flutter derleme önbellekleri
.git/, .github/, .claude/, .kimchi/, .mimocode/, .opencode/, .superpowers/ — VCS ve araç/ajan meta verisi
```

---

## İstatistikler

> **2026-09-01'de yeniden sayıldı** (`find`/`wc` ile) — v4.4.0 (Self-Driving
> branch) sonrası, PROJECT_MAP.md dokümantasyon denetimiyle birlikte. Bir
> önceki sayım 2026-08-05'ten kalmıştı (v3.3.3/v3.3.4 dönemi) — aşağıdaki
> sayılar da bir sonraki büyük değişiklikte yine bayatlayacak, elle güncel
> tutulur.

### Go (backend)

| Metrik | Değer (2026-08-05 → 2026-09-01) |
|---|---|
| Toplam `.go` dosyası (test hariç) | 220 → **327** |
| Test dosyası (`_test.go`) | 161 → **276** |
| `internal/` alt paket sayısı | 41 → **48** |
| Kaynak kod satırı (test hariç) | 53.292 → **79.180** |
| Test kodu satırı | 30.946 → **53.172** |
| **Toplam Go satırı** | 84.238 → **132.352** |
| En büyük dosya | `internal/webserver/handlers_flutter.go` (3.129 satır) |
| 2. en büyük | `internal/memory/store.go` (2.624 satır) |
| 3. en büyük | `internal/taskloop/engine.go` (1.626 satır, yeni — v4.4.0) |
| 4. en büyük | `internal/app/llm.go` (1.573 satır) |

### Flutter — Masaüstü (`frontend/`)

| Metrik | Değer (2026-08-05 → 2026-09-01) |
|---|---|
| Toplam `.dart` dosyası (lib/, test hariç) | 105 → **133** |
| Test dosyası | 17 → **45** |
| Kaynak kod satırı (lib/) | 40.341 → **53.316** |
| Test kodu satırı | 2.786 → **7.130** |
| En büyük dosya | `core/l10n.dart` (4.159 satır) |
| 2. en büyük | `core/api_client.dart` (2.778 satır) |
| 3. en büyük | `widgets/chat_input.dart` (2.223 satır) |

> `model_store_screen.dart` artık tek dev dosya değil — `settings/tabs/`'a paralel bir desenle bölündü (bkz. handoff.md, BUG-M1).

### Flutter — Mobil (`mobile/`)

| Metrik | Değer (2026-08-05 → 2026-09-01) |
|---|---|
| Toplam `.dart` dosyası (lib/, test hariç) | 21 → **22** |
| Test dosyası | 1 → **2** |
| Kaynak kod satırı (lib/) | 7.848 → **7.888** |
| En büyük dosya | `core/api_client.dart` (1.928 satır) |

> `mobile/` hâlâ ayrı, aktif geliştirilen bir proje — `frontend/`'e Android/iOS hedefi eklenip `mobile/`'ın kaldırılması yönündeki bir önceki plan (bkz. ROADMAP.md) henüz gerçekleşmedi.

### Genel Toplamlar

| Metrik | Değer |
|---|---|
| **Toplam kaynak dosyası** (Go + Dart, test dahil) | **~805** |
| **Toplam kaynak kodu satırı** (Go + Dart, test dahil) | **~200.686** |
| Shell/bat/ps1 script sayısı (kök dizin) | 1 (`build_releases.sh`) — geri kalan **20 script `scripts/`'e taşınmış** (3.633 satır) |
| Markdown doküman sayısı (repo genelinde) | 238 |
| `.github/workflows/` sayısı | 6 |
| `skills/` altındaki skill sayısı | 2 |
| Git commit sayısı | 1.156 |

---

*Son güncelleme: 2026-08-05 · v3.3.3 (yayınlandı) / v3.3.4 (geliştirmede)*