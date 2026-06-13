import 'dart:async';

import 'package:flutter/foundation.dart';
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

final modelStatusProvider = StreamProvider.autoDispose<ServerStatus>((ref) async* {
  final api = ref.read(apiClientProvider);
  while (true) {
    try {
      yield await api.getModelStatus();
    } catch (e) {
      debugPrint('models: modelStatus error: $e');
      yield const ServerStatus();
    }
    await Future.delayed(const Duration(seconds: 5));
  }
});

final embeddingStatusProvider = StreamProvider.autoDispose<ServerStatus>((ref) async* {
  final api = ref.read(apiClientProvider);
  while (true) {
    try {
      yield await api.getEmbeddingStatus();
    } catch (e) {
      debugPrint('models: embeddingStatus error: $e');
      yield const ServerStatus();
    }
    await Future.delayed(const Duration(seconds: 5));
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
    StreamProvider.autoDispose<DownloadProgress>((ref) async* {
  final api = ref.read(apiClientProvider);

  while (true) {
    var active = false;
    try {
      final progress = await api.getDownloadProgress();
      active = progress.active;
      yield progress;
    } catch (e) {
      debugPrint('models: downloadProgress error: $e');
      yield const DownloadProgress();
    }
    await Future.delayed(
        Duration(seconds: active ? 1 : 4));
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
  try {
    return await ref.read(apiClientProvider).checkLlamaInstallation();
  } catch (e) {
    // Connection error: backend unreachable — assume not installed
    // so the user sees the installer and can trigger setup.
    // Other errors: also return false to show installer.
    return false;
  }
});
