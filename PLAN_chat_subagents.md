# Normal sohbet alt ajanları

Onay: kullanıcı 2026-10-23 konuşmasında tasarımı ve kodlamayı onayladı.
ECC orch-add-feature: büyük kapsam; plan ve commit için yeniden onay istenmez
(kullanıcının açık kodlama onayı ve AGENTS.md otomatik commit kuralı geçerli).

## 1. Çalıştırma, sıra ve kurtarma
- [x] Normal ajan araçları: spawn/send/list/cancel/close/merge; aynı görevde takip mesajları.
- [x] Ayrı geçici görev kayıtları; kişisel hafıza/persona/RAG yok; Code Mode ve seçilen skill talimatları.
- [x] Ayarlanabilir 1–16 eşzamanlı çocuk, varsayılan 3; fazlası FIFO bekler.
- [x] Kalıcı, sıra numaralı tamamlanma kuyruğu. Ana sohbet için tek FIFO yürütme: kullanıcı mesajı ve sonuç çakışmaz.
- [x] Çökmede çalışan görevler interrupted; otomatik komut tekrarı yok; bekleyen sonuçlar kaybolmaz.
- [x] Paralel yazarlar ayrı memo-* branch/worktree. Ana ajan doğruladıktan sonra seri main merge; hata/çatışmada kayıtlar korunur; yalnız birleşmiş işler temizlenir.
- [x] Gerçek geçici Git reposu + fake provider ile kuyruk/sınır/takip/kurtarma/çatışma testleri.

## 2. Arayüz, doğrulama ve teslim
- [x] Sohbet akışında alt ajan kartları; sağ panelde prompt, çıktı, araç işlemleri; dar ekranda panel görünümü.
- [x] Eşzamanlılık ayarı; TR+EN yerelleştirme. Arka plan ana ajan cevabı sohbet değişmeden yenilenir.
- [x] API yetki/sahiplik denetimi; çocuklar başka sohbeti/alt ajanı yönetemez.
- [x] ECC kod ve güvenlik incelemesi; bulunan sorunları düzelt.
- [x] Go CGO+sqlite_fts5 build/vet/tam race; Flutter analyze lib/ ve tüm testler; L10n/diff kontrolü.
- [x] Doküman/handoff, doğrulanmış aşamalarda Conventional Commit; push yok.

Kabul örneği: çocuk 1 tamamlandığında ana ajan yanıt üretirken çocuk 2 tamamlanırsa
ikinci kayıt kalıcı kuyrukta durur; ilk tur bitmeden ikinci tur başlamaz. Kullanıcının
o sırada gönderdiği mesaj da aynı sohbet sırasını kullanır. Farklı sohbetler birbirini
bekletmez. Kabul edilen işin geçici içeriği ancak başarılı birleştirme/kapatma sonrası temizlenir.
