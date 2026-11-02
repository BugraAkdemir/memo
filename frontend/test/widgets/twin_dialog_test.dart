import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/twin_dialog.dart';
import 'package:memo_flutter/widgets/autonomy_panel.dart';

class _TwinAdapter implements HttpClientAdapter {
  final calls = <RequestOptions>[];
  final contacts = <Map<String, dynamic>>[];
  final actions = <Map<String, dynamic>>[];
  bool denyGit = false;
  var preferences = <String, dynamic>{
    'timezone': 'Europe/Istanbul',
    'work_start': '09:00',
    'work_end': '18:00',
    'allow_project_inspection': false,
    'radar': {'enabled': false, 'follow_up_hours': 48},
    'briefing': {
      'morning': false,
      'morning_time': '09:00',
      'pre_meeting': false,
      'pre_meeting_minutes': 15,
    },
  };
  final briefings = <Map<String, dynamic>>[
    {
      'key': 'morning:2026-11-03',
      'kind': 'morning',
      'title': '2026-11-03',
      'read': false,
      'sections': [
        {
          'heading': 'meetings_today',
          'items': ['14:00 Backend toplantısı'],
        },
      ],
    },
  ];
  final commitments = <Map<String, dynamic>>[
    {
      'id': 'c1',
      'channel': 'mail',
      'address': 'ali@example.com',
      'subject': 'Teklif',
      'status': 'open',
      'due_at': '2026-11-04T12:30:00Z',
    },
  ];
  @override
  Future<ResponseBody> fetch(
    RequestOptions o,
    Stream<Uint8List>? request,
    Future<void>? cancel,
  ) async {
    calls.add(o);
    dynamic result = <String, dynamic>{'ok': true};
    var status = 200;
    switch (o.path) {
      case '/api/twin/state':
        result = {
          'contacts': contacts,
          'actions': actions,
          'preferences': preferences,
          'commitments': commitments,
          'briefings': briefings,
          'projects': [
            {'id': 'p_test', 'name': 'Demo repo'},
          ],
        };
      case '/api/twin/briefing_read':
        briefings.firstWhere(
          (b) => b['key'] == (o.data as Map)['key'],
        )['read'] = true;
      case '/api/twin/resolve_commitment':
        commitments.removeWhere((c) => c['id'] == (o.data as Map)['id']);
      case '/api/twin/approve_action':
        actions.firstWhere((a) => a['id'] == (o.data as Map)['id'])['state'] =
            'sent';
      case '/api/twin/cancel_action':
        actions.firstWhere((a) => a['id'] == (o.data as Map)['id'])['state'] =
            'cancelled';
      case '/api/twin/contact':
        final data = Map<String, dynamic>.from(o.data as Map);
        data['id'] = 'c_test';
        data['style'] = {
          'samples': 0,
          'tone': 'unknown',
          'language': 'unknown',
          'sufficient': false,
        };
        contacts.clear();
        contacts.add(data);
        result = {'id': 'c_test'};
      case '/api/twin/merge':
        final data = o.data as Map;
        final target = contacts.firstWhere((c) => c['id'] == data['target_id']);
        final source = contacts.firstWhere((c) => c['id'] == data['source_id']);
        target['aliases'] = [
          ...target['aliases'] as List,
          ...source['aliases'] as List,
        ];
        if (source['autonomy_policy'] == 'never')
          target['autonomy_policy'] = 'never';
        contacts.remove(source);
      case '/api/twin/preferences':
        preferences = Map<String, dynamic>.from(o.data as Map);
      case '/api/twin/contact_timeline':
        result = [
          {
            'body': 'Tomorrow at noon',
            'channel': 'mail',
            'outgoing': false,
            'sent_at': '2026-10-31T20:50:00Z',
          },
        ];
      case '/api/twin/git_project_status':
        if (denyGit) {
          status = 403;
          result = {'error': 'twin_denied'};
        } else {
          result = {
            'status': '?? sample.txt',
            'merged_branches': 'main\nfeature',
            'recent_commits': 'abc123 merge feature',
          };
        }
      case '/api/twin/check_availability':
        result = {
          'busy': [
            {'start': '2026-11-01T09:00:00Z', 'end': '2026-11-01T10:00:00Z'},
          ],
          'timezone': 'Europe/Istanbul',
          'work_start': '09:00',
          'work_end': '18:00',
        };
    }
    return ResponseBody.fromString(
      jsonEncode(result),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<_TwinAdapter> _mount(
  WidgetTester tester,
  MemoLocale locale, {
  String? initialContactId,
  _TwinAdapter? initialAdapter,
}) async {
  final previous = L10n.locale;
  L10n.setLocale(locale);
  addTearDown(() => L10n.setLocale(previous));
  tester.view.physicalSize = const Size(1440, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final adapter = initialAdapter ?? _TwinAdapter();
  final api = MemoApiClient(baseUrl: 'http://memo.test');
  api.dio.httpClientAdapter = adapter;
  final container = ProviderContainer(
    overrides: [apiClientProvider.overrideWithValue(api)],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: Scaffold(body: TwinDialog(initialContactId: initialContactId)),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return adapter;
}

void main() {
  for (final locale in MemoLocale.values) {
    testWidgets('WhatsApp approval and grace window are wired ($locale)', (
      tester,
    ) async {
      final adapter = _TwinAdapter();
      adapter.preferences['autonomy'] = {
        'enabled': true,
        'whatsapp': true,
        'mode': 'draft_only',
        'grace_seconds': 60,
      };
      adapter.actions.add({
        'id': 'wa',
        'channel': 'whatsapp',
        'state': 'pending',
        'body': 'A prepared WhatsApp reply',
        'recipient': '905551234567',
        'approval_code': 12,
        'reason': 'drafted',
      });
      await _mount(tester, locale, initialAdapter: adapter);
      await tester.tap(find.text(L10n.t('auto_tab')));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.text(L10n.t('approval_send')),
        250,
        scrollable: find
            .descendant(
              of: find.byType(AutonomyPanel),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text(L10n.t('approval_send')).hitTestable());
      await tester.pumpAndSettle();
      final call = adapter.calls.singleWhere(
        (r) => r.path == '/api/twin/approve_action',
      );
      expect(call.data, {'id': 'wa'});
      expect(find.text(L10n.t('auto_state_sent')), findsOneWidget);
      await tester.scrollUntilVisible(
        find.text(L10n.t('reply_additional_settings')),
        -250,
        scrollable: find
            .descendant(
              of: find.byType(AutonomyPanel),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.pumpAndSettle();
      await tester.tap(
        find.text(L10n.t('reply_additional_settings')).hitTestable(),
      );
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.byKey(const ValueKey('grace-seconds')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('grace-seconds')));
      await tester.pumpAndSettle();
      await tester.tap(
        find.text(L10n.t('approval_seconds', {'seconds': '120'})).last,
      );
      await tester.pumpAndSettle();
      expect((adapter.preferences['autonomy'] as Map)['grace_seconds'], 120);
      expect(tester.takeException(), isNull);
    });
  }

  for (final locale in MemoLocale.values) {
    testWidgets(
      'contact links, source dates and preferences use real API payloads ($locale)',
      (tester) async {
        final a = await _mount(tester, locale);
        expect(find.text(L10n.t('twin_scope_notice')), findsOneWidget);
        await tester.enterText(
          find.byKey(const ValueKey('twin_name')),
          'Friend',
        );
        await tester.enterText(
          find.byKey(const ValueKey('twin_address')),
          'friend@example.com',
        );
        await tester.tap(find.byTooltip(L10n.t('twin_add_alias')));
        await tester.pump();
        await tester.tap(find.text(L10n.t('save')).first);
        await tester.pumpAndSettle();
        final saved = a.calls.firstWhere((c) => c.path == '/api/twin/contact');
        expect((saved.data as Map)['aliases'], [
          {'channel': 'mail', 'address': 'friend@example.com'},
        ]);
        expect(find.text('Friend'), findsNWidgets(2));
        expect(find.text(L10n.t('twin_style_insufficient')), findsOneWidget);
        await tester.tap(find.text(L10n.t('twin_timeline')));
        await tester.pumpAndSettle();
        await tester.scrollUntilVisible(
          find.text('Tomorrow at noon'),
          180,
          scrollable: find
              .descendant(
                of: find.byKey(const ValueKey('twin_contact_editor')),
                matching: find.byType(Scrollable),
              )
              .first,
        );
        expect(find.text('Tomorrow at noon'), findsOneWidget);
        expect(find.textContaining('2026-10-31T20:50:00Z'), findsOneWidget);
        await tester.tap(find.text(L10n.t('twin_preferences_tab')));
        await tester.pumpAndSettle();
        await tester.tap(find.byType(Switch));
        await tester.pump();
        await tester.tap(find.text(L10n.t('save')).last);
        await tester.pumpAndSettle();
        expect(a.preferences['allow_project_inspection'], isTrue);
        // Saving working hours must not silently switch the radar off.
        expect(a.preferences['radar'], {
          'enabled': false,
          'follow_up_hours': 48,
        });
        expect((a.preferences['briefing'] as Map)['morning_time'], '09:00');
        expect(a.preferences['briefing'], isNotEmpty);
        expect(a.calls.every((c) => c.path.startsWith('/api/twin/')), isTrue);
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets(
    'read-only inspection and masking results, denial clears stale Git evidence',
    (tester) async {
      final a = await _mount(tester, MemoLocale.en);
      await tester.tap(find.text(L10n.t('twin_tools_tab')));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(DropdownButtonFormField<String>).last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Demo repo').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text(L10n.t('twin_inspect_git')));
      await tester.pumpAndSettle();
      expect(find.text('abc123 merge feature'), findsOneWidget);
      a.denyGit = true;
      await tester.tap(find.text(L10n.t('twin_inspect_git')));
      await tester.pumpAndSettle();
      expect(find.text('abc123 merge feature'), findsNothing);
      expect(find.text(L10n.t('twin_denied')), findsOneWidget);
      await tester.tap(find.text(L10n.t('twin_check_availability')));
      await tester.pumpAndSettle();
      expect(find.textContaining('2026-11-01T09:00:00Z'), findsOneWidget);
      final payload = a.calls.last.data as Map;
      expect(
        DateTime.parse(
          payload['to'] as String,
        ).difference(DateTime.parse(payload['from'] as String)).inHours,
        24,
      );
      expect(a.calls.any((c) => c.path.contains('send')), isFalse);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'merging contacts requires an explicit choice and retains the linked identity',
    (tester) async {
      final a = await _mount(tester, MemoLocale.tr);
      Map<String, dynamic> contact(
        String id,
        String name,
        String channel,
        String address,
      ) => {
        'id': id,
        'name': name,
        'autonomy_policy': 'draft_only',
        'aliases': [
          {'channel': channel, 'address': address},
        ],
        'style': {
          'samples': 0,
          'tone': 'unknown',
          'language': 'unknown',
          'sufficient': false,
        },
      };
      a.contacts.addAll([
        contact('a', 'Mail Friend', 'mail', 'friend@example.com'),
        contact('b', 'WA Friend', 'whatsapp', '90555@s.whatsapp.net'),
      ]);
      await tester.tap(find.byTooltip(L10n.t('refresh')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Mail Friend'));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip(L10n.t('twin_merge')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('WA Friend').last);
      await tester.pumpAndSettle();
      expect(a.calls.any((c) => c.path == '/api/twin/merge'), isFalse);
      await tester.tap(find.widgetWithText(FilledButton, L10n.t('twin_merge')));
      await tester.pumpAndSettle();
      final req = a.calls.firstWhere((c) => c.path == '/api/twin/merge');
      expect(req.data, {'target_id': 'a', 'source_id': 'b'});
      expect(a.contacts.single['aliases'], hasLength(2));
      expect(tester.takeException(), isNull);
    },
  );

  for (final locale in MemoLocale.values) {
    testWidgets('autonomy settings and local cancellation use HTTP ($locale)', (
      tester,
    ) async {
      final a = await _mount(tester, locale);
      a.actions.add({
        'id': 'pending-1',
        'recipient': 'friend@example.com',
        'channel': 'mail',
        'state': 'pending',
        'reason': 'drafted',
        'body': 'Prepared reply',
      });
      await tester.tap(find.byTooltip(L10n.t('refresh')));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text(L10n.t('auto_tab')));
      await tester.tap(find.text(L10n.t('auto_tab')));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(Switch).first);
      await tester.pumpAndSettle();
      final payload =
          a.calls.lastWhere((c) => c.path == '/api/twin/preferences').data
              as Map;
      expect((payload['autonomy'] as Map)['enabled'], true);
      expect(payload['timezone'], 'Europe/Istanbul');
      await tester.scrollUntilVisible(
        find.text('Prepared reply'),
        300,
        scrollable: find
            .descendant(
              of: find.byType(AutonomyPanel),
              matching: find.byType(Scrollable),
            )
            .first,
      );
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
      await tester.pumpAndSettle();
      expect(
        a.calls.lastWhere((c) => c.path == '/api/twin/cancel_action').data,
        {'id': 'pending-1'},
      );
      expect(find.text(L10n.t('auto_state_cancelled')), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  for (final locale in MemoLocale.values) {
    testWidgets('the radar switch saves with the other preferences ($locale)', (
      tester,
    ) async {
      final a = await _mount(tester, locale);
      await tester.ensureVisible(find.text(L10n.t('auto_tab')));
      await tester.tap(find.text(L10n.t('auto_tab')));
      await tester.pumpAndSettle();
      final scrollable = find
          .descendant(
            of: find.byType(AutonomyPanel),
            matching: find.byType(Scrollable),
          )
          .first;
      await tester.scrollUntilVisible(
        find.text(L10n.t('radar_title')),
        300,
        scrollable: scrollable,
      );
      expect(find.textContaining('ali@example.com'), findsOneWidget);
      await tester.tap(
        find
            .descendant(
              of: find.ancestor(
                of: find.text(L10n.t('radar_title')),
                matching: find.byType(Material),
              ),
              matching: find.byType(Switch),
            )
            .first,
      );
      await tester.pumpAndSettle();
      final payload =
          a.calls.lastWhere((c) => c.path == '/api/twin/preferences').data
              as Map;
      expect(payload['radar'], {'enabled': true, 'follow_up_hours': 48});
      // Everything else the owner configured travels with it.
      expect(payload['timezone'], 'Europe/Istanbul');
      expect(a.preferences['radar'], {'enabled': true, 'follow_up_hours': 48});
      expect(tester.takeException(), isNull);
    });
  }

  for (final locale in MemoLocale.values) {
    testWidgets('closing a promise calls the API and removes it ($locale)', (
      tester,
    ) async {
      final a = await _mount(tester, locale);
      a.commitments.add({
        'id': 'p1',
        'kind': 'my_promise',
        'summary': 'Raporu gönder',
        'address': 'veli@example.com',
        'status': 'reminded',
        'due_at': '2026-11-03T08:00:00Z',
      });
      await tester.tap(find.byTooltip(L10n.t('refresh')));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text(L10n.t('auto_tab')));
      await tester.tap(find.text(L10n.t('auto_tab')));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.text(L10n.t('radar_promise_done')),
        300,
        scrollable: find
            .descendant(
              of: find.byType(AutonomyPanel),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      expect(find.textContaining('Raporu gönder'), findsOneWidget);
      await tester.ensureVisible(find.text(L10n.t('radar_promise_done')));
      await tester.pumpAndSettle();
      await tester.tap(find.text(L10n.t('radar_promise_done')));
      await tester.pumpAndSettle();
      expect(
        a.calls.lastWhere((c) => c.path == '/api/twin/resolve_commitment').data,
        {'id': 'p1'},
      );
      expect(find.textContaining('Raporu gönder'), findsNothing);
      expect(tester.takeException(), isNull);
    });
  }

  for (final locale in MemoLocale.values) {
    testWidgets(
      'briefing settings save with the rest and cards are read ($locale)',
      (tester) async {
        final a = await _mount(tester, locale);
        await tester.ensureVisible(find.text(L10n.t('auto_tab')));
        await tester.tap(find.text(L10n.t('auto_tab')));
        await tester.pumpAndSettle();
        final scrollable = find
            .descendant(
              of: find.byType(AutonomyPanel),
              matching: find.byType(Scrollable),
            )
            .first;
        await tester.scrollUntilVisible(
          find.byKey(const ValueKey('briefing-morning')),
          300,
          scrollable: scrollable,
        );
        await tester.ensureVisible(
          find.byKey(const ValueKey('briefing-morning')),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const ValueKey('briefing-morning')));
        await tester.pumpAndSettle();
        final payload =
            a.calls.lastWhere((c) => c.path == '/api/twin/preferences').data
                as Map;
        expect((payload['briefing'] as Map)['morning'], true);
        expect((payload['briefing'] as Map)['morning_time'], '09:00');
        // Everything else the owner configured travels with it.
        expect(payload['timezone'], 'Europe/Istanbul');
        expect(payload['radar'], {'enabled': false, 'follow_up_hours': 48});
        // Opening an unread card reports it once.
        final title = L10n.t('briefing_morning_title', {'date': '2026-11-03'});
        await tester.scrollUntilVisible(
          find.text(title),
          300,
          scrollable: scrollable,
        );
        await tester.ensureVisible(find.text(title));
        await tester.pumpAndSettle();
        await tester.tap(find.text(title));
        await tester.pumpAndSettle();
        expect(
          a.calls.lastWhere((c) => c.path == '/api/twin/briefing_read').data,
          {'key': 'morning:2026-11-03'},
        );
        expect(a.briefings.first['read'], true);
        expect(tester.takeException(), isNull);
      },
    );
  }

  testWidgets('contact editor remains usable in a narrow window', (
    tester,
  ) async {
    await _mount(tester, MemoLocale.en);
    tester.view.physicalSize = const Size(480, 900);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('twin_name')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  for (final locale in MemoLocale.values) {
    testWidgets('all twin sections fit the reported desktop window ($locale)', (
      tester,
    ) async {
      final adapter = await _mount(tester, locale);
      adapter.contacts.add({
        'id': 'blocked',
        'name': 'Test person with a longer display name',
        'autonomy_policy': 'never',
        'aliases': [
          {'channel': 'mail', 'address': 'person@example.com'},
        ],
        'style': {
          'samples': 0,
          'tone': 'unknown',
          'language': 'unknown',
          'sufficient': false,
        },
      });
      await tester.tap(find.byTooltip(L10n.t('refresh')));
      await tester.pumpAndSettle();
      tester.view.physicalSize = const Size(1000, 600);
      await tester.pumpAndSettle();
      expect(
        find.text(L10n.t('twin_policy_never')).hitTestable(),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      for (final key in [
        'twin_preferences_tab',
        'twin_tools_tab',
        'auto_tab',
      ]) {
        await tester.ensureVisible(find.text(L10n.t(key)));
        await tester.tap(find.text(L10n.t(key)));
        await tester.pumpAndSettle();
        expect(find.byTooltip(L10n.t('close')).hitTestable(), findsOneWidget);
        expect(tester.takeException(), isNull);
      }
    });
  }
  testWidgets(
    'opens the requested blocked person and keeps save visible at 1000x600',
    (tester) async {
      final adapter = _TwinAdapter();
      for (final id in ['other', 'blocked']) {
        adapter.contacts.add({
          'id': id,
          'name': id,
          'autonomy_policy': id == 'blocked' ? 'never' : 'default',
          'aliases': [
            {'channel': 'mail', 'address': '$id@example.com'},
          ],
          'style': {
            'samples': 0,
            'tone': 'unknown',
            'language': 'unknown',
            'sufficient': false,
          },
        });
      }
      await _mount(
        tester,
        MemoLocale.en,
        initialContactId: 'blocked',
        initialAdapter: adapter,
      );
      tester.view.physicalSize = const Size(1000, 600);
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('twin_name')))
            .controller!
            .text,
        'blocked',
      );
      expect(
        find.widgetWithText(FilledButton, L10n.t('save')).hitTestable(),
        findsOneWidget,
      );
      await tester.enterText(
        find.byKey(const ValueKey('twin_name')),
        'Edited name',
      );
      await tester.tap(find.byTooltip(L10n.t('refresh')));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('twin_name')))
            .controller!
            .text,
        'Edited name',
      );
      expect(tester.takeException(), isNull);
    },
  );
}
