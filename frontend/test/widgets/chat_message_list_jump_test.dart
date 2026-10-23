import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

List<ChatMessage> messages(int count) => List.generate(
  count,
  (i) => ChatMessage(role: 'user', content: 'Message $i', timestamp: '$i'),
);

Widget chat(List<ChatMessage> items) => MaterialApp(
  home: Scaffold(
    body: ChatMessageList(messages: items, apiBaseUrl: 'http://localhost:8090'),
  ),
);

void main() {
  testWidgets('one click returns to the latest message after scrolling up', (
    tester,
  ) async {
    await tester.pumpWidget(chat(messages(40)));
    await tester.pumpAndSettle();
    final list = find.byType(ListView);
    final controller = tester.widget<ListView>(list).controller!;
    controller.jumpTo(controller.position.maxScrollExtent);
    await tester.pumpAndSettle();
    final jump = find.byTooltip(L10n.t('chat_jump_to_latest'));
    expect(jump, findsNothing);
    await tester.drag(list, const Offset(0, 400));
    await tester.pumpAndSettle();
    expect(jump, findsOneWidget);
    final before = controller.offset;
    await tester.pumpWidget(chat(messages(41)));
    await tester.pumpAndSettle();
    expect(controller.offset, closeTo(before, 1));
    expect(jump, findsOneWidget);
    await tester.tap(jump);
    await tester.pumpAndSettle();
    expect(controller.position.extentAfter, lessThan(50));
    expect(jump, findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'one click reaches the end of a long conversation with varied heights',
    (tester) async {
      final items = List.generate(
        80,
        (i) => ChatMessage(
          role: 'user',
          content: List.filled(i < 40 ? 1 : 8, 'Message $i').join('\n'),
          timestamp: '$i',
        ),
      );
      await tester.pumpWidget(chat(items));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip(L10n.t('chat_jump_to_latest')));
      await tester.pumpAndSettle();
      final controller = tester
          .widget<ListView>(find.byType(ListView))
          .controller!;
      expect(controller.position.extentAfter, lessThan(50));
      expect(find.byTooltip(L10n.t('chat_jump_to_latest')), findsNothing);
    },
  );

  testWidgets('scrolling manually cancels an unfinished jump', (tester) async {
    await tester.pumpWidget(chat(messages(80)));
    await tester.pumpAndSettle();
    final jump = find.byTooltip(L10n.t('chat_jump_to_latest'));
    await tester.tap(jump);
    await tester.pump(const Duration(milliseconds: 100));
    await tester.drag(find.byType(ListView), const Offset(0, 200));
    await tester.pumpAndSettle();
    expect(jump, findsOneWidget);
    final controller = tester
        .widget<ListView>(find.byType(ListView))
        .controller!;
    final offset = controller.offset;
    await tester.pump(const Duration(seconds: 1));
    expect(controller.offset, closeTo(offset, 1));
    expect(tester.takeException(), isNull);
  });

  testWidgets('no jump button when the whole conversation fits', (
    tester,
  ) async {
    await tester.pumpWidget(chat(messages(1)));
    await tester.pumpAndSettle();
    expect(find.byTooltip(L10n.t('chat_jump_to_latest')), findsNothing);
  });

  testWidgets('button updates when the viewport grows without scrolling', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(600, 200));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(chat(messages(5)));
    await tester.pumpAndSettle();
    expect(find.byTooltip(L10n.t('chat_jump_to_latest')), findsOneWidget);
    await tester.binding.setSurfaceSize(const Size(600, 1600));
    await tester.pumpAndSettle();
    expect(find.byTooltip(L10n.t('chat_jump_to_latest')), findsNothing);
  });
}
