# Yedekleme & Restore

Memo, tam veri taşınabilirliği için zip tabanlı bir yedekleme ve geri yükleme sistemi (`.memo` formatı) ve Google Drive bulut senkronizasyonu sunar.

---

## 📦 Yerel Yedekleme (`.memo` Formatı)

### Dışa Aktarımı
```
GET /api/export
```

Bir zip arşivi oluşturur:
- Tüm sohbet oturumları (`data/sessions/`)
- Yapılandırma (`config/config.yaml`)
- Şifrelenmiş anahtarlarla sağlayıcı yapılandırması (`data/providers.json`)
- Orkestra yapılandırması (`data/orchestra.json`)
- Hafıza deposu (`data/memory/`)
- WhatsApp verileri (`data/whatsapp/`)
- Model indeksi (`data/models/`)

### İçe Aktarımı
```
POST /api/import
```

Bir `.memo` zip dosyasından geri yükler. Mevcut verilerin üzerine yazar.

### Silme (Wipe)
```
POST /api/wipe
```

Tüm verileri tamamen siler (oturumlar, hafıza, WhatsApp, sağlayıcılar). Çift onay gerektirir. Yapılandırma dosyası korunur.

## ☁️ Bulut Senkronizasyonu (Google Drive)

Memo, Google Drive'a E2E şifreli yedekleme sunar:

1. **OAuth Kimlik Doğrulama**: Google hesabınıza OAuth 2.0 ile bağlanın
2. **Şifreleme**: Veriler AES-256-GCM ile kullanıcı tarafından sağlanan parola ile şifrelenir
3. **Anahtar Türetme**: PBKDF2 (600.000 iterasyon) + rastgele 16-byte tuz
4. **Depolama**: Google Drive'da gizli uygulama verisi klasörü
5. **Senkronizasyon Tetikleyicileri**: Hafıza kaydında otomatik veya API ile manuel

### API Endpoint'leri

| Method | Endpoint | Açıklama |
|--------|----------|----------|
| `GET` | `/api/sync/status` | Kimlik durumu, son senkronizasyon zamanı |
| `POST` | `/api/sync/start` | Şimdi senkronize et |
| `POST` | `/api/sync/connect` | OAuth akışını başlat |

### Şifreleme Detayları

- **Algoritma**: AES-256-GCM
- **Anahtar Türetme**: PBKDF2 (600.000 iterasyon)
- **Tuz**: Rastgele 16 byte, şifreli metnin başına eklenir
- **Geri Dönüş Anahtarı**: Parola ayarlanmamışsa `data/.machine-id` içindeki kalıcı UUID kullanılır
- **Eski Destek**: Eski SHA-256 türetilmiş anahtarlar hala çözme için çalışır

## 🔒 Güvenlik

- Yedekleme zip dosyaları standart ZIP sıkıştırması kullanır (zip'in kendisi şifrelenmez — E2E şifreleme bulut senkronizasyonu seviyesinde uygulanır)
- Telemetri veya bulut bağımlılığı yok — her şey isteğe bağlı
- Silme işlemi atomiktir ve tamdır
