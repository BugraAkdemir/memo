import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/chat.dart';
import '../providers/chat_provider.dart';
import '../providers/agent_provider.dart';
import '../widgets/chat_message_list.dart';
import '../widgets/chat_input.dart';
import '../widgets/server_file_browser_dialog.dart';
import 'tasks_screen.dart';
import '../core/friendly_error.dart';

class AgentScreen extends ConsumerWidget {
  const AgentScreen({super.key});

  // Same reasoning (and the same 600px cut) as ChatScreen: _AgentSidebar is
  // a fixed 260px, which together with AppShell's NavRail leaves the agent
  // conversation almost nothing at a phone width.
  static const double _sidebarBreakpoint = 600;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return LayoutBuilder(
      builder: (context, constraints) {
        // Matches ChatScreen's content/Scaffold background handling: opaque
        // bgApp in dark, transparent in Glass Light so the ancestor
        // AppShell Stack's gradient shows through instead of a flat color
        // that visibly seams against the gradient at this screen's edges.
        final isGlass = MemoTheme.of(context).isGlass;
        final content = Container(
          color: isGlass ? Colors.transparent : MemoTheme.of(context).bgApp,
          child: _AgentContent(),
        );

        if (constraints.maxWidth < _sidebarBreakpoint) {
          return Scaffold(
            backgroundColor: isGlass ? Colors.transparent : null,
            drawer: Drawer(child: SafeArea(child: _AgentSidebar())),
            body: Column(
              children: [
                Align(
                  alignment: Alignment.centerLeft,
                  child: Builder(
                    builder: (ctx) => IconButton(
                      icon: const Icon(Icons.menu),
                      tooltip: L10n.t('agent_open_chats'),
                      onPressed: () => Scaffold.of(ctx).openDrawer(),
                    ),
                  ),
                ),
                Expanded(child: content),
              ],
            ),
          );
        }

        return Row(
          children: [
            _AgentSidebar(),
            Expanded(child: content),
          ],
        );
      },
    );
  }
}

class _AgentSidebar extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatListAsync = ref.watch(chatListProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);

    return Container(
      width: 260,
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        border: Border(right: BorderSide(color: MemoTheme.of(context).borderSoft)),
      ),
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: SizedBox(
              width: double.infinity,
              child: _ActionButton(
                icon: Icons.add,
                label: L10n.t('agent_new_chat'),
                  onTap: () async {
                    final result = await showServerFileBrowserDialog(
                      context,
                      mode: ServerBrowseMode.directory,
                    );
                    if (result == null) return;
                    try {
                      final api = ref.read(apiClientProvider);
                      final id = await api.createAgentChat(result);
                      await ref.read(chatListProvider.notifier).refresh();
                      ref.read(activeChatIdProvider.notifier).switchTo(id);
                      if (!ref.read(agentEnabledProvider)) {
                        ref.read(agentEnabledProvider.notifier).setEnabled(true);
                      }
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(L10n.t('agent_create_failed', {'error': FriendlyError.describeGeneric(e)}))),
                        );
                      }
                    }
                  },
              ),
            ),
          ),
          const Divider(height: 1),
          Expanded(
            child: chatListAsync.when(
              loading: () => const Center(child: CircularProgressIndicator(strokeWidth: 2)),
              error: (e, _) => Center(child: Padding(
                padding: const EdgeInsets.all(16),
                child: Text(L10n.t('error'), style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13)),
              )),
              data: (chats) {
                final activeId = activeChatAsync.valueOrNull ?? '';
                final agentChats = chats.where((c) => c.isAgentChat).toList();
                if (agentChats.isEmpty) {
                  return Center(child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Text(L10n.t('agent_no_chats'), style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13)),
                  ));
                }
                return ListView.builder(
                  padding: const EdgeInsets.symmetric(vertical: 4),
                  itemCount: agentChats.length,
                  itemBuilder: (_, i) {
                    final chat = agentChats[i];
                    final isActive = chat.id == activeId;
                    return _AgentChatItem(
                      chat: chat,
                      isActive: isActive,
                      onTap: (id) => ref.read(activeChatIdProvider.notifier).switchTo(id),
                      onDelete: (id) => ref.read(chatListProvider.notifier).delete(id),
                    );
                  },
                );
              },
            ),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            decoration: BoxDecoration(border: Border(top: BorderSide(color: MemoTheme.of(context).borderSoft))),
            child: Row(
              children: [
                Container(width: 8, height: 8, decoration: const BoxDecoration(shape: BoxShape.circle, color: MemoTheme.green)),
                const SizedBox(width: 8),
                Text(L10n.t('agent_mode'), style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _AgentChatItem extends StatelessWidget {
  final ChatSession chat;
  final bool isActive;
  final ValueChanged<String> onTap;
  final ValueChanged<String> onDelete;

  const _AgentChatItem({required this.chat, required this.isActive, required this.onTap, required this.onDelete});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => onTap(chat.id),
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: isActive ? MemoTheme.accentMuted : Colors.transparent,
          borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
          border: isActive ? Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)) : null,
        ),
        child: Row(
          children: [
            Icon(Icons.smart_toy, size: 14, color: MemoTheme.accent),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(chat.title, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: MemoTheme.of(context).textMain), maxLines: 1, overflow: TextOverflow.ellipsis),
                  const SizedBox(height: 2),
                  Text(L10n.t('message_count', {'count': chat.msgCount.toString()}), style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim)),
                ],
              ),
            ),
            // onDelete was already threaded all the way down to this widget
            // but nothing in the UI ever called it — an agent chat could
            // only be deleted by switching to the Chat tab's sidebar, which
            // lists the same sessions. A simple icon (not hover-gated like
            // the Chat sidebar's, to avoid turning this into a StatefulWidget
            // just for that) is enough to close the gap.
            GestureDetector(
              onTap: () => onDelete(chat.id),
              child: Padding(
                padding: const EdgeInsets.only(left: 4),
                child: Icon(Icons.close, size: 14, color: MemoTheme.of(context).textDim),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AgentContent extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final messagesAsync = ref.watch(messagesProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);
    final chatListAsync = ref.watch(chatListProvider);

    // Check if active chat is an agent chat
    final activeId = activeChatAsync.valueOrNull ?? '';
    final chats = chatListAsync.valueOrNull ?? [];
    final activeChat = chats.where((c) => c.id == activeId).firstOrNull;
    final isAgentActive = activeChat?.isAgentChat ?? false;

    if (!isAgentActive) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(40),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 64,
                height: 64,
                decoration: BoxDecoration(
                  color: MemoTheme.accent.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(16),
                ),
                child: const Icon(Icons.smart_toy, size: 32, color: MemoTheme.accent),
              ),
              const SizedBox(height: 20),
              Text(L10n.t('agent_empty_title'), style: TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: MemoTheme.of(context).textMain)),
              const SizedBox(height: 8),
              Text(L10n.t('agent_empty_desc'), style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textDim), textAlign: TextAlign.center),
              const SizedBox(height: 24),
              FilledButton.icon(
                onPressed: () async {
                  final result = await showServerFileBrowserDialog(context, mode: ServerBrowseMode.directory);
                  if (result == null) return;
                  try {
                    final api = ref.read(apiClientProvider);
                    final id = await api.createAgentChat(result);
                    await ref.read(chatListProvider.notifier).refresh();
                    ref.read(activeChatIdProvider.notifier).switchTo(id);
                    if (!ref.read(agentEnabledProvider)) {
                      ref.read(agentEnabledProvider.notifier).setEnabled(true);
                    }
                  } catch (e) {
                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('agent_create_failed', {'error': FriendlyError.describeGeneric(e)}))),
                      );
                    }
                  }
                },
                icon: const Icon(Icons.add, size: 18),
                label: Text(L10n.t('agent_empty_action')),
              ),
            ],
          ),
        ),
      );
    }

    return Column(
      children: [
        _AgentTopBar(activeChat: activeChat!),
        Expanded(
          child: messagesAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, _) => Center(child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, color: MemoTheme.red, size: 40),
                const SizedBox(height: 12),
                Text(FriendlyError.describeGeneric(e), style: TextStyle(color: MemoTheme.of(context).textMuted)),
                const SizedBox(height: 12),
                OutlinedButton(
                    onPressed: () => ref.invalidate(messagesProvider),
                    child: Text(L10n.t('retry'))),
              ],
            )),
            data: (messages) {
              final isSending = ref.watch(isSendingProvider);
              final streamingContent = ref.watch(streamingContentProvider);
              final streamingThinking = ref.watch(streamingThinkingProvider);
              final streamingAgentEvents = ref.watch(streamingAgentEventsProvider);
              final streamingStatus = ref.watch(streamingStatusProvider);
              if (messages.isEmpty && !isSending && streamingContent.isEmpty && streamingAgentEvents.isEmpty) {
                return _AgentWelcome(projectPath: activeChat.projectPath);
              }
              return ChatMessageList(
                messages: messages,
                isTyping: isSending,
                streamingContent: streamingContent,
                streamingThinking: streamingThinking,
                streamingAgentEvents: streamingAgentEvents,
                statusText: streamingStatus,
                onEdit: (index, newContent) => ref.read(messagesProvider.notifier).updateMessage(index, newContent),
                onDelete: (index) => ref.read(messagesProvider.notifier).deleteMessage(index),
              );
            },
          ),
        ),
        ChatInput(),
      ],
    );
  }
}

class _AgentTopBar extends ConsumerWidget {
  final ChatSession activeChat;
  const _AgentTopBar({required this.activeChat});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      height: 56,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgApp,
        border: Border(bottom: BorderSide(color: MemoTheme.of(context).borderSoft)),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
            decoration: BoxDecoration(
              color: MemoTheme.green.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(6),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.smart_toy, size: 14, color: MemoTheme.green),
                const SizedBox(width: 4),
                Text(L10n.t('nav_agent'), style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: MemoTheme.green)),
              ],
            ),
          ),
          const SizedBox(width: 12),
          if (activeChat.projectPath != null) ...[
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
              decoration: BoxDecoration(
                color: MemoTheme.accent.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.folder_outlined, size: 14, color: MemoTheme.accent),
                  const SizedBox(width: 4),
                  Text(p.basename(activeChat.projectPath!), style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: MemoTheme.accent)),
                ],
              ),
            ),
            const SizedBox(width: 12),
          ],
          Expanded(
            child: Text(activeChat.title, style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain), overflow: TextOverflow.ellipsis),
          ),
          IconButton(
            icon: const Icon(Icons.checklist),
            tooltip: L10n.t('taskloop_title'),
            color: MemoTheme.of(context).textSecondary,
            onPressed: () {
              Navigator.of(context).push(
                MaterialPageRoute(
                  builder: (_) => TasksScreen(initialChatId: activeChat.id),
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}

class _AgentWelcome extends StatelessWidget {
  final String? projectPath;
  const _AgentWelcome({this.projectPath});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.smart_toy, size: 48, color: MemoTheme.green),
            const SizedBox(height: 16),
            Text(L10n.t('agent_mode_active'),
                style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w600,
                    color: MemoTheme.of(context).textMain)),
            const SizedBox(height: 8),
            if (projectPath != null)
              Text(L10n.t('agent_project_label', {'path': projectPath!}),
                  style: TextStyle(
                      fontSize: 13, color: MemoTheme.of(context).textDim),
                  textAlign: TextAlign.center),
            const SizedBox(height: 16),
            Text(L10n.t('agent_welcome'), style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textDim), textAlign: TextAlign.center),
          ],
        ),
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _ActionButton({required this.icon, required this.label, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: MemoTheme.of(context).bgElement,
      borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 10),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(icon, size: 16, color: MemoTheme.of(context).textMuted),
              const SizedBox(width: 6),
              Text(label, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: MemoTheme.of(context).textMuted)),
            ],
          ),
        ),
      ),
    );
  }
}
