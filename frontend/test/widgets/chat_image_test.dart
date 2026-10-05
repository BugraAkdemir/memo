import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/chat_message_list.dart';

const _png1x1 =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';

/// Serves /api/image the way the backend does, and records what was asked.
class _ImageAdapter implements HttpClientAdapter {
  _ImageAdapter(this.status);
  final int status;
  final List<String> requested = [];

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    requested.add(options.uri.toString());
    final body = status == 200
        ? jsonEncode({'data': 'data:image/png;base64,$_png1x1'})
        : 'forbidden';
    return ResponseBody.fromString(body, status, headers: {
      Headers.contentTypeHeader: [
        status == 200 ? Headers.jsonContentType : 'text/plain'
      ],
    });
  }

  @override
  void close({bool force = false}) {}
}

Widget _app(MemoApiClient client, String imagePath) => ProviderScope(
      overrides: [apiClientProvider.overrideWithValue(client)],
      child: MaterialApp(
        home: Scaffold(
          body: ChatMessageList(
            apiBaseUrl: 'http://memo.test',
            messages: [
              ChatMessage(
                role: 'assistant',
                content: '',
                timestamp: '12:00',
                imagePath: imagePath,
              ),
            ],
          ),
        ),
      ),
    );

/// A message image used to be drawn with Image.file on the backend's path,
/// which only exists on the backend's machine — web, the mobile app and
/// remote sessions showed nothing. It must come from the backend instead.
void main() {
  testWidgets('a message image is fetched from the backend and shown',
      (tester) async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    final adapter = _ImageAdapter(200);
    client.dio.httpClientAdapter = adapter;

    await tester.pumpWidget(
        _app(client, '/srv/memo/data/generated-images/memo-1.png'));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(adapter.requested.single, contains('/api/image'));
    expect(adapter.requested.single,
        contains(Uri.encodeQueryComponent('/srv/memo/data/generated-images/memo-1.png')));
    final images = tester.widgetList<Image>(find.byType(Image));
    expect(images.any((i) => i.image is MemoryImage), isTrue,
        reason: 'the bytes from /api/image must be what is drawn');
  });

  testWidgets('a refused image with no local file renders nothing, no crash',
      (tester) async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _ImageAdapter(403);

    await tester.pumpWidget(_app(client, '/nonexistent/elsewhere.png'));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.byType(Image), findsNothing);
  });
}
