# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-08-05 — streaming/cancellation derin denetimi + `-race` soak; bulunan 3 bug **doğrulandı ama düzeltilmedi** (kullanıcı talimatı: sadece rapor, kod değişikliği yok — LK-1, SF-5, RC-7 aşağıda).
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
| 🟢 LOW | 2 |
| 🔧 TEKNİK BORÇ | 0 |
| **TOPLAM** | **3** |

---

## Açık Bug'lar (2026-08-05)

Denetim notu: üçü de kaynak kodu üzerinde doğrulandı (satır referansları güncel koddur); hiçbiri live senaryoda üretilmedi, düzeltme yapılmadı. `-race` soak ayrıca `count=3` × tüm `./internal/...` için **0 DATA RACE / tüm paketler PASS** sonucunu verdi.

### LK-1 — agentcli subprocess pipe sızıntısı / sonsuz blok (MEDIUM)

**Yer:** `internal/agentcli/claude_code.go` — `ChatCompletion` (satır 81-94) ve `ChatCompletionStream`'in goroutine'i (satır 145-198).

**Kök neden:** `ChatCompletion` çıktı kanalını `for chunk := range ch` ile boşaltıyor ve bu döngüde `ctx` hiç kontrol edilmiyor. Stream goroutine'i içinde `scanner.Scan()` (satır 152) EOF'a kadar bloklar; EOF ancak stdout pipe'ının tüm yazıcı uçları kapanınca gelir. `--dangerously-skip-permissions` ile çalışan `claude` alt süreçleri (shell, git, editör vb.) spawn edebilir; `exec.CommandContext` iptalinde yalnızca **doğrudan** alt süreç SIGKILL ile öldürülür, torun süreçler pipe yazma ucunu devralmışsa `Scan()` hiç EOF görmez → `defer close(ch)` (satır 146) asla yürümez → `ch` sonsuza dek açık kalır.

**Etki:** İptal edilen/takılan bir claude-code-cli sohbeti: `ChatCompletion` çağıranları (arka plan LLM yolları) sonsuza dek bekler; cli_stream yolu üzerinden kullanılıyorsa yapılan iş sonsuza dek "çalışıyor" görünür ve o sohbetteki sonraki gönderimler engellenir; sızan goroutine kalıcıdır ve yalnızca backend restart ile temizlenir. `cmd.Wait()` de hiçbir zaman geri dönmeyeceği için bu durum da olayı kalıcılaştırır.

**Önerilen düzeltme (uygulanmadı):** `internal/llama`'nın `Setpgid`/süreç grubu öldürme desenini buraya taşımak (iptalde torunları da öldürür) VEYA asgari olarak `ChatCompletion` döngüsüne `select { case chunk, ok := <-ch: ... case <-ctx.Done(): return nil, ctx.Err() }` idiomu eklemek (çağıranın takılmasını önler ama goroutine sızıntısını ve cli_stream durum sorununu çözmez).

### SF-5 — agent boş-cevap dalı terminal chunk göndermiyor (LOW)

**Yer:** `internal/app/llm.go` satır 263-268 (`callAgentStream`).

**Kök neden:** `if fullReply.Len() > 0 { finishStream + trySend(Done) } else { recordStreamError(...) }` — else dalında trySend edilmediği için yalnızca `defer close(outCh)` çalışır; akış, terminal `Done:true` chunk'ı gönderilmeden kapanır. AGENTS.md "Streaming/SSE" kuralının ihlali: her stream-dönüş dalı terminal chunk göndermeli (bunun üç turda yaptığı). 

**Etki:** Agent pipeline'ı boş bir yanıt döndürürse (nadir ama mümkün) client terminal işareti almaz — sessiz boş cevap, hata/uyarı çıkmaz. Erişilebilirlik düşük; diğer dallar aynı düzeltmeden geçtiği için tutarsız koşullar içinde kaldı.

**Önerilen düzeltme (uygulanmadı):** else dalına `trySend(ctx, outCh, api.StreamChunk{Error: "⚠️ Agent boş yanıt döndürdü", Done: true})` (veya `FinishReason:"stop"` + `Done:true`).

### RC-7 — routine/memory kanalının kapanma yarışı (LOW)

**Yer:** `internal/app/app.go` satır 699 (`close(a.memorySaveCh)`) ↔ `internal/app/llm.go` satır 1059 (`a.saveMemoryAsync(...)`, `finishStream` içinden) ↔ `internal/app/memory.go` satır 44-49 (`select` ile kanala send).

**Kök neden:** Shutdown sırasında `close(memorySaveCh)` yapılırken hâlâ tamamlanmakta olan bir stream'in `finishStream` → `saveMemoryAsync` yolu kapanmış kanala send dener; "send on closed channel" runtime panic'i fırlatır. Panic, gönderen goroutine'in `recoverStreamPanic` sarmalı tarafından yakalanır (süreç çökmez), ancak o turun memory kaydı sessizce kaybolur.

**Etki:** Dar pencere (kapanış sırasındaki in-flight stream sonu + 15s watchdog zaten süreci bitirir) ve panic recover edildiği için pratik etki düşüktür: son turdaki bir memory kaydı kaybolabilir. Ayrıca `app.go:698-699` yorumu "no more sends will occur" diyor — bu garanti, kapanan kanala send'i kısmen gözden kaçırıyor (tüm write'ların Shutdown öncesi tamamlanmış olması açıkça garanti edilmeli).

**Önerilen düzeltme (uygulanmadı):** `saveMemoryAsync`'de send seçeneğine `defer recover()` eklemek (en azından sessiz kayıp → loglu kayıp) ve/veya `close(memorySaveCh)` öncesinde in-flight stream'lerin bittiğini garanti etmek.

---

## Residual (fix değil, takip)

- **L10n:** kapatıldı (`36c8a38`) — orchestra/provider/skill config dialogları, GPU tab, sistem/incognito prompt tabları, skills boş durumu ve daha fazlası L10n'a bağlandı.
- **Streaming:** Diğer bare `select` yolları (varsa) ayrı canary/review ile taranmalı; H1/H2 class kapatıldı.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin.*
