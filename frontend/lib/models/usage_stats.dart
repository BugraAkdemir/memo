/// Aggregated LLM usage stats for the Settings usage-stats tab. Mirrors
/// internal/stats.Summary (backend), served by GET /api/stats/usage.
class ModelUsage {
  final String model;
  final int requests;
  final int promptTokens;
  final int completionTokens;

  const ModelUsage({
    required this.model,
    required this.requests,
    required this.promptTokens,
    required this.completionTokens,
  });

  factory ModelUsage.fromJson(Map<String, dynamic> json) => ModelUsage(
        model: json['model'] as String? ?? '',
        requests: (json['requests'] as num?)?.toInt() ?? 0,
        promptTokens: (json['prompt_tokens'] as num?)?.toInt() ?? 0,
        completionTokens: (json['completion_tokens'] as num?)?.toInt() ?? 0,
      );
}

class DailyUsage {
  final String date; // YYYY-MM-DD
  final int promptTokens;
  final int completionTokens;
  final int requests;

  const DailyUsage({
    required this.date,
    required this.promptTokens,
    required this.completionTokens,
    required this.requests,
  });

  int get totalTokens => promptTokens + completionTokens;

  factory DailyUsage.fromJson(Map<String, dynamic> json) => DailyUsage(
        date: json['date'] as String? ?? '',
        promptTokens: (json['prompt_tokens'] as num?)?.toInt() ?? 0,
        completionTokens: (json['completion_tokens'] as num?)?.toInt() ?? 0,
        requests: (json['requests'] as num?)?.toInt() ?? 0,
      );
}

class UsageStatsSummary {
  final int totalRequests;
  final int totalPromptTokens;
  final int totalCompletionTokens;
  final double avgTokensPerSecond;
  final String mostUsedModel;
  final int mostUsedModelRequests;
  final List<ModelUsage> modelBreakdown;
  final List<DailyUsage> daily;

  const UsageStatsSummary({
    this.totalRequests = 0,
    this.totalPromptTokens = 0,
    this.totalCompletionTokens = 0,
    this.avgTokensPerSecond = 0,
    this.mostUsedModel = '',
    this.mostUsedModelRequests = 0,
    this.modelBreakdown = const [],
    this.daily = const [],
  });

  int get totalTokens => totalPromptTokens + totalCompletionTokens;

  factory UsageStatsSummary.fromJson(Map<String, dynamic> json) {
    return UsageStatsSummary(
      totalRequests: (json['total_requests'] as num?)?.toInt() ?? 0,
      totalPromptTokens: (json['total_prompt_tokens'] as num?)?.toInt() ?? 0,
      totalCompletionTokens:
          (json['total_completion_tokens'] as num?)?.toInt() ?? 0,
      avgTokensPerSecond:
          (json['avg_tokens_per_second'] as num?)?.toDouble() ?? 0,
      mostUsedModel: json['most_used_model'] as String? ?? '',
      mostUsedModelRequests:
          (json['most_used_model_requests'] as num?)?.toInt() ?? 0,
      modelBreakdown: json['model_breakdown'] is List
          ? (json['model_breakdown'] as List)
              .map((e) => ModelUsage.fromJson(e as Map<String, dynamic>))
              .toList()
          : const [],
      daily: json['daily'] is List
          ? (json['daily'] as List)
              .map((e) => DailyUsage.fromJson(e as Map<String, dynamic>))
              .toList()
          : const [],
    );
  }
}
