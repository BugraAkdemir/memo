import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/chat_input.dart';
import 'package:memo_flutter/widgets/chat_model_selection.dart';
import 'package:memo_flutter/widgets/model_picker.dart';

const _providers = [
  ProviderConfig(
    type: 'custom',
    name: 'API',
    model: 'first',
    enabled: true,
    models: [
      ConfiguredModel(id: 'first'),
      ConfiguredModel(id: 'saved-model'),
    ],
  ),
  ProviderConfig(
    type: 'custom',
    name: kSubscriptionsProviderName,
    model: 'gpt-test-0',
    enabled: true,
  ),
  ProviderConfig(
    type: 'custom',
    name: 'Disabled',
    model: 'hidden',
    enabled: false,
  ),
  ProviderConfig(type: 'codex-cli', name: 'CLI', model: 'codex', enabled: true),
];

class _Adapter implements HttpClientAdapter {
  final writes = <({String path, Map<String, dynamic> data})>[];
  bool failSelection = false;
  @override
  Future<ResponseBody> fetch(
    RequestOptions o,
    Stream<Uint8List>? stream,
    Future<void>? cancel,
  ) async {
    Object? data;
    var status = 200;
    if (o.method != 'GET') {
      writes.add((
        path: o.path,
        data: Map<String, dynamic>.from(o.data as Map),
      ));
      if (failSelection && o.path == '/api/providers/model') status = 500;
      data = status == 200
          ? <String, dynamic>{}
          : {'error': 'disk write failed'};
    } else if (o.path == '/api/providers') {
      data = _providers.map((p) => p.toJson()).toList();
    } else if (o.path == '/api/providers/active') {
      data = {'provider': 'API'};
    } else if (o.path == '/api/chats/active') {
      data = {'id': 'chat-one'};
    } else if (o.path == '/api/chats/cli-provider') {
      data = {'cli_provider': ''};
    } else if (o.path == '/api/providers/model') {
      data = {
        'models': List.generate(
          40,
          (i) => {'id': 'gpt-test-$i', 'owned_by': 'openai', 'remaining': 0.5},
        ),
      };
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

  @override
  void close({bool force = false}) {}
}

void main() {
  test(
    'shared pool includes saved/account models, Auto Route and CLI only',
    () {
      final entries = chatModelPickerEntries(
        providers: _providers,
        subscriptionModels: const [
          ProviderModel(id: 'live', ownedBy: 'openai'),
        ],
        activeProvider: 'API',
      );
      expect(entries.where((e) => e.value != null).map((e) => e.value), [
        kAutoRouteProviderName,
        'local',
        encodeModelSelection('API', 'first'),
        encodeModelSelection('API', 'saved-model'),
        encodeModelSelection(kSubscriptionsProviderName, 'live'),
        'CLI',
      ]);
      expect(
        entries.where((e) => e.active).single.value,
        encodeModelSelection('API', 'first'),
      );
      final cli = chatModelPickerEntries(
        providers: _providers,
        subscriptionModels: const [],
        activeProvider: 'API',
        cliType: 'codex-cli',
      );
      expect(cli.where((e) => e.active).single.value, 'CLI');
    },
  );
  for (final selection in [
    kAutoRouteProviderName,
    'local',
    encodeModelSelection('API', 'saved-model'),
  ]) {
    test('selection $selection exits only selected chat CLI', () async {
      final adapter = _Adapter();
      final api = MemoApiClient(baseUrl: 'http://memo.test');
      api.dio.httpClientAdapter = adapter;
      await selectNonCLIChatModel(api, selection, chatId: 'chat-one');
      expect(adapter.writes.last.path, '/api/chats/cli-provider');
      expect(adapter.writes.last.data, {'id': 'chat-one', 'cli_provider': ''});
      if (selection.startsWith(kModelSelectPrefix))
        expect(adapter.writes.first.data, {
          'name': 'API',
          'model': 'saved-model',
          'activate': true,
        });
    });
  }
  test('failed model save preserves CLI mode', () async {
    final adapter = _Adapter()..failSelection = true;
    final api = MemoApiClient(baseUrl: 'http://memo.test');
    api.dio.httpClientAdapter = adapter;
    await expectLater(
      selectNonCLIChatModel(
        api,
        encodeModelSelection('API', 'saved-model'),
        chatId: 'chat-one',
      ),
      throwsA(isA<DioException>()),
    );
    expect(adapter.writes.map((r) => r.path), ['/api/providers/model']);
  });
  for (final locale in MemoLocale.values) {
    testWidgets(
      'composer saved models, long account list and upward panel in $locale',
      (tester) async {
        L10n.setLocale(locale);
        addTearDown(() => L10n.setLocale(MemoLocale.en));
        resetModelPickerFolds();
        tester.view.physicalSize = const Size(375, 812);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.reset);
        SharedPreferences.setMockInitialValues({});
        final prefs = await SharedPreferences.getInstance();
        final adapter = _Adapter();
        final api = MemoApiClient(baseUrl: 'http://memo.test');
        api.dio.httpClientAdapter = adapter;
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(api),
            prefsProvider.overrideWithValue(prefs),
            authGateProvider.overrideWith(
              (ref) => Stream.value(AuthGateInfo(AuthGateState.ok)),
            ),
          ],
        );
        addTearDown(container.dispose);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: const MaterialApp(
              home: Scaffold(
                body: Column(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [ChatInput()],
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.enterText(find.byType(TextField).first, '/model');
        await tester.pumpAndSettle();
        await tester.tap(find.text(L10n.t('template_switch_model')));
        await tester.pumpAndSettle();
        expect(find.text(L10n.t('auto_route')), findsOneWidget);
        expect(find.text('saved-model'), findsOneWidget);
        expect(find.text('hidden'), findsNothing);
        expect(tester.takeException(), isNull);
        final search = find.byWidgetPredicate(
          (w) => w is TextField && w.decoration?.hintText == L10n.t('search'),
        );
        expect(search, findsOneWidget);
        expect(
          tester.getBottomRight(find.text('saved-model')).dy,
          lessThan(812),
        );
        await tester.enterText(search, 'GPT Test 39');
        await tester.pumpAndSettle();
        expect(
          find.byWidgetPredicate((w) => w is Text && w.data == 'GPT Test 39'),
          findsOneWidget,
        );
        await tester.tap(
          find.byWidgetPredicate((w) => w is Text && w.data == 'GPT Test 39'),
        );
        await tester.pumpAndSettle();
        expect(
          adapter.writes.any(
            (r) =>
                r.path == '/api/providers/model' &&
                r.data['name'] == kSubscriptionsProviderName &&
                r.data['model'] == 'gpt-test-39',
          ),
          isTrue,
        );
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pumpAndSettle();
      },
    );
  }
}
