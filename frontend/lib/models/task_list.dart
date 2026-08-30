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
  });

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
    );
  }
}
