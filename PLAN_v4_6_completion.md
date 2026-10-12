# v4.6.0 — Beta dışındaki özellikleri tamamlama

Kaynaklar: `versinNote/v4.6.0.md`, TR karşılığı, `docs/AUDIT_2026-10-12.md`, güncel graph/snippet ve doğrudan kaynak kontrolleri.
Beta araçları/tarayıcı/Swarm kapsam dışı. Gerçek cihaz veya gerçek hesapla denenmemiş bir akış, kanıt olmadan bozuk sayılmaz.
Kullanıcı 12 Ekim’de kalanların hepsini sırayla bitirmeyi açıkça istedi; önceki oturum sınırı bu çalışma için kaldırıldı. Her birim ayrı doğrulanır ve commit edilir.

## Bu oturum

- [x] 1. Sağlayıcı kayıt güvenilirliği: effort/context disk roundtrip; Set/Delete/Save hatasını döndürme ve bellek rollback; App router'ını yalnız başarılı yazmadan sonra değiştirme; bağlantı testinde başarısız modelin katalogla gizlenmesini engelleme. Eski config uyumluluğu, şifreli anahtar, başarısız ekleme/düzenleme/silme ve gerçek HTTP testleri.
- [x] 2. Yeni model seçimini bütün sohbet girişlerine taşıma: üst seçici ile composer menüsünde ortak kayıtlı/abonelik model satırları, Auto Route ve CLI'dan çıkış; uzun abonelik listesinde kaydırma/arama ve TR/EN widget testleri. Katalog tarama ile kullanıcıya eklenen model ayrımını koruma.

## Kalan maddeler — kullanıcının talimatıyla bu çalışmada sırayla

- [x] Auto Route: ilk model cevap/araç yan etkisinden önce hata verdiğinde sınırlı güvenli alternatif; soğuk yerel model yükleme hatası; bilinen tükenmiş abonelik kotasının seçimde ele alınması. Farklı bağlam bütçelerinde yeniden mesaj oluşturma ve kullanılan model kaydını doğru tutma.
- [x] Ajan thinking: provider metnini canlı akışa taşıma ve final kaydına ekleme; Native blok replay'i koruma.
- [x] Hafıza: embedding kesilince pinned fallback'e ulaşma; FTS kurtarma kapsamını belirleme; limit ve timeout testleri.
- [x] Mobil takvim: dışarıdan ekleme/silme/başlık değişimi için alarm uzlaştırma; başarısız planlamayı tekrar deneme. Gerçek HTTP SSE ve Android platform-kanalı widget testi geçti.
- [ ] Mobil takvimin gerçek telefon/emülatörde OS alarmı doğrulaması (kod testinden ayrı).
- [x] Görseller/yedek: şifreli görseller ve anahtar kurtarma için açık kapsam; yerel config yolu ve okuma hataları; cihazlar arası provider credential restore. Taşınabilir anahtar politikası belirlenmeden sessizce anahtarları yedeğe ekleme.
- [x] Karar/doğrulama: kısmi transcript, atlanan mesaj/kısaltılan metin sayısı ve ölçülmemiş model tahminleri görünür; uzun çıktının test hatası içeren sonu korunur.
- [ ] Gerçek modelle doğrulayıcı kalite/gecikme/maliyet ölçümü (kod testinden ayrı; puanlar ölçülmüş olasılık olarak sunulmaz).
- [x] Web giriş sonrası onboarding yenilemesi ve sunucu cevabı bilinmeden kurulum açılmasını engelleme.
- [ ] Gerçek Android/iOS/emülatörde cleartext, klavye ve scope yenileme: ortamda yalnız Linux cihazı mevcut.
- [x] Abonelik Linux runtime sözleşmesi: gerçek pinli binary, login bayrakları, loopback anahtar kapısı/katalog/görsel endpoint kontrolü.
- [ ] Gerçek uzak OAuth callback ve Claude kota doluşu; native Windows/macOS runtime.

Ayrıntılı durum: `docs/V4_6_COMPLETION_REVIEW.md`.
