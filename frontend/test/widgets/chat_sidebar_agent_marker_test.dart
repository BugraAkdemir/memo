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
import 'package:memo_flutter/widgets/chat_sidebar.dart';

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

class _AgentOff extends AgentEnabledNotifier {
  _AgentOff(super.ref) {
    state = false;
  }
}

class _Chats extends ChatListNotifier {
  _Chats(this.path);
  final String? path;
  @override
  Future<List<ChatSession>> build() async => const [
        ChatSession(id: 'c1', title: 'Agent konuşması', createdAt: '', updatedAt: '', msgCount: 2, projectPath: '/home/u/memo'),
        ChatSession(id: 'c2', title: 'Düz sohbet', createdAt: '', updatedAt: '', msgCount: 2),
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

  testWidgets('one list holds both kinds: only the agent chat is marked', (tester) async {
    tester.view.physicalSize = const Size(900, 800);
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
        chatListProvider.overrideWith(() => _Chats(null)),
        activeChatIdProvider.overrideWith(_ActiveChat.new),
      ],
      child: MaterialApp(home: Scaffold(body: Row(children: [SizedBox(width: 320, child: ChatSidebar())]))),
    ));
    await tester.pumpAndSettle();

    expect(find.text('Agent konuşması'), findsOneWidget);
    expect(find.text('Düz sohbet'), findsOneWidget);
    expect(find.byTooltip(L10n.t('agent_chat_marker')), findsOneWidget, reason: 'exactly one of the two is an agent chat');
    // The marker sits on the agent chat's row, before its title.
    final marker = tester.getRect(find.byTooltip(L10n.t('agent_chat_marker')));
    final title = tester.getRect(find.text('Agent konuşması'));
    expect(marker.right, lessThanOrEqualTo(title.left + 0.5));
    expect((marker.center.dy - title.center.dy).abs(), lessThan(8));
  });
}
