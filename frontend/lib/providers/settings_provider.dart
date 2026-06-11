import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:shared_preferences/shared_preferences.dart';

import '../core/l10n.dart';
import '../models/gpu_info.dart';
import 'chat_provider.dart';

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

// ─── System Prompt ──────────────────────────────────────────────

final systemPromptProvider =
    AsyncNotifierProvider<SystemPromptNotifier, String>(
      SystemPromptNotifier.new,
    );

class SystemPromptNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
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
    final data = await ref.read(apiClientProvider).getMemorySettings();
    return MemorySettings.fromJson(data);
  }

  Future<void> save({required int topK, required double minSimilarity}) async {
    await ref
        .read(apiClientProvider)
        .updateMemorySettings(topK: topK, minSimilarity: minSimilarity);
    state = AsyncData(MemorySettings(topK: topK, minSimilarity: minSimilarity));
  }
}

// ─── Sync ───────────────────────────────────────────────────────

final syncAuthProvider = FutureProvider<bool>((ref) async {
  try {
    return await ref.read(apiClientProvider).checkSyncAuth();
  } catch (e) {
    return false;
  }
});

final syncAccountProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getSyncAccount();
  } catch (e) {
    return {'authenticated': false};
  }
});

final syncSettingsProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getSyncSettings();
  } catch (e) {
    return {};
  }
});

// ─── Remote Access ──────────────────────────────────────────────

final remoteAccessProvider = FutureProvider<Map<String, dynamic>>((ref) async {
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
  @override
  Future<bool> build() async {
    return ref.read(apiClientProvider).getMemoryEnabled();
  }

  Future<void> toggle() async {
    final current = state.valueOrNull ?? true;
    final next = !current;
    await ref.read(apiClientProvider).setMemoryEnabled(next);
    state = AsyncData(next);
  }
}

// ─── App Version ────────────────────────────────────────────────

final appVersionProvider = FutureProvider<String>((ref) async {
  try {
    return await ref.read(apiClientProvider).getVersion();
  } catch (e) {
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
      : super(_prefs.getString('memo_theme_mode') ?? 'system');

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
    final next = !state;
    await _prefs.setBool('memo_beta_features', next);
    state = next;
  }
}

// ─── Locale ─────────────────────────────────────────────────────

final localeProvider = StateNotifierProvider<LocaleNotifier, MemoLocale>(
  (ref) {
    final prefs = ref.read(prefsProvider);
    return LocaleNotifier(prefs);
  },
);

class LocaleNotifier extends StateNotifier<MemoLocale> {
  final SharedPreferences _prefs;

  LocaleNotifier(this._prefs) : super(_initLocale(_prefs));

  static MemoLocale _initLocale(SharedPreferences prefs) {
    final saved = prefs.getString('memo_locale');
    final locale = saved == 'en' ? MemoLocale.en : MemoLocale.tr;
    L10n.setLocale(locale);
    return locale;
  }

  Future<void> setLocale(MemoLocale locale) async {
    await _prefs.setString('memo_locale', locale == MemoLocale.en ? 'en' : 'tr');
    L10n.setLocale(locale);
    state = locale;
  }
}
