import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/subagent.dart';
import 'package:memo_flutter/providers/subagent_provider.dart';
import 'package:memo_flutter/widgets/agent/subagent_pane.dart';

void main() {
  testWidgets('tray shows only active agents and one history entry', (
    tester,
  ) async {
    final jobs = [
      const SubagentJob(
        id: 'run',
        chatId: 'room',
        title: 'Active work',
        status: 'running',
      ),
      const SubagentJob(id: 'queue', chatId: 'room', title: 'Queued work'),
      for (final status in [
        'awaiting_followup',
        'failed',
        'cancelled',
        'closed',
        'interrupted',
      ])
        SubagentJob(
          id: status,
          chatId: 'room',
          title: 'Past $status',
          status: status,
        ),
    ];
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          subagentsProvider('room').overrideWith((ref) => Stream.value(jobs)),
        ],
        child: const MaterialApp(
          home: Scaffold(body: SubagentTray(chatId: 'room')),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('subagent-run')), findsOneWidget);
    expect(find.byKey(const ValueKey('subagent-queue')), findsOneWidget);
    for (final job in jobs.where((j) => !j.active)) {
      expect(find.byKey(ValueKey('subagent-${job.id}')), findsNothing);
    }
    expect(
      find.text(L10n.t('subagent_history_count', {'n': '5'})),
      findsOneWidget,
    );
  });
  testWidgets(
    'closed history opens prompt and output and returns to the list',
    (tester) async {
      const closed = SubagentJob(
        id: 'done',
        chatId: 'room',
        title: 'Finished work',
        status: 'closed',
        prompt: 'Original assigned work',
        output: 'Finished result',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            subagentsProvider(
              'room',
            ).overrideWith((ref) => Stream.value([closed])),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: Consumer(
                builder: (context, ref, _) =>
                    subagentDetailsPane(ref, 'room') ??
                    const SubagentTray(chatId: 'room'),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('subagent-history')));
      await tester.pumpAndSettle();
      expect(find.byType(SubagentHistoryPane), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('subagent-history-done')));
      await tester.pumpAndSettle();
      expect(find.text('Original assigned work'), findsOneWidget);
      expect(find.text('Finished result'), findsOneWidget);
      expect(find.byType(TextField), findsNothing);
      await tester.tap(find.byTooltip(L10n.t('close')));
      await tester.pumpAndSettle();
      expect(find.byType(SubagentHistoryPane), findsOneWidget);
      await tester.tap(find.byTooltip(L10n.t('close')));
      await tester.pumpAndSettle();
      expect(find.byType(SubagentTray), findsOneWidget);
    },
  );

  testWidgets('completed agents leave the tray when their state updates', (
    tester,
  ) async {
    final updates = StreamController<List<SubagentJob>>();
    addTearDown(updates.close);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          subagentsProvider('room').overrideWith((ref) => updates.stream),
        ],
        child: const MaterialApp(
          home: Scaffold(body: SubagentTray(chatId: 'room')),
        ),
      ),
    );
    updates.add([
      const SubagentJob(id: 'one', chatId: 'room', status: 'running'),
    ]);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('subagent-one')), findsOneWidget);
    updates.add([
      const SubagentJob(id: 'one', chatId: 'room', status: 'awaiting_followup'),
    ]);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('subagent-one')), findsNothing);
    expect(
      find.text(L10n.t('subagent_history_count', {'n': '1'})),
      findsOneWidget,
    );
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets(
    'older erased records explain why their details are unavailable',
    (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            subagentsProvider('room').overrideWith(
              (ref) => Stream.value([
                const SubagentJob(id: 'old', chatId: 'room', status: 'closed'),
              ]),
            ),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: SubagentPane(chatId: 'room', jobId: 'old', onClose: () {}),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('subagent_history_unavailable')), findsOneWidget);
    },
  );
}
