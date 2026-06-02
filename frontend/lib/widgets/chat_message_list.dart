import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../core/theme.dart';
import '../models/chat.dart';

final _markdownStyleSheet = MarkdownStyleSheet(
  p: const TextStyle(fontSize: 14, height: 1.7, color: MemoTheme.textMain),
  strong: const TextStyle(
    fontWeight: FontWeight.w600,
    color: MemoTheme.textMain,
  ),
  em: const TextStyle(fontStyle: FontStyle.italic, color: MemoTheme.textMuted),
  code: TextStyle(
    fontSize: 13,
    color: MemoTheme.accent,
    backgroundColor: MemoTheme.bgElement,
    fontFamily: 'JetBrains Mono',
  ),
  codeblockDecoration: BoxDecoration(
    color: MemoTheme.bgPanel,
    borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
    border: Border.all(color: MemoTheme.borderSoft),
  ),
  codeblockPadding: const EdgeInsets.all(14),
  blockquoteDecoration: BoxDecoration(
    border: Border(left: BorderSide(color: MemoTheme.borderHover, width: 3)),
  ),
  blockquotePadding: const EdgeInsets.only(left: 12),
  h1: const TextStyle(
    fontSize: 20,
    fontWeight: FontWeight.w600,
    color: MemoTheme.textMain,
  ),
  h2: const TextStyle(
    fontSize: 18,
    fontWeight: FontWeight.w600,
    color: MemoTheme.textMain,
  ),
  h3: const TextStyle(
    fontSize: 16,
    fontWeight: FontWeight.w600,
    color: MemoTheme.textMain,
  ),
  tableHead: const TextStyle(
    fontWeight: FontWeight.w600,
    color: MemoTheme.textMuted,
    fontSize: 13,
  ),
  tableBody: const TextStyle(color: MemoTheme.textMain, fontSize: 13),
  tableBorder: TableBorder.all(color: MemoTheme.borderSoft),
  a: const TextStyle(color: MemoTheme.accent, decoration: TextDecoration.none),
);

/// Scrollable list of chat messages with markdown rendering.
class ChatMessageList extends StatefulWidget {
  final List<ChatMessage> messages;
  final bool isTyping;
  final String streamingContent;
  final String streamingThinking;

  const ChatMessageList({
    super.key,
    required this.messages,
    this.isTyping = false,
    this.streamingContent = '',
    this.streamingThinking = '',
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
          duration: const Duration(milliseconds: 300),
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
    final hasStreaming = widget.streamingContent.isNotEmpty;
    final showTyping = widget.isTyping && !hasStreaming;
    final itemCount =
        widget.messages.length + (hasStreaming ? 1 : 0) + (showTyping ? 1 : 0);

    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
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
            ),
          );
        }
        // Streaming message — rendered as a virtual assistant bubble
        if (hasStreaming) {
          return _StreamingBubble(
            content: widget.streamingContent,
            thinking: widget.streamingThinking,
          );
        }
        // Typing indicator — shown before first token arrives
        return const _TypingIndicator();
      },
    );
  }
}

class _MessageBubble extends StatefulWidget {
  final ChatMessage message;

  const _MessageBubble({super.key, required this.message});

  @override
  State<_MessageBubble> createState() => _MessageBubbleState();
}

class _MessageBubbleState extends State<_MessageBubble> {
  bool _hovering = false;
  bool _thinkingExpanded = false;

  @override
  Widget build(BuildContext context) {
    final isUser = widget.message.isUser;
    final maxWidth = MediaQuery.of(context).size.width * 0.55;

    return Padding(
          padding: const EdgeInsets.only(bottom: 12),
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
                  margin: const EdgeInsets.only(right: 10, top: 2),
                  decoration: BoxDecoration(
                    color: MemoTheme.accentPale,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(
                      color: MemoTheme.accent.withValues(alpha: 0.3),
                    ),
                  ),
                  child: const Center(
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
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: maxWidth),
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 12,
                    ),
                    decoration: BoxDecoration(
                      color: isUser ? MemoTheme.accentMuted : MemoTheme.bgPanel,
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      border: Border.all(
                        color: isUser
                            ? MemoTheme.accent.withValues(alpha: 0.2)
                            : MemoTheme.borderSoft,
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
                        if (isUser)
                          SelectableText(
                            widget.message.content,
                            style: const TextStyle(
                              fontSize: 14,
                              height: 1.6,
                              color: MemoTheme.textMain,
                            ),
                          )
                        else
                          MarkdownBody(
                            data: widget.message.content,
                            selectable: true,
                            styleSheet: _markdownStyleSheet,
                          ),
                        if (_hovering || isUser) ...[
                          const SizedBox(height: 6),
                          Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                widget.message.timestamp,
                                style: const TextStyle(
                                  fontSize: 10,
                                  color: MemoTheme.textDim,
                                ),
                              ),
                              if (_hovering && !isUser) ...[
                                const SizedBox(width: 8),
                                GestureDetector(
                                  onTap: () {
                                    Clipboard.setData(
                                      ClipboardData(
                                        text: widget.message.content,
                                      ),
                                    );
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      const SnackBar(
                                        content: Text('Kopyalandı'),
                                        duration: Duration(seconds: 1),
                                      ),
                                    );
                                  },
                                  child: const Icon(
                                    Icons.copy,
                                    size: 12,
                                    color: MemoTheme.textDim,
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

  const _StreamingBubble({required this.content, this.thinking = ''});

  @override
  State<_StreamingBubble> createState() => _StreamingBubbleState();
}

class _StreamingBubbleState extends State<_StreamingBubble> {
  bool _thinkingExpanded = false;

  @override
  Widget build(BuildContext context) {
    final maxWidth = MediaQuery.of(context).size.width * 0.55;

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 32,
            height: 32,
            margin: const EdgeInsets.only(right: 10, top: 2),
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: MemoTheme.accent.withValues(alpha: 0.3),
              ),
            ),
            child: const Center(
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
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              decoration: BoxDecoration(
                color: MemoTheme.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                border: Border.all(color: MemoTheme.borderSoft),
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
                  MarkdownBody(
                    data: widget.content,
                    selectable: true,
                    styleSheet: _markdownStyleSheet,
                  ),
                  const SizedBox(height: 6),
                  Text(
                    DateTime.now().toIso8601String().substring(11, 16),
                    style: const TextStyle(
                      fontSize: 10,
                      color: MemoTheme.textDim,
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

  const _ThinkingToggle({
    required this.thinking,
    required this.expanded,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Toggle pill
          GestureDetector(
            onTap: onToggle,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
              decoration: BoxDecoration(
                color: MemoTheme.bgElement,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: MemoTheme.borderSoft),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    expanded ? Icons.expand_more : Icons.chevron_right,
                    size: 16,
                    color: MemoTheme.textMuted,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    expanded ? 'Düşünme gizle' : 'Düşünme göster',
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
                      color: MemoTheme.textMuted,
                    ),
                  ),
                ],
              ),
            ),
          ),

          // Expanded thinking content
          if (expanded) ...[
            const SizedBox(height: 8),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: MemoTheme.bgElement.withValues(alpha: 0.4),
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: MemoTheme.borderSoft),
              ),
              child: SelectableText(
                thinking,
                style: TextStyle(
                  fontSize: 13,
                  height: 1.5,
                  color: MemoTheme.textMuted,
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
            const SizedBox(height: 8),
          ],
        ],
      ),
    );
  }
}

class _TypingIndicator extends StatelessWidget {
  const _TypingIndicator();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Avatar
          Container(
            width: 32,
            height: 32,
            margin: const EdgeInsets.only(right: 10, top: 2),
            decoration: BoxDecoration(
              color: MemoTheme.accentPale,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: MemoTheme.accent.withValues(alpha: 0.3),
              ),
            ),
            child: const Center(
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
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            decoration: BoxDecoration(
              color: MemoTheme.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: MemoTheme.borderSoft),
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
                const SizedBox(width: 10),
                Text(
                  'Düşünüyor...',
                  style: TextStyle(
                    fontSize: 13,
                    fontStyle: FontStyle.italic,
                    color: MemoTheme.textMuted,
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
