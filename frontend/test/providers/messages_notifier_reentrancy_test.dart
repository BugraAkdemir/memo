import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// A fake [HttpClientAdapter] that hands out one fresh [StreamController]
/// per POST /api/send/stream call (recorded in [requests]) instead of a
/// single shared stream — so the test can see, directly, how many separate
/// HTTP requests sendMessage() actually dispatched.
class _FakeMultiStreamAdapter implements HttpClientAdapter {
  final List<StreamController<Uint8List>> requests = [];
  final requestStarted = Completer<void>();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method == 'GET' && options.path == '/api/messages') {
      return ResponseBody.fromString(
        '[]',
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    if (options.method == 'POST' && options.path == '/api/send/stream') {
      final controller = StreamController<Uint8List>();
      requests.add(controller);
      if (!requestStarted.isCompleted) requestStarted.complete();
      return ResponseBody(controller.stream, 200);
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

Uint8List _sseChunk(Map<String, dynamic> data) =>
    Uint8List.fromList(utf8.encode('data: ${jsonEncode(data)}\n\n'));

// Regression test: sendMessage()'s isSendingProvider guard
// (`if (ref.read(isSendingProvider)) return;`) and its claim
// (`ref.read(isSendingProvider.notifier).state = true;`) are not atomic —
// `await _handleMemoryCommand(message, api)` sits between them. Two
// sendMessage() calls fired back-to-back (double Enter press, OS key-repeat
// while Enter is held, or a double click on the send button, all firing
// _handleKeyEvent/_send() twice in quick succession) can both read
// isSendingProvider as false and both proceed, dispatching two overlapping
// HTTP requests and appending two user-message bubbles for what the user
// experienced as a single send — with the shared `_cancelToken` field
// clobbered by whichever call reaches it second. This is fully
// provider-agnostic (it races before any provider-specific backend code
// runs), which matches it being reproducible with every LLM provider.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('two sendMessage() calls fired back-to-back both dispatch a request instead of the second being blocked',
      () async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final adapter = _FakeMultiStreamAdapter();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;

    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
    );
    addTearDown(container.dispose);

    await container.read(messagesProvider.future);
    final notifier = container.read(messagesProvider.notifier);

    // Fire twice, back-to-back, with no await in between — mirrors two
    // rapid real-world triggers (double Enter/key-repeat/double-click)
    // landing before the first call's `await _handleMemoryCommand` resolves
    // and claims isSendingProvider.
    final f1 = notifier.sendMessage('naber');
    final f2 = notifier.sendMessage('naber');

    // Let both calls run up to (and past, if the guard fails to stop the
    // second one) the point where they'd issue their HTTP request. Waiting
    // on the adapter's request-started signal (not a fixed delay) keeps the
    // assertion deterministic under parallel full-suite load, where a bare
    // Future.delayed could observe the request not yet dispatched even
    // though the guard logic is sound. f1/f2 themselves are awaited below,
    // after the streams are closed.
    await adapter.requestStarted.future;

    expect(
      adapter.requests.length,
      1,
      reason: 'a second sendMessage() call fired before the first one '
          'finished claiming isSendingProvider was not blocked by the guard '
          '— it dispatched its own HTTP request instead of returning '
          'immediately, exactly the race that can leave the UI in an '
          'inconsistent state (duplicate user bubbles, a clobbered shared '
          '_cancelToken, and a stop button whose state no longer reliably '
          'corresponds to a single in-flight request).',
    );

    // Finish whichever request(s) were actually made so the test doesn't
    // leave a dangling Future.
    for (final c in adapter.requests) {
      if (!c.isClosed) {
        c.add(_sseChunk({'content': 'reply', 'done': false}));
        c.add(_sseChunk({'done': true}));
        await c.close();
      }
    }
    await Future.wait([f1, f2]);
  });
}
