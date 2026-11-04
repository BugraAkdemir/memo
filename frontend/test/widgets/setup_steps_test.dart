import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/widgets/setup/embedding_step.dart';
import 'package:memo_flutter/widgets/setup/local_model_step.dart';
import 'package:memo_flutter/widgets/setup/mail_model_step.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/twin.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

Widget _host(Widget child) => MaterialApp(
  home: Scaffold(body: SingleChildScrollView(child: child)),
);

class _TwinApi extends MemoApiClient {
  _TwinApi() : super(baseUrl: 'http://memo.test');
  final calls = <String, Map<String, dynamic>>{};
  Map<String, dynamic> prefs = {
    'autonomy': {'enabled': false, 'mail': false, 'mode': ''},
    'timezone': 'UTC',
  };
  @override
  Future<TwinState> getTwinState() async =>
      TwinState(const [], prefs, const [], const [], const {});
  @override
  Future<dynamic> callTwin(String operation, Map<String, dynamic> args) async {
    calls[operation] = args;
    if (operation == 'preferences') prefs = Map.of(args);
    return {'ok': true};
  }
}

class _NoProviders extends ProviderListNotifier {
  @override
  Future<List<ProviderConfig>> build() async => [];
}

void main() {
  test('the time zone suggested for a device offset', () {
    expect(suggestedTimezone(const Duration(hours: 3)), 'Europe/Istanbul');
    expect(suggestedTimezone(Duration.zero), 'UTC');
    expect(suggestedTimezone(const Duration(hours: -5)), 'America/New_York');
    expect(suggestedTimezone(const Duration(hours: 7)), isNull);
  });

  testWidgets(
    'mail model step: picking a time zone saves it, drafts stay safe',
    (tester) async {
      tester.view.physicalSize = const Size(900, 1400);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final api = _TwinApi();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(api),
            providerListProvider.overrideWith(_NoProviders.new),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: SingleChildScrollView(
                child: SetupMailModelStep(
                  color: MemoTheme.light,
                  modelSet: false,
                  onModelChanged: (_) {},
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      // Drafts off by default; turning it on writes the safe mode only.
      expect(
        find.byKey(const Key('setup_mail_model_required')),
        findsOneWidget,
      );
      await tester.tap(find.byType(Switch).first);
      await tester.pumpAndSettle();
      final autonomy = api.calls['preferences']!['autonomy'] as Map;
      expect(autonomy['enabled'], true);
      expect(autonomy['mail'], true);
      expect(autonomy['mode'], 'draft_only');
      // Choose a time zone.
      await tester.tap(find.byKey(const ValueKey('setup_timezone_UTC')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Europe/Istanbul').last);
      await tester.pumpAndSettle();
      expect(api.calls['preferences']!['timezone'], 'Europe/Istanbul');
      // The rest of the preferences were kept.
      expect(
        (api.calls['preferences']!['autonomy'] as Map)['mode'],
        'draft_only',
      );
      expect(tester.takeException(), isNull);
    },
  );

  for (final locale in MemoLocale.values) {
    group('step widgets in $locale', () {
      setUp(() => L10n.setLocale(locale));
      tearDown(() => L10n.setLocale(MemoLocale.tr));

      testWidgets('local card: empty state asks for a model, store opens', (
        tester,
      ) async {
        var opened = 0;
        await tester.pumpWidget(
          _host(
            SetupLocalModelCard(
              color: MemoTheme.light,
              checking: false,
              chatModelCount: 0,
              onOpenStore: () => opened++,
            ),
          ),
        );
        expect(find.byKey(const Key('setup_local_empty')), findsOneWidget);
        expect(find.text(L10n.t('setup_local_none')), findsOneWidget);
        await tester.tap(find.byKey(const Key('setup_open_store')));
        expect(opened, 1);
      });

      testWidgets('local card: ready state names how many models', (
        tester,
      ) async {
        await tester.pumpWidget(
          _host(
            SetupLocalModelCard(
              color: MemoTheme.light,
              checking: false,
              chatModelCount: 2,
              onOpenStore: () {},
            ),
          ),
        );
        expect(find.byKey(const Key('setup_local_ready')), findsOneWidget);
        expect(
          find.text(L10n.t('setup_local_have', {'count': '2'})),
          findsOneWidget,
        );
      });

      testWidgets('embedding: missing model says what skipping costs', (
        tester,
      ) async {
        var downloads = 0, stores = 0;
        await tester.pumpWidget(
          _host(
            SetupEmbeddingStep(
              color: MemoTheme.light,
              hasModel: false,
              downloading: false,
              error: null,
              onDownloadRecommended: () => downloads++,
              onOpenStore: () => stores++,
            ),
          ),
        );
        expect(
          find.byKey(const Key('setup_embedding_skip_note')),
          findsOneWidget,
        );
        expect(find.text(L10n.t('setup_embedding_skip_note')), findsOneWidget);
        await tester.tap(find.byKey(const Key('setup_embedding_download')));
        await tester.tap(find.byKey(const Key('setup_embedding_store')));
        expect((downloads, stores), (1, 1));
      });

      testWidgets('embedding: ready model shows no warning and no buttons', (
        tester,
      ) async {
        await tester.pumpWidget(
          _host(
            SetupEmbeddingStep(
              color: MemoTheme.light,
              hasModel: true,
              downloading: false,
              error: null,
              onDownloadRecommended: () {},
              onOpenStore: () {},
            ),
          ),
        );
        expect(find.byKey(const Key('setup_embedding_ready')), findsOneWidget);
        expect(
          find.byKey(const Key('setup_embedding_skip_note')),
          findsNothing,
        );
        expect(find.byKey(const Key('setup_embedding_download')), findsNothing);
      });

      testWidgets('embedding: a download in progress disables both buttons', (
        tester,
      ) async {
        await tester.pumpWidget(
          _host(
            SetupEmbeddingStep(
              color: MemoTheme.light,
              hasModel: false,
              downloading: true,
              error: 'boom',
              onDownloadRecommended: () {},
              onOpenStore: () {},
            ),
          ),
        );
        expect(
          tester
              .widget<FilledButton>(
                find.byKey(const Key('setup_embedding_download')),
              )
              .onPressed,
          isNull,
        );
        expect(find.text('boom'), findsOneWidget);
      });
    });
  }
}
