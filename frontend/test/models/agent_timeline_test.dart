import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/models/agent_timeline.dart';

void main() {
  group('buildTimeline', () {
    test('pairs the stored executing+result shape and the streamed result-only shape alike', () {
      final stored = buildTimeline(const [
        AgentEvent(type: 'tool_executing', toolName: 'read_file'),
        AgentEvent(type: 'tool_result', toolName: 'read_file', result: 'x'),
      ], live: false);
      final streamed = buildTimeline(const [
        AgentEvent(type: 'tool_result', toolName: 'read_file', result: 'x'),
      ], live: true);
      for (final t in [stored, streamed]) {
        expect(t, hasLength(1));
        final g = t.single as ToolGroupItem;
        expect(g.calls, hasLength(1));
        expect(g.calls.single.status, ToolCallStatus.ok);
      }
    });

    test('narration splits tool groups and keeps order', () {
      final t = buildTimeline(const [
        AgentEvent(type: 'narration', content: 'one'),
        AgentEvent(type: 'tool_result', toolName: 'a'),
        AgentEvent(type: 'tool_result', toolName: 'b'),
        AgentEvent(type: 'narration', content: 'two'),
        AgentEvent(type: 'tool_result', toolName: 'c'),
      ], live: false);
      expect(t.map((e) => e.runtimeType).toList(),
          [NarrationItem, ToolGroupItem, NarrationItem, ToolGroupItem]);
      expect((t[1] as ToolGroupItem).calls, hasLength(2));
    });

    test('a trailing executing is running while live and interrupted afterwards', () {
      const ev = [AgentEvent(type: 'tool_executing', toolName: 'run_command')];
      expect((buildTimeline(ev, live: true).single as ToolGroupItem).hasRunning, isTrue);
      final done = buildTimeline(ev, live: false).single as ToolGroupItem;
      expect(done.hasRunning, isFalse);
      expect(done.calls.single.status, ToolCallStatus.interrupted);
    });

    test('accepts decoded-JSON maps (history) as well as AgentEvent', () {
      final t = buildTimeline([
        {'type': 'narration', 'content': 'hi'},
        {'type': 'tool_executing', 'tool': 'read_file'},
        {'type': 'tool_error', 'tool': 'read_file', 'error': 'nope'},
      ], live: false);
      expect(t, hasLength(2));
      expect((t[1] as ToolGroupItem).errorCount, 1);
    });

    test('ignores events that are not part of the story', () {
      final t = buildTimeline(const [
        AgentEvent(type: 'permission_request'),
        AgentEvent(type: 'final_response', content: 'done'),
        AgentEvent(type: 'narration_delta', content: 'x'),
        AgentEvent(type: 'narration', content: '   '),
      ], live: false);
      expect(t, isEmpty);
    });

    test('a denied call is shown as denied', () {
      final g = buildTimeline(const [
        AgentEvent(type: 'permission_denied', toolName: 'run_command'),
      ], live: false).single as ToolGroupItem;
      expect(g.calls.single.status, ToolCallStatus.denied);
    });
  });

  group('foldAgentEvent', () {
    test('a result replaces its executing placeholder; narration is kept in order', () {
      var list = <AgentEvent>[];
      list = foldAgentEvent(list, const AgentEvent(type: 'narration', content: 'n'));
      list = foldAgentEvent(list, const AgentEvent(type: 'tool_executing', toolName: 'a'));
      expect(list.map((e) => e.type), ['narration', 'tool_executing']);
      list = foldAgentEvent(list, const AgentEvent(type: 'tool_result', toolName: 'a'));
      expect(list.map((e) => e.type), ['narration', 'tool_result']);
    });

    test('live deltas are not kept', () {
      final list = foldAgentEvent(const [], const AgentEvent(type: 'narration_delta', content: 'x'));
      expect(list, isEmpty);
    });
  });

  group('toolArgSummary', () {
    test('picks the meaningful argument per tool', () {
      expect(toolArgSummary('run_command', {'command': 'go test ./...', 'cwd': '/x'}), 'go test ./...');
      expect(toolArgSummary('read_file', {'path': 'a/b.go'}), 'a/b.go');
      expect(toolArgSummary('search_content', {'pattern': 'func main', 'path': '.'}), 'func main');
      expect(toolArgSummary('web_search', {'query': 'dart sealed'}), 'dart sealed');
    });

    test('reads args sent as a JSON string, even twice-encoded', () {
      expect(toolArgSummary('read_file', '{"path":"a.go"}'), 'a.go');
      expect(toolArgSummary('read_file', '"{\\"path\\":\\"a.go\\"}"'), 'a.go');
    });

    test('is one line and bounded', () {
      final s = toolArgSummary('run_command', {'command': 'echo a\n   echo b ${'x' * 400}'});
      expect(s.contains('\n'), isFalse);
      expect(s.length, lessThanOrEqualTo(161));
      expect(s.startsWith('echo a echo b'), isTrue);
    });

    test('malformed or missing args give no summary, never throw', () {
      expect(toolArgSummary('read_file', null), '');
      expect(toolArgSummary('read_file', 'not json'), '');
      expect(toolArgSummary('read_file', 42), '');
    });
  });
}
