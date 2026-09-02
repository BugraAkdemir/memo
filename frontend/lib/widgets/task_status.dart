import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

/// A small rounded pill showing a task list's status / phase. Shared by the
/// task list screen and the task detail screen so the visual language stays
/// consistent as v4.4.0 adds planning/executing/waiting-limit/… phases.
class TaskStatusBadge extends StatelessWidget {
  final String status;
  const TaskStatusBadge(this.status, {super.key});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final (color, label) = _style(c, status);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Text(label, style: TextStyle(fontSize: 12, color: color)),
    );
  }

  static (Color, String) _style(ThemeColors c, String status) {
    switch (status) {
      case 'running':
      case 'executing':
        return (MemoTheme.accent, '● ${L10n.t('task_phase_executing')}');
      case 'planning':
        return (MemoTheme.accent, '◔ ${L10n.t('task_phase_planning')}');
      case 'done':
        return (MemoTheme.green, '✓ ${L10n.t('taskloop_done')}');
      case 'paused':
        return (Colors.orange, '⏸ ${L10n.t('taskloop_paused')}');
      case 'waiting-limit':
        return (Colors.orange, '⏳ ${L10n.t('task_phase_waiting_limit')}');
      case 'waiting-user':
        return (Colors.orange, '⚠ ${L10n.t('task_phase_waiting_user')}');
      case 'failed':
        return (MemoTheme.red, '✕ ${L10n.t('task_status_failed')}');
      case 'cancelled':
        return (c.textDim, '⊘ ${L10n.t('task_status_cancelled')}');
      default:
        return (c.textDim, L10n.t('taskloop_idle'));
    }
  }
}

/// Leading icon for a single task item row.
Widget taskItemStatusIcon(BuildContext context, String status) {
  switch (status) {
    case 'done':
      return const Icon(Icons.check_circle, size: 18, color: MemoTheme.green);
    case 'stuck':
      return const Icon(Icons.error, size: 18, color: MemoTheme.red);
    case 'running':
      return const SizedBox(
        width: 18,
        height: 18,
        child: CircularProgressIndicator(strokeWidth: 2),
      );
    default:
      return Icon(Icons.circle_outlined, size: 18, color: MemoTheme.of(context).textDim);
  }
}
