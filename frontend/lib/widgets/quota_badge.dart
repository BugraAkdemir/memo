import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_models.dart';

/// "NN%" of a model's allowance that is left, coloured by how much that is, with
/// the refill time as a tooltip. Renders nothing when the vendor reports no
/// figure for the model.
class QuotaBadge extends StatelessWidget {
  final ProviderModel model;
  final double fontSize;

  const QuotaBadge({super.key, required this.model, this.fontSize = 11});

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

  @override
  Widget build(BuildContext context) {
    final pct = model.remainingPercent;
    if (pct == null) return const SizedBox.shrink();
    final color = colorFor(pct);
    final reset = formatReset(model.resetAt);
    final tip = [
      L10n.t('subs_quota_left', {'pct': '$pct'}),
      if (reset.isNotEmpty) L10n.t('subs_quota_resets', {'when': reset}),
    ].join('\n');
    return Tooltip(
      message: tip,
      child: Text(
        '$pct%',
        style: TextStyle(fontSize: fontSize, fontWeight: FontWeight.w700, color: color),
      ),
    );
  }
}
