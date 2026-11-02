import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/screens/mail_screen.dart';
import 'package:memo_flutter/widgets/mail_reply_draft.dart';

class DraftAdapter implements HttpClientAdapter {
  String state = 'pending';
  String? error;
  bool empty = false;
  bool activityError = false;
  bool denied = false;
  Completer<void>? sending;
  Completer<void>? loading;
  final sent = <Map<String, dynamic>>[];
  int holds = 0;
  final action = <String, dynamic>{
    'id': 'draft',
    'account': 'owner',
    'recipient': 'friend@example.com',
    'channel': 'mail',
    'state': 'pending',
    'subject': 'Tomorrow’s meeting',
    'body': 'Hello, there are no recorded events at that time.',
    'reason': 'drafted',
    'draft_saved': true,
    'score_status': 'unavailable',
  };
  Future<ResponseBody> fetch(
    RequestOptions o,
    Stream<Uint8List>? r,
    Future<void>? c,
  ) async {
    Object data;
    var status = 200;
    switch (o.path) {
      case '/api/twin/hold_action':
        holds++;
        action['mode'] = 'awaiting_approval';
        action['execute_at'] = null;
        data = {'ok': true};
      case '/api/mail/accounts':
        data = {
          'accounts': [
            {'id': 'owner', 'email': 'owner@example.com'},
          ],
        };
      case '/api/mail/stats':
        data = <String, dynamic>{};
      case '/api/mail/inbox':
        data = {
          'total': 1,
          'messages': [
            {
              'seq_num': 1,
              'subject': 'Tomorrow’s meeting',
              'from': 'Friend <friend@example.com>',
              'date': 'Sun, 01 Nov 2026 21:00:00 +0300',
              'snippet': 'Are you available tomorrow?',
            },
          ],
        };
      case '/api/twin/activity':
        if (loading != null) await loading!.future;
        if (activityError) {
          data = {'error': denied ? 'twin_denied' : 'twin_failed'};
          status = denied ? 403 : 500;
        } else {
          data = {
            'settings': {'enabled': true, 'mail': true, 'mode': 'draft_only'},
            'reply_model': {'provider': 'test', 'model': 'writer'},
            'actions': empty
                ? []
                : [
                    {...action, 'state': state},
                  ],
          };
        }
      case '/api/twin/send_action':
        sent.add(Map<String, dynamic>.from(o.data as Map));
        if (sending != null) await sending!.future;
        if (error != null) {
          data = {'error': error};
          status = 500;
          state = 'send_uncertain';
        } else {
          state = 'sent';
          data = {'ok': true, 'state': 'sent'};
        }
      case '/api/twin/cancel_action':
        state = 'cancelled';
        action['reason'] = 'user_cancelled'; // what the real server records
        data = {'ok': true};
      default:
        status = 404;
        data = {'error': 'unexpected route ${o.path}'};
    }
    return ResponseBody.fromString(
      jsonEncode(data),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  void close({bool force = false}) {}
}

Future<ProviderContainer> mountMail(
  WidgetTester tester,
  DraftAdapter adapter,
  Size size,
) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  final api = MemoApiClient(baseUrl: 'http://memo.test')
    ..dio.httpClientAdapter = adapter;
  final container = ProviderContainer(
    overrides: [
      apiClientProvider.overrideWithValue(api),
      authGateProvider.overrideWith(
        (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
      ),
    ],
  );
  final gate = container.listen(authGateProvider, (_, _) {});
  addTearDown(gate.close);
  addTearDown(container.dispose);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await container.read(authGateProvider.future);
  tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: Scaffold(body: MailScreen())),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

void main() {
  testWidgets('reviewing a scheduled mail pauses automatic delivery first', (
    tester,
  ) async {
    final adapter = DraftAdapter();
    adapter.action.addAll({
      'mode': 'grace_period_auto',
      'delivery_version': 1,
      'execute_at': DateTime.now()
          .add(const Duration(minutes: 1))
          .toIso8601String(),
    });
    await mountMail(tester, adapter, const Size(1000, 600));
    await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '1'})));
    await tester.pumpAndSettle();
    expect(find.byType(ApprovalCountdown), findsWidgets);
    await tester.ensureVisible(
      find.text(L10n.t('mail_reply_review_send')).last,
    );
    await tester.tap(find.text(L10n.t('mail_reply_review_send')).last);
    await tester.pumpAndSettle();
    expect(adapter.holds, 1);
    expect(find.text(L10n.t('mail_reply_review_title')), findsOneWidget);
    expect(adapter.sent, isEmpty);
  });

  // DT6-3: a reply the owner cancelled must not read "prepared for review" or
  // "evidence or model unavailable".
  testWidgets('cancelled reply explains the owner cancelled it', (
    tester,
  ) async {
    final adapter = DraftAdapter();
    adapter.state = 'cancelled'; // the adapter overlays this on every action
    adapter.action.addAll({'reason': 'user_cancelled', 'draft_saved': false});
    await mountMail(tester, adapter, const Size(1000, 600));
    expect(find.text(L10n.t('auto_reason_user_cancelled')), findsOneWidget);
    expect(find.text(L10n.t('auto_reason_drafted')), findsNothing);
    expect(find.text(L10n.t('auto_reason_review')), findsNothing);
  });

  // The follow-up radar prepares a reminder, not an answer; the owner must be
  // able to tell the two apart before approving it.
  testWidgets('a pending follow-up draft says why it exists', (tester) async {
    final adapter = DraftAdapter();
    adapter.action['reason'] = 'follow_up';
    await mountMail(tester, adapter, const Size(1000, 600));
    await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '1'})));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auto_reason_follow_up')), findsWidgets);
  });

  for (final size in [
    const Size(1000, 600),
    const Size(1440, 900),
    const Size(480, 800),
  ]) {
    testWidgets('prepared reply is reviewable at $size', (tester) async {
      final adapter = DraftAdapter();
      await mountMail(tester, adapter, size);
      await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '1'})));
      await tester.pumpAndSettle();
      await tester.ensureVisible(
        find.text(L10n.t('mail_reply_review_send')).last,
      );
      await tester.tap(find.text(L10n.t('mail_reply_review_send')).last);
      await tester.pumpAndSettle();
      expect(
        find.text(
          L10n.t('mail_reply_recipient', {'recipient': 'friend@example.com'}),
        ),
        findsOneWidget,
      );
      expect(find.text('Re: Tomorrow’s meeting'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }
  testWidgets('owner edits submit once and sent state appears', (tester) async {
    final adapter = DraftAdapter()..sending = Completer<void>();
    await mountMail(tester, adapter, const Size(1000, 700));
    await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '1'})));
    await tester.pumpAndSettle();
    await tester.ensureVisible(
      find.text(L10n.t('mail_reply_review_send')).last,
    );
    await tester.tap(find.text(L10n.t('mail_reply_review_send')).last);
    await tester.pumpAndSettle();
    final body = find.byWidgetPredicate(
      (w) =>
          w is TextField &&
          w.decoration?.labelText == L10n.t('mail_reply_body_label'),
    );
    await tester.enterText(body, 'Owner edited reply');
    await tester.tap(find.widgetWithText(FilledButton, L10n.t('mail_send')));
    await tester.pump();
    await tester.tap(
      find.widgetWithText(FilledButton, L10n.t('mail_reply_sending')),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(adapter.sent.length, 1);
    expect(adapter.sent.single, {
      'id': 'draft',
      'subject': 'Re: Tomorrow’s meeting',
      'body': 'Owner edited reply',
    });
    adapter.sending!.complete();
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsNothing);
    await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '0'})));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('mail_replies_filter_sent')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auto_state_sent')), findsWidgets);
    expect(find.byType(MailReplyDraftCard), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  testWidgets('uncertain delivery preserves edits and disables retry', (
    tester,
  ) async {
    final adapter = DraftAdapter()..error = 'mail_reply_send_uncertain';
    await mountMail(tester, adapter, const Size(1000, 700));
    await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '1'})));
    await tester.pumpAndSettle();
    await tester.ensureVisible(
      find.text(L10n.t('mail_reply_review_send')).last,
    );
    await tester.tap(find.text(L10n.t('mail_reply_review_send')).last);
    await tester.pumpAndSettle();
    final body = find.byWidgetPredicate(
      (w) =>
          w is TextField &&
          w.decoration?.labelText == L10n.t('mail_reply_body_label'),
    );
    await tester.enterText(body, 'Keep my edits');
    await tester.tap(find.widgetWithText(FilledButton, L10n.t('mail_send')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('mail_reply_send_uncertain')), findsOneWidget);
    expect(tester.widget<TextField>(body).controller!.text, 'Keep my edits');
    expect(
      tester
          .widget<FilledButton>(
            find.widgetWithText(FilledButton, L10n.t('mail_send')),
          )
          .onPressed,
      isNull,
    );
    expect(adapter.sent.length, 1);
    expect(tester.takeException(), isNull);
  });
  testWidgets('revoked owner access clears stale drafts and offers refresh', (
    tester,
  ) async {
    final adapter = DraftAdapter();
    await mountMail(tester, adapter, const Size(1000, 700));
    await tester.tap(find.text(L10n.t('mail_replies_tab', {'n': '1'})));
    await tester.pumpAndSettle();
    expect(find.byType(MailReplyDraftCard), findsOneWidget);
    adapter.activityError = true;
    adapter.denied = true;
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(find.byType(MailReplyDraftCard), findsNothing);
    expect(find.text(L10n.t('mail_reply_access_denied')), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsNothing);
    adapter.activityError = false;
    adapter.denied = false;
    await tester.tap(find.byTooltip(L10n.t('refresh')));
    await tester.pumpAndSettle();
    expect(find.byType(MailReplyDraftCard), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
