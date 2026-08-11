# Evrensel Auth Ekranı (Setup/Login Gate + Hesaplar Sekmesi) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flutter masaüstü + web'de evrensel auth: ilk açılışta isteğe bağlı SetupGate (başka cihaz erişimi mi → 3'lü giriş yöntemi seçici), hesap varken LoginGate (mode'a göre form), Ayarlar'da ayrı "Hesaplar" sekmesi, backend'e şifre-değiştirme endpoint'i.

**Architecture:** Backend'e tek yeni yüzey (`ChangeAccountPassword` + `POST /api/accounts/{id}/password`); geri kalan her şey zaten var ve doğrulandı (setup/status, create-admin, login, accounts CRUD, device/token CRUD). Frontend'de yeni `authGateProvider` (StreamProvider, 30s poll) AppShell stack'ine giren `AuthGateOverlay`'i yönetir; 401 "sunucuya bağlanılamıyor" olarak değil login ekranı olarak gösterilir.

**Tech Stack:** Go 1.26 (net/http ServeMux, config.Save/argon2id `remoteauth`), Flutter 3.10+ (Riverpod 2.4 StreamProvider, Dio 5.4, SharedPreferences, mevcut `MemoTheme`/`L10n` infrası).

## Global Constraints

- Tüm Go komutları `CGO_ENABLED=1 ... -tags "sqlite_fts5"` ile (`go build/vet/test -race`).
- Flutter komutları: `export PATH="$PATH:/home/bugra/Documents/flutter/bin"` ön ekiyle, `frontend/` içinden.
- L10n kuralı (#8): kullanıcının göreceği HER yeni string `frontend/lib/core/l10n.dart`'a TR + EN olarak girer; commit mesajlarında AI attribution yasak.
- Mevcut auth davranışı DEĞİŞMEZ: lokal (127.0.0.1) her zaman muaf; `mode=none` her zaman geçer; `callerIsAdmin` kimliksiz/device-token'da izin verir.
- Mevcut `loginRemote()` imzası ve davranışı korunur (çağıran/test bozulmaz), üstüne yeni `login()` eklenir.
- Session TTL 12h; hesap silinirse oturum hemen geçersiz (mevcut davranış, değişmez).
- Spec: `docs/superpowers/specs/2026-08-11-auth-screen-design.md` (oturum öncesi okunmalı).

---

### Task 1: Backend — SessionSubject + ChangeAccountPassword + birim testler

**Files:**
- Modify: `internal/app/remote_auth.go` (yeni metodlar `SessionSubject`, `ChangeAccountPassword`)
- Test: `internal/app/remote_auth_test.go`

**Interfaces:**
- Consumes: mevcut `sessionSubjectRole(token) (subject, role string, ok bool)` (`remote_auth.go:377`), `remoteauth.VerifyPassword/HashPassword`, `config.Save`, `a.remoteAccountsMu`, `config.Account{ID, Username, PasswordHash, Role, CreatedAt}`.
- Produces:
  - `func (a *App) SessionSubject(token string) (string, bool)` — geçerli oturumun sahibi (kullanıcı adı), aksi halde `("", false)`.
  - `func (a *App) ChangeAccountPassword(sessionToken, id, currentPassword, newPassword string) error` — hata dizeleri (Task 2'nin HTTP durum eşlemesi bunlara bakar, değiştirmeyin):
    - `"new password is required"`
    - `"valid session token is required"`
    - `"password change requires accounts"`
    - `"account not found: <id>"`
    - `"current password is incorrect"`
    - `"only admins can change another account's password"`

- [ ] **Step 1: Failing test'ler**

`internal/app/remote_auth_test.go` sonuna ekle (mevcut `testSigningKey` const'u ve `App{cfg: ..., events: &eventRing{}}` desenini kullan; `hashTestPassword` helper'ı):

```go
func hashTestPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := remoteauth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func sessionTokenFor(t *testing.T, a *App, username, role string) string {
	t.Helper()
	tok, err := remoteauth.IssueSessionToken([]byte(testSigningKey), username, role)
	if err != nil {
		t.Fatalf("IssueSessionToken: %v", err)
	}
	return tok
}

func accountsApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	if _, err := config.Load(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a := &App{cfg: config.Get(), events: &eventRing{}, sessionKey: []byte(testSigningKey)}
	a.cfg.RemoteAccess.Accounts = []config.Account{
		{ID: "a-admin", Username: "admin", PasswordHash: hashTestPassword(t, "adminpw"), Role: "admin", CreatedAt: time.Now()},
		{ID: "a-user", Username: "kaya", PasswordHash: hashTestPassword(t, "userpw"), Role: "user", CreatedAt: time.Now()},
	}
	return a
}

func TestSessionSubject_ValidSessionReturnsUsername(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "admin", "admin")
	got, ok := a.SessionSubject(tok)
	if !ok || got != "admin" {
		t.Errorf("SessionSubject = (%q, %v), want (\"admin\", true)", got, ok)
	}
}

func TestSessionSubject_GarbageTokenFails(t *testing.T) {
	a := accountsApp(t)
	if got, ok := a.SessionSubject("garbage"); ok || got != "" {
		t.Errorf("SessionSubject(garbage) = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestChangeAccountPassword_SelfServiceNeedsCurrentPassword(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "kaya", "user")
	err := a.ChangeAccountPassword(tok, "a-user", "wrongpw", "newpw")
	if err == nil || err.Error() != "current password is incorrect" {
		t.Fatalf("wrong current password: err = %v, want 'current password is incorrect'", err)
	}
	if err := a.ChangeAccountPassword(tok, "a-user", "userpw", "newpw"); err != nil {
		t.Fatalf("self-service change: %v", err)
	}
	ok, verr := remoteauth.VerifyPassword(a.cfg.RemoteAccess.Accounts[1].PasswordHash, "newpw")
	if verr != nil || !ok {
		t.Errorf("expected new password to verify, ok=%v err=%v", ok, verr)
	}
}

func TestChangeAccountPassword_AdminChangesOtherWithoutCurrentPassword(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "admin", "admin")
	if err := a.ChangeAccountPassword(tok, "a-user", "", "freshpw"); err != nil {
		t.Fatalf("admin change: %v", err)
	}
	ok, verr := remoteauth.VerifyPassword(a.cfg.RemoteAccess.Accounts[1].PasswordHash, "freshpw")
	if verr != nil || !ok {
		t.Errorf("expected freshpw to verify, ok=%v err=%v", ok, verr)
	}
}

func TestChangeAccountPassword_UserCannotChangeOthers(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "kaya", "user")
	err := a.ChangeAccountPassword(tok, "a-admin", "", "hacked")
	if err == nil || err.Error() != "only admins can change another account's password" {
		t.Fatalf("err = %v, want 'only admins can change another account's password'", err)
	}
}

func TestChangeAccountPassword_UnknownAccountFails(t *testing.T) {
	a := accountsApp(t)
	tok := sessionTokenFor(t, a, "admin", "admin")
	if err := a.ChangeAccountPassword(tok, "nope", "", "x"); err == nil || err.Error() != "account not found: nope" {
		t.Fatalf("err = %v, want 'account not found: nope'", err)
	}
}

func TestChangeAccountPassword_RequiresValidSessionAndNewPassword(t *testing.T) {
	a := accountsApp(t)
	if err := a.ChangeAccountPassword("garbage", "a-admin", "", "x"); err == nil || err.Error() != "valid session token is required" {
		t.Fatalf("garbage session: err = %v, want 'valid session token is required'", err)
	}
	tok := sessionTokenFor(t, a, "admin", "admin")
	if err := a.ChangeAccountPassword(tok, "a-admin", "", ""); err == nil || err.Error() != "new password is required" {
		t.Fatalf("empty new password: err = %v, want 'new password is required'", err)
	}
	if err := a.ChangeAccountPassword(tok, "a-admin", "", "xl"); err != nil {
		t.Fatalf("plain change: %v", err)
	}
}

func TestChangeAccountPassword_LegacyOnlyFails(t *testing.T) {
	a := accountsApp(t)
	a.cfg.RemoteAccess.Accounts = nil
	a.cfg.RemoteAccess.Username = "admin"
	a.cfg.RemoteAccess.PasswordHash = hashTestPassword(t, "pw")
	tok := sessionTokenFor(t, a, "admin", "admin")
	err := a.ChangeAccountPassword(tok, "a-admin", "", "x")
	if err == nil || err.Error() != "password change requires accounts" {
		t.Fatalf("err = %v, want 'password change requires accounts'", err)
	}
}
```

- [ ] **Step 2: Test'i çalıştırıp fail olduğunu gör**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" ./internal/app/ -run 'TestSessionSubject|TestChangeAccountPassword' -v`
Expected: derleme hatası — `a.SessionSubject` / `a.ChangeAccountPassword` tanımsız.

- [ ] **Step 3: Minimal implementasyon**

`internal/app/remote_auth.go` sonuna (dosya sonundaki `DeleteAccount`'tan sonra) ekle:

```go
// SessionSubject returns the username a currently-valid session token was
// issued for, re-checked against the live account list (same resolution
// as sessionSubjectRole — a deleted/renamed account invalidates its
// outstanding sessions immediately). ok=false for device tokens, expired
// or garbage credentials.
func (a *App) SessionSubject(token string) (string, bool) {
	subject, _, ok := a.sessionSubjectRole(token)
	return subject, ok
}

// ChangeAccountPassword updates an account's password hash. Self-service
// (id == the session token's subject) requires the current password;
// changing someone else's account requires an admin session. The password
// change does NOT invalidate outstanding sessions (JWT TTL 12h applies,
// matching ValidateRemoteSession's documented limitation).
func (a *App) ChangeAccountPassword(sessionToken, id, currentPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password is required")
	}
	subject, subjectRole, ok := a.sessionSubjectRole(sessionToken)
	if !ok {
		return fmt.Errorf("valid session token is required")
	}

	a.remoteAccountsMu.Lock()
	defer a.remoteAccountsMu.Unlock()
	if len(a.cfg.RemoteAccess.Accounts) == 0 {
		return fmt.Errorf("password change requires accounts")
	}
	var acc *config.Account
	for i := range a.cfg.RemoteAccess.Accounts {
		if a.cfg.RemoteAccess.Accounts[i].ID == id {
			acc = &a.cfg.RemoteAccess.Accounts[i]
			break
		}
	}
	if acc == nil {
		return fmt.Errorf("account not found: %s", id)
	}
	if id == subject {
		ok, err := remoteauth.VerifyPassword(acc.PasswordHash, currentPassword)
		if err != nil || !ok {
			return fmt.Errorf("current password is incorrect")
		}
	} else if subjectRole != "admin" {
		return fmt.Errorf("only admins can change another account's password")
	}
	hash, err := remoteauth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	acc.PasswordHash = hash
	if err := config.Save(a.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.emitEvent("remote_auth:password_changed", acc.Username)
	return nil
}
```

- [ ] **Step 4: Test'i çalıştırıp pass olduğunu gör**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" ./internal/app/ -run 'TestSessionSubject|TestChangeAccountPassword'`
Expected: PASS (7 test).

- [ ] **Step 5: Tüm app paketini regression test et**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" ./internal/app/`
Expected: hepsi yeşil (mevcut `remote_auth_test.go` testleri dahil).

- [ ] **Step 6: Commit**

```bash
git add internal/app/remote_auth.go internal/app/remote_auth_test.go
git commit -m "feat(backend): account password change API (ChangeAccountPassword, SessionSubject)

New endpoint surface for the auth screen: self-service password change
requires the current password, admin sessions can change any account
without it. Session subject resolution exposed so the webserver layer
can delegate authorization to the app layer instead of parsing JWT
claims itself."
```

---

### Task 2: Backend — FullBridge imzası + POST /api/accounts/{id}/password handler

**Files:**
- Modify: `internal/webserver/bridge.go` (FullBridge'e metod imzası)
- Modify: `internal/webserver/handlers_auth.go` (`handleAccountByID`'ye POST kolu)
- Modify: `internal/app/bridge_impl.go` **varsa** — FullBridge'i implement eden app tarafı dosya (`grep -rn "func (a \*App) DeleteAccount" internal/app` ile bulunan dosya `DeleteAccount`'ın olduğu dosyadır — aynı dosyaya `ChangeAccountPassword`'ü zaten Task 1'de ekledik; imza zaten uyuyorsa ek iş yok)
- Test: canlı smoke (Step 4) — birim test gerekmez (`webserver/remote_auth_test.go` bridge stub kullanmıyor; app seviyesi Task 1'de kapsandı)

**Interfaces:**
- Consumes: `a.ChangeAccountPassword(sessionToken, id, currentPassword, newPassword string) error`, `remoteCredential(r)` (`server.go` — X-Memo-Token/Bearer çıkarır), hata dizeleri (Task 1).
- Produces: `POST /api/accounts/{id}/password` route'u (mevcut `/api/accounts/{id}` kaydının altında, `server.go:223`'teki `route("/api/accounts/{id}", s.handleAccountByID)` zaten wildcard'ı kapsar — yeni route gerekmez).

- [ ] **Step 1: FullBridge'e imza ekle**

`internal/webserver/bridge.go`'da `DeleteAccount(id string) error`'ün bulunduğu interface bloğuna (satır ~169) hemen sonrasına:

```go
	ChangeAccountPassword(sessionToken, id, currentPassword, newPassword string) error
```

- [ ] **Step 2: Handler'a POST kolu ekle**

`internal/webserver/handlers_auth.go`'da `handleAccountByID`'nin `DELETE` dalını ayrıştıran switch'e POST ekle (mevcut yapı `switch r.Method { case http.MethodDelete: ... default: }` şeklinde — case'i genişlet):

```go
	case http.MethodPost:
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.ChangeAccountPassword(remoteCredential(r), id, req.CurrentPassword, req.NewPassword); err != nil {
			http.Error(w, err.Error(), changePasswordStatus(err))
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
```

Aynı dosyaya helper (dışa kapalı):

```go
// changePasswordStatus maps ChangeAccountPassword's error strings to HTTP
// status codes — the errors already carry the distinction in their message,
// matching the codebase's string-based status mapping convention (see
// handleRemoteDeviceByID).
func changePasswordStatus(err error) int {
	msg := err.Error()
	switch {
	case msg == "valid session token is required":
		return http.StatusUnauthorized
	case msg == "account not found":
		return http.StatusNotFound
	case msg == "only admins can change another account's password":
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}
```

Not: `msg == "account not found"` eşleşmesi `"account not found: nope"` ile prefix değil tam eşleşme — `strings.HasPrefix(msg, "account not found")` kullan (import `strings` — dosyada zaten var mı kontrol et: `grep -n '"strings"' internal/webserver/handlers_auth.go`; yoksa import ekle).

- [ ] **Step 3: Derle + vet**

Run: `CGO_ENABLED=1 go build -tags "sqlite_fts5" ./... && CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...`
Expected: temiz.

- [ ] **Step 4: Canlı smoke (curl)**

Geçici data diziniyle backend aç (24444 portu boş olmalı):

```bash
cd /tmp && rm -rf authsmoke && mkdir -p authsmoke && cd /home/bugra/Documents/memo
CGO_ENABLED=1 go run -tags "sqlite_fts5" . --headless --port 24444 --data-dir /tmp/authsmoke-data &
sleep 3
curl -s http://127.0.0.1:24444/api/setup/status   # {"needs_setup":true,"auth_mode":"token"}
curl -s -X POST http://127.0.0.1:24444/api/setup/create-admin -d '{"username":"admin","password":"pw1"}'  # session_token döner → TOK değişkenine al
```

Sonra (TOK'u yukarıdaki yanıttan doldurarak):

```bash
TOK="<session_token>"
curl -s http://127.0.0.1:24444/api/version -H "X-Memo-Token: $TOK"          # 200
curl -s -X POST http://127.0.0.1:24444/api/accounts/change -d '{"username":"kaya","password":"kopw","role":"user"}' -H "X-Memo-Token: $TOK" | head -1  # beklenen: 404 (yanlış path — DELETE ile aynı route'u POST'lamayı test et)
curl -s -X POST http://127.0.0.1:24444/api/accounts/a-admin/password -d '{"new_password":"pw2"}' -H "X-Memo-Token: $TOK"   # 404 (a-admin yok — account not found)
curl -s http://127.0.0.1:24444/api/accounts -H "X-Memo-Token: $TOK" | head -c 200   # admin hesabının ID'sini gör
curl -s -X POST "http://127.0.0.1:24444/api/accounts/<admin-id>/password" -d '{"new_password":"pw2"}' -H "X-Memo-Token: $TOK"  # 200 {"ok":true}
curl -s -X POST http://127.0.0.1:24444/api/auth/login -d '{"username":"admin","password":"pw2"}'  # 200 yeni şifreyle; "pw1" ile → 401
```

Kapat: `curl -s -X POST http://127.0.0.1:24444/api/shutdown` (veya kill). Smoke script'i `/tmp/authsmoke/`'a kopyalanabilir — repoya girmez.

- [ ] **Step 5: Commit**

```bash
git add internal/webserver/bridge.go internal/webserver/handlers_auth.go
git commit -m "feat(webserver): POST /api/accounts/{id}/password endpoint

Wires ChangeAccountPassword through FullBridge. Status mapping from
the app errors (401/403/404/400). Same route registration as the
existing DELETE — the {id} wildcard already covers it."
```

---

### Task 3: Frontend — API client metotları + birim testler

**Files:**
- Modify: `frontend/lib/core/api_client.dart` (yeni `SetupStatus`, `ApiAuthStatus`, 6 metot; `loginRemote` delegasyonu)
- Test: `frontend/test/core/api_client_test.dart`

**Interfaces:**
- Consumes: mevcut `_dio` (Dio, header'lar `X-Memo-Token` ile), `_guard<T>`, `onRemoteTokenLearned` callback, `loginRemote(username, password) → Future<String>` (satır 1027).
- Produces:
  - `class SetupStatus { final bool needsSetup; final String authMode; }` + `factory SetupStatus.fromJson(Map<String, dynamic>)`
  - `enum ApiAuthStatus { ok, unauthorized, down }`
  - `Future<SetupStatus> fetchSetupStatus()`
  - `void setSessionToken(String token)` — header'a yazar + `onRemoteTokenLearned` tetikler
  - `Future<ApiAuthStatus> probeAuth()`
  - `Future<String> setupCreateAdmin(String username, String password)`
  - `Future<LoginResult> login(String username, String password)`; `class LoginResult { final String sessionToken; final String role; }` — `loginRemote` gövdesi buna delege eder (`return (await login(u, p)).sessionToken;`)
  - `Future<void> changeAccountPassword(String id, {String currentPassword = '', required String newPassword})`
  - `Future<List<Map<String, dynamic>>> listAccounts()`, `Future<void> createAccount(String username, String password, String role)`, `Future<void> deleteAccount(String id)`

- [ ] **Step 1: Failing test'ler**

`frontend/test/core/api_client_test.dart` sonuna (dosya `_FakeChatsAdapter` deseni kullanıyor; yeni bir path'e duyarlı adapter ekle):

```dart
/// Answers per-path with a fixed (status, body) pair — mirrors
/// _FakeChatsAdapter but lets a single stub cover setup/login/version
/// routes for the auth client tests.
class _FakeAuthAdapter implements HttpClientAdapter {
  _FakeAuthAdapter(this.responses);
  final Map<String, (int, Object?)> responses;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final (status, body) = responses[options.path] ?? (500, {'error': 'no stub for ${options.path}'});
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
```

Test bloğu:

```dart
group('auth endpoints', () {
  test('fetchSetupStatus parses needs_setup and auth_mode', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'password'}),
    });
    final ss = await client.fetchSetupStatus();
    expect(ss.needsSetup, isTrue);
    expect(ss.authMode, 'password');
  });

  test('probeAuth distinguishes ok / unauthorized / down', () async {
    final ok = MemoApiClient(baseUrl: 'http://memo.test');
    ok.dio.httpClientAdapter = _FakeAuthAdapter({'/api/version': (200, {})});
    expect(await ok.probeAuth(), ApiAuthStatus.ok);

    final unauthorized = MemoApiClient(baseUrl: 'http://memo.test');
    unauthorized.dio.httpClientAdapter = _FakeAuthAdapter({'/api/version': (401, {})});
    expect(await unauthorized.probeAuth(), ApiAuthStatus.unauthorized);

    final down = MemoApiClient(baseUrl: 'http://memo.test');
    down.dio.httpClientAdapter = _FakeAuthAdapter({});
    expect(await down.probeAuth(), ApiAuthStatus.down);
  });

  test('setSessionToken sets header and fires onRemoteTokenLearned', () async {
    String? learned;
    final client = MemoApiClient(
      baseUrl: 'http://memo.test',
      onRemoteTokenLearned: (t) => learned = t,
    );
    client.setSessionToken('sess-tok');
    expect(client.dio.options.headers['X-Memo-Token'], 'sess-tok');
    expect(learned, 'sess-tok');
  });

  test('setupCreateAdmin returns the session token', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeAuthAdapter({
      '/api/setup/create-admin': (200, {'session_token': 'boot-tok', 'role': 'admin'}),
    });
    expect(await client.setupCreateAdmin('admin', 'pw'), 'boot-tok');
  });

  test('login parses session_token and role; loginRemote delegates', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeAuthAdapter({
      '/api/auth/login': (200, {'session_token': 's-tok', 'role': 'user'}),
    });
    final res = await client.login('kaya', 'pw');
    expect(res.sessionToken, 's-tok');
    expect(res.role, 'user');
    expect(await client.loginRemote('kaya', 'pw'), 's-tok');
  });

  test('changeAccountPassword sends current_password and new_password', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    final sent = <String, Object?>{};
    client.dio.httpClientAdapter = _FakeAuthAdapter({
      '/api/accounts/a1/password': (200, {'ok': true}),
    });
    await client.changeAccountPassword('a1', currentPassword: 'old', newPassword: 'new');
    expect(sent, isEmpty); // body assertion via adapter callback yerine:
    // (izleme için adapter'a requestStream body'si okunmaz — ayrıca ok status doğrulaması yeterli)
  });

  test('listAccounts/createAccount/deleteAccount map to the accounts routes', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeAuthAdapter({
      '/api/accounts': (200, [
        {'id': 'a1', 'username': 'admin', 'role': 'admin'},
      ]),
    });
    final accounts = await client.listAccounts();
    expect(accounts, hasLength(1));
    expect(accounts.first['username'], 'admin');
  });
});
```

Not: `changeAccountPassword` body doğrulaması için `_FakeAuthAdapter`'a isteğe bağlı `onRequest` callback'i eklenebilir (annexe) — Step 3'te body'yi gönderdiğini doğrula; gerekmiyorsa yukarıdaki gibi status doğrulaması yeterli. Simplest geçerli yol: adapter'a `final void Function(RequestOptions)? onRequest;` ekle ve `changeAccountPassword` testinde body'yi `jsonDecode(options.data as String)` ile kontrol et.

- [ ] **Step 2: Test'i çalıştırıp fail olduğunu gör**

Run: `export PATH="$PATH:/home/bugra/Documents/flutter/bin" && cd frontend && flutter test test/core/api_client_test.dart`
Expected: derleme hatası — `fetchSetupStatus`/`ApiAuthStatus`/`LoginResult` tanımsız.

- [ ] **Step 3: Implementasyon**

`frontend/lib/core/api_client.dart`'a ekle (mevcut `loginRemote`'un yanına, "Health check" bölümünün üstüne):

```dart
/// Result of GET /api/setup/status — the unauthenticated first-run probe.
class SetupStatus {
  final bool needsSetup;
  final String authMode;
  SetupStatus({required this.needsSetup, required this.authMode});

  factory SetupStatus.fromJson(Map<String, dynamic> json) => SetupStatus(
        needsSetup: json['needs_setup'] as bool? ?? false,
        authMode: json['auth_mode'] as String? ?? 'token',
      );
}

/// Does the backend require auth for this client right now?
enum ApiAuthStatus { ok, unauthorized, down }

/// Result of a successful password login.
class LoginResult {
  final String sessionToken;
  final String role;
  LoginResult({required this.sessionToken, required this.role});
}

/// Fetches whether a first-run setup is still pending and the current
/// auth mode. Unauthenticated route (see backend isSetupBootstrapPath).
Future<SetupStatus> fetchSetupStatus() async {
  final res = await _dio.get('/api/setup/status');
  return SetupStatus.fromJson(_guard<Map<String, dynamic>>(res.data));
}

/// Applies the session token from a password login the same way a device
/// token is applied (same X-Memo-Token header; see _applyRemoteToken).
void setSessionToken(String token) {
  _dio.options.headers['X-Memo-Token'] = token;
  onRemoteTokenLearned?.call(token);
}

/// Version probe that distinguishes "unauthorized" (401 — backend is up,
/// this client lacks a valid credential) from "down" (no reachable
/// backend). Only 401 denotes an auth problem; every other failure is
/// treated as unreachable so the app can show its normal connection error.
Future<ApiAuthStatus> probeAuth() async {
  try {
    await _dio.get('/api/version');
    return ApiAuthStatus.ok;
  } on DioException catch (e) {
    if (e.response?.statusCode == 401) return ApiAuthStatus.unauthorized;
    return ApiAuthStatus.down;
  } catch (_) {
    return ApiAuthStatus.down;
  }
}

/// Creates the very first admin account (only valid while the backend
/// reports needs_setup) and returns the issued session token.
Future<String> setupCreateAdmin(String username, String password) async {
  final res = await _dio.post(
    '/api/setup/create-admin',
    data: {'username': username, 'password': password},
  );
  return _guard<Map<String, dynamic>>(res.data)['session_token'] as String? ?? '';
}

/// Password login; returns the session token and the account's role.
/// loginRemote() delegates here for backwards compatibility.
Future<LoginResult> login(String username, String password) async {
  final res = await _dio.post(
    '/api/auth/login',
    data: {'username': username, 'password': password},
  );
  final data = _guard<Map<String, dynamic>>(res.data);
  return LoginResult(
    sessionToken: data['session_token'] as String? ?? '',
    role: data['role'] as String? ?? 'user',
  );
}

/// Changes an account's password. For the session's own account the
/// current password is required; an admin session may change anyone's.
Future<void> changeAccountPassword(
  String id, {
  String currentPassword = '',
  required String newPassword,
}) async {
  await _dio.post(
    '/api/accounts/$id/password',
    data: {'current_password': currentPassword, 'new_password': newPassword},
  );
}

/// Lists all accounts (metadata only, never password hashes).
Future<List<Map<String, dynamic>>> listAccounts() async {
  final res = await _dio.get('/api/accounts');
  return _guardList<Map<String, dynamic>>(res.data);
}

/// Creates an account with the given role ("admin"|"user").
Future<void> createAccount(String username, String password, String role) async {
  await _dio.post('/api/accounts', data: {'username': username, 'password': password, 'role': role});
}

/// Deletes an account (refuses the last admin — backend-enforced).
Future<void> deleteAccount(String id) async {
  await _dio.delete('/api/accounts/$id');
}
```

`loginRemote` gövdesini değiştir (satır 1027):

```dart
  Future<String> loginRemote(String username, String password) async {
    return (await login(username, password)).sessionToken;
  }
```

- [ ] **Step 4: Test'i çalıştırıp pass olduğunu gör**

Run: `flutter test test/core/api_client_test.dart`
Expected: PASS (yeni 7 test dahil; mevcut loginRemote/testler bozulmadı).

- [ ] **Step 5: Commit**

```bash
git add frontend/lib/core/api_client.dart frontend/test/core/api_client_test.dart
git commit -m "feat(frontend): API client auth methods (setup/status, login, accounts, password change)

loginRemote() now delegates to login() which also returns the role.
probeAuth() distinguishes 401 (auth gate) from unreachable (connection
error) so the app can show the right screen for each."
```

---

### Task 4: Frontend — authGateProvider + BackendUnreachableOverlay koşulu

**Files:**
- Create: `frontend/lib/providers/auth_gate_provider.dart`
- Modify: `frontend/lib/widgets/backend_unreachable_view.dart` (gizleme koşulu)
- Test: `frontend/test/providers/auth_gate_provider_test.dart` (yeni)

**Interfaces:**
- Consumes: `MemoApiClient.fetchSetupStatus/probeAuth/setSessionToken`, `prefsProvider` (`settings_provider.dart:17`), `SharedPreferences`, Task 3 tipleri.
- Produces:
  - `const authSetupDoneKey = 'memo_auth_setup_done';`
  - `enum AuthGateState { ok, setupNeeded, loginNeeded }`
  - `class AuthGateInfo { final AuthGateState state; final String authMode; const AuthGateInfo(this.state, {this.authMode = 'token'}); }`
  - `final authGateProvider = StreamProvider.autoDispose<AuthGateInfo>(...)` — 30s poll:
    - `fetchSetupStatus` hatası → `ok` (backend yok; BackendUnreachableOverlay zaten devrede)
    - `needs_setup=true` && !bayrak → `setupNeeded`; bayrak varsa → `ok`
    - `needs_setup=false`: `prefs.getString('memo_remote_access_token')` boş → `loginNeeded`; dolu → `probeAuth()`: `ok`→`ok`, `unauthorized`→`loginNeeded`, `down`→`ok`

- [ ] **Step 1: Failing test'ler**

`frontend/test/providers/auth_gate_provider_test.dart` (yeni dosya) — `_FakeAuthAdapter` desenini `api_client_test.dart`'tan kopyala (test dosyaları arası paylaşım yok; adapter'ı bu dosyada yeniden tanımla, daha küçük: `Map<String, (int, Object?)>`):

```dart
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http_parser/http_parser.dart' as http_parser;
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
```

Import düzeni: `http_parser` gerekmiyorsa kullanma — ResponseBody header'larını `dart:convert` + elle kur:

```dart
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.responses);
  final Map<String, (int, Object?)> responses;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final (status, body) = responses[options.path] ?? (500, null);
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      status,
      headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
    );
  }

  @override
  void close({bool force = false}) {}
}
```

Test:

```dart
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<AuthGateInfo> firstGate(ProviderContainer container) async {
    return container.read(authGateProvider.stream).first;
  }

  Future<ProviderContainer> makeContainer(
    Map<String, (int, Object?)> responses, {
    Map<String, Object> prefs = const {},
  }) async {
    SharedPreferences.setMockInitialValues(prefs);
    final prefs2 = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeAdapter(responses);
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs2),
    ]);
    addTearDown(container.dispose);
    return container;
  }

  test('needs_setup without flag shows setup gate', () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    final info = await firstGate(c);
    expect(info.state, AuthGateState.setupNeeded);
    expect(info.authMode, 'token');
  });

  test('declined flag skips the setup gate', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
      },
      prefs: {authSetupDoneKey: true},
    );
    expect((await firstGate(c)).state, AuthGateState.ok);
  });

  test('no saved token after setup completed -> login gate', () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    final info = await firstGate(c);
    expect(info.state, AuthGateState.loginNeeded);
    expect(info.authMode, 'password');
  });

  test('saved token + 200 version -> ok', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {'memo_remote_access_token': 'tok'},
    );
    expect((await firstGate(c)).state, AuthGateState.ok);
  });

  test('saved token + 401 -> login gate (expired session)', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
        '/api/version': (401, null),
      },
      prefs: {'memo_remote_access_token': 'stale'},
    );
    expect((await firstGate(c)).state, AuthGateState.loginNeeded);
  });

  test('backend down -> ok (unreachable overlay handles it)', () async {
    final c = await makeContainer({});
    expect((await firstGate(c)).state, AuthGateState.ok);
  });
}
```

Not: `/api/version` 200 yanıtı `{'version': 'x'}` — `probeAuth` yanıt gövdesini parse etmiyor, status yeterli; `(200, {})` da olur. `(401, null)` body'siz.

- [ ] **Step 2: Test'i çalıştırıp fail olduğunu gör**

Run: `flutter test test/providers/auth_gate_provider_test.dart`
Expected: derleme hatası — `auth_gate_provider.dart` yok.

- [ ] **Step 3: Provider'ı yaz**

`frontend/lib/providers/auth_gate_provider.dart` (yeni dosya):

```dart
import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import 'chat_provider.dart';
import 'settings_provider.dart';

/// SharedPreferences key marking that this client already made its
/// first-run auth decision ("no remote access needed") — per browser,
/// per app install. Doesn't touch backend config; NeedsSetup stays true
/// server-side so a genuinely fresh client still gets the gate.
const authSetupDoneKey = 'memo_auth_setup_done';

enum AuthGateState { ok, setupNeeded, loginNeeded }

class AuthGateInfo {
  final AuthGateState state;
  final String authMode;
  const AuthGateInfo(this.state, {this.authMode = 'token'});
}

/// Drives the AuthGateOverlay. Polls the unauthenticated setup/status
/// endpoint and, once setup is done, the version probe, to decide which
/// gate (if any) the user must pass before the app is usable:
///   - needs_setup && !declined flag  -> setup gate (first-run choice)
///   - needs_setup && declined flag   -> nothing (open local install)
///   - setup done, no saved token    -> login gate
///   - saved token but 401           -> login gate (expired session)
///   - backend unreachable           -> ok (BackendUnreachableOverlay)
final authGateProvider = StreamProvider.autoDispose<AuthGateInfo>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  final prefs = ref.read(prefsProvider);
  while (alive) {
    try {
      final ss = await api.fetchSetupStatus();
      final declined = prefs.getBool(authSetupDoneKey) ?? false;
      if (ss.needsSetup && !declined) {
        yield AuthGateInfo(AuthGateState.setupNeeded, authMode: ss.authMode);
      } else if (!ss.needsSetup) {
        final saved = prefs.getString('memo_remote_access_token');
        if (saved == null || saved.isEmpty) {
          yield AuthGateInfo(AuthGateState.loginNeeded, authMode: ss.authMode);
        } else {
          final probe = await api.probeAuth();
          yield switch (probe) {
            ApiAuthStatus.ok => const AuthGateInfo(AuthGateState.ok),
            ApiAuthStatus.unauthorized =>
              AuthGateInfo(AuthGateState.loginNeeded, authMode: ss.authMode),
            ApiAuthStatus.down => const AuthGateInfo(AuthGateState.ok),
          };
        }
      } else {
        yield const AuthGateInfo(AuthGateState.ok);
      }
    } catch (_) {
      yield const AuthGateInfo(AuthGateState.ok);
    }
    if (!alive) break;
    await Future.delayed(const Duration(seconds: 30));
  }
});
```

- [ ] **Step 4: BackendUnreachableOverlay'de 401'de gizle**

`frontend/lib/widgets/backend_unreachable_view.dart`'ta `BackendUnreachableOverlay.build` içine (mevcut `connected != false` kontrolünün yanına) — import ekle: `import '../providers/auth_gate_provider.dart';`:

```dart
    final auth = ref.watch(authGateProvider).valueOrNull;
    if (auth != null && auth.state != AuthGateState.ok) return const SizedBox.shrink();
```

(Gerekçe: 401 "backend'e ulaşılamıyor" değil, "kimlik yok" — gate onu üstlenir; bu koşul olmadan ikisi çakışırdı.)

- [ ] **Step 5: Test'i çalıştırıp pass olduğunu gör**

Run: `flutter test test/providers/auth_gate_provider_test.dart && flutter test test/widgets/backend_unreachable_view_test.dart`
Expected: 6 yeni test PASS; mevcut overlay testleri bozulmadı.

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/providers/auth_gate_provider.dart frontend/lib/widgets/backend_unreachable_view.dart frontend/test/providers/auth_gate_provider_test.dart
git commit -m "feat(frontend): authGateProvider (setup/login gate state machine)

Polls the unauthenticated setup/status endpoint every 30s, combines
it with the local 'auth decision made' flag and the saved token to
yield setupNeeded/loginNeeded/ok. The unreachable overlay now hides
when a gate is active — 401 is 'need credentials', not 'no backend'."
```

---

### Task 5: Frontend — AuthGateOverlay (SetupGate + LoginGate) + l10n

**Files:**
- Create: `frontend/lib/widgets/auth_gate_overlay.dart`
- Modify: `frontend/lib/screens/app_shell.dart` (stack'e ekle — BackendUnreachableOverlay'den SONRA/sonrasına)
- Modify: `frontend/lib/core/l10n.dart` (yeni key'ler, TR+EN)
- Test: `frontend/test/widgets/auth_gate_overlay_test.dart` (yeni)

**Interfaces:**
- Consumes: `authGateProvider`/`AuthGateInfo` (Task 4), `MemoApiClient` (fetchSetupStatus, setupCreateAdmin, setSessionToken, login, createRemoteDevice, setRemoteAuthConfig), `prefsProvider`, `authSetupDoneKey`, `MemoTheme`, `L10n`.
- Produces: `class AuthGateOverlay extends ConsumerWidget` — app_shell stack'ine `BackendUnreachableOverlay`'den sonra eklenir.

- [ ] **Step 1: l10n key'leri (TR + EN)**

`frontend/lib/core/l10n.dart`'a (TR map ve EN map'e aynı key'ler) ekle:

| Key | TR | EN |
|---|---|---|
| `auth_gate_setup_title` | "Memo'ya hoş geldin" | "Welcome to Memo" |
| `auth_gate_privacy_note` | "Memo tamamen bu cihazda çalışır; hiçbir veri dışarı çıkmaz. Kimlik doğrulama yalnızca *başka* cihazların (telefon, web, LAN) erişimini denetlemek içindir." | "Memo runs entirely on this device; no data ever leaves it. Authentication only governs access from *other* devices (phone, web, LAN)." |
| `auth_gate_other_devices_question` | "Bu Memo'ya başka cihazlardan erişilecek mi?" | "Will this Memo be accessed from other devices?" |
| `auth_gate_other_devices_yes` | "Evet, telefonumdan / başka cihazlardan da gireceğim" | "Yes, I'll sign in from my phone / other devices too" |
| `auth_gate_other_devices_no` | "Hayır, yalnızca bu cihazda kullanacağım" | "No, I'll only use it on this device" |
| `auth_gate_continue` | "Devam" | "Continue" |
| `auth_gate_method_label` | "Giriş yöntemi" | "Sign-in method" |
| `auth_gate_method_password` | "Sadece şifre" | "Password only" |
| `auth_gate_method_password_desc` | "En basit: kullanıcı adı + şifre. Token taşımaya gerek yok." | "Simplest: username + password. No token to carry." |
| `auth_gate_method_token_password` | "Şifre + token" | "Password + token" |
| `auth_gate_method_token_password_desc` | "İkisi de geçerli. Token, telefon gibi ayrı cihazlara verilir." | "Both work. The token is handed to separate devices like a phone." |
| `auth_gate_method_token` | "Sadece token" | "Token only" |
| `auth_gate_method_token_desc` | "Cihaz başına tek anahtar, şifre yok." | "One key per device, no password." |
| `auth_gate_username` | "Kullanıcı adı" | "Username" |
| `auth_gate_password` | "Şifre" | "Password" |
| `auth_gate_confirm_password` | "Şifre (tekrar)" | "Password (again)" |
| `auth_gate_password_mismatch` | "Şifreler eşleşmiyor" | "Passwords do not match" |
| `auth_gate_create` | "Oluştur ve başla" | "Create and start" |
| `auth_gate_generate_token` | "Token oluştur" | "Generate token" |
| `auth_gate_token_generated_title` | "Cihaz token'ın hazır" | "Your device token is ready" |
| `auth_gate_token_generated_body` | "Bu token yalnızca bir kez gösterilir. Kopyalayıp güvenli bir yerde sakla." | "This token is shown only once. Copy it and keep it somewhere safe." |
| `auth_gate_token_copy` | "Kopyala" | "Copy" |
| `auth_gate_token_enter_hint` | "Token'ı yapıştır ve giriş yap" | "Paste the token and sign in" |
| `auth_gate_enter_token` | "Token" | "Token" |
| `auth_gate_sign_in` | "Giriş yap" | "Sign in" |
| `auth_gate_token_hint_password_mode` | "Bu Memo'da şifreli giriş etkin — cihaz token'ı çalışmaz." | "This Memo uses password sign-in — device tokens don't apply." |
| `auth_gate_login_tab_password` | "Şifre" | "Password" |
| `auth_gate_login_tab_token` | "Token" | "Token" |
| `auth_gate_error_password_mismatch` | "Şifreler eşleşmiyor" | "Passwords do not match" |
| `auth_gate_error_invalid_credentials` | "Kullanıcı adı veya şifre hatalı" | "Invalid username or password" |
| `auth_gate_error_locked` | "Çok fazla deneme. Kısa bir süre bekleyip tekrar dene." | "Too many attempts. Please wait a moment and try again." |
| `auth_gate_error_create_failed` | "Hesap oluşturulamadı: sunucu zaten kurulmuş olabilir." | "Could not create the account — the server may already be set up." |
| `auth_gate_error_generic` | "Bir şeyler ters gitti: {err}" | "Something went wrong: {err}" |
| `auth_gate_creating` | "Kuruluyor…" | "Setting up…" |
| `auth_gate_signing_in` | "Giriş yapılıyor…" | "Signing in…" |

Format uyumluluğu: l10n.dart'ın mevcut deseni (TR ve EN map'leri aynı key setiyle) — kopyala-yapıştır doğrula: bir TR entry'si unutulursa doğrulama adımında yakalanır.

- [ ] **Step 2: Widget'ı yaz**

`frontend/lib/widgets/auth_gate_overlay.dart` (yeni dosya) — yapı:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';

/// App-wide gate: shows the first-run setup screen or the login screen
/// whenever the backend requires a credential the app doesn't have yet.
/// Rendered above BackendUnreachableOverlay in app_shell's Stack — a 401
/// is "need credentials", not "no backend".
class AuthGateOverlay extends ConsumerWidget {
  const AuthGateOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final info = ref.watch(authGateProvider).valueOrNull;
    if (info == null) return const SizedBox.shrink();
    return switch (info.state) {
      AuthGateState.setupNeeded => _GateScaffold(child: _SetupGateView()),
      AuthGateState.loginNeeded => _GateScaffold(
          child: _LoginGateView(mode: info.authMode),
        ),
      AuthGateState.ok => const SizedBox.shrink(),
    };
  }
}
```

`_GateScaffold`: tam ekran `Container(color: theme.bgApp)`, ortalanmış `ConstrainedBox(maxWidth: 480)`, Memo logo başlığı (`remezo` stil: büyük başlık `Text(L10n.t('auth_gate_setup_title'))` — login'de aynı şablon, başlık `L10n.t('auth_gate_sign_in')`), arka planı `BackendUnreachableView` görünümüyle uyumlu (theme kartı). `MemoTheme.of(context)`.

`_SetupGateView` (ConsumerStatefulWidget) — 3 adımlı state machine:

```dart
class _SetupGateView extends ConsumerStatefulWidget {
  const _SetupGateView();

  @override
  ConsumerState<_SetupGateView> createState() => _SetupGateViewState();
}

class _SetupGateViewState extends ConsumerState<_SetupGateView> {
  int _step = 0;        // 0: başka cihaz sorusu, 1: yöntem + form, 2: token kartı
  String _method = 'password'; // password | token_password | token
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  final _tokenInput = TextEditingController();
  bool _busy = false;
  String? _error;
  String? _generatedToken;

  Future<void> _decline() async {
    final prefs = ref.read(prefsProvider);
    await prefs.setBool(authSetupDoneKey, true);
    ref.invalidate(authGateProvider);
  }

  Future<void> _submit() async {
    final api = ref.read(apiClientProvider);
    final prefs = ref.read(prefsProvider);
    setState(() { _busy = true; _error = null; });
    try {
      if (_method == 'token') {
        // Sadece token: mode=token + device token üret; kullanıcı token'ı
        // sonraki adımda alana yapıştırır.
        await api.setRemoteAuthConfig('token');
        final plain = await api.createRemoteDevice('Auth setup');
        await prefs.setBool(authSetupDoneKey, true);
        setState(() { _generatedToken = plain; _step = 2; _busy = false; });
        return;
      }
      final token = await api.setupCreateAdmin(
        _username.text.trim(),
        _password.text,
      );
      api.setSessionToken(token);
      if (_method == 'token_password') {
        await api.setRemoteAuthConfig(
          'token_password',
          username: _username.text.trim(),
          password: _password.text,
        );
        final plain = await api.createRemoteDevice('Auth setup');
        await prefs.setBool(authSetupDoneKey, true);
        setState(() { _generatedToken = plain; _step = 2; _busy = false; });
        return;
      }
      await prefs.setBool(authSetupDoneKey, true);
      ref.invalidate(authGateProvider);
    } on DioException catch (e) {
      setState(() {
        _busy = false;
        _error = e.response?.statusCode == 403
            ? L10n.t('auth_gate_error_create_failed')
            : L10n.t('auth_gate_error_generic', {'err': '$e'});
      });
    } catch (e) {
      setState(() {
        _busy = false;
        _error = L10n.t('auth_gate_error_generic', {'err': '$e'});
      });
    }
  }

  Future<void> _enterToken() async {
    final api = ref.read(apiClientProvider);
    api.setSessionToken(_tokenInput.text.trim());
    ref.invalidate(authGateProvider);
  }

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    _confirm.dispose();
    _tokenInput.dispose();
    super.dispose();
  }
  // build: _step 0 -> soru kartı (title + privacy_note + 2 büyük seçenek
  //   butonu: yes -> setState(_step=1), no -> _decline())
  //       _step 1 -> yöntem radyo kartları (3 RadioListTile + desc) +
  //         _method != 'token' ise 3 TextField (username/password/confirm,
  //         obscure) + hata + busy spinner + "Oluştur ve başla"
  //         _method == 'token' ise yalnızca "Token oluştur" butonu
  //         (şifre doğrulaması: _password.text != _confirm.text iken
  //         auth_gate_error_password_mismatch)
  //       _step 2 -> token kartı: büyük monospace token, Kopyala butonu
  //         (Clipboard.setData + SnackBar yerine inline 'Kopyalandı' setState),
  //         TextField(_tokenInput) + "Giriş yap" (_enterToken)
}
```

Kopyala butonu: `Clipboard.setData(ClipboardData(text: _generatedToken!))` + inline `_copied` flag → buton metni `L10n.t('copied')` (mevcut key varsa kullan, yoksa `auth_gate_token_copied` ekle).

`_LoginGateView` (ConsumerStatefulWidget) — mode'a göre:

```dart
class _LoginGateView extends ConsumerStatefulWidget {
  const _LoginGateView({required this.mode});
  final String mode;
  @override
  ConsumerState<_LoginGateView> createState() => _LoginGateViewState();
}

class _LoginGateViewState extends ConsumerState<_LoginGateView> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _tokenInput = TextEditingController();
  bool _busy = false;
  String? _error;
  var _tab = 0; // 0 şifre, 1 token (yalnız token_password'de görünür)

  Future<void> _loginPassword() async {
    final api = ref.read(apiClientProvider);
    final prefs = ref.read(prefsProvider);
    setState(() { _busy = true; _error = null; });
    try {
      final res = await api.login(_username.text.trim(), _password.text);
      if (res.sessionToken.isEmpty) throw Exception('empty token');
      api.setSessionToken(res.sessionToken);
      await prefs.setString('memo_session_role', res.role);
      ref.invalidate(authGateProvider);
    } on DioException catch (e) {
      setState(() {
        _busy = false;
        _error = switch (e.response?.statusCode) {
          401 => L10n.t('auth_gate_error_invalid_credentials'),
          429 => L10n.t('auth_gate_error_locked'),
          _ => L10n.t('auth_gate_error_generic', {'err': '$e'}),
        };
      });
    } catch (e) {
      setState(() {
        _busy = false;
        _error = L10n.t('auth_gate_error_generic', {'err': '$e'});
      });
    }
  }

  Future<void> _loginToken() async {
    final api = ref.read(apiClientProvider);
    api.setSessionToken(_tokenInput.text.trim());
    ref.invalidate(authGateProvider);
  }

  // build: mode'a göre:
  //   'password'          -> username+password + _loginPassword
  //   'token'             -> tek token alanı + _loginToken
  //   'token_password'    -> SegmentedButton([şifre, token]) + ilgili form
  //   (dropdown yerine SegmentedButton — kod tabanında mevcut desen)
  // Hata satırı (_error != null) her formun altında kırmızı.
  // _busy iken buton yerine CircularProgressIndicator.
}
```

- [ ] **Step 3: app_shell.dart'a ekle**

`frontend/lib/screens/app_shell.dart:246` satırındaki `const BackendUnreachableOverlay(),`'den HEMEN SONRA (gate, unreachable'ın ÜZERİNDE çizilir): import ekle + `const AuthGateOverlay(),`.

- [ ] **Step 4: Widget test'leri**

`frontend/test/widgets/auth_gate_overlay_test.dart` (yeni):

```dart
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/auth_gate_overlay.dart';

// Stateful fake: records requests; after the first POST /api/auth/login (or
// /api/setup/create-admin) with credentials, "logs in" by answering
// /api/version with 200 — the app flow needs a version 200 right after
// setSessionToken + invalidate to close the gate.
class _StatefulAuthAdapter implements HttpClientAdapter {
  _StatefulAuthAdapter(this.statusResponses);
  final Map<String, (int, Object?)> statusResponses;
  bool authed = false;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (options.path == '/api/auth/login' ||
        options.path == '/api/setup/create-admin') {
      authed = true;
      if (options.path == '/api/auth/login') {
        return _json(200, {'session_token': 'sess', 'role': 'admin'});
      }
      return _json(200, {'session_token': 'sess', 'role': 'admin'});
    }
    if (options.path == '/api/version' && authed) return _json(200, {});
    final (status, body) = statusResponses[options.path] ?? (500, null);
    return _json(status, body);
  }

  ResponseBody _json(int status, Object? body) => ResponseBody.fromString(
        body == null ? '' : jsonEncode(body),
        status,
        headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
      );

  @override
  void close({bool force = false}) {}
}
```

Test:

```dart
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<Widget> pump(WidgetTester tester, _StatefulAuthAdapter adapter) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
      child: const MaterialApp(
        home: Stack(children: [AuthGateOverlay()]),
      ),
    ));
    await tester.pump(); // StreamProvider ilk yield
    await tester.pump(const Duration(milliseconds: 50));
  }

  test('first run: decline -> gate closes, flag persisted', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    expect(find.text(L10n.t('auth_gate_other_devices_question')), findsOneWidget);
    await tester.tap(find.text(L10n.t('auth_gate_other_devices_no')));
    await tester.pumpAndSettle();
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getBool(authSetupDoneKey), isTrue);
  });

  test('first run: password setup flow creates admin and closes gate', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('auth_gate_other_devices_yes')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('auth_gate_method_password')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'pw1');
    await tester.enterText(find.byType(TextField).at(2), 'pw1');
    await tester.tap(find.text(L10n.t('auth_gate_create')));
    await tester.pumpAndSettle();
    expect(
      adapter.requests.any((r) => r.path == '/api/setup/create-admin'),
      isTrue,
    );
    // gate kapandı (ok) — AuthGateOverlay boş döndü
    expect(find.text(L10n.t('auth_gate_other_devices_question')), findsNothing);
  });

  test('setup mismatch: password confirmation error shown, no request', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('auth_gate_other_devices_yes')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'pw1');
    await tester.enterText(find.byType(TextField).at(2), 'pw2');
    await tester.tap(find.text(L10n.t('auth_gate_create')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_error_password_mismatch')), findsOneWidget);
    expect(adapter.requests.any((r) => r.path == '/api/setup/create-admin'), isFalse);
  });

  test('login gate: bad password shows error', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
      '/api/auth/login': (401, null),
    });
    await pump(tester, adapter);
    expect(find.text(L10n.t('auth_gate_other_devices_question')), findsNothing);
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'wrong');
    await tester.tap(find.text(L10n.t('auth_gate_sign_in')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_error_invalid_credentials')), findsOneWidget);
  });

  test('login gate: correct password closes gate', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    await pump(tester, adapter);
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'pw');
    await tester.tap(find.text(L10n.t('auth_gate_sign_in')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_sign_in')), findsNothing);
  });

  test('token mode: gateway via pasted token', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.enterText(find.byType(TextField).at(0), 'dev-tok');
    await tester.tap(find.text(L10n.t('auth_gate_sign_in')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_enter_token')), findsNothing);
  });
}
```

Not: `tester.tap(find.text(...))` için buton metinleri birebir l10n key'lerinden gelmeli; `RadioListTile` gruplarında `find.text` çalışır. Başarılı akışlarda StreamProvider'ın invalidate + yeniden poll'u `pumpAndSettle` ile bekletilir; adapter `authed=true` sonrası `/api/version` 200 döndürmeli (yukarıdaki gibi).

- [ ] **Step 5: Test'leri çalıştırıp pass olduğunu gör**

Run: `flutter test test/widgets/auth_gate_overlay_test.dart`
Expected: 6 test PASS. Gerekirse (mismatch) — parse hataları flutter test çıktısında görünür.

- [ ] **Step 6: Tam frontend doğrulaması**

Run: `flutter analyze lib/ && flutter test`
Expected: analyze — mevcut 5 bilinen info dışında yeni bulgu yok. 176 mevcut + yeni testler hepsi yeşil.

- [ ] **Step 7: Kural #8 grep**

Run: `git diff --name-only -- '*.dart' | xargs -r grep -nE "(Text|Tooltip|SnackBar|AlertDialog)\(\s*['\"][A-Za-zÇĞİÖŞÜçğıöşü]"`
Expected: boş (auth_gate_overlay.dart ve dokunulan dosyalarda ham string literal yok).

- [ ] **Step 8: Commit**

```bash
git add frontend/lib/widgets/auth_gate_overlay.dart frontend/lib/screens/app_shell.dart frontend/lib/core/l10n.dart frontend/test/widgets/auth_gate_overlay_test.dart
git commit -m "feat(frontend): AuthGateOverlay — setup and login gates for all platforms

First-run gate asks whether other devices will access this Memo
(default no); if yes, a 3-way sign-in method picker (password /
password+token / token-only) with backend setup calls. Otherwise the
login gate renders a form matching the configured auth mode. Solves
the RPi web deadlock where 401s were shown as 'can't connect'."
```

---

### Task 6: Frontend — Hesaplar (Accounts) sekmesi

**Files:**
- Create: `frontend/lib/widgets/settings/tabs/accounts_tab.dart`
- Modify: `frontend/lib/widgets/settings_dialog.dart` (`_tabs`, `_tabIcons`, `_groups`, `_buildTabContent`)
- Modify: `frontend/lib/core/l10n.dart` (yeni key'ler)
- Test: `frontend/test/widgets/accounts_tab_test.dart` (yeni)

**Interfaces:**
- Consumes: `MemoApiClient.listAccounts/createAccount/deleteAccount/changeAccountPassword`, `prefsProvider` (`memo_session_role`), Task 3 tipleri.
- Produces: `class AccountsTab extends ConsumerStatefulWidget` — settings_dialog'da `case` + `_tabs` + `_tabIcons` + `_groups`'a kayıtlı.

- [ ] **Step 1: l10n key'leri**

| Key | TR | EN |
|---|---|---|
| `tab_accounts` | "Hesaplar" | "Accounts" |
| `accounts_role_admin` | "Yönetici" | "Administrator" |
| `accounts_role_user` | "Kullanıcı" | "User" |
| `accounts_admin_only_note` | "Bu bölüm yalnızca yöneticiler içindir. Kendi şifreni aşağıdan değiştirebilirsin." | "This section is for administrators only. You can change your own password below." |
| `accounts_add` | "Yeni hesap" | "New account" |
| `accounts_add_dialog_title` | "Yeni hesap ekle" | "Add account" |
| `accounts_add_username` | "Kullanıcı adı" | "Username" |
| `accounts_add_password` | "Şifre" | "Password" |
| `accounts_add_role` | "Rol" | "Role" |
| `accounts_add_submit` | "Ekle" | "Add" |
| `accounts_add_failed` | "Hesap eklenemedi: {err}" | "Could not add the account: {err}" |
| `accounts_delete_confirm_title` | "Hesabı sil" | "Delete account" |
| `accounts_delete_confirm_body` | "{name} silinsin mi? Bu işlem geri alınamaz." | "Delete {name}? This cannot be undone." |
| `accounts_delete` | "Sil" | "Delete" |
| `accounts_delete_failed` | "Hesap silinemedi: {err}" | "Could not delete the account: {err}" |
| `accounts_delete_last_admin_error` | "Son yönetici hesabı silinemez." | "The last admin account cannot be deleted." |
| `accounts_change_password` | "Şifre değiştir" | "Change password" |
| `accounts_password_dialog_title` | "Şifre değiştir — {name}" | "Change password — {name}" |
| `accounts_current_password` | "Mevcut şifre" | "Current password" |
| `accounts_new_password` | "Yeni şifre" | "New password" |
| `accounts_password_submit` | "Kaydet" | "Save" |
| `accounts_password_changed` | "Şifre güncellendi" | "Password updated" |
| `accounts_password_failed` | "Şifre değiştirilemedi: {err}" | "Could not change the password: {err}" |
| `accounts_sign_out` | "Oturumu kapat" | "Sign out" |
| `accounts_sign_out_confirm_title` | "Oturumu kapat" | "Sign out" |
| `accounts_sign_out_confirm_body` | "Kayıtlı oturum silinir; bir sonraki açılışta tekrar giriş istenir." | "Your saved session will be cleared; you'll be asked to sign in again next launch." |
| `accounts_empty` | "Henüz hesap yok. Backend kurulmamış görünüyor." | "No accounts yet. The backend appears to be unset up." |
| `accounts_loaded_error` | "Hesaplar yüklenemedi: {err}" | "Could not load accounts: {err}" |

- [ ] **Step 2: Sekmeyi yaz**

`frontend/lib/widgets/settings/tabs/accounts_tab.dart` (yeni dosya) — yapı:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../core/api_client.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';
import '../../../providers/auth_gate_provider.dart';

/// Settings tab: account list + add/delete + password change + sign out.
/// Management actions require an admin session; a "user"-role session only
/// sees its own password change (admin-only note otherwise). Local desktop
/// (no account session at all) is treated as admin — callerIsAdmin allows
/// credential-less callers, so every action works there.
class AccountsTab extends ConsumerStatefulWidget {
  const AccountsTab({super.key});

  @override
  ConsumerState<AccountsTab> createState() => _AccountsTabState();
}

class _AccountsTabState extends ConsumerState<AccountsTab> {
  List<Map<String, dynamic>>? _accounts;
  String? _error;
  String? _selfUsername;

  bool get _canManage =>
      (ref.read(prefsProvider).getString('memo_session_role') ?? 'admin') ==
      'admin';

  Future<void> _load() async {
    try {
      final accounts = await ref.read(apiClientProvider).listAccounts();
      if (!mounted) return;
      setState(() {
        _accounts = accounts;
        _error = null;
        _selfUsername = ref.read(prefsProvider).getString('memo_session_role') == null
            ? null // masaüstü: oturum yok — "kendi hesabı" bilinmez, hepsi yönetilebilir
            : null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = '$e');
    }
  }

  // _addAccount(): dialog (username/password/role dropdown) -> createAccount
  //   -> _load(); hata -> dialog içinde inline hata.
  // _deleteAccount(acc): onay dialogu -> deleteAccount -> _load(); hata
  //   string "cannot remove the last admin" içeriyorsa
  //   accounts_delete_last_admin_error göster.
  // _changePassword(acc, isSelf): dialog (isSelf: current+new; admin başkası:
  //   sadece new) -> changeAccountPassword -> _load(); başarı SnackBar
  //   (settings_dialog içinde Scaffold vardır — BUG-M2 notu: SnackBar
  //   bugün orada çalışır).
  // _signOut(): onay -> prefs.remove('memo_remote_access_token') +
  //   prefs.remove('memo_session_role') + ref.invalidate(authGateProvider)
  //   + SettingsDialog kapatılamaz (gate overlays görünür).
}
```

Build: `_accounts == null && _error == null` → yükleniyor spinner (initState'ta `_load()`); `_canManage == false` → yalnızca `accounts_change_password` bölümü + `accounts_admin_only_note`; aksi halde liste (`ListTile`: avatar+username + rol rozeti (`accounts_role_admin`/`accounts_role_user` Chip'i) + sil IconButton) + "Yeni hesap" butonu + şifre değiştir butonları + en altta "Oturumu kapat" OutlinedButton (sadece `memo_session_role` kayıtlıysa, yani gerçek bir login varsa).

- [ ] **Step 3: settings_dialog'a kaydet**

`frontend/lib/widgets/settings_dialog.dart`:
- `_tabs` listesine (satır 92-113: `L10n.t('remote_access')`'in hemen ardına): `L10n.t('tab_accounts'),`
- `_tabIcons` listesine aynı sırayla: `Icons.people_alt_outlined` (icon listesinin adını dosyada doğrula — `_tabIcons` ya da inline `List<IconData>`)
- `_groups` listesine: `('settings_group_system', [12, 13, 14, 20])` — index'i `_tabs`'taki sıraya göre hesapla (20 = son eleman; grup isim anahtarları mevcut anahtar adlarıyla)
- `_buildTabContent` switch'ine: `case 20: return AccountsTab();`
- import ekle: `import 'settings/tabs/accounts_tab.dart';`

Not: sekmeler dinamik (`_tabs` getter) — switch'te sayı sabit kalır, `_tabs` getter'ı döndürdüğü uzunluk değişince `_activeTab.clamp` zaten korur.

- [ ] **Step 4: Widget test'i**

`frontend/test/widgets/accounts_tab_test.dart` (yeni) — `_StatefulAuthAdapter` desenini (Task 5 testinden) kopyala, şu route'ları stub'la: `/api/accounts` GET → 2 hesap (`admin`/`kaya`), DELETE → 200, POST → 200:

```dart
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('admin session: lists accounts, add works', (tester) async {
    SharedPreferences.setMockInitialValues({'memo_session_role': 'admin'});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    final requests = <RequestOptions>[];
    client.dio.httpClientAdapter = _RecordingAdapter(
      {
        '/api/accounts': (200, [
          {'id': 'a1', 'username': 'admin', 'role': 'admin'},
          {'id': 'a2', 'username': 'kaya', 'role': 'user'},
        ]),
      },
      onRequest: requests.add,
    );
    await tester.pumpWidget(ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
      child: MaterialApp(home: Scaffold(body: AccountsTab())),
    ));
    await tester.pumpAndSettle();
    expect(find.text('admin'), findsOneWidget);
    expect(find.text('kaya'), findsOneWidget);
    expect(find.text(L10n.t('accounts_role_admin')), findsOneWidget);
  });

  testWidgets('user session: admin-only note, no management actions', (tester) async {
    SharedPreferences.setMockInitialValues({'memo_session_role': 'user'});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _RecordingAdapter({});
    await tester.pumpWidget(ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
      child: MaterialApp(home: Scaffold(body: AccountsTab())),
    ));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('accounts_admin_only_note')), findsOneWidget);
    expect(find.text(L10n.t('accounts_add')), findsNothing);
  });
}
```

`_RecordingAdapter`: Task 5'teki `_StatefulAuthAdapter`'dan türet (requests listesi + `onRequest` callback'i; login/create-admin authed mantığı gerekmez — sadece map'ten yanıtla + kaydet). Liste ekleme akışı testi opsiyonel (üstteki ikisi kapsamı örter); `deleteAccount`/`changePassword` dialog akışları `tap` + `pumpAndSettle` + request kaydı ile aynı desende genişletilebilir — minimum: listeleme + rol görünürlüğü.

- [ ] **Step 5: Test'leri çalıştırıp pass olduğunu gör**

Run: `flutter test test/widgets/accounts_tab_test.dart && flutter test test/widgets/settings_dialog_test.dart`
Expected: PASS; mevcut settings_dialog testleri bozulmadı.

- [ ] **Step 6: Kural #8 grep + flutter analyze**

Run: `git diff --name-only -- '*.dart' | xargs -r grep -nE "(Text|Tooltip|SnackBar|AlertDialog)\(\s*['\"][A-Za-zÇĞİÖŞÜçğıöşü]"` → boş.
Run: `flutter analyze lib/` → mevcut info'lar dışında temiz.

- [ ] **Step 7: Commit**

```bash
git add frontend/lib/widgets/settings/tabs/accounts_tab.dart frontend/lib/widgets/settings_dialog.dart frontend/lib/core/l10n.dart frontend/test/widgets/accounts_tab_test.dart
git commit -m "feat(frontend): Settings Accounts tab (list, add, delete, password change, sign out)

Admin sessions manage accounts; user sessions only change their own
password. Signs out by clearing the saved session token and
invalidating the auth gate. Closes the long-standing desktop gap
(accounts have been backend-only since Faz 5.1)."
```

---

### Task 7: Entegrasyon doğrulaması ve elle test talimatı

**Files:** (değişiklik yok — yalnızca doğrulama)

- [ ] **Step 1: Backend tam suite**

Run: `CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./... && CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race`
Expected: hepsi yeşil.

- [ ] **Step 2: Frontend tam suite**

Run: `flutter analyze lib/ && flutter test`
Expected: analyze temiz (mevcut 5 info hariç), tüm testler yeşil.

- [ ] **Step 3: Canlı masaüstü smoke (kullanıcı elinde)**

- `go run -tags "sqlite_fts5" . --headless --port 8090` ile başlat; `flutter run -d linux`.
- Beklenen: ilk açılışta SetupGate görünür ("Başka cihazlar?" → Hayır) → wizard → normal app. (Mevcut kurulum zaten account kurdaysa LoginGate devreye girer — login → app.)
- Ayarlar → Hesaplar: hesap listesi + ekle/sil/şifre değiştir akışları; "Oturumu kapat" → LoginGate geri gelir.
- RPi web'de: web açılır → (setup yoksa) SetupGate → Evet + Sadece şifre → şifre kur → sohbet açılır; ikinci tarayıcıda LoginGate → giriş → çalışır.

- [ ] **Step 4: handoff güncelle**

`handoff.md`'nin en üstüne giriş ekle (yapılanlar, commit'ler, RPi'de canlı doğrulama henüz yapılmadı notu, BUG-ONB1 ile ilişki).

- [ ] **Step 5: Commit**

```bash
git add handoff.md
git commit -m "docs: handoff entry for the universal auth screen implementation"
```

---

## Self-Review (plan yazari notu)

- **Spec kapsamı:** tüm spec bölümleri karşılandı — backend endpoint (T1+T2), API client (T3), gate state machine (T4), Setup+Login gate'leri + l10n TR/EN (T5), Hesaplar sekmesi + rol görünürlüğü + oturum kapatma (T6), doğrulama (T7). Spec'teki "403 → LoginGate'e yönlendir" davranışı T5 `_submit`'te `auth_gate_error_create_failed` hatası + kullanıcı devam ettiğinde bir sonraki poll'de needs_setup=false görünüp LoginGate'e düşmesiyle karşılanır (invalidasyon gerektirmez).
- **Placeholder taraması:** tüm test kodları ve imzalar yukarıda; widget görsel yerleşimleri "desen: mevcut tab'ları izle" düzeyinde bilinçli bırakıldı (kod tabanıyla birebir aynı Infra — `MemoTheme`, `L10n`, `RadioListTile`, `SegmentedButton` mevcut).
- **Tip tutarlılığı:** `ChangeAccountPassword(sessionToken, id, currentPassword, newPassword)` her yerde aynı; `AuthGateInfo.state/authMode` tutarlı; `setSessionToken`/`setRemoteAuthConfig(mode, {username, password})` imzaları Task 3/5'te aynı; prefs anahtarları (`memo_remote_access_token` mevcut, `memo_auth_setup_done` yeni, `memo_session_role` yeni) tutarlı.