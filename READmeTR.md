<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="120"/>

  <h1>Memo</h1>
  <p><b>Alışkanlıklarını öğrenen ve sen sormadan harekete geçen yapay zeka asistanı.</b></p>
  <p>Yerel-öncelikli · Gizlilik-öncelikli · Sıfır bulut bağımlısı · Tamamen çevrimdışı</p>

  <br/>

  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/>
  </a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers">
    <img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57" alt="Yıldız"/>
  </a>
  &nbsp;
  <img src="https://img.shields.io/badge/Lisans-AGPL_v3-blue?style=for-the-badge" alt="Lisans"/>
  &nbsp;
  <img src="https://img.shields.io/badge/Sürüm-v3.1.0_beta-blue?style=for-the-badge" alt="Sürüm"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-gömülü-orange?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_vec0-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-entegre-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-lightgrey?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/CI-geçiyor-success?style=flat-square&logo=githubactions" alt="CI"/>

</div>

---

## Memo Nedir — ve Neden Farklı

Bugün piyasadaki her yapay zeka asistanı temelde aynı ürün: bir modele bağlı bir metin kutusu. Sen yazarsın, o kelimeler döndürür. Sohbet biter. Hiçbir şey kalıcı olmaz. Hiçbir şey öğrenilmez. Hiçbir şey aksiyon almaz.

Memo bu kalıbı üç şekilde kırar.

**Birincisi, hatırlar.** Perde arkasında, her konuşma yerel bir vektör veritabanına beslenir — sqlite-vec ile güçlendirilmiş, ANN (yaklaşık en yakın komşu) araması yapabilen bir SQLite örneği. Memo ile konuştuğunda, sadece son mesajını okumakla kalmaz. Haftalar veya aylar öncesinden anlamsal olarak alakalı konuşma parçalarını çeker ve sistem prompt'una enjekte eder. Üç hafta önce tartıştığın bir projeden bahsedersin, Memo bağlamı zaten bilir. Bir takip sorusu sorarsın, geçen sefer ne karar verdiğinizi hatırlar. Bu, tamamen senin makinen üzerinde çalışan Retrieval-Augmented Generation (RAG) — bulut vektör veritabanı yok, Pinecone yok, embedding API'si yok. Sadece SQLite ve senin GPU'n.

**İkincisi, alışkanlıklarını öğrenir.** Memo'nun arka planda çalışan bir gözlemci alt sistemi vardır — ne söylediğini değil, *ne zaman* aktif olduğunu izler. Her mesaj etkileşimi zaman damgası ve aktivite türüyle kaydedilir. Yaklaşık bir hafta sonra, dairesel istatistik analizcisi ritimleri tespit etmeye başlar: her akşam 21:00'da kod yazıyorsun, pazartesi sabahları planlama yapıyorsun, 15:00 civarı mola veriyorsun. Bu gözlemler güven skorlu pattern'ler oluşturur. Güven eşiği aştığında, proaktif motor bir LLM'e danışır (Memo'nun çoklu model koordinatörü Orchestra aracılığıyla) ve ne yapacağına karar verir: sohbette bir şey öner, telefonuna bildirim gönder veya — yüksek güvende — ajanı otonom olarak başlat. Zayıflayan pattern'ler unutulur. Açıkça reddettiğin pattern'ler anında silinir. Tüm sistem isteğe bağlı ve şeffaftır. Dışarı hiçbir şey gönderilmez; gözlemci sadece konu etiketlerini ve kelime sayılarını saklar, asla ham mesaj metnini değil.

**Üçüncüsü, harekete geçebilir.** Memo sadece bir sohbet arkadaşı değildir. Ajan motoru ona gerçek araçlar verir — sistemindeki dosyaları okumak ve yazmak, shell komutları çalıştırmak, web'de arama yapmak, WhatsApp ile etkileşime girmek. Bu araçlar yol doğrulaması, symlink koruması, komut kara listesi ve 6-politikalı izin sistemi olan bir sandbox içinde çalışır. Ajan bir şeye dokunmadan önce, sen onaylar veya reddedersin — bir kereliğine, bu oturum için veya kalıcı olarak. Ajan pipeline'ı araç çağrısı başına 60 saniye zaman aşımı ve maksimum 20 iterasyon uygular, böylece sonsuza kadar döngüye giremez veya takılı kalmaz. İzin modeli kalıcı olduğu için, her araç-bağlam için sadece bir kez karar vermen gerekir.

Sonuç: sadece konuşmayan bir asistan. İzler, hatırlar, öğrenir ve yapar — hepsi senin donanımında, sıfır telemetri ile.

---

## Kullanıcı Deneyimi

Memo'yu indiriyorsun. Tek bir kurulum dosyası — Docker yok, Python ortamı yok, PATH ayarı yok. llama.cpp binary'nin içinde gömülü. Başlatıyorsun.

İlk gördüğün şey bir kurulum sihirbazı — dilini seç, tema seç, kişilik seç. Altı hazır seçenek (Rahat, Resmi, Teknik, Yaratıcı, Eğlenceli, Kanka) ya da kendi sistem prompt'unu yaz. Otuz saniye ve bitti.

Sonra bir launchpad beliriyor — her bölümün ne yaptığını açıklayan beş kart. Sohbet. Ajan. Orkestra. WhatsApp. Takvim. Her kartta pazarlama sloganı değil, gerçek bir açıklama var. Birine dokun ve git. 4 adımlı bir spotlight turu, navigasyon ikonlarını tek tek ışıklandırarak gezdiriyor, "Geç" butonu her zaman görünür durumda. Ondan sonra, sen istemedikçe bir daha asla görünmez.

Model Mağazası'nı açıyorsun. Gerçek RAM ve VRAM'ine göre hesaplanmış donanım uygunluk rozetleriyle küratörlü modeller gösteriyor — statik bir uyumluluk listesinden değil. "Cihazına uygun — GPU'da hızlı" ya da "CPU'da çalışır" ya da "Çok büyük — belleğe sığmayabilir" görüyorsun. Birini seç, indir'e tıkla, bitince Başlat'a bas. Alt çubuk çalıştığını gösteriyor.

"Merhaba" yazıyorsun. Memo Türkçe yanıtlıyor çünkü öyle yazdın. Sohbetin ortasında İngilizceye geçiyorsun, o da geçiyor. Bir kod dosyası ekliyorsun — okuyor ve inceliyor. Bir görsel yüklüyorsun — ne gördüğünü açıklıyor (modelin görüntü desteği varsa). Üst çubuktaki web arama düğmesini açıyorsun ve birden her yanıt DuckDuckGo'dan taze sonuçlarla zenginleşiyor — API anahtarı yok, yapılandırma yok.

Memo'yu iki haftadır kullanıyorsun. Bir pazartesi sabahı açıyorsun. Sen bir şey yazmadan önce diyor ki: "Pazartesi planlaması yapalım mı? Geçen hafta şu projeye başlamıştın." Pazartesi sabahlarının senin planlama zamanın olduğunu öğrendi. Projeyi hatırladı. Önerdi, varsaymadı. Kabul edebilir, reddedebilir ya da bir daha asla önerme diyebilirsin.

Memo bu. Bir chatbot değil. Adının hakkını veren bir asistan.

---

## Her Özellik, Detaylıca

### 💬 Sohbet

Sohbet, her şeyin bir araya geldiği yer. Token-token akan çok turlu konuşmalar. Tam markdown render — sözdizimi vurgulu kod blokları, tablolar, listeler, kalın, italik. Görsel ekleyebilirsin (modelin görüntü desteği varsa görür) ve dosya ekleyebilirsin (metin dosyaları, PDF'ler, kod). Üstteki gizli mod düğmesi, hiçbir şeyin oturum dosyasına kaydedilmediği, RAG hafızası için indekslenmediği ve öğrenme motoru tarafından gözlemlenmediği bir moda geçirir. Web arama düğmesi her mesajı canlı DuckDuckGo sonuçlarıyla zenginleştirir — sonuçlar model yanıt vermeden önce sistem bağlamına enjekte edilir. WhatsApp modu düğmesi, aynı arayüz üzerinden WhatsApp hesabınla sohbet etmeni sağlar.

Üst çubuk, mevcut modu açıklayıcı rozetlerle gösterir — gizli mod aktifken "Gizli Mod", ajan sohbeti aktifken "Agent", WhatsApp modu açıkken "WhatsApp". Her rozetin üzerine gelince ne yaptığını açıklayan bir tooltip çıkar.

Mesaj kutusu bir metin alanından fazlasıdır. `/` yazınca Phosphor ikonlu bir slash-komut paleti açılır. Mikrofon düğmesine basılı tutup konuş, whisper.cpp ile sesin metne çevrilsin. Dosya ekle butonu ile dosya gönder. Taslağın, sohbet değiştirsen bile hatırlanır.

Mesajlar düzenlenebilir ve silinebilir. Senin söylediğini veya modelin söylediğini değiştirebilirsin, konuşma bağlamı buna göre güncellenir. Tek tıkla herhangi bir sohbeti markdown dosyası olarak dışa aktar.

Gönderdiğin her mesaj aynı anda dört sisteme beslenir: RAG hafıza indeksleyicisi, proaktif gözlemci, niyet çıkarıcı (takvim etkinlikleri için) ve — eğer açıksa — stokastik duygu motoru.

### 🧠 RAG Hafıza

Çoğu yapay zeka aracının hafızası bir japon balığı gibidir. Her sohbet sıfırdan başlar. Memo'nun hafıza sistemi bunu değiştirir.

Sen ve Memo mesajlaştığınızda, konuşma çifti (senin mesajın + verilen yanıt) 768 boyutlu bir vektöre dönüştürülür. Bu dönüşüm, yerel bir embedding modeli (varsayılan olarak Nomic Embed v1.5, ama herhangi bir embedding modeli çalışır) tarafından yapılır. Vektör, sqlite-vec eklentisiyle ANN indeksleme yapabilen bir SQLite veritabanında saklanır. Bu sayede, binlerce saklanmış etkileşim arasından bile, gelecekteki sorgulara anlamsal olarak en yakın geçmiş konuşmalar milisaniyeler içinde bulunur.

Yeni bir soru sorduğunda, Memo kosinüs benzerliğine göre en alakalı geçmiş konuşmaları getirir, yapılandırılmış bir hafıza bloğu halinde formatlar ve model yanıt vermeden önce sistem prompt'una enjekte eder. Model şöyle bir şey görür:

```
Below are relevant memories from your past conversations with Buğra:

[Memory 1 | Relevance: 85%]
User: Kullanıcılar tablosunun şeması neydi?
Assistant: Kullanıcılar tablosunda id, email, isim, created_at sütunları var...
```

Modelin artık bağlamı var. Kendini tekrar etmene gerek yok. Geçen hafta tartıştığın şemayı zaten biliyor.

Hafıza Ayarlar'dan yönetilebilir — top-K (kaç hafıza getirileceği) ve minimum benzerlik (alaka eşiği) ayarlanabilir. Tüm saklanan hafıza dosyaları görüntülenebilir, içerikleri incelenebilir, tek tek veya topluca silinebilir. Hafıza istenildiği zaman açılıp kapatılabilir.

### 🤖 Ajan Motoru

Ajan, Memo'nun "konuşmak"tan "yapmak"a geçtiği yer.

Bir proje klasörü seçerek ajan sohbeti başlatıyorsun. Ajan artık dosyalarını görebilir, okuyabilir, yazabilir, o klasör içinde komut çalıştırabilir ve web'de arama yapabilir. 8 yerleşik aracı var:

- `read_file` — projedeki herhangi bir dosyayı okur
- `write_file` — dosya oluşturur veya üzerine yazar (yedekli)
- `edit_file` — dosyada hedefli metin değişikliği yapar (yedekli)
- `delete_file` — dosya veya dizin siler
- `list_directory` — klasör içeriğini listeler
- `run_command` — proje dizini içinde shell komutu çalıştırır (`rm -rf /`, `sudo`, `mkfs`, `shutdown`, fork bomb'ları ve benzeri tehlikeli komutları engelleyen kara liste ile)
- `web_search` — DuckDuckGo'da canlı bilgi arar
- `search_files` — dosya sisteminde pattern'e uyan dosyaları bulur

Her araç çağrısı bir izin diyaloğu tetikler. Ajanın tam olarak ne yapmak istediğini görürsün — araç adı, argümanlar ve işlem tehlikeliyse bir uyarı. Bir kereliğine, oturum boyunca veya kalıcı olarak izin verebilir veya reddedebilirsin. Reddedilen pattern'ler hatırlanır, böylece ajan neye dokunmasını istemediğini öğrenir. İzin sistemi 6 politikalıdır ve sürekli onay istemeden hassas kontrol sağlar.

Ajan pipeline'ı 20 iterasyona kadar çalışır: model bir araç çağırmaya karar verir → sen onaylarsın → araç çalışır → sonuç modele geri beslenir → model bir sonraki adıma karar verir veya nihai yanıtı döndürür. Her araç 60 saniye zaman aşımına sahiptir. Model döngüye girerse veya takılırsa, iterasyon limiti durdurur. Akışı iptal edersen, çalışan tüm araçlar context iptali ile hemen durur.

Sandbox her dosya yolunu doğrular. Sınırları kontrol etmeden önce symlink'leri çözümler — böylece projenin içinde `/etc/`'ye işaret eden bir symlink kabul edilmez. Windows case-insensitive yollar doğru işlenir. Korumalı sistem yolları (`/etc/`, `/proc/`, `/sys/`, `/dev/` ve platforma özgü yollar) yazma ve silme işlemleri için engellenir.

Sohbet arayüzünde, ajan eylemleri ham JSON yığınları olarak değil, temiz, animasyonlu bir durum çizgisi olarak görünür. Araç çalışırken "Dosya işleniyor..." yazar, bitince "Dosya tamam ✅". Tamamlanan işlemler `[Dosya okudu 120ms]` gibi kompakt rozetler halinde görünür. Ajan deneyimi Claude Code veya Cursor benzeridir — minimal, bilgilendirici, asla teknik gürültü değil.

### 🎵 Orkestra — Çoklu Model İş Akışı

Tek bir model iyidir. Birden fazla modelin ekip olarak çalışması daha iyidir.

Orkestra, Memo'nun çoklu model koordinasyon sistemidir. Karmaşık bir istek gönderdiğinde, bir Şef model (yapılandırılabilir — herhangi bir sağlayıcı ve model olabilir) görevi analiz eder ve alt görevlere ayırır. Her alt görev bir uzman role atanır:

- **Planlayıcı** — gereksinimleri yapılandırılmış uygulama planlarına dönüştürür
- **Ön Yüz** — UI, stil, kullanıcıya dönük kod
- **Arka Yüz** — sunucu mantığı, API'ler, veritabanı sorguları
- **Hata Düzeltici** — verilen koddaki hataları bulur ve düzeltir
- **Gözden Geçirici** — kodu hatalar, stil ve doğruluk açısından inceler
- **Güvenlik Denetçisi** — güvenlik açıklarını kontrol eder
- **DevOps** — CI/CD, Docker, altyapı yapılandırması
- **Genel Uzman** — diğer rollere uymayan her şeyi halleder

Her role farklı sağlayıcılar ve modeller atayabilirsin. Mantıksal çıkarım için Claude, hızlı arama için Gemini, hız için Groq, kod üretimi için yerel llama.cpp modelin. Orchestra yapılandırma ekranı bunu kolaylaştırır — tek tıkla Şef'e ve tüm açık rollere bir model ata ya da her rolü ayrı ayrı yapılandır.

Bağımsız roller paralel çalışır. Her uzmanın işini tamamlamasını gerçek zamanlı görürsün. Şef model tüm çıktıları tek, tutarlı bir yanıtta sentezler. Paralel yürütme, sırayla 3 dakika sürecek bir görevin saniyeler içinde bitmesini sağlar.

Orkestra, Ajan ile birlikte de çalışabilir. İkisi de etkinken: önce Şef planlar, sonra Ajan planı adım adım uygular — dosyaları okur, kod yazar, komut çalıştırır — ve Şef sonuçları gözden geçirip nihai yanıtı sentezler. İki sistemin en iyi yanları: Orkestra'dan üst düzey strateji, Ajan'dan alt düzey uygulama.

### 📱 WhatsApp Entegrasyonu

Memo, WhatsApp hesabına çoklu cihaz Web API'si üzerinden bağlanır — WhatsApp Web'in kullandığı protokolün aynısı. WhatsApp Business API ücreti yok. Telefon numarası kaydı yok. Sadece bir QR kod okut ve bağlan.

Eşleştikten sonra, sohbet listen Memo'nun WhatsApp sekmesinde belirir. Mesajları okuyabilir, tüm sohbet geçmişinde arama yapabilir ve yanıt gönderebilirsin. Sohbet arayüzü Memo'nun kendi temasına uyar — bronz aksan, panel renkleri, ana sohbetle aynı stilde mesaj baloncukları. Profil fotoğrafları çekilir ve önbelleklenir, tam çözünürlüklü avatarlar için indirme butonlu büyütme diyaloğu vardır.

WhatsApp entegrasyonu bir sohbet istemcisinden daha derindir. Ajan, araçları aracılığıyla WhatsApp'a erişebilir — `whatsapp_send` ile mesaj gönderme, `whatsapp_search` ile mesaj arama, `whatsapp_latest` ile son konuşmaları listeleme. "Berra'ya mesaj at" diyebilirsin ve Memo kişi adını otomatik çözümler, JID bilmene gerek kalmaz.

WhatsApp mesajları, normal sohbetle aynı sistemlere beslenir: RAG hafıza indeksleyicisi, proaktif gözlemci, niyet çıkarıcı ve duygu motoru. Bir arkadaşın "kanka cuma akşamı sinema" der ve Memo niyeti tespit eder, takvim etkinliği oluşturur ve sonra sana hatırlatır.

Yeniden bağlanma otomatiktir. Daha önce eşleştirdiysen ve uygulamayı yeniden başlattıysan, WhatsApp hiçbir şeye tıklamana gerek kalmadan yeniden bağlanır — doğrudan sohbet listesine düşersin. Backend, bağlantı kopmalarını exponential backoff, yeniden bağlanma deneme sınırı ve 5 saniyelik logout zaman aşımı ile yönetir; böylece dengesiz bir ağ uygulamayı asla kilitlemez.

### 📅 Takvim

Memo'nun takvimi, elle doldurduğun ayrı bir araç değildir. Konuşmalarını izleyen ve zamana bağlı plan ve taahhütleri çıkaran otomatik bir yakalama sistemidir.

Niyet çıkarım pipeline'ı iki aşamalı çalışır. Önce, hızlı bir anahtar kelime filtresi her mesajı tarar — "yarın", "salı", "haftaya", "saat", sayısal zamanlar ve hem Türkçe hem İngilizce tarih kalıplarını arar. Sadece eşleşen mesajlar ikinci aşamaya gönderilir: doğal dili yapılandırılmış etkinlik verisine (başlık, tarih, saat, kaynak, kişi adı) dönüştüren bir LLM. Sıradan sohbet asla LLM çağrısı tetiklemez, böylece performans etkisi olmaz.

Bir niyet tespit edildiğinde, yerleşik takvim SQLite veritabanında bir etkinlik oluşturulur. Arayüz, etkinlik olan günlerde noktalı bir aylık ızgara görünümü gösterir. Bir güne dokunarak etkinlikleri görebilir, manuel etkinlik ekleyebilir veya silebilirsin. Takvim her 20 saniyede bir otomatik yenilenir, böylece sohbetten veya WhatsApp'tan eklenen etkinlikler yeniden başlatmadan görünür.

Bir hatırlatma döngüsü her dakika yaklaşan etkinlikleri kontrol eder. Bir etkinlik yapılandırılan süre içindeyse (10 dakika ile 2 saat arası, Ayarlar'dan ayarlanabilir), masaüstünde ve mobilde uygulama olay sistemi üzerinden bir bildirim tetiklenir. `ClaimPendingReminders` metodu, aynı hatırlatmanın iki kez tetiklenmesini önlemek için atomik bir SQL transaction'ı kullanır.

Memo bir etkinlikten emin değilse — örneğin, "belki yarın buluşalım" dediysen — bir iptal belirsizliği etkinliği oluşturur. Arayüz bunu bir uyarıyla gösterir ve tek tıkla onaylayabilir veya silebilirsin. Ayarlar'dan zaman tahmini özelliğini tamamen kapatabilir, sadece açıkça zamanlanmış etkinlikleri isteyebilirsin.

### 🧠 Proaktif Öğrenme Motoru

Memo, sana gelen tek yapay zeka asistanıdır. Ona bir şey sormayı hatırlamak zorunda değilsin.

Gözlemci alt sistemi aktivite pattern'lerini takip eder — Memo'yu *ne zaman* kullandığını, ne söylediğini değil. Her mesaj etkileşimi zaman damgası ve aktivite türüyle kaydedilir. Bir dairesel istatistik analizcisi bu veriyi periyodik olarak işler, istatistiksel olarak anlamlı pattern'ler arar: "Pazartesi sabahları 9-10 arası, planlama aktivitesi" veya "Her gün 21:00-23:00 arası, kodlama aktivitesi."

Bunun arkasındaki matematik, saat zamanına uygulanan dairesel (yönlü) istatistiktir — zamanı 23:59 ve 00:01'in bitişik olduğu, 24 saat arayla olmadığı bir daire olarak ele alır. Bu, doğrusal zaman analizinden daha doğru pattern tespiti sağlar. Pattern'ler üç faktörden hesaplanan güven skorlarına sahiptir: tutarlılık (pattern'in ne kadar düzenli olduğu), sıklık (ne sıklıkta gerçekleştiği) ve güncellik (en son ne zaman gözlemlendiği).

Bir pattern'in güveni eşiği aştığında, proaktif motor bir Şef LLM'e danışır (Orchestra aracılığıyla) — mevcut zaman, eşleşen pattern ve neler olduğuna dair bağlam ile. Şef karar verir: yararlı bir şey öner, kullanıcının telefonuna bildirim gönder veya yüksek güvende, ajanı otomatik başlatarak bir şey yap.

Sistemin katı güvenlik kuralları vardır. Açıkça izin vermeden asla işlem yapmaz. Tüm öğrenilen pattern'leri Ayarlar → Öğrenme'den görüntüleyebilir ve tek tıkla unutabilirsin. Kullanılmadığı için zayıflayan pattern'ler otomatik olarak unutulur. Gözlem katmanı sadece konu etiketlerini ve kelime sayılarını saklar — konuşmalarının ham metni asla.

### 🏪 Model Mağazası

Doğru yapay zeka modelini bulmak ve indirmek genellikle HuggingFace repo'ları, şifreli dosya adları ve donanımının kaldırıp kaldıramayacağına dair tahmin labirentidir. Memo'nun Model Mağazası tüm bunları düzeltir.

Keşfet sekmesi, temiz iki panelli düzenle küratörlü, resmi olarak yayınlanmış modeller gösterir: solda model listesi, sağda zengin detay görünümü. Her model, onu yapan şirketin gerçek logosunu gösterir (Google, Microsoft, Qwen/Alibaba, nomic ve diğerleri) — HuggingFace'ten otomatik çekilir. Modeller boyuta (1-8B, 8-14B, 14B+) ve yeteneğe (Araç, Görüntü, Kod) göre filtrelenebilir.

Donanım uygunluk rozeti öldürücü özelliktir. Memo başladığında, GPU'unu tespit eder (NVIDIA için nvidia-smi, AMD için rocm-smi ve sysfs, ya da sadece CPU) ve mevcut VRAM ile RAM'i ölçer. Her modelin her indirme seçeneği, senin gerçek donanımına göre değerlendirilir ve net bir rozetle gösterilir: "Cihazına uygun — GPU'da hızlı" ya da "CPU'da çalışır" ya da "Çok büyük — belleğe sığmayabilir." Artık 14B'lik bir model indirip 8GB kartının kaldıramadığını sonradan öğrenmek yok.

İndirme seçenekleri ham quantization kodları yerine sade dilde kalite etiketleri kullanır: `Q4_K_M` yerine "Dengeli kalite", `Q5_K_M` yerine "Yüksek kalite", `Q2_K` yerine "En küçük boyut". Detay paneli modelin HuggingFace'teki tam README'sini, yetenek etiketlerini (Araç Kullanımı, Görüntü, Kod), parametre sayısını, mimarisini ve "Bu yazardan daha fazlası" önerilerini gösterir.

İndirme sırasında ilerleme gerçek zamanlı akar. İndirmeler iptal edilebilir. Tamamlandığında, içe aktarılan modeller Modellerim'de görünür. Başlat düğmesi, tespit edilen motor moduna göre GPU offloading ile llama-server'ı otomatik yapılandırır ve başlatır — `-ngl`'nin ne olduğunu bilmene gerek yoktur.

### 🔌 8 Sağlayıcı, Tek Arayüz

Memo yerel modellerle, bulut API'leriyle veya ikisiyle aynı anda çalışır. Sekiz sağlayıcı türü desteklenir:

OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama ve herhangi bir yerel llama.cpp sunucusu. Her sağlayıcı bir API anahtarı (makinede üretilen rastgele anahtarla AES-256-GCM şifreli) ve bir model seçimi ile yapılandırılır. Birden fazla sağlayıcı aynı anda etkinleştirilebilir ve öncelik sıralı yedek zinciri ile çalışır.

Sağlayıcı sistemini production-grade yapan şey: bir sağlayıcı başarısız olursa (rate limit, ağ hatası, API kesintisi), Memo sohbetini bölmeden otomatik olarak bir sonraki etkin sağlayıcıya geçer. Üç ardışık hatadan sonra, sorunlu sağlayıcı zincirleme zaman aşımlarını önlemek için otomatik devre dışı bırakılır. Her 5 dakikada bir arka plan sağlık kontrolü çalışır ve düzelen sağlayıcıları yeniden etkinleştirir. Geçici sorunlar yüzünden sağlayıcıları elle açıp kapatman gerekmez.

Sağlayıcı değiştirme canlıdır. Sohbetin ortasında `/model` yaz, farklı bir sağlayıcı seç ve bağlam kaybetmeden devam et. Sohbet penceresinin altındaki motor çubuğu, hangi sağlayıcının aktif olduğunu gerçek şirket logosu ve yeşil bağlantı noktasıyla gösterir.

Model başına bağlam pencereleri yapılandırılabilir. Gemini varsayılan olarak 1M token, Claude 200K, diğerleri 128K — ama spesifik model varyantına uyması için herhangi bir sağlayıcıya herhangi bir pencere atayabilirsin. Bağlam bütçesi (her istekte ne kadar sohbet geçmişinin paketlendiği) sağlayıcı tipini değil, modelin gerçek penceresini takip eder.

### 🎤 Sesli Giriş

Dahili whisper.cpp ile konuşma-metne çevrimi. Mesaj kutusundaki mikrofon düğmesine basılı tut, konuş, bırak — kelimelerin yazıya dökülür ve gönderilir. Tüm işlem cihazında gerçekleşir — ses bilgisayarından asla çıkmaz. Türkçe, İngilizce ve karışık dil girişini otomatik algılar. Whisper modeli ilk kullanımda otomatik indirilir.

### ☁️ Bulut Senkronizasyonu

Cihazlar arası yedekleme isteyen kullanıcılar için Memo, şifreli Google Drive senkronizasyonu destekler. Verilerin (oturumlar, hafıza, ayarlar, sağlayıcı yapılandırmaları, WhatsApp verileri, duygu motoru veritabanı, öğrenme verileri) arşivlenir, parolandan PBKDF2 ile 600.000 iterasyonda türetilen bir anahtarla AES-256-GCM ile şifrelenir ve Google Drive'a yüklenir. Şifreleme yüklemeden önce gerçekleştiği için Google verilerini okuyamaz.

Parola belirlemezsen, Memo `crypto/rand` ile üretilen ve `data/machine.key` dosyasında 0600 izinleriyle saklanan makineye özgü bir anahtar kullanır. Senkronizasyon her N mesajda bir otomatik çalışır (yapılandırılabilir, varsayılan 50) ya da push/pull/tam senkronizasyonu manuel tetikleyebilirsin.

Her şeyi tek bir `.memo` dosyası olarak dışa aktar. Herhangi bir makinede içe aktararak tüm Memo durumunu geri yükle — konuşmalar, hafıza, ayarlar, modeller listesi.

### 🔒 Tasarımı Gereği Gizli

Memo'daki her tasarım kararı şu soruyla başlar: "bunun kullanıcının makinesinden çıkması gerekiyor mu?" Cevap neredeyse her zaman hayırdır.

- Telemetri yok. Analitik yok. Harici servislere çökme raporlaması yok. Memo'nun yaptığı tek ağ istekleri, senin açıkça yapılandırdıklarındır: model API çağrıları, WhatsApp eşleştirmesi, web arama sorguları ve isteğe bağlı bulut senkronizasyonu.
- Sağlayıcı API anahtarları, makineye özgü rastgele bir anahtarla diskte şifrelenir. Yapılandırma dosyalarında, loglarda veya bellek dökümlerinde asla düz metin olarak görünmez.
- Yapılandırma ve hassas veri dosyaları 0600 izinleriyle yazılır — sadece dosya sahibi okuyabilir.
- Gizli mod, sıfır iz bırakan konuşmalar sağlar: oturum dosyası yok, hafıza indeksi yok, gözlem yok.
- Proaktif gözlemci sadece aktivite türlerini ve zaman damgalarını saklar, mesaj içeriğini değil.
- WhatsApp mesajları yerel bir SQLite veritabanında saklanır. WhatsApp'ın zaten gördüğünün ötesinde WhatsApp sunucularından hiçbir şey geçmez.
- Rate limiting, kontrolsüz istemcilere karşı backend'i korur (IP başına saniyede 100 istek, engellemesiz token bucket).
- 50MB body boyutu sınırı, büyük boyutlu yüklemelerin belleği tüketmesini önler.

---

## Hızlı Başlangıç

**Terminal yok. Derleme yok. Tek tık.**

| Platform | İndirme | Nasıl |
|----------|---------|-------|
| **Windows** | `Memo-Setup.exe` | Kurulumu çalıştır → bitti |
| **Linux** | `.AppImage` | `chmod +x` → başlat |
| **Linux** | `.deb` | `sudo dpkg -i` → bitti |

llama.cpp gömülü gelir. İlk başlatmada her şey `~/.memo` altına kopyalanır. Uygulamayı aç, **Model Mağazası**'na git, bir model seç, sohbete başla.

<div align="center">
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Memo'yu_İndir-memo.bugradev.com-B08D57?style=for-the-badge" alt="İndir"/>
  </a>
</div>

<details>
<summary><b>Kaynaktan derleme</b></summary>

**Gereksinimler:** Go 1.26+ · Flutter 3.10+ · SQLite geliştirme kütüphaneleri (CGO için)

```bash
git clone https://github.com/BugraAkdemir/memo.git
cd memo
CGO_ENABLED=1 go run . --port 8090          # Backend
cd frontend && flutter run -d linux          # Frontend (ayrı terminal)
```

Sürüm paketleri:
```bash
./build_releases.sh     # Linux  → AppImage / deb / tar.gz
.\build_releases.bat    # Windows → Inno Setup kurulumu / zip
```
</details>

---

## Mimari

```
┌─────────────────────────────┐    ┌──────────────────────────┐
│  Flutter Masaüstü            │    │  Flutter Mobil            │
│  (Linux / Windows)           │    │  (Android / iOS)          │
│                              │    │                           │
│  Sohbet · Ajan · Orkestra    │    │  Sohbet · Bildirimler     │
│  Ayarlar · Model Mağazası    │    │  Uzak bağlantı            │
└──────────────┬───────────────┘    └───────────┬───────────────┘
               │  REST + SSE (:8090)             │  LAN / ngrok
               └──────────────┬──────────────────┘
┌──────────────────────────────┴──────────────────────────────────┐
│                 Go Backend — 25 paket, ~55 endpoint              │
│                                                                  │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │ Web Sunucu   │  │ App Motoru   │  │ Proaktif Motor         │ │
│  │ ServeMux     │  │ orkestratör  │  │ Gözlemci→Analiz→Eylem  │ │
│  │ SSE akışı    │  │ (25 dosya)   │  │                        │ │
│  └─────────────┘  └──────┬───────┘  └────────────────────────┘ │
│                           │                                     │
│  ┌────────┐ ┌──────────┐ ┌┴─────────┐ ┌──────────┐ ┌────────┐ │
│  │ Hafıza │ │ Oturumlar│ │ Llama    │ │WhatsApp  │ │ Ajan   │ │
│  │ vec0   │ │ JSON     │ │ GPU/RAM  │ │whatsmeow │ │ Pipe   │ │
│  └────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│                                                                  │
│  Orkestra · ModelMağazası · BulutSenk · Takvim · DuyguMotoru    │
│  ngrok · Tailscale · Whisper · Skill · Niyet · Gözlemci         │
└──────────────────────────────────────────────────────────────────┘
```

İki süreç `localhost:8090` üzerinden düz HTTP ile iletişim kurar. TLS yok (sadece yerel). Harici router framework'ü yok — saf `net/http` ServeMux. Frontend, Riverpod state yönetimi, Dio üzerinden SSE akışı ve flutter_markdown ile markdown render içeren tek sayfalı bir Flutter uygulamasıdır.

**Belgeler:** [Mimari](docs/architecture.md) · [API Referansı](docs/API_REFERENCE.md) · [Tasarım Sistemi](frontend/DESIGN.md)

---

## Teknoloji Yığını

| Katman | Teknoloji | Notlar |
|--------|-----------|--------|
| **Backend dili** | Go 1.26 | SQLite için CGO gerekli |
| **HTTP** | `net/http` ServeMux | Harici router bağımlılığı yok |
| **Akış** | SSE (Server-Sent Events) | Token-token sohbet akışı |
| **Masaüstü frontend** | Flutter 3.10 | Linux + Windows |
| **State yönetimi** | Riverpod 2.4 | AsyncNotifierProvider |
| **HTTP istemcisi** | Dio 5.4 | SSE ayrıştırma, interceptor'lar |
| **Markdown** | flutter_markdown 0.6 | Kod blokları, tablolar, görseller |
| **LLM çalışma zamanı** | llama.cpp | Gömülü binary, alt süreç yönetimi |
| **Vektör veritabanı** | SQLite + sqlite-vec | vec0 ANN indeks, 768 boyut embedding |
| **WhatsApp** | whatsmeow | Go kütüphanesi, çoklu cihaz Web API |
| **Ses-metne** | whisper.cpp | Gömülü binary, cihaz üzerinde |
| **Bulut senk.** | Google Drive API v3 | OAuth2, AES-256-GCM, PBKDF2 |
| **GPU tespiti** | nvidia-smi, rocm-smi, sysfs | Otomatik VRAM ölçümü |
| **Loglama** | `internal/logx` | Info/Warn/Error/Debug seviyeli slog wrapper |
| **CI/CD** | GitHub Actions | Go vet+test+build, Flutter analyze+test |
| **Lisans** | GNU AGPL v3 | Özgür yazılım |

---

## Belgeler

| | |
|-|-|
| [🏛️ Mimari](docs/architecture.md) | Paket haritası, veri akışı, modül sınırları |
| [📡 API Referansı](docs/API_REFERENCE.md) | 55+ REST endpoint'i istek/yanıt şemalarıyla |
| [🎨 Tasarım Sistemi](frontend/DESIGN.md) | "Pewter Study" tema token'ları, renk paleti, tipografi |
| [🛣️ Yol Haritası](docs/ROADMAP.md) | Sürümlü yayın planı |
| [📱 Mobil](mobile/README.md) | Flutter mobil yardımcı kurulumu ve tünel yapılandırması |
| [🔧 Sorun Giderme](docs/TROUBLESHOOTING.md) | GPU kurulumu, port çakışmaları, sık hatalar |
| [📝 Katkı](docs/CONTRIBUTING.md) | Geliştirici kurulumu, kod stili, PR süreci |
| [📋 Değişiklik Günlüğü](versinNote/tr/v3.1.0.md) | Tam v3.1.0 özellik listesi, hata düzeltmeleri ve cilalama |

---

## Katkıda Bulunma

Memo AGPL-3.0 lisanslıdır. Katkılar memnuniyetle karşılanır.

- [Yol Haritası](docs/ROADMAP.md)'nı incele — planlanan özellikler
- [Bilinen Sorunlar](docs/KNOWN_ISSUES.md)'a göz at — iyi başlangıç görevleri
- Fikirler için [Tartışma](https://github.com/BugraAkdemir/memo/discussions) aç

---

<div align="center">
  <br/>
  <p><b>Senin zihnin. Senin verin. Senin makinen.</b></p>
  <p><a href="https://github.com/BugraAkdemir">Buğra Akdemir</a> tarafından geliştirildi</p>
  <br/>
  <a href="https://github.com/BugraAkdemir/memo/issues">Hata Bildir</a> ·
  <a href="https://github.com/BugraAkdemir/memo/discussions">Tartışma</a> ·
  <a href="README.md">English</a>
</div>
