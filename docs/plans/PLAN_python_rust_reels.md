# Python × Rust — dikey Instagram kesitleri

Kullanıcı mevcut filmden birkaç ilgi çekici dikey kesit istedi; seçim editöre bırakıldı. Yeni ses/ücretli üretim/yayınlama yok.

- [x] Dört kendi içinde tamamlanan bölüm seç; orijinal ses/görüntüyü koruyan 1080×1920 çerçeve, büyük altyazı, merak başlığı ve mevcut hesapları hazırla.
- [x] Dört MP4 render; gerçek kareleri, altyazı sınırlarını, cümle bitişlerini, ses/video akışını ve tam decode sonucunu kontrol et; teslim et.

## Seçimler

1. Python görünüyor. Altında ne var? — Python/C++ ve hızın hangi katmanda olduğu, yaklaşık 52 sn.
2. Python yazıyorsun. İçinde Rust var. — Tokenizers ve aynı üründe farklı diller, yaklaşık 47 sn.
3. Daha hızlı dil, daha hızlı cevap? — trafik/köprü analojisi + ağ beklemesi + önce ölçme, yaklaşık 38 sn; iki kaynak bölümün editoryal birleşimi.
4. Rust öğrenmeli misin? — yeni başlayan, ürün geliştiren ve sistem programlama kararları, yaklaşık 48 sn.

Kesin kaynak kareleri ve süreler `build_output/marketing/python-rust-reels/edits.json` içindedir. Altyazılar mevcut filmin onaylı JSON'undan seçilen aralıklara yeniden zamanlandı. Kaynak başlıkları/diyagramları kesmemek için geniş görüntü dikey çerçeve içinde korunur; küçük eski altyazı ve hesap alanı yerine yeni büyük dikey altyazı/kimlik katmanı kullanılır.

## Uygulama

- `scripts/rust-ai-slides/src/film/ReelBackground.tsx`: konuya özel 4 dikey çerçeve.
- `scripts/rust-ai-slides/script/create_reels.py`: kare bazlı seçim, Remotion çerçeveleri, FFmpeg kesim/birleştirme/dikey kompozisyon, ASS/SRT.
- Kaynak: `build_output/marketing/python-rust-ai-tr-1080p.mp4`; orijinal film değişmez.
- Çıktılar: `build_output/marketing/python-rust-reels/`.
- TypeScript kontrolü exit 0. Son dört MP4: 1080×1920/30 fps, H.264 + AAC. 4/4 tam ses/görüntü decode exit 0, hata çıktısı 0 bayt. 103 altyazı parçası ve seçilen 407 konuşma kelimesi korundu; sıra/süre sınırları PASS. Kesin süreler 52,166667 / 46,733333 / 38,166667 / 47,966667 sn. ZIP 14 dosya / CRC PASS. Rapor: `build_output/marketing/python-rust-reels/verification.json`. Başlık taşması ve eski altyazı kenarı görsel kontrolde düzeltilip son çıktılar yeniden doğrulandı.
