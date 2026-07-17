import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/l10n.dart';
import '../core/notification_service.dart';
import '../models/routine.dart';
import 'connection_provider.dart';

// ── State ─────────────────────────────────────────────────────────────────────

class RoutineState {
  final List<Routine> routines;
  final bool loading;
  final String? error;

  const RoutineState({
    this.routines = const [],
    this.loading = false,
    this.error,
  });

  RoutineState copyWith({
    List<Routine>? routines,
    bool? loading,
    String? error,
  }) =>
      RoutineState(
        routines: routines ?? this.routines,
        loading: loading ?? this.loading,
        error: error,
      );
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class RoutineNotifier extends StateNotifier<RoutineState> {
  final MemoApiClient _api;

  RoutineNotifier(this._api) : super(const RoutineState()) {
    _load();
  }

  Future<void> _load() async {
    state = state.copyWith(loading: true);
    try {
      final raw = await _api.getRoutines();
      final routines = raw.map(Routine.fromJson).toList();
      state = state.copyWith(routines: routines, loading: false, error: null);
    } catch (e) {
      state = state.copyWith(loading: false, error: L10n.t('routines_load_error', {'e': '$e'}));
    }
  }

  Future<void> refresh() => _load();

  /// Polls for newly generated routine content and pre-schedules a local
  /// notification for each — the mobile-delivery half of the "no push
  /// channel, so generate ahead of time and let the phone schedule its own
  /// OS-level alarm" design (see internal/routine's mobileLeadDuration).
  /// Always fetches the full still-upcoming set (backend already filters out
  /// past fire times) rather than tracking a "since" cursor — scheduling is
  /// idempotent per routine ID, so re-scheduling the same entry is harmless.
  Future<void> checkMobileReady() async {
    try {
      final raw = await _api.getRoutinesMobileReady(0);
      for (final json in raw) {
        final item = RoutineMobileReady.fromJson(json);
        await NotificationService.scheduleRoutine(
          routineId: item.id,
          title: item.title,
          body: item.body,
          whenUtc: item.fireAtUtc,
        );
      }
    } catch (_) {
      // Best-effort — a missed poll just means a slightly later scheduling.
    }
  }

  Future<void> createFromDraft({
    required String originalText,
    required Map<String, dynamic> draft,
    String whatsAppTargetJid = '',
    bool autoApproveTools = false,
  }) async {
    try {
      await _api.createRoutine(
        originalText: originalText,
        draft: draft,
        whatsAppTargetJid: whatsAppTargetJid,
        autoApproveTools: autoApproveTools,
      );
      await _load();
    } catch (e) {
      state = state.copyWith(error: L10n.t('routines_save_error', {'e': '$e'}));
    }
  }

  Future<void> toggleEnabled(Routine r) async {
    try {
      final updated = Map<String, dynamic>.from(r.toJson());
      updated['enabled'] = !r.enabled;
      await _api.updateRoutine(updated);
      await _load();
    } catch (e) {
      state = state.copyWith(error: L10n.t('routines_update_error', {'e': '$e'}));
    }
  }

  Future<void> delete(String id) async {
    try {
      await _api.deleteRoutine(id);
      await NotificationService.cancelRoutine(id);
      state = state.copyWith(
        routines: state.routines.where((r) => r.id != id).toList(),
      );
    } catch (e) {
      state = state.copyWith(error: L10n.t('routines_delete_error', {'e': '$e'}));
    }
  }
}

// ── Provider ──────────────────────────────────────────────────────────────────

final routineProvider = StateNotifierProvider<RoutineNotifier, RoutineState>((ref) {
  final client = ref.watch(apiClientProvider);
  return RoutineNotifier(client);
});
