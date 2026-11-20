import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/local_model.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/widgets/model_config_dialog.dart';

class _StubLocalModelsNotifier extends LocalModelsNotifier {
  final List<LocalModel> _models;
  _StubLocalModelsNotifier(this._models);

  @override
  Future<List<LocalModel>> build() async => _models;
}

const _chatModel = LocalModel(
  repoId: 'LiquidAI/LFM2.5-24B-A2B-GGUF',
  filename: 'LFM2.5-24B-A2B-UD-IQ3_XXS.gguf',
  size: 8912057139,
  path: '/tmp/LFM2.5-24B-A2B-UD-IQ3_XXS.gguf',
  isEmbedding: false,
  supportsTools: true,
  maxContext: 131072,
);

const _embeddingModel = LocalModel(
  repoId: 'nomic-ai/nomic-embed-text-v1.5-GGUF',
  filename: 'nomic-embed-text-v1.5-very-long-quantization-name-Q8_0.gguf',
  size: 274000000,
  path: '/tmp/nomic-embed-text-v1.5.gguf',
  isEmbedding: true,
);

Future<void> _pumpDialog(
  WidgetTester tester, {
  required MemoLocale locale,
  required List<LocalModel> localModels,
  Size surfaceSize = const Size(1280, 800),
}) async {
  L10n.setLocale(locale);
  tester.view.physicalSize = surfaceSize;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });

  await tester.pumpWidget(
    ProviderScope(
      key: ValueKey('${locale.name}-${localModels.length}-${surfaceSize.width}'),
      overrides: [
        localModelsProvider.overrideWith(
          () => _StubLocalModelsNotifier(localModels),
        ),
      ],
      child: const MaterialApp(
        home: Scaffold(
          body: ModelConfigDialog(model: _chatModel),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  tearDown(() => L10n.setLocale(MemoLocale.en));

  testWidgets('no-embedding warning wraps cleanly without overflow in TR',
      (tester) async {
    await _pumpDialog(
      tester,
      locale: MemoLocale.tr,
      localModels: const [_chatModel],
    );

    expect(tester.takeException(), isNull);
    expect(find.text(L10n.t('no_embedding_model')), findsOneWidget);
  });

  testWidgets('no-embedding warning wraps cleanly on narrow screens in TR & EN',
      (tester) async {
    for (final locale in [MemoLocale.tr, MemoLocale.en]) {
      await _pumpDialog(
        tester,
        locale: locale,
        localModels: const [_chatModel],
        surfaceSize: const Size(360, 720),
      );

      expect(
        tester.takeException(),
        isNull,
        reason: 'ModelConfigDialog must not overflow at 360px in $locale',
      );
      expect(find.text(L10n.t('no_embedding_model')), findsOneWidget);
    }
  });

  testWidgets('embedding dropdown renders cleanly with long filenames',
      (tester) async {
    await _pumpDialog(
      tester,
      locale: MemoLocale.tr,
      localModels: const [_chatModel, _embeddingModel],
      surfaceSize: const Size(360, 720),
    );

    expect(tester.takeException(), isNull);
    expect(find.byType(DropdownButton<String>), findsOneWidget);
  });
}
