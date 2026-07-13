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

// Regression test: a real flutter run session showed MessagesNotifier's
// `_disposed` field reading `true` on the very first message of a fresh
// session, before any user-triggered chat switch or explicit
// invalidate(messagesProvider) call — captured via [SEND-DEBUG] logging
// (build() -> onDispose -> build() -> sendMessage() sees disposed=true,
// with no second onDispose in between). Something early in real app
// startup causes messagesProvider to go through one dispose+rebuild cycle
// before the user ever sends anything.
//
// `_disposed` was only ever reset to `false` via the class's field
// initializer (`bool _disposed = false;`), which runs once at object
// construction — build() itself never reset it. So once the *first*
// dispose+rebuild cycle happens for any reason, every future sendMessage()
// call permanently sees `_disposed == true` and its `finally` block's
// `if (!_disposed) { isSendingProvider = false; }` guard (added for
// BUG-H2 — a genuinely abandoned stream must not clobber a different
// chat's in-progress send) never fires again for the rest of the session:
// the send/stop button gets stuck on "stop" forever, on every single turn,
// regardless of provider.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('sendMessage() after a dispose+rebuild cycle still resets isSendingProvider when the stream finishes',
      () async {
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

    // Keep a live subscription so invalidate() actually tears down and
    // rebuilds the notifier right away, instead of merely marking it dirty
    // with nothing watching (see messages_notifier_dispose_test.dart's
    // comment on the same point) — this mirrors ChatScreen's permanent
    // ref.watch(messagesProvider) in the real app.
    final sub = container.listen(messagesProvider, (_, __) {});
    addTearDown(sub.close);

    await container.read(messagesProvider.future);

    // Force one dispose+rebuild cycle on messagesProvider — standing in for
    // whatever triggers it during real app startup before the user sends
    // anything.
    container.invalidate(messagesProvider);
    await container.read(messagesProvider.future);

    final notifier = container.read(messagesProvider.notifier);

    final sendFuture = notifier.sendMessage('merhaba');

    controller.add(_sseChunk({'content': 'reply', 'done': false}));
    controller.add(_sseChunk({'done': true}));
    await controller.close();
    await sendFuture;

    expect(
      container.read(isSendingProvider),
      isFalse,
      reason: 'sendMessage() must reset isSendingProvider back to false once '
          'its stream completes normally, even for a notifier that already '
          'went through one dispose+rebuild cycle earlier in the session — '
          "otherwise the send/stop button gets stuck on \"stop\" forever "
          'from that point on.',
    );
  });
}
