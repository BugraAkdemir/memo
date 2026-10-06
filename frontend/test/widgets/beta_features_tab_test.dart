import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/beta_features_tab.dart';

/// The Beta tab lists what the switch unlocks. The per-vendor subscription
/// providers (Gemini, Claude) used to be listed here; they were replaced by the
/// stable Settings → Subscriptions tab and must not come back as Beta items.
Future<void> _pump(WidgetTester tester, {required bool beta, MemoLocale locale = MemoLocale.en, Size? size}) async {
  L10n.setLocale(locale);
  tester.view.physicalSize = size ?? const Size(1000, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        remoteAccessProvider.overrideWith((ref) async => {'beta': beta}),
      ],
      child: const MaterialApp(home: Scaffold(body: BetaFeaturesTab())),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  for (final beta in [false, true]) {
    testWidgets('Beta ${beta ? 'on' : 'off'}: lists Swarm and no subscription provider', (tester) async {
      await _pump(tester, beta: beta);
      expect(find.text(L10n.t('beta_item_swarm_title')), findsOneWidget);
      expect(find.textContaining('Gemini'), findsNothing);
      expect(find.textContaining('Claude'), findsNothing);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('Turkish at phone width: no overflow', (tester) async {
    await _pump(tester, beta: true, locale: MemoLocale.tr, size: const Size(360 * 3, 1400 * 3));
    expect(tester.takeException(), isNull);
  });
}
