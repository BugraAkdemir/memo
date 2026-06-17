import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/theme.dart';
import '../providers/mood_provider.dart';

/// Engine strip'te kompakt mood göstergesi: dot + emoji + label.
/// Settings dialog'da [showLabel] = true ile genişletilmiş bar gösterir.
class MoodGauge extends ConsumerWidget {
  final bool showLabel;
  const MoodGauge({super.key, this.showLabel = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final enabled = ref.watch(moodEnabledProvider).valueOrNull ?? false;
    if (!enabled) return const SizedBox.shrink();

    final score = ref.watch(moodScoreProvider).valueOrNull ?? 0.0;
    final c = MemoTheme.of(context);

    if (showLabel) {
      return _ExpandedGauge(score: score, c: c);
    }
    return _CompactIndicator(score: score, c: c);
  }
}

// ─── Kompakt (engine strip) ──────────────────────────────────────────────────

class _CompactIndicator extends StatelessWidget {
  final double score;
  final ThemeColors c;
  const _CompactIndicator({required this.score, required this.c});

  Color get _dotColor {
    if (score <= -3) return MemoTheme.red;
    if (score >= 3) return MemoTheme.green;
    return MemoTheme.accent;
  }

  String get _emoji {
    if (score <= -7) return '😠';
    if (score <= -3) return '😒';
    if (score <= 2) return '😐';
    if (score <= 6) return '🙂';
    return '😄';
  }

  String get _label {
    if (score <= -9) return 'Breaking';
    if (score <= -7) return 'Furious';
    if (score <= -3) return 'Irritated';
    if (score <= 2) return 'Neutral';
    if (score <= 6) return 'Warm';
    return 'Elated';
  }

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: 'Ruh hali: ${score.toStringAsFixed(1)}',
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 7,
            height: 7,
            decoration: BoxDecoration(color: _dotColor, shape: BoxShape.circle),
          ),
          const SizedBox(width: 6),
          Text(_emoji, style: const TextStyle(fontSize: 13)),
          const SizedBox(width: 4),
          Text(
            _label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
              color: c.textMain,
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Genişletilmiş (settings dialog) ─────────────────────────────────────────

class _ExpandedGauge extends StatelessWidget {
  final double score;
  final ThemeColors c;
  const _ExpandedGauge({required this.score, required this.c});

  Color get _trackColor {
    if (score <= -3) return MemoTheme.red.withValues(alpha: 0.18);
    if (score >= 3) return MemoTheme.green.withValues(alpha: 0.18);
    return MemoTheme.accent.withValues(alpha: 0.12);
  }

  Color get _dotColor {
    if (score <= -3) return MemoTheme.red;
    if (score >= 3) return MemoTheme.green;
    return MemoTheme.accent;
  }

  String get _emoji {
    if (score <= -7) return '😠';
    if (score <= -3) return '😒';
    if (score <= 2) return '😐';
    if (score <= 6) return '🙂';
    return '😄';
  }

  @override
  Widget build(BuildContext context) {
    const barWidth = 200.0;
    const barHeight = 8.0;
    final t = ((score + 10) / 20).clamp(0.0, 1.0);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Tooltip(
          message: 'Ruh hali: ${score.toStringAsFixed(1)}',
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(_emoji, style: const TextStyle(fontSize: 18)),
              const SizedBox(width: 6),
              SizedBox(
                width: barWidth,
                height: barHeight,
                child: CustomPaint(
                  painter: _GaugePainter(
                    t: t,
                    trackColor: _trackColor,
                    dotColor: _dotColor,
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 4),
        SizedBox(
          width: barWidth + 24,
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('-10', style: TextStyle(fontSize: 10, color: c.textDim)),
              Text('0', style: TextStyle(fontSize: 10, color: c.textDim)),
              Text('+10', style: TextStyle(fontSize: 10, color: c.textDim)),
            ],
          ),
        ),
      ],
    );
  }
}

class _GaugePainter extends CustomPainter {
  final double t;
  final Color trackColor;
  final Color dotColor;
  const _GaugePainter({
    required this.t,
    required this.trackColor,
    required this.dotColor,
  });

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        Rect.fromLTWH(0, size.height * 0.25, size.width, size.height * 0.5),
        const Radius.circular(3),
      ),
      Paint()
        ..color = trackColor
        ..style = PaintingStyle.fill,
    );
    final centerX = size.width / 2;
    canvas.drawRect(
      Rect.fromLTWH(centerX - 0.5, size.height * 0.15, 1, size.height * 0.7),
      Paint()..color = dotColor.withValues(alpha: 0.4),
    );
    canvas.drawCircle(
      Offset(t * size.width, size.height / 2),
      size.height * 0.75,
      Paint()
        ..color = dotColor
        ..style = PaintingStyle.fill,
    );
  }

  @override
  bool shouldRepaint(_GaugePainter old) =>
      old.t != t || old.dotColor != dotColor;
}
