# 📦 Derleme ve Paketleme

Memo'yu son kullanıcı için hazır hale getirmek için otomatik paketleme betikleri kullanılır. Ana betik **`build_releases.sh`** repo kökündedir (eski `package_linux.sh`/`package_windows.sh` betiklerinin yerini aldı); diğer yardımcı script'ler `scripts/` dizininde toplanıyor (2026-08-09, root'un kalabalıklaşmaması için taşındı — bkz. `scripts/README.md`). Tüm script'ler repo kökünden çalıştırılır.

## `build_releases.sh`
1. **Backend Derleme:** Go kodunu `memo` binary'si olarak derler (`CGO_ENABLED=1`, `-tags "sqlite_fts5"` — bkz. [[CGO Bayrakları]]).
2. **Frontend Derleme:** Flutter projesini `release` modunda derler.
3. **Dosya Hazırlığı:** `build_output/` altında tüm gerekli dosyaları (config, data, assets) toplar.
4. **Paket Formatları (Linux):** `.tar.gz`, `.AppImage` (appimagetool ile), isteğe bağlı `.deb` (`dpkg-deb` varsa) — üçü de artık gerçekten üretiliyor, "planlanan" değil.
5. **Starter Script:** `run_memo.sh` dosyasını oluşturur; backend'i arka planda açar, frontend'i başlatır.

> **v3.3.4 düzeltmesi (geliştirme aşamasında):** Linux başlatıcısı (`run_memo.sh`) frontend'i yanlış çalışma dizininden başlatabiliyordu — düzeltildi.

## Windows Paketleme
Aynı betik `.exe` çıktılarını da hazırlar.

> **v3.3.4 düzeltmesi (geliştirme aşamasında):** Visual C++ Runtime kurulu olmayan temiz bir Windows makinesi Memo'yu hiç başlatamıyordu (`msvcp140.dll` eksik hatası). Installer artık Visual C++ Redistributable'ı gömüyor ve sessizce kuruyor.

## Beta Kurulum Betikleri (v3.3.3)
Beta build'leri stable'dan ayrı takip etmek isteyenler için ayrı, dedike betikler: `get-memo-beta.sh` (Linux/macOS) / `get-memo-beta.ps1` (Windows) — stable kurulum betiğinden bağımsız tutuluyor.

## macOS Sandbox Entitlement'ları (2026-08-05, aynı oturumda düzeltildi)
`frontend/macos/Runner/{Release,DebugProfile}.entitlements` dosyalarında `com.apple.security.network.client` eksikti — bu, macOS'ta yerel backend'e giden Dio çağrılarını App Sandbox seviyesinde engelliyordu ve gerçek bir kullanıcının bildirdiği "connection error" şikayetinin sebebiydi. Aynı zamanda `device.audio-input` (`record` paketi için mikrofon erişimi — Sesli Mod/STT'nin macOS'ta çalışması için gerekli) ve `files.user-selected.read-write` (`file_picker` için) de eksikti; `Info.plist`'te `NSMicrophoneUsageDescription` yoktu. Hepsi düzeltildi (commit `420e6a5`).

## AppImage ve CI yayın kanalları (v4.6.0)

`.github/workflows/build-linux.yml` içindeki Linux x86_64 job'u aynı hazırlanmış klasörden bir **tar.gz** ve bir **AppImage** üretir (`run_memo.sh` AppImage'ın `AppRun`'ıdır). AppImage, AppStream bilgisi (`com.memocpp.Memo`, `packaging/com.memocpp.Memo.metainfo.xml.in`'den üretilir) ve masaüstü kısayol bilgisi taşır; adı `Memo-v<sürüm>-x86_64.AppImage`.

| Tetikleyici | GitHub Release | `data.memocpp.com` |
|---|---|---|
| `v*` etiketi push'u | zip + `Memo-v<sürüm>-x86_64.AppImage` | `memo.tar.gz`, `memo.AppImage` (kararlı) |
| `main`'e her push | — | `memo_beta.tar.gz`, `memo_beta.AppImage` (beta) |

AppImage, tar.gz'nin kanal mantığını birebir izler. Linux arm64 hâlâ AppImage'sız zip olarak çıkar (`memo_arm.zip` / `memo_arm_beta.zip`). Etiketsiz bir derleme sürümünü `version` dosyasından, baştaki `V` atılarak alır.

## Dağıtım Formatları
- **Portable Folder / tar.gz:** Tüm bağımlılıkların içinde olduğu klasör/arşiv.
- **AppImage:** Tek dosyada çalışan Linux paketi — üretiliyor.
- **.deb:** `dpkg-deb` kuruluysa üretiliyor.

### Bağlantılı Notlar:
- [[Geliştirici Kurulum Rehberi]]
- [[Sistem Genel Bakış]]
- [[CGO Bayrakları]]
