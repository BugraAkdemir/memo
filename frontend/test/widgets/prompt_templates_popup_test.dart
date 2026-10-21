import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/prompt_templates.dart';

void main() {
  Widget host(int selected, {String query = '', ValueChanged<PopupResult>? onSelect}) => MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: Column(children: [
              PromptTemplatesPopup(
                selectedIndex: selected,
                query: query,
                onSelect: onSelect ?? (_) {},
                onDismiss: () {},
              ),
              const SizedBox(height: 1000),
            ]),
          ),
        ),
      );

  void expectVisible(WidgetTester tester, int index) {
    final popup = tester.getRect(find.byType(PromptTemplatesPopup));
    final row = tester.getRect(find.text(templates[index].key));
    expect(row.top, greaterThanOrEqualTo(popup.top));
    expect(row.bottom, lessThanOrEqualTo(popup.bottom));
  }

  testWidgets('selection scrolls down, up and wraps inside a scrollable page', (tester) async {
    await tester.pumpWidget(host(0));
    for (final index in [5, 8, 10, 0, 10, 4, 0]) {
      await tester.pumpWidget(host(index));
      await tester.pump();
      expectVisible(tester, index);
    }
  });

  testWidgets('mouse wheel scrolls the menu and its newly visible row is clickable', (tester) async {
    PopupResult? picked;
    await tester.pumpWidget(host(0, onSelect: (result) => picked = result));
    await tester.pump();
    final outer = tester.state<ScrollableState>(find.byType(Scrollable).first);
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: tester.getCenter(find.byType(PromptTemplatesPopup)));
    await tester.sendEventToBinding(PointerScrollEvent(
      position: tester.getCenter(find.byType(PromptTemplatesPopup)),
      scrollDelta: const Offset(0, 1000),
    ));
    await tester.pumpAndSettle();
    expect(outer.position.pixels, 0, reason: 'the wheel belongs to the menu');
    expectVisible(tester, 10);
    await tester.tap(find.text('/skill'));
    expect(picked, isA<PopupSkillSelect>());
    await mouse.removePointer();
  });

  testWidgets('filtering a scrolled list resets the visible results', (tester) async {
    await tester.pumpWidget(host(10));
    await tester.pump();
    await tester.pumpWidget(host(0, query: 'explain'));
    await tester.pumpAndSettle();
    expect(find.text('/explain').hitTestable(), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  for (final locale in MemoLocale.values) {
    testWidgets('selected command stays visible with large text in ${locale.name}', (tester) async {
      final previous = L10n.locale;
      L10n.setLocale(locale);
      addTearDown(() => L10n.setLocale(previous));
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: MediaQuery(
            data: const MediaQueryData(textScaler: TextScaler.linear(2)),
            child: PromptTemplatesPopup(
              selectedIndex: 10,
              onSelect: (_) {},
              onDismiss: () {},
            ),
          ),
        ),
      ));
      await tester.pumpAndSettle();
      expectVisible(tester, 10);
      expect(tester.takeException(), isNull);
    });
  }
}
