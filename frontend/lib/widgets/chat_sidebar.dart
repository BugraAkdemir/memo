import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/chat.dart';
import '../providers/chat_provider.dart';

/// Chat sidebar — chat list, new chat, incognito toggle.
class ChatSidebar extends ConsumerWidget {
  const ChatSidebar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatListAsync = ref.watch(chatListProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);
    final isIncognito = ref.watch(incognitoProvider);

    return Container(
      width: 260,
      decoration: BoxDecoration(
        color: MemoTheme.bgPanel,
        border: Border(
          right: BorderSide(color: MemoTheme.borderSoft),
        ),
      ),
      child: Column(
        children: [
          // ─── New Chat + Incognito ──────────────────
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              children: [
                Expanded(
                  child: _ActionButton(
                    icon: Icons.add,
                    label: L10n.t('new_chat'),
                    onTap: () async {
                      if (isIncognito) {
                        ref.read(incognitoProvider.notifier).toggle();
                      }
                      final notifier = ref.read(chatListProvider.notifier);
                      final id = await notifier.createNew();
                      ref.read(activeChatIdProvider.notifier).switchTo(id);
                    },
                  ),
                ),
                const SizedBox(width: 8),
                _IconActionButton(
                  icon: isIncognito
                      ? Icons.visibility_off
                      : Icons.visibility_off_outlined,
                  tooltip: L10n.t('incognito_mode'),
                  isActive: isIncognito,
                  onTap: () {
                    ref.read(incognitoProvider.notifier).toggle();
                  },
                ),
              ],
            ),
          ),

          const Divider(height: 1),

          // ─── Chat List ─────────────────────────────
          Expanded(
            child: chatListAsync.when(
              loading: () => const Center(
                child: CircularProgressIndicator(
                  color: MemoTheme.accent,
                  strokeWidth: 2,
                ),
              ),
              error: (e, _) => Center(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(
                    L10n.t('connection_error'),
                    style: TextStyle(color: MemoTheme.textDim, fontSize: 13),
                    textAlign: TextAlign.center,
                  ),
                ),
              ),
              data: (chats) {
                if (chats.isEmpty) {
                  return Center(
                    child: Padding(
                      padding: const EdgeInsets.all(24),
                      child: Text(
                        L10n.t('no_chats'),
                        style:
                            TextStyle(color: MemoTheme.textDim, fontSize: 13),
                      ),
                    ),
                  );
                }

                final activeId = activeChatAsync.valueOrNull ?? '';

                return ListView.builder(
                  padding: const EdgeInsets.symmetric(vertical: 4),
                  itemCount: chats.length,
                  itemBuilder: (context, index) {
                    final chat = chats[index];
                    final isActive = chat.id == activeId && !isIncognito;
                    return _ChatListItem(
                      chat: chat,
                      isActive: isActive,
                      onTap: () {
                        if (isIncognito) {
                          ref.read(incognitoProvider.notifier).toggle();
                        }
                        ref
                            .read(activeChatIdProvider.notifier)
                            .switchTo(chat.id);
                      },
                      onDelete: () async {
                        await ref
                            .read(chatListProvider.notifier)
                            .delete(chat.id);
                      },
                    );
                  },
                );
              },
            ),
          ),

          // ─── Status Bar ────────────────────────────
          const _SidebarStatusBar(),
        ],
      ),
    );
  }
}

class _ChatListItem extends StatefulWidget {
  final ChatSession chat;
  final bool isActive;
  final VoidCallback onTap;
  final VoidCallback onDelete;

  const _ChatListItem({
    required this.chat,
    required this.isActive,
    required this.onTap,
    required this.onDelete,
  });

  @override
  State<_ChatListItem> createState() => _ChatListItemState();
}

class _ChatListItemState extends State<_ChatListItem> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 120),
          margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: widget.isActive
                ? MemoTheme.accentMuted
                : (_hovering ? MemoTheme.bgElement : Colors.transparent),
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            border: widget.isActive
                ? Border.all(color: MemoTheme.accent.withValues(alpha: 0.3))
                : null,
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.chat.title,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight:
                            widget.isActive ? FontWeight.w600 : FontWeight.w400,
                        color: widget.isActive
                            ? MemoTheme.textMain
                            : MemoTheme.textSecondary,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 2),
                    Text(
                      '${widget.chat.msgCount} mesaj · ${widget.chat.updatedAt}',
                      style: TextStyle(
                        fontSize: 11,
                        color: MemoTheme.textDim,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              if (_hovering || widget.isActive)
                GestureDetector(
                  onTap: widget.onDelete,
                  child: Padding(
                    padding: const EdgeInsets.only(left: 4),
                    child: Icon(
                      Icons.close,
                      size: 14,
                      color: MemoTheme.textDim,
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _ActionButton({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: MemoTheme.bgElement,
      borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 10),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(icon, size: 16, color: MemoTheme.textMuted),
              const SizedBox(width: 6),
              Text(
                label,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w500,
                  color: MemoTheme.textMuted,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _IconActionButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final bool isActive;
  final VoidCallback onTap;

  const _IconActionButton({
    required this.icon,
    required this.tooltip,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: isActive
            ? MemoTheme.warmBrown.withValues(alpha: 0.15)
            : MemoTheme.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
          child: SizedBox(
            width: 40,
            height: 40,
            child: Icon(
              icon,
              size: 18,
              color: isActive ? MemoTheme.warmBrown : MemoTheme.textDim,
            ),
          ),
        ),
      ),
    );
  }
}

class _SidebarStatusBar extends ConsumerWidget {
  const _SidebarStatusBar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final connAsync = ref.watch(connectionStatusProvider);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: MemoTheme.borderSoft)),
      ),
      child: Row(
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: connAsync.when(
                loading: () => MemoTheme.textDim,
                error: (_, __) => MemoTheme.red,
                data: (connected) =>
                    connected ? MemoTheme.green : MemoTheme.red,
              ),
            ),
          ),
          const SizedBox(width: 8),
          Text(
            connAsync.when(
              loading: () => '...',
              error: (_, __) => L10n.t('connection_error'),
              data: (connected) => connected ? 'Memo Engine' : L10n.t('connection_error'),
            ),
            style: TextStyle(fontSize: 11, color: MemoTheme.textDim),
          ),
        ],
      ),
    );
  }
}
