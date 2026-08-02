import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';

/// Answers any request with a fixed body — real sockets aren't usable under
/// `flutter test`, so this hooks Dio's adapter layer directly (same approach
/// as api_client_test.dart's _FakeChatsAdapter).
class _FakeAdapter implements HttpClientAdapter {
  final dynamic body;
  RequestOptions? lastRequest;
  _FakeAdapter(this.body);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
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

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('getChatCLIModel()', () {
    test('parses the {"model": "..."} shape', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({'model': 'opus'});

      expect(await client.getChatCLIModel('chat-1'), 'opus');
    });

    test('an unset override comes back as an empty string, not null', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({'model': ''});

      expect(await client.getChatCLIModel('chat-1'), '');
    });
  });

  group('setChatCLIModel()', () {
    test('sends the chat id and model', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _FakeAdapter({'ok': 'true'});
      client.dio.httpClientAdapter = adapter;

      await client.setChatCLIModel('chat-1', 'sonnet');

      expect(adapter.lastRequest?.data, {'id': 'chat-1', 'model': 'sonnet'});
    });

    test('an empty model clears the override rather than being rejected', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _FakeAdapter({'ok': 'true'});
      client.dio.httpClientAdapter = adapter;

      await client.setChatCLIModel('chat-1', '');

      expect(adapter.lastRequest?.data, {'id': 'chat-1', 'model': ''});
    });
  });

  group('getCLIModelOptions()', () {
    test('parses the {"models": [...]} shape', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({
        'models': ['opus', 'sonnet', 'fable'],
      });

      expect(await client.getCLIModelOptions('claude-code-cli'),
          ['opus', 'sonnet', 'fable']);
    });

    test('returns an empty list rather than throwing when "models" is malformed',
        () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({'models': 'not a list'});

      // A CLI with no override list available (e.g. Codex before its cache
      // exists) is an ordinary state, not a failure the caller should crash
      // on — the picker just shows only the "CLI default" entry.
      expect(await client.getCLIModelOptions('codex-cli'), isEmpty);
    });

    test('sends the CLI type as a query parameter', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _FakeAdapter({'models': []});
      client.dio.httpClientAdapter = adapter;

      await client.getCLIModelOptions('codex-cli');

      expect(adapter.lastRequest?.queryParameters['type'], 'codex-cli');
    });
  });
}
