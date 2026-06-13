import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/agent.dart';
import 'chat_provider.dart';

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
      debugPrint('agent: init error: $e');
    }
  }

  Future<void> setEnabled(bool enabled) async {
    final previous = state;
    state = enabled;
    try {
      await _ref.read(apiClientProvider).setAgentEnabled(enabled);
    } catch (e) {
      debugPrint('agent: setEnabled error: $e');
      state = previous;
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
      debugPrint('agent: revoke error: $e');
    }
  }

  Future<void> clearAll() async {
    try {
      await ref.read(apiClientProvider).clearAgentPermissions();
      await refresh();
    } catch (e) {
      debugPrint('agent: clearAll error: $e');
    }
  }
}

/// A stream controller to broadcast agent events (like permission requests) to the UI.
final agentEventBusProvider = Provider<AgentEventBus>((ref) {
  return AgentEventBus();
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
