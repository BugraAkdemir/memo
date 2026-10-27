# Anonim günlük özet (telemetri)

Memo, insanların gerçekte hangi bilgisayarları ve modelleri kullandığını, neyin fazla yavaş kaldığını ve nelerin bozulduğunu öğrenebilmemiz için günde **bir küçük, anonim özet** gönderebilir. Resmî sürümlerde bu özet **varsayılan olarak açıktır ve istediğin zaman kapatabilirsin**: kurulum sihirbazında ya da Ayarlar › Gizlilik’te. Kapalıyken hiçbir şey sayılmaz ve özet gönderilmez. Ne seçersen seç bir şey daha gider: tek seferlik bir kurulum sayımı (aşağıya bak).

Bu sayfa sözleşmenin tamamıdır: ne sorulur, ne gönderilir, ne asla gönderilmez; ve nasıl bakılır, kapatılır, silinir. Aşağıdaki alan listesi koda karşı bir testle denetlenir, yani koddan ayrışamaz.

## Seçim

- **Resmî sürümler varsayılan olarak paylaşır.** Yeni bir kurulumda sihirbazda zaten açık bir anahtar görünür; yanında tam olarak ne gönderildiğini ve ne gönderilmediğini gösteren bir bağlantı vardır. Orada kapatırsan hiçbir şey sayılmaz ya da gönderilmez. Sonradan ayrı bir soru çıkmaz.
- **Telemetrisi olmayan bir sürümden güncelleme** de aynı şekilde ele alınır: güncellemeden sonra günlük özet **soru sorulmadan ve haber verilmeden** açıktır. Ayarlar › Gizlilik’ten kapatabilirsin.
- **Varsayılan ne olursa olsun, ilk rapor yürürlüğe girdiği günden sonraki ilk tam gün içindir.** Varsayılanın başladığı gün ve öncesi hiçbir zaman gönderilmez.
- **Kaynak koddan derlenen sürümler kapalı kalır**, sen açmadıkça (açılırsa `build: "source"` bildirirler, böylece topluluk derlemeleri resmî sayılara karışmaz) ve kurulum sayımını da göndermezler.
- Ortamdaki **`MEMO_TELEMETRY=off`** her cevabı geçersiz kılar: hiçbir şey sayılmaz, gönderilmez (kurulum sayımı dahil), ağ isteği yapılmaz.

İstediğin zaman **Ayarlar › Gizlilik**’ten ya da terminalden değiştirebilirsin:

```bash
memo telemetry status            # ne durumda
memo telemetry preview [--raw]   # bir sonraki özetin tam içeriği
memo telemetry on                # günlük anonim özeti paylaş
memo telemetry off               # paylaşma
memo telemetry reset-id          # yeni rastgele kimlik (sunucudan eskisini unutması istenir)
memo telemetry delete            # sunucudaki verini sil ve paylaşımı kapat
```

## İstediğin zaman bak

`memo telemetry preview --raw` (ve *Ayarlar › Gizlilik › İçeriği göster*) **gönderilecek baytların tamamını** yazdırır. Önizleme ve gönderici aynı fonksiyonu çağırır; okuduğun şey çıkan şeydir. Bakmak hiçbir şey göndermez.

## Ne zaman ve nasıl gönderilir

- **Tamamlanmış her UTC günü için en çok bir rapor**, yalnız o günü kapsar. Bugünü asla, evet dediğin günü ve öncesini de asla.
- Yalnız gün gönderilir, günün saati asla.
- Teslim edilemeyen rapor en çok **8 gün geriye** kadar yeniden denenir; daha eski günler bırakılır.
- `tel.memocpp.com` adresine HTTPS ile. Sunucu yalnız sabit, doğrulanmış bir biçimi kabul eder, gerisini reddeder.
- Sunucu rastgele kimliğinin kendisini değil, **anahtarlı bir özetini** saklar. IP adresin hiçbir zaman okunmaz, kaydedilmez, saklanmaz; yalnız Cloudflare kenarının eklediği ülke kullanılır.


## Kurulum sayımı (özet kapalıyken de gider)

Resmî bir sürüm ilk kez çalıştığında **tek bir** istek, `POST /v1/install`, gönderir: tam dört değer ve **kurulum kimliği yok** — `schema`, `app_version`, `os` ve `arch`. Sunucu bunu günlük bir sayaca ekler; kimlik, ülke ya da isteğe dair başka hiçbir şey saklamaz, bu yüzden sayım bir cihaza ya da günlük özete bağlanamaz. **Veri klasörü başına bir kez** gider, özete *hayır* dediysen de, böylece "kaç kurulum var" sorusu cevaplanabilir. Özet kapalıyken dışarı çıkan tek şey budur.

Dürüst olmak gereken üç sınır: bu yaklaşık bir sayıdır, cihaz sayısı değildir (veri klasörünü silip yeniden kuran tekrar sayılır ve biri onu şişirebilir); Memo, durum dosyası olmayan eski bir sürümden güncellemeyi sıfırdan kurulumdan ayıramaz, ikisi de kurulum sayılır; ve yalnızca `MEMO_TELEMETRY=off` onu durdurur, özeti kapatmak durdurmaz. Kaynak koddan derlenen sürümler bunu hiç göndermez.

## Asla gönderilmeyenler

- Mesajların, cevapların ve sohbet başlıklarının içeriği
- Anılar, kişiler, takvim, WhatsApp ve Telegram mesajları
- Dosya adları, klasör yolları, proje adları
- API anahtarları, parolalar, belirteçler, oturumlar
- IP adresin
- Kullanıcı adın, bilgisayar adın, e-postan
- Günün saati (yalnız gün, UTC)
- Ekran kartı sürücü sürümü, seri numaraları, işlemci model adı
- Kendi yaptığın modellerin adları (bilinen bir listede olmayan her şey `other` olur)
- Çökme hata mesajları, değişken değerleri ve Memo’ya ait olmayan kod
- **Gizli Sohbet (Incognito) ve Minimal Mod konuşmaları ve onlardan türeyen her sayı**

## Neler gönderilir

Her alan bir sayı, kaba bir sınıf ya da sabit bir listeden seçilmiş bir addır. Serbest metin tutabilecek hiçbir alan yoktur.

- **Kimlik ve zaman.** Rastgele bir kurulum kimliği, gün, Memo sürümü, resmî mi kaynak mı, işletim sistemi, mimari, arayüz dili. Aynı cihazın iki kez sayılmaması ve sorunların sürüme bağlanabilmesi için.
- **Donanım (kaba).** Ekran kartı sınıfı ve bilinen bir katalogdan ad (başka her şey `other_<üretici>`), video belleği, yerel modelin çalıştığı motor, işlemci çekirdek ve iş parçacığı sayısı, standart bir boyuta yuvarlanmış RAM.
- **Kullanım sayıları.** Gün başına: tür (yerel, kendi API anahtarın, abonelik), sağlayıcı türü, model (katalogdan, yoksa `other`) ve amaç başına tur ve token sayıları. Yalnız sayılar, mesajlar asla.
- **Hız.** Yerel modeller için: her (model boyutu, donanım, saniyedeki token, ilk token süresi) aralığına kaç örnek düştüğü.
- **Hatalar.** Yerel motor başlatma hataları ve beklenmedik çıkışlar sabit bir neden koduyla; sağlayıcı hataları sınıfıyla (`401`, `429`, `5xx`, `quota`, `timeout`, `network`); güncelleme sonuçları (uygulandı, başarısız, geri alındı; neden olarak sağlama toplamı ya da imza).
- **Çökmeler.** Hatanın *türü* ve Memo’nun kendi en çok beş fonksiyon adı ile bunların özeti. Hata mesajı asla gönderilmez.
- **Özellikler ve kilometre taşları.** O gün hangi özelliklerin kullanıldığı ve üç ilkin tarihi (kurulum, ilk sağlayıcı, ilk mesaj).

### Tüm alanlar

<!-- fields:begin -->
- `schema` — rapor biçiminin numarası
- `tid` — rastgele kurulum kimliği (sunucu yalnız anahtarlı özetini saklar)
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
- `crash[].frames[]` — Memo’nun kendi en çok beş fonksiyon adı
- `crash[].n` — kaç kez
- `features[]` — o gün kullanılan özellikler
- `milestones.installed` — ilk çalıştırma günü
- `milestones.provider_added` — ilk sağlayıcının kaydedildiği gün
- `milestones.first_message` — ilk sohbet turunun günü
- `memory_rows_bucket` — hafıza deposunun kaba boyut aralığı
<!-- fields:end -->

Biçimin izin verdiği her şey henüz gönderilmiyor. Bu sürüm `offered` ve `downloaded` güncelleme olaylarını ve `needs_newer_app` ile `not_for_this_machine` nedenlerini göndermez, `whisper` hatalarını bildirmez ve yalnız Memo arka ucunun (`go`) çökmelerini bildirir; uygulama penceresininkileri ve motor sürecininkileri bildirmez. Görsel ve ses kullanımı `purpose` içinde değil `features[]` içinde görünür. Sakin bir gün (hiç etkinlik yok) hiçbir şey göndermez.

## Sıfırla ve sil

- **Kimliği yenile** bu kuruluma yeni bir rastgele kimlik ve temiz bir geçmiş verir, sunucudan eskisini unutmasını ister. Çevrimdışıysan istek saklanır ve yeniden denenir.
- **Sunucudaki verilerimi sil** sunucudan bu kuruluma ait her şeyin silinmesini ister ve paylaşımı kapatır. Yerel durum dosyası yedeklerine girmez.

## Nerede durur

Yerel durum (cevabın, rastgele kimlik, son günlerin sayaçları) Memo’nun veri klasöründe yalnız sahibinin okuyabildiği tek bir dosyadır: `telemetry_state.json`. Sayaçlar yalnız paylaşım açıkken tutulur. Aynı dosya kurulum sayımının gönderildiğini de hatırlar, böylece tekrar gönderilmez.

Sunucu tarafı ayrı, açık bir projedir: yalnız yazılabilen bir uç nokta (`tel.memocpp.com`) ve yalnız okunabilen bir yönetim paneli. Veritabanı şeması ve nelerin saklandığı için o deponun belgelerine bak.
