import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/task_list.dart';
import '../../providers/chat_provider.dart';
import '../../providers/tasklist_provider.dart';
import '../../screens/task_detail_screen.dart';

/// Live, in-transcript view of the Self-Driving task bound to the current chat
/// (v4.6.0). Rendered by ChatMessageList as its last item, styled like an
/// assistant turn: a breathing "alive" dot, one canonical progress line + bar,
/// a ticking elapsed timer, a scrollable step-by-step activity log, and a
/// single primary action. Ephemeral — it disappears when the task ends and the
/// backend leaves one persisted summary line in the chat.
class TaskActivityBlock extends ConsumerStatefulWidget {
  final ChatTaskState state;
  const TaskActivityBlock({super.key, required this.state});

  @override
  ConsumerState<TaskActivityBlock> createState() => _TaskActivityBlockState();
}

class _TaskActivityBlockState extends ConsumerState<TaskActivityBlock>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pulse;
  Timer? _tick;
  final _logScroll = ScrollController();

  // Local elapsed clock: base + wall-clock delta since the last snapshot.
  int _baseElapsed = 0;
  DateTime _baseAt = DateTime.now();
  bool _expanded = true;

  @override
  void initState() {
    super.initState();
    _pulse = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1400),
    )..repeat(reverse: true);
    _baseElapsed = widget.state.elapsedSec;
    _baseAt = DateTime.now();
    _tick = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void didUpdateWidget(TaskActivityBlock old) {
    super.didUpdateWidget(old);
    if (widget.state.elapsedSec != old.state.elapsedSec) {
      _baseElapsed = widget.state.elapsedSec;
      _baseAt = DateTime.now();
    }
    if (widget.state.log.length != old.state.log.length) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (_logScroll.hasClients) {
          _logScroll.jumpTo(_logScroll.position.maxScrollExtent);
        }
      });
    }
  }

  @override
  void dispose() {
    _pulse.dispose();
    _tick?.cancel();
    _logScroll.dispose();
    super.dispose();
  }

  int get _elapsed =>
      _baseElapsed + DateTime.now().difference(_baseAt).inSeconds;

  String _fmtDuration(int s) {
    if (s < 60) return '${s}s';
    final m = s ~/ 60;
    final r = s % 60;
    return r == 0 ? '${m}dk' : '${m}dk ${r}s';
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final t = widget.state;

    return Padding(
      padding: const EdgeInsets.only(top: 6, bottom: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _avatar(c),
          const SizedBox(width: 12),
          Expanded(
            child: Container(
              padding: const EdgeInsets.fromLTRB(14, 12, 14, 10),
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                border: Border.all(color: c.borderSoft),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  _headerRow(c, t),
                  const SizedBox(height: 8),
                  _progressBar(c, t),
                  if (t.current.trim().isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      t.current.trim(),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(fontSize: 12, color: c.textSecondary),
                    ),
                  ],
                  if (t.log.isNotEmpty) ...[
                    const SizedBox(height: 10),
                    _logToggle(c),
                    if (_expanded) ...[
                      const SizedBox(height: 6),
                      _logView(c, t),
                    ],
                  ],
                  const SizedBox(height: 10),
                  _actions(c, t),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _avatar(ThemeColors c) => Container(
        width: 32,
        height: 32,
        decoration: BoxDecoration(
          color: MemoTheme.accentPale.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(10),
          border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
        ),
        alignment: Alignment.center,
        child: const Text('M',
            style: TextStyle(
                fontWeight: FontWeight.bold,
                fontSize: 14,
                color: MemoTheme.accent)),
      );

  // ── header: pulse + status word + canonical progress + elapsed ──
  Widget _headerRow(ThemeColors c, ChatTaskState t) {
    final (dotColor, statusWord) = _status(c, t);
    return Row(
      children: [
        _pulseDot(dotColor, alive: t.running && !t.stalled),
        const SizedBox(width: 8),
        Text(statusWord,
            style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w600, color: dotColor)),
        if (t.stalled) ...[
          const SizedBox(width: 6),
          Text(L10n.t('task_block_silent', {'n': '${t.silentSec}'}),
              style: TextStyle(fontSize: 11, color: MemoTheme.warningOrange)),
        ],
        const Spacer(),
        Text(_progressText(t),
            style: TextStyle(fontSize: 11, color: c.textDim)),
        const SizedBox(width: 10),
        Icon(Icons.schedule, size: 12, color: c.textDim),
        const SizedBox(width: 3),
        Text(_fmtDuration(_elapsed),
            style: TextStyle(fontSize: 11, color: c.textDim)),
      ],
    );
  }

  (Color, String) _status(ThemeColors c, ChatTaskState t) {
    if (t.awaitingPlan) {
      return (MemoTheme.accent, L10n.t('task_phase_awaiting_plan'));
    }
    if (t.paused) return (MemoTheme.warningOrange, L10n.t('task_phase_paused'));
    if (t.stalled) return (MemoTheme.warningOrange, L10n.t('task_block_maybe_stuck'));
    if (t.phase == 'planning') {
      return (MemoTheme.accent, L10n.t('task_phase_planning'));
    }
    if (t.running) return (MemoTheme.accent, L10n.t('task_phase_executing'));
    return (c.textDim, t.phase);
  }

  Widget _pulseDot(Color color, {required bool alive}) {
    if (!alive) {
      return Container(
        width: 9,
        height: 9,
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.25),
          shape: BoxShape.circle,
          border: Border.all(color: color, width: 1.5),
        ),
      );
    }
    return AnimatedBuilder(
      animation: _pulse,
      builder: (context, _) {
        final v = _pulse.value;
        return Container(
          width: 9,
          height: 9,
          decoration: BoxDecoration(
            color: Color.lerp(color.withValues(alpha: 0.35), color, v),
            shape: BoxShape.circle,
            boxShadow: [
              BoxShadow(
                color: color.withValues(alpha: 0.35 * (1 - v)),
                blurRadius: 6 * v,
                spreadRadius: 2 * v,
              ),
            ],
          ),
        );
      },
    );
  }

  String _progressText(ChatTaskState t) {
    final parts = <String>[];
    if (t.isPlanner && t.stepTotal > 0) {
      parts.add('${L10n.t('task_card_step')} ${t.stepDone}/${t.stepTotal}');
    }
    if (t.itemTotal > 0) {
      parts.add('${L10n.t('task_card_item')} ${t.itemDone}/${t.itemTotal}');
    }
    return parts.join('  ·  ');
  }

  Widget _progressBar(ThemeColors c, ChatTaskState t) {
    double? value;
    if (t.isPlanner && t.stepTotal > 0) {
      value = t.stepDone / t.stepTotal;
    } else if (t.itemTotal > 0) {
      value = t.itemDone / t.itemTotal;
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(3),
      child: LinearProgressIndicator(
        value: value,
        minHeight: 3,
        backgroundColor: c.borderSoft,
        valueColor: AlwaysStoppedAnimation(
            t.paused ? MemoTheme.warningOrange : MemoTheme.accent),
      ),
    );
  }

  // ── activity log ──
  Widget _logToggle(ThemeColors c) => InkWell(
        onTap: () => setState(() => _expanded = !_expanded),
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 3),
          child: Row(
            children: [
              Icon(_expanded ? Icons.expand_more : Icons.chevron_right,
                  size: 16, color: c.textMuted),
              const SizedBox(width: 4),
              Text(
                _expanded
                    ? L10n.t('task_block_hide_log')
                    : L10n.t('task_block_show_log', {'n': '${widget.state.log.length}'}),
                style: TextStyle(
                    fontSize: 12, fontWeight: FontWeight.w500, color: c.textMuted),
              ),
            ],
          ),
        ),
      );

  Widget _logView(ThemeColors c, ChatTaskState t) => Container(
        constraints: const BoxConstraints(maxHeight: 220),
        decoration: BoxDecoration(
          color: c.bgElement.withValues(alpha: 0.4),
          borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
          border: Border.all(color: c.borderSoft),
        ),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        child: ListView.builder(
          controller: _logScroll,
          shrinkWrap: true,
          itemCount: t.log.length,
          itemBuilder: (_, i) => _logRow(c, t.log[i]),
        ),
      );

  Widget _logRow(ThemeColors c, TaskLogEntry e) {
    final (icon, color) = _rowStyle(e.kind);
    final label = e.text.trim().isNotEmpty ? e.text.trim() : _fixedLabel(e.kind);
    final hh = e.ts.hour.toString().padLeft(2, '0');
    final mm = e.ts.minute.toString().padLeft(2, '0');
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1.5),
            child: Icon(icon, size: 12, color: color),
          ),
          const SizedBox(width: 6),
          Text('$hh:$mm',
              style: TextStyle(
                  fontSize: 10,
                  color: c.textDim,
                  fontFeatures: const [FontFeature.tabularFigures()])),
          const SizedBox(width: 8),
          Expanded(
            child: Text(label,
                style: TextStyle(fontSize: 11.5, color: c.textSecondary, height: 1.35)),
          ),
        ],
      ),
    );
  }

  (IconData, Color) _rowStyle(String kind) {
    switch (kind) {
      case 'plan_start':
      case 'plan_done':
      case 'awaiting_plan':
        return (Icons.checklist_rtl, MemoTheme.accent);
      case 'step_start':
        return (Icons.play_arrow_rounded, MemoTheme.accent);
      case 'step_done':
      case 'item_done':
        return (Icons.check_circle_outline, MemoTheme.green);
      case 'tool':
        return (Icons.bolt, MemoTheme.of(context).textMuted);
      case 'step_retry':
        return (Icons.refresh, MemoTheme.warningOrange);
      case 'step_stuck':
      case 'item_stuck':
      case 'escalate':
        return (Icons.error_outline, MemoTheme.red);
      case 'paused':
        return (Icons.pause_circle_outline, MemoTheme.warningOrange);
      case 'resumed':
        return (Icons.play_circle_outline, MemoTheme.accent);
      default:
        return (Icons.circle, MemoTheme.of(context).textDim);
    }
  }

  String _fixedLabel(String kind) {
    switch (kind) {
      case 'plan_start':
        return L10n.t('task_ev_planning');
      case 'awaiting_plan':
        return L10n.t('task_ev_awaiting_plan');
      case 'paused':
        return L10n.t('task_ev_paused');
      case 'resumed':
        return L10n.t('task_block_resumed');
      case 'item_done':
        return L10n.t('task_ev_item_done');
      case 'item_stuck':
        return L10n.t('task_ev_item_stuck');
      default:
        return kind;
    }
  }

  // ── one primary action + a quiet link ──
  Widget _actions(ThemeColors c, ChatTaskState t) {
    final running = ref.read(runningTasksProvider.notifier);
    Widget? primary;
    if (t.awaitingPlan) {
      primary = FilledButton.tonal(
        onPressed: () => _showPlanSheet(context),
        child: Text(L10n.t('task_card_view_approve_plan')),
      );
    } else if (t.paused) {
      primary = FilledButton.tonal(
        onPressed: () => running.resume(t.listId),
        child: Text(L10n.t('task_card_resume')),
      );
    } else if (t.running) {
      primary = OutlinedButton(
        onPressed: () => running.pause(t.listId),
        child: Text(L10n.t('task_card_pause')),
      );
    }
    return Row(
      children: [
        ?primary,
        if (primary != null) const SizedBox(width: 10),
        TextButton(
          onPressed: () => Navigator.of(context).push(MaterialPageRoute(
            builder: (_) => TaskDetailScreen(taskListId: t.listId),
          )),
          child: Text(L10n.t('task_card_open_in_tasks'),
              style: TextStyle(fontSize: 12, color: c.textMuted)),
        ),
      ],
    );
  }

  Future<void> _showPlanSheet(BuildContext context) async {
    final api = ref.read(apiClientProvider);
    String planMd;
    try {
      planMd = await api.getTaskPlanMd(widget.state.listId);
    } catch (_) {
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
                    onPressed: () => Navigator.of(ctx).push(MaterialPageRoute(
                      builder: (_) =>
                          TaskDetailScreen(taskListId: widget.state.listId),
                    )),
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
                  style:
                      const TextStyle(fontFamily: 'JetBrains Mono', fontSize: 12),
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
                          await api.approveTaskPlan(widget.state.listId);
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
