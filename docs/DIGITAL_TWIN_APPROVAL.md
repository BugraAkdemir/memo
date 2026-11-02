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
