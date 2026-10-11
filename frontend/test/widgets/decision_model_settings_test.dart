import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/widgets/settings/tabs/decision_model_tab.dart';

class _DecisionApi extends MemoApiClient {
  _DecisionApi() : super(baseUrl: 'http://memo.test');
  bool? savedEnabled;
  bool? savedVerification;
  int? savedThreshold, savedRisk;
  String? savedProvider;
  String? savedModel;
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
          overrides: [apiClientProvider.overrideWithValue(api)],
          child: const MaterialApp(
            home: Scaffold(body: SizedBox(child: DecisionModelTab())),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('decision_model_title')), findsOneWidget);
      expect(find.text('cheap'), findsOneWidget);
      expect(find.text('tiny'), findsOneWidget);
      await tester.enterText(
        find.widgetWithText(TextField, 'tiny'),
        'fast-model',
      );
      await tester.scrollUntilVisible(
        find.text(L10n.t('save')),
        200,
        scrollable: find
            .descendant(
              of: find.byType(ListView),
              matching: find.byType(Scrollable),
            )
            .first,
      );
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
}
