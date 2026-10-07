import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart' show apiClientProvider;
import 'package:memo_flutter/providers/image_config_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/widgets/settings/image_model_section.dart';

/// A tiny stand-in for /api/image/config and the model list, remembering every
/// write so the test can assert on what the section sent.
class _Backend implements HttpClientAdapter {
  _Backend({this.provider = '', this.model = '', this.reject});
  bool auto = true;
  String provider;
  String model;
  final String? reject;
  final List<Map<String, dynamic>> puts = [];

  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? requestStream,
      Future<void>? cancelFuture) async {
    final path = options.uri.path;
    if (path == '/api/image/config' && options.method == 'PUT') {
      if (reject != null) {
        return ResponseBody.fromString(reject!, 400, headers: {
          Headers.contentTypeHeader: ['text/plain'],
        });
      }
      final body = options.data as Map<String, dynamic>;
      puts.add(body);
      if (body.containsKey('auto_route')) auto = body['auto_route'] as bool;
      if (body.containsKey('default_provider')) provider = body['default_provider'] as String;
      if (body.containsKey('default_model')) model = body['default_model'] as String;
    }
    if (path == '/api/image/config') {
      return _json({'auto_route': auto, 'default_provider': provider, 'default_model': model});
    }
    if (path == '/api/providers/model') {
      return _json({
        'models': [
          {'id': 'gpt-image-1'},
          {'id': 'gpt-4o'},
          {'id': 'dall-e-3'},
        ],
        'current': 'gpt-4o',
      });
    }
    return _json({});
  }

  ResponseBody _json(Object o) => ResponseBody.fromString(jsonEncode(o), 200,
      headers: {Headers.contentTypeHeader: [Headers.jsonContentType]});

  @override
  void close({bool force = false}) {}
}

class _Providers extends ProviderListNotifier {
  @override
  Future<List<ProviderConfig>> build() async => const [
        ProviderConfig(type: 'custom', name: 'OpenCode Go', model: 'glm', enabled: true),
        ProviderConfig(type: 'openrouter', name: 'OpenRouter', model: 'x', enabled: true),
        ProviderConfig(type: 'custom', name: 'Off one', model: 'y', enabled: false),
      ];
}

Future<_Backend> _pump(WidgetTester tester, _Backend backend,
    {MemoLocale locale = MemoLocale.en}) async {
  L10n.setLocale(locale);
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = backend;
  await tester.pumpWidget(ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(client),
      providerListProvider.overrideWith(_Providers.new),
      authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
    ],
    // The gate provider is auto-dispose: something must be listening (app_shell
    // does in the real app) or it never settles and every read says "blocked".
    child: MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(
          child: Consumer(builder: (context, ref, _) {
            ref.watch(authGateProvider);
            return const ImageModelSection();
          }),
        ),
      ),
    ),
  ));
  await tester.pumpAndSettle();
  // The first read can land before the gate has settled (it then mounts the
  // defaults); app_shell re-reads on the gate transition — do the same here.
  ProviderScope.containerOf(tester.element(find.byType(ImageModelSection)))
      .invalidate(imageConfigProvider);
  // The re-read is a plain future: give it event-loop turns, not just frames.
  for (var i = 0; i < 5; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
  await tester.pumpAndSettle();
  return backend;
}

void main() {
  testWidgets('shows the saved settings and only offers enabled providers', (tester) async {
    await _pump(tester, _Backend(provider: 'OpenCode Go', model: 'gpt-image-1'));

    expect(find.text(L10n.t('image_section_title')), findsOneWidget);
    expect(tester.widget<SwitchListTile>(find.byKey(const Key('image_auto_route_switch'))).value, isTrue);
    expect(tester.widget<TextField>(find.byKey(const Key('image_default_model'))).controller!.text, 'gpt-image-1');

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    expect(find.text('OpenRouter'), findsWidgets);
    expect(find.text('Off one'), findsNothing, reason: 'a disabled provider cannot draw');
  });

  testWidgets('toggling the automatic routing sends only that field', (tester) async {
    final backend = await _pump(tester, _Backend(provider: 'OpenCode Go', model: 'gpt-image-1'));

    await tester.tap(find.byKey(const Key('image_auto_route_switch')));
    await tester.pumpAndSettle();

    expect(backend.puts, [
      {'auto_route': false}
    ], reason: 'a switch must not rewrite (or clear) the default model');
    expect(tester.widget<SwitchListTile>(find.byKey(const Key('image_auto_route_switch'))).value, isFalse);
  });

  testWidgets('saving sends the provider and the typed model together', (tester) async {
    final backend = await _pump(tester, _Backend());

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('OpenRouter').last);
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('image_default_model')), 'dall-e-3');
    await tester.tap(find.byKey(const Key('image_save')));
    await tester.pumpAndSettle();

    expect(backend.puts.single, {'default_provider': 'OpenRouter', 'default_model': 'dall-e-3'});
    expect(find.text(L10n.t('image_saved')), findsOneWidget);
  });

  testWidgets('choosing "Not set" clears both fields', (tester) async {
    final backend = await _pump(tester, _Backend(provider: 'OpenCode Go', model: 'gpt-image-1'));

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('image_default_none')).last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('image_save')));
    await tester.pumpAndSettle();

    expect(backend.puts.single, {'default_provider': '', 'default_model': ''});
  });

  testWidgets('a refusal from the backend is shown, not swallowed', (tester) async {
    await _pump(tester, _Backend(reject: 'provider "ghost" is not configured'));

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('OpenRouter').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('image_default_model')), 'dall-e-3');
    await tester.tap(find.byKey(const Key('image_save')));
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('image_saved')), findsNothing);
    expect(find.byType(Text).evaluate().any((e) => (e.widget as Text).data?.contains('ghost') ?? false), isTrue);
  });

  testWidgets('renders in Turkish too', (tester) async {
    await _pump(tester, _Backend(), locale: MemoLocale.tr);
    final title = L10n.t('image_section_title');
    final hint = L10n.t('image_default_title');
    expect(find.text(title), findsOneWidget);
    expect(find.text(hint), findsOneWidget);
    L10n.setLocale(MemoLocale.en);
    expect(L10n.t('image_section_title'), isNot(title), reason: 'the two languages must differ');
    expect(L10n.t('image_default_title'), isNot(hint));
  });
}
