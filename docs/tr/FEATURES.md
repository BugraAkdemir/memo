# Memo — Kapsamlı Özellik Kataloğu

Bu döküman, **Memo Yapay Zeka Hafıza Kabuğu** içine entegre edilmiş her özelliğin detaylı dökümünü sunar. Mimari veri sürekliliğinden duyusal multimodaliteye kadar, Memo'nun yerel yapay zeka deneyiminizi nasıl güçlendirdiğini keşfedin.

---

## 1. 🧠 Temel Zeka ve Hafıza

### Kalıcı RAG (Geri Getirme Destekli Üretim)
Memo sadece bir sohbet değil, bir "İkinci Beyin"dir.
- **Anlamsal İndeksleme**: Her etkileşim otomatik olarak vektörleştirilir ve yerel bir vektör veritabanında saklanır.
- **Hibrit Arama**: Geri getirme, vektör benzerliğini FTS5 anahtar kelime aramasıyla birleştirir (Reciprocal Rank Fusion ile birleştirilir) — böylece kısa, kesin bir gerçek sadece "yeterince yakın" anlamsal mesafeye bağlı kalmaz.
- **Çok-Konulu Soru Bölme**: Çok konulu bir soru ("adım, doğum günüm ve favori rengim ne") bağlaçlara göre bölünür, her konu tek bir harmanlanmış embedding'e sıkışmak yerine kendi aramasını alır.
- **Bağlamsal Hatırlama**: Her yanıttan önce Memo, en ilgili geçmiş konuşmaları geri getirmek için bu hibrit aramayı yapar (Top-K eşleşmesi).
- **Sabitlenmiş Gerçekler (2026-07-15)**: Kalıcı kişisel gerçekler (isim, doğum günü, evcil hayvan vb.) — ister `/remember` ile kaydedilsin ister normal sohbetten otomatik tespit edilsin — her promptta koşulsuz olarak enjekte edilir, arama sıralamasını tamamen atlar, böylece rutin sohbet arasında asla kaybolmaz.
- **Sonsuz Bağlam**: Uzun süreli hafıza, yapay zekanın haftalar veya aylar önceki detayları, mevcut modelin pencere sınırından bağımsız olarak hatırlamasını sağlar.

### Hafıza Ayarları — Memo'nun neyi hatırladığını gör, düzenle, sil (v4.6.0)
Ayarlar › Hafıza, bir pill çubuğuyla değiştirilen beş odaklı bölümdür (her pill canlı bir sayı taşır): **Ayarlar** (getirme seçenekleri ve hafıza dosyaları), **Bilinen Bilgiler**, **Sohbet Geçmişi**, **Analiz** ve **Hata Ayıklama**.
- **Bilinen Bilgiler** — Memo'nun senin hakkında tuttuğu her sabitlenmiş bilgi, her zaman görünür. Birini yerinde düzenle ya da bir ya da birkaçını onay kutularıyla ve tümünü seç / seçimi kaldır çubuğuyla sil.
- **Sohbet Geçmişi** — sıradan (sabitlenmemiş) sohbet hafızaları, sayfalı, aynı seçim araçlarıyla. Sabitlenmiş bilgiler ve sohbet geçmişi tamamen birbirinden bağımsız yönetilir: birini temizle, ötekini koru.
- Her silme — tek satır ya da bir seçim — önce onay ister ve kaç kaydı sildiğini bildirir. Silme kesin kayıt kimliğiyledir, asla bir desenle değil.
- **Dream** (eski, ilişkili sabitlenmiş bilgileri daha temiz özetlere sıkıştıran arka plan geçişi) artık Memo'yla birlikte başlıyor; öncesinde taze bir açılışta, alakasız bir eylem onu uyandırana kadar uyuyordu.
- "Bunu hatırla", gömme modeli olmayan yalnızca-bulut kurulum dahil her kurulumda çalışır.

### Modelden Bağımsız Motor
- **Dahili Llama-Server**: Yüksek performanslı GGUF çıkarımı için `llama.cpp` tarafından desteklenir.
- **Özel Embedding Sunucusu**: Hafıza indeksleme için özel çalışan ikinci bir dahili sunucu, ana sohbet performansını hiç etkilemez.
- **Çapraz-Mod Mimarisi**: Sohbet için harici API sağlayıcılarını (OpenAI, Claude, Gemini) kullanırken, embedding'leri küçük yerel bir model bağımsız olarak halleder.
- **Harici Sağlayıcı Desteği**: LM-Studio veya herhangi bir OpenAI uyumlu yerel API'ye (Port 1234/8081) sorunsuz bağlanır.

### Uzaktan Erişim ve Self-Hosting
- **Dört Kimlik Doğrulama Modu**: `none` (açıkça opt-in, yüksek sesle uyarılır), `token` (cihaz-başı token, varsayılan), `password` (kullanıcı adı + argon2id-hash'li parola, kısa ömürlü imzalı oturum) veya `token+password` (ikisinden biri yeterli — OR mantığı). Sunucu-başı Ayarlar → Uzaktan Erişim'den ya da `memo remote set-mode` CLI komutuyla seçilir.
- **Cihaz-Başı Tokenlar**: Eşleştirilen her cihaz (telefon, laptop, ikinci masaüstü) kendi tokenını alır, oluşturulurken bir kez gösterilir ve sadece hash'lenmiş olarak saklanır — diğerlerini döndürmeden tek bir cihazı iptal edebilirsiniz. Ayarlar'dan ya da SSH üzerinden `memo remote list-devices`/`add-device`/`revoke-device` ile yönetilir.
- **Kaba Kuvvet Koruması**: Parola-modu girişleri, genel API rate limiter'dan bağımsız olarak sınırlandırılır — birkaç ücretsiz deneme, ardından üstel geri çekilme.
- **ngrok Tüneli**: Memo backend'inize her yerden erişmek için gömülü ngrok entegrasyonu. Otomatik indirme, tünel yönetimi, yapılandırılabilir domain ve bölge.
- **Tailscale (Beta'dan çıktı)**: Tek tıkla giriş (yapıştırılacak auth key yok), varsayılan olarak açık Funnel, kopan bağlantıdan sonra otomatik yeniden bağlanma — doğrudan Ayarlar → Uzaktan Erişim'de, masaüstü ve mobilde.
- **Self-Hosted Sunucu Modu**: Sadece headless backend'i — masaüstü uygulaması olmadan — bir Raspberry Pi, ev sunucusu veya VPS üzerinde, tamamen SSH üzerinden `memo` CLI'siyle (systemd --user servisi için `memo service install`, config.yaml için `memo config get/set`, kimlik doğrulama/cihazlar için `memo remote`) yönetilebilir şekilde çalıştırın. Native yükleyici (`get-memo-server.sh`) veya çoklu-mimari (amd64+arm64) Docker/CasaOS imajı. Bkz. [Self-Hosting](SELF_HOSTED.md).
- **Çoklu Hesap, Admin/Kullanıcı Rolleri**: Paylaşılan bir self-hosted sunucu birden fazla girişi barındırabilir — bir admin artı istediği kadar kullanıcı hesabı, her biri kendi parolasıyla. Ayarlar → Hesaplar'dan ya da SSH üzerinden `memo remote list-accounts`/`add-account`/`delete-account` ile yönetilir.
- **Ayrıntılı Hesap-Başı İzinler**: Hesap başına yedi bağımsız anahtar (Modeller, Hafıza, Ajan, Takvim, WhatsApp, Telegram, Rutinler) — bir admin, örneğin, bir kullanıcının sohbet edip Ajan araçlarını kullanmasına izin verirken Model Mağazası/API Sağlayıcıları sekmelerini gizleyip hafıza yazmayı tamamen engelleyebilir. Backend'de zorunlu kılınır (sadece UI'da gizlenmez), Ayarlar → Hesaplar'da onay kutulu arayüz.

---

## 2. 🏛️ Mimari ve Veri Sürekliliği

### SQLite + sqlite-vec Kalıcılığı
- **Birleşik Depolama**: Vektör embedding'leri ve metadata aynı SQLite veritabanında yaşar.
- **ANN İndeksleme**: `vec0` sanal tablosu O(log N) sorgu süresiyle yaklaşık en-yakın-komşu aramasını sağlar.
- **ACID Uyumluluğu**: Yerleşik transaction desteği atomik yazmaları ve veri bütünlüğünü garanti eder.
- **Go Yedek Mekanizması**: vec0 eklentisi kullanılamıyorsa, kaba-kuvvet kosinüs benzerliği yedeğine düşer.

### Gizlilik ve Yerel İzolasyon
- **%100 Çevrimdışı**: Hiçbir veri bilgisayarınızdan dışarı çıkmaz. Telemetri yok, log gönderimi yok, bulut bağımlılığı yok.
- **Şifrelenmiş Yerel Depolama**: Zihniniz kendi donanımınızda kalır.
- **AES-256-GCM Şifreleme**: API anahtarları makine-türetilmiş bir anahtarla şifrelenir.
- **Görseller diskte şifreli (v4.6.0)**: Memo'ya gönderdiğin (`data/images`) ve Memo'nun çizdiği (`data/generated-images`) her görsel, dosya başına yeni bir anahtarla (ana sırdan ve rastgele tuzdan HKDF-SHA256, rastgele 192-bit nonce, kimlik doğrulamalı) XChaCha20-Poly1305 ile mühürlenir, kip 0600. Ana sır bilerek veri klasöründe **değildir**: varsayılan olarak `~/.config/Memo/image.key` (`%AppData%\Memo`, `~/Library/Application Support/Memo`) — veri klasörünü kopyalamak, yedeklemek ya da senkronlamak anahtarı da taşımaz; `MEMO_IMAGE_KEY` dosyanın yerine dışarıdan verilen bir parola koyar (scrypt ile güçlendirilir). `MEMO_DATA_DIR` ile çalışma (konteyner birimi, taşınabilir, testler) anahtarı veriyle birlikte tutar; atılan bir yapılandırma klasörü tüm görselleri kilitleyemesin diye. v4.6.0 öncesi görseller açılışta bir kez mühürlenir; uygulama bunları çözerek sunar, arayüzde bir şey değişmez. **Doğası gereği kapsam dışı:** bellekte açıkken görsel ve bir model sağlayıcısına, Telegram'a ya da WhatsApp'a gönderildiğinde onların aldığı kopya. Yedekler görsel klasörlerini (ve görsel anahtarını) içermez.

### Yedekleme & Geri Yükleme (.memo)
- **Tam Dışa Aktarma**: `GET /api/export` — sohbetler, config, sağlayıcılar, orchestra, hafıza, WhatsApp verisi, **artı takvim olayları, öğrenilmiş alışkanlıklar, rutinler, görev listeleri, ajan araç izinleri, kurulu skill'ler ve `machine.key`** (önceden eksikti — `machine.key` olmadan, geri yüklenen bir yedek her sağlayıcı API anahtarını kalıcı olarak çözülemez bırakıyordu) içeren bir zip arşivi. Modeller varsayılan olarak hariç (kendi anahtarı var).
- **Tam İçe Aktarma**: `POST /api/import` — .memo zip'inden geri yükleme. Opsiyonel model dahil etme.
- **Tüm Veriyi Sil**: `POST /api/wipe` — çift onay dialog'u, config dosyası kalıcı kalır. Artık dosyaları silmeden önce her dahili veritabanını (hafıza, istatistik, takvim, mood, WhatsApp) güvenilir şekilde kapatıyor, sadece Windows'ta yaşanan bir hatayı düzeltti.

### Telefonda Memo
- **Tek istemci, mobil için derlenmiş**: masaüstüyle aynı Flutter uygulaması (`frontend/`), Android ve iOS hedefleriyle — ayrı, daha küçük bir companion projesi değil. Yerini aldığı bağımsız `mobile/` istemcisi 2026-09'da emekliye ayrıldı.
- **Telefonda sıfır işlem yükü**: tüm yapay zeka Memo'yu çalıştıran makinede kalır; telefon LAN, ngrok ya da Tailscale üzerinden bağlanan bir uzak görüntüleyicidir.
- **Yapısal özellik paritesi**: sohbet, ajan modu, takvim, rutinler, model deposu ve ayarlar masaüstü ekranlarının kendisidir — yeniden yazılmadı, dar ekran için yerleştirildi.
- **OS seviyesi takvim hatırlatıcıları**: işletim sistemine zamanlanır, böylece uygulama kapalı ve telefon o günden beri backend'i hiç duymamış olsa bile ateşlenir.
- **Sesli giriş**: telefon kaydeder, backend yazıya çevirir. TTS yanıtları native bir ses eklentisiyle çalınır.
- **Telefonda olmayanlar**: Live Mode'un native realtime motorları (ayrık transcribe/synthesize döngüsüne düşer), masaüstü maskotu, sistem tepsisi ve CLI kurulumu.

---

## 3. 🏭 Model Yönetimi (Fabrika)

### Entegre Hugging Face Arama
- **Doğrudan Repository Erişimi**: Uygulama içinden Hugging Face'te model arayın.
- **Repo ID Desteği**: Herhangi bir Hugging Face GGUF repo ID'sini yapıştırarak mevcut dosyaları anında getirin.

### Sistem Tanılaması
- **VRAM ve GPU Kontrolü**: Mevcut NVIDIA/AMD VRAM'in otomatik tespiti.
- **Uyumluluk Rozeti**: Modelleri indirmeden önce "GPU Uyumlu" olarak işaretler veya "Yetersiz VRAM" uyarısı verir.

### Arkaplan İndirme Yöneticisi
- **Paralel İndirme**: Artık aynı anda birden fazla GGUF indirmesi çalışabiliyor (önceden ikinci bir indirme doğrudan reddediliyordu), motor durum çubuğunda birleşik ilerleme ve kaba bir süre tahminiyle.
- **Yaşam Döngüsü Kontrolü**: Tüm yerel modeller için tek tıkla Başlat, Durdur ve Güncelle.
- **Donanıma Uygun İlk-Çalıştırma Önerisi**: Kurulum RAM/GPU'nuzu okur ve eşleşen bir sohbet + hafıza model çiftini önerir, ikisini de indirmeye başlatan tek bir düğmeyle.
- **Daha Güvenli Bağlam Boyutlandırma**: Bağlam-boyutu alanı artık modelin gerçek maksimum bağlamını doğrudan GGUF dosyasından okur ve slider'ın bunu aşmasına izin vermez.
- **Doğru Yetenek Rozetleri**: Tool-calling/kod rozetleri artık sabit kodlanmış bir "bilinen" aile listesi yerine modelin gerçek chat template'inden ve etiketlerinden türetiliyor.
- **Sade-Dilli Hatalar ve Araç İpuçları**: `llama: server failed to become ready within 120s` gibi ham hatalar artık kısa ve eyleme geçirilebilir; donanım-uyumu ve kuantizasyon rozetlerinin ne anlama geldiğini açıklayan hover tooltip'leri var.
- **Discover Filtreleri**: Tools/Vision/Code/Embedding/Size filtreleri artık AND değil OR ile birleşiyor ve "N filtre aktif · temizle" göstergeli çoklu-seçim açılır menülerinde gruplanıyor.

---

## 4. ⚡ Etkileşim ve Kullanıcı Deneyimi

### Streaming Yanıtlar
- **Token-Token Render**: Yapay zekanın yanıtını gerçek zamanlı "yazmasını" izleyin.
- **Düşünme Durumu**: İlk token gelmeden önce görsel geri bildirim sağlayan nabız atan bir "Memo düşünüyor..." durumu.
- **İmleç Arayüzü**: Stream'i takip eden yanıp sönen terminal-tarzı bir imleç (`▊`).
- **Asla donmuş görünmez (v4.6.0)**: bir tur sürerken altta, geçerli evreyi ve her saniye ilerleyen bir süreyi gösteren bir ilerleme satırı durur ("Model düşünüyor · 0:45") ve bir araç bittikten sonra da hareket etmeye devam eder. 30 saniye boyunca hiçbir şey gelmezse turuncu bir "sunucudan haber yok" uyarısına döner. Backend, sessiz kaldığı her 10 saniyede bir `heartbeat` parçası gönderir; böylece uzun, sessiz bir düşünme evresi kopmuş bağlantı sanılmaz.
- **Uzunlukla değil sessizlikle biter (v4.6.0)**: düz bir sohbet cevabı yalnızca hiçbir şey gelmeden 300 saniye geçince (ilk kelimeyi beklemek dahil) ya da 30 dakikalık bir güvenlik sınırında kesilir — hâlâ akan uzun cevapları kesip "durduruldu" diye kaydeden sabit 300 sn toplam sınırıyla değil. Zaman aşımı olarak işaretlenir ve o ana kadar gelen metin saklanır. Ajan turları kendi 1200 sn bütçesini korur.
- **Bağlam halkası (v4.6.0)**: sohbetin altındaki motor şeridinde bir halka, modelin bağlam penceresinin ne kadar dolu olduğunu gösterir (sıkıştırma noktasına yaklaştıkça turuncu, sonra kırmızı); tıklayınca kullanılan / pencere, parçalı bir çubuk, mesajlar, özetlenmiş eski konuşma, sistem istemi, hafıza, skill'ler, araç tanımları ve yeni mesaj için birer satır, boş alan ve otomatik sıkıştırma payı — ve bir Abonelikler modelinde hesabın kullanım sayaçları (Codex/Claude için 5 saatlik oturum ve haftalık, Antigravity için modelin kendi rakamı) yenilenme süresiyle birlikte — içeren bir pencere açılır. Boyut, sağlayıcı bildirdiyse onun istem + cevap token sayısıdır (parçalar ona oranlanır), yoksa `~` tahmini. `GET /api/context`.
- **Her sağlayıcıda %90'da otomatik sıkıştırma (v4.6.0)**: tüm istem — sistem istemi, araç şeması, geçmiş ve yeni mesaj — modelin gerçek penceresinin `CompactThresholdPct` (varsayılan 90; kayıtlı 60 taşınır) oranına ulaşınca ya da sağlayıcının önceki turdaki kendi sayısı zaten oradaysa, geçmişin en eski ~%60'ı tek bir özet mesaja sıkıştırılır, kalanı olduğu gibi durur. Yerel, API ve Abonelikler modellerinde aynı kural (Minimal Mod atlar: Memo'nun kendi model çağrısı yapmama sözü var). Ajan araç şeması artık API turlarında da geçmişin payından düşülüyor.
- **Okunabilir sağlayıcı hataları (v4.6.0)**: başarısız bir istek, `⚠️ [custom] status 400: … request ID …` yerine arayüz dilinde bir cümleyle ve küçük bir HTTP etiketiyle anlatılır — güvenlik reddi, biten kullanım hakkı, hız sınırı, yanlış/süresi dolmuş anahtar (Abonelikler: "yeniden giriş yap"), yasak, bilinmeyen model, fazla uzun konuşma, zaman aşımı, çöken sağlayıcı, ağ yok, diğer retler. Bir hatanın insanın okuduğu şeye dönüştüğü yerde uygulanır (SSE parçası ve kaydedilen mesaj); böylece kota kartı, görev döngüsü ve yeniden denemeler ham metni görmeye devam eder; ham metin günlüğe yazılır.
- **Hâlâ çalışan diğer sohbetler** kenar çubuğunda aynı küçük döneni (ve bir "az önce bitti" işaretini) gösterir — arka plan görevleri, WhatsApp/Telegram cevapları ve ikinci bir tarayıcı sekmesi dahil.
- **Sohbetteki görseller** sunucuda saklanır ve sunucu üzerinden yüklenir; yalnızca sunucunun yanındaki masaüstü uygulamada değil, web sürümünde ve telefonlarda da görünür.

### Live Mode v2 — Native Sesten-Sese Konuşma
- Sohbet giriş kutusunun yanında küçük bir ses ikonu — ayrı bir kenar çubuğu sekmesi değil. **Google Live** ya da **OpenAI Realtime** üzerinden gerçek native sesten-sese konuşma — transkribe-sonra-TTS akışı değil.
- **Devretme (delegate) ya da bağımsız modlar**: Live Mode'u kendi başına bir konuşma olarak çalıştırın, ya da mevcut bir sohbete devredin ki ikisi hafıza/bağlamı paylaşsın.
- **Tek yönlü barge-in**: Memo konuşurken tekrar konuşmaya başlayın, üstünüze konuşmak yerine dinlemek için durur.
- **Oturum-ortası hafıza yenileme**: Hafıza bağlamı uzun bir konuşma sırasında yeniden çekilir, sadece başta değil — böylece Memo, konuşmanın ortasında bahsettiğiniz bir şeyi hatırlayabilir.
- **Daha net hatalar**: Gerçek bir ses motoru başlatamayan bir oturum artık neden başlatamadığını söylüyor (motor seçilmemiş, motor yapılandırması eksik, arkaplan sohbet oturumu açılamıyor) — sessizce kendi sesini yankı olarak duymaya düşmek yerine.
- **Yerel yedek yol**: Hiçbir native motor yapılandırılmadığında, cihaz-üstü whisper.cpp transkripsiyonu + yerel **Piper** TTS uçtan uca çalışmaya devam ediyor, çevrimdışı bir ses seçici (Türkçe/İngilizce) ve seçilebilir ElevenLabs/özel motorlarla birlikte.
- **Bilinen kısıtlama**: Henüz yankı iptali yok — hoparlörler (kulaklık yerine) Memo'nun zaman zaman kendi sesini bir kesinti sanmasına yol açabilir.

### @ Dosya-Anma
- Herhangi bir sohbetin mesaj kutusunda `@` yazarak isme göre bir dosya arayıp referans verin — ajan modunu tam yolu yazmadan belirli bir dosyaya yönlendirmek için kullanışlı.

### Masaüstü Maskotu
- Masaüstünüzde kendi her-zaman-üstte penceresi olarak yaşayan küçük animasyonlu bir karakter — sohbet penceresinden ayrı, ama aynı çalışan uygulama/süreç/durumu paylaşıyor (ikinci bir binary değil).
- **Memo'nun gerçekte ne yaptığını canlı gösterir**: her ajan araç çağrısı, düz sohbet yanıtı ya da görev-döngüsü turu — hangi kanaldan gelirse gelsin (sohbet, WhatsApp, Telegram, bir görev listesi) — maskotun yokladığı tek bir uygulama-geneli aktivite sinyalini besler; pozu gerçek zamanlı değişir (düşünüyor, yazıyor, belirli bir tool çalıştırıyor).
- Karakterin altında **sade-dilli bir durum balonu**: "Düşünüyor…", "Yazıyor…", "search_web çalıştırılıyor…" — asla modelin gerçek yanıt metni değil. Bir tur bittiğinde ayrı bir "Tamamlandı!" anı (kollar havada, birkaç saniye) yaşanır, sonra boşta durumuna döner.
- **Boştayken de canlı**: göz kırpar, nefes alır ve donuk durmak yerine ara sıra rastgele bir sallanma/zıplama/salınım yapar.
- **İki seçilebilir cilt** (Ayarlar → Genel, canlı önizlemeyle): orijinal elle-çizilmiş yaratık, ya da lacivert piksel-art bir robot — aynı animasyon sistemi, aynı ruh halleri.
- **Live Mode'da da konuşur**: konuşma zamanlamasıyla ağız açılıp kapanır, bir ses-dalgası efekti ve "Konuşuyor…" balonu (Live Mode'un sadece konuşma durumu yansıtılıyor — ne Google Live ne de OpenAI Realtime istemcisi şu an gerçek bir "dinliyor" sinyali bildirmiyor).
- Gerçekten her-zaman-üstte (Wayland'da bile, XWayland'a zorlanarak), karakterin gerçek silüetine göre şekillendirilmiş bir tıklama/sürükleme alanıyla (bounding box'ı değil).

### Gizli Mod (Incognito)
- **Sıfır Kalıcılık**: Hassas oturumlar için tüm hafıza kaydını ve geçmiş loglamasını devre dışı bırakan güvenli bir anahtar.
- **Geçici Bağlam**: Bağlam sadece o belirli oturum içinde var olur ve kapanınca silinir.

### Performans HUD'u
- **Gerçek-Zamanlı İstatistikler**: Üretim hızı (tok/s), toplam token ve kesin süre metriklerini görmek için zaman damgasının üzerine gelin.

### WhatsApp Entegrasyonu
- **QR Eşleştirme**: Uygulama içinde gösterilen QR kod üzerinden tam WhatsApp Web çoklu-cihaz eşleştirmesi.
- **Çift Yönlü Mesajlaşma**: Kişi-farkında görünümle mesaj gönder ve al.
- **Kişi Çözümleme**: Rehber senkronizasyonu, push isimleri, telefon numarasına düşme.
- **İzinli Dosya Transferi**: Güvenilir kişiler izinli dizinlerden dosya isteyebilir.
- **Ajan Araçları**: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`.
- **Özel Sohbet Modu**: Sadece-WhatsApp etkileşimleri için izole executor ve tool registry.
- **Kendine-Sohbet Asistanı**: Kendi WhatsApp numaranıza (QR girişi için eşleştirilen numara) mesaj atın ve Memo tam bir asistan olarak yanıt versin — sohbet, hafıza ve ajan araçlarının hepsi Memo'yu açmadan telefonunuzun WhatsApp uygulamasından erişilebilir.
- **Sohbetten Rutinler**: Sade dilde isteyin ("her sabah 8'de bana hava durumunu hatırlat") ve Memo, uygulama içinde de mevcut olan aynı `create_routine`/`list_routines`/`cancel_routine` ajan araçlarını kullanarak konuşmadan doğrudan bir rutin oluşturur, listeler ya da iptal eder.
- **`/auto-perm`**: O konuşma için tool-çağrısı izin sorularını otomatik-izne çeviren bir kendine-sohbet slash komutu, böylece sohbetten tetiklenen rutin/ajan eylemleri gelmeyecek bir masaüstü tıklamasını bekleyerek takılmaz.
- **Yerel Depolama**: Tüm WhatsApp mesajları izole bir SQLite veritabanında saklanır.
- **Modeller, görseller ve okunabilir hatalar (v4.6.0)**: kendine-sohbet, Telegram botuyla aynı `/model` değiştiriciye, fotoğraf anlama ve düzenlemeye, görsel üretmeye (`/image`) ve sade dilde hata mesajlarına sahip — aşağıya bak. Açıklamasız gönderilen görsel eskiden atlanıyordu; artık asistana ulaşır.

### Telegram Entegrasyonu
- **Bot Eşleştirme**: Ayarlar → Telegram'da bir Telegram bot token'ı (`@BotFather`'dan) bağlayın; yapılandırıldıktan sonra Memo Bot API'yi long-poll eder.
- **Sahip Kilidi**: Bir bot'un kullanıcı adını bulan herkes ona mesaj atabildiğinden, Memo ilk mesaj atanı bot'un kalıcı sahibi olarak kilitler ve sonrasında herkes başkasını sessizce yok sayar — bu entegrasyonun tüm erişim-kontrolü sınırı budur.
- **Asistan Sohbeti**: Bağlandıktan sonra, sahip WhatsApp kendine-sohbet yoluyla aynı yeteneğe sahip tam bir asistan alır — sohbet, hafıza ve ajan araçları — bu sefer Telegram üzerinde.
- **Sohbetten Rutinler**: Aynı `create_routine`/`list_routines`/`cancel_routine` tool akışı bir Telegram konuşmasından da çalışır.
- **Yerel Depolama**: Telegram mesajları, WhatsApp'ınkinden bağımsız kendi izole SQLite veritabanında saklanır.
- **Modelleri sohbetten değiştir (v4.6.0)**: `/model` bir mesaj uzağındaki her modeli listeler — her açık sağlayıcının modeli, Abonelik hesaplarının canlı modelleri sağlayıcıya göre gruplu, yerel model — numaralı; aktif olan ✅, görsel modelleri 🎨 ile işaretli. `/model 3`, `/model gemini` ya da `/model <sağlayıcı> <model>` ile değiştirilir. `/status` artık yalnızca sağlayıcıyı değil modeli de söyler.
- **Görseller iki yönde (v4.6.0)**: bir fotoğraf gönder, asistan ne olduğunu anlatır; altına bir değişiklik yaz ("bunu siyah beyaz yap", "make it anime") düzenlenir (görselden görsele); sözle bir görsel iste ("bana bir kedi resmi çiz", "draw me a cat") çizilir (metinden görsele) ve fotoğraf olarak geri gelir. Tur o tek mesaj için kendiliğinden bir görsel modeline gider, sonraki mesaj yine sıradan sohbettir (bkz. Harici Sağlayıcılar altında *Otomatik görsel yönlendirme*). `/image <ne çizilsin>` ifade nasıl olursa olsun görsel zorlar; görsel modeli yoksa bunu söyler.
- **Okunabilir hatalar (v4.6.0)**: başarısız bir istek sohbete ne olduğunu ve ne yapılacağını anlatan bir cümle olarak gelir ("sağlayıcı çok fazla istek aldı …", "model ya da adres bulunamadı …"), HTTP durumu etiketiyle — sağlayıcının ham `status 400 … request id …` dökümü değil.

---

## 5. 🔌 Harici Sağlayıcı Desteği

### Çoklu-Sağlayıcı Mimarisi
Memo, yerel modellerin yanı sıra harici LLM API'lerine bağlanır:
- **Desteklenen Sağlayıcılar (16 `ProviderType` değeri):** OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Groq, Ollama, gömülü `llama.cpp`, genel bir **Custom** (herhangi bir OpenAI-uyumlu endpoint), **Custom (Anthropic-uyumlu)** (herhangi bir Anthropic Messages API-şekilli endpoint — ör. kendi proxy'niz), **OpenCode Zen** (kullandıkça öde, bazı modeller ücretsiz), **OpenCode Go** (abonelik), **Kilo Code** (app.kilo.ai — kullandıkça öde, bazı modeller ücretsiz), **Cline** (api.cline.bot, v4.6.0'da yeni — OpenAI-uyumlu; OpenRouter, Kilo ve OpenCode Zen gibi model başına ücretsiz/ücretli katalogla) ve iki CLI sağlayıcısı (Claude Code, Codex). Gateway-tarzı sağlayıcılar, bir model adını elle yazmak yerine canlı bir model listesinden seçmenize izin verir, ücretsiz modeller yeşil tikle üstte sıralanır.
- **Sağlayıcı Ekle, maliyet etrafında düzenlendi (v4.6.0):** gerçek bir ücretsiz katmanı olan sağlayıcılar yeşil bir rozet taşır ve ücretlilerden ayrı kendi grubunda durur. Gerçek bir ücretsiz-model kataloğu olan dört sağlayıcıda (OpenRouter, Kilo Code, OpenCode Zen, Cline) **Ücretsiz model seç** düğmesi tek tıkla çalışan bir 0 TL'lik modeli seçer.
- **Abonelikler (v4.6.0)** — ayrı bir sağlayıcı türü değil, *Subscriptions* adlı önceden yapılandırılmış bir `custom` sağlayıcı (`internal/app/subs.go`): Ayarlar › Abonelikler'den bir **Antigravity, Claude ya da Codex** hesabıyla bir kez giriş yap; hesabın sunduğu her model sohbetin model seçicisinde, `/model`'de ve yerel `/v1` ağ geçidinde (`subs/<model>`) seçilebilir, API anahtarı yok. İşi Memo'nun içine gömülü **CLIProxyAPI** (MIT) yapar ve çalışma zamanında asla indirilmez: sabitlenmiş sürümü ve SHA-256'ları `internal/cliproxy/PINNED.txt`'te durur, derlemede ve her başlatmadan önce yeniden doğrulanır; yalnızca loopback'te, rastgele bir anahtarla dinler. Sağlayıcıların kendi OAuth istemcilerini sürekli bozduğu Beta `gemini-sub` / `claude-sub` sağlayıcılarının yerini alır. Sağlayıcılar bu hesaplarda üçüncü taraf istemcilere izin vermeyebilir — sayfa bunu söyler.
  - **Model seçici** (`widgets/model_picker.dart`): yüksekliği sınırlı, kendi içinde kayan; yedi satırdan fazlada arama, katlanabilir sağlayıcı bölümleri, aile logoları ve okunur adlar ("Gemini 3.7 Flash" + bir "High" etiketi) olan bir panel; saklanan ham kimliktir. Seçilen model yeniden başlatmadan sonra da kalır.
  - **Kalan hak**: model başına yüzde rozeti ve yenilenmeye kalan süre. Antigravity bunu model başına bildirir; Codex ve Claude hesabı pencerelerle ölçer (Claude 5 sa / 7 gün) ve en sıkışık pencere o sağlayıcının tüm modellerine uygulanır. Kota hiçbir isteği engellemez — rakamlar bir önbellekten gelir ve arka planda yenilenir.
  - **Kullanım sınırı kartı**: hak bittiği için bir tur ölürse sohbet geri sayımlı bir kart gösterir; "sıfırlanınca otomatik devam et" kutusu (varsayılan açık, cihaz başına hatırlanır) ve "Şimdi devam et" düğmesi; yenilenme anında Memo senin yerine `continue` yazar. Hakkın %10 ve altı kaldığı her pencerede bir kez `quota_low` uyarısı çıkar. Memo açıkken çalışır; kart kalıcı değildir ve CLI ajan sohbetleri, WhatsApp ve görev döngüsü kapsam dışıdır (döngü kendi zamanlayıcısıyla yeniden dener).
  - **Prompt önbelleği ve pencere (canlı ölçüldü, 2026-10-08)**: yardımcı üzerinden Antigravity'deki Claude, tekrarlanan bir önekte 4952 istem token'ının 4942'sini önbellekten verdi; Codex birebir tekrarda 4470'in 3840'ını önbellekten verdi ama yalnızca öneki paylaşan bir istek için 0 bildirdi — isabetleri düzensiz, yok değil. Memo istem önekini bayt bayt sabit tutar (zaman bloğu ve çalışma-kümesi özeti yeni mesajın üstüne biner) ve hesabın bildirdiğini Kullanım İstatistikleri'ne yazar. Yardımcı **hiçbir model için bağlam uzunluğu yayınlamaz** (ne `/v1/models` ne de Gemini biçimli katalog taşır); bu yüzden Memo'nun bir Abonelikler modeli için penceresi pratik bir kuraldır: `claude-*` 200K (temkinli), `gemini*` 1M, geri kalan — `gpt-*`/Codex modelleri ve `gpt-oss` — 128K; sağlayıcıda ayarlanmış bir bağlam boyutu her zaman bu kuraldan önce gelir. Bu, Memo'nun aşmayacağı bir bütçedir, modelin ölçülmüş sınırı değil.
  - **Görseller**: görüntü modelleri kimliğine bakılarak değil, uç noktaya sorularak bulunur — metinden resim için `POST /images/generations`, bir resim eklediğinde `/images/edits`.
- **Otomatik görsel yönlendirme (v4.6.0)**: herhangi bir sohbetten — uygulama, terminal, Telegram ya da WhatsApp — bir görsel iste, o tek mesaj bir görsel modeline gider; sohbetin kendi modeli asla değiştirilmez ve sonraki mesajı yine o yanıtlar. Söz denetimi (Türkçe ve İngilizce) bilerek temkinlidir: kod, tablo, grafik, nasıl-yapılır soruları ve yapıştırılmış belgeler metin modelinde kalır; hakkında soru sorulan fotoğraf düzenlenmez, anlatılır. **Abonelik hesabında görseli o hesabın kendi görsel modeli çizer** — önce kullanılan modelin sağlayıcısı (Codex sohbeti Codex'inkiyle, Antigravity sohbeti Antigravity'ninkiyle) — varsayılanı asla. **Diğer tüm sağlayıcılar** Ayarlar › API Sağlayıcıları › Görsel üretimi'nde seçilen *varsayılan görsel modelini* kullanır (sağlayıcı + model; ikisi birden ya da hiçbiri). Aynı kart aç/kapa anahtarını da taşır (kapalıyken görsel modeli yalnızca `/image` ile ya da elle seçilince kullanılır). Gözetimsiz işler (görev döngüsü, Code Mode) asla yönlendirilmez. Kaynak: `config.yaml` `image:` ve `GET/PUT /api/image/config`.
- **Görseli tam pencerede aç (v4.6.0)**: sohbetteki bir görsele (ya da büyüt düğmesine) dokun; düğmelerle, fare tekerleğiyle, çimdikle, çift dokunuşla ya da `+` / `-` / `0` ile yakınlaştır, yakınken kaydır, ayrıntılarını gör (çözünürlük, en-boy oranı, dosyadan okunan biçim, boyut) ve masaüstünde, mobilde ve webde platformun kaydetme penceresiyle indir.
- **Claude ve Gemini artık gerçek tool-calling destekliyor** (önceden ikisinde de tamamen eksikti — herhangi birinde bir ajan/görev-döngüsü turu sessizce tool kullanamıyordu). İkisi de tekli ve paralel tool çağrılarını her sağlayıcının kendi wire formatına göre doğru round-trip ediyor.
- **Claude Code / Codex CLI sohbet sağlayıcısı olarak (beta):** bir API çağrısı yerine, Memo yerel kurulu bir `claude`/`codex` CLI'sini alt-süreç olarak çalıştırır. Sohbet-başına (uygulama-geneli değil), gerçek zaman-sınırsız bir arkaplan işi olarak çalışır, CLI'nin kendi prompt'suz izin modunu kullanır, ve CLI'nin kendi `/` slash komutları Memo'nun komut popup'ında görünür. Hafıza/kimlik bağlamı gönderilmez — CLI kendi oturumunu yönetir.
- **Sağlayıcı Arayüzü:** `ChatCompletion`, `ChatCompletionStream`, `ListModels` ile ortak `Provider` arayüzü.
- **Yedek Zincir:** Router sağlayıcıları sırayla dener; 3 ardışık hatadan sonra otomatik devre dışı bırakır; sağlık kontrolü iyileşince yeniden etkinleştirir.

### Şifrelenmiş Anahtar Yönetimi
- **AES-256-GCM Şifreleme:** API anahtarları makine-türetilmiş bir anahtarla (`/etc/machine-id`) şifrelenir.
- **Anahtar Depolama:** Şifrelenmiş anahtar değerleriyle `data/providers.json`.
- **Bağlantıyı Test Et:** Kaydetmeden önce bağlantıyı doğrulayan yerleşik test düğmesi.

### Frontend Sağlayıcı Arayüzü
- **API Sağlayıcıları Sekmesi:** Sağlayıcı ekleme/düzenleme için Ayarlar sekmesi.
- **Yapılandırma Dialog'u:** Sağlayıcı tipi seçici, API anahtarı girişi (maskeli), base URL, model açılır listesi.
- **Aktif Sağlayıcı Seçimi:** Sohbet için hangi sağlayıcının aktif olduğunu seçin.
- **OpenRouter'daki görüntü-çıkışlı modeller (v4.6.0):** yalnızca resim üreten bir model OpenRouter'ın görüntü uç noktasına yönlendirilir ve resim cevap olarak gelir (öncesinde her tur 404 alıp "tüm sağlayıcılar başarısız" oluyordu). Hangi modellerin görüntü modeli olduğu OpenRouter'ın kendi kataloğundan okunur; metin gereken arka plan işleri (sohbet başlıkları, hafıza çıkarımı) onları hiç çağırmaz.
- **Geçersiz anahtar artık "bağlı" görünmez** ve tek aktif sağlayıcı üç hatadan sonra 5 dakika kilitlenmez — geri çekilme yalnızca geri dönülecek başka bir sağlayıcı olduğunda anlamlıdır.
- **Reddedilen örnekleme parametreleri geri çekilir:** Claude Opus 4.7+ `temperature` ve benzerlerini reddeder; Memo reddi bir kez fark eder ve o sağlayıcı için göndermeyi bırakır. Aynı dene-ve-kilitle valfi, `stream_options.include_usage` ve `cache_control` alanlarını bilmeyen uç noktalar için de çalışır.

---

## 6. 🧠 Ajan Modu (Tool Calling)

### Tool Çalıştırma Motoru
Memo, tam bilgisayar kontrolüne sahip bir yapay zeka ajanı olarak davranır:
- **Ana registry'de 40 Yerleşik Tool** (`internal/agent/tools.go`'daki `NewRegistry()` sayılarak doğrulandı — önceki "27"den yukarı): dosya G/Ç (`read_file`, `write_file`, `edit_file`, `insert_line`, `delete_lines`, `delete_file`, `list_directory`, `get_file_info`, `search_files`, `change_directory`), `run_command`, `read_env`, `web_search`, `fetch_page`, `self_clone`, `configure_provider`, `get_calendar_events`, görev-döngüsü kontrolü (`get_task_status`, `pause_task`, `resume_task`, `create_task_md`, `edit_task_md`, `start_self_driving_task` — aşağıda §6.5'e bakın), rutinler (`create_routine`, `list_routines`, `cancel_routine`), `share_file`, `save_code_plan` (aşağıda Code Mode'a bakın), `open_app` ve yedi `browser_*` tool'u (sonraki bölüme bakın) ve WhatsApp'ın dördü (`whatsapp_send`/`search`/`latest`/`messages`; ayrıca ayrı, kapsamlı bir registry'de de bulunurlar).
- **Skill tool'ları artık gerçekten çalışıyor.** Bir skill'in `SKILL.md`'si bir `command:` alanı tanımlayabilir, yerleşik tool'larla tamamen aynı tool pipeline'ına ve izin-sorusu arayüzüne bağlanır — önceden bu sadece bir bildirimdi ve hiçbir şey çalıştırmıyordu.
- **Skill'ler sohbet başına aktiftir (v4.6.0).** Bir skill'i açmak yalnızca onu açtığın sohbeti etkiler; her yeni sohbet hiçbiri olmadan başlar (öncesinde etkinleştirme geneldi ve geçmişte içe aktarılmış skill'ler fark edilmeden çalışmaya devam edebiliyordu). Ayarlar › Skill'ler yalnızca listeler, kurar ve kaldırır; açıp kapatma sohbetin içinde yapılır.
- **İçe aktarılan skill'ler artık kendi kendine aktive olmuyor (v4.5.0 güvenlik düzeltmesi).** Memo hâlâ başka araçların skill klasörlerinden (ör. Claude Code'unkinden) otomatik olarak skill'leri alıyor — ama yeni keşfedilen bir skill artık bulunduğu anda sistem-promptu yetkisi kazanmak yerine sizin açmanızı bekliyor.
- **Tool Registry:** JSON Schema parametre tanımlarıyla thread-safe registry.
- **Tehlike Seviyesi Sistemi:** `safe` (otomatik izinli), `medium` (kullanıcıya sor), `dangerous` (sor + gecikme).

### İzin Sistemi
- **6 Politika Tipi:** PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever.
- **Oturum Kalıcılığı:** İzinler `data/permissions.json`'da saklanır.
- **Argüman Hash'leme:** İzin eşleştirme için SHA-256 hash'leme.

### Güvenlik Sandbox'ı
- **Path Traversal Koruması:** Symlink çözümlemesi, `..` engelleme, proje kökü sınırlaması.
- **Komut Kara Listesi:** 43 tehlikeli desen engellenir (`rm -rf /`, `sudo`, fork bomb'ları, vb.).
- **Rate Limiting:** Dakikada 30 tool çağrısı, komut başına 5sn bekleme.

### Ajan Pipeline'ı
- **LLM ↔ Tool Döngüsü:** Kullanıcı mesajını + tool tanımlarını LLM'e gönderir, tool çağrılarını çalıştırır, sonuçları geri besler, nihai yanıta kadar döngü (`pipeline.go`'nun `maxIters`'ına karşı doğrulanmış, en fazla 40 iterasyon).
- **Event Streaming:** Tool çalıştırma olayları SSE üzerinden frontend'e akıtılır.
- **Denetim Kaydı:** Son 1000 tool çalıştırması zaman damgasıyla loglanır.

> **Not:** Ajan frontend arayüzü (izin dialogları, tool çağrı kartları, mod anahtarı) bir süredir yayında ve tamamen çalışıyor — anahtar doğrudan Sohbet'in üst çubuğunda web-arama anahtarının yanında, ayrı bir Ajan-özel ekrana gerek yok.

### Uygulama açma ve tarayıcı sürme (v4.6.0'da yeni)
- **`open_app`** — "Spotify'ı aç", "Steam'i başlat", "tarayıcıyı aç" adı verilen masaüstü uygulamasını (ya da boş sekmeli varsayılan tarayıcıyı) Windows, macOS ve Linux'ta başlatır. Gerçek bir yan etkidir, bu yüzden izin sisteminden *orta* seviyede geçer; bir kabuk komutunun bir kademe altında. Linux'ta modelin verdiği ad bir görünen addır, bu yüzden exec edilmek yerine Desktop Entry kayıt defteri üzerinden çözülür (Flatpak/Snap uygulamaları dahil). Tool açıklaması, "en son haberler ne" gibi bir soruda tetiklenmesin diye yazılmıştır — o bir web aramasıdır.
- **Canlı tarayıcı paneli** — sohbetin yanındaki bir panel gerçek ama yalıtılmış bir Chromium sekmesi gösterir (`internal/browserengine/`, kendi profili; kendi tarayıcına ya da hesaplarına asla dokunmaz). Memo onu `browser_navigate`, `browser_click`, `browser_type`, `browser_scroll`, `browser_screenshot`, `browser_get_text` ve `browser_close` ile sürer; panel sayfayı değiştiren her adımdan sonra yeniden çizilir. Sen de Ajan Modu kapalıyken sürebilirsin: bir adres yaz (`https://` gerekmez), ekran görüntüsüne tıklayarak sayfada aynı noktaya tıkla, alttaki satırdan odaklanan alana yaz. Sürükleyerek yeniden boyutlandır; telefonda tam genişlik.
- **Tıklamalar gerçek seçiciler kullanır**: `browser_get_text` önce tıklanabilir her öğeyi çalışacağı garanti bir seçiciyle listeler (devasa bir sayfa istemi boğmasın diye sınırlı), tıklama tahmin edilmiş bir CSS seçicisi yerine bunlardan birini kullanır.
- **Ekran görüntüleri sohbet geçmişine asla girmez** — yalnızca panele akar (`browser_frame` parçaları), bu yüzden token harcamaz; bir sayfanın ne dediği metin yolundan okunur. Düz bir "bu siteyi özetle" yine hızlı sayfa-getirme yolundan gider.
- **Yapısı gereği güvenli**: yalnızca `http`, `https` ve boş sayfa kabul edilir (`file://` yok) ve oturum uç noktaları ajan izni ister.

### Code Mode: Plan / Auto / Build (v4.5.0'da yeni)
Code Mode eskiden tek bir aç/kapa anahtarıydı. Artık mesaj kutusunda **Ctrl+Tab** ile (ya da alt durum çubuğundaki çip'e dokunarak) döngülenen, her biri kendi düzenlenebilir sistem promptuna sahip (Ayarlar) üç preset:
- **Plan** — kod tabanını inceler ve tek bir dosyaya bile dokunmadan somut, adım-adım bir plan yazar. Plan diske kaydedilir (`data/plans/<proje>/plan.md`, özel `save_code_plan` aracıyla — proje dizininin dışında sandbox'lanmış), ve hazır olduğunda Memo düz sohbette Build'e mi yoksa Auto'ya mı geçilsin diye sorar.
- **Auto** — bugünkü bildik Code Mode: dosya düzenlemeleri hâlâ hızlı bir onaydan geçer.
- **Build** — hızlı şerit: dosya düzenlemeleri ve `run_command` çağrıları beklemeden çalışır.
- Bir plan bittiğinde global auto-permission anahtarı zaten açıksa, Memo soruyu atlar ve **aynı yanıt içinde** (tek SSE stream'i, ikinci bir round-trip yok) doğrudan Build'e zincirler.

---

## 6.5 🚗 Self-Driving Görev Döngüsü (v4.4.0, alt-mod zincirlemesi v4.5.0'da eklendi)

Ajan Modu üzerine inşa edilmiş, gözetimsiz, çok-adımlı bir görev çalıştırıcı — ona bir kontrol listesi verirsiniz, kendi başına üzerinde çalışır.

- **`Task.md` şeması.** Modu (`worker` ya da `planlayıcı`), bildirim ayrıntısını, rol-başı model sabitlemesini, hafızayı, sağlayıcı kilidi/roaming'i (`# sağlayıcı: sabit|otomatik|<isim>`) ve plan otomatik-onayını kontrol eden opsiyonel `# anahtar: değer` başlıklarıyla düz-Markdown bir kontrol listesi (`- [ ]` maddeleri). `create_task_md`/`edit_task_md` araçlarıyla oluşturulur/düzenlenir, ya da mevcut bir dosyadan `start_self_driving_task` ile başlatılır (Tasks sekmesinden bir `Task.md` yoluna işaret ederek de erişilebilir).
- **Planlayıcı/uygulayıcı modu.** `# mod: planlayıcı` için, bir planlama turu önce kullanıcının onayladığı bir `Plan.md` (adımlar, kabul kontrolleri, bir bağımlılık DAG'ı) üretir — ya Tasks sekmesinin plan-onay kartından, ya da `# onay: otomatik` ile otomatik olarak.
- **Alt-ajan orkestrasyonu.** Büyük ya da açıkça paralelleştirilebilir bir madde en fazla 3 alt-ajana bölünebilir: tam olarak bir yazma-yetkili `coder` önce çalışır, sonra en fazla 3 salt-okunur `analyzer`/`reviewer`/`test-runner` alt-ajanı gerçek paralellikte çalışır ve sonuçları bir şef incelemesini besler.
- **Görev kartında canlı aktivite.** Tool çağrıları, alt-ajan turları (`[coder]`/`[analyzer]`/…), uzun sessiz LLM çağrıları sırasında "model üretiyor" ve yavaş-tool "başlıyor…" satırları, döngü çalışırken canlı bir uygulama-içi karta akar.
- **Sessiz başarısızlık değil, dayanıklılık.** Meşgul bir sohbet görevi anında öldürmek yerine kuyruğa alır ve tekrar dener; rate-limit'li bir sağlayıcı bekler ve aynı maddeden devam eder, listeyi hiç yeniden başlatmaz; geçici bir hata, madde kullanıcı için beklemeye alınmadan önce artan bir tekrar denemesi alır (5 sonra 10 dakika); bir kimlik doğrulama/yapılandırma hatası, sonsuza kadar döngüye girmek yerine listeyi kullanıcı-bekleniyor durumunda bekletir. Her terminal durum bildirim gönderir (sohbet mesajı + push), tasarım gereği.
- **Sohbetten duraklat/devam ettir.** `pause_task`/`resume_task`, modelin kendisinin çalışan bir görevi duraklatmasına ve aynı adımdan devam ettirmesine izin verir, duraklatılmışken kullanıcının yazdığı her şeyi taşıyarak.
- **Bilinen açık boşluklar** (bkz. `BUG_REPORT.md`, `BUG-PLAN9`/`10`/`11`/`12`): plan onayı şimdilik sadece Tasks sekmesinden (sohbet içinde değil), sohbet modeli henüz *çalışan* bir görevin canlı durumunu okuyamıyor, ve bir eskalasyon bir adımı böldüğünde adım/madde sayaçları ekranlar arasında farklılaşabiliyor.

---

## 7. 🎵 Orchestra Modu (Çoklu-Model Orkestrasyon)

### Konsept
Birden fazla yapay zeka modeli bir takım olarak işbirliği yapar:
1. **Şef Model** kullanıcı isteğini analiz eder, alt görevlere böler.
2. **Uzman Roller** görevleri paralel çalıştırır (frontend, backend, bug_fixer, vb.).
3. **Şef Model** sonuçları tek, tutarlı bir yanıtta sentezler.

### Yerleşik Roller
| Rol | Varsayılan Model | Amaç |
|------|-------------|---------|
| Planlayıcı | Claude | Yazılım mimarisi, görev ayrıştırma |
| Frontend | Grok | UI geliştirme |
| Backend | GPT-4o | API/sunucu mantığı |
| Bug Fixer | Gemini | Hata ayıklama, kök neden analizi |
| Reviewer | Claude | Kod kalitesi incelemesi |
| Security | GPT-4o | Güvenlik denetimi |
| DevOps | Grok | Altyapı/deploy |
| General | GPT-4o | Genel amaçlı yedek |

### Çalıştırma Modeli
- **Paralel Görevler:** Bağımsız görevler eş zamanlı çalışır (goroutine'ler + WaitGroup).
- **Sıralı Görevler:** `depends_on` alanıyla bağımlılık çözümü.
- **Tekrar Deneme:** Rate-limit farkında üstel geri çekilmeli tekrar deneme (en fazla 3 deneme).
- **Streaming:** Faz-başı ilerleme güncellemeleri (plan → çalıştır → sentezle).

### Frontend Kontrolleri
- **Ayarlar Sekmesi:** Aç/kapa, şef modeli yapılandır, rollere model ata.
- **Yapılandırma Dialog'u:** Model seçimi, sistem promptu düzenleme, özel rol desteğiyle rol editörü.
- **Slash Komutu:** `/orchestra on`, `/orchestra off`, `/orchestra config`, `/orchestra status`.

---

## 8. 👁️ Multimodalite ve Duyular

### Görsel Destek (Multimodal)
- **Görsel Entegrasyonu**: Analiz için sürükle-bırak ya da yükleme (Llava ya da Moondream gibi multimodal-yetenekli bir GGUF gerektirir).
- **Base64 İşleme**: Yerel, güvenli görsel encoding.

### Dosya Bağlamlaştırma
- **Belge İndeksleme**: Belirli bir görev için yapay zekaya anında devasa bağlam vermek üzere kod dosyaları (.go, .js, .py) ya da belgeler (.md, .txt) ekleyin.

### Yerel STT (Konuşma-Metin)
- **Çevrimdışı Transkripsiyon**: Uygulama içinde doğrudan sesli mesaj kaydedin.
- **Gömülü Motor**: Sıfır-gecikmeli, özel transkripsiyon için yerelleştirilmiş bir ortam (whisper.cpp) kullanır.

---

## 9. ⏰ Rutinler ve Proaktif Zeka

### Rutinler (Zamanlanmış Otomasyonlar)
- Bir görevi ve bir zamanlamayı sade dilde tarif edin; Memo bunu zamanında ateşlenen, basit bir prompt ya da tam bir tool-kullanan ajan çalıştırması olarak bir rutine çevirir.
- **Sadece Rutinler sekmesinden değil, sohbetten de oluşturun**: normal bir sohbetten ya da WhatsApp/Telegram kendine-sohbet asistanından sade dilde bir rutin isteyin, `create_routine`/`list_routines`/`cancel_routine` ajan araçları hallederi — özel Rutinler ekranını açmaya gerek yok.
- Bir rutin, nasıl oluşturulduğundan bağımsız olarak ateşlendiğinde her zaman tam ajan + web-arama tool erişimine sahiptir — önceki bir hata bu erişimi oluşturma anında yapılan tek-seferlik bir sınıflandırmaya bağlıyordu, bu yüzden daha sonra sessizce "kapanabiliyordu"; koşulsuz olacak şekilde düzeltildi.
- **Masaüstü ve mobilde** çalışır — telefonda hatırlatmalar doğrudan işletim sistemine zamanlanır, böylece uygulama açık olmasa bile gelir.
- **Kendi cihazınızın saat diliminde** ateşlenir (oluşturulurken yakalanır, her yeniden bağlantıda yeniden senkronlanır), böylece seyahat/DST dondurulmuş kalmak yerine kendini düzeltir.

### Proaktif Öğrenme ve Ortamsal Dürtüler
- Memo kullanım desenlerini fark eder (belirtilmiş bir alışkanlık, ya da belirli bir saatte yapma eğiliminde olduğunuz bir şey) ve kendiliğinden gündeme getirebilir — varsayılan olarak ince bir seviyede açık.
- Doğrudan belirtilmiş bir alışkanlık ("her gece 9 civarı kod yazarım") anında güvenilir; pasif olarak gözlemlenen bir desen önce istatistiksel olarak ortaya çıkmalı.
- Bir dürtü normal bir yanıta işlenmiş olarak, ya da bir masaüstü öneri banner'ı olarak (Evet / Şimdi değil / Sormayı Durdur) görünebilir.
- Gizli Modda tamamen kapalıdır, Minimal Modda da özellikle yeniden açılmadıkça kapalıdır.

### Öz-İçgörü (`/insight`)
- Doğrudan sorun, ya da haftalık bir Rutin sorsun, Memo gerçek bir desen için yakın zamandaki mood/hafıza geçmişine bakar — yeterli sinyal yoksa bir tane uydurmaması açıkça talimatlandırılmıştır.

### Minimal Mod (Ayarlar → Genel)
- Yerel modelini olabildiğince az yükle çalıştırmak isteyenler için her promptttan kişilik/mood/web-arama talimatlarını soyar; hafıza da kapalıyken, yazılan mesajın ötesinde hiçbir şey eklenmez.
- Persona/sistem-promptu, yetenek bildirimleri, pasif-özellik bildirimleri ve proaktif öğrenme, Minimal Mod aksi halde açıkken bile her biri bağımsız olarak yeniden etkinleştirilebilir.

### Memo'nun Kendi Kimliği
- Memo'yu kimin yaptığını, ne için olduğunu ya da neyi temsil ettiğini sormak artık bir tahmin yerine gerçek, temellendirilmiş bir yanıt alır — bu sadece sorulduğunda ortaya çıkar, günlük davranışı değiştirmez ya da hangi personanın seçildiğine bağlı değildir.

---

## 10. 🛠️ Geliştirici ve İleri-Seviye Kullanıcı Özellikleri

### Geliştirici API Ağ Geçidi (Kenar Çubuğu → Developer)
- İki yerel endpoint: bir **Anthropic-uyumlu** olanı (böylece **Claude Code** gibi araçlar, `ANTHROPIC_BASE_URL` üzerinden Memo'ya karşı çalışabilir) ve bir **OpenAI-uyumlu** kardeşi (`GET /v1/models`, `POST /v1/chat/completions`), ikisi de Memo'nun yerel modeline ya da yapılandırılmış herhangi bir sağlayıcı/anahtara yönlendirilebilir.
- `type/model-id` formatıyla (`local/qwen2.5`, `openai/gpt-4o`, ...) model seçimi. openai/custom/local/groq/openrouter/grok/opencode-zen/opencode-go sağlayıcıları için tam agentic tool calling.
- **"API Anahtarı Gerektir" artık her iki gateway'de de loopback-olmayan her çağıran için zorunlu** (v4.5.0 güvenlik düzeltmesi) — önceden, uzaktan erişim açıkken ve anahtar gereksinimi kapalı bırakıldığında, OpenAI-uyumlu çift, portla erişebilen herhangi bir şey tarafından, hiçbir kimlik bilgisi olmadan erişilebilirdi.
- Opsiyonel hafıza entegrasyonu, canlı istek günlüğü.

### Memo Swarm (Beta)
- Tek bir makinenin RAM/VRAM'ine sığmayacak kadar büyük bir GGUF modelini çalıştırmak için birden fazla PC'nin işlem gücünü havuzlayın (Ayarlar → Beta Özellikler → Swarm) — bir Host model dosyasını tutar, diğerleri bir oda koduyla Katılır ve llama.cpp'nin `rpc-server`'ı üzerinden işlem gücü ödünç verir.
- Amaç kapasite, hız değil. Henüz macOS'ta yok.

### Kullanım İstatistikleri (Ayarlar → Stats)
- KPI kartları (toplam istek, giriş/çıkış token'ları, ort. tok/s, en çok kullanılan model), 30 günlük yığılmış günlük-kullanım grafiği ve model-başı döküm — Incognito modu hariç tamamlanan her tur için (yerel, ajan, orchestra ya da harici sağlayıcı) kaydedilir.
- **Prompt Önbelleği paneli (v4.6.0)**: girdinin ne kadarı önbellekten okundu, ne kadarı ona yazıldı, ne kadarı tam fiyattan gitti, önbellekten sunulan pay ve model/kategori satırlarında "N önbellekten" rozetleri. Akışlı sohbet artık kelime sayısı tahmini yerine sağlayıcının gerçek token sayılarını kullanıyor, Anthropic'in ayrı bildirdiği önbellek token'ları geri ekleniyor (böylece önbellek iyi çalıştıkça girdi *küçük* görünmüyor) ve özel Anthropic-uyumlu uç noktalar da önbellek alıyor. Hiçbir şey bildirmeyen sağlayıcı "bildirilmedi" gösterir, asla ölçülmüş %0 değil. Önbellek yalnızca tool taşıyan (ajan) turlarda istenir — düz bir sohbet turunda getirilen hafıza bloğu her seferinde değişir ve isabet olmadan yazma primini ödetir.

### Başka Bir Yapay Zekadan Hafıza İçe Aktar (Ayarlar)
- Başka bir AI asistanından (ChatGPT, Gemini, Claude, ...) yapılandırılmış bir açıklama yapıştırın, Memo bunu `/remember`'ın yaptığı gibi atomik gerçeklere böler, artı kendi sistem promptuna katılan bir iletişim-stili özeti.

### Hata Bildir (Ayarlar)
- Tarayıcınızda bir GitHub issue'sunu önceden doldurur (son 10 arkaplan hata olayının opsiyonel bir eki ile) — siz GitHub'da gözden geçirip göndermeden hiçbir şey hiçbir yere gönderilmez.

### Yeniden Düzenlenmiş Ayarlar
- Ayarlar ~20 düz sekmeden aranabilir, gruplanmış bir rafa, üstte bir arama kutusuyla taşındı. Genel sekmesi Genel, Özellikler, Sıfırlama ve CLI & Kaldırma olarak bölündü.

---

## 🎨 Tasarım Felsefesi: "Greige" Minimalizm
- **Odak-Öncelikli Arayüz**: Bilişsel yükü azaltmak için minimalist renk paleti.
- **Duyarlı Yerleşim**: Hem masaüstü-geniş hem mobil-dar görünümler için tasarlandı.
- **Onboarding Sihirbazı**: İsim, persona ve ilk tanılama için rehberli bir kurulum.

---
*Son güncelleme: 2026-09-15 · Sürüm: v4.5.0*

**Buğra tarafından yapıldı.**
*Yapay zekanı kontrol et. Hafızana sahip çık.*
