import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../core/theme.dart';
import '../models/chat.dart';
import '../models/agent.dart';
import 'agent/agent_chat_card.dart';
import '../core/l10n.dart';

MarkdownStyleSheet _buildMarkdownStyleSheet(BuildContext context) {
  final c = MemoTheme.of(context);
  return MarkdownStyleSheet(
    p: TextStyle(fontSize: 14, height: 1.7, color: c.textMain),
    strong: TextStyle(fontWeight: FontWeight.w600, color: c.textMain),
    em: TextStyle(fontStyle: FontStyle.italic, color: c.textMuted),
    code: TextStyle(
      fontSize: 13,
      color: MemoTheme.accent,
      backgroundColor: c.bgElement,
      fontFamily: 'JetBrains Mono',
    ),
    codeblockDecoration: BoxDecoration(
      color: c.bgPanel,
      borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
      border: Border.all(color: c.borderSoft),
    ),
    codeblockPadding:  EdgeInsets.all(14),
    blockquoteDecoration: BoxDecoration(
      border: Border(left: BorderSide(color: c.borderHover, width: 3)),
    ),
    blockquotePadding:  EdgeInsets.only(left: 12),
    h1: TextStyle(fontSize: 20, fontWeight: FontWeight.w600, color: c.textMain),
    h2: TextStyle(fontSize: 18, fontWeight: FontWeight.w600, color: c.textMain),
    h3: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: c.textMain),
    tableHead: TextStyle(fontWeight: FontWeight.w600, color: c.textMuted, fontSize: 13),
    tableBody: TextStyle(color: c.textMain, fontSize: 13),
    tableBorder: TableBorder.all(color: c.borderSoft),
    a: TextStyle(color: MemoTheme.accent, decoration: TextDecoration.none),
  );
}

/// Scrollable list of chat messages with markdown rendering.
class ChatMessageList extends StatefulWidget {
  final List<ChatMessage> messages;
  final bool isTyping;
  final String streamingContent;
  final String streamingThinking;
  final List<AgentEvent>? streamingAgentEvents;
  final void Function(int index, String newContent)? onEdit;
  final void Function(int index)? onDelete;

   ChatMessageList({
    super.key,
    required this.messages,
    this.isTyping = false,
    this.streamingContent = '',
    this.streamingThinking = '',
    this.streamingAgentEvents,
    this.onEdit,
    this.onDelete,
  });

  @override
  State<ChatMessageList> createState() => _ChatMessageListState();
}

class _ChatMessageListState extends State<ChatMessageList> {
  final ScrollController _scrollController = ScrollController();

  @override
  void didUpdateWidget(ChatMessageList oldWidget) {
    super.didUpdateWidget(oldWidget);
    final messagesChanged = widget.messages.length != oldWidget.messages.length;
    final streamingLenChanged =
        widget.streamingContent.length != oldWidget.streamingContent.length;
    final typingStarted = widget.isTyping && !oldWidget.isTyping;

    if (messagesChanged || streamingLenChanged || typingStarted) {
      _scrollToBottom();
    }
  }

  bool _isNearBottom() {
    if (!_scrollController.hasClients) return true;
    final pos = _scrollController.position;
    return pos.maxScrollExtent - pos.pixels < 50;
  }

  void _scrollToBottom() {
    if (!_isNearBottom()) return; // don't yank if user scrolled up
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration:  Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final hasStreaming = widget.streamingContent.isNotEmpty || (widget.streamingAgentEvents != null && widget.streamingAgentEvents!.isNotEmpty);
    final showTyping = widget.isTyping && !hasStreaming;
    final itemCount =
        widget.messages.length + (hasStreaming ? 1 : 0) + (showTyping ? 1 : 0);

    return ListView.builder(
      controller: _scrollController,
      padding:  EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      itemCount: itemCount,
      itemBuilder: (context, index) {
        // Regular messages — use key + RepaintBoundary so Flutter only rebuilds
        // the widget whose content actually changed.
        if (index < widget.messages.length) {
          final msg = widget.messages[index];
          return RepaintBoundary(
            child: _MessageBubble(
              key: ValueKey('msg_${msg.hashCode}_$index'),
              message: msg,
              index: index,
              onEdit: widget.onEdit,
              onDelete: widget.onDelete,
            ),
          );
        }
        // Streaming message — rendered as a virtual assistant bubble
        if (hasStreaming) {
          return _StreamingBubble(
            content: widget.streamingContent,
            thinking: widget.streamingThinking,
            agentEvents: widget.streamingAgentEvents,
          );
        }
        // Typing indicator — shown before first token arrives
        return  _TypingIndicator();
      },
    );
  }
}

class _MessageBubble extends StatefulWidget {
  final ChatMessage message;
  final int index;
  final void Function(int index, String newContent)? onEdit;
  final void Function(int index)? onDelete;

   _MessageBubble({
    super.key,
    required this.message,
    required this.index,
    this.onEdit,
    this.onDelete,
  });

  @override
  State<_MessageBubble> createState() => _MessageBubbleState();
}

class _MessageBubbleState extends State<_MessageBubble> {
  bool _hovering = false;
  bool _thinkingExpanded = false;
  Offset _tapPosition = Offset.zero;

  void _showContextMenu() {
    final renderBox = context.findRenderObject() as RenderBox;
    final position = renderBox.localToGlobal(_tapPosition);
    showMenu<String>(
      context: context,
      position: RelativeRect.fromLTRB(
        position.dx,
        position.dy,
        position.dx + 1,
        position.dy + 1,
      ),
      items: [
         PopupMenuItem(value: 'edit', child: Text(L10n.t('edit'))),
         PopupMenuItem(value: 'delete', child: Text(L10n.t('delete'))),
      ],
    ).then((value) {
      if (value == 'edit') {
        _showEditDialog();
      } else if (value == 'delete') {
        _showDeleteConfirm();
      }
    });
  }

  Future<void> _showEditDialog() async {
    final controller = TextEditingController(text: widget.message.content);
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title:  Text(L10n.t('edit_message')),
        content: TextField(
          controller: controller,
          decoration:  InputDecoration(
            hintText: L10n.t('edit_message_hint'),
            border: OutlineInputBorder(),
          ),
          minLines: 3,
          maxLines: 10,
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child:  Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(controller.text.trim()),
            child:  Text(L10n.t('save')),
          ),
        ],
      ),
    );
    controller.dispose();
    if (result != null && result.isNotEmpty) {
      widget.onEdit?.call(widget.index, result);
    }
  }

  Future<void> _showDeleteConfirm() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title:  Text('Mesajı Sil'),
        content:  Text('Bu mesaj silinecek. Devam etmek istiyor musunuz?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child:  Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            style: FilledButton.styleFrom(backgroundColor: Colors.red),
            child:  Text(L10n.t('delete')),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      widget.onDelete?.call(widget.index);
    }
  }

  @override
  Widget build(BuildContext context) {
    final isUser = widget.message.isUser;
    final maxWidth = MediaQuery.of(context).size.width * 0.55;

    return Padding(
      padding:  EdgeInsets.only(bottom: 12),
      child: Row(
        mainAxisAlignment: isUser
            ? MainAxisAlignment.end
            : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (!isUser) ...[
            Container(
              width: 32,
              height: 32,
              margin:  EdgeInsets.only(right: 10, top: 2),
              decoration: BoxDecoration(
                color: MemoTheme.accentPale,
                borderRadius: BorderRadius.circular(10),
                border: Border.all(
                  color: MemoTheme.accent.withValues(alpha: 0.3),
                ),
              ),
              child:  Center(
                child: Text(
                  'M',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.accent,
                  ),
                ),
              ),
            ),
          ],
          MouseRegion(
            onEnter: (_) => setState(() => _hovering = true),
            onExit: (_) => setState(() => _hovering = false),
            child: GestureDetector(
              onSecondaryTapDown: (details) {
                setState(() => _tapPosition = details.localPosition);
              },
              onSecondaryTap: _showContextMenu,
              onLongPress: () {
                setState(() => _tapPosition = Offset.zero);
                _showContextMenu();
              },
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: maxWidth),
                child: Container(
                  padding:  EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 12,
                  ),
                  decoration: BoxDecoration(
                    color: isUser ? MemoTheme.accentMuted : MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(
                      color: isUser
                          ? MemoTheme.accent.withValues(alpha: 0.2)
                          : MemoTheme.of(context).borderSoft,
                    ),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (!isUser && widget.message.hasThinking)
                        _ThinkingToggle(
                          thinking: widget.message.thinking!,
                          expanded: _thinkingExpanded,
                          onToggle: () => setState(
                            () => _thinkingExpanded = !_thinkingExpanded,
                          ),
                        ),
                      if (!isUser && widget.message.agentEvents != null)
                        ...widget.message.agentEvents!.map((e) {
                          if (e is AgentEvent) {
                            return AgentChatCard(event: e);
                          }
                          if (e is Map<String, dynamic>) {
                            return AgentChatCard(event: AgentEvent.fromJson(e));
                          }
                          return const SizedBox.shrink();
                        }),
                      if (widget.message.content.isNotEmpty)
                        MarkdownBody(
                          data: widget.message.content,
                          selectable: true,
                          styleSheet: _buildMarkdownStyleSheet(context),
                        ),
                      if (_hovering || isUser) ...[
                         SizedBox(height: 6),
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(
                              widget.message.timestamp,
                              style:  TextStyle(
                                fontSize: 10,
                                color: MemoTheme.of(context).textDim,
                              ),
                            ),
                            if (_hovering && !isUser) ...[
                               SizedBox(width: 8),
                              GestureDetector(
                                onTap: () {
                                  Clipboard.setData(
                                    ClipboardData(
                                      text: widget.message.content,
                                    ),
                                  );
                                  ScaffoldMessenger.of(context).showSnackBar(
                                     SnackBar(
                                      content: Text('Kopyalandı'),
                                      duration: Duration(seconds: 1),
                                    ),
                                  );
                                },
                                child:  Icon(
                                  Icons.copy,
                                  size: 12,
                                  color: MemoTheme.of(context).textDim,
                                ),
                              ),
                            ],
                          ],
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Streaming assistant bubble — rendered while tokens arrive.
/// No animations (no FadeTransition) to avoid jank on every chunk.
class _StreamingBubble extends StatefulWidget {
  final String content;
  final String thinking;
  final List<AgentEvent>? agentEvents;

   _StreamingBubble({required this.content, this.thinking = '', this.agentEvents});

  @override
  State<_StreamingBubble> createState() => _StreamingBubbleState();
}

class _StreamingBubbleState extends State<_StreamingBubble> {
  bool _thinkingExpanded = false;

  @override
  Widget build(BuildContext context) {
    final maxWidth = MediaQuery.of(context).size.width * 0.55;

    return Padding(
      padding:  EdgeInsets.only(bottom: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 32,
            height: 32,
            margin:  EdgeInsets.only(right: 10, top: 2),
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: MemoTheme.accent.withValues(alpha: 0.3),
              ),
            ),
            child:  Center(
              child: Text(
                'M',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.accent,
                ),
              ),
            ),
          ),
          ConstrainedBox(
            constraints: BoxConstraints(maxWidth: maxWidth),
            child: Container(
              padding:  EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              decoration: BoxDecoration(
                color: MemoTheme.of(context).bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                border: Border.all(color: MemoTheme.of(context).borderSoft),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (widget.thinking.isNotEmpty)
                    _ThinkingToggle(
                      thinking: widget.thinking,
                      expanded: _thinkingExpanded,
                      onToggle: () => setState(
                        () => _thinkingExpanded = !_thinkingExpanded,
                      ),
                    ),
                  if (widget.agentEvents != null)
                    ...widget.agentEvents!.map((e) => AgentChatCard(event: e)),
                  if (widget.content.isNotEmpty)
                    MarkdownBody(
                      data: widget.content,
                      selectable: true,
                      styleSheet: _buildMarkdownStyleSheet(context),
                    ),
                   SizedBox(height: 6),
                  Text(
                    DateTime.now().toIso8601String().substring(11, 16),
                    style:  TextStyle(
                      fontSize: 10,
                      color: MemoTheme.of(context).textDim,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Collapsible thinking section shown inside assistant message bubbles.
class _ThinkingToggle extends StatelessWidget {
  final String thinking;
  final bool expanded;
  final VoidCallback onToggle;

   _ThinkingToggle({
    required this.thinking,
    required this.expanded,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding:  EdgeInsets.only(bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Toggle pill
          GestureDetector(
            onTap: onToggle,
            child: Container(
              padding:  EdgeInsets.symmetric(horizontal: 10, vertical: 5),
              decoration: BoxDecoration(
                color: MemoTheme.of(context).bgElement,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: MemoTheme.of(context).borderSoft),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    expanded ? Icons.expand_more : Icons.chevron_right,
                    size: 16,
                    color: MemoTheme.of(context).textMuted,
                  ),
                   SizedBox(width: 4),
                  Text(
                    expanded ? 'Düşünme gizle' : 'Düşünme göster',
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
                      color: MemoTheme.of(context).textMuted,
                    ),
                  ),
                ],
              ),
            ),
          ),

          // Expanded thinking content
          if (expanded) ...[
             SizedBox(height: 8),
            Container(
              width: double.infinity,
              padding:  EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: MemoTheme.of(context).bgElement.withValues(alpha: 0.4),
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: MemoTheme.of(context).borderSoft),
              ),
              child: SelectableText(
                thinking,
                style: TextStyle(
                  fontSize: 13,
                  height: 1.5,
                  color: MemoTheme.of(context).textMuted,
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
             SizedBox(height: 8),
          ],
        ],
      ),
    );
  }
}

class _TypingIndicator extends StatelessWidget {
   _TypingIndicator();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding:  EdgeInsets.only(bottom: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Avatar
          Container(
            width: 32,
            height: 32,
            margin:  EdgeInsets.only(right: 10, top: 2),
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: MemoTheme.accent.withValues(alpha: 0.3),
              ),
            ),
            child:  Center(
              child: Text(
                'M',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.accent,
                ),
              ),
            ),
          ),
          // Bubble
          Container(
            padding:  EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            decoration: BoxDecoration(
              color: MemoTheme.of(context).bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: MemoTheme.of(context).borderSoft),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: MemoTheme.accent,
                  ),
                ),
                 SizedBox(width: 10),
                Text(
                  'Düşünüyor...',
                  style: TextStyle(
                    fontSize: 13,
                    fontStyle: FontStyle.italic,
                    color: MemoTheme.of(context).textMuted,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
