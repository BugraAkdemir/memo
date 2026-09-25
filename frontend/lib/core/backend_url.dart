import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;

/// Whether this build runs on a phone/tablet, where no local Memo backend
/// can possibly exist.
///
/// `kIsWeb` has to be tested first and cannot be skipped: on web
/// `defaultTargetPlatform` reports the *browser's* platform, so a page
/// opened in Chrome on an Android phone answers `TargetPlatform.android`
/// while still being the web build — which is served BY the backend it
/// talks to and therefore wants the page-origin default, not the
/// mobile one. Same ordering rule app_shell.dart documents for `Platform.*`.
///
/// Uses `defaultTargetPlatform` rather than `dart:io`'s `Platform` so this
/// file stays importable from the web build.
bool get isMobilePlatform =>
    !kIsWeb &&
    (defaultTargetPlatform == TargetPlatform.android ||
        defaultTargetPlatform == TargetPlatform.iOS);

/// The address to fall back on when nothing has been configured yet.
///
/// Takes its inputs as explicit parameters instead of reading `kIsWeb` /
/// `defaultTargetPlatform` / `Uri.base` internally so every combination is
/// unit-testable under `flutter test` (which runs on the VM, where the app
/// is neither web nor mobile) — the same reason [webBackendUrl] below takes
/// its origins as parameters.
///
/// - **web** → [pageOrigin]. The embedded web app is always served BY the
///   exact backend it must talk to (see internal/webserver), so the page's
///   own origin is always right.
/// - **mobile** → empty. A phone never runs Memo's Go backend, so there is
///   no defensible guess at all: 127.0.0.1 would point at the phone itself
///   and guarantee a broken state. Empty means "the user has to tell us",
///   which is what the setup wizard's server-address step is for.
/// - **desktop** → `http://127.0.0.1:8090`, where the user plausibly
///   started a backend themselves.
String defaultBackendUrl({
  required bool isWeb,
  required bool isMobile,
  required String pageOrigin,
}) {
  if (isWeb) return pageOrigin;
  if (isMobile) return '';
  return 'http://127.0.0.1:8090';
}

/// Normalizes a user-typed backend address into a full URL Dio's
/// `BaseOptions` will accept without throwing.
///
/// Reported bug: typing just "127.0.0.1" (no scheme) into the backend URL
/// field crashed the *entire app* with Flutter's red error screen —
/// "Invalid argument (baseUrl): Must be a valid URL on platforms other than
/// Web." Dio validates `baseUrl` eagerly, synchronously, inside its
/// constructor, and `apiClientProvider` (chat_provider.dart) builds a
/// MemoApiClient directly from whatever string is saved — no try/catch
/// anywhere sits between a bad saved value and that constructor, so the
/// crash happens before any UI (including the "change server" screens that
/// exist specifically to fix a bad address) can render at all. Once a bad
/// value like this is saved, the app can never boot again without editing
/// SharedPreferences by hand — so this has to run on every *read*, not just
/// when the user clicks Apply, to self-heal a value that was already saved
/// before this existed.
///
/// - Missing scheme ("127.0.0.1", "192.168.1.50:9000") gets "http://"
///   prepended — Memo's backend is plain HTTP, never TLS (see AGENTS.md) —
///   *except* a Tailscale Funnel host ("*.ts.net"), which gets "https://".
///   A scheme the user *did* type (http:// or https://) is left alone.
/// - Missing port gets Memo's own default (8090) appended **only for
///   http**, so "192.168.1.50" and "192.168.1.50:8090" behave identically;
///   an explicit port (e.g. ":1234") is always respected. An https URL
///   never gets a port forced onto it: Funnel serves over standard,
///   implicit 443, and ":8090" there breaks the connection outright. That
///   rule (and the *.ts.net one above) came from the retired mobile client,
///   which reached Memo over Funnel routinely and had them from the start;
///   this copy used to force 8090 onto every scheme and so could not talk
///   to a Funnel address at all.
/// - Empty/unparseable input falls back to [defaultBackendUrl], which is
///   platform-dependent — notably empty on mobile.
String normalizeBackendUrl(String input) {
  // Uri.base is meaningless on non-web platforms (resolves to a file:// URI
  // or the process cwd), so it is only read when kIsWeb is already true.
  final fallback = defaultBackendUrl(
    isWeb: kIsWeb,
    isMobile: isMobilePlatform,
    pageOrigin: kIsWeb ? Uri.base.origin : '',
  );
  final trimmed = input.trim().replaceAll(RegExp(r'/+$'), '');
  if (trimmed.isEmpty) return fallback;

  final String withScheme;
  if (trimmed.contains('://')) {
    withScheme = trimmed;
  } else {
    // Split on ':' to look at the host alone — "myhost.ts.net:8443" must
    // still be recognised as a Funnel host.
    final host = trimmed.split(':').first;
    withScheme =
        (host.endsWith('.ts.net') ? 'https://' : 'http://') + trimmed;
  }
  final uri = Uri.tryParse(withScheme);
  if (uri == null || uri.host.isEmpty) {
    // Never seen a genuinely unparseable host in practice, but this must
    // never itself throw — falling back to Memo's own default is always
    // safer than risking a repeat of the exact crash this exists to fix.
    return fallback;
  }

  final normalized =
      (!uri.hasPort && uri.scheme == 'http') ? uri.replace(port: 8090) : uri;
  final result = normalized.toString();
  return result.endsWith('/') ? result.substring(0, result.length - 1) : result;
}

/// Resolves the effective backend URL for the WEB build.
///
/// [saved] is the user-configured address (SharedPreferences
/// `memo_api_base_url`); [pageOrigin] is the origin this page itself was
/// loaded from (`Uri.base.origin`). The embedded web app is always served
/// BY the backend it must talk to, so the page's own origin is the correct
/// default — but a *saved* value must be viewed through that lens:
///
/// - empty saved value → the page's own origin;
/// - saved loopback URL (localhost/127.0.0.1/::1) while the page itself
///   was loaded from a non-loopback address → **stale**: that value means
///   "this client's own machine," which on a phone/laptop pointed at
///   http://192.168.1.x:8090 is the client itself, not the server that
///   served the page. This exact mismatch is what produced the wall of
///   "Cross-Origin Request Blocked … 127.0.0.1" errors while the page ran
///   from the LAN address — every API call went to the device's own
///   loopback, which the backend's CORS correctly refused to bless.
///   Ignore the saved value, use the page origin (self-heals the bad
///   value without the user having to find "Sunucuyu Değiştir");
/// - anything else (a genuinely different server the user configured on
///   purpose) → respected as-is, so the change-server flow keeps working.
///
/// Takes the origins as explicit parameters instead of reading
/// `Uri.base`/`kIsWeb` internally so the whole decision is unit-testable
/// under `flutter test` (which runs on the VM, where kIsWeb is false and
/// Uri.base is a file:// path).
String webBackendUrl(String saved, String pageOrigin) {
  final trimmed = saved.trim();
  if (trimmed.isEmpty) return pageOrigin;
  final resolved = normalizeBackendUrl(trimmed);
  if (_isLoopbackHost(resolved) && !_isLoopbackHost(pageOrigin)) {
    return pageOrigin;
  }
  return resolved;
}

bool _isLoopbackHost(String url) {
  final uri = Uri.tryParse(url);
  if (uri == null) return false;
  final host = uri.host.toLowerCase();
  return host == 'localhost' || host == '127.0.0.1' || host == '::1';
}
