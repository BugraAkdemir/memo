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
import 'package:memo_flutter/screens/agent_screen.dart';
import 'package:memo_flutter/screens/chat_screen.dart' show QuickEffortSelector;

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

  Future<void> pump(
    WidgetTester tester, {
    required double width,
    List<String> effortLevels = const ['low', 'medium', 'high'],
    bool beta = false,
  }) async {
    tester.view.physicalSize = Size(width, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    L10n.setLocale(MemoLocale.en);
    final client = MemoApiClient(baseUrl: 'http://memo.test');
    client.dio.httpClientAdapter = _DownAdapter();
    await tester.pumpWidget(ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        apiClientProvider.overrideWithValue(client),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
        agentEnabledProvider.overrideWith((ref) => _AgentOn(ref)),
        betaEnabledProvider.overrideWithValue(beta),
        chatListProvider.overrideWith(() => _Chats('/home/user/projects/memo')),
        activeChatIdProvider.overrideWith(_ActiveChat.new),
        providerListProvider.overrideWith(_Providers.new),
        activeProviderTypeProvider.overrideWith(_ActiveProvider.new),
        effortLevelsProvider.overrideWith((ref, key) async => effortLevels),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: Column(children: [
            AgentTopBar(
              activeChat: ChatSession(
                id: 'c1',
                title: 'Agent Chat',
                createdAt: '',
                updatedAt: '',
                msgCount: 1,
                projectPath: '/home/user/projects/memo',
              ),
              leadingGap: width < 700,
            ),
          ]),
        ),
      ),
    ));
    await tester.pumpAndSettle();
  }

  // Reported: the plain chat's top bar has a model switch, the agent chat's did
  // not, so changing the model meant leaving the agent screen.
  testWidgets('the agent top bar has the model switch (and the effort level on a wide window)', (tester) async {
    await pump(tester, width: 1100);
    expect(tester.takeException(), isNull);
    expect(find.byTooltip(L10n.t('switch_model')), findsOneWidget);
    expect(find.byType(QuickEffortSelector), findsOneWidget);
    expect(find.byTooltip(L10n.t('code_mode_off_tooltip')), findsOneWidget, reason: 'the Code Mode toggle is still there');
  });

  for (final width in [900.0, 700.0, 560.0, 520.0, 390.0, 360.0]) {
    testWidgets('at ${width.toInt()} px the model switch fits, does not overlap, and stays on screen', (tester) async {
      await pump(tester, width: width);
      expect(tester.takeException(), isNull);
      final model = tester.getRect(find.byTooltip(L10n.t('switch_model')));
      final code = tester.getRect(find.byTooltip(L10n.t('code_mode_off_tooltip')));
      expect(model.right, lessThanOrEqualTo(code.left + 0.5), reason: 'the model button must end before the Code Mode toggle');
      expect(code.right, lessThanOrEqualTo(width));
      expect(model.left, greaterThanOrEqualTo(0));
    });
  }

  testWidgets('a narrow window shows the model button alone (the effort selector would not fit)', (tester) async {
    await pump(tester, width: 520);
    expect(find.byTooltip(L10n.t('switch_model')), findsOneWidget);
    expect(find.byType(QuickEffortSelector), findsNothing);
  });

  // Reported: the plain chat has the browser panel button and the agent chat did not.
  group('browser panel button (Beta)', () {
    testWidgets('is hidden while Beta is off', (tester) async {
      await pump(tester, width: 1100);
      expect(find.byTooltip(L10n.t('browser_pane_open')), findsNothing);
    });

    testWidgets('is shown with Beta on and toggles the shared browser session flag', (tester) async {
      await pump(tester, width: 1100, beta: true);
      final container = ProviderScope.containerOf(tester.element(find.byType(AgentTopBar)));
      expect(container.read(browserSessionActiveProvider), isFalse);

      await tester.tap(find.byTooltip(L10n.t('browser_pane_open')));
      await tester.pumpAndSettle();
      expect(container.read(browserSessionActiveProvider), isTrue);
      expect(find.byTooltip(L10n.t('browser_pane_close')), findsOneWidget, reason: 'the button now offers to close it');
    });

    for (final width in [1100.0, 700.0, 560.0, 520.0, 480.0]) {
      testWidgets('with the button added the bar still fits at ${width.toInt()} px', (tester) async {
        await pump(tester, width: width, beta: true);
        expect(tester.takeException(), isNull);
        final globe = tester.getRect(find.byTooltip(L10n.t('browser_pane_open')));
        final model = tester.getRect(find.byTooltip(L10n.t('switch_model')));
        expect(globe.right, lessThanOrEqualTo(width));
        expect(model.right, lessThanOrEqualTo(globe.left + 0.5));
      });
    }

    for (final width in [430.0, 390.0, 360.0]) {
      testWidgets('on a phone (${width.toInt()} px) there is no room for it, and nothing overflows', (tester) async {
        await pump(tester, width: width, beta: true);
        expect(tester.takeException(), isNull);
        expect(find.byTooltip(L10n.t('browser_pane_open')), findsNothing);
      });
    }
  });
}
