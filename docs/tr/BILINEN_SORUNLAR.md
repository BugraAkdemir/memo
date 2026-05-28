# Bilinen Sorunlar ve Teknik Riskler (Kapsamlı Denetim)

Bu belge, Memo projesindeki tüm tespit edilen hataları, mimari kısıtlamaları ve uç durumları takip eder.

## 🧵 Eşzamanlılık ve Yarış Durumları (Race Conditions)
- **Oturum Yönetimi Kilitleme:** `internal/sessions/sessions.go` içinde `mu` (RWMutex) kullanılsa da, çok hızlı oturum değiştirme ve mesaj ekleme işlemlerinde nadir görülebilecek yarış durumları oluşabilir.
- **Llama Süreç İzleme:** `internal/llama/llama.go` içindeki `waitDone` kanalı, çok uç zamanlama durumlarında iki kez kapatılmaya çalışılabilir (select kontrolü ile hafifletilmiştir).
- **Hafıza Yeniden Başlatma:** `app.go` içinde `reinitMemoryStore` çağrıldığında `store` pointer'ı değiştirilir. O sırada başka bir iş parçacığında RAG araması yapılıyorsa "nil pointer" hatası alınabilir.

## 🚀 Performans ve I/O
- **Kaba Kuvvet Vektör Arama:** Hafıza araması RAM üzerindeki tüm embeddingleri O(N) hızında tarar. Küçük koleksiyonlarda hızlı olsa da, anı sayısı on binlere ulaştığında performans düşecektir.
- **Senkron Yazma:** Her mesaj hem GOB hem JSON dosyalarına senkron yazılır. Yavaş disklerde bu işlem yanıt süresini (latency) doğrudan artırır.
- **Dizin Tarama:** `internal/memory/store.go` içindeki `ListGobFiles` tüm klasörü tarar. Binlerce dosya olduğunda Ayarlar -> Hafıza ekranı yavaş açılacaktır.

## 🛡️ Hata Yönetimi ve Güvenilirlik
- **Sessiz Hatalar:** `app.go` içindeki birçok işlem (örneğin embedding modelinin otomatik başlaması) hata aldığında sadece log dosyasına yazar. Kullanıcı RAG'in neden çalışmadığını arayüzden göremeyebilir.
- **SSE Bağlantı Yetimleri:** Kullanıcı SSE akışı sırasında tarayıcıyı/uygulamayı kapatırsa, arka uçtaki goroutine bir sonraki yazma denemesine kadar çalışmaya devam edebilir.
- **Sabit Zaman Aşımları:** Model yükleme için belirlenen 180 saniyelik sınır, çok büyük modellerde (70B+) yetersiz kalabilir.
- **Port Çakışmaları:** `killByPort` olsa bile, port kritik bir sistem süreci tarafından kullanılıyorsa Memo kullanıcıya net bir açıklama yapmadan kapanabilir.

## 📡 Donanım ve İşletim Sistemi Uyumluluğu
- **Windows'ta AMD Kısıtlamaları:** Windows'ta AMD VRAM algılaması `rocm-smi` PATH içinde değilse çalışmaz.
- **Linux Sysfs Yolları:** GPU algılama mantığı standart Linux çekirdek isimlendirmesini varsayar. Özel sürücüler bu mantığı bozabilir.

## 📱 Ön Yüz (Flutter)
- **Durum Senkronizasyonu:** Frontend, arka uçtaki durum değişikliklerini (örneğin başka bir yerden modelin durdurulması) otomatik olarak algılayıp arayüzü güncellemez.
- **Anı Silme:** Bir anı dosyası arayüzden silindiğinde, eğer o anı LLM'in mevcut "aklındaki" (prompt içindeki) bağlamdaysa, oturum yenilenene kadar etkisini sürdürür.
