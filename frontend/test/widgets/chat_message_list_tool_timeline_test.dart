import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/widgets/agent/agent_timeline.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

/// A saved agent turn shows what it did as a timeline: the model's words and
/// the tools it ran, grouped into one collapsed line per run that opens to the
/// individual commands.
///
/// Regression from the badge era, kept in spirit: pipeline.go's tool loop
/// emits `tool_executing` immediately followed by its `tool_result`/
/// `tool_error`, and one real call used to render as two badges — it looked
/// exactly like the model repeating a tool call it never repeated. One call is
/// one row. A trailing `tool_executing` with nothing after it is a call that
/// never finished (the stream was interrupted), shown as such.
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

  Finder header(String text) => find.text(text);

  testWidgets('one paired call is one command, not two', (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'list_directory', args: {'path': '.'}),
      AgentEvent(type: 'tool_result', toolName: 'list_directory', result: 'a.go', args: {'path': '.'}),
    ]));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(header(L10n.t('agent_ran_one')), findsOneWidget);
    await tester.tap(header(L10n.t('agent_ran_one')));
    await tester.pumpAndSettle();
    expect(find.textContaining('list_directory', findRichText: true), findsOneWidget);
    expect(find.byIcon(Icons.check_circle_outline), findsOneWidget);
  });

  testWidgets('two paired, both-erroring calls are two rows (not four) and both read as errors',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'get_file_info'),
      AgentEvent(type: 'tool_error', toolName: 'get_file_info', error: 'no such file: a.txt'),
      AgentEvent(type: 'tool_executing', toolName: 'get_file_info'),
      AgentEvent(type: 'tool_error', toolName: 'get_file_info', error: 'no such file: b.txt'),
    ]));
    await tester.pumpAndSettle();

    final title =
        '${L10n.t('agent_ran_many', {'n': '2'})} · ${L10n.t('agent_group_errors', {'n': '2'})}';
    expect(header(title), findsOneWidget);
    await tester.tap(header(title));
    await tester.pumpAndSettle();
    expect(find.textContaining('get_file_info', findRichText: true), findsNWidgets(2));
    expect(find.byIcon(Icons.error_outline), findsNWidgets(2));
    expect(find.byIcon(Icons.cancel_outlined), findsNothing);
  });

  testWidgets('a trailing unpaired "executing" is an interrupted call', (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'read_file', args: {'path': 'a.go'}),
    ]));
    await tester.pumpAndSettle();

    await tester.tap(header(L10n.t('agent_ran_one')));
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.cancel_outlined), findsOneWidget);
    expect(find.textContaining('a.go', findRichText: true), findsOneWidget);
  });

  testWidgets('what exactly each call did is visible once the group is opened',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'tool_executing', toolName: 'run_command', args: {'command': 'go test ./...'}),
      AgentEvent(
          type: 'tool_result',
          toolName: 'run_command',
          args: {'command': 'go test ./...'},
          result: 'ok  memo/internal/agent'),
    ]));
    await tester.pumpAndSettle();

    // Collapsed: the command is not on screen yet.
    expect(find.textContaining('go test', findRichText: true), findsNothing);
    await tester.tap(header(L10n.t('agent_ran_one')));
    await tester.pumpAndSettle();
    expect(find.textContaining('go test ./...', findRichText: true), findsOneWidget);
    // Its output opens on tap.
    expect(find.textContaining('memo/internal/agent'), findsNothing);
    await tester.tap(find.textContaining('go test ./...', findRichText: true));
    await tester.pumpAndSettle();
    expect(find.textContaining('memo/internal/agent'), findsOneWidget);
  });

  testWidgets('the model\'s words come first, in order with the tools',
      (tester) async {
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'narration', content: 'Önce yapılandırmaya bakıyorum.'),
      AgentEvent(type: 'tool_executing', toolName: 'read_file'),
      AgentEvent(type: 'tool_result', toolName: 'read_file', result: 'x'),
      AgentEvent(type: 'narration', content: 'Şimdi testi çalıştırıyorum.'),
      AgentEvent(type: 'tool_executing', toolName: 'run_command'),
      AgentEvent(type: 'tool_result', toolName: 'run_command', result: 'ok'),
    ]));
    await tester.pumpAndSettle();

    final first = tester.getTopLeft(find.textContaining('yapılandırmaya', findRichText: true)).dy;
    final second = tester.getTopLeft(find.textContaining('testi çalıştırıyorum', findRichText: true)).dy;
    final groupHeaders = find.text(L10n.t('agent_ran_one'));
    expect(groupHeaders, findsNWidgets(2));
    final g1 = tester.getTopLeft(groupHeaders.first).dy;
    expect(first < g1 && g1 < second, isTrue,
        reason: 'text, then its tools, then the next text');
  });

  testWidgets('a mixed message keeps three real calls as three rows, not six',
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

    final title =
        '${L10n.t('agent_ran_many', {'n': '3'})} · ${L10n.t('agent_group_errors', {'n': '1'})}';
    await tester.tap(header(title));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(find.byIcon(Icons.check_circle_outline), findsNWidgets(2));
    expect(find.byIcon(Icons.error_outline), findsOneWidget);
  });

  testWidgets('live: the running tool is named in the header without opening anything',
      (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: AgentTimeline(
          live: true,
          events: const [
            AgentEvent(type: 'narration', content: 'Testleri çalıştırıyorum.'),
            AgentEvent(type: 'tool_executing', toolName: 'run_command', args: {'command': 'go test ./...'}),
          ],
          liveNarration: 'Sonuca bakıyorum',
          buildText: (c, t) => Text(t),
        ),
      ),
    ));
    await tester.pump();

    expect(find.textContaining('go test ./...'), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('Testleri çalıştırıyorum.'), findsOneWidget);
    expect(find.text('Sonuca bakıyorum'), findsOneWidget);
  });

  testWidgets('a saved reply says which skill it used, with the skill description as tooltip',
      (tester) async {
    L10n.setLocale(MemoLocale.en);
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'skill_used', toolName: 'Pair Programming', content: 'Driver and navigator'),
    ]));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('Using skill: Pair Programming'), findsOneWidget);
    expect(find.byTooltip('Driver and navigator'), findsOneWidget);
    // The reply text itself is untouched.
    expect(find.textContaining('tamam', findRichText: true), findsOneWidget);
  });

  testWidgets('the skill line reads in Turkish too', (tester) async {
    L10n.setLocale(MemoLocale.tr);
    addTearDown(() => L10n.setLocale(MemoLocale.en));
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'skill_used', toolName: 'codebase-memory'),
    ]));
    await tester.pumpAndSettle();
    expect(find.text('Skill kullanılıyor: codebase-memory'), findsOneWidget);
  });

  testWidgets('a skill that was NOT applied says so instead of claiming it is in use', (tester) async {
    L10n.setLocale(MemoLocale.en);
    await tester.pumpWidget(harness(const [
      AgentEvent(type: 'skill_skipped', toolName: 'codebase-memory', content: 'minimal_mode'),
      AgentEvent(type: 'skill_skipped', toolName: 'plan', content: 'not_saved'),
    ]));
    await tester.pumpAndSettle();
    expect(find.textContaining('Skill not used: codebase-memory'), findsOneWidget);
    expect(find.textContaining('Could not switch the skill on for this chat: plan'), findsOneWidget);
    expect(find.textContaining('Using skill'), findsNothing);
  });
}
