# Dijital İkiz — Adım5: karar, araştırma ve takvim taslakları

Backend ve Flutter aynı değişiklikte bağlanır. Bu adım **hazırlama** motorudur;
gönderim zamanlayıcısı, WhatsApp/Telegram uzaktan onayı ve çok kanallı bildirim
köprüsü Adım6'dır. Kuyrukta `grace_period_auto` olması bu sürümde mesaj göndermez.

## Kullanım ve cihaz kabulü

1. Mail ekranından **Dijital İkiz → Otonomi** sekmesini açın. İlk durumda motor,
   kanallar ve ses kapalıdır. Etkin bir API sağlayıcısından
   mail yanıt modeli seçimi **Ayarlar → Mail ve Otonomi** sekmesinden yapılır;
   Öğrenme sekmesindeki karar modeli yalnız 0–1 skoru içindir. Yanıt modeli
   seçilmeden diğer modele sessiz geçiş yapılmaz; web sohbetlerine arka plan çağrısı yoktur.
2. Otonom karar hazırlamayı ve denenecek kanalı açın. Ayarlar anında kaydedilir;
   çalışma saati/proje izinleri ve kanal ayarları birbirini silmez. Yenile ile son
   100 karar görülür. Mevcut eski mail kutusu taranmaz.
3. Yeni bir maille kayıtlı bir proje hakkında soru sorun. Proje inceleme izni
   açıksa yalnız kayıtlı proje kimliği salt-okunur yürütücüye verilir. Yanıt,
   kullanılan araçlar ve aynı kişinin giden mesajlarından çıkartılan üslup ile
   hazırlanır. Mail taslağı bağlı hesabın Drafts klasörüne yüklenir. Yükleme
   başarısızsa yerel taslak görünür ve başarısız olarak işaretlenir; SMTP çağrılmaz.
4. Gelecekteki bir `.ics` toplantısını mevcut bir etkinliğin üzerine gönderin.
   Öneri takvime kullanıcı incelemesi notuyla eklenir. Karar kartı çakışmayı ve
   en fazla iki alternatif zamanı gösterir. Taslak başka etkinliğin başlığını,
   yerini veya katılımcısını açıklamamalıdır. Takvime eklemek toplantıyı kabul
   etmek değildir; yanıt her zaman onay gerektirir.
5. Aynı soruyu CC ile veya bilinen bir WhatsApp kişisinin grubundan gönderin:
   otomatik aday yerine taslak beklenir. Bilinmeyen grup üyeleri kişi grafiğine
   otomatik eklenmez; grup geçmişi kişinin özel zaman çizelgesine kopyalanmaz.
   Grup taslağı araştırılırken kişinin birebir geçmişi de modele verilmez.
6. Bekleyen kayıtta **İptal** seçin; yenileme ve yeniden başlatmadan sonra iptal
   görünmelidir. Bu yalnız yerel kuyruğu iptal eder; sağlayıcıya önceden yüklenmiş
   Drafts kopyasını veya takvim önerisini silmez.
7. Ses seçeneğini açıp WhatsApp birebir/öz sohbet veya sahibinin Telegram botuna
   ses gönderin. Çalışan yerel Whisper gerekir; Opus/sıkıştırılmış ses ayrıca
   yerel `ffmpeg` gerektirir. Ses RAM'de çözülür, diske yazılmaz; hata durumunda
   bulut STT'ye geçilmez. Telegram'da sahip kontrolü indirmeden öncedir; üçüncü
   kişi bot erişimi açılmamıştır. Sahip mesajı mevcut normal sohbet yolunu kullanır.
8. Türkçe/İngilizce ve dar pencere görünümünü deneyin. Gerçek modelin yanıt kalitesi,
   gerçek IMAP Drafts teslimi ve gerçek Whisper konuşması cihaz kabulüne aittir.

## Güvenlik ve kalıcılık sözleşmesi

- Gelen Mail ve WhatsApp aynı `HandleIncomingEvent` motoruna girer. Telegram
  mevcut sahip botudur; dış kişiler adına araştırma yapan üçüncü bir kanal değildir.
- Önce etkin kanal, kalıcı olay rezervasyonu, kaynak tarihi (en çok 6 saat),
  enjeksiyon filtresi, otomatik gönderici başlıkları ve kişi kuralı değerlendirilir.
  Bilinmeyen tarih şimdiki zaman kabul edilmez; otomatik göndericilere taslak da
  üretilmez. İptal/yanıt `.ics` yöntemleri bu adımda otomatik takvim değişikliği yapmaz.
- Seçilen mail yanıt modeli sınıflandırma, en fazla üç salt-okunur araştırma
  isteği ve yanıt taslağını üretir; genel ajan döngüsü,
  terminal, dosya değiştirme, aktif sohbet/model değiştirme veya araç kaydı yoktur.
  Model çıktısı doğrudan bir işlem olarak çalıştırılmaz; her istek beyaz liste ve
  sabit gönderici kapsamından geçer. `search_memory`, yalnız gönderenin bağlı kişi
  geçmişini okur; genel kişisel RAG havuzu dış kişiye açılmaz.
- Takvim başlığı/konumu araştırma modeline verilmez. Alternatifler, yerel çalışma
  saatleri ve gerçek kesişim sorgularıyla en çok 72 saat içinde yarım saatlik
  adımlarla aranır. Takvim boşluğu müsaitlik garantisi değildir. Modelin metinden
  çıkardığı tarihlerin doğruluğu gerçek modelle kabul testi gerektirir.
- Genel varsayılan “Herkese otomatik yanıt”, “Herkese otomatik taslak” veya
  “Kişi bazında manuel seçim” olabilir. Yeni kişiler “Genel tercihi kullan” ile
  oluşur; mevcut açık kişi tercihleri korunur. Kişinin taslak/otomatik/asla tercihi
  genel varsayılandan önceliklidir. Manuel modda genel tercihi izleyen kişi için
  model çağrılmaz; kişi seçimi beklenir. Genel ayarı değiştirmek kanal anahtarını açmaz.
  Tam otomatik aday için etkin tercih full_auto, triage uygun, grup/CC olmayan,
  başarılı araç kanıtı bulunan salt bilgi sorusu ve bağımsız karar modelinin
  en az 0.95 skoru olmalıdır. Karar modelinden yalnız tek bir 0–1 sayısı istenir;
  üretici modelin kendi confidence alanı yetki vermez. Eksik/bozuk/NaN/sınır dışı
  skor taslağı silmez, kullanıcı onayında tutar. Sayısal 0 geçerli bir ölçümdür. Takvim,
  belirsizlik ve hata otomatik aday olmaz. Model güveni doğruluk garantisi değildir.
- Kişi ve mail dizisi rezervasyonları SQLite'da saklanır: saatte 1, 24 saatte 3.
  Başarısız/iptal edilmiş adaylar da bütçeye dahildir. Süreç yeniden başlatmak veya
  kanalı değiştirmek limiti sıfırlamaz.
- Olay kimlikleri ayrı `autonomy_events` tablosundadır: kişi geçmişinin daha önce
  kaydedilmiş olması motoru atlatmaz. `.ics` UID'si hesap kapsamında ayrıca
  rezerve edilir; başka mail UID'siyle gelen aynı davet tekrar eklenmez. Bir UID'nin
  sonradan güncellenmesi/iptali bu adımda işlenmez. Belirsiz kesinti/başarısızlık
  otomatik yeniden çalıştırılmaz; yarım kalan takvim/taslak elle incelenmelidir.
- Yanıt, uzak Drafts yüklemesinden önce DLP'den geçer; sır içeren gövde karar
  kartına konulmaz. Ambient olay akışına yalnız kayıt kimliği/durumu çıkar; gövde,
  alıcı ve konu admin-only `/api/twin/state` veya `/api/twin/activity` yanıtından okunur.
  Mail ekranı görünür ve uygulama aktifken activity üç saniyede bir yenilenir;
  arka planda durur. İnceleme/araştırma/yazma/skor/Drafts yükleme aşamaları model
  çağrısından önce kalıcı kaydedilir; hata/atlama ve kayıtlı taslak sonucu görünür.
  `preparing` son kayıtlı aşamadır; uygulama kesilmişse canlı çalışma garantisi değildir.
- Kanal ayarları `twin_preferences.autonomy` alanında; yanıt sağlayıcısı/modeli
  ayrı `twin_reply_model` tablosunda kalır. Kanal değişikliği yanıt modelini silmez.
  Admin-only `POST /api/twin/reply_model` (ve `/api/v1/...`) bu çifti atomik kaydeder;
  `state.reply_model` seçimi, karar kartındaki `decision_score` ölçülen skoru döndürür.
  Kayıtlar mevcut
  `pending_autonomous_actions` tablosunda, nedenler mevcut audit tablosunda kalır.
  Twin DB'nin mevcut yerel/E2E yedekleme yolu bunları da kapsar. Yeni admin-only
  işlem `POST /api/twin/cancel_action` (ve `/api/v1/...`) yalnız bekleyeni iptal eder.

## Doğrulama

Motor testleri gerçek geçici Git deposu, çakışan özel takvim, UID/olay tekrarı,
kalıcı kişi/dizi bütçesi, grup/CC, enjeksiyon, DLP, model/araç/yükleme hatasını kapsar.
Ses testi gerçek yerel ffmpeg ile Opus→PCM ve ağ oynatma listesi reddini doğrular;
Whisper modeli/ses tanıma kalitesini taklit ederek geçmiş sayılmaz. App entegrasyonu
seçilmiş HTTP karar sağlayıcısını; E2E testi gerçek App/HTTP ile ayar/kuyruk/iptali;
Flutter testleri TR/EN dar görünümü ve gerçek API istek gövdelerini doğrular.

Adım5 ilk doğrulaması: CGO/sqlite_fts5 build/vet exit0; Go race71 paket PASS, son tarih/grup
regresyonları ayrıca PASS; Flutter870/870 PASS; analyze yalnız mevcut3 info;
6 Dart dosyasında UI literal0; diff kontrolü PASS. Gerçek hesap/cihaz
kabulü beklenir; Adım6 bu çalışmada başlatılmaz.

Adım6 gönderim köprüsü, gönderim anında ana/kanal/kişi ayarlarını ve DLP
kontrolünü tekrar uygulamalı; geçmişte hazırlanmış adayı tek başına yetki saymamalıdır.

Grup araştırmasında birebir kişi geçmişi araç sonuçlarından çıkarılır. Fatura son
ödeme ve uçuş saatleri toplantı alternatifi gibi değiştirilmez; bugün vadesi dolan
fatura tarihi gece yarısı olsa da takvime alınır.

## 1 Kasım düzeltmesi — Yanıt yazarı ve karar skoru

Önceki sürüm karar modeline plan/taslak yazdırıyordu; kullanıcı talebiyle ayrıldı.
Ayarlar’daki **Mail ve Otonomi** sekmesinde sağlayıcı ve yanıt modeli seçilip
Kaydet’e basılır. Mevcut sohbet ve Öğrenme’deki karar modeli seçimi değişmez.
Karar modeli kapalıyken de taslak üretilir; otomatik aday için bağımsız skor gerekir.
Mail güvenlik incelemesi de risk için sayısal skor kullanır (0 zararsız, 1 saldırı);
0.5 ve üzeri engellenir. Mail semantik sınıflandırması yapılandırılmış yanıt yazarıyla
çalışır; skorerden kategori JSON’u veya yanıt metni istenmez. Bu değişiklik mail
akışına aittir; diğer özelliklerin mevcut karar protokolleri yeniden yazılmamıştır.

Düzeltmenin teknik doğrulaması: CGO/sqlite_fts5 build/vet exit0; tüm Go
race testleri 71 paket PASS; Flutter874/874 PASS; analyze yalnız mevcut3 context
info; değişen/yeni7 Dart dosyasında UI literal0; diff kontrolü PASS. Gerçek
skor/yanıt modeli ve hesap cihaz kabulü beklenir; otomatik gönderim hâlâ Adım6’dır.

## Genel tercih ve kişi istisnası kabul senaryosu

1. Mail → **Yanıt tercihleri ve işlemler** ile Otonomi sekmesine doğrudan girin.
   Motoru ve mail kanalını açın; Ayarlar → Mail ve Otonomi bölümünde yanıt yazarı seçin.
2. Genel tercihi herkese taslak, test kişisini **Genel tercihi kullan** yapın.
   Bu adrese yeni bir bilgi sorusu gönderin. Mail ekranında alıcıyla birlikte
   inceleme/yazma/kaydetme ve ardından taslak sonucu görünmelidir; Drafts'ı kontrol edin.
3. Genel tercih otomatik, test kişisi yalnız taslak: yine onay gerektiren taslak
   olmalıdır. Genel tercih taslak, kişi otomatik: yeterli kanıt ve bağımsız skor
   varsa otomatik aday olabilir; bu adımda gönderim yapılmaz.
4. Manuel mod + kişi genel tercihi kullan: kişi seçimi bekleniyor/atlandı görünmeli,
   modele çağrı olmamalı. Kişiye açık taslak tercihi verip YENİ mail gönderin;
   taslak hazırlanmalıdır. Önceki mail veya UID otomatik tekrar edilmez.
5. Eski kişide “asla” varsa genel seçimle kaldırılmaz; kişi istisnasını açıkça değiştirin.

## 1 Kasım — gerçek model ve hesap testi

- Paylaşılan kayıtta analiz çağrısı 90 saniyede kesilmiş; sidecar 499 dönmüş ve
  taslak yüklemesine geçilmemişti. Önceden bu hata yalnız `review_unavailable`
  görünüyordu. Artık zaman aşımı, iptal, sağlayıcı hatası ve okunamayan model
  çıktısı ayrı nedenlerle gösterilir; başarısız aşama da kaydedilir.
- Seçili gerçek model bağlantısında analiz 7,3 saniyede yanıt verdi; geçerli JSON
  bir Markdown `json` kod bloğundaydı. Motor artık tek dış kod bloğunu kaldırır.
  Açıklama içinden rastgele JSON seçmez; çoklu/trailing payload ve eksik blok
  reddedilir. Hem analiz hem yanıt çıktısı bu yoldan okunur.
- Kullanıcının gönderdiği **MEMO CANLI TEST** gerçek IMAP alımı → seçili yanıt
  modeli → `check_availability` → yanıt → gerçek Drafts yüklemesini tamamladı.
  Hazırlama yaklaşık23 saniye sürdü; `draft_saved=true`, `state=pending` doğrulandı.
- Bu testte bağımsız karar modeli kapalı/atanmamış olduğundan skor alınmadı ve
  taslak kullanıcı onayında kaldı. Genel otomatik gönderim hâlâ Adım6 kapsamındadır.
- Kullanıcının tek test yanıtı için açık onayından sonra gerçek `/api/mail/send`
  üzerinden, Türkçe görünen alıcı adıyla gönderim denendi: HTTP200 `{ok:true}`
  ve SMTP sunucu kabulü doğrulandı. Bu, alıcı kutusuna teslim teyidiyle aynı şey
  değildir; alıcı hesabındaki görünüm kullanıcıdan ayrıca soruldu ve kullanıcı
  “Evet, yanıt geldi” diyerek gerçek teslimi doğruladı.
- Teknik kontroller: CGO/sqlite_fts5 build/vet exit0, Go race71 paket PASS,
  Flutter880 PASS, analyze yalnız mevcut3 info, değişen Dart UI literal0, diff PASS.

Bu testte SMTP gönderimi ayrı manuel API yolundan yapıldı. Hazırlama kuyruğundaki
`a7324752…` kaydı hâlâ `pending`; gönderim köprüsü olmadığı için bu kayıt teslim
kaydı değildir. Aynı test taslağını tekrar göndermeyin. Genel otomatik teslim
ve hazırlama kaydını teslimle eşleyen onay köprüsü Adım6 olarak açık kalır.


## 1 Kasım — farklı gönderen kabulü

Kullanıcı farklı bir Gmail adresinden **MEMO FARKLI GÖNDEREN TESTİ** mailini
23:11:30+03:00 tarihinde gönderdi (UID17399). Yeni kişinin tercihi `default`,
genel mod `full_auto` olarak doğrulandı. Analiz/takvim sorgusu/yanıt hazırlama ve
gerçek Drafts yüklemesi yaklaşık22 saniyede tamamlandı: `reason=drafted`,
`draft_saved=true`, `state=pending`, `mode=awaiting_approval`.

Bu ikinci testte SMTP gönderilmedi. `score_status=unavailable`; bağımsız skor
alınamadığında onay gerektirme davranışı korundu. Sonuç, farklı gönderenin genel
tercihi devralıp otomatik taslak hazırlanmasının gerçek hesapta PASS olduğudur;
otomatik teslim PASS iddiası değildir. Kod değişmedi.
