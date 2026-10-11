import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/providers/agent_provider.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/screens/chat_screen.dart';

/// Every request fails fast, so the ambient providers the top bar watches
/// degrade silently (they are written to) instead of reaching a network.
class _DownAdapter implements HttpClientAdapter {
  final puts = <(String, Object?)>[];
  @override
  Future<ResponseBody> fetch(RequestOptions o, Stream<Uint8List>? r, Future<void>? c) async {
    if (o.method == 'PUT' || o.method == 'POST') puts.add((o.path, o.data));
    return ResponseBody.fromString('{"error":"down"}', 503,
        headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});
  }

  @override
  void close({bool force = false}) {}
}

class _AgentOn extends AgentEnabledNotifier {
  _AgentOn(super.ref) {
    state = true;
  }
}

class _Chats extends ChatListNotifier {
  _Chats(this.path);
  final String? path;
  @override
  Future<List<ChatSession>> build() async => [
        ChatSession(
          id: 'c1',
          title: 'selam senden bir web sitesi yapmanı istiyorum',
          createdAt: '',
          updatedAt: '',
          msgCount: 1,
          projectPath: path,
        ),
      ];
}

class _ActiveChat extends ActiveChatIdNotifier {
  @override
  Future<String> build() async => 'c1';
}

class _Providers extends ProviderListNotifier {
  @override
  Future<List<ProviderConfig>> build() async => const [
        ProviderConfig(
          name: 'MyRouter',
          type: 'openrouter',
          model: 'some/model',
          effortLevel: 'high',
        ),
      ];
}

class _ActiveProvider extends ActiveProviderNotifier {
  @override
  Future<String> build() async => 'MyRouter';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  late _DownAdapter adapter;

  Future<void> pump(
    WidgetTester tester, {
    required double width,
    String? projectPath = '/home/user/projects/proje',
    List<String> effortLevels = const [],
    double height = 800,
  }) async {
    tester.view.physicalSize = Size(width, height);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    L10n.setLocale(MemoLocale.en);
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    adapter = _DownAdapter();
    client.dio.httpClientAdapter = adapter;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        apiClientProvider.overrideWithValue(client),
        authGateProvider.overrideWith(
          (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
        ),
        agentEnabledProvider.overrideWith((ref) => _AgentOn(ref)),
        chatListProvider.overrideWith(() => _Chats(projectPath)),
        activeChatIdProvider.overrideWith(_ActiveChat.new),
        providerListProvider.overrideWith(_Providers.new),
        activeProviderTypeProvider.overrideWith(_ActiveProvider.new),
        effortLevelsProvider.overrideWith((ref, key) async => effortLevels),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: Column(children: [
            ChatTopBar(showMenuButton: width < 700),
          ]),
        ),
      ),
    ));
    await tester.pumpAndSettle();
  }

  // The reported screenshot: an agent chat with a project folder, on a phone.
  // The "Project: proje" chip and the model button were drawn on top of each
  // other and the title had no room at all.
  for (final width in [360.0, 390.0, 430.0]) {
    testWidgets('phone top bar at ${width.toInt()} px: project chip + model button do not overflow or overlap',
        (tester) async {
      await pump(tester, width: width);
      expect(tester.takeException(), isNull);

      // Icon-only on a phone, with the full name still available as a tooltip.
      final label = '${L10n.t('agent_chat_project')}proje';
      expect(find.text(label), findsNothing);
      expect(find.byTooltip(label), findsOneWidget);

      final chip = tester.getRect(find.byIcon(Icons.folder_outlined));
      final menu = tester.getRect(find.byIcon(Icons.more_vert));
      expect(chip.right, lessThan(menu.left));
      // Everything stays on screen.
      expect(menu.right, lessThanOrEqualTo(width));
      // The model button is not covered by the chip.
      final dropdown = tester.getRect(find.byTooltip(L10n.t('switch_model')));
      expect(chip.right, lessThanOrEqualTo(dropdown.left + 0.5),
          reason: 'the chip must end before the model button starts');
    });
  }

  testWidgets('a very long project folder name still fits on a phone', (tester) async {
    await pump(tester,
        width: 360,
        projectPath: '/srv/work/${'a-very-long-project-folder-name-' * 4}');
    expect(tester.takeException(), isNull);
  });

  testWidgets('the wide (desktop) layout keeps the labelled project chip', (tester) async {
    await pump(tester, width: 1100);
    expect(tester.takeException(), isNull);
    expect(find.text('${L10n.t('agent_chat_project')}proje'), findsOneWidget);
  });

  group('reasoning effort on a phone', () {
    Future<void> openActions(WidgetTester tester) async {
      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();
    }

    testWidgets('a labelled row in the actions sheet shows the level in force and opens the choices',
        (tester) async {
      await pump(tester, width: 390, effortLevels: const ['low', 'medium', 'high']);
      await openActions(tester);

      final row = find.byKey(const Key('effort_sheet_row'));
      expect(row, findsOneWidget);
      expect(find.descendant(of: row, matching: find.text(L10n.t('effort_level_label'))),
          findsOneWidget);
      expect(find.descendant(of: row, matching: find.text('high')), findsOneWidget,
          reason: 'the current level is shown, not just a bare control');

      await tester.tap(row);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      for (final level in ['low', 'medium', 'high']) {
        expect(find.text(level), findsOneWidget, reason: level);
      }
      expect(find.text(L10n.t('effort_level_default')), findsOneWidget);
    });

    testWidgets('choosing a level saves it on the provider', (tester) async {
      await pump(tester, width: 390, effortLevels: const ['low', 'medium', 'high']);
      await openActions(tester);
      await tester.tap(find.byKey(const Key('effort_sheet_row')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('low'));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      final saved = adapter.puts.where((p) => p.$1.contains('providers'));
      expect(saved, isNotEmpty, reason: 'the choice must reach the backend');
      expect(saved.first.$2.toString(), contains('low'));
    });

    testWidgets('no row when the provider has no effort levels (nothing to choose)',
        (tester) async {
      await pump(tester, width: 390, effortLevels: const []);
      await openActions(tester);
      expect(find.byKey(const Key('effort_sheet_row')), findsNothing);
    });
  });
}
