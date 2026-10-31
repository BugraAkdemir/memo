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
  };
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
          'projects': [
            {'id': 'p_test', 'name': 'Demo repo'},
          ],
        };
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

Future<_TwinAdapter> _mount(WidgetTester tester, MemoLocale locale) async {
  final previous = L10n.locale;
  L10n.setLocale(locale);
  addTearDown(() => L10n.setLocale(previous));
  tester.view.physicalSize = const Size(1440, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final adapter = _TwinAdapter();
  final api = MemoApiClient(baseUrl: 'http://memo.test');
  api.dio.httpClientAdapter = adapter;
  final container = ProviderContainer(
    overrides: [apiClientProvider.overrideWithValue(api)],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: Scaffold(body: TwinDialog())),
    ),
  );
  await tester.pumpAndSettle();
  return adapter;
}

void main() {
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
        expect(find.text('Tomorrow at noon'), findsOneWidget);
        expect(find.textContaining('2026-10-31T20:50:00Z'), findsOneWidget);
        await tester.tap(find.text(L10n.t('twin_preferences_tab')));
        await tester.pumpAndSettle();
        await tester.tap(find.byType(Switch));
        await tester.pump();
        await tester.tap(find.text(L10n.t('save')).last);
        await tester.pumpAndSettle();
        expect(a.preferences['allow_project_inspection'], isTrue);
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

  testWidgets('contact editor remains usable in a narrow window', (
    tester,
  ) async {
    await _mount(tester, MemoLocale.en);
    tester.view.physicalSize = const Size(480, 900);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('twin_name')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
