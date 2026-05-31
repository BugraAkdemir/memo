# Bilinen Sorunlar ve Teknik Riskler (Kapsamlı Denetim)

Bu belge, Memo projesindeki tüm tespit edilen hataları, mimari kısıtlamaları ve uç durumları takip eder. Derinlemesine yapılan kod denetimi sonrası güncellenmiştir.

## 🧵 Eşzamanlılık ve Yarış Durumları (Race Conditions)
- **Oturum Yönetimi Kilitleme:** `internal/sessions/sessions.go` içinde `mu` (RWMutex) kullanılsa da, `NewManager` sırasında çağrılan `newSession` gibi bazı işlemler açık kilitler olmadan yapılır. Hızlı oturum değiştirme ve mesaj ekleme işlemlerinde nadir yarış durumları oluşabilir.
- **Llama Süreç İzleme:** `internal/llama/llama.go` içindeki `waitDone` kanalı bir `monitor` goroutine'i içinde kapatılır. Süreç dışarıdan öldürüldüğünde veya hızlı başlat/durdur döngülerinde nadir zamanlama sorunları yaşanabilir.
- **Hafıza Yeniden Başlatma:** `app.go` içinde `reinitMemoryStore` çağrıldığında `store` pointer'ı değiştirilir. `saveMemoryAsync` bir goroutine başlatıp kilidi sonra aldığı için, re-init sırasında eski store durumuna başvurma veya gereksiz bekleme riski vardır.

## 🚀 Performans ve I/O
- **Hafıza Parçalanması (Her Etkileşim İçin Bir Dosya):** Her kullanıcı etkileşimi ayrı bir `.gob` dosyası olarak kaydedilir. Bu, yüzlerce veya binlerce küçük dosya oluşturur.
  - *Risk:* Başlangıç süresi (`LoadCache`) doğrusal olarak artar.
  - *Risk:* Bulut senkronizasyonu, binlerce küçük dosya için binlerce API çağrısı yapacağından verimsizleşir.
  - *Risk:* İşletim sistemi dosya sınırı (file handles) veya disk performansı (özellikle HDD'lerde) düşer.
- **Kaba Kuvvet Vektör Arama:** Hafıza araması RAM üzerindeki tüm embeddingleri O(N) hızında tarar. İş parçacıkları kullanılsa bile, anı sayısı 10.000'i geçtiğinde performans düşecektir.
- **Senkron Yazma:** Her mesaj hem GOB (hafıza) hem JSON (oturumlar) dosyalarına senkron yazılır. Bu, yavaş disklerde kullanıcı arayüzünde "takılmalara" neden olabilir.
- **Dizin Tarama:** `internal/memory/store.go` içindeki `ListGobFiles` tüm klasörü tarar. Ayarlar -> Hafıza ekranı binlerce dosya olduğunda yavaş yüklenir.

## 🛡️ Hata Yönetimi ve Güvenilirlik
- **Bozuk Olay Sistemi (Flutter Geçişi):** `app.go` içindeki `emitEvent` fonksiyonu Flutter için devre dışıdır.
  - *Kritik:* Arka plan hataları (Bulut senkronizasyon hataları, embedding modelinin otomatik başlama hataları, Llama kurulum ilerlemesi) **arayüze asla bildirilmez**. Sadece `server.log` içinde kalır.
- **Sessiz STT Bağımlılık Hataları:** Ses kaydı (`StartRecording`) sistemde `ffmpeg`, `sox` veya `arecord` bulunmasına bağlıdır. Bunlar eksikse kullanıcıya yardımcı olmayan genel bir hata döner.
- **Kırılgan LLM Hata Tespiti:** Görsel desteği gibi özelliklerin hata yönetimi, `llama.cpp`'den gelen İngilizce hata metinlerine bağlıdır. Bu metinler değişirse hata tespiti bozulur.
- **Zaman Aşımları:** Model yükleme için belirlenen 180 saniyelik sabit sınır, çok büyük modellerde veya yavaş sistemlerde yetersiz kalabilir.
- **Yetim SSE Bağlantıları:** Kullanıcı akış (stream) sırasında bağlantıyı keserse, arka uçtaki LLM isteği tamamlanana kadar (300 saniyeye kadar) çalışmaya devam eder; işlemci/GPU kaynağı israf edilir.

## 📡 Donanım ve İşletim Sistemi Uyumluluğu
- **Windows'ta AMD Kısıtlamaları:** Windows'ta AMD VRAM algılaması güvenilir değildir; `rocm-smi` genellikle PATH içinde olmadığı için sistem CPU moduna düşer.
- **Sabitlenmiş Windows Ses Aygıtı:** Windows kayıt komutu, ses aygıtı için sabit bir GUID (`@device_cm_{...}`) kullanır. Farklı donanım veya mikrofon dizilimlerinde bu komut çalışmayacaktır.
- **Linux Sysfs Yolları:** GPU algılama mantığı `/sys/class/drm/card*` isimlendirmesini varsayar. Özel sürücüler veya Docker gibi konteyner ortamları bu mantığı bozabilir.
- **Agresif Port Kapatma:** `killByPort` portları temizlemek için kullanılır, ancak `lsof`/`fuser` eksikliği veya yetki sorunları nedeniyle başarısız olabilir, bu da "adres kullanımda" hatalarına yol açar.

## 📱 Ön Yüz (Flutter)
- **Durum Senkronizasyonu:** Frontend, arka uçtaki durum değişikliklerini (örneğin başka bir istemcinin modeli durdurması) otomatik olarak algılamaz.
- **Hafıza Geçersiz Kılma:** Bir anı dosyası arayüzden silindiğinde diskten ve RAM'den silinir ancak LLM'in o anki aktif bağlam (context window) içeriğinden hemen temizlenmez.

## 🔐 Güvenlik ve Gizlilik
- **Geçici Dosya Sızıntısı:** Web üzerinden yüklenen dosyalar ve STT ikili dosyaları sistemin genel geçici dizininde tutulur. Çok kullanıcılı sistemlerde işletim sistemi izinleri sıkı değilse veri ifşasına yol açabilir.
- **Güvensiz Varsayılan Bağlantı:** Web sunucusu uzaktan erişim için `0.0.0.0` adresine bağlanır. Güvenlik duvarı olmayan sistemlerde basit bir parola ile risk oluşturabilir.
