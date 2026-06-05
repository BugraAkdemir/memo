import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../../models/agent.dart';
import '../../providers/chat_provider.dart';

class PermissionDialog extends ConsumerStatefulWidget {
  final AgentEvent event;

  const PermissionDialog({super.key, required this.event});

  @override
  ConsumerState<PermissionDialog> createState() => _PermissionDialogState();
}

class _PermissionDialogState extends ConsumerState<PermissionDialog> {
  bool _canAllow = false;

  @override
  void initState() {
    super.initState();
    if (widget.event.dangerLevel == 'dangerous') {
      Future.delayed(const Duration(seconds: 2), () {
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

  void _submit(String policy) {
    if (widget.event.requestId != null) {
      ref.read(apiClientProvider).handleAgentPermission(widget.event.requestId!, policy);
    }
    Navigator.of(context).pop();
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
            color: isDangerous ? Colors.red : (isMedium ? Colors.orange : Colors.green),
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
                  color: Colors.red.withOpacity(0.1),
                  border: Border.all(color: Colors.red.withOpacity(0.5)),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: const Text(
                  'DİKKAT: Bu araç sisteminizde kalıcı değişiklikler yapabilir!',
                  style: TextStyle(color: Colors.red, fontWeight: FontWeight.bold),
                ),
              ),
            Text('Yapay zeka asistanı aşağıdaki aracı çalıştırmak istiyor:', style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 12),
            Text('Araç:', style: Theme.of(context).textTheme.labelLarge),
            Text(widget.event.toolName ?? 'Bilinmeyen Araç', style: const TextStyle(fontWeight: FontWeight.bold, fontFamily: 'monospace')),
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
          style: TextButton.styleFrom(foregroundColor: Colors.red),
          child: const Text('Kalıcı Reddet'),
        ),
        TextButton(
          onPressed: () => _submit('deny_once'),
          child: const Text('Reddet'),
        ),
        TextButton(
          onPressed: _canAllow ? () => _submit('allow_once') : null,
          style: TextButton.styleFrom(
            backgroundColor: _canAllow ? Theme.of(context).colorScheme.primaryContainer : Colors.grey,
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
