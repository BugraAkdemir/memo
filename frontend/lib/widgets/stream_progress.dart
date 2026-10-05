import 'dart:async';

import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/agent.dart';
import '../models/stream_timing.dart';

/// How long the backend may stay completely silent (no chunk, not even its
/// 10-second heartbeat) before the progress line warns that the connection
/// may have dropped. Three missed heartbeats.
const kStreamSilenceWarning = Duration(seconds: 30);

/// Short, localized name for a tool, as used in progress and status lines.
String toolLabel(String? toolName) {
  switch (toolName) {
    case 'read_file':
      return L10n.t('tool_label_file');
    case 'write_file':
    case 'edit_file':
    case 'insert_line':
    case 'delete_lines':
      return L10n.t('tool_label_edit');
    case 'delete_file':
      return L10n.t('tool_label_delete');
    case 'run_command':
      return L10n.t('tool_label_command');
    case 'search_files':
      return L10n.t('tool_label_search');
    case 'whatsapp_send':
      return L10n.t('tool_label_message');
    case 'whatsapp_search':
      return L10n.t('tool_label_message_search');
    default:
      return toolName ?? L10n.t('tool_label_generic');
  }
}

/// What the in-flight turn is doing right now, in a few words.
///
/// The case that matters most is the gap *after* a tool finished: the model
/// is working out its next step (in agent mode, one non-streaming request
/// that can take minutes), and the status bar used to sit on a static
/// "done ✓" for that whole time — which is exactly what read as "stuck".
String streamPhaseLabel({
  List<AgentEvent>? events,
  String statusText = '',
  String content = '',
}) {
  final last = (events != null && events.isNotEmpty) ? events.last : null;
  if (last?.type == 'tool_executing') {
    return L10n.t('progress_running_tool', {'tool': toolLabel(last!.toolName)});
  }
  if (last?.type == 'permission_request') {
    return L10n.t('progress_waiting_permission');
  }
  if (statusText == 'web_search') return L10n.t('searching_web');
  if (statusText == 'fetch_page') return L10n.t('reading_page');
  if (content.isNotEmpty) return L10n.t('progress_writing');
  return L10n.t('progress_thinking');
}

/// "m:ss" (or "h:mm:ss" past an hour).
String formatElapsed(Duration d) {
  final h = d.inHours;
  final m = d.inMinutes.remainder(60);
  final s = d.inSeconds.remainder(60).toString().padLeft(2, '0');
  return h > 0 ? '$h:${m.toString().padLeft(2, '0')}:$s' : '$m:$s';
}

/// A live line under the streaming reply: a spinner, what the turn is doing
/// and how long it has been running — ticking every second, so a turn that
/// is still working never looks frozen — plus a warning once the backend has
/// been completely silent for [kStreamSilenceWarning]. The backend sends a
/// heartbeat every 10s while a turn is busy, so that warning means the
/// connection itself is in trouble, not just that the model is slow.
class StreamProgressLine extends StatefulWidget {
  const StreamProgressLine({
    super.key,
    required this.timing,
    required this.phase,
    this.now,
  });

  final StreamTiming timing;
  final String phase;

  /// Clock override for tests.
  final DateTime Function()? now;

  @override
  State<StreamProgressLine> createState() => _StreamProgressLineState();
}

class _StreamProgressLineState extends State<StreamProgressLine> {
  Timer? _tick;

  @override
  void initState() {
    super.initState();
    _tick = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _tick?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final now = (widget.now ?? DateTime.now)();
    final elapsed = now.difference(widget.timing.startedAt);
    final silence = now.difference(widget.timing.lastActivity);
    final stale = silence >= kStreamSilenceWarning;
    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(
                width: 12,
                height: 12,
                child: CircularProgressIndicator(
                  strokeWidth: 1.6,
                  color: MemoTheme.accent,
                ),
              ),
              const SizedBox(width: 8),
              Flexible(
                child: Text(
                  '${widget.phase} · ${formatElapsed(elapsed.isNegative ? Duration.zero : elapsed)}',
                  style: TextStyle(
                    fontSize: 12,
                    fontStyle: FontStyle.italic,
                    color: theme.textMuted,
                  ),
                ),
              ),
            ],
          ),
          if (stale)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.wifi_off_rounded, size: 13, color: Colors.orange),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      L10n.t('progress_no_news', {'s': '${silence.inSeconds}'}),
                      style: const TextStyle(fontSize: 12, color: Colors.orange),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
