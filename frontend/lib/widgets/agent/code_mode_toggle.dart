import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/friendly_error.dart';
import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/agent_provider.dart';
import '../../providers/chat_provider.dart';

/// Flips Code Mode for one agent chat and refreshes its state. Shared by the
/// top-bar button and the phone actions sheet.
///
/// Code Mode strips chat-oriented prompt blocks and background LLM calls while
/// keeping the agent tool loop — leaner, faster, coding-focused. It defaults on
/// for a project chat; tapping pins the opposite choice.
Future<void> toggleChatCodeMode(BuildContext context, WidgetRef ref, String chatId, {required bool currentlyOn}) async {
  try {
    await ref.read(apiClientProvider).setChatCodeMode(chatId, !currentlyOn);
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('code_mode_toggle_failed', {'error': FriendlyError.describeGeneric(e)}))),
      );
    }
  }
  // Guard the ref too, not just the SnackBar (BUG-SCAN17): if the chat was
  // deselected/deleted while the POST was in flight this widget is gone, and
  // invalidating through a disposed ref throws StateError.
  if (context.mounted) ref.invalidate(chatCodeModeProvider(chatId));
}

/// Per-chat Code Mode switch for the top bar.
class CodeModeToggle extends ConsumerWidget {
  final String chatId;
  const CodeModeToggle({super.key, required this.chatId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final on = ref.watch(chatCodeModeProvider(chatId)).valueOrNull?.enabled ?? false;
    return IconButton(
      icon: Icon(on ? Icons.code : Icons.code_off),
      tooltip: on ? L10n.t('code_mode_on_tooltip') : L10n.t('code_mode_off_tooltip'),
      color: on ? MemoTheme.green : MemoTheme.of(context).textSecondary,
      onPressed: () => toggleChatCodeMode(context, ref, chatId, currentlyOn: on),
    );
  }
}
