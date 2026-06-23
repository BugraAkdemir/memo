/// A single step shown in the right-side activity panel during an agent /
/// orchestra turn. Steps come from two sources, unified into one timeline:
///   - Orchestra plan tasks (kind == 'task')
///   - Agent tool calls (kind == 'tool')
enum StepStatus { pending, running, done, error }

class ActivityStep {
  final String id;
  final String kind; // 'task' | 'tool'
  final String title;
  final String subtitle;
  final StepStatus status;
  final int durationMs;
  final String? detail;

  const ActivityStep({
    required this.id,
    required this.kind,
    required this.title,
    this.subtitle = '',
    this.status = StepStatus.pending,
    this.durationMs = 0,
    this.detail,
  });

  ActivityStep copyWith({
    String? title,
    String? subtitle,
    StepStatus? status,
    int? durationMs,
    String? detail,
  }) {
    return ActivityStep(
      id: id,
      kind: kind,
      title: title ?? this.title,
      subtitle: subtitle ?? this.subtitle,
      status: status ?? this.status,
      durationMs: durationMs ?? this.durationMs,
      detail: detail ?? this.detail,
    );
  }

  static StepStatus statusFromString(String? s) {
    switch (s) {
      case 'running':
        return StepStatus.running;
      case 'done':
        return StepStatus.done;
      case 'error':
        return StepStatus.error;
      default:
        return StepStatus.pending;
    }
  }

  /// Builds a step from a backend `finishReason: "activity"` JSON payload.
  factory ActivityStep.fromActivityJson(Map<String, dynamic> j) {
    return ActivityStep(
      id: j['id'] as String? ?? '',
      kind: j['kind'] as String? ?? 'task',
      title: j['title'] as String? ?? '',
      subtitle: j['subtitle'] as String? ?? '',
      status: statusFromString(j['status'] as String?),
      durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
      detail: j['detail'] as String?,
    );
  }
}
