# ☁️ Bulut Senkronizasyonu

Memo, verilerinizin cihazlar arası güvenli bir şekilde taşınması için Google Drive entegrasyonu sunar.

## Gizlilik ve Güvenlik: AES-256 E2E
Senkronizasyonun en kritik özelliği, verilerin buluta **asla açık halde gitmemesidir.**
1. **Yerel Şifreleme:** Veriler cihazınızdan çıkmadan önce sizin belirlediğiniz bir **Passphrase** (Parola) ile AES-256 standardında şifrelenir.
2. **Uçtan Uca (E2E):** Şifreleme anahtarı sadece sizdedir. Google bile verilerinizin içeriğini göremez.

## Neler Senkronize Edilir?
- Semantik Anılar (Memory)
- Kullanıcı Ayarları
- Sistem Promptları
- Sohbet Oturumları (İsteğe bağlı)

## Kurulum
1. Ayarlar > Cloud Sync sekmesine gidin.
2. Google hesabınızla giriş yapın.
3. Güçlü bir şifre belirleyin (Bu şifreyi kaybederseniz buluttaki verilere erişemezsiniz).

### Bağlantılı Notlar:
- [[Veri Katmanı ve Kalıcılık]]
- [[Mimari Yapı]]
