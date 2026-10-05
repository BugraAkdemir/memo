import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/backend_unreachable_view.dart';

/// Found live on the web build: after changing the server address, the
/// "restarting automatically in Ns" countdown went on into negative numbers
/// ("-6s") because the restart (dart:io exit) failed on web and the timer
/// was never stopped. The countdown must fire the restart exactly once, at
/// zero, and stop there.
void main() {
  testWidgets('the auto-restart countdown fires once at zero and stops',
      (tester) async {
    var restarts = 0;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: RestartRequiredDialog(onRestart: () => restarts++),
      ),
    ));

    for (var i = 0; i < 15; i++) {
      await tester.pump(const Duration(seconds: 1));
      expect(find.textContaining('-'), findsNothing,
          reason: 'the countdown must never show a negative number');
    }
    expect(restarts, 1);
    expect(find.text(L10n.t('restart_in_seconds', {'s': '0'})), findsOneWidget);
  });

  testWidgets('Restart Now restarts immediately', (tester) async {
    var restarts = 0;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: RestartRequiredDialog(onRestart: () => restarts++),
      ),
    ));
    await tester.tap(find.text(L10n.t('restart_now_button')));
    await tester.pump();
    expect(restarts, 1);
  });
}
