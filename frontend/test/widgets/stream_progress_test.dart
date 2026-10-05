import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/models/stream_timing.dart';
import 'package:memo_flutter/widgets/stream_progress.dart';

Widget _wrap(Widget w) => MaterialApp(home: Scaffold(body: w));

/// A long turn used to show nothing that moved: after a tool finished the
/// status sat on a static "done ✓" while the model worked out its next step
/// (minutes, with a real model), which read as "stuck".
void main() {
  test('after a tool finished, the phase is "thinking", not "done"', () {
    final events = [
      AgentEvent.fromJson({'type': 'tool_executing', 'tool': 'read_file'}),
      AgentEvent.fromJson({'type': 'tool_result', 'tool': 'read_file'}),
    ];
    expect(streamPhaseLabel(events: events), L10n.t('progress_thinking'));
    expect(
      streamPhaseLabel(events: events.sublist(0, 1)),
      L10n.t('progress_running_tool', {'tool': L10n.t('tool_label_file')}),
    );
    expect(streamPhaseLabel(content: 'partial'), L10n.t('progress_writing'));
    expect(streamPhaseLabel(statusText: 'web_search'), L10n.t('searching_web'));
  });

  test('formatElapsed', () {
    expect(formatElapsed(const Duration(seconds: 5)), '0:05');
    expect(formatElapsed(const Duration(minutes: 3, seconds: 7)), '3:07');
    expect(formatElapsed(const Duration(hours: 1, minutes: 2, seconds: 3)), '1:02:03');
  });

  testWidgets('shows the phase and the elapsed time; no warning while the backend is talking',
      (tester) async {
    final start = DateTime(2026, 9, 29, 12, 0, 0);
    final now = start.add(const Duration(minutes: 1, seconds: 45));
    await tester.pumpWidget(_wrap(StreamProgressLine(
      timing: StreamTiming(startedAt: start, lastActivity: now.subtract(const Duration(seconds: 8))),
      phase: L10n.t('progress_thinking'),
      now: () => now,
    )));
    expect(find.text('${L10n.t('progress_thinking')} · 1:45'), findsOneWidget);
    expect(find.textContaining(L10n.t('progress_no_news', {'s': '8'})), findsNothing);
  });

  testWidgets('warns once the backend has been completely silent for 30s',
      (tester) async {
    final start = DateTime(2026, 9, 29, 12, 0, 0);
    final now = start.add(const Duration(minutes: 2));
    await tester.pumpWidget(_wrap(StreamProgressLine(
      timing: StreamTiming(startedAt: start, lastActivity: now.subtract(const Duration(seconds: 42))),
      phase: L10n.t('progress_thinking'),
      now: () => now,
    )));
    expect(find.text(L10n.t('progress_no_news', {'s': '42'})), findsOneWidget);
  });
}
