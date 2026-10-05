import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/dev_gateway.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/beta_features_tab.dart';
import 'package:memo_flutter/widgets/settings/tabs/claude_subscription_panel.dart';

/// Claude Subscription is a Beta feature and its controls live inside the Beta
/// tab: present exactly while Beta is on. With Beta off the row still names the
/// feature (this page promises to list everything the switch unlocks) but
/// offers nothing to click — the backend would refuse it anyway.
class _FakeClaudeAccountNotifier extends ClaudeAccountNotifier {
  @override
  Future<ClaudeAccountState> build() async => const ClaudeAccountState();
}

Future<void> _pump(WidgetTester tester, {required bool beta}) async {
  L10n.setLocale(MemoLocale.en);
  tester.view.physicalSize = const Size(1000, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        remoteAccessProvider.overrideWith((ref) async => {'beta': beta}),
        claudeAccountProvider.overrideWith(_FakeClaudeAccountNotifier.new),
        claudeSubModelsProvider.overrideWith((ref) async => const <String>[]),
      ],
      child: const MaterialApp(home: Scaffold(body: BetaFeaturesTab())),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('Beta off: Claude Subscription is listed but has no controls', (tester) async {
    await _pump(tester, beta: false);
    expect(find.text(L10n.t('beta_item_claude_sub_title')), findsOneWidget);
    expect(find.byType(ClaudeSubscriptionPanel), findsNothing);
    expect(find.text(L10n.t('claude_account_connect_cta')), findsNothing);
  });

  testWidgets('Beta on: the Claude Subscription panel appears in the Beta tab', (tester) async {
    await _pump(tester, beta: true);
    expect(find.byType(ClaudeSubscriptionPanel), findsOneWidget);
    expect(find.text(L10n.t('claude_account_connect_cta')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('Beta on, Turkish, phone width: no overflow', (tester) async {
    L10n.setLocale(MemoLocale.tr);
    tester.view.physicalSize = const Size(360 * 3, 1400 * 3);
    tester.view.devicePixelRatio = 3.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
          remoteAccessProvider.overrideWith((ref) async => {'beta': true}),
          claudeAccountProvider.overrideWith(_FakeClaudeAccountNotifier.new),
          claudeSubModelsProvider.overrideWith((ref) async => const <String>[]),
        ],
        child: const MaterialApp(home: Scaffold(body: BetaFeaturesTab())),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(ClaudeSubscriptionPanel), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
