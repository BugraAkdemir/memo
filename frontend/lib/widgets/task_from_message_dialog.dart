import 'package:flutter/material.dart';
import '../core/l10n.dart';
import '../models/chat.dart';

/// Turns a message into a Self-Driving task list. The text of the message is
/// never copied in: the owner writes the steps, picks the project chat, and the
/// list is created idle, so nothing runs until the owner starts it.
class TaskFromMessageDialog extends StatefulWidget {
  static const maxSteps = 10;

  final String subject;
  final List<ChatSession> chats;
  const TaskFromMessageDialog({
    super.key,
    required this.subject,
    required this.chats,
  });

  @override
  State<TaskFromMessageDialog> createState() => _TaskFromMessageDialogState();
}

class _TaskFromMessageDialogState extends State<TaskFromMessageDialog> {
  late final TextEditingController _title = TextEditingController(
    text: widget.subject.trim(),
  );
  final TextEditingController _steps = TextEditingController();
  String? _chat;

  List<ChatSession> get _projects => [
    for (final c in widget.chats)
      if (c.isAgentChat) c,
  ];

  List<String> get _items => [
    for (final line in _steps.text.split('\n'))
      if (line.trim().isNotEmpty) line.trim(),
  ];

  bool get _tooMany => _items.length > TaskFromMessageDialog.maxSteps;

  bool get _valid =>
      _chat != null &&
      _title.text.trim().isNotEmpty &&
      _items.isNotEmpty &&
      !_tooMany;

  @override
  void dispose() {
    _title.dispose();
    _steps.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final projects = _projects;
    return AlertDialog(
      title: Text(L10n.t('task_bridge_title')),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(L10n.t('task_bridge_help')),
              const SizedBox(height: 16),
              if (projects.isEmpty)
                Text(
                  L10n.t('task_bridge_no_chat'),
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                )
              else
                DropdownButtonFormField<String>(
                  key: const ValueKey('task-chat'),
                  isExpanded: true,
                  decoration: InputDecoration(
                    labelText: L10n.t('task_bridge_chat'),
                  ),
                  items: [
                    for (final c in projects)
                      DropdownMenuItem(
                        value: c.id,
                        child: Text(c.title, overflow: TextOverflow.ellipsis),
                      ),
                  ],
                  onChanged: (v) => setState(() => _chat = v),
                ),
              const SizedBox(height: 12),
              TextField(
                key: const ValueKey('task-title'),
                controller: _title,
                maxLength: 200,
                decoration: InputDecoration(
                  labelText: L10n.t('task_bridge_task_title'),
                ),
                onChanged: (_) => setState(() {}),
              ),
              TextField(
                key: const ValueKey('task-steps'),
                controller: _steps,
                minLines: 4,
                maxLines: 8,
                decoration: InputDecoration(
                  labelText: L10n.t('task_bridge_steps'),
                  errorText: _tooMany ? L10n.t('task_bridge_too_many') : null,
                ),
                onChanged: (_) => setState(() {}),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: Text(L10n.t('cancel')),
        ),
        FilledButton(
          key: const ValueKey('task-create'),
          onPressed: _valid
              ? () => Navigator.pop(context, {
                  'chat_id': _chat,
                  'title': _title.text.trim(),
                  'items': _items,
                })
              : null,
          child: Text(L10n.t('task_bridge_create')),
        ),
      ],
    );
  }
}
