import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';

/// A fake [HttpClientAdapter] that answers GET /api/chats with a fixed body.
/// Real sockets aren't usable under `flutter test` (see
/// settings_toggle_race_test.dart for why), so this hooks Dio's adapter
/// layer directly instead.
class _FakeChatsAdapter implements HttpClientAdapter {
  final dynamic body;
  _FakeChatsAdapter(this.body);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

// Regression test for BUG-L2: listChats()'s object-wrapped fallback branch
// did `res.data['chats'] as List` guarded only by a `!= null` check, so a
// malformed 'chats' field (backend bug, proxy mangling the body, etc.) threw
// a raw, unhelpful TypeError instead of the descriptive Exception the
// sibling root-list branch already produces via `_guard<List>`.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('listChats() parses the object-wrapped {"chats": [...]} shape', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeChatsAdapter({
      'chats': [
        {
          'id': 'a',
          'title': 'Sohbet A',
          'created_at': '',
          'updated_at': '',
          'msg_count': 0,
        },
      ],
    });

    final chats = await client.listChats();

    expect(chats, hasLength(1));
    expect(chats.first.id, 'a');
  });

  test('listChats() throws a descriptive Exception when "chats" is malformed', () async {
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeChatsAdapter({'chats': 'not a list'});

    expect(
      () => client.listChats(),
      throwsA(
        isA<Exception>().having(
          (e) => e.toString(),
          'message',
          contains('Expected List'),
        ),
      ),
    );
  });
}
