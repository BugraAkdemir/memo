# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-17 (Session 39, "Deniz" persona testi) — otonom `-p --auto-allow` canlı testinde yeni bir gerçek bug bulundu: BUG-H2 (aşağıda). Fix uygulanmadı, sadece bulundu ve dokümante edildi — bkz. `handoff.md` Session 39.
>
> **Önceki güncelleme:** 2026-07-17 (2. tur) — `/code-review` (high effort, 8 bulucu açı + doğrulama) ile bulunan 19 maddeden 17'si bu turda düzeltildi (2 CRITICAL, 1 HIGH, 3 MEDIUM, 6 LOW, 5 teknik borç — her biri ayrı, doğrulanmış commit, `go build/vet/test -race` + `flutter analyze/test` yeşil). Kalan 2 MEDIUM madde (BUG-M1, BUG-M4) bilinçli olarak ertelendi: ikisi de gerçek bir API/mimari kararı gerektiriyor (backend'e dil alanı eklemek, rutin saatine zaman dilimi eklemek) — yarım yamalak, otonom bir oturumda tek taraflı karar vermek yerine sonraki oturuma/kullanıcı onayına bırakıldı.
>
> **Önceki geçmiş (2026-07-12 ve öncesi):** Bu dosya o tarihte 0 açık maddeye indirilmişti. 2026-07-17'nin ilk turunda (yukarıdaki 19 madde) eklenen yeni "rutin" (scheduled automation) motorü + geniş L10n temizliği taranmıştı.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 1 |
| 🟡 MEDIUM | 2 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 0 |
| **TOPLAM** | **3** |

---

## 🟠 HIGH

### BUG-H2: Alışkanlık (habit) deklarasyonunda `habit_days` tip uyuşmazlığı TÜM intent sonucunu sessizce iptal ediyor — model kullanıcıya yalan "kaydedildi" onayı veriyor

**Dosya:** `internal/intent/extractor.go` (`rawIntent.HabitDays []int`, `parseResponse`), `internal/app/learning.go:119-123` (`processMessageIntent`)

"Deniz" persona testinde (bkz. `handoff.md` Session 39) canlı olarak bulundu: kullanıcı "bundan sonra her gün akşam 21:00'de 20 dakika kitap okuyacağım, bunu alışkanlık olarak not al" dediğinde, LLM `habit_days` alanını beklenen `int` dizisi yerine string olarak döndürdü (muhtemelen gün adları). `json.Unmarshal(jsonStr, &ri)` bu tek alan yüzünden TÜM `rawIntent`'i reddediyor — `has_intent`/`is_habit`/`summary` gayet doğru üretilmiş olsa bile. `parseResponse` hata dönüyor, `processMessageIntent` bunu loglayıp sessizce `return` ediyor; `observerPatterns.SaveDeclared` hiç çağrılmıyor, `data/profile/patterns.json`'a hiçbir şey yazılmıyor.

**Senaryo:** Kullanıcı bir alışkanlık deklare eder, ana sohbet modeli (bu arka plan pipeline'ından habersiz olduğu için) her zaman "Alışkanlık kaydedildi! 📖" gibi kesin bir onay verir — ama backend hiçbir şey kaydetmemiştir. Kullanıcı gelecekte "bana hatırlatacağını söylemiştin" dediğinde hiçbir hatırlatma gelmez; sessiz veri kaybı + yanlış kullanıcı güveni.

**Önerilen yön:** `HabitDays`'i `[]int` yerine daha toleranslı bir tip (`json.RawMessage` veya serbest `[]any` + manuel coerce/best-effort parse) yapmak; ayrıca tek bir alanın tipi bozuksa bile geri kalan alanları (has_intent, is_habit, summary, habit_time_hhmm) kurtaracak alan-bazlı bir fallback eklemek düşünülebilir.

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

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
