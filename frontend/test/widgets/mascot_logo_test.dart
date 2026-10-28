import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/widgets/memo_mascot.dart';

void main() {
  for (final skin in MascotSkin.values) {
    testWidgets('still logo renders $skin without scheduling animations', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Center(child: MemoMascotLogo(skin: skin)),
        ),
      );
      expect(find.byType(CustomPaint), findsWidgets);
      await tester.pump(const Duration(seconds: 30));
      expect(tester.binding.transientCallbackCount, 0);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('the session logo keeps its skin across rebuilds and remounts', (
    tester,
  ) async {
    Future<Type> paintLogo() async {
      await tester.pumpWidget(
        const MaterialApp(home: Center(child: MemoMascotLogo())),
      );
      final paint = tester.widget<CustomPaint>(
        find.descendant(
          of: find.byType(MemoMascotLogo),
          matching: find.byType(CustomPaint),
        ),
      );
      return paint.painter.runtimeType;
    }

    final original = await paintLogo();
    await tester.pumpWidget(const SizedBox.shrink());
    expect(await paintLogo(), original);
    await tester.pumpAndSettle();
  });
}
