# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-12 (Session 23 — TD-1 (skill tool köprüsü) artık gerçek bir feature olarak inşa edildi, "basit köprü" değil: `skill.SkillTool`'a bir `command` alanı eklendi (skill'in kendi dizinine göre çözümlenen shell komutu; LLM'in çağrı argümanları stdin'den ham JSON olarak iletiliyor, komut string'ine hiç enjekte edilmiyor), `internal/skill/executor.go`'daki `Manager.ExecuteTool` bunu `internal/agent/tools`'un aynı sandbox'ını (destructive-pattern blacklist, 10MB çıktı sınırı, caller deadline'ı onurlandıran timeout) paylaşarak çalıştırıyor, `internal/app/skill_tools.go`'daki `skillToolRegistrar` artık `app.go`'nun `Startup()`'ında gerçekten `skillManager.SetToolRegistrar(...)` ile bağlanıyor — bir skill aktifleştirildiğinde `tools:` altında `command` tanımlı her girdi, agent pipeline'ın tool-call döngüsünün fiilen çağırabildiği gerçek bir `agent.ToolDef` olarak kayıtlı, permission/danger-level akışı dahil (mevcut izin diyaloğu otomatik olarak devreye giriyor, ek UI gerekmedi). `command` alanı olmayan girdiler (salt dokümantasyon amaçlı) sessizce kaydedilmiyor, hataya düşmüyor. Kalan tek gerçek "bug": BUG-M1 (kullanıcı isteğiyle bilinçli atlandı).)
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
| 🔧 TEKNİK BORÇ | 0 |
| **TOPLAM** | **1** |

---

## 🟡 MEDIUM

### BUG-M1: `model_store_screen.dart` — 2600+ satır tek dosya

- **Dosya:** `frontend/lib/screens/model_store_screen.dart` (doğrulandı: 2612 satır)
- **Kullanıcı etkisi:** Doğrudan bug değil ama bakım yapılamaz hale geliyor, değişikliklerde kırılma riski yüksek.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
