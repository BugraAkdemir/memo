import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/gpu_info.dart';
import 'package:memo_flutter/models/local_model.dart';
import 'package:memo_flutter/models/orchestra_config.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/mood_provider.dart';
import 'package:memo_flutter/providers/orchestra_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/engine_strip.dart';

// Fake AsyncNotifiers that skip the real network call in build() — each
// mirrors the provider it replaces so EngineStrip's ref.watch calls resolve
// to a fixed value instead of hitting apiClientProvider (unconfigured in
// tests) and falling back to an AsyncError.
class _FakeMemoryEnabled extends MemoryEnabledNotifier {
  final bool value;
  _FakeMemoryEnabled(this.value);
  @override
  Future<bool> build() async => value;
}

class _FakeLocalModels extends LocalModelsNotifier {
  final List<LocalModel> value;
  _FakeLocalModels(this.value);
  @override
  Future<List<LocalModel>> build() async => value;
}

class _FakeActiveProvider extends ActiveProviderNotifier {
  @override
  Future<String> build() async => '';
}

class _FakeOrchestraConfig extends OrchestraConfigNotifier {
  @override
  Future<OrchestraConfig> build() async => const OrchestraConfig();
}

LocalModel _embeddingModel() => LocalModel.fromJson({
      'repo_id': 'nomic-ai/nomic-embed-text-v1.5-GGUF',
      'filename': 'nomic-embed-text-v1.5.Q4_K_M.gguf',
      'size': 84106624,
      'path': 'data/models/nomic-embed-text-v1.5.Q4_K_M.gguf',
      'is_embedding': true,
    });

/// Pumps EngineStrip at a desktop-realistic width. The default flutter_test
/// surface (800x600) is narrower than this strip's content in some states
/// (e.g. the memory warning + its action text), which overflows — a real
/// constraint of this Row-based layout, not a test bug, and not something
/// this batch of tests is trying to fix. Memo's app window is always
/// considerably wider than 800px in practice, so this matches how the
/// strip actually gets used rather than papering over the overflow.
Future<void> _pumpEngineStrip(
  WidgetTester tester, {
  required List<Override> overrides,
  VoidCallback? onOpenModels,
}) async {
  tester.view.physicalSize = const Size(1400, 800);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: overrides,
      child: MaterialApp(
        home: Scaffold(
          body: EngineStrip(onOpenModels: onOpenModels ?? () {}),
        ),
      ),
    ),
  );
}

void main() {
  // Baseline overrides shared by every case — only the ones a test cares
  // about get replaced, so each test stays focused on what it's checking.
  List<Override> baseline({
    ServerStatus chat = const ServerStatus(),
    ServerStatus embedding = const ServerStatus(),
    List<DownloadProgress> downloads = const [],
    bool memoryEnabled = false,
    List<LocalModel> localModels = const [],
  }) {
    return [
      modelStatusProvider.overrideWith((ref) => Stream.value(chat)),
      embeddingStatusProvider.overrideWith((ref) => Stream.value(embedding)),
      downloadProgressProvider.overrideWith((ref) => Stream.value(downloads)),
      moodEnabledProvider.overrideWith((ref) => Stream.value(false)),
      memoryEnabledProvider.overrideWith(() => _FakeMemoryEnabled(memoryEnabled)),
      localModelsProvider.overrideWith(() => _FakeLocalModels(localModels)),
      activeProviderTypeProvider.overrideWith(() => _FakeActiveProvider()),
      orchestraConfigProvider.overrideWith(() => _FakeOrchestraConfig()),
      // agentAutoPermissionProvider is intentionally left un-overridden: its
      // real notifier always attempts one API read from its constructor
      // regardless of subclassing, but swallows the failure and stays at its
      // default `false` — which is exactly what every case here wants.
    ];
  }

  testWidgets('shows the offline hint when nothing is running', (tester) async {
    await _pumpEngineStrip(tester, overrides: baseline());
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('engine_no_model')), findsOneWidget);
    expect(find.text(L10n.t('engine_memory_missing')), findsNothing);
  });

  testWidgets('shows the chat model filename when a local model is running',
      (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(
        chat: const ServerStatus(running: true, modelPath: '/models/gemma-3-4b.gguf'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('gemma-3-4b.gguf'), findsOneWidget);
    expect(find.text(L10n.t('engine_no_model')), findsNothing);
  });

  testWidgets('shows the embedding model filename when it is running',
      (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(
        embedding: const ServerStatus(running: true, modelPath: '/models/nomic.gguf'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('${L10n.t('engine_memory')}: nomic.gguf'), findsOneWidget);
  });

  testWidgets(
      'warns "no memory model" when memory is on, nothing running, and no '
      'embedding model is installed', (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(memoryEnabled: true, localModels: const []),
    );
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('engine_memory_missing')), findsOneWidget);
    expect(find.text(L10n.t('engine_memory_stopped')), findsNothing);
  });

  testWidgets(
      'warns "memory off" (not "no model") when an embedding model exists '
      'but just isn\'t running', (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(memoryEnabled: true, localModels: [_embeddingModel()]),
    );
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('engine_memory_stopped')), findsOneWidget);
    expect(find.text(L10n.t('engine_memory_missing')), findsNothing);
  });

  testWidgets('shows no memory warning when memory is disabled', (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(memoryEnabled: false, localModels: const []),
    );
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('engine_memory_missing')), findsNothing);
    expect(find.text(L10n.t('engine_memory_stopped')), findsNothing);
  });

  testWidgets(
      'shows a divider between the offline hint and the memory warning '
      '(regression: they used to render with no gap at all — "Start a "'
      'model●No memory model" stuck together — because the divider only '
      'checked chatRunning/isApiProvider, missing the offline-hint case)',
      (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(memoryEnabled: true, localModels: const []),
    );
    await tester.pumpAndSettle();

    // _divider() renders a 1x18 Container; the status dots are 7x7, so this
    // predicate can't accidentally match one of those instead.
    final dividers = find.byWidgetPredicate(
      (w) => w is Container && w.constraints == const BoxConstraints.tightFor(width: 1, height: 18),
    );
    expect(dividers, findsOneWidget);

    final hintRight = tester.getTopRight(find.text('· ${L10n.t('engine_start_model')}')).dx;
    final warningLeft = tester.getTopLeft(find.text(L10n.t('engine_memory_missing'))).dx;
    expect(warningLeft - hintRight, greaterThan(20));
  });

  testWidgets('tapping the memory warning opens the Models tab', (tester) async {
    var tapped = false;
    await _pumpEngineStrip(
      tester,
      overrides: baseline(memoryEnabled: true),
      onOpenModels: () => tapped = true,
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text(L10n.t('engine_memory_missing')));
    await tester.pump();

    expect(tapped, true);
  });

  testWidgets('shows filename and percent for a single active download',
      (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(downloads: [
        DownloadProgress.fromJson({
          'active': true,
          'repo_id': 'Qwen/Qwen2.5-7B-Instruct-GGUF',
          'filename': 'qwen2.5-7b.Q4_K_M.gguf',
          'percent': 42.0,
        }),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.text('qwen2.5-7b.Q4_K_M.gguf'), findsOneWidget);
    expect(find.text('42%'), findsOneWidget);
  });

  testWidgets(
      'collapses several active downloads into "downloading models" with the '
      'averaged percent', (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(downloads: [
        DownloadProgress.fromJson({
          'active': true,
          'repo_id': 'a/repo',
          'filename': 'a.gguf',
          'percent': 20.0,
        }),
        DownloadProgress.fromJson({
          'active': true,
          'repo_id': 'b/repo',
          'filename': 'b.gguf',
          'percent': 60.0,
        }),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.text(L10n.t('engine_downloading_models')), findsOneWidget);
    expect(find.text('40%'), findsOneWidget);
    expect(find.text('a.gguf'), findsNothing);
  });

  testWidgets('ignores a download entry that only carries an error (dismissed, not active)',
      (tester) async {
    await _pumpEngineStrip(
      tester,
      overrides: baseline(downloads: [
        DownloadProgress.fromJson({
          'active': true,
          'repo_id': 'a/repo',
          'filename': 'a.gguf',
          'percent': 10.0,
          'error': 'download request: context canceled',
        }),
      ]),
    );
    await tester.pumpAndSettle();

    // active:true but with an error is a failed/cancelled download, not a
    // live one — the ambient strip should stay quiet about it (the Model
    // Store's banner is where errors are surfaced, not this strip).
    expect(find.text(L10n.t('engine_downloading_models')), findsNothing);
    expect(find.text('a.gguf'), findsNothing);
    expect(find.text(L10n.t('engine_no_model')), findsOneWidget);
  });
}
