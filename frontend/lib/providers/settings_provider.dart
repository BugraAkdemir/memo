import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:shared_preferences/shared_preferences.dart';

import '../core/l10n.dart';
import '../models/gpu_info.dart';
import 'chat_provider.dart';

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

// ─── Setup Complete ─────────────────────────────────────────────

final setupCompleteProvider =
    StateNotifierProvider<SetupCompleteNotifier, bool>((ref) {
      return SetupCompleteNotifier();
    });

class SetupCompleteNotifier extends StateNotifier<bool> {
  SetupCompleteNotifier() : super(false) {
    _init();
  }

  Future<void> _init() async {
    final prefs = await SharedPreferences.getInstance();
    state = prefs.getBool('memo_setup_complete') ?? false;
  }

  Future<void> completeSetup() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('memo_setup_complete', true);
    state = true;
  }

  Future<void> resetSetup() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('memo_setup_complete', false);
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
  } catch (_) {
    return false;
  }
});

final syncAccountProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getSyncAccount();
  } catch (_) {
    return {'authenticated': false};
  }
});

final syncSettingsProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getSyncSettings();
  } catch (_) {
    return {};
  }
});

// ─── Remote Access ──────────────────────────────────────────────

final remoteAccessProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  try {
    return await ref.read(apiClientProvider).getRemoteAccess();
  } catch (_) {
    return {'enabled': false};
  }
});

// ─── App Version ────────────────────────────────────────────────

final appVersionProvider = FutureProvider<String>((ref) async {
  try {
    return await ref.read(apiClientProvider).getVersion();
  } catch (_) {
    return 'unknown';
  }
});

// ─── Locale ─────────────────────────────────────────────────────

final localeProvider = StateNotifierProvider<LocaleNotifier, MemoLocale>(
  (ref) => LocaleNotifier(),
);

class LocaleNotifier extends StateNotifier<MemoLocale> {
  LocaleNotifier() : super(MemoLocale.tr) {
    _init();
  }

  Future<void> _init() async {
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString('memo_locale');
    final locale = saved == 'en' ? MemoLocale.en : MemoLocale.tr;
    L10n.setLocale(locale);
    state = locale;
  }

  Future<void> setLocale(MemoLocale locale) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('memo_locale', locale == MemoLocale.en ? 'en' : 'tr');
    L10n.setLocale(locale);
    state = locale;
  }
}
