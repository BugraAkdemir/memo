# ⚙️ Llama.cpp Entegrasyonu

Memo, LLM çıkarımı (inference) için dünyanın en popüler ve performanslı yerel motoru olan `llama.cpp`'yi kullanır.

## Entegrasyon Detayları
Backend, `llama-server` binary dosyasını bir alt süreç (subprocess) olarak yönetir.

### Akıllı Başlatma
Bir model başlatıldığında Memo şu parametreleri otomatik olarak ayarlar:
- `--port`: Dinleme portu.
- `--model`: GGUF dosya yolu.
- `--n-gpu-layers`: VRAM kapasitesine göre GPU'ya aktarılacak katman sayısı.
- `--ctx-size`: Bağlam penceresi boyutu.
- `--embedding`: Eğer embedding sunucusu olarak başlatılıyorsa bu bayrak eklenir.

## Otomatik Kurulum (Llama Installer)
Kullanıcının manuel olarak `llama.cpp` derlemesine gerek yoktur.
1. Sistem, işletim sistemini (Linux/Windows) ve donanımı (Cuda/CPU) tespit eder.
2. En uyumlu pre-built binary'yi güvenli sunuculardan indirir.
3. `data/bin/` dizinine yerleştirerek kullanıma hazır hale getirir.

## Performans İzleme
Sohbet esnasında `llama.cpp`'den gelen metrikler anlık olarak yakalanır ve kullanıcıya `tokens per second` (t/s) olarak sunulur.

### Bağlantılı Notlar:
- [[Model Yönetimi (Fabrika)]]
- [[Backend (Go) Mimarisi]]
