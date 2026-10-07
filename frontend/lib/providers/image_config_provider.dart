import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../models/image_config.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';

/// Automatic image routing settings: whether a picture asked for in chat goes to
/// an image model on its own, and the default image model API providers use.
final imageConfigProvider =
    AsyncNotifierProvider<ImageConfigNotifier, ImageConfig>(ImageConfigNotifier.new);

class ImageConfigNotifier extends AsyncNotifier<ImageConfig> {
  @override
  Future<ImageConfig> build() async {
    // Same mount-with-a-default rule as every one-shot fetch (BUG-ONB6): a build()
    // landing while the auth gate is up would 401 and stay failed for the session.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return const ImageConfig();
    }
    try {
      return await ref.read(apiClientProvider).getImageConfig();
    } catch (e) {
      // A backend that predates the endpoint, or a blip: the section shows the
      // defaults; saving reports its own failure.
      debugPrint('imageConfig: $e');
      return const ImageConfig();
    }
  }

  /// Saves any of the settings. Returns null on success, otherwise a sentence for
  /// the user (the backend's refusal, e.g. an unknown provider).
  Future<String?> save({
    bool? autoRoute,
    String? defaultProvider,
    String? defaultModel,
  }) async {
    try {
      final next = await ref.read(apiClientProvider).setImageConfig(
            autoRoute: autoRoute,
            defaultProvider: defaultProvider,
            defaultModel: defaultModel,
          );
      state = AsyncData(next);
      return null;
    } on DioException catch (e) {
      // The backend refuses nonsense (an unknown provider, half a default model)
      // with a plain-text reason that already says what to fix.
      final body = e.response?.data;
      if (body is String && body.trim().isNotEmpty) return body.trim();
      return FriendlyError.describeGeneric(e);
    } catch (e) {
      return FriendlyError.describeGeneric(e);
    }
  }
}
