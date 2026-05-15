import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import 'prompt_templates.dart';

/// Chat input bar — text field, attachment buttons, send button.
class ChatInput extends ConsumerStatefulWidget {
  const ChatInput({super.key});

  @override
  ConsumerState<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends ConsumerState<ChatInput> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();
  bool _showTemplates = false;

  @override
  void initState() {
    super.initState();
    _controller.addListener(() {
      final text = _controller.text;
      if (text == '/') {
        setState(() => _showTemplates = true);
      } else if (!text.startsWith('/')) {
        setState(() => _showTemplates = false);
      }
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    if (text.isEmpty) return;

    final isSending = ref.read(isSendingProvider);
    if (isSending) return;

    _controller.clear();
    setState(() => _showTemplates = false);

    ref.read(isSendingProvider.notifier).state = true;

    try {
      await ref.read(messagesProvider.notifier).sendMessage(text);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: $e')),
        );
      }
    } finally {
      ref.read(isSendingProvider.notifier).state = false;
    }

    _focusNode.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    final isSending = ref.watch(isSendingProvider);

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Prompt templates popup
        if (_showTemplates)
          PromptTemplatesPopup(
            onSelect: (template) {
              _controller.text = template;
              _controller.selection = TextSelection.fromPosition(
                TextPosition(offset: template.length),
              );
              setState(() => _showTemplates = false);
              _focusNode.requestFocus();
            },
            onDismiss: () => setState(() => _showTemplates = false),
          ),

        // Input area
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: MemoTheme.bgApp,
            border: Border(
              top: BorderSide(color: MemoTheme.borderSoft),
            ),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              // ─── Attachment Buttons ───────────────────
              _InputIconButton(
                icon: Icons.image_outlined,
                tooltip: L10n.t('attach_image'),
                onTap: () {
                  // TODO: file picker (Faz 8 integration)
                },
              ),
              const SizedBox(width: 4),
              _InputIconButton(
                icon: Icons.attach_file,
                tooltip: L10n.t('attach_file'),
                onTap: () {
                  // TODO: file picker (Faz 8 integration)
                },
              ),
              /* STT Disabled due to Vosk crashes
              const SizedBox(width: 4),
              _InputIconButton(
                icon: Icons.mic_none,
                tooltip: L10n.t('record_audio'),
                onTap: () {
                  // TODO: recording (Faz 8 integration)
                },
              ),
              */

              const SizedBox(width: 12),

              // ─── Text Input ──────────────────────────
              Expanded(
                child: Container(
                  constraints: const BoxConstraints(maxHeight: 160),
                  decoration: BoxDecoration(
                    color: MemoTheme.bgPanel,
                    borderRadius:
                        BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.borderSoft),
                  ),
                  child: KeyboardListener(
                    focusNode: FocusNode(),
                    onKeyEvent: (event) {
                      if (event is KeyDownEvent &&
                          event.logicalKey == LogicalKeyboardKey.enter &&
                          !HardwareKeyboard.instance.isShiftPressed) {
                        _send();
                      }
                    },
                    child: TextField(
                      controller: _controller,
                      focusNode: _focusNode,
                      maxLines: null,
                      textInputAction: TextInputAction.newline,
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.textMain,
                        height: 1.5,
                      ),
                      decoration: InputDecoration(
                        hintText: L10n.t('type_message'),
                        hintStyle: TextStyle(color: MemoTheme.textDim),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(
                            horizontal: 14, vertical: 12),
                        isDense: true,
                      ),
                    ),
                  ),
                ),
              ),

              const SizedBox(width: 12),

              // ─── Send Button ─────────────────────────
              AnimatedContainer(
                duration: const Duration(milliseconds: 150),
                child: Material(
                  color: isSending ? MemoTheme.bgElement : MemoTheme.accent,
                  borderRadius:
                      BorderRadius.circular(MemoTheme.radiusSm),
                  child: InkWell(
                    onTap: isSending ? null : _send,
                    borderRadius:
                        BorderRadius.circular(MemoTheme.radiusSm),
                    child: SizedBox(
                      width: 42,
                      height: 42,
                      child: isSending
                          ? const Padding(
                              padding: EdgeInsets.all(12),
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: MemoTheme.accent,
                              ),
                            )
                          : const Icon(
                              Icons.send_rounded,
                              size: 20,
                              color: MemoTheme.textInverse,
                            ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _InputIconButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;

  const _InputIconButton({
    required this.icon,
    required this.tooltip,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(8),
          child: Icon(icon, size: 20, color: MemoTheme.textDim),
        ),
      ),
    );
  }
}
