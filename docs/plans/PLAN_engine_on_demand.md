# PLAN — Motorları kurulumda R2'den çekme (yalnızca tag sürümleri)

> Analiz ve ölçümler: konuşma artefaktı `engine_on_demand_analysis.md` (17 Kasım 2026).
> Beta (her `main` push'u) **değişmez**: tam paket, gömülü motor, bugünkü gibi.

## Kullanıcı kararları (17 Kasım 2026)

1. İndirme **iki yerde**: Windows'ta Inno Setup sihirbazı kurulum sırasında indirir; tüm platformlarda Memo'nun ilk açılış sihirbazı eksik olanı indirir (AppImage/tar.gz/zip/mac ve Inno'nun indiremediği durumlar).
2. data.memocpp.com sabit dosyaları (`memo.tar.gz`, `memo.AppImage`, `memo.exe`, `memo-mac.zip`, `memo_arm.zip`) **slim** olur. GitHub Release'e ek olarak **offline/full** paket konur.
3. Whisper modeli (`ggml-small.bin`, 488 MB) **yalnızca sesli özellik ilk açıldığında** iner. Hiçbir motor klasöründe durmaz.
4. **`cpu` her zaman kurulur.** GPU klasörü yalnızca gerekiyorsa kurulur: AMD → `amd` (ROCm yalnızca kart destekliyorsa; yoksa `cpu` içindeki Vulkan yeterli), NVIDIA → `nvidia`, Apple Silicon → şimdilik `cpu` (Metal kaldırıldı, MLX sonraki planda). Gereksiz paket kurulmaz.
   Tespit **iki aşamalı**: sistem otomatik tespit eder, ardından sihirbaz kullanıcıya doğrulatır ("Ekran kartınız AMD Radeon RX xxxx (ROCm) mu?" / "NVIDIA xxx mi?" / "MacBook M-serisi mi?"). Kullanıcı düzeltebilir; onaydan sonra kurulur.
5. Motor sürümü: önce beacon'daki en güncel bileşen; `requires_app` uymazsa ya da motor açılmazsa **sürümün kendi bilinen-iyi motoruna** düşülür.
6. Windows ve macOS motorları güncel llama ile yenilenip bileşen olarak yayımlanır. **Her R2 yazımından önce kullanıcıya sorulur.**
7. **Tüm motorlar en son sürüme** güncellenir: llama.cpp, whisper.cpp, CLIProxyAPI.
8. **Metal kaldırılır.** `darwin/metal` güncellenmez ve R2'deki eski kopyaya dokunulmaz. macOS'ta GPU yolu olarak ileride **MLX** gelecek. Bu planda **MLX için kod değişikliği yok**, yalnızca MLX binary'leri R2'ye gönderilir.
9. **R2 yönetimi:** `/home/bugra/Documents/r2-memo-push/upload-memo.sh` scriptinin kurduğu `memo-r2` rclone remote'u (bucket `memo`) kullanılır.

## R2 erişimi ve uyarı

- Remote: `memo-r2:memo` (`~/.config/rclone/rclone.conf` içinde tanımlı; R2'deki `binaries/darwin/{cliproxy,cpu,metal}` listesi okunarak doğrulandı).
- **`upload-memo.sh` olduğu gibi ÇALIŞTIRILMAZ.** Script `/home/bugra/Documents/r2-memo-push` klasörünün tamamını `rclone copy` ile bucket köküne kopyalıyor. O klasörde 7 Ekim tarihli eski `get-memo*.sh`/`update.sh`/`uninstall*.sh` kopyaları ve `Memo-linux-x64-v4.2.0.AppImage` var; script çalışırsa CI'nin yüklediği güncel scriptlerin üzerine eskileri yazar. Bunun yerine yalnızca hedefli komutlar kullanılır: önce `rclone copy` ile bucket içinde yedek, sonra `rclone sync --dry-run --checksum`, ardından gerçek `sync` (memo-release skill'i, adım 5).

## Upstream sürümleri (17 Kasım 2026, `gh release list` ile okundu)

| Bileşen | Şu an Memo'da | En son | Not |
|---|---|---|---|
| llama.cpp | linux b12110, linux/nvidia b9441, windows (31 Mayıs, sürümü kayıtlı değil), darwin (kayıtlı değil) | stable `v1.0.0` (13 Kasım) → bNNNN `nightly-tag.txt` ile okunacak; en yeni nightly b12470 | b12470 assetlerinde artık `ubuntu-cuda-12.8-x64` ve `ubuntu/win-rocm-10.0-x64` var, yani **linux/nvidia güncellenebilir** |
| whisper.cpp | 1.9.5 | v1.9.5 | zaten güncel; yalnızca Windows/macOS klasörlerine marker gerekiyor |
| CLIProxyAPI | beacon v8.0.59 (PINNED.txt'e bakılacak) | v8.0.76 | |
| MLX | — | `ml-explore/mlx` v0.32.4, `mlx-lm` v0.32.2 | paketleme biçimi açık soru (aşağıda) |

## Açık sorular

- **MLX binary biçimi:** MLX ve mlx-lm Python paketi olarak dağıtılıyor (PyPI: `mlx`, `mlx-metal`, `mlx-lm`). Hazır tek dosyalık bir sunucu binary'si olup olmadığı doğrulanmadı. R2'ye ne konacağına karar verilmeli: wheel dosyaları mı, gömülü Python + mlx-lm içeren self-contained bir paket mi, yoksa başka bir MLX sunucusu mu?
- **macOS'ta llama:** resmî `llama-bNNNN-bin-macos-arm64` derlemesinde Metal gömülü olarak geliyor olabilir (doğrulanmadı). "Metal'i kaldır" kararı macOS arm64'te llama'yı yalnızca CPU ile mi çalıştıracak, yoksa yalnızca `darwin/metal` klasörünü mü kaldıracak?
- `darwin/metal` R2'den silinsin mi, yoksa yalnızca bırakılıp güncellenmesin mi?

## Adımlar (her adım ayrı oturum + test + commit)

- [ ] **Adım 0 — Tüm motorları en son sürüme yükselt ve R2'ye yayımla (Metal hariç, MLX eklenir).**
  - CLIProxyAPI → v8.0.76: `PINNED.txt` güncellenir, sonra `vendor_cliproxy.sh`, `verify_cliproxy.sh`, gerçek binary testi, R2 ve `publish_component.sh` (skill adım 1).
  - llama.cpp → stable v1.0.0'ın bNNNN'i: `linux/cpu|amd|cpu-arm64`, **`linux/nvidia` (CUDA 12.8)**, `windows/cpu|amd(vulkan)|nvidia(cuda)`, `darwin/cpu`. Gerçek modelle (Phi-3 + nomic) test edilir; Windows/macOS için CI job'ı ya da fiziksel kabul. Marker'lar ve `ENGINES.txt` kanıtı eklenir.
  - whisper.cpp 1.9.5: Windows/macOS klasörlerine marker eklenir.
  - `darwin/metal`: **güncellenmez.**
  - MLX: açık sorudaki biçim netleşince yalnızca R2'ye yüklenir (ör. `binaries/darwin/mlx/` + marker), uygulama kodu değişmez.
  - Beacon `components` en son adımda güncellenir. **Her R2 yazımından önce kullanıcıya sorulur.**

- [ ] **Adım 1 — Whisper modelini motor klasöründen ayır.**
  `ggml-small.bin` paket/bileşen arşivlerinden çıkar, R2'de ayrı bir dosyaya (sha256'lı imzalı küçük manifest) taşınır. Backend'e "ses modeli yoksa indir" uç noktası eklenir; `resolveModel` zaten `data/models/` içine bakıyor. Flutter tarafına ses açılırken ilerleme gösterilir (L10n TR+EN). Beta da bundan faydalanır ama beta paketi şimdilik modeli taşımaya devam eder (beta değişmez kuralı).
- [ ] **Adım 2 — GPU tespiti + "gereken klasörler" kararı (Go, saf fonksiyon + testler).**
  `RequiredEngineFolders(goos, arch, gpu)` → ör. `[cpu]`, `[cpu, amd]`, `[cpu, nvidia]`; macOS'ta şimdilik `[cpu]`. Linux'ta ROCm desteği: sysfs/`lspci` gfx kimliği → desteklenen gfx listesi; emin değilse `amd` kurulmaz, Vulkan'lı `cpu` kalır. Windows: WMI/DXGI adaptör adı. macOS: yalnızca `cpu` (MLX sonraki planda). Sonuç kullanıcıya gösterilecek insan-okur bir adla (`"AMD Radeon RX 6700 XT"`) döner.
- [ ] **Adım 3 — Motor bootstrap (Go backend, tüm platformlar).**
  İlk açılışta (ve her açılışta ucuz kontrol) gerekli klasörlerden biri yoksa: beacon → bileşen manifesti → `updater.InstallComponent` ile `<data>/components/binaries/<os>/<klasör>` altına kurulur. Fallback: binary'ye gömülü "bilinen-iyi" sürüm listesi (`ENGINES.txt` bu sürümün değerleriyle) → o sürümün manifesti. Ed25519 doğrulaması aynı kalır. REST: `GET /api/engines/plan` (tespit + gereken klasörler + boyutlar), `POST /api/engines/install` (kullanıcının onayladığı liste), ilerleme SSE.
- [ ] **Adım 4 — Flutter ilk açılış sihirbazı adımı.**
  "Ekran kartınız X mi?" doğrulama kartı (Evet / Hayır, değiştir: AMD-ROCm / NVIDIA / yalnızca CPU), arka planda indirme, diğer sihirbaz adımları sürerken ilerleme. Motor inmeden yerel model başlatılamaz; harici sağlayıcı ve abonelik akışları beklemez. Tüm metinler `L10n.t` (TR+EN).
- [ ] **Adım 5 — Windows/macOS motor bileşenlerini hazırla ve yayımla.**
  Adım 0'dan kalan Windows/macOS işleri (`windows/cpu|amd|nvidia`, `darwin/cpu`; metal yok) `vendor_llama.sh` üzerinden (gerekirse Windows/macOS CI job'ında) derlenir. Marker + `ENGINES.txt` kanıtı, gerçek model testi (Windows/macOS fiziksel kabul açık kalabilir). `linux/nvidia` Adım 0'da CUDA 12.8 ile yenilenir. **R2 yazımları kullanıcı onayıyla.**
- [ ] **Adım 6 — Inno Setup indirme sayfası (yalnızca tag `memo.exe`).**
  `CreateDownloadPage` ile Adım 3'teki manifestten `cpu` + gerekli GPU klasörü indirilir. GPU WMI ile tespit edilir ve sihirbaz sayfasında doğrulatılır, sha256 `GetSHA256OfFile` ile kontrol edilir, dosyalar `{commonappdata}\Memo\data\components\...` altına açılır. İndirme başarısızsa kurulum sürer ve Memo ilk açılışta (Adım 3) tamamlar. Sessiz (`/VERYSILENT`) uygulama içi güncellemede indirme yapılmaz; uygulama halleder.
- [ ] **Adım 7 — Linux kurulum scriptleri.**
  `engine_refresh.sh` "eskiyse güncelle" davranışına "eksikse kur (cpu + gerekli GPU)" davranışını da kazanır. Etkileşimli terminalde GPU doğrulama sorusu sorulur, etkileşimsizde otomatik tespit kullanılır. Altı kopya `TestInstallers_EmbedTheSameEngineRefreshBlock` ile senkron kalır.
- [ ] **Adım 8 — CI: tag'de slim + offline/full paket.**
  `build-{linux,windows,macos}.yml`: aynı derleme; tag ise `stage-slim` (motor klasörleri ve whisper modeli yok; `vec0`, yapılandırma ve `memo-updater` var) → R2 sabit dosyaları + güncelleme manifesti. `stage-full` → yalnızca GitHub Release (`*-offline.*`). Beta adımlarına dokunulmaz. `memo-release` skill'i ve `AGENTS.md` güncellenir.
- [ ] **Adım 9 — Eski sürümlerden yükseltme güvenliği.**
  5.0/5.1 → slim arşiv güncellemesi `SwapItems` içindeki `binaries`'i değiştirir ve gömülü motorlar gider; ilk açılış bootstrap'i (Adım 3) motoru indirir. Bunun için e2e testi yazılır. Windows: eski `binaries\windows\{cpu,amd,nvidia}` için `[InstallDelete]` kararı. `PruneOverrides` slim kurulumda override'ları silmemeli (gömülü kopya yok → "covered" değil); bunun için test yazılır.

## Kapsam dışı

- Mobil (APK/IPA), Docker, beta yapıları.
- `ngrok` (31 MB) ve `cliproxy` (~72 MB) şimdilik paket içinde; Adım 8 sonrası ayrı karar.
