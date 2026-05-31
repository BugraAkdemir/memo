# Model İndirme UI Yeniden Tasarımı

## Tamamlanan İşler
- `[x]` GPU Config sekmesi Ayarlara eklendi
- `[x]` Llama installer overlay kapatılabilir yapıldı
- `[x]` `models_provider.dart` — `downloadProgressProvider` StreamProvider'a çevrildi (1sn polling)

## Yapılacaklar

### 1. `_DownloadProgressCard` Yeniden Tasarımı (model_store_screen.dart)
- `[x]` İndirme aktifken büyük, belirgin bir kart göster
  - Dosya adı büyük font ile
  - Yüzde göstergesi (büyük, ortada, animasyonlu)
  - İndirilen / Toplam boyut (ör: "1.2 GB / 4.5 GB")
  - Hız bilgisi (ör: "12.5 MB/s")
  - Renkli LinearProgressIndicator (kalın, animasyonlu)
  - İndirme bitince otomatik olarak local models listesini yenile

### 2. `_ModelFilesDialog` Tasarım İyileştirmesi (model_store_screen.dart)
- `[x]` AlertDialog yerine özel Dialog kullan
  - Dosya boyutlarını renkli badge ile göster
  - İndir butonunu daha belirgin yap
  - Quant bilgisini (Q4_K_M, Q5_K_S vb.) vurgula

### 3. `_SearchResultCard` Tasarım İyileştirmesi (model_store_screen.dart)
- `[x]` Download/Like sayılarını daha okunabilir formatta göster
- `[x]` Kart hover efekti ekle

### 4. Derleme ve Test
- `[ ]` `flutter run -d linux` ile derle
- `[ ]` İndirme başlat, yüzde takibinin çalıştığını doğrula
