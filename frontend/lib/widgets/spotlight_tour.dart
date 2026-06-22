import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

class SpotlightTour extends StatefulWidget {
  final List<GlobalKey> targetKeys;
  final List<String> stepTexts;
  final VoidCallback onComplete;
  final VoidCallback onSkip;

  const SpotlightTour({
    super.key,
    required this.targetKeys,
    required this.stepTexts,
    required this.onComplete,
    required this.onSkip,
  });

  @override
  State<SpotlightTour> createState() => _SpotlightTourState();
}

class _SpotlightTourState extends State<SpotlightTour>
    with SingleTickerProviderStateMixin {
  int _currentStep = 0;
  late AnimationController _animCtrl;
  late Animation<double> _fadeAnim;
  bool _animating = false;

  @override
  void initState() {
    super.initState();
    _animCtrl = AnimationController(
      duration: const Duration(milliseconds: 280),
      vsync: this,
    );
    _fadeAnim = CurvedAnimation(parent: _animCtrl, curve: Curves.easeOut);
    _animCtrl.forward();
  }

  @override
  void dispose() {
    _animCtrl.dispose();
    super.dispose();
  }

  Future<void> _next() async {
    if (_animating) return;
    if (_currentStep < widget.targetKeys.length - 1) {
      _animating = true;
      await _animCtrl.reverse();
      if (!mounted) return;
      setState(() => _currentStep++);
      _animCtrl.forward();
      _animating = false;
    } else {
      widget.onComplete();
    }
  }

  void _skip() {
    widget.onSkip();
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);

    final key = widget.targetKeys[_currentStep];
    final ctx = key.currentContext;
    if (ctx == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        if (_currentStep < widget.targetKeys.length - 1) {
          setState(() => _currentStep++);
        } else {
          widget.onComplete();
        }
      });
      return const SizedBox.shrink();
    }

    final box = ctx.findRenderObject() as RenderBox?;
    if (box == null || !box.hasSize) return const SizedBox.shrink();

    final pos = box.localToGlobal(Offset.zero);
    final size = box.size;

    final screenHeight = MediaQuery.of(context).size.height;
    final targetBottom = pos.dy + size.height;

    return Stack(
      children: [
        GestureDetector(
          onTap: _skip,
          child: AnimatedBuilder(
            animation: _fadeAnim,
            builder: (_, child) => Opacity(
              opacity: _fadeAnim.value,
              child: child,
            ),
            child: ClipPath(
              clipper: _HoleClipper(
                holeRect: Rect.fromLTWH(
                  pos.dx - 12,
                  pos.dy - 12,
                  size.width + 24,
                  size.height + 24,
                ),
                radius: 16,
              ),
              child: Container(color: Colors.black),
            ),
          ),
        ),

        Positioned(
          left: pos.dx - 12,
          top: pos.dy - 12,
          child: IgnorePointer(
            child: AnimatedBuilder(
              animation: _fadeAnim,
              builder: (_, __) {
                final alpha = _fadeAnim.value;
                return Container(
                  width: size.width + 24,
                  height: size.height + 24,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(18),
                    border: Border.all(
                      color: MemoTheme.accent.withValues(alpha: 0.7 * alpha),
                      width: 3,
                    ),
                    boxShadow: [
                      BoxShadow(
                        color:
                            MemoTheme.accent.withValues(alpha: 0.35 * alpha),
                        blurRadius: 20,
                        spreadRadius: 8,
                      ),
                    ],
                  ),
                );
              },
            ),
          ),
        ),

        Positioned(
          left: 0,
          right: 0,
          top: _calcBubbleTop(targetBottom, pos.dy, screenHeight),
          child: FadeTransition(
              opacity: _fadeAnim,
              child: Center(
                child: Container(
                  width: 340,
                  padding: const EdgeInsets.all(20),
                  decoration: BoxDecoration(
                    color: c.bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: c.borderSoft),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.4),
                        blurRadius: 24,
                        offset: const Offset(0, 6),
                      ),
                    ],
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 8,
                            height: 8,
                            decoration: const BoxDecoration(
                              color: MemoTheme.accent,
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            '${_currentStep + 1}/${widget.targetKeys.length}',
                            style: TextStyle(
                              fontSize: 11,
                              color: c.textDim,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                          const Spacer(),
                          GestureDetector(
                            onTap: _skip,
                            child: Padding(
                              padding: const EdgeInsets.all(4),
                              child: Text(
                                L10n.t('tour_skip'),
                                style: TextStyle(
                                  fontSize: 12,
                                  color: c.textDim,
                                  decoration: TextDecoration.underline,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      Text(
                        widget.stepTexts[_currentStep],
                        style: TextStyle(
                          fontSize: 14,
                          color: c.textMain,
                          height: 1.45,
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 20),
                      SizedBox(
                        width: double.infinity,
                        child: FilledButton(
                          onPressed: _next,
                          style: FilledButton.styleFrom(
                            backgroundColor: MemoTheme.accent,
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            shape: RoundedRectangleBorder(
                              borderRadius:
                                  BorderRadius.circular(MemoTheme.radiusSm),
                            ),
                          ),
                          child: Text(
                            _currentStep < widget.targetKeys.length - 1
                                ? L10n.t('tour_next')
                                : L10n.t('tour_done'),
                            style: const TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                              color: Colors.white,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
      ],
    );
  }
  double _calcBubbleTop(double targetBottom, double targetTop, double screenH) {
    const bubbleH = 180.0;
    const gap = 24.0;
    if (targetBottom + gap + bubbleH <= screenH) {
      return targetBottom + gap;
    }
    final above = targetTop - gap - bubbleH;
    return above < gap ? gap : above;
  }
}

class _HoleClipper extends CustomClipper<Path> {
  final Rect holeRect;
  final double radius;

  const _HoleClipper({required this.holeRect, this.radius = 12});

  @override
  Path getClip(Size size) {
    final screen = Path()..addRect(Offset.zero & size);
    final hole = Path()
      ..addRRect(
        BorderRadius.circular(radius).toRRect(holeRect),
      );
    return Path.combine(PathOperation.difference, screen, hole);
  }

  @override
  bool shouldReclip(_HoleClipper oldClipper) =>
      oldClipper.holeRect != holeRect;
}
