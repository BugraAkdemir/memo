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

/// Serves /api/messages with an ever-growing list (each GET returns one more
/// message than the last) and lets a single test control exactly what SSE
/// bytes /api/send/stream emits.
class _FakeAdapter implements HttpClientAdapter {
  int messagesCallCount = 0;
  final StreamController<Uint8List> sseController = StreamController<Uint8List>();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'GET' && options.path == '/api/messages') {
      messagesCallCount++;
      // First call (provider init): empty history. Second call onward
      // (refresh() after the stream's error chunk): the backend's own
      // persisted turn — a user message plus the system message it recorded
      // for the cancelled/failed turn, exactly what a manual page reload
      // showed live in production (BUG: this used to only appear after a
      // reload — the catch block never called refresh(), so it stayed
      // invisible until then).
      final body = messagesCallCount == 1
          ? <Map<String, dynamic>>[]
          : <Map<String, dynamic>>[
              {'role': 'user', 'content': 'run ls -la', 'timestamp': '10:00:00'},
              {
                'role': 'assistant',
                'content': '⚠️ Agent execution cancelled (permission timeout)',
                'timestamp': '10:01:00',
              },
            ];
      return ResponseBody.fromString(
        jsonEncode(body),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    if (options.method == 'POST' && options.path == '/api/send/stream') {
      return ResponseBody(sseController.stream, 200);
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

// Regression test: sendMessageStream() throws an Exception when the backend
// sends a chunk with a non-empty "error" field (permission-timeout
// cancellation and an Orchestra chief failure both end a turn this way) —
// that's deliberate, see its own "must NOT be swallowed" comment. But
// sendMessage()'s catch block only showed a generic toast and never called
// refresh(): the backend had already persisted the real message (its own
// cancellation/error text) by the time the exception reached the frontend,
// yet the chat transcript stayed exactly as it was before the send — the
// real message was invisible until the user manually reloaded the whole
// page, which re-fetched history from scratch and showed it correctly.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('an error chunk mid-stream refreshes messages instead of leaving the real backend reply invisible',
      () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final adapter = _FakeAdapter();
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
    expect(container.read(messagesProvider).valueOrNull, isEmpty);

    final notifier = container.read(messagesProvider.notifier);
    final send = notifier.sendMessage('run ls -la');

    // Simulate the backend erroring out mid-turn, the same shape as a
    // permission-timeout cancellation or an Orchestra chief failure.
    adapter.sseController.add(Uint8List.fromList(
      utf8.encode('data: ${jsonEncode({
            'error': '⚠️ Agent execution cancelled (permission timeout)',
            'done': true,
          })}\n\n'),
    ));
    await adapter.sseController.close();
    await send;

    expect(adapter.messagesCallCount, greaterThanOrEqualTo(2),
        reason: 'the catch block must call refresh() (a GET /api/messages) '
            'to pull in what the backend actually persisted, not just show '
            'a generic toast and leave the transcript stale');

    final messages = container.read(messagesProvider).valueOrNull ?? [];
    expect(
      messages.any((m) => m.content.contains('Agent execution cancelled')),
      true,
      reason: 'the backend\'s own terminal message for this turn must be '
          'visible immediately, without requiring a manual page reload',
    );
  });
}
