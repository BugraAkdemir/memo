# Memo — Genel Özellik / Bug / Hazırlık Denetim Raporu

> **Denetim tarihi:** 2026-09-14  
> **Kapsam:** Yeni feature geliştirmek yerine mevcut ürünün tamamına yönelik ikinci genel bug/reliability taraması. Backend (Go), Flutter, CLI, memory/RAG, agent, providers, local inference, model store, orchestra, taskloop, calendar/routines, observer/proactive/mood, WhatsApp/Telegram, voice/live, cloud sync, remote/self-host, skills, installers, mascot ve güvenlik yüzeyleri incelendi.
>
> **Önemli:** Bu rapordaki yüzdeler test coverage değildir. Kodun ürünleşme/maturity tahminidir. Bu taramada yerel build veya gerçek provider/mesajlaşma hesabı E2E çalıştırılmadı; GitHub `main` kaynak ağacı ve mevcut dokümantasyon/kod aramaları kullanıldı.

## 1. Kısa karar

**Evet: sıradaki adım yeni feature değil, mevcut feature'ları sertleştirmek olmalı.**

Bu taramada yeni bir P0/P1 seviyesinde kesin fonksiyonel bug doğrulayamadım. Buna karşılık birkaç **P1/P2 reliability/design riski** açıkça görünüyor. Bunlar özellikle concurrency, uzun yaşayan bağlantılar, fallback zincirleri ve gerçek deployment kombinasyonlarında ortaya çıkabilir.

### Genel ürün olgunluğu: **%89**

| Katman | Maturity |
|---|---:|
| Core Chat | **%94** |
| Memory / RAG | **%91** |
| Agent | **%93** |
| Providers | **%92** |
| Local inference | **%91** |
| Model Store | **%92** |
| Orchestra | **%90** |
| Taskloop | **%88** |
| Calendar / Routines | **%91** |
| Observer / Proactive / Mood | **%88** |
| WhatsApp / Telegram | **%89** |
| Voice / Live | **%88** |
| Cloud Sync | **%88** |
| Self-host / Remote | **%87** |
| Security / Privacy | **%94** |
| Skills | **%90** |
| CLI | **%90** |
| Mascot | **%95** |
| Frontend | **%88** |
| Backend | **%91** |
| Deployment / Integration | **%84** |
| **GENEL** | **%89** |

---

# 2. Tarama sonucu — açık bug adayları ve riskler

## 🔴 B1 — `client` / `providerRouter` yeniden atanırken streaming goroutine'lerinin eski referansı kullanabilmesi

**Kategori:** concurrency / lifecycle  
**Öncelik:** P1 reliability  
**Durum:** Açık tasarım riski

`internal/app` içinde LLM client ve provider router yaşam döngüsü model değişimi/yeniden başlatma ile bağlantılı. Mevcut Known Issues kaydına göre `clientMu` mevcut olsa da streaming goroutine'leri model stop/start arasında eski client/router referansını taşıyabiliyor.

**Muhtemel etki:** Çok nadir zamanlamalarda eski provider'a istek gitmesi, provider state değişiminin stream ile yarışması veya stop/swap sırasında beklenmeyen hata.

**Önerilen çözüm:** Client/router snapshot'ını stream başlangıcında atomik/lock altında almak ve lifecycle generation/version ile stream'in artık geçersiz client kullanmasını engellemek. Önce concurrency regression testleri yazılmalı.

**Kaynak:** `obsidian-doc-en/Memo/Known Issues.md`.

---

## 🟠 B2 — Orchestra, provider Router/fallback zincirini bypass ediyor

**Kategori:** entegrasyon / resilience  
**Öncelik:** P2

Orchestra provider'ları doğrudan oluşturuyor; normal `provider.Router` fallback zincirinden geçmiyor.

**Muhtemel etki:** Normal chat'te çalışan fallback/retry davranışı Orchestra içinde aynı şekilde çalışmayabilir. Bir provider transient hata verdiğinde Orchestra gereksiz şekilde görevi başarısız bırakabilir.

**Öneri:** Orchestra'nın provider resolution'ını Router üzerinden geçirmek veya Orchestra için açıkça tanımlanmış ayrı fallback policy oluşturmak. Bu davranış integration test ile sabitlenmeli.

---

## 🟠 B3 — `provider.Priority` tanımlı fakat router davranışına bağlı değil

**Kategori:** davranış / configuration debt  
**Öncelik:** P2

Provider modelinde `Priority` alanı bulunmasına rağmen router tarafındaki sıralama davranışına bağlanmamış.

**Muhtemel etki:** Kullanıcı/config tarafından beklenen provider önceliği ile gerçek fallback sırası farklı olabilir.

**Öneri:** Ya alan tamamen kaldırılmalı ya da tek bir yerde gerçek routing policy'ye bağlanmalı; iki farklı "source of truth" bırakılmamalı.

---

## 🟠 B4 — Orchestra paketinde yaklaşık 800 satır kod için test boşluğu

**Kategori:** test / regression risk  
**Öncelik:** P2

Mevcut Known Issues kaydında Orchestra paketi için ayrı test dosyalarının olmadığı belirtiliyor.

**Muhtemel etki:** Parallel execution, fallback, cancellation ve multi-provider davranışlarındaki regressions kolayca production'a kaçabilir.

**Öneri:** Yeni feature eklemek yerine Orchestra için önce deterministic unit + concurrency tests: provider failure, cancellation, partial specialist failure, synthesis failure, parallel completion ordering ve context cancellation.

---

## 🟠 B5 — Taskloop uzun görevlerde recovery/soak riski

**Kategori:** state machine / persistence / concurrency  
**Öncelik:** P1 reliability

Taskloop; LLM planning, filesystem, shell, acceptance checks, retry, escalation, parallel step ve persistent state'i aynı zincirde birleştiriyor. Mevcut maturity değerlendirmesinde en düşük ana özelliklerden biri (%88) ve long-running recovery %84.

**Muhtemel etki:** 30–60 dakikalık görevlerde process restart, network/provider timeout, tek bir step'in stuck olması veya parallel step race'i tüm görevin durumunu bozabilir.

**Öneri:** Yeni Taskloop özelliği eklemek yerine crash/restart/resume, timeout, retry budget, cancellation ve idempotency test matrisi hazırlanmalı.

---

## 🟠 B6 — Live Mode'da echo cancellation yok

**Kategori:** gerçek zamanlı audio UX  
**Öncelik:** P2

Live Mode için mevcut Known Issues kaydı echo cancellation olmadığını belirtiyor.

**Muhtemel etki:** Hoparlör kullanılırken Memo kendi TTS çıktısını kullanıcı konuşması/interruption gibi algılayabilir.

**Öneri:** Audio pipeline'a echo cancellation/echo suppression veya güvenilir output-reference tabanlı filtre eklenene kadar bu durum Live Mode beta limitation olarak açıkça korunmalı. Headphones ile smoke test de release checklist'e girmeli.

---

## 🟠 B7 — Live Mode / WhatsApp / Telegram long-lived connection resilience

**Kategori:** lifecycle / network  
**Öncelik:** P1 reliability

Bu üç sistem kısa isteklerden farklı olarak uzun yaşayan bağlantılar kullanıyor. Reconnect, network drop, server restart, authentication expiry ve concurrent session kombinasyonları statik kod taramasından tam kanıtlanamıyor.

**Öneri:** Ortak long-lived-session test matrisi:

1. bağlantı kurulması
2. 5–15 dk idle
3. network drop
4. network geri gelmesi
5. backend restart
6. auth expiry/re-auth
7. iki concurrent session
8. clean shutdown

---

## 🟡 B8 — Unsupported platformlarda RAM detection `0` dönüyor

`internal/llama/sysram_other.go`, Linux/Windows/macOS dışındaki platformlarda `systemRAMMb()` için `0` döndürüyor.

Bu doğrudan crash bug'ı değil; ancak hardware-fit/model recommendation katmanında `0` değerinin "RAM bilinmiyor" ile "0 MB RAM" ayrımını doğru yapması gerekiyor.

**Öneri:** Unknown state'i açıkça modelle (`ok=false` / nullable) ve recommendation kodunun `0`'ı gerçek RAM gibi yorumlamadığını test et.

---

## 🟡 B9 — Embedding model auto-start hâlâ configuration'a bağlı

Memory/RAG dokümantasyonunda embedding model için config-driven auto-start bulunuyor fakat kurulum/konfigürasyon gerektiriyor.

Bu bir bug değil; fakat fresh install deneyiminde memory'nin ilk kullanımda neden çalışmadığı anlaşılmazsa "Memory bozuk" algısı oluşturabilir.

**Öneri:** Setup sırasında embedding dependency health-check + açık UI status + recovery path.

---

## 🟡 B10 — Pinned facts kapasitesi ve local inference slot contention

Pinned facts için sabit bir üst sınır ve consolidation davranışı mevcut. Ayrıca local model kurulumunda background fact extraction, `--parallel 1` durumunda gerçek chat ile aynı inference slotunu paylaşabiliyor.

**Muhtemel etki:** Çok yoğun memory extraction altında chat latency artabilir; çok uzun süre biriken pinned facts önemli eski bilginin yerini doldurabilir.

**Öneri:** Priority-aware eviction/consolidation ve background extraction için starvation/latency budget.

---

## 🟡 B11 — Self-host deployment matrix hâlâ en zayıf büyük yüzeylerden biri

Native server, Docker/CasaOS, systemd, remote auth, installer/update ve RPi kombinasyonları mevcut. Kod olgun olsa da gerçek cihaz/OS kombinasyonu statik taramayla doğrulanamaz.

**Öneri:** Her release'te minimum smoke matrix:

`fresh install → setup → auth → chat → memory → model → restart → update → reinstall → remote login`.

---

## 🟡 B12 — Backend user-facing error stringleri Flutter L10n sınırını aşabiliyor

Flutter tarafında user-facing stringler için L10n zorunlu. Backend'in doğrudan insan-readable hata cümlesi döndürdüğü yerlerde dil tutarlılığı bozulabilir.

**Öneri:** Backend stable error/status code + structured parameters döndürsün; son kullanıcı cümlesini Flutter L10n üretsin.

---

## 🟡 B13 — API versioning yok

API yüzeyi düz `/api/...` olarak ilerliyor; `/api/v1` gibi bir versioning stratejisi bulunmuyor.

Bug değil fakat self-hosting ve CLI/mobile client aynı anda farklı sürümlerde olduğunda geriye dönük uyumluluk maliyetini artırır.

**Öneri:** Hemen breaking change yapmak yerine yeni API surface için versioning policy belirle; mevcut endpoint'leri geriye dönük bırak.

---

# 3. Şu an BUG DEĞİL / yanlış pozitif olarak elenenler

### Mascot stale polling — **FIXED**

`frontend/lib/widgets/mascot_window.dart` şu anda request generation kontrolü kullanıyor. Eski HTTP response yeni state'i ezemiyor ve `dispose()` generation'ı invalidate ediyor. Bu nedenle önceki stale-polling bulgusu tekrar açık bug olarak yazılmadı.

### Live Mode speaking throttle race — **FIXED implementation**

`internal/app/activity.go` şu anda Load→Store yerine atomic CAS kullanıyor. Birden fazla Live Mode session aynı anda audio event gönderse de throttle penceresi yarışmadan korunuyor.

**Not:** Bu taramada bu helper için ayrı concurrency regression testinin mevcut olduğunu doğrulayamadım; dolayısıyla "test eklendi/yeşil" iddiası yazılmıyor.

### Session ID collision — **FIXED**

Eski 8-hex session ID problemi full UUID'ye çevrilmiş durumda. Bu nedenle aktif bug değildir.

### `skill.DangerLevel` / `agent.DangerLevel` — **design duplication, compile failure değil**

İki package ayrı named type tanımlıyor ancak mevcut agent kodunda dönüşüm/uyumluluk yolu bulunuyor. Bu nedenle "compile-time bug" olarak işaretlenmedi; type duplication olarak teknik borç kabul edildi.

### Unsupported RAM detection — **limitation, crash bug değil**

`sysram_other.go`'daki `0` dönüşü bilinen platform limitation'ı. Ancak downstream recommendation kodunun bunu güvenli yorumlaması ayrıca test edilmeli.

---

# 4. Özellik bazlı öncelik

| Özellik | Öncelik | Neden |
|---|---|---|
| Taskloop | 🔴 P1 | Uzun görev + state + parallelism + recovery |
| Provider lifecycle / Router | 🔴 P1 | Streaming sırasında client değişimi |
| Live/WhatsApp/Telegram | 🔴 P1 | Long-lived connection lifecycle |
| Orchestra | 🟠 P2 | Fallback + concurrency + test boşluğu |
| Self-host | 🟠 P2 | Çoklu deployment matrisi |
| Memory background extraction | 🟠 P2 | Local inference contention |
| Live echo | 🟠 P2 | Gerçek kullanım UX'i |
| L10n error seam | 🟡 P3 | Dil/UX tutarlılığı |
| API versioning | 🟡 P3 | Gelecek compatibility |
| RAM unknown state | 🟡 P3 | Edge platform correctness |

---

# 5. Benim önerdiğim sıradaki geliştirme sırası

Yeni feature **eklemeyelim**. Şu sırayla ilerlemek daha mantıklı:

### 1. Provider/client lifecycle hardening
`client` + `providerRouter` snapshot/generation modelini temizle ve race regression testlerini yaz.

### 2. Taskloop soak/recovery
Crash/restart/resume + timeout + cancellation + retry + parallel-step testleri.

### 3. Orchestra reliability
Router/fallback entegrasyonu ve concurrency testleri.

### 4. Long-lived connection matrix
Live Mode + WhatsApp + Telegram için reconnect/restart/auth-expiry testleri.

### 5. Self-host release matrix
RPi + Linux + Docker/CasaOS + remote auth üzerinde gerçek smoke test.

### 6. Memory latency/reliability
Background extraction'ın local model slotunu bloke etmesini azalt; pinned-facts consolidation/eviction davranışını sertleştir.

Bu altı başlık bitmeden yeni büyük feature açmak yerine mevcut özellikleri production-grade seviyeye taşımak daha yüksek değer üretir.

---

# 6. Verification durumu

Bu rapor **statik repo auditidir**. Bu taramada yerel ortamda şu zorunlu komutların çalıştırıldığı iddia edilmiyor:

```text
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race
cd frontend && flutter analyze lib/ && flutter test
```

GitHub tarafında mevcut audit commit'i için yayınlanmış status kaydı bulunmadı; bu nedenle "CI green" de iddia edilmiyor.

**Son hüküm:** Memo'nun temel feature seti artık yeterince geniş. Şu an en yüksek ROI yeni bir özellik değil; **reliability pass**. Özellikle Taskloop + provider lifecycle + long-lived connections üçlüsünü sertleştirmek, Memo'yu %89 maturity'den production'a daha hızlı taşır.
