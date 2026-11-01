import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/autonomy_panel.dart';

void main() {
  for (final locale in MemoLocale.values) {
    testWidgets('autonomy settings and cancellation are reviewable ($locale)', (
      tester,
    ) async {
      L10n.setLocale(locale);
      addTearDown(() => L10n.setLocale(MemoLocale.en));
      tester.view.physicalSize = const Size(480, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      Map<String, dynamic>? saved;
      String? cancelled;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AutonomyPanel(
              settings: const {
                'enabled': false,
                'mode': 'draft_only',
                'grace_seconds': 60,
              },
              actions: const [
                {
                  'id': 'a',
                  'recipient': 'friend@example.com',
                  'channel': 'mail',
                  'body': 'Review draft',
                  'state': 'pending',
                  'reason': 'drafted',
                  'conflict': true,
                  'draft_saved': true,
                },
              ],
              busy: false,
              onSettings: (v) => saved = v,
              onCancel: (v) => cancelled = v,
            ),
          ),
        ),
      );
      await tester.tap(find.byType(Switch).first);
      await tester.pump();
      expect(saved?['enabled'], isTrue);
      expect(saved?['mode'], 'draft_only');
      await tester.ensureVisible(find.text(L10n.t('auto_manual')));
      await tester.tap(find.text(L10n.t('auto_manual')));
      await tester.pumpAndSettle();
      expect(saved?['mode'], 'manual');
      await tester.scrollUntilVisible(
        find.text('Review draft'),
        300,
        scrollable: find.byType(Scrollable).first,
      );
      expect(find.text(L10n.t('auto_conflict')), findsOneWidget);
      expect(find.text(L10n.t('auto_saved')), findsOneWidget);
      await tester.scrollUntilVisible(
        find.text(L10n.t('cancel')).hitTestable(),
        150,
        scrollable: find
            .descendant(
              of: find.byType(AutonomyPanel),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text(L10n.t('cancel')));
      expect(cancelled, 'a');
      expect(tester.takeException(), isNull);
    });
  }
}
