import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/widgets/settings/tabs/taskloop_tab.dart';

// O12 regression: the granularity row (a label + 3 localized ChoiceChips)
// was a plain Row with no scroll/flex protection — Turkish's longer chip
// labels ("literal", "hibrit" plus the "Adım ayrıştırma" label) can overflow
// it at a narrow width, the same class of bug the auth gate footer had
// (AGENTS.md's Flutter Gotchas: "A Row of buttons that fits in one language
// can overflow in another"). Turkish is the widest label set in this app, so
// it's the locale that actually exercises the risk.
//
// Pumped as TaskLoopGranularityRow in isolation (extracted from
// _TaskLoopTabState._granularity) rather than the whole TaskLoopTab: the
// full tab has other, unrelated pre-existing narrow-width overflow spots
// (a DropdownButtonFormField without isExpanded, an unconstrained panel
// header Row) that are a separate, out-of-scope problem and would make a
// full-widget test fail for reasons that have nothing to do with this fix.
void main() {
  testWidgets('TaskLoopGranularityRow does not overflow at a narrow width under Turkish', (tester) async {
    L10n.setLocale(MemoLocale.tr);
    addTearDown(() => L10n.setLocale(MemoLocale.en));

    tester.view.physicalSize = const Size(280, 200);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(MaterialApp(
      home: Builder(
        builder: (context) => Scaffold(
          body: TaskLoopGranularityRow(
            color: MemoTheme.of(context),
            value: 'hybrid',
            enabled: true,
            onSelect: (_) {},
          ),
        ),
      ),
    ));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull,
        reason: 'granularity row must not throw a RenderFlex overflow under Turkish at a narrow width (O12)');
    expect(find.text(L10n.t('taskloop_granularity')), findsOneWidget);
    expect(find.text(L10n.t('taskloop_gran_hybrid')), findsOneWidget);
    expect(find.text(L10n.t('taskloop_gran_literal')), findsOneWidget);
  });
}
