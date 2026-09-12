import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';

/// Answers /api/version and /api/clients/register successfully (a fixed
/// client_id), and 200/empty for everything else (syncRoutineUtcOffset).
class _FakeReachableAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/api/clients/register') {
      return ResponseBody.fromString('{"client_id":"client-123"}', 200, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
    return ResponseBody.fromString('{}', 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

// Regression coverage for the client-unregister-on-quit fix: TrayController
// reads currentClientIdProvider to know which ID to send to
// unregisterClient before an actual window close. This provider is only
// useful if connectionStatusProvider — the one place that owns the real
// clientId lifecycle (register/heartbeat/gate-blocked/unreachable) — keeps
// it in sync. That loop's own local `clientId` was previously invisible
// outside its closure at all (the doc comment even said "there's no
// reliable app is closing hook on desktop today").
void main() {
  test('currentClientIdProvider mirrors connectionStatusProvider\'s registered client id', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeReachableAdapter();

    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      authGateProvider.overrideWith(
        (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
      ),
    ]);
    addTearDown(container.dispose);

    // Gate must resolve before connectionStatusProvider's own
    // authGateBlocked() check reads it as "ok", not "unknown/blocked".
    await container.read(authGateProvider.future);

    expect(container.read(currentClientIdProvider), isNull,
        reason: 'no client registered yet');

    // Kick the stream provider's loop off and let it run one full
    // iteration (isAlive -> register -> sync) — real Futures from the fake
    // adapter resolve near-instantly, so a few microtask turns are enough;
    // this never needs to wait out the loop's real 30s pause between ticks.
    final sub = container.listen(connectionStatusProvider, (_, _) {});
    addTearDown(sub.close);
    for (var i = 0; i < 10; i++) {
      await Future<void>.delayed(Duration.zero);
    }

    expect(container.read(currentClientIdProvider), 'client-123',
        reason: 'connectionStatusProvider should have registered and synced its client id');
  });
}
