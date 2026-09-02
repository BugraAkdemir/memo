import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/task_status.dart';

Widget _wrap(Widget child) => MaterialApp(home: Scaffold(body: Center(child: child)));

void main() {
  testWidgets('TaskStatusBadge renders a label for each v4.4.0 phase', (tester) async {
    final cases = <String, String>{
      'executing': L10n.t('task_phase_executing'),
      'planning': L10n.t('task_phase_planning'),
      'waiting-limit': L10n.t('task_phase_waiting_limit'),
      'waiting-user': L10n.t('task_phase_waiting_user'),
      'failed': L10n.t('task_status_failed'),
      'cancelled': L10n.t('task_status_cancelled'),
      'done': L10n.t('taskloop_done'),
    };
    for (final entry in cases.entries) {
      await tester.pumpWidget(_wrap(TaskStatusBadge(entry.key)));
      expect(
        find.textContaining(entry.value),
        findsOneWidget,
        reason: 'status ${entry.key} should show ${entry.value}',
      );
    }
  });

  testWidgets('taskItemStatusIcon distinguishes done/stuck/running/other', (tester) async {
    await tester.pumpWidget(_wrap(Builder(
      builder: (ctx) => Column(children: [
        taskItemStatusIcon(ctx, 'done'),
        taskItemStatusIcon(ctx, 'stuck'),
        taskItemStatusIcon(ctx, 'running'),
        taskItemStatusIcon(ctx, 'pending'),
      ]),
    )));
    expect(find.byIcon(Icons.check_circle), findsOneWidget);
    expect(find.byIcon(Icons.error), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.byIcon(Icons.circle_outlined), findsOneWidget);
  });
}
