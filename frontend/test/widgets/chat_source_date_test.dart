import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

void main() {
  test('full message time survives JSON and legacy messages stay undated', () {
    final current = ChatMessage.fromJson({
      'role': 'user',
      'content': 'Yarın 12:00',
      'timestamp': '23:50',
      'sent_at': '2026-10-31T23:50:00+03:00',
    });
    expect(current.toJson()['sent_at'], '2026-10-31T23:50:00+03:00');
    expect(current.timestamp, '23:50');
    final legacy = ChatMessage.fromJson({
      'role': 'user',
      'content': 'Yarın 12:00',
      'timestamp': '23:50',
    });
    expect(legacy.sentAt, isEmpty);
    expect(legacy.toJson().containsKey('sent_at'), isFalse);
  });
  for (final locale in MemoLocale.values) {
    testWidgets(
      'source date tooltip is localized without changing the message ($locale)',
      (tester) async {
        final previous = L10n.locale;
        L10n.setLocale(locale);
        addTearDown(() => L10n.setLocale(previous));
        const sent = '2026-10-31T23:50:00+03:00';
        await tester.pumpWidget(
          const MaterialApp(
            home: Scaffold(
              body: ChatMessageList(
                messages: [
                  ChatMessage(
                    role: 'user',
                    content: 'Yarın 12:00',
                    timestamp: '23:50',
                    sentAt: sent,
                  ),
                ],
                apiBaseUrl: 'http://localhost:8090',
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(
          find.byTooltip(L10n.t('message_sent_at', {'time': sent})),
          findsOneWidget,
        );
        expect(find.text('23:50'), findsOneWidget);
        expect(
          find.textContaining('Yarın 12:00', findRichText: true),
          findsWidgets,
        );
        expect(tester.takeException(), isNull);
      },
    );
  }
}
