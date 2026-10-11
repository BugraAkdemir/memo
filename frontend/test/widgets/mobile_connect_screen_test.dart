import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/scope_restart.dart';
import 'package:memo_flutter/core/server_probe.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/mobile_connect_screen.dart';

SetupStatus _status({String mode = 'password', bool needsSetup = false}) =>
    SetupStatus(needsSetup: needsSetup, authMode: mode, loopback: false);

void main() {
  group('shouldShowMobileConnect', () {
    test('only a phone with no saved address', () {
      expect(shouldShowMobileConnect(isMobile: true, savedUrl: ''), isTrue);
      expect(shouldShowMobileConnect(isMobile: true, savedUrl: '  '), isTrue);
      expect(
          shouldShowMobileConnect(
              isMobile: true, savedUrl: 'http://192.168.1.5:8090'),
          isFalse);
    });

    test('never on desktop or web, whatever is saved', () {
      expect(shouldShowMobileConnect(isMobile: false, savedUrl: ''), isFalse);
      expect(
          shouldShowMobileConnect(isMobile: false, savedUrl: 'http://x:8090'),
          isFalse);
    });
  });

  group('MobileConnectScreen', () {
    late List<String> probed;

    Future<ProviderContainer> pump(
      WidgetTester tester,
      Future<ServerProbeResult> Function(String url) probe,
    ) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      L10n.setLocale(MemoLocale.en);
      probed = [];
      await tester.pumpWidget(ProviderScope(
        overrides: [prefsProvider.overrideWithValue(prefs)],
        child: MaterialApp(
          home: MobileConnectScreen(probe: (url) {
            probed.add(url);
            return probe(url);
          }),
        ),
      ));
      return ProviderScope.containerOf(
          tester.element(find.byType(MobileConnectScreen)));
    }

    Future<void> type(WidgetTester tester, String text) async {
      await tester.enterText(find.byType(TextField), text);
      await tester.pump();
    }

    // On success the screen stays in its "checking" state (the real app
    // replaces it by rebuilding the scope), so its spinner never settles.
    Future<void> next(WidgetTester tester) async {
      await tester.tap(find.text(L10n.t('next')));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
    }

    testWidgets('shows the resolved address while typing: bare domain gets 8090',
        (tester) async {
      await pump(tester, (_) async => ServerProbeResult.ok(_status()));
      await type(tester, 'domain.com');
      expect(find.text(L10n.t('connect_will_use', {'url': 'http://domain.com:8090'})),
          findsOneWidget);
    });

    testWidgets('a port the user typed is the one used', (tester) async {
      await pump(tester, (_) async => ServerProbeResult.ok(_status()));
      await type(tester, 'domain.com:8540');
      expect(find.text(L10n.t('connect_will_use', {'url': 'http://domain.com:8540'})),
          findsOneWidget);
    });

    testWidgets('Next is disabled until something is typed', (tester) async {
      await pump(tester, (_) async => ServerProbeResult.ok(_status()));
      final button = find.widgetWithText(ElevatedButton, L10n.t('next'));
      expect(tester.widget<ElevatedButton>(button).onPressed, isNull);
      await type(tester, '192.168.1.50');
      expect(tester.widget<ElevatedButton>(button).onPressed, isNotNull);
    });

    testWidgets('a reachable server saves the address and rebuilds the scope',
        (tester) async {
      final container =
          await pump(tester, (_) async => ServerProbeResult.ok(_status()));
      final before = scopeEpoch.value;
      await type(tester, '192.168.1.50');
      await next(tester);

      expect(probed, ['http://192.168.1.50:8090']);
      expect(container.read(backendUrlProvider), 'http://192.168.1.50:8090');
      expect(scopeEpoch.value, before + 1);
    });

    for (final mode in ['password', 'token', 'token_password', 'none']) {
      testWidgets('auth mode "$mode" proceeds the same way — login is the gate\'s job',
          (tester) async {
        final container = await pump(
            tester, (_) async => ServerProbeResult.ok(_status(mode: mode)));
        final before = scopeEpoch.value;
        await type(tester, 'memo.example.com:8540');
        await next(tester);
        expect(scopeEpoch.value, before + 1);
        expect(container.read(backendUrlProvider), 'http://memo.example.com:8540');
      });
    }

    testWidgets('a fresh, never-set-up server connects too (the gate then runs setup)',
        (tester) async {
      await pump(tester,
          (_) async => ServerProbeResult.ok(_status(needsSetup: true)));
      final before = scopeEpoch.value;
      await type(tester, '10.0.0.2');
      await next(tester);
      expect(scopeEpoch.value, before + 1);
    });

    testWidgets('a failure stays on the screen, names the reason and saves nothing',
        (tester) async {
      final container = await pump(
          tester,
          (_) async => const ServerProbeResult.failed(
              ServerProbeFailure.refused, 'Connection refused, errno = 111'));
      final before = scopeEpoch.value;
      await type(tester, '192.168.1.50');
      await tester.tap(find.text(L10n.t('next')));
      await tester.pumpAndSettle();

      expect(find.text(L10n.t('connect_fail_title')), findsOneWidget);
      expect(find.text(L10n.t('connect_fail_refused')), findsOneWidget);
      expect(find.textContaining('errno = 111'), findsOneWidget);
      expect(container.read(backendUrlProvider), '');
      expect(scopeEpoch.value, before);
      // The button now offers a retry instead of Next.
      expect(find.text(L10n.t('retry')), findsOneWidget);
    });

    testWidgets('editing the address clears the failure and a retry can succeed',
        (tester) async {
      var attempt = 0;
      final container = await pump(tester, (_) async {
        attempt++;
        return attempt == 1
            ? const ServerProbeResult.failed(ServerProbeFailure.dns, 'lookup')
            : ServerProbeResult.ok(_status());
      });
      await type(tester, 'typo.example');
      await tester.tap(find.text(L10n.t('next')));
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('connect_fail_dns')), findsOneWidget);

      await type(tester, 'memo.example');
      expect(find.text(L10n.t('connect_fail_dns')), findsNothing);
      await next(tester);
      expect(container.read(backendUrlProvider), 'http://memo.example:8090');
    });

    testWidgets('both languages carry every failure reason', (tester) async {
      for (final locale in [MemoLocale.tr, MemoLocale.en]) {
        L10n.setLocale(locale);
        for (final k in [
          'connect_title',
          'connect_subtitle',
          'connect_address_label',
          'connect_will_use',
          'connect_checking',
          'connect_fail_title',
          'connect_fail_dns',
          'connect_fail_refused',
          'connect_fail_timeout',
          'connect_fail_tls',
          'connect_fail_unreachable',
          'connect_fail_not_memo',
          'connect_fail_unknown',
        ]) {
          expect(L10n.t(k), isNot(k), reason: '$k missing in $locale');
        }
      }
      L10n.setLocale(MemoLocale.tr);
      final tr = L10n.t('connect_title');
      L10n.setLocale(MemoLocale.en);
      expect(L10n.t('connect_title'), isNot(tr),
          reason: 'the English entry must not be a copy of the Turkish one');
    });
  });
}
