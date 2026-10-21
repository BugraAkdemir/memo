import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/agent_timeline.dart';
import '../stream_progress.dart' show toolLabel;

/// What the agent did during a turn, in order: the model's own words between
/// its steps, and the tools it ran — grouped into one collapsed line per run
/// ("Ran 3 commands") that opens to show exactly which commands, files and
/// searches, and what each returned. While a tool is running the line says
/// which one, so progress is visible without opening anything.
class AgentTimeline extends StatefulWidget {
  /// AgentEvent objects or their decoded-JSON maps, oldest first.
  final List<dynamic> events;

  /// Text the model is writing right now (its current step), not yet part of
  /// [events]: rendered after them.
  final String liveNarration;

  /// The turn is still streaming (a trailing unfinished tool is "running"
  /// rather than "interrupted").
  final bool live;

  /// Renders model text (markdown) — supplied by the chat so narration looks
  /// like the rest of the message.
  final Widget Function(BuildContext context, String text) buildText;

  const AgentTimeline({
    super.key,
    required this.events,
    required this.buildText,
    this.liveNarration = '',
    this.live = false,
  });

  @override
  State<AgentTimeline> createState() => _AgentTimelineState();
}

class _AgentTimelineState extends State<AgentTimeline> {
  // Expanded groups, by their position in the timeline. A group's position is
  // stable while the turn streams (items are only ever appended).
  final Set<int> _expanded = {};

  @override
  Widget build(BuildContext context) {
    final items = buildTimeline(widget.events, live: widget.live);
    if (items.isEmpty && widget.liveNarration.trim().isEmpty) {
      return const SizedBox.shrink();
    }
    final children = <Widget>[];
    for (var i = 0; i < items.length; i++) {
      final item = items[i];
      if (item is NarrationItem) {
        children.add(Padding(
          padding: const EdgeInsets.only(bottom: 6),
          child: widget.buildText(context, item.text),
        ));
      } else if (item is ToolGroupItem) {
        children.add(_ToolGroupTile(
          group: item,
          expanded: _expanded.contains(i),
          onToggle: () => setState(() {
            if (!_expanded.add(i)) _expanded.remove(i);
          }),
        ));
      } else if (item is SkillItem) {
        children.add(_SkillLine(item: item));
      } else if (item is VerificationItem) {
        children.add(Padding(
          padding: const EdgeInsets.only(bottom: 6),
          child: Text(
            L10n.t('verification_event_detail', {'text': item.text}),
            style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
          ),
        ));
      }
    }
    if (widget.liveNarration.trim().isNotEmpty) {
      children.add(widget.buildText(context, widget.liveNarration));
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: children,
    );
  }
}

IconData _toolIcon(String tool) {
  switch (tool) {
    case 'read_file':
    case 'get_file_info':
      return Icons.description_outlined;
    case 'write_file':
    case 'edit_file':
    case 'insert_line':
      return Icons.edit_outlined;
    case 'delete_lines':
    case 'delete_file':
      return Icons.delete_outline;
    case 'run_command':
    case 'run_command_readonly':
      return Icons.terminal;
    case 'search_files':
    case 'search_content':
    case 'list_directory':
      return Icons.search;
    case 'web_search':
    case 'fetch_page':
      return Icons.public;
    case 'use_skill':
      return Icons.extension_outlined;
    default:
      return Icons.build_outlined;
  }
}

/// One quiet line saying a skill is in use; the skill's own description is the
/// tooltip.
class _SkillLine extends StatelessWidget {
  final SkillItem item;
  const _SkillLine({required this.item});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final line = Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.extension_outlined, size: 14, color: MemoTheme.accent),
          const SizedBox(width: 6),
          Flexible(
            child: Text(
              L10n.t('skill_used_line', {'name': item.name}),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 12, color: c.textDim),
            ),
          ),
        ],
      ),
    );
    return item.description.isEmpty
        ? line
        : Tooltip(message: item.description, child: line);
  }
}

class _ToolGroupTile extends StatelessWidget {
  final ToolGroupItem group;
  final bool expanded;
  final VoidCallback onToggle;

  const _ToolGroupTile({
    required this.group,
    required this.expanded,
    required this.onToggle,
  });

  String _title() {
    final running = group.running;
    if (running != null) {
      final what = running.summary;
      final label = toolLabel(running.tool);
      return what.isEmpty
          ? L10n.t('agent_status_running', {'tool': label})
          : '${L10n.t('agent_status_running', {'tool': label})} $what';
    }
    final n = group.calls.length;
    final base = n == 1
        ? L10n.t('agent_ran_one')
        : L10n.t('agent_ran_many', {'n': '$n'});
    final errors = group.errorCount;
    return errors > 0
        ? '$base · ${L10n.t('agent_group_errors', {'n': '$errors'})}'
        : base;
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final running = group.hasRunning;
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          InkWell(
            borderRadius: BorderRadius.circular(6),
            onTap: onToggle,
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 3, horizontal: 2),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (running)
                    SizedBox(
                      width: 13,
                      height: 13,
                      child: CircularProgressIndicator(
                        strokeWidth: 1.5,
                        color: MemoTheme.accent,
                      ),
                    )
                  else
                    Icon(
                      expanded ? Icons.expand_more : Icons.chevron_right,
                      size: 16,
                      color: c.textMuted,
                    ),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      _title(),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12.5,
                        color: running ? MemoTheme.accent : c.textMuted,
                      ),
                    ),
                  ),
                  if (running) ...[
                    const SizedBox(width: 4),
                    Icon(
                      expanded ? Icons.expand_more : Icons.chevron_right,
                      size: 16,
                      color: c.textMuted,
                    ),
                  ],
                ],
              ),
            ),
          ),
          if (expanded)
            Padding(
              padding: const EdgeInsets.only(left: 8, top: 2),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (final call in group.calls) _ToolCallRow(call: call),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _ToolCallRow extends StatefulWidget {
  final ToolCallEntry call;
  const _ToolCallRow({required this.call});

  @override
  State<_ToolCallRow> createState() => _ToolCallRowState();
}

class _ToolCallRowState extends State<_ToolCallRow> {
  bool _showOutput = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final call = widget.call;
    final (IconData statusIcon, Color color) = switch (call.status) {
      ToolCallStatus.ok => (Icons.check_circle_outline, MemoTheme.accent),
      ToolCallStatus.error => (Icons.error_outline, MemoTheme.red),
      ToolCallStatus.denied => (Icons.block, MemoTheme.red),
      ToolCallStatus.interrupted => (Icons.cancel_outlined, c.textDim),
      ToolCallStatus.running => (Icons.sync, MemoTheme.accent),
    };
    final output = (call.error != null && call.error!.isNotEmpty)
        ? call.error!
        : (call.result ?? '');
    final canOpen = output.trim().isNotEmpty;
    final summary = call.summary;

    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          InkWell(
            borderRadius: BorderRadius.circular(6),
            onTap: canOpen ? () => setState(() => _showOutput = !_showOutput) : null,
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 2, horizontal: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(statusIcon, size: 14, color: color),
                  const SizedBox(width: 6),
                  Icon(_toolIcon(call.tool), size: 14, color: c.textMuted),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text.rich(
                      TextSpan(
                        children: [
                          TextSpan(
                            text: toolLabel(call.tool),
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w500,
                              color: c.textMain,
                            ),
                          ),
                          if (summary.isNotEmpty)
                            TextSpan(
                              text: '  $summary',
                              style: TextStyle(
                                fontSize: 12,
                                fontFamily: 'JetBrains Mono',
                                color: c.textMuted,
                              ),
                            ),
                        ],
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  if (call.durationMs != null && call.durationMs! > 0) ...[
                    const SizedBox(width: 6),
                    Text(
                      '${call.durationMs}ms',
                      style: TextStyle(fontSize: 10, color: c.textDim),
                    ),
                  ],
                  if (canOpen) ...[
                    const SizedBox(width: 4),
                    Icon(
                      _showOutput ? Icons.expand_less : Icons.expand_more,
                      size: 14,
                      color: c.textDim,
                    ),
                  ],
                ],
              ),
            ),
          ),
          if (_showOutput && canOpen)
            Container(
              width: double.infinity,
              margin: const EdgeInsets.only(left: 22, top: 2, bottom: 2),
              padding: const EdgeInsets.all(8),
              constraints: const BoxConstraints(maxHeight: 220),
              decoration: BoxDecoration(
                color: c.bgElement.withValues(alpha: 0.5),
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: c.borderSoft),
              ),
              child: SingleChildScrollView(
                child: SelectableText(
                  output.length > 6000 ? '${output.substring(0, 6000)}\n…' : output,
                  style: TextStyle(
                    fontSize: 11.5,
                    height: 1.4,
                    fontFamily: 'JetBrains Mono',
                    color: call.status == ToolCallStatus.error ? MemoTheme.red : c.textMuted,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
