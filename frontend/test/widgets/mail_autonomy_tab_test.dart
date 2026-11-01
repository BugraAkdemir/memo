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
import 'package:memo_flutter/widgets/settings/tabs/mail_autonomy_tab.dart';

class _ReplyAdapter implements HttpClientAdapter {
  final calls = <RequestOptions>[];
  Map<String, dynamic> selection = {'provider': 'writer', 'model': 'reply-one'};
  @override
  Future<ResponseBody> fetch(
    RequestOptions o,
    Stream<Uint8List>? request,
    Future<void>? cancel,
  ) async {
    calls.add(o);
    dynamic body = {'ok': true};
    switch (o.path) {
      case '/api/twin/state':
        body = {
          'contacts': [],
          'projects': [],
          'preferences': {},
          'reply_model': selection,
        };
      case '/api/providers':
        body = [
          {
            'name': 'writer',
            'type': 'custom',
            'enabled': true,
            'model': 'reply-one',
            'models': [
              {'id': 'reply-one'},
            ],
          },
          {
            'name': 'web-private',
            'type': 'web-chatgpt',
            'enabled': true,
            'model': 'web',
          },
          {
            'name': 'cli-private',
            'type': 'codex-cli',
            'enabled': true,
            'model': 'cli',
          },
        ];
      case '/api/providers/model':
        body = {
          'models': [
            {'id': 'reply-one'},
            {'id': 'reply-two'},
          ],
        };
      case '/api/twin/reply_model':
        selection = Map<String, dynamic>.from(o.data as Map);
      default:
        throw StateError('Unexpected endpoint: ${o.path}');
    }
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  for (final locale in MemoLocale.values) {
    testWidgets(
      'reply writer selection persists separately from decision settings ($locale)',
      (tester) async {
        L10n.setLocale(locale);
        addTearDown(() => L10n.setLocale(MemoLocale.en));
        tester.view.physicalSize = const Size(420, 850);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.reset);
        final adapter = _ReplyAdapter();
        final api = MemoApiClient(baseUrl: 'http://reply.test');
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
        final gate = container.listen(authGateProvider, (_, __) {});
        addTearDown(gate.close);
        await container.read(authGateProvider.future);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: const MaterialApp(home: Scaffold(body: MailAutonomyTab())),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text(L10n.t('mail_reply_help')), findsOneWidget);
        await tester.tap(find.byType(DropdownButtonFormField<String>).first);
        await tester.pumpAndSettle();
        expect(find.text('web-private'), findsNothing);
        expect(find.text('cli-private'), findsNothing);
        await tester.tap(find.text('writer').last);
        await tester.pumpAndSettle();
        await tester.tap(find.byType(DropdownButtonFormField<String>).last);
        await tester.pumpAndSettle();
        await tester.tap(find.text('reply-two').last);
        await tester.pumpAndSettle();
        await tester.ensureVisible(find.text(L10n.t('save')));
        await tester.tap(find.text(L10n.t('save')));
        await tester.pumpAndSettle();
        expect(adapter.selection, {'provider': 'writer', 'model': 'reply-two'});
        expect(
          adapter.calls.where((c) => c.method == 'POST').single.path,
          '/api/twin/reply_model',
        );
        expect(adapter.calls.any((c) => c.path.contains('learning')), isFalse);
        expect(tester.takeException(), isNull);
      },
    );
  }
}
