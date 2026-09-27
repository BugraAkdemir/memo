import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/usage_stats.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/stats_tab.dart';

// Coverage for the prompt-cache section of the usage-stats tab. The point
// worth protecting is not the layout but the WORDING decision: a window in
// which no provider reported a cache figure must say "not reported", never
// show a measured 0% hit rate — the local llama-server reuses its KV cache on
// every turn and reports nothing, so a zero there would be a false claim.
//
// Labels are asserted through L10n.t(), never as literals, so this stays
// correct whichever locale becomes the default (see the l10n note in
// AGENTS.md's Flutter gotchas).

class _FakeStats extends UsageStatsNotifier {
  _FakeStats(this._value);
  final UsageStatsSummary _value;

  @override
  Future<UsageStatsSummary> build() async => _value;
}

Future<void> _pumpStatsTab(WidgetTester tester, UsageStatsSummary stats) async {
  SharedPreferences.setMockInitialValues(const {});
  final prefs = await SharedPreferences.getInstance();
  L10n.setLocale(MemoLocale.en);

  tester.view.physicalSize = const Size(1200, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        // Port 1 is a privileged port nothing listens on in a test sandbox:
        // the tab's own best-effort memory-stats fetch fails fast instead of
        // needing a mocking library. Same trick as settings_dialog_test.dart.
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        // Without this the real StreamProvider makes a genuine network call
        // and leaves a pending timer that flutter_test fails the test over.
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
        usageStatsProvider.overrideWith(() => _FakeStats(stats)),
      ],
      child: MaterialApp(home: Scaffold(body: StatsTab())),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a window with reported cache figures shows the split, not a bare total', (tester) async {
    await _pumpStatsTab(
      tester,
      const UsageStatsSummary(
        totalRequests: 4,
        totalPromptTokens: 20000,
        totalCompletionTokens: 500,
        totalCachedPromptTokens: 15000,
        totalCacheWriteTokens: 2000,
        modelBreakdown: [
          ModelUsage(
            model: 'claude-opus',
            requests: 3,
            promptTokens: 18000,
            completionTokens: 400,
            cachedPromptTokens: 15000,
            cacheWriteTokens: 2000,
          ),
          ModelUsage(model: 'llama-3-8b', requests: 1, promptTokens: 2000, completionTokens: 100),
        ],
        categoryBreakdown: [
          CategoryUsage(
            category: 'agent',
            requests: 3,
            promptTokens: 18000,
            completionTokens: 400,
            cachedPromptTokens: 15000,
            cacheWriteTokens: 2000,
          ),
        ],
      ),
    );

    expect(find.text(L10n.t('stats_cache_title')), findsOneWidget);
    expect(find.text(L10n.t('stats_cache_not_reported')), findsNothing);
    // 15000 / 20000 = 75.0%
    expect(find.text(L10n.t('stats_cache_hit_ratio', {'pct': '75.0'})), findsOneWidget);
    expect(find.text(L10n.t('stats_cache_read')), findsOneWidget);
    expect(find.text(L10n.t('stats_cache_write')), findsOneWidget);
    expect(find.text(L10n.t('stats_cache_fresh')), findsOneWidget);
    // Full-price input is 20000 - 15000 - 2000 = 3000, shown as 3.0K.
    expect(find.text('3.0K'), findsWidgets);
    // The per-row badge appears for the model/category that reported a hit,
    // and exactly once each — the row with no cache figure must stay bare.
    expect(find.text(L10n.t('stats_cache_badge', {'tokens': '15.0K'})), findsNWidgets(2));
  });

  testWidgets('a window with no reported cache figures says so instead of showing 0%', (tester) async {
    await _pumpStatsTab(
      tester,
      const UsageStatsSummary(
        totalRequests: 2,
        totalPromptTokens: 5000,
        totalCompletionTokens: 200,
        modelBreakdown: [
          ModelUsage(model: 'llama-3-8b', requests: 2, promptTokens: 5000, completionTokens: 200),
        ],
      ),
    );

    expect(find.text(L10n.t('stats_cache_title')), findsOneWidget);
    expect(find.text(L10n.t('stats_cache_not_reported')), findsOneWidget);
    expect(find.text(L10n.t('stats_cache_hit_ratio', {'pct': '0.0'})), findsNothing);
    // No "From cache" stat card either — a card reading 0 would carry the same
    // false implication as a 0% hit rate.
    expect(find.text(L10n.t('stats_cached_tokens')), findsNothing);
    expect(find.textContaining(L10n.t('stats_cache_badge', {'tokens': ''}).trim()), findsNothing);
  });

  testWidgets('the cache panel fits without overflow under the longer Turkish labels', (tester) async {
    // The legend is a Wrap rather than a Row specifically because Turkish
    // labels run longer than English (AGENTS.md: a Row of localized actions
    // that fits in one language overflows in the other). A widget test only
    // exercises the locale it sets, so this one sets Turkish explicitly and
    // uses a narrow surface.
    SharedPreferences.setMockInitialValues(const {});
    final prefs = await SharedPreferences.getInstance();
    L10n.setLocale(MemoLocale.tr);
    addTearDown(() => L10n.setLocale(MemoLocale.en));

    tester.view.physicalSize = const Size(700, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          prefsProvider.overrideWithValue(prefs),
          apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
          authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
          usageStatsProvider.overrideWith(() => _FakeStats(const UsageStatsSummary(
                totalRequests: 1,
                totalPromptTokens: 12000,
                totalCompletionTokens: 300,
                totalCachedPromptTokens: 9000,
                totalCacheWriteTokens: 1500,
              ))),
        ],
        child: MaterialApp(home: Scaffold(body: StatsTab())),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('stats_cache_title')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
