import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/task_list.dart';
import '../providers/tasklist_provider.dart';
import '../providers/chat_provider.dart';

class TasksScreen extends ConsumerStatefulWidget {
  const TasksScreen({super.key});

  @override
  ConsumerState<TasksScreen> createState() => _TasksScreenState();
}

class _TasksScreenState extends ConsumerState<TasksScreen> {
  final _titleCtrl = TextEditingController();
  final _itemCtrls = <TextEditingController>[];

  @override
  void dispose() {
    _titleCtrl.dispose();
    for (final c in _itemCtrls) {
      c.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final listsAsync = ref.watch(taskListsProvider);
    final activeChatId = ref.read(activeChatIdProvider).valueOrNull ?? 'default';

    return Scaffold(
      backgroundColor: c.bgApp,
      body: Column(
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            decoration: BoxDecoration(
              color: c.bgPanel,
              border: Border(bottom: BorderSide(color: c.borderSoft)),
            ),
            child: Row(
              children: [
                Icon(Icons.checklist, size: 20, color: c.textSecondary),
                const SizedBox(width: 8),
                Text(
                  L10n.t('taskloop_title'),
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                    color: c.textMain,
                  ),
                ),
                const Spacer(),
                ElevatedButton.icon(
                  onPressed: () => _showCreateDialog(activeChatId),
                  icon: const Icon(Icons.add, size: 16),
                  label: Text(L10n.t('taskloop_new_list')),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: Colors.white,
                    textStyle: const TextStyle(fontSize: 12),
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                  ),
                ),
              ],
            ),
          ),
          Expanded(
            child: listsAsync.when(
              data: (lists) {
                if (lists.isEmpty) {
                  return Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.inbox_outlined, size: 48, color: c.textDim),
                        const SizedBox(height: 12),
                        Text(
                          L10n.t('taskloop_empty'),
                          style: TextStyle(fontSize: 15, color: c.textSecondary),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          L10n.t('taskloop_empty_desc'),
                          style: TextStyle(fontSize: 12, color: c.textDim),
                        ),
                      ],
                    ),
                  );
                }
                return ListView.builder(
                  padding: const EdgeInsets.all(16),
                  itemCount: lists.length,
                  itemBuilder: (context, index) {
                    final info = lists[index];
                    return _TaskListCard(
                      info: info,
                      onTap: () => _showDetailDialog(info.id),
                      onStart: () => _startList(info.id),
                      onStop: () => ref.read(taskListsProvider.notifier).stopTaskList(info.id),
                      onDelete: () => _deleteList(info.id),
                    );
                  },
                );
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.cloud_off, size: 48, color: c.textDim),
                    const SizedBox(height: 12),
                    Text(
                      '${L10n.t('error')}: $e',
                      style: TextStyle(fontSize: 13, color: c.textSecondary),
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 12),
                    TextButton.icon(
                      onPressed: () => ref.invalidate(taskListsProvider),
                      icon: const Icon(Icons.refresh, size: 16),
                      label: Text(L10n.t('retry')),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showCreateDialog(String activeChatId) {
    _titleCtrl.clear();
    for (final c in _itemCtrls) {
      c.dispose();
    }
    _itemCtrls.clear();
    _itemCtrls.add(TextEditingController());

    showDialog(
      context: context,
      builder: (ctx) {
        final c = MemoTheme.of(ctx);
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              backgroundColor: c.bgPanel,
              title: Text(L10n.t('taskloop_new_list')),
              content: SizedBox(
                width: 400,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextField(
                      controller: _titleCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('tasklist_title_hint'),
                        border: const OutlineInputBorder(),
                      ),
                    ),
                    const SizedBox(height: 12),
                    ...List.generate(_itemCtrls.length, (i) {
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Row(
                          children: [
                            Text('${i + 1}.', style: TextStyle(color: c.textDim)),
                            const SizedBox(width: 8),
                            Expanded(
                              child: TextField(
                                controller: _itemCtrls[i],
                                decoration: InputDecoration(
                                  hintText: L10n.t('tasklist_item_hint'),
                                  border: const OutlineInputBorder(),
                                  isDense: true,
                                ),
                              ),
                            ),
                            if (_itemCtrls.length > 1)
                              IconButton(
                                icon: Icon(Icons.remove_circle_outline, size: 18, color: MemoTheme.red),
                                onPressed: () {
                                  _itemCtrls[i].dispose();
                                  _itemCtrls.removeAt(i);
                                  setDialogState(() {});
                                },
                              ),
                          ],
                        ),
                      );
                    }),
                    TextButton.icon(
                      onPressed: () {
                        _itemCtrls.add(TextEditingController());
                        setDialogState(() {});
                      },
                      icon: const Icon(Icons.add, size: 16),
                      label: Text(L10n.t('tasklist_add_item')),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(ctx).pop(),
                  child: Text(L10n.t('cancel')),
                ),
                ElevatedButton(
                  onPressed: () {
                    final title = _titleCtrl.text.trim();
                    final items = _itemCtrls
                        .map((c) => c.text.trim())
                        .where((t) => t.isNotEmpty)
                        .toList();
                    if (title.isEmpty || items.isEmpty) return;
                    ref
                        .read(taskListsProvider.notifier)
                        .createTaskList(activeChatId, title, items);
                    Navigator.of(ctx).pop();
                  },
                  child: Text(L10n.t('save')),
                ),
              ],
            );
          },
        );
      },
    );
  }

  void _showDetailDialog(String listId) async {
    final api = ref.read(apiClientProvider);
    try {
      final tl = await api.getTaskList(listId);
      if (!mounted) return;
      showDialog(
        context: context,
        builder: (ctx) {
          final c = MemoTheme.of(ctx);
          return AlertDialog(
            backgroundColor: c.bgPanel,
            title: Text(tl.title),
            content: SizedBox(
              width: 400,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      _statusBadge(c, tl.status),
                      const SizedBox(width: 12),
                      Text(
                        '${L10n.t('taskloop_updated')}: ${tl.updatedAt}',
                        style: TextStyle(fontSize: 12, color: c.textDim),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  ...tl.items.map((item) => Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            _itemStatusIcon(item.status),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    item.text,
                                    style: TextStyle(
                                      fontSize: 13,
                                      color: c.textMain,
                                      decoration: item.status == 'done'
                                          ? TextDecoration.lineThrough
                                          : null,
                                    ),
                                  ),
                                  if (item.note.isNotEmpty)
                                    Padding(
                                      padding: const EdgeInsets.only(top: 4),
                                      child: Text(
                                        item.note,
                                        style: TextStyle(
                                          fontSize: 11,
                                          color: item.status == 'stuck'
                                              ? MemoTheme.red
                                              : c.textDim,
                                        ),
                                      ),
                                    ),
                                  if (item.rounds > 0)
                                    Text(
                                      '${item.rounds} tur',
                                      style: TextStyle(fontSize: 11, color: c.textDim),
                                    ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      )),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(ctx).pop(),
                child: Text(L10n.t('close')),
              ),
              if (tl.status == 'idle' || tl.status == 'paused')
                ElevatedButton.icon(
                  onPressed: () {
                    Navigator.of(ctx).pop();
                    _startList(listId);
                  },
                  icon: const Icon(Icons.play_arrow, size: 16),
                  label: Text(L10n.t('start')),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.green,
                    foregroundColor: Colors.white,
                  ),
                ),
              if (tl.status == 'running')
                ElevatedButton.icon(
                  onPressed: () {
                    ref.read(taskListsProvider.notifier).stopTaskList(listId);
                    Navigator.of(ctx).pop();
                  },
                  icon: const Icon(Icons.stop, size: 16),
                  label: Text(L10n.t('stop')),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.red,
                    foregroundColor: Colors.white,
                  ),
                ),
            ],
          );
        },
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: $e'), backgroundColor: MemoTheme.red),
        );
      }
    }
  }

  void _startList(String listId) {
    showDialog(
      context: context,
      builder: (ctx) {
        final c = MemoTheme.of(ctx);
        return AlertDialog(
          backgroundColor: c.bgPanel,
          title: Text(L10n.t('taskloop_start_confirm_title')),
          content: Text(L10n.t('taskloop_start_confirm')),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(L10n.t('cancel')),
            ),
            ElevatedButton(
              onPressed: () {
                ref.read(taskListsProvider.notifier).startTaskList(listId);
                Navigator.of(ctx).pop();
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: MemoTheme.green,
                foregroundColor: Colors.white,
              ),
              child: Text(L10n.t('start')),
            ),
          ],
        );
      },
    );
  }

  void _deleteList(String listId) {
    showDialog(
      context: context,
      builder: (ctx) {
        final c = MemoTheme.of(ctx);
        return AlertDialog(
          backgroundColor: c.bgPanel,
          title: Text(L10n.t('delete')),
          content: Text(L10n.t('taskloop_delete_confirm')),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: Text(L10n.t('cancel')),
            ),
            ElevatedButton(
              onPressed: () {
                ref.read(taskListsProvider.notifier).deleteTaskList(listId);
                Navigator.of(ctx).pop();
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: MemoTheme.red,
                foregroundColor: Colors.white,
              ),
              child: Text(L10n.t('delete')),
            ),
          ],
        );
      },
    );
  }

  Widget _statusBadge(ThemeColors c, String status) {
    Color color;
    String text;
    switch (status) {
      case 'running':
        color = MemoTheme.accent;
        text = '● ${L10n.t('taskloop_running')}';
        break;
      case 'done':
        color = MemoTheme.green;
        text = '✓ ${L10n.t('taskloop_done')}';
        break;
      case 'paused':
        color = Colors.orange;
        text = '⏸ ${L10n.t('taskloop_paused')}';
        break;
      default:
        color = c.textDim;
        text = L10n.t('taskloop_idle');
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Text(text, style: TextStyle(fontSize: 12, color: color)),
    );
  }

  Widget _itemStatusIcon(String status) {
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
}

class _TaskListCard extends StatelessWidget {
  final TaskListInfo info;
  final VoidCallback onTap;
  final VoidCallback onStart;
  final VoidCallback onStop;
  final VoidCallback onDelete;

  const _TaskListCard({
    required this.info,
    required this.onTap,
    required this.onStart,
    required this.onStop,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isRunning = info.status == 'running';

    return Card(
      color: c.bgPanel,
      margin: const EdgeInsets.only(bottom: 8),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        side: BorderSide(color: c.borderSoft),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            info.title,
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                              color: c.textMain,
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        if (isRunning)
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                            decoration: BoxDecoration(
                              color: MemoTheme.accent.withValues(alpha: 0.15),
                              borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                            ),
                            child: const SizedBox(
                              width: 10,
                              height: 10,
                              child: CircularProgressIndicator(strokeWidth: 1.5),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '${info.doneCount}/${info.itemCount} ${L10n.t('taskloop_items_done')} · ${L10n.t('taskloop_updated')}: ${info.updatedAt}',
                      style: TextStyle(fontSize: 12, color: c.textDim),
                    ),
                  ],
                ),
              ),
              if (info.status == 'idle' || info.status == 'paused')
                IconButton(
                  icon: const Icon(Icons.play_arrow, size: 20, color: MemoTheme.green),
                  onPressed: onStart,
                  tooltip: L10n.t('start'),
                ),
              if (isRunning)
                IconButton(
                  icon: const Icon(Icons.stop, size: 20, color: MemoTheme.red),
                  onPressed: onStop,
                  tooltip: L10n.t('stop'),
                ),
              IconButton(
                icon: const Icon(Icons.delete_outline, size: 18),
                onPressed: onDelete,
                color: c.textDim,
                tooltip: L10n.t('delete'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
