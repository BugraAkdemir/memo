import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/new_chat_provider.dart';

/// Records every request and answers /api/projects/status with [status].
class _Adapter implements HttpClientAdapter {
  _Adapter(this.status, {this.statusCode = 200, this.setupCode = 200});
  final Map<String, dynamic> status;
  final int statusCode;
  final int setupCode;
  final calls = <String>[];
  final bodies = <String, dynamic>{};

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    calls.add(options.path);
    bodies[options.path] = options.data;
    ResponseBody json(Object body, int code) => ResponseBody.fromString(jsonEncode(body), code, headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        });
    switch (options.path) {
      case '/api/projects/status':
        return json(status, statusCode);
      case '/api/projects/setup':
        return json({'root': '/p', 'created': ['AGENTS.md']}, setupCode);
      case '/api/projects/decline':
        return json({'ok': 'true'}, 200);
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

Future<_Adapter> _pump(WidgetTester tester, Map<String, dynamic> status, {int statusCode = 200, int setupCode = 200}) async {
  final adapter = _Adapter(status, statusCode: statusCode, setupCode: setupCode);
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = adapter;
  await tester.pumpWidget(ProviderScope(
    overrides: [apiClientProvider.overrideWithValue(client)],
    child: MaterialApp(
      home: Scaffold(
        body: Consumer(
          builder: (context, ref, _) => TextButton(
            onPressed: () => offerProjectSetup(context, ref, '/home/u/work/shop'),
            child: const Text('go'),
          ),
        ),
      ),
    ),
  ));
  await tester.tap(find.text('go'));
  await tester.pumpAndSettle();
  return adapter;
}

const _ask = {'root': '/home/u/work/shop', 'name': 'shop', 'allowed': true, 'should_ask': true};

void main() {
  setUp(() => L10n.setLocale(MemoLocale.en));

  testWidgets('asks, and writes nothing until the person says "Set up"', (tester) async {
    final a = await _pump(tester, _ask);
    expect(find.text('Set up Memo for this project?'), findsOneWidget);
    expect(find.textContaining('"shop"'), findsOneWidget);
    expect(a.calls, ['/api/projects/status'], reason: 'asking must not create anything');

    await tester.tap(find.text('Set up'));
    await tester.pumpAndSettle();
    expect(a.calls, ['/api/projects/status', '/api/projects/setup']);
    expect(a.bodies['/api/projects/setup'], {'path': '/home/u/work/shop'});
    expect(find.text('Memo is set up in shop.'), findsOneWidget);
  });

  testWidgets('"Not now" changes nothing and remembers nothing', (tester) async {
    final a = await _pump(tester, _ask);
    await tester.tap(find.text('Not now'));
    await tester.pumpAndSettle();
    expect(a.calls, ['/api/projects/status']);
  });

  testWidgets('"Never for this folder" is remembered by the backend', (tester) async {
    final a = await _pump(tester, _ask);
    await tester.tap(find.text('Never for this folder'));
    await tester.pumpAndSettle();
    expect(a.calls, ['/api/projects/status', '/api/projects/decline']);
    expect(a.bodies['/api/projects/decline'], {'path': '/home/u/work/shop', 'declined': true});
  });

  testWidgets('dismissing the dialog is not consent', (tester) async {
    final a = await _pump(tester, _ask);
    await tester.tapAt(const Offset(2, 2)); // outside the dialog
    await tester.pumpAndSettle();
    expect(a.calls, ['/api/projects/status']);
  });

  testWidgets('no dialog when the backend says not to ask (Minimal Mode, declined, home…)', (tester) async {
    final a = await _pump(tester, {..._ask, 'should_ask': false});
    expect(find.text('Set up Memo for this project?'), findsNothing);
    expect(a.calls, ['/api/projects/status']);
  });

  testWidgets('an old backend (404) or a failing status call never blocks the folder choice', (tester) async {
    final a = await _pump(tester, _ask, statusCode: 404);
    expect(find.text('Set up Memo for this project?'), findsNothing);
    expect(a.calls, ['/api/projects/status']);
    expect(tester.takeException(), isNull);
  });

  testWidgets('a refused setup is shown to the person, not swallowed', (tester) async {
    await _pump(tester, _ask, setupCode: 403);
    await tester.tap(find.text('Set up'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Could not set up the project'), findsOneWidget);
  });

  testWidgets('Turkish strings exist for every dialog label', (tester) async {
    L10n.setLocale(MemoLocale.tr);
    await _pump(tester, _ask);
    expect(find.text('Bu proje için Memo kurulsun mu?'), findsOneWidget);
    expect(find.text('Kur'), findsOneWidget);
    expect(find.text('Şimdi değil'), findsOneWidget);
    expect(find.text('Bu klasör için asla'), findsOneWidget);
  });
}
