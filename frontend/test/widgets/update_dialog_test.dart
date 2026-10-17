import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/models/app_update.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/providers/update_provider.dart';
import 'package:memo_flutter/widgets/update_dialog.dart';
import 'package:memo_flutter/widgets/update_prompt.dart';

/// A scriptable backend: answers the update endpoints from mutable state and
/// records every request.
class _Backend implements HttpClientAdapter {
  Map<String, dynamic> check = {};
  final List<Map<String, dynamic>> statuses = []; // popped one per status poll; the last one repeats
  final List<RequestOptions> requests = [];
  int? applyStatus; // e.g. 403 or 409
  bool versionDown = false;

  @override
  Future<ResponseBody> fetch(RequestOptions o, Stream<Uint8List>? s, Future<void>? c) async {
    requests.add(o);
    switch (o.path) {
      case '/api/update/check':
        return _json(200, check);
      case '/api/update/apply':
        if (applyStatus != null) return _json(applyStatus!, {'error': 'nope'});
        return _json(202, {'status': 'started'});
      case '/api/update/status':
        final next = statuses.length > 1 ? statuses.removeAt(0) : (statuses.isEmpty ? {'state': 'idle'} : statuses.first);
        return _json(200, next);
      case '/api/version':
        if (versionDown) throw DioException(requestOptions: o, type: DioExceptionType.connectionError);
        return _json(200, {'version': 'V4.7.0'});
    }
    return _json(404, null);
  }

  ResponseBody _json(int st, Object? b) => ResponseBody.fromString(
        b == null ? '' : jsonEncode(b),
        st,
        headers: {Headers.contentTypeHeader: [Headers.jsonContentType]},
      );

  @override
  void close({bool force = false}) {}
}

Map<String, dynamic> _appCheck({bool required = false, String mode = 'auto'}) => {
      'current': 'V4.6.0',
      'latest': 'V4.7.0',
      'available': true,
      'required': required,
      'note_tr': 'Memo daha kararlı çalışıyor.',
      'note_en': 'Memo runs more reliably.',
      'mode': mode,
      'size_bytes': 800 * 1024 * 1024,
      'components': <Object>[],
    };

/// Lets the fake backend's futures and the notifier's timers make progress: a
/// bare pump() drains microtasks once, but a request chains several of them.
Future<void> settle(WidgetTester tester, [int ms = 100]) async {
  for (var t = 0; t < ms; t += 10) {
    await tester.pump(const Duration(milliseconds: 10));
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  late _Backend backend;
  late SharedPreferences prefs;

  setUp(() async {
    L10n.setLocale(MemoLocale.en);
    backend = _Backend();
    SharedPreferences.setMockInitialValues({});
    prefs = await SharedPreferences.getInstance();
  });

  ProviderContainer? lastContainer;

  Future<void> pumpHost(
    WidgetTester tester, {
    required Widget Function(BuildContext) opener,
    bool localDesktop = true,
    void Function()? onQuit,
  }) async {
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = backend;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        prefsProvider.overrideWithValue(prefs),
        // The gate is open, as on a normal desktop.
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
        updateFlowProvider.overrideWith((ref) {
          final n = UpdateFlowNotifier(ref)
            ..pollEvery = const Duration(milliseconds: 20)
            ..isLocalDesktop = (() => localDesktop)
            ..quitApp = (onQuit ?? () {});
          return n;
        }),
      ],
      child: MaterialApp(
        theme: ThemeData(extensions: [MemoTheme.dark]),
        home: Builder(builder: (ctx) {
          lastContainer = ProviderScope.containerOf(ctx);
          return Scaffold(body: opener(ctx));
        }),
      ),
    ));
    await tester.pump();
  }

  Widget openDialogButton(AppUpdateInfo info) => Builder(
        builder: (ctx) => TextButton(
          key: const Key('open'),
          onPressed: () => showAppUpdateDialog(ctx, info),
          child: const Text('open'),
        ),
      );

  Future<void> openDialog(WidgetTester tester, AppUpdateInfo info, {bool localDesktop = true, void Function()? onQuit}) async {
    await pumpHost(tester, opener: (_) => openDialogButton(info), localDesktop: localDesktop, onQuit: onQuit);
    await tester.tap(find.byKey(const Key('open')));
    await tester.pumpAndSettle();
  }

  group('the update window', () {
    testWidgets('shows versions, size and the note in the UI language', (tester) async {
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      expect(find.text(L10n.t('update_title')), findsOneWidget);
      expect(find.byKey(const Key('update_versions')), findsOneWidget);
      expect(find.textContaining('V4.6.0'), findsWidgets);
      expect(find.textContaining('V4.7.0'), findsWidgets);
      expect(find.text('Memo runs more reliably.'), findsOneWidget);
      expect(find.textContaining('800 MB'), findsOneWidget);
      expect(find.byKey(const Key('update_now')), findsOneWidget);
      expect(find.byKey(const Key('update_skip')), findsOneWidget);
      expect(find.byKey(const Key('update_later')), findsOneWidget);
    });

    testWidgets('Turkish UI shows the Turkish note and Turkish labels', (tester) async {
      L10n.setLocale(MemoLocale.tr);
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      expect(find.text('Memo daha kararlı çalışıyor.'), findsOneWidget);
      expect(find.text(L10n.t('update_now')), findsOneWidget);
      expect(L10n.t('update_now'), isNot('update_now'));
      expect(L10n.t('update_now'), isNot(equals(_enOf('update_now'))));
    });

    testWidgets('says so when the previous attempt failed and was rolled back', (tester) async {
      final m = _appCheck()..['last_failure'] = 'update to V4.7.0 failed and was rolled back: the new version exited';
      await openDialog(tester, AppUpdateInfo.fromJson(m));
      expect(find.byKey(const Key('update_last_failure')), findsOneWidget);
      expect(find.textContaining('rolled back'), findsOneWidget);
      // ...and offers another try.
      expect(find.byKey(const Key('update_now')), findsOneWidget);
    });

    testWidgets('shows no failure line when nothing failed', (tester) async {
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      expect(find.byKey(const Key('update_last_failure')), findsNothing);
    });

    testWidgets('a required update has no "Skip this version", but keeps "Later"', (tester) async {
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck(required: true)));
      expect(find.byKey(const Key('update_skip')), findsNothing);
      expect(find.byKey(const Key('update_later')), findsOneWidget);
      expect(find.byKey(const Key('update_required_hint')), findsOneWidget);
    });

    testWidgets('Skip remembers the version and closes the window', (tester) async {
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      await tester.tap(find.byKey(const Key('update_skip')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('update_versions')), findsNothing);
      expect(prefs.getString(skippedUpdateKey), 'V4.7.0');
    });

    testWidgets('Later closes it without remembering anything', (tester) async {
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      await tester.tap(find.byKey(const Key('update_later')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('update_versions')), findsNothing);
      expect(prefs.getString(skippedUpdateKey), isNull);
    });

    testWidgets('a manual install is pointed at the download page, with no one-click button', (tester) async {
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck(mode: 'manual')));
      expect(find.byKey(const Key('update_manual_body')), findsOneWidget);
      expect(find.byKey(const Key('update_open_site')), findsOneWidget);
      expect(find.byKey(const Key('update_now')), findsNothing);
      expect(backend.requests.where((r) => r.path == '/api/update/apply'), isEmpty);
    });

    testWidgets('Update now starts the update, follows progress and quits the desktop app on restart', (tester) async {
      var quit = 0;
      backend.statuses.addAll([
        {'state': 'downloading', 'kind': 'app', 'done': 400, 'total': 1000},
        {'state': 'installing', 'kind': 'app'},
        {'state': 'restarting', 'kind': 'app'},
      ]);
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()), onQuit: () => quit++);
      await tester.tap(find.byKey(const Key('update_now')));
      await settle(tester, 30);
      expect(backend.requests.where((r) => r.path == '/api/update/apply').length, 1);
      final body = backend.requests.firstWhere((r) => r.path == '/api/update/apply').data as Map;
      expect(body['kind'], 'app');

      await settle(tester, 20);
      expect(find.byKey(const Key('update_progress')), findsOneWidget);
      // Buttons are gone while it runs: nothing to click that could start it twice.
      expect(find.byKey(const Key('update_now')), findsNothing);
      expect(find.byKey(const Key('update_skip')), findsNothing);

      await settle(tester, 100);
      expect(find.text(L10n.t('update_restarting')), findsOneWidget);
      await settle(tester, 1500);
      expect(quit, 1, reason: 'the desktop UI must leave so the updater can bring Memo back');
    });

    testWidgets('a browser UI does NOT quit; it waits for the backend to go away and return', (tester) async {
      var quit = 0;
      backend.statuses.add({'state': 'restarting', 'kind': 'app'});
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()), localDesktop: false, onQuit: () => quit++);
      await tester.tap(find.byKey(const Key('update_now')));
      await settle(tester, 100);
      expect(find.text(L10n.t('update_restarting')), findsOneWidget);
      expect(quit, 0);

      // Still up -> not "back" yet (it hasn't gone down).
      await settle(tester, 100);
      expect(find.byKey(const Key('update_reload')), findsNothing);

      backend.versionDown = true;
      await settle(tester, 100);
      backend.versionDown = false;
      await settle(tester, 100);
      expect(find.byKey(const Key('update_reload')), findsOneWidget);
      expect(quit, 0);
    });

    testWidgets('a rejected start (not an admin) shows the error and allows trying again', (tester) async {
      backend.applyStatus = 403;
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      await tester.tap(find.byKey(const Key('update_now')));
      await settle(tester, 100);
      expect(find.byKey(const Key('update_error')), findsOneWidget);
      expect(find.text(L10n.t('update_retry')), findsOneWidget);
      expect(find.byKey(const Key('update_now')), findsOneWidget);
    });

    testWidgets('a failure reported by the backend is shown', (tester) async {
      backend.statuses.addAll([
        {'state': 'downloading', 'done': 1, 'total': 10},
        {'state': 'error', 'error': 'the file does not match its checksum'},
      ]);
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      await tester.tap(find.byKey(const Key('update_now')));
      await settle(tester, 300);
      expect(find.textContaining('checksum'), findsOneWidget);
      expect(find.byKey(const Key('update_now')), findsOneWidget);
    });

    testWidgets('the window cannot be dismissed with Back while an update runs', (tester) async {
      backend.statuses.add({'state': 'downloading', 'done': 1, 'total': 10});
      await openDialog(tester, AppUpdateInfo.fromJson(_appCheck()));
      await tester.tap(find.byKey(const Key('update_now')));
      await settle(tester, 100);
      await tester.binding.handlePopRoute();
      await settle(tester, 100);
      expect(find.byKey(const Key('update_progress')), findsOneWidget);
    });
  });

  group('engine components', () {
    final comps = [
      const ComponentUpdate(name: 'llama', folder: 'cpu', installed: 'b9441', latest: 'b11456', available: true, sizeBytes: 90 * 1024 * 1024),
      const ComponentUpdate(name: 'cliproxy', folder: 'cliproxy', installed: 'v8.0.25', latest: 'v8.0.26', available: true),
    ];

    testWidgets('lists each component with its versions and applies one at a time', (tester) async {
      // Stays "installing" for the whole test: while it runs, nothing else may start.
      backend.statuses.add({'state': 'installing', 'kind': 'component', 'component': 'llama'});
      await pumpHost(tester, opener: (ctx) => TextButton(
            key: const Key('open'),
            onPressed: () => showDialog<void>(context: ctx, builder: (_) => ComponentUpdateDialog(components: comps)),
            child: const Text('open'),
          ));
      await tester.tap(find.byKey(const Key('open')));
      await tester.pumpAndSettle();

      expect(find.text(L10n.t('update_components_title')), findsOneWidget);
      expect(find.text('b9441 → b11456  ·  90.0 MB'), findsOneWidget);
      expect(find.text('v8.0.25 → v8.0.26'), findsOneWidget);

      await tester.tap(find.byKey(const Key('update_component_llama_apply')));
      await settle(tester, 30);
      final req = backend.requests.firstWhere((r) => r.path == '/api/update/apply');
      expect((req.data as Map)['kind'], 'component');
      expect((req.data as Map)['component'], 'llama');
      // While one runs, the other's button is disabled.
      await settle(tester, 20);
      final other = tester.widget<TextButton>(find.byKey(const Key('update_component_cliproxy_apply')));
      expect(other.onPressed, isNull);
    });
  });

  group('the startup prompt', () {
    Future<void> pumpPrompt(WidgetTester tester, Map<String, dynamic> check) async {
      backend.check = check;
      await pumpHost(tester, opener: (_) => const UpdatePrompt());
      await settle(tester, 200);
      await tester.pumpAndSettle();
    }

    testWidgets('opens by itself when an update exists', (tester) async {
      await pumpPrompt(tester, _appCheck());
      expect(find.byKey(const Key('update_versions')), findsOneWidget);
    });

    testWidgets('stays quiet when there is nothing new', (tester) async {
      await pumpPrompt(tester, {'current': 'V4.6.0', 'latest': 'V4.6.0', 'available': false, 'mode': 'auto', 'components': <Object>[]});
      expect(find.byType(Dialog), findsNothing);
    });

    testWidgets('does not interrupt for a version the person skipped', (tester) async {
      await prefs.setString(skippedUpdateKey, 'V4.7.0');
      await pumpPrompt(tester, _appCheck());
      expect(find.byType(Dialog), findsNothing);
    });

    testWidgets('a required update interrupts even if that version was skipped', (tester) async {
      await prefs.setString(skippedUpdateKey, 'V4.7.0');
      await pumpPrompt(tester, _appCheck(required: true));
      expect(find.byKey(const Key('update_versions')), findsOneWidget);
    });

    testWidgets('offers engine updates when there is no app update', (tester) async {
      await pumpPrompt(tester, {
        'current': 'V4.6.0',
        'latest': 'V4.6.0',
        'available': false,
        'mode': 'auto',
        'components': [
          {'name': 'llama', 'folder': 'cpu', 'installed': 'b9441', 'latest': 'b11456', 'available': true},
        ],
      });
      expect(find.text(L10n.t('update_components_title')), findsOneWidget);
    });

    testWidgets('stays quiet when the engine updates are not applicable', (tester) async {
      await pumpPrompt(tester, {
        'current': 'V4.6.0',
        'latest': 'V4.6.0',
        'available': false,
        'mode': 'auto',
        'components': [
          {'name': 'llama', 'reason': 'needs_newer_app', 'available': false},
          {'name': 'whisper', 'reason': 'unknown_installed', 'available': false},
        ],
      });
      expect(find.byType(Dialog), findsNothing);
    });

    testWidgets('Later closes it and it does not come back for the same version', (tester) async {
      await pumpPrompt(tester, _appCheck());
      await tester.tap(find.byKey(const Key('update_later')));
      await tester.pumpAndSettle();
      expect(find.byType(Dialog), findsNothing);
      // The same answer arriving again (the six-hourly re-check) must not re-open it.
      lastContainer!.invalidate(updateInfoProvider);
      await settle(tester, 200);
      await tester.pumpAndSettle();
      expect(find.byType(Dialog), findsNothing);
    });
  });

  group('updateInfoProvider is ambient', () {
    test('a failing backend yields "nothing to offer", never an error', () async {
      final b = _Backend();
      final c = MemoApiClient(baseUrl: 'http://memo.test');
      c.dio.httpClientAdapter = _Failing();
      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(c),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
      ]);
      addTearDown(container.dispose);
      await container.read(authGateProvider.future);
      final v = await container.read(updateInfoProvider.future);
      expect(v, isNull);
      expect(b.requests, isEmpty);
    });

    test('while the auth gate is up it does not even ask (a 401 would be cached all session)', () async {
      final b = _Backend()..check = _appCheck();
      final c = MemoApiClient(baseUrl: 'http://memo.test');
      c.dio.httpClientAdapter = b;
      final container = ProviderContainer(overrides: [
        apiClientProvider.overrideWithValue(c),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.loginNeeded))),
      ]);
      addTearDown(container.dispose);
      await container.read(authGateProvider.future);
      expect(await container.read(updateInfoProvider.future), isNull);
      expect(b.requests.where((r) => r.path == '/api/update/check'), isEmpty);
    });
  });

  group('shouldPromptApp', () {
    test('decision table', () {
      final base = AppUpdateInfo.fromJson(_appCheck());
      final req = AppUpdateInfo.fromJson(_appCheck(required: true));
      expect(shouldPromptApp(null, ''), isFalse);
      expect(shouldPromptApp(const AppUpdateInfo(), ''), isFalse);
      expect(shouldPromptApp(base, ''), isTrue);
      expect(shouldPromptApp(base, 'V4.7.0'), isFalse);
      expect(shouldPromptApp(base, 'V4.6.9'), isTrue); // an older skip does not cover a newer release
      expect(shouldPromptApp(req, 'V4.7.0'), isTrue);
    });
  });
}

// English text of a key, bypassing the current locale, to prove a Turkish
// string is not just the English one.
String _enOf(String key) {
  final previous = L10n.locale;
  L10n.setLocale(MemoLocale.en);
  final v = L10n.t(key);
  L10n.setLocale(previous);
  return v;
}

class _Failing implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions o, Stream<Uint8List>? s, Future<void>? c) async {
    throw DioException(requestOptions: o, type: DioExceptionType.connectionError);
  }

  @override
  void close({bool force = false}) {}
}
