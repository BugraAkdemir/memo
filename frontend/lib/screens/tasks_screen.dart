import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/chat.dart';
import '../models/task_list.dart';
import '../providers/tasklist_provider.dart';
import '../providers/chat_provider.dart';
import '../core/friendly_error.dart';
import '../widgets/task_status.dart';
import 'task_detail_screen.dart';

/// Phases where a task list is actively working (v4.4.0 replaced the single
/// 'running' status with planning/executing plus the waiting-* holds).
bool _isTaskActive(String status) =>
    status == 'running' ||
    status == 'planning' ||
    status == 'executing' ||
    status == 'waiting-limit';

class TasksScreen extends ConsumerStatefulWidget {
  /// The agent chat this screen was opened from. Pre-selects that chat as
  /// the target for newly created lists (a task list can only be bound to
  /// an agent chat — see AgentScreen, the only place this screen is reachable
  /// from).
  final String? initialChatId;

  const TasksScreen({super.key, this.initialChatId});

  @override
  ConsumerState<TasksScreen> createState() => _TasksScreenState();
}

class _TasksScreenState extends ConsumerState<TasksScreen> {
  final _titleCtrl = TextEditingController();
  final _itemCtrls = <TextEditingController>[];
  String? _dialogChatId;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(taskListsProvider.notifier).startPolling();
    });
  }

  @override
  void dispose() {
    ref.read(taskListsProvider.notifier).stopPolling();
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
    final agentChats = (ref.watch(chatListProvider).valueOrNull ?? [])
        .where((chat) => chat.isAgentChat)
        .toList();

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
                if (Navigator.of(context).canPop())
                  IconButton(
                    icon: const Icon(Icons.arrow_back),
                    color: c.textSecondary,
                    onPressed: () => Navigator.of(context).pop(),
                  ),
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
                  onPressed:
                      agentChats.isEmpty ? null : () => _showCreateDialog(agentChats),
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
          if (agentChats.isEmpty)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
              color: Colors.orange.withValues(alpha: 0.12),
              child: Text(
                L10n.t('tasklist_no_agent_chats'),
                style: TextStyle(fontSize: 12, color: c.textSecondary),
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
                // The backend (taskloopRunMu) only allows one list running at
                // a time, but the UI showed an active Start button on every
                // idle/paused card regardless — clicking a second one just
                // produced an unclear rejection error instead of the button
                // never being clickable in the first place.
                final anyRunning = lists.any((l) => _isTaskActive(l.status));
                return ListView.builder(
                  padding: const EdgeInsets.all(16),
                  itemCount: lists.length,
                  itemBuilder: (context, index) {
                    final info = lists[index];
                    final blocked = anyRunning && !_isTaskActive(info.status);
                    return _TaskListCard(
                      info: info,
                      startBlocked: blocked,
                      onTap: () => _showDetailDialog(info.id, anyRunning: blocked),
                      onStart: () => _startList(info.id),
                      onStop: () => ref.read(taskListsProvider.notifier).stopTaskList(info.id),
                      onDelete: () => _deleteList(info.id),
                      onLiveView: () => Navigator.of(context).push(MaterialPageRoute(
                        builder: (_) => TaskDetailScreen(taskListId: info.id, title: info.title),
                      )),
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
                      '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
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

  void _showCreateDialog(List<ChatSession> agentChats) {
    _titleCtrl.clear();
    for (final c in _itemCtrls) {
      c.dispose();
    }
    _itemCtrls.clear();
    _itemCtrls.add(TextEditingController());

    _dialogChatId = agentChats.any((chat) => chat.id == widget.initialChatId)
        ? widget.initialChatId
        : agentChats.first.id;

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
                    DropdownButtonFormField<String>(
                      initialValue: _dialogChatId,
                      decoration: InputDecoration(
                        labelText: L10n.t('tasklist_select_chat'),
                        border: const OutlineInputBorder(),
                        isDense: true,
                      ),
                      items: agentChats
                          .map((chat) => DropdownMenuItem(
                                value: chat.id,
                                child: Text(chat.title, overflow: TextOverflow.ellipsis),
                              ))
                          .toList(),
                      onChanged: (value) => setDialogState(() => _dialogChatId = value),
                    ),
                    const SizedBox(height: 12),
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
                    final chatId = _dialogChatId;
                    if (title.isEmpty || items.isEmpty || chatId == null) return;
                    ref
                        .read(taskListsProvider.notifier)
                        .createTaskList(chatId, title, items);
                    Navigator.of(ctx).pop();
                  },
                  child: Text(L10n.t('save')),
                ),
              ],
            );
          },
        );
      },
    ).then((_) {
      for (final c in _itemCtrls) {
        c.dispose();
      }
      _itemCtrls.clear();
    });
  }

  void _showDetailDialog(String listId, {bool anyRunning = false}) async {
    final api = ref.read(apiClientProvider);
    try {
      var tl = await api.getTaskList(listId);
      if (!mounted) return;

      // The list view behind this dialog polls every 3s (taskListsProvider),
      // but this dialog took a static snapshot at open time — a running
      // list's item statuses/notes never updated while the dialog was open,
      // so the user had to close and reopen it to see progress. Poll the
      // same list on the same cadence while the dialog is up.
      Timer? refreshTimer;
      showDialog(
        context: context,
        builder: (ctx) {
          return StatefulBuilder(builder: (ctx, setDialogState) {
            refreshTimer ??= Timer.periodic(const Duration(seconds: 3), (_) async {
              try {
                final fresh = await api.getTaskList(listId);
                setDialogState(() => tl = fresh);
              } catch (_) {
                // Transient poll failure — keep showing the last known state.
              }
            });
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
                      TaskStatusBadge(tl.status),
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
                            taskItemStatusIcon(context, item.status),
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
                  onPressed: anyRunning
                      ? null
                      : () {
                          Navigator.of(ctx).pop();
                          _startList(listId);
                        },
                  icon: const Icon(Icons.play_arrow, size: 16),
                  label: Text(anyRunning
                      ? L10n.t('taskloop_another_running')
                      : L10n.t('start')),
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
          });
        },
      ).then((_) => refreshTimer?.cancel());
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}'), backgroundColor: MemoTheme.red),
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

}

class _TaskListCard extends StatelessWidget {
  final TaskListInfo info;
  final bool startBlocked;
  final VoidCallback onTap;
  final VoidCallback onStart;
  final VoidCallback onStop;
  final VoidCallback onDelete;
  final VoidCallback onLiveView;

  const _TaskListCard({
    required this.info,
    this.startBlocked = false,
    required this.onTap,
    required this.onStart,
    required this.onStop,
    required this.onDelete,
    required this.onLiveView,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isRunning = _isTaskActive(info.status);

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
                  icon: Icon(Icons.play_arrow, size: 20,
                      color: startBlocked ? c.textDim : MemoTheme.green),
                  onPressed: startBlocked ? null : onStart,
                  tooltip: startBlocked
                      ? L10n.t('taskloop_another_running')
                      : L10n.t('start'),
                ),
              if (isRunning) ...[
                IconButton(
                  icon: Icon(Icons.speed, size: 20, color: MemoTheme.accent),
                  onPressed: onLiveView,
                  tooltip: L10n.t('task_detail_title'),
                ),
                IconButton(
                  icon: const Icon(Icons.stop, size: 20, color: MemoTheme.red),
                  onPressed: onStop,
                  tooltip: L10n.t('stop'),
                ),
              ],
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
