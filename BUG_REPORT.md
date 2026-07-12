# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-12 (Session 21, devam — AGENTS.md'deki açık teknik borç maddeleri "Teknik Borç" bölümü olarak buraya taşındı; BUG-H3 düzeltildi, chat-ID refactor Faz 3 tamamlanıp bu listeden kapatıldı (`docs/plans/PLAN_chatid_refactor.md`), eski TD-2 (DangerLevel) yeniden doğrulanıp TD-1 olarak güncellendi, TD-2 (API versioning) düzeltildi, BUG-M2 kapatıldı, BUG-L5 düzeltildi (canlı ngrok testi hariç). Kalan tek gerçek "bug": BUG-M1 (kullanıcı isteğiyle bilinçli atlandı). TD-1 bilinçli olarak koda dokunulmadan bırakıldı.)
>
> **BUG-M2 kapatma notu:** `connectionStatusProvider`'ın `app_shell.dart` tarafından sürekli izlenip 30s'de bir `autoDispose` tetiklenmemesi bug değil, kasıtlı tasarım — bu poll, backend'in client-registry'sine ("Backend process model", AGENTS.md) GUI'nin hâlâ açık olduğunu bildiren heartbeat'in ta kendisi; durdurulursa backend GUI'yi kaybolmuş sanabilir. Kod değişikliği yapılmadı, madde kapatıldı.
> **Not:** Bu dosya daha önce 1300+ satırlık, onlarca oturumun anlatısını ve 100 düzeltilmiş bug'ı içeren tarihsel bir arşivdi. Bu haliyle kullanılamaz hale gelmişti (görünüşte "27 açık bug" diyordu, gerçekte bunların çoğu zaten düzeltilmişti ama tablo hiç güncellenmemişti). Temizlendi — sadece hâlâ gerçekten açık olan maddeler kaldı.
>
> **İkinci geçiş (aynı gün):** Kalan eski maddeler tek tek koda karşı yeniden doğrulandı. Sonuç: "Mobile API client eksik" iddiası artık geçersizdi (118 backend endpoint'inin 111'i zaten destekleniyor, eksik 7'si de mobile'a hiç uygun değil — kaldırıldı). `AGENTS.md`'nin "Known Pitfalls" bölümü de tarandı: iki madde ("data race" olarak işaretlenen `a.client`/`providerRouter` reassignment'ları) meğerse zaten kilitli imiş — gerçek risk daha dar (BUG-L4), "memory full rebuild O(N)" notu ise referans verdiği `LoadCache` fonksiyonu artık kodda hiç yok, tamamen bayat — hiç eklenmedi.
>
> **Session 20:** İki kritik madde (BUG-C1, BUG-C2) düzeltildi — bkz. commit `de4450e`, `f5a579e`. Ardından sırayla eski BUG-H1/H2/H3 (auth yan etkisi, SQLite izinleri, dead-code sandbox, panic recovery) ve eski MEDIUM listesindeki 7 madde (websearch-memory race, Minimal Mod dual-source, consolidation sessiz hata, izin diyaloğu sessiz kapanma, toggle çift-tık race'i, detached backend zombi süreci, SIGTERM'de unregister eksikliği — son ikisi `4f364f4`/`14e545f`) düzeltildi. Kalan 3 HIGH maddesi (chat-switch race — mimari refactor gerektiriyor; Windows auto-shutdown — bu ortamda test edilemez) ve M1/M2 (dosya boyutu notu, kabul edilmiş polling) bilinçli olarak atlandı. Tam detay için `git log`.

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 0 |
| 🟡 MEDIUM | 1 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 1 |
| **TOPLAM** | **2** |

---

## 🟡 MEDIUM

### BUG-M1: `model_store_screen.dart` — 2600+ satır tek dosya

- **Dosya:** `frontend/lib/screens/model_store_screen.dart` (doğrulandı: 2612 satır)
- **Kullanıcı etkisi:** Doğrudan bug değil ama bakım yapılamaz hale geliyor, değişikliklerde kırılma riski yüksek.

---

## 🔧 Teknik Borç

> AGENTS.md'nin "Known Pitfalls & Technical Debt" / "Known Open Work" bölümlerinden buraya taşındı (2026-07-12) — bug değil ama açık mimari/bakım borcu, aynı formatta takip ediliyor.

### TD-1: Skill sisteminin kendi `Tools` tanımları hiçbir zaman gerçek agent tool'una dönüşmüyor (öldürülmüş köprü)

- **Dosya:** `internal/skill/manager.go` (`ToolRegistrar`, `SetToolRegistrar`, `RegisterTool`/`UnregisterTool` çağrıları), `internal/agent/tools.go` (`FromString`), `internal/app/app.go` (skill manager kurulumu)
- **2026-07-12 yeniden doğrulandı — orijinal madde ("iki paket ayrı `DangerLevel` tipi tanımlıyor, derleme zamanında uyuşmuyor") YANLIŞ çıktı:** `skill.ToolRegistrar.RegisterTool(name string, toolDef any)` zaten `any` alıyor, bugünkü kodda hiçbir yerde gerçek bir compile-time tip hatası yok.
- **Gerçek bulgu:** `skill.Manager.toolRegistrar` alanını dolduran `SetToolRegistrar()` **prod kodunda hiçbir yerde çağrılmıyor** (`app.go`'da skill manager kuruluyor ama registrar hiç set edilmiyor) — yani `toolRegistrar` her zaman `nil`, `SetActive`/`Remove` içindeki `RegisterTool`/`UnregisterTool` çağrıları sessizce no-op. `agent.FromString` (bu köprü için yazılmış skill→agent `DangerLevel` dönüştürücüsü) **0 çağırana sahip**, hiç kullanılmıyor. Ayrıca `skill.SkillTool` struct'ında bir `ExecuteFn` alanı da yok — bir skill'in manifest'inde tanımladığı `Tools` çağrıldığında ne çalıştırılacağı hiç tanımlı değil.
- **Etki:** Bir skill'in YAML manifest'inde `tools:` altında tanımladığı hiçbir şey gerçekte agent'a araç olarak eklenmiyor — tamamen deklaratif/kullanılmayan veri. Bug değil (crash/veri kaybı yok) ama tasarım eksikliği: bu bir basit tip-fix değil, "skill tool'ları nasıl çalıştırılacak" sorusuna cevap gerektiren, kapsamı belirsiz bir feature kararı. Kullanıcı isteğiyle şimdilik koda dokunulmadı — sadece bu madde gerçek bulguya göre güncellendi.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
