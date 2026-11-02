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
| 1 | Mail çekirdeği: hesap, gelen kutusu, okuma, arama, gönderme, taslak, yapısal ayrıştırma | | |
| 2 | Dinleyici: soğuk başlangıç, artımlı eşitleme, triyaj, sayaçlar | | |
| 3 | Güvenlik: prompt injection, DLP, grup/CC kapısı, sızıntı | | |
| 4 | İkiz veritabanı: kişiler, üslup, salt-okunur araçlar | | |
| 5 | Otonom karar motoru: hazırlama, takvim harmanlama | | |
| 6 | Onay köprüsü: onay, iptal, bekletme, tek gönderim, geri al | | |
| 7 | Yanıt/söz takibi, brifing, toplantı notu, proje gözcüsü, TaskLoop köprüsü | | |

## Sonuç
(gece koşusu sonunda doldurulur)
