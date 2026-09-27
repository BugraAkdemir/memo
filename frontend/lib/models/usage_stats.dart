/// Aggregated LLM usage stats for the Settings usage-stats tab. Mirrors
/// internal/stats.Summary (backend), served by GET /api/stats/usage.
class ModelUsage {
  final String model;
  final int requests;
  final int promptTokens;
  final int completionTokens;
  final int cachedPromptTokens;
  final int cacheWriteTokens;

  const ModelUsage({
    required this.model,
    required this.requests,
    required this.promptTokens,
    required this.completionTokens,
    this.cachedPromptTokens = 0,
    this.cacheWriteTokens = 0,
  });

  int get freshPromptTokens =>
      cacheSplit(promptTokens, cachedPromptTokens, cacheWriteTokens);

  factory ModelUsage.fromJson(Map<String, dynamic> json) => ModelUsage(
        model: json['model'] as String? ?? '',
        requests: (json['requests'] as num?)?.toInt() ?? 0,
        promptTokens: (json['prompt_tokens'] as num?)?.toInt() ?? 0,
        completionTokens: (json['completion_tokens'] as num?)?.toInt() ?? 0,
        cachedPromptTokens: (json['cached_prompt_tokens'] as num?)?.toInt() ?? 0,
        cacheWriteTokens: (json['cache_write_tokens'] as num?)?.toInt() ?? 0,
      );
}

/// Prompt tokens that were neither served from cache nor written to it — the
/// part billed at full input price. Clamped at zero: the backend already
/// clamps each provider's reported split, but a mixed history (rows from
/// before cache accounting existed) plus a future field change shouldn't be
/// able to render a negative token count.
int cacheSplit(int prompt, int cached, int written) {
  final fresh = prompt - cached - written;
  return fresh < 0 ? 0 : fresh;
}

/// One usage category's aggregated totals — "which kind of call is
/// spending my tokens" (chat vs agent vs Dream vs fact extraction vs
/// mood vs ...), as opposed to ModelUsage's "which model" breakdown.
/// See internal/stats.CategoryUsage (backend).
class CategoryUsage {
  final String category;
  final int requests;
  final int promptTokens;
  final int completionTokens;
  final int cachedPromptTokens;
  final int cacheWriteTokens;

  const CategoryUsage({
    required this.category,
    required this.requests,
    required this.promptTokens,
    required this.completionTokens,
    this.cachedPromptTokens = 0,
    this.cacheWriteTokens = 0,
  });

  int get totalTokens => promptTokens + completionTokens;

  factory CategoryUsage.fromJson(Map<String, dynamic> json) => CategoryUsage(
        category: json['category'] as String? ?? '',
        requests: (json['requests'] as num?)?.toInt() ?? 0,
        promptTokens: (json['prompt_tokens'] as num?)?.toInt() ?? 0,
        completionTokens: (json['completion_tokens'] as num?)?.toInt() ?? 0,
        cachedPromptTokens: (json['cached_prompt_tokens'] as num?)?.toInt() ?? 0,
        cacheWriteTokens: (json['cache_write_tokens'] as num?)?.toInt() ?? 0,
      );
}

class DailyUsage {
  final String date; // YYYY-MM-DD
  final int promptTokens;
  final int completionTokens;
  final int cachedPromptTokens;
  final int cacheWriteTokens;
  final int requests;

  const DailyUsage({
    required this.date,
    required this.promptTokens,
    required this.completionTokens,
    required this.requests,
    this.cachedPromptTokens = 0,
    this.cacheWriteTokens = 0,
  });

  int get totalTokens => promptTokens + completionTokens;

  factory DailyUsage.fromJson(Map<String, dynamic> json) => DailyUsage(
        date: json['date'] as String? ?? '',
        promptTokens: (json['prompt_tokens'] as num?)?.toInt() ?? 0,
        completionTokens: (json['completion_tokens'] as num?)?.toInt() ?? 0,
        cachedPromptTokens: (json['cached_prompt_tokens'] as num?)?.toInt() ?? 0,
        cacheWriteTokens: (json['cache_write_tokens'] as num?)?.toInt() ?? 0,
        requests: (json['requests'] as num?)?.toInt() ?? 0,
      );
}

class UsageStatsSummary {
  final int totalRequests;
  final int totalPromptTokens;
  final int totalCompletionTokens;
  /// Prompt-cache split of [totalPromptTokens], as the providers reported it.
  ///
  /// Zero means "no provider reported a cache figure in this window", NOT
  /// "nothing was cached" — a backend with automatic caching that omits the
  /// field looks identical to a genuine miss, the local llama-server reuses
  /// its KV cache without reporting anything, and rows recorded before Memo
  /// parsed these fields contribute zero. The UI must say "not reported"
  /// rather than "off".
  final int totalCachedPromptTokens;
  final int totalCacheWriteTokens;
  final double avgTokensPerSecond;
  final String mostUsedModel;
  final int mostUsedModelRequests;
  final List<ModelUsage> modelBreakdown;
  final List<CategoryUsage> categoryBreakdown;
  final List<DailyUsage> daily;

  const UsageStatsSummary({
    this.totalRequests = 0,
    this.totalPromptTokens = 0,
    this.totalCompletionTokens = 0,
    this.totalCachedPromptTokens = 0,
    this.totalCacheWriteTokens = 0,
    this.avgTokensPerSecond = 0,
    this.mostUsedModel = '',
    this.mostUsedModelRequests = 0,
    this.modelBreakdown = const [],
    this.categoryBreakdown = const [],
    this.daily = const [],
  });

  int get totalTokens => totalPromptTokens + totalCompletionTokens;

  /// Input tokens billed at full price — total minus what came from cache and
  /// what was written to it.
  int get freshPromptTokens => cacheSplit(
      totalPromptTokens, totalCachedPromptTokens, totalCacheWriteTokens);

  /// True when at least one provider reported a cache figure in this window.
  /// The cache panel uses this to decide between showing a real split and
  /// saying nothing was reported — never to claim caching is disabled.
  bool get hasCacheReporting =>
      totalCachedPromptTokens > 0 || totalCacheWriteTokens > 0;

  /// Share of input served from cache, 0..1. Measured against
  /// [totalPromptTokens] for the same rows, so turns on providers that report
  /// nothing drag it down — that is the honest reading of "how much of what I
  /// sent was cached", not a per-provider hit rate.
  double get cacheHitRatio => totalPromptTokens > 0
      ? totalCachedPromptTokens / totalPromptTokens
      : 0;

  factory UsageStatsSummary.fromJson(Map<String, dynamic> json) {
    return UsageStatsSummary(
      totalRequests: (json['total_requests'] as num?)?.toInt() ?? 0,
      totalPromptTokens: (json['total_prompt_tokens'] as num?)?.toInt() ?? 0,
      totalCompletionTokens:
          (json['total_completion_tokens'] as num?)?.toInt() ?? 0,
      totalCachedPromptTokens:
          (json['total_cached_prompt_tokens'] as num?)?.toInt() ?? 0,
      totalCacheWriteTokens:
          (json['total_cache_write_tokens'] as num?)?.toInt() ?? 0,
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
      categoryBreakdown: json['category_breakdown'] is List
          ? (json['category_breakdown'] as List)
              .map((e) => CategoryUsage.fromJson(e as Map<String, dynamic>))
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
