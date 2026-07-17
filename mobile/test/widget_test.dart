import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:memo_mobile/core/l10n.dart';
import 'package:memo_mobile/main.dart';

void main() {
  testWidgets('App displays connect screen', (WidgetTester tester) async {
    L10n.setLocale(MemoLocale.tr);
    await tester.pumpWidget(const ProviderScope(child: MemoMobileApp()));
    expect(find.text(L10n.t('app_name')), findsOneWidget);
    expect(find.text(L10n.t('connect')), findsOneWidget);
  });
}
