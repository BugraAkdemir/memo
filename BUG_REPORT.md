# Memo — Derin Kod-Kanıtlı Bug Denetimi (codebase-memory ile)

> **Denetim tarihi:** 2026-09-14
> **Yöntem:** Bu denetim, aynı gün yapılan önceki statik/doküman taramasından (aşağıda "önceki denetim" olarak anılıyor, o denetimin B1-B13 maddeleri bu raporda tek tek doğrulanıp/çürütülüyor) **farklı bir yöntemle** yapıldı: `codebase-memory-mcp` (proje `home-bugra-Documents-memo`, 17.481 node / 83.379 kenar, `main` branch, index `ready`) knowledge graph'ı üzerinden `search_graph`/`trace_path`/`get_code_snippet`/`query_graph` ile gerçek kod izlendi, her iddia `check_index_coverage` ile doğrulandı ve her bulgu gerçekten okunan dosya:satır referanslı kanıtla desteklendi (AGENTS.md'nin "no fabrication" kuralına göre — "suspected, unconfirmed" diye işaretlenmemiş her bulgu doğrudan okunan koda dayanıyor). 7 paralel, sınırlı-kapsamlı ajan (`internal/app`+`provider`+`orchestra`+`api`; `taskloop`+`agent` sandbox; `memory`+`database`+`observer`+`proactive`+`intent`+`truncate`; `livemode`+`whatsapp`+`telegram`+`whisper`; `webserver`+`cloudsync`+`tunnel`+`ngrok`+`config` güvenlik odaklı; Flutter `frontend/lib`; `replcli`+`skill`+`modelstore`+`calendar`+`identity`+`geminisub`+`stats`) kullanıldı.
> **Yine yapılmadı:** gerçek `go build`/`go test -race`/`flutter test` çalıştırılmadı, hiçbir bulgu canlı reprodüksiyonla doğrulanmadı — bu hâlâ statik bir kod incelemesi, ama artık dokümana değil gerçek kaynak koduna dayanıyor.
> **⚠️ Güvenlik notu:** Bu raporda gerçek, sömürülebilir auth-bypass ve komut-enjeksiyonu bulguları var (P0 bölümüne bakın). Bu dosya GitHub'a push edilmeden önce en azından P0 maddelerinin düzeltilmesi ya da bu detayların ayrı, düzeltilene kadar public olmayan bir yerde tutulması önerilir.

---

## 1. Kısa karar

Önceki denetimin "yeni feature değil, reliability pass" sonucu **doğrulanıyor ve güçleniyor** — ama gerçek kod okuması, önceki denetimin göremediği **5 adet P0 seviyesinde gerçek bug** ortaya çıkardı (1 tanesi tüm kullanıcıları etkileyen bir kimlik doğrulama bypass'ı, 1 tanesi tüm backend'i çökertebilen bir panic, 1 tanesi arka plan görevi çalışırken tüm agent izinlerini sessizce devre dışı bırakan bir sızıntı, 1 tanesi trivially atlatılabilen bir komut kara listesi, 1 tanesi süreç çakışmasında alt süreç sızıntısı).

---

## 2. 🔴 P0 — Kritik (güvenlik/çökme, öncelikli düzeltilmeli)

### P0-1 — Tailscale/Funnel uzak erişimi TÜM auth modlarını bypass ediyor
**Dosyalar:** `internal/app/remote_tailscale.go:64-89` (`SetTailscaleMode`), `internal/app/app.go:843-850` (`StartWebServerHTTP`), `internal/tunnel/tailscale.go:340-345`, `internal/webserver/server.go:1008-1013` (`remoteAuthOK`)

`remoteAuthOK`'nin ilk satırı: `if listenAddr != "0.0.0.0" { return true }` — yani sunucu `0.0.0.0`'a bağlı değilse auth kontrolü tamamen atlanıyor. `SetRemoteAccess`/`SetNgrokMode` sunucuyu etkinleştirirken doğru şekilde `0.0.0.0`'a yeniden bağlıyor, ama **`SetTailscaleMode` bunu yapmıyor** — sadece `ws.StartHTTPWithAddr(port, "127.0.0.1")` çağırıyor (zaten çalışıyorsa hiç çağırmıyor bile). tsnet ise tüm tailnet (ve `Funnel: true` ise **public internet**) trafiğini bu aynı `127.0.0.1` adresine proxy'liyor.

**Sonuç:** Kullanıcı Ayarlar'dan Tailscale'i (özellikle Funnel'ı) açtığında, `AuthMode = "password"` seçmiş olsa bile — **hiçbir kimlik bilgisi olmadan** her `/api/...` uç noktasına (mesaj gönder, tüm sohbet geçmişini oku/export et, hafıza ara/export et, agent tool çalıştır, WhatsApp/Telegram gönder, provider API key yönetimi, hesap/şifre yönetimi) erişilebiliyor. UI'nin kendi uyarısı (`GetRemoteAccessStatus`) sadece `AuthMode == "none"` iken tetikleniyor, yani şifre koymuş bir kullanıcı hiçbir uyarı görmeden tamamen açıkta kalıyor.

**Düzeltme:** `SetTailscaleMode` (ve `startupTailscale`), Tailscale/Funnel etkinken sunucuyu `SetRemoteAccess`'in yaptığı gibi `0.0.0.0`'a bağlamalı.

### P0-2 — Live Mode tool-call goroutine'i kapanmış kanala yazıp tüm backend'i çökertebilir
**Dosyalar:** `internal/livemode/google/client.go:396-508,543-562`, `internal/livemode/openai_realtime/client.go:326-391,399-425`

`readLoop` hata/kapanma anında `defer close(c.events)` çalıştırıyor. Aynı anda, modelin tetiklediği bir tool-call `go c.runToolCall(fc)` ile **`logx.GoRecover` olmadan** (kod tabanındaki her yerde kullanılan panic-recovery deseninin aksine) ayrı bir goroutine'de çalışıyor. `runToolCall`'un kendi hata yolu da aynı `select { case c.events <- ...: case <-c.ctx.Done(): }` desenini kullanıyor. Session kapanırken (`Close()` ya da bir okuma hatası) bu iki goroutine arasında `c.events` kapatıldıktan **sonra** `runToolCall`'un gönderim denemesi seçilirse — Go'nun "kapalı kanala gönderim" panic'i gerçek ve önlenmemiş, **recover yok, tüm Memo backend süreci çöküyor** (o oturumla sınırlı değil — aynı anda çalışan WhatsApp/Telegram/diğer tüm kullanıcılar dahil).

**Düzeltme:** `go c.runToolCall(...)` çağrısını `logx.GoRecover` ile sarmalayın; ayrıca `EchoSession`'ın zaten kullandığı `closed`-flag deseniyle post-close gönderimi tamamen engelleyin.

### P0-3 — ✅ Düzeltildi — Taskloop'un global "bypass permissions" bayrağı, ilgisiz tüm interaktif/WhatsApp/Telegram sohbetlerine sızıyor
**Dosyalar:** `internal/taskloop/engine.go:313-330,497-511`, `internal/app/app.go:704-705,726-730`, `internal/app/llm.go:301-310`, `internal/agent/executor.go:73-74,476-487`

**Düzeltildi:** `app.go:726-730`'daki `setBypass` callback'i artık no-op — `a.agentExecutor`'a hiç dokunmuyor. Gerekçe: her task-sahipli executor (`buildTaskRunConfig`, `tasklist_escalate.go`, `tasklist_stepexec.go`, `tasklist_planner.go`, `subagent_runner.go`) zaten kendi `SetBypassPermissions(true)`'ını çağırıyor; callback'in kapsadığını iddia ettiği tek kalan senaryo ("buildTaskRunConfig provider bulamazsa global executor'a düşme") aslında hiç tool-call'a ulaşmıyor — `callAgentStream`'in interaktif dalı da aynı `resolveAgentProvider()`'ı çağırıp aynı şekilde erken hata veriyor. Regresyon testi: `internal/e2e/tasklist_bypass_leak_e2e_test.go` (`TestTaskList_RunningDoesNotBypassPermissionsForUnrelatedInteractiveChat`) — eski koda karşı çalıştırıldığında `AGENT: [BYPASS] auto-approving "delete_file"` logunu üretip kırmızı yanıyor, doğrulandı. `go build/vet/test -race` (`internal/app`, `internal/taskloop`, `internal/agent`, `internal/agent/tools`, `internal/e2e`) tamamı yeşil.

İlk task listesi başladığında `Engine.Start` → `e.setBypass(true)` → `internal/app/app.go:730`: `func(v bool) { a.agentExecutor.SetBypassPermissions(v) }`. Ama `a.agentExecutor`, **normal interaktif sohbetin de** kullandığı tekil, paylaşılan executor. `callAgentStream` sadece task'ın **kendi** worker turu için ayrı bir executor'a geçiyor (`taskRunConfigFromCtx(ctx) != nil` olduğunda) — başka her çağrı (masaüstü sohbet, WhatsApp/Telegram'dan gelen agent-mode mesajı) hâlâ `a.agentExecutor`'ı kullanıyor, ve bu bayrak `bool` olarak tekil/kapsam-sız.

**Sonuç:** Kullanıcı 30-60 dakika süren bir Self-Driving görev başlattığında, o süre boyunca **ilgisiz** herhangi bir agent-mode mesajı (kendi yazdığı ya da WhatsApp'tan gelen) `run_command`/`delete_file`/`write_file` gibi normalde izin isteyen bir işlemi **hiç izin dialogu çıkmadan** çalıştırabiliyor.

**Düzeltme:** `setBypass` callback'inin `a.agentExecutor`'a etkisini tamamen kaldırın — her task worker'ının zaten kendi `NewTaskExecutor`'ı var, paylaşılan interaktif executor'a hiç dokunmamalı.

### P0-4 — ✅ Düzeltildi — `run_command` kara listesi shell quoting ile trivially atlatılabiliyor
**Dosya:** `internal/agent/tools/command.go:48-100,107-118,515-517`

`blacklistedPatterns` (`\brm\s+-rf\s+/…`, `\bsudo\b`, `\bmkfs\b` vb.) **ham, parse edilmemiş** komut string'ine regex ile bakıyor — bash'in tırnak birleştirmesini hesaba katmıyor. `rm -rf "/"` (tırnak `/`'den önceki boşluk-bitişikliğini bozuyor, pattern eşleşmiyor) veya `s''udo -k` (bash boş tırnakları birleştirip `sudo` yapıyor ama ham string'de bitişik `"sudo"` alt dizesi hiç yok) gibi basit hileler filtreyi tamamen atlatıp `bash -c` ile gerçek shell'e ulaşıyor.

**Düzeltildi:** `dequoteForBlacklist` eklendi — bash'in tırnak-kaldırma adımını (tırnak karakterlerini ve unquoted backslash-escape'leri kaldırıp bitişik parçaları birleştirme) yaklaşık olarak taklit ediyor; `isBlacklisted` artık hem ham komutu hem bu "dequoted" halini kontrol ediyor. `run_command`'ı hâlâ gerçek `bash -c` ile çalıştırıyoruz (bu, seçenek (b)'deki argv-tabanlı yeniden yazımdan çok daha küçük/az riskli bir değişiklik) — bu yüzden bu hâlâ tam bir shell-lexer değil, sadece "bilinçli olarak tırnaklarla bölünmüş kelime" sınıfını kapatan bir tespit katmanı; kendi payına düşen yeni bir false-positive riski yok (kelime zaten tırnaksız haliyle raw check'i her zaman geçiyordu). Regresyon testleri: `internal/agent/tools/command_test.go`'da `TestIsBlacklisted_DefeatsQuoteSplittingBypass` (eski koda karşı 7/8 örnekte kırmızı yandığı doğrulandı — `git stash` ile kanıtlandı) ve `TestIsBlacklisted_QuoteStrippingHasNoNewFalsePositive`. `go build/vet/test -race` (`internal/agent`, `internal/agent/tools`) tamamı yeşil.

**Sonuç:** Prompt injection'a maruz kalmış ya da halüsinasyon gören bir LLM worker turu, "temizlik yap" adı altında `rm -rf "/"` çalıştırabilir — kara liste bunu sessizce geçiriyor, komut Memo'yu çalıştıran kullanıcının tüm yetkileriyle icra ediliyor.

**Düzeltme:** Ham string regex yerine gerçek bir shell-lexer (örn. `mvdan.cc/sh/syntax`) ile normalize edip eşleştirin, ya da bu "sandbox'lı" tool'u `bash -c` yerine quoting'in imkansız olduğu kısıtlı bir argv listesiyle çalıştırın.

### P0-5 — Aynı anda başlayan iki `memo` süreci arasında bind/attach yarışı — kaybeden `os.Exit(1)` ile çöküyor, ngrok tüneli öksüz kalıyor
**Dosyalar:** `main.go:194-224,208-219`, `internal/webserver/server.go:472-475`, `internal/app/app.go:620-632,891-1020`

"Zaten çalışıyor mu" HTTP probe'u ile gerçek `net.Listen` iki ayrı, atomik olmayan adım. İki `memo --headless` süreci ~500ms içinde başlarsa ikisi de probe'da "çalışmıyor" görüp ikisi de bind etmeye çalışabiliyor; kaybeden, dokümante edilen "ikinci çağrı client olarak bağlanır" davranışı yerine `FATAL` basıp `os.Exit(1)` çağırıyor — bu da `defer a.Shutdown(ctx)`'i atlıyor. Ngrok modu açıksa, kaybeden süreç zaten gerçek bir ngrok alt süreci başlatmış olabiliyor (`Startup()` içinde, bind'dan önce) ve bu süreç **hiç temizlenmeden** öksüz kalıyor.

**Düzeltme:** `EADDRINUSE` hatasında probe'u tekrarlayıp client-attach yoluna düşün; `os.Exit(1)`'den önce açıkça `a.Shutdown(ctx)` çağırın.

---

## 3. 🟠 P1 — Yüksek öncelik

| # | Bulgu | Dosya |
|---|---|---|
| P1-1 | Admin, kendi şifresini mevcut-şifre doğrulaması **olmadan** değiştirebiliyor — `id == subject` (ID vs username) karşılaştırması hiçbir zaman doğru olmayan ölü kod, admin-kendi-şifresi durumu her iki dalı da atlıyor | `internal/app/remote_auth.go:745-780` |
| P1-2 | Live Mode'da (Google/OpenAI Realtime) hiç reconnect/backoff yok — tek websocket dial, kopunca oturum kalıcı olarak düşüyor, manuel yeniden başlatma gerekiyor | `internal/livemode/{google,openai_realtime}/client.go` |
| P1-3 | whisper.cpp alt süreci çökerse hiç yeniden başlatılmıyor — `monitor()` sadece logluyor, `a.whisperServer` `nil`'lenmiyor, her STT isteği sonsuza dek "connection refused" ile başarısız oluyor | `internal/whisper/whisper.go:251-288`, `internal/app/stt.go:291-313` |
| P1-4 | `database.DB.Write()` serileştirmesini atlayan 2 ayrı ham `sql.Open` bağlantısı — `ExportData` ve periyodik cloud sync, `memory.db`/`mood.db` üzerinde `wal_checkpoint` çalıştırırken canlı yazma döngüsüyle senkronize değil | `internal/app/backup.go:500-509`, `internal/cloudsync/sync_manager.go:440-450,513-525` |
| P1-5 | `internal/identity/identity.go`: `Update()` `UserName`/`AssistantName`/`Style`/`CustomRole`'u **kilit olmadan** yazıyor, `BuildSystemPrompt` aynı alanları **kilit olmadan** okuyor — aynı struct'ta `MinimalMode`/`LearnedStyleNotes` için zaten var olan kilit deseni buraya hiç genişletilmemiş, gerçek data race | `internal/identity/identity.go:387-398,175-321` |
| P1-6 | `a.webSearchExecutor` tekil, paylaşılan bir alan — iki farklı sohbet eşzamanlı web-search modunda çalışırsa `SyncRouter` yarışı, bir sohbetin turu başka bir sohbetin çözümlenmiş provider/model'iyle yürüyebilir | `internal/app/llm.go:562-611`, `internal/agent/executor.go:240-251` |
| P1-7 | `AgentAutoPermissionNotifier` dispose sonrası `state` yazıyor — kardeş sınıf `AgentEnabledNotifier`'ın zaten belgelediği ve düzelttiği aynı çökme (`mounted` kontrolü eksik) | `frontend/lib/providers/agent_provider.dart:156-198` |
| P1-8 | `LearningSettingsNotifier`'da hiç auth-gate guard'ı yok, ve Setup Wizard'ın kendisinden (Adım 4) izleniyor — gated backend'de kurulum sihirbazı kalıcı olarak bozuk görünebilir | `frontend/lib/providers/learning_provider.dart:71-95` |

---

## 4. 🟡 P2 — Orta öncelik

| # | Bulgu | Dosya |
|---|---|---|
| P2-1 | Orchestra gerçekten `provider.Router`'ı bypass ediyor (iddia doğru) ama **kendi** fallback/retry sistemi var (chief+specialist için); asıl eksik paylaşılan sağlık/hata-sayacı durumu — Router'ın devre dışı bıraktığı bir provider Orchestra'ya "sağlıklı" görünüyor ve tersi | `internal/orchestra/conductor.go` |
| P2-2 | `Priority` alanı **kullanılıyor** (önceki denetimin "kullanılmıyor" iddiası **çürütüldü**) ama `router.go` (azalan: yüksek sayı = önce denenir) ile `config.go GetEnabled()` (artan, ters yön, "router ile eşleşiyor" diye yanlış yorum) birbirine **zıt sıralıyor** — `resolveAgentProvider`'ın fallback-model seçimi yanlış provider'ı seçebiliyor | `internal/provider/router.go:63-66,221-224`, `config.go:162-178` |
| P2-3 | `a.activeProviderName` dolu ama `a.providerRouter` `nil` olabiliyor (örn. kısmi restore/manuel config düzenlemesi) — agent-mode kendini onarıyor (`resolveAgentProvider`), ama düz sohbet yolunda aynı onarım yok, kullanıcıya kafa karıştırıcı "yerel model yüklenmemiş" hatası gösteriyor | `internal/app/providers.go:156-211`, `llm.go:1009-1014` |
| P2-4 | Read-only sub-agent'ların `curl` allowlist'i `-o`/`--output`/redirection'ı kontrol etmiyor — "sadece coder yazar" tek-yazarlı tasarımını kırıp dosya yazabiliyor | `internal/agent/tools/command.go:561-616` |
| P2-5 | `MarkItemDone` Task.md'yi atomik olmayan, kilitlenmemiş oku-değiştir-yaz ile güncelliyor (store.go'nun kendi state'i için kullandığı `fileutil.AtomicWrite`'ın aksine) — crash-unsafe, aynı dosyayı paylaşan eşzamanlı listeler arasında kayıp güncelleme riski | `internal/taskloop/taskmd.go:133-153` |
| P2-6 | Escalation derinlik sınırı (`strings.Count(ID, ".") >= 2`) escalator LLM'i kendi (nokta içermeyen) step ID'lerini döndürürse atlanabiliyor — sınırsız yeniden-escalation riski | `internal/taskloop/engine.go:998-1001`, `plan.go:67-103` |
| P2-7 | `truncate.Text` **baş kısmı** koruyor ama Live Mode çağrı noktasının yorumu "en son turu koruyor" diyor — uzun sesli sohbetlerde 6000 rune sınırını aşınca model en eski bağlamla kalıyor, tam da ihtiyacı olan son konuşmayı kaybediyor | `internal/truncate/tokens.go:9-15`, `internal/app/livemode_session.go:645-647` |
| P2-8 | `Analyzer.Run` (saatlik çalışan pattern analizi) 3 ayrı kilit alıp atomik olmayan oku-sonra-yaz yapıyor — eşzamanlı bir "habit" deklarasyonu/suppress/confidence-ayarı sessizce ezilebiliyor | `internal/observer/analyzer.go:35-81`, `pattern.go:199-339` |
| P2-9 | OpenAI Realtime motoru hiç `EventInterrupted` yaymıyor — barge-in (sözü kesme) sadece Google motorunda çalışıyor | `internal/livemode/openai_realtime/client.go:144-150` |
| P2-10 | Eşzamanlı iki Live Mode oturumu aynı arka plan sohbetini paylaşıyor — transkriptler birbirine karışabiliyor | `internal/app/livemode_delegate.go:53-85` |
| P2-11 | Gömülü skill materyalizasyonu sadece "dosya var mı" kontrolü yapıyor — sürüm/hash kontrolü yok, atomik olmayan yazma; bir upgrade eski `SKILL.md`'yi hiç güncelleyemiyor, crash sonrası bozuk dosya "tamamlanmış" sayılıyor | `internal/skill/materialize.go:18-33,44` |
| P2-12 | İndirilen GGUF dosyalarının sadece byte-sayısı kontrol ediliyor, checksum yok — aynı boyutlu ama bozuk bir indirme sessizce "geçerli" model olarak yüklenip llama-server'ı çökertebilir | `internal/modelstore/modelstore.go:545-555` |
| P2-13 | Yarım kalan `.downloading` geçici dosyaları crash/restart sonrası hem UI'den hem `ListLocalModels`'dan gizli kalıyor, disk alanını sonsuza dek işgal ediyor | `internal/modelstore/modelstore.go:486-496,655-671` |
| P2-14 | `geminisub` token yenileme koordinasyonsuz — eşzamanlı iki istek expired token'a karşı ikisi de bağımsız refresh çağrısı yapıp token dosyasını yarış halinde üzerine yazabiliyor | `internal/geminisub/oauth.go:252-264` |
| P2-15 | `memo_session_permissions` prefs anahtarı `serverCoupledPrefsKeys`'te eksik — AGENTS.md'nin kendi uyardığı "eksik anahtar bu bug'ın dar bir versiyonunu yeniden açar" senaryosu | `frontend/lib/core/local_session_state.dart`, `permissions_provider.dart:12` |
| P2-16 | `runningTasksProvider`, `app_shell.dart`'ın auth-gate invalidation listesinde yok — gated pencerede `TaskDetailScreen` açılırsa kalıcı "task not found" | `frontend/lib/providers/tasklist_provider.dart:108-117` |
| P2-17 | `TaskListsNotifier`/`RunningTasksNotifier`'ın poll-refresh metodlarında generation-counter koruması yok — auth-gate geçişiyle çakışan bir poll, yeniden build edilmiş state'i eski veriyle ezebiliyor | `frontend/lib/providers/tasklist_provider.dart:39-70,119-142` |

---

## 5. 🟢 P3 — Düşük / bilgilendirici

- `GET /api/files/browse` tüm dosya sistemi genelinde isim (içerik değil) enumerasyonuna izin veriyor — **kasıtlı tasarım** (agent tool'larının zaten sahip olduğu blast radius ile eşdeğer sayılıyor), ama bu denklik varsayımı her hesap/rol konfigürasyonu için geçerli olmayabilir. `internal/app/server_browse.go:53-102`
- Backend restart, bir etkinliğin **tüm** hatırlatma penceresi boyunca kapalı kalırsa o hatırlatma kalıcı olarak atlanıyor (kasıtlı tasarım tercihi, ama gerçek kullanıcı etkisi var). `internal/calendar/store.go:142-150`
- `usage_events` tablosunda hiç retention/pruning yok, `usage.db` süresiz büyüyor. `internal/stats/store.go`
- Pinned facts: DB sorgusu sadece en son 75 kaydı aday olarak alıyor (recency, relevance değil) — 75'ten fazla gerçekten ayrı önemli fact'i olan kullanıcı en eskilerinin sessizce görünmez olduğunu fark edebilir (kısmen consolidation ile hafifletilmiş, dokümante edilmiş bilinen sınır). `internal/memory/store.go:1355-1402`
- `incognitoPromptProvider` / `pendingProactiveSuggestionProvider` / `model_detail_panel._loadFiles` auth-gate guard'ı eksik ama düşük risk (manuel retry var ya da kendiliğinden iyileşiyor).
- `telegram.Client.Start()`'ta check-then-act yarışı var (whatsapp'ın tam kilitleme deseninin aksine) — ama bilinen hiçbir çağrı noktası aynı `*Client`'ı iki kez `Start()` etmiyor, gizli/tetiklenmemiş risk.
- `internal/app/app.go:704`: `a.agentExecutor` oluşturulurken `a.providerRouter`'a `providerMu` almadan erişiliyor — tek-thread'li `Startup()` sırasında olduğu için pratikte riskli değil, ama dosyanın geri kalanındaki "her zaman kilitle" kuralıyla tutarsız.

---

## 6. Önceki denetimin (aynı gün, statik) iddialarının doğrulanması

| Önceki madde | Doğrulama sonucu |
|---|---|
| B1 — client/providerRouter stale-reference race | **İndirilmiş, tasarım gereği kabul edilebilir.** `clientMu`/`providerMu` kilit disiplini `internal/app`'in 24 dosyasında tek tek kontrol edildi — tüm production kod doğru kilitliyor. Bir stream başladığı client/router referansını kasıtlı olarak sonuna kadar kullanıyor (yarış değil, tasarım); swap olursa net bir "model/provider mid-stream değişti" hatası veriyor. |
| B2 — Orchestra Router fallback'ini bypass ediyor | **Kısmen doğrulandı, "görev gereksiz başarısız olur" sonucu çürütüldü.** Orchestra gerçekten Router'ı çağırmıyor ama kendi chief+specialist fallback/retry sistemi var. Gerçek eksik: paylaşılan sağlık-takibi yok (bkz. P2-1). |
| B3 — `Priority` alanı tanımlı ama kullanılmıyor | **Çürütüldü.** Alan hem `UpdateConfigs` hem `getActiveEntries`'de aktif olarak sıralamada kullanılıyor. Asıl gerçek bug: iki farklı kod yolunun ters yönde sıraladığı (bkz. P2-2). |
| B10 — arka plan fact-extraction ile ön plan chat aynı local-inference slotunu paylaşıyor | **Gerçek mimari risk doğrulandı, ama zaten `preemptBackgroundLLM` ile önlenmiş.** Her ön plan turu başlamadan hemen önce arka plan çağrısını iptal ediyor; test de var. Kalan risk (cancel-vs-server-abort arası küçük pencere) doğrulanamadı, teorik. |
| Telegram tek başarısız poll'da kalıcı olarak duruyor mu? | **Çürütüldü.** `pollLoop` 1s→30s exponential backoff ile sınırsız retry yapıyor, WhatsApp'ın `autoReconnect`'iyle aynı kalitede. |
| WhatsApp reconnect resilience | **Temiz, iyi tasarlanmış** — generation-guard'lı, backoff'lu, TOCTOU-siz `Start()`. |
| Cloud sync şifreleme (nonce reuse, plaintext sızıntısı) | **Temiz.** Her şifrelemede taze rastgele salt+nonce, AES-256-GCM (AEAD), upload öncesi her yol şifrelemeden geçiyor, secret dosyalar `0600`. |
| Brute-force lockout gerçekten kilitliyor mu (sayaç her denemede resetleniyor mu)? | **Doğru çalışıyor** — sayaç sadece başarılı girişte resetleniyor. |
| TLS durumu | **Doğrulandı, "hiç yok" doğru** — `generateSelfSignedCert` tam yazılmış ama hiçbir yerden çağrılmıyor (ölü kod), yanıltıcı bir "kısmi" güvenlik hissi yaratmıyor. |
| Backend Türkçe/İngilizce L10n seam (`Identity.UILanguage`) | **Çürütüldü — artık tamamen bağlı.** `App.GetUILanguage`/`SetUILanguage` başlangıçta ve her ayar değişikliğinde `tools`/`llama` paketlerine yayılıyor. |
| Frontend generation-counter deseni regresyonu var mı? | **Yok** — `frontend/lib/providers/`'daki 54 Notifier sınıfının tamamı tek tek kontrol edildi, güvensiz `bool`-disposed deseninin hiçbir regresyonu bulunamadı. |
| Auth-gate startup race (4 şekil) tamamen kapatıldı mı? | **Neredeyse — 2 yeni/eksik nokta bulundu** (P1-8, P2-16), geri kalan ~35 provider/screen doğru korunuyor. |
| Calendar reminder double-fire / startup-gap | **Temiz** — atomik `RETURNING` claim'i TOCTOU'yu kapatıyor, startup-gap zaten `BUG-M7` ile düzeltilmiş. |

---

## 7. Temiz bulunan alanlar (tek tek kontrol edildi, bug bulunamadı)

`internal/api` streaming client · `internal/config` (secrets `0600`, atomic write) · `internal/ngrok` (checksum eksikliği zaten dokümante edilmiş kabul edilmiş risk) · `internal/remoteauth` brute-force · `internal/cloudsync` şifreleme · `internal/skill/external.go` (kullanıcı-import edilen skill senkronizasyonu, signature-tracked) · `internal/geminisub` token depolama (AES-GCM, atomic write) · `internal/calendar` (TZ-safe epoch aritmetiği) · `internal/identity` UILanguage wiring · WhatsApp reconnect · Telegram poll-loop backoff · Frontend generation-counter deseni (54 sınıf) · `internal/taskloop/retry.go` (nil-safe, `stopped` flag race'i kendi yorumunda zaten çözülmüş) · `internal/agent/tools/file.go` `validatePath` (symlink/`~` sandbox kaçışlarına karşı sertleştirilmiş).

---

## 8. Önerilen düzeltme sırası

1. **P0-1..P0-5'i hemen düzeltin** — özellikle P0-1 (Tailscale auth bypass) ve P0-3 (taskloop izin sızıntısı) self-hosted/uzak erişim kullanan herkesi gerçek riske atıyor; P0-2 (panic) ve P0-5 (süreç çakışması) kullanılabilirlik/kararlılık sorunu. P0-4 (command blacklist) agent-mode + prompt injection kombinasyonunda gerçek bir tehdit.
2. **P1 listesini bir sonraki oturumda toplu ele alın** — çoğu küçük, izole düzeltmeler (kilit ekleme, `mounted` kontrolü, executor'ı per-call yapma).
3. **P2 listesi** normal teknik borç temizliği hızında ilerlesin — hiçbiri acil değil ama hepsi gerçek, kanıtlanmış.
4. Bu rapor commit edilmeden/push edilmeden önce en azından P0-1 ve P0-4'ün düzeltilmiş olması, ya da bu dosyanın geçici olarak `.gitignore`'a alınıp yerel tutulması önerilir (repo public ise).

---

## 9. Kapsam ve limitasyonlar

- Hiçbir bulgu `go test -race`/canlı reprodüksiyon ile doğrulanmadı — hepsi kaynak kodun doğrudan okunmasına dayanıyor (graph çıkarımı değil).
- `internal/livemode.Close` (285) ve `internal/whisper.NewServer` (215) gibi yüksek fan-in sayıları **grafiğin isim-çözümleme artefaktı** olduğu doğrulandı (Go+Dart karışık kod tabanında aynı kısa isimli fonksiyonlar birbirine bağlanıyor) — gerçek çalışma zamanı fan-in'i değil; `RunningTasksNotifier.cancel`'ın "227 çağıran" hotspot kaydı için de aynı doğrulama yapıldı (gerçekte tek bir gerçek Dart çağıran var).
- Bazı dosyalar (`internal/app/activity.go`, `frontend/lib/widgets/mascot_window.dart`) `check_index_coverage`'da `metadata_changed` (disk, graph'tan daha yeni) döndürdü — bu dosyalardaki bulgular doğrudan `Read` ile teyit edildi, graph metnine güvenilmedi.
- Tam satır-satır okunmayan alanlar (düşük öncelikli/zaman kısıtlı): `internal/agent/tools/{edit,websearch,fetchpage,whatsapp,calendar,routine,selfclone,sendfile}.go`, `SubAgentRunner`'ın somut implementasyonu, `internal/orchestra/{roles,types}.go`, `internal/intent/*`, `internal/proactive/{decision,matcher,feedback}.go`, her provider dosyasının (`grok.go`/`groq.go`/vb.) satır satır tekrarı, `mobile/` (kasıtlı olarak yüzeysel — emekliye ayrılıyor).
