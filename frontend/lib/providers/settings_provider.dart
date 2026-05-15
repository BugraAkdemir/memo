import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:shared_preferences/shared_preferences.dart';

import '../core/l10n.dart';
import '../models/gpu_info.dart';
import 'chat_provider.dart';

// ─── Setup Complete ─────────────────────────────────────────────

final setupCompleteProvider = StateNotifierProvider<SetupCompleteNotifier, bool>((ref) {
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
        SystemPromptNotifier.new);

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
        IncognitoPromptNotifier.new);

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
        MemoryFilesNotifier.new);

class MemoryFilesNotifier extends AsyncNotifier<List<MemoryFileInfo>> {
  @override
  Future<List<MemoryFileInfo>> build() async {
    return ref.read(apiClientProvider).listMemoryFiles();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
        () => ref.read(apiClientProvider).listMemoryFiles());
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

// ─── Sync ───────────────────────────────────────────────────────

final syncAuthProvider = FutureProvider<bool>((ref) async {
  return ref.read(apiClientProvider).checkSyncAuth();
});

final syncAccountProvider =
    FutureProvider<Map<String, dynamic>>((ref) async {
  return ref.read(apiClientProvider).getSyncAccount();
});

final syncSettingsProvider =
    FutureProvider<Map<String, dynamic>>((ref) async {
  return ref.read(apiClientProvider).getSyncSettings();
});

// ─── Remote Access ──────────────────────────────────────────────

final remoteAccessProvider =
    FutureProvider<Map<String, dynamic>>((ref) async {
  return ref.read(apiClientProvider).getRemoteAccess();
});

// ─── App Version ────────────────────────────────────────────────

final appVersionProvider = FutureProvider<String>((ref) async {
  return ref.read(apiClientProvider).getVersion();
});

// ─── Locale ─────────────────────────────────────────────────────

final localeProvider =
    StateNotifierProvider<LocaleNotifier, MemoLocale>(
        (ref) => LocaleNotifier());

class LocaleNotifier extends StateNotifier<MemoLocale> {
  LocaleNotifier() : super(MemoLocale.tr);

  void setLocale(MemoLocale locale) {
    L10n.setLocale(locale);
    state = locale;
  }
}
