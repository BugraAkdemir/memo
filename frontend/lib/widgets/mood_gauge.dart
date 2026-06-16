import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/theme.dart';
import '../providers/mood_provider.dart';

/// Yatay ruh hali göstergesi: -10 ←——●——→ +10
/// Engine strip'e eklenir, her zaman görünür.
class MoodGauge extends ConsumerWidget {
  const MoodGauge({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scoreAsync = ref.watch(moodScoreProvider);
    final c = MemoTheme.of(context);

    return scoreAsync.when(
      loading: () => _GaugeBar(score: 0.0, c: c),
      error: (_, __) => const SizedBox.shrink(),
      data: (score) => _GaugeBar(score: score, c: c),
    );
  }
}

class _GaugeBar extends StatelessWidget {
  final double score; // -10..+10
  final ThemeColors c;
  const _GaugeBar({required this.score, required this.c});

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
    // Normalize: -10..+10 → 0..1
    final t = ((score + 10) / 20).clamp(0.0, 1.0);

    return Tooltip(
      message: 'Ruh hali: ${score.toStringAsFixed(1)}',
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(_emoji, style: const TextStyle(fontSize: 13)),
          const SizedBox(width: 6),
          SizedBox(
            width: 80,
            height: 6,
            child: CustomPaint(
              painter: _GaugePainter(t: t, trackColor: _trackColor, dotColor: _dotColor),
            ),
          ),
        ],
      ),
    );
  }
}

class _GaugePainter extends CustomPainter {
  final double t;
  final Color trackColor;
  final Color dotColor;
  const _GaugePainter({required this.t, required this.trackColor, required this.dotColor});

  @override
  void paint(Canvas canvas, Size size) {
    final trackPaint = Paint()
      ..color = trackColor
      ..style = PaintingStyle.fill;
    final dotPaint = Paint()
      ..color = dotColor
      ..style = PaintingStyle.fill;

    // Track
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        Rect.fromLTWH(0, size.height * 0.25, size.width, size.height * 0.5),
        const Radius.circular(3),
      ),
      trackPaint,
    );

    // Center mark
    final centerX = size.width / 2;
    canvas.drawRect(
      Rect.fromLTWH(centerX - 0.5, size.height * 0.15, 1, size.height * 0.7),
      Paint()..color = dotColor.withValues(alpha: 0.4),
    );

    // Dot
    final dotX = t * size.width;
    canvas.drawCircle(
      Offset(dotX, size.height / 2),
      size.height * 0.75,
      dotPaint,
    );
  }

  @override
  bool shouldRepaint(_GaugePainter old) => old.t != t || old.dotColor != dotColor;
}
