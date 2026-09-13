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

/// Which character [MemoMascot] renders — a user-facing choice (Settings >
/// General, `mascotSkinProvider`), not a mood. Both skins share the exact
/// same [MascotMood]/gesture rig (timing, poses, particles), just painted
/// by a different [CustomPainter]; see [_PixelMascotPainter]'s doc comment
/// for how it mirrors [_MascotPainter]'s geometry on purpose.
enum MascotSkin { classic, pixel }

/// Converts [MascotSkin] to/from the raw string stored in SharedPreferences
/// (`memo_mascot_skin`, see settings_provider.dart's `mascotSkinProvider`).
/// Kept here rather than importing this enum into the providers layer, the
/// same way `memo_theme_mode` stores a plain 'light'/'dark' string instead
/// of an enum.
extension MascotSkinPrefValue on MascotSkin {
  String get prefValue => switch (this) {
        MascotSkin.classic => 'classic',
        MascotSkin.pixel => 'pixel',
      };

  static MascotSkin fromPrefValue(String? value) =>
      value == 'pixel' ? MascotSkin.pixel : MascotSkin.classic;
}

/// Memo's mascot: hand-drawn as vector shapes (no external asset, no
/// runtime image decode) so every state renders crisply at any size. Two
/// interchangeable [skin]s — [MascotSkin.classic]'s warm mocha-and-gold
/// creature, or [MascotSkin.pixel]'s blocky blue-navy robot — both driven
/// by the identical [mood]/gesture rig below. Runs a continuous idle loop
/// (breathing, antenna wobble, blink) regardless of [mood], plus a
/// mood-specific pose, prop and secondary motion. While [MascotMood.idle],
/// it also plays an occasional random flourish (wave, hop, sway — see
/// [_IdleGesture]) so it doesn't read as frozen between real activity.
class MemoMascot extends StatefulWidget {
  final MascotMood mood;
  final MascotSkin skin;
  final double size;

  const MemoMascot({
    super.key,
    this.mood = MascotMood.idle,
    this.skin = MascotSkin.classic,
    this.size = 96,
  });

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
              painter: switch (widget.skin) {
                MascotSkin.classic => _MascotPainter(
                    mood: widget.mood,
                    t: t,
                    gesture: _activeGesture,
                    gestureT: _gesture.value,
                  ),
                MascotSkin.pixel => _PixelMascotPainter(
                    mood: widget.mood,
                    t: t,
                    gesture: _activeGesture,
                    gestureT: _gesture.value,
                  ),
              },
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

// ─── Pixel skin — palette ───────────────────────────────────────────────
const _pxBody = Color(0xFF3D6FE0);
const _pxBodyShadow = Color(0xFF2A4FB8);
const _pxBodyLight = Color(0xFF8FB4F5);
const _pxScreen = Color(0xFF0F1B33);
const _pxGlow = Color(0xFF7FE0EE);
const _pxGlowSoft = Color(0xFFBFEFF5);
const _pxPaper = Color(0xFFE7F0FF);
const _pxPaperLine = Color(0xFF9FB7E8);

/// One "pixel" unit for [_PixelMascotPainter] — every shape's size snaps to
/// a multiple of this so edges line up on a grid instead of floating at
/// arbitrary sub-pixel offsets, which is what actually reads as pixel art
/// rather than just "a vector shape with sharp corners."
const double _pxUnit = 6;

/// [MascotSkin.pixel]'s character: a small blue-navy robot with a screen
/// face, built from blocky, grid-snapped rectangles (see [_pixelBlock])
/// instead of [_MascotPainter]'s smooth bezier curves. Deliberately mirrors
/// that class's structure and, where the pose allows, its exact numbers
/// (arm attachment points, particle timing, gesture transforms) — the two
/// skins are meant to feel like the same rig wearing a different look, not
/// two unrelated characters, per the user's request that both "yine aynı
/// animasyonlar olsun."
class _PixelMascotPainter extends CustomPainter {
  final MascotMood mood;
  final double t;
  final _IdleGesture gesture;
  final double gestureT;

  _PixelMascotPainter({
    required this.mood,
    required this.t,
    this.gesture = _IdleGesture.none,
    this.gestureT = 0,
  });

  double get _gestureEnvelope =>
      gesture == _IdleGesture.none ? 0 : math.sin(gestureT.clamp(0, 1) * math.pi);

  static const double _designW = 200;
  static const Offset _origin = Offset(100, 130);

  /// Approximates a rounded rect as stacked horizontal bands, each snapped
  /// to [_pxUnit] — the same "rasterize a curve into bands" idea
  /// my_application.cc's set_mascot_input_shape uses for the native input
  /// ellipse, just in Dart and for pixels instead of a click region.
  void _pixelBlock(
    Canvas canvas, {
    required double cx,
    required double top,
    required double width,
    required double height,
    required double cornerRadius,
    required Paint paint,
    double? shadeBelow,
    Paint? shadePaint,
  }) {
    final bands = (height / _pxUnit).round().clamp(1, 200);
    final bandH = height / bands;
    for (int i = 0; i < bands; i++) {
      final bandCenterY = top + (i + 0.5) * bandH;
      final edgeDist = math.min(bandCenterY - top, (top + height) - bandCenterY);
      double inset = 0;
      if (edgeDist < cornerRadius) {
        final frac = edgeDist / cornerRadius;
        final raw = cornerRadius * (1 - math.sqrt((1 - (1 - frac) * (1 - frac)).clamp(0, 1)));
        inset = (raw / _pxUnit).ceil() * _pxUnit;
        inset = inset.clamp(0, width / 2 - 1);
      }
      final w = width - 2 * inset;
      if (w <= 0) continue;
      final useShade = shadeBelow != null && bandCenterY >= shadeBelow;
      canvas.drawRect(
        Rect.fromCenter(center: Offset(cx, bandCenterY), width: w, height: bandH + 0.5),
        useShade ? (shadePaint ?? paint) : paint,
      );
    }
  }

  @override
  void paint(Canvas canvas, Size size) {
    final scale = size.width / _designW;
    canvas.save();
    canvas.scale(scale);
    canvas.translate(_origin.dx, _origin.dy);

    _drawShadow(canvas);

    canvas.save();
    if (gesture == _IdleGesture.sway) {
      final angle = 9 * math.pi / 180 * math.sin(gestureT.clamp(0, 1) * math.pi * 2);
      canvas.rotate(angle);
    }
    if (gesture == _IdleGesture.hop) {
      final g = gestureT.clamp(0.0, 1.0);
      final hopDy = -22 * 4 * g * (1 - g);
      canvas.translate(0, hopDy);
    }

    final breathe = _pingPong(t / 2.6);
    final scaleY = 1 - 0.03 * breathe;
    canvas.save();
    canvas.scale(1 + 0.015 * breathe, scaleY);

    _drawFeet(canvas);
    _drawArmsBehind(canvas);
    _drawBody(canvas);
    _drawAntenna(canvas);
    _drawArmsFront(canvas);
    _drawScreenFace(canvas);
    _drawProps(canvas);

    canvas.restore(); // breathe
    _drawParticles(canvas);

    canvas.restore(); // sway/hop
    canvas.restore(); // scale + translate
  }

  void _drawShadow(Canvas canvas) {
    final paint = Paint()..color = Colors.black.withValues(alpha: 0.2);
    canvas.drawOval(Rect.fromCenter(center: const Offset(0, 66), width: 82, height: 16), paint);
  }

  void _drawFeet(Canvas canvas) {
    final paint = Paint()..color = _pxBodyShadow;
    canvas.drawRect(Rect.fromCenter(center: const Offset(-22, 54), width: 18, height: 11), paint);
    canvas.drawRect(Rect.fromCenter(center: const Offset(22, 54), width: 18, height: 11), paint);
  }

  void _drawBody(Canvas canvas) {
    _pixelBlock(
      canvas,
      cx: 0,
      top: -97,
      width: 96,
      height: 82,
      cornerRadius: 24,
      paint: Paint()..color = _pxBody,
      shadeBelow: -30,
      shadePaint: Paint()..color = _pxBodyShadow,
    );
    canvas.drawRect(
      Rect.fromCenter(center: const Offset(-30, -84), width: 16, height: 10),
      Paint()..color = _pxBodyLight.withValues(alpha: 0.55),
    );

    _pixelBlock(
      canvas,
      cx: 0,
      top: -6,
      width: 78,
      height: 56,
      cornerRadius: 16,
      paint: Paint()..color = _pxBody,
      shadeBelow: 28,
      shadePaint: Paint()..color = _pxBodyShadow,
    );

    final chestPulse = 0.5 + 0.5 * math.sin(2 * math.pi * t / 2.4);
    canvas.drawRect(
      Rect.fromCenter(center: const Offset(0, 18), width: 10, height: 10),
      Paint()..color = _pxGlow.withValues(alpha: 0.5 + 0.4 * chestPulse),
    );
  }

  void _drawAntenna(Canvas canvas) {
    final wobblePhase = _pingPong((t - 0.12) / 2.6);
    final angle = 6 * math.pi / 180 * wobblePhase;
    canvas.save();
    canvas.translate(0, -97);
    canvas.rotate(angle);
    canvas.drawRect(
      Rect.fromCenter(center: const Offset(0, -12), width: 6, height: 24),
      Paint()..color = _pxBodyShadow,
    );
    final glowing =
        mood == MascotMood.generating || mood == MascotMood.tool || mood == MascotMood.done;
    final glowAlpha = glowing ? 0.5 + 0.5 * ((math.sin(2 * math.pi * t / 1.6) + 1) / 2) : 1.0;
    canvas.drawRect(
      Rect.fromCenter(center: const Offset(0, -26), width: 11, height: 11),
      Paint()..color = _pxGlow.withValues(alpha: glowAlpha),
    );
    canvas.restore();
  }

  double _blinkScaleY() {
    const period = 4.6;
    final p = _frac(t / period);
    if (p < 0.90 || p > 0.97) return 1.0;
    final local = (p - 0.90) / 0.07;
    return 1 - 0.88 * math.sin(math.pi * local);
  }

  void _drawScreenFace(Canvas canvas) {
    _pixelBlock(
      canvas,
      cx: 0,
      top: -78,
      width: 62,
      height: 40,
      cornerRadius: 9,
      paint: Paint()..color = _pxScreen,
    );

    final eyePaint = Paint()..color = _pxGlow;
    final happy = mood == MascotMood.generating || mood == MascotMood.done;

    late final double eyeCy, eyeH;
    switch (mood) {
      case MascotMood.thinking:
        eyeCy = -63;
        eyeH = 9;
        break;
      case MascotMood.writing:
        eyeCy = -58;
        eyeH = 5;
        break;
      case MascotMood.tool:
        eyeCy = -60;
        eyeH = 8;
        break;
      case MascotMood.idle:
      case MascotMood.generating:
      case MascotMood.done:
        eyeCy = -60;
        eyeH = 9;
        break;
    }

    if (happy) {
      // Happy squint — a flat glowing bar per eye, no blink.
      for (final dx in [-13.0, 13.0]) {
        canvas.drawRect(
          Rect.fromCenter(center: Offset(dx, eyeCy), width: 11, height: 4),
          eyePaint,
        );
      }
    } else {
      final blinkY = _blinkScaleY();
      for (final dx in [-13.0, 13.0]) {
        canvas.save();
        canvas.translate(dx, eyeCy);
        canvas.scale(1, blinkY);
        canvas.drawRect(
          Rect.fromCenter(center: Offset.zero, width: 9, height: eyeH),
          eyePaint,
        );
        canvas.restore();
      }
    }

    final mouthWidth = happy ? 22.0 : 14.0;
    canvas.drawRect(
      Rect.fromCenter(center: const Offset(0, -47), width: mouthWidth, height: 3),
      Paint()..color = _pxGlowSoft,
    );
  }

  Paint _armPaint() => Paint()..color = _pxBody;

  void _drawArmsBehind(Canvas canvas) {
    if (mood == MascotMood.thinking || mood == MascotMood.tool) {
      canvas.save();
      canvas.translate(-45, 19);
      canvas.rotate(-10 * math.pi / 180);
      canvas.drawRect(
        Rect.fromCenter(center: Offset.zero, width: 17, height: 36),
        Paint()..color = _pxBody,
      );
      canvas.restore();
    }
  }

  void _drawArmSegment(Canvas canvas, Offset shoulder, double angleDeg, double length,
      {double width = 15}) {
    canvas.save();
    canvas.translate(shoulder.dx, shoulder.dy);
    canvas.rotate(angleDeg * math.pi / 180);
    canvas.drawRect(
      Rect.fromLTWH(-width / 2, 0, width, length),
      _armPaint(),
    );
    canvas.drawRect(
      Rect.fromCenter(center: Offset(0, length), width: width * 0.7, height: width * 0.7),
      Paint()..color = _pxBodyShadow,
    );
    canvas.restore();
  }

  void _drawArmsFront(Canvas canvas) {
    switch (mood) {
      case MascotMood.idle:
        _drawArmSegment(canvas, const Offset(-42, -2), 8, 26);
        if (gesture == _IdleGesture.wave) {
          final lift = _gestureEnvelope;
          final wiggle = math.sin(gestureT.clamp(0, 1) * math.pi * 7) * 14 * lift;
          _drawArmSegment(canvas, Offset(42, 2 - 30 * lift), -160 + 70 * lift + wiggle, 26);
        } else {
          _drawArmSegment(canvas, const Offset(42, -2), -8, 26);
        }
        break;
      case MascotMood.thinking:
        _drawArmSegment(canvas, const Offset(34, 5), -125, 24);
        break;
      case MascotMood.writing:
        final tapL = math.max(0.0, math.sin(2 * math.pi * t / 0.7));
        final tapR = math.max(0.0, math.sin(2 * math.pi * (t - 0.35) / 0.7));
        _drawArmSegment(canvas, Offset(-38, 6 - 2 * tapL), 25, 26);
        _drawArmSegment(canvas, Offset(38, 6 - 2 * tapR), -25, 26);
        break;
      case MascotMood.tool:
        _drawArmSegment(canvas, const Offset(38, 2), -95, 28);
        break;
      case MascotMood.generating:
      case MascotMood.done:
        _drawArmSegment(canvas, const Offset(-38, -2), -145, 26);
        _drawArmSegment(canvas, const Offset(38, -2), 145, 26);
        break;
    }
  }

  void _drawProps(Canvas canvas) {
    switch (mood) {
      case MascotMood.writing:
        canvas.drawRect(
          Rect.fromCenter(center: const Offset(0, 40), width: 40, height: 24),
          Paint()..color = _pxPaper,
        );
        final linePaint = Paint()..color = _pxPaperLine;
        for (final dy in [33.0, 39.0, 45.0]) {
          canvas.drawRect(Rect.fromCenter(center: Offset(0, dy), width: 26, height: 2.5), linePaint);
        }
        break;
      case MascotMood.tool:
        final glintAlpha = 0.5 + 0.5 * ((math.sin(2 * math.pi * t / 1.6) + 1) / 2);
        canvas.drawRect(
          Rect.fromCenter(center: const Offset(38, -58), width: 16, height: 16),
          Paint()..color = _pxBodyShadow,
        );
        canvas.drawRect(
          Rect.fromCenter(center: const Offset(38, -58), width: 8, height: 8),
          Paint()..color = _pxGlow.withValues(alpha: glintAlpha),
        );
        break;
      case MascotMood.idle:
      case MascotMood.thinking:
      case MascotMood.generating:
      case MascotMood.done:
        break;
    }
  }

  ({double opacity, double dy}) _rise(double phase) {
    if (phase < 0.3) {
      final seg = phase / 0.3;
      return (opacity: seg, dy: 6 * (1 - seg));
    }
    final seg = (phase - 0.3) / 0.7;
    return (opacity: 1 - seg, dy: -8 * seg);
  }

  void _drawParticles(Canvas canvas) {
    if (mood == MascotMood.thinking) {
      final specs = [
        (const Offset(44, -92), 6.0, 0.0),
        (const Offset(54, -104), 7.5, 0.3),
        (const Offset(66, -118), 9.0, 0.6),
      ];
      for (final (pos, sz, delay) in specs) {
        final phase = _frac((t - delay) / 1.8);
        final s = _rise(phase);
        canvas.drawRect(
          Rect.fromCenter(center: pos.translate(0, s.dy), width: sz, height: sz),
          Paint()..color = _pxGlow.withValues(alpha: s.opacity.clamp(0, 1)),
        );
      }
    } else if (mood == MascotMood.generating || mood == MascotMood.done) {
      final specs = [
        (const Offset(-58, -90), 8.0, 0.0),
        (const Offset(58, -90), 8.0, 0.5),
        (const Offset(0, -110), 7.0, 1.0),
      ];
      for (final (pos, sz, delay) in specs) {
        final phase = _frac((t - delay) / 1.8);
        final s = _rise(phase);
        canvas.drawRect(
          Rect.fromCenter(center: pos.translate(0, s.dy), width: sz, height: sz),
          Paint()..color = _pxGlow.withValues(alpha: s.opacity.clamp(0, 1)),
        );
      }
    }
  }

  @override
  bool shouldRepaint(covariant _PixelMascotPainter oldDelegate) =>
      oldDelegate.t != t ||
      oldDelegate.mood != mood ||
      oldDelegate.gesture != gesture ||
      oldDelegate.gestureT != gestureT;
}
