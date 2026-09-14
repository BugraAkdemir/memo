# PLAN — Code Mode plan / auto / build alt-modları

> **Kaynak:** Kullanıcı isteği (2026-09-14/15 oturumu). Claude Code CLI'nin
> kendi plan/auto/build döngüsüne benzer şekilde, Code Mode açıkken Tab
> tuşuyla sohbet üç alt-mod arasında geçebilsin: **plan** (sadece planlar,
> düzenlemez), **auto** (bugünkü Code Mode davranışı, değişmeden), **build**
> (hızlı/izin isteme eşiği düşük). Plan bitince asistan düz metinle "build'e
> geçelim mi?" diye sorar; global "auto-perm" (Shift+Tab) açıksa sormadan
> hemen build'e geçip uygulamaya devam eder.
>
> **Bu plan üç paralel Explore ajanı + bir Plan ajanının derin kod taraması
> sonucu yazıldı (2026-09-14/15), sonra kritik varsayımlar elle
> doğrulandı** (`internal/webserver/handlers_flutter.go`'daki tek-`Done`
> SSE kısıtı, `internal/app/helpers.go`'daki system-prompt mesaj sırası).
> Satır numaraları o tarihe ait — uygulamadan önce grep'le doğrula.
>
> **Boyut: BÜYÜK.** 6 bağımsız birime bölündü (AGENTS.md kural #5: oturum
> başına 1-2 madde, her biri test + yeşil doğrulama ile). Aşağıdaki sırayla
> ilerle — her birim kendi başına commit'lenebilir ve öncekini bozmadan
> sonrakine geçilebilir bir ara durumda bırakır.

## Kullanıcının onayladığı 4 tasarım kararı

1. **Plan-hazır onayı:** düz metin — asistan kendi cevabında sorar, kullanıcı
   serbest metinle cevap verir ("evet"/"build"/"auto"/vs.), backend tek
   seferlik bir niyet kontrolü yapar.
2. **Build modu izin kapsamı:** bugünkü 6 araçlık `auto` seti
   (`write_file`/`edit_file`/`insert_line`/`delete_lines`/`create_task_md`/
   `edit_task_md`) + `run_command` (Dangerous ama `isBlacklisted`/
   `commandTargetsProtectedPath` engeli hâlâ devrede). `delete_file`,
   `change_directory`, `self_clone`, `configure_provider` HER modda sormaya
   devam eder.
3. **Plan modu kısıtı:** sadece prompt talimatı — araçlar teknik olarak
   erişilebilir kalır, model "düzenleme yapma" talimatını alır; `auto`/
   `build`'in aksine plan modunun otomatik-onay seti boş olduğu için bir
   düzenleme denemesi normal izin promptuna düşer (yumuşak caydırma).
4. **Global auto-perm açıkken plan bitince:** kullanıcıdan girdi beklemeden
   hemen build moduna geçip planı uygulamaya devam eder — **tek istisna:**
   aşağıdaki "Bilinen sınır" bölümüne bak (yerel llama.cpp modeli
   çalışırken bu zincirleme atlanır, düz-metin-sorma akışına düşer).

## Bilinen sınır — yerel model + zincirleme

`internal/app/helpers.go:369-394` doğrulandı: **yerel llama.cpp modeli
çalışırken system-role mesajı hiç yok** — system prompt ya tek bir
`user`-role mesaja katlanıyor (`combined = systemPrompt + "\n\n" + ...`,
geçmiş boşsa) ya da geçmişteki ilk `user`-role mesaja gömülüyor (`for i, h
:= range history { if h.Role == "user" {...} }`). Birim 4'ün zincirleme
tasarımı (`appendBuildContinuation`, aşağıda) `pMsgs[0]`'ın her zaman
`system`-role olduğunu varsayıyor — bu sadece harici sağlayıcı (API)
turlarında doğru, yerel model turlarında değil.

**Karar (bu plana göre uygulanacak):** Birim 4'te, zincirleme sadece
`a.llamaServer == nil || !a.llamaServer.IsRunning()` iken denenir. Yerel
model + plan modu + auto-perm açık kombinasyonunda sistem, karar 4'ün
"hemen uygula" davranışı yerine karar 1'in "düz metin sor, sıradaki mesajı
bekle" akışına düşer — sessizce yanlış mesaja yazma riskini almaktansa,
dar bir kenar durumda biraz daha az agresif davranmak tercih edildi. Bu
İSTENMEYEN bir davranışsa (yerel modelle de anında zincirleme isteniyorsa)
ayrı bir iş olarak `appendBuildContinuation`'ı hem `system` hem katlanmış
`user` mesajını bulup değiştirebilecek şekilde genişletmek gerekir.

---

## Birim 1 — Veri modeli + config şeması + backend prompt-swap (davranış değişmez)

**Dosyalar:** `internal/sessions/sessions.go`, `internal/config/config.go`,
`internal/app/agent_chat_context.go`, `internal/app/helpers.go`.

- `Session`'a iki alan: `CodeSubMode string` (""/"plan"/"auto"/"build",
  `omitempty`) ve `AwaitingPlanDecision bool` (`omitempty`) —
  `CodeMode *bool`'un hemen altına, aynı JSON-persist şekliyle.
- `GetCodeSubMode`/`SetCodeSubMode` (string doğrulamalı: sadece boş veya üç
  değerden biri) ve `GetAwaitingPlanDecision`/`SetAwaitingPlanDecision` —
  `GetCodeMode`/`SetCodeMode` (satır ~193-216) ile birebir aynı
  RLock/Lock+persist şekli.
- `AgentModeConfig`'e üç alan: `CodePlanPrompt`, `CodeAutoPrompt`,
  `CodeBuildPrompt string` (yaml+json tag'li) — boş string = yerleşik
  varsayılanı kullan (yeni migration/default satırı gerekmez, Go zero-value
  zaten "").
- `agent_chat_context.go`: `codingDirective` (mevcut metin, "auto" alt-modu)
  yanına `codePlanDirective` ve `codeBuildDirective` const'ları,
  `codeSubModeDirective(cfg config.AgentModeConfig, subMode string) string`
  (config override → yoksa built-in const), `withCodeSubMode`/
  `codeSubModeFromCtx` (ctx-key, `withCodeMode` ile aynı desen),
  `resolveCodeSubMode(chatID) string` (`resolveCodeMode` ile birebir aynı
  şekil, pin yoksa `"auto"` döner).
- `helpers.go`'daki `switch { case code: systemPrompt = codingDirective` →
  `systemPrompt = codeSubModeDirective(am, codeSubModeFromCtx(ctx))`.

**Davranış garantisi:** Var olan hiçbir sohbette `CodeSubMode` set
edilmediği için `resolveCodeSubMode` hep `"auto"` döner ve
`codeSubModeDirective(cfg, "auto")` — `CodeAutoPrompt` henüz boş olduğu
için — bugünkü `codingDirective` metnini birebir aynı döndürür. Sıfır
davranış değişikliği.

**Test:** `sessions_test.go`'a yeni accessor round-trip testleri,
`agent_chat_context_test.go`'a `codeSubModeDirective`'in üç mod + config
override dalı için tablo testi.

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test -tags sqlite_fts5 -race ./...`
yeşil. Flutter tarafı dokunulmadı, `flutter analyze`/`test` gerekmiyor.

- [x] Yapıldı (commit'lendi 2026-09-15)

---

## Birim 2 — İzin sisteminde build/`run_command` genellemesi

**Dosyalar:** `internal/agent/pipeline.go`, `internal/agent/turn_overrides.go`,
`internal/agent/executor.go`.

- `pipeline.go`'daki sabit `codeModeAutoApproveTools` map'i **değişmeden**
  kalır (bu artık "auto" alt-modunun seti). Yanına `codeModeBuildAutoApproveTools`
  (aynı 6 araç + `run_command`) ve `codeModeToolAutoApproveSet(subMode string)
  map[string]bool` (plan/"" → nil, auto → 6'lı set, build → 7'li set).
- `Pipeline.autoApproveMedium bool` alanı → `Pipeline.codeSubMode string`.
  Satır ~378-383'teki kapı: `DangerLevel == Medium` kontrolü **kaldırılır**
  (artık sadece `codeModeToolAutoApproveSet(p.codeSubMode)[toolName]`
  kontrolü — `run_command`'ın Dangerous olması artık engel değil, çünkü
  hangi araçların hangi modda otomatik onaylanacağını zaten set'in kendisi
  belirliyor, seviye değil).
- `TurnOverrides.AutoApproveMedium bool` → `TurnOverrides.CodeSubMode string`.
  `executor.go:374`: `pipeline.autoApproveMedium = o.AutoApproveMedium` →
  `pipeline.codeSubMode = o.CodeSubMode`.
- **Bu birimde `llm.go`'nun `TurnOverrides{...}` inşası HENÜZ değişmiyor** —
  `codeModeActive(ctx)` doğruyken hep `CodeSubMode: "auto"` sabit geçilir
  (henüz `codeSubModeFromCtx` okunmuyor). Yani mekanizma hazır ve test
  edilebilir ama uçtan uca davranış hâlâ değişmez.
- `turn_overrides_test.go`, `pipeline_test.go`'daki
  `TestRunStream_AutoApproveMedium*` testleri yeni alan adına güncellenir;
  iki yeni test eklenir: plan modunda `write_file`'ın hâlâ prompt istediği,
  build modunda `run_command`'ın otomatik onaylandığı.

**Doğrulama:** Go build/vet/test **-race** (bu birim tam olarak `-race`'in
bulmaya çalıştığı tool-call döngüsüne dokunuyor).

- [x] Yapıldı (commit'lendi 2026-09-15)

---

## Birim 3 — `save_code_plan` aracı + `data/plans/<proje>/plan.md` yazımı

**Dosyalar (yeni):** `internal/agent/tools/plan.go`, `internal/app/plan_tool.go`.
**Dosyalar (değişen):** `internal/agent/tools.go` (kayıt), `internal/app/app.go` (wiring).

- **Neden özel bir araç gerekiyor (doğrulandı):** `internal/agent/tools/file.go`'daki
  `validatePath` (~376-466), her path tabanlı aracı (`write_file` dahil)
  sohbetin `ProjectPath`'i dışına çıkan her yolu reddediyor. `data/plans/`
  Memo'nun kendi veri dizini, proje dizini değil — yani model `write_file`
  ile oraya asla yazamaz. Ayrı, sandbox'ı bypass eden bir araç şart.
- `save_code_plan(content string)` — `DangerLevel: Safe`, her zaman global
  olarak kayıtlı (Registry'de per-turn araç filtreleme mekanizması yok,
  doğrulandı — dört ayrı registry var ama hepsi call-path bazlı, sub-mode
  bazlı değil; yeni bir registry icat etmek karar 3'ün "sadece prompt
  kısıtı" seçimine göre gereksiz karmaşıklık). Plan dışı modlarda da
  teknik olarak çağrılabilir ama zararsız — sadece plan modunun promptu
  bahsediyor.
- `tools.PlanSaver` arayüzü (`TaskMdEditor` ile aynı "App sonradan wire
  eder" deseni) → `internal/app/plan_tool.go`'daki `App.saveCodePlan`:
  `currentChatIDFromContext(ctx)` → `sm.GetProjectPath(chatID)` →
  `projectSlug(projectPath, chatID)` (proje dizininin `filepath.Base`'i,
  `[^a-z0-9-]+` temizlenmiş; boşsa/proje yoksa chat ID'ye düşer) →
  `config.DataPath("plans", slug, "plan.md")` → `os.MkdirAll` +
  `fileutil.AtomicWrite`.

**Test:** `plan_test.go` (sahte `PlanSaver` ile arg-parse/hata yolları),
`plan_tool_test.go` (`projectSlug` tablo testi: normal isim, boşluk/unicode
içeren isim, boş `projectPath` → chatID'ye düşme, path-traversal
karakterlerinin temizlenmesi), bir `internal/e2e` senaryosu (`FakeProvider`
`save_code_plan` çağırır, gerçek `plan.md`'nin temp `MEMO_DATA_DIR` altında
oluştuğu doğrulanır).

**Doğrulama:** Go build/vet/test/race yeşil.

- [x] Yapıldı (commit'lendi 2026-09-15)

---

## Birim 4 — Düz-metin onay + auto-perm zincirleme (EN RİSKLİ BİRİM)

**Dosyalar:** `internal/app/chat.go`, `internal/app/llm.go` (yeni
`internal/app/code_submode.go`), `internal/agent/turn_overrides.go`
kullanımı (llm.go'da artık gerçek `codeSubModeFromCtx` okunur).

### 4a. Doğrulanmış kritik gerçek: SSE tek `Done:true`

`internal/webserver/handlers_flutter.go`'daki `writeSSEChunk` `chunk.Done`
döner, `streamSSE` bunu görünce **hemen return** eder (satır ~87-122,
elle doğrulandı). Yani "planı bitir, sonra ikinci bir turu ayrı bir
`SendMessageStreamTo` çağrısıyla başlat" YAKLAŞIMI ÇALIŞMAZ — ilk turun
`Done:true`'su anında HTTP response'u kapatır, ikinci turun içeriği hiçbir
zaman istemciye ulaşmaz.

**Doğru çözüm:** `callAgentStream`'in goroutine'i (llm.go, `lockChatStream`
hiç çağırmıyor — kilit `sendMessageStreamInnerTo`'da, bir üst seviyede
tutuluyor) kendi içinde tek bir `Done:true` gönderene kadar döngüye
sokulur. Kilit yeniden alınmadığı için reentrancy/deadlock riski yok.

### 4b. Yapılacaklar

- `callAgentStream`'in tek-pas gövdesi `runOneAgentPass(...)` adlı bir
  yardımcıya çıkarılır — **`Done:true` göndermeden**, `finishReason` ve
  "bu normal-stop mu" bilgisini döner. `drainAgentStream`'in dört dalından
  (iptal/hata/normal-stop/boş-yanıt) sadece normal-stop zincirlenebilir;
  diğer üçü her zaman olduğu gibi hemen `Done:true` gönderip döner
  (en riskli yolun güvenli tarafta kalması için).
- `onEvent` closure'ına tek satır: `save_code_plan` `EventToolResult`'ı
  görülünce bir `atomic.Bool` set edilir (aynı goroutine, `Done`'dan önce
  — happens-before garantisi var, `agentEventLog` deki mevcut desenle
  aynı mantık).
- `callAgentStream`'in gövdesi bir `for` döngüsüne alınır: normal-stop +
  `planSaved` + `subMode=="plan"` ise → (yerel model kontrolü, "Bilinen
  sınır" bölümüne bak) → auto-perm KAPALIYSA `AwaitingPlanDecision=true`
  set edip normal `Done:true` gönderir (asistanın kendi cevabı zaten
  soruyu sormuş olur, plan promptu sayesinde); auto-perm AÇIKSA
  `Done:true` GÖNDERMEDEN `CodeSubMode="build"` set eder, sistem promptunu
  + `TurnOverrides`'ı build'e çevirir, senkron bir "planı şimdi uygula"
  iç mesajı ekler (sohbet geçmişine YAZILMAZ, sadece bu tek
  `RunStreamWithRouter` çağrısına verilir), `code_submode_changed` durum
  chunk'ı gönderir, döngü `continue` ile build pasını aynı `outCh`'ye
  çalıştırır.
- `internal/app/code_submode.go` (yeni, küçük): `classifyPlanDecisionReply(msg
  string) (subMode string, matched bool)` — LLM çağrısı YOK, anahtar
  kelime tabanlı (kelime sınırı kontrollü): "build"/"auto" adı geçiyorsa o
  moda, "evet/yes/uygula/devam/tamam/olur/ok" gibi düz onaylardan biri
  varsa `"auto"`'ya, aksi halde değişiklik yok.
- `chat.go`'daki `sendMessageStreamCore`, `withCodeMode` bloğunun hemen
  altına: `AwaitingPlanDecision` set'liyse yeni mesajı
  `classifyPlanDecisionReply`'den geçirip eşleşirse `SetCodeSubMode`,
  eşleşse de eşleşmese de `SetAwaitingPlanDecision(false)` (tek seferlik).

**Test:** `classifyPlanDecisionReply` tablo testi (tüm onay kelimeleri +
"build"/"auto" + olumsuzlar + boş metin); `sendMessageStreamCore`'un karar
kancası için bir `internal/app` testi; **yük taşıyan asıl regresyon
testi** bir `internal/e2e` senaryosu — gerçek HTTP istemcisiyle: (a) plan
modunda `save_code_plan` çağrılır, tek `Done:true` geldiği ve
`AwaitingPlanDecision` set olduğu doğrulanır; (b) `SetAutoPermission(true)`
ile aynı senaryo, tek SSE cevabında hem `code_submode_changed` chunk'ı hem
ikinci pasın içeriği hem de sonda tek `Done:true` olduğu, ve per-chat
kilidin iki kez alınmadığı (timeout/deadlock olmadığı) doğrulanır.

**Doğrulama:** Go build/vet/test **-race zorunlu** (goroutine/kanal ağırlıklı
değişiklik). Flutter'a henüz dokunulmuyor — `code_submode_changed`
chunk'ının mevcut Flutter SSE tüketicisini bozmadığı (bilinmeyen
`finishReason`'ı sessizce yok saydığı) ayrıca kontrol edilmeli.

- [x] Yapıldı (commit'lendi 2026-09-15) — plan doc'unda tarif edilenden iki
  küçük sapma: (1) `data/plans/...` yazan `save_code_plan` aracının yanı
  sıra, testin bunu gerçek HTTP API üzerinden tetikleyebilmesi için Birim
  5'in `GET/POST /api/chats/code-submode` uç noktası (bridge + handler +
  route + stub) bu birimde erkenden eklendi — Flutter tarafı hâlâ yok,
  sadece backend yüzeyi; (2) `drainAgentStream`'in yeni `(finishReason,
  chainable)` dönüş imzası `callWebSearchAgentStream`'in çağrı noktasını da
  (3 satır) etkiledi, davranış değişmeden. Test edilmeyen (gerçekçi
  olmayan) kenar durum: yerel llama.cpp modeli + plan modu + auto-perm açık
  kombinasyonu — kod incelemesiyle doğrulandı (bkz. "Bilinen sınır"), gerçek
  bir yerel model gerektirdiği için otomatik testte kapsanmadı.

---

## Birim 5 — Flutter: Tab-döngüsü + görsel gösterge

**Dosyalar:** `frontend/lib/core/api_client.dart`,
`frontend/lib/providers/agent_provider.dart`,
`frontend/lib/widgets/chat_input.dart`,
`frontend/lib/screens/agent_screen.dart`.

**Not:** Backend tarafı (`GET/POST /api/chats/code-submode` — bridge +
handler + route + stub) Birim 4'te erkenden eklendi (Unit 4'ün e2e
testlerinin gerçek HTTP API üzerinden sub-mode pinleyebilmesi için gerekti)
— burada sadece Flutter tarafı kaldı.

- `getChatCodeSubMode`/`setChatCodeSubMode` (api_client.dart) —
  `getChatCodeMode`/`setChatCodeMode` ile birebir aynı şekil.
- `chatCodeSubModeProvider` (agent_provider.dart) —
  `chatCodeModeProvider`'ın aynısı, auth-gate guard'ı dahil.
- `chat_input.dart`: **Tab için ikinci bir `Shortcuts` girdisi EKLENMEZ**
  (aynı tuş için iki map girdisi derleme zamanında öngörülemez şekilde
  çakışır) — mevcut `_PopupConfirmIntent`'in `isEnabledWhen`/`onInvoke`'u
  genişletilir: popup açıksa eskisi gibi popup'ı onaylar, değilse ve
  aktif sohbette Code Mode açıksa alt-modu `plan→auto→build→plan` döngüsüne
  sokar. Code Mode kapalıyken davranış birebir eskisi gibi kalır (erişilebilirlik
  tab-sırası bozulmaz).
- `agent_screen.dart`: `_CodeModeToggle` yerine `_CodeSubModeIndicator` —
  `engine_strip.dart`'taki nokta+ikon+`L10n.t(...)` rozet deseni, sohbete
  özel (`chatCodeSubModeProvider(chatId)`), plan/auto/build için ayrı
  renk+ikon, tıklayınca da döngüye sokuyor (klavyesiz erişim için).
  `chat_provider.dart`'ın SSE `finishReason` tüketimine `code_submode_changed`
  case'i eklenir (indikatörü akış ortasında canlı günceller).

**Doğrulama:** `flutter analyze lib/ && flutter test`, AGENTS.md kural #8
grep'i, gerçek uygulamada manuel tab-döngüsü + dosya-mention popup'ıyla
çakışmadığı kontrolü.

- [x] Yapıldı (commit'lendi 2026-09-15) — `flutter analyze` temiz (bilinen
  5 info dışında yeni uyarı yok), `flutter test` 341/341, Rule #8 grep boş.
  **Gerçek masaüstü uygulamada manuel/görsel doğrulama yapılmadı** (bu
  ortamda görsel bir Flutter Linux masaüstü test ortamı yok) — Tab
  döngüsünün dosya-mention popup'ıyla gerçekten çakışmadığı ve rozetin
  gerçek pencerede doğru göründüğü kullanıcı tarafından teyit edilmeli.

---

## Birim 6 — Ayarlardan üç prompt editörü + L10n

**Dosyalar:** `internal/webserver/handlers_flutter.go`/`bridge.go`/`server.go`,
`internal/app/settings.go`, `frontend/lib/core/api_client.dart`,
`frontend/lib/providers/settings_provider.dart`,
`frontend/lib/widgets/settings/tabs/code_submode_prompts_tab.dart` (yeni),
`frontend/lib/widgets/settings_dialog.dart`, `frontend/lib/core/l10n.dart`.

- Backend: `GET/POST /api/code-mode/prompt` + `POST /api/code-mode/prompt/reset`
  (`sub_mode` alanı ile parametreli) — `system_prompt_tab.dart`'ın
  `getSystemPrompt`/`setSystemPrompt`/`resetSystemPrompt` deseninin
  birebir aynısı, üç ayrı endpoint yerine tek parametreli çift.
- Frontend: üç `AsyncNotifierProvider<..., String>` (plan/auto/build),
  `SystemPromptNotifier` ile birebir aynı `build/save/reset` şekli.
- Yeni tek sekme (üç ayrı sekme değil), içinde üç `TextField(maxLines: 12,
  monospace) + Kaydet/Sıfırla` bölümü — `system_prompt_tab.dart`'ın
  yapısının üç kopyası. `settings_dialog.dart`'a tek `_tabs`/`_tabIcons`/
  `switch` girdisi.
- L10n: `code_submode_plan/_auto/_build`, `code_submode_tab_hint`,
  `code_submode_prompts_tab_title`, `code_submode_*_prompt_label` vb. —
  TR+EN aynı commit'te (kural #8), mevcut generic `save`/`reset` anahtarları
  varsa onlar tekrar kullanılır, yoksa yeni eklenir.

**Doğrulama:** `flutter analyze`/`test` + kural #8 grep + backend endpoint'ler
için Go handler testi.

- [ ] Yapıldı
