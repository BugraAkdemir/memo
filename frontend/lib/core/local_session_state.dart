import 'package:shared_preferences/shared_preferences.dart';

/// SharedPreferences keys that describe THIS CLIENT'S relationship with a
/// particular backend — credentials, the identity behind them, and the
/// first-run milestones that only make sense against the install that was
/// there at the time.
///
/// They are meaningless (worse: actively harmful) once the backend they
/// were recorded against is gone. On the web this bites hard, because
/// localStorage is keyed by origin: wiping a self-hosted Memo and
/// reinstalling it reuses the same `http://<lan-ip>:8090`, so the browser
/// carries every one of these values over to a backend that has never
/// heard of them. Reported live from a Raspberry Pi (2026-08-13) —
/// memo_auth_setup_done alone was enough to suppress the setup screen
/// forever while every API call 401'd, and a hard reload could not fix it
/// because Ctrl+Shift+R only bypasses the HTTP cache.
const serverCoupledPrefsKeys = <String>[
  // "I already answered the first-run question" — the one that suppressed
  // the setup gate against a freshly reinstalled backend.
  'memo_auth_setup_done',
  // Credentials and who they belong to.
  'memo_remote_access_token',
  'memo_session_username',
  'memo_session_role',
  // First-run milestones: a genuinely new install should show these again.
  'memo_setup_complete',
  'memo_launchpad_seen',
  'memo_tour_seen',
  // Not a device preference despite looking like one: this mirrors the
  // backend's own cfg.Beta and is only consulted until the server answers.
  // Carried across to a different install it gates UI (the Swarm tab) on a
  // flag that install never set — which is exactly how Swarm stayed
  // visible against a backend with beta:false.
  'memo_beta_features',
];

/// The install id (see SetupStatus.installId) this client last saw. Held
/// separately from [serverCoupledPrefsKeys] because the automatic reset
/// path immediately rewrites it with the new server's id, while a manual
/// reset clears it outright.
const serverInstallIdKey = 'memo_server_install_id';

/// Drops every server-coupled value, leaving device preferences alone.
///
/// Deliberately preserved: memo_locale, memo_theme_mode and memo_streaming
/// (genuinely this device's preferences, not the server's), and above all
/// memo_api_base_url — that is *how the client reaches the
/// backend at all*, so clearing it would strand a desktop client on
/// localhost. Re-pointing at a different server is a separate, explicit
/// action (ChangeServerDialog).
///
/// [keepInstallId] is what the automatic path passes: the caller is about
/// to store the new server's id, so wiping it here would only cause the
/// next poll to treat that server as unknown all over again.
Future<void> clearServerCoupledState(
  SharedPreferences prefs, {
  String? keepInstallId,
}) async {
  for (final key in serverCoupledPrefsKeys) {
    await prefs.remove(key);
  }
  if (keepInstallId != null && keepInstallId.isNotEmpty) {
    await prefs.setString(serverInstallIdKey, keepInstallId);
  } else {
    await prefs.remove(serverInstallIdKey);
  }
}
