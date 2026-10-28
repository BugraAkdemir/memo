# Telemetri — tutulan veriler ve kullanım amaçları

Bu belge mevcut istemcinin teknik veri envanteridir. Yasal aydınlatma metni değildir. Günlük rapor, kurulum sayımı ve silme istekleri aşağıda ayrı açıklanır.

## Gönderim ve varsayılanlar

Resmî derlemelerde günlük rapor varsayılan olarak açıktır. Bu, hem yeni kurulum hem daha önce telemetri bulunmayan bir sürümden güncelleme için geçerlidir; ayrı bir onay sorusu çıkmaz. Kurulum sihirbazındaki anahtar veya Ayarlar › Gizlilik üzerinden kapatılabilir. Önceden kaydedilmiş kapalı tercihi varsayılanla ezilmez. Kaynak derlemelerde kullanıcı açmadıkça günlük rapor kapalıdır ve kurulum sayımı gönderilmez.

Günlük rapor yalnız paylaşımın açıldığı UTC gününden sonraki tamamlanmış günleri kapsar. Etkinlik olmayan gün için rapor üretilmez. Başlangıçta 0–89 saniye beklenir, ardından yaklaşık üç saatte bir gönderim döngüsü çalışır. Çevrimdışı kalan günler en fazla sekiz gün geriye kadar denenir. Aynı günün raporu ağ hatasında yeniden denenebilir. Desteklenmeyen platform, sürüm biçimi veya okunamayan RAM bilgisi günlük rapor üretimini engeller.

Varsayılan adres `https://tel.memocpp.com` ve günlük rapor yolu `POST /v1/report` olur. `MEMO_TELEMETRY_URL` geliştirme/test için adresi değiştirir. JSON dışında istemcinin eklediği başlıklar `Content-Type: application/json` ve sabit `User-Agent: memo` değeridir.

## Hangi veri, hangi amaçla?

| Veri grubu | Tutulan/gönderilen değer | Amaç |
|---|---|---|
| Kurulum kimliği | Rastgele üretilen 128 bit `tid`; hesap veya donanım seri numarası değildir | Günlük raporları aynı kuruluma bağlamak, tekrarları ayırmak ve o kimlik için silme istemek |
| Sürüm ve platform | Rapor günü (UTC), şema, uygulama sürümü, derleme türü, işletim sistemi, mimari, arayüz dili | Sorunları sürüme/platforma göre ayırmak ve dil/platform desteğini planlamak |
| Donanım | GPU sınıfı, katalog GPU adı veya `other_<üretici>`, GB’ye yuvarlanan VRAM, motor türü, varsa çekirdek/iş parçacığı sayısı, standart sınıfa yuvarlanan RAM | Donanım desteği ve yerel motor uyumluluğunu değerlendirmek |
| Model kullanımı | Yerel/API/abonelik türü; sağlayıcı türü; katalog model adı veya `other`; çağrı kategorisi; tur, girdi/çıktı ve önbellek token toplamları | Kullanılan model/sağlayıcıları, iş yükünü ve önbellek kullanımını anlamak |
| Kullanım amacı | `text` ve `code` başına tur/token toplamı; çağrı kategorisinden türetilir, mesaj okunarak sınıflandırılmaz | Metin ve ajan/kod iş yükünün dağılımını görmek |
| Yerel hız | Model boyutu, motor sınıfı, token/s ve ilk token gecikmesi aralıkları ile örnek sayısı | Yavaş kalan donanım/model gruplarını belirlemek |
| Başlatma/çıkış | Bileşen, sabit hata sınıfı ve adet | Yerel motorun başlatılamadığı veya beklenmedik kapandığı durumları saymak |
| Sağlayıcı hataları | Sağlayıcı türü, `401`, `429`, `5xx`, `quota`, `timeout`, `network` sınıfları ve adet | Kimlik doğrulama, kota ve bağlantı sorunlarını ayırmak |
| Güncelleme | Sonuç/olay kodu ve adet | Güncellemelerin başarı, hata ve geri alma oranlarını görmek |
| Çökme | Hata türü, en çok beş Memo fonksiyonu/depo içi dosya/satır, bunların SHA-256 özeti ve adet | Aynı kod hatalarını gruplamak ve düzeltilecek yeri bulmak |
| Özellikler | Özellik adları; günün kullanımından, özellik sayaçlarından ve kullanım varsa mevcut etkin yapılandırmadan türetilir | Özelliklerin kullanımını ve açık olan yetenekleri değerlendirmek |
| İlk kullanım tarihleri | Varsa ilk kurulum, ilk sağlayıcı ekleme ve ilk sohbet turunun UTC günleri | Kurulumdan ilk kullanıma geçişi değerlendirmek |
| Hafıza boyutu | `0`, `1-100`, `100-1k`, `1k+` aralığı; içerik yok | Hafıza kapasitesini ve ölçek gereksinimini anlamak |

Kullanım kategorileri: `chat`, `agent`, `fact_extraction`, `dream`, `consolidation`, `memory_import`, `mood`, `title`, `learning`, `routine`, `proactive`, `insight`, `web_search`, `task_review`, `task_plan`, `task_step`, `task_subagent`, `compaction`.

Özellik listesi: `projects`, `subagents`, `skills`, `live_mode`, `whatsapp`, `telegram`, `memory`, `cloud_sync`, `webchat_beta`, `self_driving`, `image_gen`, `voice`. Etkin yapılandırmadan gelen bir ad, o özelliğin o gün kullanıldığının tek başına kanıtı değildir.

Hız örnekleri yalnız boyutu bilinen yerel modelin `chat`/`agent` çağrılarından, en az 20 çıktı tokenı ve geçerli hız/ilk token ölçümü varsa üretilir. Token/s aralıkları `<5`, `5-10`, `10-20`, `20-40`, `40-80`, `80+`; gecikme aralıkları `<0.5s`, `0.5-1s`, `1-2s`, `2-5s`, `5-10s`, `10s+`; model boyutu aralıkları `<2b`, `2-4b`, `4-8b`, `8-14b`, `14-32b`, `32b+` olur. Rapor 16.000 baytı veya 150 kullanım satırını aşarsa en az token içeren kullanım satırları azaltılır.

## Kurulum sayımı

Resmî derlemeler veri klasörü başına bir kez `POST /v1/install` gönderir. Gövde yalnız `schema`, `app_version`, `os`, `arch` alanlarını içerir; `tid` içermez. Amaç sürüm/platform bazında yaklaşık kurulum sayısını ölçmektir. Günlük rapor anahtarı kapalıyken de gönderilir. Eski sürümden güncelleme ve veri klasörünü silerek tekrar kurma sayımı artırabilir; bu kesin kişi/cihaz sayısı değildir. Başarılı veya gövdesi kesin reddedilmiş istek yerel `install_sent` alanıyla tamamlanmış sayılır; ağ/sunucu hatası daha sonra yeniden denenir. Kaynak derlemeler ve `MEMO_TELEMETRY=off` bunu göndermez.

## İçerik sınırları ve ağ bilgileri

Telemetri gövdesinde mesaj, cevap, sohbet başlığı, hafıza içeriği, kişi/takvim/WhatsApp/Telegram içeriği, resim/ses, dosya veya proje adı, kullanıcı dosya yolu, API anahtarı, parola, oturum belirteci, e-posta, kullanıcı/bilgisayar adı ve donanım seri numarası alanı yoktur. Kişinin verdiği sağlayıcı adı yerine sağlayıcı türü kullanılır. Model/GPU adları katalogla sınırlandırılır. Çökme mesajı, değişken/argüman değerleri, mutlak dosya yolu ve üçüncü taraf yığın çerçeveleri gönderilmez. Çökmedeki depo içi kaynak yolu bunun dışındadır.

IP adresi rapor JSON’una eklenmez; ancak HTTPS bağlantısı sırasında sunucu ve ağ altyapısı kaynak IP’yi görebilir. Bu istemci kodu tek başına sunucu/Cloudflare erişim günlüklerini, ülke türetimini veya saklama politikasını doğrulamaz. Rastgele kimlik günlük raporları birbirine bağlayabildiğinden, buradaki teknik açıklama “kimlikle ilişkilendirilemez” veya “sıfır kişisel veri” şeklinde mutlak bir hukuki iddia değildir.

Gizli sohbet kullanım kayıtları tutulmaz; Minimal Mod’a ait özel kullanım satırları günlük rapor kaynağından filtrelenir. Özellik sayaçları da bu modlarda atlanır. Bu, süreç genelindeki kurulum sayımının veya motor/güncelleme olaylarının tümünü kapatan bir anahtar değildir.

## Günlük raporun eksiksiz alan listesi

Aşağıdaki liste `Report` yapısının JSON alanlarıyla otomatik testte karşılaştırılır. İsteğe bağlı alanlar her raporda bulunmayabilir.

<!-- fields:begin -->
- `schema` — rapor biçiminin numarası
- `tid` — rastgele kurulum kimliği
- `day` — raporun kapsadığı UTC günü, saat yok
- `app_version` — Memo sürümü
- `build` — `official` ya da `source`
- `os` — linux, windows ya da darwin
- `arch` — amd64 ya da arm64
- `ui_lang` — arayüz dili
- `hw.gpu_class` — none, nvidia, amd, intel ya da apple
- `hw.gpu_model` — katalogdan bir ad ya da `other_<üretici>`
- `hw.vram_gb` — GB cinsinden video belleği
- `hw.engine` — yerel modelin kullandığı donanım yolu
- `hw.cpu_cores` — işlemci çekirdekleri
- `hw.cpu_threads` — işlemci iş parçacıkları
- `hw.ram_gb` — standart bir boyuta yuvarlanmış RAM
- `usage[].kind` — local, api ya da sub
- `usage[].provider` — sağlayıcı türü ya da abonelik üreticisi
- `usage[].model` — katalogdan bir ad ya da `other`
- `usage[].category` — çağrının ne için olduğu (chat, agent, title, …)
- `usage[].turns` — tur sayısı
- `usage[].in_tokens` — girdi token sayısı
- `usage[].out_tokens` — çıktı token sayısı
- `usage[].cached_tokens` — önbellekten gelen girdi token sayısı
- `purpose.{key}.turns` — amaç başına tur (metin ya da kod)
- `purpose.{key}.in_tokens` — amaç başına girdi token sayısı
- `purpose.{key}.out_tokens` — amaç başına çıktı token sayısı
- `perf[].size_bucket` — yerel model boyut aralığı
- `perf[].hw_class` — örneğin donanım sınıfı
- `perf[].tps_bucket` — saniyedeki token aralığı
- `perf[].ttft_bucket` — ilk token süresi aralığı
- `perf[].n` — o aralıktaki örnek sayısı
- `local_start_fail[].component` — llama, embedding ya da whisper
- `local_start_fail[].code` — permission, port, oom, missing_file, crash_at_load, timeout ya da unknown
- `local_start_fail[].n` — kaç kez
- `llama_exit[].component` — llama, embedding ya da whisper
- `llama_exit[].code` — oom_killed, signal, exit_nonzero ya da unknown
- `llama_exit[].n` — kaç kez
- `provider_err[].provider` — sağlayıcı türü
- `provider_err[].code` — 401, 429, 5xx, quota, timeout ya da network
- `provider_err[].n` — kaç kez
- `update[].event` — offered, downloaded, applied, rolled_back ya da failed
- `update[].code` — checksum, signature, needs_newer_app ya da not_for_this_machine
- `update[].n` — kaç kez
- `crash[].sig` — çerçevelerin özeti
- `crash[].component` — go, dart ya da llama
- `crash[].exc_type` — hata türü adı, mesajı asla
- `crash[].frames[]` — Memo’nun kendi en çok beş fonksiyon adı, depo içi dosya yolu ve satır numarası
- `crash[].n` — kaç kez
- `features[]` — kullanılan veya etkin yapılandırmadan eklenen özellikler
- `milestones.installed` — ilk çalıştırma günü
- `milestones.provider_added` — ilk sağlayıcının kaydedildiği gün
- `milestones.first_message` — ilk sohbet turunun günü
- `memory_rows_bucket` — hafıza deposunun kaba boyut aralığı
<!-- fields:end -->

Şema ile mevcut bağlantılar aynı kapsamda değildir: `offered`/`downloaded` güncelleme olayları, `needs_newer_app`/`not_for_this_machine` nedenleri, Whisper başlatma/çıkış olayları ve Dart/motor süreç çökmeleri henüz bağlı değildir. Çökme bildirimi yakalanan Go panic’lerini kapsar. Görsel/ses kullanımı `purpose` yerine `features[]` içinde temsil edilir.

## Yerel kayıt, önizleme, kapatma ve silme

`config.DataPath("telemetry_state.json")` ile açılan yerel durum dosyası rastgele kimliği, paylaşım tercihini/gününü, kurulum gününü, son işlenen günü, kurulum sayımı işaretini, ilk kullanım tarihlerini, günlük sayaçları ve bekleyen silme kimliklerini tutar. Dosya izni `0600` olur. Sayaç yazımında on günden eski günler budanır; uygulama kapalıyken bir silme zamanlayıcısı çalışmaz. Durum dosyası yedek kapsamına alınmaz. Günlük kullanım toplamları ayrıca yerel istatistik deposundan okunur; paylaşımı kapatmak bu ayrı yerel istatistik geçmişini silmez.

```bash
memo telemetry status
memo telemetry preview --raw
memo telemetry on
memo telemetry off
memo telemetry reset-id
memo telemetry delete
```

Önizleme ağ isteği yapmaz; günlük rapor için göndericiyle aynı JSON kodlayıcısını kullanır. Kurulum sayımı ve silme isteği günlük rapor önizlemesinin parçası değildir. Ayarlar › Gizlilik aynı kontrolleri sunar.

Paylaşımı kapatmak günlük raporu durdurur ve telemetri sayaçlarını temizler; mevcut sunucu verisinin silinmesini kendiliğinden istemez. Kimliği yenilemek eski `tid` için `POST /v1/forget` gövdesinde yalnız `tid` göndererek silme ister, yeni rastgele kimlik oluşturur ve yerel günlük sayaç/geçmişini sıfırlar. `delete` aynı silme isteğini yapar ve günlük paylaşımı kapatır. Çevrimdışıysa eski kimlik bekleyen listede kalır ve sonraki döngülerde yeniden denenir. Kimliksiz kurulum sayımı `tid` ile eşleştirilmediği için bu istekle o sayımdan bir kayıt çıkarılamaz.

`MEMO_TELEMETRY=off` (ayrıca `0`, `false`, `no`) telemetri sayaçlarını ve tüm telemetri ağ isteklerini, kurulum sayımı ve bekleyen silme isteği dahil, engeller. Bu ayar Memo’nun diğer ağ özelliklerini kapatmaz. Silme isteği bu sırada kuyruğa alınır; tekrar gönderilebilmesi için bu engelin kaldırılması gerekir.

## Sunucu tarafının doğrulama sınırı

Bu belge bu depodaki istemci davranışını ve gönderim biçimini açıklar. Canlı sunucunun saklama süresi, IP/erişim günlükleri, veritabanındaki kimlik dönüşümü, yedeklerden silme ve erişim yetkileri bu koddan doğrulanamaz; bunlar sunucu yapılandırmasıyla ayrıca doğrulanmalıdır. Sunucunun canlıya dağıtılmış olduğu burada varsayılmaz.
