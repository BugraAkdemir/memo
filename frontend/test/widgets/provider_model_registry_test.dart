import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/provider_config_dialog.dart';
import 'package:memo_flutter/widgets/settings/tabs/providers_tab.dart';

class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://memo.test');
  ProviderConfig? saved;
  String? effortType;
  @override
  Future<List<String>> getEffortLevels(String type, {String? model}) async {
    effortType = type;
    return type == 'subscriptions' ? ['medium', 'high'] : [];
  }

  @override
  Future<void> updateProvider(ProviderConfig config) async {
    saved = config;
  }
}

void main() {
  testWidgets('provider model card fits a narrow window with long IDs', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: ProviderCard(
              isActive: true,
              p: ProviderConfig(
                type: 'custom',
                name: 'An exceptionally long provider connection label',
                model: 'chat-one',
                enabled: true,
                connected: true,
                models: [
                  ConfiguredModel(id: 'chat-one-with-a-very-long-model-id'),
                  ConfiguredModel(id: 'review-with-a-long-name'),
                ],
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('subscription editor discovers and saves supported effort', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1200, 1600);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final api = _Api();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(
          home: Scaffold(
            body: ProviderConfigDialog(
              existing: ProviderConfig(
                type: 'custom',
                name: 'Subscriptions',
                baseUrl: 'http://memo.test/v1',
                model: 'published-model',
                enabled: true,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.effortType, 'subscriptions');
    await tester.ensureVisible(find.text(L10n.t('advanced_settings')));
    await tester.tap(find.text(L10n.t('advanced_settings')));
    await tester.pumpAndSettle();
    final effort = find.byWidgetPredicate((widget) =>
      widget is DropdownButtonFormField<String> &&
      widget.decoration.labelText == L10n.t('effort_level_label'));
    await tester.ensureVisible(effort);
    await tester.tap(effort);
    await tester.pumpAndSettle();
    expect(find.text('medium').last, findsOneWidget);
    expect(find.text('xhigh'), findsNothing);
    await tester.tap(find.text('high').last);
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text(L10n.t('save')));
    await tester.tap(find.text(L10n.t('save')));
    await tester.pumpAndSettle();
    expect(api.saved?.effortLevel, 'high');
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'one API connection saves multiple models without role selection',
    (tester) async {
      final locale = L10n.locale;
      L10n.setLocale(MemoLocale.en);
      addTearDown(() => L10n.setLocale(locale));
      tester.view.physicalSize = const Size(1200, 1200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final api = _Api();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [apiClientProvider.overrideWithValue(api)],
          child: const MaterialApp(
            home: Scaffold(
              body: ProviderConfigDialog(
                existing: ProviderConfig(
                  type: 'custom',
                  name: 'Zen proxy',
                  baseUrl: 'http://memo.test/v1',
                  apiKey: 'existing-key',
                  enabled: true,
                  model: 'chat-one',
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.widgetWithText(TextField, 'chat-one'),
        'chat-two',
      );
      await tester.ensureVisible(find.text(L10n.t('provider_add_model')));
      await tester.tap(find.text(L10n.t('provider_add_model')));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.widgetWithText(TextField, 'chat-two'));
      await tester.enterText(
        find.widgetWithText(TextField, 'chat-two'),
        'review',
      );
      expect(find.text(L10n.t('provider_model_role')), findsNothing);
      await tester.ensureVisible(find.text(L10n.t('save')));
      await tester.tap(find.text(L10n.t('save')));
      await tester.pumpAndSettle();
      expect(api.saved?.models.map((m) => m.id).toList(), [
        'chat-one',
        'chat-two',
        'review',
      ]);
      expect(api.saved?.models.last.id, 'review');
      expect(api.saved?.model, 'review');
      expect(api.saved?.apiKey, 'existing-key');
      expect(tester.takeException(), isNull);
    },
  );
  test('model roles survive serialization and provider toggles', () {
    final p = ProviderConfig.fromJson({
      'type': 'opencode-zen',
      'name': 'Zen',
      'model': 'chat-one',
      'models': [
        {'id': 'chat-one', 'kind': 'chat'},
        {'id': 'review', 'kind': 'decision'},
      ],
    });
    final copied = ProviderConfig.fromJson(p.copyWith(enabled: true).toJson());
    expect(copied.chatModels.map((m) => m.id), ['chat-one', 'review']);
    expect(copied.configuredModels.last.id, 'review');
  });
}
