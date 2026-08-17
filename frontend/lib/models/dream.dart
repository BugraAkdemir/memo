/// Result of a manual Dream run (POST /api/memory/dream/run).
class DreamRunResult {
  final bool ran;
  final int before;
  final int after;

  const DreamRunResult({
    required this.ran,
    required this.before,
    required this.after,
  });

  factory DreamRunResult.fromJson(Map<String, dynamic> json) {
    return DreamRunResult(
      ran: json['ran'] as bool? ?? false,
      before: (json['before'] as num?)?.toInt() ?? 0,
      after: (json['after'] as num?)?.toInt() ?? 0,
    );
  }
}
