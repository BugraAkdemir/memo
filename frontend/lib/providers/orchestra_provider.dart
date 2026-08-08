import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/orchestra_config.dart';
import 'chat_provider.dart';
import '../core/friendly_error.dart';

/// Orchestra config provider.
final orchestraConfigProvider =
    AsyncNotifierProvider<OrchestraConfigNotifier, OrchestraConfig>(
      OrchestraConfigNotifier.new,
    );

class OrchestraConfigNotifier extends AsyncNotifier<OrchestraConfig> {
  @override
  Future<OrchestraConfig> build() async {
    try {
      return await ref.read(apiClientProvider).getOrchestraConfig();
    } catch (e) {
      debugPrint('orchestra: build error: ${FriendlyError.describeGeneric(e)}');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Orchestra yapılandırması alınamadı (${FriendlyError.describeGeneric(e)})';
      return const OrchestraConfig();
    }
  }

  Future<void> save(OrchestraConfig config) async {
    try {
      await ref.read(apiClientProvider).updateOrchestraConfig(config);
      state = AsyncData(config);
    } catch (e) {
      debugPrint('orchestra: save error: ${FriendlyError.describeGeneric(e)}');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Orchestra yapılandırması kaydedilemedi (${FriendlyError.describeGeneric(e)})';
      rethrow;
    }
  }

  Future<void> toggle(bool enabled) async {
    final current = state.valueOrNull ?? const OrchestraConfig();
    await save(current.copyWith(enabled: enabled));
  }
}
