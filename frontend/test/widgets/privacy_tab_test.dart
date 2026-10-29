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
import 'package:memo_flutter/widgets/settings/tabs/privacy_tab.dart';
import 'package:memo_flutter/widgets/setup_wizard_view.dart';

/// A tiny fake backend: remembers the consent it was told and records every
/// request, so a test can say both what the screen showed and what it sent.
class _FakeTelemetryBackend implements HttpClientAdapter {
  _FakeTelemetryBackend({
    this.consent = 'yes',
    this.build = 'official',
    this.failConsent = false,
    this.envOff = false,
  });
  String consent;
  final String build;
  final bool failConsent;
  final bool envOff;
  final String previewJson = '';
  final requests = <String>[];

  Map<String, Object?> get _status => {
    'enabled': consent == 'yes' && !envOff,
    'consent': consent,
    'env_off': envOff,
    'build': build,
    'endpoint': 'https://tel.memocpp.com',
    'device_id': '0f3c9a1e',
    'consent_day': consent == '' ? '' : '2026-10-26',
    'last_sent_day': '',
  };

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    String body = '';
    if (requestStream != null) {
      final bytes = await requestStream.fold<List<int>>(
        <int>[],
        (a, b) => a..addAll(b),
      );
      body = utf8.decode(bytes);
    }
    requests.add('${options.method} ${options.path} $body'.trim());
    Object? out;
    var status = 200;
    switch (options.path) {
      case '/api/telemetry/status':
        out = _status;
      case '/api/telemetry/preview':
        out = {
          'day': '2026-10-26',
          'empty': previewJson.isEmpty,
          'json': previewJson,
        };
      case '/api/telemetry/consent':
        if (failConsent) {
          status = 500;
          out = {'error': 'disk full'};
        } else {
          consent = (jsonDecode(body) as Map)['share'] == true ? 'yes' : 'no';
          out = _status;
        }
      case '/api/telemetry/reset-id':
      case '/api/telemetry/delete':
        if (options.path.endsWith('delete')) consent = 'no';
        out = _status;
      default:
        status = 404;
    }
    return ResponseBody.fromString(
      jsonEncode(out),
      status,
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

  Future<void> pumpApp(
    WidgetTester tester,
    _FakeTelemetryBackend backend,
    Widget home, {
    bool setupDone = true,
  }) async {
    SharedPreferences.setMockInitialValues({
      if (setupDone) 'memo_setup_complete': true,
    });
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = backend;
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(client),
          prefsProvider.overrideWithValue(prefs),
          authGateProvider.overrideWith(
            (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
          ),
          setupCompleteProvider.overrideWith(
            (ref) =>
                SetupCompleteNotifier(ref, prefs, confirmWithBackend: false),
          ),
        ],
        child: MaterialApp(home: Scaffold(body: home)),
      ),
    );
    // The gate answers, then the status is fetched, then the question may open:
    // a few real frames, not one.
    for (var i = 0; i < 4; i++) {
      await tester.pump(const Duration(milliseconds: 50));
    }
    await tester.pumpAndSettle();
  }

  testWidgets(
    'Privacy tab: a single switch saves off and on without changing anything on open',
    (tester) async {
      final backend = _FakeTelemetryBackend();
      await pumpApp(tester, backend, const PrivacyTab());
      final toggle = find.byKey(const Key('telemetrySharingToggle'));
      expect(tester.widget<Switch>(toggle).value, isTrue);
      expect(find.text(L10n.t('privacy_share_usage_desc')), findsOneWidget);
      expect(find.byType(OutlinedButton), findsNothing);
      expect(find.byKey(const Key('telemetryPreviewToggle')), findsNothing);
      expect(find.textContaining('0f3c9a1e'), findsNothing);
      expect(backend.requests.where((r) => r.startsWith('POST')), isEmpty);
      expect(backend.requests.any((r) => r.contains('/preview')), isFalse);

      await tester.tap(toggle);
      for (var i = 0; i < 4; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      await tester.pumpAndSettle();
      expect(
        backend.requests.where((r) => r.startsWith('POST')).last,
        'POST /api/telemetry/consent {"share":false}',
      );
      expect(tester.widget<Switch>(toggle).value, isFalse);

      await tester.tap(toggle);
      for (var i = 0; i < 4; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      await tester.pumpAndSettle();
      expect(
        backend.requests.where((r) => r.startsWith('POST')).last,
        'POST /api/telemetry/consent {"share":true}',
      );
      expect(tester.widget<Switch>(toggle).value, isTrue);
    },
  );

  testWidgets(
    'Privacy tab: a failed save keeps the saved value and shows an inline error',
    (tester) async {
      final backend = _FakeTelemetryBackend(failConsent: true);
      await pumpApp(tester, backend, const PrivacyTab());
      await tester.tap(find.byKey(const Key('telemetrySharingToggle')));
      await tester.pumpAndSettle();
      expect(find.textContaining('Could not do that'), findsOneWidget);
      expect(tester.widget<Switch>(find.byType(Switch)).value, isTrue);
    },
  );

  testWidgets('Privacy tab: the environment override disables the switch', (
    tester,
  ) async {
    final backend = _FakeTelemetryBackend(envOff: true);
    await pumpApp(tester, backend, const PrivacyTab());
    final toggle = tester.widget<Switch>(find.byType(Switch));
    expect(toggle.value, isFalse);
    expect(toggle.onChanged, isNull);
    expect(backend.requests.where((r) => r.startsWith('POST')), isEmpty);
  });

  testWidgets('Privacy tab: a source build starts with the switch off', (
    tester,
  ) async {
    await pumpApp(
      tester,
      _FakeTelemetryBackend(consent: '', build: 'source'),
      const PrivacyTab(),
    );
    expect(tester.widget<Switch>(find.byType(Switch)).value, isFalse);
  });

  testWidgets(
    'Setup wizard: the switch is on by default, turns it off on a tap and back on again',
    (tester) async {
      final backend = _FakeTelemetryBackend();
      await pumpApp(
        tester,
        backend,
        const SetupTelemetryToggle(),
        setupDone: false,
      );

      expect(
        tester.widget<Switch>(find.byType(Switch)).value,
        isTrue,
        reason: 'an official build shares by default',
      );
      expect(
        backend.requests.where((r) => r.startsWith('POST')),
        isEmpty,
        reason: 'showing the switch must not answer for the person',
      );

      await tester.tap(find.byType(Switch));
      for (var i = 0; i < 4; i++) {
        await tester.pump(
          const Duration(milliseconds: 50),
        ); // the answer is saved, then the status is read again
      }
      await tester.pumpAndSettle();
      expect(
        backend.requests.where((r) => r.startsWith('POST')).last,
        'POST /api/telemetry/consent {"share":false}',
      );
      expect(tester.widget<Switch>(find.byType(Switch)).value, isFalse);

      await tester.tap(find.byType(Switch));
      for (var i = 0; i < 4; i++) {
        await tester.pump(
          const Duration(milliseconds: 50),
        ); // the answer is saved, then the status is read again
      }
      await tester.pumpAndSettle();
      expect(
        backend.requests.where((r) => r.startsWith('POST')).last,
        'POST /api/telemetry/consent {"share":true}',
      );
    },
  );

  testWidgets(
    'Setup wizard: a source build starts off, and the details open on request',
    (tester) async {
      await pumpApp(
        tester,
        _FakeTelemetryBackend(consent: '', build: 'source'),
        const SetupTelemetryToggle(),
        setupDone: false,
      );
      expect(tester.widget<Switch>(find.byType(Switch)).value, isFalse);

      await tester.tap(find.byKey(const Key('setupTelemetryDetails')));
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('telemetry_never_title')), findsOneWidget);
    },
  );

  testWidgets(
    'Setup wizard: a save that fails leaves the switch where it was',
    (tester) async {
      final backend = _FakeTelemetryBackend(failConsent: true);
      await pumpApp(
        tester,
        backend,
        const SetupTelemetryToggle(),
        setupDone: false,
      );
      await tester.tap(find.byType(Switch));
      for (var i = 0; i < 4; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      await tester.pumpAndSettle();
      expect(
        tester.widget<Switch>(find.byType(Switch)).value,
        isTrue,
        reason: 'an unsaved "off" must not look saved',
      );
    },
  );
}
