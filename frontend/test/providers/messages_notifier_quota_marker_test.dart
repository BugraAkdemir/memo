import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/quota_notice.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/quota_notice_provider.dart';
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

Future<(ProviderContainer, _Adapter)> _boot() async {
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
  await container.read(activeChatIdProvider.future);
  await container.read(messagesProvider.future);
  return (container, adapter);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('a quota_exhausted marker becomes a card for the chat and never shows up as reply text', () async {
    final (container, adapter) = await _boot();
    final send = container.read(messagesProvider.notifier).sendMessage('keep going');

    adapter.sse.add(_data({'content': 'partial answer ', 'done': false}));
    adapter.sse.add(_data({
      'content': jsonEncode({
        'kind': 'exhausted',
        'model': 'gpt-5.5',
        'remaining_percent': 0,
        'reset_at': DateTime.now().add(const Duration(hours: 2)).toUtc().toIso8601String(),
        'window': '5h',
      }),
      'finish_reason': 'quota_exhausted',
      'done': false,
    }));
    adapter.sse.add(_data({'content': '', 'error': '⚠️ status 429: usage limit reached', 'done': true}));
    await adapter.sse.close();
    await send;

    final notice = container.read(quotaNoticeProvider)['chat-1'];
    expect(notice, isNotNull, reason: 'the card for the chat the turn ran in');
    expect(notice!.exhausted, isTrue);
    expect(notice.model, 'gpt-5.5');
    expect(notice.knownReset, isTrue);
    expect(notice.window, '5h');

    // The marker's JSON must not leak into what the user reads.
    expect(container.read(streamingContentProvider), isNot(contains('exhausted')));
    final shown = (container.read(messagesProvider).valueOrNull ?? []).map((m) => m.content).join('\n');
    expect(shown, isNot(contains('"kind"')));
  });

  test('a quota_low marker becomes a warning card and the reply is unaffected', () async {
    final (container, adapter) = await _boot();
    final send = container.read(messagesProvider.notifier).sendMessage('hello');
    adapter.sse.add(_data({'content': 'hi there', 'done': false}));
    adapter.sse.add(_data({
      'content': jsonEncode({'kind': 'low', 'model': 'claude-sonnet-4-6', 'remaining_percent': 8}),
      'finish_reason': 'quota_low',
      'done': false,
    }));
    adapter.sse.add(_data({'content': '', 'done': true, 'finish_reason': 'stop'}));
    await adapter.sse.close();
    await send;

    final notice = container.read(quotaNoticeProvider)['chat-1']!;
    expect(notice.low, isTrue);
    expect(notice.remainingPercent, 8);
    final replies = (container.read(messagesProvider).valueOrNull ?? []).where((m) => m.role == 'assistant');
    expect(replies.map((m) => m.content).join(), 'hi there', reason: 'the marker is metadata, not reply text');
  });

  test('a marker with garbage content is ignored without breaking the turn', () async {
    final (container, adapter) = await _boot();
    final send = container.read(messagesProvider.notifier).sendMessage('hello');
    adapter.sse.add(_data({'content': 'not json at all', 'finish_reason': 'quota_exhausted', 'done': false}));
    adapter.sse.add(_data({'content': 'ok', 'done': false}));
    adapter.sse.add(_data({'content': '', 'done': true, 'finish_reason': 'stop'}));
    await adapter.sse.close();
    await send;
    expect(container.read(quotaNoticeProvider), isEmpty);
    final replies = (container.read(messagesProvider).valueOrNull ?? []).where((m) => m.role == 'assistant');
    expect(replies.map((m) => m.content).join(), 'ok');
  });

  test('sending the next message takes the old card away', () async {
    final (container, adapter) = await _boot();
    container.read(quotaNoticeProvider.notifier).show(
          'chat-1',
          QuotaNotice(kind: 'exhausted', receivedAt: DateTime.now(), resetAt: DateTime.now().add(const Duration(hours: 1))),
        );
    expect(container.read(quotaNoticeProvider), isNotEmpty);

    final send = container.read(messagesProvider.notifier).sendMessage('different question');
    expect(container.read(quotaNoticeProvider), isEmpty,
        reason: 'the card is about the previous turn; the conversation moved on');
    adapter.sse.add(_data({'content': 'sure', 'done': false}));
    adapter.sse.add(_data({'content': '', 'done': true, 'finish_reason': 'stop'}));
    await adapter.sse.close();
    await send;
  });
}
