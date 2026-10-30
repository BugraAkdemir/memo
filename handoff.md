# Handoff — güncel durum

> Hedef <10 KB. Ayrıntılar `handoff-archive.md`: yalnız hedefli ara. Açık işleri ve kalıcı uyarıları koru.

## Açık işler

**Dijital İkiz / Mail — kullanıcı kabulü bekleniyor**
- PLAN_mail_and_autonomous_system.md: şimdiki iş Faz2/Adım3. Backend+frontend birlikte; Adım4 başlamadı. Gelen saldırı engeli kullanıcıda PASS; diğer güvenlik senaryoları ve charset/HTML/kalıcı sayaç cihaz kabulü bekleniyor. Eski mail/backfill yok. Senaryolar: docs/MAIL_SECURITY.md.

**Performans benchmark — kullanıcı duraklattı (29 Ekim)**
- docs/PERFORMANCE_BENCHMARK_CHECKPOINT.md + data/performance-benchmark/PAUSED.json: son koşuda yalnız aşama1 kayıtlı, data aşama2. Kalibrasyon/resume/aşama2–3/rapor/doğrulama açık. Runner baştan siler; körlemesine çalıştırma.

**Telemetri / resmî sürüm öncesi (`docs/TELEMETRY.md`, `PLAN_telemetry.md`)**
- 27 Ekim: resmî derlemede günlük paylaşım sorusuz açık; sihirbaz/Gizlilik kapatır, kayıtlı hayır korunur. Kaynak kapalı. Veri klasörü başına kimliksiz `/v1/install`, paylaşım kapalıyken de gider; kaynak ve MEMO_TELEMETRY=off engeller. Varsayılanın ilk günü raporlanmaz.
- **Sunucu dağıtımı doğrulanmadı:** `~/Documents/admin.memocpp.com`: `db/migrations/001_installs.sql` → `tel/` Pages → `admin/` Pages. Handoff'taki son duruma göre dağıtılmamıştı; `/v1/install` 404 yeniden denenir. Resmî sürümden önce doğrula. 30 sahte demo cihaz 27 Ekim'de silindi.
- Kullanıcıda: Cloudflare belirtecini sil; DB/panel parolalarını değiştir; `.env.local` DB_PASSWORD/belirteç satırlarını temizle.
- Eksik bağlantılar: Whisper başlatma/çıkış; Dart/motor çökmesi; offered/downloaded güncelleme; needs_newer_app/not_for_this_machine nedenleri; yerel paketleme betiklerinin official damgası; site gizlilik sayfası; v5.0.0 notları. Eski “telemetri yok” sürümünden varsayılan açık geçiş yayın notunda açık yazılmalı.
- Canlı tel gönderimi, native sihirbaz, Windows/macOS, tag official damgası, ertesi gün geçişi denenmedi. İzole ikili+sahte sunucuda kurulum/sayaç/preview PASS (27 Ekim).

**Yerel hız / motorlar (26 Ekim, AGENTS.md “Local-model speed”)**
- `libggml-vulkan.so` yalnız yerel linux/cpu; R2 ve `~/.memo` güncellenmedi. Yayın: vendor_llama + rclone + publish_component; `--requires-app` yeni kodlu sürüm olmalı (eskide CPU tercihi/Whisper sessiz GPU'ya geçebilir).
- Ölçülmedi: ayrık Vulkan offload, MoE --n-cpu-moe, araç üreten model+speklasyon, CUDA/ROCm spekülasyon, gerçek Whisper konuşması (--no-gpu kalır), Windows/macOS probe.
- Linux llama b11669 (nvidia b9441), cpu whisper 1.9.5. Whisper llama ggml arka uçlarını da yükler: yükseltmede tekrar dene. Statik Whisper eski CPU'da AVX2/FMA/F16C riski ölçülmedi; AMD ROCm 10 klasörü ~1,2 GB gerçek GPU'da denenmedi.

**Alt ajan (`PLAN_chat_subagents.md`, `docs/CHAT_SUBAGENTS.md`)**
- Web UI canlı kısa test PASS; uzun iş/native ölçülmedi. Gerçek Antigravity/Waydroid hız/kalite ve bildirilen 300 dk geçmişi yok (`docs/TASKLOOP_PERFORMANCE_REVIEW.md`).
- Ana incelemede sidebar spinner var, Durdur yok; kesinti/sonraki sonuç testi eksik. Kesilmiş inceleme sonraki sonuçları tutmaz; Resume main review gerekir, ana ajanda resume yok. Çocuk kayıt hatası context canceled olabilir.

**Proje / sohbet / skill**
- `PLAN_project_folders.md` P0–P5 tamam; hafıza Code Mode'dan bağımsız, hafıza anahtarlarına bağlı. Açık: Flutter `/` yalnız global skill, proje skill backend `/ad` ile çalışır; proje izin arayüzü yok; provider_policy yalnız ayrılmış alan. Skill tasarımı kapsamı belirlenmedi.
- Denenmedi: GitHub skill zip/kurulum onayı, modelin `.memo/AGENTS.md` ve izinlere uyması, kullanıcı DB göçü, Windows/macOS Guard. `data/backups/memory-pre-project-id-*` yerel kalır, otomatik silinmez.
- Live Mode ev köklü arka planı normal sohbet; diğer ev köklü ajan hafıza almaz. Code Mode kapalı projede takvim/WhatsApp açık, kişisel hafıza/kimlik izole.
- Birleşik liste: web ajan/klasör/ilk mesaj denendi; native/öneri/animasyon ölçülmedi. IndexedStack eski ajan indeksi boş, ProjectPath ajan işareti. Başlık ilk kelimeler; LLM başlığı kullanıcı kararı ister. WelcomeView CLI; CLI `/` değişmedi.
- Skill: `/` cümle ortası/imleç/IME, stop görünümü, use_skill canlı denenmedi. Adsız istek kelime eşleştirerek ≤3 seçer, model çağırmaz. Dosya/resim skill tespiti yok; prompt'tan düşen “kullanılıyor” kalabilir; hata/timeout skill_used kaybolabilir; stop Türkçe. Masaüstü toggle yok; /skill:on|off|add|remove ve REPL /skill var.

**Akış / REPL (`PLAN_agent_streaming.md`, `PLAN_cli_coding.md`)**
- Gerçek sağlayıcı araç döngüsü, modelle REPL/skill, Windows/macOS terminali, Flutter timeline ve Claude Code/opencode kalite/maliyet ölçülmedi. Gemini thoughtSignature yok; CLI skill satırı seçimi denenmedi.
- REPL hep kodlama; /agent yok; global memo agent on|off|auto var. -chat düz sohbet açabilir; /session proje sohbetleri.
- Subscriptions/Gemini canlı streamed=true, cached_tok=0 (önce %85): raporlama mı maliyet mi bilinmiyor. Araç yanında anlatım/düşünce gelmedi, boş adım nudge etkisi açık. İstenen gemini-3.8-flash-high yerine flash-n sunuldu; neden bilinmiyor.

**Güncelleme / saha**
- 4.7.0 yayında (`PLAN_auto_update.md`, `docs/AUTO_UPDATE.md`). Gerçek Windows Setup.exe/ShellExecute, macOS, GPU/model motor sonrası restart ve Flutter penceresi denenmedi; macOS betik kurulumunda başlatıcı yok. Windows/macOS ENGINES işareti kanıtsız: eklenmeden güncelleme almaz.
- issue #42–47 açık; indirme bitince kendiliğinden kapanır (yeniden başlat onayı yok); Docker daemon son denemede kapalıydı. İmza: `~/.config/memo-release/update-signing.key`, Actions MEMO_UPDATE_SIGNING_KEY; anahtar geçişi eski imzalı sürüm ister.
- PLAN_v4_6_completion.md: canlı cihaz/OAuth/kota/model kalitesi kanıtsız. Mobil cihaz/emülatör, MemoRoot provider yenileme, Android/iOS HTTP; Auto Route kalite/kota/GGUF soğuk yükleme, karar/doğrulama kalite/hız/maliyet, sihirbaz/abonelik giriş penceresi açık.
- Web sohbet Beta yalnız signed-out/fake site; signed-in ChatGPT/Claude denenmedi. Uzak giriş token değişimi, HTTP clipboard fallback, Telegram/WhatsApp, görsel üretimi ve Flutter görsel/kota kartları canlı denenmedi. Antigravity flash-image hesapta 500; WhatsApp fake taşıması yok.
- Windows/macOS/arm64 sidecar yalnız CI build; Claude login/quota yok. Codex yalnız30d; 5s/7g denenmedi.

**Bilinen küçük noktalar / test ortamı**
- Pi embedding yeni build/reboot denenmedi; eski embedding_auto_start:false ezilmez, elle açmak tercihi kaydeder. Eski origin token döndürme kullanıcıda; satılmış alan adındaki eski REPL indirmeleri kaldırılıp sürüm duyurulmalı.
- Yapıştırma kutusu Done'a dek açık; login.go s.tail erken okuma ölçülmedi. Minimal Mod auto-compact atlar; görsel modeli etiketsiz; ikinci model menüsü ortak listeyi kullanır.
- MEMO_DATA_DIR testlerinde config.ResetForTests eksikleri dosya sızdırabilir. ngrok.TestFetchURL_NoServer yerel API yüzünden düşebilir, tünele dokunma. Yoğun derlemede wav_player stop testi 2s timeout görmüştü.
- Açık kararsız testler: Tailscale RebindsFromLoopback eski8/150 yeni2/150; livemode SendAudioRoutesToCurrentInner; BootstrapTokenAuth temp temizliği; DuckDuckGo captcha; webchat PressingItAgainReplacesAWaitingAttempt. Yerel yük testi çalıştırma; kullanıcı sistemi dondurduğunu bildirdi. Yeşile dek koşma. TamperedDownloadChangesNothing 29 Ekim'de düzeltildi: eski36/150, yeni0/150.

## Kalıcı uyarılar
- Alt ajan çalıştırma; açık talep olmadan PR açma. Backend+frontend birlikte doğrulanır; kullanıcı gerçek senaryo testiyle geri bildirim verir. Sonraki adıma kendiliğinden geçme.
- Kullanıcı talebi: yerel CPU dolduran busy loop/stres testi başlatma. Önceki döngüler sonlandırıldı. Ağır doğrulamayı uzak CI üzerinde yap.
- `~/.config/Memo/image.key` olmadan yalnız data kopyası görselleri açamaz; taşınabilir/şifreli bulut yedeği anahtarı taşımadan aktarır, eski yedek kurtarabilir. Eski sürüm mühürlü görseli bozuk görür.
- Kota sonrası otomatik devam yalnız Memo açıkken; CLI ajan/WhatsApp/Self-Driving kapsam dışı. Görsel niyeti anahtar sözcüklü: yanlış tetikte negatif test; `/image` zorlar.
- Sidecar soğuk liste ~30s; kapasite/effort tam model id'sinden, offline son snapshot; bilinmeyen tahmin edilmez (`docs/SUBSCRIPTION_CAPABILITIES.md`).
- Beta kapısı open_app + yedi browser_* araç/panel/session API'yi gizler; kapanınca oturum biter. fetch_page fallback/Chromium kurulumu kapısız. Native görünüm denenmedi; PROJECT_MAP/UI_FEATURES/ROADMAP EN/TR ve Obsidian eski satırları açık.
- Mobil memo.apk/ipa (tag), memo-beta.apk/ipa (main); eski memo-android*/memo-ios* R2 adları güncellenmez. Masaüstü memo.AppImage/memo_beta.AppImage; memo.tar.gz/memo_beta.tar.gz; memo-mac.zip/memo-mac_beta.zip; memo.exe/memo-beta.exe; memo_arm.zip/memo_arm_beta.zip.
- Go sürümü go.mod + 7 workflow GO_VERSION birlikte; Docker golang:1.26-bookworm kayandır. v* tag push her seferinde açık kullanıcı talebi ister.

---

# Son oturum — 2026-10-31 — Mail kullanıcı testi düzeltmeleri
- MIME Türkçe charset → UTF-8, RAG/güvenlikten önce. HTML gömülü CSS+tablo ve TR/EN düz metin anahtarı; script/uzak resim/stylesheet yok. Tam tarayıcı CSS'i değil; html0.15.6 pini.
- Watcher sayaçları hesap kasası yanında watcher-stats.json'a seri/atomik/0600 yazılır, açılışta yüklenir. Restart/callback/IMAP testli; eski kayıp sayaç/bozuk RAG onarılmadı, backfill yok.
- Kullanıcı saldırı maili engelini onayladı; repo otomatik yanıtı Adım4–6'da, başlanmadı. docs/MAIL_SECURITY.md testleri cihazda bekler; iki süreç güncel kodla yeniden başlatılmalı.
- Doğrulama: build/vet PASS, Go race70 paket PASS; Flutter856/856 PASS; analyze mevcut3 info; UI literal3 dosya0; diff temiz. Yerel fix(mail) commit; push/PR/alt ajan yok. Diğer yerel değişiklikler korundu.
