import 'dart:async';

import 'package:dio/dio.dart';
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
  try {
    return await ref.read(apiClientProvider).getModelStatus();
  } catch (_) {
    return const ServerStatus();
  }
});

final embeddingStatusProvider = FutureProvider<ServerStatus>((ref) async {
  try {
    return await ref.read(apiClientProvider).getEmbeddingStatus();
  } catch (_) {
    return const ServerStatus();
  }
});

// ─── GPU Info ───────────────────────────────────────────────────

final gpuInfoProvider = FutureProvider<GPUInfo>((ref) async {
  try {
    return await ref.read(apiClientProvider).getGpuInfo();
  } catch (_) {
    return const GPUInfo(); // CPU fallback
  }
});

// ─── Download Progress (polling) ────────────────────────────────

final downloadProgressProvider = StreamProvider<DownloadProgress>((ref) async* {
  final api = ref.read(apiClientProvider);

  while (true) {
    try {
      final progress = await api.getDownloadProgress();
      yield progress;
      // Poll every second while active, every 3 seconds when idle
      await Future.delayed(progress.active
          ? const Duration(seconds: 1)
          : const Duration(seconds: 3));
    } catch (_) {
      yield const DownloadProgress();
      await Future.delayed(const Duration(seconds: 3));
    }
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
    // Only return true if it's truly unreachable (like backend down)
    // If we get a response but it's an error, something else is wrong.
    if (e is DioException && e.type == DioExceptionType.connectionError) {
      return true; 
    }
    // For other errors, assume not installed to be safe and show installer
    return false;
  }
});
