# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-21 — 5 paralel ajanla (`internal/agent`, `internal/orchestra`, `internal/memory`, `internal/whatsapp`, `internal/calendar`; swarm hariç) codebase-memory destekli derin bug taraması yapıldı, her bulgu elle koda bakılarak doğrulandı. 11 yeni bug bulundu (1 CRITICAL, 4 HIGH, 4 MEDIUM, 2 LOW) — hiçbiri henüz düzeltilmedi, aşağıda listeleniyor.
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
| 🔴 CRITICAL | 1 |
| 🟠 HIGH | 4 |
| 🟡 MEDIUM | 4 |
| 🟢 LOW | 2 |
| 🔧 TEKNİK BORÇ | 1 |
| **TOPLAM** | **12** |

---

## 🔴 CRITICAL

### BUG-C1 — Agent sandbox escape via a pre-existing symlink + not-yet-created target file

`internal/agent/tools/file.go`'daki `validatePath` (satır ~278-303), `write_file`/`edit_file`/`insert_line`/`delete_lines` araçlarının hepsinin ortak yol doğrulaması. `filepath.EvalSymlinks(fullPath)` başarısız olup `os.IsNotExist` dönerse (hedef dosya henüz yoksa — `write_file`'ın normal senaryosu), kod **çözümlenmemiş** `fullPath`'a geri düşüyor. Ama `EvalSymlinks` sadece son bileşen (dosyanın kendisi) eksikse de `IsNotExist` döner — yol üzerindeki, gerçekten var olan bir ara dizinin symlink olması hâlâ hiç çözümlenmeden bırakılıyor.

**Somut senaryo:** proje dizininde `link -> /tmp/disari` diye önceden var olan bir symlink olsun (npm/yarn/venv symlink'leri, ya da kullanıcının/başka bir aracın bıraktığı herhangi bir symlink — sıradışı değil). Agent `write_file("link/yeni.txt", ...)` çağırırsa: `link/yeni.txt` henüz yok → `EvalSymlinks` `IsNotExist` ile başarısız olur → kod ham `basePath + "link/yeni.txt"` yolunu kullanır → metin olarak "proje içinde" göründüğü için izin veriliyor → ama gerçekte `os.WriteFile` symlink'i takip edip `/tmp/disari/yeni.txt`'ye yazıyor, **sandbox'ın tamamen dışına**. İzin promptu sadece görünürdeki (proje-içi gibi duran) path'i gösteriyor, kullanıcı neyin gerçekte yazıldığını göremiyor. `internal/agent/tools/command.go`'daki `RunCommand`'ın CWD çözümlemesi de birebir aynı deseni taşıyor (satır ~334-340).

**Neden kritik:** `validatePath`'ın kendi doc yorumu "ensures the path is within the base path... and denies access to protected system directories" diyor — ama bu tam olarak tutmuyor, ve bu bir dosya-yazma aracının temel güvenlik sınırı.

## 🟠 HIGH

### BUG-H3 — Orchestra fallback zinciri yanlış model adını yanlış sağlayıcıya gönderiyor

`internal/orchestra/conductor.go`'daki hem `tryFallbackProviders` (satır ~647-648: `fbCfg := cfg; fbCfg.Model = task.ModelName`) hem de `CreateProviderForType`'ın "herhangi bir etkin provider'a düş" dalı (satır ~223: `cfg.Model = modelName`), fallback provider'ın **kendi** modelini, orijinal (başarısız olan) provider'ın model adıyla eziyor — model ID'leri vendor-özel olduğu halde (`gpt-4o` OpenAI'a özel, `claude-3-5-sonnet-...` Anthropic'e özel).

**Somut senaryo:** OpenAI rate-limit'e takılır, Claude ve Gemini de etkin fallback aday olsun. `tryFallbackProviders` Claude'u `Model: "gpt-4o"` ile dener → Anthropic API bilinmeyen modeli reddeder → Gemini'ye aynı yanlış model string'iyle geçer → o da reddeder → "all fallback providers failed". AGENTS.md'nin "artık fallback chain var" dediği güvenlik ağı, gerçek çoklu-vendor kurulumlarında fiilen çalışmıyor.

### BUG-H4 — Orchestra chief çağrıları (plan/synthesis) fallback zincirine hiç girmiyor

Aynı dosyada `createPlan` (satır ~244) ve `synthesize` (satır ~750), provider oluşturma/stream/ChatCompletion hatasında doğrudan `return nil, err` yapıyor — `tryFallbackProviders`'a hiç uğramadan. Fallback sadece `executeSingleTask`'ten (görev yürütme) çağrılıyor. AGENTS.md'nin "fallback chain artık var" iddiası, chief'in kendi planlama/sentez çağrıları için hiç geçerli değil.

**Somut senaryo:** Chief OpenAI'a bağlı; OpenAI geçici bir 500 hatası verir. Claude/Gemini etkin ve sağlıklı olsa bile tüm orkestrasyon "chief planning failed" ile anında iptal oluyor.

### BUG-H5 — Consolidation'la birleştirilen hafıza kayıtları RAG aramasında en fazla 187 gün daha "duplicate" olarak çıkmaya devam ediyor

`internal/memory/store.go`'daki `vecSearch` (~1045), `goSearch` (~1112), `ftsSearch` (~686) — hiçbiri `pending_deletion` filtresi taşımıyor; `RetrieveContext` de bunları çağırdıktan sonra hiçbir yerde post-filter yapmıyor. Ama `saveMergedAs` (consolidation'ın kaydetme adımı, hem genel hem pinned-facts dedup için) iki orijinal kaydı `pending_deletion = 1` işaretliyor ve tek temizleyici olan `PurgePendingDeletions` bunları ancak **orijinal oluşturulma zamanından** 187 gün sonra siliyor (birleştirilme zamanından değil).

**Somut senaryo:** Kullanıcı 1. gün "kahve seviyorum" der, 3. gün yakın bir ifadeyle tekrar söyler. 4. gün gece consolidation ikisini birleştirip yeni bir `source='merged'` kayıt ekliyor, iki orijinali `pending_deletion=1` yapıyor. 5. gün "ne içmeyi seviyorum" sorusu `RetrieveContext`'i tetiklediğinde hem 1. gün hem 3. gün hem de yeni birleşmiş kayıt geri dönüyor — 3 neredeyse aynı sonuç, `BuildSystemPrompt`'un ~16K token bütçesini gereksiz dolduruyor, consolidation'ın amacını sessizce boşa çıkarıyor. `TestSavePinnedMerged_StaysPinned` sadece `GetPinnedFacts`'i kontrol ediyor, `RetrieveContext`'i hiç test etmiyor — bu yüzden bugün eklenen pinned-facts dedup'ı bile bu boşluğu kapatmıyor.

### BUG-H6 — Canlı gelen resim/video/döküman altyazılı WhatsApp mesajları sessizce kayboluyor

`internal/whatsapp/client.go`'daki `handleMessage` (canlı mesaj işleyici, satır ~623-631) sadece `GetConversation()` ve `GetExtendedTextMessage().GetText()`'e bakıyor, ikisi de boşsa direkt `return` ediyor — hiç kaydetmeden, hiç loglamadan. Ama paketin kendi `extractText` yardımcısı (sadece history-sync tarafından kullanılıyor, satır ~603-620) resim/video/döküman caption'larını da doğru şekilde okuyor.

**Somut senaryo:** Bir kişi Memo'ya bağlıyken (canlı) altyazılı bir fotoğraf gönderirse mesaj tamamen kayboluyor — kaydedilmiyor, `msgCh`'a düşmüyor, hiçbir hata/log yok. Aynı mesaj bir reconnect sonrası history-sync üzerinden gelseydi doğru şekilde yakalanacaktı — canlı/sync yolları arasında tutarsız, sessiz veri kaybı.

---

## 🟡 MEDIUM

### BUG-M4 — WhatsApp `Unread` alanı gerçek okunmamış sayısı değil, ömür boyu toplam alınan mesaj sayısı

`internal/whatsapp/store.go`'daki `GetChatList` (satır ~238, ~258), SQL'de dürüstçe `as total` diye adlandırılan bir toplamı doğrudan `Unread` alanına (`json:"unread"`) yazıyor. Şemada hiçbir okundu/okunmadı takibi yok — bu sayı, o sohbetten gelmiş tüm mesajların (from_me=0) monoton artan toplamı, kullanıcının görüp görmediğiyle hiç ilgisi yok. `internal/agent/tools/whatsapp.go:35` üzerinden LLM agent'a da olduğu gibi gösteriliyor — yani agent, kullanıcının yüzlerce kez okuyup cevapladığı bir sohbet için "47 okunmamış mesaj var" gibi yanlış bir özet verebilir.

### BUG-M5 — Giden WhatsApp mesajının yerel kayıt hatası sessizce yutuluyor

`internal/whatsapp/client.go:309`'daki `SendMessage`, `_ = store.SaveMessage(...)` ile hatayı hiç kontrol etmiyor/loglamıyor — oysa aynı fonksiyonun gelen-mesaj karşılığı (`handleMessage`, satır ~662-666) aynı çağrı için hatayı logluyor. WhatsApp gönderimi başarılı olsa bile (karşı tarafa gerçekten ulaşsa bile) yerel SQLite yazımı başarısız olursa (disk dolu, WAL kilidi vb.) gönderilen mesaj hiçbir iz bırakmadan yerel geçmişten/aramadan kayboluyor.

### BUG-M6 — Agent mesaj budama'sı assistant+tool_call eşleşmesini bozabilir

`internal/agent/pipeline.go`'nun yorumu "en eski assistant+tool çiftlerini düşürüyoruz" diyor, ama gerçek uygulama (`internal/truncate/tokens.go`'daki `TruncateMessages`) çift kavramı taşımıyor — sondan başa doğru düz bir token bütçesi dolana kadar mesajları tutuyor. Bir iterasyonun `assistant(tool_calls=[a,b])` + `tool_a` + `tool_b` üçlüsü arasında kesme noktası düşerse, tutulan slice `tool_b` gibi öncesinde kendi `assistant` mesajı olmayan bir `tool`-rol mesajla başlayabilir — bu, sağlayıcının ChatCompletion API'sine geçersiz bir mesaj dizisi, sıradaki LLM çağrısını bozabilir. Sadece uzun/ağır tool-call'lı oturumlarda ve sıkı bir `maxTokens` bütçesinde tetikleniyor.

### BUG-M7 — Reminder loop başlangıç gecikmesi, uygulama her açıldığında ilk ~1 dakika içindeki hatırlatıcıları kalıcı olarak atlayabiliyor

`internal/calendar/reminder.go`'daki `Start`, `time.NewTicker(time.Minute)` kullanıyor — Go'da ticker ilk tick'i hemen değil, ~60 saniye sonra ateşliyor. `ClaimPendingReminders`'ın alt sınırı da her tick'te artan, dışlayıcı (`start_time > now`) bir pencere. Uygulama açıldıktan sonraki ilk dakika içinde ateşlemesi gereken bir hatırlatıcı, hiçbir tick'in penceresine asla girmiyor — sessizce, kalıcı olarak kayboluyor (`reminder_sent` hep 0 kalıyor). `internal/routine/loop.go`'daki `RoutineLoop.Start` da aynı ticker desenini taşıyor, yani bu calendar'a özel değil.

---

## 🟢 LOW

### BUG-L2 — Tehlikeli komut path-koruması `--file=/etc/passwd` tarzı token'larla atlatılabiliyor

`internal/agent/tools/command.go`'daki `commandTargetsProtectedPath`/`extractPathTokens`, `--file=/etc/passwd` gibi `=`'lı bir token'ı `/` içerdiği için yakalıyor ama `IsAbs` olmadığı için `workingDir`'a join ediyor — sonuç `/etc` altında değilmiş gibi görünüyor, koruma sessizce atlanıyor. Kullanıcı yine de literal komutu onaylamak zorunda olduğu için tam bir bypass değil, ama korumanın kendi amacını (tehlikeli hedefi önceden işaretlemek) atlıyor.

### BUG-L3 — Orchestra'da stream ortasındaki hatalar, komşu hata yollarının aksine retry/fallback'e hiç girmiyor

`internal/orchestra/conductor.go`'daki `executeSingleTask`, `ChatCompletionStream` bir chunk'ta `chunk.Error` taşıdığında direkt sonuçlandırıyor — oysa aynı fonksiyondaki hem anlık stream-açma hatası hem de non-streaming `retryTask` hatası, `tryFallbackProviders`'ı deniyor. Stream ortasında bozulan bir görev, bitişiğindeki iki yolun aksine hiç ikinci bir şans almıyor.

---

## 🔧 TEKNİK BORÇ

### TD-2 — Local model inference contention (auto fact extraction vs. chat)

`extractAndPinFacts` (`internal/app/memory.go`) auto-extraction'ı ayrı bir goroutine'de, chat cevabı kullanıcıya tamamen gönderildikten sonra tetikliyor — yani aynı turun cevabını yavaşlatmıyor. Ama local model kurulumunda `llama-server` tek slotla çalışıyor (`--parallel 1`), o yüzden extraction hâlâ sürerken kullanıcı hemen art arda yeni mesaj yazarsa, o mesaj extraction'ın arkasında sıraya girebilir — küçük, sınırlı bir gecikme riski, harici provider kullananları etkilemiyor.

(Eski cap/eviction yarısı — `pinnedFactsLimit` 50-cap + hiçbir dedup mekanizması olmaması — `a925109` ile kapatıldı: cap 75'e çıkarıldı ve pinned facts'e özel bir consolidation yolu eklendi.)

---

## Residual (fix değil, takip)

- **L10n:** `orchestra_config_dialog.dart` ve benzeri düşük-trafik dialog'larda hâlâ hardcoded TR string kalabilir — M3 high-traffic yüzeyi kapatıldı.
- **Streaming:** Diğer bare `select` yolları (varsa) ayrı canary/review ile taranmalı; H1/H2 class kapatıldı.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin.*
