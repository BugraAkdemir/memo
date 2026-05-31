# Strateji Değişimi: Bundled Llama Engine

Mevcut indirme tabanlı (download-on-demand) sistemden, tüm donanım varyasyonlarını içeren "tak-çalıştır" (bundled) sistemine geçiş planı.

## Artıları (+):
- **%100 Stabilite:** İnternet hızı, GitHub erişimi veya kütüphane bağlama (symlink) hataları tamamen ortadan kalkar.
- **Anında Kurulum:** Kullanıcı uygulamayı yüklediği anda motor hazırdır. "İndiriliyor" bekleme ekranı biter.
- **Offline Çalışma:** Uygulama ilk kez açıldığında bile internet gerektirmez.
- **Garantili Çalışma:** Senin sisteminde test ettiğimiz çalışan dosyaları gömdüğümüz için "Bende çalışmadı" şikayeti minimuma iner.

## Eksileri (-):
- **Dosya Boyutu:** Uygulama boyutu ~100 MB'tan ~700 MB - 1 GB arasına çıkar.
- **Bakım (Maintenance):** Llama.cpp güncellendiğinde her 3 binary setini de manuel güncellemek gerekir.

---

## YAPILACAKLAR

### 1. Dosya Yapısının Hazırlanması
- `[x]` `binaries/linux/` altında `cpu`, `nvidia`, `amd` klasörlerini oluştur.
- `[x]` `binaries/windows/` altında `cpu`, `nvidia`, `amd` klasörlerini oluştur.
- `[ ]` Çalışan kütüphane (`.so`/`.dll`) ve binary dosyalarını bu klasörlere yerleştir.

### 2. Backend Geliştirmeleri (`internal/llama/`)
- `[x]` `llama.go`: `resolveBinary` fonksiyonunu paket içindeki `binaries/` klasörüne öncelik verecek şekilde güncelle.
- `[x]` `installer.go`: İndirme mantığını "opsiyonel fallback" (yedek plan) haline getir veya tamamen kaldır.
- `[ ]` `gpu.go`: Algılanan donanıma göre varsayılan klasörü (cpu/nvidia/amd) seçme mantığını ekle.

### 3. Paketleme Scriptlerinin Güncellenmesi
- `[x]` `package_linux.sh`: `binaries/linux/` içeriğini `build_output` içine kopyalama adımını ekle.
- `[x]` `package_windows.sh`: `binaries/windows/` içeriğini dahil et.

### 4. Frontend UI Değişimi
- `[ ]` `llama_installer_view.dart`: "Motoru Kur" ekranını kaldır.
- `[ ]` `settings_dialog.dart`: "Motor Modu Seçimi" (Radio Button: Otomatik, CPU, NVIDIA, AMD) ekle.
- `[ ]` Mod değiştiğinde backend'e yeni yolu bildirip motoru anında yeniden başlatma tetikleyicisi koy.

### 5. Temizlik ve Test
- `[ ]` Eski `.force_cpu` mantığını "Sadece CPU" moduyla birleştir.
- `[ ]` `data/bin` klasörünü geçici cache olarak kullanmaktan vazgeç, paketlenmiş yola güven.
