import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/agent.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

final agentEnabledProvider =
    StateNotifierProvider<AgentEnabledNotifier, bool>((ref) {
  return AgentEnabledNotifier(ref);
});

class AgentEnabledNotifier extends StateNotifier<bool> {
  final Ref _ref;

  AgentEnabledNotifier(this._ref) : super(false) {
    _init();
  }

  Future<void> _init() async {
    try {
      final enabled = await _ref.read(apiClientProvider).getAgentEnabled();
      state = enabled;
    } catch (e) {
      debugPrint('agent: init error: ${FriendlyError.describeGeneric(e)}');
    }
  }

  Future<void> setEnabled(bool enabled) async {
    final previous = state;
    state = enabled;
    try {
      await _ref.read(apiClientProvider).setAgentEnabled(enabled);
    } catch (e) {
      debugPrint('agent: setEnabled error: ${FriendlyError.describeGeneric(e)}');
      state = previous;
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Ajan modu değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}

final agentPermissionsProvider =
    AsyncNotifierProvider<AgentPermissionsNotifier, List<AgentPermission>>(
  AgentPermissionsNotifier.new,
);

class AgentPermissionsNotifier extends AsyncNotifier<List<AgentPermission>> {
  @override
  Future<List<AgentPermission>> build() async {
    // BUG-ONB6 (see chat_provider.dart's ChatListNotifier for the full
    // story): a one-shot AsyncNotifier whose single build() attempt landing
    // while the auth gate is still up 401s and gets permanently cached as
    // an error. Mount empty instead; app_shell.dart's gate-transition
    // listener re-invalidates this once the gate actually opens.
    if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return const [];
    final api = ref.read(apiClientProvider);
    final data = await api.getAgentPermissions();
    return data.map((e) => AgentPermission.fromJson(e)).toList();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() async {
      final data = await ref.read(apiClientProvider).getAgentPermissions();
      return data.map((e) => AgentPermission.fromJson(e)).toList();
    });
  }

  Future<void> revoke(String id) async {
    try {
      await ref.read(apiClientProvider).revokeAgentPermission(id);
      await refresh();
    } catch (e) {
      debugPrint('agent: revoke error: ${FriendlyError.describeGeneric(e)}');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: İzin kaldırılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> clearAll() async {
    try {
      await ref.read(apiClientProvider).clearAgentPermissions();
      await refresh();
    } catch (e) {
      debugPrint('agent: clearAll error: ${FriendlyError.describeGeneric(e)}');
      ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: İzinler temizlenemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}

/// A stream controller to broadcast agent events (like permission requests) to the UI.
final agentEventBusProvider = Provider<AgentEventBus>((ref) {
  final bus = AgentEventBus();
  ref.onDispose(() => bus.dispose());
  return bus;
});

class AgentEventBus {
  final _controller = StreamController<AgentEvent>.broadcast();

  Stream<AgentEvent> get stream => _controller.stream;

  void emit(AgentEvent event) {
    _controller.add(event);
  }

  void dispose() {
    _controller.close();
  }
}

final agentEventStreamProvider = StreamProvider<AgentEvent>((ref) {
  return ref.watch(agentEventBusProvider).stream;
});

final agentAutoPermissionProvider =
    StateNotifierProvider<AgentAutoPermissionNotifier, bool>((ref) {
  return AgentAutoPermissionNotifier(ref);
});

class AgentAutoPermissionNotifier extends StateNotifier<bool> {
  final Ref _ref;

  AgentAutoPermissionNotifier(this._ref) : super(false) {
    _init();
  }

  Future<void> _init() async {
    try {
      final enabled =
          await _ref.read(apiClientProvider).getAgentAutoPermission();
      state = enabled;
    } catch (e) {
      debugPrint('agent: auto-permission init error: ${FriendlyError.describeGeneric(e)}');
    }
  }

  Future<void> toggle() async {
    final next = !state;
    state = next;
    try {
      await _ref.read(apiClientProvider).setAgentAutoPermission(next);
    } catch (e) {
      debugPrint('agent: auto-permission toggle error: ${FriendlyError.describeGeneric(e)}');
      state = !next;
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Otomatik izin değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> setEnabled(bool enabled) async {
    final previous = state;
    state = enabled;
    try {
      await _ref.read(apiClientProvider).setAgentAutoPermission(enabled);
    } catch (e) {
      debugPrint('agent: auto-permission set error: ${FriendlyError.describeGeneric(e)}');
      state = previous;
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Otomatik izin değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }
}
