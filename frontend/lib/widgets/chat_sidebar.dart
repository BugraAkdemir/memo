import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/chat.dart';
import '../providers/chat_provider.dart';

/// Chat sidebar — chat list, new chat, incognito toggle.
class ChatSidebar extends ConsumerWidget {
   ChatSidebar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatListAsync = ref.watch(chatListProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);
    final isIncognito = ref.watch(incognitoProvider);

    final c = MemoTheme.of(context);
    final glass = c.isGlass;
    return Container(
      width: 260,
      clipBehavior: glass ? Clip.antiAlias : Clip.none,
      decoration: glass
          ? BoxDecoration(
              color: c.bgPanel,
              borderRadius: const BorderRadius.only(
                topLeft: Radius.circular(MemoTheme.radiusLg),
                bottomLeft: Radius.circular(MemoTheme.radiusLg),
              ),
              border: Border.all(color: c.borderSoft),
              boxShadow: MemoTheme.shadowMd,
            )
          : BoxDecoration(
              color: c.bgPanel,
              border: Border(right: BorderSide(color: c.borderSoft)),
            ),
      child: Column(
        children: [
          // ─── New Chat + Incognito ──────────────────
          Padding(
            padding:  EdgeInsets.all(12),
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
                 SizedBox(width: 8),
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

           Divider(height: 1),

          // ─── Chat List ─────────────────────────────
          Expanded(
            child: chatListAsync.when(
              loading: () =>  Center(
                child: CircularProgressIndicator(
                  color: MemoTheme.accent,
                  strokeWidth: 2,
                ),
              ),
              error: (e, _) => Center(
                child: Padding(
                  padding:  EdgeInsets.all(16),
                  child: Text(
                    L10n.t('connection_error'),
                    style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
                    textAlign: TextAlign.center,
                  ),
                ),
              ),
              data: (chats) {
                final normalChats = chats.where((c) => !c.isAgentChat).toList();
                if (normalChats.isEmpty) {
                  return Center(
                    child: Padding(
                      padding:  EdgeInsets.all(24),
                      child: Text(
                        L10n.t('no_chats'),
                        style: TextStyle(
                          color: MemoTheme.of(context).textDim,
                          fontSize: 13,
                        ),
                      ),
                    ),
                  );
                }

                final activeId = activeChatAsync.valueOrNull ?? '';

                return ListView.builder(
                  padding:  EdgeInsets.symmetric(vertical: 4),
                  itemCount: normalChats.length,
                  itemBuilder: (context, index) {
                    final chat = normalChats[index];
                    final isActive = chat.id == activeId && !isIncognito;
                    return _ChatListItem(
                      chat: chat,
                      isActive: isActive,
                      onTap: (id) {
                        if (isIncognito) {
                          ref.read(incognitoProvider.notifier).toggle();
                        }
                        ref
                            .read(activeChatIdProvider.notifier)
                            .switchTo(id);
                      },
                      onDelete: (id) async {
                        ref
                            .read(chatListProvider.notifier)
                            .delete(id);
                      },
                      onRename: (title) async {
                        ref
                            .read(chatListProvider.notifier)
                            .rename(chat.id, title);
                      },
                    );
                  },
                );
              },
            ),
          ),

          // ─── Status Bar ────────────────────────────
           _SidebarStatusBar(),
        ],
      ),
    );
  }
}

class _ChatListItem extends StatefulWidget {
  final ChatSession chat;
  final bool isActive;
  final ValueChanged<String> onTap;
  final ValueChanged<String> onDelete;
  final ValueChanged<String> onRename;

   _ChatListItem({
    required this.chat,
    required this.isActive,
    required this.onTap,
    required this.onDelete,
    required this.onRename,
  });

  @override
  State<_ChatListItem> createState() => _ChatListItemState();
}

class _ChatListItemState extends State<_ChatListItem> {
  bool _hovering = false;
  bool _isEditing = false;
  late TextEditingController _editController;
  late FocusNode _editFocusNode;
  Offset? _secondaryTapPosition;

  @override
  void initState() {
    super.initState();
    _editController = TextEditingController(text: widget.chat.title);
    _editFocusNode = FocusNode();
  }

  @override
  void didUpdateWidget(_ChatListItem old) {
    super.didUpdateWidget(old);
    if (!_isEditing) {
      _editController.text = widget.chat.title;
    }
  }

  @override
  void dispose() {
    _editController.dispose();
    _editFocusNode.dispose();
    super.dispose();
  }

  void _startEditing() {
    setState(() {
      _isEditing = true;
      _editController.text = widget.chat.title;
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _editFocusNode.requestFocus();
    });
  }

  void _finishEditing() {
    final newTitle = _editController.text.trim();
    if (newTitle.isNotEmpty && newTitle != widget.chat.title) {
      widget.onRename(newTitle);
    }
    setState(() => _isEditing = false);
  }

  void _cancelEditing() {
    setState(() {
      _isEditing = false;
      _editController.text = widget.chat.title;
    });
  }

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: _isEditing ? null : () => widget.onTap(widget.chat.id),
        onSecondaryTapDown: (details) {
          _secondaryTapPosition = details.globalPosition;
        },
        onSecondaryTap: _isEditing
            ? null
            : () => _showContextMenu(context),
        child: AnimatedContainer(
          duration:  Duration(milliseconds: 120),
          margin:  EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          padding:  EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: widget.isActive
                ? MemoTheme.accentMuted
                : (_hovering ? MemoTheme.of(context).bgElement : Colors.transparent),
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            border: widget.isActive
                ? Border.all(color: MemoTheme.accent.withValues(alpha: 0.3))
                : null,
          ),
          child: Row(
            children: [
              Expanded(
                child: _isEditing
                    ? SizedBox(
                        height: 30,
                        child: TextField(
                          controller: _editController,
                          focusNode: _editFocusNode,
                          style: TextStyle(
                            fontSize: 13,
                            color: MemoTheme.of(context).textMain,
                          ),
                          decoration: InputDecoration(
                            isDense: true,
                            contentPadding:  EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 4,
                            ),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(4),
                              borderSide: BorderSide(
                                color: MemoTheme.accent,
                              ),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(4),
                              borderSide: BorderSide(
                                color: MemoTheme.accent,
                                width: 1.5,
                              ),
                            ),
                          ),
                          onSubmitted: (_) => _finishEditing(),
                          onTapOutside: (_) => _finishEditing(),
                          onChanged: (_) => setState(() {}),
                        ),
                      )
                    : Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            widget.chat.title,
                            style: TextStyle(
                              fontSize: 13,
                              fontWeight: widget.isActive
                                  ? FontWeight.w600
                                  : FontWeight.w400,
                              color: widget.isActive
                                  ? MemoTheme.of(context).textMain
                                  : MemoTheme.of(context).textSecondary,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                           SizedBox(height: 2),
                          Text(
                            L10n.t('message_count_short', {'count': '${widget.chat.msgCount}', 'date': widget.chat.updatedAt}),
                            style: TextStyle(
                              fontSize: 11,
                              color: MemoTheme.of(context).textDim,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ),
              ),
              if ((_hovering || widget.isActive) && !_isEditing)
                GestureDetector(
                  onTap: () => widget.onDelete(widget.chat.id),
                  child: Padding(
                    padding:  EdgeInsets.only(left: 4),
                    child: Icon(
                      Icons.close,
                      size: 14,
                      color: MemoTheme.of(context).textDim,
                    ),
                  ),
                ),
              if (_isEditing)
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    GestureDetector(
                      onTap: _finishEditing,
                      child: Padding(
                        padding:  EdgeInsets.only(left: 4),
                        child: Icon(
                          Icons.check,
                          size: 14,
                          color: MemoTheme.green,
                        ),
                      ),
                    ),
                    GestureDetector(
                      onTap: _cancelEditing,
                      child: Padding(
                        padding:  EdgeInsets.only(left: 4),
                        child: Icon(
                          Icons.close,
                          size: 14,
                          color: MemoTheme.red,
                        ),
                      ),
                    ),
                  ],
                ),
            ],
          ),
        ),
      ),
    );
  }

  void _showContextMenu(BuildContext context) {
    final pos = _secondaryTapPosition ?? Offset.zero;
    showMenu<String>(
      context: context,
      position: RelativeRect.fromLTRB(
        pos.dx, pos.dy, pos.dx + 1, pos.dy + 1,
      ),
      items: [
        PopupMenuItem(
          value: 'rename',
          child: Row(
            children: [
              Icon(Icons.edit, size: 16, color: MemoTheme.of(context).textMain),
               SizedBox(width: 8),
               Text(L10n.t('rename')),
            ],
          ),
        ),
        PopupMenuItem(
          value: 'delete',
          child: Row(
            children: [
              Icon(Icons.delete, size: 16, color: MemoTheme.red),
               SizedBox(width: 8),
              Text(L10n.t('delete'), style: TextStyle(color: MemoTheme.red)),
            ],
          ),
        ),
      ],
    ).then((value) {
      if (!mounted) return;
      if (value == 'rename') {
        _startEditing();
      } else if (value == 'delete') {
        _confirmDelete(context);
      }
    });
  }

  Future<void> _confirmDelete(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: MemoTheme.of(context).bgPanel,
        title:  Text(L10n.t('delete_chat_title')),
        content: Text(L10n.t('delete_chat_confirm_name', {'title': widget.chat.title})),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
            child:  Text(L10n.t('delete')),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      widget.onDelete(widget.chat.id);
    }
  }
}

class _ActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

   _ActionButton({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: MemoTheme.of(context).bgElement,
      borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: Container(
          padding:  EdgeInsets.symmetric(vertical: 10),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(icon, size: 16, color: MemoTheme.of(context).textMuted),
               SizedBox(width: 6),
              Text(
                label,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w500,
                  color: MemoTheme.of(context).textMuted,
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

   _IconActionButton({
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
            : MemoTheme.of(context).bgElement,
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
              color: isActive ? MemoTheme.warmBrown : MemoTheme.of(context).textDim,
            ),
          ),
        ),
      ),
    );
  }
}

class _SidebarStatusBar extends ConsumerWidget {
   _SidebarStatusBar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final connAsync = ref.watch(connectionStatusProvider);

    return Container(
      padding:  EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: MemoTheme.of(context).borderSoft)),
      ),
      child: Row(
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: connAsync.when(
                loading: () => MemoTheme.of(context).textDim,
                error: (_, __) => MemoTheme.red,
                data: (connected) =>
                    connected ? MemoTheme.green : MemoTheme.red,
              ),
            ),
          ),
           SizedBox(width: 8),
          Text(
            connAsync.when(
              loading: () => '...',
              error: (_, __) => L10n.t('connection_error'),
              data: (connected) =>
                  connected ? L10n.t('engine_status') : L10n.t('connection_error'),
            ),
            style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
          ),
        ],
      ),
    );
  }
}
