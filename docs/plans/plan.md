# Memo — Kullanıcı Dostuluğu / Onboarding Planı — ✅ TAMAMLANDI (ARŞİV)

> **Durum (2026-07-06):** Bu plandaki tüm ana maddeler uygulanmış durumda.
> Dosya, önceki halinde metin bozulması (kendini tekrar eden karışık satırlar)
> olduğu için temiz haliyle yeniden yazıldı ve arşiv olarak işaretlendi.
>
> Amaç şuydu: Kullanıcı uygulamayı ilk açtığında "bu ne ya" demesin. Özellikleri
> **kapatmadan** ilk karşılaşmayı açıklamalı hale getirmek.
> Prensip: "vitrin tam açık ama her şey etiketli".

## Uygulanan maddeler

- [x] **1. Launchpad karşılama ekranı** — `frontend/lib/widgets/launchpad_view.dart`
  (özellik kartları; `launchpadSeenProvider` ile bir kez gösterim; ayarlardan
  sıfırlama: `general_tab.dart` → `settings_reset_launchpad`)
- [x] **2. İlk açılış mini-turu (spotlight)** — `frontend/lib/widgets/spotlight_tour.dart`,
  `app_shell.dart:422`'de bağlı; `tourSeenProvider` flag'i; ayarlardan tekrar
  gösterme: `general_tab.dart` → `settings_reset_tour`
- [x] **3. Boş ekran açıklamaları** — WhatsApp (`whatsapp_empty_title/desc` + QR akışı),
  Takvim (`calendar_empty_title/desc`), Agent (`agent_empty_title/desc/action`)
- [x] **4. İkon etiketleri** — `_NavRailButton` (app_shell.dart) ikon altında
  kalıcı metin etiketi gösteriyor; ayrıca hover tooltip'e gerek kalmadı
- [x] **5. Mod seçici açıklamaları** — modlar ayrı nav sekmelerine ayrıldığı için
  (Chat / Agent / WhatsApp ayrı ekranlar) bu madde kapsamsız kaldı; her ekranın
  kendi boş durumu kendini anlatıyor (madde 3)

## Kalan (opsiyonel, düşük öncelik)

- [ ] Setup wizard bitişi → launchpad → tur geçişinin gerçek cihazda uçtan uca
  bir kez gözle doğrulanması (fresh install senaryosu)

## Sıradaki işler

Aktif iş listesi için `AGENTS.md → Known Open Work` tablosuna bak:
- `PLAN_installer_launchvbs.md` — Windows kısayol bug'ı (küçük, net iş)
- `PLAN_chatid_refactor.md` — global aktif sohbet mimarisinin kaldırılması (büyük iş)
