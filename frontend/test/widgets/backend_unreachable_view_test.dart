import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/foundation.dart'
    show TargetPlatform, debugDefaultTargetPlatformOverride;
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/scope_restart.dart';
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
          isBackendUnreachableError(
            DioException(requestOptions: req, type: type),
          ),
          isTrue,
          reason: '$type should count as unreachable',
        );
      }
    });

    test('false for a real response the backend sent back', () {
      final req = RequestOptions(path: '/x');
      expect(
        isBackendUnreachableError(
          DioException(requestOptions: req, type: DioExceptionType.badResponse),
        ),
        isFalse,
      );
    });

    test('false for a non-Dio error', () {
      expect(
        isBackendUnreachableError(Exception('some other failure')),
        isFalse,
      );
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

    testWidgets(
      'shows the configured URL in plain language, never a raw exception dump',
      (tester) async {
        await pump(tester);

        expect(find.textContaining('192.168.1.106:8090'), findsOneWidget);
        expect(find.textContaining('DioException'), findsNothing);
        expect(find.textContaining('SocketException'), findsNothing);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets(
      'tapping "Sunucuyu Değiştir" opens an inline change-server dialog, not Settings',
      (tester) async {
        await pump(tester);

        await tester.tap(
          find.text(L10n.t('backend_unreachable_change_server')),
        );
        await tester.pumpAndSettle();

        // Dialog title text is identical to the button's own label ("Sunucuyu
        // Değiştir" both places) — assert via the dialog type itself instead.
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text(L10n.t('remote_backend_url_field_label')),
          findsOneWidget,
        );
        // The full Settings dialog (20 tabs) must not be what opened.
        expect(find.text(L10n.t('tab_gpu_config')), findsNothing);
      },
    );

    testWidgets(
      'desktop address changes reconnect in place without closing the app',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.linux;
        addTearDown(() => debugDefaultTargetPlatformOverride = null);
        await pump(tester);
        final before = scopeEpoch.value;
        await tester.tap(
          find.text(L10n.t('backend_unreachable_change_server')),
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.widgetWithText(
            TextField,
            L10n.t('remote_backend_url_field_label'),
          ),
          'http://10.0.0.5:8090',
        );
        await tester.tap(find.text(L10n.t('apply')));
        await tester.pumpAndSettle();
        expect(find.byType(RestartRequiredDialog), findsNothing);
        expect(scopeEpoch.value, before + 1);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(BackendUnreachableView)),
        );
        expect(container.read(backendUrlProvider), 'http://10.0.0.5:8090');
        debugDefaultTargetPlatformOverride = null;
      },
    );

    testWidgets('change-server remains reachable in a short window', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(500, 280);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      await pump(tester);
      expect(tester.takeException(), isNull);
      final button = find.text(L10n.t('backend_unreachable_change_server'));
      await tester.ensureVisible(button);
      await tester.tap(button);
      await tester.pumpAndSettle();
      expect(find.byType(ChangeServerDialog), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'typing a bare host with no scheme does not crash the app (regression, bug3)',
      (tester) async {
        debugDefaultTargetPlatformOverride = TargetPlatform.linux;
        await pump(tester);
        await tester.tap(
          find.text(L10n.t('backend_unreachable_change_server')),
        );
        await tester.pumpAndSettle();

        // The exact reported input: no "http://", no port. Before
        // normalizeBackendUrl existed, MemoApiClient's Dio construction threw
        // "Invalid argument (baseUrl): Must be a valid URL on platforms other
        // than Web." synchronously and uncaught, crashing the whole widget
        // tree with Flutter's red error screen.
        await tester.enterText(
          find.widgetWithText(
            TextField,
            L10n.t('remote_backend_url_field_label'),
          ),
          '127.0.0.1',
        );
        await tester.tap(find.text(L10n.t('apply')));
        await tester.pumpAndSettle();
        debugDefaultTargetPlatformOverride = null;

        expect(tester.takeException(), isNull);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(BackendUnreachableView)),
        );
        expect(container.read(backendUrlProvider), 'http://127.0.0.1:8090');
      },
    );

    testWidgets(
      'on a phone, applying a new address rebuilds the provider scope instead of offering a restart',
      (tester) async {
        // flutter_test's default platform is android, i.e. mobile.
        await pump(tester);
        final before = scopeEpoch.value;
        await tester.tap(
          find.text(L10n.t('backend_unreachable_change_server')),
        );
        await tester.pumpAndSettle();

        await tester.enterText(
          find.widgetWithText(
            TextField,
            L10n.t('remote_backend_url_field_label'),
          ),
          'memo.example.com:8540',
        );
        await tester.tap(find.text(L10n.t('apply')));
        await tester.pumpAndSettle();

        expect(
          find.text(L10n.t('restart_required_title')),
          findsNothing,
          reason: 'exit(0) on a phone only closes the app on the user',
        );
        expect(scopeEpoch.value, before + 1);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(BackendUnreachableView)),
        );
        expect(
          container.read(backendUrlProvider),
          'http://memo.example.com:8540',
        );
      },
    );

    testWidgets(
      '"Bu Bilgisayarın Backend\'ine Dön" resets both fields to the local default',
      (tester) async {
        // Desktop-only behaviour by definition: this button is hidden on
        // mobile, where "this computer's backend" cannot exist. flutter_test
        // reports android by default, so pin the platform explicitly — and
        // clear it again inside the body rather than in addTearDown, because
        // testWidgets asserts every foundation debug variable is back to null
        // before its own tear-downs run.
        debugDefaultTargetPlatformOverride = TargetPlatform.linux;
        await pump(tester);
        await tester.tap(
          find.text(L10n.t('backend_unreachable_change_server')),
        );
        await tester.pumpAndSettle();

        await tester.enterText(
          find.widgetWithText(
            TextField,
            L10n.t('remote_backend_url_field_label'),
          ),
          '192.168.1.106:1234',
        );
        await tester.tap(find.text(L10n.t('reset_to_local_backend')));
        await tester.pumpAndSettle();
        debugDefaultTargetPlatformOverride = null;

        expect(find.byType(RestartRequiredDialog), findsNothing);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(MaterialApp)),
        );
        expect(container.read(backendUrlProvider), 'http://127.0.0.1:8090');
        expect(container.read(backendTokenProvider), '');
      },
    );
  });

  group('BackendUnreachableOverlay', () {
    Future<ProviderContainer> pumpApp(
      WidgetTester tester, {
      required bool? connected,
      Stream<AuthGateInfo>? gate,
      bool setupComplete = true,
    }) async {
      // Setup marked complete because the overlay deliberately stands down
      // during first run — the setup wizard owns the screen then, and on
      // mobile it is the only place a server address can be entered at all.
      SharedPreferences.setMockInitialValues({
        'memo_setup_complete': setupComplete,
      });
      final prefs = await SharedPreferences.getInstance();
      final container = ProviderContainer(
        overrides: [
          prefsProvider.overrideWithValue(prefs),
          apiClientProvider.overrideWithValue(
            MemoApiClient(baseUrl: 'http://127.0.0.1:8090'),
          ),
          // The overlay stands down during first-run setup (the wizard owns the
          // screen then), so it reads this provider — whose notifier otherwise
          // confirms the flag against GET /api/onboarding on construction.
          // Inside testWidgets' fake-async zone Dio's timeout timers never fire
          // and flutter_test fails the test for leaking them, so take the
          // prefs-only path; the mocked flag above is what these tests assert
          // against anyway.
          setupCompleteProvider.overrideWith(
            (ref) =>
                SetupCompleteNotifier(ref, prefs, confirmWithBackend: false),
          ),
          connectionStatusProvider.overrideWith(
            (ref) => connected == null
                ? const Stream.empty()
                : Stream.value(connected),
          ),
          // The overlay now consults the auth gate too; as of 2026-08 make the
          // gate report the stream the test wants — null/empty simulates the
          // gate's stream being rebuilt mid-flight (e.g. right after login,
          // when invalidate() restarts it and nothing has been yielded yet).
          authGateProvider.overrideWith((ref) => gate ?? const Stream.empty()),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(
            home: Scaffold(
              body: Stack(children: [BackendUnreachableOverlay()]),
            ),
          ),
        ),
      );
      await tester.pump();
      // Let the overridden single-value streams (connection status, auth
      // gate) actually deliver their value into the widget's watches.
      await tester.pump();
      return container;
    }

    testWidgets(
      'renders nothing while still loading the first connectivity check',
      (tester) async {
        await pumpApp(tester, connected: null);
        expect(find.byType(BackendUnreachableView), findsNothing);
      },
    );

    testWidgets('renders nothing once connected', (tester) async {
      await pumpApp(tester, connected: true);
      expect(find.byType(BackendUnreachableView), findsNothing);
    });

    testWidgets('covers the screen once the backend is confirmed unreachable', (
      tester,
    ) async {
      await pumpApp(
        tester,
        connected: false,
        gate: Stream.value(const AuthGateInfo(AuthGateState.ok)),
      );
      expect(find.byType(BackendUnreachableView), findsOneWidget);
    });

    // A failed onboarding check must not hide address recovery on desktop.
    testWidgets(
      'desktop can recover a dead saved address before onboarding completes',
      (tester) async {
        // Same inputs as the "covers the screen" case above — confirmed
        // unreachable AND a healthy auth gate — so the setup flag is the only
        // thing that can suppress the overlay here. Without it the test would
        // pass for the wrong reason: a null gate already short-circuits.
        debugDefaultTargetPlatformOverride = TargetPlatform.linux;
        await pumpApp(
          tester,
          connected: false,
          gate: Stream.value(const AuthGateInfo(AuthGateState.ok)),
          setupComplete: false,
        );
        debugDefaultTargetPlatformOverride = null;
        expect(find.byType(BackendUnreachableView), findsOneWidget);
      },
    );

    // On a phone the address is entered on the connect screen before this can
    // matter, and the wizard stays hidden until the server answers — so an
    // unfinished-setup flag must not hide the only screen that can fix a
    // server that stopped answering.
    testWidgets(
      'on a phone it still covers the screen when setup is not marked complete',
      (tester) async {
        await pumpApp(
          tester,
          connected: false,
          gate: Stream.value(const AuthGateInfo(AuthGateState.ok)),
          setupComplete: false,
        );
        expect(find.byType(BackendUnreachableView), findsOneWidget);
      },
    );

    testWidgets(
      'does not flash while the auth gate is rebuilding after login (BUG-ONB3)',
      (tester) async {
        // connected:false (stale pre-login isAlive result) + gate stream
        // restarting (invalidate() right after login) is exactly the window
        // where the RPi showed "Sunucuya bağlanılamıyor" for seconds.
        await pumpApp(tester, connected: false, gate: const Stream.empty());
        expect(find.byType(BackendUnreachableView), findsNothing);
      },
    );
  });
}
