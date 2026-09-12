import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/task_list.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
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
    // BUG-PLAN11(b): was a bare, unlabeled '1/3' — now carries the same
    // 'madde'/'item' label the chat activity block's progress line uses.
    expect(find.text('${L10n.t('task_card_item')} 1/3'), findsOneWidget);
    expect(find.text('wire the endpoint'), findsOneWidget);
    expect(find.text('coder'), findsOneWidget);
    expect(find.text('reviewer'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);

    // A running task shows Pause (not Resume — resuming an already-running
    // task is a no-op and used to 500).
    expect(find.text(L10n.t('task_pause')), findsOneWidget);
    expect(find.text(L10n.t('task_resume')), findsNothing);
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
    // Not running in-process → Resume is the offered control, not Pause.
    expect(find.text(L10n.t('task_resume')), findsOneWidget);
    expect(find.text(L10n.t('task_cancel')), findsOneWidget);
  });

  // O11 regression: _approve's catch block showed the raw exception via
  // Text('$e') instead of routing it through FriendlyError.describeGeneric
  // like every other error surface in the app — a user staring at an
  // untranslated DioException dump ("DioException [bad response]: ...")
  // instead of a short, plain-language message.
  testWidgets('a failed plan approval shows the friendly generic message, not a raw exception', (tester) async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeApprovalFailsAdapter();

    // M5 (stability audit): _PlanApprovalSection._load() now checks the auth
    // gate before fetching (routines_screen.dart's BUG-ONB11 pattern) — an
    // unresolved gate (the default with no override) reads as blocked, so
    // the container must be pre-resolved to AuthGateState.ok before
    // pumpWidget, same as settings_toggle_race_test.dart /
    // routines_channel_chips_test.dart.
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      runningTasksProvider.overrideWith(() => _FakeRunningTasks(const [])),
      authGateProvider.overrideWith(
        (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
      ),
    ]);
    addTearDown(container.dispose);
    await container.read(authGateProvider.future);

    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(
        home: Scaffold(body: TaskDetailScreen(taskListId: 'L1', title: 'T')),
      ),
    ));
    // _load() awaits two HTTP calls (getTaskList, then getTaskPlanMd) before
    // the approve button ever renders.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    final approveButton = find.text(L10n.t('taskdetail_plan_approve'));
    expect(approveButton, findsOneWidget,
        reason: 'an awaiting-plan-approval task must show the approve button');

    await tester.tap(approveButton);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.textContaining('DioException'), findsNothing,
        reason: 'must not leak a raw Dio exception dump into the SnackBar');
    // describeGeneric extracts the backend's own {"error": "..."} message
    // cleanly for a bad-response failure — this is the exact text it
    // produces for this scenario, not a fabricated expectation.
    expect(find.textContaining('approve failed'), findsOneWidget,
        reason: 'must show FriendlyError.describeGeneric\'s extracted message instead');
  });
}

/// Answers getTaskList/getTaskPlanMd successfully (status
/// "awaiting-plan-approval", a trivial plan) and saveTaskPlanMd
/// successfully, but fails the approve-plan POST with a 500 — for the O11
/// regression test above.
class _FakeApprovalFailsAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path.endsWith('/approve-plan')) {
      return ResponseBody.fromString('{"error":"approve failed"}', 500, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
    if (options.path.endsWith('/plan') && options.method == 'GET') {
      return ResponseBody.fromString('{"plan_md":"# Plan\\n- S1"}', 200, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
    if (options.path.endsWith('/plan')) {
      // saveTaskPlanMd (PUT) — succeed with an empty body.
      return ResponseBody.fromString('{}', 200, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
    // getTaskList
    return ResponseBody.fromString(
      '{"id":"L1","status":"awaiting-plan-approval","items":[]}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
