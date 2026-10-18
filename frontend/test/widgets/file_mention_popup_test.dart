import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/widgets/file_mention_popup.dart';

/// Reported live: with many files under "@", the arrow-key highlight moved past
/// the visible rows while the list stayed put (the person could not tell which
/// file was selected), and the wheel did not scroll it.
void main() {
  final files = [for (var i = 0; i < 60; i++) 'src/file_${i.toString().padLeft(2, '0')}.go'];

  Widget host(int selected, {void Function(int)? onSelect}) => MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.bottomCenter,
            child: SizedBox(
              width: 500,
              child: FileMentionPopup(
                results: files,
                selectedIndex: selected,
                onSelect: onSelect ?? (_) {},
              ),
            ),
          ),
        ),
      );

  ScrollController controller(WidgetTester tester) =>
      tester.widget<ListView>(find.byType(ListView)).controller!;

  /// The selected row's vertical position is inside the popup's own box.
  bool rowIsVisible(WidgetTester tester, int index) {
    final popup = tester.getRect(find.byType(FileMentionPopup));
    final row = find.text(files[index]);
    if (row.evaluate().isEmpty) return false; // not even built: off screen
    final r = tester.getRect(row);
    return r.top >= popup.top && r.bottom <= popup.bottom;
  }

  testWidgets('moving the selection down keeps the selected row in view', (tester) async {
    await tester.pumpWidget(host(0));
    await tester.pump();
    for (final i in [3, 7, 8, 9, 15, 30, 59]) {
      await tester.pumpWidget(host(i));
      await tester.pump();
      expect(rowIsVisible(tester, i), isTrue, reason: 'selected row $i must be visible');
    }
  });

  testWidgets('moving back up scrolls back up too', (tester) async {
    await tester.pumpWidget(host(59));
    await tester.pump();
    for (final i in [40, 20, 5, 0]) {
      await tester.pumpWidget(host(i));
      await tester.pump();
      expect(rowIsVisible(tester, i), isTrue, reason: 'selected row $i must be visible');
    }
    expect(controller(tester).offset, 0);
  });

  testWidgets('the list scrolls with the wheel / a drag', (tester) async {
    await tester.pumpWidget(host(0));
    await tester.pump();
    final before = controller(tester).offset;
    await tester.drag(find.byType(ListView), const Offset(0, -150));
    await tester.pump();
    expect(controller(tester).offset, greaterThan(before));
  });

  testWidgets('tapping a row selects it', (tester) async {
    int? picked;
    await tester.pumpWidget(host(0, onSelect: (i) => picked = i));
    await tester.pump();
    await tester.tap(find.text(files[2]));
    expect(picked, 2);
  });

  testWidgets('a short list is as tall as its rows, a long one is capped', (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(body: FileMentionPopup(results: files.take(3).toList(), selectedIndex: 0, onSelect: (_) {})),
    ));
    await tester.pump();
    final short = tester.getSize(find.byType(FileMentionPopup)).height;
    await tester.pumpWidget(host(0));
    await tester.pump();
    final long = tester.getSize(find.byType(FileMentionPopup)).height;
    expect(short, lessThan(FileMentionPopup.maxHeight));
    expect(long, FileMentionPopup.maxHeight);
  });
}
