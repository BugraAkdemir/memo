import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import 'prompt_templates.dart';

/// Chat input bar — text field, attachment buttons, send button.
class ChatInput extends ConsumerStatefulWidget {
   ChatInput({super.key});

  @override
  ConsumerState<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends ConsumerState<ChatInput> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();
  final _kbFocusNode = FocusNode();
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
    _kbFocusNode.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    if (text.isEmpty) return;

    if (ref.read(isSendingProvider)) return;

    _controller.clear();
    setState(() => _showTemplates = false);

    try {
      await ref.read(messagesProvider.notifier).sendMessage(text);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('${L10n.t('error')}: $e')));
      }
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
          padding:  EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgApp,
            border: Border(top: BorderSide(color: MemoTheme.of(context).borderSoft)),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              // ─── Attachment Buttons ───────────────────
              _InputIconButton(
                icon: Icons.image_outlined,
                tooltip: L10n.t('attach_image'),
                onTap: () async {
                  final isSending = ref.read(isSendingProvider);
                  if (isSending) return;

                  final result = await FilePicker.platform.pickFiles(
                    type: FileType.image,
                    allowMultiple: false,
                  );
                  if (result != null && result.files.single.path != null) {
                    final path = result.files.single.path!;
                    final text = _controller.text.trim();
                    _controller.clear();
                    setState(() => _showTemplates = false);
                    try {
                      await ref
                          .read(messagesProvider.notifier)
                          .sendFile(text, path);
                    } catch (e) {
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('error')}: $e')),
                        );
                      }
                    }
                  }
                },
              ),
               SizedBox(width: 4),
              _InputIconButton(
                icon: Icons.attach_file,
                tooltip: L10n.t('attach_file'),
                onTap: () async {
                  final isSending = ref.read(isSendingProvider);
                  if (isSending) return;

                  final result = await FilePicker.platform.pickFiles(
                    type: FileType.any,
                    allowMultiple: false,
                  );
                  if (result != null && result.files.single.path != null) {
                    final path = result.files.single.path!;
                    final text = _controller.text.trim();
                    _controller.clear();
                    setState(() => _showTemplates = false);
                    try {
                      await ref
                          .read(messagesProvider.notifier)
                          .sendFile(text, path);
                    } catch (e) {
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('error')}: $e')),
                        );
                      }
                    }
                  }
                },
              ),

              /* STT Disabled due to Vosk crashes
               SizedBox(width: 4),
              _InputIconButton(
                icon: Icons.mic_none,
                tooltip: L10n.t('record_audio'),
                onTap: () {
                  // TODO: recording (Faz 8 integration)
                },
              ),
              */
               SizedBox(width: 12),

              // ─── Text Input ──────────────────────────
              Expanded(
                child: Container(
                  constraints:  BoxConstraints(maxHeight: 160),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: KeyboardListener(
                    focusNode: _kbFocusNode,
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
                        color: MemoTheme.of(context).textMain,
                        height: 1.5,
                      ),
                      decoration: InputDecoration(
                        hintText: '${L10n.t('type_message')} (/)',
                        hintStyle: TextStyle(color: MemoTheme.of(context).textDim),
                        border: InputBorder.none,
                        contentPadding:  EdgeInsets.symmetric(
                          horizontal: 14,
                          vertical: 12,
                        ),
                        isDense: true,
                      ),
                    ),
                  ),
                ),
              ),

               SizedBox(width: 12),

              // ─── Send / Stop Button ──────────────────
              AnimatedContainer(
                duration:  Duration(milliseconds: 150),
                child: Material(
                  color: isSending ? MemoTheme.red : MemoTheme.accent,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  child: InkWell(
                    onTap: isSending
                        ? () => ref.read(messagesProvider.notifier).stopStreaming()
                        : _send,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                    child: SizedBox(
                      width: 42,
                      height: 42,
                      child: isSending
                          ?  Icon(
                              Icons.stop_rounded,
                              size: 22,
                              color: MemoTheme.of(context).textInverse,
                            )
                          :  Icon(
                              Icons.send_rounded,
                              size: 20,
                              color: MemoTheme.of(context).textInverse,
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

   _InputIconButton({
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
          padding:  EdgeInsets.all(8),
          child: Icon(icon, size: 20, color: MemoTheme.of(context).textDim),
        ),
      ),
    );
  }
}
