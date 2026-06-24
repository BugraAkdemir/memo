# Memo'ya Katkıda Bulunmak

Öncelikle Memo'ya katkıda bulunmayı düşündüğünüz için teşekkürler! Memo bağımsız olarak geliştirilen bir projedir ve bu "İkinci Beyin"i daha iyi hale getirmek için her türlü katkı içtenlikle takdir edilir.

## Nasıl Yardımcı Olabilirsiniz?

### 1. Hata Raporları
Bir hata bulursanız, lütfen şu bilgilerle bir issue açın:
- Yeniden üretme adımları.
- İşletim sisteminiz ve Donanımınız (GPU/RAM).
- `server.log` dosyasındaki loglar.

### 2. Özellik İstekleri
Harika bir fikriniz mi var? Bir issue açın ve bunun "yerel öncelikli" (local-first) felsefesine nasıl fayda sağlayacağını açıklayın.

### 3. Kod Katkıları
- Depoyu fork'layın.
- Bir özellik dalı (feature branch) oluşturun (`git checkout -b feature/amazing-feature`).
- Go kodunuzun formatlandığından emin olun (`go fmt ./...`).
- Flutter kodunuzun Material 3 kurallarına uygun olduğundan emin olun.
- Bir Pull Request gönderin.

## Geliştirme Standartları
- **Önce Gizlilik:** Kullanıcının açık rızası olmadan verileri harici sunuculara gönderen kodları asla eklemeyin.
- **Performans:** RAM kullanımını düşük tutmak için SQLite + sqlite-vec gibi verimli algoritmaları tercih edin.
- **Dökümantasyon:** Her yeni özellik `/docs` klasöründe ve Obsidian kasasında dökümante edilmelidir.

## Felsefe
Memo, **Egemenlik** üzerine inşa edilmiştir. Kullanıcılar yapay zekalarına, verilerine ve donanımlarına sahip olmalıdır. Yerel tutun, hızlı tutun, sizin tutun.
