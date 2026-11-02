import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/autonomy_panel.dart';
import 'package:memo_flutter/widgets/follow_up_radar_card.dart';

Widget host(Widget child) => MaterialApp(
  home: Scaffold(body: SingleChildScrollView(child: child)),
);

void main() {
  for (final locale in MemoLocale.values) {
    group('follow-up radar card ($locale)', () {
      setUp(() => L10n.setLocale(locale));
      tearDown(() => L10n.setLocale(MemoLocale.en));

      testWidgets('switch and wait time are saved as one settings map', (
        tester,
      ) async {
        Map<String, dynamic>? saved;
        await tester.pumpWidget(
          host(
            FollowUpRadarCard(
              radar: const {'enabled': false, 'follow_up_hours': 48},
              waiting: const [],
              busy: false,
              onChanged: (v) => saved = v,
            ),
          ),
        );
        // Off by default, with the promise that nothing is sent on its own.
        expect(find.text(L10n.t('radar_title')), findsOneWidget);
        expect(find.text(L10n.t('radar_help')), findsOneWidget);
        await tester.tap(find.byType(Switch));
        await tester.pump();
        expect(saved, {'enabled': true, 'follow_up_hours': 48});

        await tester.tap(find.byKey(const ValueKey('radar-hours')));
        await tester.pumpAndSettle();
        await tester.tap(
          find.text(L10n.t('radar_hours_value', {'h': '72'})).last,
        );
        await tester.pumpAndSettle();
        expect(saved, {'enabled': false, 'follow_up_hours': 72});
      });

      testWidgets('an unusual stored wait time is still shown and kept', (
        tester,
      ) async {
        await tester.pumpWidget(
          host(
            FollowUpRadarCard(
              radar: const {'enabled': true, 'follow_up_hours': 36},
              waiting: const [],
              busy: false,
              onChanged: (_) {},
            ),
          ),
        );
        expect(
          find.text(L10n.t('radar_hours_value', {'h': '36'})),
          findsOneWidget,
        );
      });

      testWidgets('controls are disabled while busy', (tester) async {
        Map<String, dynamic>? saved;
        await tester.pumpWidget(
          host(
            FollowUpRadarCard(
              radar: const {'enabled': false, 'follow_up_hours': 48},
              waiting: const [],
              busy: true,
              onChanged: (v) => saved = v,
            ),
          ),
        );
        await tester.tap(find.byType(Switch), warnIfMissed: false);
        await tester.pump();
        expect(saved, isNull);
      });

      testWidgets('waiting list shows who, what and when but no message', (
        tester,
      ) async {
        await tester.pumpWidget(
          host(
            FollowUpRadarCard(
              radar: const {'enabled': true, 'follow_up_hours': 48},
              waiting: const [
                {
                  'id': 'c1',
                  'address': 'ali@example.com',
                  'subject': 'Teklif',
                  'status': 'open',
                  'due_at': '2026-11-04T12:30:00Z',
                },
                {
                  'id': 'c2',
                  'address': 'veli@example.com',
                  'subject': '',
                  'status': 'reminded',
                  'due_at': '2026-11-03T08:00:00Z',
                },
              ],
              busy: false,
              onChanged: (_) {},
            ),
          ),
        );
        expect(
          find.text(L10n.t('radar_waiting_title', {'n': '2'})),
          findsOneWidget,
        );
        expect(find.textContaining('ali@example.com'), findsOneWidget);
        expect(find.textContaining('Teklif'), findsOneWidget);
        expect(find.textContaining('veli@example.com'), findsOneWidget);
        expect(find.text(L10n.t('radar_status_reminded')), findsOneWidget);
        expect(find.text(L10n.t('radar_status_open')), findsOneWidget);
      });

      testWidgets('empty list explains itself', (tester) async {
        await tester.pumpWidget(
          host(
            FollowUpRadarCard(
              radar: const {'enabled': true, 'follow_up_hours': 48},
              waiting: const [],
              busy: false,
              onChanged: (_) {},
            ),
          ),
        );
        expect(find.text(L10n.t('radar_waiting_empty')), findsOneWidget);
      });
    });
  }

  testWidgets('the autonomy panel hosts the radar card', (tester) async {
    Map<String, dynamic>? saved;
    tester.view.physicalSize = const Size(900, 1600);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AutonomyPanel(
            settings: const {'enabled': false, 'mode': 'draft_only'},
            actions: const [],
            busy: false,
            onSettings: (_) {},
            onCancel: (_) {},
            radar: const {'enabled': false, 'follow_up_hours': 48},
            waiting: const [],
            onRadar: (v) => saved = v,
          ),
        ),
      ),
    );
    await tester.scrollUntilVisible(
      find.text(L10n.t('radar_title')),
      300,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.text(L10n.t('radar_title')), findsOneWidget);
    expect(saved, isNull);
  });

  testWidgets('without a radar handler the panel does not show the card', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AutonomyPanel(
            settings: const {'enabled': false, 'mode': 'draft_only'},
            actions: const [],
            busy: false,
            onSettings: (_) {},
            onCancel: (_) {},
          ),
        ),
      ),
    );
    expect(find.text(L10n.t('radar_title')), findsNothing);
  });
}
