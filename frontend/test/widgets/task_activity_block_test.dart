import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/task_list.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/tasklist_provider.dart';
import 'package:memo_flutter/widgets/agent/task_activity_block.dart';

/// Answers every request with a fixed plan_md payload — enough for
/// _showPlanSheet's getTaskPlanMd call to resolve without a real backend.
class _FakePlanAdapter implements HttpClientAdapter {
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add(options.path);
    return ResponseBody.fromString(
      '{"plan_md":"# Plan\\n- S1: do the thing"}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _FakeRunningTasks extends RunningTasksNotifier {
  @override
  Future<List<RunningTaskInfo>> build() async => const [];
  @override
  void startPolling() {}
  @override
  void stopPolling() {}
}

Future<void> _pump(WidgetTester tester, ChatTaskState state) async {
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = _FakePlanAdapter();
  await tester.pumpWidget(ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(client),
      runningTasksProvider.overrideWith(() => _FakeRunningTasks()),
    ],
    child: MaterialApp(
      home: Scaffold(body: TaskActivityBlock(state: state)),
    ),
  ));
  await tester.pump();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  // BUG-PLAN9 / BUG-PLAN12 regression coverage: neither had ANY widget-level
  // test before this file — only the underlying ChatTaskState.fold() data
  // model was tested (test/models_test.dart). Both bugs were about what the
  // user actually sees rendered in the chat transcript, which fold() tests
  // can't prove either way.

  testWidgets('awaiting-plan-approval shows the approve action and opens the plan sheet (BUG-PLAN9)', (tester) async {
    final state = const ChatTaskState(
      listId: 'L1',
      phase: 'awaiting-plan-approval',
      mode: 'planlayıcı',
    );
    await _pump(tester, state);

    final approveButton = find.text(L10n.t('task_card_view_approve_plan'));
    expect(approveButton, findsOneWidget,
        reason: 'awaitingPlan must show a way to review/approve the plan without leaving the chat');

    await tester.tap(approveButton);
    // _showPlanSheet awaits one HTTP call before opening — pump through it.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.text(L10n.t('task_plan_review_title')), findsOneWidget,
        reason: 'tapping the approve action must open the plan-review bottom sheet in-chat, not navigate away');
  });

  testWidgets('running planner-mode task shows step+item progress and the activity log (BUG-PLAN12)', (tester) async {
    final state = ChatTaskState(
      listId: 'L2',
      phase: 'executing',
      mode: 'planlayıcı',
      stepDone: 7,
      stepTotal: 13,
      itemDone: 2,
      itemTotal: 6,
      elapsedSec: 42,
      log: [
        TaskLogEntry('tool', 'Dosya yazdı: signup.py', DateTime.now()),
        TaskLogEntry('step_done', 'S7 tamamlandı', DateTime.now()),
      ],
    );
    await _pump(tester, state);

    // One canonical progress line carrying BOTH numbers — the exact thing
    // BUG-PLAN11(b) asked every surface to converge on, and this surface
    // already did (task_card_step/task_card_item), unlike the two others.
    expect(find.textContaining('7/13'), findsOneWidget);
    expect(find.textContaining('2/6'), findsOneWidget);

    // The activity log itself — the actual content of BUG-PLAN12's ask
    // ("adım başladı/bitti ... gibi şeyleri anlık sohbette görebileyim").
    expect(find.text('Dosya yazdı: signup.py'), findsOneWidget);
    expect(find.text('S7 tamamlandı'), findsOneWidget);

    expect(find.text(L10n.t('task_card_pause')), findsOneWidget);
  });

  testWidgets('paused task shows the resume action', (tester) async {
    final state = const ChatTaskState(
      listId: 'L3',
      phase: 'paused',
      itemDone: 1,
      itemTotal: 4,
    );
    await _pump(tester, state);

    expect(find.text(L10n.t('task_card_resume')), findsOneWidget);
    expect(find.text(L10n.t('task_card_pause')), findsNothing);
  });
}
