import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/task_list.dart';
import '../../providers/chat_provider.dart';
import '../../providers/tasklist_provider.dart';
import '../../screens/task_detail_screen.dart';

/// Inline strip pinned above the composer that mirrors the live state of the
/// Self-Driving task bound to the active chat (v4.6.0 Faz E). Renders nothing
/// when no task is running in this chat. Covers BUG-PLAN9 (approve a plan from
/// chat) and BUG-PLAN12 (a lightweight activity feed in chat).
class TaskActivityCard extends ConsumerWidget {
  const TaskActivityCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatId = ref.watch(activeChatIdProvider).valueOrNull ?? '';
    if (chatId.isEmpty) return const SizedBox.shrink();
    final task = ref.watch(chatTaskForProvider(chatId));
    if (task == null) return const SizedBox.shrink();

    final theme = Theme.of(context);
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 4, 12, 4),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.dividerColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              _PhaseBadge(task: task),
              const SizedBox(width: 8),
              Expanded(child: Text(_progressLine(task), style: theme.textTheme.bodySmall)),
              Text('${task.elapsedSec}s', style: theme.textTheme.bodySmall),
            ],
          ),
          if (task.current.trim().isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(task.current.trim(),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodySmall),
          ],
          if (task.recent.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(task.recent.map(_eventLabel).join('  ·  '),
                style: theme.textTheme.labelSmall
                    ?.copyWith(color: theme.hintColor)),
          ],
          const SizedBox(height: 8),
          _Actions(chatId: chatId, task: task),
        ],
      ),
    );
  }

  static String _progressLine(ChatTaskState t) {
    final parts = <String>[];
    if (t.isPlanner && t.stepTotal > 0) {
      parts.add('${L10n.t('task_card_step')} ${t.stepDone}/${t.stepTotal}');
    }
    if (t.itemTotal > 0) {
      parts.add('${L10n.t('task_card_item')} ${t.itemDone}/${t.itemTotal}');
    }
    return parts.join('  ·  ');
  }

  static String _eventLabel(String e) {
    switch (e) {
      case 'planning':
        return L10n.t('task_ev_planning');
      case 'executing':
        return L10n.t('task_ev_executing');
      case 'step_done':
        return L10n.t('task_ev_step_done');
      case 'item_done':
        return L10n.t('task_ev_item_done');
      case 'item_stuck':
        return L10n.t('task_ev_item_stuck');
      case 'subagent_spawned':
        return L10n.t('task_ev_subagent');
      case 'awaiting_plan':
        return L10n.t('task_ev_awaiting_plan');
      case 'paused':
        return L10n.t('task_ev_paused');
      case 'provider_switched':
        return L10n.t('task_ev_provider_switched');
      default:
        return e;
    }
  }
}

class _PhaseBadge extends StatelessWidget {
  const _PhaseBadge({required this.task});
  final ChatTaskState task;

  @override
  Widget build(BuildContext context) {
    String label;
    Color color;
    if (task.awaitingPlan) {
      label = L10n.t('task_phase_awaiting_plan');
      color = MemoTheme.warningOrange;
    } else if (task.paused) {
      label = L10n.t('task_phase_paused');
      color = MemoTheme.warningOrange;
    } else if (task.phase == 'planning') {
      label = L10n.t('task_phase_planning');
      color = Theme.of(context).colorScheme.primary;
    } else if (task.running) {
      label = L10n.t('task_phase_executing');
      color = Theme.of(context).colorScheme.primary;
    } else {
      label = task.phase;
      color = Theme.of(context).hintColor;
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(label,
          style: TextStyle(
              fontSize: 11, fontWeight: FontWeight.w600, color: color)),
    );
  }
}

class _Actions extends ConsumerWidget {
  const _Actions({required this.chatId, required this.task});
  final String chatId;
  final ChatTaskState task;

  void _openInTasks(BuildContext context) {
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => TaskDetailScreen(taskListId: task.listId),
    ));
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final running = ref.read(runningTasksProvider.notifier);
    final buttons = <Widget>[];

    if (task.awaitingPlan) {
      buttons.add(FilledButton.tonal(
        onPressed: () => _showPlanSheet(context, ref),
        child: Text(L10n.t('task_card_view_approve_plan')),
      ));
    } else if (task.paused) {
      buttons.add(FilledButton.tonal(
        onPressed: () => running.resume(task.listId),
        child: Text(L10n.t('task_card_resume')),
      ));
    } else if (task.running) {
      buttons.add(OutlinedButton(
        onPressed: () => running.pause(task.listId),
        child: Text(L10n.t('task_card_pause')),
      ));
    }
    buttons.add(TextButton(
      onPressed: () => _openInTasks(context),
      child: Text(L10n.t('task_card_open_in_tasks')),
    ));

    return Wrap(spacing: 8, runSpacing: 4, children: buttons);
  }

  Future<void> _showPlanSheet(BuildContext context, WidgetRef ref) async {
    final api = ref.read(apiClientProvider);
    String planMd;
    try {
      planMd = await api.getTaskPlanMd(task.listId);
    } catch (e) {
      planMd = '';
    }
    if (!context.mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => DraggableScrollableSheet(
        expand: false,
        initialChildSize: 0.7,
        maxChildSize: 0.9,
        builder: (ctx, scrollCtrl) => Column(
          children: [
            Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  Expanded(
                    child: Text(L10n.t('task_plan_review_title'),
                        style: Theme.of(ctx).textTheme.titleMedium),
                  ),
                  TextButton(
                    onPressed: () => _openInTasks(ctx),
                    child: Text(L10n.t('task_plan_edit_in_tasks')),
                  ),
                ],
              ),
            ),
            Expanded(
              child: SingleChildScrollView(
                controller: scrollCtrl,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                child: SelectableText(
                  planMd.isEmpty ? L10n.t('task_plan_unavailable') : planMd,
                  style: const TextStyle(fontFamily: 'JetBrains Mono', fontSize: 12),
                ),
              ),
            ),
            SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      child: Text(L10n.t('cancel')),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: () async {
                        try {
                          await api.approveTaskPlan(task.listId);
                        } catch (_) {}
                        if (ctx.mounted) Navigator.of(ctx).pop();
                      },
                      child: Text(L10n.t('task_plan_approve_run')),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
