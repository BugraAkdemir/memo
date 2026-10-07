import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/widgets/model_picker.dart';

List<ModelPickerEntry> _twelveModels({String active = ''}) => [
      const ModelPickerEntry.item(value: 'local', title: 'Local model'),
      const ModelPickerEntry.header('Antigravity'),
      for (var i = 1; i <= 12; i++)
        ModelPickerEntry.item(
          value: 'm$i',
          title: 'Gemini 3.$i Flash',
          tag: i.isEven ? 'High' : '',
          active: 'm$i' == active,
          trailing: const Text('99%'),
        ),
    ];

/// Opens the picker from a button near the top right and returns what it picked
/// once the test is done interacting.
Future<void> _open(
  WidgetTester tester, {
  required List<ModelPickerEntry> entries,
  required void Function(String?) onResult,
  Size size = const Size(1000, 700),
}) async {
  SharedPreferences.setMockInitialValues({});
  L10n.setLocale(MemoLocale.en);
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(extensions: [MemoTheme.dark]),
      home: Scaffold(
        body: Align(
          alignment: Alignment.topRight,
          child: Builder(
            builder: (ctx) => TextButton(
              key: const Key('btn'),
              onPressed: () async {
                final box = ctx.findRenderObject() as RenderBox;
                onResult(await showModelPicker(
                  context: ctx,
                  anchor: box.localToGlobal(Offset.zero) & box.size,
                  entries: entries,
                  footer: const ModelPickerEntry.item(value: '__add__', title: 'Add provider'),
                ));
              },
              child: const Text('open'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.byKey(const Key('btn')));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a long list stays inside the window and scrolls instead of running off the bottom',
      (tester) async {
    await _open(tester, entries: _twelveModels(), onResult: (_) {}, size: const Size(1000, 480));

    final panel = tester.getRect(find.byType(Material).last);
    expect(panel.bottom, lessThanOrEqualTo(480), reason: 'the panel must end above the window edge');
    expect(panel.right, lessThanOrEqualTo(1000));
    expect(panel.left, greaterThanOrEqualTo(0));
    // Not everything fits, so the later models are not built yet…
    expect(find.text('Gemini 3.12 Flash'), findsNothing);
    // …but scrolling inside the panel reaches them, and the pinned footer stays.
    await tester.drag(find.byType(ListView), const Offset(0, -2000));
    await tester.pumpAndSettle();
    expect(find.text('Gemini 3.12 Flash'), findsOneWidget);
    expect(find.text('Add provider'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('tapping a row returns its value', (tester) async {
    String? got = 'unset';
    await _open(tester, entries: _twelveModels(), onResult: (v) => got = v);
    await tester.tap(find.text('Gemini 3.2 Flash'));
    await tester.pumpAndSettle();
    expect(got, 'm2');
  });

  testWidgets('dismissing returns null', (tester) async {
    String? got = 'unset';
    await _open(tester, entries: _twelveModels(), onResult: (v) => got = v);
    await tester.tapAt(const Offset(5, 690));
    await tester.pumpAndSettle();
    expect(got, isNull);
  });

  testWidgets('search narrows the list and drops a header with nothing under it', (tester) async {
    await _open(tester, entries: _twelveModels(), onResult: (_) {});
    await tester.enterText(find.byType(TextField), 'local');
    await tester.pump();
    expect(find.text('Local model'), findsOneWidget);
    expect(find.text('Gemini 3.1 Flash'), findsNothing);
    expect(find.text('ANTIGRAVITY'), findsNothing);

    await tester.enterText(find.byType(TextField), 'high');
    await tester.pump();
    expect(find.text('ANTIGRAVITY'), findsOneWidget);
    expect(find.text('Gemini 3.2 Flash'), findsOneWidget);
    expect(find.text('Gemini 3.1 Flash'), findsNothing);
  });

  testWidgets('Enter in the search box picks the first match', (tester) async {
    String? got = 'unset';
    await _open(tester, entries: _twelveModels(), onResult: (v) => got = v);
    await tester.enterText(find.byType(TextField), '3.4');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    expect(got, 'm4');
  });

  testWidgets('a short list has no search box', (tester) async {
    await _open(
      tester,
      entries: const [
        ModelPickerEntry.item(value: 'local', title: 'Local model'),
        ModelPickerEntry.item(value: 'a', title: 'Alpha'),
      ],
      onResult: (_) {},
    );
    expect(find.byType(TextField), findsNothing);
  });

  testWidgets('at phone width the panel fits the screen', (tester) async {
    await _open(tester, entries: _twelveModels(), onResult: (_) {}, size: const Size(360, 700));
    final panel = tester.getRect(find.byType(Material).last);
    expect(panel.left, greaterThanOrEqualTo(0));
    expect(panel.right, lessThanOrEqualTo(360));
    expect(tester.takeException(), isNull);
  });
}
