# 🕵️ Gizli Mod (Incognito)

Memo, hassas konular üzerinde çalışırken tam gizlilik sağlamak için özel bir "Gizli Mod" sunar.

## Sıfır Kalıcılık (Zero-Persistence)
Gizli Mod aktif edildiğinde:
1. **Hafıza Kaydı Durdurulur:** Yapılan konuşmalar semantik hafızaya (`data/memory/`) kaydedilmez.
2. **Oturum Geçmişi Yazılmaz:** Konuşma geçmişi diske kaydedilmez ve uygulama kapatıldığında silinir.
3. **Volatile Context:** Bağlam sadece o anki oturum içinde, RAM üzerinde yaşar.

## Kullanım Durumları
- Şifreler veya gizli anahtarlar içeren kod blokları üzerinde çalışırken.
- Geçici araştırmalar yaparken.
- Asistanın kalıcı hafızasını kirletmek istemediğiniz rastgele sohbetlerde.

## Nasıl Aktif Edilir?
Sohbet arayüzündeki "Göz" ikonuna tıklayarak veya ayarlardan "Incognito Mode" seçeneğini açarak aktif edilebilir. Aktif olduğunda arayüzde belirgin bir görsel uyarı görünür.

### Bağlantılı Notlar:
- [[RAG ve Semantik Hafıza]]
- [[Veri Katmanı ve Kalıcılık]]
