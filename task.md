# Yapılan İşlemler

## 1. Hata: Build sonrası llama.cpp bulunamıyor (`.memo/binaries/` oluşmuyor)

### Kök Neden
`build_releases.sh` içindeki `run_memo.sh` ve `AppRun` scriptleri `$HOME/.memo/binaries/` dizinini **oluşturmadan** `cp -r` ile çoklu dosya kopyalamaya çalışıyordu. `cp -r` hedef dizin yoksa sessizce başarısız oluyor, bu yüzden `llama-server` binary'leri `$HOME/.memo/binaries/linux/{cpu,nvidia,amd}/` altına hiç kopyalanmıyordu. Dev modunda (`flutter run -d linux`) CWD proje kökü olduğu için sorun yok.

### Yapılan Düzeltmeler

#### 1. `build_releases.sh` — RUNNER scripti (tar.gz/deb)
- `mkdir -p "$MEMO_HOME/binaries"` eklendi, `cp -r`den önce hedef dizin oluşturuluyor.

#### 2. `build_releases.sh` — AppRun scripti (AppImage)
- `mkdir -p "$MEMO_HOME/binaries"` eklendi, `cp -r`den önce hedef dizin oluşturuluyor.

#### 3. `internal/llama/llama.go` — `resolveBinary()` fonksiyonu
- Artık binary'yi önce CWD'de (`binaries/linux/...`), bulamazsa **çalıştırılabilir dosyanın kendi dizininde** (`/opt/Memo/binaries/linux/...` gibi) arıyor. Bu, shell script'teki kopyalama başarısız olsa bile backend'in kendi yanındaki binaries klasörünü bulmasını sağlar.
- `binarySearchBases()` helper fonksiyonu eklendi: `["."]` + `exeDir`.

## 2. Değişiklik: `.deb` paketi opsiyonel yapıldı

### Yapılan
- `BUILD_DEB`, `BUILD_APPIMAGE`, `BUILD_TARGZ` değişkenleri eklendi (varsayılan: deb=false, appimage=true, targz=true).
- `.deb` paketleme kısmı `if [ "$BUILD_DEB" = true ]` bloğuna alındı.
- AppImage ve tar.gz kısımları da kendi değişkenleriyle kontrol ediliyor.
