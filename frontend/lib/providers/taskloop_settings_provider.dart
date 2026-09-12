import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth_gate_provider.dart';
import 'chat_provider.dart' show apiClientProvider;
import 'gate_guard.dart';

/// Raw task-loop settings map from GET /api/taskloop/settings. The Task Loop
/// settings tab watches this and PUTs changes back.
///
/// Same BUG-ONB4/5/6/11-class guard every other one-shot settings fetch in
/// this app already has (see agent_provider.dart's chatCodeModeProvider):
/// a build() landing while the auth gate is still up would otherwise 401
/// and — with no retry loop of its own — leave the Task Loop settings tab
/// stuck on an error for as long as it stays mounted. Currently low-risk
/// (autoDispose + lazily mounted only when that tab is open, so navigating
/// away and back re-triggers build() with a fresh chance to succeed), but
/// the same class of bug nonetheless.
final taskLoopSettingsProvider =
    FutureProvider.autoDispose<Map<String, dynamic>>((ref) async {
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
    return const {};
  }
  final api = ref.watch(apiClientProvider);
  return api.getTaskLoopSettings();
});
