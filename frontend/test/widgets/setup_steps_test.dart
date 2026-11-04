import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/theme.dart';
import 'package:memo_flutter/widgets/setup/embedding_step.dart';
import 'package:memo_flutter/widgets/setup/local_model_step.dart';

Widget _host(Widget child) => MaterialApp(
  home: Scaffold(body: SingleChildScrollView(child: child)),
);

void main() {
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
