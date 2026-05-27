# 🚀 Geliştirici Kurulum Rehberi

Memo projesine katkıda bulunmak veya yerel ortamda çalıştırmak için aşağıdaki adımları izleyin.

## Gereksinimler
- **Go:** v1.20+
- **Flutter:** v3.10+ (Master veya Stable channel)
- **C++ Derleyici:** (Llama.cpp derlemek isterseniz, opsiyonel)
- **Linux:** `build-essential`, `libgtk-3-dev`, `libayatana-appindicator3-dev`

## Kurulum Adımları

### 1. Depoyu Klonlayın
```bash
git clone https://github.com/kullanici/memo.git
cd memo
```

### 2. Backend'i Hazırlayın
Bağımlılıkları indirin ve sunucuyu başlatın:
```bash
go mod tidy
go run . --port 8090
```

### 3. Frontend'i Hazırlayın
Ayrı bir terminalde:
```bash
cd frontend
flutter pub get
flutter run -d linux
```

## Önemli Dosyalar
- `main.go`: Giriş noktası.
- `app.go`: Ana uygulama mantığı.
- `frontend/lib/main.dart`: UI giriş noktası.
- `config/config.yaml`: Yapılandırma ayarları.

### Bağlantılı Notlar:
- [[Derleme ve Paketleme]]
- [[Backend (Go) Mimarisi]]
