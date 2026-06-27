# Bug Report — Memo v3.1.0-beta Stabilite Engelleri

> **Amaç:** Stable sürüme engel olan, kullanıcıyı direkt etkileyen bug'ların tespiti.
> **Tarih:** 2026-06-28
> **Kapsam:** Tüm Go backend + Flutter frontend + CI/CD

---

## CRITICAL — Kullanıcı Crash / Data Loss

### 1. `_AgentStatusBar.events.last` → Empty List Crash

- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:822`
- **Kod:** `final lastEvent = events.last;`
- **Risk:** Eğer `events` listesi boşsa, `StateError: Bad state: No element` ile hard crash.
- **Tetikleyici:** Widget sadece `widget.agentEvents!.isNotEmpty` kontrolünden sonra instantiate ediliyor (line 498), ama ayrı bir `StatelessWidget` olduğu için future refactoring'de kolayca atlanabilir. Listeye state değişikliği anında boş gelirse crash.
- **Fix:** `events.lastOrNull` kullan veya `if (events.isEmpty) return SizedBox.shrink()` guard'ı ekle.

### 2. `qrCodes.first` → Empty List Crash

- **Dosya:** `frontend/lib/screens/whatsapp_screen.dart:275`
- **Kod:** `data: status.qrCodes.first,`
- **Risk:** Backend'den gelen `qrCodes` listesi boşsa `StateError` ile crash. QR kodu gösterilirken backend hata dönerse veya liste gelmezse direkt patlar.
- **Tetikleyici:** Backend'in QR kod listesi boş döndüğü her an.
- **Fix:** `status.qrCodes.isNotEmpty ? status.qrCodes.first : fallbackWidget`

### 3. `as List` / `as Map` / `as String` Cast Crash (6 locations)

- **Dosyalar:**
  - `frontend/lib/widgets/chat_input.dart:494` — `(result['models'] as List)` null ise crash
  - `frontend/lib/widgets/provider_config_dialog.dart:134` — aynı pattern
  - `frontend/lib/widgets/orchestra_config_dialog.dart:539` — aynı pattern
  - `frontend/lib/widgets/orchestra_config_dialog.dart:550` — `selected['id'] as String` key yoksa crash
  - `frontend/lib/screens/model_store_screen.dart:1581-1582` — `as String` / `as int` field eksikse crash
  - `frontend/lib/widgets/chat_input.dart:204` — `json.decode(chunk.content) as Map<String, dynamic>` decode edilmiş JSON Map değilse crash
  - `frontend/lib/providers/chat_provider.dart:426,433` — aynı pattern
- **Risk:** Backend API response formatı değişirse veya hata dönerse, `as` cast runtime'da `TypeError` fırlatır. `catch` blokları `FormatException` için yazılmış, `TypeError`'ı yakalamaz (chat_input.dart:220'deki `catch (_)` generic olduğu için yakalar ama sessizce yutar — UI yanlış state'te kalır).
- **Fix:** `is` kontrolü ekle veya null-safe pattern matching: `(result['models'] as List?)?.cast<_>() ?? []`

### 4. `as RenderBox` Cast Crash

- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:181`
- **Kod:** `final renderBox = context.findRenderObject() as RenderBox;`
- **Risk:** `findRenderObject()` dispose anında `null` dönebilir. `as RenderBox` null üzerinde çağrılırsa crash.
- **Fix:** `if (renderBox is RenderBox)` guard'ı ekle.

### 5. Google Gemini API Key URL'de Sızıyor

- **Dosya:** `internal/provider/gemini.go:59,139,201`
- **Kod:** `fmt.Sprintf("%s/models?key=%s", p.baseURL, url.QueryEscape(p.apiKey))`
- **Risk:** API key URL query parameter'ında taşınıyor. HTTP proxy log'larında, server access log'larında, `Referer` header'ında sızabilir. Google Gemini API aslında `x-goog-api-key` header'ını da destekler.
- **Fix:** URL yerine `x-goog-api-key` header'ı kullan.
- **Kritiklik:** API key sızıntısı → başkası kullanıcının hesabını kullanabilir.

### 6. Fire-and-Forget Goroutine Leaks (4 adet)

- **`internal/observer/recorder.go:39`** — Her mesaj, agent run, intent, WhatsApp mesajı için `go func() { r.store.Record(obs) }()` çağrılıyor. Hiçbir timeout, cancel, wg yok. DB contention'da goroutine'ler birikir.
- **`internal/app/chat.go:60,124`** — `go a.processMessageIntent(...)` her chat mesajı için. Timeout veya iptal mekanizması yok. `a.lifecycleCtx` içeride kullanılıyor ama goroutine'in kendisinin sınırı yok.
- **`internal/app/whatsapp.go:98`** — Aynı pattern, WhatsApp mesajları için. Volume daha yüksek.
- **`internal/app/app.go:240`** — `go func() { memory.NewStore(...) }()` startup'ta. Timeout yok. `NewStore` takılırsa goroutine sızar.

- **Risk:** Uzun süreli kullanımda goroutine havuzu büyür, memory tüketimi artar, eventual crash.

### 7. `model_store_screen.dart` — 2507 Satır, Tek Dosyada

- **Dosya:** `frontend/lib/screens/model_store_screen.dart`
- **Risk:** Tek dosyada 2500+ satır. Herhangi bir değişiklikte kırılma riski yüksek. Dart'ın analyze'ı yavaşlar. Bakımı zor.
- **AGENTS.md'de de belirtilmiş:** "should be split into components"
- **Ayrıca:** Line 1577-1584'te `_moreModels![i]`'nin `onTap` closure'ına kaptırdığı index — liste kısalırsa out-of-bounds.

---

## HIGH — Kullanıcıyı Etkileyen / Feature Broken

### 8. Mobile API Client Eksik (50+ endpoint)

- **Dosya:** `mobile/lib/core/api_client.dart` (1813 satır)
- **Risk:** Backend'deki endpoint'lerin çoğu mobil client'ta yok. Mobil uygulama çalışmaz durumda.
- **Fix:** `frontend/lib/core/api_client.dart` referans alınarak tüm endpoint'ler eklenmeli.

### 9. Provider Priority UI Kontrolü Yok

- **Dosyalar:** `frontend/lib/widgets/provider_config_dialog.dart:247`, `internal/provider/router.go:64-66`
- **Kod:** Router `sort.SliceStable` ile Priority'ye göre sıralıyor ama Flutter UI'da priority ayarı yok (her zaman `0`). Kullanıcı hangi provider'ın öncelikli kullanılacağını belirleyemez.
- **Etki:** Provider sırası config dosyasındaki sıraya göre belirlenir (gizli/kontrol edilemez). Kullanıcı "önce OpenAI, düşerse Ollama" gibi bir fallback zinciri kuramaz.

### 10. Orchestra Provider Fallback Kullanmıyor

- **Dosyalar:** `internal/orchestra/conductor.go:186-237`, `internal/provider/router.go:100-172`
- **Kod:** Orchestra, `provider.Router.ChatCompletion()` yerine direkt `provider.ChatCompletion()` çağırıyor. Router'ın priority-based fallback chain'ini, auto-disable'ını, retry mekanizmasını bypass ediyor. Kendi retry'ini yazmış (3 deneme, exponential backoff) ama aynı provider'ı tekrar dener, farklı provider'a geçmez.
- **Etki:** Chief provider düştüğünde orkestra workflow'u tümden başarısız olur. Router'ın diğer provider'a fallback yapma özelliği kullanılmaz.

### 11. nil Client Dereference Riski (3 yerde)

- **`internal/app/llm.go:53`** — `providerCfgMgr.GetEnabled()` nil `providerCfgMgr` üzerinde çağrılabilir. `resolveAgentProvider()`'da bazı code path'ler `providerCfgMgr` nil olabilir.
- **`internal/app/settings.go:123`** — `checkClient.CheckConnection(tctx)` — `a.client` read lock altında okunuyor ama nil check yok. Startup tamamlanmadan çağrılırsa panic.
- **`internal/app/llm.go:693,898`** — `streamClient.ChatCompletionStream(...)` / `llmClient.ChatCompletion(...)` — local model path'inde `a.client` nil check yok.
- **`internal/memory/embedder.go:12`** — Closure içinde `client.CreateEmbedding(ctx, model, text)` — client nil geçilebilir.

### 12. HTTP Client Timeout Yok (Download Hangs Forever)

- **`internal/modelstore/modelstore.go:335`** — `dlClient := &http.Client{}` — timeout: 0, hiçbir zaman timeout olmaz.
- **`internal/app/helpers.go:287`** — `dlClient := &http.Client{Timeout: 0}` — aynı.
- **Etki:** Model indirme veya binary download'ı network hatasında sonsuza kadar asılı kalır. Kullanıcı cancel dışında çıkamaz.

### 13. Structured Logging Migration Tamamlanmamış

- **Dosyalar:** `internal/cloudsync/drive.go`, `internal/cloudsync/sync_manager.go`, `internal/provider/router.go`, `internal/provider/config.go`, `internal/memory/store.go`, `internal/app/app.go` (partial)
- **Kod:** `log.Printf()` kullanılmaya devam ediyor. Sadece `internal/webserver/server.go` `logx`'e geçmiş.
- **Etki:** Hata ayıklama ve monitoring zor. Log seviyesi kontrolü yok. Production'da gereksiz bilgi sızıntısı.

### 14. `bash -c` ile Command Injection Riski

- **Dosya:** `internal/agent/tools/command.go:164`
- **Kod:** `cmd = exec.CommandContext(execCtx, "bash", "-c", args.Command)`
- **Risk:** Kullanıcının agent'a yazdırdığı komutlar `bash -c` ile çalıştırılıyor. Blacklist (`isBlacklisted`) shell substitution karakterlerini (`$`, `` ` ``) engelliyor ama regex tabanlı yaklaşım fragile:
  - `cmdLower` regex match'i yapılıyor ama shell substitution check'i `cmd` (orijinal) üzerinde
  - Regex'ler atlatılabilir (ör: encoding tricks)
  - Blacklist approach'u asla tam güvenli değildir
- **Etki:** Agent tool permission onayı geçerse, kullanıcı keyfi komut çalıştırabilir. Bu tasarım gereği (agent tool), ama blacklist'in aşılması durumunda koruma kalmaz.

### 15. `connectionStatusProvider` Sonsuz Polling

- **Dosya:** `frontend/lib/providers/chat_provider.dart:677-690`
- **Kod:** Backend reachable değilken bile 30 saniyede bir `isAlive()` sorguluyor. Provider dispose olsa bile mevcut sleep tamamlanana kadar çalışır.
- **Etki:** Gereksiz ağ trafiği + CPU. Kullanıcı görünür bir hata görmez ama arka planda sürekli polling döner.

---

## MEDIUM — Kullanıcı Kalite Deneyimini Etkileyen

### 16. Flutter Async Gap — `mounted` Check Eksik

- **Dosya:** `frontend/lib/screens/agent_screen.dart:205-214`
- **Kod:** `onPressed` async callback'inde 3 `await` var. Hiçbirinin ardından `mounted` kontrolü yok. Widget dispose olmuşsa UI güncellemesi visual glitch'e yol açar.

### 17. `const` Constructor Eksiklikleri

- **Tüm Flutter projesinde yaygın.**
- **Etki:** Her rebuild'de yeni widget instance'ları oluşur. Flutter'ın rebuild optimizasyonu kırılır. Performans düşüşü.

### 18. `_ = err` Silent Error Ignore (4+ yerde)

- **Dosyalar:**
  - `internal/memory/store.go:936` — `_ = s.db.Write(...)` — write hatası sessizce yutuluyor
  - `internal/memory/store.go:1416,1419` — `DELETE` hataları yutuluyor
  - `internal/agent/tools/selfclone.go:82` — `_ = copyFileTo(...)` — kopyalama hatası sessiz
  - `internal/whatsapp/client.go:338` — `_ = os.WriteFile(nonePath, nil, 0644)` — yazma hatası sessiz

### 19. Skill Install Dialog Windows Path

- **AGENTS.md'de belirtilmiş:** Windows'ta Unix path hint (`/home/...`) gösteriliyor.
- **Fix:** `Platform.isWindows` kontrolü ile path example'ı değiştirilmeli.

### 20. `skill.DangerLevel` vs `agent.DangerLevel` Tip Uyuşmazlığı

- **Dosyalar:** `internal/skill/types.go:7-13`, `internal/agent/tools.go:15-36`
- **Kod:** İki ayrı pakette `type DangerLevel string` tanımlı, değerleri farklı string sabitler. `skill.ToolRegistrar` interface'i `any` kullandığı için tip güvenliği yok.
- **Etki:** Skill sistemi agent'a bağlanmak istendiğinde (şu anda bağlı değil) tip dönüşümü gerekecek. Unimplemented feature.

### 21. Whisper GPU Variant Desteği Eksik

- **AGENTS.md'de belirtilmiş:** `whisper-server` binary'si `linux/{amd,nvidia}/` dizinlerinde yok. Metal variant macOS için mevcut değil.
- **Etki:** GPU'su olan kullanıcılar CPU mode'da STT kullanır.

---

## CI/CD & Infra

### 22. Flutter Linux Build Kırık

- **Flutter Linux build:** `sudo apt install default-jdk` gerekli. JDK eksikliğinden `record` → `jni` paketi derlenemiyor.

### 23. macOS Flutter Platform Projesi Eksik

- `frontend/macos/` dizini yok. `flutter create --platforms=macos` gerekiyor.

---

## Puan Tablosu

| # | Kategori | Risk | Fix Süresi |
|---|----------|------|------------|
| 1 | Flutter crash | CRITICAL | 5 dk |
| 2 | Flutter crash | CRITICAL | 5 dk |
| 3 | Flutter crash (6x) | CRITICAL | 15 dk |
| 4 | Flutter crash | CRITICAL | 5 dk |
| 5 | Security leak | CRITICAL | 15 dk |
| 6 | Goroutine leak (4x) | CRITICAL | 30 dk |
| 7 | Bakım | CRITICAL | 2 saat |
| 8 | Mobile broken | HIGH | 4 saat |
| 9 | UX eksik | HIGH | 15 dk |
| 10 | Orkestra kırık | HIGH | 30 dk |
| 11 | nil panic (4x) | HIGH | 15 dk |
| 12 | Download hangs | HIGH | 5 dk |
| 13 | Logging | HIGH | 1 saat |
| 14 | Security | HIGH | 15 dk |
| 15 | Perf leak | HIGH | 10 dk |
| 16 | Visual glitch | MEDIUM | 5 dk |
| 17 | Perf | MEDIUM | 1 saat |
| 18 | Error handling | MEDIUM | 15 dk |
| 19 | UX | MEDIUM | 5 dk |
| 20 | Dormant bug | MEDIUM | 5 dk |
| 21 | Feature gap | MEDIUM | 2 saat |
| 22 | Build broken | MEDIUM | 1 dk |
| 23 | Build broken | LOW | 30 dk |

---

## Önerilen Aciliyet Sırası

1. Flutter crash fix'leri (#1-4) — 5 dakikalık işler, kullanıcıyı direkt çökertiyor
2. Gemini API key header'a taşı (#5) — 15 dakika, güvenlik açığı
3. nil client guard'ları (#11) — 15 dakika, latent panic
4. Goroutine leak fix (#6) — 30 dakika, uzun süreli stabilite
5. Provider priority UI (#9) + Orchestra fallback (#10) — 45 dakika, provider kullanıcı deneyimi
6. HTTP client timeout fix (#12) — 5 dakika, download asılması
7. Mobile API client (#8) — 4 saat, ayrı oturum
8. model_store_screen refactor (#7) — 2 saat, ayrı oturum
9. Logging migration (#13) — 1 saat, background task

---

## Test Coverage Durumu

```
go build ./...   ✅ (0 hata)
go vet ./...     ✅ (0 uyarı)
go test ./... -race -count=1  → 30/30 PASS
Flutter analyze  — geçiyor (CI)
```

328 test fonksiyonu var. Race detector temiz.
