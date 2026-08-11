import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';

/// Answers per-path with a fixed (status, body) pair — mirrors the adapter
/// in api_client_test.dart (test files don't share code).
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.responses);
  final Map<String, (int, Object?)> responses;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final (status, body) = responses[options.path] ?? (500, null);
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      status,
      headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<AuthGateInfo> firstGate(ProviderContainer container) async {
    return container.read(authGateProvider.future);
  }

  Future<ProviderContainer> makeContainer(
    Map<String, (int, Object?)> responses, {
    Map<String, Object> prefs = const {},
  }) async {
    SharedPreferences.setMockInitialValues(prefs);
    final prefs2 = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _FakeAdapter(responses);
    final container = ProviderContainer(overrides: [
      apiClientProvider.overrideWithValue(client),
      prefsProvider.overrideWithValue(prefs2),
    ]);
    addTearDown(container.dispose);
    return container;
  }

  test('needs_setup without flag shows setup gate', () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    final info = await firstGate(c);
    expect(info.state, AuthGateState.setupNeeded);
    expect(info.authMode, 'token');
  });

  test('declined flag skips the setup gate', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
      },
      prefs: {authSetupDoneKey: true},
    );
    expect((await firstGate(c)).state, AuthGateState.ok);
  });

  test('no saved token after setup completed -> login gate', () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    final info = await firstGate(c);
    expect(info.state, AuthGateState.loginNeeded);
    expect(info.authMode, 'password');
  });

  test('saved token + 200 version -> ok', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {'memo_remote_access_token': 'tok'},
    );
    expect((await firstGate(c)).state, AuthGateState.ok);
  });

  test('saved token + 401 -> login gate (expired session)', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
        '/api/version': (401, null),
      },
      prefs: {'memo_remote_access_token': 'stale'},
    );
    expect((await firstGate(c)).state, AuthGateState.loginNeeded);
  });

  test('backend down -> ok (unreachable overlay handles it)', () async {
    final c = await makeContainer({});
    expect((await firstGate(c)).state, AuthGateState.ok);
  });
}