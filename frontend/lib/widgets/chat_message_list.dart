import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../core/theme.dart';
import '../models/chat.dart';

/// Scrollable list of chat messages with markdown rendering.
class ChatMessageList extends StatefulWidget {
  final List<ChatMessage> messages;

  const ChatMessageList({super.key, required this.messages});

  @override
  State<ChatMessageList> createState() => _ChatMessageListState();
}

class _ChatMessageListState extends State<ChatMessageList> {
  final ScrollController _scrollController = ScrollController();

  @override
  void didUpdateWidget(ChatMessageList oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.messages.length != oldWidget.messages.length) {
      _scrollToBottom();
    }
  }

  void _scrollToBottom() {
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
    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      itemCount: widget.messages.length,
      itemBuilder: (context, index) {
        final msg = widget.messages[index];
        return _MessageBubble(message: msg);
      },
    );
  }
}

class _MessageBubble extends StatefulWidget {
  final ChatMessage message;

  const _MessageBubble({required this.message});

  @override
  State<_MessageBubble> createState() => _MessageBubbleState();
}

class _MessageBubbleState extends State<_MessageBubble>
    with SingleTickerProviderStateMixin {
  late final AnimationController _fadeController;
  late final Animation<double> _fadeAnimation;
  bool _hovering = false;

  @override
  void initState() {
    super.initState();
    _fadeController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 300),
    );
    _fadeAnimation = CurvedAnimation(
      parent: _fadeController,
      curve: Curves.easeOut,
    );
    _fadeController.forward();
  }

  @override
  void dispose() {
    _fadeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isUser = widget.message.isUser;
    final maxWidth = MediaQuery.of(context).size.width * 0.55;

    return FadeTransition(
      opacity: _fadeAnimation,
      child: SlideTransition(
        position: Tween<Offset>(
          begin: const Offset(0, 0.05),
          end: Offset.zero,
        ).animate(_fadeAnimation),
        child: Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: Row(
            mainAxisAlignment:
                isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (!isUser) ...[
                // ─── Avatar ──────────────────────────
                Container(
                  width: 32,
                  height: 32,
                  margin: const EdgeInsets.only(right: 10, top: 2),
                  decoration: BoxDecoration(
                    color: MemoTheme.accentPale,
                    borderRadius: BorderRadius.circular(10),
                    border:
                        Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
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

              // ─── Bubble ────────────────────────────
              MouseRegion(
                onEnter: (_) => setState(() => _hovering = true),
                onExit: (_) => setState(() => _hovering = false),
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: maxWidth),
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 16, vertical: 12),
                    decoration: BoxDecoration(
                      color:
                          isUser ? MemoTheme.accentMuted : MemoTheme.bgPanel,
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
                        // Message content
                        if (isUser)
                          SelectableText(
                            widget.message.content,
                            style: TextStyle(
                              fontSize: 14,
                              height: 1.6,
                              color: MemoTheme.textMain,
                            ),
                          )
                        else
                          MarkdownBody(
                            data: widget.message.content,
                            selectable: true,
                            styleSheet: MarkdownStyleSheet(
                              p: TextStyle(
                                  fontSize: 14,
                                  height: 1.7,
                                  color: MemoTheme.textMain),
                              strong: TextStyle(
                                  fontWeight: FontWeight.w600,
                                  color: MemoTheme.textMain),
                              em: TextStyle(
                                  fontStyle: FontStyle.italic,
                                  color: MemoTheme.textMuted),
                              code: TextStyle(
                                fontSize: 13,
                                color: MemoTheme.accent,
                                backgroundColor: MemoTheme.bgElement,
                                fontFamily: 'JetBrains Mono',
                              ),
                              codeblockDecoration: BoxDecoration(
                                color: MemoTheme.bgPanel,
                                borderRadius: BorderRadius.circular(
                                    MemoTheme.radiusSm),
                                border:
                                    Border.all(color: MemoTheme.borderSoft),
                              ),
                              codeblockPadding: const EdgeInsets.all(14),
                              blockquoteDecoration: BoxDecoration(
                                border: Border(
                                  left: BorderSide(
                                    color: MemoTheme.borderHover,
                                    width: 3,
                                  ),
                                ),
                              ),
                              blockquotePadding:
                                  const EdgeInsets.only(left: 12),
                              h1: TextStyle(
                                  fontSize: 20,
                                  fontWeight: FontWeight.w600,
                                  color: MemoTheme.textMain),
                              h2: TextStyle(
                                  fontSize: 18,
                                  fontWeight: FontWeight.w600,
                                  color: MemoTheme.textMain),
                              h3: TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w600,
                                  color: MemoTheme.textMain),
                              tableHead: TextStyle(
                                  fontWeight: FontWeight.w600,
                                  color: MemoTheme.textMuted,
                                  fontSize: 13),
                              tableBody: TextStyle(
                                  color: MemoTheme.textMain, fontSize: 13),
                              tableBorder: TableBorder.all(
                                  color: MemoTheme.borderSoft),
                              a: TextStyle(
                                  color: MemoTheme.accent,
                                  decoration: TextDecoration.none),
                            ),
                          ),

                        // Timestamp + copy button
                        if (_hovering || isUser) ...[
                          const SizedBox(height: 6),
                          Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                widget.message.timestamp,
                                style: TextStyle(
                                    fontSize: 10, color: MemoTheme.textDim),
                              ),
                              if (_hovering && !isUser) ...[
                                const SizedBox(width: 8),
                                GestureDetector(
                                  onTap: () {
                                    Clipboard.setData(ClipboardData(
                                        text: widget.message.content));
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      const SnackBar(
                                        content: Text('Kopyalandı'),
                                        duration: Duration(seconds: 1),
                                      ),
                                    );
                                  },
                                  child: Icon(Icons.copy,
                                      size: 12, color: MemoTheme.textDim),
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
        ),
      ),
    );
  }
}
