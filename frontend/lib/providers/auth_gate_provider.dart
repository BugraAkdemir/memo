import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

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
  Timer? pollTimer;
  ref.onDispose(() {
    alive = false;
    pollTimer?.cancel();
  });
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
    // A cancellable pause instead of Future.delayed: onDispose must be able
    // to stop the poll cleanly (widget tests assert no pending timers).
    final pause = Completer<void>();
    pollTimer = Timer(const Duration(seconds: 30), pause.complete);
    await pause.future;
  }
});
