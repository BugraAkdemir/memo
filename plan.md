# Memo — Kullanıcı Dostuluğu / Onboarding Planı

> Amaç: Kullanıcı uygulamayı ilk açtığında "bu ne ya" demesin. Özellikleri
> **kapatmadan** (Agent, Orchestra, WhatsApp, Takvim hepsi açık kalır),
> ilk karşılaşmayı **açıklamalı** hale getirmek. Sorun özellik sayısı değil,
> sıfır açıklama. Prensip: "vitrin tam açık ama her şey etiketli".

## Temel ilke
- Hiçbir ana özellik gizlenmez/kapatılmaz.
- İlk kullanım = öğretici. Sonraki kullanımlar = sade.
- Mood ve Self-Interest zaten kapalı geliyor — onlara dokunma.

---

## 1. Launchpad karşılama ekranı (EN ÖNCELİKLİ)
Açılışta boş sohbet yerine, ne yapılabileceğini gösteren kartlar.
- [ ] Welcome/launchpad ekranı: özellik kartları
  - 💬 Sohbet — "Yaz, cevap al, seni hatırlasın"
  - 🤖 Agent — "Dosya düzenler, komut çalıştırır, görev yapar"
  - 🎵 Orchestra — "Birden çok modeli ekip gibi çalıştır"
  - 📱 WhatsApp — "Sohbetlerini AI ile yönet"
  - 📅 Takvim — "Planlarını otomatik yakalar"
- [ ] Her kart: tek cümle açıklama + "başla/aç" aksiyonu (ilgili sekmeye götürür)
- [ ] Kart sadece ilgili özellik kullanılabilir olduğunda aktif (örn. WhatsApp bağlı değilse "bağlan" der)
- [ ] Sohbet geçmişi varsa launchpad atlanır, direkt sohbete düşülür

## 2. İlk açılış mini-turu (coachmark / spotlight)
Setup wizard bittikten sonra, atlanabilir kısa tur.
- [ ] 4–5 adımlık spotlight: ekrandaki ana ikonları sırayla aydınlat + tek cümle balon
  - Agent ikonu → "Sana iş yaptırır"
  - WhatsApp ikonu → "Bağlan, AI yönetsin"
  - Orchestra → "Çok modelli ekip modu"
  - Takvim → "Planlarını yakalar"
- [ ] "Geç" butonu her adımda görünür
- [ ] Sadece ilk açılışta gösterilir (bir flag ile, tekrar gösterme)
- [ ] Ayarlardan "turu tekrar göster" seçeneği

## 3. Her sekmenin "ilk kez" boş ekranı kendini anlatsın
Kontrol yığını değil, açıklama + tek aksiyon.
- [ ] WhatsApp boş/bağlı değil → "WhatsApp sohbetlerini buradan yönet, bağlanmak için QR okut" + buton
- [ ] Agent ilk giriş → "Agent dosya/komut çalıştırabilir, onayı sen verirsin" + örnek
- [ ] Takvim boş → "Planların otomatik buraya düşer, manuel de ekleyebilirsin"
- [ ] Orchestra → kısa açıklama + "nasıl çalışır" (zaten yeni dialogda var, tutarlı yap)

## 4. İkon + etiket / tooltip (sadece ikon yetmez)
- [ ] Sol menü ikonlarının altına küçük etiket VEYA hover/tap tooltip
- [ ] Sohbet kutusundaki mod toggle'larına "?" / tooltip ipucu
- [ ] Hangi ikonun ne olduğu tek bakışta anlaşılsın

## 5. Mod seçerken açıklama
- [ ] Agent / Normal / WhatsApp mod seçicisinde her modun yanında tek satır açıklama
- [ ] Kullanıcı kör seçim yapmasın

---

## Öncelik sırası (en çok etki / en az iş)
1. Launchpad karşılama eulamayı ilk açtığında "bu ne ya" demesin. Özellikleri
> **kapatmadan** (Agent, Orchestra, WhatsApp, Takvim hepsi açık kalır),
> ilk karşılaşmayı **açıklamalı** hale getirmek. Sorun özellik sayısı değil,
> sıfır açıklama. Prensip: "vitrin tam açık ama her şey etiketli".

## Temel ilke
- Hiçbir ana özellik gizlenmez/kapatılmaz.
- İlk kullanım = öğretici. Sonraki kullanımlar = sade.
- Mood ve Self-Interest zaten kapalı geliyor — onlara dokunma.

---

## 1. Launchpad karşılama ekranı (EN ÖNCELİKLİ)
Açılışta boş sohbet yerine, ne yapılabileceğini gösteren kartlar.
- [ ] Welcome/launchpad ekranı: özellik kartları
  - 💬 Sohbet — "Yaz, cevap al, seni hatırlasın"
  - 🤖 Agent — "Dosya düzenler, komut çalıştırır, görev yapar"
  - 🎵 Orchestra — "Birden çok modeli ekip gibi çalıştır"
  - 📱 WhatsApp — "Sohbetlerini AI ile yönet"
  - 📅 Takvim — "Planlarını otomatik yakalar"
- [ ] Her kart: tek cümle açıklama + "başla/aç" aksiyonu (ilgili sekmeye götürür)
- [ ] Kart sadece ilgili özellik kullanılabilir olduğunda aktif (örn. WhatsApp bağlı değilse "bağlan" der)
- [ ] Sohbet geçmişi varsa launchpad atlanır, direkt sohbete düşülür

## 2. İlk açılış mikranı  ← "ne yapabilirim" sorusunu anında cevaplar
2. İlk açılış mini-turu         ← ikonların ne olduğunu öğretir
3. Boş ekran açıklamaları       ← her özellik kendini tanıtır
4. İkon etiketleri / tooltip
5. Mod seçici açıklamaları

## Dokunulan/incelenecek dosyalar (tahmini)
- `frontend/lib/screens/app_shell.dart` — menü, ikon etiketleri, ilk-açılış turu tetikleme
- `frontend/lib/widgets/setup_wizard_view.dart` — wizard sonu → launchpad/tur geçişi
- Welcome/launchpad için yeni widget (örn. `frontend/lib/widgets/launchpad_view.dart`)
- WhatsApp / Takvim / Agent ekranlarının boş durumları
- Tur durumu için kalıcı flag (settings/shared prefs)
