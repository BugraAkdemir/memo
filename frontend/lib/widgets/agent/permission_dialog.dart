import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

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
  bool _canAllow = false;
  Timer? _allowTimer;

  @override
  void initState() {
    super.initState();
    if (widget.event.dangerLevel == 'dangerous') {
      _allowTimer = Timer(const Duration(seconds: 2), () {
        if (mounted) {
          setState(() {
            _canAllow = true;
          });
        }
      });
    } else {
      _canAllow = true;
    }
  }

  @override
  void dispose() {
    _allowTimer?.cancel();
    super.dispose();
  }

  void _submit(String policy) {
    // Fire the API call before closing so the agent pipeline unblocks immediately.
    // The call is not awaited; errors are non-critical (pipeline has its own timeout).
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

    String prettyArgs = '';
    try {
      if (widget.event.args is String) {
        final decoded = json.decode(widget.event.args);
        prettyArgs = const JsonEncoder.withIndent('  ').convert(decoded);
      } else {
        prettyArgs = const JsonEncoder.withIndent('  ').convert(widget.event.args);
      }
    } catch (_) {
      prettyArgs = widget.event.args.toString();
    }

    return AlertDialog(
      title: Row(
        children: [
          Icon(
            isDangerous ? Icons.warning_amber_rounded : (isMedium ? Icons.info_outline : Icons.security),
            color: isDangerous ? MemoTheme.red : (isMedium ? MemoTheme.warningOrange : MemoTheme.green),
          ),
          const SizedBox(width: 8),
          const Text('İzin Gerekli'),
        ],
      ),
      content: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (isDangerous)
              Container(
                padding: const EdgeInsets.all(8),
                margin: const EdgeInsets.only(bottom: 16),
                decoration: BoxDecoration(
                  color: MemoTheme.red.withValues(alpha: 0.1),
                  border: Border.all(color: MemoTheme.red.withValues(alpha: 0.5)),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: const Text(
                  'DİKKAT: Bu araç sisteminizde kalıcı değişiklikler yapabilir!',
                  style: TextStyle(color: MemoTheme.red, fontWeight: FontWeight.bold),
                ),
              ),
            Text('Yapay zeka asistanı aşağıdaki aracı çalıştırmak istiyor:', style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 12),
            Row(
              children: [
                Tooltip(message: ToolNames.description(widget.event.toolName), child: Icon(ToolNames.icon(widget.event.toolName), size: 18, color: Theme.of(context).colorScheme.primary)),
                const SizedBox(width: 8),
                Text(ToolNames.displayName(widget.event.toolName), style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
              ],
            ),
            if (ToolNames.description(widget.event.toolName).isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 4, left: 26),
                child: Text(ToolNames.description(widget.event.toolName), style: TextStyle(fontSize: 12, color: Theme.of(context).colorScheme.onSurfaceVariant)),
              ),
            const SizedBox(height: 12),
            Text('Parametreler:', style: Theme.of(context).textTheme.labelLarge),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(8),
              margin: const EdgeInsets.only(top: 4),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceVariant.withOpacity(0.5),
                borderRadius: BorderRadius.circular(4),
              ),
              child: Text(
                prettyArgs,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
              ),
            ),
            if (widget.event.preview != null && widget.event.preview!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text('Önizleme:', style: Theme.of(context).textTheme.labelLarge),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(8),
                margin: const EdgeInsets.only(top: 4),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.surfaceVariant.withOpacity(0.3),
                  borderRadius: BorderRadius.circular(4),
                  border: Border.all(color: Theme.of(context).colorScheme.primary.withOpacity(0.5)),
                ),
                child: MarkdownBody(
                  data: widget.event.preview!,
                  styleSheet: MarkdownStyleSheet(
                    code: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => _submit('deny_forever'),
          style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
          child: const Text('Kalıcı Reddet'),
        ),
        TextButton(
          onPressed: () => _submit('deny_once'),
          child: const Text('Reddet'),
        ),
        TextButton(
          onPressed: _canAllow ? () => _submit('allow_once') : null,
          style: TextButton.styleFrom(
            backgroundColor: _canAllow ? MemoTheme.accent : MemoTheme.of(context).bgHover,
          ),
          child: Text(
            isDangerous && !_canAllow ? 'Bekleyin...' : 'Bir Kez İzin Ver',
            style: TextStyle(
              color: _canAllow ? Theme.of(context).colorScheme.onPrimaryContainer : Colors.white,
            ),
          ),
        ),
        if (!isDangerous) // Dangerous araçlara kalıcı/oturum izni vermek tehlikeli olabilir
          PopupMenuButton<String>(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 8.0, vertical: 8.0),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text('Daha Fazla', style: TextStyle(color: Theme.of(context).colorScheme.primary)),
                  Icon(Icons.arrow_drop_down, color: Theme.of(context).colorScheme.primary),
                ],
              ),
            ),
            onSelected: _submit,
            itemBuilder: (context) => [
              const PopupMenuItem(
                value: 'allow_session',
                child: Text('Bu oturumda hep izin ver'),
              ),
              const PopupMenuItem(
                value: 'allow_forever',
                child: Text('Kalıcı olarak izin ver'),
              ),
            ],
          ),
      ],
    );
  }
}
