# Handoff — güncel durum

> Eski ayrıntılar: `handoff-archive.md`; yalnız hedefli ara. 11 Kasım sıkıştırma öncesi açık işler de orada.

## Açık işler

**mmproj / Windows frontend & masaüstü sahipliği — 11–14 Kasım**
- `docs/plans/PLAN_desktop_tray_lifecycle.md` Adım 1–5 `main` üzerinde tamamlandı (`9d9e3d0d`, `635a0cf6`, `d6ea5333`, `b209b35c`): Windows native tray, `StartupTiming`, normalleştirilmiş gömülü motor çözümü, çevrimdışı `Inter`/`SchibstedGrotesk` TTF fontları, `AppShell` `_visitedTabs` tembel sekme önbelleği + `CalendarReminderSync` + `--desktop-session` / `--owned` pipe-EOF oturum gözetmeni (`internal/procjob/session.go`, `EnsureSelf` `BREAKAWAY_OK` Job Object, Unix `-pgid`, `DetachedAttr`, `shutdownPreEngineTimeout`, Windows/Linux/macOS başlatıcıları `pkill`/`taskkill` olmadan).
- Açık kabul: fiziksel Windows (cold/warm `StartupTiming`, Explorer/overflow/mascot, crash/Task-Kill) ve fiziksel macOS (Force Quit + `open_app`/updater) kabulü, artı güncel çalışma için uzak CI koşusu. Linux/mac X-to-tray varsayılanı host doğrulamasına dek eski hâlinde; kayıtlı tercihler korunur.
- `docs/MMPROJ_WINDOWS_STARTUP_REVIEW.md` içindeki mmproj indirme/onarım ve Windows frontend başlangıç bulguları `main` üzerinde karşılandı (gerçek HF “ofical gemma 4ab” canlı kabulü yok). Önceden mevcut 3 sohbet testi (`TestIntegration_LongHistoryGetsCompacted`, `TestChat_StreamCutMidAnswerKeepsWhatTheUserSaw`, `TestChat_EmptyReplyIsExplainedNotSavedBlank`), `internal/updater` içindeki izlenmeyen `scripts/rust-ai-slides/node_modules` test çarpması ve `GO-2026-6859` için Go `1.26.10`→`1.26.11` (`go.mod` + workflow/Docker birlikte) güncellemesi ayrı açık.

**Tanıtım filmi / Reels**
- 4 Python/Rust Reels ve paket: build_output/marketing/python-rust-reels/; son kareler/altyazı/ZIP doğrulandı, kullanıcı geri bildirimi bekleniyor. Plan docs/plans/PLAN_python_rust_reels.md; önceki oturum ayrıntısı arşivde.
- Memo tanıtım planı 5a6f35ba + tarih süzgeci e68c0482 önceki kayıtta push edilmemiş; bu oturumda kontrol yok. Telefon bölümü çizim, gerçek kayıt kullanıcıda (§6); Whisper/SRT açık.
- Demo #73/#74/#75 (#74 #onay 74), sohbet/hafıza/takvim, iki kutuda mail ve masaüstü fotoğrafları onaysız temizleme. Config önceki hâlinde, akdemirbugra14 full_auto (önceki kayıt).
- Önceki hatalar: izin timeout'u sağlayıcı hatası görünüyor; Phi-3 mini 4k/Türkçe; Gizli Mod alt ajan; çift takvim; boş veride onboarding completed belirsiz. Ayrıntı arşivde.

**Dijital İkiz / Mail**
- Günlük brifing kod/test bitti (7 Kasım), gerçek Telegram/WhatsApp/mail teslimi, model ve native sihirbaz/panel kabulü yok. Temiz veriyle kullanıcı denemesi bekleniyor.
- PLAN_mail_and_autonomous_system.md Adım9 dahil kod+canlı doğrulandı; açık kullanıcı kabulü docs/DIGITAL_TWIN_DEVICE_ACCEPTANCE.md A+B (Adım3–4), C+D (#onay/ses/WhatsApp/Telegram), radarın vade anı.
- Gmail 5↔14 + Subscriptions/gece E2E: docs/plans/PLAN_e2e_overnight.md. Ana ayarlar geri alındı; onay timeout15sn e16125d6. Giriş incelemesi “bak ve söyle”yi bir kez saldırı saydı. Ana TZ UTC; Istanbul önerilir, sihirbaz mail varken sorar.
- a7324752 SMTP gönderildi, eski pending kayda rağmen TEKRAR GÖNDERME; yeni akış geçmişten engeller. 453da7f6 farklı gönderen pending/Drafts; SMTP/skor yok, onay gerekir.
- Adım5 docs/DIGITAL_TWIN_AUTONOMY.md gerçek IMAP/model/Drafts/eski manuel SMTP/teslim PASS; varsayılan kapalı, ayrı model/yerel Whisper/ffmpeg/Telegram sahip.
- Adım6 docs/DIGITAL_TWIN_APPROVAL.md teknik/CI PASS; gerçek kanal/native/ses kabulü açık; iptal uzak Drafts/takvimi silmez, belirsiz gönderim tekrarlanmaz. Adım3–4 contacts/mail security cihaz kabulü açık; isim backfill yok, saldırı engeli PASS. Sonraki kullanıcı geri bildirimi.

**Performans / telemetri**
- Benchmark kullanıcı duraklattı (29 Ekim): docs/PERFORMANCE_BENCHMARK_CHECKPOINT.md + data/performance-benchmark/PAUSED.json. Kayıt aşama1, data aşama2; kalibrasyon/resume/aşama2–3/rapor açık. Runner baştan siler, körlemesine çalıştırma.
- Telemetri sunucusu /v1/install dahil doğrulandı (9 Kasım teyidi); yeniden açık sunma. Demo30cihaz silindi. Kullanıcıda Cloudflare token silme, DB/panel parola ve .env.local temizliği.
- Resmî derlemede paylaşım sorusuz açık, kayıtlı hayır korunur; kaynak kapalı, MEMO_TELEMETRY=off her şeyi engeller. Kimliksiz install paylaşım kapalıyken de gider; ilk gün rapor yok. docs/TELEMETRY.md + PLAN_telemetry.md.
- Eksik: Whisper start/exit, Dart/motor crash, offered/downloaded, needs_newer_app/not_for_this_machine, yerel paketlerin official damgası, site gizlilik. v5.0.0 EN/TR notları 7 Kasım yazıldı, tag tarihinde güncelle; varsayılan açık geçiş açıklanmalı. Canlı tel/native/Windows/macOS/tag damgası/ertesi gün yok; izole fake PASS.

**Yerel motor / alt ajan**
- libggml-vulkan.so yalnız yerel linux/cpu; R2/~/.memo yok. vendor_llama+rclone+publish_component; requires-app yeni kodlu sürüm olmalı (eski CPU/Whisper sessiz GPU'ya geçebilir).
- Linux cpu/amd/arm llama b11669, nvidia b9441, cpu whisper1.9.5; Whisper ggml arka uçlarını yükler, her yükseltmede dene. Statik Whisper eski CPU AVX2/FMA/F16C riski, ROCm10 (~1,2GB), ayrık Vulkan/MoE/CUDA/ROCm spekülasyon/gerçek araç modeli/Whisper konuşması/Windows/macOS probe ölçülmedi; --no-gpu kalır.
- Alt ajan web kısa PASS; uzun/native ve gerçek Antigravity/Waydroid kalite/hız, 300dk geçmişi kanıtsız. docs/CHAT_SUBAGENTS.md + PLAN_chat_subagents.md + TASKLOOP_PERFORMANCE_REVIEW.md. Ana inceleme spinner var Durdur yok; kesinti/sonraki sonuç eksik; kesilen inceleme sonraki sonuç tutmaz, Resume main review gerekir, ana ajan resume yok; çocuk kayıt context canceled olabilir.

**Proje / skill / akış / REPL**
- PLAN_project_folders.md P0–P5 bitti. Açık: Flutter proje skill / menüsü, proje izin UI, provider_policy, skill tasarım kapsamı.
- GitHub skill zip/onay, modelin .memo/AGENTS.md/izin uyumu, kullanıcı DB göçü, Windows/macOS Guard denenmedi; migration yedeği yerel, silinmez. Proje hafıza izolasyon kuralları AGENTS.md'de.
- Birleşik liste web PASS; native/öneri/animasyon yok. LLM sohbet başlığı kullanıcı kararı ister; şimdiki ilk kelimeler.
- Skill: / imleç/IME/cümle ortası, stop, use_skill canlı yok; adsız ≤3 kelime eşleme, model yok. Dosya/resim tespiti yok; prompt'tan düşen kullanılıyor kalabilir, hata/timeout skill_used kaybolabilir, stop TR. Toggle yok; /skill:on|off|add|remove + REPL /skill.
- PLAN_agent_streaming.md + PLAN_cli_coding.md: gerçek sağlayıcı/REPL/skill, Windows/macOS terminal, timeline ve Claude Code/opencode kalite/maliyet ölçülmedi; Gemini thoughtSignature ve CLI skill satır seçimi yok. REPL kullanım kuralları AGENTS.md'de.
- Subscriptions/Gemini streamed=true cached_tok=0 (önce %85), rapor/maliyet belirsiz; anlatım/düşünce yok, nudge etkisi açık; istenen flash-high yerine flash-n nedeni bilinmiyor.

**Güncelleme / saha / küçük işler**
- 4.7.0 yayında: docs/AUTO_UPDATE.md + PLAN_auto_update.md. Gerçek Windows Setup/ShellExecute, macOS, motor sonrası restart/Flutter yok; macOS script launcher yok. Windows/macOS ENGINES kanıtsız, işaret olmadan update yok. #42–47 açık, indirme bitince onaysız kapanır; Docker kapalıydı. İmza ~/.config/memo-release/update-signing.key + Actions; key geçişi eski imzalı sürüm ister.
- PLAN_v4_6_completion.md cihaz/OAuth/kota/model kalitesi belirsiz: mobil/emülatör, MemoRoot yenileme, HTTP, Auto Route kalite/kota/soğuk GGUF, karar/doğrulama hız/maliyet ve sihirbaz/abonelik login açık. Webchat yalnız fake/signed-out; gerçek signed-in yok. Uzak token/clipboard fallback/Telegram/WhatsApp/görsel/kota native yok; Antigravity image500, WA fake yok. Sidecar Windows/macOS/arm CI-only; Claude login/kota yok; Codex yalnız30d,5s/7g yok.
- Pi embedding build/reboot yok; eski embedding_auto_start:false ezilmez. Eski origin token kullanıcıda; satılan alan adındaki eski REPL indirmelerini kaldır/duyur. Yapıştırma Done'a dek açık; login s.tail erken okuma yok; Minimal auto-compact atlar, görsel modeli etiketsiz, ikinci menü ortak liste.
- MEMO_DATA_DIR testlerinde ResetForTests eksikleri sızdırabilir. ngrok NoServer yerel API yüzünden düşebilir, tünele dokunma. wav_player yoğun derlemede 2sn timeout gördü.
- Flaky: Tailscale Rebinds eski8/150 yeni2/150, livemode SendAudioRoutesToCurrentInner, BootstrapTokenAuth temp, DDG captcha, webchat PressingItAgain. Yerel yük testi/yeşile dek tekrar yasak. TamperedDownload 29 Ekim düzeltildi eski36/150 yeni0/150.

## Kalıcı uyarılar
- Alt ajan çalıştırma; açık talep olmadan PR açma. Backend+frontend birlikte doğrulanır, ağır doğrulama uzak CI'da; yerel CPU busy loop/stres yasak. Kullanıcı senaryo geri bildirimi beklenir; sonraki adıma kendiliğinden geçme.
- Görseller için ~/.config/Memo/image.key gerekir; yalnız data kopyası açamaz. Taşınabilir/şifreli bulut yedeği anahtarsız aktarır, eski yedek kurtarabilir; eski sürüm mühürlü görseli bozuk görür.
- Kota auto-continue yalnız Memo açık; CLI ajan/WA/Self-Driving dışı. Görsel niyeti kelime bazlı; negatif test, /image zorlar. Sidecar soğuk liste ~30sn, kapasite/effort tam id, offline snapshot; tahmin yok (SUBSCRIPTION_CAPABILITIES.md).
- Beta: open_app+7 browser_* araç/panel/session gizli, kapanınca session biter; fetch_page fallback/Chromium kurulum kapısız. Native yok; PROJECT_MAP/UI_FEATURES/ROADMAP EN/TR ve Obsidian eski satırlar açık.
- Mobil tag memo.apk/ipa, main memo-beta.apk/ipa; eski memo-android*/ios* R2 güncellenmez. Masaüstü memo.AppImage/memo_beta.AppImage, memo.tar.gz/memo_beta.tar.gz, memo-mac.zip/memo-mac_beta.zip, memo.exe/memo-beta.exe, memo_arm.zip/memo_arm_beta.zip.
- Linux tray_manager appindicator3/ayatana ister; paket değişirse frontend/build/linux CMake önbelleğini silip derle. Go go.mod +7 workflow birlikte; Docker1.26-bookworm kayar. v* tag push her defasında açık kullanıcı talebi ister.

---

# Son oturum — 14 Kasım — Masaüstü başlangıç ve süreç sahipliği (Adım 2–5)
- `main` üzerinde `PLAN_desktop_tray_lifecycle.md` Adım 2–5 tamamlandı: `9d9e3d0d` (Adım 2–3a `StartupTiming`), `635a0cf6` (Adım 3b motor yol çözümü + Windows kopya/sleep temizliği), `d6ea5333` (Adım 4 çevrimdışı TTF fontlar + tembel `AppShell` sekmeleri + `CalendarReminderSync`), `b209b35c` (Adım 5 `--desktop-session`/`--owned` pipe-EOF sahipliği, Windows `BREAKAWAY_OK` Job Object, Unix `-pgid`, `DetachedAttr`, `shutdownPreEngineTimeout`, Win/Linux/mac başlatıcıları).
- Doğrulama (`b209b35c`): `go build`/`go vet -tags "sqlite_fts5" ./...` PASS; `go test -race` `.`, `procjob`, `browseropen`, `agent/tools`, `llama`, `whisper`, `tts`, `database`, `cliproxy` + `updater` (`Apply|PlaceNew|Swap`) + `app` (`ClientRegistry|Shutdown`) PASS; `GOOS=windows/darwin` build + darwin vet PASS. Flutter: `analyze lib/` yalnız 3 eski info; `flutter test --concurrency=1` `+1035 ~1` PASS; L10n literal check temiz.
- Kapsam dışı/eski kırmızılar: `e2e` `TestChat_*` (2) ve `TestIntegration_LongHistoryGetsCompacted` (pre-existing); `updater` `Publish*/VerifyScript` (untracked `scripts/rust-ai-slides/node_modules`); `agent/tools` Windows test vet (`applaunch_test.go`). Fiziksel Windows/macOS cihaz testi ve `git push` kullanıcıda.
