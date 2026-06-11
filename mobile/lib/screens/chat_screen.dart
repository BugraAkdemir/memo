import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../widgets/chat_bubble.dart';
import '../widgets/message_input.dart';
import '../widgets/session_drawer.dart';

class ChatScreen extends ConsumerStatefulWidget {
  const ChatScreen({super.key});

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(chatProvider.notifier).loadSessions();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(chatProvider);

    return Scaffold(
      backgroundColor: MemoTheme.bg,
      appBar: AppBar(
        leading: Builder(
          builder: (context) => IconButton(
            icon: const Icon(Icons.menu_rounded),
            onPressed: () => Scaffold.of(context).openDrawer(),
          ),
        ),
        title: Text(
          state.sessions
              .where((s) => s.id == state.activeSessionId)
              .firstOrNull
              ?.title ??
              'Memo',
          overflow: TextOverflow.ellipsis,
        ),
        actions: [
          if (state.streaming)
            IconButton(
              icon: const Icon(Icons.stop_rounded, color: MemoTheme.error),
              onPressed: () => ref.read(chatProvider.notifier).cancelStream(),
              tooltip: 'Stop',
            ),
          PopupMenuButton(
            icon: const Icon(Icons.more_vert),
            itemBuilder: (context) => [
              const PopupMenuItem(
                value: 'new',
                child: ListTile(
                  leading: Icon(Icons.add, size: 20),
                  title: Text('New Chat'),
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                ),
              ),
              if (state.activeSessionId != null)
                const PopupMenuItem(
                  value: 'delete',
                  child: ListTile(
                    leading: Icon(Icons.delete_outline, size: 20, color: MemoTheme.error),
                    title: Text('Delete Chat'),
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                  ),
                ),
            ],
            onSelected: (value) async {
              if (value == 'new') {
                ref.read(chatProvider.notifier).newChat();
              } else if (value == 'delete') {
                final confirm = await showDialog<bool>(
                  context: context,
                  builder: (ctx) => AlertDialog(
                    title: const Text('Delete Chat'),
                    content: const Text('Are you sure?'),
                    actions: [
                      TextButton(
                        onPressed: () => Navigator.pop(ctx, false),
                        child: const Text('Cancel'),
                      ),
                      TextButton(
                        onPressed: () => Navigator.pop(ctx, true),
                        child: const Text('Delete',
                            style: TextStyle(color: MemoTheme.error)),
                      ),
                    ],
                  ),
                );
                if (confirm == true && state.activeSessionId != null) {
                  ref
                      .read(chatProvider.notifier)
                      .deleteChat(state.activeSessionId!);
                }
              }
            },
          ),
        ],
      ),
      drawer: const SessionDrawer(),
      body: Column(
        children: [
          if (state.error != null)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(12),
              color: MemoTheme.error.withAlpha(25),
              child: Row(
                children: [
                  const Icon(Icons.error_outline,
                      size: 16, color: MemoTheme.error),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      state.error!,
                      style: const TextStyle(
                          color: MemoTheme.error, fontSize: 13),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, size: 16,
                        color: MemoTheme.error),
                    onPressed: () =>
                        ref.read(chatProvider.notifier).clearError(),
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(),
                  ),
                ],
              ),
            ),
          Expanded(
            child: state.loading
                ? const Center(child: CircularProgressIndicator())
                : state.activeSessionId == null
                    ? _buildEmptyState(context, ref)
                    : _buildMessageList(state),
          ),
          if (state.activeSessionId != null)
            MessageInput(
              onSend: (text) =>
                  ref.read(chatProvider.notifier).sendMessage(text),
              enabled: !state.streaming,
            ),
        ],
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context, WidgetRef ref) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(
              color: MemoTheme.accent.withAlpha(20),
              borderRadius: BorderRadius.circular(16),
            ),
            child: const Icon(
              Icons.chat_bubble_outline,
              size: 32,
              color: MemoTheme.accentMuted,
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Select or create a chat',
            style: TextStyle(color: MemoTheme.textDim, fontSize: 15),
          ),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: () => ref.read(chatProvider.notifier).newChat(),
            icon: const Icon(Icons.add, size: 18),
            label: const Text('New Chat'),
            style: FilledButton.styleFrom(
              backgroundColor: MemoTheme.accent,
              foregroundColor: MemoTheme.bg,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMessageList(ChatState state) {
    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      itemCount: state.messages.length + (state.streaming ? 1 : 0),
      itemBuilder: (context, index) {
        if (index < state.messages.length) {
          final msg = state.messages[index];
          return ChatBubble(
            role: msg.role,
            content: msg.content,
            thinking: msg.thinking,
          );
        }
        if (state.streaming && state.currentStreamContent.isNotEmpty) {
          return ChatBubble(
            role: 'assistant',
            content: state.currentStreamContent,
          );
        }
        if (state.streaming) {
          return _buildTypingIndicator();
        }
        return const SizedBox.shrink();
      },
    );
  }

  Widget _buildTypingIndicator() {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: MemoTheme.surfaceAlt,
              borderRadius: const BorderRadius.only(
                topLeft: Radius.circular(16),
                topRight: Radius.circular(16),
                bottomRight: Radius.circular(16),
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                _dot(0),
                const SizedBox(width: 4),
                _dot(400),
                const SizedBox(width: 4),
                _dot(800),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _dot(int delay) {
    return Container(
      width: 8,
      height: 8,
      decoration: BoxDecoration(
        color: MemoTheme.accentMuted.withAlpha(120),
        borderRadius: BorderRadius.circular(4),
      ),
    );
  }
}
