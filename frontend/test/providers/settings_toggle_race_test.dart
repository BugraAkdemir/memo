import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// A fake [HttpClientAdapter] answering GET with the current value and
/// recording every PUT it receives. Real sockets (even to 127.0.0.1) aren't
/// usable here — `flutter test` installs its own `dart:io` HTTP shim that
/// intercepts them before they reach any real server — so this hooks into
/// Dio's own adapter layer instead, which bypasses that shim entirely and
/// needs no mocking package.
class _FakeToggleAdapter implements HttpClientAdapter {
  final String path;
  final List<bool> puts = [];
  bool current = false;

  _FakeToggleAdapter(this.path);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'GET' && options.path == path) {
      return ResponseBody.fromString(
        jsonEncode({'enabled': current}),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    if (options.method == 'PUT' && options.path == path) {
      final bytes = await (requestStream ?? const Stream.empty()).expand((e) => e).toList();
      final body = jsonDecode(utf8.decode(bytes)) as Map;
      final v = body['enabled'] as bool;
      puts.add(v);
      current = v;
      // Delay before responding — widens the window for the race this test
      // guards against without changing whether it's caught: the notifier's
      // in-flight guard is checked synchronously before any await, so it's
      // deterministic regardless of how long the "network" call takes.
      await Future.delayed(const Duration(milliseconds: 30));
      return ResponseBody.fromString(
        jsonEncode({'ok': true}),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

// Regression test for the settings-toggle double-tap race: toggle() used to
// compute `next = !state.valueOrNull` and only update `state` after its own
// await finished. Two calls fired before the first resolved both read the
// same starting value and both landed on the same end state — e.g.
// off→on, off→on again (net: on, one PUT(true) then a second PUT(true))
// instead of the intended net no-op (PUT(true) then PUT(false)). Fixed with
// an in-flight guard (ignore a second toggle() while one is pending) plus
// an optimistic state update applied before the network call.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('MemoryEnabledNotifier.toggle() ignores a second call while the first is in flight', () async {
    final adapter = _FakeToggleAdapter('/api/memory/enabled');
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;

    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
    ]);
    addTearDown(container.dispose);

    final initial = await container.read(memoryEnabledProvider.future);
    expect(initial, false);

    final notifier = container.read(memoryEnabledProvider.notifier);
    final first = notifier.toggle();
    final second = notifier.toggle(); // fired before `first`'s PUT resolves
    await first;
    await second;

    expect(adapter.puts, [true],
        reason: 'exactly one PUT should reach the backend — the second '
            'toggle() call must be a no-op while the first is still in flight');
    expect(container.read(memoryEnabledProvider).valueOrNull, true);
  });

  test('MinimalModeNotifier.toggle() ignores a second call while the first is in flight', () async {
    final adapter = _FakeToggleAdapter('/api/system-prompt/minimal-mode');
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;

    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
    ]);
    addTearDown(container.dispose);

    final initial = await container.read(minimalModeProvider.future);
    expect(initial, false);

    final notifier = container.read(minimalModeProvider.notifier);
    final first = notifier.toggle();
    final second = notifier.toggle();
    await first;
    await second;

    expect(adapter.puts, [true]);
    expect(container.read(minimalModeProvider).valueOrNull, true);
  });
}
