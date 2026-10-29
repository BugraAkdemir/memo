# Handoff — güncel durum

> Hedef <10 KB. Ayrıntılar `handoff-archive.md`: yalnız hedefli ara. Açık işleri ve kalıcı uyarıları koru.

## Açık işler

**Telemetri / resmî sürüm öncesi (`docs/TELEMETRY.md`, `PLAN_telemetry.md`)**
- 2026-10-27 sözleşmesi: resmî derlemede günlük paylaşım yeni kurulum/güncellemede sorusuz varsayılan açık; sihirbaz/Ayarlar › Gizlilik kapatır, kayıtlı hayır ezilmez. Kaynak derleme kapalı. Ayrıca veri klasörü başına kimliksiz `/v1/install` günlük paylaşım kapalıyken de gider; kaynak derleme ve `MEMO_TELEMETRY=off` engeller. İlk varsayılan günü raporlanmaz.
- **Sunucu dağıtımı doğrulanmadı:** `~/Documents/admin.memocpp.com`: `db/migrations/001_installs.sql` → `tel/` Pages → `admin/` Pages. Handoff'taki son duruma göre dağıtılmamıştı; `/v1/install` 404 yeniden denenir. Resmî sürümden önce doğrula. 30 sahte demo cihaz 27 Ekim'de silindi.
- Kullanıcıda: Cloudflare belirtecini sil; DB/panel parolalarını değiştir; `.env.local` DB_PASSWORD/belirteç satırlarını temizle.
- Eksik bağlantılar: Whisper başlatma/çıkış; Dart/motor çökmesi; offered/downloaded güncelleme; needs_newer_app/not_for_this_machine nedenleri; yerel paketleme betiklerinin official damgası; site gizlilik sayfası; v5.0.0 notları. Eski “telemetri yok” sürümünden varsayılan açık geçiş yayın notunda açık yazılmalı.
- Gerçekte doğrulanmadı: tel.memocpp.com gönderimi, sihirbazın native görünümü, Windows/macOS davranışı, gerçek tag ile official damgası, gerçek ertesi gün geçişi. İzole gerçek ikili + sahte sunucuda kurulum/günlük sayaç/preview akışı doğrulandı (27 Ekim).

**Yerel hız / motorlar (26 Ekim, AGENTS.md “Local-model speed”)**
- `libggml-vulkan.so` yalnız yerel linux/cpu; R2 ve `~/.memo` güncellenmedi. Yayın: vendor_llama + rclone + publish_component; `--requires-app` yeni kodlu sürüm olmalı (eskide CPU tercihi/Whisper sessiz GPU'ya geçebilir).
- Ölçülmedi: ayrık Vulkan tam offload, MoE `--n-cpu-moe`, gerçek araç üreten model+speklasyon, CUDA/ROCm spekülasyon, gerçek Whisper konuşması (`--no-gpu` kalır), Windows/macOS probe.
- Linux llama b11669 (nvidia b9441), cpu whisper 1.9.5. Whisper llama ggml arka uçlarını da yükler: yükseltmede tekrar dene. Statik Whisper eski CPU'da AVX2/FMA/F16C riski ölçülmedi; AMD ROCm 10 klasörü ~1,2 GB gerçek GPU'da denenmedi.

**Alt ajan (`PLAN_chat_subagents.md`, `docs/CHAT_SUBAGENTS.md`)**
- Web UI canlı kısa test PASS; uzun iş/native ölçülmedi. Self-Driving readonly komut sınırı açık (`docs/SUBAGENT_REVIEW.md`). Gerçek Antigravity/Waydroid hız/kalite ve bildirilen 300 dk geçmişi yok (`docs/TASKLOOP_PERFORMANCE_REVIEW.md`).
- Otomatik ana incelemede sidebar spinner var, bestecide Durdur yok; canlı kesinti/sonraki sonuç testi tamamlanamadı. Kesilmiş inceleme sonraki sonuçları tutmaz; Resume main review gerekir, ana ajanın resume aracı yok. Çocuk kayıt hatası context canceled olabilir; durum bloğu Minimal Mod’da da eklenir.

**Proje / sohbet / skill**
- `PLAN_project_folders.md` P0–P5 tamam; hafıza Code Mode'dan bağımsız, hafıza anahtarlarına bağlı. Açık: Flutter `/` yalnız global skill, proje skill backend `/ad` ile çalışır; proje izin arayüzü yok; provider_policy yalnız ayrılmış alan. Skill tasarımı kapsamı belirlenmedi.
- Denenmedi: GitHub skill zip/kurulum onayı, modelin `.memo/AGENTS.md` ve izinlere uyması, kullanıcı DB göçü, Windows/macOS Guard. `data/backups/memory-pre-project-id-*` yerel kalır, otomatik silinmez.
- Live Mode ev köklü arka planı normal sohbet; diğer ev köklü ajan hafıza almaz. Code Mode kapalı projede takvim/WhatsApp açık, kişisel hafıza/kimlik izole.
- Birleşik sohbet listesi: webde ajan/klasör/ilk mesaj denendi; native/öneri/animasyon ölçülmedi, geçiş anlık. Eski ajan IndexedStack indeksi boş; ajan ProjectPath ile işaretli. Başlık ilk kelimelerden; LLM başlığı kullanıcı kararı ister. WelcomeView yalnız CLI. CLI'ye özel `/` popup değişmedi.
- Skill `/` menüsü (cümle ortası/imleç/IME), stop sonrası görünüm ve modelin use_skill seçimi canlı doğrulanmadı. “gerekli skilleri kullan” model çağırmadan kelime eşleşmesiyle en çok 3 seçer. Ekli dosya/resimde skill tespiti yok; prompt'tan düşen skill için “kullanılıyor” kalabilir; hata/timeout kayıtlı skill_used kaybolabilir; stop sunucu işareti Türkçe. Masaüstünde aç/kapa arayüzü yok; /skill:on|off|add|remove ve REPL /skill durur.

**Akış / REPL (`PLAN_agent_streaming.md`, `PLAN_cli_coding.md`)**
- Gerçek sağlayıcı araç döngüsü, modelle REPL/skill, Windows/macOS terminali, Flutter timeline ve Claude Code/opencode kalite/maliyet ölçülmedi. Gemini thoughtSignature yok; CLI skill satırı seçimi denenmedi.
- REPL hep kodlama; /agent yok; global memo agent on|off|auto var. -chat düz sohbet açabilir; /session proje sohbetleri.
- Subscriptions/Gemini canlı streamed=true, cached_tok=0 (önce %85): raporlama mı maliyet mi bilinmiyor. Araç yanında anlatım/düşünce gelmedi, boş adım nudge etkisi açık. İstenen gemini-3.8-flash-high yerine flash-n sunuldu; neden bilinmiyor.

**Güncelleme / saha**
- 4.7.0 yayında (`PLAN_auto_update.md`, `docs/AUTO_UPDATE.md`). Gerçek Windows Setup.exe/ShellExecute, macOS, GPU/model motor sonrası restart ve Flutter penceresi denenmedi; macOS betik kurulumunda başlatıcı yok. Windows/macOS ENGINES işareti kanıtsız: eklenmeden güncelleme almaz.
- issue #42–47 açık; indirme bitince kendiliğinden kapanır (yeniden başlat onayı yok); Docker daemon son denemede kapalıydı. İmza: `~/.config/memo-release/update-signing.key`, Actions MEMO_UPDATE_SIGNING_KEY; anahtar geçişi eski imzalı sürüm ister.
- `PLAN_v4_6_completion.md`: canlı cihaz/OAuth/kota/model kalitesi testlerle kanıtlanmaz. Mobil gerçek cihaz/emülatör, MemoRoot sonrası provider yenileme, Android/iOS düz HTTP açık. Auto Route kalite/kota/GGUF soğuk yükleme, karar/doğrulama modeli kalite/hız/maliyet, sihirbaz ve abonelik giriş penceresi açık.
- Web sohbet Beta yalnız signed-out/fake site; signed-in ChatGPT/Claude denenmedi. Uzak giriş token değişimi, HTTP clipboard fallback, Telegram/WhatsApp, görsel üretimi ve Flutter görsel/kota kartları canlı denenmedi. Antigravity flash-image hesapta 500; WhatsApp fake taşıması yok.
- Windows/macOS/arm64 sidecar yalnız CI build. Claude login/quota kartı yok. Codex yalnız 30d bildirdi; 5s/7g penceresi denenmedi.

**Bilinen küçük noktalar / test ortamı**
- Pi embedding yeni build/reboot denenmedi; eski embedding_auto_start:false ezilmez, elle açmak tercihi kaydeder. Eski origin token döndürme kullanıcıda; satılmış alan adındaki eski REPL indirmeleri kaldırılıp sürüm duyurulmalı.
- Yapıştırma kutusu Done'a dek açık; login.go s.tail erken okuma ölçülmedi. Minimal Mod auto-compact atlar; görsel modeli etiketsiz; ikinci model menüsü ortak listeyi kullanır.
- MEMO_DATA_DIR testlerinde config.ResetForTests eksikleri dosya sızdırabilir. ngrok.TestFetchURL_NoServer yerel API yüzünden düşebilir, tünele dokunma. Yoğun derlemede wav_player stop testi 2s timeout görmüştü.
- Açık kararsız testler: Tailscale RebindsFromLoopback eski8/150 yeni2/150; livemode SendAudioRoutesToCurrentInner; BootstrapTokenAuth temp temizliği; DuckDuckGo captcha; webchat PressingItAgainReplacesAWaitingAttempt. Yükte eski/yeni aynı 24 döngüyle ölç; yeşile dek koşma. TamperedDownloadChangesNothing 29 Ekim'de düzeltildi: eski36/150, yeni0/150.

## Kalıcı uyarılar
- `~/.config/Memo/image.key` olmadan yalnız data kopyası görselleri açamaz; taşınabilir/şifreli bulut yedeği anahtarı taşımadan aktarır, eski yedek kurtarabilir. Eski sürüm mühürlü görseli bozuk görür.
- Kota sonrası otomatik devam yalnız Memo açıkken; CLI ajan/WhatsApp/Self-Driving kapsam dışı. Görsel niyeti anahtar sözcüklü: yanlış tetikte negatif test; `/image` zorlar.
- Sidecar soğuk liste ~30s; kapasite/effort tam model id'sinden, offline son snapshot; bilinmeyen tahmin edilmez (`docs/SUBSCRIPTION_CAPABILITIES.md`).
- Beta kapısı open_app + yedi browser_* araç/panel/session API'yi gizler; kapanınca oturum biter. fetch_page fallback/Chromium kurulumu kapısız. Native görünüm denenmedi; PROJECT_MAP/UI_FEATURES/ROADMAP EN/TR ve Obsidian eski satırları açık.
- Mobil memo.apk/ipa (tag), memo-beta.apk/ipa (main); eski memo-android*/memo-ios* R2 adları güncellenmez. Masaüstü memo.AppImage/memo_beta.AppImage; memo.tar.gz/memo_beta.tar.gz; memo-mac.zip/memo-mac_beta.zip; memo.exe/memo-beta.exe; memo_arm.zip/memo_arm_beta.zip.
- Go sürümü go.mod + 7 workflow GO_VERSION birlikte; Docker golang:1.26-bookworm kayandır. v* tag push her seferinde açık kullanıcı talebi ister.

---

# Son oturum — 2026-10-29 — PR #52 merge edildi, CI yeşil
- https://github.com/BugraAkdemir/memo/pull/52 → main: `cc9fd58c` (21:44 İstanbul). Doğrulanan baş: `fb345a78`; PR CI 13 PASS, yayın adımı 1 SKIP. Yerel main eşitlendi; bu handoff ayrı yerel docs commit'i, push yok.
- Düzeltmeler: `e26644f6` Flutter analiz; `09746129` x/net v0.62.0 (GO-2026-6860) + aynı arşiv baytlarını bozan checksum testi; `e4f12e16` hatalı Status/login beklemesi `fb345a78` ile doğrudan sekme probe'una çevrildi. Son Go CI geçti.
- Yerel build/vet/race 09746129 içeriğiyle PASS; Flutter 848/848, hedefli11/11; lib analizinde3 eski info, tüm frontend --no-fatal-infos PASS; govulncheck0 erişilen açık. Arşiv testi aynı24 yükte eski36/150, yeni0/150.
- **Yük nedeniyle kullanıcı sistemi zorlandı:** iki ayrı denemede24'er busy-loop (aynı anda24) başlatıldı. Döngüler ve iki webchat stres testi durduruldu; kalan döngü0 doğrulandı. Webchat150 ölçümü TAMAMLANMADI. Yerelde yeni yük testi başlatılmayacağı söylendi; son doğrulama yalnız GitHub CI'da yapıldı.
- Sunucu dağıtımı/resmî yayın hâlâ ayrı açık iş. Kullanıcının AGENTS/.claude-flow değişiklikleri ve data/telemetry_state.json korundu. Tag/yayın yok.
