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
  const fallback = 'http://127.0.0.1:8090';
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
