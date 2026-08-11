import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/l10n.dart';
import '../models/swarm.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';
import '../core/friendly_error.dart';

/// Swarm room status with adaptive polling (same pattern as
/// [WhatsAppStatusNotifier] — manual Timer reschedule, not Timer.periodic).
final swarmStatusProvider =
    StateNotifierProvider.autoDispose<SwarmNotifier, AsyncValue<SwarmStatus>>(
        (ref) {
  final notifier = SwarmNotifier(ref.read(apiClientProvider), ref);
  ref.onDispose(notifier.stopPolling);
  return notifier;
});

class SwarmNotifier extends StateNotifier<AsyncValue<SwarmStatus>> {
  final MemoApiClient _api;
  final Ref _ref;
  Timer? _pollTimer;

  SwarmNotifier(this._api, this._ref) : super(const AsyncValue.loading()) {
    _fetch();
  }

  Future<void> _fetch() async {
    // BUG-ONB6 (see chat_provider.dart's ChatListNotifier for the full
    // story): lower risk here than the app-boot providers (autoDispose —
    // only fetches once the Swarm screen is actually opened, by which
    // point the gate is usually already open), but the same shape of bug —
    // mount empty instead of erroring while blocked; app_shell.dart's
    // gate-transition listener re-invalidates this once the gate opens.
    if (authGateBlocked(_ref.read(authGateProvider).valueOrNull)) {
      if (mounted) state = const AsyncValue.data(SwarmStatus());
      return;
    }
    try {
      final status = await _api.getSwarmStatus();
      if (mounted) state = AsyncValue.data(status);
    } catch (e) {
      if (mounted) state = AsyncValue.error(e, StackTrace.current);
    }
  }

  Future<void> refresh() => _fetch();

  /// Start adaptive polling: fast while waiting for workers / mid-start,
  /// slow once the swarm is stable (running host or connected worker).
  void startPolling() {
    _pollTimer?.cancel();
    _schedule();
  }

  void _schedule() {
    final status = state.valueOrNull;
    Duration interval;
    if (status == null || status.isIdle) {
      // Idle or first load — light check in case another client changed role.
      interval = const Duration(seconds: 10);
    } else if (status.isHost && !status.running) {
      // Hosting a room, waiting for workers / Start — poll fast.
      interval = const Duration(seconds: 2);
    } else if (status.isWorker && !status.connected) {
      // Joining / reconnecting — poll fast.
      interval = const Duration(seconds: 3);
    } else {
      // Host running or worker connected — heartbeat only.
      interval = const Duration(seconds: 12);
    }

    _pollTimer = Timer(interval, () async {
      await _fetch();
      if (mounted) _schedule();
    });
  }

  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  Future<String?> createHost(String modelPath) async {
    try {
      final code = await _api.swarmHostCreate(modelPath);
      await _fetch();
      startPolling();
      return code;
    } catch (e) {
      debugPrint('swarm: createHost error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Swarm oda oluşturulamadı (${FriendlyError.describeGeneric(e)})';
      return null;
    }
  }

  Future<void> removeWorker(String workerId) async {
    try {
      await _api.swarmHostRemoveWorker(workerId);
      await _fetch();
    } catch (e) {
      debugPrint('swarm: removeWorker error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Worker kaldırılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> reorderWorkers(int fromIndex, int toIndex) async {
    try {
      await _api.swarmHostReorderWorkers(fromIndex, toIndex);
      await _fetch();
    } catch (e) {
      debugPrint('swarm: reorderWorkers error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Worker sırası değiştirilemedi (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> setShare(String workerId, double pct) async {
    try {
      await _api.swarmHostSetShare(workerId, pct);
      await _fetch();
    } catch (e) {
      debugPrint('swarm: setShare error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Pay oranı ayarlanamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> start({int ctxSize = 0}) async {
    try {
      await _api.swarmHostStart(ctxSize: ctxSize);
      await _fetch();
    } catch (e) {
      debugPrint('swarm: start error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Swarm başlatılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> stop() async {
    try {
      await _api.swarmHostStop();
      await _fetch();
    } catch (e) {
      debugPrint('swarm: stop error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Swarm durdurulamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> closeRoom() async {
    try {
      await _api.swarmHostClose();
      await _fetch();
    } catch (e) {
      debugPrint('swarm: closeRoom error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Swarm odası kapatılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> join(String code) async {
    try {
      await _api.swarmJoin(code);
      await _fetch();
      startPolling();
    } catch (e) {
      debugPrint('swarm: join error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Swarm\'a katılınamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  Future<void> leave() async {
    try {
      await _api.swarmLeave();
      await _fetch();
    } catch (e) {
      debugPrint('swarm: leave error: ${FriendlyError.describeGeneric(e)}');
      _ref.read(errorMessageProvider.notifier).state =
          '${L10n.t('error')}: Swarm\'dan ayrılamadı (${FriendlyError.describeGeneric(e)})';
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
