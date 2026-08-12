import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import '../core/local_session_state.dart';
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
///   - needs_setup && declined flag, but a non-loopback probe 401s
///                                    -> setup gate (that flag was recorded
///                                       against a backend this client can
///                                       no longer authenticate to)
///   - needs_setup && declined flag   -> nothing (open local install)
///   - setup done, loopback source   -> nothing (backend trusts loopback
///     — remoteAuthOK passes credential-less loopback requests, so asking
///     for a password here would be demanding one that is never checked)
///   - setup done, no saved token    -> login gate (remote/unknown source)
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
      // Layer 1 — identity. If this backend is not the one our saved
      // state was recorded against (wiped and reinstalled, or the client
      // now points at a different Memo), that state is worthless and
      // keeping it is what left users stuck: a stale declined flag hides
      // the setup gate while every request 401s.
      await _resetIfServerReplaced(prefs, ss.installId);
      var declined = prefs.getBool(authSetupDoneKey) ?? false;
      // Layer 2 — reachability. Independent of layer 1 on purpose: it
      // works against any backend, including ones too old to report an
      // install id, and against a client whose id was never recorded.
      // "Setup pending" plus "this source cannot authenticate" is a
      // contradiction the declined flag must not be allowed to hide.
      if (ss.needsSetup && declined && !ss.loopback) {
        if (await api.probeAuth() == ApiAuthStatus.unauthorized) {
          await prefs.remove(authSetupDoneKey);
          declined = false;
        }
      }
      if (ss.needsSetup && !declined) {
        yield AuthGateInfo(AuthGateState.setupNeeded, authMode: ss.authMode);
      } else if (!ss.needsSetup) {
        if (ss.loopback) {
          yield const AuthGateInfo(AuthGateState.ok);
        } else {
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

/// Drops this client's server-coupled state when [installId] shows the
/// backend is not the one that state was recorded against.
///
/// Two cases are deliberately treated as "no reset":
///
///  * An empty [installId] — an older backend, or one that could not
///    persist an id. Unknown is not a mismatch; the gate's unauthorized
///    probe covers those.
///  * Nothing stored locally yet. Every existing client is in exactly
///    this position the first time it runs a build that knows about
///    install ids, and their saved sign-ins are overwhelmingly valid —
///    resetting on first sight would sign out every working user to
///    catch the few who are broken. Those few are caught by the probe
///    instead, which costs them nothing but a login they already needed.
Future<void> _resetIfServerReplaced(
  SharedPreferences prefs,
  String installId,
) async {
  if (installId.isEmpty) return;
  final known = prefs.getString(serverInstallIdKey);
  if (known == null || known.isEmpty) {
    await prefs.setString(serverInstallIdKey, installId);
    return;
  }
  if (known == installId) return;
  await clearServerCoupledState(prefs, keepInstallId: installId);
}
