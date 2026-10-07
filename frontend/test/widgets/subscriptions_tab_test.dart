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
  Future<void> reload() async {
    reloads++;
  }

  int reloads = 0;

  @override
  Future<void> logout(String provider) async {
    log.add('logout:$provider');
  }

  @override
  Future<void> cancelLogin() async {
    log.add('cancel');
  }
}

/// Starts with the account but no models (the sidecar serves none for ~30 s after
/// a sign-in) and "receives" them on the first reload.
class _LateModels extends SubscriptionsNotifier {
  int reloads = 0;
  @override
  Future<SubscriptionsState> build() async => const SubscriptionsState(
        bundled: true,
        running: true,
        providers: ['antigravity', 'claude', 'codex'],
        accounts: [SubscriptionAccount(provider: 'antigravity', email: 'me@example.com')],
      );

  @override
  Future<void> reload() async {
    reloads++;
    state = const AsyncValue.data(SubscriptionsState(
      bundled: true,
      running: true,
      providers: ['antigravity', 'claude', 'codex'],
      accounts: [SubscriptionAccount(provider: 'antigravity', email: 'me@example.com')],
      models: [ProviderModel(id: 'claude-sonnet-4-6', ownedBy: 'antigravity', remaining: 0.25)],
    ));
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

  // Signing in restarts the sidecar and it answers /v1/models with nothing for
  // ~30 s. The tab used to read that empty list once and keep showing "no models
  // yet" (and the model menus kept their cached, Subscriptions-less list) until
  // the app was reopened.
  testWidgets('keeps asking while an account has no models yet, then shows them', (tester) async {
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(1000, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final notifier = _LateModels();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
          subscriptionsProvider.overrideWith(() => notifier),
        ],
        child: const MaterialApp(home: Scaffold(body: SubscriptionsTab())),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(find.text(L10n.t('subs_models_none')), findsOneWidget);
    expect(find.text('claude-sonnet-4-6'), findsNothing);

    await tester.pump(const Duration(seconds: 3));
    await tester.pump();
    expect(notifier.reloads, 1);
    expect(find.text('claude-sonnet-4-6'), findsOneWidget);
    expect(find.text('25%'), findsOneWidget, reason: 'the remaining allowance is shown beside the model');

    // Once the models are here it stops asking.
    await tester.pump(const Duration(seconds: 9));
    expect(notifier.reloads, 1);
    expect(tester.takeException(), isNull);
  });

  // The allowance left changes as it is used: while the tab is open it re-reads
  // every 50 seconds. Signed out, there is nothing to measure and it stays quiet.
  testWidgets('re-reads every 50 seconds while an account is signed in', (tester) async {
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(1000, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final fake = _FakeSubscriptions(
      const SubscriptionsState(
        bundled: true,
        running: true,
        providers: ['antigravity', 'claude', 'codex'],
        accounts: [SubscriptionAccount(provider: 'antigravity', email: 'me@example.com')],
        models: [ProviderModel(id: 'claude-sonnet-4-6', ownedBy: 'antigravity', remaining: 0.5)],
      ),
      [],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
          subscriptionsProvider.overrideWith(() => fake),
        ],
        child: const MaterialApp(home: Scaffold(body: SubscriptionsTab())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(seconds: 49));
    expect(fake.reloads, 0);
    await tester.pump(const Duration(seconds: 1));
    expect(fake.reloads, 1);
    await tester.pump(const Duration(seconds: 50));
    expect(fake.reloads, 2);
  });

  testWidgets('signed out, the quota timer has nothing to refresh', (tester) async {
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(1000, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final fake = _FakeSubscriptions(_bundled, []);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
          subscriptionsProvider.overrideWith(() => fake),
        ],
        child: const MaterialApp(home: Scaffold(body: SubscriptionsTab())),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(seconds: 120));
    expect(fake.reloads, 0);
  });
}
