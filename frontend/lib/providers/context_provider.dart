import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/context_report.dart';
import 'auth_gate_provider.dart';
import 'gate_guard.dart';
import 'chat_provider.dart';

/// The context gauge of one chat (the ring at the bottom of the chat and its
/// popover): how full the model's window is, what it is made of, and a
/// Subscriptions account's allowance meters.
///
/// Re-reads itself whenever a turn starts or ends ([isSendingProvider]) — the
/// provider's own token count only exists once a turn has finished — and when
/// the chat changes. The popover also invalidates it on open, so it never shows
/// a figure older than the moment it was looked at.
///
/// Ambient: the ring is mounted whenever a chat is, so a failure here must stay
/// invisible (null = no ring), never a toast — and before the auth gate opens a
/// 401 would be cached for the whole session, so it mounts the safe default.
final contextReportProvider = FutureProvider.autoDispose.family<ContextReport?, String>((ref, chatId) async {
  ref.watch(isSendingProvider);
  if (chatId.isEmpty) return null;
  if (authGateBlocked(ref.read(authGateProvider).valueOrNull)) return null;
  try {
    return await ref.read(apiClientProvider).getContextReport(chatId);
  } catch (e) {
    debugPrint('context_provider: $e');
    return null;
  }
});
