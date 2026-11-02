# Dijital İkiz — Adım 6: onay, geri al ve gönderim

Adım 5'in hazırladığı yeni yanıtlar Mail → Yanıtlar ve Dijital İkiz → Yanıt
otomasyonu bölümlerinde görünür. Mail taslağını düzenlemeye açmak sunucudaki
otomatik geri sayımı duraklatır. WhatsApp taslağı panelden **Onayla ve gönder**
ile gönderilir; mailin mevcut incele/düzenle/gönder akışı korunur.

## Otomatik gönderim

- Yalnız yeni köprünün sürüm işaretini taşıyan, motorun bütçe rezervasyonu bulunan
  adaylar çalışır. Önceden hazırlanmış kayıtlar otomatik gönderilmez.
- Geri al süresi model ve IMAP Drafts yüklemesi bittikten sonra başlar; varsayılan
  60 saniyedir. Ek ayarlarda değiştirilebilir. Kartta sabit sunucu son tarihinden
  geri sayım görünür. **İptal**, gönderim rezervasyonundan önce kazanırsa gönderim
  yapılmaz; SMTP'ye verilmiş bir mesaj geri alınamaz.
- Gönderim rezervasyonu anında ana/kanal ayarı, mevcut kişi tercihi, bağımsız
  karar skoru (en az 0.95), grup/CC ve takvim kısıtları yeniden kontrol edilir.
  DLP transporttan önce tekrar çalışır. Mail hesabı / WhatsApp hesap kimliği
  kayıttan alınır; WhatsApp hesabı hâlâ aynı bağlı hesap olmalıdır.
- Ayar kaydetmek mevcut geri sayımları duraklatır; yeniden açmak onları otomatik
  etkinleştirmez. Uygulama yeniden başlatıldığında kalan otomatik adaylar da
  kullanıcı onayına döner. Süresi 30 saniyeden fazla geçmiş adaylar gönderilmez.
  Çevrimdışı dönemi yakalayıp topluca gönderme yoktur.
- Açık sahibin manuel mail düzenleme/gönderme isteği önceki davranışı korur:
  hazırlama motoru kapalıyken de gönderilebilir; kişi `never` ve DLP kontrolü
  yine uygulanır. Otomatik/uzaktan gönderimler kapalı kanalı aşamaz.

## Sahip bildirimleri ve komutlar

Yeni bekleyen işlemler admin-only `GET /api/twin/events` SSE akışına (ve mevcut olay listesine) `autonomy:action_pending`, sonuçlar
`autonomy:action_executed` olarak çıkar. Ambient veri yalnız işlem kimliği,
numarası ve durumudur; konu, alıcı veya gövde içermez. Özel içerik admin API'sinde
kalır. Açık Mail görünümü ve İkiz paneli SSE sinyaliyle durumları yeniler; bağlantı kesilirse periyodik yenileme devam eder. Arka planda akış iptal edilir.

Bağlı WhatsApp hesabının kendi sohbetine ve etkin/önceden bağlanmış Telegram
sahibine aynı anda bağımsız bildirim denenir. Bildirim alıcı, konu, taslak ve
benzersiz işlem numarasını içerir; DLP'den geçer. Her hedef/durum için bildirim
denemesi kalıcı olarak bir kez rezerve edilir. Belirsiz ağ hatasında otomatik
bildirim tekrarı yoktur; UI'dan inceleme her zaman mümkündür. Başka Telegram
kullanıcıları ve gruplar onay veremez; Telegram üçüncü kişilere açılmaz.

- `#onay 12` / `#approve 12`: bildirilen taslağı onayla ve gönder.
- `#iptal 12` / `#cancel 12`: bekleyen yanıtı iptal et.
- `#asla 12` / `#never 12`: bu kişiye yanıtı kapat; kişinin tüm bekleyenlerini iptal et.
- `evet`, `gönder`, `iptal`, `bir daha bu kişiye otomatik dönme` (ve İngilizce
  karşılıkları) yalnız o sahip kanalına bildirilmiş tek uygun kayıt varsa çalışır.
  Belirsizlikte numara istenir. Gönderilmiş yanıtın numarasıyla kişi kuralı da
  değiştirilebilir; bu, daha önce gönderilmiş mesajı geri almaz.

Komutlar modelle yorumlanmaz. Aynı gelen mesaj kimliği kalıcı olarak tüketilir;
yinelenmiş bir “evet” sonraki taslağı onaylayamaz. Beş dakikadan eski, tarihsiz
veya gelecekteki eski/yakalama mesajları gönderim yetkisi vermez. Açık bir ajan
izin sorusu varken sade cevaplar mevcut izin sorusuna gider; `#onay N` gibi açık
numaralı komutlar ayrı işlenir. Uzaktan onay komutları metin içindir.

## Kalıcılık ve API

Tüm gönderim yolları aynı SQLite yazıcısında `pending → sending` rezervasyonu
alır. Paralel onay/iptal/otomatik işleme yalnız biri kazanabilir. Başarılı kabul
`sent`, belirsiz transport/depolama sonucu `send_uncertain` olur; başlangıçta
kesilmiş `sending` de belirsiz sayılır. Otomatik yeniden gönderim yoktur. SMTP
kabulü alıcı kutusuna teslim teyidi değildir. Uzak Drafts kopyası silinmez;
yerel iptal takvim önerisini geri almaz.

Yeni admin-only POST uçları (normal ve `/api/v1`):

- `/api/twin/approve_action`, gövde `{"id":"action-id"}`: kayıttaki taslağı
  mevcut kanal izniyle gönderir. Alıcı/hesap/gövde dışarıdan kabul edilmez.
- `/api/twin/hold_action`, aynı gövde: geri sayımı duraklatır, taslağı korur.

Mevcut `send_action` düzenlenmiş mail içindir; HTTP hata kodları ve belirsiz
sonuç sözleşmesi [MAIL_REPLY_REVIEW.md](MAIL_REPLY_REVIEW.md) içindedir.

## Doğrulama ve cihaz kabulü

Otomatik testler sahte zamanla 60 saniye/iptal, güncel izin/skor/grup/versiyon ve
bütçe kontrolü, atomik paralel gönderim, geç hazırlamanın iptali ezememesi,
bildirim kalıcılığı ve UI'nın son-100 sınırından bağımsız kuyruk kapsamını içerir.
App testleri gerçek loopback SMTP üzerinden UI, otomatik ve iki uzaktan kontrol
hedefinin gönderim/tekrar/hata yollarını sınar. Gerçek App + HTTP + SMTP E2E;
sahte Telegram Bot API'sinde gerçek HTTP bildirim, sahip sınırı, iptal, olay
mahremiyeti ve yinelenen komut testi vardır. Flutter TR/EN testleri WhatsApp
onayı, süre ayarı ve mail incelemesinin sunucuda geri sayımı durdurmasını sınar.

Bunlar gerçek Gmail/WhatsApp/Telegram hesabı kabulü değildir. Cihazda yeni bir
izinli test kişisiyle önce taslak ve numaralı iptal, sonra açıkça onaylanmış tek
bir gönderim denenmelidir. Otomatik denemede bağımsız skorun gerçekten alınması
ve geri al süresi boyunca iptal edilebilmesi ayrıca görülmelidir. Eski
`a7324752…` yanıtını tekrar göndermeyin. Sonraki plan adımı başlatılmaz.

Son teknik doğrulama (2 Kasım 2026, kod `3471ae59`): [tam CI](https://github.com/BugraAkdemir/memo/actions/runs/41503875218)
beş işte başarılı. CGO/sqlite_fts5 build/vet exit0; Go race71 paket PASS;
Flutter890 PASS ve ek geliştirme backend adresi testi1 PASS. Yerel
`flutter analyze lib/` yalnız mevcut3 context info; CI tüm frontend
analizinde mevcut18 info (test dosyaları da dahil), yeni bulgu yok.
Değişen11 Dart dosyasında UI literal0; diff kontrolü PASS. Dar ekran
testlerinde kaydırma yerleşimini bekleme ve eski mail test adaptöründe
SSE handshake desteği düzeltildi; davranış beklentileri kaldırılmadı.


### Örnek cihaz testi: iptal → tek gönderim → geri al

**Hazırlık:** Adım 6 kodunu içeren backend ve Flutter derlemesini çalıştırın.
Önceki geliştirme oturumunda çalışan uygulama yenilenmedi; eski çalışan sürümle
bu kabulü değerlendirmeyin. İki size ait mail hesabı kullanın: Memo'nun bağlı
hesabı ve test göndereni. Mail yanıt hazırlamayı açın, ayrı yanıt modelini seçin;
genel tercihi ve test kişisini **yalnız taslak** yapın. Uzaktan komut testi için
WhatsApp kendi sohbeti veya önceden eşlenmiş özel Telegram sahip sohbeti hazır
olsun. Her turda yeni bir mail gönderin; CC, grup ve ek kullanmayın. Eski
`a7324752…` ve `453da7f6…` kayıtlarını bu teste katmayın.

1. **İptal:** Test hesabından konu `MEMO ADIM6 TEST 1 — İPTAL`, gövde
   `Merhaba, 3 Kasım 2026 saat 14:00–15:00 (Europe/Istanbul) arasında takviminde
   kayıtlı bir etkinlik var mı? Yalnızca dolu/boş bilgisini paylaşabilir misin?
   Toplantı oluşturmanı veya davet göndermeni istemiyorum.` olan maili yollayın.
   Mail → Yanıtlar'da hazırlanan taslağı ve sahip sohbetindeki bildirimi bulun.
   Bildirimdeki gerçek numarayla, beş dakika içinde `#iptal <numara>` gönderin
   (`<numara>` yerine bildirimin sayısı yazılır).
   **Beklenen:** kayıt iptal olur; test hesabına yanıt gitmez. Uzak Drafts
   kopyasının kalması normaldir. Taslak oluşmazsa gönderime geçmeden sonucu
   kaydedin; bunun nedenini yalnız bu senaryodan çıkarmayın.
2. **Tek gönderim:** Aynı gövdeyle yeni konu `MEMO ADIM6 TEST 2 — ONAY` gönderin.
   Yeni taslağın alıcısını ve içeriğini kontrol edin. Yeni bildirim numarasıyla
   `#onay <numara>` gönderin.
   **Beklenen:** UI gönderildi durumuna geçer ve test hesabının gelen kutusunda
   tek yanıt görünür. Aynı numaralı onayı yeniden iletmek ikinci mail üretmemeli.
   SMTP kabulü ile gelen kutusunda teslimi ayrı kaydedin. Sonuç belirsizse yeniden
   gönderim denemeyin; önce gönderilenler ve alıcı hesabını kontrol edin.
3. **Otomatik geri al (ayrı kabul):** Genel tercihi yalnız taslak bırakın;
   yalnız test kişisini tam otomatik yapın. Bağımsız karar modelini ve 60 saniye
   geri al süresini ayarlayın. Yeni konu `MEMO ADIM6 TEST 3 — GERİ AL` ile,
   inceleme izni verdiğiniz kayıtlı bir proje için `Merhaba, [kayıtlı proje adı]
   projesinde şu an hangi Git dalı etkin? Yalnız mevcut dal adını bildir;
   dosya değiştirme veya komut çalıştırarak değişiklik yapma.` yazın; köşeli
   parantezi gerçek proje adıyla değiştirin.
   **Beklenen:** ancak gerekli kanıt ve bağımsız skor en az 0,95 ise otomatik
   aday/geri sayım oluşur. Oluşursa 60 saniye dolmadan UI'dan veya yeni numarayla
   `#iptal <numara>` üzerinden iptal edin; yanıt gitmemeli. Geri sayım sırasında
   editörü açmak, ayar kaydetmek veya yeniden başlatmak sayacı duraklattığından
   bu turda bunları yapmayın. Skor yoksa/düşükse onay beklemesi otomatik yolun
   geçtiği anlamına gelmez; sonucu bu şekilde raporlayın. İptal edilmiş otomatik
   aday da kişi/dizi bütçesini tüketir (saatte 1, 24 saatte 3); hemen tekrar
   otomatik aday beklemeyin veya sınırı aşmaya çalışmayın.

Sonuç kaydı: çalışan kod sürümü, bildirim kanalı, her turun numarası ve UI
sonucu, test hesabına ulaşan yanıt sayısı, varsa geri sayım/skor veya hata.
Bu metin test yönergesidir; gerçek hesap/native kabulü henüz yapılmış sayılmaz.
Adım 7 kullanıcı geri bildirimi gelmeden başlatılmaz.
