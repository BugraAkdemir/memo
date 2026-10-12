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
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/screens/calendar_screen.dart';

class _CalendarAdapter implements HttpClientAdapter {
  List<Map<String, dynamic>> events = [];
  final streams = <StreamController<Uint8List>>[];
  @override
  Future<ResponseBody> fetch(
    RequestOptions o,
    Stream<Uint8List>? body,
    Future<void>? cancel,
  ) async {
    if (o.path == '/api/calendar/changes') {
      final c = StreamController<Uint8List>();
      streams.add(c);
      cancel?.then((_) {
        if (!c.isClosed) c.close();
      });
      c.add(Uint8List.fromList(utf8.encode('data: {"changed":true}\n\n')));
      return ResponseBody(
        c.stream,
        200,
        headers: {
          Headers.contentTypeHeader: ['text/event-stream'],
        },
      );
    }
    Object data;
    var status = 200;
    if (o.path == '/api/calendar/events') {
      data = events;
    } else if (o.path == '/api/calendar/settings') {
      data = {'reminder_lead_minutes': 30};
    } else {
      status = 401;
      data = {'error': 'unauthorized'};
    }
    return ResponseBody.fromString(
      jsonEncode(data),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  void signal() => streams.last.add(
    Uint8List.fromList(utf8.encode('data: {"changed":true}\n\n')),
  );
  @override
  void close({bool force = false}) {
    for (final c in streams) {
      if (!c.isClosed) c.close();
    }
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  testWidgets(
    'hidden calendar reconciles backend changes and suspends stream while paused',
    (tester) async {
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
        if (call.method == 'pendingNotificationRequests')
          return pending.values.toList();
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
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final adapter = _CalendarAdapter();
      final start = DateTime.now()
          .toUtc()
          .add(const Duration(days: 120))
          .toIso8601String();
      adapter.events = [
        {'id': 'one', 'title': 'first title', 'start_time': start},
      ];
      final api = MemoApiClient(baseUrl: 'http://memo.test');
      api.dio.httpClientAdapter = adapter;
      final container = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(api),
          prefsProvider.overrideWithValue(prefs),
          authGateProvider.overrideWith(
            (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
          ),
        ],
      );
      addTearDown(container.dispose);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: CalendarScreen()),
        ),
      );
      await tester.pumpAndSettle();
      expect(adapter.streams, isNotEmpty);
      expect(pending.values.single['title'], 'first title');
      adapter.events = [
        {'id': 'one', 'title': 'renamed elsewhere', 'start_time': start},
      ];
      adapter.signal();
      await tester.pumpAndSettle();
      expect(pending.values.single['title'], 'renamed elsewhere');
      adapter.events = [];
      adapter.signal();
      await tester.pumpAndSettle();
      expect(pending, isEmpty);
      final opened = adapter.streams.length;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      await tester.pumpAndSettle();
      adapter.events = [
        {'id': 'two', 'title': 'added while paused', 'start_time': start},
      ];
      await tester.pump(const Duration(seconds: 20));
      expect(adapter.streams.length, opened);
      expect(pending, isEmpty);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();
      expect(adapter.streams.length, greaterThan(opened));
      expect(pending.values.single['title'], 'added while paused');
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
      api.dio.close();
    },
    variant: TargetPlatformVariant.only(TargetPlatform.android),
  );
}
