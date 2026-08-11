import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/auth_gate_overlay.dart';

/// Stateful fake: records requests; answers login/create-admin from the
/// response map when stubbed (e.g. a 401), otherwise "logs in" by returning
/// a session and answering /api/version with 200 afterwards — the app flow
/// needs a version 200 right after setSessionToken + invalidate to close
/// the gate.
class _StatefulAuthAdapter implements HttpClientAdapter {
  _StatefulAuthAdapter(this.statusResponses);
  final Map<String, (int, Object?)> statusResponses;
  bool authed = false;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    final stubbed = statusResponses[options.path];
    if (stubbed != null) {
      final (status, body) = stubbed;
      return _json(status, body);
    }
    if (options.path == '/api/auth/login' ||
        options.path == '/api/setup/create-admin') {
      authed = true;
      return _json(200, {'session_token': 'sess', 'role': 'admin'});
    }
    if (options.path == '/api/version' && authed) return _json(200, {});
    return _json(500, null);
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

  Future<void> pump(WidgetTester tester, _StatefulAuthAdapter adapter) async {
    tester.view.physicalSize = const Size(1200, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    // Mirrors apiClientProvider in chat_provider.dart: the learned token is
    // persisted to prefs — without this the gate can never see the token it
    // just applied and would reopen on the next poll.
    final client = MemoApiClient(
      baseUrl: 'http://memo.test',
      onRemoteTokenLearned: (t) => prefs.setString('memo_remote_access_token', t),
    );
    client.dio.httpClientAdapter = adapter;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
      ],
      child: const MaterialApp(
        home: Stack(children: [AuthGateOverlay()]),
      ),
    ));
    await tester.pump(); // StreamProvider ilk yield
    await tester.pump(const Duration(milliseconds: 50));
  }

  testWidgets('first run: decline -> gate closes, flag persisted', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    expect(find.text(L10n.t('auth_gate_other_devices_question')), findsOneWidget);
    await tester.tap(find.text(L10n.t('auth_gate_other_devices_no')));
    await tester.pumpAndSettle();
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getBool(authSetupDoneKey), isTrue);
  });

  testWidgets('first run: connect-to-remote option opens the server dialog', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('auth_gate_connect_remote')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_join_remote')), findsOneWidget);
    expect(find.text(L10n.t('remote_backend_url_field_label')), findsOneWidget);
  });

  testWidgets('login gate: connect-to-remote link opens the server dialog', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('auth_gate_join_remote')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('remote_backend_url_field_label')), findsOneWidget);
  });

  testWidgets('first run: password setup flow creates admin and closes gate', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('auth_gate_other_devices_yes')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('auth_gate_method_password')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'pw1');
    await tester.enterText(find.byType(TextField).at(2), 'pw1');
    await tester.tap(find.text(L10n.t('auth_gate_create')));
    await tester.pumpAndSettle();
    expect(
      adapter.requests.any((r) => r.path == '/api/setup/create-admin'),
      isTrue,
    );
    // gate kapandı (ok) — AuthGateOverlay boş döndü
    expect(find.text(L10n.t('auth_gate_other_devices_question')), findsNothing);
  });

  testWidgets('setup mismatch: password confirmation error shown, no request', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': true, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('auth_gate_other_devices_yes')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'pw1');
    await tester.enterText(find.byType(TextField).at(2), 'pw2');
    await tester.tap(find.text(L10n.t('auth_gate_create')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_error_password_mismatch')), findsOneWidget);
    expect(
      adapter.requests.any((r) => r.path == '/api/setup/create-admin'),
      isFalse,
    );
  });

  testWidgets('login gate: bad password shows error', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
      '/api/auth/login': (401, null),
    });
    await pump(tester, adapter);
    expect(find.text(L10n.t('auth_gate_other_devices_question')), findsNothing);
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'wrong');
    await tester.tap(find.widgetWithText(ElevatedButton, L10n.t('auth_gate_sign_in')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_error_invalid_credentials')), findsOneWidget);
  });

  testWidgets('login gate: correct password closes gate', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    await pump(tester, adapter);
    await tester.enterText(find.byType(TextField).at(0), 'admin');
    await tester.enterText(find.byType(TextField).at(1), 'pw');
    await tester.tap(find.widgetWithText(ElevatedButton, L10n.t('auth_gate_sign_in')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_sign_in')), findsNothing);
  });

  testWidgets('token mode: gateway via pasted token', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'token'}),
    });
    await pump(tester, adapter);
    await tester.enterText(find.byType(TextField).at(0), 'dev-tok');
    await tester.tap(find.widgetWithText(ElevatedButton, L10n.t('auth_gate_sign_in')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('auth_gate_enter_token')), findsNothing);
  });

  testWidgets('login gate shows which server it is connecting to', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    await pump(tester, adapter);
    // apiClientProvider override in pump() uses baseUrl http://memo.test
    expect(find.text('http://memo.test'), findsOneWidget);
    expect(find.text(L10n.t('backend_unreachable_change_server')), findsOneWidget);
  });

  testWidgets('login gate can re-point the backend server', (tester) async {
    final adapter = _StatefulAuthAdapter({
      '/api/setup/status': (200, {'needs_setup': false, 'auth_mode': 'password'}),
    });
    await pump(tester, adapter);
    await tester.tap(find.text(L10n.t('backend_unreachable_change_server')));
    await tester.pumpAndSettle();
    // The dialog (reused from BackendUnreachableView) is open — its
    // "return to local backend" action only exists there.
    expect(find.text(L10n.t('reset_to_local_backend')), findsOneWidget);
    expect(find.text(L10n.t('remote_backend_url_field_label')), findsOneWidget);
  });
}
