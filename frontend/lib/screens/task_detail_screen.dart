import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/task_list.dart';
import '../providers/tasklist_provider.dart';
import '../widgets/task_status.dart';

/// Live view of one Self-Driving task list: phase, progress, current item,
/// elapsed time, spawned sub-agents, plus pause/resume/cancel/skip and a
/// free-text instruction box that is injected into the task's own chat.
class TaskDetailScreen extends ConsumerStatefulWidget {
  final String taskListId;
  final String title;
  const TaskDetailScreen({super.key, required this.taskListId, this.title = ''});

  @override
  ConsumerState<TaskDetailScreen> createState() => _TaskDetailScreenState();
}

class _TaskDetailScreenState extends ConsumerState<TaskDetailScreen> {
  final _injectCtrl = TextEditingController();
  bool _sending = false;
  RunningTasksNotifier? _notifier;

  @override
  void initState() {
    super.initState();
    _notifier = ref.read(runningTasksProvider.notifier);
    WidgetsBinding.instance.addPostFrameCallback((_) => _notifier?.startPolling());
  }

  @override
  void dispose() {
    _notifier?.stopPolling();
    _injectCtrl.dispose();
    super.dispose();
  }

  RunningTaskInfo? _find(List<RunningTaskInfo> list) {
    for (final r in list) {
      if (r.id == widget.taskListId) return r;
    }
    return null;
  }

  Future<void> _send() async {
    final text = _injectCtrl.text.trim();
    if (text.isEmpty || _sending) return;
    setState(() => _sending = true);
    final reply = await ref.read(runningTasksProvider.notifier).inject(widget.taskListId, text);
    if (!mounted) return;
    setState(() {
      _sending = false;
      _injectCtrl.clear();
    });
    if (reply.isNotEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(reply)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final async = ref.watch(runningTasksProvider);
    final notifier = ref.read(runningTasksProvider.notifier);
    final info = async.maybeWhen(data: _find, orElse: () => null);

    return Scaffold(
      backgroundColor: c.bgApp,
      appBar: AppBar(
        title: Text(widget.title.isNotEmpty ? widget.title : L10n.t('task_detail_title')),
        backgroundColor: c.bgPanel,
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (info == null)
              Text(L10n.t('taskloop_paused'), style: TextStyle(color: c.textDim))
            else ...[
              Row(children: [
                TaskStatusBadge(info.phase),
                const SizedBox(width: 12),
                Text('${info.doneCount}/${info.itemCount}',
                    style: TextStyle(color: c.textDim, fontSize: 12)),
                const Spacer(),
                Text('${L10n.t('task_elapsed')}: ${info.elapsedSec}s',
                    style: TextStyle(color: c.textDim, fontSize: 12)),
              ]),
              const SizedBox(height: 12),
              if (info.itemCount > 0)
                LinearProgressIndicator(
                  value: info.doneCount / info.itemCount,
                  backgroundColor: c.borderSoft,
                ),
              const SizedBox(height: 16),
              if (info.currentItem.isNotEmpty) ...[
                Text(L10n.t('task_current_item'),
                    style: TextStyle(color: c.textDim, fontSize: 12)),
                const SizedBox(height: 4),
                Text(info.currentItem, style: TextStyle(color: c.textMain)),
                const SizedBox(height: 16),
              ],
              if (info.subAgents.isNotEmpty) ...[
                Text(L10n.t('task_subagents'),
                    style: TextStyle(color: c.textDim, fontSize: 12)),
                const SizedBox(height: 6),
                Wrap(
                  spacing: 6,
                  runSpacing: 6,
                  children: info.subAgents
                      .map((r) => Chip(
                            label: Text(r, style: const TextStyle(fontSize: 11)),
                            visualDensity: VisualDensity.compact,
                          ))
                      .toList(),
                ),
                const SizedBox(height: 16),
              ],
            ],
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: () => notifier.pause(widget.taskListId),
                  icon: const Icon(Icons.pause, size: 18),
                  label: Text(L10n.t('task_pause')),
                ),
                OutlinedButton.icon(
                  onPressed: () => notifier.resume(widget.taskListId),
                  icon: const Icon(Icons.play_arrow, size: 18),
                  label: Text(L10n.t('task_resume')),
                ),
                OutlinedButton.icon(
                  onPressed: () => notifier.skip(widget.taskListId),
                  icon: const Icon(Icons.skip_next, size: 18),
                  label: Text(L10n.t('task_skip')),
                ),
                OutlinedButton.icon(
                  onPressed: () => notifier.cancel(widget.taskListId),
                  icon: const Icon(Icons.stop, size: 18),
                  label: Text(L10n.t('task_cancel')),
                ),
              ],
            ),
            const SizedBox(height: 20),
            TextField(
              controller: _injectCtrl,
              minLines: 1,
              maxLines: 4,
              decoration: InputDecoration(
                hintText: L10n.t('task_inject_hint'),
                border: const OutlineInputBorder(),
                suffixIcon: IconButton(
                  icon: _sending
                      ? const SizedBox(
                          width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Icon(Icons.send),
                  onPressed: _send,
                  tooltip: L10n.t('task_inject_send'),
                ),
              ),
              onSubmitted: (_) => _send(),
            ),
          ],
        ),
      ),
    );
  }
}
