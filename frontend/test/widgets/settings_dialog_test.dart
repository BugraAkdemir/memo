import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/gpu_info.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings_dialog.dart';

// Regression coverage for the v3.3.4 Settings redesign: 20 flat, identically
// weighted tabs became a searchable rail grouped under a handful of eyebrow
// headers. These tests cover the two things that redesign could plausibly
// break: the search filter actually narrowing/restoring the list, and the
// dialog's own layout staying overflow-free once the window gets small
// (the user explicitly flagged this as a risk to watch for).
final _railList = find.byKey(const Key('settingsRailList'));
final _railScrollable = find.descendant(of: _railList, matching: find.byType(Scrollable));

Future<void> _pumpSettingsDialog(WidgetTester tester, {Size size = const Size(1200, 800)}) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  L10n.setLocale(MemoLocale.tr);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        // Port 1 is a privileged port nothing listens on in a test sandbox —
        // connecting fails fast and reliably, no mocking library needed.
        apiClientProvider.overrideWithValue(MemoApiClient(baseUrl: 'http://127.0.0.1:1')),
        // The active tab (GeneralTab) watches this — the real provider
        // polls every 30s via Future.delayed, which flutter_test flags as
        // a leaked pending timer once the test ends. A single fixed value
        // is all this suite needs.
        embeddingStatusProvider.overrideWith((ref) => Stream.value(const ServerStatus())),
      ],
      child: MediaQuery(
        data: MediaQueryData(size: size),
        child: const MaterialApp(home: Scaffold(body: SettingsDialog())),
      ),
    ),
  );
  await tester.pump();
}

// Rail labels are read through L10n rather than hardcoded, so these tests
// assert the real UI in whatever the active locale is instead of breaking
// the moment the default language changes (it flipped to English on
// 2026-08-13).
String get _providersGroup => L10n.t('settings_group_providers');
String get _agentsGroup => L10n.t('settings_group_agents');
String get _systemGroup => L10n.t('settings_group_system');
String get _reportBugTab => L10n.t('tab_report_bug');

/// A search term guaranteed to match the Report Bug row and nothing in the
/// Providers or Agents groups: the first word of that row's own label
/// ("report" / "hata").
String get _reportBugTerm => _reportBugTab.split(' ').first.toLowerCase();

void main() {
  testWidgets('shows grouped headers and tab rows with no layout overflow at a normal window size',
      (tester) async {
    await _pumpSettingsDialog(tester);

    expect(find.text(_providersGroup), findsOneWidget);
    await tester.scrollUntilVisible(find.text(_agentsGroup), 150, scrollable: _railScrollable);
    expect(find.text(_agentsGroup), findsOneWidget);
    await tester.scrollUntilVisible(find.text(_reportBugTab), 150, scrollable: _railScrollable);
    expect(find.text(_reportBugTab), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('stays overflow-free when the window shrinks to a small size', (tester) async {
    await _pumpSettingsDialog(tester, size: const Size(360, 640));
    await tester.pump(const Duration(milliseconds: 50));

    // The redesign fixes the dialog's own pixel size once from the
    // viewport rather than letting inner Row/Column widgets assume more
    // space than they're given — this is the regression this test guards:
    // a shrinking host window must never throw a RenderFlex overflow from
    // the settings sidebar or its search field.
    expect(tester.takeException(), isNull);
  });

  testWidgets('search narrows the rail to matching rows and hides now-empty groups', (tester) async {
    await _pumpSettingsDialog(tester);

    await tester.enterText(find.byType(TextField), _reportBugTerm);
    await tester.pump();

    expect(find.text(_reportBugTab), findsOneWidget);
    // No item in the "Agents & Automation" group matches the term — the
    // whole group, header included, must disappear rather than show an
    // empty section.
    expect(find.text(_agentsGroup), findsNothing);
    expect(find.text(_providersGroup), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('shows a no-results message when nothing matches the search', (tester) async {
    await _pumpSettingsDialog(tester);

    await tester.enterText(find.byType(TextField), 'zzz-nomatch-zzz');
    await tester.pump();

    expect(find.text(L10n.t('settings_search_no_results')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('clearing the search restores every group', (tester) async {
    await _pumpSettingsDialog(tester);

    await tester.enterText(find.byType(TextField), _reportBugTerm);
    await tester.pump();
    expect(find.text(_providersGroup), findsNothing);

    await tester.enterText(find.byType(TextField), '');
    await tester.pump();

    expect(find.text(_providersGroup), findsOneWidget);
    await tester.scrollUntilVisible(find.text(_systemGroup), 150, scrollable: _railScrollable);
    expect(find.text(_systemGroup), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
