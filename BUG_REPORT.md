# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-18 (Session 41) — **BUG-H3 fix'i tamamlanmadan önce eksikti, canlı `-p` doğrulamasında yakalandı:** `chunkText`/`splitLongWord` fix'i sadece `SaveInteraction`'ın `userChunk`'ını kapsıyordu; `saveChunk` her chunk'a `assistantMsg`'i (yanıtın tamamı) sınırsız olarak ekliyordu, VE `RetrieveContext` ham `query`/`expandQuery`/`splitCompoundQuery` metnini hiç sınır olmadan doğrudan embed ediyordu — ikisi de aynı "too large to process" hatasına hâlâ açıktı. Ayrıca gerçek embedding tokenizer'ı, tekrarlı tek karakter gibi doğal olmayan içerik için `len/3` tahmininden ~1.8× daha fazla gerçek token üretiyor (canlı ölçüldü: 300 tahmini token'lık bir parça 550 gerçek token çıktı) — bu yüzden `splitLongWord`'ün bayt/token oranı `maxTokens*3`'ten `maxTokens*1`'e düşürüldü. Yeni `capForEmbedding` helper'ı (`internal/memory/store.go`) hem `saveChunk`'ın `embedText`'ini hem `RetrieveContext`'in üç embed çağrısını da (`query`, `expanded`, her `segment`) tek, güvenli boyutlu bir parçaya sınırlıyor. Regresyon testi: `TestSaveAndRetrieve_LongUnbrokenBlob` (`internal/memory/store_test.go`, gerçek sunucunun "too large" reddini taklit eden sahte bir `EmbeddingFunc` ile). **Bilinen, kabul edilen sınır:** aşırı uç (40.000+ karakter, boşluksuz) bir blob artık ne save'i ne retrieve'i kırıyor — ama bu kadar aşırı boyutta bir tek mesaj, `saveMemorySync`'in 10 saniyelik toplam bütçesine çok sayıda (~67) küçük chunk sırayla kaydedilirken sığmayabiliyor (`context deadline exceeded`, chunk N'den sonra) — bu, orijinal "batch-size" kök nedeninden TAMAMEN AYRI, önceden de var olan bir mimari sınır (chunk başına gerçek bir HTTP çağrısı + genel bir zaman bütçesi); orijinal bug'ın asıl belirtisi (retrieve/LLM cevabının TÜM TURU kırması) artık gerçekleşmiyor, sadece arka plan kaydı kısmi kalabiliyor — canlı olarak 3KB'lik gerçekçi bir blob'un (uzun URL) sorunsuz tam kaydedildiği doğrulandı, sadece 40KB'lik yapay-adversarial test girdisi bu ayrı sınıra çarpıyor.
>
> **Önceki güncelleme:** 2026-07-18 (Session 41) — BUG-L1 düzeltildi: `main.go` artık `*prompt != ""` yerine `flag.Visit` ile "p" bayrağının fiilen geçilip geçilmediğini kontrol ediyor (`promptFlagPassed`), böylece `-p ""` de `runPrintMode`'a düşüyor; `runPrintMode` de boş/whitespace-only prompt için temiz bir `FATAL` mesajıyla hemen çıkıyor (sonsuz askıda kalma yok). Regresyon testi: `TestEmptyPromptExitsCleanly` (`main_test.go`, gerçek binary'yi subprocess olarak çalıştırıp context timeout'uyla sınırlıyor).
>
> **Önceki güncelleme:** 2026-07-17 (Session 41) — BUG-M7 düzeltildi: `internal/agent/tools/command.go`'daki `RunCommand` artık komut string'inin içindeki path-benzeri argümanları da (`commandTargetsProtectedPath`/`extractPathTokens`) `read_file`'ın `validatePath`'ıyla aynı sınırla kontrol ediyor — proje dizini dışına çıkıp `defaultProtectedPaths()`'e giren bir hedef (`/etc/...`, `~/.ssh/...`, `../../etc/...` traversal) artık `run_command` ile de reddediliyor. Proje dizini İÇİNDEKİ göreli path'ler (`go build ./...` gibi) yanlışlıkla engellenmiyor — kontrol önce "proje dizini dışında mı" diye bakıyor, sadece o zaman korumalı liste kontrolü yapıyor (projenin kendisi `/home/` gibi korumalı bir önek altında olsa bile). Regresyon testleri: `TestRunCommand_BlocksProtectedPathBypass`, `TestRunCommand_AllowsOrdinaryProjectCommands`, `TestCommandTargetsProtectedPath` (`internal/agent/tools/command_test.go`).
>
> **Önceki güncelleme:** 2026-07-17 (Session 41) — BUG-M6 düzeltildi: `internal/app/memory.go`'daki `extractAndPinFacts` artık her fact'i pinlemeden önce mevcut pinned fact'lere karşı normalize edilmiş (küçük harf, trim, sondaki noktalama kırpılmış) exact-match dedup kontrolü yapıyor (`pinnedFactTexts`, `normalizeFactText`) — aynı gerçek art arda turlarda tekrar pinlenmiyor. Regresyon testi: `TestExtractAndPinFacts_SkipsAlreadyPinnedDuplicate`.
>
> **Önceki güncelleme:** 2026-07-17 (Session 41) — BUG-M5 düzeltildi: yeni `internal/agent/tools/calendar.go`'daki salt-okunur `get_calendar_events` agent tool'u eklendi (registry: `internal/agent/tools.go`), `internal/app/learning.go`'daki `calendarToolAdapter` gerçek `calendar.Store`'u sarıyor. Agent sistem promptuna (`internal/app/chat.go`'daki `buildAgentSystemPrompt`) model takvim sorgusunda RAG'dan tahmin etmek yerine bu aracı çağırsın diye bir not eklendi. Regresyon testleri: `internal/agent/tools/calendar_test.go`.
>
> **Önceki güncelleme:** 2026-07-17 (Session 41) — BUG-H4 düzeltildi: `internal/app/memory.go`'daki `extractAndPinFacts` artık extraction promptuna SADECE `userMsg`'i veriyor, asistanın (tool sonuçlarını da içerebilen) `reply`'sini hiç göndermiyor — üçüncü şahıs bilgisinin (WhatsApp vb.) kullanıcının kendi kalıcı gerçeği sanılıp pinlenmesi artık mümkün değil. Fonksiyon imzasından artık ölü olan `reply` parametresi de kaldırıldı (tek çağıran `saveMemorySync` ve testler güncellendi). Regresyon testi: `TestExtractAndPinFacts_DoesNotSendAssistantReply` (`internal/app/memory_test.go`).
>
> **Önceki güncelleme:** 2026-07-17 (Session 41) — BUG-H3 düzeltildi: `internal/memory/chunker.go`'daki `chunkText`, `maxTokens`'ı tek başına aşan boşluksuz bir "kelime"yi (uzun URL, base64, minify kod, hash) artık `splitLongWord` ile karakter bazlı zorla parçalıyor — hem `SaveInteraction` hem `RetrieveContext` aynı embed batch-size sınırına çarpmaktan kurtuldu. Regresyon testi: `TestChunkText_SingleOversizedWord` (`internal/memory/chunker_test.go`).
>
> **Önceki güncelleme:** 2026-07-17 (Session 41) — BUG-H2 düzeltildi: `internal/intent/extractor.go`'daki `rawIntent.HabitDays` artık `json.RawMessage` olarak leniently parse ediliyor (`parseHabitDays`), LLM `habit_days`'i `[]int` yerine string/string-dizisi/doğal dil ifadesi ("hafta içi") döndürdüğünde bile tüm `rawIntent` reddedilmiyor. Regresyon testleri: `TestExtractHabit_HabitDaysAsPhrase`, `TestExtractHabit_HabitDaysAsStringArray` (`internal/intent/intent_test.go`).
>
> **Önceki güncelleme:** 2026-07-17 (Session 40, "Ece" persona testi — CANLI WhatsApp ortamı) — `STRESS_TEST_FINDINGS.md` artık ayrı tutulmuyor, tüm bulgular (Session 39 + Session 40) buraya konsolide edildi ve o dosya silindi. BUG-H2 yeni kanıtla derinleştirildi; 6 yeni madde eklendi (2 HIGH, 3 MEDIUM, 1 LOW — Session 39'dan taşınan 2 madde dahil). Fix uygulanmadı, sadece bulundu ve dokümante edildi — bkz. `handoff.md` Session 40.
>
> **Önceki güncelleme:** 2026-07-17 (Session 39, "Deniz" persona testi) — otonom `-p --auto-allow` canlı testinde yeni bir gerçek bug bulundu: BUG-H2.
>
> **Daha önceki güncelleme:** 2026-07-17 (2. tur) — `/code-review` (high effort, 8 bulucu açı + doğrulama) ile bulunan 19 maddeden 17'si bu turda düzeltildi (2 CRITICAL, 1 HIGH, 3 MEDIUM, 6 LOW, 5 teknik borç — her biri ayrı, doğrulanmış commit, `go build/vet/test -race` + `flutter analyze/test` yeşil). Kalan 2 MEDIUM madde (BUG-M1, BUG-M4) bilinçli olarak ertelendi: ikisi de gerçek bir API/mimari kararı gerektiriyor (backend'e dil alanı eklemek, rutin saatine zaman dilimi eklemek) — yarım yamalak, otonom bir oturumda tek taraflı karar vermek yerine sonraki oturuma/kullanıcı onayına bırakıldı.
>
> **Önceki geçmiş (2026-07-12 ve öncesi):** Bu dosya o tarihte 0 açık maddeye indirilmişti. 2026-07-17'nin ilk turunda (yukarıdaki 19 madde) eklenen yeni "rutin" (scheduled automation) motorü + geniş L10n temizliği taranmıştı.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 0 |
| 🟡 MEDIUM | 2 |
| 🟢 LOW | 1 |
| 🔧 TEKNİK BORÇ | 0 |
| **TOPLAM** | **3** |

---

## 🟡 MEDIUM

### BUG-M1: Backend'in ürettiği rutin içeriği (sistem promptu, bildirim başlığı, boş-bağlam metinleri) tamamen hardcoded Türkçe — yeni L10n sistemini baypas ediyor

**Dosya:** `internal/app/routine.go:161` (`Title: "Rutin"` — `GetRoutinesReadyForMobile`), `:178-179` (`routineSystemPrompt`), `:279`/`:294` (`formatEventsForRoutine`/`formatWhatsAppMessagesForRoutine`)

Aynı oturumda mobile'a `routine_fallback` L10n key'i ("Rutin"/"Routine") eklenmiş ama backend bunu kullanmıyor, ham `"Rutin"` string'i basıyor. Aynı şekilde LLM'e verilen sistem promptu ve "Bugün için takvimde etkinlik yok."/"Bu sohbette yeni mesaj yok." gibi bağlam metinleri sabit Türkçe. Mobil/masaüstü dil anahtarı (`locale_provider.dart`) tamamen client-side (SharedPreferences), backend'e hiç iletilmiyor — yani backend'in bunu düzeltmesi için önce API'ye bir dil alanı eklenmesi gerekiyor.

**Senaryo:** Uygulamayı İngilizce'ye çeviren bir kullanıcı yine de her rutin bildirimini "Rutin" başlığıyla ve LLM'in ürettiği (muhtemelen Türkçe bağlam enjekte edildiği için Türkçe'ye kayabilen) içerikle alır — bu oturumun "mobile full TR/EN L10n" commit mesajlarının iddia ettiği kapsamın dışında kalan gerçek bir boşluk.

### BUG-M4: Rutin saatinde (`HH:MM`) hiç zaman dilimi bilgisi tutulmuyor — backend host'un yerel saatine göre yorumlanıyor

**Dosya:** `internal/routine/types.go:24` (`TimeOfDay`), `internal/routine/loop.go:127-133` (`ParseFireTime`)

`ParseFireTime`, `"HH:MM"`'i `now.Location()` (backend process'in çalıştığı makinenin saat dilimi) ile çözüyor. Telefon/kullanıcı backend'in çalıştığı yerden farklı bir saat diliminde olursa (uzaktan erişimle seyahat halindeyken), rutin yanlış saatte ateşlenir ve bunu tespit/düzeltecek hiçbir alan yok.

**Senaryo:** Kullanıcı seyahatteyken telefonundan "sabah 8'de" bir rutin kurar; backend farklı bir saat diliminde çalışıyorsa bildirim kullanıcının gerçek sabah 8'inde değil, backend'in yerel 8'inde gelir.

---

## 🟢 LOW

### BUG-L2: WhatsApp gönderimi iki ayrı yoldan geçiyor — AI agent tool'u (`whatsapp_send`) gerçek bir kişiye otomatik mesaj göndermeyi kendi muhakemesiyle reddedebiliyor, ama doğrudan REST/GUI yolu (`/api/whatsapp/send`) hiçbir onay olmadan koşulsuz çalışıyor

**Dosya:** `internal/agent/tools/whatsapp.go` (`SendWhatsApp`, LLM tool-calling üzerinden, izin ekranına tabi) vs. `internal/webserver/handlers_flutter.go:1478` (`handleWhatsAppSend`, GUI'nin WhatsApp sekmesinin kullandığı doğrudan endpoint, LLM'in onayına hiç girmiyor)

**Canlı doğrulama (Session 40):** Kullanıcının izniyle, SADECE `Annnem` kontağına (905457348509@s.whatsapp.net, doğrulandı), zorunlu "bu bir test mesajıdır" ibaresi içeren bir gönderim denendi. Sohbet üzerinden (`whatsapp_send` agent tool'u, `--auto-allow` açıkken) **3 farklı, dürüst rephrase denemesinde de** model kendi kararıyla reddetti: "annenin haberi olmadan otomatik mesaj göndermek doğru değil, endişelenir." `backend.log`'da bu 3 denemede de `whatsapp_send` hiç çağrılmadığı doğrulandı (model tool'u hiç çağırmadan konuşma seviyesinde reddetti). Ardından AYNI mesaj `/api/whatsapp/send` REST endpoint'i (GUI'nin WhatsApp sekmesinin kullandığı yol) üzerinden doğrudan gönderildi — **koşulsuz, anında başarılı oldu** (2 mesaj, `Annnem`'in sohbet geçmişinde `from_me:true` olarak doğrulandı).

**Değerlendirme:** Bu kod-seviyesinde bir bug değil — iki gönderim yolunun kasıtlı olarak farklı güvenlik modelleri var (biri LLM'in kendi takdirine bırakılmış, diğeri doğrudan kullanıcı eylemi). Ama bu, geliştirici için gerçek bir tutarlılık/UX sorusu: eğer "Memo'nun benim adıma WhatsApp mesajı gönderebilmesi" (agent tool üzerinden) istenen bir özellikse, model kendi başına oldukça muhafazakar davranıyor ve bu, kullanıcı için sürpriz olabilir ("neden gönderemiyorsun, ben istedim" — agent ısrarla reddederken GUI'den aynı mesaj anında gidiyor).

**Önerilen yön:** Bilinçli bir tasarım kararı olarak dokümante edilmeli; istenirse agent tool'un system promptuna, kullanıcı doğrudan ve açıkça onay verdiğinde (bu oturumdaki gibi tekrarlanan, net bir talep) ne zaman gönderime izin vermesi gerektiğine dair daha net bir yönerge eklenebilir.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
