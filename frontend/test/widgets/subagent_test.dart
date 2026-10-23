import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/subagent.dart';
import 'package:memo_flutter/models/chat.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/subagent_provider.dart';
import 'package:memo_flutter/widgets/agent/subagent_pane.dart';
import 'package:memo_flutter/widgets/agent/subagent_settings.dart';

class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://memo.test');
  int saved = 0;
  String? action, child, chat, instruction;
  @override
  Future<void> setSubagentParallelism(int value) async {
    saved = value;
  }

  @override
  Future<int> getSubagentParallelism() async => saved == 0 ? 3 : saved;
  @override
  Future<void> subagentAction(
    String id,
    String chatId,
    String operation, {
    String? prompt,
  }) async {
    child = id;
    chat = chatId;
    action = operation;
    instruction = prompt;
  }
}

class _PollingApi extends _Api {
  int calls = 0;
  Completer<List<SubagentJob>> pending = Completer();
  @override
  Future<List<SubagentJob>> listSubagents(String chatId) {
    calls++;
    return pending.future;
  }
}

class _MessagesApi extends _Api {
  Completer<List<ChatMessage>>? pendingMessages;
  int reads = 0;
  @override
  Future<String> getActiveChatId() async => 'room';
  @override
  Future<List<ChatMessage>> getMessages({String? chatId}) async {
    reads++;
    return pendingMessages == null
        ? [ChatMessage(role: 'assistant', content: 'saved', timestamp: '')]
        : pendingMessages!.future;
  }
}

class _Adapter implements HttpClientAdapter {
  dynamic payload;
  _Adapter(this.payload);
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? body,
    Future<void>? cancel,
  ) async => ResponseBody.fromString(
    jsonEncode(payload),
    200,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
  @override
  void close({bool force = false}) {}
}

void main() {
  setUp(() => L10n.setLocale(MemoLocale.en));
  const job = SubagentJob(
    id: 'one',
    chatId: 'room',
    title: 'Implement search',
    prompt: 'Add a search box',
    status: 'awaiting_followup',
    output: 'Search implemented',
    events: [
      {'tool_name': 'read_file', 'output': 'file contents'},
    ],
  );

  test(
    'malformed JSON is safe and another chat cannot leak into results',
    () async {
      final api = MemoApiClient(baseUrl: 'http://memo.test');
      final adapter = _Adapter([
        null,
        4,
        {'id': 'bad', 'chat_id': 'other'},
        {
          'id': 'ok',
          'chat_id': 'room',
          'title': 3,
          'history': [null, 4],
          'events': 'bad',
        },
        {'id': false, 'chat_id': 'room'},
      ]);
      api.dio.httpClientAdapter = adapter;
      final jobs = await api.listSubagents('room');
      expect(jobs.map((j) => j.id), ['ok']);
      expect(jobs.single.history, isEmpty);
      expect(jobs.single.events, isEmpty);
      expect(jobs.single.title, isEmpty);
      adapter.payload = {'unexpected': true};
      expect(await api.listSubagents('room'), isEmpty);
      adapter.payload = {'max_parallel': 'bad'};
      expect(await api.getSubagentParallelism(), 3);
      final messageJob = SubagentJob.fromJson({
        'id': 'x',
        'messages': [
          {'role': 'user', 'content': 'followup'},
        ],
      });
      expect(messageJob.history.single['content'], 'followup');
    },
  );

  test(
    'background main refresh respects active stream and chat ownership',
    () async {
      final api = _MessagesApi();
      final container = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(api),
          authGateProvider.overrideWith(
            (ref) => Stream.value(const AuthGateInfo(AuthGateState.ok)),
          ),
        ],
      );
      addTearDown(container.dispose);
      final gateSub = container.listen(authGateProvider, (_, _) {});
      addTearDown(gateSub.close);
      await container.read(authGateProvider.future);
      await container.read(messagesProvider.future);
      final notifier = container.read(messagesProvider.notifier);
      await notifier.refreshIfIdle('other');
      expect(api.reads, 1);
      api.pendingMessages = Completer();
      final refresh = notifier.refreshIfIdle('room');
      container.read(isSendingProvider.notifier).state = true;
      api.pendingMessages!.complete([
        ChatMessage(role: 'assistant', content: 'stale', timestamp: ''),
      ]);
      await refresh;
      expect(container.read(messagesProvider).value!.single.content, 'saved');
      container.read(isSendingProvider.notifier).state = false;
      api.pendingMessages = Completer()
        ..complete([
          ChatMessage(
            role: 'assistant',
            content: 'background finished',
            timestamp: '',
          ),
        ]);
      await notifier.refreshIfIdle('room');
      expect(
        container.read(messagesProvider).value!.single.content,
        'background finished',
      );
    },
  );

  testWidgets(
    'card opens assigned prompt and output, follow-up reuses child and chat',
    (tester) async {
      final api = _Api();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(api),
            subagentsProvider(
              'room',
            ).overrideWith((ref) => Stream.value([job])),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: Consumer(
                builder: (context, ref, _) {
                  final selected = ref.watch(selectedSubagentProvider('room'));
                  return selected == null
                      ? const SubagentTray(chatId: 'room')
                      : SubagentPane(
                          chatId: 'room',
                          jobId: selected,
                          onClose: () =>
                              ref
                                      .read(
                                        selectedSubagentProvider(
                                          'room',
                                        ).notifier,
                                      )
                                      .state =
                                  null,
                        );
                },
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('subagent-one')));
      await tester.pumpAndSettle();
      expect(find.text('Add a search box'), findsOneWidget);
      expect(find.text('Search implemented'), findsOneWidget);
      expect(find.text('read_file'), findsOneWidget);
      await tester.enterText(
        find.byType(TextField),
        'Also support keyboard navigation',
      );
      await tester.tap(find.byIcon(Icons.send));
      await tester.pumpAndSettle();
      expect(api.action, 'send');
      expect(api.child, 'one');
      expect(api.chat, 'room');
      expect(api.instruction, 'Also support keyboard navigation');
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'interrupted main review resumes saved result without rerunning child',
    (tester) async {
      final api = _Api();
      final interrupted = SubagentJob.fromJson({
        'id': 'one',
        'chat_id': 'room',
        'title': 'Implement search',
        'prompt': 'Add search',
        'status': 'awaiting_followup',
        'output': 'Saved child result',
        'result_delivery': 'interrupted',
        'delivery_error': 'Provider disconnected',
      });
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(api),
            subagentsProvider(
              'room',
            ).overrideWith((ref) => Stream.value([interrupted])),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: SubagentPane(chatId: 'room', jobId: 'one', onClose: () {}),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Saved child result'), findsOneWidget);
      expect(find.text('Provider disconnected'), findsOneWidget);
      expect(
        find.text(L10n.t('subagent_delivery_interrupted')),
        findsOneWidget,
      );
      await tester.tap(find.text(L10n.t('subagent_resume_review')));
      await tester.pumpAndSettle();
      expect(api.action, 'resume');
      expect(api.child, 'one');
      expect(api.chat, 'room');
      expect(api.instruction, isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets('selection and transcript are isolated by chat', (tester) async {
    final container = ProviderContainer(
      overrides: [
        subagentsProvider('room').overrideWith((ref) => Stream.value([job])),
        subagentsProvider(
          'other',
        ).overrideWith((ref) => Stream.value(const [])),
      ],
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(
          home: Scaffold(body: SubagentTray(chatId: 'room')),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('subagent-one')));
    expect(container.read(selectedSubagentProvider('room')), 'one');
    expect(container.read(selectedSubagentProvider('other')), isNull);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(
          home: Scaffold(body: SubagentTray(chatId: 'other')),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('Implement search'), findsNothing);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('concurrency setting writes selected value and reloads', (
    tester,
  ) async {
    final api = _Api();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(api),
          subagentParallelismProvider.overrideWith(
            (ref) => api.getSubagentParallelism(),
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: SubagentSettings())),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<int>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('5').last);
    await tester.pumpAndSettle();
    expect(api.saved, 5);
    expect(find.byKey(const ValueKey('subagent-parallel-5')), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
  });
  testWidgets('poll waits for gate, never overlaps, and stops on disposal', (
    tester,
  ) async {
    final api = _PollingApi();
    final gate = StreamController<AuthGateInfo>();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(api),
          authGateProvider.overrideWith((ref) => gate.stream),
        ],
        child: const MaterialApp(
          home: Scaffold(body: SubagentTray(chatId: 'room')),
        ),
      ),
    );
    await tester.pump();
    gate.add(const AuthGateInfo(AuthGateState.loginNeeded));
    await tester.pump();
    await tester.pump(const Duration(seconds: 5));
    expect(api.calls, 0);
    gate.add(const AuthGateInfo(AuthGateState.ok));
    await tester.pumpAndSettle();
    final container = ProviderScope.containerOf(
      tester.element(find.byType(SubagentTray)),
    );
    expect(
      container.read(authGateProvider).valueOrNull?.state,
      AuthGateState.ok,
    );
    expect(
      container.read(subagentsProvider('room')).hasError,
      isFalse,
      reason: container.read(subagentsProvider('room')).error.toString(),
    );
    expect(api.calls, 1);
    await tester.pump(const Duration(seconds: 10));
    expect(api.calls, 1);
    api.pending.complete([]);
    await tester.pump();
    api.pending = Completer();
    await tester.pump(const Duration(seconds: 2));
    expect(api.calls, 2);
    await tester.pumpWidget(const SizedBox.shrink());
    api.pending.complete([]);
    await tester.pump(const Duration(seconds: 10));
    expect(api.calls, 2);
    unawaited(gate.close());
    await tester.pump();
  });
}
