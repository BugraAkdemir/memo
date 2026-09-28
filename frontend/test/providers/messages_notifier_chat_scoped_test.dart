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

/// Records every request; the backend's "active" chat is chat-A, while this
/// client has selected chat-B.
class _Adapter implements HttpClientAdapter {
  _Adapter({this.messagesStatus = 200});
  final int messagesStatus;
  final List<RequestOptions> seen = [];

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    seen.add(options);
    final json = {Headers.contentTypeHeader: [Headers.jsonContentType]};
    if (options.path == '/api/chats/active') {
      return ResponseBody.fromString(jsonEncode({'id': 'chat-B'}), 200, headers: json);
    }
    if (options.path == '/api/messages') {
      if (messagesStatus != 200) return ResponseBody.fromString('chat not found', messagesStatus);
      return ResponseBody.fromString(
          jsonEncode([
            {'role': 'user', 'content': 'in B', 'timestamp': '12:00'}
          ]),
          200,
          headers: json);
    }
    return ResponseBody.fromString(jsonEncode({'ok': 'true'}), 200, headers: json);
  }

  @override
  void close({bool force = false}) {}
}

Future<(ProviderContainer, _Adapter)> _container({int messagesStatus = 200}) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  final adapter = _Adapter(messagesStatus: messagesStatus);
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = adapter;
  final container = ProviderContainer(overrides: [
    apiClientProvider.overrideWithValue(client),
    prefsProvider.overrideWithValue(prefs),
    authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
  ]);
  await container.read(authGateProvider.future);
  return (container, adapter);
}

/// The message list used to be read with a bare GET /api/messages — "the
/// backend's active chat" — right after switching the whole backend to the
/// chat. Another client switching in between (a second device on the same
/// backend) made this one show, and delete-by-index from, the wrong chat.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('reads and deletes name the selected chat explicitly', () async {
    final (container, adapter) = await _container();
    addTearDown(container.dispose);

    final msgs = await container.read(messagesProvider.future);
    expect(msgs.single.content, 'in B');
    final read = adapter.seen.lastWhere((o) => o.path == '/api/messages');
    expect(read.queryParameters['chat_id'], 'chat-B');

    await container.read(messagesProvider.notifier).deleteMessage(0);
    final del = adapter.seen.lastWhere((o) => o.path == '/api/messages/delete');
    expect((del.data as Map)['chat_id'], 'chat-B');
  });

  test('a chat deleted elsewhere loads as empty, not as an error', () async {
    final (container, _) = await _container(messagesStatus: 404);
    addTearDown(container.dispose);

    expect(await container.read(messagesProvider.future), isEmpty);
  });
}
