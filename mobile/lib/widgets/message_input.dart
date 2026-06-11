import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/theme.dart';
import '../providers/connection_provider.dart';

class MessageInput extends ConsumerStatefulWidget {
  final void Function(String text) onSend;
  final bool enabled;

  const MessageInput({
    super.key,
    required this.onSend,
    this.enabled = true,
  });

  @override
  ConsumerState<MessageInput> createState() => _MessageInputState();
}

class _MessageInputState extends ConsumerState<MessageInput> {
  late final TextEditingController _ctrl;
  late final FocusNode _focus;

  @override
  void initState() {
    super.initState();
    _ctrl = TextEditingController();
    _focus = FocusNode();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _send() {
    final text = _ctrl.text.trim();
    if (text.isEmpty || !widget.enabled) return;

    if (text == '/model') {
      _showModelSwitcher();
      _ctrl.clear();
      return;
    }

    widget.onSend(text);
    _ctrl.clear();
  }

  Future<void> _showModelSwitcher() async {
    final api = ref.read(apiClientProvider);
    String active;
    List<ProviderConfig> providers;
    try {
      active = await api.getActiveProvider();
      providers = await api.getProviders();
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Failed to fetch providers')),
        );
      }
      return;
    }

    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => _ModelSwitcherDialog(
        active: active,
        providers: providers,
      ),
    );

    if (result != null && mounted) {
      try {
        await api.setActiveProvider(result);
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                result.isEmpty
                    ? 'Switched to local model'
                    : 'Switched to $result',
              ),
            ),
          );
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Error: $e')),
          );
        }
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 8, 8, 12),
      decoration: const BoxDecoration(
        color: MemoTheme.surface,
        border: Border(
          top: BorderSide(color: MemoTheme.border, width: 0.5),
        ),
      ),
      child: SafeArea(
        top: false,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(
              child: TextField(
                controller: _ctrl,
                focusNode: _focus,
                enabled: widget.enabled,
                maxLines: 5,
                minLines: 1,
                textInputAction: TextInputAction.send,
                onSubmitted: (_) => _send(),
                decoration: const InputDecoration(
                  hintText: 'Message... (/model)',
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.vertical(
                      top: Radius.circular(12),
                    ),
                  ),
                  contentPadding: EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 12,
                  ),
                ),
              ),
            ),
            const SizedBox(width: 6),
            Container(
              margin: const EdgeInsets.only(bottom: 2),
              decoration: BoxDecoration(
                color: MemoTheme.accent,
                borderRadius: BorderRadius.circular(12),
              ),
              child: IconButton(
                onPressed: widget.enabled ? _send : null,
                icon: const Icon(Icons.arrow_upward),
                color: MemoTheme.bg,
                style: IconButton.styleFrom(
                  backgroundColor: MemoTheme.accent,
                  disabledBackgroundColor: MemoTheme.border,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ModelSwitcherDialog extends StatelessWidget {
  final String active;
  final List<ProviderConfig> providers;

  const _ModelSwitcherDialog({
    required this.active,
    required this.providers,
  });

  @override
  Widget build(BuildContext context) {
    final enabled = providers.where((p) => p.enabled).toList();

    return AlertDialog(
      backgroundColor: MemoTheme.surface,
      title: const Text('Model Değiştir'),
      content: SizedBox(
        width: double.maxFinite,
        child: ListView(
          shrinkWrap: true,
          children: [
            _optionTile(
              context,
              icon: Icons.computer,
              label: 'Local Model',
              subtitle: 'Local llama.cpp',
              isActive: active.isEmpty,
              value: '',
            ),
            ...enabled.map(
              (p) => _optionTile(
                context,
                icon: Icons.cloud,
                label: p.name,
                subtitle: p.model,
                isActive: active == p.type,
                value: p.type,
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Cancel'),
        ),
      ],
    );
  }

  Widget _optionTile(
    BuildContext context, {
    required IconData icon,
    required String label,
    required String subtitle,
    required bool isActive,
    required String value,
  }) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: InkWell(
        onTap: () => Navigator.pop(context, value),
        borderRadius: BorderRadius.circular(10),
        child: Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: isActive ? MemoTheme.accent.withAlpha(20) : null,
            borderRadius: BorderRadius.circular(10),
            border: isActive
                ? Border.all(color: MemoTheme.accent.withAlpha(60))
                : null,
          ),
          child: Row(
            children: [
              Icon(icon, size: 22, color: MemoTheme.accent),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      label,
                      style: TextStyle(
                        fontWeight:
                            isActive ? FontWeight.w600 : FontWeight.w400,
                        color: MemoTheme.text,
                      ),
                    ),
                    Text(
                      subtitle,
                      style: const TextStyle(
                        fontSize: 12,
                        color: MemoTheme.textDim,
                      ),
                    ),
                  ],
                ),
              ),
              if (isActive)
                const Icon(Icons.check, size: 18, color: MemoTheme.accent),
            ],
          ),
        ),
      ),
    );
  }
}
