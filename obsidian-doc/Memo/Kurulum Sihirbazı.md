# 🧭 Kurulum Sihirbazı

> **Kod:** `frontend/lib/widgets/setup_wizard_view.dart`, `frontend/lib/widgets/setup/` (kurallar `setup_progress.dart` içinde)
> Testler: `setup_progress_test.dart`, `setup_steps_test.dart`, `setup_wizard_test.dart`
> Tam sayfa: `docs/SETUP_WIZARD.md`

Eskiden Ayarlar'a gitmeyi gerektiren her şey için rehberli bir akış. `*` ile işaretli adımlar zorunludur; zorunlu bir adım bitmedikçe "İleri" kapalı kalır; yan liste kayar ve her satır o adımı açar; dar pencereler tek sütun kullanır.

## Adımlar
1. Dil ve görünüm
2. **Model bağlantısı `*`** — yerel model (Model Mağazası bir pencerede açılır), API anahtarı ya da abonelik
3. Gömme modeli (önerilir; atlamak hafızanın çalışmayacağını açıkça söyler)
4. Persona
5. Mail (isteğe bağlı) — Mail ekranıyla aynı diyaloglar
6. **Mail yanıt modeli `*`** — yalnız bir mail hesabı varsa; ayrıca güvenli bir "yanıt taslakları hazırla" anahtarı ve saat dilimin
7. Karar modeli
8. Hafıza ve öğrenme
9. Ses ve görseller
10. Kanallar — WhatsApp ve Telegram eşleştirme
11. **Günlük brifing** — bkz. [[Günlük Brifing]]
12. Gizlilik ve yedekleme — kullanım paylaşımı (bkz. [[Telemetri ve Gizlilik]]), bildirim izni, şifreli bulut yedeği
13. Özet ve kontrol — her adım Bitti / Atlandı / Eksik olarak, ayrıca bir sistem kontrolü

## Nasıl karar verir
Kurallar, gerçek uygulama durumunun bir anlık görüntüsünün saf fonksiyonlarıdır (etkin sağlayıcı, indirilen sohbet modelleri, gömme modeli, mail hesapları, ikiz yanıt modeli, brifingin açık olup olmadığı). Ayrı bir ilerleme kaydı yoktur; bu yüzden sihirbaz kaldığı yerden sürer ve başka yerde bir şey değiştirilince doğru kalır.

## Yeniden çalıştırma
Ayarlar → Genel yeniden başlatır. Yeniden çalıştırma kullanılan temadan başlar ve persona istemini yalnız orada ona dokunduysan yazar.

İlgili: [[Dijital İkiz ve Mail]], [[Geliştirici Kurulum Rehberi]].
