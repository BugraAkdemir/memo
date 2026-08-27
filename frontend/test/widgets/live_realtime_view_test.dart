import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/providers/live_realtime_session_provider.dart';
import 'package:memo_flutter/widgets/live_realtime_view.dart';

Widget _wrap(Widget child) {
  return ProviderScope(
    child: MaterialApp(
      theme: MemoTheme.themeData,
      darkTheme: MemoTheme.darkThemeData,
      home: Scaffold(body: child),
    ),
  );
}

void main() {
  // These widgets animate continuously (the breathing orb, the pulsing
  // live-pill dot) for as long as a native session is connected — never
  // use pumpAndSettle here, it would hang waiting for an animation that by
  // design never stops. A fixed handful of pump() calls is enough to prove
  // the widget builds and keeps rendering across a few frames without
  // throwing.

  testWidgets('renders with no messages, showing the empty hint', (tester) async {
    await tester.pumpWidget(_wrap(const LiveRealtimeView(messages: [], apiBaseUrl: 'http://127.0.0.1:8090')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    // L10n defaults to English when no locale has been explicitly set.
    expect(find.text('Start talking — Memo is listening'), findsOneWidget);
  });

  testWidgets('renders transcript messages as normal chat bubbles', (tester) async {
    final messages = [
      const ChatMessage(role: 'user', content: 'Merhaba!', timestamp: '2026-08-27T03:00:00.000Z'),
      const ChatMessage(
        role: 'assistant',
        content: 'Merhaba, nasıl yardımcı olabilirim?',
        timestamp: '2026-08-27T03:00:01.000Z',
      ),
    ];
    await tester.pumpWidget(_wrap(LiveRealtimeView(messages: messages, apiBaseUrl: 'http://127.0.0.1:8090')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.text('Merhaba!'), findsOneWidget);
    expect(find.text('Merhaba, nasıl yardımcı olabilirim?'), findsOneWidget);
    // The empty-hint text must not show once real messages exist.
    expect(find.text('Start talking — Memo is listening'), findsNothing);
  });

  testWidgets('shows the connecting label while the session is connecting', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          // Constructs the real notifier (it needs a working Ref for its
          // own internals) but sets its state directly rather than going
          // through connect(), which would try to open a real WebSocket.
          liveRealtimeSessionProvider.overrideWith(
            (ref) => LiveRealtimeSessionNotifier(ref)
              ..state = const LiveRealtimeSessionState(status: LiveRealtimeSessionStatus.connecting),
          ),
        ],
        child: MaterialApp(
          theme: MemoTheme.themeData,
          home: const Scaffold(body: LiveRealtimeView(messages: [], apiBaseUrl: 'http://127.0.0.1:8090')),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.text('Connecting…'), findsWidgets);
  });
}
