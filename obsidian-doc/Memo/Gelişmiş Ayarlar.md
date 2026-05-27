# 🔧 Gelişmiş Ayarlar

Memo, güç kullanıcıları için RAG ve Model parametrelerini ince ayar yapma imkanı sunar.

## Hafıza (RAG) Ayarları
- **Top-K (Hafıza Adedi):** Her sorguda kaç adet geçmiş anının çekileceğini belirler. (Varsayılan: 5)
- **Similarity Threshold (Eşik Değeri):** Bir anının "alakalı" sayılması için gereken minimum skor. (Örn: 0.75)
- **Min Similarity:** Çok alakasız anıların bağlamı kirletmesini önlemek için kullanılır.

## Model Parametreleri
- **Temperature:** Cevapların ne kadar "yaratıcı" veya "tutarlı" olacağını belirler. (0.0 - 1.0)
- **Repeat Penalty:** Modelin aynı kelimeleri tekrar etmesini engeller.
- **GPU Layers:** VRAM'e aktarılacak katman sayısı. Tam performans için tüm katmanların GPU'da olması önerilir.

## Ağ ve Erişim
- **Remote Access:** Bu ayar açıldığında Memo, yerel ağdaki (Wi-Fi) diğer cihazların erişimine açılır.
- **Port Settings:** Varsayılan 8090 portunu çakışma durumunda değiştirebilirsiniz.

### Bağlantılı Notlar:
- [[Vektör Arama Mantığı]]
- [[API Dökümantasyonu]]
