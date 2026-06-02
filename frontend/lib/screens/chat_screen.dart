import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../widgets/chat_sidebar.dart';
import '../widgets/chat_message_list.dart';
import '../widgets/chat_input.dart';
import '../widgets/welcome_view.dart';

/// Chat screen — sidebar + message list + input area.
class ChatScreen extends ConsumerWidget {
  const ChatScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Row(
      children: [
        // ─── Sidebar ──────────────────────────────
        const ChatSidebar(),

        // ─── Main Chat Area ───────────────────────
        Expanded(
          child: Container(color: MemoTheme.bgApp, child: const _ChatContent()),
        ),
      ],
    );
  }
}

class _ChatContent extends ConsumerWidget {
  const _ChatContent();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final messagesAsync = ref.watch(messagesProvider);

    ref.listen<String>(errorMessageProvider, (previous, next) {
      if (next.isNotEmpty && context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(next), backgroundColor: MemoTheme.red),
        );
        ref.read(errorMessageProvider.notifier).state = '';
      }
    });

    return Column(
      children: [
        // ─── Top Bar ──────────────────────────────
        const _ChatTopBar(),

        // ─── Messages or Welcome ──────────────────
        Expanded(
          child: messagesAsync.when(
            loading: () => const Center(
              child: CircularProgressIndicator(color: MemoTheme.accent),
            ),
            error: (e, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, color: MemoTheme.red, size: 40),
                  const SizedBox(height: 12),
                  Text(
                    L10n.t('connection_error'),
                    style: TextStyle(color: MemoTheme.textMuted),
                  ),
                  const SizedBox(height: 12),
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
              if (messages.isEmpty && !isSending && streamingContent.isEmpty) {
                return const WelcomeView();
              }
              return ChatMessageList(
                messages: messages,
                isTyping: isSending,
                streamingContent: streamingContent,
                streamingThinking: streamingThinking,
              );
            },
          ),
        ),

        // ─── Input ────────────────────────────────
        const ChatInput(),
      ],
    );
  }
}

class _ChatTopBar extends ConsumerWidget {
  const _ChatTopBar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isIncognito = ref.watch(incognitoProvider);
    final chatListAsync = ref.watch(chatListProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);

    String title = L10n.t('new_chat');
    activeChatAsync.whenData((activeId) {
      chatListAsync.whenData((chats) {
        final chat = chats.where((c) => c.id == activeId).firstOrNull;
        if (chat != null) title = chat.title;
      });
    });

    return Container(
      height: 56,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: MemoTheme.bgApp,
        border: Border(bottom: BorderSide(color: MemoTheme.borderSoft)),
      ),
      child: Row(
        children: [
          // Title
          Expanded(
            child: Row(
              children: [
                if (isIncognito) ...[
                  Container(
                    padding: const EdgeInsets.symmetric(
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
                        const SizedBox(width: 4),
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
                  const SizedBox(width: 12),
                ],
                Flexible(
                  child: Text(
                    isIncognito ? L10n.t('incognito_mode') : title,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: MemoTheme.textMain,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),

          // Export button
          IconButton(
            icon: const Icon(Icons.file_download_outlined, size: 20),
            color: MemoTheme.textDim,
            tooltip: L10n.t('export_chat'),
            onPressed: () async {
              try {
                final api = ref.read(apiClientProvider);
                final md = await api.exportChat();
                if (md.isNotEmpty && context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text('Chat exported (${md.length} chars)'),
                    ),
                  );
                }
              } catch (_) {}
            },
          ),
        ],
      ),
    );
  }
}
