# 💾 Veri Katmanı ve Kalıcılık

Memo, geleneksel veritabanları (SQL/NoSQL) yerine, yerel performans ve veri bütünlüğü için özelleşmiş bir dosya yapısı kullanır.

## SQLite/vec0 Formatı: Kalıcılık
Memo'nun tüm anıları ve ayarları SQLite/vec0 formatında saklanır.

### Avantajları:
- **Hız:** JSON veya XML'e göre çok daha hızlı serileştirme ve geri yükleme.
- **Atomik Yazma:** Her etkileşim bağımsız bir dosya olarak kaydedilir. Bu sayede bir dosyanın bozulması tüm veritabanını etkilemez.
- **Tip Güvenliği:** Go nesne yapısını birebir korur.

## Klasör Yapısı (`data/`)
- `data/memory/`: Semantik vektör dosyaları ve anılar.
- `data/sessions/`: Sohbet geçmişi (JSON formatında).
- `data/models/`: İndirilen GGUF model dosyaları.
- `data/sync_token.json`: Bulut senkronizasyon yetkilendirme verileri.

## Bellek (RAM) Yönetimi
Memo, binlerce anı olsa bile düşük RAM kullanımı sağlar:
1. Başlangıçta sadece vektörleri (sayısal veriler) RAM'e yükler.
2. Metin içerikleri diskte kalır.
3. Arama yapıldığında sadece en alakalı 5-10 anının metni diskten okunur.

### Bağlantılı Notlar:
- [[RAG ve Semantik Hafıza]]
- [[Vektör Arama Mantığı]]
