/// Normalizes a user-typed backend address into a full URL
/// MemoApiClient's Dio instance will accept without throwing.
///
/// This is the mobile counterpart of frontend/lib/core/backend_url.dart's
/// fix for the exact same bug class: MemoApiClient._initDio()
/// (mobile/lib/core/api_client.dart) constructs `Dio(BaseOptions(baseUrl:
/// ...))` synchronously, and Dio validates baseUrl eagerly inside that
/// constructor — a schemeless value throws immediately, before any
/// try/catch between a bad saved value and this construction gets a
/// chance to run.
///
/// Before this existed, that inline scheme-fixup only lived inside
/// ConnectionNotifier.connect() (a *new* manual connection attempt) —
/// ConnectionNotifier.loadSavedUrl() (called on every cold start via
/// autoConnectIfPossible()) read the raw saved value straight from
/// SharedPreferences and called MemoApiClient.updateBaseUrl() with it
/// directly, no normalization at all. And SettingsScreen's own save path
/// only stripped a trailing slash (never added a scheme), so a bare host
/// like "192.168.1.50" could genuinely end up persisted to the
/// "backend_url" key — reproducing the desktop crash exactly: the app
/// fails to boot past ConnectionNotifier's own startup call, on every
/// launch, with no in-app screen ever reachable to fix it.
///
/// Behavior deliberately mirrors ConnectionNotifier.connect()'s existing
/// logic (not frontend's normalizeBackendUrl verbatim): Tailscale
/// Funnel hosts (*.ts.net) get "https://", everything else gets "http://",
/// and — unlike the desktop version — no default port is ever appended.
/// Forcing :8090 onto a bare Funnel host would break it: Funnel serves
/// over standard HTTPS (443, implicit), not Memo's local dev port.
String normalizeBackendUrl(String input) {
  var normalized = input.trim().replaceAll(RegExp(r'/+$'), '');
  if (normalized.isEmpty) return normalized;
  if (!normalized.startsWith('http://') && !normalized.startsWith('https://')) {
    final host = normalized.split(':').first;
    normalized = (host.endsWith('.ts.net') ? 'https://' : 'http://') + normalized;
  }
  return normalized;
}
