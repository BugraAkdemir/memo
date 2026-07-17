# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-17 (Session 41) — BUG-M5 düzeltildi: yeni `internal/agent/tools/calendar.go`'daki salt-okunur `get_calendar_events` agent tool'u eklendi (registry: `internal/agent/tools.go`), `internal/app/learning.go`'daki `calendarToolAdapter` gerçek `calendar.Store`'u sarıyor. Agent sistem promptuna (`internal/app/chat.go`'daki `buildAgentSystemPrompt`) model takvim sorgusunda RAG'dan tahmin etmek yerine bu aracı çağırsın diye bir not eklendi. Regresyon testleri: `internal/agent/tools/calendar_test.go`.
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
| 🟡 MEDIUM | 4 |
| 🟢 LOW | 2 |
| 🔧 TEKNİK BORÇ | 0 |
| **TOPLAM** | **6** |

---

## 🟡 MEDIUM

### BUG-M6: `extractAndPinFacts`'te tekrar/dedup kontrolü yok — aynı gerçek tur başına tekrar tekrar pinleniyor, kapasiteli pinned-facts havuzunu kendi kopyalarıyla dolduruyor

**Dosya:** `internal/app/memory.go:182-186` (döngü, her fact için koşulsuz `SaveExplicitMemory` çağrısı, hiçbir dedup kontrolü yok)

~16 mesajlık bir persona sohbeti sonunda `source='explicit'` kayıtları arasında 19 tanesi aynı kişiyle ilgiliydi, 8 tanesi neredeyse birebir aynı ifadeydi (`"User's name is Ece."` 4 kez tam aynı metin dahil). `"User has a mother."` art arda 3 kez ayrı ayrı pinlendi.

**Kök neden:** Her turda bağımsız bir LLM çağrısıyla üretilen fact listesi, halihazırda aynı/benzer bir fact pinlenmiş mi diye kontrol edilmeden doğrudan `SaveExplicitMemory`'e veriliyor. Pinned facts RAG ranking'i bypass ederek her sisteme promptuna koşulsuz enjekte edildiği için, bu kopyalar gerçek bir token/maliyet yükü olarak her turda tekrarlanıyor.

**Senaryo:** Uzun süre kullanan bir kullanıcı için, sık tekrar eden temel bilgiler (isim, şehir, meslek), sınırlı `pinnedFactsLimit` slotunu SADECE KENDİ KOPYALARIYLA doldurup, tek seferlik bahsedilmiş değerli gerçekleri (ör. "köpeğimin adı Zeytin") dışarıda bırakabilir.

**Önerilen yön:** `SaveExplicitMemory`'den önce basit bir metin-benzerliği/exact-match dedup kontrolü eklemek; alternatif olarak periyodik bir "pinned facts consolidation" geçişi.

### BUG-M7: `run_command`, `read_file`'ın uyguladığı "korumalı dizin" sınırını tamamen atlıyor — model kendi sınırlarını yanlış anlatıyor

**Dosya:** `internal/agent/tools/command.go` (`RunCommand`, `isBlacklisted`) vs. `internal/agent/tools/file.go` (`validatePath`, `defaultProtectedPaths`)

`read_file` ile `../../../../etc/passwd` denendiğinde doğru şekilde reddedildi ("access denied: path is within protected directory"), model kullanıcıya "sistem dosyalarına erişimim yok" dedi. Hemen ardından AYNI hedefe `run_command` ile (`cat /etc/hostname && whoami && printenv HOME`) ulaşılmaya çalışıldığında **tamamen başarılı** oldu.

**Kök neden:** `file.go`'daki `validatePath`, hedef path'i `defaultProtectedPaths()`'e karşı kontrol ediyor — ama `command.go`'daki `RunCommand` SADECE `cwd` argümanının proje dizini içinde olduğunu doğruluyor; komut STRING'İNİN İÇİNDEKİ path'lere (`cat /etc/shadow`, `cat ~/.ssh/id_rsa`) hiçbir kısıtlama uygulamıyor. `isBlacklisted` sadece yıkıcı komut kalıplarını (rm -rf /, sudo, vb.) engelliyor, okuma/bilgi-sızdırma amaçlı komutlar listede yok.

**Senaryo:** Model `read_file` denerse korumalı dizin engeline takılıp kullanıcıya "erişimim yok" der — ama aynı model, aynı hedefe `run_command` ile sorunsuzca ulaşabilir. Bu hem gerçek bir güvenlik sınırı tutarsızlığı hem de kullanıcıya YANLIŞ bir güvenlik hissi veriyor.

**Önerilen yön:** Muhtemelen bilinçli bir tasarım kararı (run_command'ı tamamen kısıtlamak onu işlevsiz kılar) — ama en azından (a) izin ekranında "bu komut proje dizini dışına erişebilir" uyarısı gösterilmeli, (b) modelin sistem promptu bu ayrımı açıkça anlatmalı.

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

### BUG-L1: `memo -p ""` (boş prompt) CLI'ı sonsuza kadar askıda bırakıyor, "Shutting down backend..." yanıltıcı mesajı basıyor

**Dosya:** `main.go:38` (`if *prompt != "" { runPrintMode(...); return }`), `main.go:73-133` (headless/non-interactive fallback yolu)

```
$ timeout 8 memo-dev -p ""
Shutting down backend...
$ echo $?
124   # timeout process'i zorla öldürdü, kendiliğinden hiç çıkmadı
```

**Kök neden:** Go'nun `flag` paketinde `-p ""` geçerli, boş bir string değeridir — kontrol `*prompt != ""` olduğu için **boş prompt, "-p hiç verilmemiş" ile ayırt edilemiyor**. `runPrintMode` hiç çağrılmıyor, kod `interactive := !*headless && isInteractive()` dalına düşüyor; TTY olmadığı için (script/otomasyon bağlamı) port zaten dolu olduğundan yeni backend başlatmıyor ama kendi başına sonsuz bir SIGINT/SIGTERM bekleme döngüsüne giriyor, dışarıdan öldürülmeden asla çıkmıyor.

**Senaryo:** Bir script (`for msg in "${messages[@]}"; do memo -p "$msg"; done`) boş bir eleman üretirse (trim edilmiş boş satır, template'te boş interpolasyon), script SESSİZCE sonsuza kadar askıda kalır. Basılan "Shutting down backend..." mesajı da yanıltıcı: gerçek arka plan backend'i hiç etkilenmiyor, sadece bu çıkmayan foreground process (zorla sinyal alınca) bu mesajı basıp çıkıyor.

**Önerilen yön:** `flag.Visit` ile "p" bayrağının fiilen geçilip geçilmediğini kontrol etmek, böylece `-p ""` de `-p "gerçek mesaj"` gibi `runPrintMode`'a düşsün (muhtemelen sonra "boş mesaj gönderilemez" gibi temiz bir hata verir).

### BUG-L2: WhatsApp gönderimi iki ayrı yoldan geçiyor — AI agent tool'u (`whatsapp_send`) gerçek bir kişiye otomatik mesaj göndermeyi kendi muhakemesiyle reddedebiliyor, ama doğrudan REST/GUI yolu (`/api/whatsapp/send`) hiçbir onay olmadan koşulsuz çalışıyor

**Dosya:** `internal/agent/tools/whatsapp.go` (`SendWhatsApp`, LLM tool-calling üzerinden, izin ekranına tabi) vs. `internal/webserver/handlers_flutter.go:1478` (`handleWhatsAppSend`, GUI'nin WhatsApp sekmesinin kullandığı doğrudan endpoint, LLM'in onayına hiç girmiyor)

**Canlı doğrulama (Session 40):** Kullanıcının izniyle, SADECE `Annnem` kontağına (905457348509@s.whatsapp.net, doğrulandı), zorunlu "bu bir test mesajıdır" ibaresi içeren bir gönderim denendi. Sohbet üzerinden (`whatsapp_send` agent tool'u, `--auto-allow` açıkken) **3 farklı, dürüst rephrase denemesinde de** model kendi kararıyla reddetti: "annenin haberi olmadan otomatik mesaj göndermek doğru değil, endişelenir." `backend.log`'da bu 3 denemede de `whatsapp_send` hiç çağrılmadığı doğrulandı (model tool'u hiç çağırmadan konuşma seviyesinde reddetti). Ardından AYNI mesaj `/api/whatsapp/send` REST endpoint'i (GUI'nin WhatsApp sekmesinin kullandığı yol) üzerinden doğrudan gönderildi — **koşulsuz, anında başarılı oldu** (2 mesaj, `Annnem`'in sohbet geçmişinde `from_me:true` olarak doğrulandı).

**Değerlendirme:** Bu kod-seviyesinde bir bug değil — iki gönderim yolunun kasıtlı olarak farklı güvenlik modelleri var (biri LLM'in kendi takdirine bırakılmış, diğeri doğrudan kullanıcı eylemi). Ama bu, geliştirici için gerçek bir tutarlılık/UX sorusu: eğer "Memo'nun benim adıma WhatsApp mesajı gönderebilmesi" (agent tool üzerinden) istenen bir özellikse, model kendi başına oldukça muhafazakar davranıyor ve bu, kullanıcı için sürpriz olabilir ("neden gönderemiyorsun, ben istedim" — agent ısrarla reddederken GUI'den aynı mesaj anında gidiyor).

**Önerilen yön:** Bilinçli bir tasarım kararı olarak dokümante edilmeli; istenirse agent tool'un system promptuna, kullanıcı doğrudan ve açıkça onay verdiğinde (bu oturumdaki gibi tekrarlanan, net bir talep) ne zaman gönderime izin vermesi gerektiğine dair daha net bir yönerge eklenebilir.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
