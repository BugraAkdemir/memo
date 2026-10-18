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
import 'package:memo_flutter/widgets/agent/browser_pane.dart';

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

class _NoMessages extends MessagesNotifier {
  @override
  Future<List<ChatMessage>> build() async => const [];
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> pump(WidgetTester tester, {required double width, required bool beta, required bool sessionOpen}) async {
    tester.view.physicalSize = Size(width, 900);
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
        effortLevelsProvider.overrideWith((ref, key) async => const <String>[]),
        messagesProvider.overrideWith(_NoMessages.new),
        if (sessionOpen) browserSessionActiveProvider.overrideWith((ref) => true),
      ],
      child: const MaterialApp(home: Scaffold(body: AgentScreen())),
    ));
    await tester.pumpAndSettle();
    return ProviderScope.containerOf(tester.element(find.byType(AgentScreen)));
  }

  // Reported: the plain chat shows the browser panel when the agent opens a page,
  // the agent chat did not (it never even had a place to put it).
  testWidgets('wide: an open browser session shows the panel beside the agent chat', (tester) async {
    await pump(tester, width: 1500, beta: true, sessionOpen: true);
    expect(tester.takeException(), isNull);
    expect(find.byType(BrowserPane), findsOneWidget);
    expect(find.byType(AgentTopBar), findsOneWidget, reason: 'the chat stays visible next to it');
  });

  testWidgets('no panel while the session is closed', (tester) async {
    await pump(tester, width: 1500, beta: true, sessionOpen: false);
    expect(find.byType(BrowserPane), findsNothing);
  });

  testWidgets('no panel with Beta off, even if a session flag is set', (tester) async {
    await pump(tester, width: 1500, beta: false, sessionOpen: true);
    expect(find.byType(BrowserPane), findsNothing);
  });

  testWidgets('the top-bar button opens the panel', (tester) async {
    final container = await pump(tester, width: 1500, beta: true, sessionOpen: false);
    expect(container.read(browserSessionActiveProvider), isFalse);
    await tester.tap(find.byTooltip(L10n.t('browser_pane_open')));
    await tester.pumpAndSettle();
    expect(find.byType(BrowserPane), findsOneWidget);
  });

  testWidgets('narrow: the panel takes over the screen', (tester) async {
    await pump(tester, width: 420, beta: true, sessionOpen: true);
    expect(tester.takeException(), isNull);
    expect(find.byType(BrowserPane), findsOneWidget);
    expect(find.byType(AgentTopBar), findsNothing);
  });
}
