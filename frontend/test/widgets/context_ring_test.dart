import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/models/context_report.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/context_provider.dart';
import 'package:memo_flutter/widgets/context_ring.dart';

class _FakeActiveChat extends ActiveChatIdNotifier {
  @override
  Future<String> build() async => 'c1';
}

ContextReport _report({
  int used = 325100,
  bool real = true,
  List<QuotaMeter> limits = const [],
  int summarized = 0,
  bool autoCompact = true,
}) =>
    ContextReport(
      provider: 'Subscriptions',
      model: 'gpt-5.6-luna',
      window: 1000000,
      used: used,
      usedReal: real,
      percent: used * 100 ~/ 1000000,
      categories: const [
        ContextCategory(key: 'messages', tokens: 282200),
        ContextCategory(key: 'tools', tokens: 17400),
        ContextCategory(key: 'system', tokens: 4700),
      ],
      autoCompactEnabled: autoCompact,
      autoCompactPct: 90,
      summarizedMessages: summarized,
      limits: limits,
      limitsVendor: limits.isEmpty ? '' : 'openai',
    );

Future<void> _pumpView(WidgetTester tester, ContextReport r) async {
  await tester.pumpWidget(MaterialApp(
    theme: ThemeData(extensions: [MemoTheme.dark]),
    home: Scaffold(
      body: SingleChildScrollView(
        child: ContextReportView(report: r, now: DateTime.utc(2026, 10, 8, 12)),
      ),
    ),
  ));
}

void main() {
  setUp(() => L10n.setLocale(MemoLocale.en));

  group('formatContextTokens', () {
    test('k and M, like the Claude Code gauge', () {
      expect(formatContextTokens(840), '840');
      expect(formatContextTokens(325100), '325.1k');
      expect(formatContextTokens(1000000), '1M');
      expect(formatContextTokens(1500000), '1.5M');
    });
  });

  group('ContextReport.fromJson', () {
    test('reads every field and survives junk in the lists', () {
      final r = ContextReport.fromJson({
        'provider': 'p',
        'model': 'm',
        'window': 200000,
        'used': 50000,
        'used_real': true,
        'percent': 25,
        'categories': [
          {'key': 'messages', 'tokens': 40000},
          'not a map',
          42,
        ],
        'auto_compact_enabled': true,
        'auto_compact_pct': 90,
        'summarized_messages': 6,
        'limits': [
          {'label': '5h', 'remaining_percent': 69, 'reset_at': '2026-10-08T13:33:00Z'},
          null,
        ],
        'limits_vendor': 'openai',
      });
      expect(r.window, 200000);
      expect(r.categories.length, 1);
      expect(r.limits.single.remainingPercent, 69);
      expect(r.free, 150000);
      expect(r.compactBuffer, 20000, reason: 'the 10% above the 90% threshold');
      expect(r.fraction, 0.25);
    });

    test('an empty or malformed body is a harmless empty report', () {
      final r = ContextReport.fromJson({});
      expect(r.window, 0);
      expect(r.free, 0);
      expect(r.fraction, 0);
      expect(r.categories, isEmpty);
      expect(r.limits, isEmpty);
      final bad = ContextReport.fromJson({'window': 'big', 'categories': 'x', 'limits': {}});
      expect(bad.window, 0);
      expect(bad.categories, isEmpty);
    });

    test('a percentage outside 0..100 is clamped', () {
      expect(QuotaMeter.fromJson({'remaining_percent': 250}).remainingPercent, 100);
      expect(QuotaMeter.fromJson({'remaining_percent': -4}).remainingPercent, 0);
    });
  });

  testWidgets('the popover shows the window, each part, free space and the buffer', (tester) async {
    await _pumpView(tester, _report());
    expect(find.text(L10n.t('ctx_title')), findsOneWidget);
    expect(find.text('325.1k / 1M (32%)'), findsOneWidget);
    expect(find.text('Subscriptions · gpt-5.6-luna'), findsOneWidget);
    for (final key in ['messages', 'tools', 'system']) {
      expect(find.text(L10n.t('ctx_cat_$key')), findsOneWidget);
    }
    expect(find.text('282.2k'), findsOneWidget);
    expect(find.text('28.2%'), findsOneWidget, reason: 'messages are 282.2k of a 1M window');
    expect(find.text(L10n.t('ctx_free')), findsOneWidget);
    expect(find.text(L10n.t('ctx_buffer')), findsOneWidget);
    expect(find.text(L10n.t('ctx_auto_on', {'pct': '90'})), findsOneWidget);
    expect(find.text(L10n.t('ctx_real')), findsOneWidget);
    // Not a Subscriptions account → no usage limits section at all.
    expect(find.text(L10n.t('ctx_limits_title')), findsNothing);
  });

  testWidgets('an estimate is marked with ~ and says so', (tester) async {
    await _pumpView(tester, _report(real: false));
    expect(find.text('~325.1k / 1M (32%)'), findsOneWidget);
    expect(find.text(L10n.t('ctx_estimated')), findsOneWidget);
    expect(find.text(L10n.t('ctx_real')), findsNothing);
  });

  testWidgets('a compacted chat says how many messages a summary stands in for', (tester) async {
    await _pumpView(tester, _report(summarized: 12));
    expect(find.text(L10n.t('ctx_summarized', {'n': '12'})), findsOneWidget);
  });

  testWidgets('with auto-compact off the buffer row is gone and the text says so', (tester) async {
    await _pumpView(tester, _report(autoCompact: false));
    expect(find.text(L10n.t('ctx_buffer')), findsNothing);
    expect(find.text(L10n.t('ctx_auto_off')), findsOneWidget);
  });

  testWidgets('a Subscriptions account shows its session and weekly meters', (tester) async {
    await _pumpView(
      tester,
      _report(limits: const [
        QuotaMeter(label: '5h', remainingPercent: 31, resetAt: '2026-10-08T13:33:00Z'),
        QuotaMeter(label: '7d', remainingPercent: 34, resetAt: '2026-10-12T00:00:00Z'),
      ]),
    );
    expect(find.text(L10n.t('ctx_limits_title')), findsOneWidget);
    expect(find.text(L10n.t('ctx_limit_session')), findsOneWidget);
    expect(find.text(L10n.t('ctx_limit_weekly')), findsOneWidget);
    expect(find.text('31% left'), findsOneWidget);
    expect(find.text('34% left'), findsOneWidget);
    expect(find.text('Resets in 1h 33m'), findsOneWidget, reason: 'now is pinned to 12:00 UTC, the 5h window refills at 13:33');
    expect(find.text('Resets in 3d 12h'), findsOneWidget);
  });

  testWidgets('a per-model figure (Antigravity) is labelled "This model"', (tester) async {
    await _pumpView(tester, _report(limits: const [QuotaMeter(remainingPercent: 80)]));
    expect(find.text(L10n.t('ctx_limit_model')), findsOneWidget);
    expect(find.text('80% left'), findsOneWidget);
  });

  testWidgets('other window lengths are spelled out', (tester) async {
    await _pumpView(tester, _report(limits: const [
      QuotaMeter(label: '30d', remainingPercent: 100),
      QuotaMeter(label: '12h', remainingPercent: 50),
    ]));
    expect(find.text('30-day limit'), findsOneWidget);
    expect(find.text('12-hour limit'), findsOneWidget);
  });

  testWidgets('Turkish wording', (tester) async {
    L10n.setLocale(MemoLocale.tr);
    await _pumpView(tester, _report(limits: const [QuotaMeter(label: '5h', remainingPercent: 31)]));
    expect(find.text('Bağlam penceresi'), findsOneWidget);
    expect(find.text('%31 kaldı'), findsOneWidget);
    expect(find.text('Oturum limiti'), findsOneWidget);
  });

  testWidgets('an empty chat says it takes no context yet', (tester) async {
    await _pumpView(tester, const ContextReport(window: 128000, used: 0, usedReal: false));
    expect(find.text(L10n.t('ctx_empty')), findsOneWidget);
  });

  Future<void> pumpRing(WidgetTester tester, ContextReport? report) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        activeChatIdProvider.overrideWith(_FakeActiveChat.new),
        contextReportProvider.overrideWith((ref, chatId) async => report),
      ],
      child: MaterialApp(
        theme: ThemeData(extensions: [MemoTheme.dark]),
        home: const Scaffold(body: Center(child: ContextRing())),
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('the ring renders nothing when the backend gave no report', (tester) async {
    await pumpRing(tester, null);
    expect(find.byType(CustomPaint).evaluate().where((e) => e.widget is CustomPaint && (e.widget as CustomPaint).painter != null), isEmpty);
    expect(find.byType(GestureDetector), findsNothing);
  });

  testWidgets('tapping the ring opens the popover with the breakdown', (tester) async {
    await pumpRing(tester, _report());
    expect(find.byTooltip(L10n.t('ctx_tooltip', {'pct': '33'})), findsOneWidget);
    await tester.tap(find.byType(ContextRing));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('ctx_title')), findsOneWidget);
    expect(find.text(L10n.t('ctx_cat_messages')), findsOneWidget);
  });

  test('the ring turns amber, then red, as the window nears the compact point', () {
    expect(contextRingColor(0.30, 90), MemoTheme.accent);
    expect(contextRingColor(0.80, 90), MemoTheme.warningOrange);
    expect(contextRingColor(0.90, 90), MemoTheme.red);
    expect(contextRingColor(0.97, 90), MemoTheme.red);
  });
}
