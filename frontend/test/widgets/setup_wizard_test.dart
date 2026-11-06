import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/local_model.dart';
import 'package:memo_flutter/models/mail.dart';
import 'package:memo_flutter/models/twin.dart';
import 'package:memo_flutter/providers/mail_provider.dart';
import 'package:memo_flutter/models/gpu_info.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/models/subscriptions.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/setup_wizard_view.dart';

class _Api extends MemoApiClient {
  _Api({this.localModels = const []}) : super(baseUrl: 'http://memo.test');
  final List<LocalModel> localModels;
  String active = '', prompt = '';
  Map<String, dynamic> replyModel = {};
  bool completed = false, loginStarted = false;
  @override
  Future<bool> isAlive() async => true;
  @override
  Future<TwinState> getTwinState() async =>
      TwinState(const [], const {}, const [], const [], replyModel);
  @override
  Future<List<LocalModel>> listLocalModels() async => localModels;
  @override
  Future<String> getActiveProvider() async => active;
  @override
  Future<void> setActiveProvider(String name) async {
    active = name;
  }

  @override
  Future<bool> getOnboardingComplete() async => completed;
  @override
  Future<void> setOnboardingComplete(bool value) async {
    completed = value;
  }

  @override
  Future<String> getSystemPrompt() async => prompt;
  @override
  Future<void> setSystemPrompt(String value) async {
    prompt = value;
  }

  @override
  Future<String> startSubscriptionLogin(String vendor) async {
    loginStarted = true;
    return 'https://example.com/login';
  }
}

class _Mail extends MailAccountsNotifier {
  _Mail(this.accounts);
  final List<MailAccount> accounts;
  @override
  Future<List<MailAccount>> build() async => accounts;
}

class _Providers extends ProviderListNotifier {
  @override
  Future<List<ProviderConfig>> build() async => [];
}

class _Active extends ActiveProviderNotifier {
  @override
  Future<String> build() async => '';
}

class _Subs extends SubscriptionsNotifier {
  _Subs(this.api);
  final _Api api;
  @override
  Future<SubscriptionsState> build() async => const SubscriptionsState(
    bundled: true,
    running: true,
    providers: ['codex'],
  );
  @override
  Future<void> reload() async {
    if (api.loginStarted)
      state = const AsyncData(
        SubscriptionsState(
          bundled: true,
          running: true,
          providers: ['codex'],
          accounts: [
            SubscriptionAccount(provider: 'codex', email: 'test@example.com'),
          ],
          models: [ProviderModel(id: 'chat-model', ownedBy: 'codex')],
          model: 'chat-model',
          login: SubscriptionLogin(provider: 'codex', done: true),
        ),
      );
  }
}

void main() {
  testWidgets(
    'an embedding model alone is not a ready local chat model',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      final prefs = await SharedPreferences.getInstance();
      final api = _Api(
        localModels: [
          const LocalModel(
            repoId: 'embedding',
            filename: 'embedding.gguf',
            size: 1,
            path: '/test/embedding.gguf',
            isEmbedding: true,
          ),
        ],
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            prefsProvider.overrideWithValue(prefs),
            apiClientProvider.overrideWithValue(api),
            subscriptionsProvider.overrideWith(() => _Subs(api)),
            gpuInfoProvider.overrideWith((ref) async => const GPUInfo()),
            downloadProgressProvider.overrideWith(
              (ref) => Stream.value(<DownloadProgress>[]),
            ),
          ],
          child: const MaterialApp(home: Scaffold(body: SetupWizardOverlay())),
        ),
      );
      await tester.pumpAndSettle();
      // Language, then the model page.
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      // Local is the default choice: the store card says nothing is ready,
      // and the embedding model does not count as a chat model.
      expect(find.byKey(const Key('setup_local_empty')), findsOneWidget);
      expect(find.byKey(const Key('setup_local_ready')), findsNothing);
      expect(find.byKey(const Key('setup_open_store')), findsOneWidget);
      // Without a chat model the required step blocks "next".
      expect(
        tester
            .widget<FilledButton>(find.byKey(const Key('setup_next')))
            .onPressed,
        isNull,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  // The phone connects (MobileConnectScreen) and signs in (AuthGateOverlay)
  // before the wizard can matter, and the wizard is only for a server that
  // was never set up. These pin the three states a phone can be in.
  Future<void> pumpMobileWizard(
    WidgetTester tester, {
    required _Api api,
    String savedUrl = 'http://memo.test:8090',
    AuthGateState gate = AuthGateState.ok,
  }) async {
    SharedPreferences.setMockInitialValues({
      if (savedUrl.isNotEmpty) 'memo_api_base_url': savedUrl,
    });
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          prefsProvider.overrideWithValue(prefs),
          apiClientProvider.overrideWithValue(api),
          authGateProvider.overrideWith(
            (ref) => Stream.value(AuthGateInfo(gate)),
          ),
          gpuInfoProvider.overrideWith((ref) async => const GPUInfo()),
        ],
        child: const MaterialApp(home: Scaffold(body: SetupWizardOverlay())),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets(
    'phone: a server that is already set up never shows the wizard',
    (tester) async {
      await pumpMobileWizard(tester, api: _Api()..completed = true);
      expect(find.byKey(const Key('setup_next')), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.android}),
  );

  testWidgets(
    'phone: the wizard waits for a saved server address and for sign-in',
    (tester) async {
      await pumpMobileWizard(tester, api: _Api(), savedUrl: '');
      expect(
        find.byKey(const Key('setup_next')),
        findsNothing,
        reason: 'no address yet: the connect screen owns the phone',
      );
      await pumpMobileWizard(
        tester,
        api: _Api(),
        gate: AuthGateState.loginNeeded,
      );
      expect(
        find.byKey(const Key('setup_next')),
        findsNothing,
        reason: 'signed out: the login gate owns the phone',
      );
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.android}),
  );

  testWidgets(
    'phone: a never-set-up server starts the wizard at language and theme, with no server step',
    (tester) async {
      await pumpMobileWizard(tester, api: _Api());
      expect(find.text(L10n.t('setup_step_language_theme')), findsWidgets);
      // The old first page asked for an address and a token in two fields.
      expect(find.byType(TextField), findsNothing);
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('setup_back')));
      await tester.pumpAndSettle();
      expect(find.text(L10n.t('setup_step_language_theme')), findsWidgets);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.android}),
  );

  Future<void> pumpDesktopWizard(
    WidgetTester tester,
    _Api api, {
    List<MailAccount> mail = const [],
  }) async {
    tester.view.physicalSize = const Size(1100, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          prefsProvider.overrideWithValue(prefs),
          apiClientProvider.overrideWithValue(api),
          providerListProvider.overrideWith(_Providers.new),
          activeProviderTypeProvider.overrideWith(_Active.new),
          mailAccountsProvider.overrideWith(() => _Mail(mail)),
          subscriptionsProvider.overrideWith(() => _Subs(api)),
          gpuInfoProvider.overrideWith((ref) async => const GPUInfo()),
          downloadProgressProvider.overrideWith(
            (ref) => Stream.value(<DownloadProgress>[]),
          ),
        ],
        child: const MaterialApp(home: Scaffold(body: SetupWizardOverlay())),
      ),
    );
    await tester.pumpAndSettle();
  }

  Future<void> tapStep(WidgetTester tester, String label) async {
    // The side list tile; the summary rows repeat the same label.
    final tile = find.widgetWithText(ListTile, label);
    final f = tile.evaluate().isNotEmpty ? tile : find.text(label);
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  FilledButton nextButton(WidgetTester tester) =>
      tester.widget<FilledButton>(find.byKey(const Key('setup_next')));

  testWidgets(
    'the model step is required: starred, and nothing moves on or finishes without a model',
    (tester) async {
      await pumpDesktopWizard(tester, _Api());
      expect(find.text('${L10n.t('setup_step_model')} *'), findsOneWidget);
      // Language, then persona: free to move on.
      expect(nextButton(tester).onPressed, isNotNull);
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      // The model page: required and empty, so "next" is off.
      expect(nextButton(tester).onPressed, isNull);
      // Jumping ahead through the side list still cannot finish.
      await tapStep(tester, L10n.t('setup_step_check'));
      expect(find.byKey(const Key('setup_missing_required')), findsOneWidget);
      expect(nextButton(tester).onPressed, isNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  final chatModel = [
    const LocalModel(
      repoId: 'chat',
      filename: 'chat.gguf',
      size: 1,
      path: '/test/chat.gguf',
      isEmbedding: false,
    ),
  ];
  const account = MailAccount(id: 'g1', email: 'me@example.com');

  testWidgets(
    'no mail account: the reply-model step is not in the list at all',
    (tester) async {
      await pumpDesktopWizard(tester, _Api(localModels: chatModel));
      expect(find.text(L10n.t('setup_step_mail_model')), findsNothing);
      expect(find.text(L10n.t('setup_step_mail')), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'a connected mail account makes the reply model required and blocks finishing',
    (tester) async {
      await pumpDesktopWizard(
        tester,
        _Api(localModels: chatModel),
        mail: const [account],
      );
      expect(find.text('${L10n.t('setup_step_mail_model')} *'), findsOneWidget);
      // Jump to the reply-model page through the side list.
      await tapStep(tester, '${L10n.t('setup_step_mail_model')} *');
      expect(
        find.byKey(const Key('setup_mail_model_required')),
        findsOneWidget,
      );
      expect(nextButton(tester).onPressed, isNull);
      // The summary names the missing step and cannot finish.
      await tapStep(tester, L10n.t('setup_step_check'));
      expect(find.byKey(const Key('setup_missing_required')), findsOneWidget);
      expect(
        find.textContaining(L10n.t('setup_step_mail_model')),
        findsWidgets,
      );
      expect(nextButton(tester).onPressed, isNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'with a mail account and a saved reply model the wizard can finish',
    (tester) async {
      final api = _Api(localModels: chatModel)
        ..replyModel = {'provider': 'Subscriptions', 'model': 'm1'};
      await pumpDesktopWizard(tester, api, mail: const [account]);
      await tapStep(tester, '${L10n.t('setup_step_mail_model')} *');
      expect(find.byKey(const Key('setup_mail_model_ready')), findsOneWidget);
      expect(nextButton(tester).onPressed, isNotNull);
      await tapStep(tester, L10n.t('setup_step_check'));
      expect(find.byKey(const Key('setup_missing_required')), findsNothing);
      expect(nextButton(tester).onPressed, isNotNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'finishing a re-run does not overwrite the persona the user already has',
    (tester) async {
      tester.view.physicalSize = const Size(1100, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      SharedPreferences.setMockInitialValues({'memo_setup_rerun': true});
      final prefs = await SharedPreferences.getInstance();
      final api = _Api(localModels: chatModel)..prompt = 'my own prompt';
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            prefsProvider.overrideWithValue(prefs),
            apiClientProvider.overrideWithValue(api),
            providerListProvider.overrideWith(_Providers.new),
            activeProviderTypeProvider.overrideWith(_Active.new),
            mailAccountsProvider.overrideWith(() => _Mail(const [])),
            subscriptionsProvider.overrideWith(() => _Subs(api)),
            gpuInfoProvider.overrideWith((ref) async => const GPUInfo()),
            downloadProgressProvider.overrideWith(
              (ref) => Stream.value(<DownloadProgress>[]),
            ),
          ],
          child: const MaterialApp(home: Scaffold(body: SetupWizardOverlay())),
        ),
      );
      await tester.pumpAndSettle();
      await tapStep(tester, L10n.t('setup_step_check'));
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      expect(api.completed, true);
      expect(api.prompt, 'my own prompt');
      expect(prefs.getBool('memo_setup_rerun') ?? false, false);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'the summary lists every step with its state and a row goes back to it',
    (tester) async {
      await pumpDesktopWizard(tester, _Api(localModels: chatModel));
      await tapStep(tester, L10n.t('setup_step_check'));
      // Required and done: the model; never opened: embedding.
      expect(find.byKey(const Key('setup_summary_model')), findsOneWidget);
      expect(
        find.descendant(
          of: find.byKey(const Key('setup_summary_model')),
          matching: find.text(L10n.t('setup_summary_done')),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byKey(const Key('setup_summary_embedding')),
          matching: find.text(L10n.t('setup_summary_skipped')),
        ),
        findsOneWidget,
      );
      // The summary row of a step that was visited reads as done.
      await tapStep(tester, L10n.t('setup_step_decision'));
      await tapStep(tester, L10n.t('setup_step_check'));
      expect(
        find.descendant(
          of: find.byKey(const Key('setup_summary_decision')),
          matching: find.text(L10n.t('setup_summary_done')),
        ),
        findsOneWidget,
      );
      // Tapping a row opens that step.
      await tester.ensureVisible(
        find.byKey(const Key('setup_summary_embedding')),
      );
      await tester.tap(find.byKey(const Key('setup_summary_embedding')));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const Key('setup_embedding_skip_note')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'decision model, memory and voice/image have their own optional pages',
    (tester) async {
      await pumpDesktopWizard(tester, _Api(localModels: chatModel));
      for (final step in [
        ('setup_step_decision', 'decision_tab_title'),
        ('setup_step_preferences', 'setup_memory_enabled_title'),
        ('setup_step_voice_image', 'stt_providers_title'),
        ('setup_step_voice_image', 'image_section_title'),
        ('setup_step_channels', 'setup_channels_tip'),
        ('setup_step_briefing', 'setup_briefing_morning_title'),
        ('setup_step_permissions', 'setup_backup_title'),
      ]) {
        // Optional: no star on the label, and "next" stays available.
        expect(find.text('${L10n.t(step.$1)} *'), findsNothing);
        await tapStep(tester, L10n.t(step.$1));
        expect(find.text(L10n.t(step.$2)), findsWidgets, reason: step.$1);
        expect(nextButton(tester).onPressed, isNotNull, reason: step.$1);
      }
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'choosing local opens the Model Store window instead of one recommendation',
    (tester) async {
      await pumpDesktopWizard(tester, _Api());
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      // The old single "download these models" recommendation is gone.
      expect(find.text(L10n.t('setup_model_download_button')), findsNothing);
      await tester.tap(find.byKey(const Key('setup_open_store')));
      await tester.pump(const Duration(milliseconds: 300));
      expect(find.byKey(const Key('setup_model_store_dialog')), findsOneWidget);
      await tester.tap(find.byKey(const Key('setup_store_done')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('setup_model_store_dialog')), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  testWidgets(
    'a downloaded local chat model satisfies the required step',
    (tester) async {
      final api = _Api(
        localModels: [
          const LocalModel(
            repoId: 'chat',
            filename: 'chat.gguf',
            size: 1,
            path: '/test/chat.gguf',
            isEmbedding: false,
          ),
        ],
      );
      await pumpDesktopWizard(tester, api);
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('setup_next')));
      await tester.pumpAndSettle();
      expect(nextButton(tester).onPressed, isNotNull);
      await tapStep(tester, L10n.t('setup_step_check'));
      expect(find.byKey(const Key('setup_missing_required')), findsNothing);
      expect(nextButton(tester).onPressed, isNotNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    variant: TargetPlatformVariant({TargetPlatform.linux}),
  );

  for (final locale in MemoLocale.values) {
    for (final size in [
      const Size(360, 900),
      const Size(1100, 900),
      const Size(1100, 600),
    ]) {
      final width = size.width;
      testWidgets(
        'setup subscriptions, draft and completion $locale at $size',
        (tester) async {
          final previous = L10n.locale;
          L10n.setLocale(locale);
          addTearDown(() => L10n.setLocale(previous));
          tester.view.physicalSize = size;
          tester.view.devicePixelRatio = 1;
          addTearDown(tester.view.reset);
          SharedPreferences.setMockInitialValues({});
          final prefs = await SharedPreferences.getInstance();
          final api = _Api();
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                prefsProvider.overrideWithValue(prefs),
                apiClientProvider.overrideWithValue(api),
                providerListProvider.overrideWith(_Providers.new),
                activeProviderTypeProvider.overrideWith(_Active.new),
                subscriptionsProvider.overrideWith(() => _Subs(api)),
                gpuInfoProvider.overrideWith((ref) async => const GPUInfo()),
                downloadProgressProvider.overrideWith(
                  (ref) => Stream.value(<DownloadProgress>[]),
                ),
              ],
              child: const MaterialApp(
                home: Scaffold(body: SetupWizardOverlay()),
              ),
            ),
          );
          await tester.pumpAndSettle();
          if (width > 800) {
            await tapStep(tester, L10n.t('theme_dark'));
          }
          Future<void> next() async {
            await tester.tap(find.byKey(const Key('setup_next')));
            await tester.pumpAndSettle();
            expect(tester.takeException(), isNull);
          }

          await next(); // language -> model
          for (final path in ['local', 'api', 'subscriptions']) {
            final choice = find.byKey(ValueKey('setup_connection_$path'));
            await tester.ensureVisible(choice);
            await tester.tap(choice);
            await tester.pumpAndSettle();
            expect(tester.takeException(), isNull);
          }
          final use = find.byKey(const Key('setup_use_subscription'));
          expect(tester.widget<FilledButton>(use).onPressed, isNull);
          final signIn = find.text(L10n.t('subs_sign_in'));
          await tester.ensureVisible(signIn);
          await tester.pumpAndSettle();
          await tester.tap(signIn);
          await tester.pumpAndSettle();
          expect(api.loginStarted, true);
          await tester.pump(const Duration(seconds: 2));
          await tester.pumpAndSettle();
          await tester.ensureVisible(use);
          await tester.pumpAndSettle();
          await tester.tap(use);
          await tester.pumpAndSettle();
          expect(api.active, 'Subscriptions');
          await next(); // model -> embedding
          expect(
            find.byKey(const Key('setup_embedding_skip_note')),
            findsOneWidget,
          );
          await next(); // embedding (skipped) -> persona
          final name = find.byType(TextField).first;
          await tester.enterText(name, 'Ada');
          await next(); // persona -> mail (optional)
          expect(find.byKey(const Key('setup_mail_gmail')), findsOneWidget);
          // Return to persona; the typed name survives.
          await tester.tap(find.byKey(const Key('setup_back')));
          await tester.pumpAndSettle();
          expect(find.text('Ada'), findsOneWidget);
          await next(); // -> mail
          await next(); // mail skipped -> decision model
          await next(); // decision skipped -> memory and learning
          await next(); // memory -> voice and images
          await next(); // voice and images skipped -> channels
          await next(); // channels skipped -> daily briefing
          await next(); // briefing skipped -> privacy and backup
          await next(); // privacy skipped -> summary
          await next(); // finish
          expect(api.completed, true);
          expect(api.prompt, contains('Ada'));
          expect(prefs.getBool('memo_setup_complete'), true);
          expect(find.byKey(const Key('setup_next')), findsNothing);
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
        },
        variant: TargetPlatformVariant({TargetPlatform.linux}),
      );
    }
  }
}
