import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../core/l10n.dart';
import '../models/provider_config.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

/// Provider list provider.
final providerListProvider =
    AsyncNotifierProvider<ProviderListNotifier, List<ProviderConfig>>(
      ProviderListNotifier.new,
    );

class ProviderListNotifier extends AsyncNotifier<List<ProviderConfig>> {
  @override
  Future<List<ProviderConfig>> build() async {
    // BUG-ONB6 (see chat_provider.dart's ChatListNotifier for the full
    // story): a one-shot AsyncNotifier whose single build() attempt landing
    // while the auth gate is still up 401s and gets permanently cached as
    // an error. Mount empty instead; app_shell.dart's gate-transition
    // listener re-invalidates this once the gate actually opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const [];
    return ref.read(apiClientProvider).getProviders();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getProviders(),
    );
  }

  Future<void> updateProvider(ProviderConfig config) async {
    try {
      await ref.read(apiClientProvider).updateProvider(config);
      await refresh();
      ref.invalidate(activeProviderTypeProvider);
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Sağlayıcı kaydedilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> deleteProvider(String type, {String? name}) async {
    try {
      await ref.read(apiClientProvider).deleteProvider(type, name: name);
      await refresh();
      ref.invalidate(activeProviderTypeProvider);
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Sağlayıcı silinemedi (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<Map<String, dynamic>> testProvider(ProviderConfig config) async {
    try {
      return await ref.read(apiClientProvider).testProvider(config);
    } catch (e) {
      debugPrint('provider: test error: ${FriendlyError.describeGeneric(e)}');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Sağlayıcı test edilemedi (${FriendlyError.describeGeneric(e)})';
      return {'connected': false, 'error': e.toString()};
    }
  }
}

/// Which reasoning-effort labels a given (provider type, model) accepts —
/// same lookup provider_config_dialog.dart's _loadEffortLevels does by hand,
/// but keyed/cached by Riverpod so the chat screen's quick-select (which
/// rebuilds far more often, on every active-provider/provider-list change)
/// doesn't refire the network call on every rebuild. model only matters for
/// OpenRouter (see MemoApiClient.getEffortLevels); pass '' for every other
/// type.
final effortLevelsProvider = FutureProvider.family<List<String>, (String type, String model)>((
  ref,
  key,
) async {
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const [];
  try {
    return await ref
        .read(apiClientProvider)
        .getEffortLevels(key.$1, model: key.$2.isEmpty ? null : key.$2);
  } catch (_) {
    // Ambient/background lookup behind a chat-screen widget the user didn't
    // explicitly open — same silence rationale as ActiveProviderNotifier
    // below: surfacing a raw exception here would spam errorMessageProvider
    // on every unreachable-backend rebuild. The widget just hides itself.
    return const [];
  }
});

/// Active provider provider (yes, the name).
final activeProviderTypeProvider =
    AsyncNotifierProvider<ActiveProviderNotifier, String>(
      ActiveProviderNotifier.new,
    );

class ActiveProviderNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
    // Deliberately silent on failure: this is watched ambiently from the
    // main chat top bar and the always-visible engine strip
    // (chat_screen.dart, engine_strip.dart), not a screen the user chose to
    // open — a dead/unreachable backend used to rethrow into
    // errorMessageProvider on every rebuild, spamming a raw exception-dump
    // SnackBar over the whole app for as long as the backend stayed down
    // (same root cause and fix as remoteAccessProvider, see its own
    // comment). BackendUnreachableOverlay is what actually tells the user
    // the backend is down now — this provider doesn't need to duplicate
    // that.
    // BUG-ONB6: unlike the providers this bug hit visibly (see
    // chat_provider.dart's ChatListNotifier), this one already degraded
    // gracefully on any error — but without a gate check it still never
    // refreshed to the real value after a blocked first attempt except by
    // luck (some other rebuild). app_shell.dart's gate-transition listener
    // invalidates this like the others once the gate actually opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return '';
    try {
      return await ref.read(apiClientProvider).getActiveProvider();
    } catch (e) {
      debugPrint('provider: getActiveProvider error: ${FriendlyError.describeGeneric(e)}');
      return '';
    }
  }

  Future<void> setActive(String type) async {
    try {
      await ref.read(apiClientProvider).setActiveProvider(type);
      state = AsyncData(type);
      ref.invalidate(providerListProvider);
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Aktif sağlayıcı değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}

/// Returns the asset path for a provider logo, or null if unknown.
String? _providerAssetPath(String type) {
  switch (type) {
    case 'openai':
      return 'lib/icon/OpenAI_Symbol_0.svg';
    case 'gemini':
      return 'lib/icon/Google_Symbol_0.svg';
    case 'grok':
      return 'lib/icon/XAL.svg';
    case 'claude':
      return 'lib/icon/Claude_Symbol_1.png';
    case 'openrouter':
      return 'lib/icon/openrouter.jpeg';
    case 'ollama':
      return 'lib/icon/ollama.png';
    case 'groq':
      return 'lib/icon/groq.png';
    default:
      return null;
  }
}

/// Widget that renders the provider's actual logo image.
/// Falls back to a generic cloud icon if no logo is registered.
Widget providerLogoWidget(String type, {double size = 18}) {
  final path = _providerAssetPath(type);
  if (path == null) {
    return const Icon(Icons.cloud_outlined, size: 18);
  }
  if (path.endsWith('.svg')) {
    return SvgPicture.asset(path, width: size, height: size);
  }
  return Image.asset(path, width: size, height: size, fit: BoxFit.contain);
}

/// Legacy text-symbol helper — kept for text-only contexts (dropdowns, labels).
String providerIcon(String type) {
  switch (type) {
    case 'openai':
      return '\u25CB'; // ○
    case 'gemini':
      return '\u25C6'; // ◆
    case 'grok':
      return '\u2715'; // ✕
    case 'groq':
      return '\u26A1'; // ⚡
    case 'claude':
      return '\u25A0'; // ■
    case 'openrouter':
      return '\u2194'; // ↔
    case 'ollama':
      return '\u25B3'; // △
    case 'opencode-zen':
      return '\u26A1'; // ⚡
    case 'opencode-go':
      return '\u25B8'; // ▸
    default:
      return '\u2601'; // ☁
  }
}
