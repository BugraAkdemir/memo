// The backend's JSON is checked with `is`, never cast: a body of the wrong shape
// (an older backend, a proxy's error page) must read as "nothing", not throw.
int _int(Object? v, [int fallback = 0]) => v is num ? v.toInt() : fallback;
bool _bool(Object? v, bool fallback) => v is bool ? v : fallback;
String _str(Object? v) => v is String ? v : '';

/// One slice of what a chat's last prompt was made of.
class ContextCategory {
  /// "messages", "summary", "system", "memory", "skills", "tools", "current".
  final String key;
  final int tokens;
  const ContextCategory({required this.key, required this.tokens});

  factory ContextCategory.fromJson(Map<String, dynamic> j) => ContextCategory(
        key: _str(j['key']),
        tokens: _int(j['tokens'], 0),
      );
}

/// One allowance window of the account behind the active model (Subscriptions):
/// the 5-hour "session" meter, the weekly one, or — Antigravity meters per
/// model — the model's own single figure (empty [label]).
class QuotaMeter {
  final String label;
  final int remainingPercent;

  /// When it refills (RFC 3339); '' when the vendor does not say.
  final String resetAt;
  const QuotaMeter({this.label = '', required this.remainingPercent, this.resetAt = ''});

  factory QuotaMeter.fromJson(Map<String, dynamic> j) => QuotaMeter(
        label: _str(j['label']),
        remainingPercent: (_int(j['remaining_percent'], 0)).clamp(0, 100),
        resetAt: _str(j['reset_at']),
      );
}

/// How full a chat's context window is (GET /api/context): the ring at the
/// bottom of the chat and the popover behind it.
///
/// [used] is the conversation as the model last saw it plus its reply — the
/// provider's own count when it reported one ([usedReal]), an estimate
/// otherwise. [categories] come from the last prompt Memo assembled.
class ContextReport {
  final String provider;
  final String model;

  /// The model's context window in tokens; 0 when unknown.
  final int window;
  final int used;
  final bool usedReal;
  final int percent;
  final List<ContextCategory> categories;
  final bool autoCompactEnabled;
  final int autoCompactPct;

  /// How many of the chat's oldest messages a summary currently stands in for.
  final int summarizedMessages;

  /// Allowance meters of the Subscriptions account behind the model; empty for
  /// any other provider.
  final List<QuotaMeter> limits;
  final String limitsVendor;

  const ContextReport({
    this.provider = '',
    this.model = '',
    this.window = 0,
    this.used = 0,
    this.usedReal = false,
    this.percent = 0,
    this.categories = const [],
    this.autoCompactEnabled = true,
    this.autoCompactPct = 90,
    this.summarizedMessages = 0,
    this.limits = const [],
    this.limitsVendor = '',
  });

  /// Tokens left before the window is full; 0 when the window is unknown.
  int get free => window > 0 ? (window - used).clamp(0, window) : 0;

  /// The slice of the window held back so auto-compact can run before it fills:
  /// what is above the threshold.
  int get compactBuffer =>
      window > 0 && autoCompactEnabled ? (window * (100 - autoCompactPct) / 100).round() : 0;

  /// Used as a fraction of the window, 0..1; 0 when the window is unknown.
  double get fraction => window > 0 ? (used / window).clamp(0.0, 1.0) : 0.0;

  factory ContextReport.fromJson(Map<String, dynamic> j) {
    List<T> list<T>(Object? raw, T Function(Map<String, dynamic>) f) => raw is List
        ? raw.whereType<Map>().map((m) => f(Map<String, dynamic>.from(m))).toList()
        : <T>[];
    return ContextReport(
      provider: _str(j['provider']),
      model: _str(j['model']),
      window: _int(j['window'], 0),
      used: _int(j['used'], 0),
      usedReal: _bool(j['used_real'], false),
      percent: (_int(j['percent'], 0)).clamp(0, 100),
      categories: list(j['categories'], ContextCategory.fromJson),
      autoCompactEnabled: _bool(j['auto_compact_enabled'], true),
      autoCompactPct: _int(j['auto_compact_pct'], 90),
      summarizedMessages: _int(j['summarized_messages'], 0),
      limits: list(j['limits'], QuotaMeter.fromJson),
      limitsVendor: _str(j['limits_vendor']),
    );
  }
}
