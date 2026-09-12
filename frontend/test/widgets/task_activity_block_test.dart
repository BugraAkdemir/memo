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

/// Like _FakePlanAdapter, but the approve-plan POST fails (500) while every
/// other request (the plan_md GET the sheet loads on open) still succeeds —
/// for the Y4 regression test below. Mirrors task_detail_screen_test.dart's
/// _FakeApprovalFailsAdapter: a realistic {"error": "..."} JSON body, not a
/// bespoke DioException.error string — describeGeneric only extracts a
/// message from response.data, so an unrealistic empty body would silently
/// fall back to the generic friendly-error text instead of exercising the
/// real extraction path (M2, stability audit).
class _FakePlanAdapterApproveFails implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path.contains('approve-plan')) {
      return ResponseBody.fromString('{"error":"approve failed"}', 500, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
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

Future<void> _pump(WidgetTester tester, ChatTaskState state,
    {HttpClientAdapter? adapter}) async {
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = adapter ?? _FakePlanAdapter();
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

  // M3 (stability audit): step_stuck/item_stuck/step_retry carry the
  // backend's raw err.Error() text (engine.go's stuckActivityLine) and used
  // to render it completely verbatim in the activity log — a rate-limit
  // failure showed the raw provider dump instead of the same short,
  // friendly sentence every other error surface in the app already shows
  // for that exact case.
  testWidgets('a stuck step shows the friendly error text, not the raw provider dump', (tester) async {
    final state = ChatTaskState(
      listId: 'L4',
      phase: 'executing',
      mode: 'worker',
      itemDone: 1,
      itemTotal: 3,
      log: [
        TaskLogEntry(
          'step_stuck',
          'S2 — all providers failed: [opencode-zen] provider rate limited: Rate limit exceeded. Please try again later.',
          DateTime.now(),
        ),
      ],
    );
    await _pump(tester, state);

    expect(find.textContaining('all providers failed'), findsNothing,
        reason: 'must not leak the raw provider/router error dump into the activity log');
    expect(find.text(L10n.t('friendly_error_provider_rate_limited')), findsOneWidget,
        reason: 'must show FriendlyError.describeGeneric\'s rate-limit classification instead');
  });

  testWidgets('an ordinary log line (not an error kind) is shown verbatim, untouched by FriendlyError', (tester) async {
    final state = ChatTaskState(
      listId: 'L5',
      phase: 'executing',
      mode: 'worker',
      itemDone: 1,
      itemTotal: 3,
      log: [
        TaskLogEntry('tool', 'all providers failed: rate limit in a filename.py', DateTime.now()),
      ],
    );
    await _pump(tester, state);

    // A 'tool' line is Memo's own composed status text, never a raw error —
    // it must never be run through error classification just because its
    // content happens to mention a word FriendlyError recognizes.
    expect(find.text('all providers failed: rate limit in a filename.py'), findsOneWidget);
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

  // Y4 regression: the plan-review sheet's approve button used to swallow
  // any exception from approveTaskPlan (try/catch(_){}) and close the sheet
  // unconditionally regardless of success or failure — a 401/timeout/
  // network error looked exactly like a successful approval, and the task
  // list silently stayed in awaiting-plan-approval with no indication
  // anything went wrong. TaskDetailScreen._approve's identical call already
  // showed a SnackBar and kept the screen open on error; the sheet must now
  // behave the same way.
  testWidgets('plan sheet approve button shows an error and stays open on failure (Y4)', (tester) async {
    final state = const ChatTaskState(
      listId: 'L1',
      phase: 'awaiting-plan-approval',
      mode: 'planlayıcı',
    );
    await _pump(tester, state, adapter: _FakePlanAdapterApproveFails());

    await tester.tap(find.text(L10n.t('task_card_view_approve_plan')));
    // The activity block's own "alive" pulse AnimationController repeats
    // forever, so pumpAndSettle() never converges here — pump a fixed
    // number of frames instead, long enough for the modal bottom sheet's
    // own entrance transition to finish settling into place.
    for (var i = 0; i < 10; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }
    expect(find.text(L10n.t('task_plan_review_title')), findsOneWidget,
        reason: 'sheet must be open before we test the approve button');

    await tester.tap(find.text(L10n.t('task_plan_approve_run')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    expect(find.text(L10n.t('task_plan_review_title')), findsOneWidget,
        reason: 'a failed approve must leave the plan sheet open, not close it as if it succeeded');
    expect(find.textContaining('approve failed'), findsOneWidget,
        reason: 'a failed approve must surface the error (SnackBar), not silently swallow it');
  });
}
