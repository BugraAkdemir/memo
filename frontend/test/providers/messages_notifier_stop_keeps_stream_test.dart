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

/// Serves /api/messages and lets the test decide whether the backend has
/// already recorded the stopped turn when the app asks for it.
class _Adapter implements HttpClientAdapter {
  bool recorded = false;
  int gets = 0;

  /// How many of the next reloads fail (backend briefly unreachable).
  int failGets = 0;
  StreamController<Uint8List> sse = StreamController<Uint8List>();
  bool _sseUsed = false;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'GET' && options.path == '/api/messages') {
      gets++;
      if (gets > 1 && failGets > 0) {
        failGets--;
        return ResponseBody.fromString('{"error":"down"}', 503,
            headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});
      }
      // The first read is the empty chat; afterwards the user's message is
      // saved, and the stopped assistant turn only once [recorded] is set.
      final body = <Map<String, dynamic>>[
        if (gets > 1)
          {'role': 'user', 'content': 'fix it', 'timestamp': '10:00:00'},
        if (recorded)
          {
            'role': 'assistant',
            'content': 'saved copy\n\n⏹️ Response stopped.',
            'timestamp': '10:00:05',
          },
      ];
      return ResponseBody.fromString(jsonEncode(body), 200,
          headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});
    }
    if (options.method == 'POST' && options.path == '/api/send/stream') {
      if (_sseUsed) sse = StreamController<Uint8List>(); // a second send
      _sseUsed = true;
      return ResponseBody(sse.stream, 200);
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

Uint8List _chunk(Map<String, dynamic> m) =>
    Uint8List.fromList(utf8.encode('data: ${jsonEncode(m)}\n\n'));

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<(ProviderContainer, _Adapter)> boot() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final adapter = _Adapter();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs),
      authGateProvider
          .overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
    ]);
    addTearDown(container.dispose);
    await container.read(authGateProvider.future);
    await container.read(messagesProvider.future);
    return (container, adapter);
  }

  test('stopping an agent turn keeps its narration, tool calls and reasoning on screen',
      () async {
    final (container, adapter) = await boot();
    final notifier = container.read(messagesProvider.notifier);

    final send = notifier.sendMessage('fix it');
    adapter.sse.add(_chunk({
      'finish_reason': 'agent_event',
      'content': jsonEncode({'type': 'narration', 'content': 'Önce config\'e bakıyorum.'}),
    }));
    adapter.sse.add(_chunk({
      'finish_reason': 'agent_event',
      'content': jsonEncode({'type': 'tool_executing', 'tool': 'read_file'}),
    }));
    adapter.sse.add(_chunk({
      'finish_reason': 'agent_event',
      'content': jsonEncode({'type': 'narration_delta', 'content': 'Şimdi testleri çalıştırıyorum'}),
    }));
    await Future<void>.delayed(const Duration(milliseconds: 100));

    // The backend has not recorded the stopped turn yet.
    notifier.stopStreaming();
    await adapter.sse.close();
    await send;

    final messages = container.read(messagesProvider).valueOrNull ?? [];
    final stopped = messages.where((m) => m.role == 'assistant').toList();
    expect(stopped, hasLength(1),
        reason: 'the stopped answer must stay in the chat, not vanish');
    expect(stopped.single.content, contains('Şimdi testleri çalıştırıyorum'));
    expect(stopped.single.content, contains('⏹️'));
    expect(stopped.single.agentEvents, isNotEmpty,
        reason: 'the tool call and narration that were already shown stay too');
    expect(container.read(streamingContentProvider), isEmpty);
    expect(container.read(streamingNarrationProvider), isEmpty);
  });

  test('once the backend has recorded the stopped turn, its saved copy is what is shown',
      () async {
    final (container, adapter) = await boot();
    final notifier = container.read(messagesProvider.notifier);

    final send = notifier.sendMessage('fix it');
    adapter.sse.add(_chunk({'content': 'half an answer'}));
    await Future<void>.delayed(const Duration(milliseconds: 100));

    adapter.recorded = true;
    notifier.stopStreaming();
    await adapter.sse.close();
    await send;

    final messages = container.read(messagesProvider).valueOrNull ?? [];
    final assistants = messages.where((m) => m.role == 'assistant').toList();
    expect(assistants, hasLength(1), reason: 'no duplicate next to the saved copy');
    expect(assistants.single.content, startsWith('saved copy'));
  });

  test('a failed reload after the stop is retried, and the saved copy wins once it exists', () async {
    final (container, adapter) = await boot();
    final notifier = container.read(messagesProvider.notifier);

    final send = notifier.sendMessage('fix it');
    adapter.sse.add(_chunk({'content': 'half an answer'}));
    await Future<void>.delayed(const Duration(milliseconds: 100));

    adapter.failGets = 1; // the reload right after the stop fails
    notifier.stopStreaming();
    await adapter.sse.close();
    await send;
    var assistants = (container.read(messagesProvider).valueOrNull ?? []).where((m) => m.role == 'assistant');
    expect(assistants, hasLength(1), reason: 'a failed reload must not take the stopped answer off the screen');
    expect(assistants.single.content, startsWith('half an answer'));
    expect(container.read(messagesProvider).hasError, isFalse);

    adapter.recorded = true; // the server catches up
    await Future<void>.delayed(const Duration(milliseconds: 1500));
    assistants = (container.read(messagesProvider).valueOrNull ?? []).where((m) => m.role == 'assistant');
    expect(assistants, hasLength(1));
    expect(assistants.single.content, startsWith('saved copy'));
  });

  test('a new send right after a stop is not written over by the stop\'s retry', () async {
    final (container, adapter) = await boot();
    final notifier = container.read(messagesProvider.notifier);

    final first = notifier.sendMessage('fix it');
    adapter.sse.add(_chunk({'content': 'half an answer'}));
    await Future<void>.delayed(const Duration(milliseconds: 100));
    notifier.stopStreaming();
    await adapter.sse.close();
    await first;

    // The person types a correction while the stopped turn is still settling.
    final second = notifier.sendMessage('actually, do this instead');
    await Future<void>.delayed(const Duration(milliseconds: 100));
    adapter.recorded = true;
    await Future<void>.delayed(const Duration(milliseconds: 1600)); // past a retry
    final messages = container.read(messagesProvider).valueOrNull ?? [];
    expect(messages.last.content, 'actually, do this instead',
        reason: 'the retry must not replace the new turn with an older reload');
    adapter.sse.add(_chunk({'content': 'ok', 'done': true}));
    await adapter.sse.close();
    await second;
  });

  test('when the server never records the stopped turn the answer stays and the person is told',
      () async {
    final (container, adapter) = await boot();
    final notifier = container.read(messagesProvider.notifier);

    final send = notifier.sendMessage('fix it');
    adapter.sse.add(_chunk({'content': 'half an answer'}));
    await Future<void>.delayed(const Duration(milliseconds: 100));
    notifier.stopStreaming();
    await adapter.sse.close();
    await send;

    await Future<void>.delayed(const Duration(seconds: 7)); // 5 retries x 1.2s
    final assistants = (container.read(messagesProvider).valueOrNull ?? []).where((m) => m.role == 'assistant');
    expect(assistants, hasLength(1), reason: 'giving up must not delete what the person saw');
    expect(container.read(errorMessageProvider), isNotNull);
  }, timeout: const Timeout(Duration(seconds: 30)));
}
