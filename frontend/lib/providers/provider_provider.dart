import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

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

  Future<void> deleteProvider(String type) async {
    await ref.read(apiClientProvider).deleteProvider(type);
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

/// Provider display helpers.
IconData providerIcon(String type) {
  switch (type) {
    case 'openai':
      return Icons.radar;
    case 'gemini':
      return Icons.diamond_outlined;
    case 'grok':
      return Icons.close_fullscreen_outlined;
    case 'groq':
      return Icons.bolt;
    case 'claude':
      return Icons.square_outlined;
    case 'openrouter':
      return Icons.swap_horiz;
    case 'ollama':
      return Icons.change_circle_outlined;
    default:
      return Icons.cloud_outlined;
  }
}
