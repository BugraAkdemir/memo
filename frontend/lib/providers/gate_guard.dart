import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth_gate_provider.dart';

/// True while the auth gate has not granted access yet — the setup/login
/// screen is up, or the gate's own poll hasn't decided yet (null).
///
/// Background work must consult this before hitting the backend: on a
/// token-gated backend every such request just 401s (BUG-ONB4 — the RPi web
/// UI kept polling models/status, embedding/status, download/progress and
/// mood/* every few seconds while the login gate was up, spamming 401s into
/// the console and turning messagesProvider's 401 into a visible
/// "Bir şeyler ters gitti" instead of the login prompt). A blocked gate must
/// be treated as "unknown", never as "authorized" — a null gate (first poll
/// still in flight) is the one case where a genuine signed request could
/// still be welcome, but the safe default is to wait for the gate to speak.
bool authGateBlocked(AuthGateInfo? gate) =>
    gate == null || gate.state != AuthGateState.ok;

/// [Future.delayed] stand-in for poll loops' sleeps. A bare
/// `await Future.delayed(...)` inside a stream provider keeps its fake timer
/// alive even after the provider is disposed — widget tests then die with
/// "Pending timers" no matter how the tree is unmounted. A [Timer] cancelled
/// via `ref.onDispose` disappears cleanly, so the poll stops dead the moment
/// its provider is disposed. Stream providers that poll MUST route every
/// sleep through this.
Future<void> cancellablePause(Ref ref, Duration duration) {
  final completer = Completer<void>();
  final timer = Timer(duration, completer.complete);
  ref.onDispose(timer.cancel);
  return completer.future;
}