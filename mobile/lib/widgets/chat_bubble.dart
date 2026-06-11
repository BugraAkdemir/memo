import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../core/theme.dart';

class ChatBubble extends StatelessWidget {
  final String role;
  final String content;
  final String? thinking;

  const ChatBubble({
    super.key,
    required this.role,
    required this.content,
    this.thinking,
  });

  @override
  Widget build(BuildContext context) {
    final isUser = role == 'user';

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Column(
        crossAxisAlignment:
            isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: [
          if (thinking != null && thinking!.isNotEmpty)
            _buildThinking(context),
          Container(
            constraints: BoxConstraints(
              maxWidth: MediaQuery.of(context).size.width * 0.82,
            ),
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            decoration: BoxDecoration(
              color: isUser ? MemoTheme.accent.withAlpha(30) : MemoTheme.surfaceAlt,
              borderRadius: BorderRadius.only(
                topLeft: const Radius.circular(16),
                topRight: const Radius.circular(16),
                bottomLeft: isUser
                    ? const Radius.circular(16)
                    : const Radius.circular(4),
                bottomRight: isUser
                    ? const Radius.circular(4)
                    : const Radius.circular(16),
              ),
            ),
            child: isUser
                ? Text(
                    content,
                    style: const TextStyle(
                      color: MemoTheme.text,
                      fontSize: 15,
                      height: 1.4,
                    ),
                  )
                : MarkdownBody(
                    data: content,
                    styleSheet: MarkdownStyleSheet(
                      p: const TextStyle(
                        color: MemoTheme.text,
                        fontSize: 15,
                        height: 1.4,
                      ),
                      code: const TextStyle(
                        color: MemoTheme.accent,
                        fontSize: 13,
                        backgroundColor: MemoTheme.bg,
                      ),
                      codeblockDecoration: BoxDecoration(
                        color: MemoTheme.bg,
                        borderRadius: BorderRadius.circular(8),
                      ),
                      blockquoteDecoration: BoxDecoration(
                        border: Border(
                          left: BorderSide(
                            color: MemoTheme.accent.withAlpha(100),
                            width: 3,
                          ),
                        ),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildThinking(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: MemoTheme.accent.withAlpha(12),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: MemoTheme.accent.withAlpha(30),
          width: 0.5,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(
            Icons.psychology_outlined,
            size: 14,
            color: MemoTheme.accentMuted,
          ),
          const SizedBox(width: 6),
          const Text(
            'Thinking...',
            style: TextStyle(
              color: MemoTheme.accentMuted,
              fontSize: 12,
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ),
    );
  }
}
