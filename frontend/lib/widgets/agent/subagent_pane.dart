import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/friendly_error.dart';
import '../../core/l10n.dart';
import '../../providers/chat_provider.dart';
import '../../providers/subagent_provider.dart';

String subagentStatusLabel(String status) {
  const states = {
    'queued',
    'running',
    'awaiting_followup',
    'failed',
    'interrupted',
    'cancelled',
    'closed',
  };
  return L10n.t(
    states.contains(status) ? 'subagent_$status' : 'subagent_unknown',
  );
}

class SubagentTray extends ConsumerWidget {
  final String chatId;
  const SubagentTray({super.key, required this.chatId});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(subagentsProvider(chatId));
    if (state.hasError) {
      return TextButton(
        onPressed: () => ref.invalidate(subagentsProvider(chatId)),
        child: Text(L10n.t('subagent_load_error')),
      );
    }
    final jobs = state.valueOrNull ?? const [];
    final active = jobs.where((job) => job.active);
    final pastCount = jobs.length - active.length;
    if (jobs.isEmpty) return const SizedBox.shrink();
    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: 104),
      child: SingleChildScrollView(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        child: Wrap(
          spacing: 8,
          runSpacing: 4,
          children: [
            for (final job in active)
              ActionChip(
                key: ValueKey('subagent-${job.id}'),
                avatar: Icon(
                  job.active ? Icons.account_tree : Icons.task_alt,
                  size: 18,
                ),
                label: Text(
                  '${job.title.isEmpty ? L10n.t('subagent_title') : job.title} · ${subagentStatusLabel(job.status)}',
                  overflow: TextOverflow.ellipsis,
                ),
                onPressed: () {
                  ref.read(subagentHistoryOpenProvider(chatId).notifier).state =
                      false;
                  ref.read(selectedSubagentProvider(chatId).notifier).state =
                      job.id;
                },
              ),
            ActionChip(
              key: const ValueKey('subagent-history'),
              avatar: const Icon(Icons.history, size: 18),
              label: Text(
                L10n.t('subagent_history_count', {'n': '$pastCount'}),
              ),
              onPressed: () {
                ref.read(selectedSubagentProvider(chatId).notifier).state =
                    null;
                ref.read(subagentHistoryOpenProvider(chatId).notifier).state =
                    true;
              },
            ),
          ],
        ),
      ),
    );
  }
}

/// Shared by desktop and narrow layouts so history opens in the same place as
/// a running child's transcript, with back navigation to its history list.
Widget? subagentDetailsPane(WidgetRef ref, String chatId) {
  final selected = ref.watch(selectedSubagentProvider(chatId));
  final historyOpen = ref.watch(subagentHistoryOpenProvider(chatId));
  if (selected != null) {
    return SubagentPane(
      key: ValueKey('$chatId/$selected'),
      chatId: chatId,
      jobId: selected,
      onClose: () =>
          ref.read(selectedSubagentProvider(chatId).notifier).state = null,
    );
  }
  if (!historyOpen) return null;
  return SubagentHistoryPane(
    key: ValueKey('history-$chatId'),
    chatId: chatId,
    onClose: () =>
        ref.read(subagentHistoryOpenProvider(chatId).notifier).state = false,
  );
}

class SubagentHistoryPane extends ConsumerWidget {
  final String chatId;
  final VoidCallback onClose;
  const SubagentHistoryPane({
    super.key,
    required this.chatId,
    required this.onClose,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(subagentsProvider(chatId));
    final past = (state.valueOrNull ?? const [])
        .where((job) => !job.active)
        .toList()
        .reversed
        .toList();
    return Material(
      color: Theme.of(context).colorScheme.surface,
      child: Column(
        children: [
          ListTile(
            leading: IconButton(
              tooltip: L10n.t('close'),
              onPressed: onClose,
              icon: const Icon(Icons.arrow_back),
            ),
            title: Text(L10n.t('subagent_history')),
          ),
          const Divider(height: 1),
          Expanded(
            child: state.hasError
                ? Center(
                    child: TextButton(
                      onPressed: () =>
                          ref.invalidate(subagentsProvider(chatId)),
                      child: Text(L10n.t('subagent_load_error')),
                    ),
                  )
                : state.isLoading && !state.hasValue
                ? Center(child: Text(L10n.t('subagent_loading')))
                : past.isEmpty
                ? Center(child: Text(L10n.t('subagent_history_empty')))
                : ListView.separated(
                    padding: const EdgeInsets.all(8),
                    itemCount: past.length,
                    separatorBuilder: (context, index) =>
                        const Divider(height: 1),
                    itemBuilder: (context, index) {
                      final job = past[index];
                      return ListTile(
                        key: ValueKey('subagent-history-${job.id}'),
                        leading: Icon(
                          job.error.isEmpty
                              ? Icons.task_alt
                              : Icons.error_outline,
                        ),
                        title: Text(
                          job.title.isEmpty
                              ? L10n.t('subagent_title')
                              : job.title,
                        ),
                        subtitle: Text(subagentStatusLabel(job.status)),
                        trailing: const Icon(Icons.chevron_right),
                        onTap: () =>
                            ref
                                .read(selectedSubagentProvider(chatId).notifier)
                                .state = job
                                .id,
                      );
                    },
                  ),
          ),
        ],
      ),
    );
  }
}

class SubagentPane extends ConsumerStatefulWidget {
  final String chatId, jobId;
  final VoidCallback onClose;
  const SubagentPane({
    super.key,
    required this.chatId,
    required this.jobId,
    required this.onClose,
  });
  @override
  ConsumerState<SubagentPane> createState() => _SubagentPaneState();
}

class _SubagentPaneState extends ConsumerState<SubagentPane> {
  final _prompt = TextEditingController();
  bool _busy = false;
  String? _error;
  @override
  void dispose() {
    _prompt.dispose();
    super.dispose();
  }

  Future<void> _action(String action) async {
    final prompt = _prompt.text.trim();
    if (_busy || (action == 'send' && prompt.isEmpty)) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref
          .read(apiClientProvider)
          .subagentAction(
            widget.jobId,
            widget.chatId,
            action,
            prompt: action == 'send' ? prompt : null,
          );
      if (!mounted) return;
      if (action == 'send') _prompt.clear();
      ref.invalidate(subagentsProvider(widget.chatId));
    } catch (e) {
      if (mounted) setState(() => _error = FriendlyError.describeGeneric(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final jobs = ref.watch(subagentsProvider(widget.chatId));
    final job = jobs.valueOrNull
        ?.where((j) => j.id == widget.jobId)
        .firstOrNull;
    return Material(
      color: Theme.of(context).colorScheme.surface,
      child: Column(
        children: [
          ListTile(
            leading: IconButton(
              onPressed: widget.onClose,
              tooltip: L10n.t('close'),
              icon: const Icon(Icons.arrow_back),
            ),
            title: Text(
              job?.title.isNotEmpty == true
                  ? job!.title
                  : L10n.t('subagent_title'),
            ),
            subtitle: job == null
                ? null
                : Text(subagentStatusLabel(job.status)),
          ),
          const Divider(height: 1),
          if (job == null)
            Expanded(
              child: Center(
                child: Text(
                  L10n.t(
                    jobs.hasError ? 'subagent_load_error' : 'subagent_loading',
                  ),
                ),
              ),
            )
          else ...[
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (job.closed &&
                        job.prompt.isEmpty &&
                        job.output.isEmpty &&
                        job.error.isEmpty &&
                        job.history.isEmpty &&
                        job.events.isEmpty)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 12),
                        child: Text(L10n.t('subagent_history_unavailable')),
                      ),
                    Text(
                      L10n.t('subagent_prompt'),
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    SelectableText(job.prompt),
                    if (job.branch.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: SelectableText(job.branch),
                      ),
                    for (final message in job.history) ...[
                      const Divider(),
                      Text(
                        message['role'] == 'user'
                            ? L10n.t('subagent_instruction')
                            : L10n.t('subagent_output'),
                        style: Theme.of(context).textTheme.titleSmall,
                      ),
                      SelectableText(
                        message['content'] is String
                            ? message['content'] as String
                            : '',
                      ),
                    ],
                    const Divider(),
                    Text(
                      L10n.t('subagent_output'),
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    SelectableText(job.output),
                    if (job.error.isNotEmpty)
                      SelectableText(
                        job.error,
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.error,
                        ),
                      ),
                    if (job.events.isNotEmpty) ...[
                      const Divider(),
                      Text(
                        L10n.t('subagent_tools'),
                        style: Theme.of(context).textTheme.titleSmall,
                      ),
                      for (final event in job.events)
                        _SubagentEvent(event: event),
                    ],
                  ],
                ),
              ),
            ),
            if (_error != null)
              Padding(padding: const EdgeInsets.all(8), child: Text(_error!)),
            if (!job.closed && job.resultDelivery == 'interrupted')
              Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(L10n.t('subagent_delivery_interrupted')),
                    if (job.deliveryError.isNotEmpty)
                      SelectableText(job.deliveryError),
                    TextButton.icon(
                      onPressed: _busy ? null : () => _action('resume'),
                      icon: const Icon(Icons.play_arrow),
                      label: Text(L10n.t('subagent_resume_review')),
                    ),
                  ],
                ),
              ),
            if (!job.closed)
              Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  children: [
                    if (!job.active)
                      Row(
                        children: [
                          Expanded(
                            child: TextField(
                              controller: _prompt,
                              enabled: !_busy,
                              maxLines: 3,
                              minLines: 1,
                              decoration: InputDecoration(
                                labelText: L10n.t('subagent_followup'),
                              ),
                            ),
                          ),
                          IconButton(
                            onPressed: _busy ? null : () => _action('send'),
                            tooltip: L10n.t('send'),
                            icon: const Icon(Icons.send),
                          ),
                        ],
                      ),
                    Align(
                      alignment: Alignment.centerRight,
                      child: TextButton(
                        onPressed: _busy
                            ? null
                            : () => _action(job.active ? 'cancel' : 'close'),
                        child: Text(
                          L10n.t(
                            job.active ? 'subagent_cancel' : 'subagent_close',
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
          ],
        ],
      ),
    );
  }
}

class _SubagentEvent extends StatelessWidget {
  final Map<String, dynamic> event;
  const _SubagentEvent({required this.event});
  String text(String key) => event[key] is String ? event[key] as String : '';
  @override
  Widget build(BuildContext context) {
    final tool = text('tool_name').isNotEmpty
        ? text('tool_name')
        : text('tool');
    final content = text('content');
    if (tool.isEmpty && content.isNotEmpty) return SelectableText(content);
    return ExpansionTile(
      tilePadding: EdgeInsets.zero,
      title: Text(tool.isNotEmpty ? tool : L10n.t('subagent_tools')),
      children: [
        if (event['args'] != null)
          SelectableText(
            event['args'] is String
                ? event['args'] as String
                : jsonEncode(event['args']),
          ),
        if (content.isNotEmpty) SelectableText(content),
        if (text('output').isNotEmpty) SelectableText(text('output')),
        if (text('result').isNotEmpty) SelectableText(text('result')),
        if (text('error').isNotEmpty) SelectableText(text('error')),
      ],
    );
  }
}
