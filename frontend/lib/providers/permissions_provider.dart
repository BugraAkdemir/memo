import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';

/// SharedPreferences keys the login flow writes to (auth_gate_overlay.dart)
/// alongside the existing memo_session_role/memo_session_username — see
/// those for the established convention this follows (a plain prefs string,
/// not a dedicated Riverpod provider/notifier, since role is read the same
/// ad-hoc way everywhere it's needed).
const String memoSessionPermissionsKey = 'memo_session_permissions';

/// Reads the current session's effective permissions (Faz 5.1.1,
/// yapacam.md) from local prefs for UI gating (hiding a Settings tab a
/// restricted account can't use) — NOT a security boundary by itself, the
/// backend's own requirePermission/requirePermissionStrict is (this can be
/// bypassed by anyone editing local storage, same as the existing
/// memo_session_role check already can be). Only ever restrictive when the
/// saved role is exactly "user" AND a permissions value was actually saved;
/// every other case (no session at all — local desktop; an admin session;
/// an already-active session from before this feature existed, so nothing
/// was ever persisted for it) fails open to [AccountPermissions.allTrue],
/// matching this whole system's server-side fail-open philosophy.
AccountPermissions readMyPermissions(SharedPreferences prefs) {
  final role = prefs.getString('memo_session_role');
  if (role != 'user') return AccountPermissions.allTrue;
  final raw = prefs.getString(memoSessionPermissionsKey);
  if (raw == null || raw.isEmpty) return AccountPermissions.allTrue;
  try {
    return AccountPermissions.fromJson(
      jsonDecode(raw) as Map<String, dynamic>,
    );
  } catch (_) {
    return AccountPermissions.allTrue;
  }
}

/// Persists [permissions] under [memoSessionPermissionsKey] — called by the
/// login flow (auth_gate_overlay.dart) right after a successful password
/// login, alongside memo_session_role/memo_session_username.
Future<void> saveMyPermissions(
  SharedPreferences prefs,
  AccountPermissions permissions,
) async {
  await prefs.setString(
    memoSessionPermissionsKey,
    jsonEncode(permissions.toJson()),
  );
}
