import 'package:flutter/foundation.dart' show kIsWeb;

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
///   prepended — Memo's backend is plain HTTP, never TLS (see AGENTS.md).
///   A scheme the user *did* type (http:// or https://) is left alone.
/// - Missing port gets Memo's own default (8090) appended, so
///   "192.168.1.50" and "192.168.1.50:8090" behave identically; an
///   explicit port (e.g. ":1234") is always respected.
String normalizeBackendUrl(String input) {
  // On web, this app is always served BY the exact Memo backend it needs
  // to talk to (embedded into the Go binary, see internal/webserver) —
  // the page's own origin is always the right default, unlike desktop
  // where 127.0.0.1 is a reasonable guess for "the backend I might have
  // started myself." Hardcoding 127.0.0.1:8090 here for web was actively
  // wrong the moment the page is loaded from any address other than
  // localhost (e.g. a phone/laptop opening http://192.168.1.106:8090/ on
  // the LAN) — every API call would try to reach that *client's own*
  // loopback address instead of the server that served the page, the
  // same client/server confusion class as the file-picker bug fixed
  // earlier this session. Uri.base is meaningless on non-web platforms
  // (resolves to a file:// URI or the process cwd), so this is
  // deliberately gated on kIsWeb rather than applied universally.
  final fallback = kIsWeb ? Uri.base.origin : 'http://127.0.0.1:8090';
  final trimmed = input.trim();
  if (trimmed.isEmpty) return fallback;

  final withScheme = trimmed.contains('://') ? trimmed : 'http://$trimmed';
  final uri = Uri.tryParse(withScheme);
  if (uri == null || uri.host.isEmpty) {
    // Never seen a genuinely unparseable host in practice, but this must
    // never itself throw — falling back to Memo's own default is always
    // safer than risking a repeat of the exact crash this exists to fix.
    return fallback;
  }

  final normalized = uri.replace(port: uri.hasPort ? uri.port : 8090);
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
