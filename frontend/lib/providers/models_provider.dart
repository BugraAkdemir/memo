import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

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
    await ref.read(apiClientProvider).deleteLocalModel(path);
    await refresh();
  }
}

// ─── Model Status ───────────────────────────────────────────────

final modelStatusProvider = FutureProvider<ServerStatus>((ref) async {
  return ref.read(apiClientProvider).getModelStatus();
});

final embeddingStatusProvider = FutureProvider<ServerStatus>((ref) async {
  return ref.read(apiClientProvider).getEmbeddingStatus();
});

// ─── GPU Info ───────────────────────────────────────────────────

final gpuInfoProvider = FutureProvider<GPUInfo>((ref) async {
  return ref.read(apiClientProvider).getGpuInfo();
});

// ─── Download Progress ──────────────────────────────────────────

final downloadProgressProvider = FutureProvider<DownloadProgress>((ref) async {
  return ref.read(apiClientProvider).getDownloadProgress();
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
  return ref.read(apiClientProvider).checkLlamaInstallation();
});
