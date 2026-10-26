import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/telemetry.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';

/// Where the person's answer about the optional anonymous daily summary stands.
/// Ambient, like the update info: a failure stays invisible (null = nothing to
/// show, and above all nothing to ask), and before the auth gate opens it mounts
/// the safe default instead of caching a 401 for the whole session.
final telemetryStatusProvider = FutureProvider<TelemetryStatus?>((ref) async {
  final blocked = ref.watch(authGateProvider.select((g) => authGateBlocked(g.valueOrNull)));
  if (blocked) return null;
  try {
    final map = await ref.read(apiClientProvider).getTelemetryStatus();
    return TelemetryStatus.fromJson(map);
  } catch (e) {
    debugPrint('telemetry_provider: $e');
    return null;
  }
});

/// The report that would be sent next. Not ambient: the person asked to look, so
/// a failure is shown (AsyncError) instead of hidden.
final telemetryPreviewProvider = FutureProvider.autoDispose<TelemetryPreview>((ref) async {
  final map = await ref.read(apiClientProvider).getTelemetryPreview();
  return TelemetryPreview.fromJson(map);
});

/// The three things the person can change. Each one throws on failure so the
/// caller can say so, and refreshes the status on success.
class TelemetryActions {
  TelemetryActions(this._ref);
  final Ref _ref;

  Future<void> answer(bool share) async {
    await _ref.read(apiClientProvider).setTelemetryConsent(share);
    _ref.invalidate(telemetryStatusProvider);
    _ref.invalidate(telemetryPreviewProvider);
  }

  Future<void> resetId() async {
    await _ref.read(apiClientProvider).resetTelemetryId();
    _ref.invalidate(telemetryStatusProvider);
  }

  Future<void> deleteData() async {
    await _ref.read(apiClientProvider).deleteTelemetryData();
    _ref.invalidate(telemetryStatusProvider);
    _ref.invalidate(telemetryPreviewProvider);
  }
}

final telemetryActionsProvider = Provider<TelemetryActions>(TelemetryActions.new);
