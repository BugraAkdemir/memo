import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/models/subscriptions.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/subscriptions_tab.dart';

/// Serves a fixed state and records what the tab asks the notifier to do, so
/// the tab's behaviour can be asserted without a backend.
class _FakeSubscriptions extends SubscriptionsNotifier {
  _FakeSubscriptions(this.initial, this.log);
  final SubscriptionsState initial;
  final List<String> log;

  @override
  Future<SubscriptionsState> build() async => initial;

  @override
  Future<void> reload() async {}

  @override
  Future<void> logout(String provider) async {
    log.add('logout:$provider');
  }

  @override
  Future<void> cancelLogin() async {
    log.add('cancel');
  }
}

const _bundled = SubscriptionsState(
  bundled: true,
  version: 'v8.0.16',
  running: true,
  providers: ['antigravity', 'claude', 'codex'],
);

Future<void> _pump(
  WidgetTester tester,
  SubscriptionsState state, {
  List<String>? log,
  MemoLocale locale = MemoLocale.en,
  Size size = const Size(1000, 1800),
}) async {
  L10n.setLocale(locale);
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        subscriptionsProvider.overrideWith(() => _FakeSubscriptions(state, log ?? [])),
      ],
      child: const MaterialApp(home: Scaffold(body: SubscriptionsTab())),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a build without the sidecar says so and offers no sign-in', (tester) async {
    await _pump(tester, const SubscriptionsState(bundled: false, problem: 'no sidecar here'));
    expect(find.text('no sidecar here'), findsOneWidget);
    expect(find.text(L10n.t('subs_sign_in')), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('a build without the sidecar and no message falls back to our own text', (tester) async {
    await _pump(tester, const SubscriptionsState(bundled: false));
    expect(find.text(L10n.t('subs_not_bundled')), findsOneWidget);
  });

  testWidgets('lists the three vendors with a sign-in each when nobody is signed in', (tester) async {
    await _pump(tester, _bundled);
    for (final v in ['Antigravity', 'Claude', 'Codex']) {
      expect(find.text(v), findsOneWidget);
    }
    expect(find.text(L10n.t('subs_sign_in')), findsNWidgets(3));
    expect(find.text(L10n.t('subs_not_signed_in')), findsNWidgets(3));
    expect(find.text(L10n.t('subs_sign_out')), findsNothing);
    expect(find.text(L10n.t('subs_models_none')), findsOneWidget);
    expect(find.textContaining('v8.0.16'), findsOneWidget);
  });

  testWidgets('a signed-in account shows its email and signs out through the notifier', (tester) async {
    final log = <String>[];
    await _pump(
      tester,
      const SubscriptionsState(
        bundled: true,
        version: 'v8.0.16',
        running: true,
        providers: ['antigravity', 'claude', 'codex'],
        accounts: [SubscriptionAccount(provider: 'antigravity', email: 'me@example.com')],
        models: [
          ProviderModel(id: 'claude-sonnet-4-6', ownedBy: 'antigravity'),
          ProviderModel(id: 'gemini-3-flash', ownedBy: 'antigravity'),
        ],
        model: 'claude-sonnet-4-6',
      ),
      log: log,
    );

    expect(find.text(L10n.t('subs_signed_in_as', {'email': 'me@example.com'})), findsOneWidget);
    // Only the signed-in vendor offers sign-out; the other two still offer sign-in.
    expect(find.text(L10n.t('subs_sign_out')), findsOneWidget);
    expect(find.text(L10n.t('subs_sign_in')), findsNWidgets(2));
    expect(find.text('claude-sonnet-4-6'), findsOneWidget);
    expect(find.text(L10n.t('subs_models_count', {'n': '2'})), findsOneWidget);

    await tester.tap(find.text(L10n.t('subs_sign_out')));
    await tester.pumpAndSettle();
    expect(log, ['logout:antigravity']);
  });

  testWidgets('shows the risk note', (tester) async {
    await _pump(tester, _bundled);
    expect(find.text(L10n.t('subs_risk_note')), findsOneWidget);
  });

  testWidgets('Turkish at phone width: no overflow', (tester) async {
    await _pump(
      tester,
      const SubscriptionsState(
        bundled: true,
        version: 'v8.0.16',
        providers: ['antigravity', 'claude', 'codex'],
        accounts: [SubscriptionAccount(provider: 'claude', email: 'a.very.long.address.indeed@example.com')],
        models: [ProviderModel(id: 'claude-opus-4-6-thinking', ownedBy: 'claude')],
      ),
      locale: MemoLocale.tr,
      size: const Size(360, 1400),
    );
    expect(tester.takeException(), isNull);
  });
}
