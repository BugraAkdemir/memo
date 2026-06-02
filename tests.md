# Memo Frontend Test List

Test adımları. Başarılı testleri `[x]` ile işaretle.

---

## Bölüm 1 — Thinking/Reasoning Metnini Ayrıştırma ve Collapsible UI
- [x] DeepSeek R1 / QwQ gibi bir reasoning model seç ve bir soru sor
- [x] Assistant yanıtında `▶ Düşünme göster` butonu görünüyor mu?
- [x] Butona tıklayınca düşünme metni açılıyor ve buton `▼ Düşünme gizle` oluyor mu?
- [x] Tekrar tıklayınca düşünme metni gizleniyor mu?
- [x] Reasoning olmayan bir modelde (ör. Llama 3) düşünme butonu görünmüyor mu?

## Bölüm 2 — SSE Stream Token Rebuild Optimizasyonu
- [x] Bir model seç ve uzun bir yanıt üretecek bir soru sor (ör. "Türkiye'nin tarihini anlat")
- [x] Token'lar akarken UI donma / takılma olmuyor mu?
- [x] Chrome DevTools > Performance tab'ında `[...list]` spread işlemi görünmüyor mu?
- [x] Stream sırasında diğer mesajların üzerine gelince hover efekti çalışıyor mu? (sadece son mesaj rebuild oluyor)
- [x] Stream tamamlanınca mesaj listeye ekleniyor ve düzgün görünüyor mu?

- Bug Var ben 2. mesajı gönderdiğim zaman al ilk çıktısı siniyor

## Bölüm 3 — Incognito Toggle Race Condition
- [ ] Incognito toggle'a tıkla (açık/kapalı)
- [ ] Backend kapalıyken toggle'a tıkla — state geri alınıyor mu?
- [ ] İnternet kesikken toggle — hata sonrası eski state'de kalıyor mu?
- [ ] Normal çalışırken toggle — state doğru değişiyor ve UI güncelleniyor mu?

## Bölüm 4 — Stream İptali (Orphaned Stream)
- [ ] Stream devam ederken (yanıt gelirken) başka bir sohbete tıkla
- [ ] Eski sgohbetteki stream duruyor mu? (backend lo'unda hata/kesinti yok)
- [ ] Yeni sohbet açılıp mesaj gönderilebiliyor mu?
- [ ] Stream devam ederken "Yeni Sohbet" butonuna bas — stream iptal oluyor mu?
- [ ] Stream devam ederken uygulamayı kapat — dispose'da stream cancel oluyor (log'da hata yok)

## Bölüm 5 — Hata Mesajlarını Chat'e Yazma
- [ ] Backend'i kapat, bir mesaj göndermeyi dene
- [ ] Hata mesajı chat balonu olarak değil, **kırmızı snackbar** olarak görünüyor mu?
- [ ] Snackbar kendiliğinden kayboluyor mu?
- [ ] Backend'i tekrar aç, mesaj gönder — normal çalışıyor mu?
- [ ] Snackbar'daki hata metni anlaşılır mı? (Bağlantı hatası: ...)

## Bölüm 6 — Çift Mesaj Göndermeyi Engelle
- [ ] Stream devam ederken hızlıca 2 kere Enter'a bas / gönder butonuna tıkla
- [ ] İkinci mesaj gönderilmiyor mu? (sadece tek yanıt geliyor)
- [ ] Stream bittikten sonra yeni mesaj gönderilebiliyor mu?
- [ ] Dosya gönderirken (attach) de aynı koruma çalışıyor mu?

## Bölüm 7 — Zaman Damgasına Saniye Ekle
- [ ] İki mesajı hızlıca arka arkaya gönder (aynı dakika içinde)
- [ ] Zaman damgalarında saniye farkı görünüyor mu? (örn. `14:30:05` vs `14:30:12`)
- [ ] Eski mesajların timestamp'i `HH:mm:ss` formatında mı? (refresh sonrası backend'den gelir)

## Bölüm 8 — Export Chat İyileştirmesi
- [ ] Export butonuna bas
- [ ] Dosya kaydet dialogu açılıyor mu?
- [ ] Bir konum seçip kaydet
- [ ] Dosya içeriği chat mesajlarını içeriyor mu? (markdown formatında)
- [ ] Dosya adı varsayılan olarak `chat_export.md` mi?

## Bölüm 9 — Silme İşlemlerine Onay Dialogu
- [ ] Sidebar'da bir chat'in üzerine gel, çarpı (x) butonuna bas
- [ ] Onay dialogu açılıyor mu? Chat adı dialog'da görünüyor mu?
- [ ] "İptal" butonuna bas — chat silinmiyor mu?
- [ ] "Sil" butonuna bas — chat siliniyor mu?
- [ ] Ayarlar > Hafıza > "Hafızayı Temizle" butonuna bas — onay dialogu açılıyor mu?
- [ ] Onayla — hafıza temizleniyor mu?
- [ ] Model Store > Local Models > Sil (çöp kutusu) butonuna bas — onay dialogu açılıyor mu?
- [ ] Onayla — model siliniyor mu?

## Bölüm 10 — Boş Mesaj Kontrolü
- [ ] Sadece boşluk (`   `) girip Enter'a bas — mesaj gönderilmiyor mu?
- [ ] Tamamen boş input + Enter — mesaj gönderilmiyor mu?
- [ ] Normal bir mesaj girip gönder — çalışıyor mu?
- [ ] `/` yazıp template seç — template metni ile gönderilebiliyor mu?

## Bölüm 11 — HuggingFace İndirilen Modellerin Algılanmaması
- [ ] Model Store'dan bir model ara ve indir
- [ ] İndirme tamamlanınca model `models/model.gguf` (flat) dizininde mi? (alt dizin yok)
- [ ] Model listesinde görünüyor mu?
- [ ] Modeli çalıştırabiliyor musun?
- [ ] İçe aktarma (import) butonu ile local bir GGUF dosyası ekle — çalışıyor mu?
