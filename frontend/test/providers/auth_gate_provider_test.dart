import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/local_session_state.dart';
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

  test('loopback source skips login even with no saved token', () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {
        'needs_setup': false,
        'auth_mode': 'password',
        'loopback': true,
      }),
    });
    final info = await firstGate(c);
    expect(info.state, AuthGateState.ok,
        reason: 'loopback traffic needs no credential — the backend '
            'exempts it (remoteAuthOK), so the gate must not demand a login');
  });

  test('loopback false with no saved token still shows login gate', () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {
        'needs_setup': false,
        'auth_mode': 'password',
        'loopback': false,
      }),
    });
    expect((await firstGate(c)).state, AuthGateState.loginNeeded);
  });

  test('absent loopback field (old backend) falls back to token-based gate',
      () async {
    final c = await makeContainer({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'token'}),
    });
    expect((await firstGate(c)).state, AuthGateState.loginNeeded);
  });

  // ── Stale client state after a server wipe+reinstall (2026-08-13) ─────
  //
  // Reported live from a Raspberry Pi: uninstall-selfhosted.sh +
  // get-memo-server-beta.sh gave a brand-new backend on the same origin,
  // but the browser kept its localStorage — so the declined flag hid the
  // setup gate forever while every API call 401'd. Neither Ctrl+Shift+R
  // nor Ctrl+F5 helped (a hard reload does not touch localStorage); only
  // clearing site data by hand in DevTools did.

  test(
      'declined flag is ignored when a non-loopback probe 401s '
      '(the flag belongs to a backend that was wiped)', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': true,
          'auth_mode': 'token',
          'loopback': false,
        }),
        '/api/version': (401, null),
      },
      prefs: {authSetupDoneKey: true},
    );
    expect((await firstGate(c)).state, AuthGateState.setupNeeded,
        reason: 'setup pending + this source cannot authenticate is a '
            'contradiction; the stale flag must not hide the gate');
  });

  test('declined flag is honoured when the backend is reachable', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': true,
          'auth_mode': 'none',
          'loopback': false,
        }),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {authSetupDoneKey: true},
    );
    expect((await firstGate(c)).state, AuthGateState.ok,
        reason: 'declining remote access is a real feature — a client that '
            'can talk to the backend must keep skipping the gate');
  });

  test('declined flag is honoured on a loopback source without probing',
      () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': true,
          'auth_mode': 'token',
          'loopback': true,
        }),
        // No /api/version entry: reaching for it would fail the fake
        // adapter's lookup, proving the probe never runs for loopback.
      },
      prefs: {authSetupDoneKey: true},
    );
    expect((await firstGate(c)).state, AuthGateState.ok);
  });

  test('a changed install id wipes server-coupled state and re-gates',
      () async {
    SharedPreferences.setMockInitialValues({});
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': true,
          'auth_mode': 'token',
          'loopback': false,
          'install_id': 'new-install',
        }),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {
        serverInstallIdKey: 'old-install',
        authSetupDoneKey: true,
        'memo_remote_access_token': 'stale-token',
        'memo_session_username': 'bugra',
        'memo_tour_seen': true,
        // Device preferences — must survive.
        'memo_locale': 'en',
        'memo_theme_mode': 'dark',
        'memo_api_base_url': 'http://memo.test',
      },
    );

    expect((await firstGate(c)).state, AuthGateState.setupNeeded);

    final prefs = await SharedPreferences.getInstance();
    for (final key in serverCoupledPrefsKeys) {
      expect(prefs.get(key), isNull, reason: '$key belonged to the old install');
    }
    expect(prefs.getString(serverInstallIdKey), 'new-install');
    expect(prefs.getString('memo_locale'), 'en');
    expect(prefs.getString('memo_theme_mode'), 'dark');
    expect(prefs.getString('memo_api_base_url'), 'http://memo.test',
        reason: 'clearing the base URL would strand the client');
  });

  test('an unchanged install id leaves saved state alone', () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': false,
          'auth_mode': 'password',
          'loopback': false,
          'install_id': 'same-install',
        }),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {
        serverInstallIdKey: 'same-install',
        'memo_remote_access_token': 'valid-token',
      },
    );

    expect((await firstGate(c)).state, AuthGateState.ok);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('memo_remote_access_token'), 'valid-token');
  });

  test('first sight of an install id records it without signing the user out',
      () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': false,
          'auth_mode': 'password',
          'loopback': false,
          'install_id': 'first-seen',
        }),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {'memo_remote_access_token': 'valid-token'},
    );

    expect((await firstGate(c)).state, AuthGateState.ok,
        reason: 'every existing client upgrades into this state — resetting '
            'on first sight would sign out every working user');
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString(serverInstallIdKey), 'first-seen');
    expect(prefs.getString('memo_remote_access_token'), 'valid-token');
  });

  test('an empty install id (old backend) never counts as a mismatch',
      () async {
    final c = await makeContainer(
      {
        '/api/setup/status': (200, {
          'needs_setup': false,
          'auth_mode': 'password',
          'loopback': false,
        }),
        '/api/version': (200, {'version': 'x'}),
      },
      prefs: {
        serverInstallIdKey: 'known',
        'memo_remote_access_token': 'valid-token',
      },
    );

    expect((await firstGate(c)).state, AuthGateState.ok);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString(serverInstallIdKey), 'known');
    expect(prefs.getString('memo_remote_access_token'), 'valid-token');
  });
}