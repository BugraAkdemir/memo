# PLAN — Proje klasörü: `.memo/`, proje başına hafıza, proje skill'leri

Amaç: Memo bir klasörü proje olarak açınca o klasörde `.memo/` (kurallar, yapılandırma, notlar, skill'ler) tutar; ajan sohbetinin hafızası klasörün sabit **id**'sine bağlıdır (klasör taşınsa da eşleşir); genel sohbet kişisel hafızası ajan sohbetine asla girmez.

## Kararlar (kullanıcı, 2026-10-19)
- Katı izolasyon: genel sohbet hafızası (sabitlenmiş gerçekler ve kullanıcı adı dahil) proje sohbetine HİÇ girmez. Genel (ajan olmayan) sohbet **tüm id'leri** okur; proje sohbeti **yalnız kendi id'sini** okur/yazar.
- `.memo/ignore` yok. `.memo/skills/` var; çağrılmadıkça isteme girmez; link / `npx`-biçimli komutla kurulur (Memo komutu çalıştırmaz, kendisi HTTPS ile indirir).
- Minimal Mod açık = saf sarmalayıcı: `.memo` talimatı yok, kurulum sorusu yok, hiçbir şey okunmaz. Hafıza anahtarı bağımsız.
- Oturum başında isteme tek satır: "önce `.memo/AGENTS.md`'yi oku"; dosyalar isteme gömülmez. Kök `AGENTS.md` araç-nötr kalır (`.memo`'dan söz etmez).
- Şablonlar teknolojiden bağımsız, İngilizce, kalite odaklı; `go:embed` ile ikilide.
- 3k aktif kullanıcı: veritabanı geçişi yalnız eklemeli, yedekli, başarısızsa özellik kapalı çalışır.
- İlk sürüm Beta arkasında (`internal/beta`), varsayılan kapalı.

## Aşamalar (her biri ayrı commit, doğrulanmadan sonrakine geçilmez)
- [x] **P0** `internal/project`: kök koruması, `.memo/project.json`, id + cihaz başına kayıt defteri (taşıma/kopya algılama), atomik yazma.
- [x] **P1** Hafıza kapsamı: `memories.project_id` (eklemeli geçiş + öncesinde yedek + başarısızsa kapalı), Store API'sinde `Scope`, birleştirme/tekilleştirme/dream sınırı, ctx ile `chatID → scope`, `Session.ProjectID`, proje sohbetinde kimlik bloğundan ad/öğrenilmiş notların çıkarılması, proje kapsamında olgu çıkarımı kapalı.
- [x] **P2** İskele + talimat: gömülü şablonlar, `/api/projects/*`, onay arayüzü (TR/EN l10n), tek satırlık talimat (Minimal Mod'da yok).
- [x] **P3** Proje skill'leri: `.memo/skills`, link ile kurulum (güven kapısı). **Yapılmadı:** `/` menüsünde/uygulamada skill seçici (şimdilik `/skill`, `/skill:on`, `/skill add` sohbet komutları).
- [ ] **P4** Proje başına izinler (hash güveni) — sonraki sürüm olabilir.
- [x] **P5** Belgeler: EN/TR docs, iki Obsidian kasası, AGENTS.md, handoff.

Ayrıntılar ve gerekçeler: oturum planı (`~/.claude/plans/memo-ignore-gerek-yok-buna-dapper-lemur.md`). Doğrulanmayacaklar (gerçek pencere, gerçek sağlayıcı, modelin kuralları izlemesi) handoff'a "doğrulanmadı" olarak yazılır.
