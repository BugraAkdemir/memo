import 'dart:convert';
import 'package:flutter/material.dart';
import '../../core/theme.dart';
import '../../models/agent.dart';
import '../../utils/tool_names.dart';

class AgentChatCard extends StatelessWidget {
  final AgentEvent event;

  const AgentChatCard({super.key, required this.event});

  @override
  Widget build(BuildContext context) {
    bool isError = event.type == 'tool_error';
    bool isDenied = event.type == 'permission_denied';
    bool isExecuting = event.type == 'tool_executing' || event.type == 'permission_request';
    
    Color borderColor = Theme.of(context).colorScheme.outlineVariant;
    Color iconColor = Theme.of(context).colorScheme.onSurfaceVariant;
    IconData icon = Icons.build_circle_outlined;
    String statusText = 'Tamamlandı';

    if (isError || isDenied) {
      borderColor = MemoTheme.red.withValues(alpha: 0.5);
      iconColor = MemoTheme.red;
      icon = isDenied ? Icons.block : Icons.error_outline;
      statusText = isDenied ? 'Reddedildi' : 'Hata';
    } else if (isExecuting) {
      borderColor = Theme.of(context).colorScheme.primary.withOpacity(0.5);
      iconColor = Theme.of(context).colorScheme.primary;
      icon = Icons.sync;
      statusText = event.type == 'permission_request' ? 'İzin Bekleniyor...' : 'Çalışıyor...';
    }

    String prettyArgs = '';
    try {
      if (event.args is String) {
        final decoded = json.decode(event.args);
        prettyArgs = const JsonEncoder.withIndent('  ').convert(decoded);
      } else if (event.args != null) {
        prettyArgs = const JsonEncoder.withIndent('  ').convert(event.args);
      }
    } catch (_) {
      prettyArgs = event.args.toString();
    }

    return Container(
      margin: const EdgeInsets.only(top: 8, bottom: 8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: borderColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surfaceVariant.withOpacity(0.3),
              borderRadius: const BorderRadius.only(topLeft: Radius.circular(11), topRight: Radius.circular(11)),
            ),
            child: Row(
              children: [
                if (isExecuting && event.type != 'permission_request')
                  SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2, color: iconColor),
                  )
                else
                  Icon(icon, size: 16, color: iconColor),
                const SizedBox(width: 8),
                Icon(ToolNames.icon(event.toolName), size: 16, color: iconColor),
                const SizedBox(width: 4),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        ToolNames.displayName(event.toolName),
                        style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13),
                        overflow: TextOverflow.ellipsis,
                      ),
                      if (ToolNames.description(event.toolName).isNotEmpty)
                        Text(
                          ToolNames.description(event.toolName),
                          style: TextStyle(fontSize: 10, color: iconColor.withValues(alpha: 0.7)),
                          overflow: TextOverflow.ellipsis,
                        ),
                    ],
                  ),
                ),
                Text(
                  event.durationMs != null ? '$statusText (${event.durationMs}ms)' : statusText,
                  style: TextStyle(fontSize: 12, color: iconColor, fontWeight: FontWeight.w500),
                ),
              ],
            ),
          ),
          
          // Body (Arguments)
          if (prettyArgs.isNotEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Text(
                prettyArgs,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            
          // Result or Error
          if (event.result != null && event.result!.isNotEmpty)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(8),
              margin: const EdgeInsets.only(left: 12, right: 12, bottom: 12),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceVariant.withOpacity(0.5),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Text(
                event.result!,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                maxLines: 5,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            
          if (event.error != null && event.error!.isNotEmpty)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(8),
              margin: const EdgeInsets.only(left: 12, right: 12, bottom: 12),
              decoration: BoxDecoration(
                color: MemoTheme.red.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Text(
                event.error!,
                style: const TextStyle(color: MemoTheme.red, fontFamily: 'monospace', fontSize: 12),
              ),
            ),
        ],
      ),
    );
  }
}
