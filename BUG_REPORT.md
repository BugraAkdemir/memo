# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-08-11 — kullanıcının RPi'sindeki (`192.168.1.106:8090`) canlı web kurulumunda yeni bulgular: BUG-ONB3 (login sonrası geçici "sunucuya bağlanılamıyor" + kurulum ekranına düşme, hâlâ açık), BUG-ONB5 (RAM okuma şüphesi, hâlâ açık) — sadece kayda geçirildi, aşağıda. **BUG-ONB4 düzeltildi** (gate açıkken arka plan poll'lerinin 401 gürültüsü) — `authGateBlocked()`/`cancellablePause()` (`frontend/lib/providers/gate_guard.dart`) gate kapalıyken models/embedding/download/mood/whatsapp/cli-running/connection-status poll'lerini askıya alıyor, `messagesProvider` gate altında 401'i sessizce boş sohbet olarak açıyor ve gate kapanınca `chat_screen.dart`'ın listener'ı yeniden yüklüyor. Düzeltme sürecinde ayrı bir gerçek bug daha bulundu ve kapatıldı: `mood_provider.dart`'ın `Stream.periodic(...).asyncExpand(...).distinct()` deseni, iç generator'ın gate kapalıyken hep boş dönmesi (`return;`, hiç `yield` yok) durumunda periyodik Timer'ı dispose'da iptal etmiyordu (minimal, ağsız bir repro ile doğrulandı — asyncExpand+distinct+her-zaman-boş kombinasyonu genel olarak şüpheli, sadece mood'a özgü değil); `modelStatusProvider`'ın da kullandığı kanıtlanmış `while(alive)+cancellablePause` deseniyle yeniden yazıldı. Ayrıca önceki oturumun soruları: hesap yokken login ekranı + uninstall-selfhosted.sh eklendi.
>
> 2026-08-05 — bir önceki denetimde bulunan 3 bug (LK-1, SF-5, RC-7) `/code-review` + `/codebase-memory` ile doğrulanıp hepsi düzeltildi:
> - **LK-1** (`14f4486`) — `internal/agentcli`'nin `ChatCompletionStream`'i (Claude Code + Codex, ikisi de) ctx iptalinde sadece doğrudan alt süreci öldürüyordu; `--dangerously-skip-permissions`/`--dangerously-bypass-approvals-and-sandbox` ile başlattığı bir torun süreç stdout pipe'ını açık tutarsa `scanner.Scan()` sonsuza kadar bloklanıyordu. `cmd.Cancel` artık tüm process group'u öldürüyor (`internal/llama`'nın Setpgid deseni), `cmd.WaitDelay` (5s) torun süreç yine de kaçarsa yedek. İlk versiyon `/code-review`'dan geçti, 2 gerçek eksik bulundu ve kapatıldı: process-group kill eksikti (sadece pipe'ı zorla kapatıyordu, süreci öldürmüyordu — `--dangerously-*` yetkisiyle çalışan bir süreç arka planda öldürülmeden kalıyordu), test'in sabit 200ms bekleme süresi gerçek bir senkronizasyon garantisi değildi (marker-file polling'e çevrildi).
> - **SF-5** (`7f434ed`) — `callAgentStream`'in bir dalı (`streamCh` boş kapanırsa) terminal chunk göndermiyordu. Gerçek pipeline'ın (`agent.Executor`/`Pipeline.RunStream`) her çıkış yolu zaten terminal chunk gönderiyor — bu yüzden bugün canlı olarak tetiklenemez, ama gelecekteki bir pipeline değişikliğine karşı savunma amaçlı düzeltildi. Test edilebilir olması için `drainAgentStream` diye ayrı bir metoda çıkarıldı.
> - **RC-7** (`5294014`) — `Shutdown()`'ın `close(memorySaveCh)`'i, hâlâ süren bir stream goroutine'inin `saveMemoryAsync` gönderimiyle yarışabiliyordu (`webSrv.Stop()` sadece HTTP handler'ın kendi call stack'ini bekliyor, arkaplan goroutine'lerini değil) — panic oluyordu, başka bir goroutine'in `recoverStreamPanic`'i yanlış ilişkilendirilmiş şekilde yakalıyordu. `saveMemoryAsync` artık kendi gönderimini recover ediyor, doğru loglanmış bir kayıpla.
>
> Her üçü de kendi reprodüksiyon testiyle geldi (fix geri alınınca gerçekten kırıldığı doğrulandı), `-race` ile tüm backend yeşil.
>
> 2026-07-24 — **TD-2 tamamen kapatıldı** (`e88aa0d`/`7dfdd99`/`d875fbe`/`169e069`/`ea67c31`): inference-contention yarısı (cap/eviction yarısı zaten `a925109` ile kapanmıştı). Yeni `App.beginBackgroundLLMCall`/`preemptBackgroundLLM` (`internal/app/llm.go`) — `extractAndPinFacts` artık kendi LLM çağrısını iptal edilebilir bir context üzerinden yapıyor; gerçek bir chat mesajı local model'e (tek inference slot, `llama-server --parallel 1`) gitmek üzereyken (`callLLMStream`'in local dalı, `SendMessage`/`-WithImage`/`-WithFile`) hâlâ süren extraction çağrısını önce iptal ediyor — böylece yeni mesaj artık extraction'ın arkasında sıraya girmiyor. `callLLM`'in kendisine eklenmedi (hem gerçek gönderim hem arka plan çağrıları paylaşıyor — extraction'ın kendi çağrısını kendi kendine iptal etmesini önlemek için preemption sadece sırf-gerçek-chat giriş noktalarına eklendi). 3 regresyon testi (`TestPreemptBackgroundLLM_*`, `TestBeginBackgroundLLMCall_*`).
>
> 2026-07-22 — **CRITICAL, bulunup aynı gün düzeltildi** (`fd6fdd2`): `internal/provider`'da hiçbir vendor'a özel test yokken (`internal/agent` gibi sadece paylaşılan/genel mantık test ediliyordu) `claude.go` için test yazarken bulundu — `ChatCompletion`/`ChatCompletionStream`, `ChatRequest.Model` boşsa provider'ın kendi configured modeline düşen bir fallback hesaplıyordu ama bu hesaplanan değeri hiç kullanmıyordu; `buildClaudeRequest` doğrudan `req.Model`'i okuyordu. `internal/app/llm.go`'daki **ana, normal sohbet streaming yolu** `ChatRequest.Model`'i hiç set etmiyor — yani Claude aktif provider olarak seçiliyken **her normal sohbet mesajı Anthropic API'sine boş `"model": ""` gönderiyordu.** Gemini'de aynı fallback deseni var ama model URL path'inde doğru kullanılıyor (bug yok); OpenAI'da da body'de doğru kullanılıyor — sadece Claude etkilenmişti. Düzeltme + regresyon testleri (`TestClaudeProvider_ChatCompletion_FallsBackToConfiguredModel` ve stream eşleniği, fix'ten önce fail ettiği doğrulandı) aynı commit'te.
>
> `internal/provider` test kapsamı genel olarak da genişletildi: `openai_test.go` (`912097b`, %16→%28.2 — 6 diğer vendor'ın (`grok`/`groq`/`ollama`/`llama.cpp`/`opencode-zen`/`opencode-go`/`openrouter`) da paylaştığı ortak mantığı kapsıyor) ve `claude_test.go` (`fd6fdd2`, %28.2→%41.0).
>
> 2026-07-21'deki derin taramada (`internal/agent`, `internal/orchestra`, `internal/memory`, `internal/whatsapp`, `internal/calendar`) bulunan 11 bug'ın **hepsi** tek tek düzeltildi, her biri kendi regresyon testiyle (fix'ten önce gerçekten fail ettiği doğrulanarak) ayrı commit'te:
> - **BUG-C1** `311e5de` — agent sandbox escape (symlinked ancestor + not-yet-existing file)
> - **BUG-H3/H4** `c9fae03` — orchestra fallback zinciri yanlış model + chief çağrılarının fallback'siz olması
> - **BUG-H5** `971c9e9` — consolidation'la birleşen kayıtların RAG'da 187 güne kadar duplicate kalması
> - **BUG-H6** `a45a53e` — canlı WhatsApp medya mesajlarının (caption'lı) sessizce kaybolması
> - **BUG-M4** `a28cb06` — WhatsApp `Unread` alanı → `TotalReceived` (gerçek anlamıyla yeniden adlandırıldı)
> - **BUG-M5** `a5119d0` — giden WhatsApp mesajının yerel kayıt hatası artık loglanıyor
> - **BUG-M6** `0739234` — agent mesaj budaması artık assistant+tool_call gruplarını bozmuyor
> - **BUG-M7** `4499976` — reminder/routine loop artık başlangıçta hemen tetikleniyor (1 dakika beklemiyor)
> - **BUG-L2** `0752ba5` — tehlikeli komut path-koruması `--flag=/path` argümanlarını da yakalıyor
> - **BUG-L3** `780064a` — orchestra'da stream-ortası hatalar artık retry/fallback deniyor
>
> Kalan: **TD-2**'nin inference-contention yarısı (bilinçli kabul edilmiş, aşağıda).
>
> **TD-1 kapatıldı** (`18ea65c`/`69a4ae3`): backend'e `POST /api/routines/sync-offset` eklendi, Flutter GUI her client (re)connect'inde mevcut `DateTime.now().timeZoneOffset`'i gönderiyor, backend tüm routine'lerin `UTCOffsetMinutes`'ını buna göre güncelliyor. Gerçek IANA zone değil, ama DST geçişi/lokasyon değişikliği artık bir sonraki bağlantıda kendini düzeltiyor — donmuş offset sorunu pratikte çözüldü.
>
> **TD-2**'nin cap/eviction yarısı kapatıldı (`a925109`): `pinnedFactsLimit` 50→75, ve yeni `FindPinnedMergeCandidates`/`savePinnedMerged`/`runPinnedConsolidation` pinned facts havuzunu kendi içinde dedup'lıyor (genel consolidation zaten `source='explicit'`i hariç tutuyordu — bu boşluğu kapatan hiçbir mekanizma yoktu). TD-2'nin inference-contention yarısı (local model tek slotta extraction ile chat'in yarışması) hâlâ açık, bkz. aşağıda.
>
> `pidListeningOnPort` (`internal/llama`, `internal/whisper`) Linux'ta `lsof`/`fuser` bağımlılığı olmadan native `/proc/net/tcp` okuyacak şekilde düzeltildi (`91300f9`/`52b6e9f` + testler `2f839a2`/`d0bb02c`) — her iki araç da kurulu değilse port temizliğinin sessizce no-op olduğu senaryoyu Linux'ta tamamen kapatır (macOS `lsof`/`fuser`'da kaldı, risk zaten düşük).
>
> 2026-07-20 (Session 46 fix pass) — Session 46 review maddeleri kapatıldı:
> - **BUG-H1** `20ba4f0` — agent `trySend` non-blocking-first + regression tests  
> - **BUG-H2** `b1fad30` — WhatsApp `localTrySend` + terminal cancel chunk  
> - **BUG-L1** `a7d4ace`/`21f9623` — low-value ack/greeting RAG skip (`IsLowValueTurn`)  
> - **BUG-M1** `4670b63` — mobile `sendMessage` re-entrancy + stream generation  
> - **BUG-M2** `b77017f` — SettingsDialog nested `ScaffoldMessenger`  
> - **BUG-M3** `79bda62`/`fac700f`/`f53c2ec` — L10n chat_message_list, chat_input, provider/skill dialogs  
>
> Kalan: bilinen teknik borç (routine DST offset, pinned-facts cap) + L10n residual (orchestra_config_dialog ve diğer düşük-trafik dialog stringleri).

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 0 |
| 🟡 MEDIUM | 4 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 2 |
| **TOPLAM** | **6** |

---

## 🟡 MEDIUM — Self-hosted onboarding & web UI (2026-08-11, kullanıcının RPi'sindeki canlı `http://192.168.1.106:8090` web kurulumunda birebir yaşandı)

### BUG-ONB3: Şifreyle login sonrası 5-10 sn "sunucuya bağlanılamıyor" + birkaç saniye sonra kurulum ekranına atma (refresh'te kayboluyor)

- **Nedir:** Web UI'a ilk girişte (login'den hemen sonra ve/veya yeni sekmede) ~5-10 saniye boyunca "sunucuya bağlanılamıyor" (BackendUnreachable) ekranı görünüyor; ardından uygulama **kurulum (setup) ekranına** atıyor — kullanıcının uzak sunucuda zaten kurulu bir hesabı ve verisi varken. Sayfa yenilenince çoğu hata kayboluyor ve normal ekran geliyor.
- **Gözlemlenen konsol:** login ekranındayken her polling/background çağrısı 401 dönüyor (`/api/whatsapp/status`, `/api/models/download/progress`, `/api/cli/running` → `HTTP/1.1 401 Unauthorized`) ve `agent: init error / chat: web search init error / models: modelStatus error: Bir şeyler ters gitti` logları basıyor. Ayrıca `Password fields present on an insecure (http://) page` tarayıcı uyarısı (LAN HTTP — beklenen, bilgi amaçlı).
- **Olası kök neden (henüz incelenmedi):** setup/status poll'ü ile login sonrası token'ın kaydedilmesi arasındaki yarış; `authGateProvider`'ın 401/probe davranışı; `BackendUnreachableOverlay`'in gate'i gizleme mantığı. "Kurulum ekranına atma" özellikle yanlış state geçişi sinyali — login olmuş kullanıcıya asla setup gösterilmemeli.
- **Kullanıcı etkisi:** İlk izlenim "bozuk/bağlanamıyor" + bir de "verilerim mi gitti?" paniği. Refresh'le atlatılıyor ama onboarding deneyimini ciddi zedeliyor. Düzeltilecek ilk web-UI maddesi.
- **Düzeltme (uygulanmadı, sadece kayıt):** frontend state akışı incelenmeli: (1) login başarılı + token kaydedildikten sonra gate anında kapanmalı; (2) `authGateProvider` hiçbir koşulda `needs_setup`'a geçerken mevcut session'ı iptal etmemeli/yok saymamalı; (3) `needs_setup:true` döndüğünde bile kayıtlı geçerli bir token varsa kullanıcıya setup gösterilmemeli (login olmuş kullanıcı için setup akışına düşmek kabul edilemez).

### BUG-ONB5: RAM doğru okunmuyor şüphesi (netleştirilecek)

- **Nedir:** Kullanıcı, API provider bağlayıp cevap alabilen çalışan bir kurulumda "RAM'i doğru okumuyor" belirtiyor. Henüz hangi ekranda (Settings? Model tabı? backend `/api/status`'ta mı gösterilen değer?) ve gerçek değerin ne olduğu netleşmedi. RPi 2GB RAM; muhtemelen model yükleme/embedding bağlamında "yetersiz RAM" mesajı alıyor veya gösterilen değer yanlış.
- **Durum:** Bekleniyor — kullanıcıdan ekran/örnek talep edilecek. (Olası ilgili senaryo, alttaki embedding-RAM notunda.)
- **Düzeltme (uygulanmadı):** netleşince.

### Embedding modeli 2GB RAM'de çalıştırılamadı (bilgi, bug değil — 2026-08-11)

- **Sorun:** RPi 2GB RAM, ~1-1.5GB boş RAM varken nomic-embed-text-v1.5 Q4_K_M (~82MB dosya) / Q3_K_S embedder'ı başlatılamıyor.
- **Ölçek (doğrulanmış, llama.cpp):** nomic-embed-text-v1.5 = 137M parametre. Q4_K_M ~82MB, Q3_K_S ~55MB dosya. Embedding server'ın RSS'i dosya boyutu + model overhead + KV cache ile ~150-300MB arası oluyor — yani **model boyutu 2GB sistemde asla sorun değil**; 1-1.5GB boş RAM tek başına fazlasıyla yeterli. Çalışmama sebebi model boyutu olamaz.
- **Olası gerçek sebepler (incelenmeli):** (1) **OOM killer** — 2GB sistemde başka süreçler (chat modeli llama-server, memos uygulaması, backend, node, Docker bridge servisleri) RAM'i dolduruyorsa embedder süreci öldürülüyor olabilir (`dmesg`/`journalctl -k | grep -i oom` ile doğrulanır); (2) embedder başlatma hatası başka bir sebeple (port çakışması, arm64 binary'sinin eksik olması — RPi arm64, `binaries/` içinde linux/arm64 mevcut mu kontrol edilmeli); (3) `embedding_auto_start: false` — config'de kapalıysa embedder hiç başlatılmıyor, "çalıştıramadım" hissi veriyor.
- **Not:** RPi'deki config'de `embedding_auto_start: false` — kullanıcı elle başlatmadıkça embedding devreye girmiyor.

---

## 🟡 MEDIUM — Self-hosted onboarding (2026-08-09, kullanıcının kendi RPi'sindeki canlı `get-memo-server-beta.sh` kurulumunda birebir yaşandı)

### BUG-ONB1: Kurulum/servis script'i kullanıcıya hangi URL/porta gireceğini hiç söylemiyor

- **Dosya:** `scripts/get-memo-server.sh`, `scripts/get-memo-server-beta.sh` (aynı sorun ikisinde de — systemd servis kurulum bloğu, script sonu)
- **Nedir:** Kurulum bitip `--lan` ile systemd servisi kurulduktan sonra script'in son çıktısı "Service installed and started", auth mode notu, ve `memo service status` / `memo remote status` gibi SSH komutlarını listeliyor — ama **hiçbir satırda** kullanıcının tarayıcıdan açacağı gerçek adresi (`http://<ip>:8090`) açıkça yazmıyor. Faz 5.1 ile artık asıl akış "tarayıcıdan hesap oluştur" olduğu için bu, kullanıcının ilk adımda tam olarak nereye gideceğini bilmediği anlamına geliyor.
- **Ek bulgu (aynı kurulumda):** `memo service status`'ın loglarında birden fazla "LAN address available" satırı basılıyor (`192.168.1.106` — gerçek LAN IP'si — ile birlikte `172.18.0.1`/`172.19.0.1`/`172.17.0.1` — makinede kurulu Docker'ın bridge IP'leri). Script/CLI hangisinin "gerçek" adres olduğunu hiç ayırt etmiyor; ortalama bir kullanıcı Docker bridge IP'lerinden birine girmeyi deneyip başarısız olabilir.
- **Kullanıcı etkisi:** yapacam.md'nin Faz 5.1 bitiş kriteri — "curl kurulumundan sonra hiçbir terminal komutu çalıştırmadan tarayıcıdan bir hesap oluşturup kullanmaya başlayabilecek" — bu script'le tam sağlanmıyor: kullanıcı yine de adresi öğrenmek için ek bir komut çalıştırmak (`memo service status`, `hostname -I` vb.) zorunda kalıyor.
- **Düzeltme (uygulanmadı, sadece kayıt):** Kurulum/servis-kurulum bloğunun sonunda, script'in zaten bildiği gerçek LAN IP'sini (Go tarafı zaten `GetAddresses()`/local IP tespiti yapıyor, script bunu `memo remote status`'tan JSON olarak okuyup basabilir, ya da kendi `hostname -I`/`ip route get` mantığıyla tek bir en-olası adresi seçebilir) kalın/renkli, gözden kaçmayacak bir satırda basmalı: `Open http://<ip>:8090 in your browser to get started`. Docker bridge IP'leri filtrelenmeli (özel `172.17-31.x.x`/`docker0` arayüzü sezgisel olarak elenebilir).

### BUG-ONB2: `memo service`'te `restart` alt komutu yok; hiçbir çıktı `systemctl --user` gerektiğini söylemiyor

- **Dosya:** `cli_service.go` (sadece `install`/`uninstall`/`status` var, `restart` yok), kurulum script'lerinin systemd bloğu
- **Nedir:** Servis `systemctl --user` ile kuruluyor (doğru, kod tarafında hep `--user` kullanılıyor — `cli_service.go:92`, `scripts/get_memo_arm.sh`) ama bu **hiçbir kullanıcı-yüzü metinde açıkça söylenmiyor**. Gerçek kurulumda kullanıcı doğal refleksle önce `systemctl restart memo` denedi — sistem genelinde polkit şifre istedi, kimlik doğrulama başarısız oldu (`--user` olmayan bir servise sistem seviyesinde restart isteği, farklı bir yetkilendirme yoluna giriyor) — sonra `sudo systemctl restart memo` denedi, bu da `Unit memo.service not found` verdi (sudo'nun systemctl'i sistem birimlerine bakar, `~/.config/systemd/user/`'a değil). İki komut da başarısız oldu, ikisi de nedenini açıklamadı.
- **Kullanıcı etkisi:** Servisi yeniden başlatmanın CLI'dan hiçbir güvenilir/kolay yolu yok — `memo service restart` diye bir komut yok, doğru komut (`systemctl --user restart memo`) hiçbir yerde yazmıyor. Web UI'ın "Restart Backend" butonu var ama tarayıcıdan login olmayı gerektiriyor — backend zaten "takılmış" göründüğü an tam da erişilemeyebilecek yol.
- **Düzeltme (uygulanmadı, sadece kayıt):** (1) `cli_service.go`'ya bir `restart` alt komutu eklenmeli (`runSystemctl("restart", ...)` zaten var olan `install`/`status` mantığını taklit ederek trivial). (2) Kurulum script'inin systemd onay bloğu sonuna, `journalctl`/`memo remote status` ipuçlarının yanına `systemctl --user restart memo` / `systemctl --user status memo` da açıkça eklenmeli — kullanıcı `--user` olmayan hâlini deneyip kafası karışmadan önce.

---

## 🔧 TEKNİK BORÇ

### TD-3: `download.bugradev.com`'daki kurulum script'leri CI ile otomatik güncellenmiyor — repo'da düzeltilen bug'lar canlıda hâlâ eski

- **Dosya:** `.github/workflows/build-linux.yml`/`build-macos.yml`/`build-windows.yml` (R2'ye sadece derlenmiş binary/arşivleri yüklüyor), R2 bucket
- **Nedir:** Derlenmiş çıktılar (`memo_beta.tar.gz`, `memo_arm_beta.zip` vb.) her `main` push'unda otomatik R2'ye yükleniyor, ama `get-memo-server.sh`/`get-memo-server-beta.sh` gibi kurulum script'lerinin **kendisi hiçbir workflow tarafından yüklenmiyor** — Session 5'in handoff'unda zaten "kullanıcı R2'ye kendi eliyle yükleyecek" diye not edilmişti. Bugün somut sonucu görüldü: kullanıcının `curl -fsSL https://download.bugradev.com/get-memo-server-beta.sh | bash` ile çektiği script, commit `1fbaec6`'nın (Session 5, token-bootstrap circular-dependency fix) düzelttiği **eski** metni gösterdi — "run 'memo remote status' to see the device token" (bu komutun kendisi token olmadan zaten 401 veriyor, tam olarak `1fbaec6`'nın kapattığı döngüsel bug). Yani repo'da aylar önce düzeltilmiş bir bug, canlıda hâlâ aktif çünkü script hiç yeniden yüklenmemiş.
- **Kullanıcı etkisi:** Install-script'lere yapılan HERHANGİ bir düzeltme (BUG-ONB1/ONB2 dahil, düzeltilseler bile), birisi elle R2'ye yükleyene kadar gerçek kullanıcıları etkilemeye devam eder — sessiz, fark edilmesi zor bir regresyon kaynağı; test edip "düzelttim" demek yeterli değil, deploy de ayrı bir adım.
- **Düzeltme (uygulanmadı, sadece kayıt):** İki seçenek: (1) `scripts/*.sh`'ı da bir CI adımıyla otomatik R2'ye yükle (build workflow'larına eklenecek düşük riskli bir adım — script'ler zaten repo'da), (2) en azından `scripts/README.md`'ye/memo-release skill'ine "script değişikliğinden sonra elle R2'ye yükle" diye açık, atlanamaz bir checklist maddesi ekle. (1) daha sağlam çünkü insan hatasına bağımlı değil — bugünkü olay tam olarak bu insan-hatası senaryosu.

### TD-4: `download.bugradev.com` Cloudflare edge cache'i eski arşivleri servis edebiliyor — repo'dan bağımsız, hesap ayarı gerektiriyor

- **Dosya:** Cloudflare dashboard (bu repo'da düzeltilecek bir kod yok — hesap/DNS/cache ayarı)
- **Nedir:** Flutter-web webui migrasyonunu doğrularken (2026-08-10) tesadüfen bulundu: `curl -I https://download.bugradev.com/memo_arm_beta.zip` bazen `cf-cache-status: HIT` ile **saatler önceki** bir `last-modified`/`content-length` döndürüyor — R2'deki gerçek dosya güncel olsa bile. Aynı URL'e cache-busting query string (`?cachebust=$(date +%s)`) eklenince `cf-cache-status: MISS` ile doğru, taze dosya geliyor — yani Cloudflare bu hostname için `.zip` dosyalarını (muhtemelen "Cache Everything" tipi bir Page Rule/Cache Rule ile) agresif şekilde edge'de tutuyor, R2'deki güncelleme ile cache'in temizlenmesi arasında senkron yok.
- **Kullanıcı etkisi:** Bir kullanıcı `curl ... | bash` ile kurulum/update script'ini çalıştırdığında, hangi Cloudflare edge node'una denk geldiğine bağlı olarak **saatler eski bir binary** indirebilir — script'in kendisi hiçbir hata vermez, "başarılı" görünür, ama içerik eski kalır. Bu oturumda tam olarak bu yaşandı: kullanıcı update'i doğru çalıştırdı, script de doğru çalıştı, ama indirilen arşiv Cloudflare cache'inden geldiği için saatler önceki bir build'di — CI/kod tarafında (yanlışlıkla) uzun bir hata avı başlatıldı, gerçek sebep bu cache katmanıydı.
- **Düzeltme (uygulanmadı, bu repo'nun kapsamı dışında):** Cloudflare dashboard'undan `download.bugradev.com` için: (1) `.zip`/`.tar.gz` dosyalarına kısa bir cache TTL (ya da "bypass cache") kuralı eklenmeli, VEYA (2) CI'nın R2'ye her yükleme sonrası Cloudflare API ile o dosya için otomatik bir "purge cache" çağrısı yapması (`CF_API_TOKEN`/`CF_ZONE_ID` secret'ları eklenip build workflow'larının R2 upload adımlarının hemen ardından bir `curl -X POST .../purge_cache` adımı). (2) daha sağlam çünkü insan "hatırlayıp elle purge etme"ye bağımlı değil — tam olarak TD-3'ün aynı dersi.

---

## Residual (fix değil, takip)

- **L10n:** kapatıldı (`36c8a38`) — orchestra/provider/skill config dialogları, GPU tab, sistem/incognito prompt tabları, skills boş durumu ve daha fazlası L10n'a bağlandı.
- **Streaming:** Diğer bare `select` yolları (varsa) ayrı canary/review ile taranmalı; H1/H2 class kapatıldı.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin.*
