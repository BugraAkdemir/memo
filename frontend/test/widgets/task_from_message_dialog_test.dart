import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/task_from_message_dialog.dart';

// Test-only trigger label; named so it reads as a fixture, not UI copy.
const _openButtonLabel = 'open';

const chats = [
  ChatSession(
    id: 'c1',
    title: 'Memo projesi',
    createdAt: '',
    updatedAt: '',
    msgCount: 1,
    projectPath: '/work/memo',
  ),
  ChatSession(
    id: 'c2',
    title: 'Sıradan sohbet',
    createdAt: '',
    updatedAt: '',
    msgCount: 1,
  ),
  ChatSession(
    id: 'c3',
    title: 'Site projesi',
    createdAt: '',
    updatedAt: '',
    msgCount: 1,
    projectPath: '/work/site',
  ),
];

Future<Map<String, dynamic>?> open(
  WidgetTester tester, {
  String subject = 'Hata düzeltmesi',
  List<ChatSession> list = chats,
}) async {
  Map<String, dynamic>? result;
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: Builder(
          builder: (context) => TextButton(
            onPressed: () async {
              result = await showDialog<Map<String, dynamic>>(
                context: context,
                builder: (_) =>
                    TaskFromMessageDialog(subject: subject, chats: list),
              );
            },
            child: const Text(_openButtonLabel),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text(_openButtonLabel));
  await tester.pumpAndSettle();
  return result;
}

void main() {
  for (final locale in MemoLocale.values) {
    group('task from message dialog ($locale)', () {
      setUp(() => L10n.setLocale(locale));
      tearDown(() => L10n.setLocale(MemoLocale.en));

      testWidgets('explains that nothing is copied or started on its own', (
        tester,
      ) async {
        await open(tester);
        expect(find.text(L10n.t('task_bridge_title')), findsOneWidget);
        expect(find.text(L10n.t('task_bridge_help')), findsOneWidget);
        // The message text is not prefilled; only the subject suggests a title.
        expect(
          find.widgetWithText(TextField, 'Hata düzeltmesi'),
          findsOneWidget,
        );
      });

      testWidgets('only project chats are offered', (tester) async {
        await open(tester);
        await tester.tap(find.byKey(const ValueKey('task-chat')));
        await tester.pumpAndSettle();
        expect(find.text('Memo projesi'), findsWidgets);
        expect(find.text('Site projesi'), findsWidgets);
        expect(find.text('Sıradan sohbet'), findsNothing);
      });

      testWidgets('create stays disabled until chat, title and steps exist', (
        tester,
      ) async {
        await open(tester);
        FilledButton create() => tester.widget<FilledButton>(
          find.byKey(const ValueKey('task-create')),
        );
        expect(create().onPressed, isNull);
        await tester.tap(find.byKey(const ValueKey('task-chat')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Memo projesi').last);
        await tester.pumpAndSettle();
        expect(create().onPressed, isNull); // still no steps
        await tester.enterText(
          find.byKey(const ValueKey('task-steps')),
          'Hatayı üret\n\n  Testi yaz  \n',
        );
        await tester.pump();
        expect(create().onPressed, isNotNull);
        await tester.enterText(find.byKey(const ValueKey('task-title')), '   ');
        await tester.pump();
        expect(create().onPressed, isNull);
      });

      testWidgets(
        'submits the chat, the trimmed title and the non-blank steps',
        (tester) async {
          Map<String, dynamic>? result;
          await tester.pumpWidget(
            MaterialApp(
              home: Scaffold(
                body: Builder(
                  builder: (context) => TextButton(
                    onPressed: () async {
                      result = await showDialog<Map<String, dynamic>>(
                        context: context,
                        builder: (_) => const TaskFromMessageDialog(
                          subject: 'Hata düzeltmesi',
                          chats: chats,
                        ),
                      );
                    },
                    child: const Text(_openButtonLabel),
                  ),
                ),
              ),
            ),
          );
          await tester.tap(find.text(_openButtonLabel));
          await tester.pumpAndSettle();
          await tester.tap(find.byKey(const ValueKey('task-chat')));
          await tester.pumpAndSettle();
          await tester.tap(find.text('Site projesi').last);
          await tester.pumpAndSettle();
          await tester.enterText(
            find.byKey(const ValueKey('task-title')),
            '  Düzelt  ',
          );
          await tester.enterText(
            find.byKey(const ValueKey('task-steps')),
            'Hatayı üret\n\n  Testi yaz  \nDüzelt',
          );
          await tester.pump();
          await tester.tap(find.byKey(const ValueKey('task-create')));
          await tester.pumpAndSettle();
          expect(result, {
            'chat_id': 'c3',
            'title': 'Düzelt',
            'items': ['Hatayı üret', 'Testi yaz', 'Düzelt'],
          });
        },
      );

      testWidgets('more than ten steps are refused with a message', (
        tester,
      ) async {
        await open(tester);
        await tester.tap(find.byKey(const ValueKey('task-chat')));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Memo projesi').last);
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const ValueKey('task-steps')),
          List.generate(11, (i) => 'adım $i').join('\n'),
        );
        await tester.pump();
        expect(find.text(L10n.t('task_bridge_too_many')), findsOneWidget);
        expect(
          tester
              .widget<FilledButton>(find.byKey(const ValueKey('task-create')))
              .onPressed,
          isNull,
        );
      });

      testWidgets('without a project chat it says what to do', (tester) async {
        await open(tester, list: [chats[1]]);
        expect(find.text(L10n.t('task_bridge_no_chat')), findsOneWidget);
        expect(
          tester
              .widget<FilledButton>(find.byKey(const ValueKey('task-create')))
              .onPressed,
          isNull,
        );
      });
    });
  }
}
