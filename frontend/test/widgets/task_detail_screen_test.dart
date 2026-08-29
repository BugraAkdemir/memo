import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/task_list.dart';
import 'package:memo_flutter/providers/tasklist_provider.dart';
import 'package:memo_flutter/screens/task_detail_screen.dart';

class _FakeRunningTasks extends RunningTasksNotifier {
  _FakeRunningTasks(this._data);
  final List<RunningTaskInfo> _data;

  @override
  Future<List<RunningTaskInfo>> build() async => _data;

  // No real polling in tests (a periodic Timer would defeat pumpAndSettle).
  @override
  void startPolling() {}
  @override
  void stopPolling() {}
}

void main() {
  testWidgets('TaskDetailScreen shows phase, progress and sub-agent chips', (tester) async {
    final info = const RunningTaskInfo(
      id: 'L1',
      title: 'Ship feature',
      phase: 'executing',
      doneCount: 1,
      itemCount: 3,
      currentItem: 'wire the endpoint',
      elapsedSec: 42,
      subAgents: ['coder', 'reviewer'],
    );

    await tester.pumpWidget(ProviderScope(
      overrides: [
        runningTasksProvider.overrideWith(() => _FakeRunningTasks([info])),
      ],
      child: const MaterialApp(
        home: TaskDetailScreen(taskListId: 'L1', title: 'Ship feature'),
      ),
    ));
    await tester.pump();

    expect(find.text('Ship feature'), findsWidgets);
    expect(find.textContaining(L10n.t('task_phase_executing')), findsOneWidget);
    expect(find.text('1/3'), findsOneWidget);
    expect(find.text('wire the endpoint'), findsOneWidget);
    expect(find.text('coder'), findsOneWidget);
    expect(find.text('reviewer'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);

    // Control buttons present.
    expect(find.text(L10n.t('task_pause')), findsOneWidget);
    expect(find.text(L10n.t('task_resume')), findsOneWidget);
    expect(find.text(L10n.t('task_cancel')), findsOneWidget);
    expect(find.text(L10n.t('task_skip')), findsOneWidget);
  });

  testWidgets('TaskDetailScreen tolerates a task that is not currently running', (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        runningTasksProvider.overrideWith(() => _FakeRunningTasks(const [])),
      ],
      child: const MaterialApp(
        home: TaskDetailScreen(taskListId: 'missing', title: 'Gone'),
      ),
    ));
    await tester.pump();

    expect(find.byType(TaskDetailScreen), findsOneWidget);
    expect(find.text(L10n.t('task_pause')), findsOneWidget); // controls still render
  });
}
