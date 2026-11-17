# PLAN — Motorları kurulumda R2'den çekme (yalnızca tag sürümleri)

> Analiz ve ölçümler: konuşma artefaktı `engine_on_demand_analysis.md` (17 Kasım 2026).
> Beta (her `main` push'u) **değişmez**: tam paket, gömülü motor, bugünkü gibi.

## Kullanıcı kararları (17 Kasım 2026)

1. İndirme **iki yerde**: Windows'ta Inno Setup sihirbazı kurulum sırasında indirir; tüm platformlarda Memo'nun ilk açılış sihirbazı eksik olanı indirir (AppImage/tar.gz/zip/mac ve Inno'nun indiremediği durumlar).
2. data.memocpp.com sabit dosyaları (`memo.tar.gz`, `memo.AppImage`, `memo.exe`, `memo-mac.zip`, `memo_arm.zip`) **slim** olur. GitHub Release'e ek olarak **offline/full** paket konur.
3. Whisper modeli (`ggml-small.bin`, 488 MB) **yalnızca sesli özellik ilk açıldığında** iner. Hiçbir motor klasöründe durmaz.
4. **`cpu` her zaman kurulur.** GPU klasörü yalnızca gerekiyorsa kurulur: AMD → `amd` (ROCm yalnızca kart destekliyorsa; yoksa `cpu` içindeki Vulkan yeterli), NVIDIA → `nvidia`, Apple Silicon → `metal`. Gereksiz paket kurulmaz.
   Tespit **iki aşamalı**: sistem otomatik tespit eder, ardından sihirbaz kullanıcıya doğrulatır ("Ekran kartınız AMD Radeon RX xxxx (ROCm) mu?" / "NVIDIA xxx mi?" / "MacBook M-serisi mi?"). Kullanıcı düzeltebilir; onaydan sonra kurulur.
5. Motor sürümü: önce beacon'daki en güncel bileşen; `requires_app` uymazsa ya da motor açılmazsa **sürümün kendi bilinen-iyi motoruna** düşülür.
6. Windows ve macOS motorları güncel llama ile yenilenip bileşen olarak yayımlanır. **Her R2 yazımından önce kullanıcıya sorulur.**

## Adımlar (her adım ayrı oturum + test + commit)

- [ ] **Adım 1 — Whisper modelini motor klasöründen ayır.**
  `ggml-small.bin` paket/bileşen arşivlerinden çıkar, R2'de ayrı bir dosyaya (sha256'lı imzalı küçük manifest) taşınır. Backend'e "ses modeli yoksa indir" uç noktası eklenir; `resolveModel` zaten `data/models/` içine bakıyor. Flutter tarafına ses açılırken ilerleme gösterilir (L10n TR+EN). Beta da bundan faydalanır ama beta paketi şimdilik modeli taşımaya devam eder (beta değişmez kuralı).
- [ ] **Adım 2 — GPU tespiti + "gereken klasörler" kararı (Go, saf fonksiyon + testler).**
  `RequiredEngineFolders(goos, arch, gpu)` → ör. `[cpu]`, `[cpu, amd]`, `[cpu, nvidia]`, `[metal]`/`[cpu, metal]`. Linux'ta ROCm desteği: sysfs/`lspci` gfx kimliği → desteklenen gfx listesi; emin değilse `amd` kurulmaz, Vulkan'lı `cpu` kalır. Windows: WMI/DXGI adaptör adı. macOS: `arm64` → metal. Sonuç kullanıcıya gösterilecek insan-okur bir adla (`"AMD Radeon RX 6700 XT"`) döner.
- [ ] **Adım 3 — Motor bootstrap (Go backend, tüm platformlar).**
  İlk açılışta (ve her açılışta ucuz kontrol) gerekli klasörlerden biri yoksa: beacon → bileşen manifesti → `updater.InstallComponent` ile `<data>/components/binaries/<os>/<klasör>` altına kurulur. Fallback: binary'ye gömülü "bilinen-iyi" sürüm listesi (`ENGINES.txt` bu sürümün değerleriyle) → o sürümün manifesti. Ed25519 doğrulaması aynı kalır. REST: `GET /api/engines/plan` (tespit + gereken klasörler + boyutlar), `POST /api/engines/install` (kullanıcının onayladığı liste), ilerleme SSE.
- [ ] **Adım 4 — Flutter ilk açılış sihirbazı adımı.**
  "Ekran kartınız X mi?" doğrulama kartı (Evet / Hayır, değiştir: AMD-ROCm / NVIDIA / yalnızca CPU), arka planda indirme, diğer sihirbaz adımları sürerken ilerleme. Motor inmeden yerel model başlatılamaz; harici sağlayıcı ve abonelik akışları beklemez. Tüm metinler `L10n.t` (TR+EN).
- [ ] **Adım 5 — Windows/macOS motor bileşenlerini hazırla ve yayımla.**
  `windows/cpu|amd|nvidia`, `darwin/cpu|metal` güncel llama ile `vendor_llama.sh` üzerinden (gerekirse Windows/macOS CI job'ında) derlenir. Marker + `ENGINES.txt` kanıtı, gerçek model testi (Windows/macOS fiziksel kabul açık kalabilir). `linux/nvidia` için güncel CUDA derlemesi araştırılır. **R2 yazımları kullanıcı onayıyla.**
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
