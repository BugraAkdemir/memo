class TaskItem {
  final String id;
  final String text;
  final String status; // pending | running | done | stuck
  final String note;
  final int rounds;
  final String? startedAt;
  final String? finishedAt;

  const TaskItem({
    required this.id,
    required this.text,
    this.status = 'pending',
    this.note = '',
    this.rounds = 0,
    this.startedAt,
    this.finishedAt,
  });

  factory TaskItem.fromJson(Map<String, dynamic> json) {
    return TaskItem(
      id: json['id'] as String? ?? '',
      text: json['text'] as String? ?? '',
      status: json['status'] as String? ?? 'pending',
      note: json['note'] as String? ?? '',
      rounds: (json['rounds'] as num?)?.toInt() ?? 0,
      startedAt: json['started_at'] as String?,
      finishedAt: json['finished_at'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'text': text,
      'status': status,
      'note': note,
      'rounds': rounds,
      'started_at': startedAt,
      'finished_at': finishedAt,
    };
  }
}

class TaskList {
  final String id;
  final String chatId;
  final String title;
  final List<TaskItem> items;
  final String status; // idle | running | paused | done
  final String createdAt;
  final String updatedAt;

  const TaskList({
    required this.id,
    this.chatId = '',
    this.title = '',
    this.items = const [],
    this.status = 'idle',
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory TaskList.fromJson(Map<String, dynamic> json) {
    return TaskList(
      id: json['id'] as String? ?? '',
      chatId: json['chat_id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      items: (json['items'] as List? ?? [])
          .map((e) => TaskItem.fromJson(e as Map<String, dynamic>))
          .toList(),
      status: json['status'] as String? ?? 'idle',
      createdAt: json['created_at'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'chat_id': chatId,
      'title': title,
      'items': items.map((i) => i.toJson()).toList(),
      'status': status,
      'created_at': createdAt,
      'updated_at': updatedAt,
    };
  }
}

class TaskListInfo {
  final String id;
  final String title;
  final String status;
  final int itemCount;
  final int doneCount;
  final String createdAt;
  final String updatedAt;

  const TaskListInfo({
    required this.id,
    this.title = '',
    this.status = 'idle',
    this.itemCount = 0,
    this.doneCount = 0,
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory TaskListInfo.fromJson(Map<String, dynamic> json) {
    return TaskListInfo(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      status: json['status'] as String? ?? 'idle',
      itemCount: (json['item_count'] as num?)?.toInt() ?? 0,
      doneCount: (json['done_count'] as num?)?.toInt() ?? 0,
      createdAt: json['created_at'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}

/// Live runtime view of an executing task list (v4.4.0 Self-Driving loop),
/// from GET /api/tasks/running.
class RunningTaskInfo {
  final String id;
  final String title;
  final String chatId;
  final String phase;
  final int doneCount;
  final int itemCount;
  final String currentItem;
  final int elapsedSec;
  final List<String> subAgents;
  final String notifyLevel;
  final String mode; // '' | 'worker' | 'planlayıcı'
  final int planSteps;
  final int planStepsDone;
  final int stateDocTokens;
  final int stateDocBudget;
  final int tokens; // running approx token estimate, only grows while running

  const RunningTaskInfo({
    required this.id,
    this.title = '',
    this.chatId = '',
    this.phase = '',
    this.doneCount = 0,
    this.itemCount = 0,
    this.currentItem = '',
    this.elapsedSec = 0,
    this.subAgents = const [],
    this.notifyLevel = '',
    this.mode = '',
    this.planSteps = 0,
    this.planStepsDone = 0,
    this.stateDocTokens = 0,
    this.stateDocBudget = 0,
    this.tokens = 0,
  });

  RunningTaskInfo copyWith({String? phase, int? doneCount, int? planStepsDone}) {
    return RunningTaskInfo(
      id: id,
      title: title,
      chatId: chatId,
      phase: phase ?? this.phase,
      doneCount: doneCount ?? this.doneCount,
      itemCount: itemCount,
      currentItem: currentItem,
      elapsedSec: elapsedSec,
      subAgents: subAgents,
      notifyLevel: notifyLevel,
      mode: mode,
      planSteps: planSteps,
      planStepsDone: planStepsDone ?? this.planStepsDone,
      stateDocTokens: stateDocTokens,
      stateDocBudget: stateDocBudget,
    );
  }

  factory RunningTaskInfo.fromJson(Map<String, dynamic> json) {
    return RunningTaskInfo(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      chatId: json['chat_id'] as String? ?? '',
      phase: json['phase'] as String? ?? '',
      doneCount: (json['done_count'] as num?)?.toInt() ?? 0,
      itemCount: (json['item_count'] as num?)?.toInt() ?? 0,
      currentItem: json['current_item'] as String? ?? '',
      elapsedSec: (json['elapsed_sec'] as num?)?.toInt() ?? 0,
      subAgents: (json['sub_agents'] as List?)?.map((e) => e.toString()).toList() ?? const [],
      notifyLevel: json['notify_level'] as String? ?? '',
      mode: json['mode'] as String? ?? '',
      planSteps: (json['plan_steps'] as num?)?.toInt() ?? 0,
      planStepsDone: (json['plan_steps_done'] as num?)?.toInt() ?? 0,
      stateDocTokens: (json['state_doc_tokens'] as num?)?.toInt() ?? 0,
      stateDocBudget: (json['state_doc_budget'] as num?)?.toInt() ?? 0,
      tokens: (json['tokens'] as num?)?.toInt() ?? 0,
    );
  }
}

/// One live Self-Driving event from the GET /api/tasks/events SSE stream
/// (v4.6.0 Faz D), already enriched with the list's chat_id and a progress
/// snapshot so the chat UI can render a task card without a separate fetch.
class TaskChatEvent {
  final String event; // 'snapshot' | 'planning' | 'executing' | 'step_done' | 'item_done' | 'item_stuck' | 'paused' | 'finished' | ...
  final String listId;
  final String chatId;
  final String detail;
  final String phase;
  final String mode;
  final int stepDone;
  final int stepTotal;
  final int itemDone;
  final int itemTotal;
  final String current;
  final int elapsedSec;
  final int silentSec; // seconds since the task last did something visible
  final int tokens; // running approx token estimate, only grows while running
  final String kind; // for event == 'activity': plan_start|plan_done|step_start|step_done|step_retry|step_stuck|escalate|tool_start|tool|waiting
  final String text; // for event == 'activity': the human-readable line

  const TaskChatEvent({
    required this.event,
    required this.listId,
    this.chatId = '',
    this.detail = '',
    this.phase = '',
    this.mode = '',
    this.stepDone = 0,
    this.stepTotal = 0,
    this.itemDone = 0,
    this.itemTotal = 0,
    this.current = '',
    this.elapsedSec = 0,
    this.silentSec = 0,
    this.tokens = 0,
    this.kind = '',
    this.text = '',
  });

  factory TaskChatEvent.fromJson(Map<String, dynamic> json) {
    return TaskChatEvent(
      event: json['event'] as String? ?? '',
      listId: json['list_id'] as String? ?? '',
      chatId: json['chat_id'] as String? ?? '',
      detail: json['detail'] as String? ?? '',
      phase: json['phase'] as String? ?? '',
      mode: json['mode'] as String? ?? '',
      stepDone: (json['step_done'] as num?)?.toInt() ?? 0,
      stepTotal: (json['step_total'] as num?)?.toInt() ?? 0,
      itemDone: (json['item_done'] as num?)?.toInt() ?? 0,
      itemTotal: (json['item_total'] as num?)?.toInt() ?? 0,
      current: json['current'] as String? ?? '',
      elapsedSec: (json['elapsed_sec'] as num?)?.toInt() ?? 0,
      silentSec: (json['silent_sec'] as num?)?.toInt() ?? 0,
      tokens: (json['tokens'] as num?)?.toInt() ?? 0,
      kind: json['kind'] as String? ?? '',
      text: json['text'] as String? ?? '',
    );
  }
}

/// One line in the live task activity log (v4.6.0 in-chat block).
class TaskLogEntry {
  final String kind; // plan_start | plan_done | step_start | step_done | step_retry | step_stuck | escalate | tool_start | tool | waiting | paused | resumed | awaiting_plan | item_done | item_stuck
  final String text;
  final DateTime ts;
  const TaskLogEntry(this.kind, this.text, this.ts);
}

/// The rolled-up state of the task bound to one chat, maintained by
/// chatTaskProvider from the SSE stream and read by the inline task card.
class ChatTaskState {
  final String listId;
  final String phase;
  final String mode;
  final String current;
  final int stepDone;
  final int stepTotal;
  final int itemDone;
  final int itemTotal;
  final int elapsedSec;
  final int silentSec;
  final int tokens; // running approx token estimate, only grows while running
  final List<String> recent; // last few event names, newest last
  final List<TaskLogEntry> log; // the in-chat activity feed, newest last, capped

  const ChatTaskState({
    required this.listId,
    this.phase = '',
    this.mode = '',
    this.current = '',
    this.stepDone = 0,
    this.stepTotal = 0,
    this.itemDone = 0,
    this.itemTotal = 0,
    this.elapsedSec = 0,
    this.silentSec = 0,
    this.tokens = 0,
    this.recent = const [],
    this.log = const [],
  });

  bool get isPlanner => mode == 'planlayıcı' || mode == 'planner';
  bool get awaitingPlan => phase == 'awaiting-plan-approval';
  bool get running =>
      phase == 'planning' || phase == 'executing' || phase == 'running';
  bool get paused => phase == 'paused' || phase == 'waiting-user' || phase == 'waiting-limit';
  bool get finished => phase == 'done' || phase == 'failed' || phase == 'cancelled';

  /// Seconds of silence after which the card says "model is generating".
  ///
  /// Quiet is the normal state: the agent loop calls the model with
  /// Stream:false (internal/agent/pipeline.go), so between two tool calls
  /// there is no signal at all while the model works. Writing one medium
  /// source file measured over two minutes of legitimate silence on a
  /// free-tier model.
  static const int workingAfterSec = 5;

  /// Seconds of silence after which it might genuinely be hung. Was 60s,
  /// which labelled every ordinary long generation "maybe stuck" — the
  /// opposite of the reassurance the line exists to give.
  static const int stalledAfterSec = 180;

  /// NOTE: both getters read the *last snapshot's* silentSec, which only
  /// advances when an event arrives — i.e. never during the silence they
  /// describe. TaskActivityBlock therefore runs its own local clock off the
  /// last state change and uses the two thresholds above directly.
  bool get working => running && silentSec >= workingAfterSec && !stalled;
  bool get stalled => running && silentSec >= stalledAfterSec;

  static const int _logCap = 60;

  /// Fold one event into the running state. A 'finished' event is returned as
  /// null so the caller drops the block (the persisted finish message stays).
  static ChatTaskState? fold(ChatTaskState? prev, TaskChatEvent e) {
    // 'finished' = the list reached done/failed; 'cancelled' = the user killed
    // it. Either way the live card is gone — drop this chat's entry.
    if (e.event == 'finished' || e.event == 'cancelled') return null;

    final recent = <String>[...(prev?.recent ?? const [])];
    if (e.event.isNotEmpty && e.event != 'snapshot' && e.event != 'activity') {
      recent.add(e.event);
      while (recent.length > 4) {
        recent.removeAt(0);
      }
    }

    final log = <TaskLogEntry>[...(prev?.log ?? const [])];
    final entry = _logEntryFor(e);
    if (entry != null) {
      log.add(entry);
      while (log.length > _logCap) {
        log.removeAt(0);
      }
    }

    return ChatTaskState(
      listId: e.listId,
      phase: e.phase.isNotEmpty ? e.phase : (prev?.phase ?? ''),
      mode: e.mode.isNotEmpty ? e.mode : (prev?.mode ?? ''),
      current: e.current.isNotEmpty ? e.current : (prev?.current ?? ''),
      stepDone: e.stepTotal > 0 ? e.stepDone : (prev?.stepDone ?? 0),
      stepTotal: e.stepTotal > 0 ? e.stepTotal : (prev?.stepTotal ?? 0),
      itemDone: e.itemTotal > 0 ? e.itemDone : (prev?.itemDone ?? 0),
      itemTotal: e.itemTotal > 0 ? e.itemTotal : (prev?.itemTotal ?? 0),
      elapsedSec: e.elapsedSec > 0 ? e.elapsedSec : (prev?.elapsedSec ?? 0),
      // silent_sec is always fresh off the snapshot; 0 is a legit value.
      silentSec: e.silentSec,
      // tokens only ever grows; keep the last known value if an event omits it.
      tokens: e.tokens > 0 ? e.tokens : (prev?.tokens ?? 0),
      recent: recent,
      log: log,
    );
  }

  /// Turn one SSE event into an activity-log line, or null if it isn't worth a
  /// line. `text` is either the backend-supplied activity text or a fixed label.
  static TaskLogEntry? _logEntryFor(TaskChatEvent e) {
    final now = DateTime.now();
    if (e.event == 'activity' && e.kind.isNotEmpty) {
      return TaskLogEntry(e.kind, e.text, now);
    }
    switch (e.event) {
      case 'planning':
        return TaskLogEntry('plan_start', '', now);
      case 'awaiting_plan':
        return TaskLogEntry('awaiting_plan', '', now);
      case 'paused':
        return TaskLogEntry('paused', '', now);
      case 'waiting_retry':
        return TaskLogEntry('step_retry', e.detail, now);
      case 'waiting_user':
        return TaskLogEntry('step_stuck', e.detail, now);
      case 'waiting_limit':
        return TaskLogEntry('step_retry', e.detail, now);
      case 'item_done':
        return TaskLogEntry('item_done', '', now);
      // 'item_stuck' deliberately omitted: the engine now always emits a
      // paired 'activity'/'item_stuck' line carrying the actual reason
      // (provider 503, empty output, CEO never approved, …). Folding the bare
      // event here too would render a second, reason-less red row.
      default:
        return null;
    }
  }
}
