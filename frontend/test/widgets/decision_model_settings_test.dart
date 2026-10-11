import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/models/image_config.dart';
import 'package:memo_flutter/providers/image_config_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/providers_tab.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/settings/decision_model_section.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/providers/provider_provider.dart';

class _ProviderList extends ProviderListNotifier {
  @override
  Future<List<ProviderConfig>> build() async => [
    const ProviderConfig(
      type: 'custom',
      name: 'cheap',
      model: 'chat-llm',
      enabled: true,
      models: [
        ConfiguredModel(id: 'chat-llm'),
        ConfiguredModel(id: 'tiny'),
        ConfiguredModel(id: 'fast-model'),
      ],
    ),
  ];
}

class _DecisionApi extends MemoApiClient {
  _DecisionApi() : super(baseUrl: 'http://memo.test');
  bool? savedEnabled;
  bool? savedVerification;
  int? savedThreshold, savedRisk;
  String? savedProvider;
  String? savedModel;
  @override
  Future<ProviderModelList> listProviderModels(
    String name, {
    bool fresh = false,
  }) async => const ProviderModelList();
  @override
  Future<Map<String, dynamic>> getLearningSettings() async => {
    'decision_enabled': true,
    'verification_enabled': true,
    'verification_threshold': 90,
    'verification_max_risk': 15,
    'decision_provider': 'cheap',
    'decision_model': 'tiny',
  };
  @override
  Future<Map<String, dynamic>> getCalendarSettings() async => {};
  @override
  Future<void> updateLearningSettings(
    bool singleModelEnabled,
    String modelId, {
    bool? decisionEnabled,
    String decisionProvider = '',
    String decisionModel = '',
    bool? verificationEnabled,
    int verificationThreshold = 85,
    int verificationMaxRisk = 20,
  }) async {
    savedEnabled = decisionEnabled;
    savedVerification = verificationEnabled;
    savedThreshold = verificationThreshold;
    savedRisk = verificationMaxRisk;
    savedProvider = decisionProvider;
    savedModel = decisionModel;
  }

  @override
  Future<void> updateCalendarSettings(
    int leadMinutes, {
    bool disableTimeGuess = false,
  }) async {}
}

class _Images extends ImageConfigNotifier {
  @override
  Future<ImageConfig> build() async =>
      const ImageConfig(defaultProvider: 'cheap', defaultModel: 'image-test');
}

class _Active extends ActiveProviderNotifier {
  @override
  Future<String> build() async => 'cheap';
}

void main() {
  for (final locale in MemoLocale.values) {
    testWidgets('decision selection loads and saves in $locale', (
      tester,
    ) async {
      final previous = L10n.locale;
      L10n.setLocale(locale);
      addTearDown(() => L10n.setLocale(previous));
      final api = _DecisionApi();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(api),
            providerListProvider.overrideWith(_ProviderList.new),
          ],
          child: const MaterialApp(
            home: Scaffold(
              body: SingleChildScrollView(child: DecisionModelSection()),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('decision_model_title')), findsOneWidget);
      expect(
        find.text(L10n.t('decision_recommendation_title')),
        findsOneWidget,
      );
      expect(find.text(L10n.t('decision_recommendation_desc')), findsOneWidget);
      expect(find.text(L10n.t('decision_scores_note')), findsOneWidget);
      expect(find.text('cheap · tiny'), findsOneWidget);
      await tester.ensureVisible(find.byType(DropdownButtonFormField<String>));
      await tester.tap(find.byType(DropdownButtonFormField<String>));
      await tester.pumpAndSettle();
      expect(find.text('cheap · chat-llm'), findsWidgets);
      await tester.tap(find.text('cheap · fast-model').last);
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text(L10n.t('save')));
      await tester.pumpAndSettle();
      await tester.tap(find.text(L10n.t('save')));
      await tester.pumpAndSettle();
      expect(api.savedEnabled, true);
      expect(api.savedProvider, 'cheap');
      expect(api.savedModel, 'fast-model');
      expect(api.savedVerification, true);
      expect(api.savedThreshold, 90);
      expect(api.savedRisk, 15);
      expect(tester.takeException(), isNull);
    });
  }
  for (final locale in MemoLocale.values) {
    testWidgets(
      'provider sections preserve drafts at narrow width in $locale',
      (tester) async {
        final previous = L10n.locale;
        L10n.setLocale(locale);
        addTearDown(() => L10n.setLocale(previous));
        tester.view.physicalSize = const Size(360, 900);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.reset);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(_DecisionApi()),
              providerListProvider.overrideWith(_ProviderList.new),
              imageConfigProvider.overrideWith(_Images.new),
              activeProviderTypeProvider.overrideWith(_Active.new),
            ],
            child: MaterialApp(
              theme: MemoTheme.darkThemeData,
              home: const Scaffold(body: ProvidersTab()),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(ProviderCard), findsOneWidget);
        expect(tester.takeException(), isNull);
        await tester.ensureVisible(find.text(L10n.t('providers_images')));
        await tester.pumpAndSettle();
        await tester.tap(find.text(L10n.t('providers_images')));
        await tester.pumpAndSettle();
        final field = find.byKey(const Key('image_default_model'));
        await tester.ensureVisible(field);
        await tester.enterText(field, 'unsaved-image-draft');
        await tester.pumpAndSettle();
        // Horizontal section navigation must be reachable on small windows.
        final decisions = find.text(L10n.t('providers_decisions'));
        await tester.ensureVisible(decisions);
        await tester.tap(decisions);
        await tester.pumpAndSettle();
        expect(find.text('cheap · tiny'), findsOneWidget);
        await tester.ensureVisible(find.text(L10n.t('verification_limits')));
        await tester.pumpAndSettle();
        await tester.tap(find.text(L10n.t('verification_limits')));
        await tester.pumpAndSettle();
        expect(find.byType(Slider), findsNWidgets(2));
        expect(tester.takeException(), isNull);
        await tester.ensureVisible(find.text(L10n.t('providers_images')));
        await tester.ensureVisible(find.text(L10n.t('providers_images')));
        await tester.pumpAndSettle();
        await tester.tap(find.text(L10n.t('providers_images')));
        await tester.pumpAndSettle();
        expect(find.text('unsaved-image-draft'), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  }
}
