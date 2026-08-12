import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/screens/calendar_screen.dart';
import 'package:memo_flutter/screens/routines_screen.dart';

/// Answers 401 (a token-gated backend with the gate still up) and counts
/// calls, so a test can assert nothing was even attempted.
class _UnauthorizedAdapter implements HttpClientAdapter {
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    return ResponseBody.fromString(
      '{"error":"unauthorized"}',
      401,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// BUG-ONB11 — reported live from the desktop app connected to a
/// self-hosted Raspberry Pi: Routines showed a permanent "Rutinler
/// yüklenemedi", and Developer's model list a permanent error, while the
/// rest of the app worked fine.
///
/// Same root cause as BUG-ONB6, in two shapes that sweep could not see. It
/// audited AsyncNotifier.build() methods; these screens fetch straight from
/// initState. AppShell's IndexedStack builds every screen at app start —
/// before the auth gate opens — so that single attempt 401s. Routines then
/// has nothing to retry it (no timer, no provider to invalidate), and the
/// calendar only recovers after the user opens its tab, showing a wrong
/// error banner until then.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<(_UnauthorizedAdapter, ProviderContainer)> harness({
    required AuthGateState gateState,
  }) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final adapter = _UnauthorizedAdapter();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      authGateProvider.overrideWith(
        (ref) => Stream.value(AuthGateInfo(gateState, authMode: 'password')),
      ),
    ]);
    addTearDown(container.dispose);
    return (adapter, container);
  }

  Future<void> pump(
    WidgetTester tester,
    ProviderContainer container,
    Widget screen,
  ) async {
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: MaterialApp(home: Scaffold(body: screen)),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('RoutinesScreen makes no request and shows no error while the '
      'gate is blocked', (tester) async {
    final (adapter, container) = await harness(
      gateState: AuthGateState.loginNeeded,
    );
    await pump(tester, container, const RoutinesScreen());

    expect(adapter.paths, isEmpty,
        reason: 'the screen is built at app start behind the gate — it must '
            'wait, not burn its single attempt on a guaranteed 401');
    expect(
      find.textContaining(L10n.t('routines_load_error').split(r'${e}').first),
      findsNothing,
      reason: 'a 401 behind the gate is not a routines failure, and nothing '
          'would ever clear this message',
    );
  });

  testWidgets('RoutinesScreen does load once the gate is open', (tester) async {
    final (adapter, container) = await harness(gateState: AuthGateState.ok);
    await pump(tester, container, const RoutinesScreen());

    expect(adapter.paths, isNotEmpty,
        reason: 'an open gate must not suppress the real request');
  });

  testWidgets('CalendarScreen makes no request while the gate is blocked',
      (tester) async {
    final (adapter, container) = await harness(
      gateState: AuthGateState.loginNeeded,
    );
    await pump(tester, container, const CalendarScreen());

    expect(adapter.paths, isEmpty);
  });
}
