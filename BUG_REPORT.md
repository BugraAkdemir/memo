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
| 🟠 HIGH | 1 |
| 🟡 MEDIUM | 5 |
| 🟢 LOW | 6 |
| 🔧 TEKNİK BORÇ | 5 |
| **TOPLAM** | **17** |

---

## 🟠 HIGH

### BUG-H1: `RoutineLoop.tick()` tamamen senkron çalışıyor, hiç zaman aşımı yok — yavaş/askıda kalan bir agent rutini tüm diğer rutinleri durdurur

**Dosya:** `internal/routine/loop.go:51-62` (`Start`), `:64-123` (`tick`)

`Start(ctx)`'in `select` döngüsü her dakika `r.tick(ctx, now)`'ı **senkron** çağırıyor; `tick` de `r.store.List()` üzerinde dönüp her ateşlenmesi gereken rutin için `r.generate(ctx, rt)`'yi (agent-mode rutinlerde `sendMessageStreamInnerTo`'nun LLM stream'i, dakikalarca sürebilir) bekliyor. Hiçbir yerde `context.WithTimeout` veya per-routine deadline yok — tek context, `Start`'a `Startup()`'tan geçirilen `lifecycleCtx`, sadece app kapanışında iptal oluyor.

**Senaryo:** Bir agent-mode rutinin LLM çağrısı yanıtsız kalırsa (sağlayıcı takıldı, ağ sorunu), `tick()` hiç dönmez → ticker'ın bir sonraki tick'i hiç işlenmez → o dakikadan sonra **hiçbir rutin çalışmaz**, ne WhatsApp-only olanlar ne mobil olanlar — sıfır lead time'lı bir WhatsApp rutini bile beklemeye girer. Daha hafif durumda (sadece yavaş, askıda değil) aynı dakikaya denk gelen diğer rutinler LLM çağrısının süresi kadar gecikir.

---

## 🟡 MEDIUM

### BUG-M1: Backend'in ürettiği rutin içeriği (sistem promptu, bildirim başlığı, boş-bağlam metinleri) tamamen hardcoded Türkçe — yeni L10n sistemini baypas ediyor

**Dosya:** `internal/app/routine.go:161` (`Title: "Rutin"` — `GetRoutinesReadyForMobile`), `:178-179` (`routineSystemPrompt`), `:279`/`:294` (`formatEventsForRoutine`/`formatWhatsAppMessagesForRoutine`)

Aynı oturumda mobile'a `routine_fallback` L10n key'i ("Rutin"/"Routine") eklenmiş ama backend bunu kullanmıyor, ham `"Rutin"` string'i basıyor. Aynı şekilde LLM'e verilen sistem promptu ve "Bugün için takvimde etkinlik yok."/"Bu sohbette yeni mesaj yok." gibi bağlam metinleri sabit Türkçe. Mobil/masaüstü dil anahtarı (`locale_provider.dart`) tamamen client-side (SharedPreferences), backend'e hiç iletilmiyor — yani backend'in bunu düzeltmesi için önce API'ye bir dil alanı eklenmesi gerekiyor.

**Senaryo:** Uygulamayı İngilizce'ye çeviren bir kullanıcı yine de her rutin bildirimini "Rutin" başlığıyla ve LLM'in ürettiği (muhtemelen Türkçe bağlam enjekte edildiği için Türkçe'ye kayabilen) içerikle alır — bu oturumun "mobile full TR/EN L10n" commit mesajlarının iddia ettiği kapsamın dışında kalan gerçek bir boşluk.

### BUG-M2: Mobil Rutinler ekranı, masaüstündeki "komut çalıştırabilir" (agent-mode) rozetini göstermiyor

**Dosya:** `mobile/lib/screens/routines_screen.dart:228-231` (frontend'deki karşılığı: `frontend/lib/screens/routines_screen.dart:349-353`)

Her iki ekran da rutin kartının alt yazısını aynı üç L10n parçasını (saat, WhatsApp, mobil) birleştirerek kuruyor; masaüstü buna dördüncü bir parça daha ekliyor (`agentMode` ise "komut çalıştırabilir"), mobil eklemiyor — `mobile/lib/models/routine.dart`'ın zaten ayrıştırıp sakladığı `agentMode` alanı mobilde hiç gösterilmiyor.

**Senaryo:** Kullanıcı telefonundan rutinlerini denetlerken, hangi rutinin bilgisayarında gerçek komut/dosya işlemi çalıştırabildiğini (BUG-C1/C2'nin de gösterdiği gibi risk taşıyan kısım) **göremiyor** — güvenlik ilgili bir görünürlük eksikliği.

### BUG-M3: `_friendlyError`'ın fallback yolu, 401/timeout dışındaki hatalarda hâlâ ham exception metnini basıyor

**Dosya:** `mobile/lib/providers/connection_provider.dart:17-29` (`_friendlyError`)

Fonksiyonun kendi doc yorumu amacını "ham DioException metnini UI'a dökmemek" olarak tanımlıyor, ama generic fallback satırı (`return '$action: $e';`) tam olarak bunu yapıyor — sadece 401 ve bağlantı zaman aşımı özel olarak ele alınmış. Ayrıca `mobile/lib/core/api_client.dart`'ın zaten var olan `_extractErrorMessage`'ı (backend'in `{"error": "..."}` gövdesinden temiz mesaj çıkarıyor) yeniden kullanmak yerine bağımsız, daha zayıf bir mantık yazılmış.

**Senaryo:** Backend 400/500 gibi başka bir statü kodla, anlamlı bir hata mesajı içeren gövdeyle dönerse (örn. "routine store not initialized"), kullanıcı yine okunaksız bir `DioException [bad response]: ...` dökümü görür — bu oturumun asıl düzeltmeye çalıştığı sorunun ta kendisi, çoğunluk vakada.

### BUG-M4: Rutin saatinde (`HH:MM`) hiç zaman dilimi bilgisi tutulmuyor — backend host'un yerel saatine göre yorumlanıyor

**Dosya:** `internal/routine/types.go:24` (`TimeOfDay`), `internal/routine/loop.go:127-133` (`ParseFireTime`)

`ParseFireTime`, `"HH:MM"`'i `now.Location()` (backend process'in çalıştığı makinenin saat dilimi) ile çözüyor. Telefon/kullanıcı backend'in çalıştığı yerden farklı bir saat diliminde olursa (uzaktan erişimle seyahat halindeyken), rutin yanlış saatte ateşlenir ve bunu tespit/düzeltecek hiçbir alan yok.

**Senaryo:** Kullanıcı seyahatteyken telefonundan "sabah 8'de" bir rutin kurar; backend farklı bir saat diliminde çalışıyorsa bildirim kullanıcının gerçek sabah 8'inde değil, backend'in yerel 8'inde gelir.

### BUG-M5: Mobil rutin CRUD hataları hiç lokalize edilmemiş ve hiçbir yerde gösterilmiyor

**Dosya:** `mobile/lib/providers/routine_provider.dart` (`_load`, `createFromDraft`, `toggleEnabled`, `delete` — hepsi `error: e.toString()`)

Masaüstü `l10n.dart`'ta `routines_load_error`/`routines_save_error`/`routines_update_error`/`routines_delete_error` gibi key'ler zaten var; mobilin `l10n.dart`'ında sadece `routines_parse_error` taşınmış. Diğer üç işlem hâlâ ham `e.toString()` kullanıyor — üstüne mobil Rutinler ekranı `state.error`'ı hiçbir yerde render etmiyor, yani hata tamamen görünmez.

**Senaryo:** Mobilde bir rutini silme/güncelleme başarısız olursa kullanıcı hiçbir uyarı görmez, işlem sessizce başarısız olur.

---

## 🟢 LOW

### BUG-L1: `checkMobileReady()` bir kalem bozuksa tüm grubu iptal ediyor

**Dosya:** `mobile/lib/providers/routine_provider.dart:62-77`

`try/catch` tüm `for` döngüsünü sarmalıyor, tek tek elemanı değil. `RoutineMobileReady.fromJson`'ın `DateTime.parse(json['fire_at_utc'])` çağrısı bozuk/eksik bir tarihte patlarsa, o partide gelen **diğer tüm** rutinlerin zamanlaması da o turda atlanır (bir sonraki pollde kendini düzeltir, ama gecikme olur).

### BUG-L2: WhatsApp sohbet listesi çekilemezse hedef JID boş kalıp yine de kaydediliyor

**Dosya:** `frontend/lib/screens/routines_screen.dart` (`_parse`, sohbet listesi `catch (_) {}` ile yutuluyor) ve mobil eşdeğeri

WhatsApp bağlı değilken/geçici kesintide bir WhatsApp-teslimatlı rutin onaylanırsa, `whatsAppTargetJid: _selectedWhatsAppJid ?? ''` boş string ile kaydedilir — kullanıcıya hiç uyarı gösterilmeden.

### BUG-L3: `checkMobileReady()` sadece `routine:ready` event'iyle tetikleniyor, açılış/yeniden bağlanmada telafi çağrısı yok

**Dosya:** `mobile/lib/main.dart:118-123` (routine kısmı), karşılaştır `:138-142` (calendar'ın reconnect-refresh deseni)

Takvim akışı her yeniden bağlanmada `calendarProvider.notifier.refresh()`'i koşulsuz çağırıyor; rutin tarafının eşdeğeri yok. Uygulama tamamen kapalıyken bir `routine:ready` event'i gelirse (ya da 64 elemanlık ring buffer'dan taşarsa), o günün bildirimi hiç zamanlanmaz.

### BUG-L4: Nav rail'deki "Rutinler" etiketi, hazır L10n key'i dururken hardcoded

**Dosya:** `frontend/lib/screens/app_shell.dart:345`

`_NavRailButton(... label: 'Rutinler', ...)` — aynı serideki `l10n.dart`'a eklenen `routines_title` key'i (Rutinler ekranının kendi `AppBar` başlığında doğru kullanılıyor) burada kullanılmamış.

### BUG-L5: `formatWhatsAppMessagesForRoutine` gönderen adı boşsa fallback yapmıyor

**Dosya:** `internal/app/routine.go:294-303`, karşılaştır `internal/agent/tools/whatsapp.go:158-167`

Var olan agent tool'u (`GetWhatsAppMessages`), `SenderName` boşsa JID'den isim türetiyor (`partsBeforeAt`); yeni rutin formatlayıcısı bu fallback'i yapmadan direkt `SenderName`'i yazıyor.

**Senaryo:** Kayıtlı görünen adı olmayan bir kişiden gelen mesajlar rutin bağlamında `[15:04] : mesaj` gibi boş gönderen adıyla görünür.

### BUG-L6: `Store.Update()` global kilidi senkron disk yazımı boyunca tutuyor

**Dosya:** `internal/routine/store.go:117-130`

`s.mu.Lock()` tüm `json.MarshalIndent` + `os.WriteFile` süresince açık kalıyor; eşzamanlı `List()`/`Get()` çağıran HTTP handler'lar (mobil poll, masaüstü liste) bu süre boyunca bekler. Küçük ölçekte önemsiz, çok rutinli/yavaş disk senaryosunda gecikmeye dönüşebilir.

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
