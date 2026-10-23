import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/models/server_browse_entry.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/new_chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/screens/chat_screen.dart';
import 'package:memo_flutter/providers/skill_provider.dart';
import 'package:memo_flutter/widgets/slash_menu.dart';

import '../support/fake_skills.dart';

class _DownAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions o, Stream<Uint8List>? r, Future<void>? c) async =>
      ResponseBody.fromString('{"error":"down"}', 503, headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});
  @override
  void close({bool force = false}) {}
}

/// An in-memory backend for the chat list: setChatProject really changes the chat.
class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://memo.test') {
    dio.httpClientAdapter = _DownAdapter();
  }
  String? project; // the empty chat's project folder
  final projectCalls = <String>[];
  final sent = <String>[];

  @override
  Future<List<ChatSession>> listChats() async => [
        ChatSession(id: 'c1', title: project == null ? 'New Chat' : 'Agent Chat', createdAt: '', updatedAt: '', msgCount: 0, projectPath: project),
      ];

  @override
  Future<void> setChatProject(String chatId, String projectPath) async {
    projectCalls.add(projectPath);
    project = projectPath.isEmpty ? null : projectPath;
  }

  @override
  Future<ServerBrowseResult> browseServer(String path) async =>
      const ServerBrowseResult(path: '/home/u/proje', parent: '/home/u', entries: [ServerBrowseEntry(name: 'src', isDir: true)]);
}

class _ActiveChat extends ActiveChatIdNotifier {
  @override
  Future<String> build() async => 'c1';
}

class _Messages extends MessagesNotifier {
  @override
  Future<List<ChatMessage>> build() async => const [];
  void put(List<ChatMessage> m) => state = AsyncData(m);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  late _Api api;
  late ProviderContainer container;

  Future<void> pump(WidgetTester tester, {double width = 1200, double height = 900}) async {
    tester.view.physicalSize = Size(width, height);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    L10n.setLocale(MemoLocale.en);
    api = _Api();
    await tester.pumpWidget(ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        apiClientProvider.overrideWithValue(api),
        authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
        activeChatIdProvider.overrideWith(_ActiveChat.new),
        messagesProvider.overrideWith(_Messages.new),
        skillListProvider.overrideWith(FakeSkillList.new),
      ],
      child: const MaterialApp(home: Scaffold(body: ChatScreen())),
    ));
    await tester.pumpAndSettle();
    container = ProviderScope.containerOf(tester.element(find.byType(ChatScreen)));
  }

  Finder field() => find.byType(TextField);
  Finder chatSeg() => find.text(L10n.t('mode_chat_label'));
  Finder agentSeg() => find.text(L10n.t('mode_agent_label'));

  testWidgets('slash menu on the landing page follows arrows and keeps the composer focused', (tester) async {
    await pump(tester, height: 748);
    await tester.tap(field());
    final before = tester.state<EditableTextState>(find.byType(EditableText));
    await tester.enterText(field(), '/');
    await tester.pumpAndSettle();
    final rows = buildSlashItems(skills: fakeSkills());
    for (var i = 0; i < 7; i++) {
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await tester.pumpAndSettle();
    }
    expect(find.text(rows[7].key).hitTestable(), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(field()).controller!.text, '${rows[7].key} ');
    expect(identical(tester.state<EditableTextState>(find.byType(EditableText)), before), isTrue);
    expect(tester.widget<EditableText>(find.byType(EditableText)).focusNode.hasFocus, isTrue);
  });

  testWidgets('mouse wheel reaches the bottom of the slash menu on the landing page', (tester) async {
    await pump(tester, height: 748);
    await tester.tap(field());
    await tester.enterText(field(), '/');
    await tester.pumpAndSettle();
    final popup = find.byType(SlashMenuPopup);
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: tester.getCenter(popup));
    await tester.sendEventToBinding(PointerScrollEvent(
      position: tester.getCenter(popup),
      scrollDelta: const Offset(0, 2000),
    ));
    await tester.pumpAndSettle();
    expect(find.text('/orchestra').hitTestable(), findsOneWidget);
    expect(tester.takeException(), isNull);
    await mouse.removePointer();
  });

  testWidgets('an empty new chat: switch on top, composer in the MIDDLE, suggestions below it', (tester) async {
    await pump(tester);
    expect(chatSeg(), findsOneWidget);
    expect(agentSeg(), findsOneWidget);

    final screen = tester.view.physicalSize;
    final box = tester.getRect(field());
    expect(box.center.dy, greaterThan(screen.height * 0.25));
    expect(box.center.dy, lessThan(screen.height * 0.65), reason: 'in the middle, not docked at the bottom');
    final card = tester.getRect(find.text(L10n.t('suggest_review_label')));
    expect(card.top, greaterThan(box.bottom), reason: 'suggestions sit under the composer');
    expect(tester.getRect(chatSeg()).bottom, lessThan(box.top), reason: 'the switch is above it');
  });

  testWidgets('the composer carries the model picker inside it on this screen', (tester) async {
    await pump(tester);
    final box = tester.getRect(find.ancestor(of: field(), matching: find.byType(Container)).last);
    final picker = tester.getRect(find.byTooltip(L10n.t('switch_model')).last);
    expect(box.contains(picker.center), isTrue, reason: 'the model button is part of the composer card');
  });

  // Reported: "the underlined row looks crooked" - the send button did not reach the
  // right edge and the model button floated in the middle.
  for (final width in [1200.0, 900.0, 390.0]) {
    testWidgets('the bottom row of the composer is flush: send at the right edge, picker next to it (${width.toInt()} px)',
        (tester) async {
      await pump(tester, width: width, height: width < 500 ? 800 : 900);
      expect(tester.takeException(), isNull);
      final card = tester.getRect(find.byKey(const ValueKey('chat-input-area')));
      final send = tester.getRect(find.byIcon(Icons.send_rounded));
      final picker = tester.getRect(find.byTooltip(L10n.t('switch_model')).last);
      expect(card.right - send.right, lessThan(24), reason: 'send is at the right end of the card, not floating');
      expect(send.left - picker.right, lessThan(30), reason: 'the model button sits right next to send');
      expect((send.center.dy - picker.center.dy).abs(), lessThan(6), reason: 'same baseline');
    });
  }

  testWidgets('choosing Agent swaps the title, the suggestions and asks for a project', (tester) async {
    await pump(tester);
    await tester.tap(agentSeg());
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('landing_agent_title')), findsOneWidget);
    expect(find.text(L10n.t('landing_choose_project')), findsOneWidget);
    expect(find.text(L10n.t('suggest_agent_explore_label')), findsOneWidget);
    expect(find.text(L10n.t('suggest_review_label')), findsNothing);
    expect(container.read(pendingAgentChatsProvider), contains('c1'));

    await tester.tap(chatSeg());
    await tester.pumpAndSettle();
    expect(find.text(L10n.t('landing_agent_title')), findsNothing);
    expect(container.read(pendingAgentChatsProvider), isEmpty);
  });

  testWidgets('picking a project folder makes it an agent chat on the backend', (tester) async {
    await pump(tester);
    await tester.tap(agentSeg());
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('landing_choose_project')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('server_browse_select_folder')));
    await tester.pumpAndSettle();

    expect(api.projectCalls, ['/home/u/proje']);
    expect(find.text('proje'), findsWidgets, reason: 'the chosen folder is shown');
    expect(find.text(L10n.t('landing_change_project')), findsOneWidget);
    expect(find.byTooltip(L10n.t('agent_chat_marker')), findsWidgets, reason: 'the top bar now marks it as an agent chat');
  });

  testWidgets('going back to Chat after choosing a folder turns it into a plain chat again', (tester) async {
    await pump(tester);
    await tester.tap(agentSeg());
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('landing_choose_project')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('server_browse_select_folder')));
    await tester.pumpAndSettle();

    await tester.tap(chatSeg());
    await tester.pumpAndSettle();
    expect(api.projectCalls, ['/home/u/proje', '']);
    expect(api.project, isNull);
    expect(find.text(L10n.t('landing_choose_project')), findsNothing);
  });

  testWidgets('sending in Agent mode without a folder asks for the folder instead of sending', (tester) async {
    await pump(tester);
    await tester.tap(agentSeg());
    await tester.pumpAndSettle();
    await tester.enterText(field(), 'bu projeyi incele');
    await tester.pump();
    await tester.tap(find.byIcon(Icons.send_rounded));
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('server_browse_title')), findsOneWidget, reason: 'the folder dialog opened');
    expect(container.read(isSendingProvider), isFalse, reason: 'nothing was sent');
    // Cancelling keeps the text and sends nothing.
    await tester.tap(find.text(L10n.t('cancel')));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(field()).controller!.text, 'bu projeyi incele');
    expect(container.read(isSendingProvider), isFalse);
  });

  testWidgets('after the first message the switch is gone and the SAME composer is at the bottom', (tester) async {
    await pump(tester);
    await tester.tap(field());
    await tester.pump();
    await tester.enterText(field(), 'taslak');
    await tester.pump();
    final before = tester.state<EditableTextState>(find.descendant(of: field(), matching: find.byType(EditableText)));
    expect(tester.getRect(field()).center.dy, lessThan(tester.view.physicalSize.height * 0.65));

    (container.read(messagesProvider.notifier) as _Messages)
        .put([ChatMessage(role: 'user', content: 'merhaba', timestamp: '12:00')]);
    await tester.pumpAndSettle();

    expect(chatSeg(), findsNothing, reason: 'the kind of a chat is fixed once it has a message');
    expect(agentSeg(), findsNothing);
    expect(tester.getRect(field()).center.dy, greaterThan(tester.view.physicalSize.height * 0.8), reason: 'docked at the bottom');
    final after = tester.state<EditableTextState>(find.descendant(of: field(), matching: find.byType(EditableText)));
    expect(identical(before, after), isTrue, reason: 'same composer state: the draft and focus survive the move');
    expect(tester.widget<TextField>(field()).controller!.text, 'taslak');
  });

  testWidgets('fits a phone width', (tester) async {
    await pump(tester, width: 390, height: 800);
    expect(tester.takeException(), isNull);
    await tester.tap(agentSeg());
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
