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

/// A fake [HttpClientAdapter] that answers GET /api/messages with an empty
/// list and POST /api/send/stream with an SSE stream fed by [controller] —
/// so the test can pace out chunks and control exactly when the app code
/// is mid-`await for`, real sockets aren't usable under `flutter test` (see
/// settings_toggle_race_test.dart for why).
class _FakeStreamingAdapter implements HttpClientAdapter {
  final StreamController<Uint8List> controller;
  _FakeStreamingAdapter(this.controller);

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
      return ResponseBody(controller.stream, 200);
    }
    return ResponseBody.fromString('not found', 404);
  }

  @override
  void close({bool force = false}) {}
}

Uint8List _sseChunk(Map<String, dynamic> data) =>
    Uint8List.fromList(utf8.encode('data: ${jsonEncode(data)}\n\n'));

// Regression test for BUG-H2: MessagesNotifier's stream loop, post-stream
// finalize block, and catch block wrote to `state`/shared providers with no
// check for whether the notifier itself had since been disposed —
// ActiveChatIdNotifier.switchTo calls stopStreaming() then
// ref.invalidate(messagesProvider) mid-stream, but Riverpod doesn't cancel
// the in-flight sendMessage() coroutine, so it kept running against a torn-
// down instance.
//
// In this Riverpod version, writing `state` on a disposed AsyncNotifier is
// silently ignored rather than throwing — so the observable half of the bug
// isn't a crash, it's the *other* failure mode BUG-L1's original report
// also named: the disposed instance's `finally` block unconditionally reset
// the *shared, global* isSendingProvider — clobbering whatever the newly-
// active chat's own, legitimately in-progress send had just set it to.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('a stream abandoned by a chat switch does not clobber isSendingProvider for the new chat',
      () async {
    // sendMessage() reads streamingEnabledProvider, which reads
    // prefsProvider (SharedPreferences) — unrelated to what this test
    // exercises, but it needs a real backing value to avoid the
    // UnimplementedError prefsProvider throws by default outside main().
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();

    final controller = StreamController<Uint8List>();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeStreamingAdapter(controller);

    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
    );
    addTearDown(container.dispose);

    // Establish the notifier and capture this exact instance's method —
    // mirrors how the real widget tree holds a reference across the
    // invalidate() that follows.
    await container.read(messagesProvider.future);
    final notifier = container.read(messagesProvider.notifier);

    final sendFuture = notifier.sendMessage('hello');

    // Let the app code reach its `await for` and consume the first chunk.
    controller.add(_sseChunk({'content': 'partial', 'done': false}));
    await Future.delayed(const Duration(milliseconds: 20));

    // Simulate ActiveChatIdNotifier.switchTo: invalidate messagesProvider
    // while notifier.sendMessage() is still suspended mid-stream, then
    // force the rebuild with another read — in the real app this happens
    // implicitly because the chat screen widget keeps ref.watch-ing
    // messagesProvider, which is what actually triggers Riverpod to dispose
    // the old instance right away; a bare invalidate() with no active
    // watcher doesn't. `notifier` above still points at that old, now-
    // disposed instance.
    container.invalidate(messagesProvider);
    container.read(messagesProvider);
    await Future.delayed(const Duration(milliseconds: 20));

    // Simulate the user having switched to a different chat and started
    // their own, brand-new send there — this is the exact shared state the
    // abandoned stream from the old chat must not touch.
    container.read(isSendingProvider.notifier).state = true;

    // Finish the old stream — this resumes the disposed instance's
    // `await for` straight into its post-loop/finally code.
    controller.add(_sseChunk({'content': ' more', 'done': false}));
    controller.add(_sseChunk({'done': true}));
    await controller.close();
    await sendFuture;

    expect(
      container.read(isSendingProvider),
      isTrue,
      reason: 'the abandoned stream from the old (disposed) chat must not '
          'reset isSendingProvider — that would incorrectly signal that no '
          'send is in progress while the newly-active chat is still sending',
    );
  });
}
