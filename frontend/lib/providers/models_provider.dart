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

final modelStatusProvider = StreamProvider<ServerStatus>((ref) async* {
  while (true) {
    try {
      yield await ref.read(apiClientProvider).getModelStatus();
    } catch (_) {
      yield const ServerStatus();
    }
    await Future.delayed(const Duration(seconds: 5));
  }
});

final embeddingStatusProvider = StreamProvider<ServerStatus>((ref) async* {
  while (true) {
    try {
      yield await ref.read(apiClientProvider).getEmbeddingStatus();
    } catch (_) {
      yield const ServerStatus();
    }
    await Future.delayed(const Duration(seconds: 5));
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
    } catch (_) {
      yield const DownloadProgress();
    }
    await Future.delayed(const Duration(seconds: 1));
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
