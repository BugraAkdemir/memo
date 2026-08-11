import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// Counts calls to the endpoints a background poll would hit, and answers
/// everything with 401 — exactly what a token-gated backend answers while
/// the auth gate is still up.
class _UnauthorizedAdapter implements HttpClientAdapter {
  int messagesCalls = 0;
  int modelStatusCalls = 0;
  int embeddingStatusCalls = 0;
  int downloadProgressCalls = 0;
  int moodScoreCalls = 0;
  int moodEnabledCalls = 0;
  int listChatsCalls = 0;
  int activeChatIdCalls = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/api/messages') messagesCalls++;
    if (options.path == '/api/models/status') modelStatusCalls++;
    if (options.path == '/api/models/embedding/status') embeddingStatusCalls++;
    if (options.path == '/api/models/download/progress') downloadProgressCalls++;
    if (options.path == '/api/mood/score') moodScoreCalls++;
    if (options.path == '/api/mood/enabled') moodEnabledCalls++;
    if (options.path == '/api/chats') listChatsCalls++;
    if (options.path == '/api/chats/active') activeChatIdCalls++;
    return ResponseBody.fromString(
      '{"error":"yetkisiz"}',
      401,
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

  // BUG-ONB4: while the login/setup gate is up, the backend answers every
  // request with 401 — but the app's screens are still mounted under the
  // gate overlay. messagesProvider's 401 turned into a visible
  // "Bir şeyler ters gitti. Lütfen tekrar dene." (chat_screen renders
  // FriendlyError.describeGeneric for any non-unreachable AsyncError), and
  // every background poll (models/status, embedding/status,
  // download/progress, mood/*) kept hitting the backend every few seconds
  // for nothing, spamming 401s into the console.
  group('messagesProvider against an unauthorized backend', () {
    test('a 401 response yields an empty list, never an error state', () async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final adapter = _UnauthorizedAdapter();
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = adapter;

      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith(
          (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
        ),
      ]);
      addTearDown(container.dispose);

      final messages = await container.read(messagesProvider.future);
      expect(messages, isEmpty);
    });

    test(
        'while the login gate is up no request is even attempted — '
        'empty list without touching the backend', () async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final adapter = _UnauthorizedAdapter();
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = adapter;

      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        // The gate this client actually faces on a token-gated backend
        // (RPi in LAN mode): loginNeeded, not yet passed.
        authGateProvider.overrideWith(
          (ref) => Stream.value(
            const AuthGateInfo(AuthGateState.loginNeeded, authMode: 'password'),
          ),
        ),
      ]);
      addTearDown(container.dispose);

      final messages = await container.read(messagesProvider.future);
      expect(messages, isEmpty);
      expect(adapter.messagesCalls, 0,
          reason: 'no point 401-ing a request the user cannot authorize yet');
    });
  });

  // BUG-ONB6: reported live against the RPi — on first connecting to a
  // gated backend, the chat sidebar/active-chat area showed a permanent
  // "Bir şeyler ters gitti" that only went away once something else
  // (creating a new chat) happened to re-fetch after the gate had opened;
  // on desktop, with no page-refresh escape hatch, it stayed stuck forever.
  // Same root cause as messagesProvider above (a one-shot AsyncNotifier
  // with no gate check), just never covered by the original BUG-ONB4 fix.
  group('chatListProvider / activeChatIdProvider against an unauthorized backend', () {
    test('while the login gate is up, neither provider touches the backend',
        () async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final adapter = _UnauthorizedAdapter();
      final client = MemoApiClient(baseUrl: 'http://memo.test');
      client.dio.httpClientAdapter = adapter;

      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith(
          (ref) => Stream.value(
            const AuthGateInfo(AuthGateState.loginNeeded, authMode: 'password'),
          ),
        ),
      ]);
      addTearDown(container.dispose);

      final chats = await container.read(chatListProvider.future);
      final activeId = await container.read(activeChatIdProvider.future);

      expect(chats, isEmpty);
      expect(activeId, isEmpty);
      expect(adapter.listChatsCalls, 0);
      expect(adapter.activeChatIdCalls, 0);
    });
  });
}
