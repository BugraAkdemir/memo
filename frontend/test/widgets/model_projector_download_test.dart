import 'dart:typed_data';

import 'package:dio/dio.dart';
// Match the app's supported SDK; newer CI Flutter deprecates this import.
// ignore: deprecated_member_use
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/curated_models.dart';
import 'package:memo_flutter/models/gpu_info.dart';
import 'package:memo_flutter/models/local_model.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/screens/model_store/discover_item.dart';
import 'package:memo_flutter/screens/model_store/model_detail_panel.dart';

class _OfflineAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString('{}', 404);
  @override
  void close({bool force = false}) {}
}

class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://fixture.invalid') {
    dio.httpClientAdapter = _OfflineAdapter();
  }
  final downloads = <String>[];
  @override
  Future<List<GGUFFile>> getModelFiles(String repoId) async => [
    GGUFFile(filename: 'model.Q4_K_M.gguf', size: 5),
    GGUFFile(filename: 'mmproj-F16.gguf', size: 4),
  ];
  @override
  Future<void> downloadModel(
    String repoId,
    String filename, {
    int expectedSize = 0,
  }) async {
    downloads.add('$repoId/$filename');
  }
}

class _Models extends LocalModelsNotifier {
  _Models(this.vision);
  final bool vision;
  @override
  Future<List<LocalModel>> build() async => [
    LocalModel(
      repoId: 'acme/vision',
      filename: 'model.Q4_K_M.gguf',
      size: 5,
      path: '/fixture/model.Q4_K_M.gguf',
      isEmbedding: false,
      isVision: vision,
    ),
  ];
}

void main() {
  for (final locale in [MemoLocale.en, MemoLocale.tr]) {
    testWidgets('repair missing projector from the model panel in $locale', (
      tester,
    ) async {
      final previous = L10n.locale;
      L10n.setLocale(locale);
      addTearDown(() => L10n.setLocale(previous));
      tester.view.reset();
      tester.view.physicalSize = const Size(1400, 1000);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final api = _Api();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(api),
            localModelsProvider.overrideWith(() => _Models(false)),
            gpuInfoProvider.overrideWith((_) async => const GPUInfo()),
            llamaInstalledProvider.overrideWith((_) async => true),
            downloadProgressProvider.overrideWith((_) => Stream.value([])),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: ModelDetailPanel(
                item: DiscoverItem.fromCurated(
                  const CuratedModel(
                    repoId: 'acme/vision',
                    name: 'Vision fixture',
                    descTr: '',
                    descEn: '',
                    approxBytes: 5,
                    kind: ModelKind.vision,
                    supportsTools: true,
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final repair = find.text(L10n.t('download_missing_projector'));
      expect(repair, findsOneWidget);
      await tester.ensureVisible(repair);
      await tester.tap(repair);
      await tester.pump();
      expect(api.downloads, ['acme/vision/model.Q4_K_M.gguf']);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    });
  }
}
