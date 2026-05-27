# 🏭 Model Yönetimi (Fabrika)

Memo, yapay zeka modellerini keşfetmeyi, indirmeyi ve yönetmeyi uygulama içinden kolaylaştırır.

## Hugging Face Entegrasyonu
Model Store üzerinden doğrudan Hugging Face üzerinde GGUF formatındaki modelleri arayabilirsiniz.
- **Otomatik Filtreleme:** Sadece uyumlu GGUF formatları gösterilir.
- **Repo ID Desteği:** Herhangi bir Hugging Face URL'sini yapıştırarak doğrudan indirme başlatılabilir.

## Akıllı Tanılama (Diagnostics)
Model indirmeden veya başlatmadan önce sistem:
- **VRAM Kontrolü:** Ekran kartınızın kapasitesini kontrol eder.
- **GPU Uyumluluk Rozeti:** Modelin GPU'nuzda verimli çalışıp çalışmayacağını belirtir.

## Llama.cpp Motoru
Arka planda yüksek performanslı `llama.cpp` çalışır. Memo, işletim sisteminize uygun en güncel binary'yi otomatik olarak indirir ve yapılandırır.

### Desteklenen Özellikler:
- **GPU Offloading:** Katmanları (Layers) VRAM'e taşıyarak CPU yükünü azaltma.
- **Context Size:** Bağlam penceresini (örn: 4096, 8192) özelleştirme.

### Bağlantılı Notlar:
- [[Llama.cpp Entegrasyonu]]
- [[Backend (Go) Mimarisi]]
