import 'package:flutter/material.dart';
import '../../core/theme.dart';
import '../../models/agent.dart';
import '../../utils/tool_names.dart';

class AgentChatCard extends StatelessWidget {
  final AgentEvent event;

  const AgentChatCard({super.key, required this.event});

  @override
  Widget build(BuildContext context) {
    final isError = event.type == 'tool_error';
    final isDenied = event.type == 'permission_denied';
    final isExecuting = event.type == 'tool_executing' || event.type == 'permission_request';

    final iconColor = isError || isDenied
        ? MemoTheme.red
        : MemoTheme.accent;

    final statusIcon = isExecuting
        ? Icons.sync
        : isError
            ? Icons.error_outline
            : isDenied
                ? Icons.block
                : Icons.check_circle_outline;

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          if (isExecuting)
            SizedBox(
              width: 12,
              height: 12,
              child: CircularProgressIndicator(
                strokeWidth: 1.5,
                color: MemoTheme.accent,
              ),
            )
          else
            Icon(statusIcon, size: 14, color: iconColor),
          const SizedBox(width: 6),
          Icon(ToolNames.icon(event.toolName), size: 14, color: iconColor),
          const SizedBox(width: 4),
          Expanded(
            child: Text(
              ToolNames.displayName(event.toolName),
              style: TextStyle(
                fontSize: 12,
                color: iconColor,
                fontWeight: FontWeight.w500,
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ),
          if (event.durationMs != null && event.durationMs! > 0)
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
              decoration: BoxDecoration(
                color: iconColor.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(3),
              ),
              child: Text(
                '${event.durationMs}ms',
                style: TextStyle(fontSize: 10, color: iconColor),
              ),
            ),
        ],
      ),
    );
  }
}
