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

  // Regression tests for BUG-L5: a desktop client restarted with ngrok
  // auto-start on used to send its very first request with no
  // X-Memo-Token at all, because the token was only ever learned
  // in-memory from a getRemoteAccess()/setRemoteAccess() response —
  // reset to nothing on every app launch, while the backend could already
  // be bound to 0.0.0.0 (token-gated) before any such call happened.
  group('remote-access token persistence (BUG-L5)', () {
    test('savedRemoteToken seeds the X-Memo-Token header before any request', () {
      final client = MemoApiClient(
        baseUrl: 'http://memo.test',
        savedRemoteToken: 'cached-from-last-launch',
      );

      expect(
        client.dio.options.headers['X-Memo-Token'],
        'cached-from-last-launch',
      );
    });

    test('an empty or null savedRemoteToken does not set the header', () {
      final withNull = MemoApiClient(baseUrl: 'http://memo.test');
      final withEmpty = MemoApiClient(
        baseUrl: 'http://memo.test',
        savedRemoteToken: '',
      );

      expect(withNull.dio.options.headers.containsKey('X-Memo-Token'), isFalse);
      expect(withEmpty.dio.options.headers.containsKey('X-Memo-Token'), isFalse);
    });

    test('getRemoteAccess() applies and persists a freshly learned token', () async {
      String? persisted;
      final client = MemoApiClient(
        baseUrl: 'http://memo.test',
        onRemoteTokenLearned: (token) => persisted = token,
      );
      client.dio.httpClientAdapter = _FakeChatsAdapter({
        'enabled': true,
        'token': 'freshly-issued-token',
      });

      await client.getRemoteAccess();

      expect(client.dio.options.headers['X-Memo-Token'], 'freshly-issued-token');
      expect(persisted, 'freshly-issued-token');
    });

    test('getRemoteAccess() with no token in the response does not clobber the header or fire the callback', () async {
      var callbackFired = false;
      final client = MemoApiClient(
        baseUrl: 'http://memo.test',
        savedRemoteToken: 'still-the-cached-one',
        onRemoteTokenLearned: (_) => callbackFired = true,
      );
      client.dio.httpClientAdapter = _FakeChatsAdapter({'enabled': false});

      await client.getRemoteAccess();

      expect(client.dio.options.headers['X-Memo-Token'], 'still-the-cached-one');
      expect(callbackFired, isFalse);
    });
  });
}
