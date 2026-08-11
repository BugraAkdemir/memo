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
import 'package:memo_flutter/widgets/backend_unreachable_view.dart';

void main() {
  group('isBackendUnreachableError', () {
    test('true for connection-level DioException types', () {
      final req = RequestOptions(path: '/x');
      for (final type in [
        DioExceptionType.connectionError,
        DioExceptionType.connectionTimeout,
        DioExceptionType.receiveTimeout,
        DioExceptionType.sendTimeout,
      ]) {
        expect(
          isBackendUnreachableError(DioException(requestOptions: req, type: type)),
          isTrue,
          reason: '$type should count as unreachable',
        );
      }
    });

    test('false for a real response the backend sent back', () {
      final req = RequestOptions(path: '/x');
      expect(
        isBackendUnreachableError(DioException(
          requestOptions: req,
          type: DioExceptionType.badResponse,
        )),
        isFalse,
      );
    });

    test('false for a non-Dio error', () {
      expect(isBackendUnreachableError(Exception('some other failure')), isFalse);
    });
  });

  group('BackendUnreachableView', () {
    Future<void> pump(WidgetTester tester) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      L10n.setLocale(MemoLocale.tr);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            prefsProvider.overrideWithValue(prefs),
            apiClientProvider.overrideWithValue(
              MemoApiClient(baseUrl: 'http://192.168.1.106:8090'),
            ),
          ],
          child: const MaterialApp(
            home: Scaffold(body: BackendUnreachableView()),
          ),
        ),
      );
      await tester.pump();
    }

    testWidgets('shows the configured URL in plain language, never a raw exception dump',
        (tester) async {
      await pump(tester);

      expect(find.textContaining('192.168.1.106:8090'), findsOneWidget);
      expect(find.textContaining('DioException'), findsNothing);
      expect(find.textContaining('SocketException'), findsNothing);
      expect(tester.takeException(), isNull);
    });

    testWidgets('tapping "Sunucuyu Değiştir" opens an inline change-server dialog, not Settings',
        (tester) async {
      await pump(tester);

      await tester.tap(find.text(L10n.t('backend_unreachable_change_server')));
      await tester.pumpAndSettle();

      // Dialog title text is identical to the button's own label ("Sunucuyu
      // Değiştir" both places) — assert via the dialog type itself instead.
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text(L10n.t('remote_backend_url_field_label')), findsOneWidget);
      // The full Settings dialog (20 tabs) must not be what opened.
      expect(find.text(L10n.t('tab_gpu_config')), findsNothing);
    });

    testWidgets('applying a new server address shows the restart-required dialog with a live countdown',
        (tester) async {
      await pump(tester);
      await tester.tap(find.text(L10n.t('backend_unreachable_change_server')));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextField, L10n.t('remote_backend_url_field_label')),
        'http://10.0.0.5:8090',
      );
      await tester.tap(find.text(L10n.t('apply')));
      await tester.pumpAndSettle();

      expect(find.text(L10n.t('restart_required_title')), findsOneWidget);
      expect(find.text(L10n.t('restart_in_seconds', {'s': '10'})), findsOneWidget);

      // Advance one tick to confirm the countdown is actually live — but
      // never far enough to let it reach 0, since that path calls exit(0)
      // and would kill the test process itself.
      await tester.pump(const Duration(seconds: 1));
      expect(find.text(L10n.t('restart_in_seconds', {'s': '9'})), findsOneWidget);

      // Persisted so RemoteAccessTab (read via the same providers) agrees.
      final container = ProviderScope.containerOf(
        tester.element(find.byType(BackendUnreachableView)),
      );
      expect(container.read(backendUrlProvider), 'http://10.0.0.5:8090');
    });

    testWidgets('typing a bare host with no scheme does not crash the app (regression, bug3)',
        (tester) async {
      await pump(tester);
      await tester.tap(find.text(L10n.t('backend_unreachable_change_server')));
      await tester.pumpAndSettle();

      // The exact reported input: no "http://", no port. Before
      // normalizeBackendUrl existed, MemoApiClient's Dio construction threw
      // "Invalid argument (baseUrl): Must be a valid URL on platforms other
      // than Web." synchronously and uncaught, crashing the whole widget
      // tree with Flutter's red error screen.
      await tester.enterText(
        find.widgetWithText(TextField, L10n.t('remote_backend_url_field_label')),
        '127.0.0.1',
      );
      await tester.tap(find.text(L10n.t('apply')));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      final container = ProviderScope.containerOf(
        tester.element(find.byType(BackendUnreachableView)),
      );
      expect(container.read(backendUrlProvider), 'http://127.0.0.1:8090');
    });

    testWidgets('"Bu Bilgisayarın Backend\'ine Dön" resets both fields to the local default',
        (tester) async {
      await pump(tester);
      await tester.tap(find.text(L10n.t('backend_unreachable_change_server')));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextField, L10n.t('remote_backend_url_field_label')),
        '192.168.1.106:1234',
      );
      await tester.tap(find.text(L10n.t('reset_to_local_backend')));
      await tester.pumpAndSettle();

      expect(find.text(L10n.t('restart_required_title')), findsOneWidget);
      final container = ProviderScope.containerOf(tester.element(find.byType(MaterialApp)));
      expect(container.read(backendUrlProvider), 'http://127.0.0.1:8090');
      expect(container.read(backendTokenProvider), '');
    });
  });

  group('BackendUnreachableOverlay', () {
    Future<ProviderContainer> pumpApp(WidgetTester tester, {required bool? connected}) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final container = ProviderContainer(overrides: [
        prefsProvider.overrideWithValue(prefs),
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:8090')),
        connectionStatusProvider.overrideWith(
          (ref) => connected == null ? const Stream.empty() : Stream.value(connected),
        ),
        // The overlay now consults the auth gate too; as of 2026-08 make the
        // gate permanently "ok" so it can't veto the unreachable screen.
        authGateProvider.overrideWith(
          (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
        ),
      ]);
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(
            home: Scaffold(body: Stack(children: [BackendUnreachableOverlay()])),
          ),
        ),
      );
      await tester.pump();
      return container;
    }

    testWidgets('renders nothing while still loading the first connectivity check',
        (tester) async {
      await pumpApp(tester, connected: null);
      expect(find.byType(BackendUnreachableView), findsNothing);
    });

    testWidgets('renders nothing once connected', (tester) async {
      await pumpApp(tester, connected: true);
      expect(find.byType(BackendUnreachableView), findsNothing);
    });

    testWidgets('covers the screen once the backend is confirmed unreachable', (tester) async {
      await pumpApp(tester, connected: false);
      expect(find.byType(BackendUnreachableView), findsOneWidget);
    });
  });
}
