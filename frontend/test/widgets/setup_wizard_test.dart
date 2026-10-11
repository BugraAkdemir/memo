import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/local_model.dart';
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
  bool completed = false, loginStarted = false;
  @override
  Future<bool> isAlive() async => true;
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
      for (var i = 0; i < 2; i++) {
        await tester.tap(find.byKey(const Key('setup_next')));
        await tester.pumpAndSettle();
      }
      expect(
        find.text(L10n.t('setup_model_already', {'count': '1'})),
        findsNothing,
      );
      expect(find.text(L10n.t('setup_model_download_button')), findsOneWidget);
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
          authGateProvider.overrideWith((ref) => Stream.value(AuthGateInfo(gate))),
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
      expect(find.byKey(const Key('setup_next')), findsNothing,
          reason: 'no address yet: the connect screen owns the phone');
      await pumpMobileWizard(tester,
          api: _Api(), gate: AuthGateState.loginNeeded);
      expect(find.byKey(const Key('setup_next')), findsNothing,
          reason: 'signed out: the login gate owns the phone');
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
            await tester.tap(find.text(L10n.t('theme_dark')));
            await tester.pumpAndSettle();
          }
          Future<void> next() async {
            await tester.tap(find.byKey(const Key('setup_next')));
            await tester.pumpAndSettle();
            expect(tester.takeException(), isNull);
          }

          await next();
          final name = find.byType(TextField).first;
          await tester.enterText(name, 'Ada');
          await next();
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
          // Return through appearance and persona; the typed name survives.
          await tester.tap(find.byKey(const Key('setup_back')));
          await tester.pumpAndSettle();
          expect(find.text('Ada'), findsOneWidget);
          await next();
          await next();
          await next();
          await next();
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
