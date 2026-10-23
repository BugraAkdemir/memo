# Handoff — güncel durum

> Her yeni sohbette yüklenir; hedef <10 KB. Ayrıntılı eski kayıtlar `handoff-archive.md` içinde; yalnız gerektiğinde hedefli ara. Oturum sonunda önceki son özeti arşivin başına taşı, açık işleri burada koru.

## Açık işler

**Alt ajan (2026-10-23, `docs/SUBAGENT_REVIEW.md`)**
- Açık: readonly shell yazma açığı doğrulandı; worker repo kurallarını aktarmıyor; model başka listeden seçilebilir; özgün hatalar kayboluyor. Planlayıcı paralel yazarlarda çakışma riski ölçülmedi.
- Task.md varsayılanı planlayıcı; dört rol worker yolunda. Dinamik spawn/message/wait yok. Yeni gerçek model koşusu yok.

**Proje klasörleri (2026-10-19, `PLAN_project_folders.md`, `docs/PROJECT_FOLDER.md`)**
- P0–P5 işaretli: kök koruması/id/kayıt, yedekli hafıza göçü, iskele/API/onay, proje skill komutları, hash güvenli izinler, EN/TR belgeler. Beta'ya bağlı değil; proje hafızası Code Mode'dan bağımsız, Hafıza/Minimal Mod/görev hafızası anahtarları geçerli.
- Yapılmadı: proje skill'leri Flutter `/` menüsünde listelenmiyor (menü yalnız global skill'leri okur; backend `/ad` yazılınca proje skill'ini de tanır; `/skill add` var), proje izin düzenleme/onay arayüzü (yalnız dosya/API), `provider_policy` uygulaması (`local_only` vb. yalnız ayrılmış alan). Kullanıcı skill sistemini değiştirebileceğimizi söyledi; yeni tasarımın kapsamı belirlenmedi.
- Gerçekte doğrulanmadı: GitHub skill indirmesi (`codeload.github.com/<o>/<r>/zip/HEAD`), kurulum/onay penceresi, modelin `.memo/AGENTS.md` ve izinlere uyması, kullanıcı veritabanında göç, Windows/macOS Guard listeleri.
- Göç yedekleri `data/backups/memory-pre-project-id-*.db` yerelde kalır; dışa aktarma/bulut kapsamında değil, otomatik silinmez.
- Live Mode'un ev köklü arka plan sohbeti normal sohbet sayılır; diğer ev köklü ajan sohbeti hafıza almaz. Code Mode kapalı proje sohbetinde takvim/WhatsApp araçları açık kalır; kişisel hafıza/kimlik izoledir.

**Birleşik sohbet listesi / yeni sohbet ekranı (2026-10-19)**
- Gerçek pencerede doğrulanmadı: merkez besteci/öneriler, Ajan + klasör seçimi, başlık/üst çubuk ve ilk mesajda alta geçiş. Geçiş anlık; animasyon yok. Ajan sekmesinin eski IndexedStack indeksi boş tutulur; ajanlar ProjectPath ile işaretlenir.
- Ajan başlığı ilk kelimelerden, LLM çağrısı yok. Daha güzel başlık için Code Mode yan-çağrı yasağı hakkında kullanıcı kararı gerekir. WelcomeView yalnız CLI sohbetlerinde.
- `/` menüsü artık skill menüsü (aşağıda); CLI sağlayıcısına özel `_CLICommandPopup` bu değişikliğe dahil değil.

**Agent akışı / REPL (`PLAN_agent_streaming.md`, `PLAN_cli_coding.md`)**
- Gerçek sağlayıcılarda akış+araç döngüsü, gerçek modelle REPL/skill kullanımı, Windows/macOS terminali, Flutter zaman çizelgesi ve Claude Code/opencode kalite-maliyet karşılaştırması doğrulanmadı (sahte SSE + fakeprovider + pty/widget testleri var).
- Gemini thoughtSignature yok. CLI `/` skill satırlarının etkileşimli seçimi test edilmedi. REPL hep ajan/kodlama; `/agent` anahtarı bilerek yok, global `memo agent on|off|auto` alt komutları var. `-chat <id>` düz sohbet açabilir; `/session` proje sohbetlerini listeler.
- Gerçek Subscriptions/Gemini akışı `streamed=true`, `cached_tok=0` gösterdi (önceden %85); CLIProxyAPI raporu mu gerçek maliyet mi bilinmiyor. Model araç yanında metin/düşünce üretmedi; anlatma kuralının etkisi ölçülmedi. Boş adım nudge ile denenir; kök neden açık.
- Log'da istenen `gemini-3.8-flash-high` yerine `gemini-3.8-flash-n` sunuldu; neden bilinmiyor, aynı-model karşılaştırmasını etkiler.

**Güncelleme (4.7.0 yayında, `PLAN_auto_update.md`, `docs/AUTO_UPDATE.md`)**
- Gerçek Windows Setup.exe/ShellExecute, macOS, GPU/model ile motor sonrası restart ve Flutter güncelleme penceresi doğrulanmadı. macOS betik kurulumunda başlatıcı yok → çoğu manual.
- Windows/macOS motor sürüm işareti yok; kanıtlı ENGINES.txt satırı eklenmeden motor güncellemesi almaz. Linux llama b9441, cpu whisper 1.8.6.
- Yerel linux/cpu içindeki yarım b11456 artıkları (`libllama.so.0.0.1`, `*.so.0-new`, `llama-new`) R2'de yok; silmeden sor. llama b11456 yükseltmesi uygulanmadı, karar kullanıcıda.
- İmza anahtarı `~/.config/memo-release/update-signing.key` ve Actions `MEMO_UPDATE_SIGNING_KEY`; anahtar geçişi eski anahtarla imzalı sürüm ister.

**Saha doğrulaması / sürüm hazırlığı**
- Beta dışı kod eksikleri kapandı: `PLAN_v4_6_completion.md`, `docs/V4_6_COMPLETION_REVIEW.md`. Gerçek telefon/native OS, OAuth/Claude kota, model kalitesi/maliyeti testlerle tamamlanmış sayılmaz.
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
- Bilinen kararsızlar: update `TestRunComponentUpdate_TamperedDownloadChangesNothing` eski 5d068ac1'de de -race -count>1 düşer; livemode `TestReconnectingSession_SendAudioRoutesToCurrentInner`; `TestBootstrapTokenAuth_Succeeds` temp temizliği; DuckDuckGo captcha; webchat `TestConnect_PressingItAgainReplacesAWaitingAttempt` (Connecting bayrağı gerçek gezinmeden erken; şüphe ölçülmedi). Yükte flake: go test -c + 24 meşgul döngü, eski/yeni aynı yükte ölç; yeşile kadar yeniden koşma.

**Skill çağrısı / durdurma (2026-10-22)**
- Gerçek pencerede doğrulanmadı: `/` menüsü (cümle ortası, imleç/IME), durdurma sonrası akışın kalması, skill satırları. Gerçek modelin `use_skill`'i doğru seçmesi ölçülmedi. "gerekli skilleri kullan" kelime eşleşmesiyle en çok 3 skill seçer (model çağrısı yok), kaba.
- Bilinen boşluklar: resim/dosya ekli gönderimde skill tespiti yok; yerel modelin token bütçesi skill'i prompt'tan düşürürse "kullanılıyor" satırı yine görünür; hata/zaman aşımı yolunda kayıtlı cevap `skill_used` satırını kaybeder (canlı akışta görünmüştü); proje skill'leri Flutter menüsünde yok (backend `/ad` ile tanır); durdurma işareti sunucu kopyasında hep Türkçe (Flutter anlık kopyası arayüz dilinde).
- `/skill:on|off|add|remove` ve REPL `/skill` duruyor; masaüstünde açma/kapama arayüzü yok.

---

## Kalıcı uyarılar
- `~/.config/Memo/image.key` kaybında yalnız data kopyası görselleri açamaz. Parolalı taşınabilir/şifreli bulut yedeği ana anahtarı kopyalamadan taşır; önceden alınmış yedek kurtarabilir. Eski sürüm mühürlü görselleri bozuk görür.
- Kota dolunca otomatik devam yalnız Memo açıkken; CLI ajan/WhatsApp/Self-Driving kapsam dışı. Görsel niyet anahtar sözcüklü; yanlış tetikte negatif test ekle, `/image` zorlar.
- Sidecar model listesi soğukta ~30s gecikir. Bağlam/effort tam model id ile çevrimiçi kayıt defterinden, son başarılı snapshot çevrimdışı; bilinmeyen kapasite tahmin edilmez (`docs/SUBSCRIPTION_CAPABILITIES.md`).
- Beta kapısı: open_app + yedi browser_* aracı/paneli/session API, Beta kapalıyken yok; kapatmak oturumu bitirir. fetch_page fallback/Chromium kurulum bölümü kapısız. Gerçek pencerede Beta açık/kapalı görünümü doğrulanmadı. PROJECT_MAP/UI_FEATURES/ROADMAP EN/TR ve Obsidian'da bu kapıyı anlatmayan eski satırlar var.
- Mobil adlar: memo.apk/ipa (v*), memo-beta.apk/ipa (main; frontend/version/workflow değişiminde). Eski memo-android.apk/memo-android_beta.apk/memo-ios.ipa/memo-ios_beta.ipa R2'de kalmış, güncellenmez. Masaüstü: memo.AppImage/memo_beta.AppImage, memo.tar.gz/memo_beta.tar.gz, memo-mac.zip/memo-mac_beta.zip, memo.exe/memo-beta.exe, memo_arm.zip/memo_arm_beta.zip.
- Go sürümü go.mod + 7 workflow GO_VERSION birlikte yükseltilir; Docker golang:1.26-bookworm kayandır.

---

# Son oturum — 2026-10-23 — alt ajan yeteneği incelemesi
- Kaynak/graph incelemesi ve `docs/SUBAGENT_REVIEW.md`: worker dört rolü ile varsayılan planlayıcı paralel coder yolu ayrıştırıldı; açık bulgular yukarıda.
- Tam taskloop -race: ok 34.949s. Hedefli taskloop/agent/agent-tools/app testleri: dört paket ok. Readonly araçla geçici klasörde yazma doğrulandı; geçici dosyalar temizlendi. Gerçek model/UI ve tam repo doğrulaması yapılmadı.
- Uygulama kodu değişmedi; yalnız inceleme/handoff belgeleri. Push yok.
