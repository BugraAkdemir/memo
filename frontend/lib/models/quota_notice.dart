import 'dart:convert';

/// The message Memo types into the chat to resume after the allowance refilled.
/// It is chat content the user would have written themselves (and the model
/// reads it), not interface text, so it is the same in every UI language.
const String kQuotaContinueMessage = 'continue';

/// How long after the vendor's stated refill time to wait before continuing, so
/// the first request does not land a moment before the allowance is back.
const Duration kQuotaResumeMargin = Duration(seconds: 20);

/// What the backend says about the allowance behind the model in use — the JSON
/// payload of a `quota_exhausted` / `quota_low` marker chunk on the chat stream
/// (internal/models/quota_signal.go).
class QuotaNotice {
  /// `exhausted` (the turn failed because the allowance ran out) or `low` (it
  /// worked, but little is left).
  final String kind;
  final String model;
  final String vendor;

  /// Share left, 0..100; -1 when the backend does not know.
  final int remainingPercent;

  /// When the allowance refills, when the vendor said so.
  final DateTime? resetAt;

  /// Window the figure is for ("5h", "7d") when the vendor has several.
  final String window;

  /// When this notice arrived (the clock the fallback delays count from).
  final DateTime receivedAt;

  /// How many automatic "continue"s in a row already ended in the same
  /// exhaustion again; lengthens the fallback wait when no refill time is known.
  final int attempt;

  /// When Memo should type [kQuotaContinueMessage]. Fixed when the notice is
  /// made — recomputing it against a moving clock would keep pushing a late
  /// notice's "earliest" time ahead of itself so it never fired.
  final DateTime resumeAt;

  /// Whether [resumeAt] is the vendor's own refill time (true) or only Memo's
  /// back-off because none was given (or the given one had already passed).
  final bool knownReset;

  const QuotaNotice._({
    required this.kind,
    required this.model,
    required this.vendor,
    required this.remainingPercent,
    required this.resetAt,
    required this.window,
    required this.receivedAt,
    required this.attempt,
    required this.resumeAt,
    required this.knownReset,
  });

  factory QuotaNotice({
    required String kind,
    String model = '',
    String vendor = '',
    int remainingPercent = -1,
    DateTime? resetAt,
    String window = '',
    required DateTime receivedAt,
    int attempt = 0,
  }) {
    // With a refill time still ahead, that time plus a margin. Without one — or
    // with one that already passed, which means the vendor still refuses (clock
    // skew, a window that did not really refill) — back off from the arrival:
    // five minutes the first time, ten after that, never sooner than 30 seconds
    // from now, so a refusal that persists is not hammered.
    final known = resetAt != null && resetAt.isAfter(receivedAt);
    final DateTime resume;
    if (known) {
      resume = resetAt.add(kQuotaResumeMargin);
    } else {
      final wait = attempt == 0 ? const Duration(minutes: 5) : const Duration(minutes: 10);
      resume = receivedAt.add(wait);
    }
    return QuotaNotice._(
      kind: kind,
      model: model,
      vendor: vendor,
      remainingPercent: remainingPercent,
      resetAt: resetAt,
      window: window,
      receivedAt: receivedAt,
      attempt: attempt,
      resumeAt: resume,
      knownReset: known,
    );
  }

  /// The same notice with another [attempt] (and the wait that goes with it).
  QuotaNotice withAttempt(int attempt) => QuotaNotice(
        kind: kind,
        model: model,
        vendor: vendor,
        remainingPercent: remainingPercent,
        resetAt: resetAt,
        window: window,
        receivedAt: receivedAt,
        attempt: attempt,
      );

  bool get exhausted => kind == 'exhausted';
  bool get low => kind == 'low';

  /// Parses a marker chunk's content. Null for anything that is not a notice —
  /// the caller ignores it, as it does any malformed marker.
  static QuotaNotice? tryParse(String content, {DateTime? now, int attempt = 0}) {
    try {
      final raw = json.decode(content);
      if (raw is! Map<String, dynamic>) return null;
      final kind = raw['kind'];
      if (kind != 'exhausted' && kind != 'low') return null;
      final resetRaw = raw['reset_at'];
      return QuotaNotice(
        kind: kind as String,
        model: raw['model'] as String? ?? '',
        vendor: raw['vendor'] as String? ?? '',
        remainingPercent: (raw['remaining_percent'] as num?)?.toInt() ?? -1,
        resetAt: resetRaw is String && resetRaw.isNotEmpty ? DateTime.tryParse(resetRaw)?.toLocal() : null,
        window: raw['window'] as String? ?? '',
        receivedAt: now ?? DateTime.now(),
        attempt: attempt,
      );
    } catch (_) {
      return null;
    }
  }
}
