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
  final StreamController<Uint8List> sse = StreamController<Uint8List>();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'GET' && options.path == '/api/messages') {
      gets++;
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
}
