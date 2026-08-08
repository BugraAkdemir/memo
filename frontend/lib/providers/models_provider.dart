import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import 'chat_provider.dart';

// ─── Local Models ───────────────────────────────────────────────

final localModelsProvider =
    AsyncNotifierProvider<LocalModelsNotifier, List<LocalModel>>(
        LocalModelsNotifier.new);

class LocalModelsNotifier extends AsyncNotifier<List<LocalModel>> {
  @override
  Future<List<LocalModel>> build() async {
    return ref.read(apiClientProvider).listLocalModels();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
        () => ref.read(apiClientProvider).listLocalModels());
  }

  Future<void> deleteModel(String path) async {
    try {
      await ref.read(apiClientProvider).deleteLocalModel(path);
      await refresh();
    } catch (e) {
      debugPrint('models: deleteModel error: $e');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Model silinemedi ($e)';
    }
  }
}

// ─── Model Status ───────────────────────────────────────────────

final modelStatusProvider = StreamProvider.autoDispose<ServerStatus>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  while (alive) {
    try {
      yield await api.getModelStatus();
    } catch (e) {
      debugPrint('models: modelStatus error: $e');
      yield const ServerStatus();
    }
    await Future.delayed(const Duration(seconds: 30));
  }
});

final embeddingStatusProvider = StreamProvider.autoDispose<ServerStatus>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  while (alive) {
    try {
      yield await api.getEmbeddingStatus();
    } catch (e) {
      debugPrint('models: embeddingStatus error: $e');
      yield const ServerStatus();
    }
    await Future.delayed(const Duration(seconds: 30));
  }
});

// ─── GPU Info ───────────────────────────────────────────────────

final gpuInfoProvider = FutureProvider<GPUInfo>((ref) async {
  try {
    return await ref.read(apiClientProvider).getGpuInfo();
  } catch (e) {
    debugPrint('models: gpuInfo error: $e');
    return const GPUInfo();
  }
});

// ─── Download Progress (adaptive polling) ───────────────────────
// autoDispose: stops polling when no screen is listening (e.g. model store
// closed). Adaptive: 1s while a download is active, 4s while idle — fixes the
// old "1 request/second forever" drain (KNOWN_ISSUES H16).

final downloadProgressProvider =
    StreamProvider.autoDispose<List<DownloadProgress>>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);

  while (alive) {
    var active = false;
    try {
      final progress = await api.getDownloadProgress();
      active = progress.any((p) => p.active);
      yield progress;
    } catch (e) {
      debugPrint('models: downloadProgress error: $e');
      yield const [];
    }
    await Future.delayed(Duration(seconds: active ? 1 : 4));
  }
});

// ─── HF Search ──────────────────────────────────────────────────

final modelSearchQueryProvider = StateProvider<String>((ref) => '');

final modelSearchResultsProvider =
    FutureProvider<List<HFModelResult>>((ref) async {
  final query = ref.watch(modelSearchQueryProvider);
  if (query.isEmpty) return [];
  return ref.read(apiClientProvider).searchModels(query);
});

// ─── Llama Installation ─────────────────────────────────────────

final llamaInstalledProvider = FutureProvider<bool>((ref) async {
  // No try/catch here on purpose: a thrown DioException (backend
  // unreachable) must surface as an AsyncError, not get coerced into
  // `false`. LlamaInstallerOverlay's error branch already renders nothing
  // for that case — if this swallowed the error into `false` instead, an
  // unreachable backend (e.g. a saved remote URL that's since gone dead)
  // would look identical to "llama.cpp genuinely isn't installed" and pop
  // the full-screen installer over live data, hiding the nav rail with no
  // way back to Settings to fix the URL.
  return ref.read(apiClientProvider).checkLlamaInstallation();
});
