import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/orchestra_config.dart';
import 'chat_provider.dart';

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
    } catch (_) {
      return const OrchestraConfig();
    }
  }

  Future<void> save(OrchestraConfig config) async {
    await ref.read(apiClientProvider).updateOrchestraConfig(config);
    state = AsyncData(config);
  }

  Future<void> toggle(bool enabled) async {
    final current = state.valueOrNull ?? const OrchestraConfig();
    await save(current.copyWith(enabled: enabled));
  }
}
