# 🐾 Masaüstü Maskotu

> **Frontend dosyaları:** `frontend/lib/mascot_main.dart` (ayrı giriş noktası), `frontend/lib/widgets/mascot_window.dart`, `frontend/lib/widgets/memo_mascot.dart`, `frontend/lib/providers/mascot_provider.dart`
> **Backend:** `internal/app/activity.go`, `internal/models/activity.go`, `internal/webserver/handlers_activity.go`
> **API endpoint'i:** `GET /api/mascot/activity`
> **Tanıtıldı:** v4.5.0

Masaüstünüzde, sohbet penceresinden ayrı yaşayan ve Memo'nun gerçekte ne yaptığını gösteren küçük animasyonlu bir karakter: düşünüyor, yazıyor, bir tool çalıştırıyor, ya da sadece boşta sizi bekliyor.

---

## Gerçek bir ikinci pencere, ikinci bir uygulama değil

Ayarlar → Genel'den ya da tepsiden etkinleştirin, sürükleyerek ekranın herhangi bir yerine koyabileceğiniz küçük, her-zaman-üstte bir karakter belirir. Aynı çalışan uygulama içinde gerçek bir ikinci penceredir — aynı süreç, aynı durum, aynı yaşam döngüsü (`desktop_multi_window` ile) — ikinci bir Memo binary'si değil. (Bu özelliğin ilk versiyonu gerçekten ayrı bir süreç olarak başlatılmıştı; çalışıyordu ama belleği ikiye katlıyordu ve eklenti gibi hissettiriyordu — bu, göndermeden önce yeniden işlendi.)

`frontend/lib/mascot_main.dart`, gerçek, ayrı bir Dart giriş noktasıdır (kendi `main()`'i, codebase-memory grafiğinin `entry_points`'inde kendi satırı) — `frontend/lib/main.dart`'ın `AppShell`'ine katılmak yerine ikinci pencere olarak başlatılır.

---

## Memo'nun ne yaptığını gerçek zamanlı bilir

Her ajan tool çağrısı, her düz sohbet yanıtı, her görev-döngüsü turu — hangi kanaldan gelirse gelsin (sohbet, WhatsApp, Telegram, bir görev listesi) — maskotun yokladığı tek bir uygulama-geneli aktivite sinyalini besler (`internal/app/activity.go`, `GET /api/mascot/activity`'de sunulur). Düşünüyor, yazıyor, belirli bir tool çalıştırıyor, yanıt üretiyor: pozu gerçek zamanlı değişir.

Bu aynı aktivite sinyali, terminal REPL'inin spinner/renk çıktısını da besler (`internal/replcli/color.go`, `repl.go`) — maskot ve REPL, aynı temel "Memo şu an ne yapıyor" kaynağının iki farklı tüketicisidir.

---

## Sade-dilli bir durum balonu

Karakterin altında küçük bir balon ne olduğunu anlatır — "Düşünüyor…", "Yazıyor…", "search_web çalıştırılıyor…" — asla modelin gerçek yanıt metni değil, sadece sade bir durum. Bir tur bittiğinde, maskot kollarını havaya kaldırır ve balon birkaç saniye "Tamamlandı!" der, sonra boşta durumuna döner.

---

## Boşta donuk demek değil

Yalnız bırakıldığında karakter göz kırpar, nefes alır ve öngörülemeyen aralıklarla ara sıra rastgele bir sallanma, zıplama ya da hafif salınım yapar — böylece duraklamış değil canlı gibi görünür.

---

## Her-zaman-üstte, başka her yerde tıklama-geçirgen

Karakterin gerçekten her pencerenin üstünde kalmasını sağlamak, gerçek bir Wayland kısıtlamasının çözülmesini gerektirdi — Wayland compositor'ları bir uygulamanın kendini üste zorlamasını kasıtlı olarak reddeder (odak-çalmayı engelleyen aynı güvenlik mantığı) — bu, tüm uygulamayı gerçekten çalışan bir mekanizma elde etmek için XWayland'a zorlamayı gerektirdi. Karakterin tıklama/sürükleme alanı da artık etrafındaki kare sınırlayıcı kutu yerine gerçek silüetine göre şekillendirildi — karakterin etrafındaki şeffaf boşluk artık arkasındaki bir şeye yönelik tıklamaları yutmuyor.

---

## İki görünüm

Ayarlar → Genel'den, her biri için canlı önizlemeyle seçilir: orijinal sıcak, elle-çizilmiş yaratık, ya da lacivert piksel-art bir robot. İkisi de aynı animasyon sisteminde çalışır — aynı ruh halleri, aynı boşta hareketleri — sadece farklı bir görünüm.

---

## Live Mode'da da konuşur

Memo bir [[Multimodal Yetenekler (Görsel ve Ses)|Live Mode]] konuşmasında size konuşurken, maskotun ağzı zamanlamayla açılıp kapanır, küçük bir ses-dalgası efekti ve bir "Konuşuyor…" balonuyla. Bilinçli olarak sadece konuşmayla sınırlı: ne Google Live'ın ne de OpenAI Realtime'ın istemcisi şu an sağlayıcıdan gerçek bir "dinliyor" sinyali sunuyor, o yüzden onun için bir tane uydurulmadı.

---

### Bağlantılı Notlar:
- [[Ajan Modu]] — Maskotun yansıttığı aktivite kaynaklarından biri
- [[Otonom Görev Döngüsü]] — Başka bir aktivite kaynağı
- [[Multimodal Yetenekler (Görsel ve Ses)|Live Mode v2]] — Konuşma-durumu animasyonu
- [[Mimari Yapı]] — Sistem entegrasyonu
