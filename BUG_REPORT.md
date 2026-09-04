# Bug Report — Memo Açık Bug Listesi

> **Amaç:** Şu an gerçekten açık olan, stable sürüme engel bug'ların listesi — düzeltilmiş olanlar burada yok (git geçmişinde duruyorlar, tekrar burada tutmanın değeri yok).
> **Son güncelleme:** 2026-08-30 (gece) — v4.5.0 planlayıcı/uygulayıcı modu **iki tur canlı test edildi**. 2. turda planexec pipeline'ı ücretsiz zayıf modelle (`hy3:free`) **çalışan bir SQLite blog sitesi** kurdu: signup, salted-hash login, session/cookie, blog yazısı + index — 6 maddeden 5'i, checkbox mirror + `# onay: otomatik` + retry + escalation dahil uçtan uca doğrulandı (curl ile signup→login→post→index zinciri gerçekten çalışıyor). Yol boyunca **8 bug** bulundu, **hepsinin fix'i aynı oturumda indirildi**: BUG-PERM1 (izin dialogu flash — canlı yeniden-test bekliyor, gerçek Flutter dialogu yok), BUG-PLAN1/2 (timeout'lar — kök neden düzeltildi: agent pipeline non-stream `client` 120s'e takılıyordu, 300s yapıldı), **BUG-PLAN3/5/6/7/8 (planexec çalıştırma sertleştirmesi — 2. turda canlı doğrulandı)**. BUG-PLAN4 (Görevler sekmesi kartı onay UI'ını açmıyor) da **aynı gece düzeltildi** (`91c763bc`, kart artık `TaskDetailScreen`'e push ediyor; doğrulama `1191ae92` — bu paragraf o zaman güncellenmemişti, madde aşağıda "✅ (silinecek)" olarak duruyor, gerçek açık liste "Özet" tablosundadır).
>
> **Önceki güncelleme:** 2026-08-13 — **RPi canlı testinden 2 bug** (kullanıcı bildirimi, BUG-ONB10'un bir önceki fix turunun ardından): (1) **BUG-ONB12 — Orchestra config toast spam**: `orchestraConfigProvider`'ın `build()`'i BUG-ONB6 gate-guard'ını taşıyordu ama gate açıkken herhangi bir başka fetch hatasında hâlâ `errorMessageProvider`'a toast basıyordu; bu provider `engine_strip.dart`/`chat_input.dart` tarafından **ambient** izlendiği için (Orchestra kapalıyken bile arka planda çekiliyor), her geçici hata alakasız bir "Orchestra'da hata oluştu" toast'ına dönüşüyordu. `activeProviderTypeProvider`/`remoteAccessProvider`'ın zaten aldığı "ambient watcher'lara sessiz kal" fix'i uygulandı. (2) **BUG-ONB13 — token-only kurulum, loopback olmayan istemciden her zaman 401**: kurulumun "sadece token" seçeneği ilk çağrı olarak kimlik doğrulamalı `PUT /api/remote-access`'i (elde henüz hiç kimlik yokken) çağırıyordu — `password`/`token_password` yöntemleri kimlik doğrulamasız `/api/setup/create-admin`'den geçtiği için bu sorunu yaşamıyordu. `create-admin` ile aynı desende yeni bir `POST /api/setup/create-device` (self-gating, `NeedsSetup()` üzerinden) eklendi; `RemoteAccessConfig.SetupBootstrapped` yeni alanı, token-only yolun `Accounts`/`Username`'e hiç dokunmadığı için `needs_setup`'ın sonsuza dek `true` kalmasını da ayrıca kapattı. Go+Flutter build/vet/test `-race` yeşil, `flutter test` 253/253, yeni testlerin fix'ten önce kırıldığı doğrulandı, canlı duman testiyle (gerçek binary, izole data dir) uçtan uca doğrulandı. **Aynı gün, gerçek RPi'de gerçek non-loopback kaynaktan da doğrulandı:** eski yol 401, yeni yol 200+token, ikinci deneme 403, `config.yaml`'da `setup_bootstrapped: true`. Detay: handoff.md "Ek (2026-08-13, devam 3)".
>
> **Önceki güncelleme — İki varsayılan hatası düzeltildi** (`08d0b0d`, kullanıcı bildirimi): (1) **mood ilk kurulumda açık geliyordu** — `config.Default()` `Mood.Enabled: true` veriyordu, oysa mood motoru her mesaja ton direktifi enjekte ediyor (WebSearch'ün kapalı gelmesiyle aynı gerekçe). Kapatıldı; mevcut kurulumlar etkilenmiyor (`Load()` config.yaml'ı üstüne biniyor) ve `TestExplicitMoodEnabledSurvivesLoad` bunu sabitliyor. Not: `config.yaml.example` zaten `false`'du, sorun yalnızca "config dosyası hiç yok" yolundaydı. (2) **beta kapalıyken Swarm sekmesi görünüyordu** — `_showSwarmNav()` cevabın `beta:true` dışında ne olduğuna bakmadan yerel `memo_beta_features` aynasına düşüyordu (yorumu "yüklenene kadar" dese de kod öyle değildi), yani bayat bir yerel `true` sapasağlam bir `beta:false`'u kalıcı olarak eziyordu. Ayrıca `remoteAccessProvider` de BUG-ONB11 şeklindeydi: gate arkasında 401 alıp `{'enabled': false}` önbelleğe alıyordu ve o map'te `'beta'` anahtarı olmadığı için "henüz cevap gelmedi"den ayırt edilemiyordu. Fix: provider gate kapalıyken boş map dönüyor + merkezi listener'dan invalidate ediliyor, `_showSwarmNav` `containsKey('beta')` ile test ediyor. `memo_beta_features` ayrıca `serverCoupledPrefsKeys`'e taşındı (sunucu config'inin aynası, cihaz tercihi değil). Go+Flutter yeşil, `flutter test` 251/251, yeni testin fix'ten önce kırıldığı doğrulandı. RPi config'i SSH ile kontrol edildi (`mood: false`, `beta: false`) — sunucu tarafı zaten doğruydu.
>
> **2026-08-13 — BUG-ONB11: BUG-ONB6 taramasından kaçan 3 başlangıç isteği** (`02b762b`): kullanıcı masaüstünden RPi'ye bağlıyken Rutinler ekranında kalıcı "Rutinler yüklenemedi", Geliştirici ekranında Model ID'leri altında kalıcı aynı hata bildirdi — uygulamanın geri kalanı çalışırken. Kök neden BUG-ONB6 ile aynı (`IndexedStack` her ekranı gate açılmadan kuruyor, tek deneme 401 alıyor); **yeni olan, BUG-ONB6 taramasının bunları neden kaçırdığı** — o audit `AsyncNotifier.build()` + `apiClientProvider` şeklini arıyordu, bu üçü ise: `gatewayModelsProvider` düz bir `FutureProvider` (`gpuInfoProvider`/BUG-ONB5 ile birebir aynı kaçış, retry döngüsü olmadığı için 401 kalıcı), `RoutinesScreen._load()` ve `CalendarScreen._load()` ise provider bile değil — `initState`'ten widget state'i set eden fonksiyonlar, hiçbir provider sorgusu göremez. Rutinler'in hiçbir kurtarma yolu yok (timer yok, invalidate edilecek şey yok); Takvim 20s timer'ıyla toparlıyor ama o ana kadar yanlış hata banner'ı gösteriyor, o yüzden o da düzeltildi. **Fix:** üçü de `authGateBlocked()` kontrol edip hata yerine güvenli varsayılan gösteriyor; ekranlar `build()`'de kendi `ref.listen`'ıyla gate açılınca yükleniyor, provider `app_shell.dart`'ın merkezi listener'ından invalidate ediliyor (`gpuInfoProvider` de oraya eklendi — bugüne kadar yalnızca `auth_gate_overlay`'in 5 login yolundan invalidate ediliyordu ve o liste gate'in açılabildiği diğer yolları görmüyor). `flutter test` 249/249, 3 yeni assertion'ın fix'ten önce kırıldığı doğrulandı. **AGENTS.md'ye kalıcı kural yazıldı:** bu sınıf artık 4 kez düzeltildi, her seferinde bir öncekinin taraması yanlış *şekli* aradığı için hayatta kalan oldu — süpürmeden önce dört şeklin hepsine bakılmalı (AsyncNotifier/StateNotifier build, düz FutureProvider, StatefulWidget initState, polling). Yalnızca loopback olmayan bir adresten gated backend'e bağlanınca üretilebilir; yerel masaüstü çalıştırmada hiç görünmez.
>
> **2026-08-13 — BUG-ONB10: sunucu silinip yeniden kurulduğunda tarayıcı bayat auth state'inde kilitleniyordu** (`0a32529`, `593a7f5`, `67e310a`): kullanıcı RPi'sini `uninstall-selfhosted.sh` + `get-memo-server-beta.sh` ile sıfırdan kurdu, kurulum ekranı hiç gelmedi, konsol 401 doluydu. SSH ile doğrulandı ki backend tamamen doğruydu (`needs_setup:true`, `accounts: []`, `MEMO_DATA_DIR` doğru) — sorun %100 istemci tarafındaydı. **Kök neden:** localStorage origin bazlı, yeniden kurulum aynı `http://<ip>:8090` adresini kullanıyor, dolayısıyla tarayıcı `memo_auth_setup_done`'ı yeni backend'e taşıyor ve `authGateProvider`'ın `needs_setup && declined -> ok` dalı kurulum kapısını kalıcı olarak bastırıyor. Ctrl+Shift+R / Ctrl+F5 çare değil (hard reload sadece HTTP cache'ini atlar); kullanıcı DevTools'tan elle temizleyerek çözdüğünü bildirdi ve kök nedeni kendisi buldu. **Fix, iki bağımsız katman:** (1) `/api/setup/status` artık `install_id` dönüyor (`internal/app/install_id.go`, kuruluma özel rastgele değer, data dizini silinince yok olur) ve gate uyuşmazlıkta sunucuya bağlı tüm anahtarları siliyor (`serverCoupledPrefsKeys`, `frontend/lib/core/local_session_state.dart`); (2) `needs_setup && declined && !loopback` iken 401 probe'u declined bayrağını yok sayıyor — bu katman ID döndürmeyen eski backend'lerde de çalışır. Bir install ID ilk kez görüldüğünde sıfırlama yapılmaz (yalnızca kaydedilir), aksi halde bu build'e geçen her çalışan istemci boş yere çıkış yapardı. Ayrıca elle kaçış yolu: `ClearSavedSignInButton` (gate footer + backend-unreachable ekranı), metni bilinçli olarak sunucu verisinin silinmediğini açıkça söylüyor. Yol boyunca gerçek bir layout bug'ı da bulundu: gate footer'ı Türkçe etiketlerle 54px taşıyordu (İngilizce'de geçiyordu) — `Wrap`'a çevrildi. Go/Flutter build+vet+test `-race` yeşil, `flutter test` 246/246, yeni regresyon testlerinin fix'ten önce kırıldığı doğrulandı, ayrıca canlı duman testiyle install_id'nin restart'ta sabit kalıp wipe'ta değiştiği kanıtlandı. **Kullanıcının RPi'sinde henüz canlı doğrulanmadı** — yeni binary'nin yayınlanması gerekiyor.
>
> **UI varsayılan dili İngilizce oldu** (`8882506`, bug değil, kullanıcı kararı): `L10n._locale`/`LocaleNotifier._initLocale` artık İngilizce'ye düşüyor, yalnızca açık `'tr'` Türkçe seçiyor (dili daha önce seçmiş kullanıcılar etkilenmez). **Bilinen açık seam:** backend hâlâ bazı kullanıcıya ulaşan stringleri Türkçe basıyor ve bunlar `L10n`'dan geçmiyor — İngilizce arayüzde Türkçe sistem mesajları görünmeye devam eder. Kapatmak `Identity.UILanguage`'e bağlamayı gerektiriyor; bilinçli olarak kapsam dışı bırakıldı, AGENTS.md'ye yazıldı.
>
> **2026-08-12 — BUG-ONB9: ISP seviyesinde şeffaf cache, kurulum script'lerinin her zaman eski binary indirmesine sebep oluyordu** (`a19d223`, RPi'de canlı SSH ile teşhis edildi): `MEMO_DATA_DIR` fix'i (BUG-ONB7) R2'ye yüklendikten ve Cloudflare'in kendi edge'inin taze olduğu doğrulandıktan (`cf-cache-status: DYNAMIC`, doğru `last-modified`) sonra bile, RPi'de `get-memo-server-beta.sh`'ın kullandığı düz URL ısrarla **eski** binary'yi (`md5 3c401e2d...`) indiriyordu — art arda 3 ayrı tam uninstall+reinstall döngüsünde. Aynı URL'ye `?cachebust=$(date +%s%N)` eklenince aynı anda doğru binary (`3ead9182...`) geldi — Cloudflare değil, RPi'nin ağ yolu üzerinde bir yerde (muhtemelen Türkiye'de yaygın olan ISP seviyesi şeffaf HTTP cache) sorun olduğu kanıtlandı. **Fix:** archive indiren 6 script'in hepsi (`get-memo.sh`, `get-memo-beta.sh`, `get-memo-server.sh`, `get-memo-server-beta.sh`, `get_memo_arm.sh`, `update.sh`) artık gerçek `curl` isteğine cache-busting query string ekliyor (`$url`'ün kendisi indirilen dosya adı/ekrana basılan mesaj için temiz kalıyor). RPi'de canlı doğrulandı: fix'ten sonra unit dosyasında `Environment=MEMO_DATA_DIR=...` doğru göründü, `needs_setup:true` (gerçek fresh install). Not: `upload-memo.sh` (`/home/bugra/Documents/r2-memo-push/`) ile CI beklemeden direkt R2'ye de yüklendi.
>
> **BUG-ONB8: çift hata SnackBar'ı + ham provider hata metni** (`6863dae`, kullanıcının bir ekran görüntüsüyle bildirdiği): OpenCode Zen rate-limit yerken kullanıcı ham Go hatasını ("all providers failed: [opencode-zen] provider rate limited: ...") aynen görüyordu — Memo'nun sorunu değil (sağlayıcı kendi kendini sınırlıyor) ama teknik dökümü kullanıcıya "Memo bozuk" izlenimi veriyordu. `FriendlyError.describeGeneric`'e `_classifyProviderMessage` eklendi, "rate limit" içeren mesajları TR+EN dostça bir metinle değiştiriyor. Ayrıca aynı ekran görüntüsünde görülen ikinci bug: hata toast'ı **iki kere** çıkıyordu — `chat_screen.dart`'ta (`6 Temmuz`) `app_shell.dart`'ınkiyle (`25 Haziran`, kırmızı+floating) birebir aynı `ref.listen(errorMessageProvider,...)` kopyası vardı, ikisi de `ChatScreen`, `AppShell`'in kalıcı `IndexedStack`'i içinde sürekli mount olduğundan aynı anda tetikleniyordu. Kopya silindi. Yeni testler `friendly_error_test.dart`'ta. `flutter test` 236/236, analyze temiz.
>
> **BUG-ONB6 sistematik geçiş** (`9ce06af`): kullanıcı, ilk BUG-ONB6 fix'inin (chat ekranı) aynı sorununu Ayarlar'da ve Geliştirici Seçenekleri'nde de gördüğünü bildirdi. `codebase-memory` ile (`search_graph`, `AsyncNotifier build()` + `apiClientProvider` kullanan tüm provider'lar) tüm dosyalar tarandı, tahmin yürütülmeden — aynı korumasız desen 17 provider'da bulundu: `settings_provider.dart`'ta 9 tanesi (`DevGatewayConfigNotifier` dahil — tam olarak "Geliştirici Seçenekleri"), `agent_provider.dart`, `skill_provider.dart`, `models_provider.dart`, `tasklist_provider.dart`, `provider_provider.dart`'ta 2 tanesi, `orchestra_provider.dart` (bu ayrıca her bağlantıda görünür kırmızı SnackBar da atıyordu), ve farklı şekle sahip `swarm_provider.dart`'taki `SwarmNotifier` (constructor'da fetch eden bir `StateNotifier`). Hepsine aynı 2 parçalı fix uygulandı: (1) her `build()`/constructor-fetch artık gate kontrolü yapıp güvenli varsayılanla mount oluyor; (2) her ekrana ayrı `ref.listen` kopyalamak yerine `app_shell.dart`'a (tüm oturum boyunca canlı kalan tek widget) **tek, merkezi** bir gate-geçişi listener'ı eklendi — 17 provider'ın hepsini tek seferde invalidate ediyor. Yeni test: `gate_blocked_providers_test.dart` (16 provider + swarm, hepsi gate kapalıyken sıfır backend çağrısı + güvenli varsayılan). Ayrıca yol boyunca gerçek bir test kırılması bulundu ve düzeltildi: `settings_toggle_race_test.dart`'ın `authGateProvider` override'ı yoktu, yeni gate kontrolü yüzünden 2 testi kırdı — override eklendi + `authGateProvider.future`'ı önce `await`lemek gerekti (build() senkron çalışıyor, override'ın stream'i henüz ilk event'ini vermeden). `flutter test` 234/234, analyze temiz. **RPi'de henüz doğrulanmadı.**
>
> Kullanıcının RPi'sinde canlı SSH ile bulunan 2 bug, aynı oturumda düzeltildi:
> - **BUG-ONB7** (`53d1740`) — `memo service install`'in oluşturduğu systemd `--user` unit dosyası hiç `MEMO_DATA_DIR` ayarlamıyordu. systemd `--user` birimlerinin varsayılan çalışma dizini `$HOME` olduğundan (`WorkingDirectory=` set edilmediği için), backend'in `config.DataDir()`'ı (`internal/config/config.go`) relative `"data"` fallback'ine düşüp **`$HOME/data`**'yı kullanıyordu — `~/.memo/data` değil. Sonuç: `uninstall-selfhosted.sh` `~/.memo/`'yu tamamen silse bile (SSH ile doğrulandı: gerçekten siliniyor), gerçek hesap/hafıza/sohbet verisi `~/data/` altında el değmeden duruyordu, sıfırdan kurulum eski hesabı geri getiriyordu. Fix: `buildUnitFile()` artık `Environment=MEMO_DATA_DIR=<MEMO_HOME>/data` satırını ekliyor (`internal/app/cliadmin.go`'nun masaüstü kurulum yolunda zaten kullandığı aynı değer). **Migrasyon yok** — zaten kurulu bir servis fix'i almak için `memo service uninstall && memo service install` ile yeniden kurulmalı.
> - **BUG-ONB6** (`a0f14ce`) — chat ekranı, self-hosted sunucuya ilk bağlanıldığında (web + masaüstü) kalıcı "Bir şeyler ters gitti" gösteriyordu; yeni sohbet açmak "düzeltiyordu" (eski sohbetler de geliyordu), masaüstünde sayfa yenileme olmadığı için kalıcı kalıyordu. Kök neden BUG-ONB4/ONB5 ile aynı desen: `chatListProvider`/`activeChatIdProvider` (`chat_provider.dart`) — tek seferlik `AsyncNotifier`'lar, gate kapalıyken tek denemeleri 401 alıp kalıcı `AsyncError`'a düşüyordu; `messagesProvider` BUG-ONB4'te bu fix'i almıştı ama bu ikisi atlanmıştı. Fix: her ikisinin `build()`'i artık gate kontrolü yapıyor, `chat_screen.dart`'ın mevcut gate-geçişi listener'ı artık ikisini de invalidate ediyor.
> - Go + Flutter: build/vet/test hepsi yeşil (`TestBuildUnitFile_SetsMemoDataDir`, genişletilmiş `messages_notifier_unauthorized_test.dart` yeni). **BUG-ONB7 SSH ile canlı doğrulandı** (RPi'de gerçek uninstall+install döngüsü); BUG-ONB6 henüz RPi'de canlı doğrulanmadı.
>
> **BUG-ONB5 tamamen düzeltildi** (`bfc910a`): kullanıcının netleştirmesi ("kurulum ekranı ve Model Store, RAM'i doğru gösteriyor ama modele göre öneri yaparken bulamıyor, refresh'te düzeliyor") `codebase-memory` ile koda bağlandı — `gpuInfoProvider` (`frontend/lib/providers/models_provider.dart`), BUG-ONB4'ün diğer tüm polling provider'lara eklediği `authGateBlocked()` korumasından **kaçmış tek provider**: düz bir `FutureProvider` olduğundan (StreamProvider'lardaki gibi `while(alive)` retry döngüsü yok) tek denemesi gate kapalıyken denk gelirse `/api/gpu` 401 dönüyor, catch bunu sessizce `GPUInfo()` (ramTotalMb:0) olarak önbelleğe alıyor ve **bir daha asla yeniden denemiyor** — `recommendedChatModel`/`hardwareFit` (`curated_models.dart`) RAM'i "bilinmiyor" sayıp zayıf/genel öneriye düşüyor. Refresh "düzeltiyormuş" gibi görünmesinin sebebi tüm provider grafiğinin sıfırdan kurulması, o noktada gate zaten açık oluyor. Fix: (1) `gpuInfoProvider` artık istek atmadan önce gate'i kontrol ediyor (diğer polling provider'larla aynı desen); (2) `auth_gate_overlay.dart`'ın gate'i açan 5 noktasının hepsi (`_decline`, `_submit`, `_enterToken`, `_loginPassword`, `_loginToken`) artık `ref.invalidate(authGateProvider)`'ın yanında `ref.invalidate(gpuInfoProvider)` da çağırıyor — `ref.watch(authGateProvider)` ile reaktif bağlama denendi ama non-autoDispose bir `FutureProvider`'ın autoDispose bir `StreamProvider`'ı watch etmesi `container.read(...future)`'ı testte sonsuza kilitliyor, bu yüzden açık invalidation'a dönüldü. Yeni test: `gpu_info_provider_test.dart`. `flutter test` 231/231, analyze temiz, Rule #8 grep temiz. **Kullanıcı elinde doğrulanacak** — RPi'sine kurup gerçek sonucu bildirecek.
>
> **TD-4 kullanıcı tarafından halledildi** (Cloudflare dashboard/hesap ayarı — bu repo'nun kapsamı dışındaydı zaten, kod tarafında bir şey yok). **TD-3 tamamen düzeltildi** (`8e470e3`): `build-linux.yml`'e "Upload install/update/uninstall scripts to R2" adımı eklendi — `scripts/README.md`'nin "End-user installers" tablosundaki 11 script artık her `main` push'unda R2'ye otomatik yükleniyor (mevcut R2 secret'ları/rclone deseni yeniden kullanılıyor), böylece repo'da düzeltilen bir script bug'ı elle yükleme beklemeden canlıya çıkıyor — bu oturumun kendi BUG-ONB1/ONB2 script düzeltmeleri (`dec0c0a`) dahil.
>
> **Ayrıca aynı oturumda, kullanıcı script'lerin gerçekten çalışıp çalışmadığını sorunca bulundu:** 9 end-user script'in (get-memo.sh, get-memo-beta.sh, get-memo-server.sh, get-memo-server-beta.sh, get_memo_arm.sh, update.sh, uninstall.sh, uninstall-arm.sh, uninstall-selfhosted.sh) hepsi `set -euo pipefail`'den hemen sonra çıplak `clear` çağırıyordu — `$TERM` set değilse (pty'siz `curl | bash`, bazı SSH/provisioning senaryoları, cron) `clear` hata koduyla dönüyor ve `set -e` yüzünden **script hiçbir şey yapmadan anında ölüyordu**. `uninstall-selfhosted.sh`'ı sahte bir `$HOME` ile sandbox'ta çalıştırarak canlı olarak doğrulandı (fix'ten önce: hiçbir şey silinmedi; fix'ten sonra: doğru şekilde silindi). `8e470e3`'ün ardından `40b6b32` ile düzeltildi (`clear 2>/dev/null || true` + 3 script'teki `/dev/tty` prompt fallback'lerinde yanlış sıralı redirection'lar).
>
> **BUG-ONB1 ve BUG-ONB2 tamamen düzeltildi:**
> - **BUG-ONB1** (2 parça): (1) `1c9c33c` — `internal/webserver/server.go`'nun LAN-adres tespiti (`getLocalIPs`, `Settings`/`memo remote status`/script'lerin hepsinin kullandığı) artık `docker`/`br-`/`veth`/`virbr`/`tun`/`tap`/`podman`/`cni`/`flannel`/`kube-bridge`/`cali` önekli sanal arayüzleri atlıyor — aynı mantığın kullanılmayan bir kopyası zaten dosyada duruyordu, ikisi tek, doğru implementasyonda birleştirildi. (2) `dec0c0a` — `get-memo-server.sh`/`get-memo-server-beta.sh` artık kurulum/güncelleme sonunda gerçek `http://<ip>:<port>` adresini basıyor; adres `ip route get 1.1.1.1`'in kaynak IP'siyle (Docker bridge'lerini doğal olarak atlıyor, çünkü onlar hiç outbound routing'de kullanılmıyor) ve port, unit dosyasının kendi `ExecStart` satırından tespit ediliyor (varsayılan olmayan `--port` de doğru yansıyor).
> - **BUG-ONB2** (`97aa57f` + `dec0c0a`) — `cli_service.go`'ya `memo service restart` eklendi (`systemctl --user restart memo.service`'i sarmalıyor), `printServiceUsage()` ve script'lerin "Manage over SSH" bölümü artık `--user` gerekliliğini açıkça yazıyor.
> - Go: build/vet/test `-race` yeşil (`TestIsVirtualNetworkInterface`, `TestPrintServiceUsage_MentionsRestartAndUserFlag` yeni). Script'ler `bash -n` ile sözdizimi doğrulandı + port/`--lan`/IP çıkarma mantığı örnek unit-dosyası içeriğine karşı ayrıca test edildi; gerçek bir systemd kurulumuna karşı uçtan uca bu ortamda denenmedi.
>
> **BUG-ONB3 tamamen düzeltildi** (2 parça): (1) `6125f39` — `isAlive()` artık 401'i "canlı ama yetkisiz" sayıyor (sadece transport hatası "ölü" sayılıyor), `BackendUnreachableOverlay` gate henüz karar vermemişken (`valueOrNull == null`) de gizleniyor — login sonrası "sunucuya bağlanılamıyor" flaş'ı kapandı. (2) `576d200` — `auth_gate_overlay.dart`'taki 4 login/setup yolunun hepsinde (`_submit`, `_enterToken`, `_loginPassword`, `_loginToken`) `api.setSessionToken()`'ın kendi persistence'i (`onRemoteTokenLearned`) fire-and-forget olduğundan, hemen ardından gelen `ref.invalidate(authGateProvider)` bazen henüz diske yazılmamış (eski/boş) token'ı okuyup kullanıcıyı kurulum ekranına düşürüyordu — her 4 yolda `prefs.setString('memo_remote_access_token', token)` artık invalidate'ten önce `await`leniyor. `flutter test` 229/229, analyze temiz, Rule #8 grep temiz.
>
> 2026-08-11 — kullanıcının RPi'sindeki (`192.168.1.106:8090`) canlı web kurulumunda yeni bulgular: BUG-ONB5 (RAM okuma şüphesi, hâlâ açık) — sadece kayda geçirildi, aşağıda. **BUG-ONB4 düzeltildi** (gate açıkken arka plan poll'lerinin 401 gürültüsü) — `authGateBlocked()`/`cancellablePause()` (`frontend/lib/providers/gate_guard.dart`) gate kapalıyken models/embedding/download/mood/whatsapp/cli-running/connection-status poll'lerini askıya alıyor, `messagesProvider` gate altında 401'i sessizce boş sohbet olarak açıyor ve gate kapanınca `chat_screen.dart`'ın listener'ı yeniden yüklüyor. Düzeltme sürecinde ayrı bir gerçek bug daha bulundu ve kapatıldı: `mood_provider.dart`'ın `Stream.periodic(...).asyncExpand(...).distinct()` deseni, iç generator'ın gate kapalıyken hep boş dönmesi (`return;`, hiç `yield` yok) durumunda periyodik Timer'ı dispose'da iptal etmiyordu (minimal, ağsız bir repro ile doğrulandı — asyncExpand+distinct+her-zaman-boş kombinasyonu genel olarak şüpheli, sadece mood'a özgü değil); `modelStatusProvider`'ın da kullandığı kanıtlanmış `while(alive)+cancellablePause` deseniyle yeniden yazıldı. Ayrıca önceki oturumun soruları: hesap yokken login ekranı + uninstall-selfhosted.sh eklendi.
>
> 2026-08-05 — bir önceki denetimde bulunan 3 bug (LK-1, SF-5, RC-7) `/code-review` + `/codebase-memory` ile doğrulanıp hepsi düzeltildi:
> - **LK-1** (`14f4486`) — `internal/agentcli`'nin `ChatCompletionStream`'i (Claude Code + Codex, ikisi de) ctx iptalinde sadece doğrudan alt süreci öldürüyordu; `--dangerously-skip-permissions`/`--dangerously-bypass-approvals-and-sandbox` ile başlattığı bir torun süreç stdout pipe'ını açık tutarsa `scanner.Scan()` sonsuza kadar bloklanıyordu. `cmd.Cancel` artık tüm process group'u öldürüyor (`internal/llama`'nın Setpgid deseni), `cmd.WaitDelay` (5s) torun süreç yine de kaçarsa yedek. İlk versiyon `/code-review`'dan geçti, 2 gerçek eksik bulundu ve kapatıldı: process-group kill eksikti (sadece pipe'ı zorla kapatıyordu, süreci öldürmüyordu — `--dangerously-*` yetkisiyle çalışan bir süreç arka planda öldürülmeden kalıyordu), test'in sabit 200ms bekleme süresi gerçek bir senkronizasyon garantisi değildi (marker-file polling'e çevrildi).
> - **SF-5** (`7f434ed`) — `callAgentStream`'in bir dalı (`streamCh` boş kapanırsa) terminal chunk göndermiyordu. Gerçek pipeline'ın (`agent.Executor`/`Pipeline.RunStream`) her çıkış yolu zaten terminal chunk gönderiyor — bu yüzden bugün canlı olarak tetiklenemez, ama gelecekteki bir pipeline değişikliğine karşı savunma amaçlı düzeltildi. Test edilebilir olması için `drainAgentStream` diye ayrı bir metoda çıkarıldı.
> - **RC-7** (`5294014`) — `Shutdown()`'ın `close(memorySaveCh)`'i, hâlâ süren bir stream goroutine'inin `saveMemoryAsync` gönderimiyle yarışabiliyordu (`webSrv.Stop()` sadece HTTP handler'ın kendi call stack'ini bekliyor, arkaplan goroutine'lerini değil) — panic oluyordu, başka bir goroutine'in `recoverStreamPanic`'i yanlış ilişkilendirilmiş şekilde yakalıyordu. `saveMemoryAsync` artık kendi gönderimini recover ediyor, doğru loglanmış bir kayıpla.
>
> Her üçü de kendi reprodüksiyon testiyle geldi (fix geri alınınca gerçekten kırıldığı doğrulandı), `-race` ile tüm backend yeşil.
>
> 2026-07-24 — **TD-2 tamamen kapatıldı** (`e88aa0d`/`7dfdd99`/`d875fbe`/`169e069`/`ea67c31`): inference-contention yarısı (cap/eviction yarısı zaten `a925109` ile kapanmıştı). Yeni `App.beginBackgroundLLMCall`/`preemptBackgroundLLM` (`internal/app/llm.go`) — `extractAndPinFacts` artık kendi LLM çağrısını iptal edilebilir bir context üzerinden yapıyor; gerçek bir chat mesajı local model'e (tek inference slot, `llama-server --parallel 1`) gitmek üzereyken (`callLLMStream`'in local dalı, `SendMessage`/`-WithImage`/`-WithFile`) hâlâ süren extraction çağrısını önce iptal ediyor — böylece yeni mesaj artık extraction'ın arkasında sıraya girmiyor. `callLLM`'in kendisine eklenmedi (hem gerçek gönderim hem arka plan çağrıları paylaşıyor — extraction'ın kendi çağrısını kendi kendine iptal etmesini önlemek için preemption sadece sırf-gerçek-chat giriş noktalarına eklendi). 3 regresyon testi (`TestPreemptBackgroundLLM_*`, `TestBeginBackgroundLLMCall_*`).
>
> 2026-07-22 — **CRITICAL, bulunup aynı gün düzeltildi** (`fd6fdd2`): `internal/provider`'da hiçbir vendor'a özel test yokken (`internal/agent` gibi sadece paylaşılan/genel mantık test ediliyordu) `claude.go` için test yazarken bulundu — `ChatCompletion`/`ChatCompletionStream`, `ChatRequest.Model` boşsa provider'ın kendi configured modeline düşen bir fallback hesaplıyordu ama bu hesaplanan değeri hiç kullanmıyordu; `buildClaudeRequest` doğrudan `req.Model`'i okuyordu. `internal/app/llm.go`'daki **ana, normal sohbet streaming yolu** `ChatRequest.Model`'i hiç set etmiyor — yani Claude aktif provider olarak seçiliyken **her normal sohbet mesajı Anthropic API'sine boş `"model": ""` gönderiyordu.** Gemini'de aynı fallback deseni var ama model URL path'inde doğru kullanılıyor (bug yok); OpenAI'da da body'de doğru kullanılıyor — sadece Claude etkilenmişti. Düzeltme + regresyon testleri (`TestClaudeProvider_ChatCompletion_FallsBackToConfiguredModel` ve stream eşleniği, fix'ten önce fail ettiği doğrulandı) aynı commit'te.
>
> `internal/provider` test kapsamı genel olarak da genişletildi: `openai_test.go` (`912097b`, %16→%28.2 — 6 diğer vendor'ın (`grok`/`groq`/`ollama`/`llama.cpp`/`opencode-zen`/`opencode-go`/`openrouter`) da paylaştığı ortak mantığı kapsıyor) ve `claude_test.go` (`fd6fdd2`, %28.2→%41.0).
>
> 2026-07-21'deki derin taramada (`internal/agent`, `internal/orchestra`, `internal/memory`, `internal/whatsapp`, `internal/calendar`) bulunan 11 bug'ın **hepsi** tek tek düzeltildi, her biri kendi regresyon testiyle (fix'ten önce gerçekten fail ettiği doğrulanarak) ayrı commit'te:
> - **BUG-C1** `311e5de` — agent sandbox escape (symlinked ancestor + not-yet-existing file)
> - **BUG-H3/H4** `c9fae03` — orchestra fallback zinciri yanlış model + chief çağrılarının fallback'siz olması
> - **BUG-H5** `971c9e9` — consolidation'la birleşen kayıtların RAG'da 187 güne kadar duplicate kalması
> - **BUG-H6** `a45a53e` — canlı WhatsApp medya mesajlarının (caption'lı) sessizce kaybolması
> - **BUG-M4** `a28cb06` — WhatsApp `Unread` alanı → `TotalReceived` (gerçek anlamıyla yeniden adlandırıldı)
> - **BUG-M5** `a5119d0` — giden WhatsApp mesajının yerel kayıt hatası artık loglanıyor
> - **BUG-M6** `0739234` — agent mesaj budaması artık assistant+tool_call gruplarını bozmuyor
> - **BUG-M7** `4499976` — reminder/routine loop artık başlangıçta hemen tetikleniyor (1 dakika beklemiyor)
> - **BUG-L2** `0752ba5` — tehlikeli komut path-koruması `--flag=/path` argümanlarını da yakalıyor
> - **BUG-L3** `780064a` — orchestra'da stream-ortası hatalar artık retry/fallback deniyor
>
> Kalan: **TD-2**'nin inference-contention yarısı (bilinçli kabul edilmiş, aşağıda).
>
> **TD-1 kapatıldı** (`18ea65c`/`69a4ae3`): backend'e `POST /api/routines/sync-offset` eklendi, Flutter GUI her client (re)connect'inde mevcut `DateTime.now().timeZoneOffset`'i gönderiyor, backend tüm routine'lerin `UTCOffsetMinutes`'ını buna göre güncelliyor. Gerçek IANA zone değil, ama DST geçişi/lokasyon değişikliği artık bir sonraki bağlantıda kendini düzeltiyor — donmuş offset sorunu pratikte çözüldü.
>
> **TD-2**'nin cap/eviction yarısı kapatıldı (`a925109`): `pinnedFactsLimit` 50→75, ve yeni `FindPinnedMergeCandidates`/`savePinnedMerged`/`runPinnedConsolidation` pinned facts havuzunu kendi içinde dedup'lıyor (genel consolidation zaten `source='explicit'`i hariç tutuyordu — bu boşluğu kapatan hiçbir mekanizma yoktu). TD-2'nin inference-contention yarısı (local model tek slotta extraction ile chat'in yarışması) hâlâ açık, bkz. aşağıda.
>
> `pidListeningOnPort` (`internal/llama`, `internal/whisper`) Linux'ta `lsof`/`fuser` bağımlılığı olmadan native `/proc/net/tcp` okuyacak şekilde düzeltildi (`91300f9`/`52b6e9f` + testler `2f839a2`/`d0bb02c`) — her iki araç da kurulu değilse port temizliğinin sessizce no-op olduğu senaryoyu Linux'ta tamamen kapatır (macOS `lsof`/`fuser`'da kaldı, risk zaten düşük).
>
> 2026-07-20 (Session 46 fix pass) — Session 46 review maddeleri kapatıldı:
> - **BUG-H1** `20ba4f0` — agent `trySend` non-blocking-first + regression tests  
> - **BUG-H2** `b1fad30` — WhatsApp `localTrySend` + terminal cancel chunk  
> - **BUG-L1** `a7d4ace`/`21f9623` — low-value ack/greeting RAG skip (`IsLowValueTurn`)  
> - **BUG-M1** `4670b63` — mobile `sendMessage` re-entrancy + stream generation  
> - **BUG-M2** `b77017f` — SettingsDialog nested `ScaffoldMessenger`  
> - **BUG-M3** `79bda62`/`fac700f`/`f53c2ec` — L10n chat_message_list, chat_input, provider/skill dialogs  
>
> Kalan: bilinen teknik borç (routine DST offset, pinned-facts cap) + L10n residual (orchestra_config_dialog ve diğer düşük-trafik dialog stringleri).

---

## Özet

| Severity | Açık |
|----------|------|
| 🔴 CRITICAL | 0 |
| 🟠 HIGH | 1 (BUG-PLAN10 — sohbet modeli çalışan task için ayrıntılı YANLIŞ "bozuk" hikayesi uyduruyor) |
| 🟡 MEDIUM | 1 (BUG-PLAN12 canlı task aktivitesi sohbette yok) |
| 🟢 LOW | 2 (BUG-PLAN9 planı sohbetten onayla · BUG-PLAN11(b) tek tutarlı ilerleme metni, kozmetik) |
| 🔧 TEKNİK BORÇ | 0 |
| ⏳ FIX İNDİ, CANLI DOĞRULAMA BEKLİYOR | 2 (BUG-PERM1 + e43627e/b9fc2eb — gerçek masaüstü doğrulaması · BUG-THINK1 `08ea76ad` — gerçek Anthropic API'ye karşı doğrulanmadı, yalnızca test) |
| ✅ FIX İNDİ + CANLI DOĞRULANDI (silinecek) | 8 (PLAN1/2/3/4/5/6/7/8 — PLAN4 kod+analyze doğrulandı; PLAN11(a)+(c) `dd803d6`/`a35593f4`) |
| **AÇIK TOPLAM** | **4** (hepsi kod değişikliği = testten sonra) |

---

## ⏳ Fix indi — canlı yeniden-test bekliyor

Bu üçünün fix'i 2026-08-30 akşamı v4.5.0 canlı testinin ardından indirildi;
test ortamında güçlü model / gerçek Flutter dialogu olmadığı için **kullanıcının
canlı doğrulaması gerekiyor**. Doğrulanınca buradan silinecek (git log kayıt).

### BUG-PERM1 — interaktif sohbette agent tool izin dialogu anında kapanıyordu

- **Bulgu:** interaktif ajan sohbetinde model `create_task_md` çağırdı; izin
  ekranı **"0.1 ms bile değil, geldi gitti"** — açılıp kapandı, kullanıcı
  onaylayamadı. Backend `permission request … timed out (60s), auto-denied`
  → tur `⚠️ Agent execution cancelled (permission timeout)` ile bitti.
- **Kök neden:** `permission_dialog.dart` `isSendingProvider == false` gördüğü
  her an kendini pop ediyordu (turun bittiğinde bayat dialog kalmasın diye).
  Ama agentic döngüde tool-round sınırında `isSendingProvider` bir frame
  boyunca `false` okunabiliyor — tam da `create_task_md` /
  `start_self_driving_task` izin isteği o ana denk gelince dialog flash edip
  kapanıyordu.
- **Fix:** "build sırasında zaten false" yolundaki anında pop, **~1.4 sn
  debounce** edildi (`_staleCheckTimer`) — ancak hâlâ not-sending ise pop eder.
  `ref.listen` gerçek `true→false` geçişi yolu (turun gerçekten bitmesi)
  aynen kaldı. Yeni test: "brief not-sending dip must not flash-close a live
  dialog" + mevcut "already-ended turn" testi debounce'a göre güncellendi.

### BUG-PLAN1 — planlayıcı/uygulayıcı LLM çağrıları 90s'e takılıyor + tek hatada liste `failed`

- **Bulgu:** `# mod: planlayıcı` görevi başladı, planlayıcı turu ~2 dk sonra
  `[custom] provider request timed out` → liste `failed`, kullanıcı elden
  yeniden kurmak zorunda. Ücretsiz endpoint 90 sn'de yanıt başlığı dönemedi.
- **GERÇEK kök neden (2. turda bulundu):** agent pipeline (`pipeline.go:143`)
  **non-streaming** `ChatCompletion`'ı çağırıyor, `...Stream`'i değil — yani
  planlayıcı/coder/escalator turları `openai.go`/`claude.go`/`gemini.go`'nun
  düz `client`'ını kullanıyor, `Timeout: 120s` (claude/gemini'de ayrıca 30s
  `ResponseHeaderTimeout`). Büyük bir planlama çağrısı tek dev JSON gövdesi
  döndürüyor, ilerleme sinyali yok, 120s'de kesiliyordu. İlk fix `streamCl`'i
  değiştirmişti — pipeline onu hiç kullanmıyor.
- **Fix:** (a) üç provider'ın da non-stream `client.Timeout` **120s → 300s**
  (claude/gemini header timeout 30s → 120s). (b) `runPlanExec` planlayıcıyı
  **3 kez** dener (3s/6s backoff), bad-JSON dahil; hepsi tükenince `failPlan`
  (`plannerMaxAttempts = 3`). (c) streaming yolu için de idle-timeout altyapısı
  eklendi (`drainStreamIdle`, `tasklist_stream.go`) — `streamCl`
  `ResponseHeaderTimeout` 90s → 240s. **Canlı doğrulandı:** hy3:free her
  çağrıda ~120s timeout veriyor ama 3. denemede plan geçiyor; 300s ile ilk
  denemede geçmesi bekleniyor.

### BUG-PLAN2 — review/compaction 120s + kabul-komutu 180s sabit timeout

- **Fix:** `callLLMForReview` (worker CEO review, fuzzy kabul kontrolü,
  state-doc compaction, bitiş raporu) provider/local yolları **120s → 240s**.
  `runCheckCommand` (kabul-komutu, ör. `go test ./...`) 180s → config'lenebilir
  `TaskLoopConfig.AcceptanceCommandTimeoutSec` (default 300).

### ✅ BUG-PLAN3, 5, 6, 7, 8 — planexec çalıştırma sertleştirmesi (2. turda bulundu, 3. turda canlı doğrulandı)

**Fix commit `86de45f`. 3. canlı turda hepsi doğrulandı: aynı zayıf modelle
(`hy3:free`) 6 maddeli Task.md → 5/6 madde done, gerçekten çalışan blog
sitesi (curl ile signup→login→post→index doğrulandı).** Aşağısı bulgu/kök
neden kaydı, silinebilir.

2026-08-30 gece, güçlü planlayıcı (`claude-3-5-sonnet` local proxy) ile ikinci
tur. **Planlayıcı bu sefer sorunsuz çalıştı** — `create_task_md` mükemmel
Task.md yazdı, `start_self_driving_task` planexec listesi kurdu, planlayıcı
turu 8 adımlık (literal_content + acceptance_checks + DAG) 20 KB'lık geçerli
`Plan.md` üretti, liste `awaiting-plan-approval`'a düştü. API'den approve →
`executing` → S1 (`db.py` + `blog.db`) **gerçekten çalıştı, item 1 done**.
Sonra 6 bug yüzünden 1/8'de durdu:

- **BUG-PLAN3 — `# onay: otomatik` ve `# hafıza: kapalı` Task.md header'ları
  parse ediliyor ama TÜKETİLMİYOR.** `CreateTaskListFromTaskMd` yalnızca
  `Headers["mod"]`'u okuyor. `AutoApprovePlan` global config; per-liste
  auto-approve alanı yok. Model header'ı yazıyor, hiçbir etkisi olmuyor →
  liste onay kapısında asılı kalıyor.
- **✅ BUG-PLAN4 (fix `91c763bc`, doc `1191ae92`) — Görevler sekmesi kartı `task_detail_screen.dart`'ı
  açmıyordu.** Kart `onTap`'i eski `_showDetailDialog` statik-snapshot modalını
  açıyordu: bayat statü ("Idle"), plan onay butonu YOK → `# onay` header'sız
  planexec listesi UI'dan onaylanamıyordu. `onTap` artık `TaskDetailScreen`'e
  push ediyor (canlı görünüm + self-fetch eden `_PlanApprovalSection`
  düzenlenebilir Plan.md + "Onayla ve çalıştır" + `waiting-escalation`
  banner'ı). `_showDetailDialog` ve 2 kullanılmayan import silindi. Kod +
  `flutter analyze` + widget testleri yeşil.
- **BUG-PLAN5 — paralel step goroutine'lerinden eşzamanlı `SavePlan` yarışı.**
  `MaxParallelSteps=3` step goroutine'i `IncrementStepAttempts` → `mutatePlan`
  → `SavePlan` → `fileutil.AtomicWrite` (tmp + rename) çağırıyor. İki goroutine
  aynı `.plan.json.tmp`'yi yazıp rename edince biri `rename …tmp: no such file
  or directory` alıyor. `mutatePlan` yorumu "tek yazar: engine goroutine'i"
  diyor ama artık yanlış — `runOneStep` paralel goroutine'lerden çağırıyor.
  Transient (bu turda öldürmedi) ama attempt sayacı kaybı + potansiyel plan
  bozulması.
- **BUG-PLAN6 — fuzzy kabul kontrolü non-git projede her zaman başarısız.**
  `runFuzzyCheck` bağlam olarak `git -C projectPath diff --stat` kullanıyor.
  `~/memo-blog-test` git deposu değil → çıktı boş → doğrulayıcı "değişiklik
  özeti boş, ölçüt doğrulanamadı" → adım stuck (S7/style.css).
- **BUG-PLAN7 (HARD BLOCKER) — `command` kabul kontrolü `expect: "present"`'i
  stdout substring'i sanıyor.** Planlayıcı grep semantiğini (`expect:
  "present"`) `command` çeklerine de kopyalıyor:
  `{"kind":"command","spec":"python3 -c \"...print(callable(...))\"","expect":"present"}`.
  Komut exit 0 veriyor ama stdout `True`, `present` değil → `runCheckCommand`
  "output missing present" → adım stuck. S2 böyle takıldı, S3-S8 hepsi S2'ye
  bağlı olduğu için art arda stuck → 1/8.
- **BUG-PLAN8 (HARD BLOCKER) — takılan adım ne retry ediliyor ne escalate.**
  `executePlan` bir adım kabul kontrolünü geçmeyince **1 denemede** `stuck`
  yapıyor. `escalateStuckSteps` ise `s.Attempts >= maxAttempts` (3) istiyor —
  ama hiçbir şey attempt'i 1'in üstüne çıkarmıyor (per-adım retry döngüsü hiç
  yazılmamış). Sonuç: `MaxExecutorAttempts` fiilen "1 dene, bırak";
  escalation acceptance-check hatalarında hiç tetiklenmiyor.

**Ayrıca (bug değil ama gürültü):** her mesajda `LATENCY app.retrieve_memory
status=error` + `memory_save_sync status=error` — embedder :8082'de
başlamamış/erişilemiyor olabilir, "Memory off — RAG not working" banner'ıyla
tutarlı. Provider auto-revert (frontend her reconnect'te aktif provider'ı
kendi cache'ine çeviriyor) da tekrar görüldü — pre-existing, v4.5.0 dışı.

---

## 🟢 AÇIK — kod değişikliği TESTTEN SONRA (kullanıcı 2026-08-30 canlı testte istedi)

Kullanıcı v4.5.0 canlı testinde bunları bildirdi; **"kullanıcı deneyimini
kötü etkileyen bir hata değil ama"** dedi, "şimdilik kodda değişiklik yapma,
testten sonra" — sadece not.

### BUG-PLAN9 — planexec planı yalnızca Görevler sekmesinden onaylanabiliyor

- **Bulgu (orijinal):** onay UI'ı (düzenlenebilir plan + "Approve & run")
  **yalnızca** Görevler sekmesi → task detay ekranında vardı, sohbetten
  onaylanamıyordu.
- **İstenen'in kodu artık var:** `TaskActivityBlock` (`63cc1ad`/`adb363e7`),
  `awaitingPlan` fazında sohbette inline bir "Planı gör ve onayla" butonu
  gösterip `showModalBottomSheet` ile in-chat bir plan-onay sayfası açıyor
  (`_showPlanSheet`, `task_activity_block.dart`).
- **2026-09-04 eklenen:** bu widget'ın hiç testi yoktu — yalnızca altındaki
  veri modeli (`ChatTaskState.fold`) test ediliyordu, gerçekte ekrana ne
  çizildiği hiç kanıtlanmamıştı. `task_activity_block_test.dart` eklendi:
  `awaitingPlan` durumunda buton render oluyor mu, tıklanınca gerçekten
  bottom sheet açılıyor mu (fake HTTP adapter ile).
- **Öncelik:** düşük — akış artık kod+widget test seviyesinde doğrulandı,
  **gerçek backend/model'e karşı canlı doğrulanmadı**.

### BUG-PLAN10 — sohbet modeli görev durumunu okuyamıyor, AYRINTILI YANLIŞ HİKAYE uyduruyor

- **Bulgu:** kullanıcı sohbette "görev durumu ne" / "task duruyor mu" diye
  sordu. Model canlı task-listesi durumunu görebileceği bir araca sahip değil.
  `read_file` ile `Task.md`'yi okudu, 6 kutunun da `[ ]` olduğunu gördü ve
  **baştan sona uydurma bir başarısızlık anlatısı üretti:** "döngü hiçbir
  maddeyi işleyememiş", "LLM sağlayıcı yapılandırılmamış / LLM Error: no
  provider configured", "`app.py` veya `blog.db` de oluşturulmamış", "çözüm:
  Settings → Provider'dan sağlayıcı ekle". **Hepsi yanlıştı** — o sırada
  planexec **7/13 adımı bitirmiş, aktif koşuyordu**, `app.py`/`blog.db` vardı,
  escalation 2 kez tetiklenmişti. Model, checkbox'ların boş olmasını
  (BUG-PLAN11 yüzünden boşlar) "hiçbir şey olmadı"ya + "sağlayıcı yok"a
  genişletti.
- **İstenen:** sohbet modeline canlı görev durumunu verecek bir yol — bir
  agent aracı (`get_task_status` / `list_running_tasks` → id, statü, faz,
  adım ilerleme `N/M`, son adım metni, son hata, escalation sayısı), ya da
  aktif proje bir task-listesine bağlıysa sistem prompt'una özet enjeksiyonu.
  Model **veri yokken / araç yokken durum uydurmamalı**, "göremiyorum, Görevler
  sekmesine bak" demeli.
- **Öncelik:** YÜKSEK — ayrıntılı, ikna edici yanlış bilgi kullanıcıyı çalışan
  bir sistemi "komple bozuk" sanıp iptal et/yeniden kur'a itiyor.
- **Not (2026-09-01, dokümantasyon denetimi sırasında bulundu):** İstenen
  `get_task_status` aracı `def5ac1c` commit'inde ("feat(agent): add
  get_task_status read-only tool + anti-fabrication prompt") gerçekten
  eklenmiş — bu satır yazıldıktan sonraki bir commit. Araç kayıtlı, "veri
  yokken uydurma" talimatı sistem promptunda var (bkz. tools.go'daki
  açıklaması). **Ama bu, canlı olarak doğrulanmadı** — modelin gerçekten
  bu aracı çağırıp uydurmaktan vazgeçtiği bir canlı testle kanıtlanmadı,
  sadece kod var olduğu için burada "muhtemelen düzeldi" diye not
  düşülüyor. Kapatmadan önce gerçek bir "görev durumu ne" sorusuyla canlı
  test edilmeli.
- **Not (2026-09-04):** `formatRunningTask`'ın (aracın modele döndürdüğü asıl
  metni üreten fonksiyon) hiç testi yoktu — yalnızca "hiçbir şey çalışmıyor"/
  "engine yok" negatif dalları test ediliyordu, tam da bu bug'ın gerçekleştiği
  "görev gerçekten koşuyor" pozitif dalı hiç kanıtlanmamıştı.
  `TestFormatRunningTask_ReportsRealProgressNotFabrication` eklendi (7/13
  adım, 2/6 madde, sub-agent, geçen süre — hepsinin doğru metne yansıdığını
  doğruluyor) + `TestFormatRunningTask_WorkerMode_OmitsStepCount`. Bu,
  aracın **doğruyu söylediğini** kanıtlıyor; modelin **bu aracı gerçekten
  çağırıp uydurmayı bıraktığını** kanıtlamıyor — o kısım hâlâ gerçek bir LLM
  ile canlı test gerektiriyor, yukarıdaki not geçerliliğini koruyor.

### BUG-PLAN12 — canlı task aktivitesi sohbette görünmüyor, ayrı Görevler ekranı şart

- **Bulgu (kullanıcı):** "alt agent çalıştırmış / adım tamamlanmış / plan onayı
  gibi şeyleri anlık sohbette görebileyim, ayrı task screen'a muhtaç kalmak
  istemiyorum." Şu an çalışan bir planexec listesinin tüm canlı sinyalleri
  (adım başladı/bitti, alt-agent turu, kabul kontrolü, escalation, handoff
  context doluluğu, `awaiting-plan-approval`) **sadece** Görevler sekmesi → task
  detay ekranında. Sohbet tarafında hiçbir iz yok; sohbet modeli de göremiyor
  (BUG-PLAN10).
- **İstenen:** `start_self_driving_task` bir sohbetten başlatıldığında o
  sohbete canlı bir aktivite akışı düşsün — ajan `tool_executing` /
  `tool_result` baloncukları gibi hafif satırlar: "Adım 4/13: create_post()…",
  "Adım 4/13 geçti", "Adım 6 tekrar kuyruğa alındı (kabul kontrolü)",
  "Adım 6 escalate → 6 alt-adıma bölündü", "plan hazır — [Onayla]".
  Muhtemelen mevcut `agentEventBus`
  + `app_shell` listener deseni; backend zaten `event: taskloop:*` yayınlıyor
  (`taskloop:escalating/escalated` loglarda görülüyor) — bunları SSE'den
  sohbete köprülemek yeterli olabilir.
- **İstenen'in kodu artık var:** aynı `TaskActivityBlock` (`63cc1ad`/`adb363e7`)
  bu bug'ı da kapsıyor — canlı adım/madde ilerlemesi, zaman damgalı aktivite
  logu (`tool`/`step_done`/`step_retry`/`escalate`/... satırları), escalation
  artık (`a35593f4`, BUG-PLAN11(c)) alt-adım sayısını da açıklıyor. Sohbette,
  ayrı ekrana gitmeden. **2026-09-04:** `task_activity_block_test.dart`
  eklendi — koşan bir görevde ilerleme satırının hem adım hem madde sayısını
  taşıdığı ve log satırlarının gerçekten render olduğu kanıtlandı.
- **Öncelik:** orta-yüksek (BUG-PLAN9 + BUG-PLAN10'un şemsiye çözümü; "sohbetten
  yönet" vaadinin özü) — kod+widget test seviyesinde doğrulandı, **gerçek
  backend/model'e karşı canlı doğrulanmadı**.

### ✅ BUG-THINK1 (fix `08ea76ad`) — Claude'un extended thinking içeriği hiçbir zaman gösterilmiyordu

- **Kök neden (buradaydı, artık düzeltildi):** `claude.go`, effort level
  seçiliyken Anthropic'e gerçekten `thinking:{type:"adaptive"}` gönderip
  ([claude.go:478](internal/provider/claude.go:478)) gerçek para/token
  harcıyordu, ama dönen `"thinking"` content block'ları/`thinking_delta`
  event'leri ne `ChatCompletion`'da ne `processSSE`'de işleniyordu —
  `claudeBlock`/`ChatResponse`'ta alan bile yoktu, `internal/app`'ta
  `.Thinking` okunan tek satır yoktu. Flutter tarafı zaten tam hazırdı
  (`ChatMessage.thinking`/`hasThinking`, `_ThinkingSection`), hiç Dart
  değişikliği gerekmedi.
- **Fix (`08ea76ad`):** `claudeBlock`/`ChatResponse` `Thinking` alanı
  kazandı; `ChatCompletion` `"thinking"` bloklarını, `processSSE`
  `thinking_delta`'yı ayrıştırıyor. `callLLMStream`'in external-provider
  döngüsü artık `chunk.Thinking`'i hem canlı SSE'ye hem (yeni
  `thinkingCtxKey`/`finishStream` üzerinden) `sessions.ChatMessage.Thinking`'e
  köprülüyor. **Yan ürün:** local reasoning modellerin (`<think>` etiketi)
  thinking-only chunk'ları da `chunk.Content != ""` kapısı yüzünden hiç
  SSE'ye ulaşmıyordu — o da aynı commit'te düzeltildi.
- **Kapsam dışı bırakılanlar (bilinçli):** non-stream yol (yalnızca title-gen/
  memory/mood/routine gibi arka plan çağrıları kullanıyor, interaktif
  sohbet değil — `trace_path` ile doğrulandı); Gemini'nin cevap tarafı
  (zaten thought summary istemiyor, aktif kayıp değil); Claude thinking-block
  signature echo'su çoklu-tur tool-use continuation için (düz sohbet yolu
  hiç `ChatRequest.Tools` göndermiyor, o yüzden gerekmiyor).
- **Doğrulama:** 4 yeni test (provider wire-format parse × 2, sessions
  persistence + reload × 1, `callLLMStream` uçtan uca × 1), hepsi fix'ten
  önce derlenmediği doğrulanarak eklendi. `go build/vet/test -race` tüm
  paketlerde yeşil. **Gerçek Anthropic API'ye karşı canlı doğrulanmadı** —
  yalnızca sahte HTTP sunucularıyla.

### BUG-PLAN11 — planexec sayaç kaosu: (a)+(c) düzeltildi, (b) hâlâ açık

- **Orijinal bulgu (3 parça):** aynı anda ekranlarda kart "0/6 done", detay
  bar "Steps: 7/13", detay satır "Executing 0/6", sohbet modeli "6 madde
  hepsi boş" — (a) madde ilerlemesi 0'da donuk kalıyordu, (b) dört farklı
  sayı aynı anda gösteriliyordu, (c) escalation'ın adım eklediği (6→8→13)
  UI'da hiç açıklanmıyordu.
- **(a) zaten düzeltilmişti, dokümana hiç işlenmemiş:** kök neden —
  planlayıcının verdiği `item_id="1".."N"` (sıra no) ile gerçek
  `TaskList.items[].id` (UUID) arasındaki eşleme — `dd803d6`'da (2026-08-30)
  çözülmüş: `Engine.syncItemProgress` her `executePlan` dalgasından sonra
  (yalnızca en sonda değil) `idx+1` sırasını `plan.StepsForItem(ordinal)`
  ile eşleştirip gerçek `TaskItem.ID`/`.Line`'ı `SetItemDone`/`MarkItemDone`'a
  veriyor. `TestEngine_PlanExec_ItemsCompleteIncrementally` bunu kilitliyor.
  `codebase-memory` ile doğrulandı (2026-09-04): mevcut kod tam olarak bunu
  yapıyor, dokümanın üstteki iddiası (checkbox'lar hiç işaretlenmiyor) artık
  gerçek değil.
- **(c) bu oturumda düzeltildi** (`a35593f4`): `escalateStuckSteps` /
  `resumePendingEscalation` escalation **başladığında** ("S6 takıldı →
  yeniden planlanıyor") aktivite satırı basıyordu ama **çözüldüğünde** hiç
  basmıyordu — payda sessizce büyüyordu. Artık ikisi de çözüm anında ikinci
  bir `escalate` aktivite satırı basıyor: gerçek bölünmede
  `"S6 3 alt-adıma bölündü (+2 adım)"`, 1'e-1 yeniden yazımda (bölünme değilse)
  `"S1 yeniden planlandı"`. Frontend değişikliği gerekmedi — `escalate` kind'i
  zaten stilize ediliyordu (başlangıç satırından). Test:
  `TestEscalationResolvedText`, `TestEngine_PlanExec_EscalationAnnouncesStepCount`.
- **(b) kısmen düzeltildi (`849f84fa`), tam birleştirme hâlâ açık:** sohbet
  aktivite bloğu zaten örnek alınacak formattaydı (`task_activity_block.dart`'ın
  `_progressText`'i: `"adım N/M · madde a/b"`, `task_card_step`/`task_card_item`
  etiketleriyle). `task_detail_screen.dart`'ta madde sayısı rozetin yanında
  **hiç etiketsiz** çıplak "N/M" olarak duruyordu (adım bölümünün kendi
  başlığı vardı, bu ikincisinin yoktu) — artık aynı `task_card_item`
  ("madde"/"item") etiketini kullanıyor. `tasks_screen.dart`'ın kartı hâlâ
  dokunulmadı: `TaskListInfo` (düz liste endpoint'i) adım sayısını hiç
  taşımıyor, eklemek backend alanı gerektirir — kapsam dışı bırakıldı.
  Kozmetik, düşük risk, isterse ayrı bir tur.
- **Öncelik:** düşük (kalan tek parça kozmetik; asıl "ilerleme hiç
  çalışmıyor" ve "escalation açıklanmıyor" şikayetleri kapandı).

### Not — BUG-PERM1 canlı durumu (2026-08-30 öğleden sonra)

Yeni web build'le sohbetten `create_task_md` denendi: izin dialogu **anında
kapanmadı**, "0:59" sayaçla stabil göründü, kullanıcı Allow'a bastı, `Task.md`
+ `Plan.md` üretildi. Yani BUG-PERM1'in çekirdeği çözülmüş görünüyor. Ek
olarak `e43627e` (auto-permission: canlı getter + bekleyen istekleri drenaj)
ve `b9fc2eb` (dialog en az 2 sn görünür kalır) da indi — kullanıcının gerçek
masaüstü uygulamasında bunları da doğrulaması iyi olur, sonra BUG-PERM1
buradan silinebilir.

---

## Bilgi notları (bug değil, takip amaçlı)

### Embedding modeli 2GB RAM'de çalıştırılamadı (bilgi, bug değil — 2026-08-11)

- **Sorun:** RPi 2GB RAM, ~1-1.5GB boş RAM varken nomic-embed-text-v1.5 Q4_K_M (~82MB dosya) / Q3_K_S embedder'ı başlatılamıyor.
- **Ölçek (doğrulanmış, llama.cpp):** nomic-embed-text-v1.5 = 137M parametre. Q4_K_M ~82MB, Q3_K_S ~55MB dosya. Embedding server'ın RSS'i dosya boyutu + model overhead + KV cache ile ~150-300MB arası oluyor — yani **model boyutu 2GB sistemde asla sorun değil**; 1-1.5GB boş RAM tek başına fazlasıyla yeterli. Çalışmama sebebi model boyutu olamaz.
- **Olası gerçek sebepler (incelenmeli):** (1) **OOM killer** — 2GB sistemde başka süreçler (chat modeli llama-server, memos uygulaması, backend, node, Docker bridge servisleri) RAM'i dolduruyorsa embedder süreci öldürülüyor olabilir (`dmesg`/`journalctl -k | grep -i oom` ile doğrulanır); (2) embedder başlatma hatası başka bir sebeple (port çakışması, arm64 binary'sinin eksik olması — RPi arm64, `binaries/` içinde linux/arm64 mevcut mu kontrol edilmeli); (3) `embedding_auto_start: false` — config'de kapalıysa embedder hiç başlatılmıyor, "çalıştıramadım" hissi veriyor.
- **Not:** RPi'deki config'de `embedding_auto_start: false` — kullanıcı elle başlatmadıkça embedding devreye girmiyor.

---

## 🔧 TEKNİK BORÇ

*(hiçbiri açık değil — TD-3 `8e470e3` ile, TD-4 kullanıcı tarafından Cloudflare dashboard'undan halledildi)*

---

## Residual (fix değil, takip)

- **L10n:** kapatıldı (`36c8a38`) — orchestra/provider/skill config dialogları, GPU tab, sistem/incognito prompt tabları, skills boş durumu ve daha fazlası L10n'a bağlandı.
- **Streaming:** Diğer bare `select` yolları (varsa) ayrı canary/review ile taranmalı; H1/H2 class kapatıldı.

---

*Düzeltilen bir bug'ı burada tekrar dokümante etmeye gerek yok — `git log`/commit mesajları zaten kalıcı kayıt. Bir madde düzeltilince buradan tamamen silinsin.*
