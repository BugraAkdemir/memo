# PLAN — Uygulama içi güncelleme (v4.6.1) + bileşen güncellemesi

Amaç: yeni sürüm çıkınca kullanıcı uygulamayı açar, "güncelleme var" penceresini görür, tek tıkla günceller; siteyle uğraşmaz.
Ayrıca ana sürümü değiştirmeden yalnızca bir motor bileşenini (cliproxy / llama.cpp / whisper) R2'den yenilemek.

## Kararlar (kullanıcı, 2026-10-17)
- **Windows:** `memo.exe` (Inno Setup kurulumu) indirilir → Memo kapanır → kurulum başlar. UAC ve devamı kullanıcıda. Ayrı updater yok.
- **Linux / macOS:** otomatik; ayrı `memo-updater` süreci, bitince Memo yeniden başlar.
- **Ana sürüm güncellemesi = tam arşiv** (motor dahil, ~778 MB). Uygulama/motor ayrımı yapılmıyor.
- **Bileşen güncellemesi** (cliproxy, llama, whisper): ana `version` değişmeden, yalnız ilgili binary'ler R2'den; Memo yeniden başlamaz.
- Kod değişikliği gerektiren motor yükseltmesi **patch sürümle** çıkar (4.6.0 → 4.6.1); manifestte `requires_app`.
- Güncelleyicinin kendisi 4.6.1 ile gelir → **4.6.0 kullanıcıları son bir kez elle güncellemeli** (`get-memo.sh` / yeni kurulum).

## version.json (version-zeta.vercel.app) — yeni alanlar
Eski istemciler bilmediği alanları yok sayar (`CheckLatestVersion` yalnız `version` okur) → geriye dönük uyumlu.
```json
{
  "version": "V4.6.1",
  "mini_version_note_tr": "kısa not",
  "mini_version_note_en": "short note",
  "required_update": false,
  "components": { "cliproxy": "v8.0.26", "llama": "b9441", "whisper": "x.y.z" }
}
```
- `required_update: true` → "Bu sürümü atla" gizli, "Sonra" kalır. Atlanan sürüm cihaz tercihidir (SharedPreferences).
- Yalnız bileşen yamasında `version` DEĞİŞMEZ → eski istemcilere banner çıkmaz.
- Sürüm karşılaştırması gerçek semver; `!=` değil. Geri sürüm = güncelleme değil.

## Güven modeli (kullanıcının makinesine dosya indirip çalıştırıyoruz)
- `version.json` imzasızdır ve yalnız "hangi sürüm" der. URL, boyut ve sha256 **imzalı manifestten** gelir:
  `data.memocpp.com/update/app/<sürüm>/manifest.json(.sig)` ve `…/update/components/<ad>/<sürüm>/manifest.json(.sig)`.
- ed25519; açık anahtar binary'ye gömülü, özel anahtar CI secret'ında (**karar bekliyor, aşağıda**).
- Manifest: sürüm, `requires_app`, platform/mimari/flavor başına dosya + sha256 + boyut. İmza ya da sha256 tutmazsa indirme silinir, kurulum yapılmaz.

## Bileşen yerleşimi
- Sürüm dosyaları: `binaries/<os>/<flavor>/LLAMA_VERSION`, `WHISPER_VERSION` (cliproxy zaten `VERSION` taşıyor). `scripts/vendor_llama.sh` ve helper derlemesi yazar.
- Güncellenen dosyalar kurulum klasörüne YAZILMAZ (Windows'ta Program Files yönetici ister): `<DataDir>/components/binaries/<os>/<flavor>/…` (aynı ağaç).
  Aramada bu kök en başa eklenir; geri alma = klasörü silmek; ana sürüm güncellemesi gömülü sürüm ≥ override ise override'ı siler.
- **llama + whisper + lora yardımcıları tek paket** (Linux CPU'da `whisper-server` / `memo-lora-train` klasörün `libggml*.so`'larına bağlı — AGENTS.md "llama.cpp upgrade traps"). Tek başına llama değiştirmek ikisini açılmaz hâle getirir.
- `vec0` (sqlite-vec) backend süreci içine yüklenir → bu mekanizmaya dahil DEĞİL.

## Adımlar (her biri testli; oturum başına en çok 1–2)
- [x] 0. Karar: imza anahtarı GitHub Actions secret'ında (R2 anahtarlarıyla aynı yerde). Whisper sürümü: `libwhisper.so.1`'in kendi `whisper_version()` çıktısı **1.8.6** (ctypes ile okundu). Yerel llama-server: `--version` → 9441.
- [x] 1. `internal/components` (kurulu sürümü okur: override > gömülü, geçerli marker şartı, `Compare`: llama `bNNNN` sayısal, diğerleri noktalı) + `scripts/stamp_engine_versions.sh` (`stamp`/`check`, `--only OS/FLAVOR`, libllama build numarasıyla çapraz kontrol, iki geçişli yazma). **Henüz yapılmayan:** gerçek `binaries/`/R2'ye damga basmak (aşağıdaki "Sürüm bilgisi" notu) ve CI'a bağlamak (Adım 7).
- [ ] 2. `internal/updater`: manifest tipleri, ed25519 doğrulama, semver, downgrade reddi, sha256; anahtar üretme betiği. Testler: kurcalanmış manifest, yanlış imza, eski sürüm, sha256 uyuşmazlığı.
- [ ] 3. Backend: `CheckLatestVersion` semver'e + yeni alanlar; `GET /api/update/check` (not TR/EN, required, bileşen farkı); `POST /api/update/apply` + ilerleme (`adminOnly`). `/api/version/check` eski şekliyle kalır.
- [ ] 4. Bileşen güncelleyici: indir (Range ile devam, disk alanı denetimi), doğrula, `components/` altına hazırla, alt süreci durdur (llama-server / sidecar / whisper), yer değiştir, başlat, sağlık denetimi, başarısızsa geri al. Arama köklerine override ekle: `internal/llama`, `internal/whisper`, `internal/cliproxy` (Binary() bugün exe dizinini DataDir'den ÖNCE arıyor — override en başa).
- [ ] 5. `cmd/memo-updater` (CGO'suz, tüm platformlar için derlenir): Linux/macOS tam güncelleme — PID'leri bekle, eski kurulumu `.old` yap, yenisini yerleştir, config/data/models'a dokunma, yeniden başlat, sağlık denetimi, olmazsa geri al. Windows: backend `memo.exe`'yi indirip ayrık başlatır ve kapanır.
- [ ] 6. Flutter: açılışta güncelleme penceresi (TR+EN L10n, Kural #8), "Güncelle / Sonra / Bu sürümü atla", `required_update`, ilerleme, yeniden başlama durumu. Atlanan sürüm cihaz tercihi.
- [ ] 7. CI / sürüm süreci: manifest üretme + imzalama, `scripts/publish_component.sh` (R2'ye yükle + manifest), `memo-release` skill Faz 4'ün genişletilmesi (işaret en son bump'lanır).
- [ ] 8. Belgeler (`docs/`, AGENTS.md), `handoff.md`.

## Sürüm bilgisi: hangi klasör hangi llama.cpp'yi taşıyor (2026-10-17, yerel `binaries/` üzerinde ölçüldü)
- **Linux amd / nvidia / cpu-arm64: b9441** — `libllama.so.0.0.9441` (kanıtlı). **Linux cpu:** `llama-server --version` = 9441, ama klasörde yarım kalmış bir b11456 hazırlığından artıklar var (`libllama.so.0.0.1`, `libggml*.so.0.14.0`, `*.so.0-new`, `llama-new`); damgadan önce temizlenmeli.
- **Windows: muhtemelen b9441** — `ggml-base.dll` 0.13.1, Linux b9441'in ggml'i ile aynı. Kanıt değil (bir ggml sürümü birkaç llama build'ine denk gelebilir); damga basmadan önce kullanıcıdan/kaynak arşivden teyit.
- **macOS: b9441 DEĞİL** — dylib'ler ggml 0.15.3 (Linux b9441: 0.13.1). Tam tag bilinmiyor (2026-07-02 commit'i `291b549b`). Kanıtlanmadan damga basılmaz.
- **Sonuç:** llama sürümü klasör (platform/flavor) başınadır, ağaç başına değil. Bu yüzden `version.json`'daki `components.llama` bir **etiket/tetikleyicidir**; hangi klasörün neyi alacağını imzalı bileşen manifesti söyler (manifestte olmayan flavor güncellenmez — NVIDIA b9441'de kalır).
- `binaries/` git'te yok sayılıyor (yalnız macOS dosyaları zorla eklenmiş) → Linux/Windows için marker'ların kaynağı **R2**'dir; R2'ye yazmak kullanıcı onayı ister.

## Bilinen tuzaklar (kodda doğrulanmış)
- `llama.resolveBinary` arama tabanları üzerinde **dış döngü** olarak dolaşıp içte amd → nvidia → cpu bakıyor (`internal/llama/llama.go`, "Check Bundled Paths"). Yalnız `cpu` içeren bir override tabanı, makinede gömülü `amd` varken bile kazanır. Override eklerken sıra (flavor-major mı, mod önceden çözülüyor mu) testle kanıtlanmalı.
- Çalışan `llama-server`'ı durdurmak sohbeti keser → bileşen güncellemesi kullanıcıya sorulur (ana pencerede), `required` ise "Sonra" kalır.
- Uzak (non-loopback) hesap sunucuyu güncelleyebilir → uçlar `s.adminOnly(...)`.
- Windows: Setup `CloseApplications=yes` + sabit AppId zaten yerinde yükseltiyor; veri `ProgramData`'da kalır.

## Kapsam dışı / doğrulanamayacak
- AppImage ve deb: self-update yok; yalnız bildirim. Docker / headless: pencere yok, REPL `/update` ve API.
- Windows / macOS / arm64'te updater çalıştırma bu makinede doğrulanamaz; CI'da yalnız derleme. Handoff'ta "doğrulanmadı" yazılır.
- llama b9441 → b11456 geçişi ayrı iş (helper'ların yeniden derlenmesi gerekir); bu plan yalnız dağıtım mekanizması.
- Doğrulama: izole `MEMO_DATA_DIR`, yerel sahte "R2" HTTP sunucusu, sahte arşiv, `cmd/fakeprovider`, e2e. Gerçek uçtan uca yalnız Linux.
