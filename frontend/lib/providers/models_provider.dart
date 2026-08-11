import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import 'auth_gate_provider.dart';
import '../core/friendly_error.dart';

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
      debugPrint('models: deleteModel error: ${FriendlyError.describeGeneric(e)}');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Model silinemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}

// ─── Model Status ───────────────────────────────────────────────

final modelStatusProvider = StreamProvider.autoDispose<ServerStatus>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  while (alive) {
    // BUG-ONB4: no point polling a token-gated backend while the login gate
    // is up — every tick would 401. Re-check every 5s until the gate opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      yield const ServerStatus();
      await cancellablePause(ref, const Duration(seconds: 5));
      continue;
    }
    try {
      yield await api.getModelStatus();
    } catch (e) {
      debugPrint('models: modelStatus error: ${FriendlyError.describeGeneric(e)}');
      yield const ServerStatus();
    }
    await cancellablePause(ref, const Duration(seconds: 30));
  }
});

final embeddingStatusProvider = StreamProvider.autoDispose<ServerStatus>((ref) async* {
  var alive = true;
  ref.onDispose(() => alive = false);
  final api = ref.read(apiClientProvider);
  while (alive) {
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      yield const ServerStatus();
      await cancellablePause(ref, const Duration(seconds: 5));
      continue;
    }
    try {
      yield await api.getEmbeddingStatus();
    } catch (e) {
      debugPrint('models: embeddingStatus error: ${FriendlyError.describeGeneric(e)}');
      yield const ServerStatus();
    }
    await cancellablePause(ref, const Duration(seconds: 30));
  }
});

// ─── GPU Info ───────────────────────────────────────────────────

// BUG-ONB5: on a token/password-gated backend, this used to fire once via
// a plain ref.read with no gate check at all — the one provider BUG-ONB4's
// fix missed, since that fix targeted the StreamProvider polling loops
// above and this is a one-shot FutureProvider with no retry loop of its
// own. If that single attempt landed while the login gate was still up,
// getGpuInfo() 401'd, the catch below silently swallowed it into GPUInfo()
// (ramTotalMb: 0), and — because a FutureProvider only ever runs its
// builder once and caches the result — every screen reading RAM/GPU (setup
// wizard's model recommendation, Model Store's hardware-fit badges) stayed
// stuck on "unknown hardware" for the rest of the session, recommending/
// fitting models as if RAM were 0 instead of the real value. A manual
// refresh "fixed" it only because reloading the app rebuilds the whole
// provider graph from scratch, by which point the gate had already opened.
//
// Fixed in two parts: (1) a gate check here, matching the polling
// providers above, so a blocked gate never even attempts the request
// (avoids the 401 in the first place — same reasoning as BUG-ONB4); (2)
// auth_gate_overlay.dart's four successful-login paths now also
// ref.invalidate(gpuInfoProvider) right alongside their existing
// ref.invalidate(authGateProvider) — since this provider isn't watched
// reactively (a plain, not autoDispose, FutureProvider watching an
// autoDispose StreamProvider deadlocks container.read(....future) — tried
// and reverted, see this commit's message), the only way it learns the
// gate has actually opened is being told to recompute explicitly at the
// one moment that matters: right after login succeeds.
final gpuInfoProvider = FutureProvider<GPUInfo>((ref) async {
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
    return const GPUInfo();
  }
  try {
    return await ref.read(apiClientProvider).getGpuInfo();
  } catch (e) {
    debugPrint('models: gpuInfo error: ${FriendlyError.describeGeneric(e)}');
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
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      yield const [];
      await cancellablePause(ref, const Duration(seconds: 5));
      continue;
    }
    try {
      final progress = await api.getDownloadProgress();
      active = progress.any((p) => p.active);
      yield progress;
    } catch (e) {
      debugPrint('models: downloadProgress error: ${FriendlyError.describeGeneric(e)}');
      yield const [];
    }
    await cancellablePause(ref, Duration(seconds: active ? 1 : 4));
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
