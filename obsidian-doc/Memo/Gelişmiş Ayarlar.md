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

## Beta Özellikler (Ayarlar → Beta Özellikler)
- Deneysel özelliklerin **tek anahtarı** buradadır (eski yer: Uzaktan Erişim sayfasının içi).
- **Açıkken:** [[Memo Swarm]], gömülü Tailscale tüneli ve ileride eklenecek diğer beta parçalar kullanılabilir hale gelir.
- **Kapalıyken:** Swarm menüsü ve Tailscale bölümü gizlenir / devre dışıdır.
- Her beta özelliğin asıl ayarı kendi ekranındadır (Swarm → yan menü; Tailscale → Uzaktan Erişim).

### Bağlantılı Notlar:
- [[Vektör Arama Mantığı]]
- [[API Dökümantasyonu]]
- [[Memo Swarm]]
