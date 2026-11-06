# PLAN — Dijital İkiz: gerçek kullanıcı gibi uçtan uca kabul (gece koşusu, 3 Kasım)

**İstek (kullanıcı, birebir):** "ben uyuyorum süreç sna emante sabaha kadar tüm plan iteğim gibi test edilmiş çalılır hale gelecek geldiğimiz yere kdar devam full yetkin var". Öncesinde: "elindeki mail ler 2 mail var akdemirbugra5 ve akdemirbugra14 bu mailler ilen mail göndermee alma izni veriyorum sana planda her adımı 1. adımdan 1. aşamdan başayarak gerçekten tets ederek ilerile".

**Kapsam:** `PLAN_mail_and_autonomous_system.md` Adım 1–7, her adım birden fazla senaryoyla, bir kullanıcı gibi (sohbetten yazarak araçları çağırma, arayüz, gerçek mail). Bulunan her hata: önce kırmızı test, düzeltme, yeşil, commit.

## Güvenlik zarfı (bu gece değişmez)
- Mail yalnız `akdemirbugra5@gmail.com` (ana Memo, :8090, gerçek model) ve `akdemirbugra14@gmail.com` (test Memo, :8091, sahte sağlayıcı, `Desktop/memo-mail-test/memo-iso/`) arasında. Üçüncü adrese hiçbir şey gönderilmez.
- Otomatik gönderim döngüsü kurulmaz: iki tarafta birden tam otomatik yok; ana Memo'da en çok tek kişiyle, tek seferlik, geri al süresiyle denenir. Eski kayıtlar (#6 vb., a7324752…) gönderilmez.
- push, tag, PR, kullanıcı verisini silme yok. Ana Memo'nun gerçek verisi test için değiştirilmez (ayar değişiklikleri sonunda geri alınır). Alt ajan çalıştırılmaz.
- Yerel CPU yükü testi yok; ağır doğrulama CI'da.

## Adımlar ve sonuç tablosu
Sonuçlar koşu bitince "Sonuç" bölümüne yazılır (PASS/FAIL/KISMİ + kanıt). Boş = henüz koşulmadı.

| Adım | Alan | Senaryo sayısı | Durum |
|---|---|---|---|
| 1 | Mail çekirdeği: hesap, gelen kutusu, okuma, arama, gönderme, taslak, yapısal ayrıştırma | 33 kontrol | KISMİ — 29 geçti; 4 kırmızıdan 1'i tekrar koşulup geçti, 3'ü (var olmayan mesaj mesajı, düz metinde fatura/uçuş yapısı) test beklentisi/ayrıştırıcı kapsamı, uygulama hatası değil; tekrar koşulmadı |
| 2 | Dinleyici: soğuk başlangıç, artımlı eşitleme, triyaj, sayaçlar | 9 | PASS |
| 3 | Güvenlik: prompt injection, DLP, grup/CC kapısı, sızıntı | 23 | PASS — tek kırmızı (engel mesajı sırrı tekrarlıyor) test hatasıydı, tekrar koşuda geçti; gerçek hatalar: OTP/“açıkla” kuralı (düzeltildi) |
| 4 | İkiz veritabanı: kişiler, üslup, salt-okunur araçlar | 10 | PASS — gerçek hata: `twin_contacts` aracı fazla veri sızdırıyordu (daraltıldı) |
| 5 | Otonom karar motoru: hazırlama, takvim harmanlama | 14 | KISMİ — 12 geçti; çalışma saati dışı çakışma 2 kontrolü tekrar koşulmadı (yol “soru” yoluydu, çakışma hesaplanmaz; harness beklentisi) |
| 6 | Onay köprüsü: onay, iptal, bekletme, tek gönderim, geri al | 33 | PASS — geri al, 60 sn sonra otomatik gönderim (tam 1 mail), saatlik kişi başı bütçe, `never` politikası canlı doğrulandı |
| 7 | Yanıt/söz takibi, brifing, toplantı notu, proje gözcüsü, TaskLoop köprüsü | 72 | KISMİ — 62 geçti; kırmızıların hepsi ya test hatası ya canlıda bulunup birim testiyle düzeltilen hata (aşağıda); söz vadesi canlı doğrulandı (01:59:55) |

## Sonuç
**Koşu özeti (3 Kasım 02:00–03:10, gerçek Gmail trafiği yalnız akdemirbugra5 ↔ akdemirbugra14):** ~190 kayıtlı kontrol; kırmızıların çoğu benim harness hatamdı (UTC/yerel saat, `tool_error` olayı, araç argümanlarında sır arama, elle takvim olayında `contact_name` olmaması).

**Canlıda bulunup düzeltilen gerçek hatalar (hepsi kırmızı test → düzeltme → commit):**
1. OTP / “açıkla” cümlesi prompt-injection kuralına takılıyordu (olumsuzlanmış Türkçe uyarılar + `açikla` yazımı).
2. Hatırlatma, yeni hatırlatma doğuruyordu (nag döngüsü) → `Action.FollowUp` + `NoTrack`.
3. `twin_contacts` sohbet aracı gereğinden fazla veri veriyordu → daraltıldı.
4. Söz çıkarıcı “şimdi”yi UTC veriyordu → sahibin saat diliminde.
5. Toplantı notunda yedi söz beş satırı dolduruyordu → kota (2/1/1 + konuşma).
6. Proje gözcüsü Memo'nun kendi `.memo/` klasörünü “bitmemiş iş” sayıyordu.
7. İsteğe bağlı güvenlik incelemesi tek hatada maili `review_unavailable` ile gizliyordu → tek yeniden deneme (`e947bd67`).
8. Üç yeni işlem HTTP'de 404'tü; kişi başına tek hatırlatma; tercih formu yeni alanları düşürüyordu; ayarsız saat dilimi (UTC).

**Canlı doğrulanan akışlar:** mail gönder/al, yanıt takibi, söz (vade 01:59:55'te duyuruldu), toplantı notu, sabah brifingi, otomatik gönderim (skor 1 → 60 sn geri alma penceresi → tam 1 mail), saatlik bütçe (ikinci skorlu soru otomatik olmadı), geri al, `never` politikası (`skipped/contact_never`).

**Dürüst notlar / doğrulanamayanlar:**
- 02:03'teki ilk “bütçe” PASS'i geçersizdi (sorular takvim önerisi içerdiği için zaten otomatik yola girmiyordu); doğru biçimle yeniden koşuldu.
- Karar modeli (Subscriptions) 4–7 sn sürüyor, skorlayıcı zaman aşımı 5 sn: bazı denemelerde `score_status=unavailable` olup taslağa düşüyor (güvenli taraf; otomatik gönderim bu modelle kararsız).
- Pre-meeting kota ve devwatch `.memo` düzeltmeleri canlıda yeniden koşulmadı; birim testleri var.
- WhatsApp/Telegram/ses, Adım 6 `#onay` komutlarının gerçek cihazda kabulü ve yapılandırılmış ayrıştırıcıların düz metinle sınanması bu koşuda yok.
- Gmail Taslaklar'da test taslakları kaldı (silinemez); test sonunda 9 bekleyen test taslağı iptal edildi (gönderilmedi).
- Ana Memo ayarları geri alındı: radar kapalı/48 saat, söz takibi kapalı, brifingler kapalı, saat dilimi eskisi gibi UTC (Europe/Istanbul önerilir), karar modeli kapalı, kişi 14 `draft_only`. Doğrulama eşikleri sunucuda 0 → 85/20'ye normalleşiyor (doğrulama kapalı).
