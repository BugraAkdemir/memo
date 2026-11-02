import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/widgets/autonomy_panel.dart';
import 'package:memo_flutter/widgets/briefing_panel.dart';

Widget host(Widget child) => MaterialApp(
  home: Scaffold(body: SingleChildScrollView(child: child)),
);

const morning = {
  'key': 'morning:2026-11-03',
  'kind': 'morning',
  'title': '2026-11-03',
  'read': false,
  'sections': [
    {
      'heading': 'meetings_today',
      'items': ['14:00 Backend toplantısı'],
    },
    {
      'heading': 'overnight',
      'items': [],
      'counts': {'pending': 2, 'skipped': 1},
    },
    {
      'heading': 'unfinished_code',
      'items': ['Memo (3): a.go, b.go, c.go'],
    },
  ],
};

const meeting = {
  'key': 'meeting:abc',
  'kind': 'meeting',
  'title': 'Ali ile mimari',
  'read': true,
  'sections': [
    {
      'heading': 'meeting_when',
      'items': ['15:00–16:00'],
    },
    {
      'heading': 'meeting_unlinked',
      'items': [],
    },
  ],
};

void main() {
  for (final locale in MemoLocale.values) {
    group('briefing panel ($locale)', () {
      setUp(() => L10n.setLocale(locale));
      tearDown(() => L10n.setLocale(MemoLocale.en));

      testWidgets('both briefings are separate opt-ins that keep each other', (
        tester,
      ) async {
        Map<String, dynamic>? saved;
        await tester.pumpWidget(
          host(
            BriefingPanel(
              settings: const {
                'morning': false,
                'morning_time': '09:00',
                'pre_meeting': false,
                'pre_meeting_minutes': 15,
              },
              briefings: const [],
              busy: false,
              onSettings: (v) => saved = v,
              onRead: (_) {},
            ),
          ),
        );
        expect(find.text(L10n.t('briefing_title')), findsOneWidget);
        expect(find.text(L10n.t('briefing_help')), findsOneWidget);
        await tester.tap(find.byKey(const ValueKey('briefing-morning')));
        await tester.pump();
        expect(saved, {
          'morning': true,
          'morning_time': '09:00',
          'pre_meeting': false,
          'pre_meeting_minutes': 15,
        });
        await tester.tap(find.byKey(const ValueKey('briefing-meeting')));
        await tester.pump();
        expect(saved?['pre_meeting'], true);
        expect(saved?['morning'], false);
      });

      testWidgets(
        'time and lead time are chosen from lists and an odd stored value survives',
        (tester) async {
          Map<String, dynamic>? saved;
          await tester.pumpWidget(
            host(
              BriefingPanel(
                settings: const {
                  'morning': true,
                  'morning_time': '08:15',
                  'pre_meeting': true,
                  'pre_meeting_minutes': 20,
                },
                briefings: const [],
                busy: false,
                onSettings: (v) => saved = v,
                onRead: (_) {},
              ),
            ),
          );
          expect(find.text('08:15'), findsOneWidget);
          expect(
            find.text(L10n.t('briefing_minutes_value', {'m': '20'})),
            findsOneWidget,
          );
          await tester.tap(find.byKey(const ValueKey('briefing-time')));
          await tester.pumpAndSettle();
          await tester.tap(find.text('07:30').last);
          await tester.pumpAndSettle();
          expect(saved?['morning_time'], '07:30');
          expect(saved?['pre_meeting_minutes'], 20);
          await tester.tap(find.byKey(const ValueKey('briefing-minutes')));
          await tester.pumpAndSettle();
          await tester.tap(
            find.text(L10n.t('briefing_minutes_value', {'m': '30'})).last,
          );
          await tester.pumpAndSettle();
          expect(saved?['pre_meeting_minutes'], 30);
        },
      );

      testWidgets('controls are disabled while busy', (tester) async {
        Map<String, dynamic>? saved;
        await tester.pumpWidget(
          host(
            BriefingPanel(
              settings: const {'morning': false},
              briefings: const [],
              busy: true,
              onSettings: (v) => saved = v,
              onRead: (_) {},
            ),
          ),
        );
        await tester.tap(
          find.byKey(const ValueKey('briefing-morning')),
          warnIfMissed: false,
        );
        await tester.pump();
        expect(saved, isNull);
      });

      testWidgets(
        'cards are translated and opening an unread one marks it read',
        (tester) async {
          final read = <String>[];
          await tester.pumpWidget(
            host(
              BriefingPanel(
                settings: const {},
                briefings: const [morning, meeting],
                busy: false,
                onSettings: (_) {},
                onRead: read.add,
              ),
            ),
          );
          final morningTitle = L10n.t('briefing_morning_title', {
            'date': '2026-11-03',
          });
          final meetingTitle = L10n.t('briefing_meeting_title', {
            'title': 'Ali ile mimari',
          });
          expect(find.text(morningTitle), findsOneWidget);
          expect(find.text(meetingTitle), findsOneWidget);
          // Sections stay collapsed until opened.
          expect(find.textContaining('Backend toplantısı'), findsNothing);
          await tester.tap(find.text(morningTitle));
          await tester.pumpAndSettle();
          expect(
            find.text(L10n.t('briefing_h_meetings_today')),
            findsOneWidget,
          );
          expect(find.textContaining('Backend toplantısı'), findsOneWidget);
          expect(
            find.text(L10n.t('briefing_h_unfinished_code')),
            findsOneWidget,
          );
          expect(find.textContaining('Memo (3)'), findsOneWidget);
          // Counts use the existing state names, never a raw code.
          expect(
            find.textContaining(L10n.t('auto_state_pending')),
            findsOneWidget,
          );
          expect(read, ['morning:2026-11-03']);
          // A card that was already read is not reported again.
          await tester.tap(find.text(meetingTitle));
          await tester.pumpAndSettle();
          expect(
            find.text(L10n.t('briefing_h_meeting_unlinked')),
            findsOneWidget,
          );
          expect(read, ['morning:2026-11-03']);
        },
      );

      testWidgets('shows which time zone the briefing time is read in', (
        tester,
      ) async {
        await tester.pumpWidget(
          host(
            BriefingPanel(
              settings: const {'morning': true, 'morning_time': '09:00'},
              timezone: 'UTC',
              briefings: const [],
              busy: false,
              onSettings: (_) {},
              onRead: (_) {},
            ),
          ),
        );
        expect(
          find.text(L10n.t('briefing_timezone', {'tz': 'UTC'})),
          findsOneWidget,
        );
      });

      testWidgets('with nothing yet it says so', (tester) async {
        await tester.pumpWidget(
          host(
            BriefingPanel(
              settings: const {},
              briefings: const [],
              busy: false,
              onSettings: (_) {},
              onRead: (_) {},
            ),
          ),
        );
        expect(find.text(L10n.t('briefing_empty')), findsOneWidget);
      });
    });
  }

  testWidgets('the autonomy panel hosts the briefing panel only when wired', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(900, 2200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    Widget panel({bool wired = true}) => MaterialApp(
      home: Scaffold(
        body: AutonomyPanel(
          settings: const {'enabled': false, 'mode': 'draft_only'},
          actions: const [],
          busy: false,
          onSettings: (_) {},
          onCancel: (_) {},
          briefing: wired ? const {'morning': false} : const {},
          briefings: const [],
          onBriefing: wired ? (_) {} : null,
          onBriefingRead: wired ? (_) {} : null,
        ),
      ),
    );
    await tester.pumpWidget(panel(wired: false));
    expect(find.text(L10n.t('briefing_title')), findsNothing);
    await tester.pumpWidget(panel());
    await tester.scrollUntilVisible(
      find.text(L10n.t('briefing_title')),
      300,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.text(L10n.t('briefing_title')), findsOneWidget);
  });
}
