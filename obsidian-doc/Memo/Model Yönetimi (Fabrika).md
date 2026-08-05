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
- **Context Size:** Bağlam penceresini özelleştirme — dosyanın gerçek maksimum context'i model dosyasından okunuyor, slider onun ötesine geçemiyor (önceden serbest metin alanıydı, gerçekçi olmayan bir değer engine'i çökertebiliyordu).

## Daha Akıllı Keşif ve İndirme (v3.3.3)

- **İlk çalıştırmada donanıma uygun model önerisi** — kurulum sihirbazının yeni bir adımı RAM/GPU'yu okuyup uygun bir sohbet + hafıza modeli çiftini önerir, tek butonla ikisini birden indirmeye başlatır.
- **Aynı anda birden fazla indirme** — önceden ikinci bir indirme başlatmak reddediliyordu; artık birkaçı aynı anda çalışabiliyor, birleşik ilerleme engine durum çubuğunda gösteriliyor.
- **Araç-çağırma/kod-yeteneği rozetleri artık modelin gerçek chat template ve etiketlerine dayanıyor** — sabit kodlanmış "bilinen model ailesi" listesi yerine, özellikle yeni/az bilinen modellerde daha doğru.
- **Marka logosu gerçek üreticiyi yansıtıyor** — requantize edilmiş bir yükleme bile (ör. bir Gemma requant) doğru logoyu gösteriyor.
- Boş "?" yazar avatarları ve gated bir modelin 401 ile başarısız olan indirmesinde "Cancel" yazısında takılı kalan buton düzeltildi.
- Gerçekten erişim-kısıtlı olduğu doğrulanan Gemma 3 4B/12B önerilen listeden çıkarıldı, kısıtlı olmayan eşdeğerleriyle değiştirildi.
- **Discover filtreleri artık OR ile birleşiyor** (Tools/Vision/Code/Embedding/Size) — önceden AND ile birleşip iki filtre seçince genelde hiç sonuç dönmüyordu — ve düz bir chip satırı yerine "N filtre aktif · temizle" göstergeli çoklu-seçim dropdown'larına taşındı.

### Bağlantılı Notlar:
- [[Llama.cpp Entegrasyonu]]
- [[Backend (Go) Mimarisi]]
