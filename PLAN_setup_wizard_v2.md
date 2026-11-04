# PLAN — Kapsamlı kurulum sihirbazı (v2)

**İstek (kullanıcı, özet):** Her şey kurulum sırasında yapılsın, sonradan Ayarlar'a girmek gerekmesin.
Zorunlu adımlar `*` ile işaretli olsun. Model bağlama zorunlu (yerel / API / abonelik fark etmez; **yerel
seçilirse tek öneri yerine Model Mağazası pencere olarak açılsın**). Gömme modeli atlanabilir ama önerilen.
Mail bağlama isteğe bağlı; **mail bağlanırsa mail (yanıt) modeli seçmek zorunlu**. Ayarlar'daki çoğu şey
(karar modeli, hafıza, ses, görsel model, kanallar, izinler …) kurulumda yapılabilsin.

Önceki plan: `PLAN_setup_redesign.md` (karar modeli açıklaması, abonelik girişi). Bu plan onun devamıdır.

## Mevcut durum (okundu, 5 Kasım)
- `frontend/lib/widgets/setup_wizard_view.dart`: 1834 satır, tek parça, 5 adım (dil/tema, persona, model
  bağlantısı, tercihler, sistem kontrolü). Bitirme: `setupCompleteProvider.completeSetup()`.
- Yeniden kullanılabilir: `ProviderConfigDialog`, `SubscriptionsTab`, `ModelStoreScreen`, `DecisionModelSection`,
  `ImageModelSection`, `STTProviderSection`, `TTSProviderSection`, `TTSVoiceSection`, `PersonaPicker`.
- Mail bağlama `screens/mail_screen.dart` içinde **özel** sınıflar (`_MailOAuthDialog`, `_MailCustomConnectDialog`):
  ortak dosyaya çıkarılmalı. Yanıt modeli seçimi `autonomy_panel.dart`/`twin_dialog.dart` içinde.

## Tasarım kararları
1. **Durum her zaman gerçek veriden hesaplanır** (aktif sağlayıcı, yerel model listesi, gömme modeli, mail
   hesapları, ikiz yanıt modeli). Ayrı bir "ilerleme" kaydı yok: uygulama kapanıp açılsa sihirbaz kaldığı yerden
   devam eder, ayarlar başka yerden değişse de doğru gösterir.
2. **Adım çerçevesi**: her adım `SetupStepDef` (id, zorunlu mu, tamamlandı mı) + küçük bir widget dosyası
   (`frontend/lib/widgets/setup/`). Zorunluluk/tamamlanma mantığı saf Dart'tır ve ayrıca birim testlidir.
3. **Zorunluluk kuralları**: Model bağlantısı `*` her zaman. Mail yanıt modeli `*` **yalnız mail hesabı varsa**.
   Sihirbaz yalnız zorunlular tamamsa bitirilebilir; eksik varsa özet o adıma götürür.
4. **Her şey atlanabilir (zorunlular hariç) ve sonradan Ayarlar'dan değişir.** Sihirbaz Ayarlar'dan
   yeniden açılabilir ve mevcut seçimlerle dolu gelir.
5. TR + EN birlikte (AGENTS.md kural 8), dar/geniş pencere, koyu/açık tema, klavye ile gezinme.

## Adımlar (* zorunlu)
| # | Adım | Davranış |
|---|---|---|
| 1 | Dil ve tema | Varsayılan seçili. |
| 2 | **Model bağlantısı \*** | Yerel / API anahtarı / Abonelik. **Yerel** → Model Mağazası penceresi (indirilenler + keşfet, önerilen rozetli). Tamam: aktif sağlayıcı var **veya** indirilmiş sohbet modeli seçili. |
| 3 | Gömme modeli (önerilen) | Tek tıkla önerileni indir, mağazadan seç ya da atla. Atlanırsa "hafıza kapalı kalır" uyarısı. |
| 4 | Kişilik | İsim + persona. |
| 5 | Mail (isteğe bağlı) | Gmail / Outlook / Diğer bağla (ortak bağlama pencereleri). |
| 6 | **Mail yanıt modeli \*** (mail varsa) | Yanıt modeli seçimi + güvenli varsayılanlar (hepsi taslak, otonomi kapalı). |
| 7 | Karar / doğrulama modeli | `DecisionModelSection`; mail bağlıysa güvenlik incelemesi için önerilir. |
| 8 | Hafıza ve öğrenme | Hafıza, rüya (dream), proaktif öneriler, ruh hali, minimal mod. |
| 9 | Ses ve görsel | STT, TTS (+ ses), görsel model. |
| 10 | Kanallar | WhatsApp, Telegram eşleştirme (atlanabilir). |
| 11 | İzinler ve gizlilik | Ajan izinleri, bildirim izni, telemetri, gizli mod, yedekleme. |
| 12 | Özet ve bitir | Her adımın durumu (tamam / atlandı / eksik*), eksik zorunluya git, **Kurulumu bitir**. |

## İş birimleri (sırayla, her biri testli ve ayrı commit)
- [x] **U1 — Çerçeve ve mantık:** `widgets/setup/setup_progress.dart` (saf zorunlu/tamamlandı kuralları, 11 birim
      testi) + mevcut kabuğa bağlandı: zorunlu adım `*` ile işaretli, tamamlanmadan "Sonraki" kapalı, eksik
      zorunlu varken "Başla" kapalı ve özet eksik adımı söyler (2 widget testi). **Sayfa sırası bilerek değişmedi**
      (dil, kişilik, model, tercihler, kontrol): mevcut testler buna bağlı; sıra ve adım dosyalarına bölme U2'de
      model adımı yeniden yazılırken yapılır.
- [ ] **U2 — Model ve gömme adımları:** Yerel → Model Mağazası penceresi, API, abonelik; gömme modeli adımı.
- [ ] **U3 — Mail ve yanıt modeli:** mail bağlama pencerelerini ortak dosyaya çıkar; mail adımı; yanıt modeli
      (mail varsa zorunlu) + güvenli varsayılanlar.
- [ ] **U4 — Model/ayar adımları:** karar modeli, hafıza ve öğrenme, ses ve görsel.
- [ ] **U5 — Kanallar, izinler, özet:** WhatsApp/Telegram, izinler ve gizlilik, özet ekranı, Ayarlar'dan yeniden aç.
- [ ] **U6 — Cilalama:** dar/geniş düzen, TR/EN, erişilebilirlik, gerçek uygulamada kabul.

## Doğrulama (her birim)
`flutter analyze lib/` (yalnız bilinen 3 info), `flutter test`, L10n literal taraması (AGENTS.md kural 8),
Go tarafı değişirse `CGO_ENABLED=1 go build/vet/test -tags "sqlite_fts5" ./... -race`. Gerçek uygulamada
temiz veri klasörüyle (`MEMO_DATA_DIR`) sihirbaz baştan sona denenir; kullanıcının gerçek verisi kullanılmaz.
