import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/subagent.dart';
import '../core/api_client.dart';
import 'auth_gate_provider.dart';
import 'chat_provider.dart';
import 'gate_guard.dart';

/// Scoped selection cannot carry a child from one conversation into another.
final selectedSubagentProvider = StateProvider.autoDispose
    .family<String?, String>((ref, chatId) => null);

/// Sequential await + cancellable pause: polls never overlap, and an abandoned
/// conversation cannot publish a late response into the newly selected chat.
final subagentsProvider = StreamProvider.autoDispose
    .family<List<SubagentJob>, String>((ref, chatId) {
      // Register dependencies synchronously: an async* body's deferred watch can
      // lose an auto-disposed gate subscription before the first gate event.
      final gate = ref.watch(authGateProvider).valueOrNull;
      final api = ref.watch(apiClientProvider);
      if (chatId.isEmpty || authGateBlocked(gate)) {
        return Stream.value(const []);
      }
      return _pollSubagents(ref, api, chatId);
    });

Stream<List<SubagentJob>> _pollSubagents(
  Ref ref,
  MemoApiClient api,
  String chatId,
) async* {
  var disposed = false;
  ref.onDispose(() => disposed = true);
  List<SubagentJob> previous = const [];
  while (!disposed) {
    try {
      final jobs = await api.listSubagents(chatId);
      if (disposed) return;
      previous = jobs;
      yield jobs;
      if (disposed) return;
      if (jobs.isNotEmpty &&
          ref.read(activeChatIdProvider).valueOrNull == chatId &&
          !ref.read(isSendingProvider)) {
        await ref.read(messagesProvider.notifier).refreshIfIdle(chatId);
      }
    } catch (_) {
      if (disposed) return;
      // Preserve transcripts during reconnects. Initial failures remain visible
      // through the provider's error state instead of becoming empty success.
      if (previous.isEmpty) rethrow;
      yield previous;
    }
    await cancellablePause(ref, const Duration(seconds: 2));
  }
}

final subagentParallelismProvider = FutureProvider.autoDispose<int>((
  ref,
) async {
  if (authGateBlocked(ref.watch(authGateProvider).valueOrNull)) return 3;
  return ref.watch(apiClientProvider).getSubagentParallelism();
});
