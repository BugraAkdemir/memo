# Sorun Giderme Kılavuzu

Memo için yaygın sorunlar ve çözümleri.

---

## 1. Port Çakışmaları (Address already in use)

**Sorun**: Backend `listen tcp :8090: bind: address already in use` hatası ile başlatılamıyor.

**Çözüm**:
- Mevcut süreci sonlandırın: `fuser -k 8090/tcp` (Linux)
- Veya farklı bir port kullanın: `go run . --port 8091`

## 2. VRAM / GPU Sorunları

**Sorun**: Model yüklenemiyor veya CPU üzerinde çok yavaş çalışıyor.

**Çözüm**:
- `n_gpu_layers` çok yüksekse büyük modeller (7B+) 4GB-6GB VRAM'e sığmayabilir
- Doğru `llama.cpp` binary'sini kullandığınızdan emin olun (NVIDIA için CUDA, AMD için ROCm)
- VRAM kısıtlıysa ayarlardan `n_gpu_layers` değerini düşürün

## 3. Llama Sunucusu Bulunamadı

**Sorun**: Backend loglarında `llama-server binary not found` hatası.

**Çözüm**:
- `data/bin/` klasörünü kontrol edin — Memo otomatik indirir
- Başarısız olursa, `llama-server` binary'sini resmi `llama.cpp` release sayfasından manuel indirip `data/bin/` içine koyun

## 4. Boş Ön Yüz Ekranı

**Sorun**: Flutter uygulaması açılıyor ama beyaz ekran gösteriyor.

**Çözüm**:
- Backend'in 8090 portunda çalıştığından emin olun
- `server.log` dosyasını kontrol edin
- Güvenlik duvarının localhost:8090'u engellemediğinden emin olun

## 5. Hafıza Geri Getirme Hataları

**Sorun**: AI geçmiş konuşmaları hatırlamıyor.

**Çözüm**:
- "Gizli Mod" hafızayı devre dışı bırakır — aktif olup olmadığını kontrol edin
- Embedding modelinin yüklü olduğundan emin olun (RAG aktif embedding sunucusu gerektirir, genelde port 8082)
- Hafıza deposunun aktif olduğunu kontrol edin — SQLite (sqlite-vec) kullanır

## 6. Harici Sağlayıcı Bağlantı Sorunları

**Sorun**: Sağlayıcılarda "Failed to connect" veya "Authentication error".

**Çözüm**:
- API anahtarını Ayarlar → API Sağlayıcıları'nda doğrulayın
- Kaydetmeden önce "Test Bağlantısı" kullanın
- Sağlayıcı base URL'ini kontrol edin (özellikle özel endpoint'ler için)
- Hız sınırları: bazı sağlayıcıların (Grok, Gemini ücretsiz) katı limitleri vardır
- Router 3 hatadan sonra otomatik devre dışı bırakır — sağlayıcı ayarlarından yeniden etkinleştirin

## 7. Ajan Modu Çalışmıyor

**Sorun**: Ajan modu yanıt vermiyor veya araç çağrıları başarısız.

**Çözüm**:
- Ajan modu aktif harici sağlayıcı gerektirir (yerel llama.cpp tool calling'i güvenilir şekilde desteklemez)
- API ile etkinleştirin: `PUT /api/agent/enabled {"enabled": true}`
- Ajan frontend UI henüz uygulanmadı — şimdilik doğrudan backend API kullanın
- İzin politikalarının araçları engelleyip engellemediğini kontrol etmek için `data/permissions.json` dosyasını inceleyin
