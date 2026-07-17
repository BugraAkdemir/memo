# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-17 — `/code-review` (high effort, 8 bulucu açı + doğrulama) + `/codebase-memory` ile son 15 commit'lik aralık (`HEAD~15..HEAD`, `HEAD~17..HEAD` bazı ajanlarda) tarandı: yeni "rutin" (scheduled automation) motoru — `internal/routine`, `internal/app/routine.go`, `internal/webserver/handlers_routine.go`, masaüstü+mobil Rutinler ekranları — ve ayrı, geniş bir L10n (TR/EN localizasyon) temizliği. 8 bulucu ajan (satır satır tarama, kaldırılan davranış denetimi, çapraz-dosya izleme, reuse, sadeleştirme, verimlilik, altitude, AGENTS.md uyumu) paralel çalıştı; en kritik 2 iddia bizzat kodda doğrulandı. Bu turda hiç fix uygulanmadı — sadece bulundu ve raporlandı.
>
> **Önceki geçmiş (2026-07-12 ve öncesi):** Bu dosya o tarihte 0 açık maddeye indirilmişti (bkz. eski özet altında). Aşağıdaki tüm maddeler 2026-07-17'de eklenen yeni özellikler.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 0 |
| 🟡 MEDIUM | 2 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 5 |
| **TOPLAM** | **7** |

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

## 🔧 TEKNİK BORÇ

### TD-1: `extractJSON` (`internal/routine/extractor.go`), `internal/proactive/decision.go`'daki `extractJSONObject` ile neredeyse birebir aynı

Dengeli-parantez JSON çıkarma deseni artık en az 2 (fiilen `internal/intent`'i de sayarsak 3) kopya halinde — ortak bir helper'a çıkarılmamış. Bir kopyada bulunacak bir hata diğerlerinde sessizce kalır.

### TD-2: `truncateForError`, aynı "kes + ..." helper'ının (en az) 6. kopyası

`internal/app/helpers.go` (`truncateLog`), `internal/intent` (`truncate`), `internal/proactive` (`truncate`), `internal/taskloop` (`truncateText`, rune-safe), `internal/orchestra` (`truncateUTF8`, rune-safe), şimdi `internal/routine` (`truncateForError`, rune-safe DEĞİL). Rune-safe olmayan kopyalar çok baytlı UTF-8 karakterleri (Türkçe ı,ş,ğ,ü,ö,ç) ortasından kesip bozabilir.

### TD-3: `logout_2`/`retry_2` L10n key'leri, mevcut `logout`/`retry` ile kazara neredeyse birebir aynı

`frontend/lib/core/l10n.dart` — İngilizce'de byte-byte aynı, Türkçe'de sadece büyük/küçük harf farkı. Aynı serideki `vision_2`/`code_2` gibi meşru varyantların aksine bu ikisi gerçek bir kopya, farklı bir anlamı yok.

### TD-4: `learning_saved` (mobil) / `learning_settings_saved` (masaüstü) — aynı kavram, aynı serüvende farklı isimlendirilmiş

Aynı L10n temizliği sırasında eklenen, birebir aynı anlamdaki iki key'in adı iki uygulama arasında tutarsız — gelecekte cross-app L10n parity denetimini zorlaştırır.

### TD-5: `Routine.LastGeneratedForDate`, `LastGeneratedAt`'tan türetilebilir — gereksiz ayrı alan

`internal/routine/types.go:79`, `loop.go:93-95` — ikisi her zaman birlikte, aynı `now`'dan set ediliyor. Gelecekte `LastGeneratedAt`'ı güncelleyip `LastGeneratedForDate`'i unutan bir değişiklik, aynı-gün-koruma mantığını sessizce bozar.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
