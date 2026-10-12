# v4.6.0 — Beta dışındaki özellikleri tamamlama

Kaynaklar: `versinNote/v4.6.0.md`, TR karşılığı, `docs/AUDIT_2026-10-12.md`, güncel graph/snippet ve doğrudan kaynak kontrolleri.
Beta araçları/tarayıcı/Swarm kapsam dışı. Gerçek cihaz veya gerçek hesapla denenmemiş bir akış, kanıt olmadan bozuk sayılmaz.
Kullanıcı 12 Ekim’de kalanların hepsini sırayla bitirmeyi açıkça istedi; önceki oturum sınırı bu çalışma için kaldırıldı. Her birim ayrı doğrulanır ve commit edilir.

## Bu oturum

- [x] 1. Sağlayıcı kayıt güvenilirliği: effort/context disk roundtrip; Set/Delete/Save hatasını döndürme ve bellek rollback; App router'ını yalnız başarılı yazmadan sonra değiştirme; bağlantı testinde başarısız modelin katalogla gizlenmesini engelleme. Eski config uyumluluğu, şifreli anahtar, başarısız ekleme/düzenleme/silme ve gerçek HTTP testleri.
- [x] 2. Yeni model seçimini bütün sohbet girişlerine taşıma: üst seçici ile composer menüsünde ortak kayıtlı/abonelik model satırları, Auto Route ve CLI'dan çıkış; uzun abonelik listesinde kaydırma/arama ve TR/EN widget testleri. Katalog tarama ile kullanıcıya eklenen model ayrımını koruma.

## Sonraki oturumlar — tamamlanmış sayılmayacak

- [x] Auto Route: ilk model cevap/araç yan etkisinden önce hata verdiğinde sınırlı güvenli alternatif; soğuk yerel model yükleme hatası; bilinen tükenmiş abonelik kotasının seçimde ele alınması. Farklı bağlam bütçelerinde yeniden mesaj oluşturma ve kullanılan model kaydını doğru tutma.
- [ ] Ajan thinking: provider metnini canlı akışa taşıma ve final kaydına ekleme; Native blok replay'i koruma.
- [ ] Hafıza: embedding kesilince pinned fallback'e ulaşma; FTS kurtarma kapsamını belirleme; limit ve timeout testleri.
- [ ] Mobil takvim: dışarıdan ekleme/silme/başlık değişimi için alarm uzlaştırma; başarısız planlamayı tekrar deneme; gerçek cihaz doğrulaması.
- [ ] Görseller/yedek: şifreli görseller ve anahtar kurtarma için açık kapsam; yerel config yolu ve okuma hataları; cihazlar arası provider credential restore. Taşınabilir anahtar politikası belirlenmeden sessizce anahtarları yedeğe ekleme.
- [ ] Karar/doğrulama: sınırlı transcript nedeniyle eksik kanıtın görünürlüğü; gerçek kalite/gecikme/maliyet ölçümü. Puanları ölçülmüş hata olasılığına dönüştürme.
- [ ] Mobil bağlantı: gerçek Android/iOS/emülatörde cleartext, klavye ve scope yenileme; web'de giriş sonrası onboarding yenilemesi.
- [ ] Abonelikler: gerçek uzak callback girişi ve Claude kota biçimi; platform sidecar runtime doğrulaması.

Ayrıntılı durum: `docs/V4_6_COMPLETION_REVIEW.md`.
