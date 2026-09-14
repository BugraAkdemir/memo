import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/providers/tasklist_provider.dart';

/// Answers /api/tasklists differently by call count:
///   1st call (the provider's own build()) resolves immediately.
///   2nd call (a poll tick's refresh(), simulated) blocks on [gate].
///   3rd+ calls (the fresh notifier's build() after an invalidate) resolve
///   immediately with different content — standing in for whatever the
///   backend's real current state is by the time the rebuild happens.
class _RaceAdapter implements HttpClientAdapter {
  _RaceAdapter(this.gate);
  final Completer<void> gate;
  int calls = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    calls++;
    String body;
    switch (calls) {
      case 1:
        body = '[{"id":"initial"}]';
        break;
      case 2:
        await gate.future;
        body = '[{"id":"stale"}]';
        break;
      default:
        body = '[{"id":"fresh"}]';
    }
    return ResponseBody.fromString(body, 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

// P2-17 (bug-report.md): TaskListsNotifier/RunningTasksNotifier's poll
// methods (_silentRefresh/refresh) write `state = ...` after an unguarded
// await, with nothing checking whether this notifier instance is still the
// live one. app_shell.dart's auth-gate transition listener calls
// ref.invalidate(taskListsProvider) — if that fires while a poll tick's
// HTTP request is still in flight, the poll's stale response can land
// *after* the freshly-rebuilt notifier already has correct data, silently
// overwriting it with old information (which then sits there until the
// next 3-second poll tick happens to correct it again).
void main() {
  test('a poll response that lands after a gate-transition invalidate does not clobber the freshly-rebuilt state',
      () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final gate = Completer<void>();
    final adapter = _RaceAdapter(gate);
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;

    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      authGateProvider.overrideWith(
        (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
      ),
    ]);
    addTearDown(container.dispose);

    // authGateProvider is StreamProvider.autoDispose — in the real app
    // AppShell's own permanent ref.listen keeps it alive; here nothing does,
    // so a plain container.read would let it get disposed and restart from
    // AsyncValue.loading() (== blocked) on the very next read, same "test
    // trap" gate_blocked_providers_test.dart already documents. Keep a
    // listener alive for the rest of this test so both builds below see the
    // already-resolved "ok" state, not a fresh loading one.
    container.listen(authGateProvider, (_, __) {}, fireImmediately: true);
    await container.read(authGateProvider.future);

    final initial = await container.read(taskListsProvider.future);
    expect(initial.single.id, 'initial');

    // Simulate a poll tick: call the same public refresh() _silentRefresh
    // itself uses internally, in flight against `gate`.
    final notifierA = container.read(taskListsProvider.notifier);
    final pollFuture = notifierA.refresh();

    // The auth gate flips (app_shell.dart's ref.listen) while that poll is
    // still in flight — invalidate forces a brand new notifier + build().
    container.invalidate(taskListsProvider);
    final rebuilt = await container.read(taskListsProvider.future);
    expect(rebuilt.single.id, 'fresh');

    // Now let the stale in-flight poll's response land.
    gate.complete();
    await pollFuture;
    await Future<void>.delayed(Duration.zero);

    final finalState = container.read(taskListsProvider).value;
    expect(finalState!.single.id, 'fresh',
        reason: 'the stale poll response (from before the gate-transition '
            'invalidate) must not overwrite the freshly-rebuilt state');
  });
}
