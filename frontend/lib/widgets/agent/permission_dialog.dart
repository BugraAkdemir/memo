import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/theme.dart';
import '../../models/agent.dart';
import '../../providers/chat_provider.dart';
import '../../utils/tool_names.dart';

class PermissionDialog extends ConsumerStatefulWidget {
  final AgentEvent event;

  const PermissionDialog({super.key, required this.event});

  @override
  ConsumerState<PermissionDialog> createState() => _PermissionDialogState();
}

class _PermissionDialogState extends ConsumerState<PermissionDialog> {
  @override
  void dispose() {
    super.dispose();
  }

  void _submit(String policy) {
    if (widget.event.requestId != null) {
      unawaited(
        ref.read(apiClientProvider).handleAgentPermission(widget.event.requestId!, policy),
      );
    }
    if (mounted) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final isDangerous = widget.event.dangerLevel == 'dangerous';
    final isMedium = widget.event.dangerLevel == 'medium';
    final toolName = ToolNames.displayName(widget.event.toolName);
    final toolIcon = ToolNames.icon(widget.event.toolName);

    String? shortArg;
    try {
      if (widget.event.args is String) {
        final decoded = json.decode(widget.event.args);
        if (decoded is Map && decoded.length == 1) {
          shortArg = decoded.values.first.toString();
        }
      } else if (widget.event.args is Map) {
        final m = widget.event.args as Map;
        if (m.length == 1) {
          shortArg = m.values.first.toString();
        }
      }
    } catch (_) {}

    return AlertDialog(
      title: Row(
        children: [
          Icon(toolIcon, size: 20, color: MemoTheme.accent),
          const SizedBox(width: 8),
          const Text('Izin Gerekli'),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (isDangerous)
            Container(
              padding: const EdgeInsets.all(8),
              margin: const EdgeInsets.only(bottom: 12),
              decoration: BoxDecoration(
                color: MemoTheme.red.withValues(alpha: 0.1),
                border: Border.all(color: MemoTheme.red.withValues(alpha: 0.5)),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(Icons.warning_amber_rounded, size: 16, color: MemoTheme.red),
                  const SizedBox(width: 8),
                  const Expanded(
                    child: Text(
                      'Bu arac sistemde degisiklik yapabilir!',
                      style: TextStyle(color: MemoTheme.red, fontWeight: FontWeight.bold, fontSize: 12),
                    ),
                  ),
                ],
              ),
            ),
          Text(
            '$toolName aracini kullanmak istiyor',
            style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
          ),
          if (shortArg != null)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: MemoTheme.of(context).bgElement,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  shortArg,
                  style: TextStyle(
                    fontFamily: 'monospace',
                    fontSize: 12,
                    color: MemoTheme.of(context).textDim,
                  ),
                ),
              ),
            ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => _submit('deny_once'),
          child: const Text('Reddet', style: TextStyle(color: Colors.grey)),
        ),
        if (isMedium || !isDangerous) ...[
          TextButton(
            onPressed: () => _submit('allow_session'),
            child: Text(
              'Oturum Boyunca Izin Ver',
              style: TextStyle(color: MemoTheme.accent.withValues(alpha: 0.7), fontSize: 12),
            ),
          ),
        ],
        TextButton(
          onPressed: () => _submit('allow_once'),
          style: TextButton.styleFrom(backgroundColor: MemoTheme.accent),
          child: const Text(
            'Izin Ver',
            style: TextStyle(color: Colors.white, fontWeight: FontWeight.w600),
          ),
        ),
      ],
    );
  }
}
