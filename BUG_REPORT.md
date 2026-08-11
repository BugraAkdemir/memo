# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-08-12 — **BUG-ONB1 ve BUG-ONB2 tamamen düzeltildi:**
> - **BUG-ONB1** (2 parça): (1) `1c9c33c` — `internal/webserver/server.go`'nun LAN-adres tespiti (`getLocalIPs`, `Settings`/`memo remote status`/script'lerin hepsinin kullandığı) artık `docker`/`br-`/`veth`/`virbr`/`tun`/`tap`/`podman`/`cni`/`flannel`/`kube-bridge`/`cali` önekli sanal arayüzleri atlıyor — aynı mantığın kullanılmayan bir kopyası zaten dosyada duruyordu, ikisi tek, doğru implementasyonda birleştirildi. (2) `dec0c0a` — `get-memo-server.sh`/`get-memo-server-beta.sh` artık kurulum/güncelleme sonunda gerçek `http://<ip>:<port>` adresini basıyor; adres `ip route get 1.1.1.1`'in kaynak IP'siyle (Docker bridge'lerini doğal olarak atlıyor, çünkü onlar hiç outbound routing'de kullanılmıyor) ve port, unit dosyasının kendi `ExecStart` satırından tespit ediliyor (varsayılan olmayan `--port` de doğru yansıyor).
> - **BUG-ONB2** (`97aa57f` + `dec0c0a`) — `cli_service.go`'ya `memo service restart` eklendi (`systemctl --user restart memo.service`'i sarmalıyor), `printServiceUsage()` ve script'lerin "Manage over SSH" bölümü artık `--user` gerekliliğini açıkça yazıyor.
> - Go: build/vet/test `-race` yeşil (`TestIsVirtualNetworkInterface`, `TestPrintServiceUsage_MentionsRestartAndUserFlag` yeni). Script'ler `bash -n` ile sözdizimi doğrulandı + port/`--lan`/IP çıkarma mantığı örnek unit-dosyası içeriğine karşı ayrıca test edildi; gerçek bir systemd kurulumuna karşı uçtan uca bu ortamda denenmedi.
>
> **BUG-ONB3 tamamen düzeltildi** (2 parça): (1) `6125f39` — `isAlive()` artık 401'i "canlı ama yetkisiz" sayıyor (sadece transport hatası "ölü" sayılıyor), `BackendUnreachableOverlay` gate henüz karar vermemişken (`valueOrNull == null`) de gizleniyor — login sonrası "sunucuya bağlanılamıyor" flaş'ı kapandı. (2) `576d200` — `auth_gate_overlay.dart`'taki 4 login/setup yolunun hepsinde (`_submit`, `_enterToken`, `_loginPassword`, `_loginToken`) `api.setSessionToken()`'ın kendi persistence'i (`onRemoteTokenLearned`) fire-and-forget olduğundan, hemen ardından gelen `ref.invalidate(authGateProvider)` bazen henüz diske yazılmamış (eski/boş) token'ı okuyup kullanıcıyı kurulum ekranına düşürüyordu — her 4 yolda `prefs.setString('memo_remote_access_token', token)` artık invalidate'ten önce `await`leniyor. `flutter test` 229/229, analyze temiz, Rule #8 grep temiz.
>
> 2026-08-11 — kullanıcının RPi'sindeki (`192.168.1.106:8090`) canlı web kurulumunda yeni bulgular: BUG-ONB5 (RAM okuma şüphesi, hâlâ açık) — sadece kayda geçirildi, aşağıda. **BUG-ONB4 düzeltildi** (gate açıkken arka plan poll'lerinin 401 gürültüsü) — `authGateBlocked()`/`cancellablePause()` (`frontend/lib/providers/gate_guard.dart`) gate kapalıyken models/embedding/download/mood/whatsapp/cli-running/connection-status poll'lerini askıya alıyor, `messagesProvider` gate altında 401'i sessizce boş sohbet olarak açıyor ve gate kapanınca `chat_screen.dart`'ın listener'ı yeniden yüklüyor. Düzeltme sürecinde ayrı bir gerçek bug daha bulundu ve kapatıldı: `mood_provider.dart`'ın `Stream.periodic(...).asyncExpand(...).distinct()` deseni, iç generator'ın gate kapalıyken hep boş dönmesi (`return;`, hiç `yield` yok) durumunda periyodik Timer'ı dispose'da iptal etmiyordu (minimal, ağsız bir repro ile doğrulandı — asyncExpand+distinct+her-zaman-boş kombinasyonu genel olarak şüpheli, sadece mood'a özgü değil); `modelStatusProvider`'ın da kullandığı kanıtlanmış `while(alive)+cancellablePause` deseniyle yeniden yazıldı. Ayrıca önceki oturumun soruları: hesap yokken login ekranı + uninstall-selfhosted.sh eklendi.
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
| 🟡 MEDIUM | 1 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 2 |
| **TOPLAM** | **3** |

---

## 🟡 MEDIUM — Self-hosted onboarding & web UI (2026-08-11, kullanıcının RPi'sindeki canlı `http://192.168.1.106:8090` web kurulumunda birebir yaşandı)

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
