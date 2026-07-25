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

  group('syncRoutineUtcOffset (BUG_REPORT TD-1)', () {
    test('posts the current UTC offset to the sync-offset endpoint', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _CapturingAdapter({'changed': 0});
      client.dio.httpClientAdapter = adapter;

      await client.syncRoutineUtcOffset();

      expect(adapter.lastPath, '/api/routines/sync-offset');
      expect(
        adapter.lastData,
        containsPair(
          'utc_offset_minutes',
          DateTime.now().timeZoneOffset.inMinutes,
        ),
      );
    });

    test('swallows a failed request instead of throwing', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _ThrowingAdapter();

      // Must not throw — this is a best-effort background call fired
      // unawaited from connectionStatusProvider; a network error here must
      // never surface as an unhandled exception.
      await client.syncRoutineUtcOffset();
    });
  });

  // Faz 1.4, docs/plans/PLAN_voice_live_mode_faz1.md: synthesizeSpeech is
  // handleTTSSynthesize's client-side counterpart -- unlike most endpoints
  // here it must round-trip raw, non-JSON bytes (Content-Type: audio/wav),
  // not a decoded JSON body.
  group('synthesizeSpeech (Faz 1.4)', () {
    test('posts {"text": ...} and returns the raw WAV bytes unchanged', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      final wantBytes = Uint8List.fromList([82, 73, 70, 70, 1, 2, 3, 4]); // "RIFF" + fake data
      final adapter = _CapturingBytesAdapter(wantBytes);
      client.dio.httpClientAdapter = adapter;

      final gotBytes = await client.synthesizeSpeech('merhaba');

      expect(adapter.lastPath, '/api/tts/synthesize');
      expect(adapter.lastData, containsPair('text', 'merhaba'));
      expect(gotBytes, equals(wantBytes));
    });

    test('propagates a synthesis failure instead of swallowing it', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _ThrowingAdapter();

      // Unlike syncRoutineUtcOffset's best-effort fire-and-forget, a voice
      // test button (_LiveModeVoiceTest) needs the real error to show the
      // user why nothing played -- this must not be silently swallowed.
      expect(
        () => client.synthesizeSpeech('merhaba'),
        throwsA(isA<DioException>()),
      );
    });
  });
}

/// A fake [HttpClientAdapter] that records the last request's path and JSON
/// body like [_CapturingAdapter], but answers with raw bytes and an
/// audio/wav content type instead of encoding [body] as JSON -- for
/// endpoints like /api/tts/synthesize that reply with binary data directly.
class _CapturingBytesAdapter implements HttpClientAdapter {
  final Uint8List bytes;
  String? lastPath;
  Map<String, dynamic>? lastData;
  _CapturingBytesAdapter(this.bytes);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastPath = options.path;
    if (options.data is Map) {
      lastData = Map<String, dynamic>.from(options.data as Map);
    }
    return ResponseBody.fromBytes(
      bytes,
      200,
      headers: {
        Headers.contentTypeHeader: ['audio/wav'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// A fake [HttpClientAdapter] that records the last request's path and JSON
/// body, and answers with [body].
class _CapturingAdapter implements HttpClientAdapter {
  final dynamic body;
  String? lastPath;
  Map<String, dynamic>? lastData;
  _CapturingAdapter(this.body);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastPath = options.path;
    if (options.data is Map) {
      lastData = Map<String, dynamic>.from(options.data as Map);
    }
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

/// A fake [HttpClientAdapter] that always fails the request, standing in for
/// an unreachable backend.
class _ThrowingAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    throw DioException(
      requestOptions: options,
      error: 'connection refused',
    );
  }

  @override
  void close({bool force = false}) {}
}
