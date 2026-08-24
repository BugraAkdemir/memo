import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

/// Smoke test for the "N memories used" indicator (see chat_provider.dart's
/// memoryUsed handling and ChatMessage.memoryUsed). Renders a real assistant
/// message carrying memoryUsed > 0 through the actual widget tree used in
/// the app — catches asset-loading failures (brain.svg not registered/
/// found), layout overflow, and type errors that a pure Dart unit test on
/// the model alone would miss.
void main() {
  testWidgets(
    'renders an assistant message with memoryUsed without throwing',
    (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatMessageList(
              apiBaseUrl: 'http://localhost:8090',
              messages: const [
                ChatMessage(
                  role: 'user',
                  content: 'adimi hatirliyor musun',
                  timestamp: '12:00',
                ),
                ChatMessage(
                  role: 'assistant',
                  content: 'evet, adin Ahmet',
                  timestamp: '12:00',
                  memoryUsed: 3,
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('evet, adin Ahmet'), findsOneWidget);
    },
  );

  testWidgets(
    'renders a message with memoryUsed == 0 without throwing (no badge, no crash)',
    (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: ChatMessageList(
              apiBaseUrl: 'http://localhost:8090',
              messages: [
                ChatMessage(
                  role: 'assistant',
                  content: 'selam',
                  timestamp: '12:00',
                  memoryUsed: 0,
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
    },
  );
}
