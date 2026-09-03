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
/// the live-discovered types (openrouter/claude/gemini/ollama — see
/// MemoApiClient.getEffortLevels); every other type ignores it and returns
/// empty regardless, since there's no known capability signal to check.
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
    case 'opencode-zen':
    case 'opencode-go':
      // Same OpenCode brand mark for both — they're the same product's two
      // billing tiers (pay-as-you-go vs. subscription), not different
      // services. Source: Simple Icons (cdn.simpleicons.org/opencode).
      return 'lib/icon/opencode.svg';
    case 'kilo':
      // Source: kilo.ai's own favicon (kilo.ai/favicon/favicon.svg).
      return 'lib/icon/kilo.svg';
    default:
      return null;
  }
}

/// Widget that renders the provider's actual logo image. Falls back to a
/// fitting generic icon (not a brand logo, since these three types aren't
/// external branded services) for local CLI-backed providers and the
/// user-supplied custom endpoint; any other unregistered type gets a plain
/// cloud icon.
Widget providerLogoWidget(String type, {double size = 18}) {
  final path = _providerAssetPath(type);
  if (path == null) {
    final fallback = switch (type) {
      'claude-code-cli' || 'codex-cli' => Icons.terminal,
      'custom' => Icons.settings_ethernet,
      'custom-anthropic' => Icons.settings_ethernet,
      _ => Icons.cloud_outlined,
    };
    // BUG fix: this used to hardcode size: 18 regardless of the caller's
    // requested size, so every fallback-icon provider (previously every
    // type without a registered asset) rendered at a fixed 18px even where
    // callers asked for 24-28px — visibly smaller/inconsistent next to a
    // real logo in the same row.
    return Icon(fallback, size: size);
  }
  if (path.endsWith('.svg')) {
    // These three brand marks are a single-colour glyph (OpenAI ships it
    // white, OpenCode and xAI ship it black), so on the "wrong" theme they
    // used to vanish into the background. Tint them with the ambient text
    // colour at render time — no call site has to pass anything. The
    // multi-colour logos (Google, Kilo) must NOT be tinted, so they fall
    // through to the plain branch.
    const monochrome = {
      'lib/icon/OpenAI_Symbol_0.svg',
      'lib/icon/opencode.svg',
      'lib/icon/XAL.svg',
    };
    if (monochrome.contains(path)) {
      return Builder(
        builder: (context) => SvgPicture.asset(
          path,
          width: size,
          height: size,
          colorFilter: ColorFilter.mode(
            DefaultTextStyle.of(context).style.color ?? const Color(0xFF000000),
            BlendMode.srcIn,
          ),
        ),
      );
    }
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
