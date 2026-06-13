import 'package:flutter/material.dart';

import '../core/theme.dart';

/// The Memo app icon — a bronze "M" on a warm graphite tile.
/// Mirrors memo.png / the desktop mark, drawn in-app so it scales crisply.
class MemoLogo extends StatelessWidget {
  final double size;
  const MemoLogo({super.key, this.size = 40});

  @override
  Widget build(BuildContext context) {
    final r = size * 0.28;
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(r),
        gradient: const LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [Color(0xFF2C281F), Color(0xFF15130E)],
        ),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.30), width: size * 0.012 + 0.6),
        boxShadow: [
          BoxShadow(color: Colors.black.withValues(alpha: 0.35), blurRadius: size * 0.18, offset: Offset(0, size * 0.06)),
        ],
      ),
      alignment: Alignment.center,
      child: ShaderMask(
        shaderCallback: (rect) => const LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [MemoTheme.accentBright, MemoTheme.accentDeep],
        ).createShader(rect),
        child: Text(
          'M',
          style: MemoTheme.display(size * 0.62, w: FontWeight.w800, ls: -1, color: Colors.white),
        ),
      ),
    );
  }
}

/// A soft radial bronze glow — the "lamp" atmosphere behind hero content.
class LampGlow extends StatelessWidget {
  final double size;
  final double opacity;
  const LampGlow({super.key, this.size = 360, this.opacity = 0.5});

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Container(
        width: size,
        height: size,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          gradient: RadialGradient(
            colors: [
              MemoTheme.accent.withValues(alpha: 0.28 * opacity),
              MemoTheme.accent.withValues(alpha: 0.0),
            ],
            stops: const [0.0, 0.7],
          ),
        ),
      ),
    );
  }
}

/// A small status dot. The sage "running" variant breathes; others are static.
class StatusDot extends StatefulWidget {
  final Color color;
  final double size;
  final bool pulse;
  const StatusDot({super.key, required this.color, this.size = 8, this.pulse = false});

  @override
  State<StatusDot> createState() => _StatusDotState();
}

class _StatusDotState extends State<StatusDot> with SingleTickerProviderStateMixin {
  late final AnimationController _c;

  @override
  void initState() {
    super.initState();
    _c = AnimationController(vsync: this, duration: const Duration(milliseconds: 2200));
    if (widget.pulse) _c.repeat();
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final dot = Container(
      width: widget.size,
      height: widget.size,
      decoration: BoxDecoration(color: widget.color, shape: BoxShape.circle),
    );
    if (!widget.pulse) return dot;
    return SizedBox(
      width: widget.size + 8,
      height: widget.size + 8,
      child: Stack(
        alignment: Alignment.center,
        children: [
          AnimatedBuilder(
            animation: _c,
            builder: (_, _) {
              final t = Curves.easeOut.transform(_c.value);
              return Container(
                width: widget.size + 8 * t,
                height: widget.size + 8 * t,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: widget.color.withValues(alpha: (1 - t) * 0.5),
                ),
              );
            },
          ),
          dot,
        ],
      ),
    );
  }
}

/// JetBrains-Mono status pill — the recurring "engine readout" motif.
class StatPill extends StatelessWidget {
  final Widget child;
  final Color? border;
  final VoidCallback? onTap;
  final EdgeInsets padding;
  const StatPill({
    super.key,
    required this.child,
    this.border,
    this.onTap,
    this.padding = const EdgeInsets.symmetric(horizontal: 11, vertical: 7),
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.black.withValues(alpha: 0.18),
      borderRadius: BorderRadius.circular(9),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(9),
        child: Container(
          padding: padding,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(9),
            border: Border.all(color: border ?? MemoTheme.border),
          ),
          child: child,
        ),
      ),
    );
  }
}
