# 📦 Derleme ve Paketleme

Memo'yu son kullanıcı için hazır hale getirmek için otomatik paketleme betikleri kullanılır.

## Linux Paketleme
Kök dizindeki `package_linux.sh` betiği şu işlemleri yapar:
1. **Backend Derleme:** Go kodunu `memo` binary'si olarak derler.
2. **Frontend Derleme:** Flutter projesini `release` modunda derler.
3. **Dosya Hazırlığı:** `build_output/memo-linux-x64/` klasörü altında tüm gerekli dosyaları (config, data, assets) toplar.
4. **Starter Script:** `run_memo.sh` dosyasını oluşturur. Bu script arka planda backend'i açar ve frontend'i başlatır.

### Çalıştırma:
```bash
./package_linux.sh
```

## Windows Paketleme
`package_windows.sh` betiği (henüz tam destek aşamasında) benzer mantıkla `.exe` çıktılarını hazırlar.

## Dağıtım Formatları
- **Portable Folder:** Tüm bağımlılıkların içinde olduğu klasör.
- **AppImage (Planlanan):** Tek dosyada çalışan Linux paketi.

### Bağlantılı Notlar:
- [[Geliştirici Kurulum Rehberi]]
- [[Sistem Genel Bakış]]
