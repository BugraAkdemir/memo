import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/providers/learning_provider.dart';
import 'package:memo_flutter/widgets/proactive_suggestion_banner.dart';

Widget _wrap(Widget child, {required Stream<PendingProactiveSuggestion?> stream}) {
  return ProviderScope(
    overrides: [
      pendingProactiveSuggestionProvider.overrideWith((ref) => stream),
    ],
    child: MaterialApp(
      home: Scaffold(body: Stack(children: [child])),
    ),
  );
}

void main() {
  testWidgets('renders nothing when there is no pending suggestion', (tester) async {
    await tester.pumpWidget(_wrap(
      const ProactiveSuggestionBanner(),
      stream: Stream.value(null),
    ));
    await tester.pump();

    expect(find.byType(ProactiveSuggestionBanner), findsOneWidget);
    expect(find.text('Evet'), findsNothing);
  });

  testWidgets('shows the suggestion message and all three response buttons', (tester) async {
    await tester.pumpWidget(_wrap(
      const ProactiveSuggestionBanner(),
      stream: Stream.value(const PendingProactiveSuggestion(
        id: 'sugg-1',
        message: 'Kod yazma zamanın geldi, başlayalım mı?',
        patternId: 'declared:21:00',
        action: 'suggest',
      )),
    ));
    await tester.pump();

    expect(find.text('Kod yazma zamanın geldi, başlayalım mı?'), findsOneWidget);
    expect(find.text('Evet'), findsOneWidget);
    expect(find.text('Şimdi değil'), findsOneWidget);
    expect(find.text('Artık sorma'), findsOneWidget);
  });
}
