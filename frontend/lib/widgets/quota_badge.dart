import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_models.dart';

/// "NN%" of a model's allowance that is left, coloured by how much that is, with
/// how long until it refills beside it ("6d 2h") and the exact time as a tooltip.
/// Renders nothing when the vendor reports no figure for the model.
class QuotaBadge extends StatelessWidget {
  final ProviderModel model;
  final double fontSize;

  /// Show the time until the refill next to the percentage.
  final bool showReset;

  /// Clock for the countdown; tests pin it.
  final DateTime? now;

  const QuotaBadge({super.key, required this.model, this.fontSize = 11, this.showReset = true, this.now});

  /// Green while plenty is left, amber when it is getting low, red when nearly
  /// gone.
  static Color colorFor(int percent) {
    if (percent >= 50) return const Color(0xFF3FB950);
    if (percent >= 20) return MemoTheme.warningOrange;
    return MemoTheme.red;
  }

  /// "13.10 23:54" in local time, or '' when [rfc3339] does not parse.
  static String formatReset(String rfc3339) {
    final t = DateTime.tryParse(rfc3339)?.toLocal();
    if (t == null) return '';
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(t.day)}.${two(t.month)} ${two(t.hour)}:${two(t.minute)}';
  }

  /// Time from [now] until [rfc3339], as the two largest non-zero units: "6d 2h",
  /// "3h 20m", "12m". '' when it does not parse; the "refilling" word once the
  /// moment has passed (the next refresh will bring the new figure).
  static String formatRemaining(String rfc3339, {DateTime? now}) {
    final t = DateTime.tryParse(rfc3339);
    if (t == null) return '';
    final left = t.difference(now ?? DateTime.now());
    if (left.inSeconds <= 0) return L10n.t('quota_resets_now');
    final d = left.inDays, h = left.inHours % 24, m = left.inMinutes % 60;
    final u = (d: L10n.t('quota_unit_d'), h: L10n.t('quota_unit_h'), m: L10n.t('quota_unit_m'));
    if (d > 0) return h > 0 ? '$d${u.d} $h${u.h}' : '$d${u.d}';
    if (h > 0) return m > 0 ? '$h${u.h} $m${u.m}' : '$h${u.h}';
    return '${m < 1 ? 1 : m}${u.m}';
  }

  @override
  Widget build(BuildContext context) {
    final pct = model.remainingPercent;
    if (pct == null) return const SizedBox.shrink();
    final color = colorFor(pct);
    final reset = formatReset(model.resetAt);
    final tip = [
      model.quotaWindow.isEmpty
          ? L10n.t('subs_quota_left', {'pct': '$pct'})
          : L10n.t('subs_quota_left_window', {'pct': '$pct', 'window': model.quotaWindow}),
      if (reset.isNotEmpty) L10n.t('subs_quota_resets', {'when': reset}),
    ].join('\n');
    final inText = showReset ? formatRemaining(model.resetAt, now: now) : '';
    final dim = MemoTheme.of(context).textDim;
    return Tooltip(
      message: tip,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            '$pct%',
            style: TextStyle(fontSize: fontSize, fontWeight: FontWeight.w700, color: color),
          ),
          if (inText.isNotEmpty) ...[
            const SizedBox(width: 6),
            Icon(Icons.schedule, size: fontSize, color: dim),
            const SizedBox(width: 2),
            Text(inText, style: TextStyle(fontSize: fontSize - 1, color: dim)),
          ],
        ],
      ),
    );
  }
}
