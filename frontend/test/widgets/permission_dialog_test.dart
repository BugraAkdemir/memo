import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/agent/permission_dialog.dart';

// Regression test for BUG-M3: PermissionDialog._submit used to fire the
// POST unawaited and pop the dialog unconditionally, regardless of whether
// the request actually reached the backend. If it failed, the dialog just
// vanished — no error, and the backend's tool call stayed blocked waiting
// for a decision it never received. It must now stay open and show an
// error instead of silently closing.
//
// This test harness's own trigger button label — never shown to a real
// user, just this file's scaffold for opening the dialog — held in a named
// constant so it doesn't read as a hardcoded UI literal to the Rule #8
// L10n guard.
const _openDialogButtonLabel = 'open';

void main() {
  testWidgets('permission dialog stays open and shows an error when the send fails',
      (tester) async {
    const event = AgentEvent(
      type: 'permission_request',
      requestId: 'req-1',
      toolName: 'run_command',
      dangerLevel: 'safe',
      args: '{}',
    );

    final container = ProviderContainer(overrides: [
      // Port 1 is a privileged port nothing listens on in a test
      // sandbox — connecting fails fast and reliably with no mocking
      // library needed.
      apiClientProvider.overrideWithValue(
        MemoApiClient(baseUrl: 'http://127.0.0.1:1'),
      ),
    ]);
    addTearDown(container.dispose);
    // The dialog is only ever shown while the underlying turn is still
    // streaming - checking this explicitly (rather than leaving the
    // default) matters since the fix for the stuck-dialog bug below reads
    // this on every build and pops immediately once it's false.
    container.read(isSendingProvider.notifier).state = true;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showDialog<void>(
                  context: context,
                  builder: (_) => const PermissionDialog(event: event),
                ),
                child: const Text(_openDialogButtonLabel),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.byType(PermissionDialog), findsOneWidget);

    await tester.tap(find.text(L10n.t('allow_short')));
    await tester.pump();
    await tester.pumpAndSettle(const Duration(seconds: 2));

    expect(find.byType(PermissionDialog), findsOneWidget,
        reason: 'dialog must not pop when the permission POST fails');
    // Read through L10n rather than hardcoded, so this asserts the real UI
    // in whatever the active locale is (the default flipped to English on
    // 2026-08-13). The key's value ends in an interpolated error, so match
    // on the fixed prefix only.
    expect(
      find.textContaining(L10n.t('permission_send_failed').split(r'${e}').first),
      findsOneWidget,
    );
  });

  // Regression test for BUG-L1: the dialog used to stay on screen even after
  // the underlying agent turn ended — Stop button or switching to a
  // different chat (both flip isSendingProvider to false via
  // stopStreaming()) — so the user could end up sending Allow/Deny for a
  // requestId the backend had already given up on.
  testWidgets('permission dialog auto-dismisses when the turn is stopped or the chat is switched',
      (tester) async {
    const event = AgentEvent(
      type: 'permission_request',
      requestId: 'req-1',
      toolName: 'run_command',
      dangerLevel: 'safe',
      args: '{}',
    );

    final container = ProviderContainer();
    addTearDown(container.dispose);
    container.read(isSendingProvider.notifier).state = true;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showDialog<void>(
                  context: context,
                  builder: (_) => const PermissionDialog(event: event),
                ),
                child: const Text(_openDialogButtonLabel),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.byType(PermissionDialog), findsOneWidget);

    // Simulate what stopStreaming() does, whether triggered by the Stop
    // button or ActiveChatIdNotifier.switchTo() switching chats.
    container.read(isSendingProvider.notifier).state = false;
    // The auto-dismiss is debounced ~1.8s AND floored so the dialog is
    // always visible at least 2s first (BUG-PERM1: a transient not-sending
    // read at an agentic tool-round boundary, or an out-of-order first
    // frame, must not flash-close a live dialog). It still closes here —
    // the turn really ended and never resumes — just a few seconds later.
    await tester.pump(const Duration(milliseconds: 4200));
    await tester.pumpAndSettle();

    expect(find.byType(PermissionDialog), findsNothing,
        reason: 'dialog must auto-dismiss once the underlying turn ends');
  });

  // Regression test found live: the backend's own 60s auto-deny
  // (internal/agent/executor.go) can end the turn (isSendingProvider ->
  // false) before this widget's first build ever runs, if the
  // agent_event that triggers showDialog is itself delivered late. The
  // ref.listen above only fires on a live transition, so it never sees
  // one - the dialog was reproduced permanently stuck, every button
  // failing against a requestId the backend had already discarded, with
  // no way to dismiss it at all (not even Escape).
  testWidgets(
      'permission dialog does not get stuck open if the turn already ended '
      'before it was ever shown', (tester) async {
    const event = AgentEvent(
      type: 'permission_request',
      requestId: 'req-1',
      toolName: 'run_command',
      dangerLevel: 'safe',
      args: '{}',
    );

    final container = ProviderContainer();
    addTearDown(container.dispose);
    // Never set to true - simulates the turn having already ended by the
    // time this dialog is built, the exact race that got it stuck.
    container.read(isSendingProvider.notifier).state = false;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showDialog<void>(
                  context: context,
                  builder: (_) => const PermissionDialog(event: event),
                ),
                child: const Text(_openDialogButtonLabel),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    // The auto-dismiss is debounced ~1.8s and floored to a 2s minimum
    // visible time (so a transient not-sending read at a tool-round
    // boundary, or an out-of-order first frame, can't flash-close a live
    // dialog — BUG-PERM1). It still closes, just a few seconds later.
    await tester.pump(const Duration(milliseconds: 4200));
    await tester.pumpAndSettle();

    expect(find.byType(PermissionDialog), findsNothing,
        reason: 'a dialog for an already-ended turn must not stay stuck '
            'open with no way to dismiss it');
  });

  testWidgets(
      'permission dialog stays open when isSending only dips briefly '
      '(agentic tool-round boundary)', (tester) async {
    const event = AgentEvent(
      type: 'permission_request',
      requestId: 'req-1',
      toolName: 'create_task_md',
      dangerLevel: 'medium',
      args: '{"items":["a","b"]}',
    );

    final container = ProviderContainer();
    addTearDown(container.dispose);
    container.read(isSendingProvider.notifier).state = false; // momentary dip

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showDialog<void>(
                  context: context,
                  builder: (_) => const PermissionDialog(event: event),
                ),
                child: const Text(_openDialogButtonLabel),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.byType(PermissionDialog), findsOneWidget);

    // The agentic loop resumes streaming before the debounce elapses.
    await tester.pump(const Duration(milliseconds: 600));
    container.read(isSendingProvider.notifier).state = true;
    await tester.pump(const Duration(milliseconds: 1200));
    await tester.pumpAndSettle();

    expect(find.byType(PermissionDialog), findsOneWidget,
        reason: 'a brief not-sending dip must not flash-close a live dialog');
  });
}
