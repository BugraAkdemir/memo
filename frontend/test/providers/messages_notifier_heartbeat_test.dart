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

class _Adapter implements HttpClientAdapter {
  final StreamController<Uint8List> sse = StreamController<Uint8List>();

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    final json = {Headers.contentTypeHeader: [Headers.jsonContentType]};
    if (options.path == '/api/messages') {
      return ResponseBody.fromString('[]', 200, headers: json);
    }
    if (options.path == '/api/chats/active') {
      return ResponseBody.fromString(jsonEncode({'id': 'c1'}), 200, headers: json);
    }
    if (options.path == '/api/send/stream') {
      return ResponseBody(sse.stream, 200);
    }
    return ResponseBody.fromString(jsonEncode({}), 200, headers: json);
  }

  @override
  void close({bool force = false}) {}
}

void _emit(_Adapter a, Map<String, dynamic> chunk) =>
    a.sse.add(Uint8List.fromList(utf8.encode('data: ${jsonEncode(chunk)}\n\n')));

/// The backend now writes {"finish_reason":"heartbeat","content":""} every
/// 10s while a turn is silently busy. It must keep the turn's "last heard
/// from" fresh without leaking into the reply, and the timing must be gone
/// once the turn ends.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('a heartbeat keeps the turn alive without touching the reply', () async {
    SharedPreferences.setMockInitialValues({});
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
    await container.read(messagesProvider.future);

    final send = container.read(messagesProvider.notifier).sendMessage('long task');
    await Future<void>.delayed(const Duration(milliseconds: 20));
    final started = container.read(streamTimingProvider);
    expect(started, isNotNull, reason: 'a running turn must expose its timing');

    await Future<void>.delayed(const Duration(milliseconds: 1100));
    _emit(adapter, {'content': '', 'done': false, 'finish_reason': 'heartbeat'});
    await Future<void>.delayed(const Duration(milliseconds: 30));
    final afterBeat = container.read(streamTimingProvider)!;
    expect(afterBeat.lastActivity.isAfter(started!.lastActivity), isTrue,
        reason: 'a heartbeat must count as hearing from the backend');
    expect(container.read(streamingContentProvider), '',
        reason: 'a heartbeat must not add anything to the reply');

    _emit(adapter, {'content': 'done!', 'done': false});
    _emit(adapter, {'content': '', 'done': true, 'finish_reason': 'stop'});
    await adapter.sse.close();
    await send;
    expect(container.read(streamTimingProvider), isNull,
        reason: 'no progress line once the turn is over');
  });
}
