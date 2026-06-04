import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/provider_provider.dart';
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

  void _onPopupResult(PopupResult result) {
    setState(() => _showTemplates = false);

    if (result is PopupInsertText) {
      _controller.text = result.text;
      _controller.selection = TextSelection.fromPosition(
        TextPosition(offset: result.text.length),
      );
      _focusNode.requestFocus();
    } else if (result is PopupModelSwitch) {
      _showModelSwitcher();
    }
  }

  Future<void> _showModelSwitcher() async {
    // Fetch current state
    final api = ref.read(apiClientProvider);
    String activeProvider;
    List<ProviderConfig> providers;

    try {
      activeProvider = await api.getActiveProvider();
      providers = await api.getProviders();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to load providers: $e')),
        );
      }
      return;
    }

    if (!mounted) return;

    // Build options: local + external providers
    final options = <_ModelOption>[];
    options.add(_ModelOption(
      type: 'local',
      name: 'Local Model',
      icon: '🖥️',
      subtitle: 'llama.cpp',
    ));
    for (final p in providers) {
      if (p.enabled) {
        options.add(_ModelOption(
          type: p.type,
          name: p.name,
          icon: providerIcon(p.type),
          subtitle: p.model,
        ));
      }
    }

    if (options.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('No models available')),
      );
      return;
    }

    final selected = await showDialog<String>(
      context: context,
      builder: (ctx) => _ModelSwitcherDialog(
        options: options,
        activeType: activeProvider.isEmpty ? 'local' : activeProvider,
      ),
    );

    if (selected == null || !mounted) return;

    try {
      if (selected == 'local') {
        await api.setActiveProvider('');
        ref.read(activeProviderTypeProvider.notifier).setActive('');
      } else {
        await api.setActiveProvider(selected);
        ref.read(activeProviderTypeProvider.notifier).setActive(selected);
      }
      if (mounted) {
        final name = options.firstWhere((o) => o.type == selected).name;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Switched to $name'),
            duration: const Duration(seconds: 2),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to switch: $e')),
        );
      }
    }
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
            onSelect: _onPopupResult,
            onDismiss: () => setState(() => _showTemplates = false),
          ),

        // Input area
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
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
              const SizedBox(width: 4),
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
              const SizedBox(width: 12),

              // ─── Text Input ──────────────────────────
              Expanded(
                child: Container(
                  constraints: const BoxConstraints(maxHeight: 160),
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
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 14,
                          vertical: 12,
                        ),
                        isDense: true,
                      ),
                    ),
                  ),
                ),
              ),

              const SizedBox(width: 12),

              // ─── Send / Stop Button ──────────────────
              AnimatedContainer(
                duration: const Duration(milliseconds: 150),
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
                          ? Icon(
                              Icons.stop_rounded,
                              size: 22,
                              color: MemoTheme.of(context).textInverse,
                            )
                          : Icon(
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
          child: Icon(icon, size: 20, color: MemoTheme.of(context).textDim),
        ),
      ),
    );
  }
}

// ─── Model Switcher Dialog ──────────────────────────────────────

class _ModelOption {
  final String type;
  final String name;
  final String icon;
  final String subtitle;

  const _ModelOption({
    required this.type,
    required this.name,
    required this.icon,
    required this.subtitle,
  });
}

class _ModelSwitcherDialog extends StatelessWidget {
  final List<_ModelOption> options;
  final String activeType;

  const _ModelSwitcherDialog({
    required this.options,
    required this.activeType,
  });

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Switch Model'),
      content: SizedBox(
        width: 320,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              'Choose which model to use for chat:',
              style: TextStyle(
                fontSize: 13,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            const SizedBox(height: 16),
            ...options.map((opt) => _ModelOptionTile(
              option: opt,
              isActive: opt.type == activeType,
              onTap: () => Navigator.of(context).pop(opt.type),
            )),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
      ],
    );
  }
}

class _ModelOptionTile extends StatelessWidget {
  final _ModelOption option;
  final bool isActive;
  final VoidCallback onTap;

  const _ModelOptionTile({
    required this.option,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      color: isActive
          ? MemoTheme.accent.withValues(alpha: 0.08)
          : null,
      child: ListTile(
        leading: Text(option.icon, style: const TextStyle(fontSize: 24)),
        title: Text(
          option.name,
          style: TextStyle(
            fontWeight: isActive ? FontWeight.w600 : FontWeight.w500,
          ),
        ),
        subtitle: Text(
          option.subtitle,
          style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
        ),
        trailing: isActive
            ? Icon(Icons.check_circle, color: MemoTheme.accent, size: 20)
            : null,
        onTap: onTap,
        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 0),
      ),
    );
  }
}
