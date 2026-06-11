import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:memo_mobile/main.dart';

void main() {
  testWidgets('App displays connect screen', (WidgetTester tester) async {
    await tester.pumpWidget(const ProviderScope(child: MemoMobileApp()));
    expect(find.text('Memo'), findsOneWidget);
    expect(find.text('Connect'), findsOneWidget);
  });
}
