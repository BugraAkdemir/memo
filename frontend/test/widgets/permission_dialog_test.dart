import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/agent/permission_dialog.dart';

// Regression test for BUG-M3: PermissionDialog._submit used to fire the
// POST unawaited and pop the dialog unconditionally, regardless of whether
// the request actually reached the backend. If it failed, the dialog just
// vanished — no error, and the backend's tool call stayed blocked waiting
// for a decision it never received. It must now stay open and show an
// error instead of silently closing.
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

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          // Port 1 is a privileged port nothing listens on in a test
          // sandbox — connecting fails fast and reliably with no mocking
          // library needed.
          apiClientProvider.overrideWithValue(
            MemoApiClient(baseUrl: 'http://127.0.0.1:1'),
          ),
        ],
        child: MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showDialog<void>(
                  context: context,
                  builder: (_) => const PermissionDialog(event: event),
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.byType(PermissionDialog), findsOneWidget);

    await tester.tap(find.text('Izin Ver'));
    await tester.pump();
    await tester.pumpAndSettle(const Duration(seconds: 2));

    expect(find.byType(PermissionDialog), findsOneWidget,
        reason: 'dialog must not pop when the permission POST fails');
    expect(find.textContaining('İzin gönderilemedi'), findsOneWidget);
  });
}
