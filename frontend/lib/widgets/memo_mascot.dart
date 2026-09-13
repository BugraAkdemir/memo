import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../core/l10n.dart';

/// A one-shot idle flourish (wave, hop, sway) played at random intervals
/// while [MascotMood.idle] so the character doesn't read as frozen between
/// real activity — separate from the continuous breathing/blink loop, which
/// keeps running underneath regardless of mood.
enum _IdleGesture { none, wave, hop, sway }

/// What Memo's mascot is currently "doing" — driven by GET
/// /api/mascot/activity (see mascot_window.dart). [done] is a brief
/// "just finished" beat (models.ActivityDone on the Go side): visually the
/// same celebratory pose as [generating], reported as its own mood only so
/// the status bubble can show a distinct "Completed!" instead of either
/// lingering on "Preparing a response…" or silently going blank.
enum MascotMood { idle, thinking, writing, generating, tool, done }

/// Memo's mascot: a warm mocha-and-gold character, hand-drawn as vector
/// shapes (no external asset) so every state renders crisply at any size.
/// Runs a continuous idle loop (breathing, antenna wobble, blink) regardless
/// of [mood], plus a mood-specific pose, prop and secondary motion. While
/// [MascotMood.idle], it also plays an occasional random flourish (wave,
/// hop, sway — see [_IdleGesture]) so it doesn't read as frozen between
/// real activity.
class MemoMascot extends StatefulWidget {
  final MascotMood mood;
  final double size;

  const MemoMascot({super.key, this.mood = MascotMood.idle, this.size = 96});

  @override
  State<MemoMascot> createState() => _MemoMascotState();
}

class _MemoMascotState extends State<MemoMascot>
    with TickerProviderStateMixin {
  late final AnimationController _loop;
  late final AnimationController _gesture;
  Timer? _gestureTimer;
  _IdleGesture _activeGesture = _IdleGesture.none;
  final math.Random _rng = math.Random();

  // One long looping controller; every sub-animation derives its own phase
  // from elapsed seconds via modulo, rather than juggling several
  // controllers that would need their periods kept in sync.
  static const double _loopSeconds = 20;

  @override
  void initState() {
    super.initState();
    _loop = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 20),
    )..repeat();
    _gesture = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1100),
    );
    if (widget.mood == MascotMood.idle) _scheduleNextGesture();
  }

  @override
  void didUpdateWidget(covariant MemoMascot oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.mood == widget.mood) return;
    if (widget.mood != MascotMood.idle) {
      // Real activity started — stop mid-flourish rather than let a wave
      // finish while the mascot is supposed to be thinking/working.
      _gestureTimer?.cancel();
      _gestureTimer = null;
      _gesture.stop();
      if (_activeGesture != _IdleGesture.none) {
        setState(() => _activeGesture = _IdleGesture.none);
      }
    } else if (_gestureTimer == null) {
      _scheduleNextGesture();
    }
  }

  void _scheduleNextGesture() {
    _gestureTimer?.cancel();
    // Randomized so the idle character never settles into a predictable
    // rhythm — long enough gaps that a gesture reads as a deliberate
    // flourish, not a nervous tic.
    final delay = Duration(milliseconds: 3500 + _rng.nextInt(6000));
    _gestureTimer = Timer(delay, _playRandomGesture);
  }

  Future<void> _playRandomGesture() async {
    if (!mounted || widget.mood != MascotMood.idle) {
      _scheduleNextGesture();
      return;
    }
    const options = [_IdleGesture.wave, _IdleGesture.hop, _IdleGesture.sway];
    setState(() => _activeGesture = options[_rng.nextInt(options.length)]);
    await _gesture.forward(from: 0);
    if (!mounted) return;
    setState(() => _activeGesture = _IdleGesture.none);
    if (widget.mood == MascotMood.idle) _scheduleNextGesture();
  }

  @override
  void dispose() {
    _gestureTimer?.cancel();
    _loop.dispose();
    _gesture.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: L10n.t('mascot_semantic_label'),
      child: SizedBox(
        width: widget.size,
        height: widget.size * 1.05,
        child: AnimatedBuilder(
          animation: Listenable.merge([_loop, _gesture]),
          builder: (context, _) {
            final t = _loop.value * _loopSeconds;
            return CustomPaint(
              painter: _MascotPainter(
                mood: widget.mood,
                t: t,
                gesture: _activeGesture,
                gestureT: _gesture.value,
              ),
            );
          },
        ),
      ),
    );
  }
}

// ─── Palette ──────────────────────────────────────────────────────────────
const _cBody = Color(0xFF4A3F35);
const _cBodyShadow = Color(0xFF362D25);
const _cBodyHighlight = Color(0xFF5C4E40);
const _cEye = Color(0xFFE3A94B);
const _cGlint = Color(0xFFFFF6E8);
const _cBlush = Color(0xFFD98B4F);
const _cMouth = Color(0xFF2A211A);
const _cPaper = Color(0xFFE8DCC8);
const _cPaperLine = Color(0xFF8A7357);

double _pingPong(double phase) {
  final p = phase - phase.floorToDouble();
  return (1 - math.cos(2 * math.pi * p)) / 2; // 0 at p=0, 1 at p=0.5, 0 at p=1
}

double _frac(double x) => x - x.floorToDouble();

class _MascotPainter extends CustomPainter {
  final MascotMood mood;
  final double t;
  final _IdleGesture gesture;
  final double gestureT;

  _MascotPainter({
    required this.mood,
    required this.t,
    this.gesture = _IdleGesture.none,
    this.gestureT = 0,
  });

  /// 0 at the start/end of a gesture, 1 at its midpoint — the shared
  /// envelope every gesture eases in and out of instead of snapping.
  double get _gestureEnvelope =>
      gesture == _IdleGesture.none ? 0 : math.sin(gestureT.clamp(0, 1) * math.pi);

  // Design space: a fixed local coordinate system the character is drawn
  // in, then uniformly scaled to fit whatever pixel size the widget gets.
  static const double _designW = 200;
  static const Offset _origin = Offset(100, 112);

  @override
  void paint(Canvas canvas, Size size) {
    final scale = size.width / _designW;
    canvas.save();
    canvas.scale(scale);
    canvas.translate(_origin.dx, _origin.dy);

    _drawShadow(canvas);

    // Sway and hop are whole-body flourishes — applied here, before the
    // breathe scale and limb drawing below, so every part of the character
    // moves together instead of the sway looking like just a head tilt.
    canvas.save();
    if (gesture == _IdleGesture.sway) {
      final angle = 9 * math.pi / 180 * math.sin(gestureT.clamp(0, 1) * math.pi * 2);
      canvas.rotate(angle);
    }
    if (gesture == _IdleGesture.hop) {
      final g = gestureT.clamp(0.0, 1.0);
      final hopDy = -22 * 4 * g * (1 - g); // parabolic arc, peak mid-gesture
      canvas.translate(0, hopDy);
    }

    // Squash & stretch: neutral at the loop's start/midpoint, extreme at
    // the quarter-points in between (matches the source design's
    // 0%/50%/100% keyframes).
    final breathe = _pingPong(t / 2.6);
    final scaleX = 1 + 0.02 * breathe;
    final scaleY = 1 - 0.035 * breathe;
    canvas.save();
    canvas.scale(scaleX, scaleY);

    _drawFeet(canvas);
    _drawArmsBehind(canvas);
    _drawBody(canvas);
    _drawTopHighlight(canvas);
    _drawAntenna(canvas);
    _drawArmsFront(canvas);
    _drawFaceVignette(canvas);
    _drawBlush(canvas);
    _drawEyesAndMouth(canvas);
    _drawProps(canvas);

    canvas.restore(); // breathe
    _drawParticles(canvas);

    canvas.restore(); // sway/hop
    canvas.restore(); // scale + translate
  }

  void _drawShadow(Canvas canvas) {
    final paint = Paint()..color = Colors.black.withValues(alpha: 0.22);
    canvas.drawOval(Rect.fromCenter(center: const Offset(0, 78), width: 88, height: 18), paint);
  }

  void _drawFeet(Canvas canvas) {
    final paint = Paint()..color = _cBodyShadow;
    canvas.drawOval(Rect.fromCenter(center: const Offset(-30, 57), width: 40, height: 22), paint);
    canvas.drawOval(Rect.fromCenter(center: const Offset(30, 57), width: 40, height: 22), paint);
  }

  Path _bodyPath() {
    return Path()
      ..moveTo(0, -60)
      ..cubicTo(-26, -60, -41, -43, -43, -18)
      ..cubicTo(-48, -9, -48, 21, -39, 34)
      ..cubicTo(-29, 51, -12, 60, 0, 60)
      ..cubicTo(12, 60, 29, 51, 39, 34)
      ..cubicTo(48, 21, 48, -9, 43, -18)
      ..cubicTo(41, -43, 26, -60, 0, -60)
      ..close();
  }

  void _drawBody(Canvas canvas) {
    canvas.drawPath(_bodyPath(), Paint()..color = _cBody);
  }

  void _drawTopHighlight(Canvas canvas) {
    canvas.save();
    canvas.translate(-15, -38);
    canvas.rotate(-18 * math.pi / 180);
    canvas.drawOval(
      Rect.fromCenter(center: Offset.zero, width: 30, height: 22),
      Paint()..color = _cBodyHighlight.withValues(alpha: 0.35),
    );
    canvas.restore();
  }

  void _drawAntenna(Canvas canvas) {
    final wobblePhase = _pingPong((t - 0.12) / 2.6);
    final angle = 6 * math.pi / 180 * wobblePhase;
    canvas.save();
    canvas.translate(0, -60);
    canvas.rotate(angle);
    canvas.drawLine(
      Offset.zero,
      const Offset(0, -23),
      Paint()
        ..color = _cBodyHighlight
        ..strokeWidth = 6
        ..strokeCap = StrokeCap.round,
    );
    final glowing =
        mood == MascotMood.generating || mood == MascotMood.tool || mood == MascotMood.done;
    final glowAlpha = glowing ? 0.4 + 0.6 * ((math.sin(2 * math.pi * t / 1.6) + 1) / 2) : 1.0;
    canvas.drawCircle(const Offset(0, -27), 7, Paint()..color = _cEye.withValues(alpha: glowAlpha));
    canvas.restore();
  }

  void _drawFaceVignette(Canvas canvas) {
    final layers = [
      (35.0, 30.0, 0.22),
      (28.0, 24.0, 0.28),
      (20.0, 18.0, 0.32),
    ];
    for (final (rx, ry, alpha) in layers) {
      canvas.drawOval(
        Rect.fromCenter(center: const Offset(0, -8), width: rx * 2, height: ry * 2),
        Paint()..color = _cBodyShadow.withValues(alpha: alpha),
      );
    }
  }

  void _drawBlush(Canvas canvas) {
    final paint = Paint()..color = _cBlush.withValues(alpha: 0.3);
    canvas.drawOval(Rect.fromCenter(center: const Offset(-26, 6), width: 20, height: 11), paint);
    canvas.drawOval(Rect.fromCenter(center: const Offset(26, 6), width: 20, height: 11), paint);
  }

  double _blinkScaleY() {
    const period = 4.6;
    final p = _frac(t / period);
    if (p < 0.90 || p > 0.97) return 1.0;
    final local = (p - 0.90) / 0.07;
    return 1 - 0.88 * math.sin(math.pi * local);
  }

  void _drawEyesAndMouth(Canvas canvas) {
    final eyePaint = Paint()..color = _cEye;
    final mouthPaint = Paint()
      ..color = _cMouth
      ..style = PaintingStyle.stroke
      ..strokeWidth = 3
      ..strokeCap = StrokeCap.round;

    if (mood == MascotMood.generating || mood == MascotMood.done) {
      // Happy closed eyes — no blink, this expression already reads as joy.
      final arcPaint = Paint()
        ..color = _cEye
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3.4
        ..strokeCap = StrokeCap.round;
      final left = Path()..moveTo(-20, -12)..quadraticBezierTo(-12, -3, -3, -12);
      final right = Path()..moveTo(3, -12)..quadraticBezierTo(12, -3, 20, -12);
      canvas.drawPath(left, arcPaint);
      canvas.drawPath(right, arcPaint);
      canvas.drawPath(Path()..moveTo(-11, 16)..quadraticBezierTo(0, 28, 11, 16), mouthPaint);
      return;
    }

    late final Rect eyeL, eyeR;
    switch (mood) {
      case MascotMood.thinking:
        eyeL = Rect.fromLTWH(-21, -27, 8, 13);
        eyeR = Rect.fromLTWH(13, -27, 8, 13);
        break;
      case MascotMood.writing:
        eyeL = Rect.fromLTWH(-21, -14, 8, 10);
        eyeR = Rect.fromLTWH(13, -14, 8, 10);
        break;
      case MascotMood.tool:
        eyeL = Rect.fromLTWH(-20, -26, 7, 11);
        eyeR = Rect.fromLTWH(13, -26, 7, 11);
        break;
      case MascotMood.idle:
      case MascotMood.generating:
      case MascotMood.done:
        // generating/done never actually reach here (handled above), kept
        // only so this switch stays exhaustive.
        eyeL = Rect.fromLTWH(-21, -21, 8, 13);
        eyeR = Rect.fromLTWH(13, -21, 8, 13);
        break;
    }

    final blinkY = _blinkScaleY();
    for (final r in [eyeL, eyeR]) {
      canvas.save();
      canvas.translate(r.center.dx, r.center.dy);
      canvas.scale(1, blinkY);
      canvas.translate(-r.center.dx, -r.center.dy);
      canvas.drawRRect(
        RRect.fromRectAndRadius(r, Radius.circular(r.width / 2)),
        eyePaint,
      );
      canvas.restore();
    }

    canvas.drawPath(Path()..moveTo(-9, 16)..quadraticBezierTo(0, 24, 9, 16), mouthPaint);
  }

  Paint _armPaint() => Paint()
    ..color = _cBody
    ..style = PaintingStyle.stroke
    ..strokeWidth = 17
    ..strokeCap = StrokeCap.round;

  void _drawArmsBehind(Canvas canvas) {
    // Arms drawn before the body for poses where a limb tucks behind it.
    if (mood == MascotMood.thinking) {
      canvas.save();
      canvas.translate(-45, 19);
      canvas.rotate(-10 * math.pi / 180);
      canvas.drawRRect(
        RRect.fromRectAndRadius(
          Rect.fromCenter(center: Offset.zero, width: 19, height: 38),
          const Radius.circular(9),
        ),
        Paint()..color = _cBody,
      );
      canvas.restore();
    }
  }

  void _drawArmsFront(Canvas canvas) {
    final bodyFill = Paint()..color = _cBody;
    switch (mood) {
      case MascotMood.idle:
        // Left arm: normal resting pose, always.
        canvas.save();
        canvas.translate(-45.5, -5);
        canvas.rotate(6 * math.pi / 180);
        canvas.drawRRect(
          RRect.fromRectAndRadius(
            Rect.fromCenter(center: Offset.zero, width: 19, height: 30),
            const Radius.circular(9),
          ),
          bodyFill,
        );
        canvas.restore();

        if (gesture == _IdleGesture.wave) {
          // Right arm: raises and wiggles side to side — a "hi" wave.
          final lift = _gestureEnvelope;
          final wiggle = math.sin(gestureT.clamp(0, 1) * math.pi * 7) * 16 * lift;
          canvas.save();
          canvas.translate(38, 4 - 34 * lift);
          canvas.rotate((-70 * lift + wiggle) * math.pi / 180);
          canvas.drawRRect(
            RRect.fromRectAndRadius(
              const Rect.fromLTWH(-9.5, 0, 19, 30),
              const Radius.circular(9),
            ),
            bodyFill,
          );
          canvas.restore();
        } else {
          canvas.save();
          canvas.translate(45.5, -5);
          canvas.rotate(-6 * math.pi / 180);
          canvas.drawRRect(
            RRect.fromRectAndRadius(
              Rect.fromCenter(center: Offset.zero, width: 19, height: 30),
              const Radius.circular(9),
            ),
            bodyFill,
          );
          canvas.restore();
        }
        break;
      case MascotMood.thinking:
        final p = Path()..moveTo(43, 7)..quadraticBezierTo(58, -7, 26, -21);
        canvas.drawPath(p, _armPaint());
        canvas.drawCircle(const Offset(24, -22), 9, bodyFill);
        break;
      case MascotMood.writing:
        final tapPhase = math.max(0.0, math.sin(2 * math.pi * t / 0.7));
        final tapPhaseR = math.max(0.0, math.sin(2 * math.pi * (t - 0.35) / 0.7));
        final left = Path()
          ..moveTo(-39, 9)
          ..quadraticBezierTo(-34, 27, -14, 33 + 3 * tapPhase);
        final right = Path()
          ..moveTo(39, 9)
          ..quadraticBezierTo(34, 27, 14, 33 + 3 * tapPhaseR);
        canvas.drawPath(left, _armPaint()..strokeWidth = 15);
        canvas.drawCircle(Offset(-13, 33 + 3 * tapPhase), 7.7, bodyFill);
        canvas.drawPath(right, _armPaint()..strokeWidth = 15);
        canvas.drawCircle(Offset(13, 33 + 3 * tapPhaseR), 7.7, bodyFill);
        break;
      case MascotMood.generating:
      case MascotMood.done:
        final left = Path()..moveTo(-39, 9)..quadraticBezierTo(-62, -9, -51, -34);
        final right = Path()..moveTo(39, 9)..quadraticBezierTo(62, -9, 51, -34);
        canvas.drawPath(left, _armPaint());
        canvas.drawCircle(const Offset(-51, -36), 9.4, bodyFill);
        canvas.drawPath(right, _armPaint());
        canvas.drawCircle(const Offset(51, -36), 9.4, bodyFill);
        break;
      case MascotMood.tool:
        canvas.save();
        canvas.translate(-46, 21);
        canvas.rotate(8 * math.pi / 180);
        canvas.drawRRect(
          RRect.fromRectAndRadius(
            Rect.fromCenter(center: Offset.zero, width: 17, height: 34),
            const Radius.circular(8.5),
          ),
          bodyFill,
        );
        canvas.restore();
        final right = Path()..moveTo(39, 9)..quadraticBezierTo(57, -15, 34, -47);
        canvas.drawPath(right, _armPaint());
        canvas.drawCircle(const Offset(33, -49), 8.6, bodyFill);
        break;
    }
  }

  void _drawProps(Canvas canvas) {
    switch (mood) {
      case MascotMood.writing:
        canvas.drawOval(
          Rect.fromCenter(center: const Offset(0, 56), width: 52, height: 10),
          Paint()..color = Colors.black.withValues(alpha: 0.15),
        );
        canvas.drawRRect(
          RRect.fromRectAndRadius(
            const Rect.fromLTWH(-21, 26, 43, 27),
            const Radius.circular(5),
          ),
          Paint()..color = _cPaper,
        );
        final linePaint = Paint()..color = _cPaperLine.withValues(alpha: 0.7);
        canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(-15, 33, 24, 2.5), const Radius.circular(1.2)), linePaint);
        canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(-15, 39, 17, 2.5), const Radius.circular(1.2)), linePaint);
        canvas.drawRRect(RRect.fromRectAndRadius(const Rect.fromLTWH(-15, 45, 21, 2.5), const Radius.circular(1.2)), linePaint);
        final finger = Paint()..color = _cBody;
        canvas.drawOval(Rect.fromCenter(center: const Offset(-15, 27), width: 8, height: 10), finger);
        canvas.drawOval(Rect.fromCenter(center: const Offset(-8, 25), width: 6.6, height: 8.6), finger);
        canvas.drawOval(Rect.fromCenter(center: const Offset(15, 27), width: 8, height: 10), finger);
        canvas.drawOval(Rect.fromCenter(center: const Offset(8, 25), width: 6.6, height: 8.6), finger);
        break;
      case MascotMood.tool:
        final ring = Paint()
          ..color = _cEye
          ..style = PaintingStyle.stroke
          ..strokeWidth = 3.4;
        canvas.drawCircle(const Offset(33, -65), 7.7, ring);
        final glintAlpha = 0.4 + 0.6 * ((math.sin(2 * math.pi * t / 1.6) + 1) / 2);
        canvas.drawOval(
          Rect.fromCenter(center: const Offset(30, -68), width: 4.2, height: 3.4),
          Paint()..color = _cGlint.withValues(alpha: glintAlpha),
        );
        final keyPaint = Paint()..color = _cEye;
        canvas.drawRect(const Rect.fromLTWH(30, -57, 5, 14), keyPaint);
        canvas.drawRect(const Rect.fromLTWH(35, -50, 5, 3.4), keyPaint);
        canvas.drawRect(const Rect.fromLTWH(35, -44, 5, 3.4), keyPaint);
        break;
      case MascotMood.idle:
      case MascotMood.thinking:
      case MascotMood.generating:
      case MascotMood.done:
        break;
    }
  }

  void _drawStar(Canvas canvas, Offset center, double scale, double opacity) {
    final path = Path()
      ..moveTo(0, -7)
      ..lineTo(1.75, -1.75)
      ..lineTo(7, 0)
      ..lineTo(1.75, 1.75)
      ..lineTo(0, 7)
      ..lineTo(-1.75, 1.75)
      ..lineTo(-7, 0)
      ..lineTo(-1.75, -1.75)
      ..close();
    canvas.save();
    canvas.translate(center.dx, center.dy);
    canvas.scale(scale);
    canvas.drawPath(path, Paint()..color = _cEye.withValues(alpha: opacity));
    canvas.restore();
  }

  ({double opacity, double dy, double scale}) _rise(double phase) {
    if (phase < 0.3) {
      final seg = phase / 0.3;
      return (opacity: seg, dy: 6 * (1 - seg), scale: 0.7 + 0.3 * seg);
    }
    final seg = (phase - 0.3) / 0.7;
    return (opacity: 1 - seg, dy: -8 * seg, scale: 1.0);
  }

  ({double opacity, double dy, double scale, double rotation}) _spark(double phase) {
    if (phase < 0.35) {
      final seg = phase / 0.35;
      return (opacity: seg, dy: 6 * (1 - seg), scale: 0.6 + 0.4 * seg, rotation: 0);
    }
    final seg = (phase - 0.35) / 0.65;
    return (opacity: 1 - seg, dy: 6 - 20 * seg, scale: 1.0, rotation: 40 * math.pi / 180 * seg);
  }

  void _drawParticles(Canvas canvas) {
    if (mood == MascotMood.thinking) {
      final positions = [
        (const Offset(47, -62), 4.3, 0.0),
        (const Offset(58, -77), 5.6, 0.3),
        (const Offset(71, -94), 6.9, 0.6),
      ];
      for (final (pos, r, delay) in positions) {
        final phase = _frac((t - delay) / 1.8);
        final s = _rise(phase);
        canvas.drawCircle(
          pos.translate(0, s.dy),
          r * s.scale,
          Paint()..color = _cEye.withValues(alpha: s.opacity.clamp(0, 1)),
        );
      }
    } else if (mood == MascotMood.generating || mood == MascotMood.done) {
      final specs = [
        (const Offset(26, -66), 1.0, 0.0),
        (const Offset(44, -46), 0.8, 0.5),
      ];
      for (final (pos, scale, delay) in specs) {
        final phase = _frac((t - delay) / 1.8);
        final s = _spark(phase);
        canvas.save();
        canvas.translate(pos.dx, pos.dy + s.dy);
        canvas.rotate(s.rotation);
        _drawStar(canvas, Offset.zero, scale * s.scale, s.opacity.clamp(0, 1));
        canvas.restore();
      }
      final phase = _frac((t - 1.0) / 1.8);
      final s = _spark(phase);
      canvas.drawCircle(
        Offset(-44, -42 + s.dy),
        3.5 * s.scale,
        Paint()..color = _cEye.withValues(alpha: s.opacity.clamp(0, 1)),
      );
    }
  }

  @override
  bool shouldRepaint(covariant _MascotPainter oldDelegate) =>
      oldDelegate.t != t ||
      oldDelegate.mood != mood ||
      oldDelegate.gesture != gesture ||
      oldDelegate.gestureT != gestureT;
}
