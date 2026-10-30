import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/screens/mail_screen.dart';

class _MailAdapter implements HttpClientAdapter {
  int count = 1;
  int statsRequests = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? request,
    Future<void>? cancel,
  ) async {
    if (options.path == '/api/mail/stats') {
      statsRequests++;
      return ResponseBody.fromString(
        '{"calendar_extracted":$count}',
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString(
      '{"accounts":[]}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  testWidgets('mail counter refreshes only while visible and resumed', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1600, 1000);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final adapter = _MailAdapter();
    final api = MemoApiClient(baseUrl: 'http://memo.test');
    api.dio.httpClientAdapter = adapter;
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(api),
        authGateProvider.overrideWith(
          (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
        ),
      ],
    );
    addTearDown(container.dispose);
    final gate = container.listen(authGateProvider, (_, next) {});
    addTearDown(gate.close);
    await container.read(authGateProvider.future);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    Future<void> mount(bool active) => tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(body: MailScreen(isActive: active)),
        ),
      ),
    );
    await mount(true);
    await tester.pumpAndSettle();
    expect(
      find.text(L10n.t('mail_stats_calendar', {'n': '1'})),
      findsOneWidget,
    );
    adapter.count = 2;
    await tester.pump(const Duration(seconds: 15));
    await tester.pumpAndSettle();
    expect(
      find.text(L10n.t('mail_stats_calendar', {'n': '2'})),
      findsOneWidget,
    );
    await mount(false);
    await tester.pumpAndSettle();
    final beforeHidden = adapter.statsRequests;
    await tester.pump(const Duration(seconds: 30));
    expect(adapter.statsRequests, beforeHidden);
    await mount(true);
    await tester.pumpAndSettle();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    final beforePaused = adapter.statsRequests;
    await tester.pump(const Duration(seconds: 30));
    expect(adapter.statsRequests, beforePaused);
    adapter.count = 3;
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pumpAndSettle();
    expect(
      find.text(L10n.t('mail_stats_calendar', {'n': '3'})),
      findsOneWidget,
    );
    await tester.pumpWidget(const SizedBox.shrink());
  });
}
