# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-07-12 (Session 24 — BUG-M1 kapatıldı: `frontend/lib/screens/model_store_screen.dart` (2612 satır) `PLAN_modelstore_refactor.md`'deki 5 faza göre bölündü — `settings_dialog.dart`'ın daha önce aynı sorun için kullandığı desen (`settings/tabs/` → burada `screens/model_store/`). Sonuç: shell 180 satıra indi, `model_store/discover_item.dart` (194), `discover_tab.dart` (809), `model_detail_panel.dart` (956), `my_models_tab.dart` (515). Saf mekanik taşıma — sadece dosya sınırları arasında referans edilen 8 sembol private'tan public'e çevrildi (`DiscoverTab`, `ModelDetailPanel`, `MyModelsTab`, `DownloadBanner`, `DiscoverItem`, `humanizeName`, `timeAgo`, `fmtCount`), geri kalan her şey (grep + codebase-memory ile doğrulanmış şekilde tek bölüme özel olan ~25 widget/fonksiyon) private kaldı. `flutter analyze`/`flutter test` yeşil; ayrıca gerçek derlenmiş binary + gerçek backend ile uygulama açılıp ekran görüntüsüyle Discover sekmesinin (arama, filtre/sort chip'leri, model listesi, boş detay durumu) birebir doğru render edildiği doğrulandı — Model Detail Panel'e tıklayarak geçiş bu ortamda otomatize edilemedi (xdotool/wmctrl yok, XTest sentetik input'u native Wayland pencereye ulaşmadı), ama `flutter analyze`'ın temiz çıkması + `IndexedStack`'in tüm sekmeleri (My Models dahil) ekran açılışında zaten inşa etmesi (hata kutusu çıkmadı) dolaylı güçlü kanıt. TD-1 (Session 23) hâlâ kapalı. Artık 0 açık madde.)
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
| 🟡 MEDIUM | 0 |
| 🟢 LOW | 0 |
| 🔧 TEKNİK BORÇ | 0 |
| **TOPLAM** | **0** |

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin, "~~üstü çizili~~" olarak bırakılmasın.*
