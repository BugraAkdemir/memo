import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/telemetry.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/privacy_tab.dart';
import 'package:memo_flutter/widgets/telemetry_prompt.dart';

/// A tiny fake backend: remembers the consent it was told and records every
/// request, so a test can say both what the screen showed and what it sent.
class _FakeTelemetryBackend implements HttpClientAdapter {
  _FakeTelemetryBackend({this.consent = '', this.build = 'official', this.failConsent = false, this.previewJson = ''});
  String consent;
  final String build;
  final bool failConsent;
  final String previewJson;
  final requests = <String>[];

  Map<String, Object?> get _status => {
        'enabled': consent == 'yes',
        'needs_ask': build == 'official' && consent == '',
        'consent': consent,
        'env_off': false,
        'build': build,
        'endpoint': 'https://tel.memocpp.com',
        'device_id': '0f3c9a1e',
        'consent_day': consent == '' ? '' : '2026-10-26',
        'last_sent_day': '',
      };

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    String body = '';
    if (requestStream != null) {
      final bytes = await requestStream.fold<List<int>>(<int>[], (a, b) => a..addAll(b));
      body = utf8.decode(bytes);
    }
    requests.add('${options.method} ${options.path} $body'.trim());
    Object? out;
    var status = 200;
    switch (options.path) {
      case '/api/telemetry/status':
        out = _status;
      case '/api/telemetry/preview':
        out = {'day': '2026-10-26', 'empty': previewJson.isEmpty, 'json': previewJson};
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
    return ResponseBody.fromString(jsonEncode(out), status, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpApp(WidgetTester tester, _FakeTelemetryBackend backend, Widget home, {bool setupDone = true}) async {
    SharedPreferences.setMockInitialValues({if (setupDone) 'memo_setup_complete': true});
    final prefs = await SharedPreferences.getInstance();
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = backend;
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
        setupCompleteProvider.overrideWith((ref) => SetupCompleteNotifier(ref, prefs, confirmWithBackend: false)),
      ],
      child: MaterialApp(home: Scaffold(body: home)),
    ));
    // The gate answers, then the status is fetched, then the question may open:
    // a few real frames, not one.
    for (var i = 0; i < 4; i++) {
      await tester.pump(const Duration(milliseconds: 50));
    }
    await tester.pumpAndSettle();
  }

  group('shouldAskTelemetry', () {
    test('asks only an unanswered official install that is past setup', () {
      const due = TelemetryStatus(needsAsk: true, consent: '', build: 'official');
      expect(shouldAskTelemetry(due, setupComplete: true), isTrue);
      expect(shouldAskTelemetry(due, setupComplete: false), isFalse, reason: 'never on top of onboarding');
      expect(shouldAskTelemetry(null, setupComplete: true), isFalse, reason: 'unknown is not a reason to ask');
      expect(shouldAskTelemetry(const TelemetryStatus(consent: 'no'), setupComplete: true), isFalse);
      expect(shouldAskTelemetry(const TelemetryStatus(build: 'source'), setupComplete: true), isFalse, reason: 'a source build never asks');
    });
  });

  testWidgets('Privacy tab: shows "not asked yet" and answers only when a button is pressed', (tester) async {
    final backend = _FakeTelemetryBackend();
    await pumpApp(tester, backend, const PrivacyTab());

    expect(find.byKey(const Key('telemetryStatusText')), findsOneWidget);
    expect(find.text(L10n.t('telemetry_status_not_asked')), findsOneWidget);
    expect(backend.requests.where((r) => r.startsWith('POST')), isEmpty, reason: 'opening the tab must not answer anything');

    await tester.tap(find.byKey(const Key('telemetryShare')));
    await tester.pumpAndSettle();
    expect(backend.requests.where((r) => r.startsWith('POST')).last, 'POST /api/telemetry/consent {"share":true}');
    expect(find.text(L10n.t('telemetry_status_yes')), findsOneWidget);
  });

  testWidgets('Privacy tab: the two answers are equal buttons and neither is pre-pressed', (tester) async {
    final backend = _FakeTelemetryBackend();
    await pumpApp(tester, backend, const PrivacyTab());
    final yes = tester.widget<OutlinedButton>(find.byKey(const Key('telemetryShare')));
    final no = tester.widget<OutlinedButton>(find.byKey(const Key('telemetryDecline')));
    expect(yes.runtimeType, no.runtimeType);
    expect(yes.style, no.style, reason: 'no button is styled as the recommended one');
  });

  testWidgets('Privacy tab: a failed save is said out loud and the status does not change', (tester) async {
    final backend = _FakeTelemetryBackend(failConsent: true);
    await pumpApp(tester, backend, const PrivacyTab());
    await tester.tap(find.byKey(const Key('telemetryShare')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Could not do that'), findsOneWidget);
    expect(find.text(L10n.t('telemetry_status_not_asked')), findsOneWidget);
  });

  testWidgets('Privacy tab: the preview is fetched only when asked for, and shows the content', (tester) async {
    final backend = _FakeTelemetryBackend(previewJson: '{"schema":1,"day":"2026-10-26","usage":[]}');
    await pumpApp(tester, backend, const PrivacyTab());
    expect(backend.requests.any((r) => r.contains('/preview')), isFalse);

    await tester.ensureVisible(find.byKey(const Key('telemetryPreviewToggle')));
    await tester.tap(find.byKey(const Key('telemetryPreviewToggle')));
    await tester.pumpAndSettle();
    expect(backend.requests.any((r) => r.contains('/preview')), isTrue);
    expect(find.byKey(const Key('telemetryPreviewJson')), findsOneWidget);
    expect(find.textContaining('"schema": 1'), findsOneWidget);
  });

  testWidgets('Privacy tab: delete asks first and does nothing on cancel', (tester) async {
    final backend = _FakeTelemetryBackend(consent: 'yes');
    await pumpApp(tester, backend, const PrivacyTab());
    await tester.ensureVisible(find.byKey(const Key('telemetryDelete')));
    await tester.tap(find.byKey(const Key('telemetryDelete')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('telemetry_delete_confirm_title')), findsOneWidget);
    await tester.tap(find.text(L10n.t('cancel')));
    await tester.pumpAndSettle();
    expect(backend.requests.any((r) => r.contains('/delete')), isFalse);

    await tester.tap(find.byKey(const Key('telemetryDelete')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('delete')));
    await tester.pumpAndSettle();
    expect(backend.requests.any((r) => r == 'POST /api/telemetry/delete {}'), isTrue);
  });

  testWidgets('Privacy tab: a source build says it never asks', (tester) async {
    final backend = _FakeTelemetryBackend(build: 'source');
    await pumpApp(tester, backend, const PrivacyTab());
    expect(find.text(L10n.t('telemetry_source_build')), findsOneWidget);
  });

  testWidgets('First-run question: appears for an unanswered official install, cannot be dismissed, and answers on a button press', (tester) async {
    final backend = _FakeTelemetryBackend();
    await pumpApp(tester, backend, const TelemetryPrompt());
    expect(find.text(L10n.t('telemetry_ask_title')), findsOneWidget);

    // Tapping outside does not close it.
    await tester.tapAt(const Offset(5, 5));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('telemetry_ask_title')), findsOneWidget);
    expect(backend.requests.where((r) => r.startsWith('POST')), isEmpty);

    await tester.tap(find.byKey(const Key('telemetryAskDecline')));
    await tester.pumpAndSettle();
    expect(backend.requests.where((r) => r.startsWith('POST')).last, 'POST /api/telemetry/consent {"share":false}');
    expect(find.text(L10n.t('telemetry_ask_title')), findsNothing);
  });

  testWidgets('First-run question: not shown during setup, after an answer, or on a source build', (tester) async {
    await pumpApp(tester, _FakeTelemetryBackend(), const TelemetryPrompt(), setupDone: false);
    expect(find.text(L10n.t('telemetry_ask_title')), findsNothing);

    await pumpApp(tester, _FakeTelemetryBackend(consent: 'no'), const TelemetryPrompt());
    expect(find.text(L10n.t('telemetry_ask_title')), findsNothing);

    await pumpApp(tester, _FakeTelemetryBackend(build: 'source'), const TelemetryPrompt());
    expect(find.text(L10n.t('telemetry_ask_title')), findsNothing);
  });

  testWidgets('First-run question: a save that fails keeps the question open and says why', (tester) async {
    final backend = _FakeTelemetryBackend(failConsent: true);
    await pumpApp(tester, backend, const TelemetryPrompt());
    await tester.tap(find.byKey(const Key('telemetryAskShare')));
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('telemetry_ask_title')), findsOneWidget);
    expect(find.byKey(const Key('telemetryAskError')), findsOneWidget);
  });
}
