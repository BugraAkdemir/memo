import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/notification_service.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/learning_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/screens/app_shell.dart';
import 'package:memo_flutter/screens/calendar_screen.dart';
import 'package:memo_flutter/screens/chat_screen.dart';
import 'package:memo_flutter/screens/developer_screen.dart';
import 'package:memo_flutter/screens/mail_screen.dart';
import 'package:memo_flutter/screens/model_store_screen.dart';
import 'package:memo_flutter/screens/routines_screen.dart';
import 'package:memo_flutter/screens/swarm_screen.dart';

class _ShellFakeAdapter implements HttpClientAdapter {
  final List<String> requestedPaths = [];
  List<Map<String, dynamic>> calendarEvents = [];
  final List<StreamController<Uint8List>> calendarStreams = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requestedPaths.add(options.path);

    if (options.path == '/api/calendar/changes') {
      final c = StreamController<Uint8List>();
      calendarStreams.add(c);
      cancelFuture?.then((_) {
        if (!c.isClosed) c.close();
      });
      return ResponseBody(
        c.stream,
        200,
        headers: {
          Headers.contentTypeHeader: ['text/event-stream'],
        },
      );
    }

    if (options.path == '/api/tasks/events' ||
        options.path == '/api/agent/events' ||
        options.path == '/api/proactive/stream') {
      final c = StreamController<Uint8List>();
      cancelFuture?.then((_) {
        if (!c.isClosed) c.close();
      });
      return ResponseBody(
        c.stream,
        200,
        headers: {
          Headers.contentTypeHeader: ['text/event-stream'],
        },
      );
    }

    Object data = <String, dynamic>{};
    if (options.path == '/api/calendar/events') {
      data = calendarEvents;
    } else if (options.path == '/api/calendar/settings') {
      data = {'reminder_lead_minutes': 30};
    } else if (options.path == '/api/status') {
      data = {'status': 'ok', 'llama_running': false, 'model': ''};
    } else if (options.path == '/api/chats' ||
        options.path == '/api/models' ||
        options.path == '/api/providers' ||
        options.path == '/api/skills' ||
        options.path == '/api/tasklists' ||
        options.path == '/api/tasks/running' ||
        options.path == '/api/mail/accounts' ||
        options.path == '/api/routines' ||
        options.path == '/api/learning/patterns' ||
        options.path == '/api/memory/files') {
      data = <dynamic>[];
    } else if (options.path == '/api/auth/status') {
      data = {'required': false, 'mode': 'none'};
    } else if (options.path == '/api/onboarding/status') {
      data = {'complete': true};
    } else if (options.path == '/api/remote-access') {
      data = {'enabled': false, 'beta': false};
    } else if (options.path == '/api/version') {
      data = {'current': '4.8.0'};
    }

    return ResponseBody.fromString(
      jsonEncode(data),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  void emitCalendarChange() {
    calendarStreams.last.add(
      Uint8List.fromList(utf8.encode('data: {"changed":true}\n\n')),
    );
  }

  @override
  void close({bool force = false}) {
    for (final c in calendarStreams) {
      if (!c.isClosed) c.close();
    }
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<(_ShellFakeAdapter, MemoApiClient, ProviderContainer)> createHarness() async {
    SharedPreferences.setMockInitialValues({
      'memo_setup_complete': true,
      'memo_launchpad_seen': true,
      'memo_tour_seen': true,
      'memo_backend_url': 'http://memo.test',
    });
    final prefs = await SharedPreferences.getInstance();
    final adapter = _ShellFakeAdapter();
    final api = MemoApiClient(baseUrl: 'http://memo.test');
    api.dio.httpClientAdapter = adapter;
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(api),
        prefsProvider.overrideWithValue(prefs),
        authGateProvider.overrideWith(
          (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
        ),
        setupCompleteProvider.overrideWith(
          (ref) => SetupCompleteNotifier(ref, prefs, confirmWithBackend: false),
        ),
        onboardingCheckedProvider.overrideWith((ref) => true),
        launchpadSeenProvider.overrideWith((ref) => LaunchpadSeenNotifier(prefs)),
        tourSeenProvider.overrideWith((ref) => TourSeenNotifier(prefs)),
        connectionStatusProvider.overrideWith((ref) => Stream.value(true)),
        llamaInstalledProvider.overrideWith((ref) async => true),
        pendingProactiveSuggestionProvider.overrideWith(
          (ref) => const Stream.empty(),
        ),
      ],
    );
    return (adapter, api, container);
  }

  Future<void> disposeHarness(
    WidgetTester tester,
    MemoApiClient api,
    ProviderContainer container,
  ) async {
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(milliseconds: 500));
    container.dispose();
    api.dio.close(force: true);
  }

  testWidgets(
    'AppShell only mounts active tab initially, caches visited tabs, and keeps global error toast active',
    (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final (_, api, container) = await createHarness();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: AppShell()),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));

      // Initial startup builds ChatScreen only; deferred tabs are not mounted yet.
      expect(find.byType(ChatScreen, skipOffstage: false), findsOneWidget);
      expect(find.byType(ModelStoreScreen, skipOffstage: false), findsNothing);
      expect(find.byType(CalendarScreen, skipOffstage: false), findsNothing);
      expect(find.byType(RoutinesScreen, skipOffstage: false), findsNothing);
      expect(find.byType(DeveloperScreen, skipOffstage: false), findsNothing);
      expect(find.byType(SwarmScreen, skipOffstage: false), findsNothing);
      expect(find.byType(MailScreen, skipOffstage: false), findsNothing);

      // Global error toast listener on AppShell remains live from frame 1.
      container.read(errorMessageProvider.notifier).state = 'global_boom';
      await tester.pump();
      expect(find.text('global_boom'), findsOneWidget);

      // Visit tab 2 (Model Store nav button) -> builds lazily and caches in IndexedStack.
      await tester.tap(find.byIcon(Icons.memory_outlined));
      await tester.pump();
      expect(find.byType(ModelStoreScreen, skipOffstage: false), findsOneWidget);
      final modelElement = tester.element(
        find.byType(ModelStoreScreen, skipOffstage: false),
      );

      // Switch back to Chat and then back to Models -> same widget element stays mounted.
      await tester.tap(find.byIcon(Icons.chat_bubble_outline));
      await tester.pump();
      expect(find.byType(ModelStoreScreen, skipOffstage: false), findsOneWidget);
      await tester.tap(find.byIcon(Icons.memory_outlined));
      await tester.pump();
      expect(
        tester.element(find.byType(ModelStoreScreen, skipOffstage: false)),
        same(modelElement),
      );

      await disposeHarness(tester, api, container);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.linux),
  );

  testWidgets(
    'AppShell reconciles mobile calendar OS reminders from launch even before visiting CalendarScreen',
    (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      NotificationService.resetForTest();
      addTearDown(NotificationService.resetForTest);
      final pending = <int, Map<String, dynamic>>{};
      const channel = MethodChannel(
        'dexterous.com/flutter/local_notifications',
      );
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
        call,
      ) async {
        if (call.method == 'initialize') return true;
        if (call.method == 'pendingNotificationRequests') {
          return pending.values.toList();
        }
        if (call.method == 'zonedSchedule') {
          final a = Map<String, dynamic>.from(call.arguments as Map);
          pending[a['id'] as int] = {
            'id': a['id'],
            'title': a['title'],
            'body': a['body'],
            'payload': a['payload'],
          };
        } else if (call.method == 'cancel') {
          pending.remove((call.arguments as Map)['id']);
        }
        return null;
      });
      addTearDown(
        () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
          channel,
          null,
        ),
      );
      await NotificationService.init();

      final (adapter, api, container) = await createHarness();
      final start = DateTime.now()
          .toUtc()
          .add(const Duration(days: 60))
          .toIso8601String();
      adapter.calendarEvents = [
        {'id': 'ev-1', 'title': 'Mobile Launch Reminder', 'start_time': start},
      ];

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: AppShell()),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));

      expect(find.byType(CalendarScreen, skipOffstage: false), findsNothing);
      expect(adapter.calendarStreams, isNotEmpty);
      expect(pending.values.single['title'], 'Mobile Launch Reminder');

      adapter.calendarEvents = [
        {'id': 'ev-1', 'title': 'Updated Launch Reminder', 'start_time': start},
      ];
      adapter.emitCalendarChange();
      await tester.pump(const Duration(milliseconds: 200));
      expect(pending.values.single['title'], 'Updated Launch Reminder');

      await disposeHarness(tester, api, container);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.android),
  );
}
