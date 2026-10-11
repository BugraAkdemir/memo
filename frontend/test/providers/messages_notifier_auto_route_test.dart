import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// A backend that serves one chat ("chat-1") and a scripted SSE stream.
class _Adapter implements HttpClientAdapter {
  final StreamController<Uint8List> sse = StreamController<Uint8List>();

  @override
  Future<ResponseBody> fetch(RequestOptions o, Stream<Uint8List>? r, Future<void>? c) async {
    ResponseBody json(Object body) => ResponseBody.fromString(
          jsonEncode(body),
          200,
          headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
        );
    if (o.method == 'GET' && o.path == '/api/providers/active') return json({'provider': 'auto-route'});
    if (o.method == 'GET' && o.path == '/api/chats/active') return json({'id': 'chat-1'});
    if (o.method == 'GET' && o.path == '/api/messages') return json(<Object>[]);
    if (o.method == 'GET' && o.path == '/api/chats/cli-provider') return json({'cli_provider': ''});
    if (o.method == 'POST' && o.path == '/api/send/stream') return ResponseBody(sse.stream, 200);
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

Uint8List _data(Map<String, Object?> chunk) => Uint8List.fromList(utf8.encode('data: ${jsonEncode(chunk)}\n\n'));

Future<(ProviderContainer, _Adapter)> _boot({bool streaming = true}) async {
  SharedPreferences.setMockInitialValues({'memo_streaming': streaming});
  final prefs = await SharedPreferences.getInstance();
  final adapter = _Adapter();
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = adapter;
  final container = ProviderContainer(overrides: [
    apiClientProvider.overrideWithValue(client),
    prefsProvider.overrideWithValue(prefs),
    authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
  ]);
  addTearDown(container.dispose);
  await container.read(authGateProvider.future);
  await container.read(activeChatIdProvider.future);
  await container.read(messagesProvider.future);
  return (container, adapter);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('routing marker is live attribution and survives stream completion', () async {
    final (container, adapter) = await _boot();
    final live = Completer<void>();
    final subscription = container.listen(streamingModelRouteProvider, (_, next) {
      if (next?.model == 'coder' && !live.isCompleted) live.complete();
    });
    addTearDown(subscription.close);
    final send = container.read(messagesProvider.notifier).sendMessage('write code');
    adapter.sse.add(_data({'finish_reason': 'model_route', 'content': jsonEncode({'provider': 'Zen', 'model': 'coder'}), 'done': false}));
    await live.future.timeout(const Duration(seconds: 2));
    expect(container.read(streamingModelRouteProvider)?.provider, 'Zen');
    adapter.sse.add(_data({'content': 'answer', 'done': false}));
    adapter.sse.add(_data({'content': '', 'done': true, 'finish_reason': 'stop'}));
    await adapter.sse.close();
    await send;
    final reply = container.read(messagesProvider).valueOrNull!.last;
    expect(reply.content, 'answer');
    expect(reply.routeModel, 'coder');
    expect(reply.routeProvider, 'Zen');
    expect(container.read(streamingModelRouteProvider), isNull);
  });
  test('malformed routing markers never leak into the answer', () async {
    final (container, adapter) = await _boot();
    final send = container.read(messagesProvider.notifier).sendMessage('hello');
    adapter.sse.add(_data({'finish_reason': 'model_route', 'content': 'invalid metadata', 'done': false}));
    adapter.sse.add(_data({'content': 'answer', 'done': false}));
    adapter.sse.add(_data({'content': '', 'done': true, 'finish_reason': 'stop'}));
    await adapter.sse.close();
    await send;
    expect(container.read(messagesProvider).valueOrNull!.last.content, 'answer');
    expect(container.read(messagesProvider).valueOrNull!.last.routeModel, isEmpty);
  });
  test('Auto Route metadata works with token streaming disabled', () async {
    final (container, adapter) = await _boot(streaming: false);
    final live = Completer<void>();
    final subscription = container.listen(streamingModelRouteProvider, (_, next) { if (next != null && !live.isCompleted) live.complete(); });
    addTearDown(subscription.close);
    final send = container.read(messagesProvider.notifier).sendMessage('hello');
    adapter.sse.add(_data({'finish_reason': 'model_route', 'content': jsonEncode({'provider': 'Zen', 'model': 'general'}), 'done': false}));
    adapter.sse.add(_data({'content': 'buffered answer', 'done': false}));
    await live.future.timeout(const Duration(seconds: 2));
    expect(container.read(streamingContentProvider), isEmpty);
    adapter.sse.add(_data({'content': '', 'done': true, 'finish_reason': 'stop'}));
    await adapter.sse.close();
    await send;
    final reply = container.read(messagesProvider).valueOrNull!.last;
    expect(reply.content, 'buffered answer');
    expect(reply.routeModel, 'general');
  });

}
