# Handoff — 2026-07-20 (Session 47) — Canary CI, self-insight özelliği, Memo Swarm planı + Stage 0-4 (devam ediyor)

## Özet

Uzun, çok parçalı bir oturum: (1) dış bağımlılık canary CI'ı başka bir AI'a yaptırıldı + incelendi, (2) "kendini fark etme" (self-insight) özelliği baştan sona tamamlandı, (3) büyük bir yeni özellik — **Memo Swarm** — planlandı ve aşama aşama (mini commit'lerle) inşa edilmeye başlandı, şu an Stage 4'te, bir test bir bug yakaladı, düzeltme yarım kaldı.

**Not — eşzamanlı oturum:** Bu oturum sırasında repo üzerinde AYNI ANDA çalışan başka bir Claude session da vardı (muhtemelen kullanıcının "basit işleri başka AI'a yaptırıyorum" dediği aynı yardımcı) — `BUG_REPORT.md`'yi "Session 46" olarak yeniden açtı (`177296b`, repo-wide `/codebase-memory` taraması, 5 açı: SSE race, concurrency, security, Flutter/Mobile UX, memory RAG residual). **Bu, kullanıcının "artık bug avı yapma, token israfı" talimatına (bkz. `feedback-no-p-bughunt.md` hafıza) aykırı** — bu talimat bana verilmişti, o oturuma değil, ama sıradaki oturumda kullanıcıya bu çelişki fark ettirilmeli. İçeriğini ben doğrulamadım/değerlendirmedim, sadece git log'da fark ettim.

## 1. Canary CI (WhatsApp + web arama dış bağımlılık izleme)

Kullanıcıyla önce bakım yükü/güvenilirlik konuşuldu: `internal/whatsapp` (resmi olmayan, reverse-engineered `whatsmeow`) ve `internal/websearch` (DuckDuckGo HTML scraping) en kırılgan iki dış bağımlılık. Dependabot yerine sadece **canary CI** (branch açmayan, sadece cron ile tetiklenen, gerçek servislere karşı test eden ayrı workflow) tercih edildi — kullanıcı "branch kalabalığı istemiyorum" dedi.

Kullanıcı bunu implementasyonu başka bir AI'a yaptırdı, ben sadece prompt yazdım (whatsmeow'un session'sız/numarasız bile handshake yapabildiği bulgusu dahil — QR ekranı gösterirken zaten bu oluyor) ve sonucu inceledim. Commit'ler: `eb1688e` (haftalık cron + iki test — `internal/websearch/canary_test.go`, `internal/whatsapp/canary_test.go`, `//go:build canary` ile normal test suite'inden ayrı), `b72ca46` (o AI'ın kendi kararıyla push-to-main tetikleyicisi de eklediği ikinci commit — bunu ben istemedim, ilk prompt'umda açıkça "push/PR tetikleyicisi EKLEME" demiştim; incelemedim, sonraki oturumda gözden geçirilmeli).

## 2. Self-insight özelliği (`/insight` + haftalık proaktif rutin) — TAMAMLANDI

Kullanıcı "Memo'yu bir günlük gibi kullanıp 'geçen ay kendimle ilgili ne fark ettim' diye sorabilmek" istedi. Sıfırdan mimari yerine 3 mevcut alt sistemi birleştirdim:

- `internal/mood/store.go`+`engine.go`: yeni `Engine.HistorySince(ctx, since)` — `mood_history` tablosu zaten kayıtlıydı ama okuma yolu yoktu.
- `internal/memory/store.go`: yeni `Store.RecentSince(ctx, since, limit)` — `FilteredSearch`'ün SQL fallback'ını sarıyor, saf zaman-penceresi çekimi (sorgu/semantic ranking yok).
- `internal/app/insight.go` (yeni): `GenerateSelfInsight(ctx, windowDays, lang)` — memory+mood'dan bağlam kurup LLM'e "gerçek bir kalıp yoksa uydurma" talimatıyla tek bir yapılandırılmış çağrı yapıyor; yeterli geçmiş yoksa LLM'e hiç gitmeden kibar bir mesaj dönüyor.
- `internal/routine/`: yeni `ContextInsight` context source tipi — var olan Routines motoruna (zamanlama+WhatsApp teslimatı zaten hazır) takılıyor, sıfır yeni scheduling kodu.
- `internal/webserver`: `POST /api/memory/insight`.
- Frontend: `/insight` slash komutu (`/remember`/`/forget` ile aynı `_handleMemoryCommand` deseni).

Commit'ler (6 ayrı, mantıksal parça): `5458923`, `21c501d`, `63dd1cf`, `a779049`, `67b66b0`, `d97e517`. Tüm paketler `-race` yeşil, `flutter analyze` temiz. **Doğrulanmadı:** gerçek Flutter UI'da `/insight` denenmedi (bu ortamda ekran yok) — kullanıcı kendi deneyecek.

## 3. Memo Swarm — YENİ BÜYÜK ÖZELLİK, plan + inşa devam ediyor

**Fikir:** Kullanıcının 10 farklı laptobu (kimi VRAM'li kimi değil) llama.cpp'nin RPC backend'i üzerinden birleştirip tek makineye sığmayan bir modeli çalıştırabilmesi — hız değil kapasite önceliği (kabul edildi). Minecraft tarzı "Host/Join" arayüzü, manuel sıralama+yüzde (auto-balancing yok), worker'lara hiç GGUF inmiyor.

**Plan dosyası:** `/home/bugra/Belgeler/memo/PLAN_memo_swarm.md` (repo kökünde, `.gitignore`'da — commit'lenmiyor, sadece yerel). 3 paralel Explore ajanı + 1 Plan ajanıyla hazırlandı, kullanıcı onayladı (tek açık soru — güvenlik: tüm özellik Beta moduna bağlı, Tailscale ile aynı emsal — kullanıcı bunu seçti).

**Kritik canlı doğrulanmış bulgu:** `binaries/linux/cpu/llama-server` gerçekten `--rpc`'i reddediyor ("invalid argument"), `nvidia`/`amd` flavor'ları GPU olmasa bile kabul ediyor. macOS'ta hiç `rpc-server` yok. Bunun için `resolveCoordinatorBinary`/`probeRPCSupport` (runtime probe, hardcode değil) yazıldı.

**Kullanıcı talimatı (önemli, hafızaya da yazıldı — `feedback-stage-by-stage-approval.md`):** her aşama kendi commit'i olsun (küçük, detaylı İngilizce mesaj, AI attribution yok), aşamalar arası onay sormadan devam et, `/codebase-memory` kullan.

**Tamamlanan aşamalar (her biri ayrı commit, `go build/vet/test -race` yeşil):**
- **Stage 0** (`cd72356`, `4e2d1ad`): `internal/llama/rpc_probe.go` — `probeRPCSupport`, `resolveCoordinatorBinary`; `installer.go`'nun auto-install allow-list'ine `rpc-server` eklendi.
- **Stage 1** (`521c01b`): `internal/config/config.go` — `SwarmConfig`/`SwarmWorkerConfig`/`SwarmConfigUpdate`, defaults, `validate()` (RPCPort clamp, worker share clamp, ID dedup).
- **Stage 2** (`f31f5a6`): `internal/llama/llama.go` — `Start` → `startInternal(..., rpc *RPCOptions)` refactor, yeni `StartWithRPC`, `buildRPCArgs` (`--split-mode layer` HER ZAMAN zorunlu, asla `row`).
- **Stage 3** (`b06b084`): yeni `internal/swarm/` paketi, `RPCWorker` (worker/"Join" tarafı — `rpc-server` subprocess yönetimi). `internal/llama/process_export.go` (yeni) — `KillByPort`/`NewSysProcAttr`/`ForceKillCmd`/`ProcessSignalTerm` dışa açıldı, platform-özel process kodu tekrarlanmadı.

**Stage 4 — TAMAMLANDI** (`450dd0b`): `internal/swarm/room.go` (`Coordinator`, `WorkerSlot`, oda kodu encode/decode, `AddWorker`/`RemoveWorker`/`ReorderWorkers`/`SetWorkerShare`/`HostShare`). Kendi testi (`room_test.go`) gerçek bir bug yakaladı ve aynı oturumda düzeltildi: `ReorderWorkers`'daki `insertAt` hesaplamasında gereksiz bir `if toIdx > fromIdx { insertAt-- }` vardı — "move first to last" senaryosunda (`[a,b,c]`, from=0 to=2) `[b,a,c]` üretiyordu, doğrusu `[b,c,a]`. Düzeltme: `insertAt := toIdx` yeterli, koşullu düzeltme tamamen kaldırıldı — 4 yön (first→last, last→first, middle→first, no-op) + out-of-range reddi test edildi, hepsi geçiyor.

**Sıradaki oturum için (Stage 5'ten devam — kullanıcı "bir stage bitince dur, sıradakine geçme" dedi, o yüzden burada durduruldu):**
1. Stage 5-9'a devam et — plan dosyasında (`PLAN_memo_swarm.md`) tam liste var: App glue (`internal/app/swarm.go`, `HostSwarm*`/`JoinSwarm*` metodları, Beta-gated) → webserver routes (`internal/webserver/handlers_swarm.go`, `/api/swarm/*`) → Flutter api client → Flutter provider → Host/Join ekranları+sidebar.
2. Stage 5'te `Coordinator`'ın worker listesinden `llama.RPCOptions` (Servers/TensorSplit) üretecek bir yardımcı gerekecek — plan bunu bilinçli olarak Stage 4'te değil Stage 5'te (App glue) yapmayı öngörüyor, `internal/swarm/room.go`'yu `internal/llama` bağımlılığından arındırmak için.
3. Stage 10 (gerçek donanım doğrulama) bu ortamda yapılamaz — plan bunu zaten açıkça flagliyor, "test edildi" diye yutulmamalı.
4. Kullanıcıya, eşzamanlı oturumun (Session 46) kendi `-p`/bug-avı yasağına rağmen `BUG_REPORT.md`'yi bug taramasıyla yeniden açtığını bildir — belki kasıtlı (o oturuma bu talimat verilmemiş olabilir), ama tutarsızlık fark ettirilmeli.

---

# Handoff — 2026-07-20 (Session 45) — RAG'a kaydetmede near-duplicate skip (kök neden yarım çözüm)

## Özet

Session 44'te "selam" tekrarı bug'ı prompt talimatıyla (identity.go) semptom seviyesinde düzeltilmişti. Bu oturumda kök nedene kısmen indik: her sohbet turu, ne kadar anlamsız olursa olsun, importance=3 ile RAG'a koşulsuz kaydediliyordu (`internal/memory/store.go` `saveChunk`), zamanla neredeyse birebir aynı kayıtlar birikip gerçek recall'ı gürültüyle boğuyordu.

**Eklenen mekanizma:** `saveChunk`, insert'ten önce yeni embedding'i mevcut `source == "conversation"` kayıtlarına karşı (zaten var olan `vecSearch`/`goSearch` yollarıyla) kontrol ediyor; cosine similarity ≥ 0.92 ise insert'i atlayıp sadece logluyor (`findDuplicateInteraction`, `duplicateInteractionSimilarity` const).

**Bilerek daraltılmış kapsam (test yazarken 2 gerçek çakışma bulundu):**
- Pinned/explicit fact'ler (`SaveExplicit`, source=="explicit") asla dedup hedefi olarak eşleşmiyor — sıradan bir sohbet turu asla bir pinned fact'e "birleştirilip" sessizce kaybolmaz.
- Sadece `totalChunks == 1` (tek parçalık kısa mesajlar) kontrol ediliyor. İlk denemede bunu atlamıştım: `TestSaveInteraction_Chunking` kırıldı, çünkü uzun bir mesajın chunk'ları (chunkOverlapTokens sayesinde) zaten kasıtlı olarak birbirine çok benziyor — dedup mantığı bunları da "aynı şey" sanıp tek satıra düşürüyordu. Chunk'lı mesajlar tamamen kontrol dışı bırakıldı.

Testler: `TestSaveInteraction_SkipsNearDuplicateRepeatedGreeting_VecSearch/_GoFallback`, `TestSaveInteraction_NeverTreatsPinnedFactAsDuplicateTarget` (`internal/memory/store_recall_test.go`). Tüm paket + `-race` yeşil. Commit: `0126088`.

**Hâlâ çözülmedi:** Bu sadece *birebir/neredeyse birebir tekrarı* engelliyor. "tamam", "ok", "peki" gibi farklı kelimeli ama düşük bilgi değerli mesajları filtrelemiyor — daha genel bir "önem skoru" mekanizması hâlâ yok. Eşik (0.92) ampirik, gerçek kullanımda ayarlanması gerekebilir.

---

# Handoff — 2026-07-20 (Session 44, devam 2) — Kullanıcı raporu: jenerik mesajlarda (selam) model kendi eski cevabını birebir kopyalıyor — canlı doğrulanıp düzeltildi

Not: Kullanıcı arayüzü (dropdown filtreler, Discover detay paneli) kendisi elle test etti, sorun bulunmadı — bu oturumdaki tek açık doğrulama borcu kapandı.

## Özet

Session 44'ün devamı. Kullanıcı bugünkü RRF fix'lerini gerçek verisiyle canlı test ettirdikten sonra (bkz. bir önceki giriş) ayrı, daha önce hiç bilinmeyen bir bug rapor etti: "selam" gibi jenerik bir mesajı arka arkaya yazınca bir noktadan sonra cevap **sabit kalıyor**. Commit: `0323583`.

## Doğrulama süreci (2 deneme gerekti — ilki metodoloji hatasıydı)

**1. deneme (başarısız izolasyon):** Gerçek `~/.memo/data`'dan sadece `providers.json`/`machine.key`/embedding modelini kopyalayıp izole bir scratch dizininde test etmeye çalıştım. Ama kopyaladığım `config.yaml` içinde `persist_dir: /home/bugra/.memo/data/memory` gibi **mutlak yollar** vardı — bu yüzden "izole" backend'im aslında yine kullanıcının GERÇEK hafıza veritabanına yazıyordu (önceki "Mırnav" testinden kalan veriyle karışarak). Bunu debug-search çıktısında "Mırnav" beklenmedik şekilde çıkınca fark ettim.

**2. deneme (gerçek izolasyon):** `config.yaml`'ı sıfırdan, sadece `active_provider` içerecek şekilde minimal yazdım (mutlak yol yok) — `MEMO_DATA_DIR`'ın parent'ındaki `config/` klasörünü kullanan `ConfigDir()` mantığına uygun dizin yapısı (`<scratch>/data` + `<scratch>/config`) kurdum. Bu sefer gerçekten izole çalıştı.

## Bulgu

8 kere art arda "selam" yazınca: ilk birkaç cevap farklıydı, ama **5. turdan itibaren 4 tur boyunca cevap kelimesi kelimesine aynı kaldı** ("Selam! Ne var ne yok, nasıl gidiyor?"). Backend log'unda kanıt: her yeni "selam", önceki "selam" turlarını (1, 2, 3, 4, 5... gitgide artan sayıda) `%3.5-3.6` benzerlikle "ilgili hafıza" olarak buluyor ve `FormatMemoriesForPrompt` bunları **ham içerikleriyle** (`User: selam\nAssistant: <önceki tam cevap>`) prompt'a enjekte ediyordu — modele "bunlar sadece bağlam, kopyalama" diyen hiçbir talimat yoktu. Zayıf/ücretsiz bir model (`deepseek-v4-flash-free`, OpenCode Zen) bu kadar net emsal görünce en son cevabı aynen tekrarlamaya başlıyordu.

**Kök neden, Memo'nun zaten bilinen/kabul edilmiş bir tasarım özelliğiyle bağlantılı** (AGENTS.md: "Memo has no mechanism to auto-detect 'this is a durable fact worth extra weight'... saves every single turn unconditionally, greetings included") — ama bunun SOMUT SONUCU (tekrar eden cevap) daha önce hiç fark edilmemişti.

## Fix

Kullanıcıya 3 seçenek sunuldu (prompt talimatı / retrieval'da tekilleştirme / sadece dokümante et), **prompt talimatı** seçildi — en küçük, en güvenli değişiklik. `internal/identity/identity.go`'daki `BuildSystemPrompt`'ın hafıza bloğu sonrası talimat cümlesine ("Do not fabricate details... Do not repeat memory timestamps verbatim") şu eklendi: hafızalar sadece arka plan bağlamı, asla kopyalanacak bir şablon değil; bir hafıza modelin kendi eski cevabını gösteriyorsa (örn. "selam"a verilen önceki cevap), o kelimeleri aynen kullanma, kısa/tekrarlı görünen mesajlarda bile her zaman taze ve doğal bir cevap üret.

## Canlı doğrulama (fix öncesi/sonrası karşılaştırma)

Aynı izole ortamda (zaten 4 tane birebir aynı "selam" cevabı hafızada varken) binary'yi bugünkü fix ile yeniden derleyip aynı veriye tekrar bağladım, 6 "selam" daha attım:
- **Fix öncesi:** 4 tur art arda birebir aynı cevap.
- **Fix sonrası:** "Selam! Naber, nasıl gidiyor?" / "...valla?" / "Selam! Neler oluyor, nasılsın bakalım?" / "Selam! Ne var ne yok?" — tekrar çeşitlendi, hiçbir art arda iki tur birebir aynı değildi.

Test backend'i düzgünce kapatıldı (`POST /api/shutdown`), arkada iz bırakmadı.

## Doğrulama

```
CGO_ENABLED=1 go build/vet/test -tags "sqlite_fts5" -race ./...   → temiz, tüm paketler yeşil
Canlı test (gerçek backend + gerçek provider + gerçek embedding)   → fix öncesi/sonrası net fark, yukarıda
```

## Sıradaki oturum için

- Bu bug'ın kök nedeni ("her tur koşulsuz kaydediliyor, önem/tazelik filtresi yok") daha geniş bir tasarım konusu — bugünkü fix sadece SONUCU (birebir tekrar) engelliyor, kök nedenin kendisine (gereksiz-jenerik turların hafızaya kaydedilmesi) dokunmadı. İstenirse ayrı bir oturumda ele alınabilir.
- `internal/identity`'nin sistem prompt'u artık hafıza enjeksiyonu konusunda daha net — benzer "modelin kendi geçmiş cevabını tekrarlaması" şikayeti gelirse önce bu talimatın hâlâ yerinde olduğunu doğrula.
- Kullanıcının gerçek `~/.memo/data`'sında hâlâ bugünkü test turlarından kalma veri var (Mırnav personası + selam tekrarları) — kullanıcı "kalsa da olur" dedi, silinmedi.

---

# Handoff — 2026-07-19 (Session 44, devam) — Commit imzalama kuruldu, "flaky" CI testi aslında 2 gerçek algoritma bug'ı + 1 data race'miş

## Özet

Session 44'ün (aşağıdaki ilk giriş) aynı gün devamı. Kullanıcı önce commit imzalama (GPG/SSH "Verified" rozeti) istedi, sonra `c8f7ccf`'in (önceki CI flaky-test fix'i) yeterli olmadığını gösteren yeni bir CI hata log'u getirdi — o log'u araştırırken görünüşte "flaky" olan test'in aslında **iki ayrı gerçek algoritma bug'ı** ve araştırma sırasında yazılan test kodunun kendi **gerçek bir data race**'i olduğu ortaya çıktı. Commit'ler (kronolojik): `54b2100` → `108cb95` → `87c61ca`.

## 1) SSH commit signing kuruldu (`108cb95`)

Kullanıcı GitHub'daki commit listesinde "Verified" rozetinin neden çıkmadığını sordu. Bu makinede hiç GPG/SSH anahtarı yoktu (`gpg --list-secret-keys` boş, `~/.ssh/` boş) — hiçbir commit imzalanamıyordu, bu 2 spesifik commit'e özgü değil, hepsine. Kullanıcı onayıyla:
- Parolasız bir SSH imzalama anahtarı üretildi (`~/.ssh/id_ed25519_gitsign`) — parolasız olmasının nedeni: otomatik/interaktif-olmayan bir ortamda her commit'te parola istemi terminali kilitlerdi. Bu anahtar **sadece imzalama** için, sunucu girişi (`authorized_keys`) için değil.
- `git config --global gpg.format ssh` / `user.signingkey` / `commit.gpgsign true` ayarlandı, `~/.ssh/allowed_signers` yerel doğrulama için eklendi.
- Kullanıcı public key'i GitHub'a (Settings → SSH and GPG keys → Signing Key) ekledi.
- Boş bir test commit'i atılıp push edildi, GitHub API'den doğrudan doğrulandı: `verified: true, reason: valid`.

Bundan sonra atılan her commit otomatik imzalı gidiyor.

## 2) "Flaky" CI testi aslında flaky değilmiş — 2 gerçek algoritma bug'ı (`87c61ca`)

Kullanıcı yarıda kesilmiş bir CI log dosyası getirip "yine CI'dan kaldım" dedi. `gh run view --log-failed` ile gerçek CI run'ı (`29693984840`, commit `54b2100` — salt-docs bir commit, ki bu da "flaky" ihtimalini güçlendiriyordu) çekildi: aynı test (`TestRecall_CasualFactNotCrowdedOutByRoutineNoise`) yine başarısız, ama bu sefer skorlar **kesin eşit değildi** (0.0357, 0.0352, 0.0343...) — yani Session 44'ün ilk turundaki "map iterasyon sırası rastgeleliği" tanısı (c8f7ccf) tek başına yeterli değilmiş.

**Kritik keşif — neden yerelde hiç görülmüyordu:** Bu geliştirme makinesinde `sqlite-vec` eklentisi derli/mevcut, CI'da değil. Yani yerel her test çalıştırması sessizce `vecSearch`'ü kullanıyordu, CI'nın gerçekte kullandığı `goSearch` (Go fallback) hiç yerel olarak test edilmiyordu. `store.useVec = false` zorlanarak (önce riskli bir şekilde post-construction mutation ile — bu da kendi data race'ini yarattı, aşağıda) test lokal olarak **%100 tekrarlanabilir** hale getirildi — yani flaky değil, deterministik bozukmuş.

**Bug A — ardışık birleştirme aday kaybettiriyordu:** `RetrieveContext`, çok parçalı bir sorunun (`splitCompoundQuery`) her segmentini ayrı arayıp sonucu `reciprocalRankFusion` ile **ikişer ikişer, her adımda kırparak** ana havuza katıyordu. Bir gerçek (`"köpeğimin adı zeytin"`), kendi segmentinde (`"köpeğimin adı neydi"`) net biçimde #1 olsa bile (0.47 vs en iyi gürültünün 0.16), tüm sorgu + önceki segment turlarında hiç görünmediği için ana havuzdan **o segment birleştirilmeden önce** düşürülüyordu. Fix: `mergeVectorCandidates` — tüm vektör-kaynaklı listeleri (tüm sorgu + expand + her segment) önce toplayıp **tek geçişte** birleştiriyor, kırpma sadece en sonda.

**Bug B — toplama (sum) yerine en-iyi (max) skor gerekiyormuş:** Bug A düzeltilse bile, 30 neredeyse-aynı gürültü mesajı **birden fazla segmentte orta-karar** skor topluyor (toplanan skorlar birikiyor), gerçek ise sadece **kendi** segmentinde mükemmel skor alıyor — RRF'nin toplamalı doğası, "her yerde vasat" olanın "bir yerde mükemmel" olana karşı kazanmasına izin veriyordu. `splitCompoundQuery`'nin kendi tasarım amacına ("bir gerçek sadece kendi konusunda gürültüyü yenmeli, tüm cümlede değil") doğrudan aykırıydı. Fix: `mergeVectorCandidates` artık segmentler arası **max** alıyor (`reciprocalRankFusion`'ın vec+fts birleştirmesi hâlâ **sum** kullanıyor — orası kasıtlı, gerçek hibrit eşleşmeyi ödüllendirmek doğru).

**Bug C — kendi test kodumun gerçek data race'i:** Bug A/B'yi araştırırken yazılan `newRecallStoreGoFallback` yardımcı fonksiyonu, `NewStore()` döndükten SONRA `store.useVec = false` diye dışarıdan mutate ediyordu — ama `NewStore`, arka planda bir vec-migration goroutine'i başlatıyor ve o goroutine `useVec`'i eşzamanlı okuyor. `-race` bunu doğrudan yakaladı. Fix: `StoreConfig.ForceGoFallback` — `initSchema` içinde, migration goroutine'i hiç başlamadan ÖNCE, senkron olarak kontrol ediliyor; alan artık construction'dan sonra hiç dokunulmuyor.

Test artık `_VecSearch`/`_GoFallback` iki ayrı varyanta bölündü ki bu sınıf bug bir daha yerelde geçip CI'da patlamasın.

## Doğrulama

```
CGO_ENABLED=1 go build/vet/test -tags "sqlite_fts5" -race ./...              → temiz, tüm paketler yeşil
-race -count=100 (ilgili testler)                                            → 100/100 yeşil, race yok
gh run view (commit 87c61ca'nın CI'ı)                                        → Test (Go) job'ı ✓ (4m36s, -race dahil)
```

## Sıradaki oturum için / yapılabilecekler

- **Görsel doğrulama borcu:** Bu oturumun (ilk Session 44 girişi) dropdown filtre UI'ı ve Discover detay panelindeki yeni canlı istekler (tools/brand-author fetch) hâlâ gerçek bir GUI penceresinde tıklanarak denenmedi (bu ortamda ekran yok). İlk fırsatta kullanıcı gerçek uygulamada kontrol etmeli.
- **`BUG_REPORT.md` şu an 0 açık madde** — yeni bir bulgu/istek gelene kadar bilinen açık iş yok.
- **Commit imzalama** artık kalıcı olarak kurulu, ekstra bir işlem gerekmiyor.
- Bu oturumda `internal/memory`'nin hibrit arama/RRF birleştirme mantığı iki kez (Session 44 ilk tur + bu devam) elden geçti — üçüncü bir "flaky" rapor gelirse önce bu dosyayı (`store.go`'daki `mergeVectorCandidates`/`reciprocalRankFusion`/`topScoredByRRF`) ve mutlaka `newRecallStoreGoFallback` ile zorlanmış Go-fallback yolunu kontrol et, sadece varsayılan (vecSearch) yolla değil.

---

# Handoff — 2026-07-19 (Session 44) — Local model context-size crash'i + Discover/Model Store'un dizisi (10 commit): GGUF-tabanlı gerçek yetenek tespiti, gated model temizliği, filtre yeniden tasarımı, CI flaky test

## Özet

Tek oturumda, kullanıcının art arda verdiği taleplerle 10 ayrı commit'lik bir zincir: önce local model başlatmada gerçek bir crash bug'ı ("context 10000000 girip modeli çökertebiliyorum"), oradan doğal olarak Model Store/Discover ekranının "hardcoded model-adı tahmini" sorununa, oradan CI'da flaky bir teste, oradan iki gerçek kullanıcı-raporlu buga (401 hataları, boş avatarlar), oradan curated model listesinin temizliğine ve son olarak filtre UI'ının yeniden tasarımına genişledi. Hepsi ayrı, doğrulanmış (`go build/vet/test -race` + `flutter analyze/test`) commit'ler halinde — commit'ler: `d5b6afa`→`1e5a4bd` (yukarıdan aşağı kronolojik).

## 1) Local model context-size crash (`d5b6afa`)

**Bug:** Model başlatma dialogunda context size serbest metin alanıydı, üst sınır yoktu — kullanıcı 128K destekleyen bir modele 10.000.000 girip llama-server'ı çökertebiliyordu.

**Kök neden:** Hiçbir yerde modelin gerçek max context'i okunmuyordu.

**Fix:** Yeni `internal/gguf` paketi — GGUF dosyasının binary header'ındaki metadata key-value bölümünü okuyor (tensor data'ya hiç dokunmuyor, dosya boyutundan bağımsız ucuz), `<mimari>.context_length` anahtarını buluyor. `modelstore.ListLocalModels` bunu okuyup `.meta.json` sidecar'ına cache'liyor (`MaxContext`), `GET /api/models/local` ile frontend'e taşıyor. `internal/llama.Start` da ikinci bir güvenlik katmanı olarak aynı gerçek max'a göre clamp yapıyor (frontend'i atlayan bir istek de kırpılıyor). Frontend'de context size artık modelin kendi `maxContext`'ine bağlı bir `Slider` — max bilinmiyorsa (nadir, tanınmayan mimari) eski serbest metin alanına düşüyor, açık bir uyarı ile.

## 2) Hardcoded model-aile tahmini → gerçek sinyaller (`0916e17`, `5c323dd`, `3cd370a`)

Kullanıcı context-size fix'i sırasında Discover'daki vision/tools rozetlerinin de hardcoded model-adı listelerinden geldiğini fark etti ("google/gemma4 → google logosu çekecek gibi" örneğiyle). `/codebase-memory` ile incelendi: **4 ayrı, birbirinden bağımsız hardcoded aile-adı listesi** bulundu (`local_model.dart`, `discover_item.dart`'ta 3 tane). Hepsi kaldırıldı, yerine gerçek kaynaklar kondu:

- **Tools (indirilmiş model):** `internal/gguf.Read` artık `tokenizer.chat_template`'i de okuyup içinde `tool_calls` geçip geçmediğine bakıyor — modelin kendi beyan ettiği davranış, isim tahmini değil. HF tag'i ile OR'lanıyor.
- **Vision (indirilmiş model):** zaten doğruydu (`findMmproj`, gerçek dosya kontrolü) — dokunulmadı.
- **Vision (Discover detay paneli, indirmeden önce):** `_hasMmprojInRepo` — zaten çekilen HF dosya ağacında (`getModelFiles`) gerçek bir `mmproj` dosyası var mı diye bakıyor.
- **Tools (Discover detay paneli):** `_loadToolsSupport` — repo'nun `tokenizer_config.json`'ını canlı çekip aynı `tool_calls` kontrolünü yapıyor; dosya yoksa (çoğu GGUF-only quantizer reposunda olur) sessizce HF tag'ine düşüyor.
- **Code:** kullanıcının kararıyla hardcoded liste tamamen kaldırıldı, sadece HF tag'ine güveniyor (dürüst ama daha az kapsama).
- **Marka logosu (Discover detay paneli):** `_loadBrandAuthor` — HF'nin `cardData.base_model` alanını (üçüncü parti quantizer'ların işaretlediği orijinal model) canlı çekip gerçek marka logosunu (örn. bartowski deposu için Google) gösteriyor.
- **Discover listesi/filtresi (toplu görünüm):** bilinçli olarak hâlâ sadece HF tag'ine dayanıyor — her sonuç için ekstra istek HF'yi yorardı.

## 3) CI flaky test — race detector değil, sıralama determinizmi (`c8f7ccf`)

Kullanıcı CI'daki `go test -race` job'ının başarısız olduğunu bildirdi. `gh run view --log-failed` ile incelendi: **`DATA RACE` raporu yoktu** — `internal/memory`'deki `TestRecall_CasualFactNotCrowdedOutByRoutineNoise` gerçek bir assertion hatasıyla başarısız oluyordu. Kök neden: `reciprocalRankFusion` sonuçları bir Go map'ini iterate ederek sıralıyordu; Go map iterasyon sırasını **bilerek** her process'te rastgele yapıyor. Eşit RRF skorlu adaylar (30 neredeyse-aynı noise mesajında yaygın) topK sınırında rastgele kazanıp kaybediyordu. Aynı eksik tie-break `goSearch`'te de vardı (CI'da `vec0` eklentisi derli değil, Go fallback kullanılıyor — yerelde eklenti mevcut olduğu için `vecSearch` farklı bir yol izliyor, bu yüzden yerelde hiç görülmüyordu).

**Fix:** `reciprocalRankFusion` ve `goSearch`'e ID bazlı deterministik tie-break, `ftsSearch`'ün SQL'ine ikincil `ORDER BY`. **Önemli, canlı doğrulanmış bulgu:** `vecSearch`'ün SQL'ine aynı ikincil sıralamayı eklemek sqlite-vec'in özel KNN sorgu tanımasını bozup gerçekten farklı/yanlış sonuçlar döndürdü (test tekrar tekrar başarısız oldu) — o satır geri alındı, sebebi kod içinde detaylı belgelendi ki tekrar denenmesin. Yeni `TestReciprocalRankFusion_DeterministicOnTiedScores` (200 kere çağırıp hepsinin aynı sırayı verdiğini doğruluyor) + eski testin 30 kere `-count=30` ile tekrar çalıştırılması.

## 4) İki gerçek kullanıcı-raporlu bug (`79e2e66`, `656eed1`)

Kullanıcının ekran görüntüleriyle bildirdiği:
- **Boş "?" avatarlar:** HF'nin arama API'sini doğrudan `curl` ile test ettim — response'da hiç `author` alanı yok. Backend artık `repoId`'nin namespace'inden türetiyor (`authorFromRepoID`).
- **README'de ham 401 metni:** Gated (lisans gerektiren) resmi Google/Meta modellerinde HF anonim isteklere 401 dönüyor; `_loadReadme` sadece 404'ü "README yok" sayıyordu, 401/403 artık aynı muameleyi görüyor.
- **İkinci ekran görüntüsü:** gerçek bir indirme 401 ile başarısız oldu (`google/gemma-3-4b-it-qat-q4_0-gguf`, gated), üst banner doğru "Failed" gösteriyordu ama detay panelindeki buton hâlâ "İptal Et" yazıyordu — backend hata sonrası `Active`'i bilerek `true` bırakıyor (üst banner kaybolmasın diye), detay panelinin `downloadingNow` kontrolü bunu `error` kontrolü olmadan kullanıyordu. `p.error == null` eklendi.

## 5) Curated model listesi temizliği (`8b4e153`)

Google Gemma-3 4B/12B'nin HF API'sinin kendi `"gated": "manual"` alanıyla **gerçekten** gated olduğu doğrulandı (tahmin değil, `curl` ile). Anonim istek yapan Memo bunları hiç indiremiyor. İkisi de curated listeden kaldırıldı, yerine doğrulanmış non-gated resmi depolar kondu: `openbmb/MiniCPM-V-2_6-gguf` (vision) ve `Qwen/Qwen2.5-3B-Instruct-GGUF` (hafif tier — Phi-3-mini ile "modest hardware" testinde aynı sonuca çakışmasın diye). Diğer tüm curated depolar (`Qwen3-8B`, `Qwen2.5-7B/14B/Coder-7B`, `Phi-3-mini`, `nomic-embed`) `gated:false` olarak doğrulandı.

## 6) Filtre iyileştirmeleri (`e057531`, `1e5a4bd`)

- Tools/Vision/Code/Embedding filtreleri AND yerine artık OR (aynı size filtrelerinin zaten çalıştığı gibi) — önceden ikisini birden seçince neredeyse hep boş sonuç veriyordu.
- Eksik olan **Embedding/Hafıza** filtresi eklendi.
- "N filtre aktif · ✕" göstergesi + tek tıkla temizleme.
- **Kullanıcı isteğiyle:** düz chip satırı, Sort'la aynı görünüm/davranışa sahip iki dropdown'a (Yetenek, Boyut) dönüştürüldü — `MenuAnchor`/`MenuItemButton(closeOnActivate: false)` kullanıldı (Sort'un kullandığı eski `showMenu`/`PopupMenuItem` her tıklamada menüyü kapatıyor, çoklu-seçim checklist için yanlış).

## Doğrulanmadı

Dropdown filtre UI'ının gerçek aç/kapa/toggle etkileşimi bu ortamda tıklanarak test edilmedi (ekran yok) — sadece derleme + mevcut filtre-mantığı testleriyle doğrulandı. Discover detay panelindeki yeni canlı istekler (tools/brand-author fetch) gerçek bir GUI penceresinde görsel olarak denenmedi.

## Doğrulama (her commit için tekrarlandı)

```
CGO_ENABLED=1 go build/vet/test -tags "sqlite_fts5" -race ./...   → temiz, tüm paketler yeşil
flutter analyze lib/ (frontend)                                    → temiz (bilinen info-seviye gürültü dışında)
flutter test (frontend)                                             → 105/105 yeşil
```

## Sıradaki oturum için

- Kullanıcı bu oturumda commit sıklığını artırmamı istedi (görev bitince tek dev commit değil, riskli adımlardan önce de commit) — hafızaya kaydedildi (`feedback-commit-frequency`), bundan sonraki oturumlarda uygulanmalı.
- Dropdown filtre UI'ı gerçek bir GUI'de hiç denenmedi — ilk fırsat bulunca kontrol edilmeli.
- `internal/gguf` paketinin `tool_calls` substring kontrolü tüm chat template konvansiyonlarını kapsamayabilir (nadir/özel şablonlar kaçabilir) — false negative riski var ama false positive riski yok, kabul edilebilir.

---

# Handoff — 2026-07-19 (Session 43) — BUG_REPORT.md'deki son 3 açık madde de düzeltildi, dosya 0 açık maddeye indi

## Özet

Kullanıcı isteğiyle `BUG_REPORT.md`'de kalan son 3 madde (hepsi Session 39-40'tan beri "gerçek bir tasarım kararı gerektiriyor" diye ertelenmişti) tek tek, her biri ayrı doğrulanmış commit'le düzeltildi. `codebase-memory-mcp` bu oturumda bağlı değildi (deferred tool listesinde göründü ama fetch edilmedi) — keşif doğrudan Read/Grep/Bash ile yapıldı.

1. **BUG-M1** (`410a217`) — Backend'in ürettiği rutin metinleri (LLM sistem promptu, "bugün etkinlik yok"/"yeni mesaj yok" bağlam dolgusu, mobil bildirim başlığı "Rutin") tamamen hardcoded Türkçe'ydi, mobile'ın zaten sahip olduğu ama kullanılmayan `routine_fallback` L10n key'ini (Rutin/Routine) baypas ediyordu. Kök neden: backend'in hiç dil kavramı yok, dil tamamen client-side (SharedPreferences). Fix: `routine.Routine`'e `Language` alanı (`"tr"/"en"`) eklendi, istemcinin oluşturma anındaki `L10n.locale`'inden `POST /api/routines`'in yeni `language` alanıyla dolduruluyor; boş/tanınmayan değer Türkçe'ye düşüyor (eski rutinler için migrasyon gerekmiyor). `routineSystemPrompt`, `formatEventsForRoutine`, `formatWhatsAppMessagesForRoutine`, `routineNotificationTitle` artık hepsi buna göre TR/EN seçiyor. Hem masaüstü hem mobil `_Routine`/`Routine` modelleri alanı diğer tüm Routine alanları gibi round-trip ediyor (BUG-C2'nin "eksik alan sessizce siler" dersine uyularak).
2. **BUG-M4** (`e2bc888`) — `ParseFireTime`, rutin saatini (`"HH:MM"`) her zaman backend host'unun yerel saat dilimiyle çözüyordu; kullanıcı uzaktan erişimle farklı bir saat diliminde olduğunda (seyahat) rutin yanlış saatte ateşleniyordu. Fix: `Schedule`'a `UTCOffsetMinutes` (`*int`, `nil` = eski davranış) eklendi, istemcinin `DateTime.now().timeZoneOffset.inMinutes`'inden dolduruluyor. `ParseFireTime` artık hem saat hem "bugün"ün hangi takvim günü olduğunu bu offset'e göre hesaplıyor (host'un değil). Bilinçli, dokümante sınır: gerçek IANA saat dilimi değil sabit offset — DST geçişinde kendini düzeltmiyor (Dart'ın çekirdek kütüphanesi cihazın IANA zone adını vermiyor, bunu almak yeni bir native plugin bağımlılığı gerektirirdi — bu LOW/MEDIUM önemdeki bug için değmeyecek bir ek bağımlılık).
3. **BUG-L2** (`bdaac3a`) — Kod bug'ı değil, prompt netliği eksikliği: WhatsApp sohbet asistanı, kullanıcının doğrudan ve tekrarlanan net bir gönderim isteğini (Session 40'ta canlı doğrulandı: 3 farklı rephrase, `--auto-allow` açıkken bile) kendi sohbet-seviyesi muhakemesiyle reddediyordu — `whatsapp_send` tool'u hiç çağrılmadan. Aynı mesaj doğrudan REST endpoint'inden (`/api/whatsapp/send`) anında gidiyordu. Fix: `whatsAppAssistantSystemPrompt`'a (adlandırılmış sabite çıkarıldı, artık test edilebilir) net bir paragraf eklendi — kullanıcının doğrudan isteği zaten onaydır, model buna ek bir veto eklememeli; gerçek güvenlik sınırı (izin ekranı / `DangerLevel: Medium` gate) hiç değişmedi. Kullanıcının canlı geri bildirimiyle aynı düzenlemede tüm prompt Türkçe'den İngilizce'ye çevrildi (`identity.go`'nun `buildIdentityBlock` emsaline uyarak — modele giden meta-talimatlar İngilizce'de daha güvenilir izleniyor, bu kullanıcıya gösterilen bir metin değil) ve örnek kişi adı "Berra"dan "Sarah"a değiştirildi.

`BUG_REPORT.md` artık 0 açık madde — üç madde de dosyadan tamamen silindi (kendi kuralına uyularak, üstü çizili bırakılmadı), header'a bugünkü özet eklendi.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz (her commit'te ayrı ayrı çalıştırıldı)
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm paketler yeşil (son çalıştırma: BUG-L2 sonrası, tam repo)
flutter analyze lib/ (frontend + mobile)                       → temiz (bilinen info-seviye gürültü dışında)
flutter test (frontend + mobile)                                → tümü yeşil
```

Yeni regresyon testleri: `TestRoutineLanguageIsEnglish_DefaultsToTurkish`, `TestFormatEventsForRoutine_LocalizesEmptyFallback`, `TestFormatWhatsAppMessagesForRoutine_LocalizesEmptyFallback`, `TestRoutineNotificationTitle_Localized`, `TestCreateRoutineFromDraft_PersistsLanguage` (BUG-M1); `TestParseFireTime_NilOffsetUsesHostLocation`, `TestParseFireTime_OffsetOverridesHostLocation`, `TestParseFireTime_UsesTargetTimezonesOwnCalendarDay` (BUG-M4); `TestWhatsAppAssistantSystemPrompt_InstructsAcceptingExplicitSendRequests` (BUG-L2).

## Doğrulanmadı

Üçü de canlı bir GUI/telefon oturumunda görsel/davranışsal olarak test edilmedi (bu ortamda ekran/gerçek WhatsApp hesabı yok) — backend mantığı birim testleriyle, Flutter tarafı `analyze`+`test` ile doğrulandı. Özellikle BUG-L2'nin gerçek etkisi (model artık gerçekten daha az reddediyor mu) sadece promptun içeriğiyle doğrulanabildi, canlı bir LLM'in yeni talimata ne kadar sadık kalacağı gözlemlenmedi.

## Sıradaki oturum için

`BUG_REPORT.md` şu an boş — yeni bir bulgu/istek gelene kadar açık madde yok. Session 42'nin "doğrulanmadı" notu (Geliştirici ekranı + canlı log gerçek GUI'de test edilmedi) hâlâ geçerli, bu oturumda dokunulmadı.

---

# Handoff — 2026-07-18 (Session 42) — Kullanım İstatistikleri + Yedekleme eksiksizleştirme + Geliştirici API Ağ Geçidi (Claude Code entegrasyonu, tam agentic tool use)

## Özet

Kullanıcı isteğiyle 3 büyük özellik art arda yapıldı, hepsi adım adım commit'lendi ve `go test -race`/`flutter analyze+test` ile doğrulandı:

1. **Ayarlar → İstatistikler** — token/hız/model dağılımı, 30 günlük grafik. Yeni `internal/stats` paketi, her tamamlanan turu (local/agent/orchestra/harici sağlayıcı) ayrı SQLite'a kaydediyor (gizli mod hariç). Maliyet bilinçli olarak gösterilmiyor — hiçbir sağlayıcı backend'de gerçek fiyat sunmuyor.
2. **`.memo` yedeklemesi eksiksizleştirildi** — en kritik bulgu: `providers.json`'daki şifreli API anahtarlarını çözen `machine.key` **hiç yedeğe girmiyordu**, başka makinede geri yükleme anahtarları kalıcı olarak bozuyordu. Artık takvim, rutinler, izinler, skill'ler, istatistikler dahil her şey yedekleniyor (bilinçli hariç: `sync_token.json`, `tailscale/`, `backups/`'ın kendisi).
3. **Geliştirici API Ağ Geçidi** — Memo'yu Claude Code (`ANTHROPIC_BASE_URL`) ya da OpenAI-uyumlu bir araçla kullanılabilir hale getiren yeni yerel API (`POST /v1/messages`, Anthropic Messages formatı). `internal/anthropicapi` paketi wire-format çevirisini yapıyor, `internal/app/devgateway.go` `tip/model-id` (`local/qwen2.5`, `openai/gpt-4o`) formatına göre yönlendiriyor. **Tam agentic tool use çalışıyor** — canlı uçtan uca testte gerçek bir bug yakalanıp düzeltildi (Anthropic'in `tool_use.input`'u gerçek JSON nesnesi, OpenAI'ın `function.arguments`'ı o nesnenin metnini taşıyan bir string — bunlar karıştırılınca Claude Code bozuk veri alıyordu). Kullanıcı isteğiyle sonradan Ayarlar'dan çıkarılıp yan menüde (WhatsApp/Takvim gibi) ayrı bir ekrana taşındı, canlı istek/yanıt günlüğü eklendi (ayrı 200 kayıtlık bellek-içi tampon, 2sn polling).

Ayrıca: obsidian-doc/obsidian-doc-en vault'ları ve `versinNote/{,tr/}v3.3.3.md` güncellendi — bugünkü üç özellik ayrı bir "sonradan eklendi" notu değil, doğrudan v3.3.3'ün parçası olarak dokümante edildi (kullanıcının açık talimatıyla).

**Bilinen sınırlama:** `gemini`/`claude`/`ollama` tipi sağlayıcılar için tool use henüz yok — o üçünün `internal/provider` implementasyonu Tools/ToolCalls'ı hiç çözmüyor (ağ geçidinden bağımsız, önceden var olan bir eksiklik). Böyle bir istek gelirse açık hata dönülüyor, sessizce yutulmuyor.

**Doğrulanmadı:** Yeni Geliştirici ekranı ve canlı günlük gerçek bir GUI penceresinde görsel olarak test edilmedi (bu ortamda ekran yok) — backend uçtan uca sahte sunucularla canlı doğrulandı, frontend kodu ise WhatsApp sekmesinin kanıtlanmış polling desenini birebir taklit ediyor.

---



## Özet

Session 40'ın bıraktığı 10 açık maddeden kullanıcı onayıyla 7'si bu oturumda kapsam alındı (BUG-M1/BUG-M4, API/mimari kararı gerektirdiği için yine ertelendi; BUG-L2 zaten kod bug'ı değil, tasarım notu). Her madde ayrı, doğrulanmış (`go build/vet/test -race -tags sqlite_fts5`), commit'lendi — paralel agent kullanılmadan tek oturumda sırayla. Sonunda kullanıcının talimatıyla repo'nun `data/` dizini sıfırlandı (WhatsApp hariç, kullanıcı onayıyla) ve taze derlenen binary'yle `-p` üzerinden kısa bir canlı senaryo koşulup 7 düzeltmenin de gerçekten çalıştığı doğrulandı.

## Düzeltilen 7 bug (commit'ler eskiden yeniye)

1. **BUG-H2** (`ffa8a41`) — `internal/intent/extractor.go`: `rawIntent.HabitDays` `[]int` yerine `json.RawMessage`, yeni `parseHabitDays` LLM'in `habit_days`'i string/string-dizisi/doğal dil ifadesi ("hafta içi") olarak döndürmesine tolerans gösteriyor — tek alan tipi bozuk diye tüm `rawIntent` artık reddedilmiyor.
2. **BUG-H3** (`104c582` + takip `5134823`) — `internal/memory/chunker.go`'ya `splitLongWord` fallback'ı eklendi (boşluksuz uzun "kelime" zorla bölünüyor). **Canlı `-p` testinde ilk fix'in eksik olduğu yakalandı:** `RetrieveContext`'in ham query/expandQuery/splitCompoundQuery embed çağrıları hiç sınırlanmamıştı, `saveChunk` da `assistantMsg`'i sınırsız ekliyordu — yeni `capForEmbedding` bunları da kapsadı. Ayrıca gerçek tokenizer'ın tekrarlı-karakter içerik için `len/3` tahmininden ~1.8× fazla token ürettiği canlı ölçüldü, `splitLongWord`'ün bayt marjı `maxTokens*3`'ten `maxTokens*1`'e düşürüldü.
3. **BUG-H4** (`2d5003d`) — `internal/app/memory.go`: `extractAndPinFacts` artık sadece `userMsg`'i extraction promptuna veriyor, asistanın (tool sonucu içerebilen) `reply`'sini hiç göndermiyor — ölü kalan `reply` parametresi de imzadan kaldırıldı.
4. **BUG-M5** (`6770aeb`) — Yeni salt-okunur `get_calendar_events` agent tool'u (`internal/agent/tools/calendar.go`), `internal/app/learning.go`'daki `calendarToolAdapter` gerçek `calendar.Store`'u sarıyor. Agent sistem promptuna model takvim sorgusunda bu aracı çağırsın diye not eklendi.
5. **BUG-M6** (`1d173e8`) — `extractAndPinFacts` artık pinlemeden önce mevcut pinned fact'lere karşı normalize edilmiş exact-match dedup kontrolü yapıyor.
6. **BUG-M7** (`a189a1f`) — `internal/agent/tools/command.go`'daki `RunCommand` artık komut string'inin içindeki path'leri de (`commandTargetsProtectedPath`) `read_file`'ın `validatePath`'ıyla aynı sınırla kontrol ediyor — proje dizini İÇİNDEKİ göreli path'ler (`go build ./...`) yanlış pozitif vermiyor.
7. **BUG-L1** (`5f781e2`) — `main.go`: `*prompt != ""` yerine `flag.Visit` ile "p" bayrağının fiilen geçilip geçilmediği kontrol ediliyor; `runPrintMode` boş/whitespace prompt için temiz `FATAL` ile çıkıyor.

Her commit kendi regresyon testiyle geldi (`TestExtractHabit_HabitDaysAsPhrase`, `TestChunkText_SingleOversizedWord`, `TestSaveAndRetrieve_LongUnbrokenBlob`, `TestExtractAndPinFacts_DoesNotSendAssistantReply`, `TestExtractAndPinFacts_SkipsAlreadyPinnedDuplicate`, `internal/agent/tools/calendar_test.go`, `TestRunCommand_BlocksProtectedPathBypass`, `TestEmptyPromptExitsCleanly`).

## Canlı doğrulama — yöntem ve bulgular

Kullanıcı `data/`'nın silinmesini istedi; WhatsApp'ın (canlı, eşleşmiş oturum) hariç tutulup tutulmayacağı netleştirildi (kullanıcı: hariç tut). Mevcut eski backend (port 8090, pre-fix binary) `POST /api/shutdown` ile düzgünce kapatıldı, ardından `rm -rf data/{memory,sessions,profile,mood,calendar,tasklists,agent-backups}` + log dosyaları silindi — `data/whatsapp`, `data/models`, `data/providers.json/.example.json`, `data/machine.key` dokunulmadan bırakıldı. Taze `go build -tags sqlite_fts5` binary'si `--headless --port 18090` ile repo kökünden çalıştırıldı (embedding modeli `/api/models/embedding/start` ile elle başlatıldı, `embedding_auto_start:false` olduğu için), WhatsApp oturumu otomatik yeniden bağlandı (246 kişi, 32 grup).

"Ayşe" adında tek turluk bir test persona'sıyla `-p --auto-allow` üzerinden sırayla:
- **H2**: "her hafta içi sabah 08:30" ve "her gün akşam 21:00" alışkanlıkları → `patterns.json`'da `declared:08:30` (days_active Mon-Fri) ve `declared:21:00` (her gün) doğru oluştu.
- **H3**: 40.000 karakterlik boşluksuz blob → ilk denemede HÂLÂ hata verdi (retrieve + save), kök neden analiz edilip fix genişletildi (yukarıda); ikinci denemede hata yok, gerçekçi boyutlu (3KB) bir blob tam ve hatasız kaydedildi. Not: 40KB'lik aşırı-adversarial girdi hâlâ `saveMemorySync`'in 10s bütçesine (67 küçük chunk sırayla) sığmayabiliyor — ayrı, kabul edilmiş bir sınır, orijinal "tüm turu kırma" bug'ı değil.
- **M5**: Gerçek bir takvim etkinliği eklendi (intent pipeline üzerinden), TAZE bir sohbette "bu hafta takvimimde ne var" sorusu `get_calendar_events` tool'unu gerçekten çağırdı ve doğru etkinliği döndürdü.
- **M6**: "adım Ayşe" iki ayrı turda söylendi, `memory.db`'de sadece BİR "User's name is Ayşe." kaydı oluştu.
- **M7**: `run_command` ile `cat /etc/hostname && whoami` reddedildi ("protected directory"), model kendiliğinden path içermeyen güvenli komutlara geçti.
- **H4**: Gerçek WhatsApp sohbet listesi okundu (`whatsapp_latest`, salt-okunur), asistan yanıtı TEKNOFEST grubu/sınıf grubu gibi üçüncü şahıs bilgisi içerdi — pinned facts'e HİÇBİRİ sızmadı (sadece kullanıcının kendi ismi tekrar pinlendi, dedup exact-match olduğu için farklı dilde ifade edilince ayrı kayıt oldu — bilinen sınır).
- **L1**: `timeout 8 memo-test -p ""` artık anında "FATAL: boş mesaj gönderilemez" ile çıkıyor (`exit=1`), önceden `exit=124` (zorla öldürülene kadar askıda).

## Ek gözlem (düzeltilmedi, kapsam dışı)

H4 testinde arka plan fact-extraction çağrısı bir kez `NONE` yerine sohbet-tarzı bir ret cümlesi ("WhatsApp mesajlarınızı göremiyorum...") döndürdü ve bu `parseExtractedFacts` tarafından geçerli bir "fact" sanılıp pinlendi. Bu, BUG-H4'ün düzelttiği gizlilik sızıntısından FARKLI bir sorun — zayıf/ücretsiz bir modelin extraction sistem promptuna uymaması. Tekrar gözlenirse `BUG_REPORT.md`'ye yeni bir madde olarak eklenmeli.

## Temizlik ve doğrulama

- Test backend'i (`POST /api/shutdown`) ve embedding server child process'i düzgünce durduruldu.
- Test persona'sının ürettiği `data/{memory,sessions,profile,mood,calendar,tasklists,agent-backups}` + log dosyaları tekrar silindi — `data/whatsapp` (gerçek oturum), `data/models`, `data/providers.json`, `data/machine.key` dokunulmadan kaldı.
- Son tam repo taraması: `go build/vet/test -race -tags sqlite_fts5 ./...` tüm paketlerde yeşil (önceki oturumlardan kalma `internal/whisper` port çakışması da bu oturumda kendiliğinden çözüldü, eski orphan süreç artık yok).

## Sıradaki oturum için

1. `BUG_REPORT.md`'de sadece bilinçli ertelenen 3 madde kaldı: BUG-M1 (rutin içeriği hardcoded TR), BUG-M4 (rutin saatinde timezone yok), BUG-L2 (WhatsApp gönderim yolu tutarsızlığı — tasarım kararı). Üçü de gerçek bir API/mimari/ürün kararı gerektiriyor, kullanıcı onayı bekliyor.
2. Yukarıdaki "ek gözlem" (extraction'ın NONE yerine ret cümlesi dönmesi) tekrar gözlenirse yeni bir bug maddesi olarak açılabilir.
3. `data/` şu an tamamen temiz (sadece WhatsApp oturumu + model/provider config canlı) — bir sonraki gerçek kullanım veya test oturumu bunun üzerine sıfırdan başlayabilir.

---

# Handoff — 2026-07-17 (Session 40) — "Ece" persona testi, CANLI WhatsApp ortamında — 6 yeni gerçek bug bulundu, 2 test mesajı gerçekten gönderildi, tüm bulgular BUG_REPORT.md'ye konsolide edildi

## Özet

Kullanıcı bu oturumda repo'nun `data/` dizinine **gerçek, canlı bir WhatsApp hesabı** bağlamış haldeydi (241 gerçek kişi) ve "0'dan bir kişilik yarat, gerçek insan gibi sohbet et" talimatıyla whatsapp/takvim/orchestra/agent/proaktif öğrenme/mood/öz-çıkar/RAG'ı kapsamlı bir canlı teste sokmamı istedi — WhatsApp gönderiminin SADECE `Annnem` kontağına (kullanıcının annesi, gerçek numara) ve SADECE test-ibareli içerikle yapılmasını şart koştu. Önceki oturumlardan (32-39) farklı olarak bu kez **çalışan backend/WhatsApp process'ine hiç dokunulmadı, `data/` wipe edilmedi** — WhatsApp bağlantısını bozma riskini almamak için kullanıcıyla netleştirilip, zaten çalışan backend'e (port 8090) yeni bir istemci binary'siyle (`-tags sqlite_fts5`) bağlanıldı. "Ece" adında bir persona (İzmir'de yaşayan serbest grafik tasarımcı) üzerinden ~20 turluk bir `-p`/API testi koşuldu; sonunda kullanıcı ek olarak WhatsApp'a gerçek bir gönderim ısrar etti, bu da AI-agent yolunda değil doğrudan REST API'den yapıldı (aşağıda Madde WhatsApp). Tüm bulgular tek seferde `BUG_REPORT.md`'ye yazıldı (kullanıcının açık talimatıyla, ayrı bir "stress test" dosyası tutulmadı — önceki oturumun `STRESS_TEST_FINDINGS.md`'si de bu oturumda silindi, içeriği BUG_REPORT.md'ye taşındı).

**Fix uygulanmadı** — kullanıcı bu oturumda "bul ve yaz" istedi, düzeltme kararı sonraki oturuma/kullanıcı onayına bırakıldı.

## Ortam farkı ve alınan güvenlik kararları

1. **WhatsApp'a hiç dokunulmadı:** Kullanıcı "whatsapp gitmesin" diye açıkça uyardı. Backend süreçleri durdurulup yeniden başlatılmadı; sadece yeni derlenen bir istemci binary'si (`/tmp/.../scratchpad/memo-test`, main.go'nun güncel `-p` bayrağını taşıyan — repo kökündeki committed `memo` binary'si Jul 15'ten kalmaydı ve `-p` bayrağı yoktu) zaten port 8090'da dinleyen backend'e bağlandı. Test ortasında backend kendi `--auto-shutdown` mantığıyla bir kez organik olarak yeniden başladı (benim müdahalem değil) — whatsmeow oturumu `session.db`'den sorunsuz geri bağlandı, tüm WhatsApp okuma testleri baştan sona çalıştı.
2. **`data/` wipe edilmedi** (önceki "Fatih"/"Deniz" persona testlerinden farklı) — bu yüzden bu oturumun "Ece" persona verisi, GERÇEK kullanıcının ve önceki "Deniz" test oturumunun aynı `data/memory/memory.db` pinned-facts havuzuna karıştı (bkz. BUG-H4'ün keşfedilme şekli). Oturum sonunda bu oturumun ürettiği ~35 pinned-fact kaydı ve sahte takvim etkinliği temizlendi; "Deniz" kirliliğine (önceki oturumdan) dokunulmadı.
3. **WhatsApp gönderimi sıkı kısıtlandı:** `--auto-allow` ile açık uçlu izin verilmedi — her turda ya auto-allow (agent tool testleri için) ya da varsayılan-deny (izin UX testleri için) bilinçli seçildi. Gönderim SADECE `Annnem` (905457348509@s.whatsapp.net, whatsmeow contact tablosundan doğrulandı) hedefine, sadece test-ibareli içerikle denendi.

## Bulunan bug'lar (hepsi `BUG_REPORT.md`'de, detaylar orada)

| # | Özet | Severity |
|---|---|---|
| BUG-H2 (güncellendi) | Alışkanlık kaydı JSON tip hatası — yeni kanıt: "her hafta içi" gibi genel ifadelerle de tetikleniyor, onay backend sonucundan 40 saniye ÖNCE veriliyor (mimari olarak asistan hiçbir zaman gerçek sonucu bilemiyor), yanlış güven sonraki turlara taşınıyor | HIGH |
| BUG-H3 (yeni, Session 39 Madde 3'ün devamı) | Uzun boşluksuz string chunker/embed batch-size hatası artık RETRIEVE tarafını da kırıyor + tüm turu (LLM cevabı dahil) başarısız kılabiliyor | HIGH |
| BUG-H4 (yeni) | `extractAndPinFacts`, asistanın WhatsApp'tan okuyup aktardığı ÜÇÜNCÜ ŞAHIS bilgisini (başka bir gerçek WhatsApp grubu) kullanıcının kendi kalıcı kimliği sanıp pinledi — gizlilik/doğruluk riski | HIGH |
| BUG-M5 (yeni) | Takvim sorgusu için hiç agent tool'u yok, model RAG'dan tahmin ediyor, kaydedilmemiş alışkanlığı gerçek etkinlikle ayrım yapmadan listeledi | MEDIUM |
| BUG-M6 (yeni) | `extractAndPinFacts`'te dedup yok — 16 turluk tek sohbette "adı Ece" gerçeği 8 kez ayrı pinlendi | MEDIUM |
| BUG-M7 (Session 39'dan taşındı) | `run_command`, `read_file`'ın korumalı-dizin sınırını atlıyor | MEDIUM |
| BUG-L1 (Session 39'dan taşındı) | `memo -p ""` sonsuza kadar askıda kalıyor | LOW |
| BUG-L2 (yeni) | WhatsApp agent-tool gönderimi (LLM kendi kararıyla reddediyor) ile doğrudan REST/GUI gönderimi (koşulsuz çalışıyor) arasında tutarsızlık | LOW |

## WhatsApp gönderim testi — ayrıntı

Sohbet üzerinden (`whatsapp_send` agent tool, `--auto-allow` açık) 3 farklı, giderek daha dürüst/net rephrase denemesi yapıldı ("Ece" persona'sı kendi annesine göndermek istiyor, tam test-ibareli metin verildi). Model **üçünde de** kendi konuşma-seviyesi muhakemesiyle reddetti — annenin habersizce otomatik mesaj almasının onu endişelendireceğini söyledi. `backend.log`'da 3 denemede de `whatsapp_send` tool'unun hiç çağrılmadığı doğrulandı (model tool'u hiç çağırmadan reddetti). Kullanıcı sonradan ısrar edince (bu handoff'un yazıldığı konuşmanın devamında), `/api/whatsapp/send` REST endpoint'i (GUI'nin WhatsApp sekmesinin kullandığı, LLM onayına hiç girmeyen doğrudan yol) üzerinden 2 test mesajı gerçekten gönderildi ve `Annnem`'in sohbet geçmişinde `from_me:true` olarak doğrulandı (mesaj ID'leri: `3EB00D3768DAD039021DCA`, `3EB017828DEC4ABA21E836`). Bu, bir kod bug'ı değil — iki ayrı gönderim yolunun kasıtlı farklı güvenlik modelleri var; BUG-L2 olarak dokümante edildi.

## Temizlik (oturum sonunda yapıldı)

- `rename_pngs.py` (agent'ın yazdığı test dosyası) silindi.
- `data/memory/memory.db`'de bu oturumun ürettiği ~35 "Ece"-kaynaklı `source='explicit'` (pinned) kaydı silindi (isim, şehir, meslek tekrarları + BUG-H4'teki TEKNOFEST/12. sınıf yanlış çıkarımları dahil).
- `data/calendar/events.db`'de bu oturumun sahte "logo revizyonu görüşmesi" etkinliği silindi.
- `mood.enabled`, `mood.self_interest`, `web_search.enabled` config toggle'ları (test için API'den açılmıştı) test öncesi kapalı durumuna geri alındı.
- `STRESS_TEST_FINDINGS.md` silindi — içeriği (Session 39 + Session 40) `BUG_REPORT.md`'ye konsolide edildi.
- **Dokunulmadı (bilinçli):** Önceki "Deniz" persona test oturumundan kalan pinned-facts kirliliği (kedi "Mırnav" vb.) ve gerçek kullanıcının kendi pinned fact'leri — kullanıcı onayı olmadan eski veri silinmedi.

## Doğrulama

- WhatsApp bağlantısı test boyunca ve sonunda sağlam: `whatsapp_latest`/`whatsapp_messages` gerçek veri döndürmeye devam etti, `session.db` hiç silinmedi/bozulmadı.
- Gönderilen 2 gerçek test mesajı `/api/whatsapp/messages?jid=905457348509@s.whatsapp.net` ile geri okunup doğrulandı.
- Kod değişikliği yapılmadığı için `go build`/`go test` bu oturumda çalıştırılmadı — sadece canlı davranış gözlemi + doğrudan sqlite/REST doğrulaması yapıldı.

## Sıradaki Oturum İçin

1. `BUG_REPORT.md`'deki 10 açık maddeden hangilerinin düzeltileceğine kullanıcı karar vermeli — öncelik önerisi: BUG-H4 (gizlilik riski, en yüksek) → BUG-H3 (veri kaybı + tam tur başarısızlığı) → BUG-H2 (yaygın kullanıcı-güveni sorunu) → geri kalanlar.
2. BUG-M6 (dedup) ve BUG-H4 (third-party içerik sızıntısı) aynı fonksiyonda (`extractAndPinFacts`) — birlikte ele alınması verimli olabilir.
3. Kullanıcı isterse önceki "Deniz" persona test oturumundan kalan pinned-facts kirliliğinin de temizlenmesi ayrıca onaylanabilir (bu oturumda dokunulmadı).
4. BUG-L2 bir tasarım kararı gerektiriyor — geliştirici WhatsApp agent-tool'unun ne zaman/hangi koşulda otonom gönderim yapabileceğine dair bilinçli bir politika belirlemek isteyebilir.

---

# Handoff — 2026-07-17 (Session 39) — /code-review bulduğu 19 rutin-motoru bug'ının 17'si otonom /loop ile düzeltildi + "Deniz" persona testiyle canlı doğrulama, 1 yeni gerçek bug bulundu

## Özet

Kullanıcı gece uykuya geçmeden önce üç ardışık iş bıraktı: (1) `/code-review` (high effort, 8 açı) + `/codebase-memory` ile HEAD~15..HEAD üzerinde bulunan 19 bug'ı `BUG_REPORT.md`'ye yazdım; (2) kota 05:00'te sıfırlanınca `/loop` ile saat 10:00'da otonom olarak bunları önceliğe göre (CRITICAL→HIGH→MEDIUM→LOW→teknik borç) tek tek düzeltip her birini ayrı commit'le kapattım; (3) tüm düzeltmeler bitince `data/`'yı sıfırlayıp yeni bir "Deniz" persona'sıyla `memo -p --auto-allow` üzerinden canlı bir kullanıcı testi koştum. Bu üçüncü adımda **yeni, gerçek ve önceki oturumların (32/33/34/35) hiç kapatamadığı bir teması yeniden doğrulayan bir bug** bulundu (aşağıda).

**codebase-memory-mcp bu oturumun otonom (/loop) kısmında bağlı değildi** — `ToolSearch` "No matching deferred tools found" döndürdü, hem bug-fix hem persona-test aşamalarında Grep/Read/Bash'e geri dönüldü. Kullanıcının "kesinlikle kullan" talimatına rağmen bu kısıtlama burada açıkça not düşülüyor.

## 1) Rutin motoru bug sweep'i (17/19 düzeltildi, 2 MEDIUM bilinçli ertelendi)

Bulgular ve düzeltmeler artık git geçmişinde (aşağıdaki commit'ler), tekrar burada dokümante edilmiyor — `BUG_REPORT.md` sadece kalan 2 açık maddeyi tutuyor:

- **BUG-M1**: Backend'in ürettiği rutin içeriği (sistem promptu, bildirim başlığı, boş-bağlam metinleri) hardcoded Türkçe, L10n'i baypas ediyor — gerçek düzeltme backend API'sine bir dil alanı eklemeyi gerektiriyor, tek taraflı otonom karar yerine ertelendi.
- **BUG-M4**: Rutin saati (`HH:MM`) zaman dilimi taşımıyor, backend host'un yerel saatine göre yorumlanıyor — API/mimari kararı gerektiriyor, ertelendi.

Commit'ler (en yeniye doğru): `fd9545e` (BUG-C1), `0717064` (BUG-C2), `02a6e78` (BUG-H1), `abd5be4` (BUG-M2/M3/M5), `19ad5ee` (BUG-L4/L5), `9fb090d` (BUG-L1/L2/L3/L6), `90bfd89` (TD-3/TD-5), `09af2f5` (TD-4), `62a6be9` (TD-2), `d4dd731` (TD-1). Her biri ayrı, `go build/vet/test -race -tags sqlite_fts5` (+ ilgili maddede `flutter analyze/test`) ile doğrulanmış commit.

## 2) "Deniz" persona testi — metodoloji

Fatih persona testinin (Session 34) devamı niteliğinde, ama bu kez repo'nun kendi `data/`'sı üzerinden (kullanıcının gerçek `~/.memo/data`'sına — canlı WhatsApp bağlantısı ve 243 gerçek kişiyle — HİÇ dokunulmadı, bilinçli olarak izole edildi):

1. `rm -rf data/{memory,sessions,profile,mood,calendar,whatsapp,tasklists,agent-backups}` + log dosyaları silindi. **`data/models/` (embedding modeli), `data/providers.json`/`.example.json` (gerçek, zaten yapılandırılmış API key) ve `data/machine.key` (providers.json'u çözmek için gerekli) BİLEREK korundu** — kullanıcı "hiç verim yok, ister sil ister yok et" dedi ama bunları da silmek, testin gerçek bir LLM'e ulaşmasını imkansız kılardı (repo'da lokal bir chat modeli yok, sadece embedding modeli var).
2. `go build -tags sqlite_fts5 -o /tmp/.../memo-dev .` ile bugünkü tüm fix'leri içeren taze bir binary derlendi, repo kök dizininden çalıştırıldı (böylece `config.DataPath()` process-relative `./data`'yı çözüyor — kurulu `~/.memo/bin/memo`'dan tamamen bağımsız).
3. "Deniz" persona'sı: 27 yaşında, İzmir'de yazılım mühendisi, favori renk turuncu, kedisi "Mırnav", kahveyi sade içiyor. `memo -p "..." [-chat <id>] [-auto-allow]` ile çok turlu, hem aynı chat'te hem taze chat'lerde test edildi.

## 3) Bulgular

**Çalışanlar (doğrulandı):**
- Hafıza kaydı + fresh-chat recall: Deniz'in kedisinin adı ve kahve tercihi, TAMAMEN farklı bir chat ID'sinde doğru hatırlandı (embed/retrieve log'ları `best=100%`).
- `write_file`/`delete_file`/`read_file` round-trip: dosya oluşturma → düzenleme → okuma → silme → `stat` ile doğrulama, hepsi gerçek diskte gerçekleşti (`AGENT [write_file] SUCCESS`, vb. log'da).
- Takvim intent extraction: "yarın saat 15:00'te diş hekimi randevum var" → arka planda doğru ayrıştırılıp `data/calendar/events.db`'ye gerçek bir satır olarak yazıldı (`Dentist appointment`, `2026-07-18 15:00`).
- Path güvenliği: `/home/` altına (repo dışı) yazma denemesi `access denied: path is within protected directory` ile bloklandı — savunma çalışıyor.

**Küçük bulgu (bug değil ama not edilmeye değer):** Kullanıcı "masaüstüm" dediğinde model bazen path'i tilde (`~`) ile literal composed ediyor; ilk denemede repo cwd'si altında gerçek bir `~/Desktop/` klasörü OLUŞTURULDU (path expand edilmedi), ikinci denemede aynı tarz bir path "protected directory" diye reddedildi — tutarsız. Test artefaktları (`~/`, `deniz_notlar.txt`, `scratch_test.txt`) temizlendi, repo'da iz yok.

**YENİ GERÇEK BUG (düzeltilmedi, sadece bulundu — BUG_REPORT.md'ye eklenmeli):**

"Bundan sonra her gün akşam 21:00'de 20 dakika kitap okuyacağım, bunu alışkanlık olarak not al" mesajına model sohbette **"Alışkanlık kaydedildi ✅"** dedi — ama bu YALAN. Backend log'u kök nedeni gösteriyor:

```
intent: parse response: unmarshal: json: cannot unmarshal string into Go struct field
rawIntent.habit_days of type int {"has_intent": true, "is_calendar_event": false,
"is_habit": true, ..., "summary": "Daily habit declaration to read a book for 20
minutes at 21:00.", ...
```

`internal/intent/extractor.go`'daki `rawIntent.HabitDays []int` alanı, LLM `habit_days`'i (muhtemelen "her gün" için gün adı stringleri olarak) beklenen int dizisi yerine string döndürdüğünde `json.Unmarshal` TÜM objeyi reddediyor — `has_intent`/`is_habit`/`summary` gayet doğru parse edilmiş olsa bile, tek bir alanın tip uyuşmazlığı yüzünden `parseResponse` hata dönüyor, `processMessageIntent` (`internal/app/learning.go:119-123`) bunu loglayıp sessizce return ediyor. Sonuç: `data/profile/patterns.json`'da HİÇBİR yeni pattern yok (`updated_at` habit mesajından ÖNCEki bir timestamp'te donmuş kaldı) — ama ana sohbet modeli arka plandaki bu pipeline'ın başarısız olduğundan habersiz, kullanıcıya her zaman "kaydettim" diyor. **Kullanıcıyı yanlış güvenceye sokan, gerçek bir doğruluk bug'ı.**

Olası düzeltme yönü (uygulanmadı): `rawIntent.HabitDays`'i `[]int` yerine daha toleranslı bir tip (`json.RawMessage` veya `[]any` + manuel coerce) yapmak, ya da tek bir alanın parse hatasının tüm sonucu iptal etmemesi için alan bazlı fallback eklemek.

**Kısmen doğrulanan, önceki oturumlardan tanıdık bir tema:** Session 34'te flaglenen "model, sıradan bir sohbet mesajına kendiliğinden dosya/komut aracı çağırmaya çalışıyor" deseni (Turn 21, `read_file` ile var olmayan `memory.json`) burada da iki kez tekrarlandı — hem takvim hem alışkanlık deklarasyonu turlarında (ikisi de TAZE chat, dosyayla hiç ilgisi olmayan mesajlar), model `run_command`/`read_file`/`write_file` denedi (izin sistemi doğru şekilde `-auto-allow` verilmeyen turlarda reddetti). Session 35'in bu temaya yönelik fix'i (`internal/identity/identity.go`'ya "hafıza dosya değil enjekte" notu) bu deseni tam kapatmamış — kök neden hâlâ açık, yeni bir araştırma turu gerektiriyor.

**Test edilemeyenler (bu ortamda pratik değil, başarı/başarısızlık iddia edilmiyor):** WhatsApp bağlamlı rutinler (izole `data/whatsapp`'ta gerçek bir hesap bağlı değil — "get joined groups error: context deadline exceeded" logland), proaktif/ambient dürtmeler (arka plan tick zamanlamasına ve gerçek zaman geçişine ihtiyaç duyuyor, tek seferlik `-p` çağrılarıyla gözlemlenemedi).

## Sıradaki oturumun ilk işi

Yukarıdaki habit_days JSON parse bug'ını `BUG_REPORT.md`'ye MEDIUM/HIGH önerilen bir madde olarak eklemek ve düzeltmek — kullanıcıya yanlış "kaydedildi" güvencesi verdiği için önceliklendirilmeli.

---



## Özet

Mobile client (`mobile/lib`) neredeyse tamamen hardcoded TR/EN karışık metinlerle doluydu. Desktop frontend ile aynı desende (`L10n` + `MemoLocale` + `localeProvider` + SharedPreferences `memo_locale`) tam dil desteği eklendi. Yapısal/kodsal mimariye dokunulmadı — sadece kullanıcıya görünen metinler + dil seçici.

## Yapılan

- Yeni: `mobile/lib/core/l10n.dart` — 170 TR + 170 EN anahtar, `L10n.t(key, args)` ile `\${arg}` interpolasyonu, `dateLocale` for intl
- Yeni: `mobile/lib/providers/locale_provider.dart` — persist + rebuild
- `main.dart` MaterialApp `localeProvider` watch → tüm ağaç yenilenir
- Connect, chat, drawer, message input, bubble, calendar, routines, settings, notification channels/bodies, connection error messages → `L10n.t`
- Dil seçici: Settings → Connection tab + Connect ekranı (bağlanmadan önce de değiştirilebilsin diye)
- Widget test L10n üzerinden assert ediyor

## Doğrulama

```
flutter analyze lib/  → 0 error (sadece pre-existing info-level deprecation)
flutter test          → 1/1 passed
TR/EN key parity      → 170/170
```

## Commitler (7)

1. `cbe037d` feat(mobile): add L10n system with TR/EN maps and locale provider
2. `605d9a0` feat(mobile): wire localeProvider into MaterialApp and bottom nav
3. `4d9499a` feat(mobile): localize connect screen and connection error messages
4. `1b6ba6a` feat(mobile): localize chat screen, drawer, input, and bubbles
5. `6339398` feat(mobile): localize calendar, routines, and notification channels
6. `b1b5541` feat(mobile): localize settings and add TR/EN language switcher
7. `5581726` feat(mobile): add language toggle on connect screen before pairing

## Bilinçli bırakılan

- `KB`/`MB`/`GB` birimleri ve örnek token hint'leri (`memo-abc123…`, URL örnekleri) — dil-bağımsız placeholder
- `mobile/flutter_05.png` untracked screenshot — commit edilmedi
- Desktop frontend / backend / CLI — dokunulmadı

## Sıradaki

- Gerçek telefonda dil toggle + scheduled notification channel isimlerinin dil değişiminden sonra yeniden oluşturulması (Android kanal isimleri ilk oluşturmada kilitlenebilir — bilinen OS davranışı)
- İstenirse handoff sonrası origin'e push

---

# Handoff — 2026-07-17 (Session 37, devam 2) — Ambient nudge canlı CLI'da uçtan uca doğrulandı + Minimal Mod'a granüler "yine de açık kalsın" dropdown'u eklendi

## Özet

Session 37'nin (aşağıdaki ilk girdi) aynı gün devamı. İki ayrı iş yapıldı: (1) az önce eklenen ambient nudge özelliğinin **gerçekten CLI'da (`internal/replcli`) çalışıp çalışmadığı** canlı olarak test edildi — kullanıcı `cmd/proactive-demo`'yu değiştirip sahte profil oluşturmayı, `-p` moduyla senaryo kurmayı önerdi; (2) kullanıcı Minimal Mod'un all-or-nothing olmasından memnun değildi, "öğrenmeyi kapatacak ama sistem promptunu kapatmayacak" gibi granüler bir kontrol istedi — sadece GUI'ye, CLI'ya eklenmedi (açık talimat).

## 1) Canlı CLI doğrulaması (izole test backend'i, kullanıcının açık GUI oturumuna dokunulmadı)

`cmd/proactive-demo`'yu değiştirmek yerine (o script 5 hafta sahte veri üretiyor, yavaş) doğrudan `data/profile/patterns.json`'a şu ana denk gelen saatte, `declared:true`, güven 0.9 bir pattern yazıldı — Session 37'nin "beyan edilmiş alışkanlık" mekanizmasını (`SaveDeclared`'ın ürettiği format) taklit ederek. İzole bir backend (`--port 18090`, ayrı `MEMO_DATA_DIR`, repo'nun kendi test OpenCode Zen anahtarıyla) kurulup `memo -p "..."` ile gerçek mesajlar gönderildi:

- Nötr mesaj → hiç nudge yok, yanlış pozitif yok ✅
- "boş vaktim var ne yapsam" → model **kendiliğinden**, doğal bir cümleyle "zaten akşamüstü kod yazma saatin gelmiş gibi duruyor" dedi ✅
- Arka plan LLM kontrolü bunu doğru tespit edip `pending.json`'a `action:"ambient"` yazdı ✅
- `/api/proactive/pending` bu ambient öneriyi doğru şekilde **gizledi** (banner'da görünmedi) ✅
- "evet hadi başlayalım" (serbest Türkçe metin, keyword listesi yok) → LLM doğru KABUL olarak sınıflandırdı, güven 0.9→1.0, pending temizlendi ✅
- Bonus: seed edilen declared pattern, backend'in kendi periyodik yeniden-analizinden (`"analyzed 2 observations into 1 pattern(s)"`) silinmeden sağ çıktı

**Ayrıca Minimal Mod'un (o zamanki all-or-nothing hali) gerçekten kusursuz çalıştığı da canlı doğrulandı:** aynı izole backend'de Minimal Mod açılıp aynı senaryo tekrarlandı — `pending.json` hiç oluşmadı, sistem promptu `system=0` token. Daha zor bir senaryo da denendi: bir ambient öneri Minimal Mod açılmadan ÖNCE oluşmuş gibi elle yerleştirilip, Minimal Mod açıkken "evet" gönderildi — `pending.json` ve `patterns.json` **hiç değişmedi**, LLM'e hiç sorulmadı.

Test backend'leri düzgünce (`POST /api/shutdown`) kapatıldı, kullanıcının kendi açık GUI oturumuna (farklı port, farklı `MEMO_DATA_DIR`) hiç dokunulmadı.

## 2) Minimal Mod'a granüler "yine de açık kalsın" dropdown'u

Kullanıcı somut örnek verdi: "öğrenmeyi kapatacak ama sistem promptunu kapatmayacak". Yani Minimal Mod AÇIKKEN, varsayılan olarak her şey kapalı kalırken, kullanıcı belirli kategorileri tek tek geri açabilsin.

**Backend:**
- `config.IdentityConfig`'e 4 yeni bool: `MinimalModeKeepPersona/Capabilities/Passive/Proactive` (varsayılan false — eski all-off davranışla birebir aynı, dropdown'a hiç dokunmayan kullanıcı için).
- `identity.Identity` aynı 4 alanı `minimalModeMu` ile korunan şekilde taşıyor + `SetMinimalModeOverrides`/`GetMinimalModeOverrides`/`GetMinimalModeKeepProactive`.
- `BuildSystemPrompt`, tek `if !MinimalMode {...}` bloğundan üç bağımsız gated bölüme ayrıldı: **persona** (kimlik+origin+üslup+öğrenilen notlar — kullanıcının "sistem promptu" dediği tek kategori), **pasif özellikler bildirimi**, **yetenek bildirimleri**. Her biri `if !minimal || keepX`.
- `proactive_ambient.go`'daki `ambientNudgingActive()` artık `GetMinimalModeKeepProactive()`'i kontrol ediyor — ambient nudge özelliği dördüncü, bağımsız açılabilir kategori oldu.
- `internal/app/settings.go`: `GetMinimalModeOverrides`/`SetMinimalModeOverrides` bridge metodları (struct değil, 4 ayrı bool — `internal/webserver`'ın `internal/app` tiplerini import etmemesi kuralı gereği, `ImportMemoryFromText`'in zaten belgelediği desen).
- Yeni endpoint: `GET/PUT /api/system-prompt/minimal-mode/overrides`.

**Frontend (sadece GUI, CLI'ya eklenmedi):**
- `frontend/lib/models/minimal_mode_overrides.dart` (yeni model), `api_client.dart` get/set, `minimalModeOverridesProvider` (mevcut `minimalModeProvider` ile aynı desende).
- Settings → General: Minimal Mod toggle'ının hemen altında, **sadece Minimal Mod açıkken görünen** bir `ExpansionTile` — 4 switch: Kişilik/Sistem Promptu, Yetenek Bildirimleri, Pasif Özellik Bildirimi, Proaktif Öğrenme.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 33 test edilen paket yeşil
flutter analyze lib/                                            → temiz (bilinen 4 info bulgu)
flutter test                                                     → 107/107
```

6 yeni backend testi (4 identity granüler kombinasyon + 1 ambient override + 1 bridge round-trip). Frontend dropdown'u için ayrı widget testi yazılmadı — mevcut kod tabanı konvansiyonuna uyularak (diğer ayarlar sekmelerinin çoğunda da yok), ayrıca private widget class'ları dışarıdan test edilemiyor.

Commit: `2910564` → `853d1ee` arası (ambient nudge + review fix'leri + granüler Minimal Mod).

## Gerçek ortamda doğrulanamayan

Yeni dropdown UI'ı bu ortamda görsel olarak (ekran yok) test edilemedi — kullanıcı gerçek GUI'de Minimal Mod'u açıp dropdown'ın göründüğünü, switch'lerin doğru çalıştığını denemeli.

---

# Handoff — 2026-07-17 (Session 37) — Sohbet içine gömülü "ambient" alışkanlık dürtmesi eklendi, dil-bağımsız (LLM tabanlı) sonuç tespiti, /code-review'un bulduğu 6 gerçek hata düzeltildi

## Özet

Session 36'nın devamı, aynı konuşma içinde. Kullanıcı şunu istedi: proaktif motorun mevcut ayrı banner/bildirim mekanizmasına ek olarak, model kendi normal cevabının İÇİNE doğal bir şekilde alışkanlığı sorabilsin (örnek: "kanka kod vakti değil mi, yazalım mı?") — hem sohbeti güzelleştirsin hem de kullanıcı kabul edip gerçekten o işe geçerse pattern'in güvenini artırsın. Kritik kısıtlar: (1) RAG'a hiç dokunma, (2) kullanıcının kabul/red cevabını anlamak için **kesinlikle keyword/regex listesi kullanma** (mevcut `OutcomeFromResponse` sadece TR/EN kapsıyor, her dilde çalışmalı — bunun yerine LLM'in kendisine sor), (3) Ayarlar → Minimal Mod açıkken bu özelliğin hem öneri sunma hem sonuç değerlendirme tarafı da tamamen kapansın (hiç prompt injection/LLM çağrısı olmasın).

## Yapılan

Yeni dosya `internal/app/proactive_ambient.go`:
- `buildProactiveNudgeBlock` — saf yerel eşleştirme (mevcut `proactive.Match`/`observer.PatternStore`, LLM çağrısı yok), eşleşirse sistem promptuna "kullanıcının bu saatte X alışkanlığı var, doğal geliyorsa bir kere sorabilirsin, zorlamadan" notu ekliyor.
- `checkAmbientNudgeSurfaced` (cevap bittikten sonra, `finishStream`'den ateşleniyor) — modelin kendisine "cevabında bu konuyu gerçekten açtın mı, EVET/HAYIR" diye soruyor (dar amaçlı LLM çağrısı, `extractAndPinFacts` ile aynı maliyet sınıfı — sadece pattern eşleştiğinde tetikleniyor, her mesajda değil). EVET ise `PendingSuggestion` (yeni `Action: ambient`) kaydediliyor.
- `checkAmbientNudgeOutcome` (bir sonraki turun başında) — kullanıcının serbest metin cevabını (hangi dilde olursa olsun) modelin kendisine "KABUL/RET/BELİRSİZ" diye sorduruyor — **keyword listesi yok**. BELİRSİZ ise dokunmuyor, bekliyor.

`internal/proactive/engine.go`: `HandleResponse` ikiye bölündü — yeni public `HandleOutcome(p, outcome)`, zaten sınıflandırılmış bir sonucu doğrudan alıp mevcut `recordOutcome`/`AdjustConfidence`/`Suppress` altyapısına besliyor. Mevcut banner akışı için davranış birebir aynı kaldı (tüm eski testler değişmeden geçiyor).

## /code-review (5 paralel bulucu ajan) — 6 gerçek hata bulundu ve düzeltildi

1. **En kritik**: `checkAmbientNudgeOutcome`, `Action`'a bakmadan HERHANGİ bir pending suggestion'ı işliyordu — arka plan motorunun resmi banner önerisi ekrandayken kullanıcı alakasız bir mesaj yazsa bile o banner'ı sessizce yanlış yorumlayıp temizleyebiliyordu. Fix: sadece `Action == ActionAmbient` işleniyor.
2. **Gerçek race condition**: eşleşen pattern, paylaşılan bir App alanına sadece ID olarak yazılıp arka plan goroutine'inde tekrar diskten okunuyordu — hızlı bir sonraki tur bu alanı üzerine yazabiliyordu. Fix: `finishStream`'de SENKRON olarak yakalanıp (`takeNudgedPattern`) tam pattern değeri doğrudan goroutine'e parametre olarak geçiliyor.
3. `checkAmbientNudgeSurfaced` artık `isLLMErrorReply` kontrolü yapıyor (hata cevaplarında boşuna LLM çağrısı yapılmıyordu).
4. Aynı pending suggestion'ın art arda iki mesajla çift işlenmesine karşı `resolvingSuggestionID` tek-uçuş koruması + `HandleOutcome` öncesi yeniden-fetch eklendi.
5. `GetPendingSuggestion` (desktop banner + mobile'ın kullandığı endpoint) artık `Action == ActionAmbient` olanları hariç tutuyor — ambient önerinin tüm amacı zaten ayrı bir popup OLMAMAK, banner'da da göstermek anlamsız/rahatsız edici olurdu.
6. `chat.go`, özellik tamamen kapalıyken bile her mesajda goroutine+disk okuması yapmasın diye çağrı noktasında da erken çıkış ekledi.

**Ek, ajanların bulmadığı ama tutarlılık için eklenen düzeltme:** Incognito Mode'da da bu özellik tamamen kapatıldı (`routeStream`, `saveMemoryAsync`/`updateMoodAsync` ile aynı `!incog` bloğu) — incognito'nun "hiçbir şey kalıcı olmaz" sözüne aykırı düşerdi.

## Bilinçli olarak düzeltilmeyen, kabul edilen sınırlama

`routeStream`, task-loop'un otomatik worker prompt'ları tarafından da çağrılıyor (kullanıcının yazmadığı metin) — teorik olarak bu metin de bir ambient önerinin cevabı gibi yanlış sınıflandırılabilir. Bu, AGENTS.md'nin zaten belgelediği "tek global aktif sohbet, otomatik çağıranlar SwitchChat yapmalı" mimari sınırlamasıyla aynı kategoride — bu özelliğe özgü değil, kapsam dışı bırakıldı. WhatsApp'ın bu koda hiç girmediği ayrıca doğrulandı (temiz).

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 34 paket yeşil (internal/memory dahil)
```
13 yeni test (`internal/app/proactive_ambient_test.go`) + `internal/proactive`'e 1 yeni test (`HandleOutcome`).

**Gerçek ortamda doğrulanamayan:** Bu ortamda ekran/gerçek LLM olmadığı için modelin gerçekten doğal bir şekilde alışkanlığı sohbete getirip getirmediği, ve LLM'in EVET/HAYIR/KABUL/RET formatına ne kadar sadık kaldığı canlı test edilemedi — testler sahte (canned) LLM cevaplarıyla mantığı doğruluyor, gerçek model davranışını değil.

Commit: `2910564`.

---

# Handoff — 2026-07-16 (Session 36) — Proaktif öğrenme sistemi baştan sona denetlendi, gerçek hatalar bulunup düzeltildi, desktop UI'ı ilk kez inşa edildi, default açıldı

## Özet

Kullanıcı proaktif öğrenmenin hiç çalıştığını görmediğini, üstüne "ben hiç kod ile konuşmama rağmen coding oluyor gibi" dedi. Bu şikayetten yola çıkılıp `internal/proactive` + `internal/observer` (+ ilgili `internal/app`/`internal/intent`/`frontend` uçları) sistematik olarak denetlendi. codebase-memory-mcp (search_graph/trace_path/get_code_snippet) boyunca kullanıldı. Bulunan gerçek hatalar sırayla düzeltildi, her biri ayrı, doğrulanmış, RAG'a (`internal/memory/*`) hiç dokunmayan commit'ler halinde — kullanıcının açık talimatı buydu. `/code-review` her bir Flutter/Go değişiklik grubunda ayrıca çalıştırıldı.

## Bulunan ve düzeltilen 4 gerçek hata

1. **`ClassifyTopic`'in Türkçe kelime çakışmaları (`afa9857`)** — `topicKeywords["coding"]` içinde `"git"` (Türkçe "gitmek" kökü) ve `"hata"` (Türkçe "yanlış/mistake") bare substring olarak duruyordu; `"research"` içinde `"nasıl"` (günlük "nasılsın" selamlaşmasının kökü) vardı. `strings.Contains` ham taraması yaptığı için "dün eve gittim" ya da "büyük bir hata yaptım" gibi kodlamayla hiç alakası olmayan sıradan Türkçe cümleler "coding"/"research" aktivitesi olarak etiketleniyordu — kullanıcının tam şikayeti. Fix: en tehlikeli kelimeler (git/hata/nasıl/neden/bul/test, ve testin kendisi "bug"→"bugün" çakışmasını ortaya çıkardığı için o da) listeden çıkarıldı; `ClassifyTopic` artık Unicode harf/rakam runlarına göre tokenize edip kelime-başı (prefix) eşleşmesi yapıyor (ham substring yerine) — çekimli formlar hâlâ yakalanıyor ama "beyaz" gibi kelimenin ortasına gömülü kökler artık yakalanmıyor.

2. **Beyan edilen alışkanlıklar hiç güvenilmiyordu (`7a59c00`)** — Kullanıcının sohbette açıkça söylediği bir alışkanlık ("her akşam 9'da kod yazarım", `internal/intent`'in zaten çalışan LLM tespiti üzerinden `IsHabit=true` dönüyor) bile genel istatistik havuzuna düşüp `AnalyzePatterns`'ın `MinObservations>=3` + güven eşiği (0.3) barajını geçmek zorundaydı — hafızanın pinned facts'inin RAG sıralamasını atlaması gibi bir garanti katmanı yoktu. Fix: `TimePattern.Declared` alanı + `PatternStore.SaveDeclared` (upsert, `declared:HH:MM` ID'si) + `Analyzer.Run`'ın periyodik istatistiksel yeniden hesaplamadan sonra declared pattern'leri geri birleştirmesi (`Suppress` hâlâ çalışıyor). `processMessageIntent` artık `IsHabit=true` geldiğinde bunu direkt 0.9 güvenle pinliyor.

3. **Proaktif öneriler yanlış sohbete karışıyordu (`f8f6c5a`)** — `proactiveEmit`, `sm.AddMessage(...)` ile öneriyi **o an her ne sohbet aktifse ona** enjekte ediyordu; hiç `SwitchChat` yapmadan. AGENTS.md'nin kendi belgelediği "tek global aktif sohbet" tuzağının tam örneği — arka planda 30 saniyede bir tetiklenen bir motorun, kullanıcının o an tamamen alakasız bir konuda sürdürdüğü bir konuşmaya rastgele mesaj sıkıştırması. Fix: `AddMessage` çağrısı tamamen kaldırıldı, tek teslimat yolu artık `proactive_suggestion` event'i + pending-suggestion store (zaten mobile'ın kullandığı).

4. **Desktop'ta öneriyi gösterecek hiçbir UI yoktu** — backend'in `GetPendingSuggestion`/`RespondToSuggestion` bridge metodları ve Flutter `api_client.dart`'ın `getPendingSuggestion()`/`respondToSuggestion()` metodları zaten vardı ama `frontend/lib` içinde hiçbir çağıran yoktu (mobile ayrı, kendi UI'ını kullanıyor). 3. maddedeki fix'ten sonra bu durum "hiç görünmez" hale gelirdi. Yeni: `pendingProactiveSuggestionProvider` (20s polling, `connectionStatusProvider`'ın autoDispose+alive-flag desenini taklit ediyor) + `ProactiveSuggestionBanner` (AppShell'in overlay Stack'inde, `VersionBanner` gibi her sekmeden görünür, Evet/Şimdi değil/Artık sorma butonları). Widget testi yazılırken gerçek bir overflow bug'ı yakalandı (3 buton 800x600 test viewport'ta bile taşıyordu) — `Row` yerine `Wrap`'e çevrildi.

## Default açıldı (`cffc7f1`)

`config.Default()`'taki `Proactive.Enabled: false` → `true`, `Level: "off"` → `"subtle"` — kullanıcının açık talimatıyla, yukarıdaki 4 fix + UI'dan SONRA. Not: `config.Load()` YAML'i `Default()` üzerine bindiriyor, yani `proactive:` bölümü hiç olmayan (özellik var olmadan önce kurulmuş) mevcut kullanıcılar bunu sessizce miras alacak — aynı mekanizma daha önce `AutoFactExtraction` için flaglenmişti (2026-07-15), burada flip doğrudan istenmişti ama mekanizma kayda geçsin diye not edildi.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 34 paket yeşil (internal/memory dahil — RAG'a dokunulmadığı da testle doğrulandı)
flutter analyze lib/                                            → temiz (bilinen 4 info-level bulgu)
flutter test                                                     → 107/107 (105 + proactive_suggestion_banner_test.dart'ın 2 testi)
```

Her commit ayrı ayrı `/code-review` ile gözden geçirildi (Go tarafı: `ReportFindings` boş döndü; Flutter tarafı: widget testinin kendisi overflow bug'ını yakaladı, fix commit'e dahil edildi).

## Kapsam dışı bırakılan / dokunulmayan

- `internal/memory/*` (RAG/vektör arama) — kullanıcının açık talimatıyla hiç dokunulmadı.
- Proaktif motorun `Config{}` sabitleri (Interval=30s, MinScore=0.1, MinConfidence=0.3, Cooldown=30dk) config.yaml'dan ayarlanabilir değil, hardcoded kalıyor — denetlendi, bug değil, kapsam dışı bırakıldı.
- `RecordIntent`'in `ActivityIntent` gözlemleri hâlâ `Topic: "general"` ile genel istatistik havuzuna da düşüyor (declared-pattern katmanına EK olarak, ondan bağımsız) — önceden var olan bir davranış, bu oturumda dokunulmadı.

## Sıradaki oturum için

- Kullanıcı gerçek ortamda (rebuild edilmiş binary) proaktif öneriyi canlı görüp Evet/Şimdi değil/Artık sorma akışını denemeli — bu ortamda görüntü/tarayıcı olmadığı için görsel doğrulama yapılamadı.
- "Beyan edilmiş alışkanlık" akışı da canlı test edilmeli: sohbette "her akşam X saatinde Y yaparım" gibi bir cümle söylenip `data/profile/patterns.json`'da `declared:HH:MM` ID'li, `"declared":true` alanlı bir pattern'in gerçekten oluştuğu doğrulanmalı.

---

# Handoff — 2026-07-16 (Session 35) — Session 34'ün bıraktığı açık uç kapatıldı: agent sistem promptu artık hafızanın dosya değil, hazır enjekte metin olduğunu söylüyor

## Özet

Session 34'ün handoff'unun önerdiği ilk iş buydu: `internal/identity/identity.go`'nun sistem promptuna hafızanın nasıl çalıştığını (dosya değil, otomatik enjeksiyon) açıklayan bir not eklemenin işe yarayıp yaramayacağı denendi. codebase-memory-mcp'nin graph araçlarıyla (`search_graph`, `trace_path`, `get_code_snippet`) kod okunmadan doğrudan kaynağa gidildi.

## Bulgu

Not aslında `internal/identity/identity.go` değil, `internal/app/chat.go`'daki `buildAgentSystemPrompt()` fonksiyonundaydı — agent modu açıkken (`routeStream`, `chat.go:171`) sistem promptuna ek olarak eklenen ayrı bir blok. Bu blokun zaten bir "Hafıza Hakkında" bölümü vardı (Session 33'te eklenmiş, commit `2eaa9b7`) ama SADECE dosyaya **yazmayı** yasaklıyordu ("dosya yazma araçlarını hafıza amacıyla asla kullanma"). **Okuma** tarafında hiçbir kısıtlama yoktu, ve modele hafızasının zaten `identity.BuildSystemPrompt`'un ürettiği "relevant memories" bölümünde hazır metin olarak durduğu hiç söylenmiyordu.

CLI'da `activateChat` (`internal/replcli/repl.go:202`) her sohbete geçişte agent modunu koşulsuz açtığı için (`SetAgentEnabled(true)`), bu eksik blok pratikte HER CLI turunda devrede — ve model gerçek dosya araçlarına erişebiliyor. Bu, Session 34'te gözlemlenen somut örneği (Turn 21: "boş zamanlarında ne yapıyorsun" gibi sıradan bir soruya model kendiliğinden `read_file` ile var olmayan `.../fatih_workspace/memory.json`'ı okumaya çalıştı) tam olarak açıklıyor.

## Fix

`buildAgentSystemPrompt`'un "Hafıza Hakkında" bloğu iki maddeye ayrıldı:
1. **Yeni:** Hafızanın zaten sistem promptunun "relevant memories" bölümünde düz metin olarak verildiği, hatırlayıp hatırlamadığını kontrol etmek için `read_file`/`list_directory`/`search` gibi bir dosya aracının ASLA çağrılmaması gerektiği, `memory.json` gibi bir dosyanın diskte olmadığı (gerçek hafıza SQLite'ta, modelin erişimi dışında) açıkça yazıldı.
2. **Korundu:** Session 33'ün orijinal yazma yasağı ikinci madde olarak aynen kaldı.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → whisper paketindeki TestGetStatus_NewServer hariç hepsi yeşil
```
`internal/whisper`'daki tek FAIL yine bu oturumla ilgisiz: `lsof -i :9877` ile doğrulandı, gerçek bir `whisper-server` süreci (önceki bir canlı test oturumundan kalma) o portu hâlâ dinliyor.

**Gerçek ortamda doğrulanamayan:** Bu, salt bir sistem promptu metni değişikliği — modelin gerçekten `read_file`'ı çağırmayı bırakıp bırakmayacağı probabilistik bir davranış, otomatik testle garanti edilemez. Bir sonraki canlı/agent testinde (özellikle "hatırlıyor musun" tarzı sorularla) tekrar gözlemlenmeli.

Commit: (bu handoff girişiyle birlikte, ayrı `docs:` commit'i).

---

# Handoff — 2026-07-15 (Session 34) — Session 33'ün açık ucu kapatıldı: CLI'nın "hafıza bazen kaydediyor bazen kaydetmiyor" şikayetinin gerçek kök nedeni bulundu, düzeltildi, canlı doğrulandı

## Özet

Session 33'ün bıraktığı gerçek açık uç ("CLI'da hafıza neden tutarsız?") bu oturumda çözüldü. Kullanıcı canlı test istedi; bunun için önce `memo -p "mesaj"` adında yeni, non-interaktif bir CLI modu eklendi (terminal REPL'siz, tek mesaj gönderip `[chat:<id>]` + `[memory:<durum>]` basıp çıkan), sonra bunu gerçek bir backend + gerçek OpenCode Zen (mimo-v2.5-free) sağlayıcısına karşı hem elle hem de Haiku 4.5 ile sürülen otomatik bir "Fatih" persona testiyle (23 turn, dosya oluşturma/düzenleme/silme + web arama + tekrarlı hatırlama testleri) canlı olarak koşturuldu.

## Bulunan ve düzeltilen gerçek bug

`internal/replcli/repl.go`'daki `memorySavedSince`/`eventDataSince` — CLI'nın "✓ hafıza kaydedildi" onayı — turun başında `/api/events` halkasının SON elemanını `before` olarak yakalayıp, turdan sonra "bu `before`'a eşit olan son event'ten SONRAKİ bir memory:saved var mı" diye arıyordu; eşitlik kontrolü **Name+Data** üzerindendi. Ama her `memory:saved` event'i aynı (boş) Data'yı taşıyor — yani art arda iki turun İKİSİ de kaydettiğinde (gerçek bir sohbette normal durum, çünkü `saveMemorySync` neredeyse her turu kaydediyor), önceki turun kaydı ile bu turun YENİ kaydı **içerik olarak birbirinin aynısı**. "before'a eşit olan SON occurrence" araması bu yüzden bu turun kendi yeni event'ine denk geliyor, ondan sonrası boş kalıyor — onay sessizce "hiçbir şey olmadı" diyor, halbuki backend log'u her seferinde gerçek, hızlı (<300ms) bir `SaveInteraction` tamamlandığını gösteriyor.

Canlı doğrulama: Haiku'nun sürdüğü 23 turn'lük gerçek CLI oturumunda **23/23 turda** `[memory:none-detected]` çıktı — ama model turlar arası gerçekleri (iş, evcil hayvan, yemek tercihi) fresh chat'lerde %94 doğrulukla hatırlamaya devam etti. Yani asıl kayıt/RAG mekanizması çalışıyordu, sadece CLI'nın kullanıcıya gösterdiği "kaydedildi" onayı yalan söylüyordu — tam da kullanıcının "bazen kaydediyor bazen kaydetmiyor" şikayetiyle birebir örtüşüyor.

**Fix:** Her event'e, ring buffer'a push edilirken bir daha asla tekrarlanmayan, artan bir `Seq` (`internal/app/app.go`'nun `eventRing`'i) atandı. `memorySavedSince`/`eventDataSince` artık Name+Data eşitliği yerine `Seq > snapshotlanan seq` karşılaştırması yapıyor — içerik ne kadar özdeş olursa olsun yanlışsız çalışıyor. `GetEvents()` yeni alanı `"seq"` (string) olarak dışa veriyor, CLI tarafı `json:"seq,string"` ile geri okuyor.

Regresyon testi: `TestMemorySavedSince_ConsecutiveSavesLookIdentical` (`internal/replcli/repl_test.go`) — tam bu senaryoyu (ring'de art arda iki eşit-içerikli save, aradan snapshot alınmış) reprodüklüyor, eski Name+Data implementasyonuna karşı FAIL, Seq fix'ine karşı PASS olduğu doğrulandı. Diğer tüm `memorySavedSince`/`eventDataSince` testleri yeni `(events, afterSeq)` / `(events, afterSeq, name)` imzasına güncellendi.

Canlı yeniden test: fix'li binary ile aynı sohbette art arda 3 mesaj gönderildi, üçünde de `[memory:saved]` doğru şekilde göründü (fix öncesi ikinci mesajdan itibaren hep "none-detected" oluyordu).

Commit: `c1fd2bd` (fix), `4653b66` (`-p` CLI özelliği, ayrı commit — fix'e bağımlı olduğu için sırayla).

## Yan ürün: `memo -p "mesaj" [--chat <id>] [--auto-allow]`

Interaktif REPL'e girmeden tek mesaj gönderip çıkan yeni bir CLI modu (`main.go`). Gerçek terminal oturumuyla AYNI client çağrılarını (NewAgentChat/SwitchChat/SetAgentEnabled/SendStream, aynı memory-saved-since-snapshot polling) kullanıyor — ayrı bir test-özel implementasyon değil. `--auto-allow` (varsayılan kapalı) araç izin isteklerini `deny_once` yerine `allow_once` ile otomatik geçiyor; sadece scriptli/otomatik test senaryoları için, insan onayı olmadan dosya/web aracı çalıştırmayı göze alan bilinçli bir opt-in. Bu oturumda hem elle hem Haiku 4.5 alt-agent'ıyla canlı test için kullanıldı; genel olarak scripting/CI için de kullanılabilir.

## Fatih persona testinin diğer bulguları

- **Araç çalıştırma:** `write_file`/`edit_file`/`delete_file`/`web_search` dördü de gerçek turlarda doğrulandı çalıştı (stderr'de tool_executing/tool_result, dosya diskte gerçekten oluştu/değişti/silindi, web araması gerçekçi güncel içerik döndürdü). Bu tarafta bir sorun yok.
- **"Buğra" karışıklığı (bug DEĞİL, test metodolojisi hatası):** Bir fresh-chat recall turunda bot "sen Buğra'sın, en sevdiğim renk mor" dedi — Fatih'in adı/rengi değil. Sebep: aynı oturumda Fatih testinden ÖNCE ben kendi elle smoke testimde aynı paylaşılan `data/` deposuna "Buğra, favori renk mor" gerçeğini kaydetmiştim. Memo tek-kullanıcılı bir uygulama olarak tasarlandığı için pinned facts global — iki farklı "persona"yı aynı depoya karıştırmak benim test kurulumumun hatası, üründe bir izolasyon bug'ı değil.
- **YENİ AÇIK UÇ (düzeltilmedi, sadece gözlemlendi):** Turn 21'de kullanıcı sadece "boş zamanlarında ne yapıyorsun" diye normal bir soru sordu, ama model KENDİLİĞİNDEN `read_file` aracını `.../fatih_workspace/memory.json` üzerinde çağırdı — böyle bir dosya hiç yok (gerçek hafıza SQLite, `.db`), araç `stat failed` hatası verdi, ama model yine de doğru cevabı (muhtemelen zaten sistem promptuna enjekte edilmiş pinned facts'ten) verdi. Bu, Session 32/33'te flaglenip hiç kök nedeni bulunmamış "model 'hatırlıyor musun' sorusuna dosya okuma/yazma ile cevap vermeye çalışıyor" temasının YENİ bir somut örneği — muhtemelen agent modunun her zaman açık olması (`repl.go`'nun `activateChat`'i koşulsuz `SetAgentEnabled(true)` çağırıyor) + sistem promptunun modele "hafızan zaten context'e enjekte edilmiş, kontrol etmen gerekmiyor" demiyor olması kombinasyonu. Bu oturumda kapsam dışı bırakıldı (probabilistik model davranışı, tek bir örnekten kök neden çıkarmak riskli) — **sıradaki oturumun ilk işi bu olabilir**: `internal/identity/identity.go`'nun sistem promptuna hafızanın nasıl çalıştığını (dosya değil, otomatik enjeksiyon) açıklayan bir not eklemek işe yarar mı, denenmeli.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → whisper paketindeki TestGetStatus_NewServer hariç hepsi yeşil
```
`internal/whisper`'daki tek FAIL bu oturumla ilgisiz: test gerçek bir TCP bağlantı kontrolü yapıyor ve benim canlı test için ayakta bıraktığım backend'in kendi whisper sunucusu tam o portu (9877) gerçekten dinliyor olduğu için ortam kirliliğinden başarısız oluyor — kod değişikliğiyle alakası yok, backend kapatılınca geçer.

## Not: gerçek API key

Kullanıcı bu oturumda gerçek bir OpenCode Zen API key'i paylaştı, `data/providers.json`'a (şifreli) yazıldı ve `active_provider` olarak ayarlandı — bu repo'nun yerel test verisi olarak kalıyor, kullanıcı test verisinin bozulmasının/silinmesinin önemli olmadığını belirtti.

---

# Handoff — 2026-07-15 (Session 33) — Yanlış hedefe kilitlenme: agent/tool-call sızıntısını düzelttim, ama asıl şikayet hafızanın CLI'da kararsız çalışmasıydı — HÂLÂ AÇIK

## ÖNEMLİ: bu oturumda yanlış anlaşılma oldu, düzeltiyorum

Session 32'nin sonunda kullanıcı "CLI garip davranıyor" dedi ve bir transkript paylaştı (ham `<function_calls>` XML sızıntısı + uydurma "memory.json" izin isteği). Ben bunu "CLI'daki garip davranış" olarak yorumlayıp derin bir `/code-review` + `--fix` turu yaptım (bkz. aşağıdaki "Yapılan (agent/tool-call turu)" bölümü) — 5 bulgu ajanı, kök neden tespiti, gerçek fix'ler, testler, 2 commit (`f2f3594`, `2eaa9b7`).

**Ama oturumun sonunda kullanıcı netleştirdi:** asıl şikayet bu değilmiş. Kullanıcının gerçek şikayeti: **hafızanın CLI'da "garip, saçma, stabil değil, bir çalışıp bir çalışmaması"** — yani GUI'de "harika" çalışan hafızanın (bugünkü pinned-facts + FTS5 + compound-query fix'leri sonrası) CLI'da hâlâ tutarsız davranması. Bu, agent/tool-call konusundan tamamen ayrı bir şikayet ve **hiç araştırılmadı, hiç düzeltilmedi.**

## Gerçek açık uç: CLI'da hafıza neden tutarsız? (araştırılmadı)

Mimari olarak CLI ve GUI **aynı backend binary'sini ve aynı `internal/memory`/`internal/app` kodunu** kullanıyor (main.go: CLI kendi backend'ini `os.Executable()` ile `--headless` olarak yeniden başlatıyor, ayrı bir CLI-özel kod yolu yok — bu daha önce de doğrulanmıştı). Yani bugünkü tüm hafıza fix'leri (FTS5 tag, AND→OR, compound query bölme, pinned facts) İKİSİNE de eşit uygulanıyor OLMALI. Kullanıcı yine de CLI'da tutarsızlık görüyorsa, olası sebepler (hiçbiri doğrulanmadı):

1. **Kurulu CLI binary'si güncel değil** — GUI taze derlenmiş/çalıştırılmış olabilir ama `~/.memo/bin/memo` (kurulu CLI kopyası) eski bir build olabilir. Session 32'de bu ihtimal öne sürülmüştü ama doğrulanmadı.
2. CLI'nın kendi REPL akışında (`internal/replcli`) hafıza sorgusunu nasıl inşa ettiği (`buildMemoryQuery` vb.) GUI'den farklı bir yol izliyor olabilir — kontrol edilmedi.
3. CLI'da "önceki sohbet geçmişi" enjeksiyonu (Session 30/31'deki transkriptlerde görülen "── önceki sohbet geçmişi ──" bloğu) hafıza sorgusuna farklı bir bağlam katıyor olabilir, bu da RAG aramasının farklı sonuçlar vermesine sebep olabilir — kontrol edilmedi.
4. Kullanıcının paylaştığı gerçek transkriptlerde (Session 30) chat1/chat2/chat3 arası tutarsızlık zaten dokümante edilmişti (isim/doğum günü/renk bazen hatırlanıyor bazen hatırlanmıyor) — bugünkü compound-query-splitting fix'i (`d81d2da`) bunun bir kısmını çözmüş olabilir ama kullanıcı hâlâ "garip" diyor, yani ya fix yetersiz ya da farklı bir sorun var.

**Sıradaki oturumun ilk işi bu olmalı:** CLI'da spesifik olarak hafızanın ne zaman/nasıl "çalışmadığını" gerçek bir örnekle (kullanıcıdan yeni bir transkript istenerek) yakalamak, agent/tool-call konusuyla karıştırmadan.

## Yapılan (agent/tool-call turu — gerçek fix'ler ama YANLIŞ ŞİKAYETE cevaben)

Bu kısım hâlâ değerli, gerçek bug'lardı, ama kullanıcının bugünkü asıl şikayetine cevap DEĞİL:

- `internal/agent/pipeline.go`: modelin düz metnine sızan sahte `<function_calls>` XML'i artık temizleniyor (backend seviyesinde, hem CLI hem GUI'yi korur).
- `internal/app/chat.go`: agent sistem promptu artık gerçek tool-calling protokolünü anlatıyor, dosya yazarak "hafıza taklidi" yapmaması gerektiğini söylüyor.
- `internal/replcli/repl.go`: cevap bittikten sonraki ~2.4s hafıza-kayıt bekleme penceresinde klavye girdisinin sessizce yutulması düzeltildi (gerçek, bağımsız bir CLI input bug'ıydı, ama "hafıza garip" değil "klavye garip" kategorisinde).
- CLI izin diyaloğu GUI ile eşitlendi (danger-level gösterimi, allow_session, Preview kullanımı).
- Hafıza debug panelindeki "pinned" etiketleme eksikliği düzeltildi.

Detaylar için commit `f2f3594` ve `2eaa9b7`'nin mesajlarına bakılabilir.

## Doğrulama

Tüm 34 paket `-race` ile yeşil, `flutter analyze lib/` temiz (ilgisiz, önceden var olan info-seviye bulgular hariç).

---

# Handoff — 2026-07-15 (Session 32) — Gün sonu: dokümantasyon senkronu + kullanıcıdan ilk canlı geri bildirim + iki açık uç

## Özet

Session 28-31'in tamamı (FTS5 tag, AND→OR, compound query bölme, pinned-facts katmanı, /code-review'un bulduğu 5 hata) aynı günün ürünü. Bu son oturumda: (1) obsidian-doc/obsidian-doc-en/docs dokümantasyonu gerçek mimariyle senkronize edildi, (2) kullanıcı gerçek ortamda ilk kez test etti ve ekran görüntüsü paylaştı, (3) alakasız ama ciddi görünen yeni bir bug rapor edildi (henüz araştırılmadı, kullanıcı kendi test edip rapor verecek).

## Dokümantasyon senkronu (commit `fecd9d4`)

`obsidian-doc/Memo`, `obsidian-doc-en/Memo` ve `docs/`'taki hafıza/RAG sayfaları, bugünkü değişikliklerin ötesinde **hiç var olmamış bir mimariyi** anlatıyordu (her etkileşim ayrı dosya, RAM'de vektör önbelleği, "multi-worker" paralel tarama — hiçbiri `internal/memory/store.go`'da yok). Gerçek şema ve gerçek hibrit arama akışıyla (vektör+FTS5+RRF, compound query bölme, pinned facts) uyumlu hale getirildi. 15 dosya güncellendi. `docs/RESOLVED_ISSUES.md`'ye dokunulmadı (kendi altbilgisi donmuş bir 2026-06-03 denetim kaydı olduğunu söylüyor).

## Kullanıcının canlı testi: iyi haber + bir gözlem

Kullanıcı Ayarlar → Bellek → "Bellek Ara (Debug)" panelinden gerçek bir sorgu denedi (ekran görüntüsü paylaşıldı). Sonuçlar tam olarak istenen formatta: "User's name is Buğra Akdemir.", "User's favorite color is red.", "User's dog Zeytin is 3 years old." gibi temiz, üçüncü şahıs, atomik cümleler — `extractAndPinFacts`'in ürettiği tam format. **Otomatik gerçek çıkarımı gerçek ortamda çalışıyor, doğrulandı.**

**Gözlem (henüz düzeltilmedi):** Debug panelindeki tüm sonuçlar `MatchType="Vektör"` gösteriyor, "Pinned" değil. Sebebi muhtemelen: debug arama endpoint'i (`DebugMemorySearch` → `Store.DebugSearch` → `RetrieveContext`) doğrudan store katmanını çağırıyor, `internal/app/memory.go`'daki `retrieveMemory`'nin pinned-facts merge adımını (GetPinnedFacts + öne ekleme) hiç görmüyor. Yani debug ekranı gerçek sohbette kullanılan tam yolu göstermiyor, sadece alttaki hibrit RAG aramasını gösteriyor — pinned olan bir gerçek de aynı zamanda ham vektör aramasında bulunabildiği için orada "Vektör" olarak görünüyor. Kullanıcıya bunu düzeltip düzeltmemesini sordum, henüz cevap yok.

Kullanıcının kendi ifadesi: "gerçekten hissettim akıllandığını... önceden merhaba yazdığımda eski sohbete gönderdiği çıktıyı atıyordu gibi" — olumlu ama temkinli ("gibi"), kendi deyimiyle uzun soluklu bir test yapıp ayrıca rapor edecek.

## AÇIK UÇ — henüz hiç araştırılmadı: agent/tool-call sızıntısı + beklenmedik dosya yazma izni

Kullanıcı bambaşka, alakasız ve ciddi görünen bir transkript paylaştı (CLI, OpenCode Zen/mimo-v2.5-free): basit bir "adımı hatırlıyor musun" sorusuna cevapta ham `<function_calls><invoke name="read_env">...<invoke name="list_directory">...` XML'i sohbet metnine SIZMIŞ (gerçek bir tool-call formatı, ekran çıktısına düz metin olarak karışmış). Ayrı bir sohbette (chat1), kullanıcı **"memory.json" adında bir dosya yazma izni istendiğini** bildirdi — repo'da `memory.json` diye bir şey yok (`grep` ile doğrulandı), yani model bunu uydurmuş; gerçek hafıza `memory.db` (SQLite), asla `.json` değil.

Başlanan ama tamamlanmayan araştırma:
- `read_env`/`list_directory` gerçek agent tool isimleri (`internal/agent/tools/*.go`) — yani agent modu bu oturumlarda gerçekten aktifti.
- `internal/replcli/repl.go:206`, `activateChat()` içinde her sohbete geçişte `SetAgentEnabled(ctx, true)` **koşulsuz olarak** çağrılıyor — CLI'da agent modu kullanıcı hiç istemeden her zaman açık olabilir. Bu, kullanıcının sıradan bir "hatırlıyor musun" sorusunda bile modelin dosya sistemi araçlarına erişebiliyor olmasını açıklar.
- Henüz kontrol edilmedi: (a) REPL'in tool-call XML'ini neden düz metin olarak gösterdiği (parse edilmeyip sızdığı) — model belki Memo'nun gerçek tool-calling formatını değil, Claude tarzı bir XML formatını taklit ediyor ve Memo bunu tanımıyor; (b) sistem promptunun modelin "hatırlama" isteğini dosya yazmaya yönlendirmesine neden olan bir şey içerip içermediği; (c) bu izin isteğinin CLI'ya özgü mü yoksa GUI'de de olur mu.

**Sıradaki oturum için ilk iş bu olmalı** — kullanıcı kendi testini yapıp rapor verecek, ama `repl.go:206`'daki koşulsuz `SetAgentEnabled(true)` çağrısı şimdiden şüpheli, oradan başlanabilir.

## Cevap bekleyen, kodda karara bağlanmamış soru (Session 31'den beri açık)

`config.Load()`, YAML'i `Default()`'in üzerine bindiriyor — mevcut kullanıcıların güncelleme sonrası `AutoFactExtraction=true`'yu sessizce (onay istenmeden) alması. Henüz kullanıcıdan cevap yok.

---

# Handoff — 2026-07-15 (Session 31) — "Profil gerçekleri" mekanizması inşa edildi: RAG dışında, garantili hafıza katmanı

## Özet

Session 30'un handoff'unda önerilmiş ama inşa edilmemiş fikir aynı gün hayata geçirildi: isim/doğum günü/evcil hayvan gibi çekirdek bilgilerin RAG sıralamasına hiç girmeden, her sistem promptuna garantili enjekte edildiği ayrı bir "pinned facts" katmanı.

## Araştırma

mem0 ve MemGPT/Letta'nın bu tam sorunu ("çekirdek bir gerçek asla bir benzerlik yarışması kazanmaya bağlı olmamalı") nasıl çözdüğü araştırıldı (web search): ikisi de küçük, küratörlü bir gerçek listesini HER promptta komple enjekte ediyor, arama/sıralama sadece geri kalan her şey için kullanılıyor (Letta: "Core Memory" vs "Archival Memory").

## Yapılan

- `Store.GetPinnedFacts` (`internal/memory/store.go`) — `source='explicit'`/`importance=5` hafızaları, 50 ile sınırlı, döndürüyor.
- `retrieveMemory` (`internal/app/memory.go`) bunları RAG sonucuna koşulsuz birleştiriyor — `RetrieveContext`'in sıralamasını tamamen atlıyor.
- `extractAndPinFacts` — her kaydedilen sohbet turundan SONRA, arka planda, dar kapsamlı bir LLM çağrısı ("bu mesajda kalıcı bir kişisel gerçek var mı, varsa kısa metin") çalıştırıp cevabı mevcut `SaveExplicit`/`/remember` yoluyla pinliyor. Bu, normal sohbette söylenen gerçekleri de yakalıyor — önceki iki fix bunu çözemiyordu çünkü onlar sadece RAG sıralamasını iyileştiriyordu, hiçbir şeyi "pinlemiyordu".
- Yeni `Memory.AutoFactExtraction` config bayrağı (varsayılan `true`) — tam kapatma anahtarı.

## Elenen tasarımlar

- Regex/anahtar kelime tabanlı tespit: Memo iki dilli (TR/EN), regex ölçeklenmiyor.
- Cevabı üreten AYNI çağrının gizli bir etiket basması: bu, hafızanın güvenilirliğini agent modunun "model formatı doğru takip eder mi" kırılganlığına bağlar — agent henüz tam stabil değilken, hafızanın ondan DAHA sağlam olması gereken tam da bu özellik için yanlış bir taviz.

## /code-review'un bulduğu 5 gerçek hata (hepsi aynı gün düzeltildi)

1. **KRİTİK** — `extractAndPinFacts` başlangıçta `a.callLLM` yerine `a.providerRouter.ChatCompletion`'ı doğrudan çağırıyordu — bu AYNI anti-pattern, bu dosyada daha önce bulunup düzeltilmiş (`ImportMemoryFromText`, 2026-07-13). Sonuç: local-only kurulumlarda (Memo'nun asıl "local-first" kullanım senaryosu) özellik sessizce hiç çalışmıyordu.
2. Pinned facts, sonuç dizisinin SONUNA ekleniyordu, ama `identity.BuildSystemPrompt` bütçe aşıldığında bloğu KUYRUKTAN kesiyor — ağır kullanıcılarda (çok RAG geçmişi + çok pinned fact) tam da korunması gereken şeyi ilk atıyordu. Fix: pinned facts başa alındı.
3. Arka plan extraction çağrısı, tek `memorySaveWorker` goroutine'i içinde SENKRON çalışıyordu — yoğun kullanımda kuyruğun birikme riski. Fix: kendi goroutine'inde ateşleniyor.
4. `FindMergeCandidates`'ın gece consolidation'ı iki neredeyse-aynı pinned fact'i birleştirip sessizce un-pin edebiliyordu (`source` `'merged'` oluyor). Fix: `source='explicit'` aday havuzundan çıkarıldı.
5. `GetPinnedFacts` sadece `source='explicit'` kontrol ediyordu, `importance=5`'i değil — kendi doc yorumunun iddiasıyla tutarsız. Fix: eklendi.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 34 paket yeşil
```

## Bilinen, kabul edilmiş sınırlamalar (düzeltilmedi, dokümante edildi)

- 50'lik cap sadece "en yeni" mantığıyla eviction yapıyor — çok eski bir çekirdek gerçek (bir kere söylenmiş isim gibi) auto-extraction listeyi hızla doldurunca teorik olarak düşebilir.
- Local modelde extraction ile gerçek sohbet cevapları aynı tek-slotlu (`--parallel 1`) llama-server için yarışıyor.

## Kullanıcıya sorulması gereken, koda tek taraflı yazılmayan karar

`config.Load()`, YAML'i `Default()`'in üzerine bindiriyor — yani mevcut bir kullanıcının yükseltme öncesi `config.yaml`'ı (yeni anahtar yok) sessizce `AutoFactExtraction=true` alıyor, opt-in gerektirmiyor. Bu, mevcut testerlar için gerçek bir onay/şeffaflık sorusu — kodda tek taraflı karar verilmedi, kullanıcıya soruldu.

## Sıradaki oturum için

- Kullanıcının config-default sorusuna cevabı bekleniyor.
- Gerçek ortamda test: extraction'ın gerçekten local modelle çalıştığını, pinned facts'in gerçekten her promptta göründüğünü doğrulamak.

---

# Handoff — 2026-07-15 (Session 30) — Üçüncü kök neden: her sohbet turu eşit önemde kaydediliyor, compound sorular tek bir vektöre sıkışıyor

## Özet

Kullanıcı gerçek, çok oturumlu bir CLI transkripti paylaştı: bazı sohbetlerde tüm gerçekleri (isim, doğum günü, renk, kedi, köpek) doğru hatırlıyor, başka bir sohbette köpeği hatırlayıp rengi unutuyor — görünürde bir örüntü yok. "Her chatte bazı şeyleri hatırlıyor bazılarını hatırlamıyor, garip, önce nedenini anla, sonra RAG/hafıza sistemini %100 stabil hale getir" dedi.

## Araştırma

1. `internal/app/memory.go`'daki `saveMemoryAsync` incelendi: HER sohbet turu (selam/naber dahil) `SaveInteraction` ile aynı varsayılan `importance=3` ile kaydediliyor. Kişisel bir gerçek ("köpeğimin adı Zeytin" gibi normal sohbette söylenen) ile "selam" arasında ÖNCELİK farkı yok — hiçbir "bu kalıcı bir gerçek" tespiti yok.
2. Gerçekçi bir senaryo simüle edildi (30 rutin "kanka naber" sohbeti + normal sohbette söylenmiş 1 gerçek, kelime örtüşmesini gerçekten yansıtan `bagOfWordsEmbedding` test embedding'i ile): compound bir soru ("kanka naber ve köpeğimin adı neydi") TEK bir harmanlı vektöre dönüşüyor, ve düzinelerce rutin sohbet turu bu harmanlı vektöre gerçeğin kendisinden DAHA benzer çıkıyor — gerçek topK=5 kesiminin tamamen dışında kalıyor. Fix devre dışı bırakılıp aynı test tekrar çalıştırılarak doğrulandı: 0/5 sonuç gerçeği içeriyordu.
3. İlk denenen ama işe yaramayan yaklaşım: `importance>=5` olan hafızalara RRF sıralamasından bağımsız garanti bir yer ayırmak. Bu HEM işe yaramadı (bir hafıza en baştaki `candidateK` aday havuzuna hiç girmediyse, sonradan tarayarak kurtarılamıyor) HEM DE mevcut bir testi (`TestRecall_TopKLimitRespected`) kırdı — çok sayıda hafıza `importance=5` paylaştığında sonuç listesini sınırsızca şişiriyordu. Bu yaklaşım geri alındı.

## Fix

`splitCompoundQuery` (`internal/memory/store.go`) — sorguyu bağlaç/noktalama işaretlerine göre ("ve"/"ile"/"and"/",") ayrı konu segmentlerine bölüyor. `RetrieveContext` artık her segment için TAM bütçeli (candidateK) ayrı bir vektör araması yapıp RRF ile ana sonuca birleştiriyor — her konu artık SADECE kendi kelimeleriyle gürültüyü yenmek zorunda, tüm cümleye harmanlanmış haliyle değil. Tek-konulu sorgularda `splitCompoundQuery` `nil` döner, yani mevcut davranış değişmiyor.

Ayrıca `Memory.TopK` varsayılanı 5'ten 8'e çıkarıldı (`internal/config/config.go`) — segment birleştirmesi artık daha fazla bağımsız-alakalı aday çıkarabildiği için biraz ekstra alan.

## Testler

- `TestSplitCompoundQuery` — saf bölme mantığının birim testi
- `TestRecall_CasualFactNotCrowdedOutByRoutineNoise` — transkriptteki TAM senaryoyu tekrar eder (gerçek `SaveInteraction` ile kaydedilmiş, `SaveExplicit` DEĞİL), fix devre dışıyken FAIL, etkinken PASS olduğu doğrulandı

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 34 paket yeşil
```

## Dürüst sınır: bu "%100 stabil" değil

`splitCompoundQuery` bağlaç/noktalama tabanlı, semantik değil — bağlaç içermeyen bir compound soru, ya da çok büyük bir hafıza deposunda tek-konulu bir soru bile, hâlâ düz sıralamaya bağlı ve gürültüde kaybolabilir. İsim/doğum günü/evcil hayvan gibi az sayıda çekirdek gerçeğin GERÇEKTEN deterministik hatırlanması için, RAG sıralamasının tamamen dışında, her sistem promptuna her zaman enjekte edilen ayrı bir "profil gerçekleri" mekanizması gerekir — bu önerildi ama inşa edilmedi (kapsam kararı, bug değil).

## Sıradaki oturum için

- Kullanıcı gerçek ortamda tekrar test etmeli; aynı sınıf bug tekrar rapor edilirse önce "bağlaçsız ifade mi" yoksa "çok büyük depo mu" ayrımını yap, otomatik olarak aynı kök neden sanma.
- "Profil gerçekleri" mekanizması (isim/doğum günü/evcil hayvan gibi sabit alanları RAG dışında her zaman sisteme enjekte etmek) kullanıcıya önerildi, onay beklemeden inşa edilmedi — istenirse ayrı bir görev olarak ele alınmalı.

---

# Handoff — 2026-07-15 (Session 29) — İkinci, daha derin kök neden: FTS5 sorgusu implicit AND kullanıyordu, çok-konulu sorularda hiç eşleşme bulamıyordu

## Özet

Session 28'in FTS5 build-tag fix'i commit'lendikten (`e4889e1`) sonra kullanıcı "hafıza RAG sistemi için kapsamlı testler yaz, gerçekten hatırlayıp hatırlamadığını kontrol edecek" dedi. Bu testleri yazarken **ikinci, daha derin bir kök neden** bulundu: build-tag fix'i tek başına, kullanıcının bildirdiği asıl senaryoyu düzeltmiyordu.

## Bulgu

`internal/memory/store.go`'daki `escapeFTSQuery`, sorgunun her kelimesini tırnaklı bir ifadeye çeviriyor ve boşlukla birleştiriyordu. FTS5'te boşlukla ayrılmış terimler **implicit AND** demek — yani "adımı ve doğum günümü ve en sevdiğim rengi biliyor musun" gibi çok-konulu, doğal bir soru, TEK bir hafıza satırının "adımı", "ve", "doğum", "biliyor", "musun" dahil HER kelimeyi içermesini arayan bir MATCH ifadesine dönüşüyordu. Hiçbir gerçek hafıza satırı bu kadar spesifik olamayacağı için `ftsSearch` bu tarz sorularda her zaman 0 satır döndürüyordu, ve `RetrieveContext`'teki `if len(ftsMemories) > 0` guard'ı FTS/RRF birleştirmeyi tamamen atlayıp sessizce vektör-only'e düşüyordu — yani build-tag fix'inden SONRA bile, kullanıcının bildirdiği tam senaryo hâlâ aynı şekilde başarısız olurdu.

Gerçek bir sqlite3 fts5 tablosuyla doğrudan doğrulandı: AND-join edilmiş sorgu 0 satır döndürdü; aynı kelimeler OR-join edildiğinde doğru satır bm25'e göre ilk sırada çıktı.

## Fix

`escapeFTSQuery`, kelimeleri `" "` yerine `" OR "` ile birleştirecek şekilde değiştirildi — her kelime bağımsız bir aday eşleşme oluyor, bm25 (yaygın kelimeleri IDF ile zaten düşük ağırlıklandırıyor) sıralamayı yapıyor.

## Yeni test paketi: `internal/memory/store_recall_test.go`

Hafıza RAG/embedding pipeline'ı için kapsamlı, "gerçekten hatırlıyor mu" odaklı bir test paketi eklendi:
- `bagOfWordsEmbedding` yardımcı fonksiyonu — mevcut basit 3-eksenli `testEmbedding`'in aksine, kelime örtüşmesini gerçekten takip eden bir cosine similarity üretiyor, bu yüzden gerçek embedding "dilution" (seyrelme) etkilerini test edebiliyor.
- `TestRecall_CompoundQuery_ShortFactSurvivesNoise` — kullanıcının bildirdiği TAM senaryonun regresyon testi: kısa bir gerçek (favori renk), sorgunun diğer konularıyla kısmen örtüşen "gürültü" (rastgele sohbetler) arasında gömülü — bu test, fix öncesi AND-join koduna karşı FAIL, OR-join fix'ine karşı PASS olacak şekilde bizzat doğrulandı (geçici olarak fix geri alınıp test çalıştırıldı, gerçekten kırıldığı görüldü).
- Ayrıca: bağımsız çoklu-gerçek recall, store kapat/yeniden aç arası kalıcılık (yeni sohbet oturumu simülasyonu), importance-tabanlı sıralama, minSimilarity filtresi, topK limiti, parçalanmış (chunked) uzun metinde gömülü detay recall, ve `expandQuery`/`escapeFTSQuery`/`reciprocalRankFusion` için birim testleri.

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...              → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                 → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 34 paket yeşil
```

## Ders

FTS5 build-tag fix'i "zaten yazılmış, zaten test edilmiş bir mekanizmayı açıyor" diye düşünülmüştü — ama hiç çalıştırılmamış bir kod yolunun kendi test edildiği iddiası şüpheyle karşılanmalı: `TestHybridSearch_MatchTypeSet` FTS'in AÇIK olduğu bir ortamda hiç çalışmamıştı, bu yüzden `escapeFTSQuery`'nin AND semantiği hiç fark edilmemişti. Bir kod yolu ilk kez gerçekten çalışır hale geldiğinde, "derleniyor ve testi var" varsayımıyla yetinmeyip sorgu semantiğini sıfırdan doğrulamak gerekiyor.

## Sıradaki oturum için

- Kullanıcının gerçek ortamda (rebuild edilmiş binary, gerçek embedding modeli) tam senaryoyu tekrar denemesi hâlâ asıl doğrulama — bu sefer hem build-tag hem query-semantiği fix'i birlikte devrede.
- `escapeFTSQuery`'nin OR-join'i Türkçe "ve/bir/bu" gibi çok yaygın kelimelerde bm25 sıralamasını nasıl etkiliyor, büyük (binlerce kayıtlı) gerçek bir hafıza deposunda henüz gözlemlenmedi — sadece küçük sentetik testlerde doğrulandı.

---

# Handoff — 2026-07-15 (Session 28) — Kök neden bulundu: FTS5 hiçbir zaman derlenmemiş, hibrit hafıza araması yıllardır sadece vektörle çalışıyormuş

## Özet

Kullanıcı gerçek bir CLI kullanım örneği yapıştırdı: bir sohbette "en sevdiğim renk kırmızı" dedi, `✓ hafıza kaydedildi` gördü (kayıt gerçekten başarılı oldu). Yeni bir sohbette "adımı, doğum günümü ve en sevdiğim rengi biliyor musun" diye tek, birleşik bir soru sordu — Memo adını ve doğum YILINI (günü/ayını değil) hatırladı ama rengi bilmediğini söyledi, az önce kaydedilmiş olmasına rağmen. Kullanıcı "detaylı ve profesyonel bir araştırma yap, bu hatayı artık düzelt" dedi.

## Araştırma zinciri

1. `internal/app/memory.go`'daki `saveMemorySync` incelendi — `memory:saved` event'i SADECE gerçek bir başarılı yazmadan sonra emit ediliyor (önceki oturumda doğrulanmış davranış). Yani kayıt gerçekten olmuş.
2. `retrieveMemory`/`buildMemoryQuery` (`internal/app/helpers.go`) incelendi — yeni bir sohbette geçmiş yoksa RAG sorgusu doğrudan kullanıcının ham mesajı. Kullanıcının mesajı ÜÇ farklı konuyu (isim+doğum günü+renk) TEK bir cümlede soruyordu — tek bir embedding vektörü üç konuyu harmanlıyor.
3. `internal/memory/store.go`'daki `RetrieveContext` (satır 693) incelendi — burada ZATEN yazılmış, test edilmiş, gerçek bir hibrit sistem var: vektör arama + sorgu genişletme + **FTS5 anahtar kelime arama**, hepsi Reciprocal Rank Fusion ile birleştiriliyor. Ama bu FTS5 yarısı `s.useFTS` bayrağının arkasında — ve `tryCreateFTSTable`'ın `CREATE VIRTUAL TABLE ... USING fts5(...)`'i her zaman `no such module: fts5` hatasıyla başarısız oluyordu (backend loglarında daha önce de görülmüştü: "MEMORY: fts5 not available").
4. Kök neden: `mattn/go-sqlite3`, FTS5'i **varsayılan olarak derlemiyor** — `-tags "sqlite_fts5"` gerekiyor. Repodaki HİÇBİR build komutu (CI'nin 4 workflow'u, `build_releases.sh`/`.bat`, `macrelease.sh`, `package_linux.sh`, `package_windows.sh`, hatta `upload-r2.yml`) bu tag'i hiç geçmiyordu. `internal/memory/store_test.go`'daki `TestHybridSearch_MatchTypeSet`'in kendi yorumu bile bunu "test ortamında fts5 yok" diye normalmiş gibi kabul ediyordu — yani bu, fark edilip düzeltilmemiş, uzun süredir var olan bir eksiklikti.
5. **Doğrulama (varsayım değil, gerçek test):** `-tags "sqlite_fts5"` ile ayrı bir binary derlendi, boş bir scratch dizininden çalıştırıldı. Log satırı `"MEMORY: fts5 not available (no such module: fts5)"`'ten `"MEMORY: FTS migration complete"`'e değişti — FTS5'in gerçekten aktifleştiğini kanıtladı.

## Sonuç: FTS5 hibrit arama hiçbir yayınlanmış sürümde hiç çalışmamış

Bu, "kırmızı" örneğinin ötesinde geniş kapsamlı bir bulgu: kısa, kesin anahtar kelime tabanlı gerçekler (favori renk gibi), çok konulu/birleşik bir soruyla karşılaştığında ya da hafıza deposu (haftalarca test session'ından birikmiş) büyüdükçe, salt vektör benzerliğiyle top-K dışında kalabiliyor — tam da FTS5'in var olma sebebi. Bu mekanizma zaten yazılmış ve test edilmişti, sadece hiç açılmamıştı.

## Yapılan fix

`-tags "sqlite_fts5"` şu dosyaların HEPSİNE eklendi:
- `.github/workflows/{ci,build-linux,build-macos,build-windows,upload-r2}.yml` (upload-r2.yml'de 3 ayrı yer)
- `build_releases.sh`, `build_releases.bat`, `macrelease.sh`, `package_linux.sh`, `package_windows.sh`
- `AGENTS.md` (Quick Start, Build, Verification Commands, Code Style bölümleri)
- `README.md`, `READmeTR.md`
- `docs/CGO_FLAGS.md` — yeni bir "FTS5 — ZORUNLU" bölümü eklendi, neden önemli olduğu detaylıca anlatıldı
- `.claude/skills/memo-release/SKILL.md`

## Doğrulama

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...             → temiz
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...                → temiz
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race -count=1 → tüm 34 paket yeşil
```

Gerçek bir binary derlenip boş bir data dizininden çalıştırılarak FTS5'in gerçekten aktifleştiği doğrulandı (bkz. yukarıdaki araştırma zinciri madde 5).

**Gerçek uygulamada test edilemeyen:** Kullanıcının bildirdiği TAM senaryonun (birleşik "adımı, doğum günümü ve rengimi biliyor musun" sorgusu) bu fix'ten sonra gerçekten doğru cevap verdiğini uçtan uca doğrulamak — bu ortamda mevcut gerçek hafıza veritabanı/embedding modeliyle canlı bir test yapılamadı. Fix, zaten var olan ve test edilmiş bir mekanizmayı açıyor, yeni retrieval mantığı eklemiyor — ama kullanıcının rebuild edip gerçek kullanımda tekrar denemesi asıl doğrulama olacak.

**Commit edilecek** (AGENTS.md kuralı gereği).

**Sıradaki oturum için:**
1. Kullanıcı `-tags "sqlite_fts5"` ile yeniden derleyip (CLI ve/veya GUI) aynı senaryoyu tekrar denemeli — özellikle birleşik/çok konulu sorular.
2. Fark edilen ama dokunulmayan, alakasız bir bug: `build_releases.sh`'ın macOS bölümünde `GOARCH=$(uname -m)` kullanılıyor — Intel Mac'lerde bu "x86_64" döner, ama Go'nun geçerli `GOARCH` değeri "amd64"tür ("arm64" Apple Silicon'da zaten doğru çıkıyor). Intel Mac'lerde bu satır muhtemelen build'i bozar. Bu oturumun kapsamı dışında bırakıldı, dokunulmadı.
3. `obsidian-doc/`, `obsidian-doc-en/`, `docs/plans/`, `docs/superpowers/` gibi ikincil/arşiv dokümanlardaki eski `go build`/`go test` örnekleri güncellenmedi — bilinçli olarak kapsam dışı bırakıldı (asıl CI/release/AGENTS.md/README önceliklendirildi).

---

# Handoff — 2026-07-14/15 (Session 27, devam 3) — Yeni özellik: "Hata Bildir" ayarlar sekmesi (telemetri tartışmasından çıktı)

## Özet

Kullanıcıyla telemetri konusunda uzun bir sohbet oldu: uygulama "0 telemetri, 0 veri" iddiasında, kullanıcı hata raporu/indirme sayısı/kullanım amacı gibi bir telemetri eklemeyi düşünüyordu ama bunun felsefeye ters düşüp şüphe yaratacağından endişeliydi. Tartışma sonucu: indirme sayıları zaten web sitesi tarafında (client'a dokunmadan) tutuluyor; hata raporları için opsiyonel ama tamamen **elle tetiklenen**, arka planda hiçbir şey göndermeyen bir yöntem kararlaştırıldı — GitHub Issues'a önceden doldurulmuş bir link açmak (kullanıcı kendi hesabıyla, kendi gözden geçirip gönderiyor). Kullanıcı sonra "kodlamaya başla, sabah bitmiş görmek istiyorum, soru sorma" dedi — bu yüzden fazla soru sormadan uygulandı.

## Yapılan

Yeni Settings sekmesi: `frontend/lib/widgets/settings/tabs/report_bug_tab.dart` (son sekme, "Hata Bildir"/"Report Bug", wrench.svg ikonu — özel bug ikonu asset'i yok, mevcut ikonlardan en yakını kullanıldı).

- Çok satırlı metin kutusu: "ne oldu, ne yapmaya çalışıyordun, ne bekliyordun" açıklaması.
- "Son 10 hatayı da ekle" checkbox'ı — **varsayılan kapalı**.
- "GitHub'da Bildir" butonu: `https://github.com/BugraAkdemir/memo/issues/new?title=...&body=...` linkini `url_launcher` ile tarayıcıda açar. Hiçbir şey bizim sunucumuza gitmez — kullanıcı GitHub'ın kendi sayfasında son kez görüp düzenleyip kendi hesabıyla gönderir.
- Ekran görüntüsü YOK (bilinçli — ekranda özel sohbet içeriği görünebilir).
- "Son 10 hata" için yeni bir takip sistemi eklenmedi — zaten var olan `internal/app`'in `eventRing`/`GetEvents()` (`GET /api/events`, önceden Flutter tarafından hiç kullanılmıyordu) tekrar kullanıldı. Yeni `MemoApiClient.getEvents()` eklendi, client-side `name` alanı "error" içeren event'ler filtrelenip son 10'u alınıyor.
- Yeni backend endpoint'i YOK — bilinçli tercih, "sıfır veri topluyoruz" iddiasını bozmamak için.

`settings_dialog.dart`'a sekme kaydı eklendi (case 16, mevcut desene uyularak). l10n.dart'a TR+EN 14 yeni key eklendi.

## Doğrulama

```
flutter analyze lib/   → temiz (bilinen 4 info-level use_build_context_synchronously)
flutter test           → 105/105 yeşil
```

**Gerçek uygulamada test edilemeyen:** Bu ortamda tarayıcı/display olmadığı için gerçek bir GitHub issue linkinin açılıp doğru doldurulduğunu gözle doğrulamak mümkün değildi. Widget test de eklenmedi (bu kod tabanındaki diğer settings sekmelerinin çoğunda da yok, ör. `memory_import_tab.dart` — gerçek kullanımla test edilmişti).

**Commit edilecek** (AGENTS.md kuralı gereği, bu handoff girişiyle birlikte).

**Bonus fix (fark edilen ayrı, alakasız bir bug):** `settings_dialog.dart`'ın `_tabIcons` listesinde Task Loop sekmesi `'lib/icon/slash/gears.svg'`ye işaret ediyordu ama gerçek dosya adı `gear.svg` (tekil, ve zaten General sekmesi tarafından kullanılıyor) — hiç var olmayan bir asset, muhtemelen kırık/boş görünüyordu. `list-checks.svg` ile değiştirildi (zaten kullanılmıyordu, "görev listesi" semantiğine de daha uygun).

**Sıradaki oturum için:**
1. Kullanıcı gerçek uygulamada "Hata Bildir" sekmesini deneyip GitHub linkinin doğru başlık/gövdeyle açıldığını ve Task Loop sekmesinin artık düzgün bir ikon gösterdiğini doğrulamalı.

---

# Handoff — 2026-07-14 (Session 27, devam 2) — Ayarlar sekmelerindeki hardcoded (dile duyarsız) metinler düzeltildi

## Özet

Kullanıcı: özellikle Ayarlar sekmesinde ve birkaç başka yerde, dil TR/EN değiştirilince değişmeyen hardcoded metinler olduğunu bildirdi, bulunup düzeltilmesini istedi.

## Bulgular

`frontend/lib/widgets/settings/tabs/*.dart`'ın hepsinde (11 dosya) birikmiş hardcoded `Text()`/`hintText`/`labelText`/`SnackBar` string'leri vardı — çoğu Türkçe, bazıları İngilizce, hiçbiri dil değiştirmeye tepki vermiyordu. İki farklı alt-sorun tipi çıktı:

1. **Yarım kalmış migrasyon:** `backup_restore_tab.dart` ve `learning_tab.dart`'ta l10n.dart'ta TAM ve DOĞRU TR+EN key'ler zaten vardı ama widget hâlâ ham Türkçe literal kullanıyordu — kimse gerçekten bağlamamış.
2. **Yanlış çeviri:** Bazı mevcut key'lerin (`add_provider`, `no_providers`, `add_provider_hint`, `connected`, `enable`, `disable`, `delete_provider`, `delete_provider_confirm` vb.) **Türkçe haritasında İngilizce metin** duruyordu — yani widget doğru şekilde `L10n.t()` çağırsa bile Türkçe modda İngilizce görünüyordu.
3. Diğer dosyalarda (`remote_access_tab.dart`, `mood_tab.dart`'ın rıza diyaloğu, `general_tab.dart`'ın CLI/kaldırma bölümü, `about_tab.dart`'ın gövde metni) hiç key yoktu, sıfırdan eklendi.

## Yapılan

11 dosyanın hepsi tek tek okunup yeniden yazıldı: `backup_restore_tab`, `learning_tab`, `mood_tab`, `remote_access_tab`, `providers_tab`, `gpu_config_tab`, `orchestra_tab`, `general_tab`, `memory_tab`, `skills_tab`, `about_tab`. ~150+ string düzeltildi, l10n.dart'a onlarca yeni key (TR+EN) eklendi, birkaç yanlış çevrilmiş key düzeltildi. `mood_tab.dart`'ın 3 aşamalı sistem-yönetimi onay diyaloglarında widget'ın GERÇEK (daha uzun) metniyle l10n key'lerinin (daha kısa, farklı) metni birbirinden kaymıştı — key'ler widget'ın gerçek içeriğine göre güncellendi, davranış değişmedi.

Bilinçli olarak dokunulmadı: dil-bağımsız placeholder/örnek değerler (`hintText: 'tskey-auth-...'`, `'http://127.0.0.1:8090'`) ve "Client ID" gibi teknik terimler.

## Doğrulama

```
flutter analyze lib/   → temiz (bilinen 4 info-level use_build_context_synchronously)
flutter test           → 105/105 yeşil, değişmedi
```

**Commit:** `fe872eb` (AGENTS.md kuralı gereği otomatik atıldı).

**Kapsam dışı bırakılan (kullanıcı onayıyla):** Aynı sınıf bug'ın settings dışında da olduğu doğrulandı (grep ile) — `agent_screen.dart`, `app_shell.dart`, `chat_input.dart`, `chat_message_list.dart`, `permission_dialog.dart`, `permission_history.dart`, `orchestra_config_dialog.dart`, `provider_config_dialog.dart`, `skill_config_dialog.dart`. Kullanıcıya sorulduğunda "şimdilik bu kadar yeter" dedi — bu dosyalar dokunulmadan bırakıldı, ileride istenirse aynı yöntemle (önce l10n.dart'ta mevcut key var mı kontrol et, yoksa TR+EN ekle, widget'ı `L10n.t()`'a bağla) devam edilebilir.

---

# Handoff — 2026-07-14 (Session 27, devam) — Bug #9: Memo, hep-açık takvim/hatırlatma özelliğini bilmiyor, sorulunca inkar ediyordu

## Özet

Kullanıcı ekran görüntüsü: "yarın saat 1pm'de toplantım var" dedi, Memo doğru şekilde arka planda hatırlatıcı kurdu (takvime ekleme sorunsuz çalışıyor, kullanıcı bunu doğruladı) — ama kullanıcı "olur öyle bir yeteneğin var mı" diye sorunca, Memo "otomatik hatırlatma kurma gibi bir sistemim yok" dedi ve kullanıcıya kendi alarmını kurmasını önerdi. Tam da o mesaj için arka planda gerçek bir hatırlatıcı kurulurken.

## Kök neden

`internal/app/chat.go`'daki `processMessageIntent` her mesajda koşulsuz çalışıyor (config'te bir toggle yok) ve takvim-değerli planları algılayıp otomatik hatırlatıcı kuruyor — ama `internal/identity/identity.go`'nun system prompt'unda bu konuda hiçbir bilgi yoktu. 2026-07-12'deki `buildCapabilitiesBlock` fix'i (agent modu/web arama için "kapalıyken inkar etme" sorununu çözen) sadece TOGGLE'I OLAN özellikleri kapsıyordu; takvim özelliğinin hiç toggle'ı yok, hep açık — o yüzden o fix'in kapsamı dışında kalmıştı.

## Fix

Yeni `buildPassiveFeaturesBlock()` (`identity.go`) — hep-açık takvim/hatırlatma özelliğini açıkça belirtiyor, `buildOriginBlock`'un hemen ardına, aynı gating (MinimalMode dışında koşulsuz, `CustomRole`'dan bağımsız) ile ekleniyor. Testler: `TestBuildSystemPrompt_MentionsPassiveReminderFeature`, `TestBuildSystemPrompt_MinimalMode_OmitsPassiveFeaturesBlock`.

## Doğrulama

```
CGO_ENABLED=1 go build/vet/test ./... -race -count=1   → tüm 34 paket yeşil
```

**Gerçek uygulamada test edilemeyen:** GUI'de gerçekten "böyle bir yeteneğin var mı" diye sorup modelin artık doğru cevap verdiğini gözle doğrulamak (input-automation/display kısıtı). Statik doğrulama: yeni prompt bloğu test edildi, içeriği net ve doğru.

**Commit:** AGENTS.md kuralı gereği otomatik atıldı.

**Sıradaki oturum için:**
1. Kullanıcı gerçek uygulamada tekrar sorup Memo'nun artık "evet, arka planda otomatik hatırlatıcı kurabiliyorum" gibi doğru bir cevap verdiğini doğrulamalı.
2. Aynı desenin başka hep-açık arka plan özellikleri için de geçerli olup olmadığı düşünülebilir (ör. proaktif öneri motoru — ama o zaten `cfg.Proactive` ile varsayılan kapalı, bu kategoriye girmiyor).

---

# Handoff — 2026-07-14 (Session 27) — Bug #8: Agent modu her dış provider'da 400 veriyordu (leaked `danger` alanı) + ham JSON hata mesajları

## Özet

Kullanıcı ekran görüntüsüyle bildirdi: Agent Chat'te (agent modu açık) bir mesaj gönderince hata: `all providers failed: [opencode-zen] status 400: {"error":{"message":"Error from provider (Console): Upstream request failed",...}}`. Düz sohbet (agent modu kapalı) aynı provider'la sorunsuz çalışıyordu — demek ki fark tam olarak "tool/araç kullanımı" ile ilgiliydi. Kullanıcı ayrıca hata mesajının kullanıcı için anlaşılmaz olduğunu da belirtti.

## Kök neden

`internal/provider/provider.go`'daki `ToolDefinition` struct'ı bir `Danger string` alanı taşıyordu (`json:"danger,omitempty"`), `internal/agent/tools.go`'un `ToOpenAITools()`'u bunu agent'ın iç `DangerLevel`'ından kopyalıyordu. Ama bu struct, `internal/provider/openai.go`'nun **dış provider'a gönderilen ham JSON isteğinin bizzat kendisi** (`openAIChatRequest.Tools []ToolDefinition`) — yani her tool tanımı, standart OpenAI tool-calling şemasının (`{type, function}`) yanında **standart olmayan bir `"danger"` alanı** taşıyarak provider'a gidiyordu. Bu alan kod tabanında hiçbir yerde okunmuyordu (izin kontrolleri ayrı, tamamen iç `agent.ToolDef.DangerLevel`'ı kullanıyor) — yani tek işlevi provider'ın gerçek API'sine sızmaktı. Bazı provider'lar bilinmeyen alanları yok sayıyor, OpenCode Zen'in gateway'i ise katı şema doğrulaması yapıp reddediyor.

## Fix 1: leaked `danger` alanı

`provider.ToolDefinition`'dan `Danger` alanı tamamen kaldırıldı. Regresyon testi `TestToOpenAITools_OnlyStandardFields` (`internal/agent/tools_test.go`) — tüm built-in tool'ların wire temsilini JSON'a çevirip her objede sadece `"type"`/`"function"` anahtarları olduğunu doğruluyor. Fix'ten önce **18 tool'un hepsinde** `"danger"` alanı sızdığını kanıtlayarak fail etti.

## Fix 2: ham JSON hata mesajları

Kullanıcının ikinci şikayeti: hata mesajı `{"error":{"message":"...","type":"...","param":null,"code":"..."}}` gibi ham bir JSON blob'u — normal bir kullanıcı bundan bir şey anlayamaz. `internal/provider/openai.go`/`claude.go`/`gemini.go`'nun üçü de `parseError`'da HTTP body'sini olduğu gibi hata mesajına gömüyordu.

Yeni `provider.ExtractErrorMessage(body []byte) string` (`provider.go`) — OpenAI-uyumlu/Claude/Gemini API'lerinin ortak kullandığı `{"error": {"message": "..."}}` şeklini çözüp sadece asıl mesajı çıkarıyor, şekil uymuyorsa ya da mesaj boşsa ham body'ye düşüyor. Üç provider'ın `parseError`'ına da bağlandı. Testler: `TestExtractErrorMessage_UnwrapsTheRealMessage`/`_FallsBackToRawBody`/`_FallsBackWhenMessageEmpty` (`internal/provider/error_message_test.go`).

## Doğrulama

```
CGO_ENABLED=1 go build/vet/test ./... -race -count=1   → tüm 34 paket yeşil (yeni testler dahil)
```

**Gerçek uygulamada test edilemeyen:** Bu ortamda GUI'yi gerçekten çalıştırıp OpenCode Zen'e karşı agent modunda gerçek bir tool-call isteği atıp 400'ün gerçekten gitmediğini gözle doğrulamak (input-automation/display kısıtı, önceki oturumlarla aynı sebep). Statik/birim test kanıtı güçlü: leak'in kaynağı net (tek yazma noktası, hiç okuma yok), fix mekanik ve düşük riskli.

**Commit:** AGENTS.md kuralı gereği otomatik atıldı.

**Sıradaki oturum için:**
1. Kullanıcı gerçek uygulamada agent modunda OpenCode Zen (ya da başka bir dış provider) ile tekrar test edip 400 hatasının gittiğini doğrulamalı.
2. Hata mesajı hâlâ `"all providers failed: [opencode-zen] status 400: <mesaj>"` gibi bir prefix taşıyor — tamamen kullanıcı dostu bir cümleye çevrilmedi, sadece ham JSON gürültüsü temizlendi. İstenirse `internal/app/llm.go`'daki hata gösterim zincirine daha kapsamlı bir "kullanıcı dostu mesaj" katmanı eklenebilir.

---

# Handoff — 2026-07-14 (Session 26, devam 3) — Bug #7'nin GERÇEK kök nedeni: `_disposed` boolean, `build()`'da hiç resetlenmiyormuş

## Özet

Bug #7'nin (re-entrancy) fix'i de yetmedi — kullanıcı gerçek bir `flutter run -d linux` oturumunda **ilk mesajda** aynı şekilde takıldı. `backend.log` o oturumda her turun tertemiz tamamlandığını gösterdi (tek istek, `chat:done`, `memory:saved`), ve CLI (`internal/replcli`, aynı `/api/send/stream` endpoint'ini kullanıyor) hiç bu sorunu üretmiyor — bu, sorunu tamamen Flutter'ın kendi state katmanına indirgedi.

## Kanıt toplama: geçici `[SEND-DEBUG]` logları

`chat_provider.dart` ve `chat_input.dart`'a geçici `debugPrint('[SEND-DEBUG] ...')` satırları eklendi (`MessagesNotifier.sendMessage()`'ın giriş/claim/finally noktaları, `build()`/`onDispose`, `ChatInput.build()`'ın `isSending` değeri). Kullanıcı `flutter run -d linux`'ı çalıştırıp gerçek bir mesaj gönderdi, log'u yapıştırdı. Kritik satır:

```
[SEND-DEBUG] enter sendMessage instance=139305540 msg=merhaba isSending=false disposed=true
```

**Daha `sendMessage()` başlarken bile `disposed=true`!** Log'un tam sırası: `build()` → `onDispose` → `build()` (aynı instance ID) → hiç ikinci bir `onDispose` olmadan → `sendMessage()` `disposed=true` görüyor.

## Kök neden

Riverpod, `messagesProvider` invalidate edilip hemen tekrar okunduğunda (bir listener aktifken — gerçek uygulamada `ChatScreen`'in kalıcı `ref.watch(messagesProvider)`'ı bunu tetikliyor), **aynı `MessagesNotifier` nesnesini** tekrar kullanıyor (yeni bir nesne değil). Bu dispose+rebuild döngüsü, uygulama açılışında, kullanıcı hiçbir şey yapmadan **otomatik olarak bir kez** oluyor (kesin tetikleyicisi tam olarak bulunmadı, ama fix için gerekli değildi). `_disposed` alanı sadece field initializer'da (`bool _disposed = false;`) bir kere set ediliyordu, `build()` içinde hiç resetlenmiyordu — yani bu ilk zararsız görünen dispose+rebuild'den sonra `_disposed` **oturumun geri kalanı boyunca kalıcı olarak `true`** kalıyordu. Her `sendMessage()`'ın `finally` bloğundaki `if (!_disposed) { isSendingProvider = false; }` koruması (BUG-H2 için eklenmişti) bu yüzden bir daha hiç çalışmıyordu — buton her turda, her provider'da sonsuza dek "durdur"da takılı kalıyordu.

## İlk yanlış fix denemesi (ve neden yanlış olduğu)

İlk deneme: `build()`'ın başında `_disposed = false;` resetlemek. Bu, **var olan** `messages_notifier_dispose_test.dart` (BUG-H2) regresyon testini bozdu — çünkü nesne tekrar kullanıldığı için, bu reset aynı zamanda BAŞKA bir chat'ten kalan, hâlâ çalışan, terk edilmiş bir stream'i de "dispose değil" gibi gösteriyor, o da paylaşılan `isSendingProvider`'ı yanlış bir şekilde bozabiliyordu (tam olarak BUG-H2'nin önlemeye çalıştığı şey).

## Doğru fix: generation sayacı

`bool _disposed` → `int _generation` (her `build()` çağrısında ++). `sendMessage()`/`sendFile()`/`refresh()` kendi başladıkları anda `final myGeneration = _generation;` yakalıyor, ve daha önce `_disposed` kontrol edilen her yerde `_generation == myGeneration` / `!=` karşılaştırması yapıyor. Bir çağrı sadece KENDİ kuşağı hâlâ güncelken paylaşılan state'e dokunuyor — nesne tekrar kullanılsa da kullanılmasa da, herhangi bir sonraki `build()` eski çağrıları doğru şekilde geçersiz kılıyor, yeni başlayan çağrılar ise yeni kuşağı yakalayıp normal çalışıyor.

## Test kanıtı

`messages_notifier_stale_disposed_flag_test.dart` (yeni): canlı bir listener ile `container.invalidate(messagesProvider)` + hemen `read()` yaparak gerçek dispose+rebuild döngüsünü zorluyor, sonra `sendMessage()`'ın `isSendingProvider`'ı yine de `false`'a resetlediğini doğruluyor. **Hem orijinal koda hem de "build()'da reset" ilk denemesine karşı fail ediyor**, generation-counter fix'ine karşı geçiyor. Aynı anda `messages_notifier_dispose_test.dart` (BUG-H2) ve `messages_notifier_reentrancy_test.dart` da geçmeye devam ediyor — üçü birlikte, birbirini bozmadan.

Geçici `[SEND-DEBUG]` logları (2 ayrı debug commit'i) tamamen temizlendi.

## Doğrulama

```
flutter analyze lib/   → temiz (bilinen 4 info-level use_build_context_synchronously)
flutter test           → 105/105 yeşil
```

**Gerçek uygulamada test edilemeyen:** Bu ortamda GUI'yi gerçekten çalıştırıp doğrulamak mümkün değildi (display/input-automation kısıtı) — fix, kullanıcının kendi gerçek `flutter run` oturumundan aldığı canlı log kanıtına dayanarak yapıldı ve birim testleriyle doğrulandı, ama fix SONRASI gerçek bir retest henüz yok.

**Commit:** AGENTS.md kuralı gereği otomatik atıldı.

**Sıradaki oturum için:**
1. Kullanıcı gerçek uygulamada tekrar test etmeli — bu sefer gerçekten "artık hiç takılmıyor" diyebilecek mi görmemiz lazım.
2. `MessagesNotifier`'ın neden açılışta bir kez otomatik dispose+rebuild olduğu hâlâ tam olarak bilinmiyor (belirtilmedi, fix'in gerekliliğini etkilemiyor ama merak konusu olarak kalabilir — `ActiveChatIdNotifier.build()`'ın async `getActiveChatId()` çözümlenmesiyle ilgili bir zamanlama olabilir).
3. `chat_input.dart`'ın `_sendWhatsApp`'ındaki status/usage chunk filtreleme eksikliği hâlâ açık, düzeltilmedi.

---

# Handoff — 2026-07-14 (Session 26, devam 2) — Bug #7: "durdur" ikonu takılması — asıl kök neden bulundu (Bug #5'in fix'i yetersizmiş)

## Özet

Bug #5'in fix'inden sonra kullanıcı `flutter run` ile (taze kod, stale binary değil) aynı hatayı tekrar üretti — hızlı/kısa yanıtlarda, her provider'da. Bu, Bug #5'in (300s ctxDone) bu spesifik semptomun kök nedeni OLMADIĞINI kanıtladı. Kullanıcı `/codebase-memory` ve `/code-review` ile detaylı analiz istedi.

## Gerçek kök neden

`frontend/lib/providers/chat_provider.dart`'ın `MessagesNotifier.sendMessage()`'ı: guard kontrolü (`if (isSendingProvider) return;`) ile asıl `isSendingProvider = true` ataması arasında `await _handleMemoryCommand(...)` var — **atomik değil**. İki `sendMessage()` çağrısı art arda (çift Enter basışı, Enter'ı basılı tutunca OS'un ürettiği tuş tekrarı, ya da gönder butonuna çift tık — `chat_input.dart`'ın `_handleKeyEvent`'i Enter'ı direkt `_send()`'e bağlıyor) bu boşluktan ikisi de `isSendingProvider`'ı `false` görüp geçebiliyor: iki ayrı HTTP isteği, paylaşılan `_cancelToken` alanının üzerine yazılması, aynı gönderim için iki kullanıcı balonu. Bu tamamen frontend'de, herhangi bir provider'a bağlı olmadan tetikleniyor — "her provider'da aynı sorun" gözlemiyle birebir örtüşüyor.

## Doğrulama süreci

Önce `frontend/test/providers/messages_notifier_reentrancy_test.dart` yazıldı: `sendMessage()`'ı arada `await` olmadan iki kez çağırıp kaç HTTP isteği gittiğini sayıyor. Fix'ten ÖNCE çalıştırıldı → **fail etti (2 istek)**, bug'ı kanıtladı. Sonra fix uygulandı (`isSendingProvider` claim'i guard'dan hemen sonra, `await`'ten önce, senkron olarak taşındı; `_handleMemoryCommand` gerçekten bir komutu işlerse claim geri `false`'a alınıyor). Test tekrar çalıştırıldı → **geçti (1 istek)**.

## Bu oturumda ayrıca elenen teoriler (gerçek kod okuması ile, spekülasyon değil)

- `internal/app/chat.go`'daki `forwardStream` — select/ctx-done deseni doğru, sızıntı yok.
- `a.streamMu` global kilidi — her `TryLock()`'un `defer Unlock()`'u var, panic bile `close(outCh)`'tan önce `recoverStreamPanic` ile yakalanıyor.
- Dart `stream.timeout()`'un `onTimeout`'ta `sink.close()` çağırmaması — ilk bakışta şüpheli ama `await for` bir error event'i gelince zaten throw edip çıkıyor, güvenli.
- `isSendingProvider`'a yazılan her nokta (7 yer) tek tek listelenip kontrol edildi — sahipsiz `true` yok.
- Yan bulgu (asıl bug'la alakasız, ayrı gerçek bug): `chat_input.dart`'taki `_sendWhatsApp`, `finishReason == 'status'`/`'usage'` chunk'larını filtrelemiyor — bu chunk'ların JSON içeriği yanıt metnine karışabilir. Düzeltilmedi, kapsam dışı bırakıldı (kullanıcıya bildirildi).

## Doğrulama

```
flutter analyze lib/providers/chat_provider.dart   → temiz
flutter test                                        → 104/104 yeşil (yeni test dahil)
```

**Gerçek uygulamada test edilemeyen:** Bu ortamda gerçek bir çift Enter/tuş-tekrarı senaryosunu GUI'de fiilen tetikleyip görsel olarak doğrulamak (input-automation/display kısıtı, önceki oturumlarla aynı sebep). Birim testi kanıtı güçlü (fix öncesi fail, fix sonrası geç), ama gerçek kullanıcı testi bekliyor.

**Commit:** AGENTS.md kuralı gereği otomatik atıldı (bkz. commit log).

**Sıradaki oturum için:**
1. Kullanıcı gerçek uygulamada tekrar test edip "durdur" ikonu takılmasının artık olmadığını doğrulamalı — özellikle Enter'a hızlı basarak/basılı tutarak deneyerek.
2. `chat_input.dart`'ın `_sendWhatsApp`'ındaki status/usage chunk filtreleme eksikliği hâlâ açık, düzeltilmedi.
3. `sendFile()` ve `_sendWhatsApp()` zaten claim'i await'ten önce/hemen sonra yapıyordu (bu race'e sahip değillerdi) — sadece `sendMessage()` etkilenmişti, ama gelecekte bu iki fonksiyona benzer bir await eklenirse aynı deseni tekrarlamamak gerekir.

---

# Handoff — 2026-07-14 (Session 26, devam) — Bug #6: CLI sohbetleri GUI'nin "Sohbetler" listesinde görünmüyordu

## Özet

Kullanıcı: "CLI'de yapılan chat geçmişini GUI'da göremiyorum". Kök neden: `internal/replcli/repl.go`'daki `startFreshChat()` her zaman `NewAgentChat(s.projectPath)` çağırıyor — agent modu hiç kullanılmasa bile. Backend'in `IsAgentChat` ve frontend'in `ChatSession.isAgentChat`'i salt "project_path dolu mu"ya bakıyor, CLI her zaman cwd'yi project_path olarak geçtiği için **her** CLI sohbeti `isAgentChat=true` oluyor. `chat_sidebar.dart`'ın "Sohbetler" listesi bunları bilerek dışlıyordu (`!c.isAgentChat`), sadece "Ajan" sekmesinde görünüyorlardı.

Gerçek veriyle doğrulandı: `~/.memo/data/sessions/`'daki 13 sohbetten 11'i CLI'den (project_path = `/home/bugra`), başlıkları "Kanka Muhabbeti", "Hobi Sohbeti" gibi sıradan sohbetler — agent/tool kullanımıyla alakasız, ama "Ajan" sekmesine hapsolmuşlar.

## Düzeltme

`frontend/lib/widgets/chat_sidebar.dart` — "Sohbetler" listesi artık `isAgentChat` filtresi uygulamıyor, tüm sohbetleri gösteriyor. "Ajan" sekmesi (`agent_screen.dart`) dokunulmadı, kendi filtreli görünümünü korumaya devam ediyor — bir sohbet artık her ikisinde de görünebilir. `ChatScreen` zaten `agent_event` badge'lerini render ediyor, o yüzden eski bir CLI sohbeti açıldığında tool aktivitesi doğru görünüyor.

## Doğrulama

```
flutter analyze lib/widgets/chat_sidebar.dart   → temiz (bilinen tek info uyarısı)
flutter test                                     → 103/103 yeşil
```

**Gerçek uygulamada test edilemeyen:** Bu ortamda GUI'yi gerçekten açıp "Sohbetler" listesinde CLI sohbetlerinin göründüğünü gözle doğrulamak (input-automation/display kısıtı, önceki oturumlarla aynı). Statik olarak: filtre kaldırıldı, `chats` listesi doğrudan kullanılıyor, `chatListProvider` zaten backend'in tüm sohbetlerini (project_path farketmeksizin) döndürüyor — mantık zinciri eksiksiz.

**Commit edilmedi.**

---

# Handoff — 2026-07-14 (Session 26) — Bug #5: "durdur" ikonu takılı kalıyordu, cevap sessizce kesiliyordu

## Oturum Özeti

Kullanıcı ekran görüntüsüyle bildirdi: cevap geliyor ama token'lar kesiliyor, gönder/durdur butonu "durdur" (kırmızı kare) ikonunda takılı kalıyor. Ekran görüntüsündeki aktif provider "OpenCode Zen" (dış sağlayıcı) idi. AGENTS.md'de bu tam semptomun 2026-07-12'de zaten düzeltildiği yazıyordu (SSE `Done:true` chunk'ının `ctx.Done()` ile yarışıp düşürülmesi) — ama kurulu binary bugünkü tarihli (00:14) ve bu fix çoktan içindeydi, yani gerçekten farklı bir kaynak aranması gerekiyordu.

## Kök neden

`internal/app/llm.go`'da `recvChunk`'ı tüketen üç döngünün de (`callAgentStream`, `callLLMStream`'in external-provider döngüsü, aynı fonksiyonun local-model döngüsü) `ctxDone` dalı — 2026-07-12'nin kapsamadığı ayrı bir durum: kanalda gerçekten hiçbir şey hazır değilken `ctx` gerçekten sona eriyor (300s generation bütçesi doldu ya da istemci bağlantıyı kesti) — sadece `a.recordStreamError(...)` (sadece session'a yazılıyor, akışa değil) çağırıp `return` ediyordu. Dosyadaki **her** başka hata/dönüş yolu `trySend(...Done:true)` çağırıp öyle dönerken, bu tek dal hiç çağırmıyordu. Sonuç: `outCh` sessizce kapanıyor, istemciye ne bir hata ne bir "durduruldu" mesajı ulaşıyor — kullanıcı sadece cevabın ortasında kesildiğini görüyor, açıklama yok.

Bu, halihazırda dokümante edilmiş "chunk'ın select yarışında düşmesi" bug'ından **farklı ve ayrı** bir gerçek kod yolu eksikliği — o fix zaten doğruydu ama bu üç `ctxDone` dalını kapsamıyordu.

## Düzeltme

`internal/app/llm.go`'da üç noktaya da (satır ~172, ~671, ~777) `return`'den önce `trySend(ctx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})` eklendi — dosyadaki diğer tüm hata dallarıyla aynı desen.

**Regresyon testi:** `TestCallLLMStream_ExternalProvider_CtxDoneSendsTerminalChunk` (`internal/app/llm_test.go`) — `httptest` ile sahte bir OpenAI-uyumlu SSE sunucusu kuruluyor, bir parça içerik gönderip bağlantıyı `ctx` iptal edilene kadar açık tutuyor (gerçek bir zaman aşımı/kopan bağlantı senaryosunu simüle ediyor), sonra `outCh`'nin kapanmadan önce mutlaka `Done:true` + `Error` dolu bir terminal chunk yaydığını doğruluyor. Fix'ten önceki koda karşı `git stash` ile manuel doğrulandı — gerçekten fail ediyor.

## Doğrulama

```
CGO_ENABLED=1 go build ./...              → temiz
CGO_ENABLED=1 go vet ./...                → temiz
CGO_ENABLED=1 go test ./... -race -count=1 → tüm 34 paket yeşil (yeni test dahil)
```

**Gerçek uygulamada test edilemeyen:** Bu ortamda GUI'yi gerçekten çalıştırıp "OpenCode Zen" ile 300s'lik bir gecikmeyi tetiklemek pratik değildi — fix, kod yolunun statik analiziyle (ekran görüntüsündeki aktif provider + backend log'larındaki gerçek session geçmişi) izole edilip `httptest` ile izole bir birim testinde doğrulandı, gerçek bir canlı OpenCode Zen isteğine karşı değil.

**Commit edilmedi** — kullanıcı henüz commit istemedi.

**Sıradaki oturum için:**
1. Kullanıcı gerçek uygulamada tekrar aynı senaryoyu (yavaş/uzun bir dış provider cevabı) deneyip artık "⏹️ Cevap durduruldu." mesajının canlı olarak sohbette göründüğünü doğrulamalı.
2. Eğer sorun hâlâ tekrarlanırsa, bir sonraki şüpheli: frontend'in `errorMessageProvider`'ının bu mesajı gerçekten bir banner/snackbar olarak gösterip göstermediği (memory-import özelliğinde daha önce yakalanan SnackBar-arkasında-modal deseniyle karşılaştırılmalı).

---



## Oturum Özeti

Kullanıcı yeni bir özellik istedi: Ayarlar'da, kullanıcının ChatGPT/Gemini/Claude gibi başka AI'lara verebileceği bir prompt üretilsin, kullanıcı o AI'nin cevabını yapıştırsın, Memo bu bilgiyi profesyonelce parçalayıp hafızaya işlesin — ayrıca konuşma tarzı bilgisi de öğrenilip kullanıcıya özel system promptuna yansısın. Netleştirme turunda kullanıcı: yapısallaştırma için aktif LLM kullanılsın (kural tabanlı bölme değil), çıktı formatı bana bırakıldı (JSON seçildi). Özellik ilk yazıldıktan sonra kullanıcı gerçekten Gemini ve ChatGPT'ye karşı test etti, sonra REPL CLI'da ayrı bir gerçek bug daha buldu — oturum, ilk build + 3 ayrı gerçek-dünya bug'ının bulunup düzeltilmesi şeklinde ilerledi. 13 commit, hepsi local (henüz push edilmedi).

## 1) İlk build (backend + frontend, Memory tab'a gömülü)

**Backend:**
- `internal/config/config.go` — `IdentityConfig.LearnedStyleNotes string` (config.yaml'da kalıcı).
- `internal/identity/identity.go` — `Identity.LearnedStyleNotes` + `SetLearnedStyleNotes`/`GetLearnedStyleNotes` (mutex korumalı, `MinimalMode` ile aynı desen). `BuildSystemPrompt`'ta stil talimatlarından hemen sonra additive enjekte ediliyor — `CustomRole`'ü değiştirmiyor, wizard persona altında da çalışıyor (origin block ile aynı gerekçe). `MinimalMode`'da kesiliyor.
- `internal/app/app.go` `Startup()` — `a.identity.SetLearnedStyleNotes(cfg.Identity.LearnedStyleNotes)` kalıcı notu her açılışta yükler.
- `internal/app/memory_import.go` (yeni dosya) — `ImportMemoryFromText(ctx, rawText) (factsSaved int, styleUpdated bool, err error)`. `extractJSON` (intent/orchestra/taskloop/proactive ile aynı desen — paket-özel private kopya).
- `internal/webserver/bridge.go`/`handlers_flutter.go`/`server.go` — `POST /api/memory/import-text`.

**Frontend (ilk hali):** `api_client.dart`'a `importMemoryFromText`, Memory tab'ın içine bir alt bölüm (prompt + yapıştırma kutusu + buton), `l10n.dart`'a yeni key'ler.

Commit'ler: `ff30a22`(*), `f017f21`, `e92905a`, `69cce8d`.

## 2) Bug #1 (kullanıcı buldu, koddan bağımsız bir keşif): `SaveExplicit` hiçbir zaman çalışmamış

Backend'i gerçek bir çalışan instance ile test ederken (`/api/memory/explicit/save` canlı çağrısı), `internal/memory/store.go`'daki `SaveExplicit`'in INSERT'inin VALUES tuple'ı 16 öğe listeliyordu ama kolon listesi 15'ti (chunk_index'in literal `0`'ından önce fazladan bir `?`). SQLite bunu "16 values for 15 columns" hatasıyla reddediyor — yani **`/remember` komutu (ve şimdi yeni özellik) hiçbir zaman gerçekten hafızaya kaydetmemiş**, sessizce başarısız oluyormuş. `SaveExplicit`'in sıfır test coverage'ı vardı, hiç yakalanmamıştı. Düzeltildi + `TestSaveExplicit` regresyon testi eklendi. Commit: `ff30a22`.

## 3) Gerçek dünya testi: kullanıcı prompt'u Gemini + ChatGPT'ye verdi

İlk prompt (basit, tek paragraf) Gemini'de neredeyse boş sonuç verdi. Kullanıcı Gemini'nin **kendi gerçek** "Hafızayı Gemini'a aktarın" ayarlar sayfasından bir örnek prompt paylaştı (3. şahıs anlatım, kategorili, kanıt alıntılı, "Source: X" ile biten). Prompt o örnek temel alınarak **tamamen İngilizce, her zaman** (uygulama dili ne olursa olsun) yeniden yazıldı: 6 kategori (Demographics, Interests & Preferences, Relationships, Dated Events, **Communication Style & Personality** — bu kategori Gemini'nin orijinalinde yok, özellikle bu özellik için eklendi —, Instructions), her girişte Evidence/Basis satırı, "Source: <name>" ile bitiş.

İlk redesign sonrası tekrar test edildiğinde: **Gemini yine zayıf sonuç verdi** (sadece 1 talimat + 1 demografik bilgi), **ChatGPT aynı prompt'la çok zengin, 6 kategorinin tamamını dolduran bir profil üretti**. Sonuç: prompt'un kendisi çalışıyor, Gemini'nin zayıflığı Gemini'nin kendi sınırlaması (muhtemelen tam konuşma geçmişini taramıyor), prompt sorunu değil.

Backend'in yapılandırma promptu (`importMemorySystemPrompt`) bu kategorili/Evidence-Basis/Source formatını tanıyacak şekilde güncellendi: kategori 1-4'ü ayrı fact'lere, 5+6'yı (kişilik + davranış talimatları, ör. "her zaman neden açıklaması ekle" gibi standing instruction'lar) tek bir `style_summary`'ye katlıyor — çünkü style_summary her yanıta enjekte ediliyor, fact'ler gibi olasılıksal aranmıyor.

Commit'ler: `d72abe1`, `a5c4a4b`.

## 4) Kullanıcı isteği: ayrı sayfa + tasarım + kapsam daraltma

Kullanıcı Gemini'nin gerçek ayarlar sayfasının ekran görüntüsünü verdi ("bu tarz renklere uygun bir tasarım istiyorum"), 3 şey istedi: (a) bu tasarıma benzer bir sayfa, (b) Memory tab'ın **içinde değil, ayrı bir Settings sayfası** olarak, (c) "önce biraz sohbet et" uyarısı, (d) Gemini'nin sayfasındaki "Sohbetleri içe aktarın" (.zip conversation import) kısmını **yapma**.

- Yeni `frontend/lib/widgets/settings/tabs/memory_import_tab.dart` — numaralı adım rozetleri (①②), kart yerleşimi, pill butonlar, `translate.svg` ikonu. `memory_tab.dart`'tan bölüm tamamen çıkarıldı.
- `settings_dialog.dart`'a yeni tab kaydı (Memory'nin hemen sağı, index 4, sonraki tüm case'ler bir kaydırıldı).
- `warningOrange` tonlu tip banner: "önce biraz sohbet etmiş olman gerekiyor, yoksa zayıf sonuç dönebilir."
- Conversation-import (.zip) kısmı bilinçli olarak yapılmadı.

Commit'ler: `1d92b73`, `a5c4a4b`.

## 5) Bug #2 (kullanıcı buldu): local-only kurulumda özellik hiç çalışmıyormuş + timeout'a kadar bekliyormuş

`ImportMemoryFromText` doğrudan `a.providerRouter.ChatCompletion`'a gidiyordu — `callLLM`'in gerçek yönlendirme zincirini (Orchestra → dış provider → yerel model, `internal/app/llm.go`) tamamen atlıyordu. Sonuç: (1) sadece yerel model çalışıyorsa özellik hiç çalışmıyordu (yerel modeli hiç denemiyordu), (2) hiçbir şey bağlı değilse canlı bir ağ isteği yapıp kendi 90s timeout'una çarpana kadar sessizce bekliyordu, kullanıcıya hiçbir şey söylemeden.

Düzeltme: artık `a.callLLM(ctx, msgs)` kullanıyor (bu paketteki `updateMoodAsync`/`buildLearningDecider` ile aynı desen) — doğru yönlendirme bedavaya geliyor, ve callLLM'in zaten var olan anlık "⚠️ Yerel model yüklenmemiş..." mesajı (`isLLMErrorReply` ile tespit edilip Go error'a çevriliyor) hemen dönüyor, artık timeout yok. Regresyon testi: `TestImportMemoryFromTextNoModelFailsFastWithClearMessage`.

Commit'ler: `20b3bd9`, `a82214f`.

## 6) Bug #3 (kullanıcı buldu): "içe aktardığımda hiçbir tepki yok"

`MemoryImportTab`, `SettingsDialog`'un modal `Dialog`'unun içinde — bu dialog, uygulamanın kök `Scaffold`'unun overlay yığınında üstünde duruyor. `ScaffoldMessenger.of(context).showSnackBar(...)` kök Scaffold'a bağlanıyor, yani snackbar dialog'un **arkasında** render ediliyor — Settings açıkken hiç görünmüyor. Hem "Hafızaya İşle" hem "Prompt'u Kopyala" bunu kullanıyordu.

Düzeltme: ikisi de artık sonucu sayfanın kendi içinde inline bir `_StatusBanner` (yeşil ✓ başarı / kırmızı ⚠ hata) olarak gösteriyor, modal'dan bağımsız her zaman görünür. **Not:** Bu SnackBar-arkasında-kalma sorunu muhtemelen Settings'teki diğer sekmelerde de var (dokunulmadı, kapsam dışı bırakıldı — sadece bu özellik için düzeltildi).

Commit'ler: `5be26ca`, `4fc4aa9`.

## 7) Bug #4 (kullanıcı buldu, farklı bir yerde — terminal REPL): embedding hatası hiç gösterilmiyormuş

Kullanıcı terminal REPL'i (`internal/replcli`) test ederken şunu fark etti: karşılama bannerındaki "Hafıza:" satırı embedding gerçekten çalışmasa bile hep "açık" görünüyor, ve mesaj başına "✓ hafıza kaydedildi" bazen hiç yazmıyor ama neden yazmadığı hiç açıklanmıyor. Araştırma: backend zaten (`autoStartEmbeddingModel`/`startupEmbeddingModel`/`saveMemorySync`, `internal/app/llama.go`+`embedding.go`+`memory.go`) port dolu/auto-start başarısız/model bulunamadı gibi durumlarda net `memory:error` event'leri yayınlıyordu — ama **REPL bunları hiç dinlemiyordu**, sadece `memory:saved`'a bakıyordu.

Düzeltme (`internal/replcli/repl.go`): yeni `eventDataSince()` helper (`memorySavedSince`'in "bu turdan sonra" mantığını genelleştirip event data'sını da döndürüyor). `printWelcome()` artık hafıza "kapalı" gösterildiğinde ring buffer'daki mevcut `memory:error`'ı arayıp gerçek sebebi banner'ın altında yazıyor. `reportMemorySaved` artık "✓ hafıza kaydedildi" gelmezse ~2.4s'lik pencerede bir `memory:error` var mı diye de bakıp "⚠ <gerçek sebep>" yazıyor, sessizce vazgeçmek yerine. `saveMemorySync`'in "bağlantı yok" tipi hataları susturma mantığına (API-only kurulumlar için bilinçli) dokunulmadı — sadece zaten yayınlanan event'ler REPL'e görünür hale getirildi. 4 regresyon testi eklendi.

Commit'ler: `6d94f63`, `b04c631`.

## Doğrulama (oturum boyunca, her adımdan sonra tekrar tekrar)

```
CGO_ENABLED=1 go build/vet/test ./... -race -count=1   → tüm 34 paket yeşil
GOOS=windows go vet ./...                               → temiz
flutter analyze lib/                                    → aynı 4 bilinen info-uyarısı, yeni uyarı yok
flutter test                                             → 103/103 yeşil
flutter build linux --debug                             → gerçek binary başarıyla derlendi
```

**Gerçek uygulamada test edilen kısım:** `/api/memory/explicit/save` ve `/api/memory/import-text` canlı bir backend'e karşı curl ile çağrıldı (bug #1'i bu şekilde yakaladık). Gerçek Gemini + ChatGPT çıktıları kullanıcı tarafından test edildi (bug ne yok ne var, prompt kalitesini doğruladı). Terminal REPL kullanıcı tarafından gerçek kullanımda test edildi (bug #4'ü bu şekilde bulduk).

**Gerçek uygulamada test edilemeyen:** Flutter GUI'de Settings → "Hafızayı İçe Aktar" sayfasına tıklayıp gerçek render'ı görmek — bu ortamda input-automation aracı yok (xdotool/ydotool/wmctrl), uygulama direkt chat ekranına açılıyor.

**Sıradaki oturum için:**
1. Kullanıcı Flutter GUI'deki yeni sayfayı gerçekten deneyip inline status banner'ın (bug #3 düzeltmesi) düzgün göründüğünü doğrulamalı.
2. Commit'ler push edilmedi (13 commit, `origin/main`'in 13 ilerisinde).
3. SnackBar-arkasında-modal sorunu Settings'in diğer sekmelerinde de olabilir — audit edilmedi, bilinçli olarak bu oturumun kapsamı dışında bırakıldı.
4. Eğer gerçek dünyada zayıf bir lokal modelin "sadece JSON dön" talimatına uymadığı görülürse, bir retry/fallback mekanizması eklenebilir.

---

# Handoff — 2026-07-12 (Session 24) — BUG-M1 kapatıldı: model_store_screen.dart 5 dosyaya bölündü

## Oturum Özeti

Kullanıcı BUG_REPORT.md'deki tek kalan madde (BUG-M1: `model_store_screen.dart` 2612 satır) için plan istedi, sonra "tamam başla" dedi. `docs/plans/PLAN_modelstore_refactor.md` yazıldı (grep + codebase-memory ile class haritası çıkarılıp doğrulandı, `settings_dialog.dart`'ın daha önce aynı sorun için kullandığı `settings/tabs/` desenine birebir paralel), sonra 5 fazın tamamı uygulandı.

## Yapılan bölme

`frontend/lib/screens/model_store_screen.dart` (2612 satır) → shell (180 satır) + `frontend/lib/screens/model_store/`:
- `discover_item.dart` (194 satır) — `DiscoverItem` veri modeli + `humanizeName`/`timeAgo`/`fmtCount` formatlama yardımcıları (public, çünkü hem Discover hem detay panelinde kullanılıyor)
- `discover_tab.dart` (809 satır) — Discover sekmesi (arama, filtre/sort, model listesi)
- `model_detail_panel.dart` (956 satır) — seçili modelin detay/indirme paneli
- `my_models_tab.dart` (515 satır) — My Models sekmesi

Sadece 8 sembol private'tan public'e çevrildi (dosya sınırları arası referans edilenler): `DiscoverTab`, `ModelDetailPanel`, `MyModelsTab`, `DownloadBanner`, `DiscoverItem`, `humanizeName`, `timeAgo`, `fmtCount`. Geri kalan ~25 widget/fonksiyon (grep + codebase-memory `search_graph` ile tek bölüme özel olduğu doğrulanmış) private kaldı — bölme başta düşünülenden daha az invaziv çıktı.

**Yol boyunca yakalanan 2 gerçek hata:**
1. `_ModelListRow`'daki local `timeAgo` değişkeni, yeni public `timeAgo` fonksiyonuyla aynı isimde olunca kendi kendini referans eden bir initializer'a dönüştü (`referenced_before_declaration` derleme hatası) — `timeAgoText` olarak yeniden adlandırıldı.
2. `_Pill` widget'ı fiziksel olarak `_ModelDetailPanel`'in hemen yanındaydı ama kod içindeki yorum zaten "reused in My Models" diyordu — ben yine de ilk yazımda `model_detail_panel.dart`'a dahil ettim, `flutter analyze`'ın "unused_element" uyarısı yakaladı, `my_models_tab.dart`'a taşındı.

## Commit'ler (3 kod + kendi plan/docs commit'leri, hepsi local)

- `4076bf2` docs: plan dosyası
- `b1c12a4` refactor: Faz 1 (DiscoverItem)
- `2d87364` refactor: Faz 2-4 tek commit'te (discover_tab.dart, model_detail_panel.dart birbirine derleme-bağımlı; ayrı commit'lere zorlamak ara durumda derlenmeyen bir ağaç bırakırdı — bu gerekçe commit mesajında da açık)

## Doğrulama

```
flutter analyze lib/   → temiz (sadece bilinen 4 info-level use_build_context_synchronously)
flutter test           → 103/103 yeşil
```

**Gerçek uygulama testi (kısmi):** Gerçek backend (`go build` + `--headless --port 8090`) ve gerçek Linux binary (`flutter build linux --debug`) çalıştırıldı, `python3 PIL ImageGrab` ile ekran görüntüsü alındı (bu ortamda X11 `import`/ImageMagick screen-grab çalışmadı, PIL çalıştı). Model Store → Discover sekmesi ekran görüntüsüyle doğrulandı: arama çubuğu, sort/filter chip'leri, model listesi (capability ikonları dahil), boş detay durumu, `_HardwareChip`'in GPU adı — hepsi doğru render ediliyor.

**Doğrulanamayan kısım:** Model Detail Panel'e tıklayarak geçiş ve My Models sekmesi. Bu ortamda `xdotool`/`wmctrl`/`ydotool` yok, passwordless sudo yok (kuramadım), ve `python-xlib` ile denenen `XTestFakeInput` sentetik tıklaması native Wayland penceresine hiç ulaşmadı (aynı koordinatlarda bilinen bir sekme değiştirme denemesi bile tetiklenmedi — sistemik bir kısıt, koordinat hatası değil). Dolaylı kanıt: `flutter analyze` tamamen temiz + `app_shell.dart`'ın `IndexedStack`'i tüm sekmeleri (My Models dahil) ekran açılışında zaten inşa ediyor ve hiç hata kutusu çıkmadı — yani `MyModelsTab.build()` da hatasız çalıştı. Sadece `ModelDetailPanel`'in kendisi (bir model seçilmeden inşa edilmiyor) gerçek çalışırken hiç görülmedi.

**BUG_REPORT.md:** BUG-M1 tamamen kapatıldı ve silindi. **Toplam açık madde: 0.**

**Sıradaki oturum için:**
1. Bu ortamda input-automation aracı (xdotool vb.) kurulabilirse Model Detail Panel'e gerçek tıklamayla bakılabilir — küçük bir doğrulama boşluğu, ama risk düşük (flutter analyze zaten temiz).
2. Commit'ler henüz push edilmedi.

---



## Oturum Özeti

Kullanıcı: "basit bir köprü değil gerekirse skill sistemini baştan yaz komple çalışır her özelliği stable yakın skill feature istiyorum." Session 22'de TD-1 (skill manifest'lerindeki `tools:` tanımlarının hiçbir zaman gerçek bir agent tool'una dönüşmemesi) sadece dokümante edilip koda dokunulmamıştı — bu oturumda gerçek bir çözüm inşa edildi: skill tool'ları artık gerçekten çalıştırılabiliyor, agent'ın permission/danger-level akışına giriyor, ve mevcut izin diyaloğu UI'si ek iş gerekmeden devreye giriyor.

## Kök tasarım eksikliği (araştırma bulgusu)

`skill.SkillTool` (manifest'teki her `tools:` girdisi) hiçbir zaman "bu tool çağrıldığında ne çalıştırılacağı"nı tanımlayan bir alana sahip olmamış — ne bir `command`, ne bir script yolu, hiçbir şey. `skill.ToolRegistrar.SetToolRegistrar()` da prod kodunda hiç çağrılmıyordu. Yani iki ayrı eksiklik vardı: (1) hiçbir execution mekanizması tanımlı değildi, (2) var olsa bile hiçbir yere bağlı değildi. `docs/superpowers/specs/2026-06-13-skill-system-design.md` ve `docs/superpowers/plans/2026-06-13-skill-system.md` (orijinal tasarım/plan dokümanları) okunarak bu, "basit bir tip uyuşmazlığı" değil, tasarımın hiç bitirilmemiş bir parçası olduğu doğrulandı.

## Yapılan değişiklikler

1. **`internal/skill/types.go`** — `SkillTool`'a `Command string` alanı eklendi (skill'in kendi dizinine göre çözümlenen shell komutu). Ayrıca `SkillToolRegistration{SkillName, Tool}` struct'ı eklendi — `RegisterTool`'a artık salt `SkillTool` değil, hangi skill'e ait olduğu bilgisi de geçiyor (`skillToolName()`'in `"skill_<skill>_<tool>"` string'ini geri parse etmek underscore içeren isimlerde güvenilir olmazdı).
2. **`internal/skill/executor.go`** (yeni) — `Manager.ExecuteTool(ctx, skillName, toolName, args, basePath)`: komutu skill'in kendi dizininde çalıştırıyor, LLM'in argümanlarını stdin'e ham JSON olarak yazıyor (komut string'ine hiç enjekte etmiyor — `run_command`'dan farklı olarak enjeksiyon yüzeyi yok), `MEMO_SKILL_ARGS`/`MEMO_SKILL_NAME`/`MEMO_SKILL_DIR`/`MEMO_PROJECT_DIR` env değişkenlerini set ediyor.
3. **`internal/agent/tools/command.go`** — `run_command`'ın sandbox mantığı (OS'e göre shell seçimi, env kurulumu, output truncation, deadline handling) `PrepareCommand`/`FormatCommandOutput` olarak dışa çıkarıldı ki skill executor aynı, test edilmiş mantığı tekrar yazmadan kullansın. `CheckDestructivePatterns` (yalnızca yıkıcı komut kalıpları) `CheckBlacklist`'ten (o da `$`/backtick shell-substitution bloğunu içeriyor) ayrıldı — skill `command` alanı LLM'in o an ürettiği bir string değil, skill yazarının önceden belirlediği statik içerik, bu yüzden `$VAR` kullanımını (tam da yukarıdaki env değişkenlerini okumak için gerekli) bloke etmemesi gerekiyordu.
4. **`internal/agent/tools.go`** — `ToolRegistry.Unregister(name)` eklendi (önceden yoktu, sadece `Register`/`Get` vardı).
5. **`internal/agent/executor.go`** — `Executor.Registry()` erişimcisi eklendi, `internal/app`'ın gerçek `agent.ToolRegistry`'ye ulaşabilmesi için.
6. **`internal/app/skill_tools.go`** (yeni) — `skillToolRegistrar`: `skill.ToolRegistrar` arayüzünü gerçek `agent.ToolRegistry`'ye bağlıyor. `Command` alanı boş olan tool'lar sessizce atlanıyor (hata değil — manifest'te salt dokümantasyon amaçlı bir tool listelenebilir).
7. **`internal/app/app.go`** — `Startup()`'ta `a.skillManager.SetToolRegistrar(newSkillToolRegistrar(a.agentExecutor.Registry(), a.skillManager))` eklendi — TD-1'in asıl eksik satırı.

## Neden ek frontend işi gerekmedi

Skill tool'ları gerçek `agent.ToolDef` olarak kaydedildiği için, mevcut permission/danger-level/preview akışı (`internal/agent/pipeline.go`, izin diyaloğu) hiçbir değişiklik yapılmadan otomatik olarak devreye giriyor — built-in tool'lardan (read_file, run_command, vb.) hiçbir farkı yok LLM'in ve UI'nin bakış açısından. Backend API/bridge/route/Flutter tarafı (`/api/skills/*`, `skill_provider.dart`, `skills_tab.dart`) zaten Session öncesinde tam çalışır durumdaydı — sadece tool execution bacağı eksikti.

## Testler (hepsi yeni, hepsi gerçek yürütmeyle — mock yok)

- `internal/skill/executor_test.go`: stdin'den argüman alma (`cat` ile), env değişkenlerinin gerçekten set edilmesi, `command` olmayan tool'un hata vermesi, bilinmeyen skill/tool hatası, blacklist reddi, skill dizinine göreli bundled script çalıştırma (`bash scripts/hello.sh`).
- `internal/app/skill_tools_test.go`: `SetActive` → gerçek `agent.ToolRegistry`'de tool'un göründüğünü, doğru `DangerLevel`'e sahip olduğunu, `registry.Execute()` ile gerçekten çalıştırılabildiğini, deaktivasyonda kaydın silindiğini, `command`'sız tool'un hiç kaydedilmediğini, yanlış tipte `toolDef` verilirse hata döndüğünü doğruluyor.

## Doğrulama

```
CGO_ENABLED=1 go build ./...                                    → temiz
CGO_ENABLED=1 go vet ./...                                       → temiz
GOOS=windows go vet ./...                                        → temiz
CGO_ENABLED=1 go test ./... -race -count=1                       → tüm paketler yeşil (skill, agent, agent/tools, app dahil)
```
Frontend'e bu oturumda dokunulmadı (zaten tam wired durumdaydı, gerek yoktu).

**BUG_REPORT.md:** TD-1 tamamen kapatıldı ve listeden silindi (toplam açık madde: 2 → 1, kalan tek madde BUG-M1, kullanıcı isteğiyle bilinçli atlanmış).

**Sıradaki oturum için:**
1. Gerçek bir skill scriptiyle (Python/Node, sadece `cat`/`echo` değil) elle uçtan uca doğrulama yapılmadı — sadece unit/entegrasyon testleriyle kanıtlandı.
2. Orchestra modu skill tool'larını hâlâ görmüyor (kapsam dışı bırakıldı — tasarım dokümanı orchestra entegrasyonunu yalnızca instruction/rol enjeksiyonu olarak tanımlıyor, tool execution değil).
3. Gerçek bir skill (`skills/memo`, `skills/memo-project`) hâlâ `tools:` kullanmıyor — bunlar salt instruction-injection skill'leri, bilerek dokunulmadı.

---



## Oturum Özeti

Kullanıcı önce AGENTS.md'deki açık teknik borç maddelerinin BUG_REPORT.md'ye "bilinen hatalar" gibi taşınmasını istedi, sonra "adım adım tüm hataları fixleyecez" dedi. Bir soru turu sonrası kullanıcı "kanka soru sorma, model store refactor dışında bug repotu tertemiz et, tüm bugları gerekli şekilde düzelt" dedi — bunun üzerine BUG-M1 (model_store_screen.dart bölme) hariç kalan her şey soru sorulmadan sırayla düzeltildi.

## Taşıma (kod değişikliği yok)

AGENTS.md'nin "Known Pitfalls & Technical Debt" ve "Known Open Work" bölümlerindeki hâlâ açık (üstü çizilmemiş) maddeler BUG_REPORT.md'ye yeni bir "🔧 Teknik Borç" bölümü olarak taşındı: chat-ID refactor'ün eksik kısmı, `skill.DangerLevel`/`agent.DangerLevel` "tip uyumsuzluğu" iddiası, API versioning yokluğu. AGENTS.md'deki "Known Open Work" tablosu **kullanıcı talimatıyla** boş bırakıldı ama tablo iskeleti korundu (kural: AGENTS.md'deki hiçbir tablo yapısı bozulmayacak, sadece içerik satırları silinebilir/değişebilir).

## Düzeltilen maddeler (5/7, kod + test + commit her biri ayrı)

1. **TD-2 (DangerLevel) → araştırma, KOD DEĞİŞİKLİĞİ YOK, kullanıcı kararıyla:** Gerçek bulgu iddia edilenden çok farklı çıktı — `skill.RegisterTool(name, toolDef any)` zaten `any` alıyor, hiçbir compile-time tip hatası yok. Asıl sorun: `skill.Manager.toolRegistrar` hiçbir yerde set edilmiyor (`SetToolRegistrar` 0 çağıran), `agent.FromString` 0 çağıran — skill manifestlerindeki `Tools` tanımı hiçbir zaman gerçek bir agent tool'una dönüşmüyor, üstelik `SkillTool`'da `ExecuteFn` bile yok. Kullanıcıya 3 seçenek sunuldu (dokümanı düzelt / basit köprü kur / dead code temizle), "sadece dokümanı düzelt" seçildi. Madde TD-1 olarak yeniden numaralandırılıp güncellendi, kod dokunulmadı.

2. **BUG-H3 (Windows'ta backend self-shutdown çalışmıyor) → gerçek kök neden bulunup düzeltildi (commit `70f021f`):** Sadece client-registry auto-shutdown değil, `POST /api/shutdown` HTTP handler'ı da AYNI hatayı taşıyordu — ikisi de `os.Process.Signal(os.Interrupt)` ile kendine sinyal göndermeye çalışıyordu, Go bunu Windows'ta sadece `os.Kill` için implemente ediyor (`os.Interrupt` dahil her şey `EWINDOWS` ile sessizce başarısız). Yeni `internal/shutdown` paketi (`Request()`/`Requested()`, process-wide kanal) her iki çağrı noktasını da OS sinyaline hiç bağımlı olmayan bir mekanizmaya taşıdı; `main.go`'nun sinyal-bekleme döngüsü artık ikisini birden `select`liyor. `GOOS=windows go vet` temiz, gerçek Windows makine testi hâlâ yapılmadı.

3. **TD-1 (chat-ID refactor Faz 3) → tamamlandı (commit `098bee7`):** `docs/plans/PLAN_chatid_refactor.md`'nin planladığı public `SendMessageStreamTo(ctx, chatID, userMsg)` eklendi — `routeStream`'e yeni `forceAgent bool` parametresi (chat'in kendisi `sm.IsAgentChat` ise global `agentEnabled` bayrağına dokunmadan o TEK çağrı için tool execution'ı açıyor). `tasklist.go`'daki `SwitchChat` + global `agentEnabled` zorlama + race'li geri-alma bloğu tamamen kaldırıldı, yerine `SendMessageStreamTo` çağrısı kondu. `taskloopRunMu` bilinçli olarak korundu (artık "sohbet karışması önleme" değil, "task-list turlarını sıraya sokma" amaçlı). Yeni regresyon testleri: chatB aktifken `SendMessageStreamTo(chatA,...)` mesajının chatA'ya yazılıp `GetActiveID()`'in hiç değişmediğini kanıtlıyor.

4. **BUG-M2 (connectionStatusProvider sonsuz polling) → kapatıldı, KOD DEĞİŞİKLİĞİ YOK:** `app_shell.dart` bu provider'ı hep dinliyor (autoDispose hiç tetiklenmiyor) ama bu **kasıtlı** — backend'in client-registry'sine GUI'nin canlı olduğunu bildiren heartbeat'in ta kendisi. Bug değil, madde kapatıldı.

5. **TD-2 (API versioning yok) → gerçek versiyonlama stratejisi eklendi (commit `5df2a50`):** 118 route'u taşımak/yeniden yazmak aşırı riskli olurdu (3 istemci: Flutter desktop, mobile, CLI) — bunun yerine `server.go`'daki `route()` yardımcı fonksiyonu her `/api/...` pattern'ini (düz route'lar, Go 1.22+ `{wildcard}`'lar, trailing-slash prefix'ler dahil) hem eski haliyle hem `/api/v1/...` alias'ı olarak kayıt ediyor — sıfır risk, hiçbir route taşınmadı/yeniden adlandırılmadı. Gerçek bir HTTP sunucusu başlatıp hem düz hem v1 path'lerini (plain/wildcard/trailing-slash) test eden entegrasyon testi eklendi.

6. **BUG-L5 (ngrok token restart sonrası kayboluyor) → kök neden düzeltildi (commit `b8eb483`):** Token backend'de restart'lar arası sabit kalıyor (`internal/app/remote.go` sadece boşsa üretiyor) ama masaüstü istemci sadece bellekte tutuyordu — her açılışta sıfırlanıyordu. `MemoApiClient` artık `savedRemoteToken` (constructor'da hemen header'a uygulanıyor) ve `onRemoteTokenLearned` callback'i alıyor; `apiClientProvider` bunları `SharedPreferences`'a bağladı. Bayat/rotated token hâlâ 401 verir (öncekinden kötü değil), bir sonraki `getRemoteAccess()` çağrısında kendini düzeltir. Unit test'lerle doğrulandı, **canlı ngrok+telefon testi yapılmadı** (bu ortamda mümkün değil).

## Bilinçli atlanan / dokunulmayan

- **BUG-M1** (`model_store_screen.dart` 2612 satır) — kullanıcı açıkça hariç tuttu.
- **TD-1** (skill→agent tool köprüsü hiç kurulmamış, dead code) — kullanıcı kararıyla sadece dokümante edildi, kod dokunulmadı (kapsamı belirsiz bir feature kararı, basit bir fix değil).

**BUG_REPORT.md nihai durum:** 0 kritik, 0 HIGH, 1 MEDIUM (BUG-M1, bilinçli atlandı), 0 LOW, 1 TEKNİK BORÇ (TD-1, bilinçli atlandı) — toplam 2 açık madde, ikisi de kullanıcı kararıyla kasıtlı olarak açık bırakıldı.

## Doğrulama

```
CGO_ENABLED=1 go build/vet/test ./... -race -count=1  → tüm paketler yeşil
GOOS=windows go vet ./...                              → temiz
flutter analyze lib/                                   → aynı 4 bilinen info-uyarısı
flutter test                                            → 103/103 yeşil
```

**Commit'ler (6 kod + 6 docs, hepsi local, henüz push edilmedi):** `70f021f`, `098bee7`, `5df2a50`, `b8eb483` (kod) + `f83666c`, `e66890d`, `0eacf0e`, `0eaf5c1` (docs) + oturum başındaki 2 taşıma commit'i.

**Sıradaki oturum için:**
1. Bu oturumun commit'lerini push et (kullanıcıya sorulmadı, bilinçli).
2. BUG-H3 ve BUG-L5 gerçek ortamlarda (Windows makine, canlı ngrok+telefon) doğrulanmalı.
3. BUG-M1 (model store split) kullanıcı isteyince ele alınabilir.
4. TD-1 (skill tool köprüsü) — ürün kararı gerekiyor: gerçekten inşa edilsin mi, yoksa dead code temizlensin mi?

---

# Handoff — 2026-07-12 (Session 21) — Dış AI analizi doğrulama + chunking token-fix + BUG_REPORT.md durum teyidi

## Oturum Özeti

Kullanıcı `yapacam.md` adlı bir dosyaya başka bir AI'nın Memo kod tabanı üzerine yazdığı 5 maddelik bir "bug" analizini yapıştırmıştı (dosya 6. maddenin ortasında kesik geldi — devamı istenmedi). Görev: hiçbir iddiayı körü körüne kabul etmeden kod tabanında tek tek doğrulamak, gerçek olanları düzeltmek.

### Doğrulama sonucu (5/5 madde incelendi)

1. **Chunking word-based, token-based değil** (`internal/memory/chunker.go`) — ✅ **gerçek**, düzeltildi (aşağıda).
2. **Heading-bazlı semantic chunking yok** — kısmen doğru ama **kapsam dışı**: `chunkText`'in tek çağıranı `SaveInteraction`, sadece tek bir chat mesajını (kullanıcı+asistan) chunk'lıyor — Memo'da belge/PDF import → RAG ingestion pipeline hiç yok, bu yüzden başlık-bazlı bölme (LangChain'in `RecursiveCharacterTextSplitter`'ı gibi) bu kullanım senaryosuna uymuyor. Atlandı.
3. **"Re-ranking yok" iddiası** — ❌ **yanlış**, güncel değil. `internal/memory/store.go` zaten vektör+FTS hibrit arama + `reciprocalRankFusion` (RRF) kullanıyor (satır 635, 748). Diğer AI muhtemelen eski (pre-hybrid-rework) mimariyi görmüş.
4. **`model_store_screen.dart` 2612 satır** — zaten AGENTS.md'de bilinen bir bakım notu, yeni bir bug değil.
5. **"Agent'ta panic recovery yok" iddiası** — ❌ **yanlış**, bayat bilgi. Tam olarak geçen oturumda (BUG-H3, commit `9fb11b7`) düzeltilmişti; `executor.go`'nun `RunStream`'i `pipeline.RunStream`'e delege ediyor, orada `defer recoverStreamPanic(...)` zaten var (`internal/agent/pipeline.go:96`).

**Ders:** başka bir AI'nın analizini kaynak olarak kullanırken her iddia ayrı ayrı koda karşı doğrulanmalı — 5 maddenin 2'si tamamen bayat/yanlış çıktı.

## Yapılan tek kod değişikliği: token-bazlı chunking (commit `05d3142`)

`internal/memory/chunker.go`'daki `chunkText`, `strings.Fields` ile **kelime sayıyordu** — ama kelime sayısı token sayısının kötü bir vekili: uzun bir identifier/URL/bitişik yazılan Türkçe kelime birden fazla token'a karşılık gelebiliyor. Sonuç: kelime sayımına göre "300 kelimeden az, tek chunk" denen bir mesaj, gerçek token sayımında budget'ı aşabiliyordu.

**Düzeltme:** `chunkText` artık projede zaten var olan `internal/truncate.EstimateTokens` (char/3 sezgisel) ile tahmini token sayısına göre bölüyor — prefix-sum ile O(n) sliding window, aynı overlap/min-chunk-merge davranışını koruyor. `store.go`'daki `chunkMaxWords`/`chunkOverlapWords` sabitleri `chunkMaxTokens`/`chunkOverlapTokens` olarak yeniden adlandırıldı (değer aynı: 300/50, artık token cinsinden).

**Testler:** `chunker_test.go`'a yeni `TestChunkText_LongWordsExceedBudget` eklendi — 300 kelimelik ama her kelimesi uzun (12 karakter ≈ 5 token) bir metin: eski kod bunu tek chunk sayıyordu (kelime sayısı ≤300), yeni kod doğru bölüyor. Bu test, `git stash` ile eski koda karşı çalıştırılıp **gerçekten fail ettiği** kanıtlandı. Var olan 3 test (`ShortText`, `LongText`, `OverlapContent`) token-semantiğine uyacak şekilde güncellendi (`OverlapContent` artık tam 50-kelime eşitliği yerine son/ilk kelimenin ortaklığını kontrol ediyor, çünkü overlap genişliği artık token maliyetine göre değişken). Ayrıca `TestChunkText_NoDataLoss` eklendi. `store_test.go`'daki `TestSaveInteraction_Chunking` de aynı sebeple güncellendi (sabit "2 chunk" beklentisi yerine ">1 chunk").

## BUG_REPORT.md durumu — kullanıcı "HIGH'lar bitti sanıyordu", DOĞRU DEĞİL

Kullanıcı bu oturuma "HIGH'lar bittiydi sanırım, sadece LOW kalmıştı" diyerek başladı. Dosyayı ve ilgili kodu (chat.go, chat_provider.dart, clients.go, api_client.dart) yeniden kontrol ettim: **yanlış** — Session 20'den beri hiçbir HIGH/MEDIUM/LOW maddesine dokunulmamış (aradaki tek commit'ler bu oturumun chunking fix'i ve docs). Güncel durum aynen BUG_REPORT.md'de yazdığı gibi:

- 🟠 **HIGH: 3 açık** — BUG-H1 (chat-switch backend race, `chat.go:210-217`), BUG-H2 (chat-switch frontend, `chat_provider.dart`), BUG-H3 (Windows'ta auto-shutdown sinyali çalışmıyor, `clients.go:136-142`). Hepsi Session 20'de bilinçli atlanmıştı (H1+H2 birlikte ele alınması gereken büyük bir mimari iş — chat-id refactor planıyla kesişiyor; H3 bu ortamda [Linux] test edilemez).
- 🟡 **MEDIUM: 2 açık** — M1 (`model_store_screen.dart` boyutu, bakım notu), M2 (`connectionStatusProvider` sonsuz polling, kabul edilmiş).
- 🟢 **LOW: 5 açık** — L1-L5, hepsi spot-check ile doğrulandı, hâlâ kod tabanında aynen duruyor (satır numaraları ~10 satır kaymış olabilir ama bug'lar aynen mevcut).

Kullanıcıya bu düzeltildi, sonraki adım için üç seçenek sunuldu (LOW'lardan devam / HIGH'a başla / sadece docs) — kullanıcı bu oturumda **sadece dokümanları güncellemeyi** seçti, koda dokunmadı.

## Doğrulama

```
CGO_ENABLED=1 go build ./... && go vet ./... && go test ./... -race -count=1
  → tüm paketler yeşil (memory dahil, chunking testleri geçti)
```
Frontend'e bu oturumda dokunulmadı.

## Ek iş (aynı oturum) — CLI: her açılışta temiz sohbet + CLI/GUI sohbet senkronu (commit `4e58b76`)

Kullanıcı ayrı bir istekle geldi: "CLI modunu açınca hemen bir önceki eski sohbetten başlatıyor, bu kirli bir arayüz/context anlamına geliyor — `/session` zaten eski sohbete dönmeyi sağlıyor, CLI direkt `new chat` olarak açılsın. Ayrıca CLI ve GUI'deki sohbetler senkron olsun — şu an CLI sohbeti GUI'de görünmüyor, GUI sohbeti CLI'de istediği yerden devam edemiyor."

**Araştırma (agent + doğrudan kod okuma):** Backend'de aslında **tek global sohbet listesi** var (`internal/sessions.Manager.active`, `AGENTS.md`'nin "one global active chat" notuyla uyumlu) — CLI ve GUI aynı `/api/chats`/`/api/chats/switch` uç noktalarını kullanıyor, ayrı listeler değil. Gerçek asimetri şuydu:
- **GUI zaten tüm sohbetleri gösteriyordu** (`chatListProvider`, filtre yok) — CLI'nin oluşturduğu agent sohbetleri GUI'de zaten görünüyordu. Bu yön zaten çalışıyordu.
- **CLI'nin sorunu iki yerdeydi:** (1) `repl.go`'daki `resumeOrStartChat()`, her `memo` başlatışında `s.projectPath`'e eşit `ProjectPath`'li en son sohbeti otomatik resume ediyordu; (2) `/session` komutunun `projectChats()`'i de aynı proje-yolu filtresini uyguluyordu — GUI'de oluşturulan düz sohbetler (`ProjectPath` boş) ve başka bir dizinden açılmış CLI sohbetleri `/session` listesinde bile hiç görünmüyordu.

**Düzeltme:**
- `resumeOrStartChat`/`findRecentChat` tamamen kaldırıldı; `Run()` artık koşulsuz `s.startFreshChat()` çağırıyor — her `memo` başlatışı temiz bir sohbetle açılıyor, eski context hiç sızmıyor.
- `projectChats()` → `allChats()` (proje filtresi olmadan `ListChats` çağrısı) — `/session list`, `/session` (arrow-key picker) ve `/session <sorgu>` artık GUI dahil her istemcinin her sohbetini gösteriyor, GUI'nin kendi listesiyle birebir aynı küme.
- Liste/menü girdilerine küçük bir ipucu eklendi: `ProjectPath` set edilmişse (yani CLI'nin agent modunda oluşturduğu bir sohbetse) proje dizininin son bileşeni gösteriliyor, GUI'nin düz sohbetlerinden ayırt edilebilsin diye.

**Bilinçli sınır:** `switchToChat`/`activateChat` her zaman `SetAgentEnabled(true)` çağırıyor (CLI'nin tasarım gereği her zaman agent modunda çalışması, `AGENTS.md`'de zaten dokümante) — CLI'den düz bir GUI sohbetine geçilirse o sohbette agent modu zorla açılır, ama sohbetin kendi `ProjectPath`'i (dolayısıyla GUI'nin `isAgentChat` algısı) değişmez. Bu, "hangi sohbet agent-tipli" bilgisinin kalıcı olmayışından kaynaklanan daha derin bir mimari nüans — bu oturumun kapsamı dışında bırakıldı, ileride bir "chat mode" alanı gerekebilir.

**Testler:** `TestRun_ResumesExistingAgentChat` → `TestRun_AlwaysStartsFreshChat` (artık her zaman yeni sohbet oluşturulduğunu VE geçmiş replay edilmediğini doğruluyor). `TestHandleCommand_Session_List_FiltersByProject` → `TestHandleCommand_Session_List_ShowsAllChats` (üç farklı `ProjectPath` — biri boş, biri farklı proje, biri eşleşen — hepsinin listede çıktığını doğruluyor).

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil (`internal/replcli` dahil, 15.7s). Ayrıca gerçek derlenmiş binary ile: geçici bir headless backend başlatılıp `POST /api/chats/new` ile "GUI-stili" bir sohbet oluşturuldu, `curl` ile `/api/chats` yanıtının testlerin varsaydığı JSON şekliyle (id/title/created_at/updated_at/msg_count) birebir eşleştiği doğrulandı. Piped-stdin ile gerçek REPL akışını uçtan uca sürmeye çalışan bir deneme, `main.go`'nun tasarım gereği non-TTY girdide headless moda düşmesi yüzünden başarısız oldu (bug değil, benim test kurgum yanlıştı) — asıl REPL akışı zaten `repl_test.go`'nun gerçek bir `httptest.Server`'a karşı çalışan testleriyle kapsanıyor.

Frontend'e bu ek işte dokunulmadı (GUI tarafı zaten doğru davranıyordu).

## Üçüncü iş (aynı oturum) — Embedding auto-start bazen çalışmıyor, port reboot'a kadar kilitli kalıyor (commit `6ce9e36`)

Kullanıcı: "Embedding modelinin auto-start'ı bazen çalışmıyor, çoğunlukla çalışıyor ama bir kere çalışmayınca sistemi reboot edene kadar çalışmıyor ve port işgal ediyor." Bir agent'la araştırılıp kod okumasıyla bizzat doğrulanan kök neden:

**Zincir:**
1. `internal/llama/sysproc_linux.go`/`sysproc_darwin.go`/`sysproc_other.go` — **hiçbiri** `Pdeathsig` set etmiyor (`Setpgid` ile teknik olarak uyumsuz — Go runtime'ın thread reuse'u erken çocuk ölümüne yol açıyor). `llama.go`'daki eski yorum bunun tersini iddia ediyordu (bayat/yanlış, düzeltildi).
2. Sonuç: Memo'nun ana Go process'i anormal biçimde ölürse (crash, `kill -9`, OOM) — kendi `Stop()` akışından geçmeden — spawn ettiği `llama-server` alt süreci **yetim kalıyor**, portu (embedding için varsayılan 8082) sonsuza kadar tutmaya devam ediyor.
3. Memo tekrar başlatıldığında yepyeni bir `llama.Server` struct'ı oluşuyor (`app.go:343`), bu yetimden hiç haberi yok.
4. `StartEmbeddingModel` (`embedding.go`) portu bağlamayı dener: `cmd.Start()` OS seviyesinde başarılı olur (fork/exec her zaman başarılı), ama `llama-server`'ın kendisi portu bağlayamayıp ~1 saniyede çöker. `WaitReady` bunu hemen fark edip hata döner.
5. Ardından çağrılan `Stop()`, `s.cmd` bu denemenin (artık ölü) kendi process'ine işaret ettiği için "tracked cmd" dalına giriyor — kendi (zaten ölü) PID'ine SIGTERM gönderip dönüyor. Port-discovery fallback'i olan `killByPort` **hiç tetiklenmiyor** (o sadece `s.cmd == nil` olduğunda, yani "no tracked cmd" dalında çalışıyor).
6. 3 denemenin hepsi aynı şekilde başarısız oluyor — gerçek işgalci hiç dokunulmadan kalıyor, `StartEmbeddingModel` pes ediyor. Port, o yetim süreç bir şekilde ölene kadar (kullanıcının deneyiminde: reboot) kilitli kalıyor.

**Düzeltme:** `Start()` artık her denemede (retry'lar dahil, koşulsuz), süreci spawn etmeden **önce** hedef portu `s.killByPort(actualPort)` ile temizliyor — zaten var olan, `lsof`/`fuser` tabanlı mekanizma, sadece doğru yerden çağrılıyor. Port boşsa no-op (zaten öyle davranıyordu). Bu hem embedding hem chat-model sunucusunu aynı anda düzeltiyor (`Start()` ikisi için de ortak).

**Test:** `internal/llama/process_test.go` (yeni dosya) — klasik `os/exec` re-exec pattern'iyle test binary'sinin kendisini "helper process" modunda spawn edip gerçek bir TCP dinleyici + PID oluşturuyor, `killByPort`'un bunu gerçekten bulup öldürdüğünü kanıtlıyor. İlk yazımda `processIsAlive`'ın zombie-process'i "canlı" sayması yüzünden yanlış-negatif fail aldı (test kendi helper'ının doğrudan parent'ı olduğu için, reap edilmeden zombie kalıyor) — `cmd.Wait()` ile düzeltildi, daha doğru bir canlılık kanıtı zaten.

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil (`internal/llama` dahil, yeni test dahil 4.3s). `GOOS=windows`/`GOOS=darwin go vet ./internal/llama/...` → temiz.

## Dördüncü iş (aynı oturum) — BUG_REPORT.md'de LOW'lardan devam + HIGH'lara geçiş (chat-switch race)

Kullanıcı "BUG_REPORT.md'den adım adım devam edelim" dedi. Önce 3 LOW madde tek tek düzeltildi, sonra kullanıcı "HIGH'lara geçelim" dedi — bu da mevcut `docs/plans/PLAN_chatid_refactor.md`'nin (2026-07-06 tarihli, önceki bir oturumda yazılmış, "BÜYÜK, faz faz ilerle" notlu) Faz 1 ve Faz 2'sinin uygulanmasına denk geldi.

### LOW'lar (3/5 düzeltildi, testli, her biri kendi commit'i + docs commit'i)

- **BUG-L2** (`12c26f1`) — `api_client.dart`'ın `listChats()` object-wrapped fallback dalı (`res.data['chats'] as List`, sadece `!= null` korumalı) `_guard<List>` ile değiştirildi — kardeş root-list dalıyla tutarlı hale geldi. Test eski koda karşı gerçekten fail etti (raw `TypeError` vs. beklenen `Exception`).
- **BUG-L1** (`0290066`) — İzin diyaloğu artık `isSendingProvider`'ı dinliyor, `false` olunca kendini kapatıyor (Stop/chat-switch, ikisi de `stopStreaming()` üzerinden bunu tetikliyor). Eski davranışta diyalog stream durdurulsa/sohbet değiştirilse bile ekranda kalıyordu.
- **BUG-L3** (`6eadbef`) — `main.go`'nun `--auto-shutdown` sinyal bekleme döngüsü artık sinyali işlemeden **hemen önce** `a.HasActiveClients()` (yeni) ile tekrar doğruluyor — `selfShutdownIfIdle`'ın kararı (registry boştu) ile sinyalin gerçekten teslim edilmesi arasındaki dar pencerede yeni bir client (ör. `/gui`) kaydolursa artık sinyal görmezden geliniyor.
- **Bilinçli bırakılan 2 LOW:** BUG-L4 (model-swap mid-stream — gerçek düzeltme daha büyük bir mimari karar), BUG-L5 (ngrok token yarışı — dokümanın kendisi zaten "canlı ngrok/telefon testi gerektirir" diyor, bu ortamda test edilemez).

### HIGH'lar — chat-switch race (BUG-H1 backend `f00197f`, BUG-H2 frontend `d18f99e`)

`PLAN_chatid_refactor.md`'nin Faz 1'i (session'ın başında, `a632873`) ve Faz 2'si uygulandı — ama Faz 2 planın istediği **public** `SendMessageStreamTo(ctx, chatID, userMsg)` API'siyle değil, daha dar bir mekanizmayla: `sendMessageStreamInner`/`SendMessageWithImageStream`/`SendMessageWithFileStream` artık `chatID := sm.GetActiveID()`'i çağrının en başında **bir kez** yakalayıp `buildMessagesForSession`/`AddMessageToSession`/`routeStream` boyunca aynı değeri kullanıyor — eskiden bu üçü ayrı ayrı "şu an aktif olan ne" diye soruyordu, aralarında bir switch olursa history bir sohbetten okunup mesaj/reply başka birine yazılabiliyordu (BUG-H1). Kanıt: `TestBuildMessagesForSession_IgnoresConcurrentActiveChatSwitch` + ad-hoc bir sanity testiyle eski global `buildMessages()`'ın bu senaryoda gerçekten yanlış sohbetin geçmişini sızdırdığı doğrulandı.

Frontend tarafında (BUG-H2), `ActiveChatIdNotifier.switchTo`'nun `stopStreaming()` + `ref.invalidate(messagesProvider)` çağrısı eski notifier'ı dispose ediyor ama Riverpod in-flight `sendMessage()`/`sendFile()` coroutine'ini iptal etmiyor — devam edip dispose olmuş instance'a yazmaya çalışıyordu. **Beklenmedik bulgu:** bu Riverpod sürümünde disposed notifier'a `state` yazmak throw etmiyor (sessizce yutuluyor) — asıl gözlemlenebilir zarar, disposed instance'ın `finally` bloğunun paylaşılan/global `isSendingProvider`'ı koşulsuz `false`'a çekmesi, yani B sohbetine geçilip orada yeni bir gönderim başlatılsa bile A'nın terk edilmiş stream'i bitince B'nin "gönderiliyor" göstergesini yanlışlıkla kapatıyordu. Test tasarımı da bu yüzden iki kez revize edildi (önce "throw etmiyor mu" diye test edildi, atlıyordu bile eski kodda — sonra gerçek semptomu, `isSendingProvider` klobber'ını, doğrulayacak şekilde yeniden yazıldı; `git stash` ile eski koda karşı gerçekten fail ettiği kanıtlandı).

**Plan'da bilinçli açık bırakılan:** `PLAN_chatid_refactor.md` güncellendi — Faz 2 "kısmen tamamlandı" olarak işaretlendi. Asıl planın istediği, **dışarıdan explicit bir chatID kabul eden** public `SendMessageStreamTo` hâlâ yok (mevcut düzeltme sadece "çağrı sırasında aktif olanı sabitliyor," aktif-olmayan bir sohbete dışarıdan mesaj göndermeyi sağlamıyor) — Faz 3'ün (task loop workaround'unu kaldırma) ihtiyaç duyduğu tam olarak bu, yani Faz 3'e başlamadan önce bu public API eklenmeli.

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil. `flutter analyze lib/` → aynı 4 bilinen info-uyarısı. `flutter test` → 99/99 yeşil.

## Beşinci iş (aynı oturum) — BUG-L4: model/sağlayıcı swap'ı mid-stream'de net hata mesajına dönüştürüldü (commit `07930f4`)

Kullanıcıya kalan LOW/HIGH durumu özetlendikten sonra "BUG-L4'e devam" dendi — küçük kapsamlı versiyon: in-flight isteği iptal etmek yerine (büyük mimari değişiklik, BUG_REPORT.md'nin kendisi de bunu ayrı bırakmıştı), sadece swap yüzünden başarısız olan isteklerde ham "connection refused" yerine net bir hata mesajı vermek.

`internal/app/llm.go`'ya iki küçük karşılaştırma fonksiyonu eklendi: `clientSwapped(streamClient)` ve `providerSwapped(router)` — çağrının başında `clientMu`/`providerMu` altında yakalanan kopyayı, hata anında güncel `a.client`/`a.providerRouter` ile karşılaştırıyor. Bu, `callLLMStream`/`callLLM`'in **4 hata noktasına** (streaming+non-streaming × local-model+external-provider) eklendi — swap tespit edilirse `modelSwappedMidStreamMsg` ("Model veya sağlayıcı bu mesaj akarken değiştirildi...") kullanılıyor, tespit edilmezse davranış aynen eskisi gibi.

`TestClientSwapped`/`TestProviderSwapped` (yeni `llm_test.go`) karşılaştırma mantığını doğrudan test ediyor — tam bir concurrent integration testi (gerçek bir stream'i ortasında swap ederek) flaky olma riski yüksek olduğu için tercih edilmedi, yeni eklenen TEK karar mantığı (`clientSwapped`/`providerSwapped`) zaten doğrudan test edildiği için bu yeterli görüldü.

`AGENTS.md`'nin "Data Races" notu güncellendi — BUG-L4 artık düzeltilmiş olarak işaretli.

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil.

**BUG_REPORT.md son durum:** 0 kritik, 1 HIGH (BUG-H3, Windows-only, bu ortamda test edilemez), 2 MEDIUM (M1 bakım notu, M2 kabul edilmiş), 1 LOW (BUG-L5, canlı ngrok/telefon testi gerektiriyor). Bu oturumun başında (Session 20'nin bıraktığı yerden) **10 açık madde** vardı, şimdi **4**'e indi.

**Bu noktaya kadar toplam** (bu oturum devam etti, aşağıda 3 iş daha var — nihai toplam dosyanın sonunda): 10 kod commit'i (chunking, CLI fresh-start+sync, embedding port fix, 3×LOW, Faz 1, BUG-H1, BUG-H2, BUG-L4) + 11 docs commit'i.

## Altıncı iş (aynı oturum) — v3.3.3 sürüm notları güncellendi (EN+TR)

Kullanıcı `versinNote/v3.3.3.md` ve `versinNote/tr/v3.3.3.md`'yi bu oturumdaki kullanıcı-görünür değişikliklerle güncellemeyi istedi. Sadece gerçek kullanıcı etkisi olanlar eklendi, iç/nadir-edge-case düzeltmeler (BUG-L2 güvensiz cast, BUG-L3 auto-shutdown yarışı, chunking token-fix) release notlarına girecek kadar kullanıcı-görünür değil diye bilinçli atlandı:

- Yeni bölüm: "Fixed: Embedding Model Could Get Stuck Until Reboot" (bu oturumun embedding port fix'i)
- Yeni bölüm: "Changed: The Terminal CLI Always Starts a Fresh Chat" (CLI fresh-start + `/session`'ın artık her sohbeti göstermesi)
- Small Fixes'e 3 yeni madde: izin diyaloğunun stream durunca/sohbet değişince kendini kapatması (BUG-L1), chat-switch race'in düzeltilmesi (BUG-H1+H2, tek kullanıcı-görünür cümleye birleştirildi), model/sağlayıcı swap'ında net hata mesajı (BUG-L4)

Sürüm tarihi (10 Temmuz 2026) ve version.json'a **dokunulmadı** — Session 20'nin kendi docs commit'i de aynı dosyaya, tarihi değiştirmeden eklemişti; bu, aktif geliştirme sırasında release notlarının biriktirilip tarihin/version bump'ının sadece gerçek `memo-release` skill'i çalıştırıldığında yapıldığı kurulmuş bir örüntü. Bu oturumda `memo-release` skill'i çalıştırılmadı, sadece not dosyaları düzenlendi.

**Doğrulama:** Sadece markdown içerik değişikliği, kod dokunulmadı — `go build`/`flutter analyze` gerektirmiyor. İki dosya da manuel olarak yeniden okunup akış/ton tutarlılığı kontrol edildi.

## Yedinci iş (aynı oturum) — Plain Chat'te agent/web-arama farkındalığı yoktu (commit `6209f5e` + `941dae2`)

Kullanıcı `test_sohbet_memo.md` diye bir GUI sohbet transkripti paylaştı (attach) — normal Chat sekmesinde modele bir Python dosyası oluşturmasını ve web'de arama yapmasını istemiş, Memo ikisine de "böyle bir yeteneğim yok, feature request olarak Buğra'ya gider" diye yanıt vermiş. Kullanıcı bunun doğru olup olmadığını sordu, sinirliydi.

**Araştırma (agent + doğrudan kod okuma):** İki ayrı gerçek buldu:
1. **Agent modu** gerçekten Chat ekranından ulaşılamıyor — ayrı bir "Agent" sekmesine geçmek gerekiyor. Kodda zaten yazılmış bir `AgentModeToggle` widget'ı vardı ama **hiçbir yere bağlanmamıştı** (dead code, sıfır importer).
2. **Web arama** ise modelin iddiasının aksine Chat ekranında zaten vardı (üst barda 🌐 ikonu) — model burada düpedüz yanlış cevap vermiş, sadece o sohbette kapalıydı.
3. **Kök sebep (ikisi için de):** Sistem promptu, agent modu/web arama kapalıyken bunlardan **hiç bahsetmiyordu** — sadece açıkken ekstra talimat ekleniyordu (`buildAgentSystemPrompt` chat.go'da, canlı arama sonuçları helpers.go'da). Kapalıyken modelin elinde "bu özellik var ama kapalı" diyebileceği hiç bilgi yoktu.

**Fix A — backend (`identity.go`):** `BuildSystemPrompt`'a `agentEnabled`/`webSearchEnabled` parametreleri eklendi, yeni `buildCapabilitiesBlock` sadece **kapalı olan** özellikleri isimlendiriyor (açık olan zaten başka yerde detaylı anlatıldığı için tekrar etmiyor). `buildOriginBlock` gibi MinimalMode'da tamamen atlanıyor. 2 üretim çağrı yeri (`chat.go`, `helpers.go`) ve 8 mevcut test call site'ı güncellendi, 3 yeni test eklendi.

**Fix B — frontend (`chat_screen.dart`):** Chat üst barına, web-arama ikonunun hemen yanına yeni bir agent-modu toggle IconButton'ı eklendi (`Icons.smart_toy`, aynı stil). Kullanılmayan `AgentModeToggle` widget'ı silindi (gerçekten dead code olduğu doğrulandıktan sonra) — onun yerine mevcut butonlarla birebir stil tutarlılığı olan basit bir IconButton yazıldı. Session-scoped (web-arama toggle'ı gibi) — sohbet değiştirip geri dönünce o sohbetin gerçek tipine göre sıfırlanıyor, kalıcı değil.

**Uçtan uca doğrulama (bu oturumda ilk kez gerçek GUI'yi çalıştırarak):** `flutter build linux --debug` ile gerçek Linux binary'si derlendi, geçici bir headless backend başlatılıp binary gerçekten çalıştırıldı, `PIL.ImageGrab` ile ekran görüntüsü alınıp yeni butonun doğru konumda/ikonla/renkte render olduğu görsel olarak doğrulandı (`import`/`xwd` çalışmadı, Python PIL çözüm oldu). Ekran görüntüsünde kullanıcının **başka bir terminal/agent oturumunda** yazdığı, bu konuşmayla ilgisiz bir CLI-embedding şikayeti de görüldü — ona müdahale edilmedi, sadece not düşülüyor (kullanıcı muhtemelen paralel bir oturumda başka bir agent'a da bir şey yazdırıyor).

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil (identity dahil, 3 yeni test). `flutter analyze lib/` → aynı 4 bilinen uyarı. `flutter test` → 99/99 yeşil. Test/backend/screenshot süreçleri ve geçici dosyalar oturum sonunda temizlendi.

## Sekizinci iş (aynı oturum) — CLI'de "embedding açık görünüyor ama hafıza çalışmıyor" (commit `40ef7e2`)

Kullanıcı, ekran görüntüsünde gördüğüm başka bir terminal oturumunda yazdığı şikayeti bu sefer doğrudan bana da yazdı: CLI modunda embedding modeli açık görünüyor ama hafıza arama hiçbir şey bulamıyor, "kaydedildi" diyor ama gerçekte kaydetmiyor; `/embedding` yazınca düzeliyor; GUI'de bu sorun hiç yok. Bir agent'la araştırıp kendim de doğrudan koddan ve gerçek config dosyalarından teyit ettim.

**Kök sebep:** `Startup()`, hafıza deposunu ana chat client'ı (`a.client`) ile **placeholder** bir embed fonksiyonuna bağlıyor — gerçek embedding client'a yalnızca `StartEmbeddingModel` çalışırsa geçiyor. Bu da `cfg.Memory.EmbeddingAutoStart`'a bağlı, ki bu alanın **hiçbir varsayılanı yok** (zero-value `false`) ve makinedeki üç gerçek config.yaml'ın (GUI'nin kullandığı `~/.memo/config/config.yaml` dahil) **hepsinde** `embedding_auto_start: false` olarak doğrulandı. Bu arada `llama.Server.GetStatus()`'un `pingPort()` fallback'ı — portta *herhangi bir şey* yanıt verirse "running" diyor, ama hafıza deposunu asla o porta **bağlamıyor** — sadece kozmetik bir durum. CLI'nin welcome banner'ı tam olarak bu yanıltıcı durumu gösteriyordu. GUI'de sorun görünmemesinin sebebi muhtemelen kullanıcının Models sekmesinden elle bir model başlatmış olması, bu da tesadüfen doğru wiring'i tetikliyor.

**Düzeltme:** `Startup()`'a yeni `reconnectEmbeddingIfAlreadyRunning()` eklendi — placeholder store hazır olduktan SONRA (bir channel ile sıralanıyor, yoksa daha yavaş olan placeholder goroutine'i sonradan bitip doğru wiring'i geri placeholder'a çevirebilirdi) — configured portta zaten canlı ama bu process'e track edilmemiş bir embedding sunucusu varsa, `Start()`'a hiç gitmeden (o `killByPort` çağırıp gayet iyi çalışan sunucuyu öldürüp yeniden başlatırdı) doğrudan gerçek client'ı bağlıyor ve `reinitMemoryStore` çağırıyor. Bilinçli olarak `EmbeddingAutoStart`'a bağlı değil — o bayrak "yeni bir model süreci başlat" kararını kontrol ediyor (kaynak/rıza kararı), zaten çalışan bir şeye yeniden bağlanmak çok daha düşük riskli, farklı bir eylem.

**Doğrulama:** İki yeni test (`TestReconnectEmbeddingIfAlreadyRunning_WiresUpExternalServer`/`_NoopWhenPortIsEmpty`) fonksiyonu izole test ediyor. Ayrıca **gerçek derlenmiş binary ile uçtan uca doğrulandı**: embedding portunda sahte bir "zaten çalışıyor" sunucusu başlatılıp, TEMİZ bir backend başlatıldı — log satırı ("found an already-running embedding server... reconnecting") ve `/api/models/embedding/status` endpoint'i üzerinden gerçekten yeniden bağlandığı kanıtlandı. `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil.

## Dokuzuncu iş (aynı oturum) — GUI'de durdurma butonu takılı kalıyor (araştırma sürüyor) + CLI'de "hafıza kaydedildi" onayı çoğu mesajda hiç görünmüyor (commit `3170fae`)

Kullanıcı iki ekran görüntüsü paylaştı. **Birincisi (`hataa.png`):** GUI'de bir mesaj tam yanıtlanmış görünüyor ama gönder butonu hâlâ "durdur" (kırmızı kare) ikonunu gösteriyor — kullanıcı "eskiden yoktu bu bug" dedi (bilinçli regresyon iddiası). **Araştırma:** `MessagesNotifier.sendMessage`'ın normal (dispose olmamış) tamamlanma yolunu gerçek bir stream ile izole test ettim — `isSendingProvider` doğru `false`'a dönüyor, yani bugünkü BUG-H2 düzeltmem bunun sebebi değil. Kullanıcı butonun **süresiz** takılı kaldığını (geçici bir yavaş-bağlantı-kapanması değil) doğruladı — bu ortamda klavye/mouse simülasyonu yapamadığım için (xdotool/pyautogui yok) gerçek GUI'de tekrar üretemedim. **Backend log'u istendi, kullanıcı henüz paylaşmadı** — bir sonraki oturum/mesajda gelirse kesin kök nedeni bulmak için kullanılmalı. Şu anki en güçlü teori: dış sağlayıcı (OpenCode Zen) tarafında bir stream, hiçbir `Done:true` chunk'ı frontend'e ulaşmadan (`providerCtx.Done()` context iptali `outCh`'a hiç yazmadan `return` ediyor olabilir, `internal/app/llm.go`'nun `callLLMStream` dış-sağlayıcı dalında) sonlanıyor olabilir — doğrulanmadı, sadece hipotez.

**İkincisi (`logs.png`, gerçek CLI transkripti + Ayarlar → Bellek Debug ekran görüntüsü):** Kullanıcı, CLI'de sadece **ilk** mesajdan sonra "✓ hafıza kaydedildi" onayının göründüğünü, sonraki mesajlarda hiç görünmediğini fark etti — ama Ayarlar'daki bellek arama debug aracı, onaysız mesajları (ör. "bisiklet kanka en sevdiğim hobi mtb") yüksek skorla (0.917) buluyordu, yani kayıt **gerçekten oluyordu**, sadece onay gösterilmiyordu. **Kök sebep bulundu ve düzeltildi:** `reportMemorySaved`, `/api/events` ring buffer'ının sadece **son** elemanına bakıyordu — ama bu ring buffer TÜM alt sistemler (mood scoring, learning/calendar intent, proaktif öneriler, sync) tarafından paylaşılıyor. `finishStream` `chat:done`'ı senkron, `memory:saved`'ı ASYNC worker'dan geç ateşliyor — normalde `memory:saved` son sırada kalırdı, ama session ilerledikçe (mood/intent gibi arka plan işleri arttıkça) araya başka bir event girip onu "son" konumdan itmesi gitgide daha olası hale geliyordu — tam gözlemlenen örüntü (ilk mesaj güvenilir, sonrakiler gitgide güvenilmez). Düzeltme: yeni `memorySavedSince` fonksiyonu tüm event listesini tarayıp `before`'dan sonra **herhangi bir yerde** `memory:saved` var mı diye bakıyor, sadece son elemana değil. Eski mantığın (`events[len-1]` kontrolü) tam bu senaryoda gerçekten `false` döndürdüğü ayrı bir script ile kanıtlandı.

**Testler:** `TestMemorySavedSince_*` (5 test) — araya başka event giren durum, turn-öncesi bayat kayıt, ring'den düşmüş `before`, hiç kayıt olmayan durum dahil.

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil.

## Onuncu iş (aynı oturum, gece — otonom `/loop`) — GUI durdurma butonu bug'ının kök nedeni bulundu ve düzeltildi (commit `5ea0dd2`)

Kullanıcı "uyuyorum, uyanana kadar düzelt" diyerek beni `/loop` skill'i ile otonom bıraktı; `opencode` CLI'ı tekrar denememem, soru sormamam, push etmemem söylendi. Dokuzuncu iş'te bırakılan hipotez ("dış sağlayıcı tarafında bir stream, hiçbir `Done:true` chunk'ı frontend'e ulaşmadan `providerCtx.Done()` erken-dönüşüyle sonlanıyor olabilir") **doğru çıktı** — ama tek bir yerde değil, tüm stream pipeline'ında (backend'den Flutter'a giden her hop'ta) aynı desen tekrarlanıyordu.

**Kök sebep:** Pipeline'daki her SSE üretici/aktarıcı, `select { case <-ctx.Done(): return; case chunk, ok := <-ch: ... }` şeklinde ctx iptalini chunk almaya karşı **yarıştırıyordu**. Go, aynı anda hazır olan `select` dallarından **rastgele** birini seçer — yani `ctx.Done()` tam da son `Done:true` chunk'ının da hazır olduğu anda tetiklenirse, chunk %50 ihtimalle **sessizce kayboluyordu**. Frontend hiçbir zaman `"done":true` satırını görmüyor, `isSendingProvider` sonsuza kadar `true` kalıyor, gönder butonu durdur ikonunda takılı kalıyordu. Bu desen 4 katmanda tekrarlanmıştı: `internal/provider/{openai,gemini,claude}.go`'nun `processSSE`'si (gönderme tarafı), `internal/app/llm.go`'nun `agentLoop`/`providerLoop`/`localLoop`'u (alma tarafı), `internal/app/chat.go`'nun stream sarmalayıcıları (aktarma tarafı), ve en dıştaki `internal/webserver/handlers_flutter.go`'nun `handleSendStream`/`handleSendFileStream`/`handleWhatsAppChatStream`'i (HTTP/SSE yanıtına yazma tarafı — Flutter'a giden son hop).

**Düzeltme:** Her katmanda "önce bloklamayan bir deneme, sadece o başarısız olursa ctx-farkında bloklayan `select`'e düş" deseni uygulandı — hazır bir değer, yarışan bir iptalden her zaman önceliklendiriliyor:
- `internal/provider/provider.go`: yeni paylaşılan `trySend` helper'ı, `openai.go`/`gemini.go`/`claude.go`'daki tüm racy `select`'lerin yerini aldı.
- `internal/app/llm.go`: `trySend`'in gövdesi düzeltildi (fonksiyon zaten vardı, sadece body buggy'ydi), yeni generic `recvChunk[T any]` helper'ı eklendi ve `agentLoop`/`providerLoop`/`localLoop`'a uygulandı.
- `internal/app/chat.go`: yeni `forwardStream` helper'ı, 6 özdeş inline racy pattern + 1 farklı şekilli (web-search dalı) örneği değiştirdi.
- `internal/webserver/handlers_flutter.go`: yeni paylaşılan `streamSSE` helper'ı, üç handler'daki özdeş racy for-select döngülerinin yerini aldı.

**Testler:** Her üç pakette (`internal/app`, `internal/provider`, `internal/webserver`) eski buggy tek-`select` deseni izole şekilde yeniden üretilip (`naiveSendOrCancel`/`naiveSSELoop`), ctx zaten iptal edilmişken hazır bir chunk verildiğinde 2000 (webserver'da 500) denemede ne sıklıkla düştüğü ölçüldü: **provider %48 (958/2000), app %51 (1021/2000), webserver %46 (230/500)** — yani hipotez sadece doğru değil, neredeyse yazı-tura kadar güvenilmezmiş. Aynı senaryoda gerçek düzeltilmiş fonksiyonlar (`trySend`, `recvChunk`, `streamSSE`) **0/2000 ve 0/500** kayıpla geçti.

**Doğrulama:** `CGO_ENABLED=1 go build/vet/test ./... -race -count=1` → tüm paketler yeşil (frontend'e dokunulmadı, `flutter analyze`/`test` gerekmedi). Watchdog mitigasyonu (kullanıcının önceden onayladığı, `isSendingProvider`'ı 320s sonra zorla `false`'a çeken client-side timer) **eklenmedi** — kök sebep bulunup düzeltildiği ve regresyon testleriyle kanıtlandığı için görev talimatındaki "ya kök-sebep düzeltmesi ya watchdog" ayrımına göre gerek kalmadı.

**Not:** `internal/provider/gemini.go`'da fark edilen ayrı bir asimetri (tool-only/boş içerikli yanıtlarda `fullContent.Len() == 0` ise stream hiç `Done` chunk'ı göndermeden kapanıyor) — incelendi, bu durum `internal/app/llm.go`'nun `providerLoop`'unda zaten telafi ediliyor (`!ok` ile kanal kapanması durumunda `fullReply.Len()` kontrolüyle her hâlükârda bir `Done` chunk'ı gönderiliyor), yani canlı bir bug değil — düzeltme kapsamına alınmadı.

## Oturum kapanışı — nihai durum (durdurma-butonu bug'ı artık kapalı)

**Toplam (chunking doğrulamasından bu satıra kadar, doğrudan doğrulanabilir):** 15 kod commit'i (chunking, CLI fresh-start+sync, embedding port fix, Faz 1, 3×LOW, BUG-H1, BUG-H2, BUG-L4, agent-toggle, capabilities-block, embedding-reconnect, memory-saved-onayı, durdurma-butonu race fix) + 16 docs commit'i, artı kullanıcının kendi attığı 2 commit (`19280ca` versiyon bump'ı, `10976a3` `.gitignore`). `docs/plans/PLAN_chatid_refactor.md`'nin Faz 1'i tamamlandı, Faz 2 kısmen (bkz. Dördüncü iş).

**Push durumu:** `origin/main` şu an `19280ca`'ya (versiyon 3.3.3 bump'ı) kadar push edilmiş durumda. **7 commit hâlâ local'de, push edilmedi:** `6209f5e`, `941dae2`, `e2f4a86` (Yedinci iş), `40ef7e2`, `8a6dbca` (Sekizinci iş), `3170fae` (Dokuzuncu iş — memory-saved onayı), `5ea0dd2` (Onuncu iş — durdurma-butonu race fix). Otonom görev talimatı gereği push edilmedi (kullanıcı uyanınca kendi kararına bırakıldı).

**BUG_REPORT.md nihai durum:** 0 kritik, 1 HIGH (BUG-H3, Windows-only), 2 MEDIUM (M1 bakım notu, M2 kabul edilmiş), 1 LOW (BUG-L5, canlı test gerektiriyor) — toplam 4 açık madde. GUI durdurma butonu takılı kalması artık **kapalı** (Onuncu iş, kök sebep bulundu ve düzeltildi) — BUG_REPORT.md'ye hiç eklenmemişti, eklemeye gerek kalmadı.

**Sıradaki oturum için:**
1. **Kullanıcı gerçek GUI'de doğrulasın** — bu düzeltme statik analiz + izole regresyon testleriyle doğrulandı (bu ortamda xdotool/pyautogui yok, gerçek Flutter GUI'yi tıklayarak test edemedim); kullanıcının kendi ortamında birkaç mesajlık normal kullanımda buton davranışını teyit etmesi gerekiyor.
2. **7 commit'i push et** — kullanıcıya sorulmadan yapılmadı, bilinçli.
3. **`PLAN_chatid_refactor.md` Faz 3** (task loop workaround temizliği) — önce Faz 2'nin asıl istediği, dışarıdan explicit `chatID` kabul eden public `SendMessageStreamTo` API'si eklenmeli.
4. **BUG-H3** (Windows auto-shutdown) — gerçek Windows makine/VM gerekiyor.
5. **BUG-L5** (ngrok token yarışı) — canlı ngrok + telefon testi gerektiriyor.
6. `test_sohbet_memo.md`, `hataa.png`, `logs.png` hâlâ untracked duruyor (kullanıcının attach ettiği dosyalar) — commit edilmedi, silinmedi.

---

# Handoff — 2026-07-11 (Session 20) — İki yeni provider + auto-permission race + BUG_REPORT.md'deki tüm kritik/HIGH/MEDIUM maddelerin adım adım temizliği

## Oturum Özeti

Üç ayrı istekle ilerleyen uzun bir oturum: (1) iki yeni AI provider eklendi (backend+frontend), (2) Shift+Tab auto-permission modunun neden çalışmadığı araştırılıp gerçek kök nedeni (bir data race) bulunup düzeltildi, (3) kullanıcı "BUG_REPORT.md'deki kritik hataları adım adım düzelt, her adımda commit at, dosyayı güncel tut" dedi — bunun üzerine 2 kritik + 3 eski-HIGH + 7 MEDIUM madde tek tek düzeltildi, her biri kendi testi ve kendi commit'iyle, ardından ayrı bir docs commit'iyle BUG_REPORT.md'den çıkarıldı. Toplam **14 kod commit'i + 12 docs commit'i**. `BUG_REPORT.md`'nin açık madde sayısı **22 → 10** düştü (0 kritik, 3 HIGH — bilinçli atlandı, 2 MEDIUM — biri gerçek bug değil, 5 LOW — hiç dokunulmadı).

Tüm adımlarda doğrulama: `CGO_ENABLED=1 go build/vet/test ./... -race` ve `flutter analyze`/`flutter test` her commit'ten önce yeşil görüldü; Windows cross-derleme (`GOOS=windows go vet`) `main.go` değişen commit'lerde ayrıca kontrol edildi.

---

## 1) İki yeni provider: OpenCode Zen ve OpenCode Go (commit `9988bb1`)

Kullanıcı iki yeni provider istedi, modellerin OpenRouter'daki gibi **elle yazılmadan**, API'den dinamik listelenmesini istedi.

- **Backend** (`internal/provider/`): `opencode_zen.go`/`opencode_go.go` — OpenRouter/Ollama'nın kullandığı `openAIProvider` sarmalama deseniyle, Bearer auth + OpenAI-uyumlu `/chat/completions`+`/models`. Base URL'ler: `https://opencode.ai/zen/v1` (Zen — kullandığın kadar öde, bazı modeller ücretsiz) ve `https://opencode.ai/zen/go/v1` (Go — abonelik). Yeni **jenerik** `POST /api/providers/models` endpoint'i eklendi (`handlers_flutter.go`) — `provider.NewProvider(cfg).ListModels()`'i çağırıyor; OpenRouter'ın kendine özel zengin endpoint'ini kopyalamak yerine herhangi bir OpenAI-uyumlu provider için tek, yeniden kullanılabilir mekanizma.
- **Frontend**: `provider_config.dart`'a iki yeni provider girdisi, `api_client.dart`'a `fetchProviderModels()`, `provider_config_dialog.dart`'a "Seç" butonu artık `openrouter`'a özel değil (`hasModelBrowser()` ile genel), yeni `_SimpleModelBrowserDialog` (fiyatsız, sade model-ID listesi, arama filtreli).
- **Bilinçli dokunulmayan yer**: `chat_input.dart`'taki OpenRouter'a özel "OAuth ile hızlı bağlan" akışı — OpenCode için eşdeğeri yok, istenmedi.

## 2) Shift+Tab auto-permission modu çalışmıyordu (commit `56c82bf`)

Kullanıcı: "agent modunda Shift+Tab ile auto mode var, çalışmıyor." Önce klavye kısayolunun kendisini şüpheli buldum (izole widget testiyle doğru çalıştığını kanıtladım), sonra kullanıcıdan "bir şey değişiyor gibi ama tool çağrıları hâlâ izin soruyor" bilgisini alınca gerçek kök nedeni buldum:

`internal/agent/executor.go`'daki `RunStream`, `e.bypassPermissions`/`e.autoPermission` flag'lerini **kilitsiz** okuyordu — halbuki `SetAutoPermission`/`SetBypassPermissions` bunları `e.mu` altında yazıyordu. Toggle bir HTTP handler goroutine'inde çalışıyor, mesaj gönderme başka bir goroutine'de `RunStream`'i tetikliyor — Go'nun memory modeli senkronizasyon olmadan görünürlük garanti etmiyor, bu yüzden Shift+Tab'a basınca arayüz güncellenmiş görünse de agent turu başlatan goroutine eski (kapalı) değeri görüp izin ekranını göstermeye devam edebiliyordu.

**Düzeltme:** `GetBypassPermissions()` adında (mevcut `GetAutoPermission()` ile aynı kilitleme deseninde) yeni bir getter eklendi, `RunStream` artık ham alan yerine iki kilitli getter'ı kullanıyor. Regresyon testi (100 goroutine'le eşzamanlı Set/Get) `-race` altında ekli.

---

## 3) BUG_REPORT.md temizliği — kullanıcı isteğiyle, adım adım

Kullanıcı: "BUG_REPORT.md'deki kritik hataları düzeltelim, adım adım, paralel agent çalıştırma sen yap, her adımda commit at, BUG_REPORT.md'yi güncel tut." Aşağıdaki sıra izlendi: kod düzelt → test yaz/doğrula → commit → BUG_REPORT.md'den o maddeyi kaldır → ayrı docs commit'i. HIGH'ın 3 maddesi bilinçli atlandıktan sonra kullanıcı onayıyla MEDIUM'a geçildi.

### 🔴 CRITICAL (2/2 düzeltildi)

**BUG-C2 — `rm -rf` kara liste bypass'ı** (`de4450e`)
`internal/agent/tools/command.go`'daki `\brm\s+-rf\s+/\b` deseni, `/`, `~`, `.`'nin **non-word karakter** olması nedeniyle `\b` sınırının hiç oluşmamasından dolayı "rm -rf /", "rm -rf /*", "sudo rm -rf /" gibi tam olarak engellemesi gereken komutları **hiç yakalamıyordu** — ama "rm -rf /home/user/foo" gibi göreceli güvenli, derin bir yolu engelliyordu. Üç desen de aynı hatadan muzdaripti (`/`, `~`, `.`). Düzeltme: `\b` yerine açık bir terminator sınıfı (`$`|boşluk|`;`|`&`|`|`|`*`) — hem gerçek wipe'ları yakalıyor hem de `./build`, `.git` gibi scoped silmelere dokunmuyor.

**BUG-C1 — Uzak erişimde (LAN/ngrok) sıfır kimlik doğrulama** (`f5a579e`)
Üretilen `RemoteAccess.Token` hiçbir handler'da karşılaştırılmıyordu — LAN/ngrok erişimi olan biri `/api/wipe`, `/api/agent/permission` (→ host'ta keyfi komut), `/api/import`, `/api/shutdown` dahil her şeye kimlik bilgisi olmadan erişebiliyordu. `remoteAuthMiddleware` eklendi: sunucu `0.0.0.0`'a bağlandığında (LAN modu VEYA ngrok — ikisi de aynı bind'i kullanıyor) her istek `X-Memo-Token` ya da `Authorization: Bearer <token>` istiyor, sabit-zamanlı karşılaştırmayla. Localhost-only mod (varsayılan) tamamen etkilenmedi. Mobile client zaten `X-Memo-Token` gönderiyordu (ölü kod, hiç kontrol edilmediği için); desktop client'a aynı mekanizma eklendi — `PUT /api/remote-access` artık token'ı içeren tam durumu döndürüyor (eskiden bare `{"ok":true}`), böylece desktop client token'ı tam da bağlantının 0.0.0.0'a geçtiği anda, hâlâ yetkisiz olan o istekten yakalıyor.

Bu düzeltmenin **yan etkisi**: eski BUG-H1 (kimlik doğrulamasız GET ile provider anahtarlarının okunabilmesi) de otomatik kapandı, çünkü yeni middleware tüm HTTP metodlarını aynı şekilde koruyor.

**Bilinen dar kapsamlı takip maddesi (BUG-L5, düzeltilmedi):** ngrok otomatik-başlatma açıksa ve uygulama yeniden başlatılırsa, masaüstü GUI'si restart sonrası token'ı henüz öğrenmeden ilk isteğini atıp 401 alabilir (workaround: Ayarlar'dan Uzak Erişim'i kapat/aç). Gerçek çözüm ya ngrok trafiğini loopback'ten güvenilir şekilde ayırt etmeyi (mümkün değil, ngrok zaten `127.0.0.1`'e bağlanıyor) ya da Tailscale'in zaten kullandığı reverse-proxy desenine geçişi gerektiriyor — canlı ngrok/telefon testi gerektirdiği için bu oturumun kapsamı dışında bırakıldı.

### 🟠 eski HIGH (3/6 düzeltildi, 3'ü bilinçli atlandı)

**eski BUG-H1 — SQLite dosyaları 0644 (dünyaya-okunabilir)** (`7e8860e`)
`memory.db`, `mood.db`, `observations.db`, `events.db`, `messages.db`, `session.db` (WhatsApp'ın Signal/Noise oturum anahtarları!) hepsi process umask'ında (`-rw-r--r--`) kalıyordu — mattn/go-sqlite3 dosyayı kendi yaratıyor, Go tarafından perm parametresi geçirilemiyor. `internal/database.Open` (memory/calendar/observer'ın paylaştığı wrapper) artık `Ping()` sonrası `os.Chmod(path, 0600)` çağırıyor; ayrıca `sql.Open`'ı doğrudan kullanan `mood`/`whatsapp` paketlerine ve whatsmeow'un kendi `sqlstore.New` çağrılarına da aynı chmod eklendi.

**eski BUG-H2 — Agent sandbox "bypass"** (`c59b459`) — **meğerse gerçek değilmiş.**
İddia: `Sandbox.ValidatePath`, proje dizini dışındaki, kısa bir protected-list'te olmayan mutlak yolları (`/srv/`, ikinci disk vb.) serbest bırakıyor. Tüm repo'da grep ile doğrulandı: bu fonksiyonun **hiçbir çağıranı yok** — dead code. Gerçek dosya araçlarının kullandığı `internal/agent/tools/file.go`'daki AYRI `validatePath` fonksiyonu zaten doğru: protected-list eşleşmese bile basePath dışını koşulsuz reddediyor (protected-list sadece daha spesifik bir hata mesajı için). Dead+hatalı kod silindi (`ProtectedPaths` alanı ve `defaultProtectedPaths()` de dahil), gerçek yolun doğru davrandığını kanıtlayan testler eklendi.

**eski BUG-H3 — Streaming goroutine'lerinde panic recovery yok** (`9fb11b7`)
`internal/taskloop/engine.go`'da zaten var olan `recover()` deseni, `internal/app/llm.go`'daki 5 streaming goroutine'ine ve `agent.Pipeline.RunStream`'e uygulandı. Her `defer close(outCh)`'ten sonra `defer recoverStreamPanic(ctx, outCh, "<label>")` — defer'lar LIFO çalıştığı için recover, channel kapanmadan önce çalışıp kullanıcıya görünür bir hata chunk'ı gönderebiliyor, panic'in tüm process'i (tüm sohbetler, WhatsApp köprüsü, takvim hatırlatıcıları) düşürmesi yerine.

**Bilinçli atlanan 3 HIGH:** chat-switch sırasında mesajın yanlış sohbete karışması (backend+frontend, ikisi birden) — AGENTS.md'nin "Known Open Work" listesindeki tek-global-aktif-chat mimarisi sorununun belirtisi, düzgün çözümü büyük bir refactor; Windows'ta auto-shutdown çalışmaması — bu ortamda (Linux) hiç test edilemez, canlı bir Windows makine gerektirir.

### 🟡 MEDIUM (7/9 düzeltildi, 2'si zaten bug değildi/kabul edilmişti)

**eski M3 — Websearch/hafıza ayarları kilitsiz r/w** (`eeee9e2`) — `a.cfg.Memory.MemoryEnabled`/`a.cfg.WebSearch.Enabled` `cfgMu` altında yazılıyor ama 6 dosyada (`helpers.go`, `chat.go`, `models.go`, `llama.go`, `app.go`) kilitsiz okunuyordu. `GetMemoryEnabled()` artık kilitli, yeni `GetWebSearchEnabled()` eklendi, tüm okuma noktaları bu getter'lara yönlendirildi.

**eski M4 — Minimal Mod iki farklı, senkronize olmayan kopyadan okunuyordu** (`095565a`) — `identity.Identity.MinimalMode` (kilitsiz) ve `a.cfg.Identity.MinimalMode` (ayrı kopya) `buildMessages` içinde farklı anlarda okunuyordu; bir toggle iki yazım arasına denk gelirse yarı-uygulanmış (identity bloğu atlanmış ama mood/websearch hâlâ enjekte edilmiş, ya da tersi) bir prompt üretilebiliyordu. `Identity`'ye kilit eklendi, `buildMessages` artık TEK kaynaktan (`a.identity.GetMinimalMode()`) okuyor — `a.cfg.Identity.MinimalMode` sadece persistence/API durumu için kalıyor.

**eski M3 — Hafıza consolidation'ı embedding hatasında sessizce vektör aramadan düşüyordu** (`86b9a09`) — `saveMerged`, embed başarısız olursa birleşmiş anıyı embedding'siz kaydediyordu (doğru davranış — merge'ü tamamen kaybetmek daha kötü olurdu) ama **hiçbir log satırı yoktu**. İki log satırı eklendi (embed hatası / boyut uyuşmazlığı), davranış değişmedi, artık gözlemlenebilir.

**eski M4 — İzin diyaloğu, gönderim başarısız olsa bile başarılıymış gibi kapanıyordu** (`0a3acd1`) — `_submit`, POST'u `unawaited()` ile ateşleyip koşulsuz `Navigator.pop` çağırıyordu; başarısız olursa kullanıcı hiçbir şey görmüyordu, backend'deki tool call kendi timeout'una kadar askıda kalıyordu. `_submit` artık async: başarıda pop, başarısızlıkta diyalog açık kalıyor + hata gösteriliyor + buton spinner'ı.

**eski M3 — Hafıza/Minimal Mod anahtarına hızlı çift-tıklama yanlış son duruma yol açabiliyordu** (`c022e1b`) — `toggle()`, `state`'i kendi `await`'i bitmeden güncellemiyordu; iki hızlı tık aynı bayat değeri okuyup aynı yöne toggle ediyordu (net: yanlış yön). In-flight guard (`_toggling`) + optimistic update eklendi. Test, `flutter test`'in gerçek soket bağlantılarını (127.0.0.1 dahil) engellediğini keşfetti — Dio'nun kendi `HttpClientAdapter`'ını fake'leyerek çözüldü (ek bağımlılık gerekmedi).

**eski M3 — Detached backend süreci CLI açıkken zombi kalabiliyordu** (`4f364f4`) — `spawnDetachedBackend`, `cmd.Process.Release()` kullanıyordu; `Setsid` sadece yeni bir session açıyor, OS-seviyesi parent-child ilişkisini değiştirmiyor — backend kendi kendine kapanırsa ve CLI hâlâ açıksa, hiç kimse `wait()` çağırmadığı için zombi kalıyordu. `reapInBackground(cmd)` eklendi — `cmd.Wait()`'i arka planda çağırıyor, çağırıcıyı bloklamadan, backend'in bağımsız yaşam süresini bozmadan. `kill(pid,0)`'ın `ESRCH` dönene kadar (gerçek reap kanıtı, sadece exit değil) poll eden bir test eklendi; eski `Release()`-only deseninin gerçekten zombi bıraktığı ayrıca manuel doğrulandı.

**eski M3 — Dışarıdan SIGTERM gelirse CLI'ın unregister çağrısı hiç çalışmıyordu** (`14e545f`) — `main()`'in sinyal dalı, `replcli.Run()`'ın goroutine'ini beklemeden dönüyordu, `Run()`'ın deferred `UnregisterClient`'ı hiç çalışmıyordu (aynı kök neden, terminal-restore fix'inin zaten çözdüğü sorunla — o da goroutine'i beklemeden dönme). `Run()` artık variadic bir `onClientRegistered` callback alıyor (9 mevcut test call site'ı değişmeden derleniyor), clientID bir kanaldan (paylaşılan değişken değil — race olurdu) `main()`'e taşınıyor, sinyal dalı doğrudan unregister ediyor.

**Dokunulmayan 2 MEDIUM:** M1 (`model_store_screen.dart` 2600+ satır) gerçek bir bug değil, bakım notu; M2 (`connectionStatusProvider` sonsuz polling) zaten AGENTS.md'de "kabul edilebilir" diye işaretli.

---

## Doğrulama

Her commit'ten önce: `CGO_ENABLED=1 go build ./... && go vet ./... && go test ./... -race -count=1` (tüm paketler yeşil, her seferinde) ve dokunulan Flutter dosyaları için `flutter analyze`/`flutter test` (92 → 95 test, sadece 4 önceden var olan info-seviye uyarı). `main.go` değişen commit'lerde ayrıca `GOOS=windows go vet .` ile cross-derleme kontrolü yapıldı.

Yeni testlerin çoğu, düzeltmeden ÖNCEKİ koda karşı gerçekten fail ettiği doğrulanarak (git stash / geçici revert ile) teyit edildi — sadece "yeşil test yazdım" değil, "bu test gerçekten bu bug'ı yakalıyor" kanıtlandı.

## Sıradaki Oturum İçin

1. **Kalan 3 HIGH** — en yüksek kaldıraçlı: chat-switch race'i (backend `internal/app/chat.go` + frontend `chat_provider.dart`, ikisi birlikte ele alınmalı, muhtemelen AGENTS.md'nin "Chat-ID refactor" planıyla birleştirilmeli — `docs/plans/PLAN_chatid_refactor.md`), Windows auto-shutdown (gerçek bir Windows makine ya da VM gerektirir, bu ortamda test edilemez).
2. **5 LOW madde** hiç ele alınmadı — kullanıcı MEDIUM'dan sonra durmayı seçti.
3. **BUG-L5** (bu oturumda BUG-C1 düzeltmesinin yan etkisi olarak doğdu) — ngrok auto-start + restart kombinasyonunda masaüstü GUI'nin token'ı öğrenemeden 401 alması. Gerçek çözüm loopback/ngrok ayrımı ya da Tailscale'in reverse-proxy desenine geçiş gerektiriyor.
4. **OpenCode Zen/Go** — sadece backend + masaüstü Settings dialog'una eklendi; mobile app'e hiç dokunulmadı (istenmedi).
5. Bu oturumda push edilmedi — kullanıcı henüz istemedi, `origin/main`'in kaç commit gerisinde olduğunu bir sonraki oturumda kontrol et.

---

# Handoff — 2026-07-09 (Session 19) — REPL CLI: model indirmeyi kaldırıp GUI'ye yönlendirme + /gui path bug'ı

## Oturum Özeti

Kullanıcı `internal/replcli/`'de iki bug bildirdi: (1) model indirirken CLI "buga giriyor" / "takılıyor", (2) `/gui` komutu GUI kurulu olmasına rağmen bulamıyor. İstek: CLI'dan ağır işleri (model indirme) kaldırıp GUI'ye yönlendirmek. İki kök neden bulundu ve düzeltildi, tek commit'lik iş.

## Yapılanlar

### 1. Kök neden — `/model-download`'ın ilerleme döngüsü klavyeyi hiç okumuyordu
`trackDownloadProgress` sadece bir `ticker.C`'den okuyan bir `for` döngüsüydü — hiç klavye okumuyordu. Raw mode'da (`term.MakeRaw`) ISIG kapalı olduğu için Ctrl+C bir sinyal üretmiyor, sadece normal bir tuş basımına dönüşüyor; bu döngü o tuşu hiç okumadığı için indirme başladıktan sonra iptal etmenin **hiçbir yolu yoktu** — bağlantı yavaşlarsa/donarsa tüm REPL indirme bitene kadar tıkanıp kalıyordu. Kullanıcının "buga giriyor, takılıyor" tarifi tam olarak buydu.

**Düzeltme (mimari, patch değil):** `/model-download`'ın Hugging Face arama/seçim/indirme/ilerleme-izleme akışının tamamı kaldırıldı (`cmdModelDownload` artık argüman almıyor, `interactiveModelDownload`/`trackDownloadProgress` silindi). Artık `/model-download` kısa bir mesaj basıp `cmdGui()`'yi çağırıyor — indirme masaüstü uygulamasının Modeller sekmesinden yapılıyor (orada gerçek ilerleme çubukları var ve hiçbir şeyi bloklamıyor). Bununla birlikte artık kullanılmayan `download_client.go` (+ testi), ve sadece o akışa hizmet eden `progressBar`/`humanSize` (`color.go` + testi) de silindi.

### 2. Kök neden — `/gui` sadece kendi exe dizinine bakıyordu
Kurulu CLI `~/.memo/bin/memo`'da yaşıyor ama bundled GUI bir üst dizinde, `~/.memo/memo_flutter`'da (`get-memo.sh`'den doğrulandı). Eski `cmdGui`, `filepath.Dir(exe)` içine (yani `~/.memo/bin`'e) bakıyordu — `memo_flutter` orada hiç olmadığı için gerçek bir kurulumda GUI asla bulunamıyordu. Bu, `internal/llama`'daki `binarySearchBasesFrom`'un zaten çözdüğü, dokümante edilmiş aynı problem (AGENTS.md Gotchas: "bundled dosya aramaları exe dizininin üstüne de bakmalı").

**Düzeltme:** Yeni `guiSearchDirs(exePath)` yardımcı fonksiyonu — `internal/llama`'daki desenin aynısı — hem exe dizinine hem de üst dizinine bakıyor. `cmdGui` artık `cmd.Dir`'i bulunan GUI binary'sinin kendi dizinine ayarlıyor (CLI'ninkine değil) — Flutter build'inin `lib/`/`flutter_assets/` klasörleri binary ile aynı yerde olduğu için bu da ayrı bir gerçek düzeltme.

## Doğrulama

```
CGO_ENABLED=1 go build ./... && go vet ./... && go test ./... -race -count=1
  → tüm paketler yeşil (bu kez ngrok/whisper testleri de dahil, önceki
    oturumlarda bahsedilen ortam kaynaklı hata bu seferki koşumda çıkmadı)
gofmt -l internal/replcli/  → temiz
```
Yeni/güncellenen testler: `TestHandleCommand_ModelDownload_RedirectsToGui`, `TestGuiSearchDirs`, `TestGuiSearchDirs_RootDoesNotRepeat`; `TestHandleCommand_Gui_MissingBinary` değişmeden geçti. `internal/app/config/config.yaml` bu oturumda mutasyona uğramadı (git status temiz çıktı).

Frontend'e dokunulmadı, bu oturum sadece backend/`internal/replcli`.

## İkinci tur (aynı oturum) — 3 ek bug (kod okumasıyla bulundu)

Kullanıcı sonra "CLI'de başka ne bugları var, mantıksal kontrol et" dedi. Tüm paket taranıp 3 gerçek bug daha bulundu, hepsi düzeltildi:

### 3. `/model`/`/embedding` — 10s client timeout vs. 120-180s gerçek yükleme süresi (en kritik, indirme bug'ıyla aynı desen)
`Client.httpClient`'ın sabit 10 saniyelik timeout'u `doJSON` üzerinden `StartModel`/`StartEmbedding` için de geçerliydi. Ama backend tarafında `internal/app/llama.go:31` (`WaitReady(180*time.Second)`) ve `internal/app/embedding.go:55` (`WaitReady(120*time.Second)`) modeli tamamen yüklenene kadar senkron bekliyor. Orta/büyük bir model 10 saniyeden uzun sürerse CLI "Başlatılamadı: ... Client.Timeout exceeded" hatası basıyordu — backend arka planda yüklemeye devam edip muhtemelen başarıyla bitiriyordu. Düzeltme: `client.go`'ya sabit timeout'suz ikinci bir `*http.Client` (`longOpHTTP`) + `doJSONWith` eklendi; `StartModel`/`StartEmbedding` artık kendi context timeout'unu kuruyor (185s/125s). `commands.go`'daki `startAndReport` artık Esc/Ctrl+C ile iptal edilebilir + spinner gösteriyor.

### 4. Dışarıdan SIGTERM/SIGINT gelirse terminal raw modda kalabiliyordu
`main.go`'da `select { case <-sigCh: ... }; return` REPL goroutine'ini beklemeden dönüyordu, `replcli.Run()`'ın `defer term.Restore(...)`'u hiç çalışmıyordu. Düzeltme: `main.go` REPL goroutine'i başlamadan önce `term.GetState` ile orijinal terminal durumunu yakalıyor, sinyal dalında doğrudan `term.Restore` ile geri yüklüyor.

### 5. Çok satırlı yapıştırma satır satır ayrı mesaj olarak gönderiliyordu
Bracketed-paste modu hiç açılmıyordu, `keys.go`'daki `assemble()` her `\r`/`\n` baytını doğrudan `keyEnter`e çeviriyordu. Düzeltme: `repl.go` artık raw moda girerken `ESC[?2004h` ile bracketed paste açıyor, `keys.go`'ya yeni `keyPaste` türü + `readBracketedPaste`/`collapsePasteNewlines` eklendi.

## Üçüncü tur (aynı oturum) — Backend süreç modeli: referans sayımlı client registry

Kullanıcı `--port 8090` ile `go run .` çalıştırınca REPL açılmasına şaşırdı, bunu açıklarken asıl mimari isteğini netleştirdi: **CLI açıkken Flutter açılıp CLI kapatılırsa Flutter çökmemeli, tam tersi de geçerli, ama ikisi de kapandığında backend gereksiz yere arka planda kalmamalı.** Referans sayımlı bir lifecycle tasarlandı ve uygulandı:

1. **Backend: client registry** (`internal/app/clients.go`) — `RegisterClient`/`HeartbeatClient`/`UnregisterClient`, 15sn'de bir eski (90sn+) client'ları temizleyen sweep goroutine. Registry boşalınca (`sawClient=true` ve `autoShutdown=true` ise) backend kendine `os.Interrupt` gönderiyor — `/api/shutdown`'ın kullandığı aynı mekanizma. `EnableAutoShutdown()` sadece CLI'nin kendi başlattığı backend'de açılıyor; plain `--headless` (bağımsız servis) hiçbir zaman kendi kendine kapanmıyor.
2. **Backend: 3 yeni endpoint** (`internal/webserver/handlers_clients.go`) — `POST /api/clients/{register,heartbeat,unregister}`, `AppBridge` interface'ine eklendi.
3. **`main.go`: gerçek process ayrımı** — interaktif `memo` artık backend'i in-process başlatmıyor, `spawnDetachedBackend()` ile kendi binary'sini `--headless --port N --auto-shutdown` ile ayrı, detached process olarak başlatıyor (`main_unix.go`: `Setsid`, `main_windows.go`: `CREATE_NEW_PROCESS_GROUP`).
4. **CLI tarafı** (`internal/replcli/clients_client.go`, `repl.go`) — `Run()` başlarken register oluyor, 25sn'de bir heartbeat, çıkışta unregister.
5. **Flutter tarafı** (`api_client.dart`, `chat_provider.dart`'ın `connectionStatusProvider`'ı) — zaten var olan 30sn'lik polling genişletildi: ilk erişilebilir tick'te register, sonra her tick'te heartbeat.

### Doğrulama (round 2+3)
`CGO_ENABLED=1 go build/vet/test ./... -race` → tüm paketler yeşil. `flutter analyze/test` → temiz, 91/91. Gerçek derlenmiş binary ile curl üzerinden canlı doğrulama: `--auto-shutdown` + 2 client, biri ayrılınca hayatta kalıyor, ikisi ayrılınca kendi kendine kapanıyor; hiç client register olmazsa sonsuza kadar hayatta kalıyor; plain `--headless` (bayrak yok) asla kendi kendine kapanmıyor. Doğrulanmayan: gerçek Flutter penceresiyle `/gui` senaryosu (bu ortamda display yok), gerçek Windows'ta detach.

## ⚠️ Oturum İçi Veri Kaybı Olayı — ÖNEMLİ

Yukarıdaki round 2 ve round 3'ün ilk commit'leri (**hiç push edilmeden**) kullanıcı projeyi yanlışlıkla silip GitHub'dan yeniden clone edince **tamamen kayboldu** — sadece round 1'in commit'i (`b34c907`) hayatta kaldı çünkü push edilmişti. Round 2 ve round 3'ün TÜMÜ bu oturumda ikinci kez, sıfırdan yeniden yazıldı (konuşma geçmişindeki tam dosya içerikleri hafızadan reconstruct edildi) ve yeniden commit edildi.

**Güncelleme: push edildi, doğrulandı** — `git rev-list --count origin/main..HEAD` → 0, yani `origin/main` şu an bu oturumun tüm commit'lerini içeriyor (son: `635f060`). Riskli pencere kapandı. **Ders:** bu repo'da commit'ten hemen sonra push etmeyi alışkanlık haline getir — bu oturumdaki gibi bir "yanlışlıkla sil + yeniden clone" senaryosunda push edilmemiş her şey gerçekten geri getirilemez şekilde kayboluyor (reflog bile işe yaramadı, çünkü `.git`'in tamamı silinmişti).

## Dördüncü tur (aynı oturum) — iki kısa takip konusu

1. **`go run . --port 8090` neden CLI açıyor — bug mu, kasıtlı mı?** Kullanıcı bunu görünce şaşırdı, ben de araştırıp netleştirdim: `--port` hiçbir zaman interaktiflik belirlemiyor, sadece `--headless` + `isInteractive()` (stdin gerçek tty mi) belirliyor — bu, önceki bir session'da eklenmiş, `AGENTS.md`'de zaten dokümante edilmiş kasıtlı bir tasarım. Üretimde (`run_memo.sh`'ın başlattığı `memo-backend`, AppImage/.exe) hiç sorun değil çünkü o process'in stdin'i zaten gerçek bir terminale bağlı değil (script arka planda başlatıyor) — `isInteractive()` orada baştan `false`. Sadece geliştiricinin `go run .`'ı elle bir terminalden çalıştırması bu davranışı tetikliyor. Kullanıcı önce "sadece backend açılsın" isteğini dile getirdi, sonra üretim akışının zaten böyle çalıştığını anlayınca **"dokunma, olduğu gibi kalsın"** dedi — kod değişikliği yapılmadı, sadece açıklandı.
2. **`mobile/lib/core/api_client.dart` içinde `package:dio/dio.dart` bulunamıyor hatası** — kök sebep: `mobile/` (ayrı bir Flutter projesi, `frontend/`'den farklı) içinde `.dart_tool/` hiç yoktu, yani `flutter pub get` o proje için hiç çalıştırılmamıştı (muhtemelen bu oturumun başındaki silme/yeniden-clone olayından kalma — `.dart_tool` git'e commit edilmiyor). `cd mobile && flutter pub get` ile düzeltildi, `flutter analyze` temiz. Kod değişikliği değil, sadece eksik bağımlılık kurulumu.

## Beşinci tur (aynı oturum) — EngineStrip: offline hint + hafıza uyarısı yapışıklığı

Kullanıcı bir ekran görüntüsü paylaştı: chat inputun altındaki `EngineStrip` barında "No model running · Start a model" hemen ardından boşluksuz "No memory model" görünüyordu ("birbirlerine yapışmış gibi"). Kök sebep: `engine_strip.dart`'ta hafıza uyarısından önceki divider `if (chatRunning || isApiProvider) _divider(...)` koşuluyla ekleniyordu — ama offline hint tam olarak `!chatRunning && !isApiProvider && !embRunning` durumunda gösteriliyor, yani bu koşul offline-hint senaryosunu hiç kapsamıyordu. Aynı eksik mantık indirme göstergesinin divider'ında da vardı.

Düzeltme: iki isimli boolean (`firstSlotHasContent`, `secondSlotHasContent`) — şeridin ilk/ikinci "slot"unun gerçekten bir şey render edip etmediğini doğru hesaplıyor, üç divider kontrolü de (embedding göstergesi, hafıza uyarısı, indirme göstergesi) bunları kullanıyor. `engine_strip_test.dart`'a hem divider'ın varlığını hem iki metin arasındaki piksel boşluğunu doğrulayan bir regresyon testi eklendi (91 → 92 test). `flutter analyze`/`flutter test` temiz.

Commit edildi (`43741a8`).

## Altıncı tur (aynı oturum) — Memo'nun kimliği: köken bilgisi + Minimal Mod

Kullanıcı: "Memo kendini pek tanımıyor — yaratıcın kim, amacın ne, felsefen ne diye sorunca saçmalıyor, ama akış/sohbet kalitesi harika, bunu bozmayalım." Birkaç adımda ilerledi:

1. **Köken bloğu eklendi** (`internal/identity/identity.go`'nun `buildIdentityBlock`'una) — kim yaptı/amaç/felsefe, **sadece sorulunca anlatılacak, hiç kendiliğinden konuya girmeyecek** şekilde. Aynı zamanda kullanıcının isteğiyle tüm sistem promptu Türkçe'den İngilizce'ye çevrildi (talimatlar İngilizce'de daha güvenilir takip ediliyor; sohbetin dili bundan etkilenmiyor, "hangi dilde yazılırsa o dilde cevap ver" talimatı ayrı ve hâlâ geçerli). Wizard'ın kendi persona promptlarına (`setup_wizard_view.dart`, `prompt_tr`/`prompt_en`) dokunulmadı, onlar zaten ayrı ve dil-duyarlı.
2. **Gerçek bug bulundu:** wizard'dan bir persona seçilince (`CustomRole` dolunca) `buildIdentityBlock` hiç çağrılmıyor — yani köken bilgisi kayboluyordu, tam da tester'ların çoğunun yolu. Düzeltme: köken bloğu `buildOriginBlock()` adıyla ayrı bir fonksiyona çıkarıldı, `BuildSystemPrompt`'ta `CustomRole` durumundan bağımsız her zaman ekleniyor.
3. **Token maliyeti fazla bulundu** — köken bloğu tek başına ~259 token, toplam varsayılan prompt ~735 token (4096 ctx'lik bir modelde ~%18) ölçüldü (`internal/identity`'nin kendi `truncate.EstimateTokens`'ıyla). Kullanıcı "hedef kitlemize (zayıf donanım, yerel model) ağır" dedi, haklı bulundu — köken bloğu ~99 token'a kadar kısaltıldı (toplam ~575'e düştü).
4. **Minimal Mod özelliği eklendi** — kullanıcının fikri: Ayarlar'da bir toggle, açılınca kimlik/persona/mood/web-arama enjeksiyonunun **tamamı** kapansın, sadece hafıza (ayrı toggle'ı açıksa) modele gitsin; ikisi de kapalıyken sıfır ekstra token.
   - `internal/config/config.go`: `IdentityConfig.MinimalMode bool`.
   - `internal/identity/identity.go`: `Identity.MinimalMode` alanı + `SetMinimalMode()`; `BuildSystemPrompt` açıkken identity/origin/style'ı (ve `CustomRole`'u da) tamamen atlıyor, hafızayı hâlâ dahil ediyor.
   - `internal/app/helpers.go`: `buildMessages`'ta mood directive + web arama enjeksiyonu da `MinimalMode` açıkken atlanıyor.
   - `internal/app/settings.go`: `GetMinimalMode`/`SetMinimalMode`.
   - `internal/webserver`: `GET/PUT /api/system-prompt/minimal-mode` (`FullBridge`'e eklendi, `nil_fullbridge_test.go` tablosuna eklendi).
   - Frontend: `api_client.dart` (`getMinimalMode`/`setMinimalMode`), `settings_provider.dart` (`minimalModeProvider`/`MinimalModeNotifier`, `memoryEnabledProvider` ile aynı desen), `general_tab.dart`'a Ayarlar → Genel'de yeni bir toggle bölümü, `l10n.dart`'a TR/EN string'ler.

### Doğrulama
`CGO_ENABLED=1 go build/vet/test ./... -race` → tüm paketler yeşil (whisper'daki tek hata bu oturum boyunca zaten var olan, ortam kaynaklı, ilgisiz). `flutter analyze --no-fatal-infos && flutter test` → temiz (4 önceden var olan info), 92/92 test yeşil. Yeni Go testleri: `TestBuildSystemPrompt_MinimalMode_*` (3 test, identity paketinde), `TestGetSetMinimalMode` (app paketinde), `TestHandlers_NoFullBridge/MinimalMode` (webserver). `internal/app/config/config.yaml` test koşumları sonrası her seferinde `git checkout --` ile geri alındı, commit edilmedi.

Commit edildi (`daf3428`, `0f1ee93`, `a298b3a`).

## Yedinci tur (aynı oturum) — "Stable ne kadar uzakta?" denetimi + BUG_REPORT.md temizliği

Kullanıcı: "uygulama şu an yüzde kaç stable, stable için ne gerekiyor, genel tüm bug'ları bulsana." 4 paralel ajan (bugünkü yeni kod, chat/hafıza pipeline, Flutter frontend, güvenlik) eşzamanlı tarama yaptı — **19 yeni bug** bulundu, hiçbiri bu oturumda düzeltilmedi (sadece tespit/dokümantasyon). İki en kritik bulgu bizzat kodda çalıştırılarak doğrulandı:

1. **Uzak erişim (ngrok/LAN) açıkken sıfır kimlik doğrulama** — `internal/webserver/server.go`'nun middleware zinciri (`limitBodyMiddleware(rateLimitMiddleware(...corsMiddleware(mux)))`) hiçbir auth kontrolü içermiyor; `RemoteAccess.Token` üretilip arayüze gösteriliyor ama hiçbir handler'da hiç karşılaştırılmıyor. `POST /api/wipe`, `/api/agent/permission` (→ `run_command` ile host'ta keyfi komut), `/api/import`, `/api/shutdown` hepsi kimlik doğrulamasız erişilebilir.
2. **Agent'ın `rm -rf /` kara listesi regex hatası** — `internal/agent/tools/command.go:27`'deki `\brm\s+-rf\s+/\b`, gerçek Go regex testiyle doğrulandı: `"rm -rf /"`, `"rm -rf /*"`, `"sudo rm -rf /"` — **hiçbiri eşleşmiyor**. Sadece görece güvenli alt-yollar (`rm -rf /home/user/foo`) yakalanıyor. Filtre tam olarak engellemesi gereken komutu hiç engellemiyor.

Sonuç kullanıcıya bir **Artifact** (HTML rapor, severity-kodlu, `~55/100` genel değerlendirme) olarak sunuldu. Kullanıcı Artifact linkine giremeyince, aynı raporu doğrudan repo köküne **`STABILITY_AUDIT.html`** olarak kaydettim (git'e eklenmedi, kullanıcı zaten `.gitignore`'a kendi eklemiş).

### BUG_REPORT.md tam temizlik

Kullanıcı dosyanın "çok şiş" olduğunu söyledi — haklıydı: 1312 satır, 128 madde, ama **100'ü zaten düzeltilmişti** (çoğu farklı bir işaretleme biçimiyle — `✅ (commit)` — üstteki özet tablosu bunu hiç saymıyordu, "27 açık" diyordu ama gerçek açık sayısı 4'tü). Python ile her maddenin TAM gövdesini (sadece başlığı değil) hem `~~...~~ **→ DÜZELTİLDİ/DÜZELTİLMİŞ**` hem `✅ (commit)` kalıpları için taradım, gerçek durumu çıkardım. Dosya sıfırdan yazıldı: session anlatısı yok, düzeltilmiş bug arşivi yok (git log zaten var), sadece **22 gerçek açık madde** (4 eski + 19 yeni) düz bir liste olarak kaldı — 1312 satır → 182 satır.

### İkinci doğrulama geçişi — kalan eski maddeler de tek tek koda karşı test edildi

Kullanıcı: "kalan eski bugları da incele, gerçekten var mı, AGENTS.md Known Pitfalls'ı da kontrol et." Üç bulgu:
- **"Mobile API client eksik" iddiası artık yanlıştı** — `grep`'le sayıldı: backend'in 118 endpoint'inden 111'i mobile'da zaten var; eksik 7'si ya bugünkü yeni client-registry uçları (mobile'a hiç gerekmiyor) ya da CLI-yönetim uçları (mobile'da CLI yok). BUG_REPORT.md'den kaldırıldı, AGENTS.md'deki karşılığı düzeltildi.
- **AGENTS.md'nin "data race" dediği `a.client`/`providerRouter` reassignment'ları aslında hiç race değilmiş** — hem okuma hem yazma tarafının `clientMu`/`providerMu` ile düzgün kilitli olduğu doğrulandı. Gerçek kalan (daha dar) risk: bir stream client'ı kilit altında local değişkene kopyalayıp saniyelerce tutuyor — model swap tam o sırada olursa stream eski client ile konuşmaya devam ediyor. Bu, doğru tanımıyla yeni **BUG-L4** oldu.
- **"Memory full rebuild O(N), `LoadCache`" notu tamamen bayattı** — `LoadCache` fonksiyonu artık `internal/memory/store.go`'da hiç yok (RAG mimarisi RRF/hibrit arama'ya geçeli beri kalkmış). Hiçbir yere eklenmedi, AGENTS.md'den kaldırıldı.

BUG_REPORT.md'nin açık sayısı yine 22'de kaldı (1 bayat madde çıktı, 1 doğru tanımlı madde girdi) ama artık hepsi bugün koda karşı gerçekten teyit edilmiş.

**Commit'ler:** `fba6a08` (ilk 19 bulgunun eklenmesi), `42b743e` (tam temizlik/yeniden yazım), `9470e5f` (ikinci doğrulama geçişi + AGENTS.md düzeltmeleri).

### Doğrulama
Bu tur salt dokümantasyon — kod değişikliği yok, `go build/test` çalıştırılmadı (gerek yoktu). `BUG_REPORT.md`'nin madde sayıları elle (Python script ile grep/sayım) doğrulandı, tekrar tekrar kontrol edildi.

## Sıradaki Oturum İçin

1. **En yüksek öncelik — 2 kritik bug düzeltilmeli:** uzak-erişim auth eksikliği (üretilen `RemoteAccess.Token`'ı gerçekten kullanan bir middleware eklemek) ve `rm -rf` regex'i (`\b` yerine `(^|[\s;&|])rm\s+-rf\s+/($|[\s;&|*])` gibi sağlam bir desen). İkisi de küçük, izole, hızlı düzeltmeler — BUG_REPORT.md'de tam detay var.
2. Gerçek terminalde uçtan uca doğrulanmadı — SIGTERM/terminal-restore, bracketed-paste, `/model` ile büyük model başlatma, backend süreç ayrımı (`/gui` ile Flutter aç, CLI'den çık, Flutter'ın canlı kaldığını doğrula).
3. **Minimal Mod gerçek Flutter arayüzünde hiç denenmedi** — toggle açılıp sohbete hiçbir kişilik/kimlik sızmadığı canlı bir chat ile doğrulanmalı.
4. `BUG_REPORT.md`'deki kalan 22 maddeden H4 (panic recovery eksikliği) muhtemelen en yüksek kaldıraçlı — `taskloop/engine.go`'daki `recover()` deseni streaming/agent goroutine'lerine de uygulanmalı.
5. Session 18'in bekleyen maddeleri hâlâ geçerli — test kapsamı, `pickBestAsset` belirsizliği, `internal/app` test hijyeni.
6. **Küçük borç:** `mobile/` projesinin `.dart_tool`/pub-cache durumu her taze clone'da kontrol edilmeli.

---

# Handoff — 2026-07-07 (Session 18) — Model önerisi + eşzamanlı indirme + sessiz RAG hatası + test kapsamı

## Oturum Özeti

Uzun, karma bir oturum: küçük bir crash fix'ten başlayıp donanıma göre model önerisi özelliğine, oradan backend'in tek-seferde-tek-indirme kısıtına, kullanıcının kendi sorusuyla bulduğumuz sessiz bir RAG hatasına, ve son olarak hem frontend hem backend'de gerçek test kapsamı eklemeye uzandı. 9 commit, hepsi ayrı ayrı, İngilizce, attribution'sız (`4346096`..`42c3d3b`).

## Yapılanlar

### 1. Crash fix: `ref.listen` initState'te çağrılıyordu (`4346096`)
Kullanıcının gönderdiği ekran görüntüsünde `ChatInput` her mount olduğunda kırmızı hata sayfası veriyordu: `ref.listen can only be used within the build method`. Kök sebep: `chat_input.dart`'ta `activeChatIdProvider` dinleyicisi `initState()` içindeydi. `build()`'e taşındı, `flutter analyze` temiz.

### 2. Donanıma göre model önerisi — setup wizard'a yeni adım (`bee551e`, `33f7f58`)
Kullanıcı: uygulama sadece geliştiricilere değil, "interneti olmadığında yapay zeka istiyorum" diyen en basit kullanıcıya da hitap etmeli. İlk kurulum sihirbazına yeni bir "Model Önerisi" adımı eklendi:
- `curated_models.dart`'a `recommendedChatModel(GPUInfo)` / `recommendedMemoryModel` saf fonksiyonları (testli) — donanıma göre en uygun modeli seçiyor.
- RAM/GPU'yu sade dille gösteriyor, "Bu Modelleri İndir" butonu iki modeli de indirmeye başlatıyor, ilerleme arka planda gösteriliyor, kurulum indirme bitmeden tamamlanabiliyor.
- Zaten model kuruluysa gereksiz öneri yerine kısa bir onay gösteriyor.
- Görsel doğrulama: izole bir test build'i + gerçek backend'e karşı ekran görüntüsüyle.

### 3. Backend: eşzamanlı model indirme desteği (`d41c761`, `f803312`)
Kullanıcı geri bildirimi: "1-2 model aynı anda inerken sadece birini görebiliyoruz, indirme durumu her yerde (chat inputun altındaki bar) görünsün." Kök sebep: `modelstore.Store` tek bir `*DownloadProgress` alanı tutuyordu, ikinci `DownloadModel` çağrısı "another download in progress" hatasıyla reddediliyordu.
- Backend: tek-slot yerine repo+dosya anahtarlı map'e geçildi — farklı dosyalar gerçekten eşzamanlı iniyor. `GetDownloadProgress()` artık liste dönüyor, `CancelDownload(repoID, filename)` artık hedefli. REPL CLI de güncellendi.
- Frontend: Model Mağazası'nın indirme banner'ı artık liste (alt alta), `EngineStrip`'e (chat inputun altındaki, her ekranda görünen bar) yeni bir gösterge eklendi — tek indirme dosya adı+yüzde, çoklu indirme "Modeller iniyor" + ortalama yüzde gösteriyor.
- Canlı doğrulama: backend yeniden derlenip nazikçe yeniden başlatıldı (`/api/shutdown` ile), curl ile eşzamanlı indirme + yinelenen-anahtar reddi + iptal test edildi, ekran görüntüsüyle "Modeller iniyor %51" doğrulandı.

### 4. Kullanıcının tek sorusuyla bulunan gerçek bug: sessiz RAG hatası (`6ce158a`)
Kullanıcı ekran görüntüsünde Model Yöneticisi boş ama alt barda bir embedding modelinin "çalışıyor" göründüğünü fark etti — araştırınca bu, benim test sırasında indirip sonra dosyasını silmeden önce durdurmadığım bir "hayalet" process'ti (`/api/models/embedding/stop` ile düzeltildi). Ama bu arayış **gerçek ve önceden var olan bir hata** ortaya çıkardı: `autoStartEmbeddingModel` embedding modeli bulamadığında sadece backend logu + hiç kimsenin okumadığı bir `memory:error` eventi üretiyordu — kullanıcı arayüzünde hiçbir uyarı yoktu, Hafıza (RAG) sessizce çalışmıyordu.
- Düzeltme: `EngineStrip`'e yeni bir uyarı — Hafıza açık ama embedding çalışmıyorsa "Hafıza modeli yok · model indir" ya da "Hafıza kapalı — RAG çalışmıyor · başlat", tıklayınca Modeller sekmesine gidiyor.

### 5. CI kırığı düzeltmesi (`3220e82`)
GitHub Actions `flutter analyze --no-fatal-infos` exit code 1 veriyordu — `agent_screen.dart` ve `chat_screen.dart`'ta önceden var olan, kullanılmayan `import '../models/agent.dart'` satırları (warning seviyesi, `--no-fatal-infos` bunu yumuşatmıyor). Bu oturumun değişiklikleriyle ilgisizdi, düzeltildi.

### 6. Test kapsamı — frontend (`b4f878d`)
Kullanıcı sordu: "test eksikliğini nasıl giderecez". Gerçek ölçüm: `frontend/test/` fiilen boştu (sadece model sınıfları + 1 placeholder), gerçek kapsama tüm `lib/`'in **~%1.2**'si. Eklenenler:
- `test/providers/settings_provider_test.dart` (9 test) — `SetupCompleteNotifier`/`LaunchpadSeenNotifier`/`TourSeenNotifier`, `SharedPreferences.setMockInitialValues()` ile ağsız.
- `test/widgets/engine_strip_test.dart` (10 test) — bugünün en riskli widget'ını (`EngineStrip`) gerçek widget ağacı olarak render edip doğruluyor (offline hint, model göstergeleri, yeni hafıza uyarısının iki dalı, indirme göstergesi, hatalı indirmenin görmezden gelinmesi).
- Yol boyunca bulunan ayrı bir gerçek sorun (kapsam dışı bırakıldı): `EngineStrip`'in `Row`'u dar pencerede taşabilir (Expanded/ellipsis yok), test 1400px genişlikte pompalanarak atlatıldı.
- Sonuç: 72 → 91 test, gerçek kapsama ~%1.2 → ~%1.9 (küçük ama gerçek bir başlangıç, "çözüldü" değil).

### 7. Test kapsamı — backend'in en zayıf 3 paketi (`42c3d3b`)
Kullanıcı: "backend'in en zayıf 3 paketine test yazalım." Ölçüm: `internal/app` %6.9, `internal/llama` %7.6, `internal/webserver` %8.2 (backend toplamı %29.9, ama neredeyse her paketin testi var — sadece bu üçü zayıf).
- `internal/app` → %9.0: `agent_test.go` (yeni) — agent.go'daki tüm wrapper fonksiyonlar (nil-executor + gerçek executor); `helpers_test.go`'ya 3 saf fonksiyon (`stripOrchestraLines`, `buildConversationContext`, `detectMime`).
- `internal/llama` → %16.5: `extractModelName`, `findMmproj`, ve yeni `installer_test.go` (`copyFile`, `HasGPUSupport`, `IsInstalled`, `pickBestAsset`).
- `internal/webserver` → %15.4: yeni `nil_fullbridge_test.go` — 39 Flutter-özel handler'ın FullBridge yokken panic değil düzgün hata döndürdüğünü tablo tabanlı tek testte doğruluyor.
- Backend toplamı: %29.9 → %31.7.
- Yol boyunca bulunan, düzeltilmeyen bir bulgu: `pickBestAsset`'in Linux CPU tercihi sadece "ubuntu" anahtar kelimesine bakıyor — CUDA/Vulkan etiketli bir asset de bunu içerdiği için asset sırasına göre yanlış seçim riski var.

## Doğrulama

```
CGO_ENABLED=1 go build ./... && go vet ./... && go test ./... -race -count=1
  → tüm paketler yeşil (ngrok/whisper'daki 2 test bu makinede gerçek servis çalıştığı için
    önceden var olan, ortam kaynaklı hata — ilgisiz)

flutter analyze --no-fatal-infos && flutter test
  → temiz (4 pre-existing info), 91/91 test yeşil
```
`internal/app/config/config.yaml` her test koşumunda mutasyona uğruyor (bilinen borç), her seferinde `git checkout --` ile geri alındı, commit edilmedi.

Flutter SDK: `/home/bugra/Belgeler/flutter/bin` (PATH'te değil, `export PATH="$PATH:/home/bugra/Belgeler/flutter/bin"`). Görsel doğrulamalar bu oturumda gerçek backend'e karşı izole test build'leri + `spectacle` ekran görüntüleriyle yapıldı (Wayland/KDE — ImageMagick `import` çalışmıyor, `spectacle -b -n -f -o <path>` kullan).

## Sıradaki Oturum İçin

1. **Test kapsamı devam etmeli** — hem frontend (~%1.9, 7 ekranın 7'si + ~21 widget hâlâ sıfır test) hem backend'in geri kalan paketleri (webserver'ın "başarı" yolları ~90 metotluk bir `FullBridge` mock'u gerektiriyor — büyük iş). Kullanıcı muhtemelen bir sonraki oturumda devam etmek isteyecek.
2. **`EngineStrip`'in dar pencerede overflow riski** (Row'da Expanded/ellipsis yok) — session 18'de bulundu, düzeltilmedi.
3. **`pickBestAsset`'in Linux CPU/GPU asset seçim belirsizliği** (yukarıda) — düzeltilmedi, sadece flagged.
4. **`internal/app` test hijyeni** (config.yaml mutasyonu) — session 17'den beri bilinen borç, hâlâ düzeltilmedi.
5. Session 15-17'nin bekleyen büyük işleri hâlâ geçerli: `PLAN_chatid_refactor.md`, `PLAN_installer_launchvbs.md`, FM20'nin genişletilmiş i18n denetimi (`general_tab.dart`).

---

# Handoff — 2026-07-07 (Session 17) — BUG_REPORT.md tam temizlik turu: CRITICAL/HIGH/MEDIUM sıfırlandı

## Oturum Özeti

Kullanıcı `BUG_REPORT.md`'deki kalan tüm bug'ları CRITICAL'den başlayıp aşağı doğru, agent kullanmadan tek tek düzeltmemi istedi ("her büyük küçük demeden detaylı commit at"). ~40 commit'te (fix + docs karışık, hepsi ayrı ayrı) CRITICAL/HIGH/MEDIUM seviyesindeki **tüm** bug'lar ve LOW seviyesinin büyük kısmı düzeltildi. Ayrıca ayrı bir RAG stabilite incelemesi (kullanıcı "RAG stable mi" diye sordu) 3 gerçek bug buldu — biri `internal/database`'de tüm SQLite store'ları etkileyen bir deadlock'tu. Attribution kuralı korundu: 60+ commit tarandı, hiçbirinde Co-Authored-By/Claude/Anthropic imzası yok.

**Backend doğrulama:** her fix'ten sonra `go build/vet/test` (çoğunda `-race`) çalıştırıldı, hepsi yeşil. **Frontend:** bu makinede Flutter SDK yok — Dart değişiklikleri derlenemedi/test edilemedi, sadece dikkatli kod okuması + mevcut pattern'lerle birebir eşleştirme ile doğrulandı (her commit mesajında açıkça belirtildi).

## Yapılanlar

### 1. RAG stabilite incelemesi → 3 gerçek bug (`333e95e`, `c8a5ec3`)
- `internal/memory/store.go`'yu satır satır okuyup (1838 satır) mimariyi değerlendirdim: hibrit vektör+FTS arama, RRF, multi-query expansion — sağlam. Ama:
  1. `Store.Close()` sonrası migration goroutine'i nil pointer'a çarpabiliyordu (canlı olarak `go test -race` ile tetiklendi, kanıtlandı).
  2. Embedding modeli değiştirilince vektör arama sessizce kalıcı olarak boşa düşüyordu (`vec_migration_done` bayrağı temizlenmiyordu + tek uyumsuz satır tüm migration batch'ini iptal ediyordu).
  3. Bunları test ederken **üçüncü, daha derin bir bug** buldum: `internal/database/sqlite.go`'daki `DB.Write()`, `Close()` ile yarışınca kalıcı olarak asılı kalabiliyordu (`go test -count=3` ile ~1/3 ihtimalle reprodüklendi, goroutine dump ile kök neden bulundu). Bu memory/mood/calendar/whatsapp — **her SQLite store'u** etkiliyordu.

### 2. BUG_REPORT.md tam geçiş — CRITICAL → HIGH → MEDIUM → LOW

**CRITICAL (2/2):** QC1 (import config.yaml'ı atlıyordu — path resolution saf bir fonksiyona çıkarıldı, test edildi), QC2 (import sonrası memory store reinit yoktu).

**HIGH (5/5):** QH1 (shutdown method kontrolü yok), QH2 (streaming upload MIME spoofing — content sniffing paylaşımlı helper'a çıkarıldı, test yazarken kendi ilk halimin de eksik olduğunu buldum ve sıkılaştırdım), QH3 (WhatsApp `stopCh` bir kere kapandıktan sonra bir daha asla auto-reconnect çalışmıyordu), QH4/QH5 (import/cloud restore sonrası provider+orchestra+session reinit — `reinitProviderAndOrchestra()` ortak fonksiyonuna çıkarıldı, Startup() da onu kullanıyor artık).

**MEDIUM (8/8):** M6 (Tailscale auto-start web server var olmadan çalışıyordu, hiç işe yaramıyordu), QM1 (shutdown handler hem direkt Shutdown() çağırıyor hem SIGINT gönderiyordu — tek yola indirildi), QM2 (Temperature/TopP=0 ayarlanamıyordu — **iki katmanlı bug**: hem `UpdateLlamaConfig`'in `!=0` kontrolü hem `config.validate()`'in aynı hatası, ikisi de düzeltildi), QM3 (rate limiter IP:port'a göre bucket'lıyordu, port hariç bırakıldı), QM4 (WhatsApp stream cancel hata gösteriyordu), QM5 (agent "New Chat" hatası yutuluyordu), QM6 (resim/dosya stream'leri agent modunu ve `buildMessages()`'in mood/web-search/token-budget mantığını tamamen bypass ediyordu — `routeStream()` ortak fonksiyonuna çıkarıldı), QM7 (WhatsApp optimistic mesaj çift görünebiliyordu).

**Kalan (NM7, NL3, FM8, FM10-13, FM16, FM17, FM20-kısmi, QL1, QL2, QL4, QL5, FL3, FL4, FL1/L4 stale-claim düzeltmesi):** hepsi tek tek düzeltildi. Öne çıkanlar: NL3 aslında "redundant" değilmiş — `sessionID==""` durumunda `recordStreamError` hiç çağrılmıyordu, yani hata hiç kaydedilmiyordu (NH1-NH6'nın düzelttiği "yetim mesaj" bug'ı buradan sızmış). QL5'i düzeltirken `SendMessageWithImageStream`'in elle mesaj kurduğunu, `buildMessages()`'i hiç çağırmadığını (dolayısıyla mood/web-search'ü atladığını) buldum.

**False positive olarak işaretlenip dokunulmayanlar (kanıtla):** NM9, NM10 (önceki oturumdan), QL3 (`_styleCache` — key space 2 ile sınırlı, `MemoTheme.accent` sabit), FL2 (`onDispose` aslında invalidate'te tetikleniyor, riskli mimari değişikliğe değmez).

**Dokümantasyon tutarsızlığı bulundu ve düzeltildi:** BUG-FL1/L4 (`streamingAgentEventsProvider` double-clear) "önceki session'da düzeltildi" diye işaretliydi ama kod hâlâ bozuktu — şimdi gerçekten düzeltildi (`ea5f9d2`).

### 3. Ayrı bulgu: Windows/Linux uninstaller'lar Flutter'ın `shared_preferences` verisini silmiyordu (`279fc00`)
Kullanıcı "kurulum sıfırlanmıyor" diye şikayet etti; kök neden App'in KENDİ config'i değil, Flutter'ın `~/.local/share/com.memo.memo_flutter/` (Linux) / `%APPDATA%\com.memo\` (Windows) altında tuttuğu ayarlardı (dil, tema, `memo_setup_complete`) — hangi build çalışırsa çalışsın aynı dosya. `uninstall.sh` ve `installer.iss` artık bunu da temizliyor.

## Doğrulama

```
go build ./... && go vet ./... && go test ./... -race -count=1   → tüm paketler yeşil
```
Not: `internal/app` testleri `internal/app/config/config.yaml` adlı gerçek/commit'li bir fixture dosyasını `config.Save()` yan etkisiyle mutasyona uğratıyor (pre-existing test hijyeni sorunu, bu oturumda düzeltilmedi) — her test koşumundan sonra `git checkout -- internal/app/config/config.yaml` ile geri alındı, commit edilmedi.

Frontend: Flutter SDK bu ortamda yok, `flutter analyze`/`flutter test` hiç çalıştırılamadı. Tüm Dart fix'leri sadece kod okuması + mevcut çalışan pattern'lerle birebir eşleştirme ile doğrulandı.

## Sıradaki Oturum İçin

1. **BUG_REPORT.md'de artık gerçek anlamda açık bug yok.** Kalan sadece 4 madde, hepsi raporun kendisinde zaten "mevcut, önceden bilinen borç" diye işaretliydi: M9 (`bash -c` hardening, tasarım kararı gerektiriyor), M10 (`model_store_screen.dart` 2500+ satır refactor), M11 (mobile API client eksik endpoint'ler, ~4 saatlik iş), M12 (`connectionStatusProvider` sürekli polling — AGENTS.md'de zaten "kabul edilebilir" diye not düşülmüş, muhtemelen bilinçli tasarım).
2. **FM20 genişletildi:** `general_tab.dart`'ta CLI yönetimi ve hafıza silme dialog'larında onlarca ek hardcoded Türkçe string bulundu (örn. "CLI yeniden yüklendi...", "CLI'ı Kaldır", "Memo'yu Kaldır") — orijinal bug'ın dar kapsamının çok ötesinde, ayrı bir i18n denetimi gerektiriyor. Henüz yapılmadı.
3. **`internal/app` test hijyeni:** `internal/app/config/config.yaml` gerçek bir git-tracked fixture ama bazı testler (`TestGetSetSystemPrompt`, `TestUpdateLlamaConfig_*` vb.) `config.Save()` çağırıp onu mutasyona uğratıyor — `config.DataDir()`/`ConfigDir()` process-global `sync.Once` olduğu için düzgün izole edilemiyor. Küçük ama gerçek bir teknik borç; düzeltme muhtemelen `config` paketine bir test-injection noktası eklemeyi gerektirir.
4. Session 15-16'nın bekleyen büyük işleri hâlâ geçerli: `PLAN_chatid_refactor.md` (tek-global-aktif-sohbet mimarisinin kaldırılması) hiç başlanmadı; `PLAN_installer_launchvbs.md` de henüz uygulanmadı (durumu bu oturumda kontrol edilmedi).
5. Bu oturumda RAG'ı "%75-80 stable" olarak değerlendirmiştim (2 gerçek açık + test kapsamı boşluğu yüzünden); o 2 açık artık kapalı ve `internal/database` deadlock'ı da düzeldi — yeniden değerlendirilirse muhtemelen daha yüksek bir puan çıkar, ama consolidation/merge, importance decay, export/import gibi kısımlar hâlâ test edilmemiş durumda.

---

# Handoff — 2026-07-06 (Session 16) — memo-release skill validation + documentation links

## Oturum Özeti

memo-release skill (Session 15'te yazılmış) doğrulandı. Phase 1 audit başarılı: tüm 4 versiyon lokasyonu (version dosyası, installer.iss:8, README.md ×2, READmeTR.md ×2) mevcut, drift yok. Commit disiplini kuralları açık (İngilizce, Conventional Commits, detaylı WHY, attribution'sız), repo'nun son 5 commit'i kuralları tamamen takip ediyor. Skill artık production-ready. Yer işaretleri güncellendi (handoff.md bu entry, AGENTS.md release seksiyon genişletildi).

## Yapılanlar

1. **Explore ajan:** Release-pipeline haritası zaten `.claude/skills/memo-release/SKILL.md`'de tamamlanmış olduğunu doğruladı.
2. **Phase 1 audit (subagent):** 3.1.2 → 3.1.3 senaryosu — tüm 4 lokasyon mevcut, değişim hedefleri net.
3. **Commit disiplini audit (subagent):** Skill kuralları açık ve takip edilebilir; repo'nun kendi commit'leri yaşayan örnek.
4. **Documentation updates:**
   - `AGENTS.md` Release seksiyon `memory-release` → `memo-release` ve `.claude/skills/memo-release/SKILL.md` doğru yola işaret ediyor.
   - handoff.md memo-release skill'i ve doğrulama sonuçları bu entry'de belirtildi.

## Sıradaki Oturum İçin

1. Release çıkmak gerekirse `/memo-release` skill'ini çağır — Phase 1'den Phase 5'e kadar tam rehber.
2. Commit disiplini: Conventional Commits, English, WHY bodys, zero AI attribution — Skill'de net, repo'da precedent yok.
3. Küçük iş: `PLAN_installer_launchvbs.md` uygulaması (Session 15 karşılığı).
4. Büyük iş: `PLAN_chatid_refactor.md` Faz 1 (Session 15 karşılığı).

---

# Handoff — 2026-07-06 (Session 15) — Bilgi aktarımı: AGENTS.md kuralları + plan dosyaları + kaçan commit

## Oturum Özeti

Amaç: proje bilgisini yazıya dökmek ki sonraki oturumlar (farklı/daha küçük
modellerle bile) aynı kalitede devam edebilsin. Kod değişikliği yok (bir kaçan
commit hariç), tamamen dokümantasyon/plan turu.

**Commit durumu:** `cb5c995` — `fix(frontend): make stop button cancel WhatsApp stream and add 300s timeout` (önceki oturumdan working tree'de unutulmuş `chat_input.dart` değişikliği; `flutter analyze` temiz, sadece 2 önceden var olan info). Bu oturumun doküman değişiklikleri de commitlendi.

## Yapılanlar

1. **AGENTS.md büyük güncelleme:**
   - Yeni "Agent Working Rules (READ FIRST, EVERY SESSION)" bölümü — oturum
     başı/sonu ritüeli (handoff okuma/yazma), zorunlu doğrulama komutları,
     commit kuralları.
   - Yeni "Gotchas" bölümü — projeye özel tuzakların tek listesi
     (config.DataPath, DB.Write serialized loop, global aktif sohbet, 300s SSE
     sözleşmesi, IndexedStack polling, package:path, `is` cast kontrolü, vb.).
   - Yeni "Known Open Work" işaretçi tablosu.
   - Versiyon satırı 3.1.1 → 3.1.2 düzeltildi.
2. **plan.md** — dosya bozulmuştu (kendini tekrar eden karışık metin);
   incelendi ve onboarding işinin **zaten tamamen uygulanmış** olduğu görüldü
   (launchpad_view.dart, spotlight_tour.dart, boş ekranlar, nav etiketleri,
   ayarlardan tur/launchpad sıfırlama hepsi kodda mevcut). Temiz arşiv
   olarak yeniden yazıldı.
3. **PLAN_installer_launchvbs.md (yeni)** — Session 14'te tespit edilen
   Windows kısayol bug'ının adım adım çözüm planı (staging'e VBS wrapper
   üretimi, iki build script'i, doğrulama listesi).
4. **PLAN_chatid_refactor.md (yeni)** — Session 13'ün kapsam dışı bıraktığı
   global-aktif-sohbet mimarisi refactor'ünün 4 fazlı planı. Kod okunarak
   yazıldı: sessions.Manager'da session-scoped API'nin (AddMessageToSession
   vb.) ve llm.go'da çift yollu persist'in zaten var olduğu tespit edildi —
   plan bunların üzerine kuruluyor.

## Ek (aynı gün, devam) — memo-release skill + iki canlı bug düzeltmesi

5. **Canlı bug: `installer.iss` 3.1.1'de kalmıştı** — 3.1.2 bump'ı installer.iss'i
   kaçırmış; Windows kurulumu kendini 3.1.1 olarak tanıtacaktı. Düzeltildi
   (`d4b2178`).
6. **Canlı bug: README/READmeTR changelog linkleri v3.1.1.md'ye işaret ediyordu**
   — v3.1.2 notları hiçbir README'den erişilemiyordu. Düzeltildi (`ddbd3fe`).
7. **`.claude/skills/memo-release/SKILL.md` (yeni)** — tam release prosedürü:
   7 versiyon lokasyonu, EN+TR release notları, platform bazlı build komutları
   ve artifact isimleri, download.bugradev.com'a versiyonlu→jenerik isim
   dönüşümü, version-zeta.vercel.app/version.json beacon'ının EN SON
   güncellenmesi kuralı, katı commit disiplini (İngilizce, detaylı gövde,
   attribution yok). Bir Explore ajanıyla pipeline haritalandı, Sonnet
   ajanıyla dry-run testi yapıldı (tüm adımları doğru üretti), testin
   yakaladığı macOS zip/tar.gz karışıklığı düzeltildi.
8. `.claude/settings.json` izin listesi temizlendi (`677009f`), boş
   `.claude/skills/memo-dev/` klasörü silindi.

## Sıradaki Oturum İçin

1. Küçük iş: `PLAN_installer_launchvbs.md`'yi uygula (tek oturumluk).
2. Büyük iş: `PLAN_chatid_refactor.md` Faz 1'den başla (fazlar arası commit).
3. Bir sonraki sürümde release süreci için memo-release skill'ini kullan.
4. Session 14'ün diğer maddeleri hâlâ geçerli (aşağıda).

---

# Handoff — 2026-07-05 (Session 14) — Installer / Updater / Uninstaller scripts + README düzenlemesi

## Oturum Özeti

Kullanıcı `get-memo.sh` ve `get-memo.ps1` script'lerinin çalışma mantığını inceletti. Script'ler review edildi, Türkçe → İngilizce çevrildi, banner/renkli çıktı eklendi, download progress bar eklendi. `get-memo.sh` artık full kurulum yapıyor (CLI + Flutter GUI + .desktop + icon) ve mevcut kurulumu algılayıp update moduna geçiyor. `update.sh` (veri koruyan güncelleyici) ve `uninstall.sh` (hafıza yedekleme soran kaldırıcı) sıfırdan yazıldı. README'ler güncellendi.

**Commit durumu:** Bu oturumda commit yapılmadı. Değişen dosyalar:
- `get-memo.sh` — yeniden yazıldı (full installer + updater, banner, renk)
- `get-memo.ps1` — güncellendi (banner, renk, progress, İngilizce)
- `update.sh` — yeni dosya
- `uninstall.sh` — yeniden yazıldı (memory backup, onay, PATH temizleme)
- `README.md` — Quick Start bölümü yenilendi
- `READmeTR.md` — Quick Start bölümü yenilendi

---

## Yapılanlar

### 1. `get-memo.sh` review + rewrite

**Eski durum:** Sadece CLI binary'sini kuruyordu, engine binary'ler ilk kurulumda kopyalanıyordu. Mesajlar Türkçeydi. Progress bar yoktu. `.desktop` dosyası oluşturmuyordu.

**Yeni durum:**
- `clear` + ASCII banner + renkli çıktı (ANSI escape kodları)
- Tüm mesajlar İngilizce
- `curl -fSL -#` ile download progress bar (% gösterimi)
- **Full kurulum:** CLI (`~/.memo/bin/memo` + `~/.local/bin/memo` wrapper), backend (`~/.memo/memo-backend`), Flutter GUI (`~/.memo/memo_flutter` + `lib/` + `flutter_assets/`), runner (`~/.memo/run_memo.sh`), engine binary'ler, `.desktop` app menü girişi, ikon
- **Auto-detect update:** `~/.memo/` dizini varsa → update modu (çalışan backend'i durdurur, tüm binary'leri yeniler, config/verilere dokunmaz)
- Config seeding sadece fresh install'da yapılıyor
- Guide linki: `https://memo.bugradev.com/guide`
- Teşekkür mesajı

### 2. `get-memo.ps1` güncellemesi

- Banner + renkli çıktı + `Clear-Host`
- `$ProgressPreference = "Continue"` ile download progress
- Try/catch ile download hata yakalama
- Tüm mesajlar İngilizce
- Guide linki

### 3. `update.sh` (yeni)

- Mevcut kurulumu kontrol eder (yoksa installer'a yönlendirir)
- Çalışan backend'i durdurur (`/api/shutdown` → kill fallback)
- Tüm binary'leri yeniler: engine, backend, Flutter, lib/, runner, CLI
- **Korunanlar:** config.yaml, .env, providers.json, permissions.json, memory/, sessions/, models/, skills/, whatsapp/

### 4. `uninstall.sh` (rewrite)

- ASCII banner + renkli çıktı
- Nelerin silineceğini listeler
- Memory verisi varsa "Save your memory data?" diye sorar
- Yes → `~/Documents/memo-memory-{timestamp}.zip` olarak yedekler (zip yoksa Python fallback, o da yoksa klasör kopyası). Belgeler/Documents farketmeden bulur.
- "Proceed with uninstall?" son onay
- Çalışan process'leri kill eder
- Şunları siler: `~/.memo/`, `~/.local/bin/memo`, `~/.local/share/applications/memo.desktop`, ikonlar
- `.bashrc`, `.zshrc`, `config.fish`'ten PATH satırlarını temizler

### 5. README düzenlemesi

- "Engine binaries not included" uyarısı kaldırıldı (artık gömülü)
- Quick Start: önce tek komutla kurulum, sonra manuel alternatif
- Update / Uninstall komutları eklendi
- `get-memo.sh`'in update modu da belirtildi
- Tüm URL'ler `https://download.bugradev.com/` olarak düzeltildi

---

## Tespit edilen bug — sonraki bir oturumda düzeltildi (DÜZELTİLDİ)

**`installer.iss`'te `launch.vbs` referansı:** Inno Setup script'i Start Menu ikonu, Desktop ikonu ve `[Run]` post-install başlatma için `{app}\launch.vbs` dosyasına işaret ediyor. Bu oturumda `launch.vbs` ne repo'da ne de `build_releases.sh` staging dizininde bulunmuyordu.

**Güncelleme (Session 18'de doğrulandı):** Aslında çözülmüş — `.github/workflows/build-windows.yml` (satır ~152-227) Windows release'i CI'da build ederken `launch.ps1` ve onu gizlice çalıştıran `launch.vbs` wrapper'ını dinamik olarak üretip staging dizinine yazıyor. `PLAN_installer_launchvbs.md` planı hiç uygulanmadı çünkü ihtiyaç zaten CI workflow üzerinden karşılanmış. Artık açık bug değil.

---

## URL yapısı

| Amaç | URL |
|---|---|
| Linux/macOS installer | `https://download.bugradev.com/get-memo.sh` |
| Windows installer | `https://download.bugradev.com/get-memo.ps1` |
| Updater | `https://download.bugradev.com/update.sh` |
| Uninstaller | `https://download.bugradev.com/uninstall.sh` |
| Linux archive | `https://download.bugradev.com/memo.tar.gz` |
| macOS archive | `https://download.bugradev.com/memo-mac.zip` |
| Windows setup | `https://download.bugradev.com/memo.exe` |
| Guide / website | `https://memo.bugradev.com/guide` |

---

# Handoff — 2026-07-05 (Session 13) — Task Loop bug fix turu + ActivityPanel'in tamamen kaldırılması

## Oturum Özeti

Kullanıcı, daha önceki oturumda eklenen **task loop** (otonom görev döngüsü) özelliğinin düzgün çalışmadığını bildirdi: nav bazen görünmüyordu, görev oluşturulamıyordu, görev penceresi normal sohbette çıkıyordu. Sonrasında iki ekran görüntüsüyle iki somut bug daha geldi: bir Flutter overflow hatası ve normal sohbette beliren, task loop'la hiç ilgisi olmayan alakasız bir "Görevler" paneli. Üç ayrı düzeltme turu yapıldı:

1. **Task loop mimari düzeltmesi** — işçinin (worker) yanlış sohbete/moda yazması.
2. **`/code-review` ile derin bug taraması** (8 paralel ajan açısı) — task loop koduna özel 9 mantıksal bug bulundu ve düzeltildi.
3. **`ActivityPanel` widget'ının komple kaldırılması** — task loop'la alakasız, önceden var olan, redundant ve overflow'a sebep olan ayrı bir panel.

**Commit durumu:** `31d7c66`, `9c8cb71` commitlendi (kullanıcı tarafından, oturum içinde). **Şu an working tree'de commitlenmemiş değişiklikler var:** `frontend/lib/models/activity_step.dart` ve `frontend/lib/widgets/agent/activity_panel.dart` dosyalarının silinmesi + `frontend/test/models_test.dart` güncellemesi (9c8cb71, bu dosyaları kullanan kodu sildi ama dosyaların kendisini ve ona bağlı testi silmedi — bu oturumun sonunda tamamlandı, henüz commit edilmedi).

---

## İş 1 — Task Loop mimari düzeltmesi

**Kök sorun:** `internal/app/tasklist.go`'daki `buildTaskLoopRunWorker`, kendisine geçilen `chatID` parametresini hiç kullanmıyor, direkt `a.SendMessageStream(ctx, prompt)` çağırıyordu — bu da uygulamanın tek global "aktif sohbet" işaretçisine yazıyor (agent modu da ayrı bir global bayrak). Sonuç: görev listesi hangi sohbete bağlıysa bağlansın, işçi mesajı o an her ne sohbet aktifse oraya (çoğunlukla normal sohbete) gönderiyordu, araç kullanmadan.

| Dosya | Değişiklik |
|---|---|
| `internal/app/tasklist.go` | Worker artık `taskloopRunMu` mutex'i altında önce `SwitchChat(chatID)` çağırıyor, agent modunu zorla açıyor, işi bitirince eski haline döndürüyor. `CreateTaskList`/`StartTaskList` artık `sessions.Manager.IsAgentChat` ile chat_id'nin gerçek bir ajan sohbeti olduğunu doğruluyor. |
| `internal/app/app.go` | `taskloopRunMu sync.Mutex` alanı eklendi. |
| `frontend/lib/screens/app_shell.dart` | Global "Görevler" nav sekmesi kaldırıldı (6 → 5 buton). |
| `frontend/lib/screens/agent_screen.dart` | Agent ekranının üst çubuğuna, o an açık ajan sohbetine bağlı bir checklist butonu eklendi — Tasks ekranına giriş artık **sadece** buradan. |
| `frontend/lib/screens/tasks_screen.dart` | `initialChatId` parametresi, geri butonu, "hangi ajan sohbeti" dropdown'u eklendi (artık `activeChatIdProvider`'a körü körüne güvenmiyor). |
| `frontend/lib/core/l10n.dart` | Yeni dropdown/boş-durum string'leri eklendi. |

---

## İş 2 — `/code-review` ile bulunan ve düzeltilen 9 mantıksal bug

8 paralel bulucu ajan (correctness ×3, reuse, simplification, efficiency, altitude, conventions) + kendi doğrulamam. En kritik olanı **canlı test ile doğrulandı**.

| # | Bug | Dosya | Düzeltme |
|---|---|---|---|
| 1 | Durdurulan (Stop/shutdown) madde kalıcı "stuck" oluyordu, liste bazen yanlış "done" oluyordu — spec açıkça "pending"e dönmesini istiyordu | `internal/taskloop/engine.go` | `processItem` artık `(ok, cancelled bool)` döndürüyor; iptalde madde "pending"e, liste "paused"a dönüyor. 2 yeni test eklendi. |
| 2 | Çökme sonrası "running" kalan liste sonsuza dek kurtarılamıyordu | `internal/taskloop/store.go` | `loadAll()` artık "running"i "paused"a, "running" maddeleri "pending"e çeviriyor. |
| 3 | Goroutine'de `recover()` yoktu — bir panic tüm uygulamayı çökertebilirdi | `internal/taskloop/engine.go` | `run()`'a panic recovery eklendi. |
| 4 | CEO geri bildirimi `}` içerirse JSON parse bozuluyordu | `internal/taskloop/engine.go` | `extractJSON`'daki derinlik sayacı artık tırnak-farkında (`scanBalanced`). Test eklendi. |
| 5 | Store hataları sessizce yutuluyordu; olay string'leri (`:` ayraçlı) serbest metinle bozulabiliyordu | `internal/taskloop/engine.go` | Hatalar loglanıyor; event payload'ları sadece ID taşıyor. |
| 6 | `agentEnabled` restore, kullanıcının elle yaptığı değişikliği ezebiliyordu | `internal/app/tasklist.go` | Sadece hâlâ kendi zorladığımız değerdeyse geri alıyor. |
| 7 | Frontend create/start/stop/delete hatalarını yutuyordu | `frontend/lib/providers/tasklist_provider.dart` | Hepsi `errorMessageProvider` üzerinden toast'a bağlandı. |
| 8 | Görevler ekranı canlı ilerleme göstermiyordu (tek seferlik refresh) | `frontend/lib/providers/tasklist_provider.dart`, `tasks_screen.dart` | WhatsApp'takiyle aynı desenle 3sn'lik polling eklendi. |
| 9 | Başlatma onay metni sadece izin bypass'ından bahsediyordu, aktif sohbetin kayacağından değil | `frontend/lib/core/l10n.dart` | Metin güncellendi. |

**Bilinçli olarak düzeltilmeyen (kapsam dışı bırakılan):** Uygulamanın tek global "aktif sohbet" mimarisi yüzünden, loop çalışırken kullanıcı gerçek zamanlı başka bir sohbette yazışırsa mesajlar teorik olarak yanlış sohbete karışabilir (altitude/cross-file ajanları tarafından ayrıntılıca tespit edildi). Tam çözüm, tüm mesaj gönderme altyapısını chat-id'ye göre yeniden yazmayı gerektirir — bu, mevcut oturumun kapsamının çok ötesinde, riskli bir çekirdek mimari değişikliği. Ayrıca "concurrent" çalışan görev listeleri aslında `taskloopRunMu` yüzünden gerçekte paralel değil, sıralı — bilinçli bir tradeoff (tek global sohbet kaynağı paylaşıldığı için güvenli tarafta kalındı).

---

## İş 3 — `ActivityPanel` widget'ının komple kaldırılması

Kullanıcının ikinci ekran görüntüsünde gördüğü "Görevler" paneli (checklist ikonu, "Henüz görev yok" boş durumu) **task loop özelliğiyle hiç ilgili değildi** — `activity_panel.dart` adında, tek bir sohbet turunda hangi araçların çalıştığını gösteren, önceden var olan ayrı bir widget'tı. Aynı bilgi zaten sohbet içinde satır arası rozetlerle (`streamingAgentEventsProvider`) gösteriliyordu; bu panel gereksiz bir kopyaydı ve pencere darlaşınca yatay overflow'a sebep oluyordu.

- `frontend/lib/widgets/agent/activity_panel.dart` ve `frontend/lib/models/activity_step.dart` **silindi**.
- `chat_screen.dart`, `chat_provider.dart`'taki tüm besleme kodu (`activityStepsProvider`, `_upsertActivity`, `_settleRunningSteps`, `_toolEventToActivity`) temizlendi.
- `frontend/test/models_test.dart`'taki ilgili test grubu kaldırıldı.

**Bilinçli olarak dokunulmayan:** `internal/app/llm.go`'daki `emitActivity`/`"activity"` finishReason gönderimi backend'de aynen bırakıldı — bu event akışı **sadece Orchestra Mode'un** (çoklu-uzman/chief sistemi) plan/ilerleme görünürlüğünü sağlıyor, normal ajan sohbetinde satır arası bir eşdeğeri yok. Frontend artık bu event'leri parse etmiyor (zararsızca yutuluyor) ama backend'den de sökmek, Orchestra Mode'u kördüğüş bırakırdı (kullanıcı hiç ilerleme görmeden en sonda chief'in cevabını görür). Kullanıcıya bu tradeoff açıkça söylendi, onay beklemeden ileri gidilmedi.

---

## Doğrulama

- Backend: `go build ./...`, `go vet ./...`, `go test ./...` — hepsi yeşil (yeni testler dahil: `TestEngineContextCancel` güçlendirildi, `TestEngineContextCancelLastItem` ve `TestExtractAndParseReview/feedback_containing_a_literal_brace` eklendi).
- Frontend: `dart analyze` (tüm proje, sadece 4 önceden var olan `use_build_context_synchronously` info'su kaldı), `flutter test` — 68/68 geçti.
- Flutter SDK bu makinede `/home/bugra/Belgeler/flutter/bin`'de (PATH'te değil, `export PATH="$PATH:/home/bugra/Belgeler/flutter/bin"` ile çağırıldı).

---

## Sıradaki Oturum İçin

1. **Commit bekliyor:** `activity_step.dart`/`activity_panel.dart` silinmesi + `models_test.dart` güncellemesi henüz commitlenmedi — kullanıcı onaylarsa commit edilmeli.
2. Kullanıcıdan görsel geri bildirim iste: overflow ve alakasız "Görevler" paneli düzeldi mi, task loop artık ajan sohbetinden düzgün başlatılabiliyor mu (gerçek bir ajan sohbetinde bir liste oluşturup başlatarak uçtan uca denenmeli — bu oturumda backend/frontend ayrı ayrı test edildi ama gerçek Flutter uygulaması hiç çalıştırılmadı, çünkü bu makinede görsel bir masaüstü test ortamı kurulmadı).
3. Bilinçli olarak kapsam dışı bırakılan iki mimari kısıt hâlâ geçerli: (a) task loop çalışırken kullanıcı elle başka sohbette yazışırsa mesaj çapraz karışabilir, (b) aynı anda birden fazla görev listesi gerçekte paralel değil sıralı çalışır. İkisi de tek-global-aktif-sohbet mimarisinden kaynaklanıyor; gerçek çözüm `SendMessageStream`'i chat-id parametreli hale getirmek — büyük, ayrı bir iş olarak ele alınmalı.
4. Kullanıcı isterse Orchestra Mode'un `emitActivity` event akışını da backend'den tamamen sökebiliriz (şu an zararsız ama kullanılmıyor) — henüz yapılmadı, yukarıda gerekçesi açıklandı.
5. Session 12'nin kendi bekleyen adımı hâlâ geçerli olabilir: `go build -o ~/.memo/bin/memo .` ile kurulu binary güncellenmiş mi, yeni REPL gerçek terminalde denendi mi — bu oturumda dokunulmadı, doğrulanmadı.
