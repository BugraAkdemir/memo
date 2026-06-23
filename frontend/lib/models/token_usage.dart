/// Live token accounting for a turn, surfaced in the top bar like Claude Code.
class TokenUsage {
  final int input;
  final int output;
  final int budget; // model context-window budget, 0 = unknown

  const TokenUsage({this.input = 0, this.output = 0, this.budget = 0});

  int get total => input + output;

  /// Fraction of the context budget used (0..1), or null when budget unknown.
  double? get fraction =>
      budget > 0 ? (total / budget).clamp(0.0, 1.0) : null;

  factory TokenUsage.fromJson(Map<String, dynamic> j) {
    return TokenUsage(
      input: (j['input'] as num?)?.toInt() ?? 0,
      output: (j['output'] as num?)?.toInt() ?? 0,
      budget: (j['budget'] as num?)?.toInt() ?? 0,
    );
  }
}
