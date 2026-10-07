import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/widgets/quota_badge.dart';

final _now = DateTime.utc(2026, 10, 7, 12, 0, 0);

void main() {
  group('formatRemaining', () {
    test('two largest units, in English', () {
      L10n.setLocale(MemoLocale.en);
      String f(String t) => QuotaBadge.formatRemaining(t, now: _now);
      expect(f('2026-10-13T14:00:00Z'), '6d 2h');
      expect(f('2026-10-13T12:00:00Z'), '6d');
      expect(f('2026-10-07T15:20:00Z'), '3h 20m');
      expect(f('2026-10-07T15:00:00Z'), '3h');
      expect(f('2026-10-07T12:12:00Z'), '12m');
      expect(f('2026-10-07T12:00:20Z'), '1m', reason: 'a few seconds left is "1m", never "0m"');
    });

    test('Turkish units', () {
      L10n.setLocale(MemoLocale.tr);
      expect(QuotaBadge.formatRemaining('2026-10-13T14:00:00Z', now: _now), '6g 2sa');
      expect(QuotaBadge.formatRemaining('2026-10-07T12:12:00Z', now: _now), '12dk');
    });

    test('a moment already past says it is refilling; garbage says nothing', () {
      L10n.setLocale(MemoLocale.en);
      expect(QuotaBadge.formatRemaining('2026-10-07T11:59:00Z', now: _now), L10n.t('quota_resets_now'));
      expect(QuotaBadge.formatRemaining('', now: _now), '');
      expect(QuotaBadge.formatRemaining('tomorrow-ish', now: _now), '');
    });
  });

  Future<void> pump(WidgetTester tester, ProviderModel m, {bool showReset = true}) async {
    L10n.setLocale(MemoLocale.en);
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(extensions: [MemoTheme.dark]),
      home: Scaffold(body: Center(child: QuotaBadge(model: m, now: _now, showReset: showReset))),
    ));
  }

  testWidgets('shows the percentage and, beside it, how long until the refill', (tester) async {
    await pump(tester, const ProviderModel(id: 'x', remaining: 0.34, resetAt: '2026-10-13T14:00:00Z'));
    expect(find.text('34%'), findsOneWidget);
    expect(find.text('6d 2h'), findsOneWidget);
  });

  testWidgets('no reset time reported: the percentage stands alone', (tester) async {
    await pump(tester, const ProviderModel(id: 'x', remaining: 0.5));
    expect(find.text('50%'), findsOneWidget);
    expect(find.byIcon(Icons.schedule), findsNothing);
  });

  testWidgets('showReset: false hides the countdown', (tester) async {
    await pump(tester, const ProviderModel(id: 'x', remaining: 0.5, resetAt: '2026-10-13T14:00:00Z'), showReset: false);
    expect(find.text('6d 2h'), findsNothing);
  });

  testWidgets('no figure at all renders nothing', (tester) async {
    await pump(tester, const ProviderModel(id: 'x'));
    expect(find.byType(Tooltip), findsNothing);
  });

  testWidgets('the tooltip names the window and the exact refill time', (tester) async {
    await pump(tester, const ProviderModel(id: 'x', remaining: 0.34, resetAt: '2026-10-13T14:00:00Z', quotaWindow: '7d'));
    final tip = tester.widget<Tooltip>(find.byType(Tooltip)).message!;
    expect(tip, contains('7d window'));
    expect(tip, contains('34%'));
    expect(tip, contains(L10n.t('subs_quota_resets', {'when': QuotaBadge.formatReset('2026-10-13T14:00:00Z')})));
  });
}
