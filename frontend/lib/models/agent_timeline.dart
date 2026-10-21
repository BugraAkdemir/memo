import 'dart:convert';

import 'agent.dart';

/// What an agent turn did, in order, as the chat shows it: the model's own
/// words ("I'll look at the config first"), the tools it ran in between
/// (grouped, so a dozen reads read as "Ran 12 commands"), and verification
/// notes. Built from the same list of events whether the turn is still
/// streaming or was loaded from history, so both render identically.
sealed class TimelineItem {
  const TimelineItem();
}

/// Text the model wrote next to its tool calls.
class NarrationItem extends TimelineItem {
  final String text;
  const NarrationItem(this.text);
}

/// A run of consecutive tool calls with no narration between them.
class ToolGroupItem extends TimelineItem {
  final List<ToolCallEntry> calls;
  const ToolGroupItem(this.calls);

  bool get hasRunning => calls.any((c) => c.status == ToolCallStatus.running);
  int get errorCount =>
      calls.where((c) => c.status == ToolCallStatus.error).length;
  ToolCallEntry? get running {
    for (final c in calls.reversed) {
      if (c.status == ToolCallStatus.running) return c;
    }
    return null;
  }
}

class VerificationItem extends TimelineItem {
  final String text;
  const VerificationItem(this.text);
}

/// "Using skill X": the turn called a skill, from the message itself or by the
/// model's own `use_skill` call.
class SkillItem extends TimelineItem {
  final String name;
  final String description;

  /// Why the call did NOT take effect ("minimal_mode", "not_saved"); empty when
  /// the skill is in use.
  final String skipReason;
  const SkillItem(this.name, this.description, {this.skipReason = ''});
}

enum ToolCallStatus { running, ok, error, denied, interrupted }

class ToolCallEntry {
  final String tool;
  final dynamic args;
  final ToolCallStatus status;
  final String? result;
  final String? error;
  final int? durationMs;

  const ToolCallEntry({
    required this.tool,
    this.args,
    required this.status,
    this.result,
    this.error,
    this.durationMs,
  });

  /// The one-line "what exactly" for the row: the command, the path, the
  /// pattern, the query.
  String get summary => toolArgSummary(tool, args);
}

AgentEvent? _asEvent(dynamic e) {
  if (e is AgentEvent) return e;
  if (e is Map<String, dynamic>) return AgentEvent.fromJson(e);
  if (e is Map) return AgentEvent.fromJson(Map<String, dynamic>.from(e));
  return null;
}

/// Turns an event list (live or stored) into the timeline.
///
/// The backend stores `tool_executing` immediately followed by its
/// `tool_result`/`tool_error` for one call; while streaming, the client keeps
/// only the latest of the pair. Both shapes yield one entry per call. A
/// `tool_executing` with nothing after it is still running when [live], and an
/// interrupted call otherwise.
List<TimelineItem> buildTimeline(List<dynamic> raw, {required bool live}) {
  final events = <AgentEvent>[];
  for (final r in raw) {
    final ev = _asEvent(r);
    if (ev != null) events.add(ev);
  }

  final items = <TimelineItem>[];
  List<ToolCallEntry>? group;

  void addCall(ToolCallEntry c) {
    if (group == null) {
      group = <ToolCallEntry>[];
      items.add(ToolGroupItem(group!));
    }
    group!.add(c);
  }

  for (var i = 0; i < events.length; i++) {
    final ev = events[i];
    switch (ev.type) {
      case 'narration':
        final t = (ev.content ?? '').trim();
        if (t.isEmpty) break;
        group = null;
        items.add(NarrationItem(t));
        break;
      case 'verification':
        final t = (ev.content ?? '').trim();
        if (t.isEmpty) break;
        group = null;
        items.add(VerificationItem(t));
        break;
      case 'skill_used':
        final name = (ev.toolName ?? '').trim();
        if (name.isEmpty) break;
        group = null;
        items.add(SkillItem(name, (ev.content ?? '').trim()));
        break;
      case 'skill_skipped':
        final skipped = (ev.toolName ?? '').trim();
        if (skipped.isEmpty) break;
        group = null;
        items.add(SkillItem(skipped, '', skipReason: (ev.content ?? '').trim()));
        break;
      case 'tool_executing':
        final next = i + 1 < events.length ? events[i + 1] : null;
        final completes = next != null &&
            (next.type == 'tool_result' || next.type == 'tool_error') &&
            next.toolName == ev.toolName;
        if (completes) break; // the completion event renders this call
        final isLast = i == events.length - 1;
        addCall(ToolCallEntry(
          tool: ev.toolName ?? '',
          args: ev.args,
          status: live && isLast
              ? ToolCallStatus.running
              : ToolCallStatus.interrupted,
        ));
        break;
      case 'tool_result':
        addCall(ToolCallEntry(
          tool: ev.toolName ?? '',
          args: ev.args,
          status: ToolCallStatus.ok,
          result: ev.result,
          durationMs: ev.durationMs,
        ));
        break;
      case 'tool_error':
        addCall(ToolCallEntry(
          tool: ev.toolName ?? '',
          args: ev.args,
          status: ToolCallStatus.error,
          error: ev.error,
          durationMs: ev.durationMs,
        ));
        break;
      case 'permission_denied':
        addCall(ToolCallEntry(
          tool: ev.toolName ?? '',
          args: ev.args,
          status: ToolCallStatus.denied,
        ));
        break;
      default:
        // permission_request (its own dialog), final_response (that is the
        // message itself), narration_delta (live only) and anything unknown
        // are not part of the timeline.
        break;
    }
  }
  return items;
}

/// Folds one event from the stream into the running list the chat keeps while
/// a turn streams. Tool events replace their own "executing" placeholder; model
/// text (`narration`) is kept in order. Returns a new list.
List<AgentEvent> foldAgentEvent(List<AgentEvent> current, AgentEvent ev) {
  final out = [...current];
  switch (ev.type) {
    case 'tool_executing':
      if (out.isNotEmpty && out.last.type == 'tool_executing') {
        out[out.length - 1] = ev;
      } else {
        out.add(ev);
      }
      break;
    case 'tool_result':
    case 'tool_error':
    case 'permission_denied':
      if (out.isNotEmpty && out.last.type == 'tool_executing') {
        out[out.length - 1] = ev;
      } else {
        out.add(ev);
      }
      break;
    case 'narration':
    case 'verification':
    case 'skill_used':
    case 'skill_skipped':
    case 'permission_request':
      out.add(ev);
      break;
    default:
      break; // narration_delta and unknown events are not kept
  }
  return out;
}

/// A short, single-line description of what a tool call was asked to do.
String toolArgSummary(String tool, dynamic args) {
  final map = _argsAsMap(args);
  if (map == null) return '';
  String? pick(List<String> keys) {
    for (final k in keys) {
      final v = map[k];
      if (v is String && v.trim().isNotEmpty) return v;
    }
    return null;
  }

  String? v;
  switch (tool) {
    case 'run_command':
    case 'run_command_readonly':
      v = pick(['command']);
      break;
    case 'search_files':
    case 'search_content':
      v = pick(['pattern']);
      break;
    case 'web_search':
      v = pick(['query']);
      break;
    case 'fetch_page':
    case 'browser_navigate':
      v = pick(['url']);
      break;
    case 'use_skill':
      v = pick(['name']);
      break;
    default:
      v = pick(['path', 'command', 'pattern', 'query', 'url']);
  }
  v ??= map.values.whereType<String>().firstWhere(
        (s) => s.trim().isNotEmpty,
        orElse: () => '',
      );
  final oneLine = v.replaceAll(RegExp(r'\s+'), ' ').trim();
  return oneLine.length > 160 ? '${oneLine.substring(0, 160)}…' : oneLine;
}

Map<String, dynamic>? _argsAsMap(dynamic args) {
  try {
    var a = args;
    // The backend sends args as a JSON object, or as a string holding one
    // (OpenAI-compatible providers), occasionally twice-encoded.
    for (var i = 0; i < 2 && a is String; i++) {
      a = json.decode(a);
    }
    if (a is Map) return Map<String, dynamic>.from(a);
  } catch (_) {
    // malformed args: no summary
  }
  return null;
}
