# Dijital İkiz — Adım5: karar, araştırma ve takvim taslakları

Backend ve Flutter aynı değişiklikte bağlanır. Bu adım **hazırlama** motorudur;
gönderim zamanlayıcısı, WhatsApp/Telegram uzaktan onayı ve çok kanallı bildirim
köprüsü Adım6'dır. Kuyrukta `grace_period_auto` olması bu sürümde mesaj göndermez.

## Kullanım ve cihaz kabulü

1. Mail ekranından **Dijital İkiz → Otonomi** sekmesini açın. İlk durumda motor,
   kanallar ve ses kapalıdır. Öğrenme ayarlarında etkin bir API sağlayıcısından
   karar modeli seçilmelidir; web sohbet sağlayıcılarına arka plan çağrısı yapılmaz.
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
- Model önce en fazla üç salt-okunur araştırma isteği üretir; genel ajan döngüsü,
  terminal, dosya değiştirme, aktif sohbet/model değiştirme veya araç kaydı yoktur.
  Model çıktısı doğrudan bir işlem olarak çalıştırılmaz; her istek beyaz liste ve
  sabit gönderici kapsamından geçer. `search_memory`, yalnız gönderenin bağlı kişi
  geçmişini okur; genel kişisel RAG havuzu dış kişiye açılmaz.
- Takvim başlığı/konumu araştırma modeline verilmez. Alternatifler, yerel çalışma
  saatleri ve gerçek kesişim sorgularıyla en çok 72 saat içinde yarım saatlik
  adımlarla aranır. Takvim boşluğu müsaitlik garantisi değildir. Modelin metinden
  çıkardığı tarihlerin doğruluğu gerçek modelle kabul testi gerektirir.
- Her kişi başlangıçta yalnız taslaktır. Tam otomatik aday için kullanıcı o kişide
  “Otonomi modunu izle” seçmeli; genel mod full_auto, triage uygun, grup/CC olmayan,
  başarılı araç kanıtı bulunan yüksek güvenli salt bilgi sorusu olmalıdır. Takvim,
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
  alıcı ve konu admin-only `/api/twin/state` yanıtından okunur.
- Ayarlar mevcut `twin_preferences` kaydının `autonomy` alanında, kayıtlar mevcut
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

Doğrulama: CGO/sqlite_fts5 build/vet exit0; Go race71 paket PASS, son tarih/grup
regresyonları ayrıca PASS; Flutter870/870 PASS; analyze yalnız mevcut3 info;
6 Dart dosyasında UI literal0; diff kontrolü PASS. Gerçek hesap/cihaz
kabulü beklenir; Adım6 bu çalışmada başlatılmaz.

Adım6 gönderim köprüsü, gönderim anında ana/kanal/kişi ayarlarını ve DLP
kontrolünü tekrar uygulamalı; geçmişte hazırlanmış adayı tek başına yetki saymamalıdır.

Grup araştırmasında birebir kişi geçmişi araç sonuçlarından çıkarılır. Fatura son
ödeme ve uçuş saatleri toplantı alternatifi gibi değiştirilmez; bugün vadesi dolan
fatura tarihi gece yarısı olsa da takvime alınır.
