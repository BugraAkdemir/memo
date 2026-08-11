# Evrensel Auth Ekranı (Setup + Login Gate + Hesaplar Sekmesi) — Tasarım

**Tarih:** 2026-08-11
**Kapsam:** Backend (`internal/app/remote_auth.go`, `internal/webserver/handlers_auth.go`, `bridge.go`) + Flutter masaüstü ve Flutter web (`frontend/`). Mobil (`mobile/`) bu spec'in kapsamı dışında — ayrı, daha sonra yapılacak bir iş.

## Problem

Raspberry Pi kurulumunda (`--lan` + auth modu `password`) Flutter web uygulaması "Sunucuya bağlanılamıyor" hatası veriyor ve kullanıcı uygulamaya hiç giremiyor.

**Kök neden (kodla doğrulandı, 2026-08-11):**

1. `--lan` → sunucu `0.0.0.0:8090`'a bağlanır → `remoteAuthMiddleware` (`internal/webserver/server.go:846` → `remoteAuthOK`) her `/api/*` isteğinden kimlik ister.
2. Auth modu `password` → geçerli tek kimlik `/api/auth/login`'den alınan **JWT session token**'ıdır; cihaz token'ı password modunu karşılamaz (bilinçli tasarım).
3. Flutter web istemcisinde **login UI hiç yok** — el yazması webui'nin login ekranı vardı, Flutter web build'e geçilince kayboldu (`loginRemote()` `api_client.dart:1027`'de hâlâ mevcut ama **ölü kod** — hiçbir çağıranı yok).
4. Uygulama açılışında `isAlive()` → `GET /api/version` → 401 → `false` → `connectionStatusProvider` 30 saniyede bir poll edip `BackendUnreachableOverlay`'i gösterir: "Sunucuya bağlanılamıyor". Yani bağlantı **var**, kimlik **yok** — ama kullanıcıya "bağlanamıyor" deniyor.
5. Overlay'ın tek kaçış yolu "Sunucu değiştir" dialog'undaki token alanı — password modunda token işe yaramadığı için tam kilit.

## Karar Özeti (brainstorming'de netleşenler)

| Soru | Karar |
|---|---|
| Kapsam | Auth ekranı **tüm platformlarda** (masaüstü Linux/macOS/Windows + web) aynı mekanizmayla |
| Yerleşim | Gate'ler mevcut overlay stack'ine `AuthGateOverlay` olarak girer; alternatif (root-level boot screen) reddedildi |
| Sıralama | Auth ekranı kişiselleştirme wizard'ından **önce** (uygun, kullanıcı onayı) |
| Hesap kimliği | Tek serbest-metin **Username** alanı (ayrı e-posta alanı yok) |
| Yöntem seçimi | Setup'ta 3 seçenekli seçici: **Sadece şifre** ⭐ / Şifre + token / Sadece token |
| Auth isteğe bağlı | "Bu Memo'ya başka cihazlardan erişilecek mi?" toggle (varsayılan: **hayır**) — hayır = auth kurulmaz, wizard devam eder |
| Ayarlardan değiştirme | Evet — giriş yöntemi (Remote Access tab'ında mevcut) + **yeni ayrı "Hesaplar" sekmesi** (liste, ekle, sil, şifre değiştir) |
| Şifre değiştirme | Backend'e **yeni endpoint** eklenir (`POST /api/accounts/{id}/password`) — kendi hesabı → eski şifre şart; başka hesap → admin şart |
| Token yönetimi | Remote Access tab'ında mevcut (üret/iptal) — taşınmaz |
| Açıklayıcı metinler | Auth ekranında gizlilik/offline ipuçları şart (Memo "veri dışarı çıkmaz" markası — seçenekler kafa karıştırmasın) |
| Tüm yeni metinler | TR + EN `l10n.dart` (kural #8) |

## Mevcut Altyapıda Neyi Kullanıyoruz (kod okuması ile doğrulandı)

**Backend — neredeyse hepsi hazır:**

| Bileşen | Yer |
|---|---|
| `GET /api/setup/status` → `{needs_setup, auth_mode}` — muaf (kimliksiz erişilebilir) | `handlers_auth.go:194` |
| `POST /api/setup/create-admin` — muaf; bir kez; mode'u otomatik `password`'e yükseltir (`none`/`token`/`""` → `password`); session token döner | `remote_auth.go:433`, `handlers_auth.go:213` |
| `POST /api/auth/login` → `{session_token, role}` — muaf; brute-force limiter (+ `Retry-After`); Accounts doluysa kaynak Accounts, boşsa legacy Username/PasswordHash | `remote_auth.go:296`, `handlers_auth.go:38` |
| `GET/POST /api/accounts`, `DELETE /api/accounts/{id}` — POST/DELETE admin-only; son-admin koruması; duplicate-username koruması | `remote_auth.go:487,502,545`, `handlers_auth.go:247,280` |
| `GET/POST /api/remote-access/devices`, `DELETE .../{id}` — token üret/iptal (plaintext sadece üretimde) | `remote_auth.go:136,157,179`, `handlers_auth.go:90,124` |
| `SetRemoteAuthConfig(mode, username, password)` — mode değişimi; `""` username gerekmez, `""` şifre mevcut hash'i korur | `remote_auth.go:98` |
| Session doğrulama: `ValidateRemoteSession` / `SessionRole` — JWT 12h TTL; hesap silinirse/taşınırsa hemen geçersiz | `remote_auth.go:358,371` |

**Önemli davranışlar (doğrulandı):**

- `NeedsSetup()` = `Accounts boş && legacy Username boş` (`remote_auth.go:411`). `migrateLegacyRemoteAccount` (config) eski tek-şifre kurulumlarını Accounts'a taşır → eski kullanıcılar upgrade'de setup ekranını **tekrar görmez**.
- **"Sadece token" seçeneği `NeedsSetup()`'ı kapatamaz** (Accounts'a ve legacy Username'e hiçbir şey yazılmaz) → tekrar gösterimi engellemek setup akışının başarı bitişinde **lokal bayrak** (SharedPreferences) koyar. Yeni tarayıcı/cihaz (web) SetupGate'i yeniden görür ve "sadece token" seçerse **yeni token üretilir** — zararsız: device listesi yönetimli (revoke var), her cihazın kendi token'ını alması zaten tasarımın ta kendisi.
- `callerIsAdmin` (`handlers_auth.go:161`): kimliksiz tanıma/geçersiz kredans/device token → izin verilir; sadece tanınan "user" rolü reddedilir. Lokal masaüstü davranışı hiç değişmez.
- Session token, her istekte `X-Memo-Token`/Bearer olarak gider (handleRemoteLogin doc'u bunu söylüyor) → Flutter'ın mevcut `MemoApiClient.token` alanı yeniden kullanılır.

**Frontend:**

| Bileşen | Yer |
|---|---|
| Overlay stack: `SetupWizardOverlay` + `BackendUnreachableOverlay` + `LlamaInstallerOverlay` (AppShell `Stack`'i) | `app_shell.dart:242` |
| `connectionStatusProvider` (30s poll, `isBackendAlive` üzerinden, SharedPreferences token'ı header'a bağlar) | `chat_provider.dart:866` |
| `isAlive()` — şu an 401'de `false` döner (ayrım yok) | `api_client.dart:1317` |
| `loginRemote()` — **mevcut ama ölü kod**, aktifleştirilecek | `api_client.dart:1027` |
| `BackendUnreachableOverlay` — 401'i hâlâ "bağlanamıyor" diye gösterir (yanlış mesaj) | `backend_unreachable_view.dart` |
| Remote Access tab'ı — mode chips + username/password alanları + device/token yönetimi | `settings/tabs/remote_access_tab.dart:575` |
| Setup wizard — auth'tan sonra aynen çalışır (auth gate ile ilişkisiz) | `setup_wizard_view.dart` |

**Eksik olan (bu spec'in yapacağı):** Flutter'da login/setup UI, 401'de auth gate'e geçiş, `GET /api/setup/status` + `POST /api/setup/create-admin` + accounts CRUD + şifre değiştirme için API client metotları, Hesaplar sekmesi, backend şifre-değiştirme endpoint'i.

## Yeni Bileşenler

### 1. Backend — şifre değiştirme

**`internal/app/remote_auth.go`:**

- `SessionSubject(token string) (string, bool)` — `sessionSubjectRole`'un public sarmalayıcısı (`remote_auth.go:377`), credential'ın hangi kullanıcı adına ait olduğunu döndürür. Webserver'ın şifre-değiştirme yetkilendirmesini App'e devretmesi için gerekli.
- `ChangeAccountPassword(token, id, currentPassword, newPassword string) error`:
  - `newPassword == ""` → hata.
  - `subject, ok := sessionSubjectRole(token)`; `!ok` → yetki hatası (tanınmayan oturum).
  - `id == subject` (kendi hesabı) → `currentPassword` doğrulanmalı (ilgili hesabın `PasswordHash`'i ile `VerifyPassword`); yanlışsa hata.
  - `id != subject` (başka hesap) → subject'in rolü "admin" olmalı; `currentPassword` gerekmez.
  - Hesaplar boşsa (legacy mod) → hata ("password change requires accounts" — legacy tek kullanıcı zaten `SetRemoteAuthConfig` üzerinden değişir; UI bunu gizler, aşağıda).
  - Başarıda: `account.PasswordHash = HashPassword(newPassword)` + `config.Save`.
  - **Kısıtlama (bilinen, kabul):** şifre değişimi mevcut oturumları geçersiz kılmaz (JWT 12h TTL ile kendiliğinden düşer) — `ValidateRemoteSession` doc'unun zaten belirttiği bilinen durum, değişmez.

**`internal/webserver/`:**

- `FullBridge`'e `ChangeAccountPassword(token, id, currentPassword, newPassword string) error` ve `SessionSubject` gerekmiyorsa metod imzaları eklenir; `handleAccountPassword` (POST `/api/accounts/{id}/password`):
  - Request: `{current_password, new_password}`.
  - `DELETE /api/accounts/{id}` ile aynı route ailesinde (wildcard `{id}`), method ayrımı switch'e eklenir.
  - Hata → 400 (yanlış eski şifre/eksik alan), 403 (yetki), 404 (hesap yok).
- Route kaydı: mevcut `/api/accounts/{id}` DELETE'in yanına POST eklenir (v1/alias otomatik, `server.go`'daki `route()` mekanizması halleder).

### 2. Frontend — API client (`api_client.dart`)

- `fetchSetupStatus() → (needsSetup, authMode)` — `GET /api/setup/status` (token'sız, muaf).
- `setupCreateAdmin(username, password) → sessionToken` — `POST /api/setup/create-admin`.
- `login(username, password) → (sessionToken, role)` — `loginRemote()`'u **aktifleştir** (ölü kod → kullanılır hâle gelir; yanıt `session_token` + `role` parse).
- `changeAccountPassword(id, currentPassword, newPassword)`.
- `listAccounts()` / `createAccount(username, password, role)` / `deleteAccount(id)`.
- Her başarılı login/create-admin'de: `sessionToken`'ı `MemoApiClient.token`'a ve SharedPreferences'a yaz (`memo_session_token` anahtarı, mevcut `memo_remote_access_token` deseniyle aynı) → sonraki açılışta 401 vermez.

### 3. Frontend — AuthGateOverlay (`frontend/lib/widgets/auth_gate_overlay.dart`, yeni)

AppShell stack'ine, `BackendUnreachableOverlay`'den **daha üstte** girer. İç durum kaynağı olarak yeni bir `authGateProvider` (Riverpod, `setup/status` poll'u + lokal bayrak + connection durumu birleştirir):

- **Durum `setupNeeded`** (`needs_setup=true` && lokal bayrak `memo_auth_setup_done` yok):
  - **Adım 1 — "Bu Memo'ya başka cihazlardan (telefon, web, LAN) erişilecek mi?"** — büyük toggle, varsayılan **hayır**. Altında açıklama:
    > "Memo tamamen bu cihazda çalışır; hiçbir veri dışarı çıkmaz. Kimlik doğrulama yalnızca *başka* cihazların erişimini denetlemek içindir — yalnızca burada kullanacaksan gerekmez."
  - **Hayır** → bayrak koy (`memo_auth_setup_done=true`), gate kapanır → mevcut akış (wizard) devam eder. Backend'a hiç dokunulmaz (config güvende).
  - **Evet** → **Adım 2 — giriş yöntemi seçici**, 3 kart, her birinin altında ne anlattığı metni:
    - **Sadece şifre** ⭐ ("En basit: kullanıcı adı + şifre. Token taşımaya gerek yok.")
    - **Şifre + token** ("İkisi de geçerli. Token telefon gibi cihazlara verilir.")
    - **Sadece token** ("Cihaz başına tek anahtar, şifre yok.")
  - Seçime göre form + backend çağrıları:
    - *Sadece şifre:* username + password + onay → `setupCreateAdmin` → token kaydet → bayrak → gate kapanır. Mode'u backend zaten `password`'e yükseltir.
    - *Şifre + token:* yukarıdaki form → `setupCreateAdmin` sonrası `setRemoteAuthConfig("token_password", <username>, <şifre>)` → `createRemoteDevice("Masaüstü/Web")` → **token'ı büyük, kopyalanabilir kartta göster** → bayrak → gate kapanır. (Şifrenin `SetRemoteAuthConfig`'e tekrar verilmesi zorunlu: boş bırakılırsa `PasswordHash == ""` validation'ı "password is required" der — create-admin legacy hash'i set etmez; yazılan legacy hash zararsız, login Accounts'tan gider.)
    - *Sadece token:* `setRemoteAuthConfig("token", "", "")` → `createRemoteDevice` → token kartı → bayrak → kullanıcının alttaki token alanına token'ı yapıştırması (ya da karttaki "girdim" onayı) → token `MemoApiClient.token`'a → gate kapanır.
  - `setupCreateAdmin` 403 dönerse (yarış/ikinci çağrı): bayrak koyup LoginGate'e yönlendir.
- **Durum `loginNeeded`** (`needs_setup=false` && son API çağrısı 401 döndü — ya da başlangıçta kayıtlı token yok):
  - `auth_mode`'a göre form: `password` → username+password; `token` → tek token alanı (+ "token'ı nerede bulursun" ipucu: Remote Access tab / kurulum sırasında verildi); `token_password` → **Şifre / Token sekmeleri** (el yazması webui'nin deseni).
  - Hata gösterimleri: 401 → "kullanıcı adı veya şifre hatalı"; 429 → Retry-After'a dayalı "çok fazla deneme, biraz bekleyin".
  - Başarı: token kaydet → gate kapanır.
- **Durum `ok`** → gate görünmez.
- `mode=none` veya `127.0.0.1` local host → hiçbir durum tetiklenmez (middleware zaten geçer) — masaüstü kullanıcısı hiçbir fark hissetmez (bir kez setup adımı hariç).

**`connectionStatusProvider` değişikliği:** 401 yanıtı artık "backend unreachable" değil, **`unauthorized`** durumu sayılır; `BackendUnreachableOverlay` yalnızca gerçek bağlantı hatasında görünür. `isAlive()` 401'de `false` yerine ayırt edici sonuç döner (impl: Dio hatasında `response?.statusCode == 401` kontrolü).

### 4. Frontend — Hesaplar sekmesi (`frontend/lib/widgets/settings/tabs/accounts_tab.dart`, yeni)

Settings dialog'unda ayrı sekme (simge: people). İçerik:

- Hesap listesi: kullanıcı adı + rol rozeti (`admin`/`user`) + oluşturma tarihi.
- **Yeni hesap ekle** dialog'u: username + password + rol seçimi ("user" rolı varsayılan). Backend duplicate koruması → hata ile gösterilir.
- **Şifre değiştir** dialog'u: seçili hesap için; oturum sahibi kendiyse "mevcut şifre" + "yeni şifre" alanları; admin başkasını değiştiriyorsa sadece "yeni şifre" (+ onay). Başarı SnackBar.
- Sil: onay dialog'u; son-admin koruması hatası gösterilir.
- **Oturumu kapat** butonu: lokal session token'ı siler → `authGateProvider` yeniden `loginNeeded` olur (LoginGate geri gelir; başka şifreyle girilebilir). (Backend session iptali yok — token'ı garantili devre dışı bırakmanın tek yolu hesap silmek; 12h TTL kabul edilen sınır.)
- Görünürlük: oturum rolü `user` ise yönetim bölümü gizlenir, "yalnızca yöneticiler" notu gösterilir (kendi şifresini değiştirme yine verilir).

### 5. Frontend — akış bütünlüğü

- LoginGate/SetupGate **her durumda** AppShell'i kilitlemeden önce backend'i yoklamalı — web'de `Uri.base.origin` (`backend_url.dart` mevcut fallback) zaten doğru adrese gider; değişmez.
- Token TTL (12h) düşünce 401 → otomatik LoginGate: beklenen akış.
- Web'de tarayıcı değişince SetupGate veya LoginGate kendiliğinden doğru olanı gösterir (setup/status'a göre).

## Kapsam Dışı (bilinçli)

- `mobile/` istemcisi (ayrı iş — aynı API'yi kullanır, kendi gate'ini gerektirir).
- Backend session iptali / "tüm oturumları düşür" endpoint'i.
- Token yenileme (refresh) — TTL 12h, yeniden login kabul edilir.
- Yeni rol çeşitleri (admin/user dışı).

## Test & Doğrulama

**Backend (Go):**
- `ChangeAccountPassword` birim testleri (`remote_auth_test.go` — var olan test dosyası varsa oraya): yanlış eski şifre → red; admin başkası → eski şifre gerekmez; user kendisi → eski şifre şart; user başkasına → red; bilinmeyen hesap → hata; boş yeni şifre → hata; legacy (Accounts boş) → hata.
- `SessionSubject`: tanınan/geçersiz/device token senaryoları.
- Mevcut auth testleri kırılmamalı (`go test ./internal/app/... ./internal/webserver/... -race`).

**Frontend (Flutter):**
- `auth_gate_overlay_test.dart`: durum geçişleri (setupNeeded → hayır → ok; setupNeeded → evet+şifre → ok; loginNeeded password/token/token_password formları; 401 vs bağlantı hatası ayrımı).
- `accounts_tab_test.dart`: admin görür/user görmez; ekle/sil hataları; şifre değiştir akışı.
- `api_client` ek metotları için mevcut `api_client_test.dart` deseninde mock testleri.
- `flutter analyze` temiz (mevcut 5 info hariç), `flutter test` hepsi yeşil.

**Canlı doğrulama:**
- curl smoke: `setup/status` → `create-admin` → `/api/version` token'sız 401, token'la 200 → `auth/login` yanlış şifre 401, doğru şifre `session_token` → şifre değiştirme (eski şifreyle yeni şifre arası).
- RPi web'de gerçek akış (kullanıcı cihazında): gate görünür → şifre kur → giriş → sohbet açılır. (Bu ortamda RPi'ye erişim yok — kullanıcının onayıyla canlı test.)

**L10n:** Tüm yeni UI metinleri `frontend/lib/core/l10n.dart`'a TR + EN (kural #8), tek commit'te.

## Uygulama Sırası (plan fazları)

1. **Backend:** `SessionSubject` + `ChangeAccountPassword` + handler + bridge + route + Go testleri → `go vet`, `go test -race`.
2. **API client:** setup/status, create-admin, login (aktifleştir), accounts CRUD, change-password, token persistence.
3. **AuthGateOverlay:** connectionStatus 401 ayrımı + `authGateProvider` + setup/login gate'leri + l10n.
4. **Hesaplar sekmesi:** liste/ekle/sil/şifre/oturum kapat + l10n.
5. **Doğrulama:** flutter analyze/test, curl smoke, commit'ler faz sonlarında.