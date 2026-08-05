# 💾 Veri Katmanı ve Kalıcılık

Memo, hafıza depolaması için tek bir gömülü **SQLite** veritabanı kullanır — ayrı "her etkileşim kendi dosyası" gibi bir yapı yoktur, hepsi tek dosyada (`data/memory/memory.db`).

## SQLite Formatı: Kalıcılık

### Avantajları:
- **ACID İşlemler:** Yazma sırasında oluşabilecek hatalar veritabanının tamamını bozmaz.
- **Eşzamanlı Erişim:** WAL (Write-Ahead Logging) modu okuma/yazmanın aynı anda çalışmasına izin verir.
- **Artımlı Yazma:** Yeni bir mesaj kaydederken sadece bir `INSERT` çalışır, tüm veritabanı yeniden yazılmaz.

## Gerçek Şema (`internal/memory/store.go`)

Tek bir `memory.db` dosyası içinde birkaç tablo/sanal tablo bulunur:

| Tablo | Ne İçin |
|-------|---------|
| `memories` | Asıl satırlar: içerik, zaman damgası, `importance`, `source` (`conversation`/`explicit`/`merged`), embedding blob |
| `memories_fts` | FTS5 sanal tablosu — anahtar kelime (tam metin) araması için |
| `vec_memories` | `sqlite-vec`'in `vec0` sanal tablosu — vektör ANN araması için |
| `_metadata` | Migration bayrakları, embedding boyutu gibi iç durum |

`vec0` ya da `fts5` derleme zamanında kullanılamıyorsa (bkz. [[CGO Bayrakları]]), ilgili özellik sessizce devre dışı kalır ve Go tarafında yazılmış bir yedek arama yoluna düşülür — bu yüzden backend'i her zaman doğru derleme bayraklarıyla derlemek kritik önemde.

## Klasör Yapısı (`data/`)
- `data/memory/memory.db`: Tüm hafıza (tek dosya).
- `data/sessions/`: Sohbet geçmişi (JSON formatında, hafızadan ayrı).
- `data/models/`: İndirilen GGUF model dosyaları.
- `data/sync_token.json`: Bulut senkronizasyon yetkilendirme verileri.
- `data/routines/`: (v3.3.3) Routines tanımları.
- `data/tasklists/`: Görev listeleri.
- `data/stats/`: (v3.3.3) Kullanım istatistikleri.
- `data/calendar/`, `data/profile/`: Takvim etkinlikleri, öğrenilen alışkanlıklar/bekleyen öneriler.
- `data/permissions.json`: Ajan izin politikaları.
- `data/machine.key`: `providers.json`'daki şifreli API anahtarlarını çözen anahtar — v3.3.3'e kadar `.memo` yedeğine hiç dahil edilmiyordu (bkz. [[Yedekleme & Restore]]).

Tam yedekleme kapsamı için [[Yedekleme & Restore]]'a bakın.

## Bellek (RAM) Yönetimi

Ayrı bir "vektörleri RAM'e önceden yükle" mekanizması **yoktur** — her arama, o anda doğrudan SQLite'a sorgu atarak yapılır:
- `sqlite-vec` mevcutsa, `vec0` sanal tablosu kendi ANN indeksini disk üzerinde tutar, sorgu diske gider.
- Değilse, Go tarafındaki yedek yol tüm embedding'leri tek seferlik okuyup kosinüs benzerliğini bellekte hesaplar (küçük/orta ölçekli hafıza depoları için yeterli, kalıcı bir RAM önbelleği değildir).

### Bağlantılı Notlar:
- [[RAG ve Semantik Hafıza]]
- [[Vektör Arama Mantığı]]
- [[Hafıza Deposu (SQLite + vec0)]]
