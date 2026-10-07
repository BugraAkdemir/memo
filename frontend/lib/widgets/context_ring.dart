import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/context_report.dart';
import '../providers/chat_provider.dart';
import '../providers/context_provider.dart';
import 'quota_badge.dart';

/// Compact token count: 325.1k, 1M, 840.
String formatContextTokens(int n) {
  if (n >= 1000000) {
    final m = n / 1000000;
    return '${m == m.roundToDouble() ? m.round() : m.toStringAsFixed(1)}M';
  }
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}k';
  return '$n';
}

/// Colour of the ring for a given fill: calm while there is room, amber as the
/// window fills, red once it is at or past the point where Memo summarizes.
Color contextRingColor(double fraction, int autoCompactPct) {
  final pct = fraction * 100;
  if (pct >= autoCompactPct) return MemoTheme.red;
  if (pct >= autoCompactPct - 15) return MemoTheme.warningOrange;
  return MemoTheme.accent;
}

/// The small ring at the bottom of the chat (in the engine strip): how full the
/// model's context window is. Tapping it opens a popover with what the window is
/// made of and, for a Subscriptions model, the account's usage limits.
///
/// Renders nothing until the backend has answered, and nothing at all when it
/// cannot (an old backend, a gated one) — it is ambient, so a failure here must
/// never surface as an error.
class ContextRing extends ConsumerWidget {
  const ContextRing({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final chatId = ref.watch(activeChatIdProvider).valueOrNull ?? '';
    final report = ref.watch(contextReportProvider(chatId)).valueOrNull;
    if (report == null) return const SizedBox.shrink();
    final c = MemoTheme.of(context);
    final known = report.window > 0;
    final color = contextRingColor(report.fraction, report.autoCompactPct);

    return Builder(builder: (ctx) {
      return Tooltip(
        message: known
            ? L10n.t('ctx_tooltip', {'pct': '${(report.fraction * 100).round()}'})
            : L10n.t('ctx_tooltip_unknown'),
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () {
              final box = ctx.findRenderObject() as RenderBox;
              final anchor = box.localToGlobal(Offset.zero) & box.size;
              ref.invalidate(contextReportProvider(chatId));
              showContextPopover(context, anchor: anchor, chatId: chatId);
            },
            child: Padding(
              padding: const EdgeInsets.all(6),
              child: SizedBox(
                width: 18,
                height: 18,
                child: CustomPaint(
                  painter: _RingPainter(
                    fraction: known ? report.fraction : 0,
                    color: color,
                    track: c.borderSoft,
                  ),
                ),
              ),
            ),
          ),
        ),
      );
    });
  }
}

class _RingPainter extends CustomPainter {
  final double fraction;
  final Color color;
  final Color track;
  const _RingPainter({required this.fraction, required this.color, required this.track});

  @override
  void paint(Canvas canvas, Size size) {
    const stroke = 2.6;
    final rect = Offset.zero & size;
    final arcRect = rect.deflate(stroke / 2);
    canvas.drawArc(
      arcRect,
      0,
      math.pi * 2,
      false,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = stroke
        ..color = track,
    );
    if (fraction <= 0) return;
    canvas.drawArc(
      arcRect,
      -math.pi / 2,
      math.pi * 2 * fraction.clamp(0.0, 1.0),
      false,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = stroke
        ..strokeCap = StrokeCap.round
        ..color = color,
    );
  }

  @override
  bool shouldRepaint(_RingPainter old) =>
      old.fraction != fraction || old.color != color || old.track != track;
}

/// Opens the popover above [anchor] (the ring's rectangle).
Future<void> showContextPopover(
  BuildContext context, {
  required Rect anchor,
  required String chatId,
}) {
  return showGeneralDialog<void>(
    context: context,
    barrierDismissible: true,
    barrierLabel: L10n.t('ctx_title'),
    barrierColor: Colors.transparent,
    transitionDuration: const Duration(milliseconds: 110),
    transitionBuilder: (_, anim, _, child) => FadeTransition(
      opacity: CurvedAnimation(parent: anim, curve: Curves.easeOut),
      child: child,
    ),
    pageBuilder: (ctx, _, _) => _ContextPopover(anchor: anchor, chatId: chatId),
  );
}

class _ContextPopover extends ConsumerWidget {
  final Rect anchor;
  final String chatId;
  const _ContextPopover({required this.anchor, required this.chatId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final size = MediaQuery.sizeOf(context);
    const margin = 12.0;
    final width = (size.width - margin * 2).clamp(260.0, 360.0);
    final left = (anchor.right - width).clamp(margin, size.width - width - margin);
    final bottom = size.height - anchor.top + 8;
    final maxHeight = (anchor.top - 8 - margin).clamp(200.0, 620.0);
    final report = ref.watch(contextReportProvider(chatId)).valueOrNull;

    return Stack(
      children: [
        Positioned(
          left: left,
          bottom: bottom,
          width: width,
          child: Material(
            color: c.bgPanel,
            elevation: 8,
            shadowColor: Colors.black54,
            borderRadius: BorderRadius.circular(12),
            clipBehavior: Clip.antiAlias,
            child: Container(
              constraints: BoxConstraints(maxHeight: maxHeight),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: c.borderSoft),
              ),
              child: report == null
                  ? const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))),
                    )
                  : SingleChildScrollView(
                      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
                      child: ContextReportView(report: report),
                    ),
            ),
          ),
        ),
      ],
    );
  }
}

const _categoryColors = <String, Color>{
  'messages': Color(0xFF4C8DF6),
  'summary': Color(0xFF9B7BEF),
  'system': Color(0xFFE0863A),
  'memory': Color(0xFF2FB5A8),
  'skills': Color(0xFFD9A441),
  'tools': Color(0xFFD9534F),
  'current': Color(0xFF58B368),
};

/// What the popover shows, apart from its container so tests can pump it alone.
class ContextReportView extends StatelessWidget {
  final ContextReport report;

  /// Clock for the allowance countdowns; tests pin it.
  final DateTime? now;
  const ContextReportView({super.key, required this.report, this.now});

  String _limitLabel(QuotaMeter m) {
    switch (m.label) {
      case '5h':
        return L10n.t('ctx_limit_session');
      case '7d':
        return L10n.t('ctx_limit_weekly');
      case '':
        return L10n.t('ctx_limit_model');
      default:
        // "12h", "30d": any other window length, spelled out.
        final n = int.tryParse(m.label.substring(0, m.label.length - 1));
        if (n != null && m.label.endsWith('h')) return L10n.t('ctx_limit_window', {'n': '$n'});
        if (n != null && m.label.endsWith('d')) return L10n.t('ctx_limit_days', {'n': '$n'});
        return m.label;
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final r = report;
    final known = r.window > 0;
    final tokens = formatContextTokens;
    final cats = r.categories.where((e) => e.tokens > 0).toList();
    final catSum = cats.fold<int>(0, (a, e) => a + e.tokens);

    Widget row(Color? swatch, String label, int value, {bool dim = false}) {
      final pct = known ? (value * 100 / r.window) : 0.0;
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 3),
        child: Row(
          children: [
            Container(
              width: 9,
              height: 9,
              decoration: BoxDecoration(
                color: swatch ?? Colors.transparent,
                borderRadius: BorderRadius.circular(2),
                border: swatch == null ? Border.all(color: c.borderSoft) : null,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(label,
                  style: TextStyle(fontSize: 12.5, color: dim ? c.textDim : c.textMain),
                  overflow: TextOverflow.ellipsis),
            ),
            Text(tokens(value),
                style: TextStyle(fontSize: 12, color: c.textMuted, fontFeatures: const [FontFeature.tabularFigures()])),
            SizedBox(
              width: 50,
              child: Text(known ? '${pct.toStringAsFixed(1)}%' : '',
                  textAlign: TextAlign.right,
                  style: TextStyle(fontSize: 12, color: c.textDim, fontFeatures: const [FontFeature.tabularFigures()])),
            ),
          ],
        ),
      );
    }

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(L10n.t('ctx_title'),
                  style: TextStyle(fontSize: 13, color: c.textMuted, fontWeight: FontWeight.w500)),
            ),
            Text(
              known
                  ? L10n.t('ctx_used_of', {
                      'used': '${r.usedReal ? '' : '~'}${tokens(r.used)}',
                      'window': tokens(r.window),
                      'pct': '${r.percent}',
                    })
                  : L10n.t('ctx_used_only', {'used': '${r.usedReal ? '' : '~'}${tokens(r.used)}'}),
              style: TextStyle(fontSize: 13, color: c.textMain, fontWeight: FontWeight.w600),
            ),
          ],
        ),
        if (r.model.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Text('${r.provider} · ${r.model}',
                style: TextStyle(fontSize: 11.5, color: c.textDim), overflow: TextOverflow.ellipsis),
          ),
        const SizedBox(height: 8),
        // The window as one bar: each part in its colour, free space as the track.
        ClipRRect(
          borderRadius: BorderRadius.circular(3),
          child: SizedBox(
            height: 6,
            child: Row(
              children: [
                for (final e in cats)
                  Flexible(
                    flex: math.max(e.tokens, 1),
                    child: Container(color: _categoryColors[e.key] ?? c.textMuted),
                  ),
                if (known && r.free > 0)
                  Flexible(flex: r.free, child: Container(color: c.bgElement))
                else if (!known && catSum == 0)
                  Expanded(child: Container(color: c.bgElement)),
              ],
            ),
          ),
        ),
        const SizedBox(height: 10),
        if (cats.isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Text(L10n.t('ctx_empty'), style: TextStyle(fontSize: 12, color: c.textDim)),
          )
        else
          for (final e in cats) row(_categoryColors[e.key] ?? c.textMuted, L10n.t('ctx_cat_${e.key}'), e.tokens),
        if (known) ...[
          row(null, L10n.t('ctx_free'), r.free, dim: true),
          if (r.compactBuffer > 0) row(c.borderSoft, L10n.t('ctx_buffer'), r.compactBuffer, dim: true),
        ],
        const SizedBox(height: 6),
        if (r.summarizedMessages > 0)
          Padding(
            padding: const EdgeInsets.only(bottom: 4),
            child: Text(L10n.t('ctx_summarized', {'n': '${r.summarizedMessages}'}),
                style: TextStyle(fontSize: 11.5, color: c.textMuted)),
          ),
        Text(
          r.autoCompactEnabled ? L10n.t('ctx_auto_on', {'pct': '${r.autoCompactPct}'}) : L10n.t('ctx_auto_off'),
          style: TextStyle(fontSize: 11.5, color: c.textDim),
        ),
        Padding(
          padding: const EdgeInsets.only(top: 2),
          child: Text(r.usedReal ? L10n.t('ctx_real') : L10n.t('ctx_estimated'),
              style: TextStyle(fontSize: 11.5, color: c.textDim)),
        ),
        if (r.limits.isNotEmpty) ...[
          const SizedBox(height: 10),
          Divider(height: 1, color: c.borderSoft),
          const SizedBox(height: 10),
          Text(L10n.t('ctx_limits_title'), style: TextStyle(fontSize: 13, color: c.textMuted, fontWeight: FontWeight.w500)),
          const SizedBox(height: 8),
          for (final m in r.limits) _LimitRow(meter: m, label: _limitLabel(m), now: now),
        ],
      ],
    );
  }
}

class _LimitRow extends StatelessWidget {
  final QuotaMeter meter;
  final String label;
  final DateTime? now;
  const _LimitRow({required this.meter, required this.label, this.now});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final color = QuotaBadge.colorFor(meter.remainingPercent);
    final reset = meter.resetAt.isEmpty ? '' : QuotaBadge.formatRemaining(meter.resetAt, now: now);
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(label, style: TextStyle(fontSize: 12.5, color: c.textMain, fontWeight: FontWeight.w500)),
              ),
              if (reset.isNotEmpty)
                Text(L10n.t('ctx_resets_in', {'when': reset}),
                    style: TextStyle(fontSize: 11.5, color: c.textDim)),
              const SizedBox(width: 10),
              Text(L10n.t('ctx_left_pct', {'pct': '${meter.remainingPercent}'}),
                  style: TextStyle(fontSize: 12, color: color, fontWeight: FontWeight.w600)),
            ],
          ),
          const SizedBox(height: 5),
          ClipRRect(
            borderRadius: BorderRadius.circular(3),
            child: LinearProgressIndicator(
              value: meter.remainingPercent / 100,
              minHeight: 6,
              backgroundColor: c.bgElement,
              valueColor: AlwaysStoppedAnimation(color),
            ),
          ),
        ],
      ),
    );
  }
}
