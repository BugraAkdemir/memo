# Handoff — güncel durum

> Her sohbette yüklenir; hedef <10 KB. Eski kayıtlar `handoff-archive.md`: yalnız hedefli ara. Eski son özeti arşiv başına taşı; açık işleri koru.

## Açık işler

**Yerel model hızı (2026-10-26, AGENTS.md "Local-model speed")**
- Eklenti `libggml-vulkan.so` yalnız yerel `binaries/linux/cpu`'da; R2'ye YAYINLANMADI → kullanıcılara ulaşmadı. Yayınlarken `vendor_llama.sh linux/cpu` + rclone + `publish_component.sh` ve **`--requires-app` bu kodu içeren sürüm olmalı**: eski uygulamada eklentili cpu klasörü Whisper'ı sessizce GPU'ya geçirir (`--no-gpu` yok) ve `engine_mode: cpu` GPU'yu kapatmaz. `~/.memo` (kurulu kopya) güncellenmedi.
- Doğrulanmadı: ayrık Vulkan GPU'da tam katman offload (yalnız APU vardı), `--n-cpu-moe` (MoE modeli yok), gerçek araç çağrısı üreten model + spekülasyon, Whisper'ı GPU'da gerçek konuşmayla (örnek ses yok → şimdilik `--no-gpu`), Windows/macOS (probe Linux'a kapalı), CUDA/ROCm donanımıyla spekülasyon.

**Alt ajan (2026-10-23, `PLAN_chat_subagents.md`, `docs/CHAT_SUBAGENTS.md`)**
- Canlı web UI alt ajan testi PASS (aşağıda); uzun iş/native pencere ölçülmedi. Açık: Self-Driving readonly komut sınırı (`docs/SUBAGENT_REVIEW.md`). Worker kuralları/routing/hata aktarımı düzeltildi (2026-10-24, `docs/TASKLOOP_PERFORMANCE_REVIEW.md`); gerçek Antigravity/Waydroid hız-kalite ölçümü ve bildirilen 300 dk geçmişi yok.

- **Yeni açık UI eksiği:** otomatik ana inceleme çalışırken sidebar spinner var ama bestecide Durdur yok. Bu nedenle canlı kesinti/sonraki sonuç testi ekrandan tamamlanamadı; düzeltilmedi.
- Alt ajan: kesilmiş inceleme artık sonraki sonuçları tutmaz (2026-10-24). Kendi incelemesi Resume main review ister; ana ajanın resume aracı yok. Çocuk kalıcılık hatası context canceled görünebilir; durum bloğu Minimal Mod’da da eklenir.

**Proje klasörleri (2026-10-19, `PLAN_project_folders.md`, `docs/PROJECT_FOLDER.md`)**
- P0–P5 bitti; proje hafızası Code Mode’dan bağımsız, hafıza anahtarlarına bağlı.
- Yapılmadı: proje skill'leri Flutter `/` menüsünde listelenmiyor (menü yalnız global skill'leri okur; backend `/ad` yazılınca proje skill'ini de tanır; `/skill add` var), proje izin düzenleme/onay arayüzü (yalnız dosya/API), `provider_policy` uygulaması (`local_only` vb. yalnız ayrılmış alan). Kullanıcı skill sistemini değiştirebileceğimizi söyledi; yeni tasarımın kapsamı belirlenmedi.
- Gerçekte doğrulanmadı: GitHub skill indirmesi (`codeload.github.com/<o>/<r>/zip/HEAD`), kurulum/onay penceresi, modelin `.memo/AGENTS.md` ve izinlere uyması, kullanıcı veritabanında göç, Windows/macOS Guard listeleri.
- `data/backups/memory-pre-project-id-*.db` yereldir; dışa/buluta aktarılmaz, otomatik silinmez.
- Live Mode'un ev köklü arka plan sohbeti normal sohbet sayılır; diğer ev köklü ajan sohbeti hafıza almaz. Code Mode kapalı proje sohbetinde takvim/WhatsApp araçları açık kalır; kişisel hafıza/kimlik izoledir.

**Birleşik sohbet listesi / yeni sohbet ekranı (2026-10-19)**
- Webde Ajan/klasör seçimi ve ilk mesaj sonrası alta geçiş denendi; native pencere, öneriler/animasyon ölçülmedi. Geçiş anlık; animasyon yok. Ajan sekmesinin eski IndexedStack indeksi boş tutulur; ajanlar ProjectPath ile işaretlenir.
- Ajan başlığı ilk kelimelerden; LLM başlığı Code Mode yan-çağrı yasağı için kullanıcı kararı ister. WelcomeView yalnız CLI.
- `/` menüsü artık skill menüsü (aşağıda); CLI sağlayıcısına özel `_CLICommandPopup` bu değişikliğe dahil değil.

**Agent akışı / REPL (`PLAN_agent_streaming.md`, `PLAN_cli_coding.md`)**
- Gerçek sağlayıcılarda akış+araç döngüsü, gerçek modelle REPL/skill kullanımı, Windows/macOS terminali, Flutter zaman çizelgesi ve Claude Code/opencode kalite-maliyet karşılaştırması doğrulanmadı (sahte SSE + fakeprovider + pty/widget testleri var).
- Gemini thoughtSignature yok. CLI `/` skill satırlarının etkileşimli seçimi test edilmedi. REPL hep ajan/kodlama; `/agent` anahtarı bilerek yok, global `memo agent on|off|auto` alt komutları var. `-chat <id>` düz sohbet açabilir; `/session` proje sohbetlerini listeler.
- Gerçek Subscriptions/Gemini akışı `streamed=true`, `cached_tok=0` gösterdi (önceden %85); CLIProxyAPI raporu mu gerçek maliyet mi bilinmiyor. Model araç yanında metin/düşünce üretmedi; anlatma kuralının etkisi ölçülmedi. Boş adım nudge ile denenir; kök neden açık.
- Log'da istenen `gemini-3.8-flash-high` yerine `gemini-3.8-flash-n` sunuldu; neden bilinmiyor, aynı-model karşılaştırmasını etkiler.

**Güncelleme (4.7.0 yayında, `PLAN_auto_update.md`, `docs/AUTO_UPDATE.md`)**
- Gerçek Windows Setup.exe/ShellExecute, macOS, GPU/model ile motor sonrası restart ve Flutter güncelleme penceresi doğrulanmadı. macOS betik kurulumunda başlatıcı yok → çoğu manual.
- Windows/macOS motor sürüm işareti yok; kanıtlı ENGINES.txt satırı eklenmeden motor güncellemesi almaz. Linux llama b11669 (nvidia b9441), cpu whisper 1.9.5 (2026-10-25).
- llama yükseltmesi 2026-10-25'te uygulandı (b11669); eski b11456 artıkları yeni klasörle gitti.
- 2026-10-25 motor güncellemesinden kalan: whisper-server klasördeki llama ggml arka uçlarını da yükler (llama yükseltmesinde yeniden dene); statik whisper AVX2+FMA+F16C varsayar (eski CPU'da çökebilir, ölçülmedi); amd klasörü ROCm 10 ile ~1,2 GB, gerçek AMD GPU ile denenmedi; issue #42-#47 açık; "indirdim, Yeniden başlat" onay adımı yok (komple güncelleme indirme bitince kendiliğinden kapanır); Docker daemon kapalıydı.
- İmza anahtarı `~/.config/memo-release/update-signing.key` ve Actions `MEMO_UPDATE_SIGNING_KEY`; anahtar geçişi eski anahtarla imzalı sürüm ister.

**Saha doğrulaması / sürüm hazırlığı**
- Canlı cihaz/OAuth/kota ve model kalitesi testlerle doğrulanmış sayılmaz; tamamlanan beta dışı işler: `PLAN_v4_6_completion.md`.
- Mobil akış (`feat/mobile-connect-flow`, 2026-10-12 main): gerçek cihaz/emülatör, MemoRoot sonrası provider yenileme, Android/iOS düz HTTP açık.
- Auto Route gerçek kalite/kota/GGUF soğuk yükleme; yeni karar/doğrulama modelinin gerçek kalitesi/hızı/maliyeti; kurulum sihirbazı ve abonelik giriş penceresi açık.
- Web sohbet Beta (`docs/WEB_CHAT.md`): giriş yapılmış ChatGPT/claude.ai hesabıyla hiç denenmedi; yalnız signed-out sayfalar ve sahte site.
- Gerçek uzak giriş token değişimi, HTTP tarayıcı kopyala execCommand yedeği, Telegram/WhatsApp, görsel üretimi, Flutter görsel/halka/yapıştırma/kota kartları açık. Antigravity gemini-3.1-flash-image bu hesapta 500 verdi. WhatsApp sahte aktarımı yok.
- Windows/macOS/arm64 sidecar yalnız CI build ile doğrulandı. Claude girişi yok: oauth/usage ve gerçek quota_exhausted kartı denenmedi. Codex bu hesapta yalnız 30d bildirdi; 5s/7g pencereleri denenmedi.

**Kullanıcıda / bilinen küçük noktalar**
- Embedding düzeltmesi: Pi üzerinde yeni build ile reboot/çökme henüz denenmedi. Eski `embedding_auto_start: false` otomatik ezilmez; yeni sürümde modeli bir kez elle açmak tercihi kaydeder.
- Eski origin push URL'sinde düz token/satılmış alan adı vardı; son kayıtta yalnız GitHub görünüyordu, token döndürme kullanıcıda. Eski REPL update / download.bugradev.com/get-memo.sh başkasına gider; eski satırları kaldırıp sürümü duyur.
- Yapıştırma kutusu Done'a kadar açık; login.go sonuç goroutine'inin s.tail'i erken okuma ihtimali ölçülmedi.
- Minimal Mod auto-compact atlar; görsel modeli seçicide etiketsiz. İkinci model menüsü ortak liste/arama/kaydırma kullanır.
- Test izolasyonu: DataDir önbelleği sıfırlanmadan MEMO_DATA_DIR değiştiren testler silinmiş temp klasörünü yeniden oluşturup dört dosya sızdırabiliyor; config.ResetForTests eksikleri açık.
- Yerel test ortamı: `ngrok.TestFetchURL_NoServer` gerçek yerel API’den URL alınca düşüyor; tünele dokunma. Yoğun ilk araç-zinciri derlemesinde `wav_player_test.dart` stop testi 2s zaman aşımı aldı (bu oturumda oynatıcı kodu değişmedi).
- Kararsız testler: Tailscale RebindsFromLoopback eski 8/150, yeni 2/150; update TamperedDownloadChangesNothing eski 5d068ac1’de de düşer (bu oturumda da uzunluk/checksum beklentisi); livemode SendAudioRoutesToCurrentInner; BootstrapTokenAuth temp temizliği; DuckDuckGo captcha; webchat PressingItAgainReplacesAWaitingAttempt (Connecting erken; ölçülmedi). Yükte eski/yeni aynı 24 döngüyle ölç; yeşile kadar koşma.

**Skill çağrısı / durdurma (2026-10-22)**
- Gerçek pencerede doğrulanmadı: `/` menüsü (cümle ortası, imleç/IME), durdurma sonrası akışın kalması, skill satırları. Gerçek modelin `use_skill`'i doğru seçmesi ölçülmedi. "gerekli skilleri kullan" kelime eşleşmesiyle en çok 3 skill seçer (model çağrısı yok), kaba.
- Bilinen boşluklar: resim/dosya ekli gönderimde skill tespiti yok; yerel modelin token bütçesi skill'i prompt'tan düşürürse "kullanılıyor" satırı yine görünür; hata/zaman aşımı yolunda kayıtlı cevap `skill_used` satırını kaybeder (canlı akışta görünmüştü); proje skill'leri Flutter menüsünde yok (backend `/ad` ile tanır); durdurma işareti sunucu kopyasında hep Türkçe (Flutter anlık kopyası arayüz dilinde).
- `/skill:on|off|add|remove` ve REPL `/skill` duruyor; masaüstünde açma/kapama arayüzü yok.

---

## Kalıcı uyarılar
- `~/.config/Memo/image.key` kaybında yalnız data kopyası görselleri açamaz. Parolalı taşınabilir/şifreli bulut yedeği ana anahtarı kopyalamadan taşır; önceden alınmış yedek kurtarabilir. Eski sürüm mühürlü görselleri bozuk görür.
- Kota dolunca otomatik devam yalnız Memo açıkken; CLI ajan/WhatsApp/Self-Driving kapsam dışı. Görsel niyet anahtar sözcüklü; yanlış tetikte negatif test ekle, `/image` zorlar.
- Sidecar soğuk liste ~30s. Bağlam/effort tam model id kaydından; çevrimdışı son snapshot. Bilinmeyen kapasite tahmin edilmez (`docs/SUBSCRIPTION_CAPABILITIES.md`).
- Beta kapısı: open_app + yedi browser_* aracı/paneli/session API, Beta kapalıyken yok; kapatmak oturumu bitirir. fetch_page fallback/Chromium kurulum bölümü kapısız. Gerçek pencerede Beta açık/kapalı görünümü doğrulanmadı. PROJECT_MAP/UI_FEATURES/ROADMAP EN/TR ve Obsidian'da bu kapıyı anlatmayan eski satırlar var.
- Mobil adlar: memo.apk/ipa (v*), memo-beta.apk/ipa (main; frontend/version/workflow değişiminde). Eski memo-android.apk/memo-android_beta.apk/memo-ios.ipa/memo-ios_beta.ipa R2'de kalmış, güncellenmez. Masaüstü: memo.AppImage/memo_beta.AppImage, memo.tar.gz/memo_beta.tar.gz, memo-mac.zip/memo-mac_beta.zip, memo.exe/memo-beta.exe, memo_arm.zip/memo_arm_beta.zip.
- Go sürümü go.mod + 7 workflow GO_VERSION birlikte yükseltilir; Docker golang:1.26-bookworm kayandır.

---

# Son oturum — 2026-10-26 — yerel model hızlandırma
- **Kök neden:** auto modda `resolveBinary` ilk klasörü (amd, nvidia, cpu) seçiyordu; bu APU'da `amd` (ROCm) hiç GPU görmüyor → her şey CPU'da. Şimdi her build'e `--list-devices` sorulur (`gpuSelector`, `internal/llama/device_probe.go`); yalnız Vulkan gören klasör "yerel GPU" sayılmaz. `nvidia` (b9441) klasörü de Vulkan taşıyor ve burada Vulkan0 görüyor; gerçek üç klasörlü düzende doğrulandı.
- **Ölçüm (Ryzen 5 5600H APU, Phi-3 Q4, b11669):** `linux/cpu`'ya `libggml-vulkan.so` → 1641 token prompt eval 65,7 → 146 t/s (ilk token ~25 → ~11 sn), decode aynı (~13-15). Vulkan-yalnız GPU'da katman 0 (8 katman 10,5 t/s'ye düşürüyordu), embedding ve `engine_mode: cpu` için `--device none` (Vulkan'da vektörler CPU'dan farklı, cosine 0,991-0,994), Whisper `--no-gpu`.
- **Spekülatif decode:** `--spec-type ngram-simple` (n=16, min-hits=2): kod kopyalama 14,6 → ~50 t/s, düz metinde fark yok; varsayılan ayarlar düz metni %8-11 yavaşlattı. `MEMO_LLAMA_SPECULATIVE=off` ile kapanır.
- **Ölçülüp reddedildi:** 12 thread, `--threads-batch`, ubatch, CPU'da q8_0 KV ve flash attention.
- **Commit'ler (push edilmedi):** `b9d442a7`, `ea2dbab4` (vendor_llama.sh), `a7692f38`, `694ef6d2`, `aa0ae468` (+ bu doküman commit'i). Doğrulama: RED/GREEN testler, mutasyon kontrolü, gerçek `Server.Start`/`whisper.Server.Start` koşuları, `go build/vet` + `go test ./... -race`. Tam koşuda yalnız `TestRunComponentUpdate_TamperedDownloadChangesNothing` düştü: temiz `7eb50de6`'da da 15'te 2 düşüyor (önceden var). `TestEmbeddingLifecycle` `internal/app/.starts` dosyası sızdırıyor (silindi; test izolasyonu açığı).
- **Araçlar:** ruflo/claude-flow MCP araçları bu oturumda bağlanmadı (yalnız ruflo-console arayüzü var), `ecc:go-reviewer` ajanı kullanıldı; bulguları (önbellek süresi, klasör mtime, Linux kapısı, mode cpu kapsamı) düzeltildi.
