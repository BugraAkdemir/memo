import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'dart:io';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/whatsapp_provider.dart';
import '../widgets/chat_sidebar.dart';
import '../widgets/chat_message_list.dart';
import '../widgets/chat_input.dart';
import '../widgets/welcome_view.dart';
import '../widgets/agent/permission_dialog.dart';
import '../providers/agent_provider.dart';
import '../models/agent.dart';

/// Chat screen — sidebar + message list + input area.
class ChatScreen extends ConsumerWidget {
   ChatScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Row(
      children: [
        // ─── Sidebar ──────────────────────────────
         ChatSidebar(),

        // ─── Main Chat Area ───────────────────────
        Expanded(
          child: Container(color: MemoTheme.of(context).bgApp, child:  _ChatContent()),
        ),
      ],
    );
  }
}

class _ChatContent extends ConsumerStatefulWidget {
  @override
  ConsumerState<_ChatContent> createState() => _ChatContentState();
}

class _ChatContentState extends ConsumerState<_ChatContent> {
  @override
  Widget build(BuildContext context) {
    final messagesAsync = ref.watch(messagesProvider);

    ref.listen<String>(errorMessageProvider, (previous, next) {
      if (next.isNotEmpty && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(next), backgroundColor: MemoTheme.red),
        );
        ref.read(errorMessageProvider.notifier).state = '';
      }
    });

    ref.listen<AsyncValue<AgentEvent>>(agentEventStreamProvider, (prev, next) {
      if (next.hasValue && next.value != null && mounted) {
        final event = next.value!;
        if (event.type == 'permission_request') {
          showDialog(
            context: context,
            barrierDismissible: false,
            // PopScope prevents the back button from closing the dialog without
            // a policy response — otherwise the agent pipeline would block forever
            // waiting for a permission answer that never comes.
            builder: (context) => PopScope(
              canPop: false,
              child: PermissionDialog(event: event),
            ),
          );
        }
      }
    });

    return Column(
      children: [
        // ─── Top Bar ──────────────────────────────
         _ChatTopBar(),

        // ─── Messages or Welcome ──────────────────
        Expanded(
          child: messagesAsync.when(
            loading: () =>  Center(
              child: CircularProgressIndicator(color: MemoTheme.accent),
            ),
            error: (e, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, color: MemoTheme.red, size: 40),
                   SizedBox(height: 12),
                  Text(
                    '$e',
                    style: TextStyle(color: MemoTheme.of(context).textMuted),
                    textAlign: TextAlign.center,
                  ),
                   SizedBox(height: 12),
                  OutlinedButton(
                    onPressed: () => ref.invalidate(messagesProvider),
                    child: Text(L10n.t('retry')),
                  ),
                ],
              ),
            ),
            data: (messages) {
              final isSending = ref.watch(isSendingProvider);
              final streamingContent = ref.watch(streamingContentProvider);
              final streamingThinking = ref.watch(streamingThinkingProvider);
              final streamingAgentEvents = ref.watch(streamingAgentEventsProvider);
              if (messages.isEmpty && !isSending && streamingContent.isEmpty && streamingAgentEvents.isEmpty) {
                return  WelcomeView();
              }
              return ChatMessageList(
                messages: messages,
                isTyping: isSending,
                streamingContent: streamingContent,
                streamingThinking: streamingThinking,
                streamingAgentEvents: streamingAgentEvents,
                onEdit: (index, newContent) {
                  ref.read(messagesProvider.notifier).updateMessage(index, newContent);
                },
                onDelete: (index) {
                  ref.read(messagesProvider.notifier).deleteMessage(index);
                },
              );
            },
          ),
        ),

        // ─── Input ────────────────────────────────
         ChatInput(),
      ],
    );
  }
}

class _ChatTopBar extends ConsumerWidget {
   _ChatTopBar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isIncognito = ref.watch(incognitoProvider);
    final isAgentEnabled = ref.watch(agentEnabledProvider);
    final isWhatsAppMode = ref.watch(whatsAppChatModeProvider);
    final waStatus = ref.watch(whatsAppStatusProvider);
    final chatListAsync = ref.watch(chatListProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);

    String title = L10n.t('new_chat');
    String? agentProjectPath;
    activeChatAsync.whenData((activeId) {
      chatListAsync.whenData((chats) {
        final chat = chats.where((c) => c.id == activeId).firstOrNull;
        if (chat != null) {
          title = chat.title;
          agentProjectPath = chat.projectPath;
        }
      });
    });

    return Container(
      height: 56,
      padding:  EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgApp,
        border: Border(bottom: BorderSide(color: MemoTheme.of(context).borderSoft)),
      ),
      child: Row(
        children: [
          // Title
          Expanded(
            child: Row(
              children: [
                if (isIncognito) ...[
                  Container(
                    padding:  EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.warmBrown.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.visibility_off,
                          size: 14,
                          color: MemoTheme.warmBrown,
                        ),
                         SizedBox(width: 4),
                        Text(
                          L10n.t('incognito_on'),
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.warmBrown,
                          ),
                        ),
                      ],
                    ),
                  ),
                   SizedBox(width: 12),
                ],
                if (isAgentEnabled && agentProjectPath != null) ...[
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.green.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.folder_outlined,
                          size: 14,
                          color: MemoTheme.green,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          '${L10n.t('agent_chat_project')}${agentProjectPath!.split('/').last}',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.green,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 12),
                ] else if (isAgentEnabled && agentProjectPath == null) ...[
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.accent.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.psychology,
                          size: 14,
                          color: MemoTheme.accent,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          L10n.t('agent_badge'),
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.accent,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 12),
                ],
                Flexible(
                  child: Text(
                    isIncognito ? L10n.t('incognito_mode') : title,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: MemoTheme.of(context).textMain,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),

          // Undo button (only when agent mode is on)
          if (isAgentEnabled)
            IconButton(
              icon: Icon(Icons.undo, size: 20),
              color: MemoTheme.of(context).textDim,
              tooltip: L10n.t('agent_undo'),
              onPressed: () async {
                try {
                  await ref.read(apiClientProvider).undoAgentEdit();
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('agent_undone'))),
                    );
                  }
                } catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('agent_undo_failed', {'e': e.toString()})), backgroundColor: MemoTheme.red),
                    );
                  }
                }
              },
            ),
          
          // WhatsApp mode toggle (only when connected)
          if (waStatus.asData?.value.connected == true)
            IconButton(
              icon: Icon(
                Icons.chat,
                size: 20,
                color: isWhatsAppMode ? MemoTheme.green : MemoTheme.of(context).textDim,
              ),
              tooltip: isWhatsAppMode ? L10n.t('whatsapp_mode_on') : L10n.t('whatsapp_mode_off'),
              onPressed: () => ref.read(whatsAppChatModeProvider.notifier).toggle(),
            ),
          
          // Export button
          IconButton(
            icon:  Icon(Icons.file_download_outlined, size: 20),
            color: MemoTheme.of(context).textDim,
            tooltip: L10n.t('export_chat'),
            onPressed: () async {
              try {
                final api = ref.read(apiClientProvider);
                final md = await api.exportChat();
                if (md.isEmpty || !context.mounted) return;
                final path = await FilePicker.platform.saveFile(
                  dialogTitle: L10n.t('export_chat'),
                  fileName: 'chat_export.md',
                  type: FileType.custom,
                  allowedExtensions: ['md'],
                );
                if (path != null) {
                  await File(path).writeAsString(md);
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('chat_exported', {'path': path}))),
                    );
                  }
                }
              } catch (e) {
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(L10n.t('export_failed', {'e': e.toString()}))),
                  );
                }
              }
            },
          ),
        ],
      ),
    );
  }
}
