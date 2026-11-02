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

class _ReplyAdapter implements HttpClientAdapter {
  _ReplyAdapter({
    this.sender = 'Buğra Akdemir 3 <friend@example.com>',
    this.subject = 'Original subject',
    this.rejectSend = false,
  });

  final String sender;
  final String subject;
  bool rejectSend;
  final sent = <Map<String, dynamic>>[];
  final unexpected = <String>[];

  Map<String, dynamic> get message => {
    'seq_num': 1,
    'uid': 42,
    'message_id': '<original-42@example.com>',
    'subject': subject,
    'from': sender,
    'to': 'owner@example.com',
    'date': 'Sun, 01 Nov 2026 21:00:00 +0300',
    'text_body': 'Are you available tomorrow?',
    'snippet': 'Are you available tomorrow?',
  };

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? request,
    Future<void>? cancel,
  ) async {
    Object data;
    var status = 200;
    switch (options.path) {
      case '/api/twin/events':
        return ResponseBody.fromString(
          ': ready\n\n',
          200,
          headers: {
            Headers.contentTypeHeader: ['text/event-stream'],
          },
        );
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
          'messages': [message],
        };
      case '/api/mail/message':
        data = message;
      case '/api/twin/activity':
        data = {
          'settings': {'enabled': true, 'mail': true, 'mode': 'full_auto'},
          'reply_model': {'provider': 'test', 'model': 'writer'},
          'actions': [],
          'mail_contacts': [
            {
              'id': 'c_friend',
              'name': 'Friend',
              'policy': 'never',
              'addresses': ['friend@example.com'],
            },
          ],
        };
      case '/api/mail/send':
        sent.add(Map<String, dynamic>.from(options.data as Map));
        if (rejectSend) {
          status = 400;
          data = {'error': 'invalid recipient address'};
        } else {
          data = {'ok': true};
        }
      default:
        unexpected.add(options.path);
        status = 404;
        data = {'error': 'unexpected test endpoint'};
    }
    return ResponseBody.fromString(
      jsonEncode(data),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _openReply(WidgetTester tester, _ReplyAdapter adapter) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final api = MemoApiClient(baseUrl: 'http://memo.test');
  api.dio.httpClientAdapter = adapter;
  final container = ProviderContainer(
    overrides: [
      apiClientProvider.overrideWithValue(api),
      authGateProvider.overrideWith(
        (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
      ),
    ],
  );
  addTearDown(container.dispose);
  final gate = container.listen(authGateProvider, (_, next) {});
  addTearDown(gate.close);
  await container.read(authGateProvider.future);
  tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: Scaffold(body: MailScreen())),
    ),
  );
  await tester.pumpAndSettle();
  expect(tester.takeException(), isNull);
  await tester.tap(find.text(adapter.subject));
  await tester.pumpAndSettle();
  expect(find.text(L10n.t('mail_contact_never')), findsOneWidget);
  expect(
    find.widgetWithText(TextButton, L10n.t('mail_contact_edit')),
    findsOneWidget,
  );
  await tester.tap(find.text(L10n.t('mail_reply')));
  await tester.pumpAndSettle();
}

Finder _composeField(String label) => find.descendant(
  of: find.byType(AlertDialog),
  matching: find.byWidgetPredicate(
    (w) =>
        w is TextField &&
        (w.decoration?.labelText == label || w.decoration?.hintText == label),
  ),
);

void main() {
  testWidgets(
    'manual reply sends a display-name recipient and thread headers',
    (tester) async {
      final adapter = _ReplyAdapter();
      await _openReply(tester, adapter);
      final recipient = tester.widget<TextField>(
        _composeField(L10n.t('mail_to')),
      );
      expect(recipient.controller!.text, adapter.sender);
      final subject = tester.widget<TextField>(
        _composeField(L10n.t('mail_subject')),
      );
      expect(subject.controller!.text, 'Re: Original subject');
      await tester.enterText(
        _composeField(L10n.t('mail_body_hint')),
        'Tomorrow at noon works.',
      );
      await tester.tap(find.text(L10n.t('mail_send')));
      await tester.pumpAndSettle();
      expect(adapter.sent, [
        {
          'account_id': 'owner',
          'to': [adapter.sender],
          'cc': [],
          'subject': 'Re: Original subject',
          'body': 'Tomorrow at noon works.',
          'in_reply_to': '<original-42@example.com>',
        },
      ]);
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text(L10n.t('mail_sent_success')), findsOneWidget);
      expect(adapter.unexpected, isEmpty);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'quoted commas remain one recipient and replies keep one Re prefix',
    (tester) async {
      final adapter = _ReplyAdapter(
        sender: '"Akdemir, Buğra" <friend@example.com>',
        subject: 'Re: Original subject',
      );
      await _openReply(tester, adapter);
      expect(
        tester
            .widget<TextField>(_composeField(L10n.t('mail_subject')))
            .controller!
            .text,
        'Re: Original subject',
      );
      await tester.enterText(
        _composeField(L10n.t('mail_cc')),
        '"Other, Person" <other@example.com>, third@example.com',
      );
      await tester.enterText(
        _composeField(L10n.t('mail_body_hint')),
        'Confirmed.',
      );
      await tester.tap(find.text(L10n.t('mail_send')));
      await tester.pumpAndSettle();
      expect(adapter.sent.single['to'], [adapter.sender]);
      expect(adapter.sent.single['cc'], [
        '"Other, Person" <other@example.com>',
        'third@example.com',
      ]);
      expect(adapter.sent.single['subject'], 'Re: Original subject');
      expect(find.byType(AlertDialog), findsNothing);
      expect(adapter.unexpected, isEmpty);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets('failed manual send retains editable body and can retry', (
    tester,
  ) async {
    final adapter = _ReplyAdapter(rejectSend: true);
    await _openReply(tester, adapter);
    await tester.enterText(
      _composeField(L10n.t('mail_body_hint')),
      'Do not lose this reply.',
    );
    await tester.tap(find.text(L10n.t('mail_send')));
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsOneWidget);
    expect(
      tester
          .widget<TextField>(_composeField(L10n.t('mail_body_hint')))
          .controller!
          .text,
      'Do not lose this reply.',
    );
    adapter.rejectSend = false;
    await tester.tap(find.text(L10n.t('mail_send')));
    await tester.pumpAndSettle();
    expect(adapter.sent, hasLength(2));
    expect(adapter.sent.last['body'], 'Do not lose this reply.');
    expect(find.byType(AlertDialog), findsNothing);
    expect(adapter.unexpected, isEmpty);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
  });
}
