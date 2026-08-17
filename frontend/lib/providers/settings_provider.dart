import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import '../core/backend_url.dart';
import '../core/l10n.dart';
import '../models/dev_gateway.dart';
import '../models/gpu_info.dart';
import '../models/minimal_mode_overrides.dart';
import '../models/usage_stats.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

final prefsProvider = Provider<SharedPreferences>((ref) {
  throw UnimplementedError('Override prefsProvider in main()');
});

class MemorySettings {
  final int topK;
  final double minSimilarity;

  const MemorySettings({required this.topK, required this.minSimilarity});

  factory MemorySettings.fromJson(Map<String, dynamic> json) {
    final topKValue = json['top_k'] ?? json['TopK'];
    final minSimilarityValue = json['min_similarity'] ?? json['MinSimilarity'];

    return MemorySettings(
      topK: topKValue is num ? topKValue.toInt() : 5,
      minSimilarity: minSimilarityValue is num
          ? minSimilarityValue.toDouble()
          : 0.1,
    );
  }
}

/// Dream (periodic pinned-facts compression) settings — parsed from the same
/// GET /api/memory/settings response MemorySettings above reads, just
/// different fields on it. Kept as its own model/provider rather than
/// folded into MemorySettings since the Dream tab is a separate settings
/// surface with no need for topK/minSimilarity.
class DreamSettings {
  final bool enabled;
  final int initialDelayMinutes;
  final int intervalHours;

  const DreamSettings({
    required this.enabled,
    required this.initialDelayMinutes,
    required this.intervalHours,
  });

  factory DreamSettings.fromJson(Map<String, dynamic> json) {
    final enabledValue = json['dream_enabled'];
    final delayValue = json['dream_initial_delay_minutes'];
    final intervalValue = json['dream_interval_hours'];
    return DreamSettings(
      enabled: enabledValue is bool ? enabledValue : true,
      initialDelayMinutes: delayValue is num ? delayValue.toInt() : 5,
      intervalHours: intervalValue is num ? intervalValue.toInt() : 24,
    );
  }
}

class LlamaSettings {
  final String engineMode;
  final String binaryPath;
  final int port;
  final int ctxSize;
  final double temperature;
  final double topP;
  final int maxTokens;

  const LlamaSettings({
    required this.engineMode,
    required this.binaryPath,
    required this.port,
    required this.ctxSize,
    this.temperature = 0.7,
    this.topP = 0.9,
    this.maxTokens = 0,
  });

  factory LlamaSettings.fromJson(Map<String, dynamic> json) {
    return LlamaSettings(
      engineMode: json['engine_mode'] ?? 'auto',
      binaryPath: json['binary_path'] ?? '',
      port: json['port'] ?? 8081,
      ctxSize: json['ctx_size'] ?? 4096,
      temperature: (json['temperature'] as num?)?.toDouble() ?? 0.7,
      topP: (json['top_p'] as num?)?.toDouble() ?? 0.9,
      maxTokens: json['max_tokens'] as int? ?? 0,
    );
  }
}

final llamaSettingsProvider =
    AsyncNotifierProvider<LlamaSettingsNotifier, LlamaSettings>(
      LlamaSettingsNotifier.new,
    );

class LlamaSettingsNotifier extends AsyncNotifier<LlamaSettings> {
  @override
  Future<LlamaSettings> build() async {
    // BUG-ONB6 (see chat_provider.dart's ChatListNotifier for the full
    // story): a one-shot AsyncNotifier whose single build() attempt landing
    // while the auth gate is still up 401s and gets permanently cached as
    // an error. Mount safe defaults instead; app_shell.dart's gate-
    // transition listener re-invalidates this once the gate actually opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return LlamaSettings.fromJson(const {});
    }
    final data = await ref.read(apiClientProvider).getLlamaConfig();
    return LlamaSettings.fromJson(data);
  }

  Future<void> save(LlamaSettings settings) async {
    await ref
        .read(apiClientProvider)
        .updateLlamaConfig(
          engineMode: settings.engineMode,
          binaryPath: settings.binaryPath,
          port: settings.port,
          ctxSize: settings.ctxSize,
          temperature: settings.temperature,
          topP: settings.topP,
          maxTokens: settings.maxTokens,
        );
    state = AsyncData(settings);
  }
}

// ─── Setup Complete ─────────────────────────────────────────────

final setupCompleteProvider =
    StateNotifierProvider<SetupCompleteNotifier, bool>((ref) {
      final prefs = ref.read(prefsProvider);
      return SetupCompleteNotifier(prefs);
    });

class SetupCompleteNotifier extends StateNotifier<bool> {
  final SharedPreferences _prefs;

  SetupCompleteNotifier(this._prefs)
      : super(_prefs.getBool('memo_setup_complete') ?? false);

  Future<void> completeSetup() async {
    await _prefs.setBool('memo_setup_complete', true);
    state = true;
  }

  Future<void> resetSetup() async {
    await _prefs.setBool('memo_setup_complete', false);
    state = false;
  }
}

// ─── Launchpad Seen ─────────────────────────────────────────────

final launchpadSeenProvider =
    StateNotifierProvider<LaunchpadSeenNotifier, bool>((ref) {
      final prefs = ref.read(prefsProvider);
      return LaunchpadSeenNotifier(prefs);
    });

class LaunchpadSeenNotifier extends StateNotifier<bool> {
  final SharedPreferences _prefs;
  bool _forceShow = false;

  LaunchpadSeenNotifier(this._prefs)
      : super(_prefs.getBool('memo_launchpad_seen') ?? false);

  bool get forceShow => _forceShow;

  Future<void> markSeen() async {
    _forceShow = false;
    await _prefs.setBool('memo_launchpad_seen', true);
    state = true;
  }

  Future<void> reset() async {
    _forceShow = true;
    await _prefs.setBool('memo_launchpad_seen', false);
    state = false;
  }
}

// ─── Tour Seen ───────────────────────────────────────────────────

final tourSeenProvider =
    StateNotifierProvider<TourSeenNotifier, bool>((ref) {
      final prefs = ref.read(prefsProvider);
      return TourSeenNotifier(prefs);
    });

class TourSeenNotifier extends StateNotifier<bool> {
  final SharedPreferences _prefs;

  TourSeenNotifier(this._prefs)
      : super(_prefs.getBool('memo_tour_seen') ?? false);

  Future<void> markSeen() async {
    await _prefs.setBool('memo_tour_seen', true);
    state = true;
  }

  Future<void> resetTour() async {
    await _prefs.setBool('memo_tour_seen', false);
    state = false;
  }
}

// ─── System Prompt ──────────────────────────────────────────────

final systemPromptProvider =
    AsyncNotifierProvider<SystemPromptNotifier, String>(
      SystemPromptNotifier.new,
    );

class SystemPromptNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return '';
    return ref.read(apiClientProvider).getSystemPrompt();
  }

  Future<void> save(String prompt) async {
    await ref.read(apiClientProvider).setSystemPrompt(prompt);
    state = AsyncData(prompt);
  }

  Future<void> reset() async {
    await ref.read(apiClientProvider).resetSystemPrompt();
    ref.invalidateSelf();
  }
}

// ─── Incognito Prompt ───────────────────────────────────────────

final incognitoPromptProvider =
    AsyncNotifierProvider<IncognitoPromptNotifier, String>(
      IncognitoPromptNotifier.new,
    );

class IncognitoPromptNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
    return ref.read(apiClientProvider).getIncognitoPrompt();
  }

  Future<void> save(String prompt) async {
    await ref.read(apiClientProvider).setIncognitoPrompt(prompt);
    state = AsyncData(prompt);
  }
}

// ─── Memory Files ───────────────────────────────────────────────

final memoryFilesProvider =
    AsyncNotifierProvider<MemoryFilesNotifier, List<MemoryFileInfo>>(
      MemoryFilesNotifier.new,
    );

class MemoryFilesNotifier extends AsyncNotifier<List<MemoryFileInfo>> {
  @override
  Future<List<MemoryFileInfo>> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const [];
    return ref.read(apiClientProvider).listMemoryFiles();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).listMemoryFiles(),
    );
  }

  Future<void> deleteFile(String path) async {
    await ref.read(apiClientProvider).deleteMemoryFile(path);
    await refresh();
  }

  Future<void> clearAll() async {
    await ref.read(apiClientProvider).clearAllMemory();
    await refresh();
  }
}

final memorySettingsProvider =
    AsyncNotifierProvider<MemorySettingsNotifier, MemorySettings>(
      MemorySettingsNotifier.new,
    );

class MemorySettingsNotifier extends AsyncNotifier<MemorySettings> {
  @override
  Future<MemorySettings> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return MemorySettings.fromJson(const {});
    }
    final data = await ref.read(apiClientProvider).getMemorySettings();
    return MemorySettings.fromJson(data);
  }

  Future<void> save({required int topK, required double minSimilarity}) async {
    try {
      await ref
          .read(apiClientProvider)
          .updateMemorySettings(topK: topK, minSimilarity: minSimilarity);
      state = AsyncData(MemorySettings(topK: topK, minSimilarity: minSimilarity));
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Hafıza ayarları kaydedilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}

final dreamSettingsProvider =
    AsyncNotifierProvider<DreamSettingsNotifier, DreamSettings>(
      DreamSettingsNotifier.new,
    );

class DreamSettingsNotifier extends AsyncNotifier<DreamSettings> {
  @override
  Future<DreamSettings> build() async {
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return DreamSettings.fromJson(const {});
    }
    final data = await ref.read(apiClientProvider).getMemorySettings();
    return DreamSettings.fromJson(data);
  }

  // Deliberately lets errors propagate (unlike MemorySettingsNotifier.save
  // above, which swallows into errorMessageProvider) — DreamTab shows its
  // own inline save-result feedback, matching most other settings tabs.
  Future<void> save({
    required bool enabled,
    required int initialDelayMinutes,
    required int intervalHours,
  }) async {
    await ref
        .read(apiClientProvider)
        .setDreamSettings(
          enabled: enabled,
          initialDelayMinutes: initialDelayMinutes,
          intervalHours: intervalHours,
        );
    state = AsyncData(
      DreamSettings(
        enabled: enabled,
        initialDelayMinutes: initialDelayMinutes,
        intervalHours: intervalHours,
      ),
    );
  }
}

// ─── Usage stats ──────────────────────────────────────────────────

final usageStatsProvider =
    AsyncNotifierProvider<UsageStatsNotifier, UsageStatsSummary>(
      UsageStatsNotifier.new,
    );

class UsageStatsNotifier extends AsyncNotifier<UsageStatsSummary> {
  @override
  Future<UsageStatsSummary> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return UsageStatsSummary.fromJson(const {});
    }
    return ref.read(apiClientProvider).getUsageStats();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getUsageStats(),
    );
  }
}

// ─── Dev gateway ──────────────────────────────────────────────────

final devGatewayConfigProvider =
    AsyncNotifierProvider<DevGatewayConfigNotifier, DevGatewayConfig>(
      DevGatewayConfigNotifier.new,
    );

class DevGatewayConfigNotifier extends AsyncNotifier<DevGatewayConfig> {
  @override
  Future<DevGatewayConfig> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return const DevGatewayConfig();
    }
    return ref.read(apiClientProvider).getDevGatewayConfig();
  }

  Future<void> save({required bool requireAPIKey, required bool useMemory}) async {
    await ref
        .read(apiClientProvider)
        .setDevGatewayConfig(requireAPIKey: requireAPIKey, useMemory: useMemory);
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getDevGatewayConfig(),
    );
  }
}

final gatewayModelsProvider = FutureProvider<List<GatewayModel>>((ref) async {
  // BUG-ONB11: same trap as gpuInfoProvider (BUG-ONB5), and missed by the
  // BUG-ONB6 sweep because that audit searched for AsyncNotifier.build()
  // and this is a plain FutureProvider. DeveloperScreen lives in
  // AppShell's IndexedStack, so it is built at app start — its single
  // attempt lands while the auth gate is still up, 401s, and that error
  // is cached forever with no retry loop to recover it. Mount empty
  // instead; app_shell.dart's gate-transition listener refetches.
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const [];
  return ref.read(apiClientProvider).getGatewayModels();
});

/// Live log of /v1/messages requests, for the Developer screen. Polling is
/// explicitly started/stopped by AppShell based on which NavRail tab is
/// active (same pattern as WhatsAppStatusNotifier) — the Developer screen
/// stays mounted at all times inside the app's IndexedStack, so relying on
/// the widget's own initState/dispose would keep polling even while a
/// different tab is showing.
final gatewayLogsProvider =
    StateNotifierProvider<GatewayLogsNotifier, AsyncValue<List<GatewayLogEntry>>>(
  (ref) => GatewayLogsNotifier(ref.read(apiClientProvider)),
);

class GatewayLogsNotifier extends StateNotifier<AsyncValue<List<GatewayLogEntry>>> {
  final MemoApiClient _api;
  Timer? _pollTimer;

  GatewayLogsNotifier(this._api) : super(const AsyncValue.loading());

  Future<void> _fetch() async {
    try {
      final logs = await _api.getGatewayLogs();
      if (mounted) state = AsyncValue.data(logs);
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  void startPolling() {
    _pollTimer?.cancel();
    _fetch();
    _pollTimer = Timer.periodic(const Duration(seconds: 2), (_) => _fetch());
  }

  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }
}

// ─── Sync ───────────────────────────────────────────────────────

final syncAuthProvider = FutureProvider<bool>((ref) async {
  try {
    return await ref.read(apiClientProvider).checkSyncAuth();
  } catch (e) {
    ref.read(errorMessageProvider.notifier).state =
        '${L10n.t('error')}: Sync durumu alınamadı (${FriendlyError.describeGeneric(e)})';
    return false;
  }
});

final syncAccountProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getSyncAccount();
  } catch (e) {
    ref.read(errorMessageProvider.notifier).state =
        '${L10n.t('error')}: Sync hesap bilgisi alınamadı (${FriendlyError.describeGeneric(e)})';
    return {'authenticated': false};
  }
});

final syncSettingsProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getSyncSettings();
  } catch (e) {
    ref.read(errorMessageProvider.notifier).state =
        '${L10n.t('error')}: Sync ayarları alınamadı (${FriendlyError.describeGeneric(e)})';
    return {};
  }
});

// ─── Remote Access ──────────────────────────────────────────────

// Deliberately silent on failure, unlike most of this file's other
// FutureProviders: this one isn't scoped to a screen the user chose to
// open (Remote Access settings has its own explicit fetch/error handling)
// — app_shell.dart's _showSwarmNav() watches it on every rebuild just to
// gate a nav icon, so a dead/unreachable backend used to mean this rethrew
// into errorMessageProvider on every single rebuild, spamming a raw
// exception-dump SnackBar over the *entire* app for as long as the
// backend stayed down. Same quiet-default pattern gpuInfoProvider/
// downloadProgressProvider already use for the same reason.
final remoteAccessProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  // BUG-ONB11 family: another plain FutureProvider read during app start.
  // AppShell's nav rail watches this to decide whether the Swarm tab is
  // visible, so a 401 behind the auth gate used to cache {'enabled': false}
  // for the session — and because that fallback carries no 'beta' key, the
  // nav fell back to the local mirror pref forever, showing Swarm against a
  // backend with beta:false. Returning the same shape here would defeat
  // app_shell's containsKey('beta') check, so stay unresolved instead and
  // let the gate-transition listener refetch.
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
    return const <String, dynamic>{};
  }
  try {
    return await ref.read(apiClientProvider).getRemoteAccess();
  } catch (e) {
    return {'enabled': false};
  }
});

// ─── Memory Enabled ────────────────────────────────────────────

final memoryEnabledProvider =
    AsyncNotifierProvider<MemoryEnabledNotifier, bool>(
      MemoryEnabledNotifier.new,
    );

class MemoryEnabledNotifier extends AsyncNotifier<bool> {
  // Guards against a fast double-tap: toggle() used to read `state` (stale
  // until its own await finished) to compute the next value, so two calls
  // fired before the first one resolved both negated the same starting
  // value and landed on the *same* end state instead of toggling twice —
  // e.g. off→on, off→on again (net: on) instead of the intended net no-op.
  bool _toggling = false;

  @override
  Future<bool> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return true;
    return ref.read(apiClientProvider).getMemoryEnabled();
  }

  Future<void> toggle() async {
    if (_toggling) return;
    _toggling = true;
    final current = state.valueOrNull ?? true;
    final next = !current;
    // Optimistic update: applied before the network call, not after, so a
    // second toggle() (once _toggling allows it again) always negates the
    // value this call is actually settling on, not a stale one.
    state = AsyncData(next);
    try {
      await ref.read(apiClientProvider).setMemoryEnabled(next);
    } catch (e) {
      state = AsyncData(current);
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Hafıza durumu değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    } finally {
      _toggling = false;
    }
  }
}

// ─── Minimal Mode ──────────────────────────────────────────────
//
// When on, identity/persona/mood/web-search prompt injection is disabled
// entirely — only memory context (if separately enabled, see
// memoryEnabledProvider above) still reaches the model.

final minimalModeProvider =
    AsyncNotifierProvider<MinimalModeNotifier, bool>(
      MinimalModeNotifier.new,
    );

class MinimalModeNotifier extends AsyncNotifier<bool> {
  // See MemoryEnabledNotifier._toggling above — same fast-double-tap race.
  bool _toggling = false;

  @override
  Future<bool> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return false;
    return ref.read(apiClientProvider).getMinimalMode();
  }

  Future<void> toggle() async {
    if (_toggling) return;
    _toggling = true;
    final current = state.valueOrNull ?? false;
    final next = !current;
    state = AsyncData(next);
    try {
      await ref.read(apiClientProvider).setMinimalMode(next);
    } catch (e) {
      state = AsyncData(current);
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Minimal mod değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    } finally {
      _toggling = false;
    }
  }
}

// ─── Minimal Mode granular overrides ───────────────────────────
//
// Shown as a dropdown under the Minimal Mode toggle: lets a user keep
// Minimal Mode's overall "strip everything" intent while selectively
// re-enabling one category (e.g. keep ambient proactive nudging off but the
// persona/system prompt on) instead of the only alternative being to turn
// Minimal Mode off entirely and get everything back at once.

final minimalModeOverridesProvider = AsyncNotifierProvider<
    MinimalModeOverridesNotifier, MinimalModeOverrides>(
  MinimalModeOverridesNotifier.new,
);

class MinimalModeOverridesNotifier extends AsyncNotifier<MinimalModeOverrides> {
  // See MemoryEnabledNotifier._toggling above — same fast-double-tap race,
  // one flag shared across all four checkboxes since they save together.
  bool _saving = false;

  @override
  Future<MinimalModeOverrides> build() async {
    // BUG-ONB6: see LlamaSettingsNotifier's comment above.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return MinimalModeOverrides.allOff;
    }
    return ref.read(apiClientProvider).getMinimalModeOverrides();
  }

  Future<void> save(MinimalModeOverrides next) async {
    if (_saving) return;
    _saving = true;
    final previous = state.valueOrNull ?? MinimalModeOverrides.allOff;
    state = AsyncData(next);
    try {
      await ref.read(apiClientProvider).setMinimalModeOverrides(next);
    } catch (e) {
      state = AsyncData(previous);
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Minimal mod ayarları değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    } finally {
      _saving = false;
    }
  }
}

// ─── App Version ────────────────────────────────────────────────

final appVersionProvider = FutureProvider<String>((ref) async {
  try {
    return await ref.read(apiClientProvider).getVersion();
  } catch (e) {
    ref.read(errorMessageProvider.notifier).state =
        '${L10n.t('error')}: Uygulama sürümü alınamadı (${FriendlyError.describeGeneric(e)})';
    return 'unknown';
  }
});

// ─── Theme Mode ─────────────────────────────────────────────────

final themeModeProvider = StateNotifierProvider<ThemeModeNotifier, String>(
  (ref) {
    final prefs = ref.read(prefsProvider);
    return ThemeModeNotifier(prefs);
  },
);

class ThemeModeNotifier extends StateNotifier<String> {
  final SharedPreferences _prefs;

  ThemeModeNotifier(this._prefs)
      : super(_prefs.getString('memo_theme_mode') ?? 'light');

  Future<void> setMode(String mode) async {
    await _prefs.setString('memo_theme_mode', mode);
    state = mode;
  }
}

// ─── Streaming Enabled ─────────────────────────────────────────

final streamingEnabledProvider =
    StateNotifierProvider<StreamingEnabledNotifier, bool>(
  (ref) {
    final prefs = ref.read(prefsProvider);
    return StreamingEnabledNotifier(prefs);
  },
);

class StreamingEnabledNotifier extends StateNotifier<bool> {
  final SharedPreferences _prefs;

  StreamingEnabledNotifier(this._prefs)
      : super(_prefs.getBool('memo_streaming') ?? true);

  Future<void> toggle() async {
    final next = !state;
    await _prefs.setBool('memo_streaming', next);
    state = next;
  }
}

// ─── Beta Features ─────────────────────────────────────────────

final betaFeaturesProvider = StateNotifierProvider<BetaFeaturesNotifier, bool>(
  (ref) {
    final prefs = ref.read(prefsProvider);
    return BetaFeaturesNotifier(prefs);
  },
);

class BetaFeaturesNotifier extends StateNotifier<bool> {
  final SharedPreferences _prefs;

  BetaFeaturesNotifier(this._prefs)
      : super(_prefs.getBool('memo_beta_features') ?? false);

  Future<void> toggle() async {
    await setEnabled(!state);
  }

  /// Set beta enabled and persist locally. Prefer calling this after a
  /// successful backend `setBeta` so prefs never disagree with `cfg.Beta`.
  Future<void> setEnabled(bool enabled) async {
    await _prefs.setBool('memo_beta_features', enabled);
    state = enabled;
  }
}

// ─── Locale ─────────────────────────────────────────────────────

final localeProvider = StateNotifierProvider<LocaleNotifier, MemoLocale>(
  (ref) {
    final prefs = ref.read(prefsProvider);
    final apiClient = ref.read(apiClientProvider);
    return LocaleNotifier(prefs, apiClient);
  },
);

class LocaleNotifier extends StateNotifier<MemoLocale> {
  final SharedPreferences _prefs;
  final MemoApiClient _apiClient;

  LocaleNotifier(this._prefs, this._apiClient) : super(_initLocale(_prefs));

  static MemoLocale _initLocale(SharedPreferences prefs) {
    final saved = prefs.getString('memo_locale');
    // Only an explicit 'tr' selects Turkish — an unset key falls through
    // to English, matching L10n's own default (flipped 2026-08-13; see
    // the doc comment there). A user who already chose Turkish keeps it.
    final locale = saved == 'tr' ? MemoLocale.tr : MemoLocale.en;
    L10n.setLocale(locale);
    return locale;
  }

  Future<void> setLocale(MemoLocale locale) async {
    await _prefs.setString('memo_locale', locale == MemoLocale.en ? 'en' : 'tr');
    L10n.setLocale(locale);
    state = locale;
    // Best-effort, fire-and-forget: the backend has no locale of its own —
    // this is purely so a second client with no SharedPreferences (the
    // terminal REPL) can follow along. An unreachable backend must never
    // block or fail the GUI's own language switch.
    unawaited(
      _apiClient
          .setUILanguage(locale == MemoLocale.en ? 'en' : 'tr')
          .catchError((_) {}),
    );
  }
}

// ─── Backend URL ────────────────────────────────────────────────

final backendUrlProvider = StateNotifierProvider<BackendUrlNotifier, String>(
  (ref) {
    final prefs = ref.read(prefsProvider);
    return BackendUrlNotifier(prefs);
  },
);

class BackendUrlNotifier extends StateNotifier<String> {
  final SharedPreferences _prefs;

  // Runs whatever was already on disk through normalizeBackendUrl() too —
  // a value saved before that function existed (e.g. a bare "127.0.0.1"
  // with no scheme) would otherwise keep crashing the whole app on every
  // launch forever, since MemoApiClient's Dio instance validates baseUrl
  // eagerly and there's no UI screen reachable before that construction
  // runs.
  BackendUrlNotifier(this._prefs)
      : super(normalizeBackendUrl(_prefs.getString('memo_api_base_url') ?? ''));

  Future<void> save(String url) async {
    final trimmed = url.trim();
    if (trimmed.isEmpty) {
      // Reset to default
      await _prefs.remove('memo_api_base_url');
      state = 'http://127.0.0.1:8090';
      return;
    }
    // normalizeBackendUrl adds a scheme/port if missing (e.g. "127.0.0.1"
    // -> "http://127.0.0.1:8090") and strips a trailing slash.
    final normalized = normalizeBackendUrl(trimmed);
    await _prefs.setString('memo_api_base_url', normalized);
    state = normalized;
  }
}

// ─── Backend Token ──────────────────────────────────────────────
//
// Manual counterpart to apiClientProvider's onRemoteTokenLearned
// (chat_provider.dart): that callback only ever *learns* a token from a
// backend this app spawned itself and later exposed via LAN mode. Pointing
// the Backend URL above at a foreign backend that requires the token from
// the very first request (e.g. a headless `--lan` container) has no such
// bootstrap moment — the token has to be entered by hand at least once.
// Same SharedPreferences key as apiClientProvider's _remoteTokenPrefsKey
// (not imported directly to avoid a chat_provider.dart <-> settings_provider.dart
// cycle — chat_provider.dart already imports this file).
const _backendTokenPrefsKey = 'memo_remote_access_token';

final backendTokenProvider =
    StateNotifierProvider<BackendTokenNotifier, String>((ref) {
  final prefs = ref.read(prefsProvider);
  return BackendTokenNotifier(prefs);
});

class BackendTokenNotifier extends StateNotifier<String> {
  final SharedPreferences _prefs;

  BackendTokenNotifier(this._prefs)
      : super(_prefs.getString(_backendTokenPrefsKey) ?? '');

  Future<void> save(String token) async {
    final trimmed = token.trim();
    if (trimmed.isEmpty) {
      await _prefs.remove(_backendTokenPrefsKey);
      state = '';
    } else {
      await _prefs.setString(_backendTokenPrefsKey, trimmed);
      state = trimmed;
    }
  }
}
