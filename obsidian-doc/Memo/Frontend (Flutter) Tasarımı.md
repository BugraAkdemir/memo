# 📱 Frontend (Flutter) Tasarımı

Memo'nun kullanıcı arayüzü, modern, akıcı ve profesyonel bir deneyim sunmak için Flutter kullanılarak geliştirilmiştir.

## Tasarım Dili: "Greige" Minimalism
- **Renk Paleti:** Gözü yormayan pastel bej ve gri tonları (Greige).
- **Tipografi:** Okunabilirlik odaklı modern yazı tipleri.
- **Layout:** Yan navigasyon rayı (NavRail) ile odaklanmış çalışma alanları.

## Teknik Stack
- **Framework:** Flutter (Linux & Windows Native).
- **State Management:** Riverpod (Reaktif ve test edilebilir durum yönetimi).
- **İletişim:** Dio (HTTP istemcisi) üzerinden Go Backend ile haberleşme.

## Ana Ekranlar
1. **Sohbet (ChatScreen):** Zengin metin desteği, akışlı mesajlar ve multimodal (görsel/dosya) girdi alanı.
2. **Model Deposu (ModelStore):** Hugging Face üzerinden model arama ve indirme yönetimi.
3. **Ayarlar (Settings):** Sistem promptu, hafıza parametreleri ve senkronizasyon kontrolleri.

### Reaktif Bileşenler
- **Thinking State:** Model yanıt verirken oluşan görsel geri bildirim.
- **Performance HUD:** Mesajların üzerine gelindiğinde görünen `tokens/sec` ve `ms` verileri.

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[Multimodal Yetenekler (Görsel ve Ses)]]
