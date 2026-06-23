import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/theme.dart';
import '../../models/activity_step.dart';
import '../../providers/chat_provider.dart';

/// Right-side panel showing the live, unified activity timeline of a turn:
/// orchestra plan tasks + agent tool steps, each updating in place from
/// pending → running → done/error. Designed to make the agent feel transparent
/// and trustworthy: you can always see exactly what it's doing.
class ActivityPanel extends ConsumerWidget {
  const ActivityPanel({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final steps = ref.watch(activityStepsProvider);
    final isSending = ref.watch(isSendingProvider);

    final doneCount =
        steps.where((s) => s.status == StepStatus.done).length;

    return Container(
      width: 300,
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(left: BorderSide(color: c.borderSoft)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Container(
            height: 56,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            decoration: BoxDecoration(
              border: Border(bottom: BorderSide(color: c.borderSoft)),
            ),
            child: Row(
              children: [
                Icon(Icons.account_tree_outlined, size: 18, color: MemoTheme.accent),
                const SizedBox(width: 8),
                Text(
                  'Görevler',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
                const Spacer(),
                if (steps.isNotEmpty)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                    decoration: BoxDecoration(
                      color: MemoTheme.accentMuted,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Text(
                      '$doneCount/${steps.length}',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w600,
                        color: MemoTheme.accent,
                      ),
                    ),
                  ),
              ],
            ),
          ),

          // Body
          Expanded(
            child: steps.isEmpty
                ? _EmptyState(isSending: isSending)
                : ListView.builder(
                    padding: const EdgeInsets.all(12),
                    itemCount: steps.length,
                    itemBuilder: (context, i) => _StepTile(
                      step: steps[i],
                      isLast: i == steps.length - 1,
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  final bool isSending;
  const _EmptyState({required this.isSending});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isSending ? Icons.hourglass_top : Icons.checklist_rtl,
              size: 30,
              color: c.textDim,
            ),
            const SizedBox(height: 10),
            Text(
              isSending ? 'Plan hazırlanıyor…' : 'Henüz görev yok',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 13, color: c.textDim),
            ),
            const SizedBox(height: 4),
            Text(
              'Bir istek gönder; Memo adımları burada\ncanlı gösterecek.',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 11, color: c.textDim, height: 1.4),
            ),
          ],
        ),
      ),
    );
  }
}

class _StepTile extends StatelessWidget {
  final ActivityStep step;
  final bool isLast;
  const _StepTile({required this.step, required this.isLast});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final running = step.status == StepStatus.running;
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: running ? MemoTheme.accentMuted : c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(
          color: running
              ? MemoTheme.accent.withValues(alpha: 0.35)
              : c.borderSoft,
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: _StatusIcon(status: step.status),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(
                      step.kind == 'tool'
                          ? Icons.build_outlined
                          : Icons.person_outline,
                      size: 12,
                      color: c.textDim,
                    ),
                    const SizedBox(width: 4),
                    Flexible(
                      child: Text(
                        step.title,
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: c.textMain,
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    if (step.durationMs > 0) ...[
                      const SizedBox(width: 6),
                      Text(
                        '${(step.durationMs / 1000).toStringAsFixed(1)}s',
                        style: TextStyle(fontSize: 10, color: c.textDim),
                      ),
                    ],
                  ],
                ),
                if (step.subtitle.isNotEmpty) ...[
                  const SizedBox(height: 3),
                  Text(
                    step.subtitle,
                    style: TextStyle(fontSize: 11, color: c.textMuted, height: 1.35),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                if (step.status == StepStatus.error && step.detail != null) ...[
                  const SizedBox(height: 3),
                  Text(
                    step.detail!,
                    style: const TextStyle(fontSize: 11, color: MemoTheme.red, height: 1.35),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StatusIcon extends StatelessWidget {
  final StepStatus status;
  const _StatusIcon({required this.status});

  @override
  Widget build(BuildContext context) {
    switch (status) {
      case StepStatus.running:
        return const SizedBox(
          width: 16,
          height: 16,
          child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent),
        );
      case StepStatus.done:
        return const Icon(Icons.check_circle, size: 16, color: MemoTheme.green);
      case StepStatus.error:
        return const Icon(Icons.error, size: 16, color: MemoTheme.red);
      case StepStatus.pending:
        return Icon(Icons.radio_button_unchecked,
            size: 16, color: MemoTheme.of(context).textDim);
    }
  }
}
