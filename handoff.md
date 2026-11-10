# Handoff — güncel durum

> Eski ayrıntılar: `handoff-archive.md`; yalnız hedefli ara.

## Açık işler

**Dijital İkiz / Mail**
- **Günlük brifing (7 Kasım):** kod ve testler bitti, canlı kabul yok: gerçek Telegram/WhatsApp/mail teslimi, brifing modeli, sihirbaz/panel görünümü. Kullanıcı temiz veri klasörüyle gezip saati/kanalları denemeli. Ayrıntı aşağıda “Son oturum”.
- Plan (`docs/plans/PLAN_mail_and_autonomous_system.md`) kod ve canlı doğrulama olarak bitti (Adım 9 dahil, `docs/DIGITAL_TWIN_AND_MAIL.md`; gerçek Gmail `akdemirbugra5` ↔ `akdemirbugra14` + Subscriptions). Açık olan yalnız kullanıcı kabulü: Adım 3–4 kutuları (`docs/DIGITAL_TWIN_DEVICE_ACCEPTANCE.md` A+B), WhatsApp/Telegram `#onay` ve ses (C+D); canlıda radar vade anı gözlenmedi.
- Bilinen zayıflık: giriş incelemesi (LLM, eşik ≥0,5) doğal bir "bak ve söyle" cümlesini bir kez saldırı saydı. Ana Memo ayarlarında saat dilimi UTC (Istanbul önerilir; sihirbaz artık saat dilimi sorar, yalnız mail bağlıyken).
- 3 Kasım gece E2E (gerçek Gmail 5↔14): Adım 1–7 canlı koşuldu, 8 gerçek hata düzeltildi (son: `e947bd67`); sonuçlar `docs/plans/PLAN_e2e_overnight.md`. Açık: WhatsApp/Telegram/ses ve `#onay` cihaz kabulü (skorlayıcı zaman aşımı 15 sn'ye çıkarıldı, `e16125d6`); ana Memo ayarları geri alındı.
- a7324752 SMTP gönderildi/eski kayıt pending: TEKRAR GÖNDERME. Yeni akış geçmiş kontrolüyle engeller. 453da7f6 farklı gönderen pending/Drafts; SMTP yok, skor yok/onay gerekir.
- Adım5 teknik ve gerçek IMAP/model/Drafts/eski manuel SMTP/teslim kullanıcı PASS: docs/DIGITAL_TWIN_AUTONOMY.md. Varsayılan kapalı; ayrı yanıt modeli, yerel Whisper/ffmpeg, Telegram sahip.
- Adım6 teknik/CI PASS: docs/DIGITAL_TWIN_APPROVAL.md. Geri al/otomatik gönderim ve UI/WhatsApp/Telegram sahip onayı bağlı; eski adaylar açılmaz. Gerçek kanal/native/ses kabulü açık; cihaz test yönergesi aynı dokümanda. İptal uzak Drafts/takvimi silmez; belirsiz gönderim tekrarlanmaz.
- Adım4 kişi/üslup/Git/doluluk ve Adım3 görünüm/charset/sayaç/göreli tarih cihaz kabulü açık: docs/DIGITAL_TWIN_CONTACTS.md + docs/MAIL_SECURITY.md. İsim eşleme/backfill yok; saldırı engeli kullanıcı PASS, izin aynı. Sonraki adım kullanıcı geri bildirimiyle.

**Performans benchmark — kullanıcı duraklattı (29 Ekim)**
- docs/PERFORMANCE_BENCHMARK_CHECKPOINT.md + data/performance-benchmark/PAUSED.json: son koşuda yalnız aşama1 kayıtlı, data aşama2. Kalibrasyon/resume/aşama2–3/rapor/doğrulama açık. Runner baştan siler; körlemesine çalıştırma.

**Telemetri / resmî sürüm öncesi (`docs/TELEMETRY.md`, `PLAN_telemetry.md`)**
- 27 Ekim: resmî derlemede günlük paylaşım sorusuz açık; sihirbaz/Gizlilik kapatır, kayıtlı hayır korunur. Kaynak kapalı. Veri klasörü başına kimliksiz `/v1/install`, paylaşım kapalıyken de gider; kaynak ve MEMO_TELEMETRY=off engeller. Varsayılanın ilk günü raporlanmaz.
- **Sunucu (`/v1/install` dahil) dağıtıldı ve doğrulandı** — kullanıcı 9 Kasım'da teyit etti; bir daha açık iş diye sunma. 30 sahte demo cihaz 27 Ekim'de silindi.
- Kullanıcıda: Cloudflare belirtecini sil; DB/panel parolalarını değiştir; `.env.local` DB_PASSWORD/belirteç satırlarını temizle.
- Eksik bağlantılar: Whisper başlatma/çıkış; Dart/motor çökmesi; offered/downloaded güncelleme; needs_newer_app/not_for_this_machine nedenleri; yerel paketleme betiklerinin official damgası; site gizlilik sayfası. (v5.0.0 notları 7 Kasım'da yazıldı: `versinNote/v5.0.0.md` + `versinNote/tr/`; tarih tag anında güncellenmeli.) Eski “telemetri yok” sürümünden varsayılan açık geçiş yayın notunda açık yazılmalı.
- Canlı tel gönderimi, native sihirbaz, Windows/macOS, tag official damgası, ertesi gün geçişi denenmedi. İzole ikili+sahte sunucuda kurulum/sayaç/preview PASS (27 Ekim).

**Yerel hız / motorlar (26 Ekim, AGENTS.md “Local-model speed”)**
- `libggml-vulkan.so` yalnız yerel linux/cpu; R2 ve `~/.memo` güncellenmedi. Yayın: vendor_llama + rclone + publish_component; `--requires-app` yeni kodlu sürüm olmalı (eskide CPU tercihi/Whisper sessiz GPU'ya geçebilir).
- Ölçülmedi: ayrık Vulkan offload, MoE --n-cpu-moe, araç üreten model+speklasyon, CUDA/ROCm spekülasyon, gerçek Whisper konuşması (--no-gpu kalır), Windows/macOS probe.
- Linux llama b11669 (nvidia b9441), cpu whisper 1.9.5. Whisper llama ggml arka uçlarını da yükler: yükseltmede tekrar dene. Statik Whisper eski CPU'da AVX2/FMA/F16C riski ölçülmedi; AMD ROCm 10 klasörü ~1,2 GB gerçek GPU'da denenmedi.

**Alt ajan (`docs/plans/PLAN_chat_subagents.md`, `docs/CHAT_SUBAGENTS.md`)**
- Web UI canlı kısa test PASS; uzun iş/native ölçülmedi. Gerçek Antigravity/Waydroid hız/kalite ve bildirilen 300 dk geçmişi yok (`docs/TASKLOOP_PERFORMANCE_REVIEW.md`).
- Ana incelemede sidebar spinner var, Durdur yok; kesinti/sonraki sonuç testi eksik. Kesilmiş inceleme sonraki sonuçları tutmaz; Resume main review gerekir, ana ajanda resume yok. Çocuk kayıt hatası context canceled olabilir.

**Proje / sohbet / skill**
- `docs/plans/PLAN_project_folders.md` P0–P5 tamam; hafıza Code Mode'dan bağımsız, hafıza anahtarlarına bağlı. Açık: Flutter `/` yalnız global skill, proje skill backend `/ad` ile çalışır; proje izin arayüzü yok; provider_policy yalnız ayrılmış alan. Skill tasarımı kapsamı belirlenmedi.
- Denenmedi: GitHub skill zip/kurulum onayı, modelin `.memo/AGENTS.md` ve izinlere uyması, kullanıcı DB göçü, Windows/macOS Guard. `data/backups/memory-pre-project-id-*` yerel kalır, otomatik silinmez.
- Live Mode ev köklü arka planı normal sohbet; diğer ev köklü ajan hafıza almaz. Code Mode kapalı projede takvim/WhatsApp açık, kişisel hafıza/kimlik izole.
- Birleşik liste: web ajan/klasör/ilk mesaj denendi; native/öneri/animasyon ölçülmedi. IndexedStack eski ajan indeksi boş, ProjectPath ajan işareti. Başlık ilk kelimeler; LLM başlığı kullanıcı kararı ister. WelcomeView CLI; CLI `/` değişmedi.
- Skill: `/` cümle ortası/imleç/IME, stop görünümü, use_skill canlı denenmedi. Adsız istek kelime eşleştirerek ≤3 seçer, model çağırmaz. Dosya/resim skill tespiti yok; prompt'tan düşen “kullanılıyor” kalabilir; hata/timeout skill_used kaybolabilir; stop Türkçe. Masaüstü toggle yok; /skill:on|off|add|remove ve REPL /skill var.

**Akış / REPL (`docs/plans/PLAN_agent_streaming.md`, `docs/plans/PLAN_cli_coding.md`)**
- Gerçek sağlayıcı araç döngüsü, modelle REPL/skill, Windows/macOS terminali, Flutter timeline ve Claude Code/opencode kalite/maliyet ölçülmedi. Gemini thoughtSignature yok; CLI skill satırı seçimi denenmedi.
- REPL hep kodlama; /agent yok; global memo agent on|off|auto var. -chat düz sohbet açabilir; /session proje sohbetleri.
- Subscriptions/Gemini canlı streamed=true, cached_tok=0 (önce %85): raporlama mı maliyet mi bilinmiyor. Araç yanında anlatım/düşünce gelmedi, boş adım nudge etkisi açık. İstenen gemini-3.8-flash-high yerine flash-n sunuldu; neden bilinmiyor.

**Güncelleme / saha**
- 4.7.0 yayında (`docs/plans/PLAN_auto_update.md`, `docs/AUTO_UPDATE.md`). Gerçek Windows Setup.exe/ShellExecute, macOS, GPU/model motor sonrası restart ve Flutter penceresi denenmedi; macOS betik kurulumunda başlatıcı yok. Windows/macOS ENGINES işareti kanıtsız: eklenmeden güncelleme almaz.
- issue #42–47 açık; indirme bitince kendiliğinden kapanır (yeniden başlat onayı yok); Docker daemon son denemede kapalıydı. İmza: `~/.config/memo-release/update-signing.key`, Actions MEMO_UPDATE_SIGNING_KEY; anahtar geçişi eski imzalı sürüm ister.
- docs/plans/PLAN_v4_6_completion.md: cihaz/OAuth/kota/model kalitesi kanıtsız. Mobil cihaz/emülatör, MemoRoot provider yenileme, Android/iOS HTTP; Auto Route kalite/kota/GGUF soğuk yükleme, karar/doğrulama kalite/hız/maliyet, sihirbaz/abonelik giriş penceresi açık.
- Web sohbet Beta yalnız signed-out/fake site; signed-in ChatGPT/Claude denenmedi. Uzak giriş token değişimi, HTTP clipboard fallback, Telegram/WhatsApp, görsel üretimi ve Flutter görsel/kota kartları canlı denenmedi. Antigravity flash-image hesapta 500; WhatsApp fake taşıması yok.
- Windows/macOS/arm64 sidecar yalnız CI build; Claude login/quota yok. Codex yalnız30d; 5s/7g denenmedi.

**Küçük noktalar / test ortamı**
- Pi embedding build/reboot denenmedi; eski embedding_auto_start:false ezilmez, elle açmak tercihi kaydeder. Eski origin token döndürme kullanıcıda; satılmış alan adındaki eski REPL indirmeleri kaldırılıp sürüm duyurulmalı.
- Yapıştırma kutusu Done'a dek açık; login.go s.tail erken okuma ölçülmedi. Minimal Mod auto-compact atlar; görsel modeli etiketsiz; ikinci model menüsü ortak listeyi kullanır.
- MEMO_DATA_DIR testlerinde config.ResetForTests eksikleri dosya sızdırabilir. ngrok.TestFetchURL_NoServer yerel API yüzünden düşebilir, tünele dokunma. Yoğun derlemede wav_player stop testi 2s timeout görmüştü.
- Açık kararsız testler: Tailscale RebindsFromLoopback eski8/150 yeni2/150; livemode SendAudioRoutesToCurrentInner; BootstrapTokenAuth temp temizliği; DuckDuckGo captcha; webchat PressingItAgainReplacesAWaitingAttempt. Yerel yük testi çalıştırma; kullanıcı sistemi dondurduğunu bildirdi. Yeşile dek koşma. TamperedDownloadChangesNothing 29 Ekim'de düzeltildi: eski36/150, yeni0/150.

## Kalıcı uyarılar
- Alt ajan çalıştırma; açık talep olmadan PR açma. Backend+frontend birlikte doğrulanır; kullanıcı gerçek senaryo testiyle geri bildirim verir. Sonraki adıma kendiliğinden geçme.
- Yerel CPU dolduran busy loop/stres testi yasak; eski döngüler kapalı. Ağır doğrulama uzak CI’da.
- `~/.config/Memo/image.key` olmadan yalnız data kopyası görselleri açamaz; taşınabilir/şifreli bulut yedeği anahtarı taşımadan aktarır, eski yedek kurtarabilir. Eski sürüm mühürlü görseli bozuk görür.
- Kota sonrası otomatik devam yalnız Memo açıkken; CLI ajan/WhatsApp/Self-Driving kapsam dışı. Görsel niyeti anahtar sözcüklü: yanlış tetikte negatif test; `/image` zorlar.
- Sidecar soğuk liste ~30s; kapasite/effort tam model id'sinden, offline son snapshot; bilinmeyen tahmin edilmez (`docs/SUBSCRIPTION_CAPABILITIES.md`).
- Beta kapısı open_app + yedi browser_* araç/panel/session API'yi gizler; kapanınca oturum biter. fetch_page fallback/Chromium kurulumu kapısız. Native görünüm denenmedi; PROJECT_MAP/UI_FEATURES/ROADMAP EN/TR ve Obsidian eski satırları açık.
- Mobil memo.apk/ipa (tag), memo-beta.apk/ipa (main); eski memo-android*/memo-ios* R2 adları güncellenmez. Masaüstü memo.AppImage/memo_beta.AppImage; memo.tar.gz/memo_beta.tar.gz; memo-mac.zip/memo-mac_beta.zip; memo.exe/memo-beta.exe; memo_arm.zip/memo_arm_beta.zip.
- Linux Flutter derlemesi `tray_manager` yüzünden `ayatana-appindicator3-0.1` ya da `appindicator3-0.1` ister. Bu paket değişince (kurulum/kaldırma) `frontend/build/linux` CMake önbelleği bayatlar ve derleme “Imported target PkgConfig::APPINDICATOR includes non-existent path” ile düşer: `frontend/build/linux` silinip yeniden derlenir.
- Go sürümü go.mod + 7 workflow GO_VERSION birlikte; Docker golang:1.26-bookworm kayandır. v* tag push her seferinde açık kullanıcı talebi ister.

---

# Son oturum — 9–10 Kasım (gece) — pazarlama, demo video, tanıtım filmi
- **Yapılan:** GitHub About + 2 konu değişti (`gh repo edit`); 61 sn demo + EN/TR ElevenLabs seslendirmeli sürümler; README'lere GIF (`28cee1a7`, push edildi). Takvim düzeltmeleri `09676bd4` + `201ec021` (sekme indeksi 4→3 ve girişte yenileme, push edildi). Gece: tanıtım filmi planı `docs/plans/PLAN_showcase_video.md` (`5a6f35ba`) ve ay adlı tarih ön süzgeci `e68c0482` ("3 Aralık" artık takvim çıkarıcısına ulaşıyor). **`5a6f35ba` ve `e68c0482` yerel, push edilmedi.**
- **Film:** Remotion projesi `build_output/showcase` (gitignore'lu). Kayıtlar `public/rec/` (gerçek backend `data/`, Playwright), anlatım `public/voice/{tr,en}` (ElevenLabs, 39 vuruş/dil), zaman çizelgesi `script/build_timeline.py` → `src/data/timeline.json`. Çıktı `out/memo-showcase-{tr,en}-raw.mp4`. **Telefon bölümü (07) şimdilik çizim;** kullanıcı planın §6 listesindeki çekimleri verince değiştirilecek.
- **`data/`'da bırakılanlar:** bekleyen taslaklar #73/#74/#75 (#74 telefondan `#onay 74` ile onaylanacak, #73 ve #75 iptal edilebilir); kayıt sohbetleri; takvimde 3 Aralık "Dinner with mom" ve 17 Kasım "Launch Memo on Hacker News"; hafızada demo bilgileri (annenin doğum günü, HN lansmanı); iki gelen kutusunda test mailleri. Masaüstünde `Karışık Fotoğraflar` ve `Messy Photos` (ajanın düzenlediği deneme klasörleri). Kişi akdemirbugra14 tekrar `full_auto`; `config.yaml` kayıt öncesiyle aynı.
- **Bulunan, düzeltilmeyen hatalar:** (1) izin zaman aşımı kullanıcıya "sağlayıcı zamanında yanıt vermedi" diye gösteriliyor; (2) Phi-3 mini (4k bağlam) + ~3100 token sistem istemi → bağlam taşıp anlamsız çıktı (Gizli Mod'da düzgün); (3) Gizli Mod'da "Alt ajanlar yüklenemedi"; (4) plan hem otomatik yakalanıp hem açıkça istenince takvimde çift etkinlik; (5) Phi-3 Türkçede zayıf, TR kullanıcıya başka model önerilmeli; (6) boş veri klasöründe web istemcisi açılınca `onboarding.completed` true olmuştu, sihirbaz görünmedi (sebep doğrulanmadı).
