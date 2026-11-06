import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import 'setup_progress.dart';
import 'setup_widgets.dart';

/// The last page: every step with its state, so the user sees what is done,
/// what was skipped and what is still missing (required steps first). A row
/// takes the user back to that step.
class SetupSummaryStep extends StatelessWidget {
  final ThemeColors color;
  final List<SetupStepId> steps;
  final SetupSnapshot snapshot;

  /// Steps the user has opened at least once: an optional step that was
  /// reviewed counts as done even though the app cannot tell what was chosen.
  final Set<SetupStepId> visited;
  final String Function(SetupStepId) labelOf;
  final void Function(SetupStepId) onGoTo;

  const SetupSummaryStep({
    super.key,
    required this.color,
    required this.steps,
    required this.snapshot,
    required this.visited,
    required this.labelOf,
    required this.onGoTo,
  });

  @override
  Widget build(BuildContext context) {
    return SetupCard(
      color: color,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('setup_summary_intro'),
            style: TextStyle(color: color.textDim, fontSize: 12, height: 1.5),
          ),
          const SizedBox(height: 12),
          for (final id in steps)
            if (id != SetupStepId.summary) _row(id),
        ],
      ),
    );
  }

  Widget _row(SetupStepId id) {
    final state = stepState(id, snapshot);
    final missing = state == SetupStepState.requiredTodo;
    final done = state == SetupStepState.done || visited.contains(id);
    final label = missing || requiredSteps(snapshot).contains(id)
        ? '${labelOf(id)} *'
        : labelOf(id);
    final status = missing
        ? 'setup_summary_missing'
        : done
        ? 'setup_summary_done'
        : 'setup_summary_skipped';
    final tint = missing
        ? MemoTheme.warningOrange
        : done
        ? MemoTheme.green
        : color.textDim;
    return InkWell(
      key: Key('setup_summary_${id.name}'),
      onTap: () => onGoTo(id),
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 4),
        child: Row(
          children: [
            Icon(
              missing
                  ? Icons.error_outline
                  : done
                  ? Icons.check_circle_outline
                  : Icons.remove_circle_outline,
              size: 18,
              color: tint,
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Text(label, style: TextStyle(color: color.textMain)),
            ),
            Text(L10n.t(status), style: TextStyle(color: tint, fontSize: 12)),
          ],
        ),
      ),
    );
  }
}
