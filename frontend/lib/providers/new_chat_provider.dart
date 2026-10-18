import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../models/chat.dart';
import '../widgets/server_file_browser_dialog.dart';
import 'agent_provider.dart' show chatCodeModeProvider;
import 'chat_provider.dart';

/// Chats on the new-chat screen whose person has picked "Agent" but not yet a
/// project folder. Once a folder is chosen the chat IS an agent chat on the
/// backend (it has a project path) and this set no longer matters for it.
final pendingAgentChatsProvider = StateProvider<Set<String>>((ref) => const {});

/// Whether the new-chat screen should show its Agent variant for [chat].
bool isAgentModeFor(ChatSession? chat, Set<String> pending) {
  if (chat == null) return false;
  return chat.isAgentChat || pending.contains(chat.id);
}

/// The Chat | Agent switch on a NEW (empty) chat. Choosing Agent only marks the
/// chat as waiting for a folder; choosing Chat on a chat that already got a
/// folder turns it back into a plain chat. Both are refused by the backend once
/// the chat has a message, and the switch is not shown then.
Future<void> setNewChatMode(BuildContext context, WidgetRef ref, ChatSession chat, {required bool agent}) async {
  final pending = {...ref.read(pendingAgentChatsProvider)};
  if (agent) {
    pending.add(chat.id);
    ref.read(pendingAgentChatsProvider.notifier).state = pending;
    return;
  }
  pending.remove(chat.id);
  ref.read(pendingAgentChatsProvider.notifier).state = pending;
  if (!chat.isAgentChat) return;
  try {
    await ref.read(apiClientProvider).setChatProject(chat.id, '');
    ref.invalidate(chatCodeModeProvider(chat.id));
    await ref.read(chatListProvider.notifier).refresh();
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('agent_create_failed', {'error': FriendlyError.describeGeneric(e)}))),
      );
    }
  }
}

/// Asks for the project folder of a new agent chat (the server file browser) and
/// binds it to the chat. Returns whether the chat now has a folder; false when the
/// person cancelled or the backend refused.
Future<bool> chooseProjectForNewChat(BuildContext context, WidgetRef ref, String chatId) async {
  final chosen = await showServerFileBrowserDialog(context, mode: ServerBrowseMode.directory);
  if (chosen == null || chosen.isEmpty) return false;
  try {
    await ref.read(apiClientProvider).setChatProject(chatId, chosen);
    ref.invalidate(chatCodeModeProvider(chatId));
    await ref.read(chatListProvider.notifier).refresh();
    return true;
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('agent_create_failed', {'error': FriendlyError.describeGeneric(e)}))),
      );
    }
    return false;
  }
}
