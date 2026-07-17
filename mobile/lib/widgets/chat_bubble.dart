import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:google_fonts/google_fonts.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import 'branding.dart';

class ChatBubble extends StatelessWidget {
  final String role;
  final String content;
  final String? thinking;
  final bool streaming;

  const ChatBubble({
    super.key,
    required this.role,
    required this.content,
    this.thinking,
    this.streaming = false,
  });

  void _copy(BuildContext context) {
    HapticFeedback.mediumImpact();
    Clipboard.setData(ClipboardData(text: content));
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(L10n.t('copied')), duration: const Duration(milliseconds: 1200)));
  }

  @override
  Widget build(BuildContext context) {
    final isUser = role == 'user';
    return Padding(
      padding: EdgeInsets.only(bottom: 18, left: isUser ? 36 : 0, right: isUser ? 0 : 4),
      child: Column(
        crossAxisAlignment: isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: [
          if (thinking != null && thinking!.isNotEmpty) _ThinkingBlock(text: thinking!),
          Row(
            mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (!isUser) ...[
                const Padding(padding: EdgeInsets.only(top: 2), child: MemoLogo(size: 28)),
                const SizedBox(width: 10),
              ],
              Flexible(
                child: GestureDetector(
                  onLongPress: () => _copy(context),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 15, vertical: 11),
                    decoration: BoxDecoration(
                      color: isUser ? MemoTheme.accent.withValues(alpha: 0.14) : MemoTheme.surface,
                      borderRadius: BorderRadius.only(
                        topLeft: const Radius.circular(16),
                        topRight: const Radius.circular(16),
                        bottomLeft: Radius.circular(isUser ? 16 : 5),
                        bottomRight: Radius.circular(isUser ? 5 : 16),
                      ),
                      border: Border.all(
                        color: isUser ? MemoTheme.accent.withValues(alpha: 0.22) : MemoTheme.border,
                      ),
                    ),
                    child: isUser
                        ? Text(content, style: MemoTheme.body(15, color: MemoTheme.text, height: 1.45))
                        : _AssistantBody(content: content, streaming: streaming),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _AssistantBody extends StatelessWidget {
  final String content;
  final bool streaming;
  const _AssistantBody({required this.content, required this.streaming});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        MarkdownBody(
          data: content.isEmpty ? '…' : content,
          selectable: false,
          styleSheet: MarkdownStyleSheet(
            p: MemoTheme.body(15, color: MemoTheme.text, height: 1.5),
            strong: MemoTheme.body(15, w: FontWeight.w700, color: MemoTheme.text, height: 1.5),
            em: MemoTheme.body(15, color: MemoTheme.text, height: 1.5).copyWith(fontStyle: FontStyle.italic),
            listBullet: MemoTheme.body(15, color: MemoTheme.text, height: 1.5),
            a: MemoTheme.body(15, color: MemoTheme.accentBright, height: 1.5),
            code: GoogleFonts.jetBrainsMono(fontSize: 13, color: MemoTheme.accentBright, backgroundColor: MemoTheme.bg),
            codeblockPadding: const EdgeInsets.all(14),
            codeblockDecoration: BoxDecoration(
              color: MemoTheme.bg,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: MemoTheme.border),
            ),
            blockquoteDecoration: BoxDecoration(
              border: Border(left: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.5), width: 3)),
            ),
            blockquotePadding: const EdgeInsets.only(left: 12),
            h1: MemoTheme.display(20, w: FontWeight.w700),
            h2: MemoTheme.display(17, w: FontWeight.w700),
            h3: MemoTheme.display(15.5, w: FontWeight.w600),
          ),
        ),
        if (streaming)
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Container(width: 8, height: 14, color: MemoTheme.accent.withValues(alpha: 0.7)),
          ),
      ],
    );
  }
}

class _ThinkingBlock extends StatefulWidget {
  final String text;
  const _ThinkingBlock({required this.text});

  @override
  State<_ThinkingBlock> createState() => _ThinkingBlockState();
}

class _ThinkingBlockState extends State<_ThinkingBlock> {
  bool _open = false;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: 38, bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          InkWell(
            onTap: () => setState(() => _open = !_open),
            borderRadius: BorderRadius.circular(8),
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 4, horizontal: 2),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.auto_awesome_outlined, size: 13, color: MemoTheme.textFaint),
                  const SizedBox(width: 6),
                  Text(L10n.t('reasoning'), style: MemoTheme.mono(11.5, color: MemoTheme.textFaint, ls: 0.4)),
                  const SizedBox(width: 4),
                  Icon(_open ? Icons.keyboard_arrow_up_rounded : Icons.keyboard_arrow_down_rounded,
                      size: 16, color: MemoTheme.textFaint),
                ],
              ),
            ),
          ),
          AnimatedCrossFade(
            firstChild: const SizedBox(width: double.infinity),
            secondChild: Container(
              width: double.infinity,
              margin: const EdgeInsets.only(top: 4),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Colors.black.withValues(alpha: 0.18),
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: MemoTheme.border),
              ),
              child: Text(widget.text, style: MemoTheme.body(13, color: MemoTheme.textDim, height: 1.5)),
            ),
            crossFadeState: _open ? CrossFadeState.showSecond : CrossFadeState.showFirst,
            duration: const Duration(milliseconds: 200),
          ),
        ],
      ),
    );
  }
}

/// Animated three-dot indicator shown while the assistant is preparing a reply.
class TypingIndicator extends StatefulWidget {
  const TypingIndicator({super.key});

  @override
  State<TypingIndicator> createState() => _TypingIndicatorState();
}

class _TypingIndicatorState extends State<TypingIndicator> with SingleTickerProviderStateMixin {
  late final AnimationController _c;

  @override
  void initState() {
    super.initState();
    _c = AnimationController(vsync: this, duration: const Duration(milliseconds: 1100))..repeat();
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 18),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(padding: EdgeInsets.only(top: 2), child: MemoLogo(size: 28)),
          const SizedBox(width: 10),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 15),
            decoration: BoxDecoration(
              color: MemoTheme.surface,
              borderRadius: const BorderRadius.only(
                topLeft: Radius.circular(16),
                topRight: Radius.circular(16),
                bottomRight: Radius.circular(16),
                bottomLeft: Radius.circular(5),
              ),
              border: Border.all(color: MemoTheme.border),
            ),
            child: AnimatedBuilder(
              animation: _c,
              builder: (_, _) {
                return Row(
                  mainAxisSize: MainAxisSize.min,
                  children: List.generate(3, (i) {
                    final t = (_c.value - i * 0.18) % 1.0;
                    final lift = (t < 0.5 ? t : 1 - t) * 2; // 0..1
                    return Padding(
                      padding: EdgeInsets.only(right: i < 2 ? 5 : 0),
                      child: Transform.translate(
                        offset: Offset(0, -3 * lift),
                        child: Container(
                          width: 7,
                          height: 7,
                          decoration: BoxDecoration(
                            color: MemoTheme.accent.withValues(alpha: 0.45 + 0.45 * lift),
                            shape: BoxShape.circle,
                          ),
                        ),
                      ),
                    );
                  }),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
