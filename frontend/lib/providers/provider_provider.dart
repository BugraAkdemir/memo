import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_svg/flutter_svg.dart';

import '../models/provider_config.dart';
import 'chat_provider.dart';

/// Provider list provider.
final providerListProvider =
    AsyncNotifierProvider<ProviderListNotifier, List<ProviderConfig>>(
      ProviderListNotifier.new,
    );

class ProviderListNotifier extends AsyncNotifier<List<ProviderConfig>> {
  @override
  Future<List<ProviderConfig>> build() async {
    return ref.read(apiClientProvider).getProviders();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => ref.read(apiClientProvider).getProviders(),
    );
  }

  Future<void> updateProvider(ProviderConfig config) async {
    await ref.read(apiClientProvider).updateProvider(config);
    await refresh();
  }

  Future<void> deleteProvider(String type, {String? name}) async {
    await ref.read(apiClientProvider).deleteProvider(type, name: name);
    await refresh();
  }

  Future<Map<String, dynamic>> testProvider(ProviderConfig config) async {
    try {
      return await ref.read(apiClientProvider).testProvider(config);
    } catch (e) {
      debugPrint('provider: test error: $e');
      return {'connected': false, 'error': e.toString()};
    }
  }
}

/// Active provider provider (yes, the name).
final activeProviderTypeProvider =
    AsyncNotifierProvider<ActiveProviderNotifier, String>(
      ActiveProviderNotifier.new,
    );

class ActiveProviderNotifier extends AsyncNotifier<String> {
  @override
  Future<String> build() async {
    try {
      return await ref.read(apiClientProvider).getActiveProvider();
    } catch (e) {
      debugPrint('provider: getActiveProvider error: $e');
      return '';
    }
  }

  Future<void> setActive(String type) async {
    await ref.read(apiClientProvider).setActiveProvider(type);
    state = AsyncData(type);
    ref.invalidate(providerListProvider);
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
    default:
      return '\u2601'; // ☁
  }
}
