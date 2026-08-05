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

## Güvenilirlik Düzeltmeleri (v3.3.3 / v3.3.4)

- **Takılı port artık otomatik temizleniyor (v3.3.3).** Embedding (veya sohbet) modeli bir kez başlamayı başaramazsa — genelde önceki bir çökmeden kalan bir sürecin portu tutmasından — önceden tam bir bilgisayar yeniden başlatmasına kadar bozuk kalıyordu; her tekrar deneme aynı sebeple başarısız oluyordu. Artık her başlatma denemesinden önce takılı port otomatik temizleniyor.
- **Varsayılan yerel context boyutu 4096 → 8192'ye çıkarıldı (v3.3.4).** Küçük context'li modellerde agent modunun araç tanımları context bütçesine hiç dahil edilmiyordu — tek kelimelik bir mesaj bile başarısız olabiliyordu; artık araç şeması context'e göre doğru bütçeleniyor, varsayılan da yükseltildi.
- **Embedding sunucusu artık varsayılan olarak sadece-CPU (v3.3.4).** Önceden hem sohbet hem embedding sunucusu GPU'da VRAM için yarışıyor, sohbet modelini kısmi CPU fallback'ine itebiliyordu — hafıza/RAG açıkken yerel üretim hızı 4-5 kat düşebiliyordu. `embedding_gpu_layers` config seçeneğiyle isteğe bağlı olarak tekrar GPU'ya alınabilir.
- **CLI ve masaüstü artık aynı backend'i paylaşabiliyor (v3.3.3).** Terminal CLI'nin başlattığı backend artık ayrı bir süreç; CLI ya da masaüstü uygulamasından herhangi biri kullandığı sürece açık kalıyor, ikisi de kapanınca ~1-2 dakika içinde kendiliğinden kapanıyor.

### Bağlantılı Notlar:
- [[Model Yönetimi (Fabrika)]]
- [[Backend (Go) Mimarisi]]
- [[Gelişmiş Ayarlar]]
