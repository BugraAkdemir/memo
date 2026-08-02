import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/cli_command.dart';

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

  group('CLICommand', () {
    test('parses the backend shape and renders its slash form', () {
      final cmd = CLICommand.fromJson({
        'name': 'frontend-design',
        'description': 'Distinctive visual design guidance',
        'source': 'skill',
      });

      expect(cmd.name, 'frontend-design');
      expect(cmd.description, 'Distinctive visual design guidance');
      expect(cmd.source, 'skill');
      expect(cmd.slash, '/frontend-design');
    });

    test('tolerates missing optional fields', () {
      final cmd = CLICommand.fromJson({'name': 'init'});

      expect(cmd.description, isEmpty);
      expect(cmd.source, isEmpty);
      expect(cmd.slash, '/init');
    });
  });

  group('listCLICommands()', () {
    test('parses the {"commands": [...]} shape', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({
        'commands': [
          {'name': 'review', 'description': 'Review a PR', 'source': 'builtin'},
          {'name': 'deploy', 'description': '', 'source': 'project'},
        ],
      });

      final cmds = await client.listCLICommands('claude-code-cli', 'chat-1');

      expect(cmds, hasLength(2));
      expect(cmds.first.slash, '/review');
      expect(cmds.last.source, 'project');
    });

    test('sends the CLI type and chat id the backend needs to resolve them', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _FakeAdapter({'commands': []});
      client.dio.httpClientAdapter = adapter;

      await client.listCLICommands('codex-cli', 'chat-42');

      // chat_id is what lets the backend find PROJECT-level commands (they
      // live under that chat's own CLI working directory) — dropping it
      // would silently reduce every chat to user-level commands only.
      expect(adapter.lastRequest?.queryParameters['type'], 'codex-cli');
      expect(adapter.lastRequest?.queryParameters['chat_id'], 'chat-42');
    });

    test('returns an empty list instead of throwing when "commands" is malformed',
        () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({'commands': 'not a list'});

      // A bad payload must not take the composer down with it — the "/"
      // popup just shows its empty state and typing the command by hand
      // still works, since the CLI resolves it either way.
      expect(await client.listCLICommands('claude-code-cli', 'c'), isEmpty);
    });

    test('skips non-object entries rather than crashing on a mixed list', () async {
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = _FakeAdapter({
        'commands': [
          {'name': 'good', 'description': '', 'source': 'user'},
          'unexpected string entry',
          42,
        ],
      });

      final cmds = await client.listCLICommands('claude-code-cli', 'c');

      expect(cmds, hasLength(1));
      expect(cmds.single.name, 'good');
    });
  });
}
