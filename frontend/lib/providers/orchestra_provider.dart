import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/orchestra_config.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

/// Orchestra config provider.
final orchestraConfigProvider =
    AsyncNotifierProvider<OrchestraConfigNotifier, OrchestraConfig>(
      OrchestraConfigNotifier.new,
    );

class OrchestraConfigNotifier extends AsyncNotifier<OrchestraConfig> {
  @override
  Future<OrchestraConfig> build() async {
    // BUG-ONB6 (see chat_provider.dart's ChatListNotifier for the full
    // story): without this, a build() landing while the auth gate was
    // still up 401'd and surfaced a red "Orchestra yapılandırması
    // alınamadı" SnackBar via errorMessageProvider below — a spurious
    // error toast on every connect to a gated backend, not just a silent
    // wrong value. app_shell.dart's gate-transition listener re-invalidates
    // this once the gate actually opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return const OrchestraConfig();
    }
    try {
      return await ref.read(apiClientProvider).getOrchestraConfig();
    } catch (e) {
      // Deliberately silent on failure, same reasoning and same fix as
      // activeProviderTypeProvider/remoteAccessProvider (see their own
      // comments): this is watched ambiently from the always-visible engine
      // strip and chat input bar (engine_strip.dart, chat_input.dart), not
      // just the Orchestra dialog/settings tab a user chose to open — those
      // two already show this same error inline via their own `.when()`
      // error branch, so rethrowing into errorMessageProvider here only
      // ever added a redundant, ambiently-spammed SnackBar (reported live:
      // a transient failure while Orchestra was off kept surfacing an
      // "Orchestra yapılandırması alınamadı" toast with no relation to
      // anything the user was doing).
      debugPrint('orchestra: build error: ${FriendlyError.describeGeneric(e)}');
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
