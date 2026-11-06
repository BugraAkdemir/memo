# Dijital İkiz — Faz 2 / Adım 4

Bu adım kişi bağlantıları, tarihli kişi geçmişi, üslup özeti ve salt okunur
araştırma araçlarını içerir. Gelen mesaja kendiliğinden yanıt verme motoru
Adım 5'e, otonom gönderim/onay kuyruğu Adım 6'ya aittir. Bu özellikler burada
başlatılmaz. Kullanıcının normal sohbetten istediği mevcut mail gönderimi
çalışmaya devam eder; gönderim izin politikası değiştirilmemiştir.

## Kullanım ve cihaz kabul senaryoları

Güncel backend ve frontend ile uygulamayı başlat. **E-posta → Dijital İkiz**
panelinde Kişiler, Tercihler ve Salt okunur araçlar sekmeleri bulunur.

1. **Kişi/adres eşleme:** Bir kişi oluştur; e-posta adresini ve ülke koduyla
   WhatsApp numarasını ekleyip kaydet. Aynı adrese sahip ikinci kişi oluşturmayı
   dene: adres çakışması gösterilmelidir. Yeni gelen mesajlar ayrı kişiler
   oluşturmuşsa hedef kişiyi seç, **Kişileri birleştir** menüsünden diğer kişiyi
   seç ve isimleri kontrol ederek onayla. Daha kısıtlayıcı `never` kuralı korunur.
   Uygulamayı kapat/aç: bağlantılar ve tercihler kalmalıdır.
2. **Çapraz kanal geçmişi ve tarihler:** Eşlediğin kişiden yeni bir mail ve
   WhatsApp özel mesajı al. **Geçmişi göster** çıktısında iki kanal ve kaynak
   tarihleri görünmelidir. Başka kişinin mesajı görünmemelidir. Önceki günün
   “yarın” metni değiştirilmez; yanında orijinal zaman damgası kalır. Eski mail
   alınmaz veya yeniden işlenmez. Tarihi bilinmeyen mail bu geçmişe eklenmez.
3. **Giden mesaj/üslup:** Aynı kişiye normal sohbetten veya mail ekranından
   başarılı şekilde üç yeni kısa yanıt gönder. Paneli yenile: örnek sayısı
   artmalıdır. Basit metin işaretlerine göre samimi/resmi üslup, dil ve selamlama
   profili çıkarılır; girdi son en fazla 100 **giden** mesajdır. Gelen mesajlar,
   kaydedilen taslaklar ve başarısız gönderimler kullanıcının üslubu sayılmaz.
   Bu bir modelle eğitilmiş üslup klonu değildir; az örnekte yetersizlik bildirilir.
4. **Git kanıtı/izin:** Tercihlerde proje inceleme kapalıyken Git durumunu oku:
   reddedilmelidir. İzni açıp kaydet. Araçlar sekmesinde kayıtlı, `.memo`
   kurulumu tamamlanmış bir Memo projesini seç: çalışma ağacı, mevcut HEAD'e
   birleştirilmiş yerel/önceden bilinen uzak dallar ve son 10 commit görünmelidir.
   Normal sohbette Agent kapalıyken aynı kayıtlı projenin durumunu sor: yerleşik
   `twin_contacts` ve `git_project_status` araçları kullanılabilir. Keyfi dosya
   yolu kabul edilmez; fetch/push/merge yapılmaz. Repo listede yoksa önce mevcut
   proje kurulum akışından Memo projesi olarak kur; bu panel proje kaydetmez.
5. **Takvim mahremiyeti:** Takvime özel başlıklı bir etkinlik ekle. Araçlar
   sekmesinde o günü kontrol et: yalnız başlangıç/bitiş doluluğu ve çalışma saati
   tercihleri görünmelidir. Özel başlık, açıklama, kişi ve konum görünmemelidir.
   Önceki günden başlayıp seçilen güne taşan etkinlik de dahil edilir. Bitiş
   belirtilmemişse kesişim kontrolü için bir saat kabul edilir; çıktıda bitişin
   bilinmediği yazılır. Boş takvim, çalışma saatleri dışında müsaitlik garantisi
   değildir. Gün seçimi cihaz saat dilimine, tercih saatleri kaydedilen IANA
   saat dilimine göredir; sonuçlar zaman dilimi bilgisi taşır.
6. **Arayüz ve sınırlar:** TR/EN değiştir; panel başlıkları, hatalar ve araç
   adları çevrilmelidir. Pencereyi daralt: kişi listesi düzenleyicinin üstüne
   geçmelidir. Proje iznini kapattıktan sonraki başarısız sorgu önceki Git
   sonucunu ekranda bırakmamalıdır. Kısıtlı kullanıcı bu özel API'leri ve sohbet
   araçlarını kullanamamalıdır. Code Mode kişisel ikiz araçlarını açmamalıdır.

**Telegram sınırı:** Mevcut bot yalnız hesap sahibinin konuşmasını kabul eder;
başka kişilerin Telegram geçmişini getirmez. Sayısal özel sohbet adresleri kişi
grafiğinde açıkça saklanabilir, fakat yeni bir Telegram üçüncü kişi bağlantısı
kurulmuş sayılmaz. Sahibin Telegram bot konuşmaları, WhatsApp öz sohbeti,
gruplar ve yayın listeleri dış kişi geçmişine kopyalanmaz.

## Veri ve güvenlik sözleşmesi

- SQLite dosyası `config.DataPath("twin", "twin.db")` üzerinden açılır.
  `contacts`, `contact_aliases`, `twin_preferences`, `commitments`,
  `processed_external_events`, `pending_autonomous_actions`, `twin_audit_log`
  ve tarihli `contact_messages` tabloları bulunur. Söz ve bekleyen aksiyon
  tablolarının varlığı, sonraki adımların motorlarının çalıştığı anlamına gelmez.
- Tüm yazılar `database.DB.Write` işlemleriyle seri ve atomiktir. Gelen olay
  kimliği kanal/hesap kapsamında tekrarları engeller; bir mailin farklı
  alıcıları ayrı kaydedilir. Kişi adları otomatik birleştirme gerekçesi değildir.
  Birleştirme, ilerideki bekleyen aksiyonları otomatik olarak yeni kimliğe
  devretmez; bekleyenleri iptal eder.
- Yeni, filtreden geçmiş mailler ve yeni birebir WhatsApp metinleri kaydedilir.
  Başarılı yerleşik mail/WhatsApp gönderimleri giden geçmişe eklenir. Eski
  mesajların toplu aktarımı veya mail backfill yapılmaz. Mesaj metni yeniden
  yazılmaz; kaynak zamanı orijinal saat dilimi farkıyla saklanır; farklı dilimlerdeki mesajlar gerçek zaman sırasıyla listelenir.
- `ExternalAutonomousExecutor.Execute` göndericiyi güvenilir girişten alır,
  adresi grafikte çözer ve yalnız o kişinin geçmişini açar. Model argümanı
  gönderici kimliğini değiştiremez. Bilinmeyen gönderen kapalı kalır.
  Yürütücüde genel araç kaydı, terminal veya dosya yazma geri çağrısı yoktur;
  `run_command`, `write_file`, `edit_file`, gönderim ve diğer araçlar reddedilir.
  Sahip adına kullanım ayrı owner API'si/sohbet servisinden geçer.
- Git yalnız kayıtlı proje kimliğiyle, kabuksuz sabit argümanlarla çağrılır.
  Kalıtılan `GIT_*` yönlendirmeleri temizlenir; global/sistem ayarları, fsmonitor,
  hook ve clean/process filtreleri devre dışı bırakılır. İsteğe bağlı indeks
  yazıları kapalıdır. Toplam 10 saniye ve komut başına 32 KiB çıktı sınırı vardır;
  kesilmiş çıktı başarı/kanıt olarak sunulmaz. Uzak sunucu durumu güncellenmez.
- `check_availability` en fazla 31 günlük aralık kabul eder; takvim nesnesini
  doğrudan serileştirmez. Çalışma saatleri bir tercih çıktısıdır; bu adım boş
  saat öneren veya toplantı kabul eden motoru çalıştırmaz.
- Araç sonuçları modele güvenilmeyen veri zarfında verilir. Yeni kurulmuş boş
  depo normal sohbeti araç döngüsüne sokmaz. Kişi veya kaydedilmiş tercih varsa
  ikiz araçları Agent kapalı normal sohbette de sunulur. Minimal/Code Mode ve
  yalnız metin destekleyen web sağlayıcılarının mevcut sınırları korunur.
- `GET /api/twin/state`; `POST /api/twin/contact`, `/merge`, `/preferences`,
  `/contact_timeline`, `/check_availability`, `/git_project_status` admin-only
  uçlarıdır. `/api/v1/twin/...` eşleri de vardır. Dış mesajlar owner yazma
  uçlarına bağlanmaz. Bu panelin salt okunur araçları gerçek alıcılara mesaj göndermez.
- İkiz verisi yerel ve şifreli bulut yedek envanterine dahildir; canlı depo
  üzerinden WAL checkpoint yapılır. Geri yükleme/silme/kapanış dosya tutamacını
  kapatır. Geri yükleme sonrası yeniden başlatma gerekir. Per-device proje
  kayıt dosyasının mevcut yedekleme dışı durumu değişmez.

## Teknik doğrulama

Bu değişikliğin otomatik testleri gerçek geçici SQLite/Git depolarında kişi
ayırma/birleştirme, yeniden açma, eşzamanlı tekrar, giden mesajdan üslup,
başlık maskeleme ve önceki günden taşan etkinlikleri sınar. Git testinde repo
fsmonitor/clean/process komutları çalışmamalı ve indeks baytları değişmemelidir.
Gerçek App + HTTP/SSE testi Agent kapalı kişi aracını; loopback SMTP testi
başarılı gönderimin kişi geçmişine yazılmasını doğrular. Widget testleri TR/EN
kayıt, tarihli geçmiş, tercihler, açık birleştirme onayı, Git reddinde eski
çıktının temizlenmesi ve dar pencere düzenini sınar. Gerçek kullanıcı kabulü
bu belgede yazan senaryolarla ayrıca beklenir.

31 Ekim 2026 son çalıştırma sonuçları: `CGO_ENABLED=1` ve `sqlite_fts5` ile
`go build ./...` ve `go vet ./...` exit 0; `go test ./... -race` 71 paket
PASS, 0 FAIL (düşük paralellik). `flutter test` 866/866 PASS;
`flutter analyze lib/` yalnız önceden var olan 3 context info bildirir.
Değişen/yeni 6 UI Dart dosyasında hardcoded UI literal taraması 0 eşleşme;
`git diff --check` PASS. Gerçek cihaz/hesap senaryoları kullanıcı kabulünü bekler.
