import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

void main() {
  test('routing attribution round trips without changing message content', () {
    final m = ChatMessage.fromJson({'role': 'assistant', 'content': 'answer', 'timestamp': '12:00', 'route_provider': 'Zen', 'route_model': 'coder'});
    expect(m.routeModel, 'coder');
    expect(m.toJson()['route_provider'], 'Zen');
    expect(m.content, 'answer');
    expect(ChatMessage.fromJson({'content': 'legacy'}).routeModel, isEmpty);
  });
  for (final locale in MemoLocale.values) {
    testWidgets('live and saved model attribution fits narrow layout in $locale', (tester) async {
      L10n.setLocale(locale);
      addTearDown(() => L10n.setLocale(MemoLocale.en));
      tester.view.physicalSize = const Size(360, 700);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(MaterialApp(home: Scaffold(body: ChatMessageList(
        apiBaseUrl: 'http://localhost:8090',
        modelRoute: const ModelRoute('Subscriptions', 'a-very-long-model-name'),
        streamingContent: 'streamed answer',
        messages: const [ChatMessage(role: 'assistant', content: 'previous answer', timestamp: '12:00', routeProvider: 'Zen', routeModel: 'coder')],
      ))));
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('auto_route_used', {'provider': 'Zen', 'model': 'coder'})), findsOneWidget);
      expect(find.text(L10n.t('auto_route_used', {'provider': 'Subscriptions', 'model': 'a-very-long-model-name'})), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }
}
