import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/engine_strip.dart';

/// Stateful fake backend: answers /api/setup/status with a done-but-not-
/// loopback setup (so the real authGateProvider lands on loginNeeded), gives
/// 401 to everything while `authed` is false, and 200s once it turns true.
/// Counts how often the background-poll endpoints get hit.
class _StatefulAuthAdapter implements HttpClientAdapter {
  bool authed = false;
  int modelStatus = 0;
  int embeddingStatus = 0;
  int downloadProgress = 0;
  int moodScore = 0;
  int moodEnabled = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/api/setup/status') {
      return _json(200, {
        'needs_setup': false,
        'auth_mode': 'password',
        'loopback': false,
      });
    }
    if (options.path == '/api/models/status') modelStatus++;
    if (options.path == '/api/models/embedding/status') embeddingStatus++;
    if (options.path == '/api/models/download/progress') downloadProgress++;
    if (options.path == '/api/mood/score') moodScore++;
    if (options.path == '/api/mood/enabled') moodEnabled++;
    if (!authed) {
      return _json(401, {'error': 'yetkisiz'});
    }
    if (options.path == '/api/models/status' ||
        options.path == '/api/models/embedding/status') {
      return _json(200, {'status': 'running'});
    }
    if (options.path == '/api/models/download/progress') {
      return _json(200, <Object>[]);
    }
    if (options.path == '/api/mood/score') return _json(200, 0.5);
    if (options.path == '/api/mood/enabled') return _json(200, true);
    return _json(200, {});
  }

  ResponseBody _json(int status, Object? body) => ResponseBody.fromString(
        body == null ? '' : jsonEncode(body),
        status,
        headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
      );

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> pumpStrip(
    WidgetTester tester,
    _StatefulAuthAdapter adapter,
  ) async {
    // EngineStrip renders every indicator (model/embedding/memory/download/
    // orchestra/mood) at once once the gate is open — narrower than this and
    // it overflows even outside these tests (engine_strip_test.dart uses the
    // same size for the same reason).
    tester.view.physicalSize = const Size(1400, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = adapter;
    L10n.setLocale(MemoLocale.tr);

    final container = ProviderContainer(overrides: [
      prefsProvider.overrideWithValue(prefs),
      apiClientProvider.overrideWithValue(client),
    ]);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(
          home: Scaffold(
            body: EngineStrip(onOpenModels: _noop),
          ),
        ),
      ),
    );
    await tester.pump();
    return container;
  }

  /// Unmounts the strip, then disposes the whole container — the only way to
  /// guarantee every ambient poll's timer is gone before the framework's
  /// pending-timer check.
  Future<void> teardown(WidgetTester tester, ProviderContainer container) async {
    await tester.pumpWidget(const SizedBox());
    await tester.pump();
    container.dispose();
  }

  testWidgets(
      'while the login gate is up, EngineStrip feeds no 401s to the backend '
      '(BUG-ONB4)', (tester) async {
    final adapter = _StatefulAuthAdapter();
    final container = await pumpStrip(tester, adapter);

    // EngineStrip is always mounted (AppShell), so its ambient polls (model
    // status 30s, embedding status 30s, download progress 4s, mood 10s)
    // would normally hammer the token-gated backend every few seconds. The
    // real authGateProvider (setup/status -> loginNeeded, not loopback)
    // is what the polls consult. 65s in chunks: enough for several rounds
    // of every poll if the guard were missing.
    await tester.pump(const Duration(seconds: 65));

    expect(adapter.modelStatus, 0,
        reason: 'model status polled a token-gated backend behind the login gate');
    expect(adapter.embeddingStatus, 0,
        reason: 'embedding status polled a token-gated backend behind the login gate');
    expect(adapter.downloadProgress, 0,
        reason: 'download progress polled a token-gated backend behind the login gate');
    expect(adapter.moodScore, 0,
        reason: 'mood score polled a token-gated backend behind the login gate');
    expect(adapter.moodEnabled, 0,
        reason: 'mood enabled polled a token-gated backend behind the login gate');

    await teardown(tester, container);
  });

  testWidgets('the same polls resume once the gate closes', (tester) async {
    final adapter = _StatefulAuthAdapter();
    final container = await pumpStrip(tester, adapter);

    // The user logs in: the client gets a session token and the backend
    // starts accepting requests. The gate notices on its own 30s poll.
    SharedPreferences prefs = await SharedPreferences.getInstance();
    await prefs.setString('memo_remote_access_token', 'sess');
    adapter.authed = true;

    // Chunked pumps: each elapse flushes its microtask queue before the
    // next chunk starts, so the gate's transition lands deterministically.
    await tester.pump(const Duration(seconds: 30)); // gate poll -> ok
    await tester.pump(const Duration(seconds: 30));
    await tester.pump(const Duration(seconds: 5));

    expect(adapter.modelStatus, greaterThan(0),
        reason: 'polls must resume once the gate grants access');
    expect(adapter.embeddingStatus, greaterThan(0),
        reason: 'embedding poll must resume once the gate grants access');

    await teardown(tester, container);
  });
}

void _noop() {}