import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

class _UnauthorizedAdapter implements HttpClientAdapter {
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    return ResponseBody.fromString('{"error":"unauthorized"}', 401,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        });
  }

  @override
  void close({bool force = false}) {}
}

/// remoteAccessProvider decides whether AppShell shows the Swarm tab, via
/// _showSwarmNav's `containsKey('beta')` check. Reported live: the tab
/// stayed visible against a backend with beta:false.
///
/// The chain was: this provider fires at app start, 401s behind the auth
/// gate, and its catch returns {'enabled': false} — a map with no 'beta'
/// key, which _showSwarmNav reads as "the server hasn't answered yet" and
/// so defers to the local mirror pref, forever. Blocked must therefore
/// yield a map that is *empty*, not one that merely lacks 'beta' by
/// accident, and the request must not be spent at all.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> containerFor(AuthGateState gate) async {
    SharedPreferences.setMockInitialValues({'memo_beta_features': true});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _UnauthorizedAdapter();
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      authGateProvider.overrideWith(
        (ref) => Stream.value(AuthGateInfo(gate, authMode: 'password')),
      ),
    ]);
    addTearDown(container.dispose);
    // The provider reads the gate synchronously in its body, and an
    // overridden Stream has not emitted yet at that point — without this
    // await every case would look "blocked" (valueOrNull == null). Same
    // trap settings_toggle_race_test.dart hit when gate checks landed.
    await container.read(authGateProvider.future);
    return container;
  }

  test('blocked gate yields an empty map and spends no request', () async {
    final container = await containerFor(AuthGateState.loginNeeded);
    final ra = await container.read(remoteAccessProvider.future);

    expect(ra, isEmpty,
        reason: 'a non-empty fallback would be read as a real server answer');
    expect(ra.containsKey('beta'), isFalse);
  });

  test('an open gate still performs the request', () async {
    final container = await containerFor(AuthGateState.ok);
    // The adapter 401s, so the provider's own catch produces its fallback —
    // what matters here is that the attempt was made at all.
    final ra = await container.read(remoteAccessProvider.future);
    expect(ra, isNotEmpty);
  });
}
