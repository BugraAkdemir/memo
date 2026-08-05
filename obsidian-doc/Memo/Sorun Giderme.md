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

## 4b. macOS'ta "Connection Error" (Yerel Backend'e Ulaşılamıyor)

**Sorun**: macOS'ta uygulama açılıyor ama backend'e hiçbir istek gitmiyor gibi görünüyor — bağlantı hatası.

**Çözüm**: 2026-08-05'te düzeltilen gerçek bir App Sandbox entitlement eksikliğiydi — `frontend/macos/Runner/{Release,DebugProfile}.entitlements`'ta `com.apple.security.network.client` yoktu, bu da macOS'un yerel backend'e giden Dio çağrılarını sandbox seviyesinde engellemesine yol açıyordu (commit `420e6a5`). Güncel bir sürümde bu düzeltilmiş olmalı; eski bir build kullanıyorsanız güncelleyin. Aynı düzeltmeyle birlikte mikrofon (`device.audio-input`, Sesli Mod/STT için) ve dosya seçici (`files.user-selected.read-write`) izinleri de eklendi.

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
- Ajan modu aktif harici sağlayıcı gerektirir (yerel llama.cpp genel olarak güvenilir tool calling desteği sunmaz)
- Sohbet üst çubuğundaki agent toggle'ından açıp kapatabilirsiniz, ya da API ile: `PUT /api/agent/enabled {"enabled": true}`
- İzin politikalarının araçları engelleyip engellemediğini kontrol etmek için `data/permissions.json` dosyasını inceleyin
- **Küçük context'li yerel bir modelde** agent modu açıkken tek kelimelik bir mesaj bile "request exceeds the available context size" ile başarısız oluyorsa: bu v3.3.4'te düzeltildi (araç şeması artık context'e doğru bütçeleniyor, varsayılan yerel context 4096→8192'ye çıkarıldı) — hâlâ görüyorsanız güncel sürümde olduğunuzdan emin olun

## 8. Hafıza Açıkken Yerel Üretim Çok Yavaş

**Sorun**: Yerel bir modelde hafıza/RAG'ı açmak üretim hızını belirgin şekilde düşürüyor (10 t/s'den 2-3 t/s'ye gibi).

**Çözüm** (v3.3.4, geliştirme aşamasında — düzeltildi):
- Sebep, embedding sunucusunun sohbet modeliyle aynı anda VRAM için yarışması ve her prompt'a enjekte edilen hafıza bloğunun gereğinden büyük bir bütçeye (16K token) sahip olmasıydı
- Embedding sunucusu artık varsayılan olarak sadece-CPU çalışıyor; gerçekten boş VRAM'in varsa `embedding_gpu_layers` ile tekrar GPU'ya alabilirsiniz (bkz. [[Gelişmiş Ayarlar]])
- Hafıza context bütçesi artık ~4096 token'a sabitlendi
