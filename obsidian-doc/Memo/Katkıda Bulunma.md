# Memo'ya Katkıda Bulunmak

Memo'ya katkıda bulunmayı düşündüğünüz için teşekkürler! Solo geliştirici projesi olarak her türlü yardım takdir edilir.

---

## Nasıl Yardımcı Olabilirsiniz?

### 1. Hata Raporları
Bir issue açın:
- Yeniden üretme adımları
- İşletim sisteminiz ve Donanımınız (GPU/RAM)
- `server.log` dosyasındaki loglar

### 2. Özellik İstekleri
"Yerel öncelikli" felsefesine nasıl fayda sağlayacağını açıklayan bir issue açın.

### 3. Kod Katkıları
1. Depoyu fork'layın
2. Feature branch oluşturun: `git checkout -b feature/amazing-feature`
3. Go kodunu formatlayın: `go fmt ./...`
4. Flutter için Material 3 kurallarına uyun
5. Pull Request gönderin

## Geliştirme Standartları

- **Önce Gizlilik**: Kullanıcı rızası olmadan harici sunuculara veri göndermeyin
- **Performans**: SQLite + sqlite-vec gibi verimli algoritmaları tercih edin
- **Dökümantasyon**: Her özellik `docs/` ve Obsidian kasasında belgelenmelidir
- **Test**: Backend için `go test ./...`; frontend için `flutter test`

## Katkı İçin Kilit Alanlar

| Alan | Açıklama |
|------|----------|
| Ajan Frontend UI | İzin diyalogları, araç çağrı kartları, ajan açma/kapama (Flutter) |
| Testler | `internal/provider/`, `internal/agent/`, `internal/orchestra/` için birim testler |
| Çok Adımlı Planlama | Plan oluşturma, toplu izin, adım adım yürütme |
| Dosya Düzenleme Aracı | Diff önizlemeli ve geri almalı satır tabanlı düzenleme |
| Git Araçları | git_status, git_diff, git_commit, git_push entegrasyonu |

## Felsefe

Memo, **Egemenlik** üzerine inşa edilmiştir. Kullanıcılar yapay zekalarına, verilerine ve donanımlarına sahip olmalıdır. Yerel tutun, hızlı tutun, sizin tutun.
