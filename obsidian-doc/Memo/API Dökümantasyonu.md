# 📡 API Dökümantasyonu

Memo Backend, Flutter Frontend veya üçüncü parti istemciler için kapsamlı bir REST API sunar. Varsayılan olarak `localhost:8090` portunda çalışır.

## Temel Endpointler

### Sohbet ve Mesajlaşma
- `POST /api/send`: Normal mesaj gönderimi.
- `POST /api/send/stream`: Akışlı (SSE) mesaj gönderimi.
- `POST /api/send_file`: Dosya veya görsel içeren mesaj gönderimi.
- `GET /api/messages`: Mevcut oturumun mesaj geçmişini getirir.

### Hafıza Yönetimi
- `GET /api/status`: Hafızadaki toplam anı sayısını ve sistem durumunu döndürür.
- `POST /api/incognito`: Gizli modu açar/kapatır.
- `GET /api/memory/files`: Semantik hafıza dosyalarını listeler.
- `POST /api/memory/clear`: Tüm hafızayı sıfırlar.

### Model Kontrolü
- `GET /api/models/local`: İndirilmiş modelleri listeler.
- `POST /api/models/start`: Belirli bir modeli başlatır.
- `POST /api/models/stop`: Çalışan modeli durdurur.
- `GET /api/gpu`: GPU ve VRAM bilgisini döndürür.

### Senkronizasyon
- `GET /api/sync/settings`: Cloud Sync ayarlarını getirir.
- `PUT /api/sync/settings`: Ayarları günceller.

---
> **Not:** API kullanımı hakkında daha fazla detay için `internal/webserver/server.go` dosyasını inceleyebilirsiniz.
