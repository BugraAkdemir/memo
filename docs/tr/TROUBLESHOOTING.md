# Sorun Giderme Kılavuzu

Memo için yaygın sorunlar ve çözümleri.

## 1. Port Çakışmaları (Address already in use)
**Sorun:** Backend, `listen tcp :8090: bind: address already in use` gibi bir hata ile başlatılamıyor.
**Çözüm:**
- Memo'nun başka bir örneği veya farklı bir servis 8090 portunu kullanıyor.
- Mevcut süreci sonlandırın: `fuser -k 8090/tcp` (Linux) veya farklı bir port kullanın: `go run . --port 8091`.

## 2. VRAM / GPU Sorunları
**Sorun:** Model yüklenemiyor veya CPU üzerinde çok yavaş çalışıyor.
**Çözüm:**
- VRAM kullanımınızı kontrol edin. Eğer `n_gpu_layers` çok yüksek ayarlanmışsa, büyük modeller (örn: 7B+) 4GB-6GB VRAM'e sığmayabilir.
- Doğru `llama.cpp` binary dosyasına sahip olduğunuzdan emin olun (NVIDIA için CUDA, AMD için ROCm).
- VRAM kısıtlıysa, yükün daha fazlasını CPU'ya aktarmak için ayarlardan `n_gpu_layers` değerini düşürün.

## 3. Llama Sunucusu Bulunamadı
**Sorun:** Backend loglarında `llama-server binary not found` hatası görünüyor.
**Çözüm:**
- Memo bunu genellikle otomatik olarak indirir. `data/bin/` klasörünü kontrol edin.
- Başarısız olursa, işletim sisteminiz için uygun `llama-server` binary dosyasını resmi `llama.cpp` release sayfasından manuel olarak indirin ve `data/bin/` içine yerleştirin.

## 4. Boş Ön Yüz Ekranı
**Sorun:** Flutter uygulaması açılıyor ancak beyaz bir ekran gösteriyor veya verileri yüklemiyor.
**Çözüm:**
- Backend'in doğru portta (varsayılan 8090) çalıştığından emin olun.
- Herhangi bir panic hatası için `server.log` dosyasını kontrol edin.
- Güvenlik duvarınızın 8090 portundaki yerel bağlantıları engellemediğinden emin olun.

## 5. Hafıza Geri Getirme Hataları
**Sorun:** Yapay zeka geçmiş konuşmaları hatırlamıyor gibi görünüyor.
**Çözüm:**
- "Gizli Mod"un aktif olup olmadığını kontrol edin (Hafızayı devre dışı bırakır).
- Bir embedding modelinin yüklü olduğundan emin olun. RAG, aktif bir embedding sunucusu gerektirir (genellikle port 8082).
- Hafıza deposunun aktif olup olmadığını kontrol edin. Hafıza, SQLite (sqlite-vec) vektör deposu kullanır.
