import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

/// Regression tests for a real, reproducible UX bug found by driving the app
/// directly (a fake-provider "playground" session, not a hypothetical):
/// pipeline.go's tool loop (internal/agent/pipeline.go) always emits
/// EventToolExecuting immediately followed by EventToolResult/EventToolError
/// for the same call, sequentially, never interleaved — but the chat bubble
/// rendered BOTH as separate badges. One real list_directory call read as
/// two "list_directory" pills side by side; two real get_file_info calls
/// (both erroring on missing files) read as four — looking exactly like the
/// model repeated its own tool calls, which it never did.
///
/// _visibleToolBadges (chat_message_list.dart, private — these tests exercise
/// it only through the public widget, matching this file's existing
/// memoryUsed test's own approach) collapses an executing+completion pair
/// into the single final-state badge, and leaves a genuinely unpaired
/// trailing "executing" alone (still in flight / stream interrupted).
void main() {
  Widget harness(List<dynamic> agentEvents) => MaterialApp(
        home: Scaffold(
          body: ChatMessageList(
            apiBaseUrl: 'http://localhost:8090',
            messages: [
              ChatMessage(
                role: 'assistant',
                content: 'tamam',
                timestamp: '12:00',
                agentEvents: agentEvents,
              ),
            ],
          ),
        ),
      );

  testWidgets('one paired tool call renders exactly one badge, not two',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'list_directory'),
      AgentEvent(type: 'tool_result', toolName: 'list_directory', result: 'ok'),
    ]));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    // 'list_directory' has no special-cased label in _AgentStatusBadge, so it
    // renders as its own literal tool name — exactly one occurrence, not two.
    expect(find.text('list_directory'), findsOneWidget);
  });

  testWidgets('two paired, both-erroring calls render exactly two badges, not four',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'get_file_info'),
      AgentEvent(type: 'tool_error', toolName: 'get_file_info', error: 'no such file: a.txt'),
      AgentEvent(type: 'tool_executing', toolName: 'get_file_info'),
      AgentEvent(type: 'tool_error', toolName: 'get_file_info', error: 'no such file: b.txt'),
    ]));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('get_file_info'), findsNWidgets(2));
    // Both must render as errors (red, error icon) — the completion event's
    // state must win, not the transient executing one.
    expect(find.byIcon(Icons.error_outline), findsNWidgets(2));
    expect(find.byIcon(Icons.cancel_outlined), findsNothing);
  });

  testWidgets('a trailing unpaired "executing" is kept, shown as interrupted',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'read_file'),
    ]));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    // read_file IS special-cased ('Dosya okudu') — exactly one badge, and it
    // must render in the dim/"interrupted" style (_AgentStatusBadge's own
    // documented behavior for a historical, never-completed executing event),
    // not silently dropped by the new pairing logic.
    expect(find.text('Dosya okudu'), findsOneWidget);
    expect(find.byIcon(Icons.cancel_outlined), findsOneWidget);
  });

  testWidgets('mixed message keeps three real calls as three badges, not six',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'list_directory'),
      AgentEvent(type: 'tool_result', toolName: 'list_directory', result: 'ok'),
      AgentEvent(type: 'tool_executing', toolName: 'write_file'),
      AgentEvent(type: 'tool_result', toolName: 'write_file', result: 'ok'),
      AgentEvent(type: 'tool_executing', toolName: 'run_command'),
      AgentEvent(type: 'tool_error', toolName: 'run_command', error: 'exit 1'),
    ]));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('list_directory'), findsOneWidget);
    expect(find.text('Dosya yazdi'), findsOneWidget);
    expect(find.text('Komut calistirdi'), findsOneWidget);
  });
}
